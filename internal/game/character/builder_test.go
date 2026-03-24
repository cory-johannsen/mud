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
	return &ruleset.Team{ID: "test_team", Name: "Test Team", StartRoom: "battle_infirmary"}
}

func TestBuildWithJob_AppliesRegionModifiers(t *testing.T) {
	region := makeRegion(map[string]int{
		"brutality": 2,
		"flair":     2,
		"savvy":     -2,
	})
	job := makeJob("brutality", 10)
	team := makeTeam()

	c, err := character.BuildWithJob("Hero", region, job, team)
	require.NoError(t, err)

	assert.Equal(t, 14, c.Abilities.Brutality) // 10 + 2 region + 2 key ability
	assert.Equal(t, 10, c.Abilities.Quickness) // base
	assert.Equal(t, 10, c.Abilities.Grit)      // base
	assert.Equal(t, 10, c.Abilities.Reasoning) // base
	assert.Equal(t, 8, c.Abilities.Savvy)      // 10 - 2
	assert.Equal(t, 12, c.Abilities.Flair)     // 10 + 2
}

func TestBuildWithJob_CalculatesHP(t *testing.T) {
	region := makeRegion(map[string]int{"grit": 2})
	job := makeJob("reasoning", 10)
	team := makeTeam()

	c, err := character.BuildWithJob("Hero", region, job, team)
	require.NoError(t, err)

	assert.Equal(t, 12, c.Abilities.Grit)
	assert.Equal(t, 11, c.MaxHP)
	assert.Equal(t, 11, c.CurrentHP)
}

func TestBuildWithJob_HPNeverBelowOne(t *testing.T) {
	region := makeRegion(map[string]int{"grit": -4})
	job := makeJob("brutality", 6)
	team := makeTeam()

	c, err := character.BuildWithJob("Hero", region, job, team)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, c.MaxHP, 1)
}

func TestBuildWithJob_EmptyNameError(t *testing.T) {
	_, err := character.BuildWithJob("", makeRegion(nil), makeJob("brutality", 8), makeTeam())
	require.Error(t, err)
}

func TestBuildWithJob_NilRegionError(t *testing.T) {
	_, err := character.BuildWithJob("Hero", nil, makeJob("brutality", 8), makeTeam())
	require.Error(t, err)
}

func TestBuildWithJob_NilJobError(t *testing.T) {
	_, err := character.BuildWithJob("Hero", makeRegion(nil), nil, makeTeam())
	require.Error(t, err)
}

func TestBuildWithJob_NilTeamError(t *testing.T) {
	_, err := character.BuildWithJob("Hero", makeRegion(nil), makeJob("brutality", 8), nil)
	require.Error(t, err)
}

func TestBuildWithJob_DefaultLocation(t *testing.T) {
	c, err := character.BuildWithJob("Hero", makeRegion(nil), makeJob("brutality", 8), makeTeam())
	require.NoError(t, err)
	assert.Equal(t, "battle_infirmary", c.Location)
	assert.Equal(t, 1, c.Level)
}

func TestBuildWithJob_FallbackLocationWhenStartRoomEmpty(t *testing.T) {
	team := &ruleset.Team{ID: "legacy_team", Name: "Legacy Team"}
	c, err := character.BuildWithJob("Hero", makeRegion(nil), makeJob("brutality", 8), team)
	require.NoError(t, err)
	assert.Equal(t, "grinders_row", c.Location)
}

// Property: MaxHP is always >= 1 regardless of region modifiers.
func TestBuildWithJob_MaxHPAlwaysPositive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		grtMod := rapid.IntRange(-8, 8).Draw(rt, "grtMod")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		region := makeRegion(map[string]int{"grit": grtMod})
		job := makeJob("brutality", hpPerLevel)
		c, err := character.BuildWithJob("Hero", region, job, makeTeam())
		if err != nil {
			rt.Fatal(err)
		}
		if c.MaxHP < 1 {
			rt.Fatalf("MaxHP %d < 1 with grtMod=%d hpPerLevel=%d", c.MaxHP, grtMod, hpPerLevel)
		}
	})
}

