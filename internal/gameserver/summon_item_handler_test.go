package gameserver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// testSummonItemDef is a minimal ItemDef used across summon_item tests.
var testSummonItemDef = &inventory.ItemDef{
	ID:       "sword_01",
	Name:     "Iron Sword",
	MaxStack: 1,
}

// newSummonItemServiceOpts builds a GameServiceServer wired with the given
// FloorManager (may be nil) and inventory Registry for use in summon_item tests.
func newSummonItemServiceOpts(t *testing.T, fm *inventory.FloorManager, reg *inventory.Registry) *GameServiceServer {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	return NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, nil, nil, nil,
		nil, fm, nil, nil, reg,
		nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
	)
}

// newSummonItemService builds a GameServiceServer wired with the given
// FloorManager and inventory Registry for use in summon_item tests.
func newSummonItemService(t *testing.T, fm *inventory.FloorManager, reg *inventory.Registry) *GameServiceServer {
	t.Helper()
	return newSummonItemServiceOpts(t, fm, reg)
}

// addSummonTestPlayer is a helper that adds a player with the given uid, roomID, and role to svc.sessions.
func addSummonTestPlayer(t *testing.T, svc *GameServiceServer, uid, roomID, role string) {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:               uid,
		Username:          uid + "_user",
		CharName:          uid + "_char",
		CharacterID:       1,
		RoomID:            roomID,
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              role,
		RegionDisplayName: "",
		Class:             "",
		Level:             1,
	})
	require.NoError(t, err)
}

// TestHandleSummonItem_EditorSuccess verifies that an editor-role player can summon
// an item and the item appears on the floor of their current room.
func TestHandleSummonItem_EditorSuccess(t *testing.T) {
	fm := inventory.NewFloorManager()
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(testSummonItemDef))

	svc := newSummonItemService(t, fm, reg)
	addSummonTestPlayer(t, svc, "u1", "room_a", "editor")

	resp, err := svc.handleSummonItem("u1", &gamev1.SummonItemRequest{
		ItemId:   "sword_01",
		Quantity: 2,
	})
	require.NoError(t, err)

	msg := resp.GetMessage()
	require.NotNil(t, msg, "expected message event on success")
	assert.Contains(t, msg.Content, "Summoned")
	assert.Contains(t, msg.Content, "Iron Sword")

	items := fm.ItemsInRoom("room_a")
	require.Len(t, items, 1, "expected exactly one item on floor")
	assert.Equal(t, "sword_01", items[0].ItemDefID)
	assert.Equal(t, 2, items[0].Quantity)
}

// TestHandleSummonItem_AdminSuccess verifies that an admin-role player can summon an item.
func TestHandleSummonItem_AdminSuccess(t *testing.T) {
	fm := inventory.NewFloorManager()
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(testSummonItemDef))

	svc := newSummonItemService(t, fm, reg)
	addSummonTestPlayer(t, svc, "u2", "room_a", "admin")

	resp, err := svc.handleSummonItem("u2", &gamev1.SummonItemRequest{
		ItemId:   "sword_01",
		Quantity: 1,
	})
	require.NoError(t, err)

	msg := resp.GetMessage()
	require.NotNil(t, msg, "expected message event for admin success")
	assert.Contains(t, msg.Content, "Summoned")

	items := fm.ItemsInRoom("room_a")
	require.Len(t, items, 1)
}

// TestHandleSummonItem_PlayerDenied verifies that a player-role user gets permission denied
// and no item is placed on the floor.
func TestHandleSummonItem_PlayerDenied(t *testing.T) {
	fm := inventory.NewFloorManager()
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(testSummonItemDef))

	svc := newSummonItemService(t, fm, reg)
	addSummonTestPlayer(t, svc, "u3", "room_a", "player")

	resp, err := svc.handleSummonItem("u3", &gamev1.SummonItemRequest{
		ItemId:   "sword_01",
		Quantity: 1,
	})
	require.NoError(t, err)

	errEvt := resp.GetError()
	require.NotNil(t, errEvt, "expected error event for permission denied")
	assert.Contains(t, errEvt.Message, "permission denied")

	items := fm.ItemsInRoom("room_a")
	assert.Empty(t, items, "no items should be placed when permission is denied")
}

// TestHandleSummonItem_UnknownItemID verifies that an unknown item ID returns an error
// message and places no item on the floor.
func TestHandleSummonItem_UnknownItemID(t *testing.T) {
	fm := inventory.NewFloorManager()
	reg := inventory.NewRegistry()

	svc := newSummonItemService(t, fm, reg)
	addSummonTestPlayer(t, svc, "u4", "room_a", "editor")

	resp, err := svc.handleSummonItem("u4", &gamev1.SummonItemRequest{
		ItemId:   "does_not_exist",
		Quantity: 1,
	})
	require.NoError(t, err)

	errEvt := resp.GetError()
	require.NotNil(t, errEvt, "expected error event for unknown item")
	assert.Contains(t, errEvt.Message, "unknown item")

	items := fm.ItemsInRoom("room_a")
	assert.Empty(t, items, "no items should be placed for unknown item ID")
}

