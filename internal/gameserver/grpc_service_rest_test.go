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

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
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
func (f *fakeCharSaver) LoadJobs(_ context.Context, _ int64) (map[string]int, string, error) {
	return map[string]int{}, "", nil
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
