package gameserver

import (
	"fmt"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/scripting"
)

// CombatHandler handles attack, strike, pass, flee, and round timer management
// for the 3-action-point economy (Stage 4).
//
// Precondition: All fields must be non-nil after construction.
//
// combatMu serialises all access to combat state (Combat structs, ActionQueues)
// and timer management so that the timer goroutine and caller goroutines cannot
// race on shared mutable data.
type CombatHandler struct {
	engine        *combat.Engine
	npcMgr        *npc.Manager
	sessions      *session.Manager
	dice          *dice.Roller
	broadcastFn   func(roomID string, events []*gamev1.CombatEvent)
	roundDuration time.Duration
	condRegistry  *condition.Registry
	worldMgr      *world.Manager
	scriptMgr     *scripting.Manager
	invRegistry   *inventory.Registry
	combatMu      sync.Mutex
	timersMu      sync.Mutex
	timers        map[string]*combat.RoundTimer
	loadoutsMu    sync.Mutex
	loadouts      map[string]*inventory.Loadout
}

// NewCombatHandler creates a CombatHandler with a round timer and broadcast function.
//
// Precondition: all pointer arguments except invRegistry must be non-nil; roundDuration must be > 0.
// Postcondition: Returns a non-nil CombatHandler.
func NewCombatHandler(
	engine *combat.Engine,
	npcMgr *npc.Manager,
	sessions *session.Manager,
	diceRoller *dice.Roller,
	broadcastFn func(roomID string, events []*gamev1.CombatEvent),
	roundDuration time.Duration,
	condRegistry *condition.Registry,
	worldMgr *world.Manager,
	scriptMgr *scripting.Manager,
	invRegistry *inventory.Registry,
) *CombatHandler {
	return &CombatHandler{
		engine:        engine,
		npcMgr:        npcMgr,
		sessions:      sessions,
		dice:          diceRoller,
		broadcastFn:   broadcastFn,
		roundDuration: roundDuration,
		condRegistry:  condRegistry,
		worldMgr:      worldMgr,
		scriptMgr:     scriptMgr,
		invRegistry:   invRegistry,
		timers:        make(map[string]*combat.RoundTimer),
		loadouts:      make(map[string]*inventory.Loadout),
	}
}

// Attack queues a 1-AP ActionAttack for uid against target.
// If no combat is active in the room, it is started first.
// If AllActionsSubmitted after queuing, the round resolves immediately.
//
// Precondition: uid must be a valid connected player; target must be non-empty.
// Postcondition: Returns events to return to the requesting player, or an error.
func (h *CombatHandler) Attack(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return nil, fmt.Errorf("you don't see %q here", target)
	}
	if inst.IsDead() {
		return nil, fmt.Errorf("%s is already dead", inst.Name)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	var initEvents []*gamev1.CombatEvent
	if !ok {
		var err error
		cbt, initEvents, err = h.startCombatLocked(sess, inst)
		if err != nil {
			return nil, err
		}
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAttack, Target: inst.Name}); err != nil {
		return nil, fmt.Errorf("queuing attack: %w", err)
	}

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Target:    inst.Name,
		Narrative: fmt.Sprintf("%s readies an attack against %s.", sess.CharName, inst.Name),
	}

	if len(initEvents) > 0 {
		// Combat was just started — auto-queue NPCs and start timer.
		h.autoQueueNPCsLocked(cbt)
		h.startTimerLocked(sess.RoomID)
		return append(initEvents, confirmEvent), nil
	}

	// Combat was already active.
	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
		return []*gamev1.CombatEvent{confirmEvent}, nil
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}

// Strike queues a 2-AP ActionStrike for uid against target.
// Requires active combat. Early-resolves if all actions submitted.
//
// Precondition: uid must be a valid connected player in active combat; target must be non-empty.
// Postcondition: Returns events to return to the requesting player, or an error.
func (h *CombatHandler) Strike(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionStrike, Target: target}); err != nil {
		return nil, fmt.Errorf("queuing strike: %w", err)
	}

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Target:    target,
		Narrative: fmt.Sprintf("%s launches a full strike against %s.", sess.CharName, target),
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
		return []*gamev1.CombatEvent{confirmEvent}, nil
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}

