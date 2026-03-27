package ruleset_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeJobFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("writeJobFile: %v", err)
	}
}

func TestLoadJob_MissingTier_Fatal(t *testing.T) {
	// job YAML without tier field should cause LoadJobs error
	dir := t.TempDir()
	writeJobFile(t, dir, "notier.yaml", `
id: notier
name: No Tier
archetype: test
key_ability: grit
hit_points_per_level: 8
`)
	_, err := ruleset.LoadJobs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tier")
}

func TestLoadJob_TierPresent_NoError(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "tiered.yaml", `
id: tiered
name: Tiered
archetype: test
key_ability: grit
hit_points_per_level: 8
tier: 1
`)
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, 1, jobs[0].Tier)
}

func TestLoadJob_DrawbackDef_FullSchema(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "withdrawback.yaml", `
id: withdrawback
name: With Drawback
archetype: test
key_ability: grit
hit_points_per_level: 8
tier: 1
drawbacks:
  - id: glass_jaw
    type: passive
    description: "You hit hard but can't take a hit."
    stat_modifier:
      stat: grit
      amount: -1
  - id: blood_fury
    type: situational
    trigger: on_leave_combat_without_kill
    effect_condition_id: demoralized
    duration: "1h"
    description: "If you didn't finish anyone off, you spiral."
`)
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Len(t, jobs[0].Drawbacks, 2)
	assert.Equal(t, "glass_jaw", jobs[0].Drawbacks[0].ID)
	assert.Equal(t, "passive", jobs[0].Drawbacks[0].Type)
	assert.NotNil(t, jobs[0].Drawbacks[0].StatModifier)
	assert.Equal(t, "grit", jobs[0].Drawbacks[0].StatModifier.Stat)
	assert.Equal(t, -1, jobs[0].Drawbacks[0].StatModifier.Amount)
	assert.Equal(t, "situational", jobs[0].Drawbacks[1].Type)
	assert.Equal(t, "on_leave_combat_without_kill", jobs[0].Drawbacks[1].Trigger)
	assert.Equal(t, "1h", jobs[0].Drawbacks[1].Duration)
}

func TestLoadJob_InvalidDrawbackDuration_Fatal(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "badduration.yaml", `
id: badduration
name: Bad Duration
archetype: test
key_ability: grit
hit_points_per_level: 8
tier: 1
drawbacks:
  - id: bad_timer
    type: situational
    trigger: on_leave_combat_without_kill
    effect_condition_id: demoralized
    duration: "not-a-duration"
    description: "Invalid duration."
`)
	_, err := ruleset.LoadJobs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duration")
}

func TestLoadJob_Tier2_DefaultsMinLevel10(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "specialist.yaml", `
id: specialist_goon
name: Specialist Goon
archetype: aggressor
key_ability: brutality
hit_points_per_level: 10
tier: 2
`)
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	assert.Equal(t, 10, jobs[0].AdvancementRequirements.MinLevel)
}

func TestLoadJobs_AllHaveTier(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../../content/jobs")
	require.NoError(t, err)
	for _, j := range jobs {
		assert.NotZero(t, j.Tier, "job %q missing tier field", j.ID)
	}
}
