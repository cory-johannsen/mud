package combat

// CombatantsInCells returns all living combatants whose grid cell appears in the
// provided cell set. Order follows cbt.Combatants iteration order.
//
// Precondition: cbt must not be nil.
// Postcondition: returned slice contains only non-dead combatants whose
// (GridX, GridY) is a member of cells; nil if none.
func CombatantsInCells(cbt *Combat, cells []Cell) []*Combatant {
	if len(cells) == 0 {
		return nil
	}
	inSet := make(map[Cell]struct{}, len(cells))
	for _, c := range cells {
		inSet[c] = struct{}{}
	}
	var result []*Combatant
	for _, c := range cbt.Combatants {
		if c.IsDead() {
			continue
		}
		if _, ok := inSet[Cell{X: c.GridX, Y: c.GridY}]; ok {
			result = append(result, c)
		}
	}
	return result
}

// CombatantsInRadius returns all living combatants within radiusFt feet (Chebyshev) of center.
//
// Precondition: cbt must not be nil; radiusFt must be >= 0.
// Postcondition: returned slice contains only non-dead combatants within range; nil if none.
//
// Implementation note (AOE-19): retained as a thin wrapper for back-compat;
// delegates to CombatantsInCells over BurstCells so behaviour stays in lock-step
// with the cell-based AOE pipeline.
func CombatantsInRadius(cbt *Combat, center Combatant, radiusFt int) []*Combatant {
	return CombatantsInCells(cbt, BurstCells(Cell{X: center.GridX, Y: center.GridY}, radiusFt))
}
