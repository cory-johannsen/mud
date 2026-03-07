package xp

// AwardResult holds the outcome of an XP award calculation.
type AwardResult struct {
	// NewXP is the character's total XP after the award.
	NewXP int
	// NewLevel is the character's level after the award.
	NewLevel int
	// HPGained is the total max HP increase from level-ups this award.
	HPGained int
	// NewBoosts is the number of new ability boosts earned this award.
	NewBoosts int
	// LeveledUp is true if the character gained at least one level.
	LeveledUp bool
}

// XPToLevel returns the total XP required to reach the given level.
//
// Precondition: level >= 1; baseXP > 0.
// Postcondition: Returns level² × baseXP.
func XPToLevel(level, baseXP int) int {
	return level * level * baseXP
}

// LevelForXP returns the level a character with the given XP should be at.
//
// Precondition: cfg must be non-nil; cfg.LevelCap >= 1; cfg.BaseXP > 0.
// Postcondition: Returns a value in [1, cfg.LevelCap].
func LevelForXP(xp int, cfg *XPConfig) int {
	level := 1
	for level < cfg.LevelCap {
		next := level + 1
		if xp < XPToLevel(next, cfg.BaseXP) {
			break
		}
		level = next
	}
	return level
}

// Award calculates the result of adding awardXP to a character at the given
// level with the given currentXP.
//
// Precondition: level >= 1; currentXP >= 0; awardXP >= 0; cfg must be non-nil;
// cfg.BaseXP > 0; cfg.LevelCap >= 1.
// Postcondition: Returns an AwardResult with updated level, XP, HP gain, and boost count.
// Level never exceeds cfg.LevelCap. XP always reflects currentXP + awardXP.
func Award(level, currentXP, awardXP int, cfg *XPConfig) AwardResult {
	newXP := currentXP + awardXP
	newLevel := LevelForXP(newXP, cfg)

	levelsGained := newLevel - level
	hpGained := levelsGained * cfg.HPPerLevel

	newBoosts := 0
	if cfg.BoostInterval > 0 {
		for l := level + 1; l <= newLevel; l++ {
			if l%cfg.BoostInterval == 0 {
				newBoosts++
			}
		}
	}

	return AwardResult{
		NewXP:     newXP,
		NewLevel:  newLevel,
		HPGained:  hpGained,
		NewBoosts: newBoosts,
		LeveledUp: newLevel > level,
	}
}
