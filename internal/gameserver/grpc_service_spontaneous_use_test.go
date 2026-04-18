package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// fakeSpontaneousUsePoolRepo is an in-memory stub implementing SpontaneousUsePoolRepo.
type fakeSpontaneousUsePoolRepo struct {
	pools           map[int]session.UsePool
	restoreAllCalled bool
}

func newFakeSpontaneousUsePoolRepo(pools map[int]session.UsePool) *fakeSpontaneousUsePoolRepo {
	return &fakeSpontaneousUsePoolRepo{pools: pools}
}

func (r *fakeSpontaneousUsePoolRepo) GetAll(_ context.Context, _ int64) (map[int]session.UsePool, error) {
	result := make(map[int]session.UsePool, len(r.pools))
	for k, v := range r.pools {
		result[k] = v
	}
	return result, nil
}

func (r *fakeSpontaneousUsePoolRepo) Set(_ context.Context, _ int64, techLevel, usesRemaining, maxUses int) error {
	r.pools[techLevel] = session.UsePool{Remaining: usesRemaining, Max: maxUses}
	return nil
}

func (r *fakeSpontaneousUsePoolRepo) Decrement(_ context.Context, _ int64, techLevel int) error {
	p := r.pools[techLevel]
	if p.Remaining > 0 {
		p.Remaining--
	}
	r.pools[techLevel] = p
	return nil
}

func (r *fakeSpontaneousUsePoolRepo) RestoreAll(_ context.Context, _ int64) error {
	r.restoreAllCalled = true
	for k, v := range r.pools {
		v.Remaining = v.Max
		r.pools[k] = v
	}
	return nil
}

func (r *fakeSpontaneousUsePoolRepo) RestorePartial(_ context.Context, _ int64, fraction float64) error {
	for k, v := range r.pools {
		gain := int(float64(v.Max-v.Remaining) * fraction)
		v.Remaining += gain
		if v.Remaining > v.Max {
			v.Remaining = v.Max
		}
		r.pools[k] = v
	}
	return nil
}

func (r *fakeSpontaneousUsePoolRepo) DeleteAll(_ context.Context, _ int64) error {
	r.pools = make(map[int]session.UsePool)
	return nil
}

// newSpontaneousSvc creates a minimal GameServiceServer for handleUse spontaneous tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr.
func newSpontaneousSvc(t *testing.T, repo SpontaneousUsePoolRepo) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		repo,
		nil,
		nil, nil,
	)
	return svc, sessMgr
}

// addSpontaneousPlayer adds a player session with the given spontaneous tech configuration.
//
// Precondition: sessMgr must be non-nil; uid must be non-empty.
// Postcondition: Returns the created PlayerSession with KnownTechs and SpontaneousUsePools set.
func addSpontaneousPlayer(
	t *testing.T,
	sessMgr *session.Manager,
	uid string,
	spontTechs map[int][]string,
	spontPools map[int]session.UsePool,
) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "Tester",
		CharName:    "Tester",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.KnownTechs = spontTechs
	sess.SpontaneousUsePools = spontPools
	return sess
}

