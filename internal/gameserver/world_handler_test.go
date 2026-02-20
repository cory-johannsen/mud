package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

func testWorldAndSession(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test",
		Name:        "Test",
		Description: "Test zone",
		StartRoom:   "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test",
				Title:       "Room A",
				Description: "The first room.",
				Exits: []world.Exit{
					{Direction: world.North, TargetRoom: "room_b"},
				},
				Properties: map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "The second room.",
				Exits: []world.Exit{
					{Direction: world.South, TargetRoom: "room_a"},
				},
				Properties: map[string]string{},
			},
		},
	}

	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)

	return worldMgr, session.NewManager()
}

func TestWorldHandler_Look(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Equal(t, "room_a", view.RoomId)
	assert.Equal(t, "Room A", view.Title)
	assert.Contains(t, view.Description, "first room")
}

func TestWorldHandler_Look_NotFound(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr)

	_, err := h.Look("nonexistent")
	assert.Error(t, err)
}

func TestWorldHandler_Move(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10)
	require.NoError(t, err)

	view, err := h.Move("u1", world.North)
	require.NoError(t, err)
	assert.Equal(t, "room_b", view.RoomId)
	assert.Equal(t, "Room B", view.Title)
}

func TestWorldHandler_Move_NoExit(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10)
	require.NoError(t, err)

	_, err = h.Move("u1", world.West)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no exit")
}

func TestWorldHandler_MoveWithContext(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10)
	require.NoError(t, err)

	result, err := h.MoveWithContext("u1", world.North)
	require.NoError(t, err)
	assert.Equal(t, "room_a", result.OldRoomID)
	assert.Equal(t, "room_b", result.View.RoomId)
}

func TestWorldHandler_Exits(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10)
	require.NoError(t, err)

	exitList, err := h.Exits("u1")
	require.NoError(t, err)
	require.Len(t, exitList.Exits, 1)
	assert.Equal(t, "north", exitList.Exits[0].Direction)
	assert.Equal(t, "room_b", exitList.Exits[0].TargetRoomId)
}

func TestWorldHandler_RoomViewExcludesSelf(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10)
	require.NoError(t, err)
	_, err = sessMgr.AddPlayer("u2", "Bob", "Bob", 0, "room_a", 10)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Contains(t, view.Players, "Bob")
	assert.NotContains(t, view.Players, "Alice")
}
