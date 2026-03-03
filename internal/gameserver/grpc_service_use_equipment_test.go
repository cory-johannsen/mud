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
)

// newUseEquipServer builds a GameServiceServer using testServiceWithAdmin but
// injects a RoomEquipmentManager when equipMgr is non-nil.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil *GameServiceServer with a single player session
// in room_a of the test world.
func newUseEquipServer(t *testing.T, equipMgr *inventory.RoomEquipmentManager) (*GameServiceServer, string) {
	t.Helper()
	svc := testServiceWithAdmin(t, nil)
	svc.roomEquipMgr = equipMgr

	uid := "ue_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "equip_user",
		CharName:    "EquipChar",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// TestHandleUseEquipment_UnknownSession verifies that an unregistered UID returns an error.
//
// Precondition: uid does not exist in the session manager.
// Postcondition: Returns nil event and non-nil error.
func TestHandleUseEquipment_UnknownSession(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)
	evt, err := svc.handleUseEquipment("nonexistent", "inst-1")
	assert.Nil(t, evt)
	assert.Error(t, err)
}

// TestHandleUseEquipment_NoEquipmentManager verifies that a nil roomEquipMgr returns
// a "No equipment available" message.
//
// Precondition: roomEquipMgr is nil; uid is a valid session.
// Postcondition: Returns a non-nil event whose message content contains "No equipment".
func TestHandleUseEquipment_NoEquipmentManager(t *testing.T) {
	svc, uid := newUseEquipServer(t, nil)
	evt, err := svc.handleUseEquipment(uid, "inst-1")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "No equipment")
}

// TestHandleUseEquipment_ItemNotHere verifies that requesting an instance not present
// in the room returns a "not here" message.
//
// Precondition: roomEquipMgr is initialised for room_a with no matching instance ID.
// Postcondition: Returns a non-nil event whose message content contains "not here".
func TestHandleUseEquipment_ItemNotHere(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "console", MaxCount: 1, Immovable: true, Script: ""},
	})
	svc, uid := newUseEquipServer(t, mgr)

	evt, err := svc.handleUseEquipment(uid, "nonexistent-instance-id")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not here")
}

// TestHandleUseEquipment_NoScript verifies that an item with an empty Script field
// returns a "Nothing happens" message.
//
// Precondition: roomEquipMgr contains an instance with Script == "" in room_a.
// Postcondition: Returns a non-nil event whose message content contains "Nothing happens".
func TestHandleUseEquipment_NoScript(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "terminal", MaxCount: 1, Immovable: true, Script: ""},
	})

	// Retrieve the real instance ID from the manager.
	instances := mgr.EquipmentInRoom("room_a")
	require.Len(t, instances, 1, "expected exactly 1 instance in room_a")
	instanceID := instances[0].InstanceID

	svc, uid := newUseEquipServer(t, mgr)
	evt, err := svc.handleUseEquipment(uid, instanceID)
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Nothing happens")
}

// TestProperty_HandleUseEquipment_NilMgr_NeverPanics is a property test verifying
// that arbitrary instance IDs with a nil equipment manager never panic.
//
// Precondition: roomEquipMgr is nil.
// Postcondition: handleUseEquipment never panics for any instanceID string.
func TestProperty_HandleUseEquipment_NilMgr_NeverPanics(t *testing.T) {
	svc, uid := newUseEquipServer(t, nil)
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.String().Draw(rt, "id")
		evt, err := svc.handleUseEquipment(uid, id)
		assert.NoError(t, err)
		assert.NotNil(t, evt)
	})
}
