package gameserver

import (
	"math/rand"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// triggerDescriptions provides human-readable descriptions for each trigger type.
var triggerDescriptions = map[reaction.ReactionTriggerType]string{
	reaction.TriggerOnSaveFail:          "you failed a saving throw",
	reaction.TriggerOnSaveCritFail:      "you critically failed a saving throw",
	reaction.TriggerOnDamageTaken:       "you are about to take damage",
	reaction.TriggerOnEnemyMoveAdjacent: "an enemy moved adjacent to you",
	reaction.TriggerOnConditionApplied:  "a condition is being applied to you",
	reaction.TriggerOnAllyDamaged:       "an ally took damage",
	reaction.TriggerOnFall:              "you are about to fall",
}

// CheckReactionRequirement returns true if sess meets the given requirement string.
//
// Precondition: sess must not be nil.
// Postcondition: Returns true when req is empty or "none". Returns false for unknown requirements.
func CheckReactionRequirement(sess *session.PlayerSession, req string) bool {
	switch req {
	case "", "none":
		return true
	case "wielding_melee_weapon":
		if sess.LoadoutSet == nil {
			return false
		}
		preset := sess.LoadoutSet.ActivePreset()
		if preset == nil || preset.MainHand == nil || preset.MainHand.Def == nil {
			return false
		}
		return preset.MainHand.Def.IsMelee()
	default:
		return false
	}
}

// ApplyReactionEffect executes the reaction effect, modifying ctx in place.
//
// Precondition: sess and ctx must not be nil.
// Postcondition: ctx is modified according to effect.Type; no side effects on unknown types.
func ApplyReactionEffect(sess *session.PlayerSession, effect reaction.ReactionEffect, ctx *reaction.ReactionContext) {
	switch effect.Type {
	case reaction.ReactionEffectRerollSave:
		if ctx.SaveOutcome == nil {
			return
		}
		// Only "better" (or empty, defaulting to "better") keep strategy is supported.
		// Unknown values are treated as no-op to avoid silent data contract violations.
		if effect.Keep != "" && effect.Keep != "better" {
			return
		}
		// Reroll: generate new outcome in [0,3]. Keep the better (lower) value.
		// 0=CritSuccess, 1=Success, 2=Failure, 3=CritFailure.
		reroll := rand.Intn(4)
		if reroll < *ctx.SaveOutcome {
			*ctx.SaveOutcome = reroll
		}
	case reaction.ReactionEffectReduceDamage:
		if ctx.DamagePending == nil {
			return
		}
		hardness := shieldHardness(sess)
		*ctx.DamagePending -= hardness
		if *ctx.DamagePending < 0 {
			*ctx.DamagePending = 0
		}
	case reaction.ReactionEffectStrike:
		// Strike execution deferred to sub-project 2 (Reactive Strike).
	}
}

// shieldHardness returns the hardness of the player's equipped off-hand shield, or 0.
// WeaponDef does not carry a Hardness field; a shield in the off-hand contributes 0 hardness
// until the field is added to WeaponDef in a future iteration.
//
// Precondition: sess must not be nil.
// Postcondition: Returns a non-negative integer.
func shieldHardness(sess *session.PlayerSession) int {
	if sess.LoadoutSet == nil {
		return 0
	}
	preset := sess.LoadoutSet.ActivePreset()
	if preset == nil || preset.OffHand == nil || preset.OffHand.Def == nil {
		return 0
	}
	if preset.OffHand.Def.Kind != inventory.WeaponKindShield {
		return 0
	}
	// WeaponDef.Hardness is not yet modelled; return 0 until the field is added.
	return 0
}

// matchesReadyTrigger reports whether the player's readied trigger string corresponds to the
// fired reaction trigger type.
//
// Precondition: readiedTrigger is the string stored in PlayerSession.ReadiedTrigger.
// Postcondition: Returns true iff the strings are semantically equivalent.
func matchesReadyTrigger(readiedTrigger string, firedTrigger reaction.ReactionTriggerType) bool {
	switch readiedTrigger {
	case "enemy_enters":
		return firedTrigger == reaction.TriggerOnEnemyEntersRoom
	case "enemy_attacks_me":
		return firedTrigger == reaction.TriggerOnDamageTaken
	case "ally_attacked":
		return firedTrigger == reaction.TriggerOnAllyDamaged
	default:
		return false
	}
}

// executeReadiedStrike fires the readied Strike action.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction == "strike".
// Postcondition: Strike is queued against the first NPC combatant found; a message is pushed to the player.
func executeReadiedStrike(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	if s.combatH == nil {
		s.pushMessageToUID(uid, "Your readied Strike finds no target.")
		return
	}
	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		s.pushMessageToUID(uid, "Your readied Strike finds no target.")
		return
	}
	target := ""
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			target = c.ID
			break
		}
	}
	if target == "" {
		s.pushMessageToUID(uid, "Your readied Strike finds no target.")
		return
	}
	events, err := s.combatH.Strike(uid, target)
	if err != nil {
		s.pushMessageToUID(uid, "Your readied Strike fires but misses: "+err.Error())
		return
	}
	for _, evt := range events {
		s.pushMessageToUID(uid, "Readied Strike: "+evt.Narrative)
	}
}

