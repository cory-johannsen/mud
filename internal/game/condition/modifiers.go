package condition

// AttackBonus returns the net attack roll modifier from all active conditions.
// Positive AttackBonus on a condition adds to the total (buff); positive AttackPenalty
// subtracts from the total (debuff). For stackable conditions, values are multiplied
// by the current stack count.
//
// Precondition: s may be nil.
// Postcondition: Returns the net modifier; may be positive when attack bonuses are active.
func AttackBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.AttackPenalty > 0 {
			total -= ac.Def.AttackPenalty * ac.Stacks
		}
		if ac.Def.AttackBonus > 0 {
			total += ac.Def.AttackBonus * ac.Stacks
		}
	}
	return total
}

// ACBonus returns the net AC modifier from all active conditions.
// Positive ACBonus on a condition adds to the total (buff); positive ACPenalty
// subtracts from the total (debuff). For stackable conditions, values are multiplied
// by the current stack count.
//
// Precondition: s may be nil.
// Postcondition: May be positive when AC bonuses exceed penalties.
func ACBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.ACPenalty > 0 {
			total -= ac.Def.ACPenalty * ac.Stacks
		}
		if ac.Def.ACBonus > 0 {
			total += ac.Def.ACBonus * ac.Stacks
		}
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

// DamageBonus returns the total damage bonus granted by all active conditions.
// For stackable conditions, the bonus is multiplied by the current stack count.
//
// Precondition: s must not be nil.
// Postcondition: returns >= 0.
func DamageBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.DamageBonus * ac.Stacks
	}
	if total < 0 {
		total = 0
	}
	return total
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

// ReflexBonus returns the total Reflex save bonus granted by all active conditions.
// For stackable conditions, the bonus is multiplied by the current stack count.
//
// Precondition: s must not be nil.
// Postcondition: Returns >= 0.
func ReflexBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.ReflexBonus * ac.Stacks
	}
	if total < 0 {
		total = 0
	}
	return total
}

// StealthBonus returns the total Stealth skill bonus granted by all active conditions.
// For stackable conditions, the bonus is multiplied by the current stack count.
//
// Precondition: s must not be nil.
// Postcondition: Returns >= 0.
func StealthBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.StealthBonus * ac.Stacks
	}
	if total < 0 {
		total = 0
	}
	return total
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

// SkillPenalty returns the total skill penalty from all active conditions.
// Each condition contributes SkillPenalty * Stacks.
//
// Precondition: s must not be nil.
// Postcondition: Returns >= 0.
func SkillPenalty(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.SkillPenalty * ac.Stacks
	}
	if total < 0 {
		total = 0
	}
	return total
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
