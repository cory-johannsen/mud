package gameserver

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/world"
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

// fakeCharSaver records SaveState calls for test assertions.
type fakeCharSaver struct {
	saveStateCalls []saveStateCall
	returnErr      error
}

type saveStateCall struct {
	id        int64
	location  string
	currentHP int
}

// Implement the full CharacterSaver interface — only SaveState is under test;
// all other methods return zero values.
func (f *fakeCharSaver) GetByID(_ context.Context, _ int64) (*character.Character, error) {
	return nil, nil
}
func (f *fakeCharSaver) SaveState(_ context.Context, id int64, location string, currentHP int) error {
	f.saveStateCalls = append(f.saveStateCalls, saveStateCall{id, location, currentHP})
	return f.returnErr
}
func (f *fakeCharSaver) LoadWeaponPresets(_ context.Context, _ int64, _ *inventory.Registry) (*inventory.LoadoutSet, error) {
	return nil, nil
}
func (f *fakeCharSaver) SaveWeaponPresets(_ context.Context, _ int64, _ *inventory.LoadoutSet) error {
	return nil
}
func (f *fakeCharSaver) LoadEquipment(_ context.Context, _ int64) (*inventory.Equipment, error) {
	return nil, nil
}
func (f *fakeCharSaver) SaveEquipment(_ context.Context, _ int64, _ *inventory.Equipment) error {
	return nil
}
func (f *fakeCharSaver) LoadInventory(_ context.Context, _ int64) ([]inventory.InventoryItem, error) {
	return nil, nil
}
func (f *fakeCharSaver) SaveInventory(_ context.Context, _ int64, _ []inventory.InventoryItem) error {
	return nil
}
func (f *fakeCharSaver) HasReceivedStartingInventory(_ context.Context, _ int64) (bool, error) {
	return false, nil
}
func (f *fakeCharSaver) MarkStartingInventoryGranted(_ context.Context, _ int64) error { return nil }
func (f *fakeCharSaver) SaveAbilities(_ context.Context, _ int64, _ character.AbilityScores) error {
	return nil
}
func (f *fakeCharSaver) SaveProgress(_ context.Context, _ int64, _, _, _, _ int) error { return nil }
func (f *fakeCharSaver) SaveDefaultCombatAction(_ context.Context, _ int64, _ string) error {
	return nil
}
func (f *fakeCharSaver) SaveCurrency(_ context.Context, _ int64, _ int) error { return nil }
func (f *fakeCharSaver) LoadCurrency(_ context.Context, _ int64) (int, error) { return 0, nil }
func (f *fakeCharSaver) SaveGender(_ context.Context, _ int64, _ string) error { return nil }
func (f *fakeCharSaver) SaveHeroPoints(_ context.Context, _ int64, _ int) error { return nil }
func (f *fakeCharSaver) LoadHeroPoints(_ context.Context, _ int64) (int, error) { return 0, nil }
func (f *fakeCharSaver) SaveJobs(_ context.Context, _ int64, _ map[string]int, _ string) error {
	return nil
}
func (f *fakeCharSaver) SaveInstanceCharges(_ context.Context, _ int64, _, _ string, _ int, _ bool) error {
	return nil
}
func (f *fakeCharSaver) LoadJobs(_ context.Context, _ int64) (map[string]int, string, error) {
	return map[string]int{}, "", nil
}
func (f *fakeCharSaver) LoadFocusPoints(_ context.Context, _ int64) (int, error) { return 0, nil }
func (f *fakeCharSaver) SaveFocusPoints(_ context.Context, _ int64, _ int) error { return nil }
func (f *fakeCharSaver) SaveHotbar(_ context.Context, _ int64, _ [10]string) error { return nil }
func (f *fakeCharSaver) LoadHotbar(_ context.Context, _ int64) ([10]string, error) {
	return [10]string{}, nil
}

