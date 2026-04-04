package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// hotbarCharSaver is a CharacterSaver test double that records SaveHotbar calls.
type hotbarCharSaver struct {
	fakeCharSaver // embed the rest_test.go stub for all other methods
	saved   map[int64][10]session.HotbarSlot
	loadErr error
}

func newHotbarCharSaver() *hotbarCharSaver {
	return &hotbarCharSaver{saved: make(map[int64][10]session.HotbarSlot)}
}

func (h *hotbarCharSaver) SaveHotbar(_ context.Context, characterID int64, slots [10]session.HotbarSlot) error {
	h.saved[characterID] = slots
	return nil
}

func (h *hotbarCharSaver) LoadHotbar(_ context.Context, _ int64) ([10]session.HotbarSlot, error) {
	return [10]session.HotbarSlot{}, h.loadErr
}

func testHotbarService(t *testing.T) (*GameServiceServer, *session.Manager, string) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	svc.SetCharSaver(newHotbarCharSaver())

	uid := "hotbar-test-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 42,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)
	return svc, sessMgr, uid
}

// REQ-HB-3: set valid slot 1 returns HotbarUpdateEvent and writes command slot.
func TestHandleHotbar_SetSlot1(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 1, Text: "look"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu, "set must return HotbarUpdateEvent")
	assert.Len(t, hu.Slots, 10)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, "look", sess.Hotbar[0].ActivationCommand())
}

// REQ-HB-3: set slot 10 (boundary) returns HotbarUpdateEvent.
func TestHandleHotbar_SetSlot10(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 10, Text: "status"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu, "set must return HotbarUpdateEvent")
	assert.Len(t, hu.Slots, 10)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, "status", sess.Hotbar[9].ActivationCommand())
}

// REQ-HB-3: set out-of-range slot (0) returns error message with no side effect.
func TestHandleHotbar_SetOutOfRangeLow(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 0, Text: "look"})
	require.NoError(t, err)
	assert.Equal(t, "Slot out of range (1-10).", evt.GetMessage().GetContent())
}

// REQ-HB-3: set out-of-range slot (11) returns error message with no side effect.
func TestHandleHotbar_SetOutOfRangeHigh(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 11, Text: "look"})
	require.NoError(t, err)
	assert.Equal(t, "Slot out of range (1-10).", evt.GetMessage().GetContent())

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, [10]session.HotbarSlot{}, sess.Hotbar)
}

// REQ-HB-4: clear valid slot returns HotbarUpdateEvent.
func TestHandleHotbar_ClearSlot(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Hotbar[2] = session.CommandSlot("heal")

	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "clear", Slot: 3})
	require.NoError(t, err)
	require.NotNil(t, evt)

	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu, "clear must return HotbarUpdateEvent")
	assert.True(t, sess.Hotbar[2].IsEmpty())
}

// REQ-HB-4: clear out-of-range slot returns error message.
func TestHandleHotbar_ClearOutOfRange(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "clear", Slot: 11})
	require.NoError(t, err)
	assert.Equal(t, "Slot out of range (1-10).", evt.GetMessage().GetContent())
}

// REQ-HB-5: show returns nil event.
func TestHandleHotbar_Show(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Hotbar[0] = session.CommandSlot("look")
	sess.Hotbar[9] = session.CommandSlot("status")

	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "show"})
	require.NoError(t, err)
	assert.Nil(t, evt)
}

// REQ-HB-TS-1: handleHotbar set returns HotbarUpdateEvent (not MessageEvent).
func TestHandleHotbar_SetSendsUpdateEvent(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 1, Text: "look"})
	require.NoError(t, err)
	_, isUpdate := evt.Payload.(*gamev1.ServerEvent_HotbarUpdate)
	assert.True(t, isUpdate, "handleHotbar set should return a HotbarUpdateEvent")
}

// REQ-HB-TS-1: SaveHotbar called with updated slots on set.
func TestHandleHotbar_PersistsOnSet(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	saver := newHotbarCharSaver()
	svc.SetCharSaver(saver)

	uid := "persist-test"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 42,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	_, err = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 5, Text: "attack goblin"})
	require.NoError(t, err)

	saved, ok := saver.saved[42]
	require.True(t, ok, "SaveHotbar must be called with characterID 42")
	assert.Equal(t, "attack goblin", saved[4].ActivationCommand())
}

// REQ-HB-TS-1: SaveHotbar called after clear.
func TestHandleHotbar_PersistsOnClear(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	saver := newHotbarCharSaver()
	svc.SetCharSaver(saver)

	uid := "clear-persist-test"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 55,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, _ := sessMgr.GetPlayer(uid)
	sess.Hotbar[2] = session.CommandSlot("heal")

	_, err = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "clear", Slot: 3})
	require.NoError(t, err)

	saved, ok := saver.saved[55]
	require.True(t, ok, "SaveHotbar must be called after clear")
	assert.True(t, saved[2].IsEmpty())
}

// REQ-HB-TS-2: kind+ref creates a typed feat slot and returns HotbarUpdateEvent.
func TestHandleHotbar_SetTypedFeatSlot(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 2, Kind: "feat", Ref: "power_strike"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"}, sess.Hotbar[1])
	assert.Equal(t, "use power_strike", sess.Hotbar[1].ActivationCommand())

	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)
	assert.Len(t, hu.Slots, 10)
	assert.Equal(t, "feat", hu.Slots[1].GetKind())
	assert.Equal(t, "power_strike", hu.Slots[1].GetRef())
}

// REQ-HB-TS-3: resolveHotbarSlotDisplay returns empty strings when registries are nil.
func TestResolveHotbarSlotDisplay_NilRegistriesReturnsEmpty(t *testing.T) {
	t.Parallel()
	svc := &GameServiceServer{} // nil registries
	for _, kind := range []string{
		session.HotbarSlotKindFeat,
		session.HotbarSlotKindTechnology,
		session.HotbarSlotKindThrowable,
		session.HotbarSlotKindConsumable,
	} {
		name, desc := svc.resolveHotbarSlotDisplay(session.HotbarSlot{Kind: kind, Ref: "some_id"})
		assert.Equal(t, "", name, "kind=%s", kind)
		assert.Equal(t, "", desc, "kind=%s", kind)
	}
}

// Property: set with valid slot 1–10 and non-empty text always writes to index slot-1.
func TestPropertyHandleHotbar_SetValidSlot(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		slot := rapid.Int32Range(1, 10).Draw(rt, "slot")
		text := rapid.StringOfN(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz ")), 1, 40, -1).Draw(rt, "text")

		svc, _, uid := testHotbarService(t)
		evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: slot, Text: text})
		require.NoError(t, err)
		require.NotNil(t, evt.GetHotbarUpdate(), "set must return HotbarUpdateEvent for non-empty text")

		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(t, ok)
		require.Equal(t, text, sess.Hotbar[slot-1].ActivationCommand())
	})
}
