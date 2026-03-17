package gameserver

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// fakeSessionStream is a minimal gamev1.GameService_SessionServer for testing handleRest.
// It records sent messages and serves pre-loaded messages on Recv.
type fakeSessionStream struct {
	sent []*gamev1.ServerEvent
	recv []*gamev1.ClientMessage
	pos  int
}

func (f *fakeSessionStream) Send(evt *gamev1.ServerEvent) error {
	f.sent = append(f.sent, evt)
	return nil
}

func (f *fakeSessionStream) Recv() (*gamev1.ClientMessage, error) {
	if f.pos >= len(f.recv) {
		return nil, io.EOF
	}
	msg := f.recv[f.pos]
	f.pos++
	return msg, nil
}

func (f *fakeSessionStream) Context() context.Context { return context.Background() }

func (f *fakeSessionStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeSessionStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeSessionStream) SetTrailer(metadata.MD)       {}
func (f *fakeSessionStream) SendMsg(m interface{}) error  { return nil }
func (f *fakeSessionStream) RecvMsg(m interface{}) error  { return nil }

// lastMessage returns the content from the last sent ServerEvent MessageEvent.
func lastMessage(stream *fakeSessionStream) string {
	for i := len(stream.sent) - 1; i >= 0; i-- {
		evt := stream.sent[i]
		if msg := evt.GetMessage(); msg != nil {
			return msg.GetContent()
		}
	}
	return ""
}

// fakePreparedRepoRest is a local PreparedTechRepo fake for package gameserver tests.
type fakePreparedRepoRest struct {
	slots map[int][]*session.PreparedSlot
}

func (r *fakePreparedRepoRest) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) {
	return r.slots, nil
}
func (r *fakePreparedRepoRest) Set(_ context.Context, _ int64, level, index int, techID string) error {
	if r.slots == nil {
		r.slots = make(map[int][]*session.PreparedSlot)
	}
	for len(r.slots[level]) <= index {
		r.slots[level] = append(r.slots[level], nil)
	}
	r.slots[level][index] = &session.PreparedSlot{TechID: techID}
	return nil
}
func (r *fakePreparedRepoRest) DeleteAll(_ context.Context, _ int64) error {
	r.slots = nil
	return nil
}
func (r *fakePreparedRepoRest) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
	if r.slots != nil {
		if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
			slots[index].Expended = expended
		}
	}
	return nil
}

// TestPropertyHandleRest_InCombat_NeverMutatesState verifies that handleRest never
// mutates session prepared tech state when the player is in combat, regardless of
// job/slot configuration (property, REQ-RAR6).
func TestPropertyHandleRest_InCombat_NeverMutatesState(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numSlots := rapid.IntRange(0, 3).Draw(rt, "numSlots")
		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		uid := fmt.Sprintf("player-prop-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:      uid,
			Username: uid,
			CharName: uid,
			RoomID:   "room_a",
			Role:     "player",
		})
		if err != nil {
			rt.Skip()
		}

		sess, ok := sessMgr.GetPlayer(uid)
		if !ok {
			rt.Skip()
		}
		sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)
		initial := make([]*session.PreparedSlot, numSlots)
		for i := range initial {
			initial[i] = &session.PreparedSlot{TechID: fmt.Sprintf("tech_%d", i)}
		}
		sess.PreparedTechs = map[int][]*session.PreparedSlot{1: initial}

		prepRepo := &fakePreparedRepoRest{
			slots: map[int][]*session.PreparedSlot{1: initial},
		}
		svc.SetPreparedTechRepo(prepRepo)

		stream := &fakeSessionStream{}
		_ = svc.handleRest(uid, "req", stream)

		// State must be unchanged: repo slots still set (DeleteAll not called).
		if numSlots > 0 {
			if prepRepo.slots == nil {
				rt.Fatalf("repo slots were deleted while in combat")
			}
			for i, slot := range prepRepo.slots[1] {
				if slot.TechID != fmt.Sprintf("tech_%d", i) {
					rt.Fatalf("slot %d mutated while in combat: got %q", i, slot.TechID)
				}
			}
		}
		// Exactly one message sent: the combat guard message.
		if len(stream.sent) != 1 {
			rt.Fatalf("expected 1 message sent, got %d", len(stream.sent))
		}
	})
}

// TestHandleRest_InCombat_Rejected verifies that handleRest rejects the request
// when the player is in combat (REQ-RAR6).
func TestHandleRest_InCombat_Rejected(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-rest-combat"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)

	prepRepo := &fakePreparedRepoRest{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "original_tech"}},
		},
	}
	svc.SetPreparedTechRepo(prepRepo)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req1", stream))

	msg := lastMessage(stream)
	assert.Contains(t, msg, "can't rest")

	// Repo must be unchanged.
	assert.Equal(t, "original_tech", prepRepo.slots[1][0].TechID)
}

// TestHandleRest_NotInCombat_Rearranges verifies that handleRest rearranges
// prepared tech slots when the player is not in combat (REQ-RAR7).
func TestHandleRest_NotInCombat_Rearranges(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-rest-idle"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   "room_a",
		Role:     "player",
		Level:    1,
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.Class = "test_job"
	// Prepopulate one slot at level 1 so RearrangePreparedTechs does not no-op.
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {{TechID: "old_tech"}},
	}

	// Build job with one pool entry at level 1.
	job := &ruleset.Job{
		ID:   "test_job",
		Name: "Test Job",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "new_tech", Level: 1},
				},
			},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.SetJobRegistry(jobReg)

	prepRepo := &fakePreparedRepoRest{}
	svc.SetPreparedTechRepo(prepRepo)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req2", stream))

	msg := lastMessage(stream)
	assert.Contains(t, msg, "prepared")

	// The session's prepared slot at level 1 should now be "new_tech".
	require.NotNil(t, sess.PreparedTechs[1])
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "new_tech", sess.PreparedTechs[1][0].TechID)
}
