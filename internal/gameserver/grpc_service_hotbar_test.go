package gameserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// hotbarCharSaver is a CharacterSaver test double that records SaveHotbars calls.
type hotbarCharSaver struct {
	fakeCharSaver // embed the rest_test.go stub for all other methods
	saved      map[int64][][10]session.HotbarSlot
	savedIdx   map[int64]int
	loadErr    error
}

func newHotbarCharSaver() *hotbarCharSaver {
	return &hotbarCharSaver{
		saved:    make(map[int64][][10]session.HotbarSlot),
		savedIdx: make(map[int64]int),
	}
}

func (h *hotbarCharSaver) SaveHotbars(_ context.Context, characterID int64, bars [][10]session.HotbarSlot, activeIdx int) error {
	h.saved[characterID] = bars
	h.savedIdx[characterID] = activeIdx
	return nil
}

func (h *hotbarCharSaver) LoadHotbars(_ context.Context, _ int64) ([][10]session.HotbarSlot, int, error) {
	return [][10]session.HotbarSlot{{}}, 0, h.loadErr
}

// newTestHotbarServer returns a configured GameServiceServer, the player session pointer,
// and the uid string. maxHotbars sets the server's multi-bar limit.
//
// Precondition: t is non-nil; maxHotbars >= 1.
// Postcondition: Server has one player with uid "hotbar-test-uid" and one empty hotbar.
func newTestHotbarServer(t testing.TB, maxHotbars int) (*GameServiceServer, *session.PlayerSession, string) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t.(*testing.T), sessMgr)
	svc.SetCharSaver(newHotbarCharSaver())
	svc.maxHotbars = maxHotbars

	uid := "hotbar-test-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 42,
		RoomID:      "room_a",
		Role:        "player",
	})
	if err != nil {
		t.(*testing.T).Fatalf("AddPlayer: %v", err)
	}
	sess, ok := sessMgr.GetPlayer(uid)
	if !ok {
		t.(*testing.T).Fatal("GetPlayer returned false")
	}
	return svc, sess, uid
}

func testHotbarService(t *testing.T) (*GameServiceServer, *session.Manager, string) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	svc.SetCharSaver(newHotbarCharSaver())
	svc.maxHotbars = 4

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

// extractMessage extracts the text content from a ServerEvent MessageEvent payload.
//
// Precondition: ev may be nil.
// Postcondition: Returns "" for nil events or events without a message payload.
func extractMessage(ev *gamev1.ServerEvent) string {
	if ev == nil {
		return ""
	}
	if m := ev.GetMessage(); m != nil {
		return m.GetContent()
	}
	return ""
}

// REQ-HB-3: set valid slot 1 returns confirmation MessageEvent and writes command slot.
func TestHandleHotbar_SetSlot1(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 1, Text: "look"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.Equal(t, "Slot 1 set.", evt.GetMessage().GetContent(), "set must return confirmation MessageEvent")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, "look", sess.Hotbars[0][0].ActivationCommand())
}

// REQ-HB-3: set slot 10 (boundary) returns confirmation MessageEvent.
func TestHandleHotbar_SetSlot10(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 10, Text: "status"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.Equal(t, "Slot 10 set.", evt.GetMessage().GetContent(), "set must return confirmation MessageEvent")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, "status", sess.Hotbars[0][9].ActivationCommand())
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
	assert.Equal(t, [10]session.HotbarSlot{}, sess.Hotbars[0])
}

// REQ-HB-4: clear valid slot returns confirmation MessageEvent.
func TestHandleHotbar_ClearSlot(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Hotbars[0][2] = session.CommandSlot("heal")

	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "clear", Slot: 3})
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.Equal(t, "Slot 3 cleared.", evt.GetMessage().GetContent(), "clear must return confirmation MessageEvent")
	assert.True(t, sess.Hotbars[0][2].IsEmpty())
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
	sess.Hotbars[0][0] = session.CommandSlot("look")
	sess.Hotbars[0][9] = session.CommandSlot("status")

	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "show"})
	require.NoError(t, err)
	assert.Nil(t, evt)
}

