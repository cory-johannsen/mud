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

// RoomQuerier provides creature presence information for a room.
// The sensingUID identifies the player activating the tremorsense effect,
// whose entry is returned as CreatureInfo{Name: "you"}.
type RoomQuerier interface {
	CreaturesInRoom(roomID, sensingUID string) []CreatureInfo
}

// CreatureInfo describes a creature present in a room for tremorsense output.
type CreatureInfo struct {
	Name   string
	Hidden bool
}

// FormatTremorsenseOutput formats a []CreatureInfo into a [Seismic Sense] message.
// Hidden creatures are suffixed with " (concealed)".
// Returns a no-creatures message if the slice is empty.
func FormatTremorsenseOutput(creatures []CreatureInfo) string {
	if len(creatures) == 0 {
		return "[Seismic Sense] No creatures detected."
	}
	parts := make([]string, len(creatures))
	for i, c := range creatures {
		if c.Hidden {
			parts[i] = c.Name + " (concealed)"
		} else {
			parts[i] = c.Name
		}
	}
	return "[Seismic Sense] Creatures detected in this room: " + strings.Join(parts, ", ")
}

// diceSrcAdapter wraps a combat.Source so it satisfies dice.Source.
// Both interfaces require Intn(int) int — this adapter is structurally redundant
// but required because Go does not unify separately-defined identical interfaces.
type diceSrcAdapter struct{ src combat.Source }

func (a diceSrcAdapter) Intn(n int) int { return a.src.Intn(n) }

// ResolveTechEffects resolves all effects for a tech activation and returns a slice of
// human-readable result messages (one per target). Does not expend uses — the caller
// has already done that.
//
// Equivalent to ResolveTechEffectsWithHeighten(..., 0) — callers that do not
// track heighten delta should use this function.
func ResolveTechEffects(
	sess *session.PlayerSession,
	tech *technology.TechnologyDef,
	targets []*combat.Combatant,
	cbt *combat.Combat,
	condRegistry *condition.Registry,
	src combat.Source,
	querier RoomQuerier,
) []string {
	return ResolveTechEffectsWithHeighten(sess, tech, targets, cbt, condRegistry, src, querier, 0)
}

