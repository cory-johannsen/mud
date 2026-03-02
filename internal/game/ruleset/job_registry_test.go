package ruleset_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestJobRegistry_TeamFor_KnownJob(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "libertarian", Team: "gun"})
	assert.Equal(t, "gun", reg.TeamFor("libertarian"))
}

func TestJobRegistry_TeamFor_UnknownJobReturnsEmpty(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	assert.Equal(t, "", reg.TeamFor("nonexistent"))
}

func TestJobRegistry_TeamFor_NoTeamJobReturnsEmpty(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "drifter", Team: ""})
	assert.Equal(t, "", reg.TeamFor("drifter"))
}

func TestJobRegistry_LoadFromDir(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../../content/jobs")
	require.NoError(t, err)
	reg := ruleset.NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	// All jobs must round-trip
	for _, j := range jobs {
		assert.Equal(t, j.Team, reg.TeamFor(j.ID))
	}
}

func TestJobRegistry_Register_NilJobPanics(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	assert.Panics(t, func() { reg.Register(nil) })
}

func TestJobRegistry_Register_EmptyIDPanics(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	assert.Panics(t, func() { reg.Register(&ruleset.Job{ID: "", Team: "gun"}) })
}

func TestJob_ParsesStartingInventory(t *testing.T) {
	dir := t.TempDir()
	content := `id: boot_gun
name: Boot (Gun)
archetype: aggressor
team: gun
key_ability: quickness
hit_points_per_level: 8
starting_inventory:
  weapon: heavy_revolver
  currency: 100
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "boot_gun.yaml"), []byte(content), 0644))
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].StartingInventory)
	assert.Equal(t, "heavy_revolver", jobs[0].StartingInventory.Weapon)
	assert.Equal(t, 100, jobs[0].StartingInventory.Currency)
}

func TestJob_ParsesStartingInventory_WithArmor(t *testing.T) {
	dir := t.TempDir()
	content := `id: striker_gun
name: Striker (Gun)
archetype: aggressor
team: gun
key_ability: brutality
hit_points_per_level: 10
starting_inventory:
  weapon: heavy_revolver
  armor:
    head: combat_helmet
  currency: 50
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "striker_gun.yaml"), []byte(content), 0644))
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].StartingInventory)
	assert.Equal(t, "heavy_revolver", jobs[0].StartingInventory.Weapon)
	assert.Equal(t, "combat_helmet", jobs[0].StartingInventory.Armor["head"])
	assert.Equal(t, 50, jobs[0].StartingInventory.Currency)
}

func TestJob_NoStartingInventory_FieldIsNil(t *testing.T) {
	dir := t.TempDir()
	content := `id: drifter
name: Drifter
archetype: wanderer
team: ""
key_ability: grit
hit_points_per_level: 8
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "drifter.yaml"), []byte(content), 0644))
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Nil(t, jobs[0].StartingInventory)
}

func TestJobRegistry_Job_ReturnsKnownJob(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	job := &ruleset.Job{ID: "libertarian", Team: "gun"}
	reg.Register(job)
	got, ok := reg.Job("libertarian")
	require.True(t, ok)
	assert.Equal(t, job, got)
}

func TestJobRegistry_Job_UnknownReturnsNilFalse(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	got, ok := reg.Job("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestProperty_JobRegistry_RoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z_]{1,15}`).Draw(rt, "id")
		team := rapid.SampledFrom([]string{"", "gun", "machete"}).Draw(rt, "team")
		reg := ruleset.NewJobRegistry()
		reg.Register(&ruleset.Job{ID: id, Team: team})
		assert.Equal(t, team, reg.TeamFor(id))
	})
}