// REQ-HB-TS-1: handleHotbar set returns confirmation MessageEvent; HotbarUpdateEvent is pushed via entity.
func TestHandleHotbar_SetSendsUpdateEvent(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 1, Text: "look"})
	require.NoError(t, err)
	_, isMessage := evt.Payload.(*gamev1.ServerEvent_Message)
	assert.True(t, isMessage, "handleHotbar set should return a MessageEvent (HotbarUpdateEvent is pushed via entity)")
}

// REQ-HB-TS-1: SaveHotbars called with updated slots on set.
func TestHandleHotbar_PersistsOnSet(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	saver := newHotbarCharSaver()
	svc.SetCharSaver(saver)
	svc.maxHotbars = 4

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
	require.True(t, ok, "SaveHotbars must be called with characterID 42")
	assert.Equal(t, "attack goblin", saved[0][4].ActivationCommand())
}

// REQ-HB-TS-1: SaveHotbars called after clear.
func TestHandleHotbar_PersistsOnClear(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	saver := newHotbarCharSaver()
	svc.SetCharSaver(saver)
	svc.maxHotbars = 4

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
	sess.Hotbars[0][2] = session.CommandSlot("heal")

	_, err = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "clear", Slot: 3})
	require.NoError(t, err)

	saved, ok := saver.saved[55]
	require.True(t, ok, "SaveHotbars must be called after clear")
	assert.True(t, saved[0][2].IsEmpty())
}

// REQ-HB-TS-2: kind+ref creates a typed feat slot and returns confirmation MessageEvent; HotbarUpdateEvent is pushed via entity.
func TestHandleHotbar_SetTypedFeatSlot(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 2, Kind: "feat", Ref: "power_strike"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"}, sess.Hotbars[0][1])
	assert.Equal(t, "use power_strike", sess.Hotbars[0][1].ActivationCommand())

	assert.Equal(t, "Slot 2 set.", evt.GetMessage().GetContent(), "set must return confirmation MessageEvent")
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
	sess.Hotbars[0][0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}
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
	sess.Hotbars[0][0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}

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
	sess.Hotbars[0][3] = session.CommandSlot("look")

	evt := svc.hotbarUpdateEvent(sess)
	require.NotNil(t, evt)
	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)

	slot3 := hu.Slots[3]
	assert.Equal(t, int32(0), slot3.GetMaxUses(), "MaxUses must be 0 for command slot")
	assert.Equal(t, int32(0), slot3.GetUsesRemaining(), "UsesRemaining must be 0 for command slot")
}

// REQ-HB-UC-4: hotbarUpdateEvent populates UsesRemaining, MaxUses, and RechargeCondition
// for an innate technology slot when the session has InnateTechs populated.
func TestHotbarUpdateEvent_InnateTechSlot_PopulatesUseCounts(t *testing.T) {
	t.Parallel()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	const techID = "gunchete_neural_dart"
	reg := technology.NewRegistry()
	reg.Register(&technology.TechnologyDef{
		ID:                techID,
		Name:              "Neural Dart",
		Tradition:         technology.TraditionBioSynthetic,
		Level:             1,
		UsageType:         technology.UsageSpontaneous,
		Range:             technology.RangeSelf,
		Targets:           technology.TargetsSingle,
		Duration:          "instant",
		Resolution:        "none",
		RechargeCondition: "Recharges on rest",
	})
	svc.SetTechRegistry(reg)

	uid := "innate-tech-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 20,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Hotbars[0][0] = session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: techID}
	sess.InnateTechs = map[string]*session.InnateSlot{
		techID: {MaxUses: 3, UsesRemaining: 1},
	}

	evt := svc.hotbarUpdateEvent(sess)
	require.NotNil(t, evt)
	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)
	require.Len(t, hu.Slots, 10)

	slot0 := hu.Slots[0]
	assert.Equal(t, int32(3), slot0.GetMaxUses(), "MaxUses must equal InnateSlot.MaxUses")
	assert.Equal(t, int32(1), slot0.GetUsesRemaining(), "UsesRemaining must equal InnateSlot.UsesRemaining")
	assert.Equal(t, "Recharges on rest", slot0.GetRechargeCondition())
}

