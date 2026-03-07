package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// stubProficienciesRepo is an in-memory implementation of CharacterProficienciesRepository for testing.
type stubProficienciesRepo struct {
	data map[int64]map[string]string
}

func newStubProficienciesRepo() *stubProficienciesRepo {
	return &stubProficienciesRepo{data: make(map[int64]map[string]string)}
}

func (r *stubProficienciesRepo) GetAll(_ context.Context, characterID int64) (map[string]string, error) {
	if m, ok := r.data[characterID]; ok {
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out, nil
	}
	return map[string]string{}, nil
}

func (r *stubProficienciesRepo) Upsert(_ context.Context, characterID int64, category, rank string) error {
	if r.data[characterID] == nil {
		r.data[characterID] = make(map[string]string)
	}
	r.data[characterID][category] = rank
	return nil
}

// buildProficienciesFromJob applies the same backfill logic used in Session() login:
// always grants unarmored=trained and merges job proficiencies for categories not yet set.
//
// Precondition: existing and jobProfs may be nil (treated as empty).
// Postcondition: returns the map that should be upserted to the repo; unarmored is always present.
func buildProficienciesFromJob(existing map[string]string, jobProfs map[string]string) map[string]string {
	toUpsert := map[string]string{"unarmored": "trained"}
	for cat, rank := range jobProfs {
		toUpsert[cat] = rank
	}
	out := make(map[string]string)
	for cat, rank := range toUpsert {
		if _, alreadySet := existing[cat]; !alreadySet {
			out[cat] = rank
		}
	}
	return out
}

// TestProficiencyBackfill_UnarmoredAlwaysGranted verifies that `unarmored: trained`
// is always added to characters regardless of job proficiencies.
func TestProficiencyBackfill_UnarmoredAlwaysGranted(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		existing := map[string]string{}
		jobProfs := map[string]string{} // empty job proficiencies
		toUpsert := buildProficienciesFromJob(existing, jobProfs)
		rank, ok := toUpsert["unarmored"]
		assert.True(rt, ok, "unarmored must be present")
		assert.Equal(rt, "trained", rank, "unarmored must be trained")
	})
}

// TestProficiencyBackfill_JobProficienciesAdded verifies that proficiencies from
// a job definition are added when not already in the existing set.
func TestProficiencyBackfill_JobProficienciesAdded(t *testing.T) {
	jobProfs := map[string]string{
		"simple_weapons": "trained",
		"light_armor":    "trained",
	}
	existing := map[string]string{}
	toUpsert := buildProficienciesFromJob(existing, jobProfs)
	assert.Equal(t, "trained", toUpsert["simple_weapons"])
	assert.Equal(t, "trained", toUpsert["light_armor"])
	assert.Equal(t, "trained", toUpsert["unarmored"])
}

// TestProficiencyBackfill_ExistingNotOverwritten verifies that already-set proficiencies
// are not overwritten by the backfill (idempotency guarantee).
func TestProficiencyBackfill_ExistingNotOverwritten(t *testing.T) {
	jobProfs := map[string]string{"light_armor": "trained"}
	// Simulate character already having a higher rank.
	existing := map[string]string{"light_armor": "expert"}
	toUpsert := buildProficienciesFromJob(existing, jobProfs)
	// light_armor should NOT be in toUpsert (already set).
	_, present := toUpsert["light_armor"]
	assert.False(t, present, "already-set proficiency must not be upserted again")
}

// TestProficiencyBackfill_Idempotent verifies that running the backfill twice
// produces the same state (no duplicate entries, no overwrite).
func TestProficiencyBackfill_Idempotent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		repo := newStubProficienciesRepo()
		ctx := context.Background()
		characterID := int64(42)

		jobProfs := map[string]string{
			"simple_weapons": "trained",
			"light_armor":    "trained",
		}

		// First backfill.
		existing1, err := repo.GetAll(ctx, characterID)
		require.NoError(rt, err)
		for cat, rank := range buildProficienciesFromJob(existing1, jobProfs) {
			require.NoError(rt, repo.Upsert(ctx, characterID, cat, rank))
		}

		// Second backfill (all categories already set — nothing changes).
		existing2, err := repo.GetAll(ctx, characterID)
		require.NoError(rt, err)
		toUpsert2 := buildProficienciesFromJob(existing2, jobProfs)
		assert.Empty(rt, toUpsert2, "second backfill must have nothing to upsert")

		// Final state matches expectations.
		final, err := repo.GetAll(ctx, characterID)
		require.NoError(rt, err)
		assert.Equal(rt, "trained", final["unarmored"])
		assert.Equal(rt, "trained", final["simple_weapons"])
		assert.Equal(rt, "trained", final["light_armor"])
	})
}

// TestProficiencyBackfill_PlayerSession_ProficienciesFieldExists verifies that
// the Proficiencies field is accessible on PlayerSession (compile-time check).
func TestProficiencyBackfill_PlayerSession_ProficienciesFieldExists(t *testing.T) {
	_, sessMgr := testWorldAndSession(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "prof_uid_1",
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Role:        "player",
	})
	require.NoError(t, err)
	// Assign proficiencies map — should compile.
	sess.Proficiencies = map[string]string{"unarmored": "trained"}
	assert.Equal(t, "trained", sess.Proficiencies["unarmored"])
}

// TestCharacterProficienciesRepository_Interface verifies that *stubProficienciesRepo
// satisfies the CharacterProficienciesRepository interface.
func TestCharacterProficienciesRepository_Interface(t *testing.T) {
	var _ CharacterProficienciesRepository = newStubProficienciesRepo()
}

// TestJobRegistry_HasProficiencies verifies that jobs loaded with a proficiencies section
// expose their proficiencies through the registry.
func TestJobRegistry_HasProficiencies(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../content/jobs")
	require.NoError(t, err)
	require.NotEmpty(t, jobs)
	reg := ruleset.NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	// grunt is known to have simple_weapons and light_armor proficiencies.
	job, ok := reg.Job("grunt")
	require.True(t, ok, "grunt job must exist")
	require.NotNil(t, job.Proficiencies)
	assert.Equal(t, "trained", job.Proficiencies["simple_weapons"])
	assert.Equal(t, "trained", job.Proficiencies["light_armor"])
}
