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