// Pass forfeits uid's remaining AP for this round.
// Requires active combat. Early-resolves if all actions submitted.
//
// Precondition: uid must be a valid connected player in active combat.
// Postcondition: Returns events to return to the requesting player, or an error.
func (h *CombatHandler) Pass(uid string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		return nil, fmt.Errorf("queuing pass: %w", err)
	}

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Narrative: fmt.Sprintf("%s holds their ground.", sess.CharName),
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
		return []*gamev1.CombatEvent{confirmEvent}, nil
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}

// Flee attempts to remove the player from combat using an opposed Athletics check.
//
// Precondition: uid must be a valid connected player in active combat.
// Postcondition: Returns events describing the flee attempt outcome.
func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	playerCbt := h.findCombatant(cbt, uid)
	if playerCbt == nil {
		return nil, fmt.Errorf("you are not a combatant")
	}

	playerRoll, _ := h.dice.RollExpr("d20")
	playerTotal := playerRoll.Total() + playerCbt.StrMod

	bestNPC := h.bestNPCCombatant(cbt)
	npcTotal := 0
	if bestNPC != nil {
		npcRoll, _ := h.dice.RollExpr("d20")
		npcTotal = npcRoll.Total() + bestNPC.StrMod
	}

	var events []*gamev1.CombatEvent
	if playerTotal > npcTotal {
		// removeCombatant sets CurrentHP=0 (dead) so ResolveRound skips the fleeing player.
		// Safe: entire Flee path holds combatMu; pending timer callback will no-op
		// (GetCombat returns false after EndCombat).
		h.removeCombatant(cbt, uid)
		if !cbt.HasLivingPlayers() {
			h.stopTimerLocked(sess.RoomID)
			h.engine.EndCombat(sess.RoomID)
		}
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
			Attacker:  sess.CharName,
			Narrative: fmt.Sprintf("%s breaks free and runs!", sess.CharName),
		})
	} else {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
			Attacker:  sess.CharName,
			Narrative: fmt.Sprintf("%s tries to flee but can't escape!", sess.CharName),
		})
	}
	return events, nil
}

// Equip equips weaponID into the given slot for uid.
//
// Precondition: uid must be non-empty; weaponID must identify a registered weapon.
// Postcondition: Returns nil events and nil error on success; the loadout is updated.
func (h *CombatHandler) Equip(uid, weaponID, slotName string) ([]*gamev1.CombatEvent, error) {
	def := h.invRegistry.Weapon(weaponID)
	if def == nil {
		return nil, fmt.Errorf("weapon %q not found", weaponID)
	}

	slot := inventory.SlotPrimary
	if slotName != "" {
		slot = inventory.Slot(slotName)
	}

	h.loadoutsMu.Lock()
	lo, ok := h.loadouts[uid]
	if !ok {
		lo = inventory.NewLoadout()
		h.loadouts[uid] = lo
	}
	if err := lo.Equip(slot, def); err != nil {
		h.loadoutsMu.Unlock()
		return nil, fmt.Errorf("equipping weapon: %w", err)
	}
	h.loadoutsMu.Unlock()

	// If player is in active combat, update their Combatant.Loadout.
	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	// Retrieve session to find roomID.
	sess, ok := h.sessions.GetPlayer(uid)
	if ok {
		if cbt, active := h.engine.GetCombat(sess.RoomID); active {
			if combatant := h.findCombatant(cbt, uid); combatant != nil {
				combatant.Loadout = lo
			}
		}
	}

	return nil, nil
}