// TestHandleSummonItem_SessionNotFound verifies that a missing session returns an error
// message.
func TestHandleSummonItem_SessionNotFound(t *testing.T) {
	fm := inventory.NewFloorManager()
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(testSummonItemDef))

	svc := newSummonItemService(t, fm, reg)
	// No player added — uid does not exist in session manager.

	resp, err := svc.handleSummonItem("no_such_uid", &gamev1.SummonItemRequest{
		ItemId:   "sword_01",
		Quantity: 1,
	})
	require.NoError(t, err)

	errEvt := resp.GetError()
	require.NotNil(t, errEvt, "expected error event for missing session")
	assert.Contains(t, errEvt.Message, "player not found")

	items := fm.ItemsInRoom("room_a")
	assert.Empty(t, items, "no items should be placed when session is missing")
}

// TestHandleSummonItem_ZeroQuantityDefaultsToOne verifies that Quantity=0 is treated as 1.
func TestHandleSummonItem_ZeroQuantityDefaultsToOne(t *testing.T) {
	fm := inventory.NewFloorManager()
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(testSummonItemDef))

	svc := newSummonItemService(t, fm, reg)
	addSummonTestPlayer(t, svc, "u5", "room_a", "editor")

	resp, err := svc.handleSummonItem("u5", &gamev1.SummonItemRequest{
		ItemId:   "sword_01",
		Quantity: 0,
	})
	require.NoError(t, err)

	msg := resp.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "1x")

	items := fm.ItemsInRoom("room_a")
	require.Len(t, items, 1)
	assert.Equal(t, 1, items[0].Quantity, "quantity 0 must default to 1")
}

// TestHandleSummonItem_FloorMgrNil verifies that when the FloorManager is nil,
// handleSummonItem returns an error event containing "floor system not available"
// and does not panic.
func TestHandleSummonItem_FloorMgrNil(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(testSummonItemDef))

	svc := newSummonItemServiceOpts(t, nil, reg)
	addSummonTestPlayer(t, svc, "u_nil_fm", "room_a", "editor")

	resp, err := svc.handleSummonItem("u_nil_fm", &gamev1.SummonItemRequest{
		ItemId:   "sword_01",
		Quantity: 1,
	})
	require.NoError(t, err)

	errEvt := resp.GetError()
	require.NotNil(t, errEvt, "expected error event when floorMgr is nil")
	assert.Contains(t, errEvt.Message, "floor system not available")
}

// TestPropertySummonItem_EditorValidQtyAlwaysSucceeds is a property-based test that
// verifies that any editor or admin with a positive quantity always produces a success
// message and places exactly one ItemInstance (with correct quantity) on the floor.
func TestPropertySummonItem_EditorValidQtyAlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		fm := inventory.NewFloorManager()
		reg := inventory.NewRegistry()
		if err := reg.RegisterItem(testSummonItemDef); err != nil {
			rt.Fatalf("RegisterItem: %v", err)
		}

		svc := newSummonItemService(t, fm, reg)

		role := rapid.SampledFrom([]string{"editor", "admin"}).Draw(rt, "role")
		qty := rapid.Int32Range(1, 100).Draw(rt, "qty")
		uid := fmt.Sprintf("prop_u_%d", rapid.IntRange(0, 99999).Draw(rt, "uid_n"))

		_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID:               uid,
			Username:          uid + "_user",
			CharName:          uid + "_char",
			CharacterID:       1,
			RoomID:            "room_a",
			CurrentHP:         10,
			MaxHP:             10,
			Abilities:         character.AbilityScores{},
			Role:              role,
			RegionDisplayName: "",
			Class:             "",
			Level:             1,
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}

		resp, err := svc.handleSummonItem(uid, &gamev1.SummonItemRequest{
			ItemId:   "sword_01",
			Quantity: qty,
		})
		if err != nil {
			rt.Fatalf("handleSummonItem error: %v", err)
		}

		msg := resp.GetMessage()
		if msg == nil {
			rt.Fatalf("expected message event for role=%s qty=%d, got: %v", role, qty, resp)
		}
		if !strings.Contains(msg.Content, "Summoned") {
			rt.Fatalf("expected 'Summoned' in response, got: %q", msg.Content)
		}

		items := fm.ItemsInRoom("room_a")
		if len(items) != 1 {
			rt.Fatalf("expected 1 item on floor, got %d", len(items))
		}
		if items[0].Quantity != int(qty) {
			rt.Fatalf("expected quantity %d, got %d", qty, items[0].Quantity)
		}
	})
}

