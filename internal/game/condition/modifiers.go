package condition

// AttackBonus returns the net attack roll modifier from all active conditions.
// For stackable conditions (e.g. frightened), the penalty is multiplied by
// the current stack count (frightened 2 = -2 to attack).
//
// Postcondition: Returns <= 0.
func AttackBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.AttackPenalty > 0 {
			total -= ac.Def.AttackPenalty * ac.Stacks
		}
	}
	return total
}

// ACBonus returns the net AC modifier from all active conditions.
// For stackable conditions, the penalty is multiplied by the current stack count.
//
// Postcondition: Returns <= 0.
func ACBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.ACPenalty > 0 {
			total -= ac.Def.ACPenalty * ac.Stacks
		}
	}
	return total
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
	for _, ac := range s.All() {
		stacks := ac.Stacks
		if stacks < 1 {
			stacks = 1
		}
		total += ac.Def.DamageBonus * stacks
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
