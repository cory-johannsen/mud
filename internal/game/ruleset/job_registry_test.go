package ruleset_test

import (
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

func TestProperty_JobRegistry_RoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z_]{1,15}`).Draw(rt, "id")
		team := rapid.SampledFrom([]string{"", "gun", "machete"}).Draw(rt, "team")
		reg := ruleset.NewJobRegistry()
		reg.Register(&ruleset.Job{ID: id, Team: team})
		assert.Equal(t, team, reg.TeamFor(id))
	})
}