// REQ-HB-UC-5: hotbarUpdateEvent counts non-expended prepared slots for a prepared technology slot.
func TestHotbarUpdateEvent_PreparedTechSlot_PopulatesUseCounts(t *testing.T) {
	t.Parallel()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	const techID = "arc_bolt"
	uid := "prepared-tech-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 21,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Hotbars[0][0] = session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: techID}
	// 3 total slots for techID: 2 not expended, 1 expended.
	sess.InnateTechs = nil
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {
			{TechID: techID, Expended: false},
			{TechID: techID, Expended: false},
			{TechID: techID, Expended: true},
		},
	}

	evt := svc.hotbarUpdateEvent(sess)
	require.NotNil(t, evt)
	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)
	require.Len(t, hu.Slots, 10)

	slot0 := hu.Slots[0]
	assert.Equal(t, int32(3), slot0.GetMaxUses(), "MaxUses must equal total prepared slots")
	assert.Equal(t, int32(2), slot0.GetUsesRemaining(), "UsesRemaining must equal non-expended prepared slots")
}

// REQ-HB-UC-6: hotbarUpdateEvent reflects the spontaneous use pool for a spontaneous technology slot.
func TestHotbarUpdateEvent_KnownTechSlot_PopulatesUseCounts(t *testing.T) {
	t.Parallel()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	const techID = "phase_shift"
	uid := "spontaneous-tech-uid"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tester",
		CharName:    "Tester",
		CharacterID: 22,
		RoomID:      "room_a",
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Hotbars[0][0] = session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: techID}
	sess.InnateTechs = nil
	// PreparedTechs must be non-nil but empty so the code enters the else-if branch
	// and falls through to the spontaneous pool lookup (total == 0 → spontaneous path).
	sess.PreparedTechs = map[int][]*session.PreparedSlot{}
	sess.KnownTechs = map[int][]string{
		1: {techID},
	}
	sess.SpontaneousUsePools = map[int]session.UsePool{
		1: {Max: 4, Remaining: 2},
	}

	evt := svc.hotbarUpdateEvent(sess)
	require.NotNil(t, evt)
	hu := evt.GetHotbarUpdate()
	require.NotNil(t, hu)
	require.Len(t, hu.Slots, 10)

	slot0 := hu.Slots[0]
	assert.Equal(t, int32(4), slot0.GetMaxUses(), "MaxUses must equal pool.Max")
	assert.Equal(t, int32(2), slot0.GetUsesRemaining(), "UsesRemaining must equal pool.Remaining")
}

// REQ-HB-UC-7 (property): command and consumable slots always report zero MaxUses and UsesRemaining.
func TestPropertyHotbarUpdateEvent_CommandConsumable_ZeroMaxUses(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.SampledFrom([]string{
			session.HotbarSlotKindCommand,
			session.HotbarSlotKindConsumable,
		}).Draw(rt, "kind")
		ref := rapid.StringOfN(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz_0123456789")), 1, 20, -1).Draw(rt, "ref")

		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		uid := "prop-cmd-cons-uid"
		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "tester",
			CharName:    "Tester",
			CharacterID: 30,
			RoomID:      "room_a",
			Role:        "player",
		})
		require.NoError(rt, err)

		sess, ok := sessMgr.GetPlayer(uid)
		require.True(rt, ok)
		sess.Hotbars[0][0] = session.HotbarSlot{Kind: kind, Ref: ref}

		evt := svc.hotbarUpdateEvent(sess)
		require.NotNil(rt, evt)
		hu := evt.GetHotbarUpdate()
		require.NotNil(rt, hu)
		require.Len(rt, hu.Slots, 10)

		slot0 := hu.Slots[0]
		assert.Equal(rt, int32(0), slot0.GetMaxUses(), "MaxUses must be 0 for kind=%s", kind)
		assert.Equal(rt, int32(0), slot0.GetUsesRemaining(), "UsesRemaining must be 0 for kind=%s", kind)
	})
}

