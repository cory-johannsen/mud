package ruleset

// AbilityBoostGrant describes the ability boosts a content source provides.
// Fixed boosts are always applied. Free boosts require player selection.
//
// Valid ability IDs: "brutality", "grit", "quickness", "reasoning", "savvy", "flair"
type AbilityBoostGrant struct {
	Fixed []string `yaml:"fixed"` // ability IDs always boosted by this source
	Free  int      `yaml:"free"`  // number of player-chosen free boost slots
}

// AllAbilities returns the canonical ordered list of all six ability IDs.
func AllAbilities() []string {
	return []string{"brutality", "grit", "quickness", "reasoning", "savvy", "flair"}
}