// Reload queues ActionReload for uid.
//
// Precondition: uid must be a valid connected player in active combat with a primary weapon equipped.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) Reload(uid string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	h.loadoutsMu.Lock()
	lo := h.loadouts[uid]
	h.loadoutsMu.Unlock()

	var weaponID string
	if lo != nil {
		if primary := lo.Primary(); primary != nil {
			weaponID = primary.Def.ID
		}
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionReload, WeaponID: weaponID}); err != nil {
		return nil, fmt.Errorf("queuing reload: %w", err)
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// FireBurst queues ActionFireBurst for uid against target.
//
// Precondition: uid must be a valid connected player in active combat; target must be non-empty.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) FireBurst(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	h.loadoutsMu.Lock()
	lo := h.loadouts[uid]
	h.loadoutsMu.Unlock()

	var weaponID string
	if lo != nil {
		if primary := lo.Primary(); primary != nil {
			weaponID = primary.Def.ID
		}
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionFireBurst, Target: target, WeaponID: weaponID}); err != nil {
		return nil, fmt.Errorf("queuing fire burst: %w", err)
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// FireAutomatic queues ActionFireAutomatic for uid against target.
//
// Precondition: uid must be a valid connected player in active combat; target must be non-empty.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) FireAutomatic(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	h.loadoutsMu.Lock()
	lo := h.loadouts[uid]
	h.loadoutsMu.Unlock()

	var weaponID string
	if lo != nil {
		if primary := lo.Primary(); primary != nil {
			weaponID = primary.Def.ID
		}
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionFireAutomatic, Target: target, WeaponID: weaponID}); err != nil {
		return nil, fmt.Errorf("queuing fire automatic: %w", err)
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// Throw queues ActionThrow for uid using explosiveID.
//
// Precondition: uid must be a valid connected player in active combat; explosiveID must identify a registered explosive.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) Throw(uid, explosiveID string) ([]*gamev1.CombatEvent, error) {
	if h.invRegistry.Explosive(explosiveID) == nil {
		return nil, fmt.Errorf("explosive %q not found", explosiveID)
	}

	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionThrow, ExplosiveID: explosiveID}); err != nil {
		return nil, fmt.Errorf("queuing throw: %w", err)
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// resolveAndAdvance is the timer-fired entry point. It acquires combatMu, then
// delegates to resolveAndAdvanceLocked.
//
// Precondition: a combat must be active in roomID.
// Postcondition: round events are broadcast; combat is ended or next round is started.
func (h *CombatHandler) resolveAndAdvance(roomID string) {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return
	}
	h.resolveAndAdvanceLocked(roomID, cbt)
}

// resolveAndAdvanceLocked resolves the current round and either ends combat or
// starts the next round. Caller must hold combatMu.
//
// Precondition: combatMu is held; cbt is the active Combat for roomID.
// Postcondition: round events are broadcast; combat is ended or next round is started.
func (h *CombatHandler) resolveAndAdvanceLocked(roomID string, cbt *combat.Combat) []*gamev1.CombatEvent {
	targetUpdater := func(id string, hp int) {
		if inst, found := h.npcMgr.Get(id); found {
			inst.CurrentHP = hp
		}
		if sess, found := h.sessions.GetPlayer(id); found {
			sess.CurrentHP = hp
		}
	}

	roundEvents := combat.ResolveRound(cbt, h.dice.Src(), targetUpdater)

	var events []*gamev1.CombatEvent
	for _, re := range roundEvents {
		events = append(events, h.roundEventToProto(re))
	}

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Narrative: fmt.Sprintf("Round %d complete.", cbt.Round),
	})

	if !cbt.HasLivingNPCs() || !cbt.HasLivingPlayers() {
		var endNarrative string
		if !cbt.HasLivingNPCs() {
			endNarrative = "Combat is over. You stand victorious."
		} else {
			endNarrative = "Everything goes dark."
		}
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
			Narrative: endNarrative,
		})
		h.broadcastFn(roomID, events)
		h.engine.EndCombat(roomID)
		return events
	}

	h.broadcastFn(roomID, events)

	// Start the next round.
	condEvents := cbt.StartRound(3)
	condCombatEvents := conditionEventsToProto(condEvents, h.condRegistry)
	h.autoQueueNPCsLocked(cbt)

	roundStartEvents := []*gamev1.CombatEvent{
		{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Narrative: fmt.Sprintf("Round %d begins!", cbt.Round),
		},
	}
	roundStartEvents = append(roundStartEvents, condCombatEvents...)
	h.broadcastFn(roomID, roundStartEvents)
	h.startTimerLocked(roomID)
	return events
}

