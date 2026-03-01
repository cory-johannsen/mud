package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

type mockCharSaver struct {
	saved map[int64]string // characterID → roomID
}

func (m *mockCharSaver) SaveState(_ context.Context, id int64, location string, _ int) error {
	m.saved[id] = location
	return nil
}

func (m *mockCharSaver) LoadWeaponPresets(_ context.Context, _ int64) (*inventory.LoadoutSet, error) {
	return inventory.NewLoadoutSet(), nil
}

func (m *mockCharSaver) SaveWeaponPresets(_ context.Context, _ int64, _ *inventory.LoadoutSet) error {
	return nil
}

func (m *mockCharSaver) LoadEquipment(_ context.Context, _ int64) (*inventory.Equipment, error) {
	return inventory.NewEquipment(), nil
}

func (m *mockCharSaver) SaveEquipment(_ context.Context, _ int64, _ *inventory.Equipment) error {
	return nil
}

func TestHandleTeleport_AdminSuccess(t *testing.T) {
	saver := &mockCharSaver{saved: make(map[int64]string)}
	svc := testServiceWithAdmin(t, nil)
	svc.charSaver = saver

	_, err := svc.sessions.AddPlayer("admin1", "admin_user", "Admin", 1, "room_a", 10, "admin", "", "", 0)
	require.NoError(t, err)
	_, err = svc.sessions.AddPlayer("target1", "target_user", "Target", 2, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleTeleport("admin1", &gamev1.TeleportRequest{
		TargetCharacter: "Target",
		RoomId:          "room_b",
	})
	require.NoError(t, err)
	msg := resp.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Teleported Target to Room B")

	// Verify session was moved.
	target, ok := svc.sessions.GetPlayer("target1")
	require.True(t, ok)
	assert.Equal(t, "room_b", target.RoomID)

	// Verify location was persisted.
	assert.Equal(t, "room_b", saver.saved[2])
}

func TestHandleTeleport_NonAdminDenied(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	_, err := svc.sessions.AddPlayer("u1", "user", "User", 1, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleTeleport("u1", &gamev1.TeleportRequest{
		TargetCharacter: "Someone",
		RoomId:          "room_b",
	})
	require.NoError(t, err)
	errEvt := resp.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "permission denied")
}

func TestHandleTeleport_InvalidRoom(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	_, err := svc.sessions.AddPlayer("admin1", "admin_user", "Admin", 1, "room_a", 10, "admin", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleTeleport("admin1", &gamev1.TeleportRequest{
		TargetCharacter: "Someone",
		RoomId:          "nonexistent",
	})
	require.NoError(t, err)
	errEvt := resp.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "room")
	assert.Contains(t, errEvt.Message, "not found")
}

func TestHandleTeleport_TargetNotOnline(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	_, err := svc.sessions.AddPlayer("admin1", "admin_user", "Admin", 1, "room_a", 10, "admin", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleTeleport("admin1", &gamev1.TeleportRequest{
		TargetCharacter: "Nobody",
		RoomId:          "room_b",
	})
	require.NoError(t, err)
	errEvt := resp.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "not online")
}
