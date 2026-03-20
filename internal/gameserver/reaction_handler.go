package gameserver

import (
	"math/rand"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
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
		reroll := rand.Intn(4) // 0=CritSuccess, 1=Success, 2=Failure, 3=CritFailure
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
		chosen, err := s.promptFeatureChoice(stream, "reaction", choices)
		if err != nil {
			return false, err
		}
		if chosen != "yes" {
			return false, nil
		}

		sess.ReactionsRemaining--
		ApplyReactionEffect(sess, pr.Def.Effect, &ctx)
		return true, nil
	}
}
