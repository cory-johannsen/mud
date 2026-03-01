package gameserver

import (
	"fmt"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newLoadoutServer creates a GameServiceServer with a default loadout-aware session.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a server with one player session that has LoadoutSet and Equipment initialized.
func newLoadoutServer(t *testing.T, uid string) *GameServiceServer {
	t.Helper()
	svc := testServiceWithAdmin(t, nil)
	_, err := svc.sessions.AddPlayer(uid, "test_user", "TestChar", 1, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	sess.Equipment = inventory.NewEquipment()
	return svc
}

// TestHandleLoadout_DisplaysPresets verifies that handleLoadout with no arg returns
// a message containing "Preset", indicating the loadout set was rendered.
//
// Precondition: Player session has a non-nil LoadoutSet.
// Postcondition: Returns a ServerEvent whose MessageEvent.Content contains "Preset".
func TestHandleLoadout_DisplaysPresets(t *testing.T) {
	svc := newLoadoutServer(t, "u1")

	evt, err := svc.handleLoadout("u1", &gamev1.LoadoutRequest{Arg: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Preset")
}

// TestHandleUnequip_UnknownSlot verifies that handleUnequip with an invalid slot name
// returns a message containing "unknown slot".
//
// Precondition: Player session has a non-nil LoadoutSet and Equipment.
// Postcondition: Returns a ServerEvent whose MessageEvent.Content contains "unknown slot".
func TestHandleUnequip_UnknownSlot(t *testing.T) {
	svc := newLoadoutServer(t, "u1")

	evt, err := svc.handleUnequip("u1", &gamev1.UnequipRequest{Slot: "bad_slot"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Unknown slot")
}

// TestHandleEquipment_ReturnsDisplay verifies that handleEquipment returns a message
// containing "Weapons", indicating the full equipment display was rendered.
//
// Precondition: Player session has non-nil LoadoutSet and Equipment.
// Postcondition: Returns a ServerEvent whose MessageEvent.Content contains "Weapons".
func TestHandleEquipment_ReturnsDisplay(t *testing.T) {
	svc := newLoadoutServer(t, "u1")

	evt, err := svc.handleEquipment("u1", &gamev1.EquipmentRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Weapons")
}

// TestPropertyHandleEquipment_ValidSessionAlwaysReturnsEvent is a property test
// verifying that any uid mapped to a valid session always produces a non-nil ServerEvent.
//
// Precondition: uid must be registered in the session manager with valid LoadoutSet and Equipment.
// Postcondition: handleEquipment always returns a non-nil event and nil error for valid sessions.
func TestPropertyHandleEquipment_ValidSessionAlwaysReturnsEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid := fmt.Sprintf("prop_u_%d", rapid.IntRange(0, 99999).Draw(rt, "uid"))
		svc := testServiceWithAdmin(t, nil)
		_, addErr := svc.sessions.AddPlayer(uid, "u", "Char", 1, "room_a", 10, "player", "", "", 0)
		if addErr != nil {
			rt.Fatalf("AddPlayer: %v", addErr)
		}
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session must exist after AddPlayer")
		}
		sess.LoadoutSet = inventory.NewLoadoutSet()
		sess.Equipment = inventory.NewEquipment()

		evt, err := svc.handleEquipment(uid, &gamev1.EquipmentRequest{})
		if err != nil {
			rt.Fatalf("handleEquipment: %v", err)
		}
		if evt == nil {
			rt.Fatal("expected non-nil ServerEvent for valid session")
		}
	})
}

// TestHandleLoadout_PlayerNotFound verifies that handleLoadout returns an error event
// when the uid does not match any session.
//
// Precondition: uid is not registered.
// Postcondition: Returns an ErrorEvent with message "player not found".
func TestHandleLoadout_PlayerNotFound(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	evt, err := svc.handleLoadout("missing", &gamev1.LoadoutRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "player not found")
}

// TestHandleUnequip_PlayerNotFound verifies that handleUnequip returns an error event
// when the uid does not match any session.
//
// Precondition: uid is not registered.
// Postcondition: Returns an ErrorEvent with message "player not found".
func TestHandleUnequip_PlayerNotFound(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	evt, err := svc.handleUnequip("missing", &gamev1.UnequipRequest{Slot: "main"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "player not found")
}

// TestHandleEquipment_PlayerNotFound verifies that handleEquipment returns an error event
// when the uid does not match any session.
//
// Precondition: uid is not registered.
// Postcondition: Returns an ErrorEvent with message "player not found".
func TestHandleEquipment_PlayerNotFound(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	evt, err := svc.handleEquipment("missing", &gamev1.EquipmentRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "player not found")
}

// TestPropertyHandleLoadout_ValidSessionAlwaysReturnsEvent asserts that any valid
// session uid always receives a non-nil ServerEvent from handleLoadout, regardless of arg.
// Precondition: uid maps to a valid player session.
// Postcondition: event is non-nil and has a message payload.
func TestPropertyHandleLoadout_ValidSessionAlwaysReturnsEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid := fmt.Sprintf("prop_loadout_%d", rapid.IntRange(0, 99999).Draw(rt, "uid"))
		svc := testServiceWithAdmin(t, nil)
		_, addErr := svc.sessions.AddPlayer(uid, "u", "Char", 1, "room_a", 10, "player", "", "", 0)
		if addErr != nil {
			rt.Fatalf("AddPlayer: %v", addErr)
		}
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session must exist after AddPlayer")
		}
		sess.LoadoutSet = inventory.NewLoadoutSet()
		sess.Equipment = inventory.NewEquipment()

		arg := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter)).Draw(rt, "arg")
		evt, err := svc.handleLoadout(uid, &gamev1.LoadoutRequest{Arg: arg})
		require.NoError(t, err)
		require.NotNil(t, evt)
		require.NotNil(t, evt.GetMessage())
	})
}

// TestPropertyHandleUnequip_ValidSessionAlwaysReturnsEvent asserts that any valid
// session uid always receives a non-nil ServerEvent from handleUnequip, regardless of slot.
// Precondition: uid maps to a valid player session.
// Postcondition: event is non-nil and has a message payload.
func TestPropertyHandleUnequip_ValidSessionAlwaysReturnsEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid := fmt.Sprintf("prop_unequip_%d", rapid.IntRange(0, 99999).Draw(rt, "uid"))
		svc := testServiceWithAdmin(t, nil)
		_, addErr := svc.sessions.AddPlayer(uid, "u", "Char", 1, "room_a", 10, "player", "", "", 0)
		if addErr != nil {
			rt.Fatalf("AddPlayer: %v", addErr)
		}
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session must exist after AddPlayer")
		}
		sess.LoadoutSet = inventory.NewLoadoutSet()
		sess.Equipment = inventory.NewEquipment()

		slot := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter)).Draw(rt, "slot")
		evt, err := svc.handleUnequip(uid, &gamev1.UnequipRequest{Slot: slot})
		require.NoError(t, err)
		require.NotNil(t, evt)
		require.NotNil(t, evt.GetMessage())
	})
}
