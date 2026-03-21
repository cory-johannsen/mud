package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// newInteractServer builds a GameServiceServer with an optional RoomEquipmentManager
// and a single player session in room_a for handleInteract tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil *GameServiceServer and a valid player UID.
func newInteractServer(t *testing.T, equipMgr *inventory.RoomEquipmentManager) (*GameServiceServer, string) {
	t.Helper()
	svc := testServiceWithAdmin(t, nil)
	svc.roomEquipMgr = equipMgr

	uid := "int_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: "interact_user",
		CharName: "IntChar",
		RoomID:   "room_a",
		CurrentHP: 10,
		MaxHP:    10,
		Role:     "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// TestHandleInteract_UnknownSession verifies that handleInteract returns an error
// when the UID does not correspond to any active session.
//
// Precondition: uid "nonexistent_interact" is not registered.
// Postcondition: Returns nil event and a non-nil error.
func TestHandleInteract_UnknownSession(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)
	evt, err := svc.handleInteract("nonexistent_interact", "inst-1")
	assert.Nil(t, evt)
	assert.Error(t, err)
}

// TestHandleInteract_NoEquipmentManager verifies that handleInteract returns a
// "No equipment available" message when roomEquipMgr is nil.
//
// Precondition: roomEquipMgr is nil; uid is a valid session.
// Postcondition: Returns a non-nil event whose message contains "No equipment".
func TestHandleInteract_NoEquipmentManager(t *testing.T) {
	svc, uid := newInteractServer(t, nil)
	evt, err := svc.handleInteract(uid, "inst-1")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "No equipment")
}

// TestHandleInteract_ItemNotHere verifies that handleInteract returns a "not here"
// message when the given instanceID does not exist in the player's room.
//
// Precondition: roomEquipMgr is initialised for room_a with no matching instance ID.
// Postcondition: Returns a non-nil event whose message contains "not here".
func TestHandleInteract_ItemNotHere(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "lever", MaxCount: 1, Immovable: true, Script: ""},
	})
	svc, uid := newInteractServer(t, mgr)

	evt, err := svc.handleInteract(uid, "nonexistent-instance-id")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not here")
}

// TestHandleInteract_NoScript verifies that handleInteract returns a "Nothing happens"
// message when the matched equipment instance has no Lua script.
//
// Precondition: roomEquipMgr contains an instance with Script == "" in room_a.
// Postcondition: Returns a non-nil event whose message contains "Nothing happens".
func TestHandleInteract_NoScript(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "door_handle", MaxCount: 1, Immovable: true, Script: ""},
	})

	instances := mgr.EquipmentInRoom("room_a")
	require.Len(t, instances, 1, "expected exactly 1 instance in room_a")
	instanceID := instances[0].InstanceID

	svc, uid := newInteractServer(t, mgr)
	evt, err := svc.handleInteract(uid, instanceID)
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Nothing happens")
}

// TestProperty_HandleInteract_NilMgr_NeverPanics is a property test verifying that
// arbitrary instance IDs with a nil equipment manager never panic.
//
// Precondition: roomEquipMgr is nil.
// Postcondition: handleInteract never panics for any instanceID string.
func TestProperty_HandleInteract_NilMgr_NeverPanics(t *testing.T) {
	svc, uid := newInteractServer(t, nil)
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.String().Draw(rt, "id")
		evt, err := svc.handleInteract(uid, id)
		assert.NoError(t, err)
		assert.NotNil(t, evt)
	})
}

// TestHandleInteract_DelegatesTo_HandleUseEquipment verifies that handleInteract
// produces exactly the same result as a direct call to handleUseEquipment with the
// same arguments, confirming the delegation contract.
//
// Precondition: Two identical GameServiceServer instances share the same equipment setup.
// Postcondition: handleInteract and handleUseEquipment return identical message content.
func TestHandleInteract_DelegatesTo_HandleUseEquipment(t *testing.T) {
	mgr1 := inventory.NewRoomEquipmentManager()
	mgr1.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "wall_switch", MaxCount: 1, Immovable: true, Script: ""},
	})
	instances1 := mgr1.EquipmentInRoom("room_a")
	require.Len(t, instances1, 1)
	instanceID1 := instances1[0].InstanceID

	svc1, uid1 := newInteractServer(t, mgr1)
	interactEvt, interactErr := svc1.handleInteract(uid1, instanceID1)

	// Second identical server for direct handleUseEquipment call.
	mgr2 := inventory.NewRoomEquipmentManager()
	mgr2.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "wall_switch", MaxCount: 1, Immovable: true, Script: ""},
	})
	instances2 := mgr2.EquipmentInRoom("room_a")
	require.Len(t, instances2, 1)
	instanceID2 := instances2[0].InstanceID

	svc2, uid2 := newInteractServer(t, mgr2)
	useEvt, useErr := svc2.handleUseEquipment(uid2, instanceID2)

	// Both must agree on error status.
	assert.Equal(t, useErr != nil, interactErr != nil)

	// Both must return the same message content.
	require.NoError(t, interactErr)
	require.NoError(t, useErr)
	require.NotNil(t, interactEvt)
	require.NotNil(t, useEvt)
	assert.Equal(t, useEvt.GetMessage().Content, interactEvt.GetMessage().Content)
}