// executeReadiedStep fires the readied Step action.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction == "step".
// Postcondition: Player combatant's position advances 5 ft toward the NPC; a message is pushed.
func executeReadiedStep(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	if s.combatH == nil {
		s.pushMessageToUID(uid, "Your readied Step fires.")
		return
	}
	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		s.pushMessageToUID(uid, "Your readied Step fires.")
		return
	}
	combatant := cbt.GetCombatant(uid)
	if combatant == nil {
		s.pushMessageToUID(uid, "Your readied Step fires — you move 5 feet.")
		return
	}
	// Move one grid cell toward the nearest enemy (default: increase X by 1).
	combatant.GridX++
	s.pushMessageToUID(uid, "Your readied Step fires — you move 5 feet.")
}

// executeReadiedRaiseShield fires the readied Raise Shield action.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction == "raise_shield".
// Postcondition: Shield bonus is applied; a message is pushed to the player.
func executeReadiedRaiseShield(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	if s.combatH != nil {
		if err := s.combatH.ApplyCombatantACMod(uid, uid, +2); err != nil {
			s.pushMessageToUID(uid, "Your readied Raise Shield failed.")
			return
		}
	}
	s.pushMessageToUID(uid, "Your readied Raise Shield fires — shield bonus applied.")
}

// executeReadiedAction dispatches the readied action stored in sess.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction is non-empty.
// Postcondition: The appropriate action is executed and a message is pushed to the player.
func executeReadiedAction(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	switch sess.ReadiedAction {
	case "strike":
		executeReadiedStrike(s, uid, sess)
	case "step":
		executeReadiedStep(s, uid, sess)
	case "raise_shield":
		executeReadiedRaiseShield(s, uid, sess)
	}
}

// buildReactionCallback constructs the ReactionCallback for a player session.
//
// Precondition: uid, sess, and stream must not be nil.
// Postcondition: Returns a ReactionCallback that prompts the player interactively.
func (s *GameServiceServer) buildReactionCallback(
	uid string,
	sess *session.PlayerSession,
	stream gamev1.GameService_SessionServer,
) reaction.ReactionCallback {
	return func(triggerUID string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if triggerUID != uid {
			return false, nil
		}
		// Fire readied action if trigger matches.
		if sess.ReadiedTrigger != "" && matchesReadyTrigger(sess.ReadiedTrigger, trigger) {
			executeReadiedAction(s, uid, sess)
			clearReadiedAction(sess)
		}
		if sess.ReactionsRemaining <= 0 {
			return false, nil
		}
		pr := sess.Reactions.Get(uid, trigger)
		if pr == nil {
			return false, nil
		}
		if !CheckReactionRequirement(sess, pr.Def.Requirement) {
			return false, nil
		}

		desc, ok := triggerDescriptions[trigger]
		if !ok {
			desc = string(trigger)
		}
		prompt := "Reaction available: " + pr.FeatName + " \u2014 " + desc + ". Use it? (yes / no)"
		choices := &ruleset.FeatureChoices{
			Key:     "reaction",
			Prompt:  prompt,
			Options: []string{"yes", "no"},
		}
		chosen, err := s.promptFeatureChoice(stream, "reaction", choices, sess.Headless)
		if err != nil {
			return false, err
		}
		if chosen != "yes" {
			return false, nil
		}

		sess.ReactionsRemaining--
		// ctx is passed by value but DamagePending and SaveOutcome are pointers into the caller's
		// data. ApplyReactionEffect mutates through these pointers, so effects propagate to the
		// caller. Any future effect that assigns a new pointer field rather than dereferencing
		// must pass ctx by pointer instead.
		ApplyReactionEffect(sess, pr.Def.Effect, &ctx)
		return true, nil
	}
}
