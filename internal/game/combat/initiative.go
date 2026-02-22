package combat

// RollInitiative rolls initiative for all combatants and sets their Initiative field.
// Formula: d20 + DEX modifier.
//
// Precondition: combatants must be non-nil; src must be non-nil.
// Postcondition: Each combatant's Initiative field is set to d20+DexMod.
func RollInitiative(combatants []*Combatant, src Source) {
	for _, c := range combatants {
		roll := src.Intn(20) + 1
		c.Initiative = roll + c.DexMod
	}
}
