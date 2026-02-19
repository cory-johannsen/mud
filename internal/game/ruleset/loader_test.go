package ruleset_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestLoadRegions_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old_town.yaml"), `
id: old_town
name: "Old Town"
description: "The neon-stained ruins of Portland's oldest district."
modifiers:
  charisma: 2
  dexterity: 2
  strength: -2
traits:
  - street_smart
  - scrapper
`)
	regions, err := ruleset.LoadRegions(dir)
	require.NoError(t, err)
	require.Len(t, regions, 1)
	r := regions[0]
	assert.Equal(t, "old_town", r.ID)
	assert.Equal(t, "Old Town", r.Name)
	assert.Equal(t, 2, r.Modifiers["charisma"])
	assert.Equal(t, 2, r.Modifiers["dexterity"])
	assert.Equal(t, -2, r.Modifiers["strength"])
	assert.Equal(t, []string{"street_smart", "scrapper"}, r.Traits)
}

func TestLoadClasses_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ganger.yaml"), `
id: ganger
name: "Ganger"
description: "Melee combatant hardened by street warfare."
key_ability: strength
hit_points_per_level: 10
proficiencies:
  weapons: trained
  armor: trained
features:
  - name: "Gang Tactics"
    level: 1
    description: "You fight dirty and effectively in groups."
`)
	classes, err := ruleset.LoadClasses(dir)
	require.NoError(t, err)
	require.Len(t, classes, 1)
	c := classes[0]
	assert.Equal(t, "ganger", c.ID)
	assert.Equal(t, "Ganger", c.Name)
	assert.Equal(t, "strength", c.KeyAbility)
	assert.Equal(t, 10, c.HitPointsPerLevel)
	assert.Equal(t, "trained", c.Proficiencies["weapons"])
	require.Len(t, c.Features, 1)
	assert.Equal(t, "Gang Tactics", c.Features[0].Name)
	assert.Equal(t, 1, c.Features[0].Level)
}

func TestLoadRegions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	regions, err := ruleset.LoadRegions(dir)
	require.NoError(t, err)
	assert.Empty(t, regions)
}

func TestLoadRegions_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `{{{ not yaml`)
	_, err := ruleset.LoadRegions(dir)
	require.Error(t, err)
}

func TestLoadClasses_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	classes, err := ruleset.LoadClasses(dir)
	require.NoError(t, err)
	assert.Empty(t, classes)
}

func TestLoadClasses_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `{{{ not yaml`)
	_, err := ruleset.LoadClasses(dir)
	require.Error(t, err)
}

// Property: every loaded region has a non-empty ID and Name.
func TestLoadRegions_AllHaveIDAndName(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		dir := t.TempDir()
		for i := 0; i < n; i++ {
			content := fmt.Sprintf(`
id: region_%d
name: "Region %d"
description: "Test region."
modifiers: {}
traits: []
`, i, i)
			fname := filepath.Join(dir, fmt.Sprintf("region_%d.yaml", i))
			if err := os.WriteFile(fname, []byte(content), 0644); err != nil {
				rt.Fatal(err)
			}
		}
		regions, err := ruleset.LoadRegions(dir)
		if err != nil {
			rt.Fatal(err)
		}
		for _, r := range regions {
			if r.ID == "" {
				rt.Fatalf("region has empty ID")
			}
			if r.Name == "" {
				rt.Fatalf("region has empty Name")
			}
		}
	})
}

func TestLoadRegions_ActualContent(t *testing.T) {
	regions, err := ruleset.LoadRegions("../../../content/regions")
	require.NoError(t, err)
	assert.Len(t, regions, 5, "expected 5 home regions")
	ids := make(map[string]bool)
	for _, r := range regions {
		assert.NotEmpty(t, r.ID)
		assert.NotEmpty(t, r.Name)
		assert.NotEmpty(t, r.Description)
		assert.False(t, ids[r.ID], "duplicate region ID: %s", r.ID)
		ids[r.ID] = true
	}
}

func TestLoadClasses_ActualContent(t *testing.T) {
	classes, err := ruleset.LoadClasses("../../../content/classes")
	require.NoError(t, err)
	assert.Len(t, classes, 5, "expected 5 classes")
	for _, c := range classes {
		assert.NotEmpty(t, c.ID)
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.KeyAbility)
		assert.Greater(t, c.HitPointsPerLevel, 0)
	}
}

// Property: every loaded class has a non-empty ID, Name, and positive HitPointsPerLevel.
func TestLoadClasses_AllHaveRequiredFields(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		hpPerLevel := rapid.IntRange(6, 12).Draw(rt, "hpPerLevel")
		dir := t.TempDir()
		for i := 0; i < n; i++ {
			content := fmt.Sprintf(`
id: class_%d
name: "Class %d"
description: "Test class."
key_ability: strength
hit_points_per_level: %d
proficiencies: {}
features: []
`, i, i, hpPerLevel)
			fname := filepath.Join(dir, fmt.Sprintf("class_%d.yaml", i))
			if err := os.WriteFile(fname, []byte(content), 0644); err != nil {
				rt.Fatal(err)
			}
		}
		classes, err := ruleset.LoadClasses(dir)
		if err != nil {
			rt.Fatal(err)
		}
		for _, c := range classes {
			if c.ID == "" {
				rt.Fatalf("class has empty ID")
			}
			if c.Name == "" {
				rt.Fatalf("class has empty Name")
			}
			if c.HitPointsPerLevel <= 0 {
				rt.Fatalf("class has non-positive HitPointsPerLevel: %d", c.HitPointsPerLevel)
			}
		}
	})
}
