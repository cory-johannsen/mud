package combat

// CombatantsInRadius returns all living combatants within radiusFt feet (Chebyshev) of center.
//
// Precondition: cbt must not be nil; radiusFt must be >= 0.
// Postcondition: returned slice contains only non-dead combatants within range; nil if none.
func CombatantsInRadius(cbt *Combat, center Combatant, radiusFt int) []*Combatant {
	var result []*Combatant
	for _, c := range cbt.Combatants {
		if !c.IsDead() && CombatRange(*c, center) <= radiusFt {
			result = append(result, c)
		}
	}
	return result
}
