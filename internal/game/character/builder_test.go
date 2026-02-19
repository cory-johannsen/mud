package character_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func makeRegion(mods map[string]int) *ruleset.Region {
	return &ruleset.Region{
		ID:        "test_region",
		Name:      "Test Region",
		Modifiers: mods,
	}
}

func makeClass(keyAbility string, hpPerLevel int) *ruleset.Class {
	return &ruleset.Class{
		ID:                "test_class",
		Name:              "Test Class",
		KeyAbility:        keyAbility,
		HitPointsPerLevel: hpPerLevel,
	}
}

func TestBuildCharacter_AppliesRegionModifiers(t *testing.T) {
	region := makeRegion(map[string]int{
		"strength": 2,
		"charisma": 2,
		"wisdom":   -2,
	})
	class := makeClass("strength", 10)

	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)

	assert.Equal(t, 14, c.Abilities.Strength)    // 10 + 2 region + 2 key ability
	assert.Equal(t, 10, c.Abilities.Dexterity)    // base
	assert.Equal(t, 10, c.Abilities.Constitution) // base
	assert.Equal(t, 10, c.Abilities.Intelligence) // base
	assert.Equal(t, 8, c.Abilities.Wisdom)        // 10 - 2
	assert.Equal(t, 12, c.Abilities.Charisma)     // 10 + 2
}

func TestBuildCharacter_CalculatesHP(t *testing.T) {
	// CON 12 → modifier +1; HP = 10 + 1 = 11
	region := makeRegion(map[string]int{"constitution": 2})
	class := makeClass("intelligence", 10)

	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)

	assert.Equal(t, 12, c.Abilities.Constitution)
	assert.Equal(t, 11, c.MaxHP)
	assert.Equal(t, 11, c.CurrentHP)
}

func TestBuildCharacter_HPNeverBelowOne(t *testing.T) {
	// Large CON penalty → HP floor at 1
	region := makeRegion(map[string]int{"constitution": -4})
	class := makeClass("strength", 6)

	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, c.MaxHP, 1)
}

func TestBuildCharacter_NameSet(t *testing.T) {
	region := makeRegion(nil)
	class := makeClass("strength", 8)
	c, err := character.Build("Aria", region, class)
	require.NoError(t, err)
	assert.Equal(t, "Aria", c.Name)
}

func TestBuildCharacter_RegionAndClassIDSet(t *testing.T) {
	region := &ruleset.Region{ID: "old_town", Name: "Old Town", Modifiers: nil}
	class := &ruleset.Class{ID: "ganger", Name: "Ganger", KeyAbility: "strength", HitPointsPerLevel: 10}
	c, err := character.Build("Zara", region, class)
	require.NoError(t, err)
	assert.Equal(t, "old_town", c.Region)
	assert.Equal(t, "ganger", c.Class)
}

func TestBuildCharacter_LevelOne(t *testing.T) {
	region := makeRegion(nil)
	class := makeClass("strength", 8)
	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)
	assert.Equal(t, 1, c.Level)
	assert.Equal(t, 0, c.Experience)
}

func TestBuildCharacter_DefaultLocation(t *testing.T) {
	region := makeRegion(nil)
	class := makeClass("strength", 8)
	c, err := character.Build("Hero", region, class)
	require.NoError(t, err)
	assert.Equal(t, "grinders_row", c.Location)
}

func TestBuildCharacter_EmptyNameError(t *testing.T) {
	region := makeRegion(nil)
	class := makeClass("strength", 8)
	_, err := character.Build("", region, class)
	require.Error(t, err)
}

func TestBuildCharacter_NilRegionError(t *testing.T) {
	class := makeClass("strength", 8)
	_, err := character.Build("Hero", nil, class)
	require.Error(t, err)
}

func TestBuildCharacter_NilClassError(t *testing.T) {
	region := makeRegion(nil)
	_, err := character.Build("Hero", region, nil)
	require.Error(t, err)
}

// Property: MaxHP is always >= 1 regardless of region modifiers.
func TestBuildCharacter_MaxHPAlwaysPositive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		conMod := rapid.IntRange(-8, 8).Draw(rt, "conMod")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		region := makeRegion(map[string]int{"constitution": conMod})
		class := makeClass("strength", hpPerLevel)
		c, err := character.Build("Hero", region, class)
		if err != nil {
			rt.Fatal(err)
		}
		if c.MaxHP < 1 {
			rt.Fatalf("MaxHP %d < 1 with conMod=%d hpPerLevel=%d", c.MaxHP, conMod, hpPerLevel)
		}
	})
}

// Property: CurrentHP == MaxHP on a freshly built character.
func TestBuildCharacter_CurrentHPEqualsMaxHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		conMod := rapid.IntRange(-4, 4).Draw(rt, "conMod")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		region := makeRegion(map[string]int{"constitution": conMod})
		class := makeClass("dexterity", hpPerLevel)
		c, err := character.Build("Hero", region, class)
		if err != nil {
			rt.Fatal(err)
		}
		if c.CurrentHP != c.MaxHP {
			rt.Fatalf("CurrentHP %d != MaxHP %d on new character", c.CurrentHP, c.MaxHP)
		}
	})
}

// Property: all ability scores are >= 4 (no modifier in normal play drops below 4).
func TestBuildCharacter_AbilitiesInReasonableRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mod := rapid.IntRange(-4, 4).Draw(rt, "mod")
		ability := rapid.SampledFrom([]string{
			"strength", "dexterity", "constitution",
			"intelligence", "wisdom", "charisma",
		}).Draw(rt, "ability")
		region := makeRegion(map[string]int{ability: mod})
		class := makeClass("strength", 8)
		c, err := character.Build("Hero", region, class)
		if err != nil {
			rt.Fatal(err)
		}
		scores := []int{
			c.Abilities.Strength, c.Abilities.Dexterity, c.Abilities.Constitution,
			c.Abilities.Intelligence, c.Abilities.Wisdom, c.Abilities.Charisma,
		}
		for _, s := range scores {
			if s < 4 {
				rt.Fatalf("ability score %d < 4", s)
			}
		}
	})
}
