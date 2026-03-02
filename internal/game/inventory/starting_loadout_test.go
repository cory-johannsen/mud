package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func TestLoadStartingLoadout_BaseOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  armor:
    torso: kevlar_vest
  consumables:
    - item: canadian_bacon
      quantity: 2
  currency: 50
`)
	sl, err := inventory.LoadStartingLoadout(dir, "aggressor", "")
	require.NoError(t, err)
	assert.Equal(t, "combat_knife", sl.Weapon)
	assert.Equal(t, "kevlar_vest", sl.Armor[inventory.SlotTorso])
	assert.Equal(t, 50, sl.Currency)
	require.Len(t, sl.Consumables, 1)
	assert.Equal(t, "canadian_bacon", sl.Consumables[0].ItemID)
	assert.Equal(t, 2, sl.Consumables[0].Quantity)
}

func TestLoadStartingLoadout_TeamGunOverridesWeapon(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  armor:
    torso: kevlar_vest
  currency: 50
team_gun:
  weapon: ganger_pistol
  armor:
    torso: tactical_vest
`)
	sl, err := inventory.LoadStartingLoadout(dir, "aggressor", "gun")
	require.NoError(t, err)
	assert.Equal(t, "ganger_pistol", sl.Weapon)
	assert.Equal(t, "tactical_vest", sl.Armor[inventory.SlotTorso])
	// Currency not overridden by team
	assert.Equal(t, 50, sl.Currency)
}

func TestLoadStartingLoadout_JobOverrideWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  currency: 50
team_gun:
  weapon: ganger_pistol
`)
	jobOverride := &inventory.StartingLoadoutOverride{
		Weapon:   "heavy_revolver",
		Currency: 100,
	}
	sl, err := inventory.LoadStartingLoadoutWithOverride(dir, "aggressor", "gun", jobOverride)
	require.NoError(t, err)
	assert.Equal(t, "heavy_revolver", sl.Weapon)
	assert.Equal(t, 100, sl.Currency)
}

func TestLoadStartingLoadout_MissingArchetypeReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := inventory.LoadStartingLoadout(dir, "unknown_archetype", "")
	assert.Error(t, err)
}

func TestProperty_LoadStartingLoadout_NeverPanics(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  currency: 50
`)
	rapid.Check(t, func(rt *rapid.T) {
		archetype := rapid.SampledFrom([]string{"aggressor", "nonexistent"}).Draw(rt, "archetype")
		team := rapid.SampledFrom([]string{"", "gun", "machete"}).Draw(rt, "team")
		assert.NotPanics(rt, func() {
			inventory.LoadStartingLoadout(dir, archetype, team) //nolint:errcheck
		})
	})
}