// REQ-HB-UC-8: after activating a feat with limited uses, handleUse pushes a
// HotbarUpdateEvent with decremented UsesRemaining for the feat's hotbar slot.
func TestHandleUse_FeatWithLimitedUses_PushesHotbarUpdateEvent(t *testing.T) {
	const featID = "power_strike"
	sessMgr := session.NewManager()

	feat := &ruleset.Feat{
		ID:           featID,
		Name:         "Power Strike",
		Active:       true,
		PreparedUses: 3,
		ActivateText: "You strike with devastating power!",
	}
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{0: {featID}},
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	svc.characterFeatsRepo = featsRepo

	uid := "handleuse-hotbar-uid"
	sess := addPlayerForFeatTest(t, sessMgr, uid)
	sess.Conditions = condition.NewActiveSet()

	// Simulate 2 uses remaining before activation.
	sess.ActiveFeatUses = map[string]int{featID: 2}
	// Slot 0 assigned to this feat.
	sess.Hotbars[0][0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}

	// Drain any pre-existing events from the entity.
	for {
		select {
		case <-sess.Entity.Events():
		default:
			goto drained5
		}
	}
drained5:

	evt, err := svc.handleUse(uid, featID, "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	// After activation, sess.ActiveFeatUses[featID] must be decremented.
	assert.Equal(t, 1, sess.ActiveFeatUses[featID], "use count must decrement from 2 to 1")

	// Collect pushed events from entity channel.
	var hotbarEvt *gamev1.HotbarUpdateEvent
	for {
		select {
		case data := <-sess.Entity.Events():
			var pushed gamev1.ServerEvent
			if unmarshalErr := proto.Unmarshal(data, &pushed); unmarshalErr == nil {
				if hu := pushed.GetHotbarUpdate(); hu != nil {
					hotbarEvt = hu
				}
			}
		default:
			goto collected5
		}
	}
collected5:
	require.NotNil(t, hotbarEvt, "handleUse must push a HotbarUpdateEvent after feat activation")
	require.Len(t, hotbarEvt.Slots, 10)
	assert.Equal(t, int32(1), hotbarEvt.Slots[0].GetUsesRemaining(),
		"slot 0 UsesRemaining must reflect decremented count")
}

// REQ-HB-UC-9: after a full long rest, handleRest pushes a HotbarUpdateEvent
// with UsesRemaining restored to PreparedUses for the feat's hotbar slot.
func TestApplyFullLongRest_PushesHotbarUpdateEvent(t *testing.T) {
	const featID = "battle_cry"
	sessMgr := session.NewManager()

	feat := &ruleset.Feat{
		ID:                featID,
		Name:              "Battle Cry",
		Active:            true,
		PreparedUses:      3,
		RechargeCondition: "Recharges on rest",
		ActivateText:      "You roar a battle cry!",
	}
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{0: {featID}},
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	svc.characterFeatsRepo = featsRepo

	uid := "rest-hotbar-uid"
	sess := addPlayerForFeatTest(t, sessMgr, uid)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.CurrentHP = 1
	sess.MaxHP = 20

	// All uses exhausted before rest.
	sess.ActiveFeatUses = map[string]int{featID: 0}
	// Slot 0 assigned to this feat.
	sess.Hotbars[0][0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}

	// Provide a minimal job so applyFullLongRestCtx reaches the hotbarUpdateEvent push.
	job := &ruleset.Job{
		ID:   "test_rest_job",
		Name: "Test Rest Job",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{},
				Pool:         []ruleset.PreparedEntry{},
			},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.SetJobRegistry(jobReg)
	sess.Class = "test_rest_job"

	prepRepo := &fakePreparedRepoRest{}
	svc.SetPreparedTechRepo(prepRepo)

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	// Drain any pre-existing events from the entity.
	for {
		select {
		case <-sess.Entity.Events():
		default:
			goto drained6
		}
	}
drained6:

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req-rest-hotbar", stream))

	// After rest, uses must be restored.
	assert.Equal(t, 3, sess.ActiveFeatUses[featID], "uses must be restored to PreparedUses after rest")

	// Collect pushed events from entity channel.
	var hotbarEvt *gamev1.HotbarUpdateEvent
	for {
		select {
		case data := <-sess.Entity.Events():
			var pushed gamev1.ServerEvent
			if unmarshalErr := proto.Unmarshal(data, &pushed); unmarshalErr == nil {
				if hu := pushed.GetHotbarUpdate(); hu != nil {
					hotbarEvt = hu
				}
			}
		default:
			goto collected6
		}
	}
collected6:
	require.NotNil(t, hotbarEvt, "handleRest must push a HotbarUpdateEvent after long rest")
	require.Len(t, hotbarEvt.Slots, 10)
	assert.Equal(t, int32(3), hotbarEvt.Slots[0].GetUsesRemaining(),
		"slot 0 UsesRemaining must be restored to PreparedUses after rest")
}

// Property: set with valid slot 1–10 and non-empty text always writes to index slot-1 and returns confirmation.
func TestPropertyHandleHotbar_SetValidSlot(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		slot := rapid.Int32Range(1, 10).Draw(rt, "slot")
		text := rapid.StringOfN(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz ")), 1, 40, -1).Draw(rt, "text")

		svc, _, uid := testHotbarService(t)
		evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: slot, Text: text})
		require.NoError(t, err)
		require.NotNil(t, evt.GetMessage(), "set must return confirmation MessageEvent for non-empty text")

		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(t, ok)
		require.Equal(t, text, sess.Hotbars[0][slot-1].ActivationCommand())
	})
}