// REQ-LR1/LR2 (property): For any CurrentHP in [0, MaxHP], after rest,
// sess.CurrentHP == sess.MaxHP and SaveState called once with MaxHP.
func TestPropertyHandleRest_HPRestoredToMax(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxHP := rapid.IntRange(1, 50).Draw(rt, "maxHP")
		currentHP := rapid.IntRange(0, maxHP).Draw(rt, "currentHP")

		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		charSaver := &fakeCharSaver{}
		svc.SetCharSaver(charSaver)

		uid := fmt.Sprintf("lr-prop-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
		})
		if err != nil {
			rt.Skip()
		}
		sess, ok := sessMgr.GetPlayer(uid)
		if !ok {
			rt.Skip()
		}
		sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
		sess.CurrentHP = currentHP
		sess.MaxHP = maxHP

		stream := &fakeSessionStream{}
		_ = svc.handleRest(uid, "req", stream)

		assert.Equal(rt, maxHP, sess.CurrentHP, "CurrentHP must equal MaxHP after rest")
		require.Len(rt, charSaver.saveStateCalls, 1, "SaveState must be called exactly once")
		assert.Equal(rt, maxHP, charSaver.saveStateCalls[0].currentHP, "SaveState called with MaxHP")
	})
}

// REQ-LR4: Rest with nil charSaver succeeds; HP updated in memory.
func TestHandleRest_NilCharSaver_HPRestoredInMemory(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	// charSaver is nil by default in testMinimalService — do not set one.

	uid := "lr-nil-saver"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.CurrentHP = 3
	sess.MaxHP = 20

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req", stream))
	assert.Equal(t, 20, sess.CurrentHP)
}

// REQ-LR5: HP must NOT be modified when combat guard fires.
func TestHandleRest_InCombat_HPUnchanged(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	uid := "lr-combat-hp"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)
	sess.CurrentHP = 5
	sess.MaxHP = 20

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req", stream))
	assert.Equal(t, 5, sess.CurrentHP, "HP must not change when in combat")
	assert.Empty(t, charSaver.saveStateCalls, "SaveState must not be called when in combat")
}

// REQ-LR3: Success message contains "HP" and "prepared".
func TestHandleRest_SuccessMessage_ContainsHP(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	uid := "lr-msg-hp"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.CurrentHP = 5
	sess.MaxHP = 20

	// Give it a valid job so it reaches the final success message.
	job := &ruleset.Job{
		ID:   "test_job_msg",
		Name: "Test Job Msg",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "some_tech", Level: 1}},
			},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.SetJobRegistry(jobReg)
	sess.Class = "test_job_msg"

	prepRepo := &fakePreparedRepoRest{}
	svc.SetPreparedTechRepo(prepRepo)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req", stream))

	msg := lastMessage(stream)
	assert.Contains(t, msg, "HP", "success message must mention HP")
	assert.Contains(t, msg, "prepared", "success message must mention prepared technologies")
}

// REQ-LR3: Early-return message (no job) contains "HP".
func TestHandleRest_NoJob_EarlyReturnMessageContainsHP(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	// testMinimalService sets jobRegistry to nil — no job lookup will succeed.
	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	uid := "lr-nojob-hp"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.CurrentHP = 5
	sess.MaxHP = 20

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req", stream))

	msg := lastMessage(stream)
	assert.Contains(t, msg, "HP", "early-return message must mention HP")
}

// REQ-LF9: Completion message uses flavor RestMessage for the player's class.
func TestHandleRest_CompletionMessage_UsesFlavorRestMessage(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	uid := "lf9-flavor-msg"
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
	sess.Class = "nerd" // DominantTradition("nerd") == "technical"; FlavorFor("technical").RestMessage == "Field loadout configured."
	sess.CurrentHP = 5
	sess.MaxHP = 20

	job := &ruleset.Job{
		ID:   "nerd",
		Name: "Nerd",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "some_tech", Level: 1}},
			},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.SetJobRegistry(jobReg)

	prepRepo := &fakePreparedRepoRest{}
	svc.SetPreparedTechRepo(prepRepo)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req", stream))

	msg := lastMessage(stream)
	assert.Equal(t, "You finish your rest. HP restored to maximum. Field loadout configured.", msg)
}

// REQ-LR2 error path: If SaveState returns an error, handleRest returns that error.
func TestHandleRest_SaveStateError_ReturnsError(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	charSaver := &fakeCharSaver{returnErr: fmt.Errorf("db error")}
	svc.SetCharSaver(charSaver)

	uid := "lr-save-err"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.CurrentHP = 3
	sess.MaxHP = 20

	stream := &fakeSessionStream{}
	err = svc.handleRest(uid, "req", stream)
	require.Error(t, err, "handleRest must return error when SaveState fails")
}

// newRestSvc creates a GameServiceServer for handleRest routing tests.
// It uses the standard test world (room_a in zone "test", no DangerLevel).
// The returned sessMgr is the session manager for adding players.
func newRestSvc(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	logger := zaptest.NewLogger(t)
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
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr
}

