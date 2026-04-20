package gameserver

import (
	"fmt"
	"math/rand"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// equippedAIItem records one equipped AI item awaiting its combat turn.
type equippedAIItem struct {
	instanceID string
	itemDefID  string
	domain     string
	script     string
}

// armorSlotOrder defines the ordered iteration for armor slots (REQ-AIE-4).
var armorSlotOrder = []inventory.ArmorSlot{
	inventory.SlotHead, inventory.SlotLeftArm, inventory.SlotRightArm,
	inventory.SlotTorso, inventory.SlotHands, inventory.SlotLeftLeg,
	inventory.SlotRightLeg, inventory.SlotFeet,
}

// collectEquippedAIItems returns all equipped items with a non-empty CombatDomain,
// in slot order: main hand → off hand → armor slots → accessories.
// Precondition: sess must not be nil; h.invRegistry must not be nil.
func (h *CombatHandler) collectEquippedAIItems(sess *session.PlayerSession) []equippedAIItem {
	var items []equippedAIItem

	addFromItemDef := func(instanceID, itemDefID string) {
		if itemDefID == "" {
			return
		}
		def, ok := h.invRegistry.Item(itemDefID)
		if !ok || def.CombatDomain == "" {
			return
		}
		items = append(items, equippedAIItem{
			instanceID: instanceID,
			itemDefID:  itemDefID,
			domain:     def.CombatDomain,
			script:     def.CombatScript,
		})
	}

	// Main hand and off hand.
	if sess.LoadoutSet != nil {
		if preset := sess.LoadoutSet.ActivePreset(); preset != nil {
			if preset.MainHand != nil {
				addFromItemDef(preset.MainHand.InstanceID, preset.MainHand.ItemDefID)
			}
			if preset.OffHand != nil {
				addFromItemDef(preset.OffHand.InstanceID, preset.OffHand.ItemDefID)
			}
		}
	}

	// Armor slots in order.
	if sess.Equipment != nil {
		for _, slot := range armorSlotOrder {
			slotted := sess.Equipment.Armor[slot]
			if slotted == nil {
				continue
			}
			addFromItemDef(slotted.InstanceID, slotted.ItemDefID)
		}
		// Accessories (map iteration order is undefined; stable sort not required by spec).
		for _, slotted := range sess.Equipment.Accessories {
			if slotted == nil {
				continue
			}
			addFromItemDef(slotted.InstanceID, slotted.ItemDefID)
		}
	}

	return items
}

// runAIItemPhaseLocked executes the AI item phase for all player combatants in cbt.
// Must be called with combatMu held. Runs after StartRound (AP queues reset) and
// before player AP notification. Satisfies REQ-AIE-3 and REQ-AIE-4.
//
// Precondition: cbt must not be nil; combatMu must be held by caller.
// Postcondition: Each equipped AI item has contributed +1 AP and run its HTN operator.
// Returns CombatEvents for damage and speech produced during item turns.
func (h *CombatHandler) runAIItemPhaseLocked(cbt *combat.Combat) []*gamev1.CombatEvent {
	if h.aiItemRegistry == nil || h.invRegistry == nil {
		return nil
	}

	var events []*gamev1.CombatEvent

	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer || c.IsDead() {
			continue
		}
		sess, ok := h.sessions.GetPlayer(c.ID)
		if !ok {
			continue
		}

		aiItems := h.collectEquippedAIItems(sess)
		if len(aiItems) == 0 {
			continue
		}

		// REQ-AIE-3: add 1 AP per AI item to the player's queue.
		q := cbt.ActionQueues[c.ID]
		if q != nil {
			q.AddAP(len(aiItems))
			q.MaxPoints += len(aiItems)
		}

		// REQ-AIE-4: execute each AI item's turn.
		for _, item := range aiItems {
			planner, ok := h.aiItemRegistry.PlannerFor(item.domain)
			if !ok {
				continue
			}

			// Build combat snapshot for this item.
			snap := h.buildItemCombatSnapshot(c, cbt, q)

			// Get or initialize per-encounter script state.
			var scriptState map[string]interface{}
			if sess.Backpack != nil {
				mi := sess.Backpack.MutableItem(item.instanceID)
				if mi != nil {
					if mi.CombatScriptState == nil {
						mi.CombatScriptState = make(map[string]interface{})
					}
					scriptState = mi.CombatScriptState
				}
			}
			if scriptState == nil {
				scriptState = make(map[string]interface{})
			}

			// Build callbacks.
			itemCbs := h.buildItemPrimitiveCalls(c, cbt, q, item)

			// Execute the HTN plan.
			_ = planner.Execute(item.script, scriptState, snap, itemCbs.cbs)
			events = append(events, itemCbs.getEvents()...)
		}
	}

	return events
}

