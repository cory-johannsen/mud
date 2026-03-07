package xp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/xp"
)

func TestXPToLevel_Formula(t *testing.T) {
	// xp_to_reach_level(n) = n² × baseXP
	assert.Equal(t, 100, xp.XPToLevel(1, 100))
	assert.Equal(t, 400, xp.XPToLevel(2, 100))
	assert.Equal(t, 900, xp.XPToLevel(3, 100))
	assert.Equal(t, 10000, xp.XPToLevel(10, 100))
	assert.Equal(t, 1000000, xp.XPToLevel(100, 100))
}

func TestLevelForXP_Boundaries(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100}
	// 0 XP → level 1
	assert.Equal(t, 1, xp.LevelForXP(0, cfg))
	// exactly enough for level 2 (400 XP)
	assert.Equal(t, 2, xp.LevelForXP(400, cfg))
	// one short of level 2
	assert.Equal(t, 1, xp.LevelForXP(399, cfg))
	// exactly enough for level 3 (900 XP)
	assert.Equal(t, 3, xp.LevelForXP(900, cfg))
	// massive XP → capped at LevelCap
	assert.Equal(t, 100, xp.LevelForXP(999_999_999, cfg))
}

func TestProperty_LevelForXP_Inverse(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100}
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 100).Draw(rt, "level")
		threshold := xp.XPToLevel(level, cfg.BaseXP)
		got := xp.LevelForXP(threshold, cfg)
		if got != level {
			rt.Fatalf("LevelForXP(XPToLevel(%d)) = %d, want %d", level, got, level)
		}
	})
}

func TestProperty_LevelForXP_NeverExceedsCap(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 50}
	rapid.Check(t, func(rt *rapid.T) {
		xpVal := rapid.IntRange(0, 10_000_000).Draw(rt, "xp")
		got := xp.LevelForXP(xpVal, cfg)
		if got > cfg.LevelCap {
			rt.Fatalf("level %d exceeds cap %d for xp=%d", got, cfg.LevelCap, xpVal)
		}
		if got < 1 {
			rt.Fatalf("level %d < 1 for xp=%d", got, xpVal)
		}
	})
}

func TestAward_NoLevelUp(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	result := xp.Award(1, 0, 50, cfg) // level=1, currentXP=0, award 50
	assert.Equal(t, 50, result.NewXP)
	assert.Equal(t, 1, result.NewLevel)
	assert.Equal(t, 0, result.HPGained)
	assert.Equal(t, 0, result.NewBoosts)
	assert.False(t, result.LeveledUp)
}

func TestAward_LevelUp(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	// 400 XP needed for level 2; award 400 at XP=0
	result := xp.Award(1, 0, 400, cfg)
	assert.Equal(t, 400, result.NewXP)
	assert.Equal(t, 2, result.NewLevel)
	assert.Equal(t, 5, result.HPGained)
	assert.True(t, result.LeveledUp)
	assert.Equal(t, 0, result.NewBoosts) // boost at level 5, not 2
}

func TestAward_BoostAtInterval(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	// Start at level 4 with just under level-4 threshold XP, award exactly enough to hit level 5
	startXP := xp.XPToLevel(4, 100) - 1
	awardAmt := xp.XPToLevel(5, 100) - startXP // lands exactly at level 5 threshold
	result := xp.Award(4, startXP, awardAmt, cfg)
	assert.Equal(t, 5, result.NewLevel)
	assert.Equal(t, 1, result.NewBoosts) // level 5 is a boost level
}

func TestAward_AtLevelCap(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 3, HPPerLevel: 5, BoostInterval: 5}
	// Already at cap, extra XP should not level up
	result := xp.Award(3, 900, 10000, cfg)
	assert.Equal(t, 3, result.NewLevel)
	assert.False(t, result.LeveledUp)
	assert.Equal(t, 10900, result.NewXP) // XP still accumulates
}

func TestProperty_Award_NeverExceedsCap(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 99).Draw(rt, "level")
		currentXP := xp.XPToLevel(level, cfg.BaseXP)
		awardAmt := rapid.IntRange(0, 10_000_000).Draw(rt, "award")
		result := xp.Award(level, currentXP, awardAmt, cfg)
		if result.NewLevel > cfg.LevelCap {
			rt.Fatalf("level %d exceeded cap %d", result.NewLevel, cfg.LevelCap)
		}
		if result.NewLevel < level {
			rt.Fatalf("level went backward: %d → %d", level, result.NewLevel)
		}
	})
}