// ResolveTechEffectsWithHeighten is ResolveTechEffects plus a heightenDelta
// (slotLevel - techLevel, clamped to >= 0). The delta is applied to effect
// types that scale with heighten — currently the `projectiles` field on damage
// effects (GH #224): each additional level adds one more independent projectile
// roll.
//
// Preconditions:
//   - sess must be non-nil.
//   - tech must be non-nil.
//   - targets is empty for self/utility/no-roll; one entry for single; all enemies for area.
//   - cbt may be nil when not in combat; condition effects are silently skipped when nil.
//   - condRegistry may be nil; condition effects are silently skipped when nil.
//   - src must be non-nil (satisfies combat.Source: Intn method).
//   - heightenDelta must be >= 0; negative values are treated as 0.
//
// Postconditions:
//   - Returns zero or more non-empty messages.
//   - target.CurrentHP and sess.CurrentHP never go below 0.
//   - sess.CurrentHP never exceeds sess.MaxHP.
func ResolveTechEffectsWithHeighten(
	sess *session.PlayerSession,
	tech *technology.TechnologyDef,
	targets []*combat.Combatant,
	cbt *combat.Combat,
	condRegistry *condition.Registry,
	src combat.Source,
	querier RoomQuerier,
	heightenDelta int,
) []string {
	if heightenDelta < 0 {
		heightenDelta = 0
	}
	if len(targets) == 0 {
		if tech.Resolution == "attack" || tech.Resolution == "save" {
			// #349: when out-of-combat AND the tech has authored damage/condition
			// blocks, surface a clearer "requires a combat target" instead of the
			// generic "No valid target." Pure passives (e.g. tremorsense) keep
			// existing behaviour.
			if cbt == nil && techHasTargetedEffects(tech) {
				return []string{fmt.Sprintf("%s requires a combat target — start combat or pick a target first.", tech.Name)}
			}
			return []string{"No valid target."}
		}
		return applyEffects(sess, tech.Effects.OnApply, nil, cbt, condRegistry, src, querier, heightenDelta)
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
			outcome, roll, total := resolveAttackRollWithTotal(sess, tech, target, src)
			tier, label = selectAttackTier(tech.Effects, outcome, target.Name)
			// GH #226: include raw roll, total, and target AC in the label so
			// players see what happened mechanically, matching the detail level
			// of weapon attack narratives.
			label = annotateAttackLabel(label, roll, total, target.AC)
		default: // "none" or ""
			tier = tech.Effects.OnApply
			label = ""
		}

		effectMsgs := applyEffects(sess, tier, target, cbt, condRegistry, src, querier, heightenDelta)
		if len(effectMsgs) == 0 && label != "" {
			standalone := strings.TrimSuffix(label, ": ")
			if !strings.HasSuffix(standalone, ".") {
				standalone += "."
			}
			msgs = append(msgs, standalone)
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
	outcome, _, _ := resolveAttackRollWithTotal(sess, tech, target, src)
	return outcome
}

// resolveAttackRollWithTotal is resolveAttackRoll but also returns the raw d20
// roll and the post-modifier total, so callers can display roll details to
// the player (GH #226).
//
// Postcondition: roll in [1,20]; total = roll + techAttackMod(sess, tech).
func resolveAttackRollWithTotal(sess *session.PlayerSession, tech *technology.TechnologyDef, target *combat.Combatant, src combat.Source) (combat.Outcome, int, int) {
	roll := src.Intn(20) + 1
	total := roll + techAttackMod(sess, tech)
	return combat.OutcomeFor(total, target.AC), roll, total
}

// annotateAttackLabel appends the raw d20 roll, post-modifier total, and
// target AC to an attack-outcome label (GH #226). The annotation format is
// " (rolled N, total=M vs AC=A)" with punctuation preserved at the original
// label end: a label ending in ": " receives the annotation before the colon,
// a label ending in "." receives it before the period.
func annotateAttackLabel(label string, roll, total, ac int) string {
	ann := fmt.Sprintf(" (rolled %d, total=%d vs AC %d)", roll, total, ac)
	switch {
	case strings.HasSuffix(label, ": "):
		return strings.TrimSuffix(label, ": ") + ann + ": "
	case strings.HasSuffix(label, "."):
		return strings.TrimSuffix(label, ".") + ann + "."
	default:
		return label + ann
	}
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
	querier RoomQuerier,
	heightenDelta int,
) []string {
	var msgs []string
	for _, e := range effects {
		msg := applyEffect(sess, e, target, cbt, condRegistry, src, querier, heightenDelta)
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
	querier RoomQuerier,
	heightenDelta int,
) string {
	switch e.Type {
	case technology.EffectDamage:
		if target == nil {
			return ""
		}
		// GH #224: Magic Missile-style multi-projectile resolution. When
		// Projectiles > 0, roll the damage expression independently for each
		// projectile and sum. Heighten delta adds one projectile per level.
		if e.Projectiles > 0 {
			shots := e.Projectiles + heightenDelta
			if shots < 1 {
				shots = 1
			}
			total := 0
			for i := 0; i < shots; i++ {
				total += rollAmount(e.Dice, e.Amount, src)
			}
			target.CurrentHP -= total
			if target.CurrentHP < 0 {
				target.CurrentHP = 0
			}
			return fmt.Sprintf("%d %s damage across %d projectiles.", total, e.DamageType, shots)
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
		// Convert feet to grid cells (1 cell = 5 ft), minimum 1 cell.
		pushCells := e.Distance / 5
		if pushCells < 1 {
			pushCells = 1
		}
		// Resolve direction using the activating player's combatant as the source reference.
		var source *combat.Combatant
		if cbt != nil {
			source = cbt.GetCombatant(sess.UID)
		}
		width := 10
		height := 10
		if cbt != nil && cbt.GridWidth > 0 {
			width = cbt.GridWidth
		}
		if cbt != nil && cbt.GridHeight > 0 {
			height = cbt.GridHeight
		}
		for i := 0; i < pushCells; i++ {
			dx, dy := combat.CompassDelta(e.Direction, target, source)
			newX := target.GridX + dx
			newY := target.GridY + dy
			if newX < 0 {
				newX = 0
			} else if newX >= width {
				newX = width - 1
			}
			if newY < 0 {
				newY = 0
			} else if newY >= height {
				newY = height - 1
			}
			target.GridX = newX
			target.GridY = newY
		}
		return fmt.Sprintf("Pushed %d feet %s.", e.Distance, e.Direction)

	case technology.EffectUtility:
		// GH #329: description-only utility effects (no UtilityType) previously
		// returned an empty message, causing techs like Litany of Iron and
		// Fervor Pulse on_success to silently no-op. Surface the description
		// when present so the player at least sees the narrative outcome;
		// fall back to the typed-utility format for unlock/reveal/hack.
		if e.Description != "" {
			return e.Description
		}
		if e.UtilityType != "" {
			return fmt.Sprintf("Utility effect: %s.", e.UtilityType)
		}
		return ""

	case technology.EffectTremorsense:
		if querier == nil {
			return ""
		}
		creatures := querier.CreaturesInRoom(sess.RoomID, sess.UID)
		return FormatTremorsenseOutput(creatures)

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

// techHasTargetedEffects reports whether tech has any tier with damage or
// condition effects that need a target to apply (#349). Pure narrative or
// passive techs return false so the legacy "No valid target." message is used.
func techHasTargetedEffects(tech *technology.TechnologyDef) bool {
	for _, tier := range [][]technology.TechEffect{
		tech.Effects.OnHit, tech.Effects.OnCritHit, tech.Effects.OnMiss,
		tech.Effects.OnSuccess, tech.Effects.OnCritSuccess,
		tech.Effects.OnFailure, tech.Effects.OnCritFailure,
	} {
		for _, e := range tier {
			switch e.Type {
			case technology.EffectDamage, technology.EffectCondition, technology.EffectMovement:
				return true
			}
		}
	}
	return false
}