func TestBuildSkillsFromJob_FixedOnly(t *testing.T) {
	allIDs := []string{"parkour", "ghosting", "muscle", "patch_job", "hustle"}
	job := &ruleset.Job{
		SkillGrants: &ruleset.SkillGrants{
			Fixed: []string{"muscle", "patch_job"},
		},
	}
	skills := character.BuildSkillsFromJob(job, allIDs, nil)

	if len(skills) != len(allIDs) {
		t.Fatalf("expected %d skills, got %d", len(allIDs), len(skills))
	}
	if skills["muscle"] != "trained" {
		t.Errorf("expected muscle=trained, got %q", skills["muscle"])
	}
	if skills["patch_job"] != "trained" {
		t.Errorf("expected patch_job=trained, got %q", skills["patch_job"])
	}
	if skills["parkour"] != "untrained" {
		t.Errorf("expected parkour=untrained, got %q", skills["parkour"])
	}
}

func TestBuildSkillsFromJob_WithChoices(t *testing.T) {
	allIDs := []string{"parkour", "ghosting", "grift", "muscle", "patch_job"}
	job := &ruleset.Job{
		SkillGrants: &ruleset.SkillGrants{
			Fixed: []string{"muscle"},
			Choices: &ruleset.SkillChoices{
				Pool:  []string{"parkour", "ghosting", "grift"},
				Count: 2,
			},
		},
	}
	skills := character.BuildSkillsFromJob(job, allIDs, []string{"parkour", "ghosting"})

	if skills["muscle"] != "trained" {
		t.Errorf("expected muscle=trained")
	}
	if skills["parkour"] != "trained" {
		t.Errorf("expected parkour=trained (chosen)")
	}
	if skills["ghosting"] != "trained" {
		t.Errorf("expected ghosting=trained (chosen)")
	}
	if skills["grift"] != "untrained" {
		t.Errorf("expected grift=untrained (not chosen), got %q", skills["grift"])
	}
}

func TestBuildSkillsFromJob_NilGrantsAllUntrained(t *testing.T) {
	allIDs := []string{"parkour", "muscle"}
	job := &ruleset.Job{SkillGrants: nil}
	skills := character.BuildSkillsFromJob(job, allIDs, nil)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	for id, prof := range skills {
		if prof != "untrained" {
			t.Errorf("expected %s=untrained, got %q", id, prof)
		}
	}
}

func TestBuildFeatsFromJob_FixedAndChosen(t *testing.T) {
	job := &ruleset.Job{
		FeatGrants: &ruleset.FeatGrants{
			GeneralCount: 1,
			Fixed:        []string{"quick_dodge"},
			Choices:      &ruleset.FeatChoices{Pool: []string{"twin_strike", "trap_eye"}, Count: 1},
		},
	}
	chosen := []string{"twin_strike"}
	generalChosen := []string{"toughness"}
	skillChosen := []string{"combat_patch"}
	got := character.BuildFeatsFromJob(job, chosen, generalChosen, skillChosen)
	want := map[string]bool{
		"quick_dodge":  true,
		"twin_strike":  true,
		"toughness":    true,
		"combat_patch": true,
	}
	if len(got) != len(want) {
		t.Errorf("expected %d feats, got %d: %v", len(want), len(got), got)
	}
	for _, f := range got {
		if !want[f] {
			t.Errorf("unexpected feat %q", f)
		}
	}
}

func TestBuildFeatsFromJob_NilGrants(t *testing.T) {
	job := &ruleset.Job{FeatGrants: nil}
	got := character.BuildFeatsFromJob(job, nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty feats for nil FeatGrants, got %v", got)
	}
}

// Property: CurrentHP == MaxHP on a freshly built character.
func TestBuildWithJob_CurrentHPEqualsMaxHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		grtMod := rapid.IntRange(-4, 4).Draw(rt, "grtMod")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		region := makeRegion(map[string]int{"grit": grtMod})
		job := makeJob("quickness", hpPerLevel)
		c, err := character.BuildWithJob("Hero", region, job, makeTeam())
		if err != nil {
			rt.Fatal(err)
		}
		if c.CurrentHP != c.MaxHP {
			rt.Fatalf("CurrentHP %d != MaxHP %d on new character", c.CurrentHP, c.MaxHP)
		}
	})
}