// startCombatLocked initialises a new combat encounter for sess attacking inst.
// Caller must hold combatMu.
//
// Precondition: combatMu is held; sess and inst must be non-nil.
// Postcondition: combat is registered in the engine; StartRound(3) is called.
func (h *CombatHandler) startCombatLocked(sess *session.PlayerSession, inst *npc.Instance) (*combat.Combat, []*gamev1.CombatEvent, error) {
	// Placeholder defaults: AC/Level/StrMod/DexMod will come from character sheet once Stage 7 (inventory) is complete.
	playerCbt := &combat.Combatant{
		ID:        sess.UID,
		Kind:      combat.KindPlayer,
		Name:      sess.CharName,
		MaxHP:     sess.CurrentHP,
		CurrentHP: sess.CurrentHP,
		AC:        12,
		Level:     1,
		StrMod:    2,
		DexMod:    1,
	}

	h.loadoutsMu.Lock()
	if lo, ok := h.loadouts[sess.UID]; ok {
		playerCbt.Loadout = lo
	}
	h.loadoutsMu.Unlock()

	npcCbt := &combat.Combatant{
		ID:        inst.ID,
		Kind:      combat.KindNPC,
		Name:      inst.Name,
		MaxHP:     inst.MaxHP,
		CurrentHP: inst.CurrentHP,
		AC:        inst.AC,
		Level:     inst.Level,
		StrMod:    combat.AbilityMod(inst.Perception),
		DexMod:    1,
	}

	combatants := []*combat.Combatant{playerCbt, npcCbt}
	combat.RollInitiative(combatants, h.dice.Src())

	var scriptMgr *scripting.Manager
	var zoneID string
	if h.scriptMgr != nil && h.worldMgr != nil {
		scriptMgr = h.scriptMgr
		if room, ok := h.worldMgr.GetRoom(sess.RoomID); ok {
			zoneID = room.ZoneID
		}
	}
	cbt, err := h.engine.StartCombat(sess.RoomID, combatants, h.condRegistry, scriptMgr, zoneID)
	if err != nil {
		return nil, nil, fmt.Errorf("starting combat: %w", err)
	}

	initCondEvents := cbt.StartRound(3)
	_ = initCondEvents // round 1 starts with no active conditions; events are empty

	// Build initiative events.
	var events []*gamev1.CombatEvent
	for _, c := range cbt.Combatants {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Attacker:  c.Name,
			Narrative: fmt.Sprintf("%s rolls initiative: %d", c.Name, c.Initiative),
		})
	}

	turnOrder := make([]string, 0, len(cbt.Combatants))
	for _, c := range cbt.Combatants {
		turnOrder = append(turnOrder, c.Name)
	}

	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
		Narrative: fmt.Sprintf("Round %d begins! Turn order: %v", cbt.Round, turnOrder),
	})

	return cbt, events, nil
}

// autoQueueNPCsLocked queues ActionAttack for every living NPC in cbt, targeting
// the first living player by name. Caller must hold combatMu.
//
// Precondition: combatMu is held; cbt must be non-nil.
// Postcondition: Each living NPC has ActionAttack queued for this round.
func (h *CombatHandler) autoQueueNPCsLocked(cbt *combat.Combat) {
	var playerName string
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer && !c.IsDead() {
			playerName = c.Name
			break
		}
	}
	if playerName == "" {
		return
	}

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC && !c.IsDead() {
			// Ignore errors — NPC may already have its AP queued.
			_ = cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionAttack, Target: playerName})
		}
	}
}

// startTimerLocked starts or replaces the round timer for roomID.
// Caller must hold combatMu. The timer callback acquires combatMu independently.
//
// Precondition: combatMu is held; roomID must be non-empty.
// Postcondition: A running RoundTimer is stored for roomID.
func (h *CombatHandler) startTimerLocked(roomID string) {
	h.timersMu.Lock()
	if existing, ok := h.timers[roomID]; ok {
		existing.Stop()
	}
	h.timers[roomID] = combat.NewRoundTimer(h.roundDuration, func() {
		h.resolveAndAdvance(roomID)
	})
	h.timersMu.Unlock()
}