// REQ-HB-MB-1: "create" appends a new empty bar and switches to it.
//
// Precondition: Server has one bar; maxHotbars=4.
// Postcondition: Hotbars has 2 entries; ActiveHotbarIndex==1.
func TestHandleHotbar_Create_AddsBar(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 4)
	if len(sess.Hotbars) != 1 {
		t.Fatalf("expected 1 bar initially, got %d", len(sess.Hotbars))
	}
	ev, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "create"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = ev
	if len(sess.Hotbars) != 2 {
		t.Fatalf("expected 2 bars after create, got %d", len(sess.Hotbars))
	}
	if sess.ActiveHotbarIndex != 1 {
		t.Fatalf("expected ActiveHotbarIndex=1 after create, got %d", sess.ActiveHotbarIndex)
	}
}

// REQ-HB-MB-2: "create" returns a limit message when maxHotbars is reached.
//
// Precondition: Hotbars already at limit (maxHotbars=2, len=2).
// Postcondition: Hotbars count unchanged; returned event contains "limit" text.
func TestHandleHotbar_Create_EnforcesLimit(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 2)
	sess.Hotbars = [][10]session.HotbarSlot{{}, {}} // already at limit
	ev, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "create"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sess.Hotbars) != 2 {
		t.Fatalf("expected bars unchanged at limit, got %d", len(sess.Hotbars))
	}
	msg := extractMessage(ev)
	if !strings.Contains(msg, "limit") && !strings.Contains(msg, "Hotbar limit") {
		t.Fatalf("expected limit message, got %q", msg)
	}
}

