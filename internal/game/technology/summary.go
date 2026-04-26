package technology

import (
	"fmt"
	"strings"
)

// FormatEffectsSummary returns a compact, human-readable summary of a technology's
// mechanical effects suitable for display in a game client.
//
// Precondition: def must be non-nil.
// Postcondition: Returns a non-empty string describing the tech's mechanics.
func FormatEffectsSummary(def *TechnologyDef) string {
	var lines []string

	// Header line: action cost, range/targets, resolution, duration
	lines = append(lines, formatHeader(def))

	// Effect lines grouped by tier
	switch def.Resolution {
	case "save":
		lines = append(lines, formatTierLines("Crit Success", def.Effects.OnCritSuccess)...)
		lines = append(lines, formatTierLines("Success", def.Effects.OnSuccess)...)
		lines = append(lines, formatTierLines("Failure", def.Effects.OnFailure)...)
		lines = append(lines, formatTierLines("Crit Failure", def.Effects.OnCritFailure)...)
	case "attack":
		lines = append(lines, formatTierLines("Miss", def.Effects.OnMiss)...)
		lines = append(lines, formatTierLines("Hit", def.Effects.OnHit)...)
		lines = append(lines, formatTierLines("Crit Hit", def.Effects.OnCritHit)...)
	default:
		lines = append(lines, formatEffectList(def.Effects.OnApply)...)
	}

	return strings.Join(lines, "\n")
}

func formatHeader(def *TechnologyDef) string {
	var parts []string

	if def.Reaction != nil {
		parts = append(parts, "Reaction")
	} else if def.Passive {
		parts = append(parts, "Passive")
	} else {
		switch def.ActionCost {
		case 1:
			parts = append(parts, "1 action")
		case 2:
			parts = append(parts, "2 actions")
		case 3:
			parts = append(parts, "3 actions")
		default:
			if def.ActionCost > 0 {
				parts = append(parts, fmt.Sprintf("%d actions", def.ActionCost))
			}
		}
	}

	if def.FocusCost {
		parts = append(parts, "+focus")
	}

	rangeStr := formatRangeTargets(def.Range, def.Targets)
	if rangeStr != "" {
		parts = append(parts, rangeStr)
	}

	// #363: surface AoE shape parameters in the header so the per-tech UI
	// row shows the area at a glance (e.g. "burst 10 ft", "cone 30 ft", "line 25x5 ft").
	if aoe := formatAoeHeader(def); aoe != "" {
		parts = append(parts, aoe)
	}

	switch def.Resolution {
	case "save":
		parts = append(parts, fmt.Sprintf("Save: %s DC %d", strings.Title(strings.ReplaceAll(def.SaveType, "_", " ")), def.SaveDC)) //nolint:staticcheck
	case "attack":
		parts = append(parts, "Attack roll")
	}

	if dur := formatDuration(def.Duration); dur != "" && dur != "instant" {
		parts = append(parts, dur)
	}

	return strings.Join(parts, " | ")
}

func formatRangeTargets(r Range, t Targets) string {
	switch r {
	case RangeSelf:
		return "Self"
	case RangeMelee:
		return "Melee"
	case RangeRanged:
		if t == TargetsSingle {
			return "Ranged"
		}
		return "Ranged, " + formatTargets(t)
	case RangeZone:
		return "Zone, " + formatTargets(t)
	}
	return ""
}

func formatTargets(t Targets) string {
	switch t {
	case TargetsSingle:
		return "single target"
	case TargetsAllEnemies:
		return "all enemies"
	case TargetsAllAllies:
		return "all allies"
	case TargetsZone:
		return "zone"
	}
	return string(t)
}

func formatDuration(d string) string {
	switch {
	case d == "instant":
		return "instant"
	case strings.HasPrefix(d, "rounds:"):
		n := strings.TrimPrefix(d, "rounds:")
		if n == "1" {
			return "1 round"
		}
		return n + " rounds"
	case strings.HasPrefix(d, "minutes:"):
		n := strings.TrimPrefix(d, "minutes:")
		if n == "1" {
			return "1 minute"
		}
		return n + " minutes"
	case strings.HasPrefix(d, "hours:"):
		n := strings.TrimPrefix(d, "hours:")
		if n == "1" {
			return "1 hour"
		}
		return n + " hours"
	}
	return d
}

