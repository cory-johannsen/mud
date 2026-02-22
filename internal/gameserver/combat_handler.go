package gameserver

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// CombatHandler handles attack, flee, and NPC turn execution.
//
// Precondition: All fields must be non-nil after construction.
type CombatHandler struct {
	engine   *combat.Engine
	npcMgr   *npc.Manager
	sessions *session.Manager
	dice     *dice.Roller
}

// NewCombatHandler creates a CombatHandler.
//
// Precondition: all arguments must be non-nil.
// Postcondition: Returns a non-nil CombatHandler.
func NewCombatHandler(engine *combat.Engine, npcMgr *npc.Manager, sessions *session.Manager, diceRoller *dice.Roller) *CombatHandler {
	return &CombatHandler{engine: engine, npcMgr: npcMgr, sessions: sessions, dice: diceRoller}
}

// Attack processes a player's attack on a named NPC target.
// If no combat is active in the room, it starts one with initiative rolls.
// After the player's attack, any living NPC takes its turn immediately.
//
// Precondition: uid must be a valid connected player; target must be non-empty.
// Postcondition: Returns a slice of CombatEvents to broadcast, or an error.
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

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		cbt = h.startCombat(sess, inst)
	}

	var events []*gamev1.CombatEvent

	current := cbt.CurrentTurn()
	if current == nil {
		h.engine.EndCombat(sess.RoomID)
		return nil, fmt.Errorf("combat is over")
	}
	if current.ID != uid {
		return nil, fmt.Errorf("it's not your turn")
	}

	playerCbt := h.findCombatant(cbt, uid)
	npcCbt := h.findCombatant(cbt, inst.ID)
	if playerCbt == nil || npcCbt == nil {
		return nil, fmt.Errorf("combatant not found in combat")
	}

	atkResult := combat.ResolveAttack(playerCbt, npcCbt, h.dice.Src())
	dmg := atkResult.EffectiveDamage()
	inst.CurrentHP -= dmg
	if inst.CurrentHP < 0 {
		inst.CurrentHP = 0
	}
	npcCbt.ApplyDamage(dmg)

	events = append(events, &gamev1.CombatEvent{
		Type:        gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:    sess.CharName,
		Target:      inst.Name,
		AttackRoll:  int32(atkResult.AttackRoll),
		AttackTotal: int32(atkResult.AttackTotal),
		Outcome:     atkResult.Outcome.String(),
		Damage:      int32(dmg),
		TargetHp:    int32(inst.CurrentHP),
		Narrative:   h.attackNarrative(sess.CharName, inst.Name, atkResult),
	})

	if npcCbt.IsDead() {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH,
			Target:    inst.Name,
			Narrative: fmt.Sprintf("%s falls to the ground.", inst.Name),
		})
		if !cbt.HasLivingNPCs() {
			h.engine.EndCombat(sess.RoomID)
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
				Narrative: "Combat is over. You stand victorious.",
			})
			return events, nil
		}
	}

	cbt.AdvanceTurn()
	npcEvents := h.runNPCTurns(cbt, sess)
	events = append(events, npcEvents...)

	if playerCbt.IsDead() {
		h.engine.EndCombat(sess.RoomID)
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
			Narrative: "Everything goes dark.",
		})
	}

	return events, nil
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
		h.removeCombatant(cbt, uid)
		if !cbt.HasLivingPlayers() {
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

func (h *CombatHandler) startCombat(sess *session.PlayerSession, inst *npc.Instance) *combat.Combat {
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

	cbt, _ := h.engine.StartCombat(sess.RoomID, combatants)
	return cbt
}

func (h *CombatHandler) runNPCTurns(cbt *combat.Combat, sess *session.PlayerSession) []*gamev1.CombatEvent {
	var events []*gamev1.CombatEvent
	for {
		current := cbt.CurrentTurn()
		if current == nil || current.Kind == combat.KindPlayer {
			break
		}

		playerCbt := h.findCombatant(cbt, sess.UID)
		if playerCbt == nil {
			break
		}

		atkResult := combat.ResolveAttack(current, playerCbt, h.dice.Src())
		dmg := atkResult.EffectiveDamage()
		playerCbt.ApplyDamage(dmg)
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}

		events = append(events, &gamev1.CombatEvent{
			Type:        gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:    current.Name,
			Target:      sess.CharName,
			AttackRoll:  int32(atkResult.AttackRoll),
			AttackTotal: int32(atkResult.AttackTotal),
			Outcome:     atkResult.Outcome.String(),
			Damage:      int32(dmg),
			TargetHp:    int32(sess.CurrentHP),
			Narrative:   h.attackNarrative(current.Name, sess.CharName, atkResult),
		})

		if playerCbt.IsDead() {
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH,
				Target:    sess.CharName,
				Narrative: fmt.Sprintf("%s is incapacitated!", sess.CharName),
			})
			break
		}

		cbt.AdvanceTurn()
	}
	return events
}

func (h *CombatHandler) findCombatant(cbt *combat.Combat, id string) *combat.Combatant {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

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

func (h *CombatHandler) removeCombatant(cbt *combat.Combat, id string) {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			c.CurrentHP = 0
			return
		}
	}
}

func (h *CombatHandler) attackNarrative(attacker, target string, result combat.AttackResult) string {
	switch result.Outcome {
	case combat.CritSuccess:
		return fmt.Sprintf("%s lands a devastating blow on %s for %d damage!", attacker, target, result.EffectiveDamage())
	case combat.Success:
		return fmt.Sprintf("%s hits %s for %d damage.", attacker, target, result.EffectiveDamage())
	case combat.Failure:
		return fmt.Sprintf("%s swings at %s but misses.", attacker, target)
	default:
		return fmt.Sprintf("%s fumbles their attack against %s.", attacker, target)
	}
}