// stopTimerLocked stops and removes the round timer for roomID.
// Caller must hold combatMu.
//
// Precondition: combatMu is held; roomID must be non-empty.
// Postcondition: No running timer for roomID remains.
func (h *CombatHandler) stopTimerLocked(roomID string) {
	h.timersMu.Lock()
	if t, ok := h.timers[roomID]; ok {
		t.Stop()
		delete(h.timers, roomID)
	}
	h.timersMu.Unlock()
}

// cancelTimer stops and removes the round timer for roomID without requiring
// combatMu to be held. Safe to call from tests or external code.
//
// Postcondition: No running timer for roomID remains.
func (h *CombatHandler) cancelTimer(roomID string) {
	h.timersMu.Lock()
	if t, ok := h.timers[roomID]; ok {
		t.Stop()
		delete(h.timers, roomID)
	}
	h.timersMu.Unlock()
}

// roundEventToProto converts a combat.RoundEvent to a gamev1.CombatEvent.
//
// Postcondition: Returns a non-nil CombatEvent.
func (h *CombatHandler) roundEventToProto(re combat.RoundEvent) *gamev1.CombatEvent {
	if re.AttackResult == nil {
		return &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  re.ActorName,
			Narrative: re.Narrative,
		}
	}

	r := re.AttackResult
	dmg := r.EffectiveDamage()

	evt := &gamev1.CombatEvent{
		Type:        gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:    re.ActorName,
		AttackRoll:  int32(r.AttackRoll),
		AttackTotal: int32(r.AttackTotal),
		Outcome:     r.Outcome.String(),
		Damage:      int32(dmg),
		Narrative:   re.Narrative,
	}

	// Resolve target name and HP from managers.
	if r.TargetID != "" {
		if inst, ok := h.npcMgr.Get(r.TargetID); ok {
			evt.Target = inst.Name
			evt.TargetHp = int32(inst.CurrentHP)
		}
		if sess, ok := h.sessions.GetPlayer(r.TargetID); ok {
			evt.Target = sess.CharName
			evt.TargetHp = int32(sess.CurrentHP)
		}
	}

	return evt
}

// findCombatant returns the Combatant with the given id, or nil.
func (h *CombatHandler) findCombatant(cbt *combat.Combat, id string) *combat.Combatant {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// bestNPCCombatant returns the living NPC combatant with the highest StrMod.
func (h *CombatHandler) bestNPCCombatant(cbt *combat.Combat) *combat.Combatant {
	var best *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC && !c.IsDead() {
			if best == nil || c.StrMod > best.StrMod {
				best = c
			}
		}
	}
	return best
}

// removeCombatant sets a combatant's HP to 0, marking them as dead/removed.
//
// Setting CurrentHP=0 marks the combatant as dead so ResolveRound skips them
// in subsequent resolution passes. This is safe because the entire Flee path
// holds combatMu; any pending timer callback will no-op because GetCombat
// returns false after EndCombat is called.
func (h *CombatHandler) removeCombatant(cbt *combat.Combat, id string) {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			c.CurrentHP = 0
			c.Dead = true
			return
		}
	}
}

// conditionEventsToProto converts a slice of RoundConditionEvents into CombatEvents
// for broadcast using the narrative channel.
//
// Precondition: reg must not be nil.
// Postcondition: Returns one CombatEvent per RoundConditionEvent.
func conditionEventsToProto(events []combat.RoundConditionEvent, reg *condition.Registry) []*gamev1.CombatEvent {
	result := make([]*gamev1.CombatEvent, 0, len(events))
	for _, ce := range events {
		def, _ := reg.Get(ce.ConditionID)
		name := ce.ConditionID
		if def != nil {
			name = def.Name
		}
		var narrative string
		if ce.Applied {
			narrative = fmt.Sprintf("%s is now %s (stacks: %d).", ce.Name, name, ce.Stacks)
		} else {
			narrative = fmt.Sprintf("%s fades from %s.", name, ce.Name)
		}
		result = append(result, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_CONDITION,
			Narrative: narrative,
		})
	}
	return result
}

// Status returns the active conditions for the player with the given uid.
// Returns nil, nil if no combat is active in the player's room.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns active conditions or nil if not in combat.
func (h *CombatHandler) Status(uid string) ([]*condition.ActiveCondition, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, nil // no combat active; return empty
	}
	return cbt.GetConditions(uid), nil
}
