package gameserver

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// --- local fakes (package gameserver, no conflict with gameserver_test fakes) ---

type sslFakeSpontaneousRepo struct {
	techs map[int][]string
}

func (r *sslFakeSpontaneousRepo) GetAll(_ context.Context, _ int64) (map[int][]string, error) {
	if r.techs == nil {
		return make(map[int][]string), nil
	}
	return r.techs, nil
}
func (r *sslFakeSpontaneousRepo) Add(_ context.Context, _ int64, techID string, level int) error {
	if r.techs == nil {
		r.techs = make(map[int][]string)
	}
	r.techs[level] = append(r.techs[level], techID)
	return nil
}
func (r *sslFakeSpontaneousRepo) DeleteAll(_ context.Context, _ int64) error {
	r.techs = nil
	return nil
}

// sslFakeProgressRepo implements ProgressRepository (9 methods).
// Only GetPendingTechLevels and SetPendingTechLevels are exercised by handleSelectTech;
// the rest are no-op stubs to satisfy the interface.
type sslFakeProgressRepo struct{}

func (r *sslFakeProgressRepo) GetProgress(_ context.Context, _ int64) (level, experience, maxHP, pendingBoosts int, err error) {
	return 1, 0, 10, 0, nil
}
func (r *sslFakeProgressRepo) GetPendingSkillIncreases(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (r *sslFakeProgressRepo) IncrementPendingSkillIncreases(_ context.Context, _ int64, _ int) error {
	return nil
}
func (r *sslFakeProgressRepo) ConsumePendingBoost(_ context.Context, _ int64) error { return nil }
func (r *sslFakeProgressRepo) ConsumePendingSkillIncrease(_ context.Context, _ int64) error {
	return nil
}
func (r *sslFakeProgressRepo) IsSkillIncreasesInitialized(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (r *sslFakeProgressRepo) MarkSkillIncreasesInitialized(_ context.Context, _ int64) error {
	return nil
}
func (r *sslFakeProgressRepo) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
	return nil, nil
}
func (r *sslFakeProgressRepo) SetPendingTechLevels(_ context.Context, _ int64, _ []int) error {
	return nil
}

// spontaneousSelectionTestService builds a minimal GameServiceServer suitable for
// handleSelectTech tests. Injects a non-nil jobRegistry, spontaneousTechRepo, and
// progressRepo — the three dependencies that handleSelectTech requires to proceed past
// its early-exit guards.
func spontaneousSelectionTestService(t *testing.T) (*GameServiceServer, *session.Manager, *sslFakeSpontaneousRepo) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	// jobRegistry must be non-nil for handleSelectTech to proceed past the early-exit check.
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "influencer", Name: "Influencer"})
	svc.SetJobRegistry(reg)

	spontRepo := &sslFakeSpontaneousRepo{}
	svc.SetSpontaneousTechRepo(spontRepo)
	svc.SetProgressRepo(&sslFakeProgressRepo{})

	return svc, sessMgr, spontRepo
}

// pendingGrant builds a PendingTechGrants map with a single spontaneous grant at level 3
// containing a 3-tech pool at level 1 with 1 open slot.
// This guarantees pool(3) > open(1) so PartitionTechGrants always defers.
func pendingGrant() map[int]*ruleset.TechnologyGrants {
	return map[int]*ruleset.TechnologyGrants{
		3: {
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: 1},
				Pool: []ruleset.SpontaneousEntry{
					{ID: "mind_spike", Level: 1},
					{ID: "neural_static", Level: 1},
					{ID: "synaptic_surge", Level: 1},
				},
			},
		},
	}
}

// sayMsg builds a ClientMessage with a SayRequest, simulating a player typing a number.
func sayMsg(text string) *gamev1.ClientMessage {
	return &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Say{
			Say: &gamev1.SayRequest{Message: text},
		},
	}
}

// REQ-SSL1: selecttech resolves a deferred spontaneous grant when a valid choice is submitted.
func TestHandleSelectTech_SpontaneousGrant_ValidChoice_ResolvesGrant(t *testing.T) {
	svc, sessMgr, spontRepo := spontaneousSelectionTestService(t)

	uid := "player-ssl1"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", Class: "influencer",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingTechGrants = pendingGrant()

	// Option 2 = "neural_static" (pool order: mind_spike=1, neural_static=2, synaptic_surge=3).
	// techReg is nil so buildSpontaneousOptions returns raw IDs in pool order.
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{sayMsg("2")},
	}

	err = svc.handleSelectTech(uid, "req1", stream)
	require.NoError(t, err)

	// Grant cleared.
	assert.Empty(t, sess.PendingTechGrants, "PendingTechGrants must be empty after resolution")

	// Tech added to session.
	require.NotNil(t, sess.SpontaneousTechs)
	assert.Contains(t, sess.SpontaneousTechs[1], "neural_static",
		"neural_static must appear in SpontaneousTechs[1]")

	// Repo persisted the choice.
	assert.Contains(t, spontRepo.techs[1], "neural_static",
		"spontaneousTechRepo must have recorded neural_static at level 1")
}

// REQ-SSL2: selecttech sends "Invalid selection" when an out-of-range numeric choice is submitted.
// Known limitation: the pending grant is also cleared (silently lost) on invalid input.
func TestHandleSelectTech_SpontaneousGrant_InvalidChoice_SendsInvalidSelection(t *testing.T) {
	svc, sessMgr, _ := spontaneousSelectionTestService(t)

	uid := "player-ssl2"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", Class: "influencer",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingTechGrants = pendingGrant()

	// "99" is out of range (only 3 options).
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{sayMsg("99")},
	}

	err = svc.handleSelectTech(uid, "req2", stream)
	require.NoError(t, err)

	// "Invalid selection" must appear in at least one sent message.
	found := false
	for _, evt := range stream.sent {
		if msg := evt.GetMessage(); msg != nil && strings.Contains(msg.Content, "Invalid selection") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'Invalid selection' in stream.sent messages")

	// No valid pool tech must be assigned — implementation may append "" on invalid input,
	// so we assert that none of the known pool IDs appear in the result.
	assigned := sess.SpontaneousTechs[1]
	assert.NotContains(t, assigned, "mind_spike", "mind_spike must not be assigned after invalid selection")
	assert.NotContains(t, assigned, "neural_static", "neural_static must not be assigned after invalid selection")
	assert.NotContains(t, assigned, "synaptic_surge", "synaptic_surge must not be assigned after invalid selection")
}

// REQ-SSL3: The prompt sent by selecttech lists all three pool tech options.
func TestHandleSelectTech_SpontaneousGrant_PromptListsAllPoolOptions(t *testing.T) {
	svc, sessMgr, _ := spontaneousSelectionTestService(t)

	uid := "player-ssl3"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", Class: "influencer",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingTechGrants = pendingGrant()

	// Pre-queue a valid choice so the stream doesn't EOF before the prompt is sent.
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{sayMsg("1")},
	}

	err = svc.handleSelectTech(uid, "req3", stream)
	require.NoError(t, err)

	// Collect all message content.
	var allContent strings.Builder
	for _, evt := range stream.sent {
		if msg := evt.GetMessage(); msg != nil {
			allContent.WriteString(msg.Content)
			allContent.WriteString("\n")
		}
	}
	combined := allContent.String()

	assert.Contains(t, combined, "mind_spike", "prompt must list mind_spike")
	assert.Contains(t, combined, "neural_static", "prompt must list neural_static")
	assert.Contains(t, combined, "synaptic_surge", "prompt must list synaptic_surge")
}
