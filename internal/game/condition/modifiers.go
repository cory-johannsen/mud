package condition

import "github.com/cory-johannsen/mud/internal/game/effect"

// AttackBonus returns the net attack roll modifier from all active conditions.
// Positive values indicate a net bonus; negative values indicate a net penalty.
//
// Precondition: s may be nil.
// Postcondition: value may be positive or negative.
func AttackBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	return effect.Resolve(s.Effects(), effect.StatAttack).Total
}

// ACBonus returns the net AC modifier from all active conditions.
// Positive values indicate a net bonus; negative values indicate a net penalty.
//
// Precondition: s may be nil.
// Postcondition: value may be positive or negative.
func ACBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	return effect.Resolve(s.Effects(), effect.StatAC).Total
}

// DamageBonus returns the total damage bonus granted by all active conditions.
//
// Precondition: s may be nil.
// Postcondition: returns >= 0 (negative totals are clamped to 0 per legacy contract).
func DamageBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	v := effect.Resolve(s.Effects(), effect.StatDamage).Total
	if v < 0 {
		return 0
	}
	return v
}

// SkillPenalty returns the flat all-skill penalty magnitude from active conditions.
// Returns the absolute value of any negative total from StatSkill; 0 when net is positive.
//
// Precondition: s may be nil.
// Postcondition: returns >= 0.
func SkillPenalty(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	v := effect.Resolve(s.Effects(), effect.StatSkill).Total
	if v >= 0 {
		return 0
	}
	return -v
}

// StealthBonus returns the Stealth skill bonus from active conditions.
//
// Precondition: s may be nil.
// Postcondition: returns >= 0.
func StealthBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	v := effect.Resolve(s.Effects(), effect.Stat("skill:stealth")).Total
	if v < 0 {
		return 0
	}
	return v
}

// FlairBonus returns the Flair ability bonus from active conditions.
//
// Precondition: s may be nil.
// Postcondition: returns >= 0.
func FlairBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	v := effect.Resolve(s.Effects(), effect.StatFlair).Total
	if v < 0 {
		return 0
	}
	return v
}

// ReflexBonus returns the total Reflex save bonus granted by all active conditions.
// For stackable conditions, the bonus is multiplied by the current stack count.
// ReflexBonus is not routed through the EffectSet because ConditionDef.ReflexBonus
// is not synthesised into Bonuses (Gunchete saves are handled separately).
//
// Precondition: s may be nil.
// Postcondition: Returns >= 0.
func ReflexBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.ReflexBonus * ac.Stacks
	}
	if total < 0 {
		total = 0
	}
	return total
}

// IsMovementPrevented reports whether any active condition's PreventMovement flag
// is set, indicating the entity cannot move between rooms.
//
// Precondition: s may be nil.
func IsMovementPrevented(s *ActiveSet) bool {
	if s == nil {
		return false
	}
	for _, ac := range s.conditions {
		if ac.Def.PreventMovement {
			return true
		}
	}
	return false
}

// IsCommandsPrevented reports whether any active condition's PreventCommands flag
// is set, indicating the entity cannot issue action commands.
//
// Precondition: s may be nil.
func IsCommandsPrevented(s *ActiveSet) bool {
	if s == nil {
		return false
	}
	for _, ac := range s.conditions {
		if ac.Def.PreventCommands {
			return true
		}
	}
	return false
}

// IsTargetingPrevented reports whether any active condition's PreventTargeting flag
// is set, indicating the entity cannot be targeted.
//
// Precondition: s may be nil.
func IsTargetingPrevented(s *ActiveSet) bool {
	if s == nil {
		return false
	}
	for _, ac := range s.conditions {
		if ac.Def.PreventTargeting {
			return true
		}
	}
	return false
}

// IsActionRestricted reports whether the given action type string is blocked
// by any active condition's RestrictActions list.
func IsActionRestricted(s *ActiveSet, actionType string) bool {
	for _, ac := range s.conditions {
		for _, r := range ac.Def.RestrictActions {
			if r == actionType {
				return true
			}
		}
	}
	return false
}

// ExtraWeaponDice returns the total number of extra weapon damage dice granted by all active conditions.
// Each die is of the weapon's own die type, rolled on a hit and doubled on a crit.
//
// Precondition: s may be nil.
// Postcondition: Returns >= 0.
func ExtraWeaponDice(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.ExtraWeaponDice * ac.Stacks
	}
	if total < 0 {
		total = 0
	}
	return total
}

// StunnedAPReduction returns the number of AP to subtract from the action queue
// this round due to the stunned condition. Equal to the current stunned stack count.
//
// Postcondition: Returns >= 0.
func StunnedAPReduction(s *ActiveSet) int {
	return s.Stacks("stunned")
}

// APReduction returns the total AP reduction from all active conditions.
// Each condition contributes APReduction * Stacks.
//
// Precondition: s must not be nil.
// Postcondition: Returns >= 0.
func APReduction(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.APReduction * ac.Stacks
	}
	if total < 0 {
		total = 0
	}
	return total
}

// SkipTurn returns true if any active condition has SkipTurn set.
//
// Precondition: s must not be nil.
func SkipTurn(s *ActiveSet) bool {
	if s == nil {
		return false
	}
	for _, ac := range s.conditions {
		if ac.Def.SkipTurn {
			return true
		}
	}
	return false
}

// ForcedActionType returns the forced_action value from the first active condition
// that has one, or empty string if none. Map iteration order is non-deterministic;
// simultaneous forced conditions from different tracks are not expected in practice.
//
// Precondition: s may be nil.
// Postcondition: Returns "" or one of "random_attack", "lowest_hp_attack".
func ForcedActionType(s *ActiveSet) string {
	if s == nil {
		return ""
	}
	for _, ac := range s.conditions {
		if ac.Def.ForcedAction != "" {
			return ac.Def.ForcedAction
		}
	}
	return ""
}
