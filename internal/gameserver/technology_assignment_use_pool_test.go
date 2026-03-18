package gameserver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/gameserver"
)

// fakeSpontaneousUsePoolRepo is an in-memory SpontaneousUsePoolRepo for tests.
type fakeSpontaneousUsePoolRepo struct {
	pools   map[int]session.UsePool
	setCalls []usePoolSetCall
}

type usePoolSetCall struct {
	level, remaining, max int
}

func (r *fakeSpontaneousUsePoolRepo) GetAll(_ context.Context, _ int64) (map[int]session.UsePool, error) {
	if r.pools == nil {
		return map[int]session.UsePool{}, nil
	}
	out := make(map[int]session.UsePool, len(r.pools))
	for k, v := range r.pools {
		out[k] = v
	}
	return out, nil
}

func (r *fakeSpontaneousUsePoolRepo) Set(_ context.Context, _ int64, techLevel, usesRemaining, maxUses int) error {
	if r.pools == nil {
		r.pools = make(map[int]session.UsePool)
	}
	r.pools[techLevel] = session.UsePool{Remaining: usesRemaining, Max: maxUses}
	r.setCalls = append(r.setCalls, usePoolSetCall{level: techLevel, remaining: usesRemaining, max: maxUses})
	return nil
}

func (r *fakeSpontaneousUsePoolRepo) Decrement(_ context.Context, _ int64, _ int) error { return nil }
func (r *fakeSpontaneousUsePoolRepo) RestoreAll(_ context.Context, _ int64) error        { return nil }
func (r *fakeSpontaneousUsePoolRepo) DeleteAll(_ context.Context, _ int64) error         { return nil }

// TestAssignTechnologies_InitializesUsePools verifies that AssignTechnologies
// initializes SpontaneousUsePools when the job has UsesByLevel entries.
func TestAssignTechnologies_InitializesUsePools(t *testing.T) {
	ctx := context.Background()
	const charID int64 = 42

	job := &ruleset.Job{
		ID: "test-job",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: 0},
				UsesByLevel:  map[int]int{1: 3},
			},
		},
	}
	sess := &session.PlayerSession{}

	hwRepo := &fakeHardwiredRepo{}
	prepRepo := &fakePreparedRepo{}
	spontRepo := &fakeSpontaneousRepo{}
	innateRepo := &fakeInnateRepo{}
	poolRepo := &fakeSpontaneousUsePoolRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, charID, job, nil, nil, noPrompt,
		hwRepo, prepRepo, spontRepo, innateRepo, poolRepo)
	require.NoError(t, err)

	// Assert the session pool was set.
	assert.Equal(t, session.UsePool{Remaining: 3, Max: 3}, sess.SpontaneousUsePools[1])

	// Assert Set was called with the correct arguments.
	require.Len(t, poolRepo.setCalls, 1)
	call := poolRepo.setCalls[0]
	assert.Equal(t, 1, call.level)
	assert.Equal(t, 3, call.remaining)
	assert.Equal(t, 3, call.max)
}

// TestLevelUpTechnologies_AddsToUsePools verifies that LevelUpTechnologies
// additively increments existing pool entries.
func TestLevelUpTechnologies_AddsToUsePools(t *testing.T) {
	ctx := context.Background()
	const charID int64 = 99

	sess := &session.PlayerSession{
		SpontaneousUsePools: map[int]session.UsePool{
			1: {Remaining: 2, Max: 3},
		},
	}
	levelGrants := &ruleset.TechnologyGrants{
		Spontaneous: &ruleset.SpontaneousGrants{
			KnownByLevel: map[int]int{},
			UsesByLevel:  map[int]int{1: 2},
		},
	}

	hwRepo := &fakeHardwiredRepo{}
	prepRepo := &fakePreparedRepo{}
	spontRepo := &fakeSpontaneousRepo{}
	innateRepo := &fakeInnateRepo{}
	poolRepo := &fakeSpontaneousUsePoolRepo{
		pools: map[int]session.UsePool{1: {Remaining: 2, Max: 3}},
	}

	err := gameserver.LevelUpTechnologies(ctx, sess, charID, levelGrants, nil, noPrompt,
		hwRepo, prepRepo, spontRepo, innateRepo, poolRepo)
	require.NoError(t, err)

	assert.Equal(t, session.UsePool{Remaining: 4, Max: 5}, sess.SpontaneousUsePools[1])

	require.Len(t, poolRepo.setCalls, 1)
	call := poolRepo.setCalls[0]
	assert.Equal(t, 1, call.level)
	assert.Equal(t, 4, call.remaining)
	assert.Equal(t, 5, call.max)
}

// TestLoadTechnologies_LoadsUsePools verifies that LoadTechnologies populates
// SpontaneousUsePools from the repo.
func TestLoadTechnologies_LoadsUsePools(t *testing.T) {
	ctx := context.Background()
	const charID int64 = 7

	sess := &session.PlayerSession{}

	hwRepo := &fakeHardwiredRepo{}
	prepRepo := &fakePreparedRepo{}
	spontRepo := &fakeSpontaneousRepo{}
	innateRepo := &fakeInnateRepo{}
	poolRepo := &fakeSpontaneousUsePoolRepo{
		pools: map[int]session.UsePool{
			2: {Remaining: 1, Max: 3},
		},
	}

	err := gameserver.LoadTechnologies(ctx, sess, charID, hwRepo, prepRepo, spontRepo, innateRepo, poolRepo)
	require.NoError(t, err)

	assert.Equal(t, session.UsePool{Remaining: 1, Max: 3}, sess.SpontaneousUsePools[2])
}
