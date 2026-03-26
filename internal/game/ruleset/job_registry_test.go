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
tier: 1
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
tier: 1
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
tier: 1
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

func TestJobRegistry_ArchetypesForTeam_ReturnsMatchingArchetypes(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "striker_gun", Archetype: "aggressor", Team: "gun"})
	reg.Register(&ruleset.Job{ID: "striker_machete", Archetype: "aggressor", Team: "machete"})
	reg.Register(&ruleset.Job{ID: "fence", Archetype: "criminal", Team: "machete"})

	gun := reg.ArchetypesForTeam("gun")
	assert.Equal(t, []string{"aggressor"}, gun)

	machete := reg.ArchetypesForTeam("machete")
	assert.ElementsMatch(t, []string{"aggressor", "criminal"}, machete)
}

func TestJobRegistry_ArchetypesForTeam_UnknownTeamReturnsEmpty(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "striker_gun", Archetype: "aggressor", Team: "gun"})
	assert.Empty(t, reg.ArchetypesForTeam("unknown"))
}

func TestJobRegistry_JobsForTeamAndArchetype_FiltersCorrectly(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "striker_gun", Archetype: "aggressor", Team: "gun"})
	reg.Register(&ruleset.Job{ID: "fence", Archetype: "criminal", Team: "machete"})
	reg.Register(&ruleset.Job{ID: "scout", Archetype: "aggressor", Team: "machete"})

	jobs := reg.JobsForTeamAndArchetype("gun", "aggressor")
	require.Len(t, jobs, 1)
	assert.Equal(t, "striker_gun", jobs[0].ID)
}

func TestJobRegistry_JobsForTeamAndArchetype_NoMatchReturnsEmpty(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "striker_gun", Archetype: "aggressor", Team: "gun"})
	assert.Empty(t, reg.JobsForTeamAndArchetype("machete", "aggressor"))
}

func TestJob_SkillGrantsFieldExists(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../../content/jobs")
	require.NoError(t, err)
	reg := ruleset.NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	job, ok := reg.Job("anarchist")
	require.True(t, ok, "anarchist job not found")
	// SkillGrants field must exist on Job (may be nil until Task 4 adds YAML)
	_ = job.SkillGrants
}

func TestProperty_JobRegistry_ArchetypesForTeam_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		team := rapid.String().Draw(rt, "team")
		reg := ruleset.NewJobRegistry()
		reg.Register(&ruleset.Job{ID: "j1", Archetype: "a1", Team: "gun"})
		_ = reg.ArchetypesForTeam(team)
		_ = reg.JobsForTeamAndArchetype(team, "a1")
	})
}

// TestJob_MissingTier_FatalError asserts that loading a job YAML without a
// tier field returns an error containing REQ-DTQ-13 context.
func TestJob_MissingTier_FatalError(t *testing.T) {
	dir := t.TempDir()
	content := `id: test_job
name: Test Job
archetype: aggressor
key_ability: brutality
hit_points_per_level: 8
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test_job.yaml"), []byte(content), 0644))
	_, err := ruleset.LoadJobs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "REQ-DTQ-13")
}

// TestJob_TierField_LoadsCorrectly asserts that a job YAML with tier: 2 is
// parsed and the Tier field equals 2 after loading.
func TestJob_TierField_LoadsCorrectly(t *testing.T) {
	dir := t.TempDir()
	content := `id: test_job
name: Test Job
archetype: aggressor
tier: 2
key_ability: brutality
hit_points_per_level: 8
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test_job.yaml"), []byte(content), 0644))
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, 2, jobs[0].Tier)
}

// TestProperty_Job_TierInRange asserts that for any valid tier value [1,3],
// the job loads without error and Tier is preserved exactly.
func TestProperty_Job_TierInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.IntRange(1, 3).Draw(rt, "tier")
		dir := t.TempDir()
		content := fmt.Sprintf(`id: prop_job
name: Prop Job
archetype: aggressor
tier: %d
key_ability: brutality
hit_points_per_level: 8
`, tier)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "prop_job.yaml"), []byte(content), 0644))
		jobs, err := ruleset.LoadJobs(dir)
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		assert.Equal(t, tier, jobs[0].Tier)
	})
}