// itemPhaseCallbacks packages the callbacks and an event getter for one item turn.
type itemPhaseCallbacks struct {
	cbs       ai.ItemPrimitiveCalls
	getEvents func() []*gamev1.CombatEvent
}

// buildItemPrimitiveCalls constructs the ItemPrimitiveCalls for one AI item turn.
// The returned getEvents() function must be called AFTER Execute to retrieve
// collected events (events are accumulated by the closure callbacks during Execute).
func (h *CombatHandler) buildItemPrimitiveCalls(
	actor *combat.Combatant,
	cbt *combat.Combat,
	q *combat.ActionQueue,
	item equippedAIItem,
) itemPhaseCallbacks {
	var collectedEvents []*gamev1.CombatEvent

	spendAP := func(n int) bool {
		if q == nil {
			return false
		}
		if q.RemainingPoints() < n {
			return false
		}
		_ = q.DeductAP(n)
		return true
	}

	attack := func(targetID, formula string, cost int) bool {
		if cost > 0 && !spendAP(cost) {
			return false
		}
		target := cbt.GetCombatant(targetID)
		if target == nil {
			return false
		}
		result, err := h.dice.RollExpr(formula)
		if err != nil {
			return false
		}
		dmg := result.Total()
		if dmg < 1 {
			dmg = 1
		}
		target.CurrentHP -= dmg
		if target.CurrentHP < 0 {
			target.CurrentHP = 0
		}
		cbt.RecordDamage(actor.ID, dmg)
		collectedEvents = append(collectedEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  item.itemDefID,
			Target:    target.Name,
			Damage:    int32(dmg),
			Narrative: fmt.Sprintf("%s attacks %s for %d damage.", item.itemDefID, target.Name, dmg),
		})
		return true
	}

	say := func(textPool []string) {
		if len(textPool) == 0 {
			return
		}
		line := textPool[rand.Intn(len(textPool))]
		collectedEvents = append(collectedEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  item.itemDefID,
			Narrative: line,
		})
	}

	applyCondition := func(targetID, effectID string, rounds, cost int) bool {
		if cost > 0 && !spendAP(cost) {
			return false
		}
		if err := cbt.ApplyCondition(targetID, effectID, 1, rounds); err != nil {
			return false
		}
		collectedEvents = append(collectedEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_CONDITION,
			Target:    targetID,
			Narrative: fmt.Sprintf("%s applies %s to %s.", item.itemDefID, effectID, targetID),
		})
		return true
	}

	return itemPhaseCallbacks{
		cbs: ai.ItemPrimitiveCalls{
			Attack:  attack,
			Say:     say,
			Buff:    applyCondition,
			Debuff:  applyCondition,
			GetAP:   func() int { if q == nil { return 0 }; return q.RemainingPoints() },
			SpendAP: spendAP,
		},
		getEvents: func() []*gamev1.CombatEvent { return collectedEvents },
	}
}

// buildItemCombatSnapshot constructs the ItemCombatSnapshot for one player's item turn.
func (h *CombatHandler) buildItemCombatSnapshot(
	player *combat.Combatant,
	cbt *combat.Combat,
	q *combat.ActionQueue,
) ai.ItemCombatSnapshot {
	var enemies []ai.ItemEnemySnapshot
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}
		var condIDs []string
		if set := cbt.Conditions[c.ID]; set != nil {
			for _, ac := range set.All() {
				condIDs = append(condIDs, ac.Def.ID)
			}
		}
		enemies = append(enemies, ai.ItemEnemySnapshot{
			ID:         c.ID,
			Name:       c.Name,
			HP:         c.CurrentHP,
			MaxHP:      c.MaxHP,
			Conditions: condIDs,
		})
	}

	ap := 0
	if q != nil {
		ap = q.RemainingPoints()
	}

	return ai.ItemCombatSnapshot{
		Enemies: enemies,
		Player: ai.ItemPlayerSnapshot{
			ID:    player.ID,
			HP:    player.CurrentHP,
			MaxHP: player.MaxHP,
			AP:    ap,
		},
		Round: cbt.Round,
	}
}

// clearAIItemCombatStates clears CombatScriptState for all equipped AI items of a player.
// Called when combat ends (win, loss, flee). Satisfies REQ-AIE-2.
// Precondition: sess must not be nil.
func (h *CombatHandler) clearAIItemCombatStates(sess *session.PlayerSession) {
	if sess.Backpack == nil {
		return
	}
	aiItems := h.collectEquippedAIItems(sess)
	for _, item := range aiItems {
		mi := sess.Backpack.MutableItem(item.instanceID)
		if mi != nil {
			mi.CombatScriptState = nil
		}
	}
}
