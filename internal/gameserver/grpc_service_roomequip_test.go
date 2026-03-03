package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newRoomEquipServer builds a GameServiceServer with an optional RoomEquipmentManager
// and a player session in room_a.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil *GameServiceServer with a single player session in room_a.
func newRoomEquipServer(t *testing.T, equipMgr *inventory.RoomEquipmentManager) (*GameServiceServer, string) {
	t.Helper()
	svc := testServiceWithAdmin(t, nil)
	svc.roomEquipMgr = equipMgr

	uid := "re_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "equip_editor",
		CharName:    "EditorChar",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "admin",
	})
	require.NoError(t, err)
	return svc, uid
}

// TestHandleRoomEquip_UnknownSession verifies that an unregistered UID returns an error.
//
// Precondition: uid does not exist in the session manager.
// Postcondition: Returns nil event and non-nil error.
func TestHandleRoomEquip_UnknownSession(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)
	evt, err := svc.handleRoomEquip("nonexistent", &gamev1.RoomEquipRequest{SubCommand: "list"})
	assert.Nil(t, evt)
	assert.Error(t, err)
}

// TestHandleRoomEquip_NilManager verifies that a nil roomEquipMgr returns a
// "not available" message.
//
// Precondition: roomEquipMgr is nil; uid is a valid session.
// Postcondition: Returns a non-nil event whose message content contains "not available".
func TestHandleRoomEquip_NilManager(t *testing.T) {
	svc, uid := newRoomEquipServer(t, nil)
	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "list"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not available")
}

// TestHandleRoomEquip_ListEmpty verifies that listing equipment for a room with no
// config returns a "No equipment configured" message.
//
// Precondition: roomEquipMgr is initialised but room_a has no equipment configs.
// Postcondition: Returns a non-nil event whose message content contains "No equipment configured".
func TestHandleRoomEquip_ListEmpty(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "list"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "No equipment configured")
}

// TestHandleRoomEquip_AddValid verifies that adding a valid item returns an "Added" message.
//
// Precondition: roomEquipMgr is initialised; item_id is non-empty.
// Postcondition: Returns a non-nil event whose message content contains "Added".
func TestHandleRoomEquip_AddValid(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{
		SubCommand: "add",
		ItemId:     "console",
		MaxCount:   1,
		Immovable:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Added")
	assert.Contains(t, msg.Content, "console")
}

// TestHandleRoomEquip_AddEmptyItemID verifies that an add subcommand with an empty
// item_id returns a usage message.
//
// Precondition: item_id is "".
// Postcondition: Returns a non-nil event whose message content contains "Usage".
func TestHandleRoomEquip_AddEmptyItemID(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "add", ItemId: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Usage")
}

// TestHandleRoomEquip_RemoveExisting verifies that removing an existing item returns a
// "Removed" message.
//
// Precondition: item "terminal" has been added to room_a.
// Postcondition: Returns a non-nil event whose message content contains "Removed".
func TestHandleRoomEquip_RemoveExisting(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.AddConfig("room_a", world.RoomEquipmentConfig{ItemID: "terminal", MaxCount: 1, Immovable: true})
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "remove", ItemId: "terminal"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Removed")
}

// TestHandleRoomEquip_RemoveNonExistent verifies that removing an item not present in
// the room returns a "not found" message.
//
// Precondition: room_a has no equipment configs.
// Postcondition: Returns a non-nil event whose message content contains "not found".
func TestHandleRoomEquip_RemoveNonExistent(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "remove", ItemId: "ghost_item"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not found")
}

// TestHandleRoomEquip_RemoveEmptyItemID verifies that a remove subcommand with an empty
// item_id returns a usage message.
//
// Precondition: item_id is "".
// Postcondition: Returns a non-nil event whose message content contains "Usage".
func TestHandleRoomEquip_RemoveEmptyItemID(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "remove", ItemId: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Usage")
}

// TestHandleRoomEquip_ModifyExisting verifies that modifying an existing item returns a
// "Modified" message.
//
// Precondition: item "locker" has been added to room_a.
// Postcondition: Returns a non-nil event whose message content contains "Modified".
func TestHandleRoomEquip_ModifyExisting(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.AddConfig("room_a", world.RoomEquipmentConfig{ItemID: "locker", MaxCount: 1, Immovable: true})
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{
		SubCommand: "modify",
		ItemId:     "locker",
		MaxCount:   3,
		Immovable:  false,
	})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Modified")
}

// TestHandleRoomEquip_ModifyEmptyItemID verifies that a modify subcommand with an empty
// item_id returns a usage message.
//
// Precondition: item_id is "".
// Postcondition: Returns a non-nil event whose message content contains "Usage".
func TestHandleRoomEquip_ModifyEmptyItemID(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "modify", ItemId: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Usage")
}

// TestHandleRoomEquip_DefaultSubcommand verifies that an unknown subcommand returns a
// usage message.
//
// Precondition: SubCommand is not one of add/remove/list/modify.
// Postcondition: Returns a non-nil event whose message content contains "Usage".
func TestHandleRoomEquip_DefaultSubcommand(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "bogus"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Usage")
}

// TestHandleRoomEquip_ListAfterAdd verifies that after adding an item via "add",
// a subsequent "list" includes the item ID.
//
// Precondition: roomEquipMgr is initialised; no items in room_a initially.
// Postcondition: "list" returns a message containing the newly added item ID.
func TestHandleRoomEquip_ListAfterAdd(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)

	_, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{
		SubCommand: "add",
		ItemId:     "crate",
		MaxCount:   2,
	})
	require.NoError(t, err)

	evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{SubCommand: "list"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "crate")
}

// TestProperty_HandleRoomEquip_NeverPanics is a property test verifying that
// handleRoomEquip never panics for any sub_command or item_id combination.
//
// Precondition: roomEquipMgr is initialised; uid is a valid session.
// Postcondition: handleRoomEquip returns without panicking for all inputs.
func TestProperty_HandleRoomEquip_NeverPanics(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	svc, uid := newRoomEquipServer(t, mgr)
	rapid.Check(t, func(rt *rapid.T) {
		sub := rapid.String().Draw(rt, "sub")
		item := rapid.String().Draw(rt, "item")
		count := rapid.Int32Range(0, 10).Draw(rt, "count")
		evt, err := svc.handleRoomEquip(uid, &gamev1.RoomEquipRequest{
			SubCommand: sub,
			ItemId:     item,
			MaxCount:   count,
		})
		// Either a valid event or a valid error — never a panic.
		if err == nil {
			assert.NotNil(t, evt)
		}
	})
}