// formatTierLines formats a named tier (e.g. "Hit", "Failure") + its effects.
// Returns nothing if effects is empty.
func formatTierLines(tier string, effects []TechEffect) []string {
	if len(effects) == 0 {
		return nil
	}
	effectStrs := formatEffectList(effects)
	if len(effectStrs) == 0 {
		return nil
	}
	// Single effect: "Hit: 1d6 acid damage"
	// Multiple: "Failure:\n  frightened 2\n  prone"
	if len(effectStrs) == 1 {
		return []string{tier + ": " + effectStrs[0]}
	}
	lines := []string{tier + ":"}
	for _, s := range effectStrs {
		lines = append(lines, "  "+s)
	}
	return lines
}

func formatEffectList(effects []TechEffect) []string {
	var out []string
	for _, e := range effects {
		if s := formatEffect(e); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func formatEffect(e TechEffect) string {
	switch e.Type {
	case EffectDamage:
		dmg := formatDiceAmount(e.Dice, e.Amount)
		var s string
		if e.DamageType != "" {
			s = dmg + " " + e.DamageType + " damage"
		} else {
			s = dmg + " damage"
		}
		// #363: surface multi-projectile + persistent flags so the count is
		// visible in the Technologies panel and hotbar tooltip without the
		// player having to drill into the YAML or read the description prose.
		if e.Projectiles > 0 {
			s += fmt.Sprintf(" × %d projectiles", e.Projectiles)
		}
		if e.Persistent {
			s += " (persistent)"
		}
		return s
	case EffectHeal:
		return formatDiceAmount(e.Dice, e.Amount) + " healing"
	case EffectDrain:
		res := e.Resource
		if res == "" {
			res = "HP"
		} else {
			res = strings.ToUpper(res)
		}
		return formatDiceAmount(e.Dice, e.Amount) + " " + res + " drain"
	case EffectCondition:
		s := e.ConditionID
		if e.Value > 1 {
			s = fmt.Sprintf("%s %d", s, e.Value)
		}
		if dur := formatDuration(e.Duration); dur != "" && dur != "instant" {
			s += " (" + dur + ")"
		}
		return s
	case EffectMovement:
		if e.Direction == "teleport" {
			return fmt.Sprintf("teleport %d ft", e.Distance)
		}
		return fmt.Sprintf("%d ft %s", e.Distance, e.Direction)
	case EffectSkillCheck:
		return fmt.Sprintf("Skill check: %s DC %d", e.Skill, e.DC)
	case EffectZone:
		return fmt.Sprintf("%d ft zone", e.Radius)
	case EffectSummon:
		s := "summon " + e.NPCID
		if e.SummonRounds > 0 {
			s += fmt.Sprintf(" (%d rounds)", e.SummonRounds)
		}
		return s
	case EffectUtility:
		if e.Description != "" {
			return e.Description
		}
		return e.UtilityType
	case EffectTremorsense:
		return "tremorsense"
	}
	return ""
}

func formatDiceAmount(dice string, amount int) string {
	switch {
	case dice != "" && amount != 0:
		return fmt.Sprintf("%s+%d", dice, amount)
	case dice != "":
		return dice
	case amount != 0:
		return fmt.Sprintf("%d", amount)
	default:
		return "?"
	}
}

// formatAoeHeader returns a compact AoE shape descriptor for the header line,
// or "" when the tech has no AoE shape. Generic across burst / cone / line so
// any tech declaring an AoE shape gets the same affordance (#363).
func formatAoeHeader(def *TechnologyDef) string {
	// Legacy back-compat: aoe_radius without an explicit shape implies burst.
	if def.AoeRadius > 0 && def.AoeShape == "" {
		return fmt.Sprintf("burst %d ft", def.AoeRadius)
	}
	switch string(def.AoeShape) {
	case "burst":
		if def.AoeRadius > 0 {
			return fmt.Sprintf("burst %d ft", def.AoeRadius)
		}
	case "cone":
		if def.AoeLength > 0 {
			return fmt.Sprintf("cone %d ft", def.AoeLength)
		}
	case "line":
		if def.AoeLength > 0 {
			w := def.AoeWidth
			if w == 0 {
				w = 5
			}
			return fmt.Sprintf("line %dx%d ft", def.AoeLength, w)
		}
	}
	return ""
}
