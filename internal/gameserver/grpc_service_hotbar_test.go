package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
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

// REQ-HB-INV-1: handleInventory sets Throwable=true for items tagged "throwable".
//
// Precondition: invRegistry has a throwable item; player backpack contains one instance.
// Postcondition: InventoryView contains the item with Throwable=true; non-throwable items have Throwable=false.
func TestHandleInventory_ThrowableFlag(t *testing.T) {
	t.Parallel()
	sessMgr := session.NewManager()
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID:       "grenade",
		Name:     "Frag Grenade",
		Kind:     "consumable",
		Tags:     []string{"throwable"},
		MaxStack: 10,
	}))
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID:       "stimpak",
		Name:     "Stimpak",
		Kind:     "consumable",
		MaxStack: 10,
	}))

	svc := newTestGameServiceServer(
		nil, sessMgr, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, reg,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	require.NotNil(t, svc)

	uid := "inv-throwable-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 99,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)

	_, err = sess.Backpack.Add("grenade", 1, reg)
	require.NoError(t, err)
	_, err = sess.Backpack.Add("stimpak", 1, reg)
	require.NoError(t, err)

	evt, err := svc.handleInventory(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	view := evt.GetInventoryView()
	require.NotNil(t, view)
	require.Len(t, view.Items, 2)

	itemsByID := make(map[string]*gamev1.InventoryItem)
	for _, item := range view.Items {
		itemsByID[item.ItemDefId] = item
	}

	require.Contains(t, itemsByID, "grenade")
	assert.True(t, itemsByID["grenade"].Throwable, "grenade must have Throwable=true")

	require.Contains(t, itemsByID, "stimpak")
	assert.False(t, itemsByID["stimpak"].Throwable, "stimpak must have Throwable=false")
}

// REQ-HB-UC-1: hotbarUpdateEvent populates UsesRemaining and MaxUses for a feat slot
// when the feat has PreparedUses > 0 and session.ActiveFeatUses is set.
func TestHotbarUpdateEvent_FeatSlot_PopulatesUseCounts(t *testing.T) {
	t.Parallel()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	const featID = "test_feat_limited"
	feat := &ruleset.Feat{
		ID:                featID,
		Name:              "Test Limited Feat",
		Description:       "A feat with limited uses.",
		Active:            true,
		PreparedUses:      2,
		RechargeCondition: "Recharges on long rest",
	}
	svc.featRegistry = ruleset.NewFeatRegistry([]*ruleset.Feat{feat})

	uid := "feat-use-count-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 10,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Hotbar[0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}
	sess.ActiveFeatUses = map[string]int{featID: 1}

	evt := svc.hotbarUpdateEvent(sess)
	require.NotNil(t, evt)
	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)
	require.Len(t, hu.Slots, 10)

	slot0 := hu.Slots[0]
	assert.Equal(t, int32(1), slot0.GetUsesRemaining(), "UsesRemaining must reflect session.ActiveFeatUses")
	assert.Equal(t, int32(2), slot0.GetMaxUses(), "MaxUses must equal feat.PreparedUses")
	assert.Equal(t, "Recharges on long rest", slot0.GetRechargeCondition())
}

// REQ-HB-UC-2: hotbarUpdateEvent reports zero MaxUses for an unlimited feat (PreparedUses==0).
func TestHotbarUpdateEvent_UnlimitedFeat_ZeroMaxUses(t *testing.T) {
	t.Parallel()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	const featID = "test_feat_unlimited"
	feat := &ruleset.Feat{
		ID:           featID,
		Name:         "Test Unlimited Feat",
		Description:  "A feat with no use limit.",
		Active:       true,
		PreparedUses: 0,
	}
	svc.featRegistry = ruleset.NewFeatRegistry([]*ruleset.Feat{feat})

	uid := "feat-unlimited-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 11,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Hotbar[0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}

	evt := svc.hotbarUpdateEvent(sess)
	require.NotNil(t, evt)
	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)

	slot0 := hu.Slots[0]
	assert.Equal(t, int32(0), slot0.GetMaxUses(), "MaxUses must be 0 for unlimited feat")
	assert.Equal(t, int32(0), slot0.GetUsesRemaining(), "UsesRemaining must be 0 for unlimited feat")
}

// REQ-HB-UC-3: hotbarUpdateEvent reports zero use counts for a command slot.
func TestHotbarUpdateEvent_CommandSlot_ZeroUseCounts(t *testing.T) {
	t.Parallel()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "cmd-slot-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 12,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Hotbar[3] = session.CommandSlot("look")

	evt := svc.hotbarUpdateEvent(sess)
	require.NotNil(t, evt)
	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)

	slot3 := hu.Slots[3]
	assert.Equal(t, int32(0), slot3.GetMaxUses(), "MaxUses must be 0 for command slot")
	assert.Equal(t, int32(0), slot3.GetUsesRemaining(), "UsesRemaining must be 0 for command slot")
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
