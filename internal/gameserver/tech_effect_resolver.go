package gameserver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// diceSrcAdapter wraps a combat.Source so it satisfies dice.Source.
// Both interfaces require Intn(int) int — this adapter is structurally redundant
// but required because Go does not unify separately-defined identical interfaces.
type diceSrcAdapter struct{ src combat.Source }

func (a diceSrcAdapter) Intn(n int) int { return a.src.Intn(n) }

// ResolveTechEffects resolves all effects for a tech activation and returns a slice of
// human-readable result messages (one per target). Does not expend uses — the caller
// has already done that.
//
// Preconditions:
//   - sess must be non-nil.
//   - tech must be non-nil.
//   - targets is empty for self/utility/no-roll; one entry for single; all enemies for area.
//   - cbt may be nil when not in combat; condition effects are silently skipped when nil.
//   - condRegistry may be nil; condition effects are silently skipped when nil.
//   - src must be non-nil (satisfies combat.Source: Intn method).
//
// Postconditions:
//   - Returns at least one message.
//   - target.CurrentHP and sess.CurrentHP never go below 0.
//   - sess.CurrentHP never exceeds sess.MaxHP.
func ResolveTechEffects(
	sess *session.PlayerSession,
	tech *technology.TechnologyDef,
	targets []*combat.Combatant,
	cbt *combat.Combat,
	condRegistry *condition.Registry,
	src combat.Source,
) []string {
	if len(targets) == 0 {
		msgs := applyEffects(sess, tech.Effects.OnApply, nil, cbt, condRegistry, src)
		if len(msgs) == 0 {
			msgs = append(msgs, "No effect.")
		}
		return msgs
	}

	var msgs []string
	for _, target := range targets {
		var tier []technology.TechEffect
		var label string

		switch tech.Resolution {
		case "save":
			outcome := combat.ResolveSave(tech.SaveType, target, tech.SaveDC, src)
			tier, label = selectSaveTier(tech.Effects, outcome, target.Name)
		case "attack":
			outcome := resolveAttackRoll(sess, tech, target, src)
			tier, label = selectAttackTier(tech.Effects, outcome, target.Name)
		default: // "none" or ""
			tier = tech.Effects.OnApply
			label = ""
		}

		effectMsgs := applyEffects(sess, tier, target, cbt, condRegistry, src)
		if len(effectMsgs) == 0 {
			if label != "" {
				msgs = append(msgs, label+"No effect.")
			} else {
				msgs = append(msgs, "No effect.")
			}
		} else {
			for _, m := range effectMsgs {
				if label != "" {
					msgs = append(msgs, label+m)
				} else {
					msgs = append(msgs, m)
				}
			}
		}
	}
	if len(msgs) == 0 {
		msgs = append(msgs, "Nothing happens.")
	}
	return msgs
}

// selectSaveTier returns the effect tier and a prefix label for a save outcome.
//
// CritSuccess falls back to OnSuccess when OnCritSuccess is empty.
// CritFailure falls back to OnFailure when OnCritFailure is empty.
// This mirrors PF2E convention: a more extreme outcome uses the adjacent tier's effects
// when the extreme tier has no defined effects.
func selectSaveTier(effects technology.TieredEffects, outcome combat.Outcome, targetName string) ([]technology.TechEffect, string) {
	switch outcome {
	case combat.CritSuccess:
		if len(effects.OnCritSuccess) > 0 {
			return effects.OnCritSuccess, fmt.Sprintf("%s critically succeeds: ", targetName)
		}
		return effects.OnSuccess, fmt.Sprintf("%s succeeds: ", targetName)
	case combat.Success:
		return effects.OnSuccess, fmt.Sprintf("%s succeeds: ", targetName)
	case combat.Failure:
		return effects.OnFailure, fmt.Sprintf("%s fails: ", targetName)
	case combat.CritFailure:
		if len(effects.OnCritFailure) > 0 {
			return effects.OnCritFailure, fmt.Sprintf("%s critically fails: ", targetName)
		}
		return effects.OnFailure, fmt.Sprintf("%s fails: ", targetName)
	default:
		return nil, ""
	}
}

// selectAttackTier returns the effect tier and a prefix label for an attack outcome.
func selectAttackTier(effects technology.TieredEffects, outcome combat.Outcome, targetName string) ([]technology.TechEffect, string) {
	switch outcome {
	case combat.CritSuccess:
		return effects.OnCritHit, fmt.Sprintf("Critical hit on %s: ", targetName)
	case combat.Success:
		return effects.OnHit, fmt.Sprintf("Hit %s: ", targetName)
	default:
		return effects.OnMiss, fmt.Sprintf("Missed %s.", targetName)
	}
}

// resolveAttackRoll resolves an attack roll for tech vs target.
// Formula: 1d20 + techAttackMod(sess, tech) vs target.AC.
func resolveAttackRoll(sess *session.PlayerSession, tech *technology.TechnologyDef, target *combat.Combatant, src combat.Source) combat.Outcome {
	roll := src.Intn(20) + 1
	total := roll + techAttackMod(sess, tech)
	return combat.OutcomeFor(total, target.AC)
}