// newRestSvcWithSafeRoom creates a GameServiceServer with a room whose DangerLevel is "safe".
func newRestSvcWithSafeRoom(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "safe_zone",
		Name:        "Safe Zone",
		Description: "A safe zone.",
		StartRoom:   "room_safe",
		DangerLevel: "safe",
		Rooms: map[string]*world.Room{
			"room_safe": {
				ID:          "room_safe",
				ZoneID:      "safe_zone",
				Title:       "Safe Room",
				Description: "A safe room.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
				DangerLevel: "safe",
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	logger := zaptest.NewLogger(t)
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
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr
}

// newRestSvcWithDangerousRoom creates a GameServiceServer with a room whose DangerLevel is "dangerous".
func newRestSvcWithDangerousRoom(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "danger_zone",
		Name:        "Danger Zone",
		Description: "A dangerous zone.",
		StartRoom:   "room_danger",
		DangerLevel: "dangerous",
		Rooms: map[string]*world.Room{
			"room_danger": {
				ID:          "room_danger",
				ZoneID:      "danger_zone",
				Title:       "Dangerous Room",
				Description: "A dangerous room.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	logger := zaptest.NewLogger(t)
	invReg := inventory.NewRegistry()
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, invReg, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr
}

// addRestPlayer adds a player in the given room with combat idle status.
func addRestPlayer(t *testing.T, sessMgr *session.Manager, uid, roomID string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   5,
		MaxHP:       20,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	return sess
}

// TestHandleRest_BlockedInCombat verifies that rest is blocked when the player is in combat (REQ-REST-1).
//
// Precondition: sess.Status == COMBAT_STATUS_IN_COMBAT.
// Postcondition: message contains "can't rest"; no state mutation.
func TestHandleRest_BlockedInCombat(t *testing.T) {
	svc, sessMgr, _ := newRestSvc(t)
	sess := addRestPlayer(t, sessMgr, "rest-combat", "room_a")
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-combat", "req", stream))
	assert.Contains(t, lastMessage(stream), "can't rest")
}

// TestHandleRest_SafeRoom_NoMotelNPC_Blocked verifies that rest is blocked in a safe room
// with no motel NPC present (REQ-REST-4).
//
// Precondition: room DangerLevel == "safe"; no NPC with RestCost > 0 in room.
// Postcondition: message mentions "motel".
func TestHandleRest_SafeRoom_NoMotelNPC_Blocked(t *testing.T) {
	svc, sessMgr, _ := newRestSvcWithSafeRoom(t)
	addRestPlayer(t, sessMgr, "rest-safe-no-motel", "room_safe")

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-safe-no-motel", "req", stream))
	assert.Contains(t, lastMessage(stream), "motel")
}

// TestHandleRest_NonSafeRoom_NoExploreMode_Blocked verifies that rest is blocked in a dangerous
// room when the player is not in exploration mode (REQ-REST-4).
//
// Precondition: room DangerLevel == "dangerous"; sess.ExploreMode == "".
// Postcondition: message mentions "exploration mode".
func TestHandleRest_NonSafeRoom_NoExploreMode_Blocked(t *testing.T) {
	svc, sessMgr, _ := newRestSvcWithDangerousRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-danger-no-explore", "room_danger")
	sess.ExploreMode = ""

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-danger-no-explore", "req", stream))
	assert.Contains(t, lastMessage(stream), "exploration mode")
}

// TestHandleRest_Camping_MissingSleepingBag_Blocked verifies that camping requires a sleeping_bag
// item (REQ-REST-10/12).
//
// Precondition: room DangerLevel == "dangerous"; ExploreMode != ""; backpack has no sleeping_bag.
// Postcondition: message mentions "sleeping_bag".
func TestHandleRest_Camping_MissingSleepingBag_Blocked(t *testing.T) {
	svc, sessMgr, _ := newRestSvcWithDangerousRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-camp-nosleep", "room_danger")
	sess.ExploreMode = session.ExploreModeCaseIt

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-camp-nosleep", "req", stream))
	msg := lastMessage(stream)
	assert.Contains(t, msg, "sleeping_bag", "error must name sleeping_bag")
}

// TestHandleRest_Camping_MissingFireMaterial_Blocked verifies that camping requires a fire_material
// tagged item (REQ-REST-11/12).
//
// Precondition: room DangerLevel == "dangerous"; ExploreMode != ""; backpack has sleeping_bag but no fire_material.
// Postcondition: message mentions "fire_material".
func TestHandleRest_Camping_MissingFireMaterial_Blocked(t *testing.T) {
	svc, sessMgr, _ := newRestSvcWithDangerousRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-camp-nofire", "room_danger")
	sess.ExploreMode = session.ExploreModeCaseIt

	// Add sleeping_bag to inventory registry and backpack.
	invReg := inventory.NewRegistry()
	sleepingBagDef := &inventory.ItemDef{
		ID:       "sleeping_bag",
		Name:     "Sleeping Bag",
		Kind:     inventory.KindJunk,
		Weight:   2.0,
		MaxStack: 1,
	}
	require.NoError(t, invReg.RegisterItem(sleepingBagDef))
	svc.invRegistry = invReg
	_, err := sess.Backpack.Add("sleeping_bag", 1, invReg)
	require.NoError(t, err)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-camp-nofire", "req", stream))
	msg := lastMessage(stream)
	assert.Contains(t, msg, "fire_material", "error must name fire_material")
}

// TestHandleRest_Camping_Start_SetsSession verifies that valid camping gear starts a camping rest
// (REQ-REST-13).
//
// Precondition: room DangerLevel == "dangerous"; ExploreMode != ""; backpack has sleeping_bag + fire_material item.
// Postcondition: sess.CampingActive == true; sess.CampingDuration == 5min (no camping_gear bonus).
func TestHandleRest_Camping_Start_SetsSession(t *testing.T) {
	svc, sessMgr, _ := newRestSvcWithDangerousRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-camp-start", "room_danger")
	sess.ExploreMode = session.ExploreModeCaseIt

	invReg := inventory.NewRegistry()
	require.NoError(t, invReg.RegisterItem(&inventory.ItemDef{
		ID: "sleeping_bag", Name: "Sleeping Bag", Kind: inventory.KindJunk, Weight: 2.0, MaxStack: 1,
	}))
	require.NoError(t, invReg.RegisterItem(&inventory.ItemDef{
		ID: "torch", Name: "Torch", Kind: inventory.KindJunk, Weight: 0.5, MaxStack: 5,
		Tags: []string{"fire_material"},
	}))
	svc.invRegistry = invReg
	_, err := sess.Backpack.Add("sleeping_bag", 1, invReg)
	require.NoError(t, err)
	_, err = sess.Backpack.Add("torch", 1, invReg)
	require.NoError(t, err)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-camp-start", "req", stream))

	require.True(t, sess.CampingActive, "CampingActive must be true after starting camping")
	assert.Equal(t, 5*time.Minute, sess.CampingDuration, "CampingDuration must be 5 minutes with no camping_gear")
}

// TestHandleRest_Camping_DurationReducedByCampingGear verifies that camping_gear items reduce
// the camping duration by 30s each (REQ-REST-14).
//
// Precondition: 4 camping_gear items in backpack; sleeping_bag + fire_material present.
// Postcondition: sess.CampingDuration == 5min - 4*30s == 3min.
func TestHandleRest_Camping_DurationReducedByCampingGear(t *testing.T) {
	svc, sessMgr, _ := newRestSvcWithDangerousRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-camp-gear", "room_danger")
	sess.ExploreMode = session.ExploreModeCaseIt

	invReg := inventory.NewRegistry()
	require.NoError(t, invReg.RegisterItem(&inventory.ItemDef{
		ID: "sleeping_bag", Name: "Sleeping Bag", Kind: inventory.KindJunk, Weight: 2.0, MaxStack: 1,
	}))
	require.NoError(t, invReg.RegisterItem(&inventory.ItemDef{
		ID: "camp_kit", Name: "Camp Kit", Kind: inventory.KindJunk, Weight: 1.0, MaxStack: 10,
		Tags: []string{"fire_material", "camping_gear"},
	}))
	svc.invRegistry = invReg
	_, err := sess.Backpack.Add("sleeping_bag", 1, invReg)
	require.NoError(t, err)
	_, err = sess.Backpack.Add("camp_kit", 4, invReg)
	require.NoError(t, err)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-camp-gear", "req", stream))

	assert.True(t, sess.CampingActive)
	assert.Equal(t, 3*time.Minute, sess.CampingDuration, "4 camping_gear should reduce duration to 3min")
}

// TestHandleRest_Camping_MinimumDuration2Min verifies that camping duration cannot go below
// 2 minutes regardless of camping_gear count (REQ-REST-14).
//
// Precondition: 100 camping_gear items; sleeping_bag + fire_material present.
// Postcondition: sess.CampingDuration == 2min.
func TestHandleRest_Camping_MinimumDuration2Min(t *testing.T) {
	svc, sessMgr, _ := newRestSvcWithDangerousRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-camp-min", "room_danger")
	sess.ExploreMode = session.ExploreModeCaseIt

	invReg := inventory.NewRegistry()
	require.NoError(t, invReg.RegisterItem(&inventory.ItemDef{
		ID: "sleeping_bag", Name: "Sleeping Bag", Kind: inventory.KindJunk, Weight: 0.0, MaxStack: 1,
	}))
	require.NoError(t, invReg.RegisterItem(&inventory.ItemDef{
		ID: "mega_camp_kit", Name: "Mega Camp Kit", Kind: inventory.KindJunk, Weight: 0.0, MaxStack: 200,
		Tags: []string{"fire_material", "camping_gear"},
	}))
	svc.invRegistry = invReg
	// Use AddInstance to bypass backpack slot/weight limits for large quantities.
	require.NoError(t, sess.Backpack.AddInstance(&inventory.ItemInstance{InstanceID: "sb-1", ItemDefID: "sleeping_bag", Quantity: 1}))
	require.NoError(t, sess.Backpack.AddInstance(&inventory.ItemInstance{InstanceID: "mck-1", ItemDefID: "mega_camp_kit", Quantity: 100}))

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-camp-min", "req", stream))

	assert.True(t, sess.CampingActive)
	assert.Equal(t, 2*time.Minute, sess.CampingDuration, "CampingDuration must not go below 2 minutes")
}

// TestHandleRest_MotelRest_InsufficientCredits_Blocked verifies that motel rest is blocked when
// the player cannot afford it (REQ-REST-7).
//
// Precondition: room DangerLevel == "safe"; motel NPC with RestCost=50; sess.Currency=10.
// Postcondition: message mentions "50".
func TestHandleRest_MotelRest_InsufficientCredits_Blocked(t *testing.T) {
	svc, sessMgr, npcMgr := newRestSvcWithSafeRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-motel-broke", "room_safe")
	sess.Currency = 10

	// Spawn a motel NPC and set RestCost.
	tmpl := &npc.Template{
		ID:      "motel_clerk",
		Name:    "Motel Clerk",
		NPCType: "merchant",
		MaxHP:   10,
		Level:   1,
	}
	inst, err := npcMgr.Spawn(tmpl, "room_safe")
	require.NoError(t, err)
	inst.RestCost = 50

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-motel-broke", "req", stream))
	msg := lastMessage(stream)
	assert.Contains(t, msg, "50", "error must mention the cost (50)")
}

// TestHandleRest_MotelRest_IsInstant verifies that motel rest with sufficient credits
// immediately restores HP to maximum (REQ-REST-5).
//
// Precondition: room DangerLevel == "safe"; motel NPC with RestCost=20; sess.Currency=100.
// Postcondition: sess.CurrentHP == sess.MaxHP; sess.Currency == 80.
func TestHandleRest_MotelRest_IsInstant(t *testing.T) {
	svc, sessMgr, npcMgr := newRestSvcWithSafeRoom(t)
	sess := addRestPlayer(t, sessMgr, "rest-motel-ok", "room_safe")
	sess.Currency = 100

	tmpl := &npc.Template{
		ID:      "motel_clerk2",
		Name:    "Motel Clerk",
		NPCType: "merchant",
		MaxHP:   10,
		Level:   1,
	}
	inst, err := npcMgr.Spawn(tmpl, "room_safe")
	require.NoError(t, err)
	inst.RestCost = 20

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("rest-motel-ok", "req", stream))

	assert.Equal(t, 20, sess.MaxHP, "MaxHP sanity check")
	assert.Equal(t, 20, sess.CurrentHP, "CurrentHP must equal MaxHP after motel rest")
	assert.Equal(t, 80, sess.Currency, "Currency must be decremented by RestCost (20)")
}

// TestHandleRest_RepairFull_RestoresAllEquipmentDurability verifies REQ-EM-16:
// handleRest restores all equipped weapons and armor to MaxDurability.
func TestHandleRest_RepairFull_RestoresAllEquipmentDurability(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-rest-dur"
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

	// Set up a damaged weapon in the active preset.
	swordDef := &inventory.WeaponDef{
		ID: "sword", Name: "Sword",
		DamageDice: "1d6", DamageType: "slashing",
		ProficiencyCategory: "martial_melee",
		Rarity:              "street", // MaxDurability=40
	}
	preset := sess.LoadoutSet.ActivePreset()
	require.NoError(t, preset.EquipMainHand(swordDef))
	preset.MainHand.Durability = 10 // damaged

	// Set up damaged armor.
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "chest_armor",
		Name:       "Chest Armor",
		InstanceID: "inst-armor-1",
		Durability: 5,
		Rarity:     "street", // MaxDurability=40
	}

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req-dur", stream))

	// Weapon should be fully repaired.
	assert.Equal(t, 40, preset.MainHand.Durability,
		"weapon durability should be restored to MaxDurability (40) on rest")

	// Armor should be fully repaired.
	si := sess.Equipment.Armor[inventory.SlotTorso]
	require.NotNil(t, si)
	assert.Equal(t, 40, si.Durability,
		"armor durability should be restored to MaxDurability (40) on rest")
}

// TestHandleRest_MotelRest_SendsInteractiveChoicePrompt verifies REQ-BUG97-1:
// when a motel rest requires tech slot preparation and pool size > open slots,
// handleRest MUST send a sentinel-encoded FeatureChoicePrompt via the stream
// instead of auto-selecting.
//
// Precondition: safe room; motel NPC with RestCost=20; player.Currency=100;
// player.Class has prepared grants with 2 pool options for 1 slot (must prompt);
// stream has "1" pre-queued as the choice response.
// Postcondition: stream.sent contains at least one event whose MessageEvent content
// begins with "\x00choice\x00"; the tech slot is filled with one of the pool entries.
func TestHandleRest_MotelRest_SendsInteractiveChoicePrompt(t *testing.T) {
	zone := &world.Zone{
		ID:          "safe_zone_tech",
		Name:        "Safe Zone Tech",
		Description: "A safe zone.",
		StartRoom:   "room_safe_tech",
		DangerLevel: "safe",
		Rooms: map[string]*world.Room{
			"room_safe_tech": {
				ID:          "room_safe_tech",
				ZoneID:      "safe_zone_tech",
				Title:       "Safe Room",
				Description: "A safe room.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
				DangerLevel: "safe",
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()

	// Job with 2 pool options for 1 slot: pool size > slots → must prompt.
	job := &ruleset.Job{
		ID: "test_psion",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "tech_alpha", Level: 1},
					{ID: "tech_beta", Level: 1},
				},
			},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)

	techReg := technology.NewRegistry()
	techReg.Register(&technology.TechnologyDef{ID: "tech_alpha", Name: "Tech Alpha", Level: 1})
	techReg.Register(&technology.TechnologyDef{ID: "tech_beta", Name: "Tech Beta", Level: 1})

	prepRepo := &fakePreparedRepoRest{}

	logger := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, jobReg, nil, techReg,
		nil, prepRepo, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "rest-motel-tech",
		Username:    "rest-motel-tech",
		CharName:    "rest-motel-tech",
		CharacterID: 1,
		RoomID:      "room_safe_tech",
		CurrentHP:   5,
		MaxHP:       20,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, addErr)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.Class = "test_psion"
	sess.Currency = 100
	sess.Level = 1
	// Pre-populate PreparedTechs so RearrangePreparedTechs runs (non-empty check).
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {{TechID: "tech_alpha"}},
	}

	tmpl := &npc.Template{
		ID:      "motel_clerk_tech",
		Name:    "Motel Clerk",
		NPCType: "motel_keeper",
		MaxHP:   10,
		Level:   1,
	}
	inst, spawnErr := npcMgr.Spawn(tmpl, "room_safe_tech")
	require.NoError(t, spawnErr)
	inst.RestCost = 20

	// Pre-queue choice "1" (selects first pool option).
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{moveMsg("1")},
	}

	require.NoError(t, svc.handleRest("rest-motel-tech", "req", stream))

	const sentinel = "\x00choice\x00"
	var foundPrompt bool
	for _, evt := range stream.sent {
		if content := evt.GetMessage().GetContent(); strings.HasPrefix(content, sentinel) {
			foundPrompt = true
			break
		}
	}
	assert.True(t, foundPrompt,
		"handleRest motel path must send a sentinel-encoded choice prompt; got %d sent events", len(stream.sent))
}
