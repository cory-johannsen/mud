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

// ---------------------------------------------------------------------------
// Test double for CharacterFeatsRepo
// ---------------------------------------------------------------------------

type fakeFeatsRepo struct{ feats map[string]bool }

func (r *fakeFeatsRepo) GetAll(_ context.Context, _ int64) ([]string, error) {
	out := make([]string, 0, len(r.feats))
	for id := range r.feats {
		out = append(out, id)
	}
	return out, nil
}
func (r *fakeFeatsRepo) Add(_ context.Context, _ int64, featID string) error {
	if r.feats == nil {
		r.feats = make(map[string]bool)
	}
	r.feats[featID] = true
	return nil
}

// ---------------------------------------------------------------------------
// Tests for ApplyFeatGrant
// ---------------------------------------------------------------------------

func TestApplyFeatGrant_GrantsFixedFeats(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	existing := map[string]bool{}
	grants := &ruleset.FeatGrants{Fixed: []string{"snap_shot", "raging_threat"}}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"snap_shot", "raging_threat"}, granted)
	assert.True(t, repo.feats["snap_shot"])
	assert.True(t, repo.feats["raging_threat"])
}

func TestApplyFeatGrant_SkipsDuplicateFixed(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{feats: map[string]bool{"snap_shot": true}}
	existing := map[string]bool{"snap_shot": true}
	grants := &ruleset.FeatGrants{Fixed: []string{"snap_shot", "raging_threat"}}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.Equal(t, []string{"raging_threat"}, granted)
}

func TestApplyFeatGrant_AutoPicksChoices(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	existing := map[string]bool{}
	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"reactive_block", "overpower", "snap_shot"},
			Count: 1,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.Len(t, granted, 1)
	assert.Equal(t, "reactive_block", granted[0]) // first from pool
}

func TestApplyFeatGrant_SkipsAlreadyOwnedChoices(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{feats: map[string]bool{"reactive_block": true}}
	existing := map[string]bool{"reactive_block": true}
	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"reactive_block", "overpower", "snap_shot"},
			Count: 1,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	// Already has 1 pool feat and count is 1 → no new grant needed.
	assert.Empty(t, granted)
}

func TestApplyFeatGrant_PicksRemainingChoices(t *testing.T) {
	t.Parallel()
	// Has 1 of 2 required choices already.
	repo := &fakeFeatsRepo{feats: map[string]bool{"reactive_block": true}}
	existing := map[string]bool{"reactive_block": true}
	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"reactive_block", "overpower", "snap_shot"},
			Count: 2,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.Len(t, granted, 1)
	assert.Equal(t, "overpower", granted[0])
}

// ---------------------------------------------------------------------------
// Tests for BackfillLevelUpFeats
// ---------------------------------------------------------------------------

func TestBackfillLevelUpFeats_NoOpAtLevelOne(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	sess := &session.PlayerSession{Level: 1}
	grants := map[int]*ruleset.FeatGrants{
		2: {Fixed: []string{"snap_shot"}},
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	assert.Empty(t, repo.feats)
}

func TestBackfillLevelUpFeats_GrantsMissingFeats(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{feats: map[string]bool{"creation_feat": true}}
	sess := &session.PlayerSession{Level: 4}
	grants := map[int]*ruleset.FeatGrants{
		2: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_a", "feat_b"}, Count: 1}},
		4: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_a", "feat_b", "feat_c"}, Count: 1}},
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	// Should have granted 2 feats (1 per level), plus creation_feat.
	assert.Len(t, repo.feats, 3)
}

func TestBackfillLevelUpFeats_IdempotentWhenAlreadyGranted(t *testing.T) {
	t.Parallel()
	// Player already has feats from levels 2 and 4.
	repo := &fakeFeatsRepo{feats: map[string]bool{"feat_a": true, "feat_b": true}}
	sess := &session.PlayerSession{Level: 4}
	grants := map[int]*ruleset.FeatGrants{
		2: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_a", "feat_c"}, Count: 1}},
		4: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_b", "feat_d"}, Count: 1}},
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	// No new feats added — already has 1 from each pool.
	assert.Len(t, repo.feats, 2)
}

func TestBackfillLevelUpFeats_SkipsHigherLevels(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	sess := &session.PlayerSession{Level: 3}
	grants := map[int]*ruleset.FeatGrants{
		2: {Fixed: []string{"feat_lvl2"}},
		4: {Fixed: []string{"feat_lvl4"}}, // above current level
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	assert.True(t, repo.feats["feat_lvl2"])
	assert.False(t, repo.feats["feat_lvl4"])
}

func TestProperty_BackfillLevelUpFeats_Idempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(2, 10).Draw(rt, "level")
		pool := []string{"feat_a", "feat_b", "feat_c", "feat_d", "feat_e"}

		grants := make(map[int]*ruleset.FeatGrants)
		for lvl := 2; lvl <= level; lvl += 2 {
			grants[lvl] = &ruleset.FeatGrants{
				Choices: &ruleset.FeatChoices{Pool: pool, Count: 1},
			}
		}

		repo := &fakeFeatsRepo{}
		sess := &session.PlayerSession{Level: level}
		ctx := context.Background()

		// First call.
		require.NoError(rt, BackfillLevelUpFeats(ctx, sess, 1, grants, nil, repo))
		countAfterFirst := len(repo.feats)

		// Second call must be idempotent.
		require.NoError(rt, BackfillLevelUpFeats(ctx, sess, 1, grants, nil, repo))
		assert.Equal(rt, countAfterFirst, len(repo.feats), "second call must not add more feats")
	})
}
