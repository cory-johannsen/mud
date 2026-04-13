package gameserver

import (
	"context"
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

// REQ-HB-3: set valid slot 1 returns confirmation MessageEvent and writes command slot.
func TestHandleHotbar_SetSlot1(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 1, Text: "look"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.Equal(t, "Slot 1 set.", evt.GetMessage().GetContent(), "set must return confirmation MessageEvent")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, "look", sess.Hotbar[0].ActivationCommand())
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

// REQ-HB-4: clear valid slot returns confirmation MessageEvent.
func TestHandleHotbar_ClearSlot(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Hotbar[2] = session.CommandSlot("heal")

	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "clear", Slot: 3})
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.Equal(t, "Slot 3 cleared.", evt.GetMessage().GetContent(), "clear must return confirmation MessageEvent")
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

// REQ-HB-TS-1: handleHotbar set returns confirmation MessageEvent; HotbarUpdateEvent is pushed via entity.
func TestHandleHotbar_SetSendsUpdateEvent(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 1, Text: "look"})
	require.NoError(t, err)
	_, isMessage := evt.Payload.(*gamev1.ServerEvent_Message)
	assert.True(t, isMessage, "handleHotbar set should return a MessageEvent (HotbarUpdateEvent is pushed via entity)")
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

// REQ-HB-TS-2: kind+ref creates a typed feat slot and returns confirmation MessageEvent; HotbarUpdateEvent is pushed via entity.
func TestHandleHotbar_SetTypedFeatSlot(t *testing.T) {
	svc, _, uid := testHotbarService(t)
	evt, err := svc.handleHotbar(uid, &gamev1.HotbarRequest{Action: "set", Slot: 2, Kind: "feat", Ref: "power_strike"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"}, sess.Hotbar[1])
	assert.Equal(t, "use power_strike", sess.Hotbar[1].ActivationCommand())

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
	sess.Hotbar[0] = session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: techID}
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
	sess.Hotbar[0] = session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: techID}
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
func TestHotbarUpdateEvent_SpontaneousTechSlot_PopulatesUseCounts(t *testing.T) {
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
	sess.Hotbar[0] = session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: techID}
	sess.InnateTechs = nil
	// PreparedTechs must be non-nil but empty so the code enters the else-if branch
	// and falls through to the spontaneous pool lookup (total == 0 → spontaneous path).
	sess.PreparedTechs = map[int][]*session.PreparedSlot{}
	sess.SpontaneousTechs = map[int][]string{
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
		sess.Hotbar[0] = session.HotbarSlot{Kind: kind, Ref: ref}

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
	sess.Hotbar[0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}

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
	sess.Hotbar[0] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: featID}

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
		require.Equal(t, text, sess.Hotbar[slot-1].ActivationCommand())
	})
}
