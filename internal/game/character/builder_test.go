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

func makeJob(keyAbility string, hpPerLevel int) *ruleset.Job {
	return &ruleset.Job{
		ID:                "test_job",
		Name:              "Test Job",
		KeyAbility:        keyAbility,
		HitPointsPerLevel: hpPerLevel,
	}
}

func makeTeam() *ruleset.Team {
	return &ruleset.Team{ID: "test_team", Name: "Test Team"}
}

func TestBuildWithJob_AppliesRegionModifiers(t *testing.T) {
	region := makeRegion(map[string]int{
		"strength": 2,
		"charisma": 2,
		"wisdom":   -2,
	})
	job := makeJob("strength", 10)
	team := makeTeam()

	c, err := character.BuildWithJob("Hero", region, job, team)
	require.NoError(t, err)

	assert.Equal(t, 14, c.Abilities.Strength)    // 10 + 2 region + 2 key ability
	assert.Equal(t, 10, c.Abilities.Dexterity)    // base
	assert.Equal(t, 10, c.Abilities.Constitution) // base
	assert.Equal(t, 10, c.Abilities.Intelligence) // base
	assert.Equal(t, 8, c.Abilities.Wisdom)        // 10 - 2
	assert.Equal(t, 12, c.Abilities.Charisma)     // 10 + 2
}

func TestBuildWithJob_CalculatesHP(t *testing.T) {
	region := makeRegion(map[string]int{"constitution": 2})
	job := makeJob("intelligence", 10)
	team := makeTeam()

	c, err := character.BuildWithJob("Hero", region, job, team)
	require.NoError(t, err)

	assert.Equal(t, 12, c.Abilities.Constitution)
	assert.Equal(t, 11, c.MaxHP)
	assert.Equal(t, 11, c.CurrentHP)
}

func TestBuildWithJob_HPNeverBelowOne(t *testing.T) {
	region := makeRegion(map[string]int{"constitution": -4})
	job := makeJob("strength", 6)
	team := makeTeam()

	c, err := character.BuildWithJob("Hero", region, job, team)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, c.MaxHP, 1)
}

func TestBuildWithJob_EmptyNameError(t *testing.T) {
	_, err := character.BuildWithJob("", makeRegion(nil), makeJob("strength", 8), makeTeam())
	require.Error(t, err)
}

func TestBuildWithJob_NilRegionError(t *testing.T) {
	_, err := character.BuildWithJob("Hero", nil, makeJob("strength", 8), makeTeam())
	require.Error(t, err)
}

func TestBuildWithJob_NilJobError(t *testing.T) {
	_, err := character.BuildWithJob("Hero", makeRegion(nil), nil, makeTeam())
	require.Error(t, err)
}

func TestBuildWithJob_NilTeamError(t *testing.T) {
	_, err := character.BuildWithJob("Hero", makeRegion(nil), makeJob("strength", 8), nil)
	require.Error(t, err)
}

func TestBuildWithJob_DefaultLocation(t *testing.T) {
	c, err := character.BuildWithJob("Hero", makeRegion(nil), makeJob("strength", 8), makeTeam())
	require.NoError(t, err)
	assert.Equal(t, "grinders_row", c.Location)
	assert.Equal(t, 1, c.Level)
}

// Property: MaxHP is always >= 1 regardless of region modifiers.
func TestBuildWithJob_MaxHPAlwaysPositive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		conMod := rapid.IntRange(-8, 8).Draw(rt, "conMod")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		region := makeRegion(map[string]int{"constitution": conMod})
		job := makeJob("strength", hpPerLevel)
		c, err := character.BuildWithJob("Hero", region, job, makeTeam())
		if err != nil {
			rt.Fatal(err)
		}
		if c.MaxHP < 1 {
			rt.Fatalf("MaxHP %d < 1 with conMod=%d hpPerLevel=%d", c.MaxHP, conMod, hpPerLevel)
		}
	})
}

// Property: CurrentHP == MaxHP on a freshly built character.
func TestBuildWithJob_CurrentHPEqualsMaxHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		conMod := rapid.IntRange(-4, 4).Draw(rt, "conMod")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		region := makeRegion(map[string]int{"constitution": conMod})
		job := makeJob("dexterity", hpPerLevel)
		c, err := character.BuildWithJob("Hero", region, job, makeTeam())
		if err != nil {
			rt.Fatal(err)
		}
		if c.CurrentHP != c.MaxHP {
			rt.Fatalf("CurrentHP %d != MaxHP %d on new character", c.CurrentHP, c.MaxHP)
		}
	})
}
