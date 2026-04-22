package gameserver

// resolveWeaponProficiency returns the player's effective proficiency rank for
// a weapon whose YAML proficiency_category is `category`. It walks a fallback
// chain because job/archetype grants use broader keys (e.g. "simple_weapons",
// "martial_weapons") than weapons themselves (e.g. "simple_melee",
// "martial_ranged"). A direct hit wins; otherwise the best-ranked parent is
// used.
//
// Fallback chain:
//
//	simple_melee   → simple_weapons
//	simple_ranged  → simple_weapons
//	martial_melee  → martial_weapons → simple_weapons
//	martial_ranged → martial_weapons → simple_weapons
//	simple_weapons → (direct only)
//	martial_weapons→ (direct only)
//
// The PF2E convention that martial training implies simple training is
// represented by the martial_* → simple_weapons step; a player with only
// martial_melee still counts as trained with a vibroblade's martial_melee
// weapon and as trained with a simple melee weapon too.
//
// Returns ("untrained", "") when no match is found.
//
// Precondition: profs may be nil (returns "untrained" in that case).
// Postcondition: matchedCategory is "" when returning "untrained".
func resolveWeaponProficiency(profs map[string]string, category string) (rank string, matchedCategory string) {
	if profs == nil {
		return "untrained", ""
	}
	check := func(key string) (string, string, bool) {
		if r, ok := profs[key]; ok && r != "" && r != "untrained" {
			return r, key, true
		}
		return "", "", false
	}
	if r, k, ok := check(category); ok {
		return r, k
	}
	// Fallback chain per category.
	var fallbacks []string
	switch category {
	case "simple_melee", "simple_ranged":
		fallbacks = []string{"simple_weapons"}
	case "martial_melee", "martial_ranged":
		fallbacks = []string{"martial_weapons", "simple_weapons"}
	}
	// Track best rank across fallbacks so (e.g.) a player with both
	// simple_weapons=trained and martial_weapons=expert gets "expert" for a
	// martial weapon rather than stopping at the first non-empty entry.
	bestRank, bestKey := "", ""
	for _, k := range fallbacks {
		if r, ok := profs[k]; ok && r != "" && r != "untrained" {
			if rankOrder(r) > rankOrder(bestRank) {
				bestRank, bestKey = r, k
			}
		}
	}
	if bestRank == "" {
		return "untrained", ""
	}
	return bestRank, bestKey
}

// rankOrder maps a PF2E proficiency rank to an integer so callers can compare
// two ranks. Unknown or empty ranks sort below untrained.
func rankOrder(rank string) int {
	switch rank {
	case "untrained":
		return 0
	case "trained":
		return 1
	case "expert":
		return 2
	case "master":
		return 3
	case "legendary":
		return 4
	default:
		return -1
	}
}