// REQ-HB-MB-3: "switch" updates ActiveHotbarIndex to target bar (1-based).
//
// Precondition: Hotbars has 2 entries; HotbarIndex=2 (1-based).
// Postcondition: ActiveHotbarIndex==1 (0-based).
func TestHandleHotbar_Switch_ChangesIndex(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 4)
	sess.Hotbars = [][10]session.HotbarSlot{{}, {}}
	sess.ActiveHotbarIndex = 0
	_, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ActiveHotbarIndex != 1 {
		t.Fatalf("expected ActiveHotbarIndex=1, got %d", sess.ActiveHotbarIndex)
	}
}

// REQ-HB-MB-4: "switch" with out-of-range HotbarIndex does not change ActiveHotbarIndex.
//
// Precondition: Hotbars has 1 entry; HotbarIndex=5 (1-based, out of range).
// Postcondition: ActiveHotbarIndex unchanged at 0; event contains error message.
func TestHandleHotbar_Switch_OutOfRange(t *testing.T) {
	svc, sess, uid := newTestHotbarServer(t, 4)
	_, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ActiveHotbarIndex != 0 {
		t.Fatalf("expected ActiveHotbarIndex unchanged at 0, got %d", sess.ActiveHotbarIndex)
	}
}

// REQ-HB-MB-5 (property): after any sequence of create and switch operations,
// ActiveHotbarIndex is always within [0, len(Hotbars)-1].
func TestProperty_Hotbar_ActiveIndexAlwaysValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sess, uid := newTestHotbarServer(t, 4)
		ops := rapid.SliceOfN(rapid.IntRange(0, 5), 1, 20).Draw(rt, "ops")
		for _, op := range ops {
			switch op % 3 {
			case 0: // create
				_, _ = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "create"})
			case 1: // switch to next bar (wrapping)
				target := sess.ActiveHotbarIndex
				if len(sess.Hotbars) > 1 {
					target = (target + 1) % len(sess.Hotbars)
				}
				_, _ = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: int32(target + 1)})
			case 2: // switch to random index (may be out of range)
				targetIdx := rapid.IntRange(0, 10).Draw(rt, "idx")
				_, _ = svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "switch", HotbarIndex: int32(targetIdx)})
			}
			if sess.ActiveHotbarIndex < 0 || sess.ActiveHotbarIndex >= len(sess.Hotbars) {
				rt.Fatalf("ActiveHotbarIndex %d out of range [0, %d)", sess.ActiveHotbarIndex, len(sess.Hotbars))
			}
		}
	})
}

// TestParseTechRef verifies that parseTechRef decodes plain and level-encoded refs correctly.
func TestParseTechRef(t *testing.T) {
	tests := []struct {
		ref     string
		wantID  string
		wantLvl int
	}{
		{"frost_bolt", "frost_bolt", 0},
		{"frost_bolt:2", "frost_bolt", 2},
		{"frost_bolt:0", "frost_bolt:0", 0}, // level 0 is not a valid encoding — ref returned as-is
		{"a:b:3", "a:b", 3},               // last colon used
		{"no_colon", "no_colon", 0},
		{"tech:notanumber", "tech:notanumber", 0},
	}
	for _, tt := range tests {
		id, lvl := parseTechRef(tt.ref)
		assert.Equal(t, tt.wantID, id, "techID for ref=%q", tt.ref)
		assert.Equal(t, tt.wantLvl, lvl, "level for ref=%q", tt.ref)
	}
}

// TestProperty_ParseTechRef_RoundTrip verifies that encoding level > 0 as "id:level" always
// round-trips through parseTechRef correctly.
func TestProperty_ParseTechRef_RoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z0-9_]{0,19}`).Draw(rt, "id")
		level := rapid.IntRange(1, 10).Draw(rt, "level")
		encoded := fmt.Sprintf("%s:%d", id, level)
		gotID, gotLevel := parseTechRef(encoded)
		if gotID != id {
			rt.Fatalf("got techID=%q, want %q (encoded=%q)", gotID, id, encoded)
		}
		if gotLevel != level {
			rt.Fatalf("got level=%d, want %d (encoded=%q)", gotLevel, level, encoded)
		}
	})
}
