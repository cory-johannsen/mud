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
// Test double for CharacterFeatLevelGrantsRepo
// ---------------------------------------------------------------------------

type fakeLevelGrantsRepo struct{ granted map[int]bool }

func (r *fakeLevelGrantsRepo) IsLevelGranted(_ context.Context, _ int64, level int) (bool, error) {
	if r.granted == nil {
		return false, nil
	}
	return r.granted[level], nil
}
func (r *fakeLevelGrantsRepo) MarkLevelGranted(_ context.Context, _ int64, level int) error {
	if r.granted == nil {
		r.granted = make(map[int]bool)
	}
	r.granted[level] = true
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
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo, &fakeLevelGrantsRepo{})
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
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo, &fakeLevelGrantsRepo{})
	require.NoError(t, err)
	// Should have granted 2 feats (1 per level), plus creation_feat.
	assert.Len(t, repo.feats, 3)
}

func TestBackfillLevelUpFeats_IdempotentWhenAlreadyGranted(t *testing.T) {
	t.Parallel()
	// Player already has feats from levels 2 and 4 — both levels are recorded in
	// levelGrantsRepo as already processed, so no further grants occur.
	repo := &fakeFeatsRepo{feats: map[string]bool{"feat_a": true, "feat_b": true}}
	sess := &session.PlayerSession{Level: 4}
	grants := map[int]*ruleset.FeatGrants{
		2: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_a", "feat_c"}, Count: 1}},
		4: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_b", "feat_d"}, Count: 1}},
	}
	lgr := &fakeLevelGrantsRepo{granted: map[int]bool{2: true, 4: true}}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo, lgr)
	require.NoError(t, err)
	// No new feats added — both levels are already marked as granted.
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
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo, &fakeLevelGrantsRepo{})
	require.NoError(t, err)
	assert.True(t, repo.feats["feat_lvl2"])
	assert.False(t, repo.feats["feat_lvl4"])
}

// TestBackfillLevelUpFeats_CreationFeatPoolOverlap is the regression test for
// BUG: creation feats in same pool as level-up feats must not block level grants.
func TestBackfillLevelUpFeats_CreationFeatPoolOverlap(t *testing.T) {
	t.Parallel()
	// Character got reactive_block at creation; the level-2 pool also contains it.
	// Without the fix, alreadyOwned=1 satisfied count=1 → remaining=0 → no grant.
	// With the fix, levelGrantsRepo controls idempotency: level 2 is not yet granted,
	// so BackfillLevelUpFeats grants from pool (skipping reactive_block since it's in
	// existing) and picks overpower instead.
	repo := &fakeFeatsRepo{feats: map[string]bool{"reactive_block": true}}
	sess := &session.PlayerSession{Level: 2}
	grants := map[int]*ruleset.FeatGrants{
		2: {Choices: &ruleset.FeatChoices{Pool: []string{"reactive_block", "overpower"}, Count: 1}},
	}
	lgr := &fakeLevelGrantsRepo{}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo, lgr)
	require.NoError(t, err)
	// Should have granted overpower (reactive_block already owned, pool picks next).
	assert.True(t, repo.feats["overpower"], "expected overpower to be granted as level-2 feat")
	assert.True(t, lgr.granted[2], "expected level 2 to be marked as granted")
}

// TestApplyFeatGrant_PF2EIDResolved is the regression test for the bug where a
// pool entry using a legacy PF2E ID (e.g. "rage") would not match a canonically-
// stored feat (e.g. "wrath"), causing the player to be offered a feat they already own.
func TestApplyFeatGrant_PF2EIDResolved(t *testing.T) {
	t.Parallel()
	// Build a registry with "wrath" (canonical) + pf2e alias "rage".
	wrathFeat := &ruleset.Feat{ID: "wrath", Name: "Wrath", Category: "job", PF2E: "rage"}
	reg := ruleset.NewFeatRegistry([]*ruleset.Feat{wrathFeat})

	// Player already has "wrath" stored under its canonical ID.
	repo := &fakeFeatsRepo{feats: map[string]bool{"wrath": true}}
	existing := map[string]bool{"wrath": true}

	// Pool uses the legacy PF2E ID "rage".
	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"rage"},
			Count: 1,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, reg, repo)
	require.NoError(t, err)
	// Player already owns "wrath" (canonical of "rage") — nothing should be granted.
	assert.Empty(t, granted, "should not re-grant a feat the player already owns via PF2E alias")
}

// TestApplyFeatGrant_PF2EIDGrantsCanonical verifies that when a pool uses a legacy
// PF2E ID and the player does NOT yet own the feat, it is stored under the canonical ID.
func TestApplyFeatGrant_PF2EIDGrantsCanonical(t *testing.T) {
	t.Parallel()
	wrathFeat := &ruleset.Feat{ID: "wrath", Name: "Wrath", Category: "job", PF2E: "rage"}
	reg := ruleset.NewFeatRegistry([]*ruleset.Feat{wrathFeat})

	repo := &fakeFeatsRepo{}
	existing := map[string]bool{}

	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"rage"},
			Count: 1,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, reg, repo)
	require.NoError(t, err)
	// Should be granted and stored as the canonical ID "wrath", not "rage".
	require.Len(t, granted, 1)
	assert.Equal(t, "wrath", granted[0])
	assert.True(t, repo.feats["wrath"])
	assert.False(t, repo.feats["rage"], "should not store feat under legacy PF2E ID")
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
		lgr := &fakeLevelGrantsRepo{}
		require.NoError(rt, BackfillLevelUpFeats(ctx, sess, 1, grants, nil, repo, lgr))
		countAfterFirst := len(repo.feats)

		// Second call must be idempotent.
		require.NoError(rt, BackfillLevelUpFeats(ctx, sess, 1, grants, nil, repo, lgr))
		assert.Equal(rt, countAfterFirst, len(repo.feats), "second call must not add more feats")
	})
}