// TestHandleUse_SpontaneousActivation_REQ_SUC1 verifies that activating a known spontaneous
// tech with uses remaining decrements the pool and returns the expected confirmation message.
//
// Precondition: session has KnownTechs = {1: ["mind_spike"]}, SpontaneousUsePools = {1: UsePool{Remaining:2, Max:3}}.
// Postcondition: message = "You activate mind_spike. (1 uses remaining at level 1.)"; pool.Remaining == 1.
func TestHandleUse_SpontaneousActivation_REQ_SUC1(t *testing.T) {
	pools := map[int]session.UsePool{1: {Remaining: 2, Max: 3}}
	svc, sessMgr := newSpontaneousSvc(t, newFakeSpontaneousUsePoolRepo(pools))

	sess := addSpontaneousPlayer(t, sessMgr, "u_spont_1",
		map[int][]string{1: {"mind_spike"}},
		map[int]session.UsePool{1: {Remaining: 2, Max: 3}},
	)

	evt, err := svc.handleUse("u_spont_1", "mind_spike", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Equal(t, "You activate mind_spike. (1 uses remaining at level 1.)", msg)
	assert.Equal(t, 1, sess.SpontaneousUsePools[1].Remaining)
}

// TestHandleUse_SpontaneousNoUsesRemaining_REQ_SUC2 verifies that attempting to activate a
// spontaneous tech when the pool is exhausted returns the appropriate error message.
//
// Precondition: session has KnownTechs = {1: ["mind_spike"]}, SpontaneousUsePools = {1: UsePool{Remaining:0, Max:3}}.
// Postcondition: message = "No level 1 uses remaining."
func TestHandleUse_SpontaneousNoUsesRemaining_REQ_SUC2(t *testing.T) {
	svc, sessMgr := newSpontaneousSvc(t, newFakeSpontaneousUsePoolRepo(map[int]session.UsePool{}))

	addSpontaneousPlayer(t, sessMgr, "u_spont_2",
		map[int][]string{1: {"mind_spike"}},
		map[int]session.UsePool{1: {Remaining: 0, Max: 3}},
	)

	evt, err := svc.handleUse("u_spont_2", "mind_spike", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Equal(t, "No level 1 uses remaining.", msg)
}

// TestHandleUse_SpontaneousUnknownTech_REQ_SUC3 verifies that activating a tech not in
// the character's spontaneous tech list returns a "You don't know X." message.
//
// Precondition: session has KnownTechs = {1: ["mind_spike"]}.
// Postcondition: message contains "You don't know unknown_tech."
func TestHandleUse_SpontaneousUnknownTech_REQ_SUC3(t *testing.T) {
	svc, sessMgr := newSpontaneousSvc(t, newFakeSpontaneousUsePoolRepo(map[int]session.UsePool{}))

	addSpontaneousPlayer(t, sessMgr, "u_spont_3",
		map[int][]string{1: {"mind_spike"}},
		map[int]session.UsePool{1: {Remaining: 2, Max: 3}},
	)

	evt, err := svc.handleUse("u_spont_3", "unknown_tech", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Equal(t, "You don't know unknown_tech.", msg)
}

// TestHandleUse_ListMode_IncludesSpontaneous_REQ_SUC4 verifies that listing available
// abilities (abilityID == "") includes spontaneous tech entries with use counts.
//
// Precondition: session has KnownTechs = {1: ["mind_spike"]}, SpontaneousUsePools = {1: UsePool{Remaining:2, Max:3}}.
// Postcondition: UseResponse.Choices contains an entry with Description matching "mind_spike (2 uses remaining at level 1)".
func TestHandleUse_ListMode_IncludesSpontaneous_REQ_SUC4(t *testing.T) {
	svc, sessMgr := newSpontaneousSvc(t, newFakeSpontaneousUsePoolRepo(map[int]session.UsePool{}))

	addSpontaneousPlayer(t, sessMgr, "u_spont_4",
		map[int][]string{1: {"mind_spike"}},
		map[int]session.UsePool{1: {Remaining: 2, Max: 3}},
	)

	evt, err := svc.handleUse("u_spont_4", "", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	choices := evt.GetUseResponse().GetChoices()
	var found bool
	for _, c := range choices {
		if c.GetDescription() == "mind_spike (2 uses remaining at level 1)" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected spontaneous tech entry in choices, got: %v", choices)
}

// TestHandleRest_RestoresSpontaneousPools_REQ_SUC5 verifies that handleRest restores all
// spontaneous use pools to their Max values and updates the session.
//
// Precondition: session has SpontaneousUsePools = {1: UsePool{Remaining:0, Max:3}}.
// Postcondition: RestoreAll is called; sess.SpontaneousUsePools[1].Remaining == 3.
func TestHandleRest_RestoresSpontaneousPools_REQ_SUC5(t *testing.T) {
	repoPools := map[int]session.UsePool{1: {Remaining: 0, Max: 3}}
	repo := newFakeSpontaneousUsePoolRepo(repoPools)
	svc, sessMgr := newSpontaneousSvc(t, repo)

	// Set up a job registry so handleRest can pass the job lookup and reach pool restoration.
	job := &ruleset.Job{ID: "influencer", Name: "Influencer"}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.jobRegistry = jobReg

	sess := addSpontaneousPlayer(t, sessMgr, "u_rest_1",
		map[int][]string{},
		map[int]session.UsePool{1: {Remaining: 0, Max: 3}},
	)
	sess.Class = "influencer"

	stream := &fakeSessionStream{}
	err := svc.handleRest(sess.UID, "req-rest-1", stream)
	require.NoError(t, err)

	// RestoreAll must have been explicitly called.
	assert.True(t, repo.restoreAllCalled, "expected RestoreAll to be called")
	// RestoreAll should have been called, so repo pools are restored.
	assert.Equal(t, 3, repo.pools[1].Remaining)
	// Session should reflect the restored pools.
	assert.Equal(t, 3, sess.SpontaneousUsePools[1].Remaining)
}

// TestHandleUse_SpontaneousProperty_REQ_SUC7 is a property-based test verifying that
// activating a spontaneous tech N times succeeds and the N+1th activation fails.
//
// Precondition: N drawn from [1, 5]; SpontaneousUsePools = {1: UsePool{Remaining:N, Max:N}}.
// Postcondition: first N activations return success; N+1th returns "No level 1 uses remaining."
func TestHandleUse_SpontaneousProperty_REQ_SUC7(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		uid := rapid.StringMatching(`u_prop_[a-z]{4}`).Draw(rt, "uid")

		pools := map[int]session.UsePool{1: {Remaining: n, Max: n}}
		svc, sessMgr := newSpontaneousSvc(t, newFakeSpontaneousUsePoolRepo(pools))

		sess := addSpontaneousPlayer(t, sessMgr, uid,
			map[int][]string{1: {"mind_spike"}},
			map[int]session.UsePool{1: {Remaining: n, Max: n}},
		)

		for i := 0; i < n; i++ {
			evt, err := svc.handleUse(uid, "mind_spike", "", 0, 0)
			require.NoError(rt, err)
			require.NotNil(rt, evt)
			msg := evt.GetMessage().GetContent()
			assert.Contains(rt, msg, "You activate mind_spike.")
		}

		assert.Equal(rt, 0, sess.SpontaneousUsePools[1].Remaining)

		evt, err := svc.handleUse(uid, "mind_spike", "", 0, 0)
		require.NoError(rt, err)
		require.NotNil(rt, evt)
		assert.Equal(rt, "No level 1 uses remaining.", evt.GetMessage().GetContent())
	})
}