// techAttackMod returns the tech attack bonus for the given session and tech tradition.
//
// Formula: Level/2 + primary ability modifier.
// Tradition → primary ability: neural→Savvy, bio_synthetic→Grit, technical→Quickness, others→Quickness.
func techAttackMod(sess *session.PlayerSession, tech *technology.TechnologyDef) int {
	if sess == nil {
		return 0
	}
	levelBonus := sess.Level / 2
	var abilityScore int
	switch tech.Tradition {
	case technology.TraditionNeural:
		abilityScore = sess.Abilities.Savvy
	case technology.TraditionBioSynthetic:
		abilityScore = sess.Abilities.Grit
	default:
		abilityScore = sess.Abilities.Quickness
	}
	return levelBonus + abilityModifier(abilityScore)
}

// abilityModifier returns the PF2E ability modifier for a score.
// Formula: floor((score - 10) / 2). Go integer division truncates toward zero,
// so negative odd differences require explicit floor adjustment.
func abilityModifier(score int) int {
	diff := score - 10
	if diff >= 0 {
		return diff / 2
	}
	return (diff - 1) / 2
}

// applyEffects applies a slice of TechEffects and returns result messages.
func applyEffects(
	sess *session.PlayerSession,
	effects []technology.TechEffect,
	target *combat.Combatant,
	cbt *combat.Combat,
	condRegistry *condition.Registry,
	src combat.Source,
) []string {
	var msgs []string
	for _, e := range effects {
		msg := applyEffect(sess, e, target, cbt, condRegistry, src)
		if msg != "" {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

// applyEffect applies a single TechEffect and returns a description message.
func applyEffect(
	sess *session.PlayerSession,
	e technology.TechEffect,
	target *combat.Combatant,
	cbt *combat.Combat,
	condRegistry *condition.Registry,
	src combat.Source,
) string {
	switch e.Type {
	case technology.EffectDamage:
		if target == nil {
			return ""
		}
		dmg := rollAmount(e.Dice, e.Amount, src)
		target.CurrentHP -= dmg
		if target.CurrentHP < 0 {
			target.CurrentHP = 0
		}
		return fmt.Sprintf("%d %s damage.", dmg, e.DamageType)

	case technology.EffectHeal:
		heal := rollAmount(e.Dice, e.Amount, src)
		sess.CurrentHP += heal
		if sess.CurrentHP > sess.MaxHP {
			sess.CurrentHP = sess.MaxHP
		}
		return fmt.Sprintf("Healed %d HP.", heal)

	case technology.EffectCondition:
		if condRegistry == nil || cbt == nil {
			return "" // silently skip — no condition system available
		}
		def, ok := condRegistry.Get(e.ConditionID)
		if !ok {
			return fmt.Sprintf("Unknown condition %q.", e.ConditionID)
		}
		stacks := e.Value
		if stacks == 0 {
			stacks = 1
		}
		dur := parseDuration(e.Duration)
		var uid string
		var applySet *condition.ActiveSet
		if target != nil {
			uid = target.ID
			applySet = cbt.Conditions[target.ID]
		} else {
			uid = sess.UID
			applySet = cbt.Conditions[sess.UID]
		}
		if applySet == nil {
			return ""
		}
		if err := applySet.Apply(uid, def, stacks, dur); err != nil {
			return fmt.Sprintf("Failed to apply %s.", e.ConditionID)
		}
		return fmt.Sprintf("%s %d applied.", e.ConditionID, stacks)

	case technology.EffectMovement:
		if target == nil {
			return ""
		}
		if e.Direction == "away" {
			target.Position += e.Distance
		} else if e.Direction == "toward" {
			target.Position -= e.Distance
			if target.Position < 0 {
				target.Position = 0
			}
		}
		return fmt.Sprintf("Pushed %d feet %s.", e.Distance, e.Direction)

	case technology.EffectUtility:
		if e.UtilityType != "" {
			return fmt.Sprintf("Utility effect: %s.", e.UtilityType)
		}
		return ""

	default:
		return ""
	}
}

// rollAmount rolls a dice expression and adds a flat amount.
//
// Precondition: src must be non-nil.
// Postcondition: returns at least numDice+flat for any valid NdX expression; returns flat when expr is empty.
// If the dice expression is invalid (e.g. sides < 2), the minimum possible roll (numDice*1+flat) is returned.
func rollAmount(expr string, flat int, src combat.Source) int {
	if expr == "" {
		return flat
	}
	result, err := dice.RollExpr(expr, diceSrcAdapter{src: src})
	if err != nil {
		// Parse failed — compute minimum by parsing count manually.
		numDice := parseDiceCount(expr)
		return numDice + flat
	}
	return result.Total() + flat
}

// parseDiceCount extracts the number of dice from an expression like "NdX" or "NdX+M".
// Returns 1 if the expression cannot be parsed.
func parseDiceCount(expr string) int {
	lower := strings.ToLower(expr)
	idx := strings.Index(lower, "d")
	if idx <= 0 {
		return 1
	}
	n, err := strconv.Atoi(expr[:idx])
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// parseDuration converts a duration string to rounds.
//
// Supported formats:
//   - "rounds:N"  → N rounds
//   - "minutes:N" → N*10 rounds
//   - "instant" or "" → 1 round
func parseDuration(s string) int {
	if s == "" || s == "instant" {
		return 1
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 1
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n <= 0 {
		return 1
	}
	switch parts[0] {
	case "rounds":
		return n
	case "minutes":
		return n * 10
	default:
		return 1
	}
}
