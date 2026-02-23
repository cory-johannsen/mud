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

// StunnedAPReduction returns the number of AP to subtract from the action queue
// this round due to the stunned condition. Equal to the current stunned stack count.
//
// Postcondition: Returns >= 0.
func StunnedAPReduction(s *ActiveSet) int {
	return s.Stacks("stunned")
}
