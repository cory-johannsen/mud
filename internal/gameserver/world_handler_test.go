package gameserver

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
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
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Equal(t, "room_a", view.RoomId)
	assert.Equal(t, "Room A", view.Title)
	assert.Contains(t, view.Description, "first room")
}

func TestWorldHandler_Look_NotFound(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)

	_, err := h.Look("nonexistent")
	assert.Error(t, err)
}

func TestWorldHandler_Move(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Move("u1", world.North)
	require.NoError(t, err)
	assert.Equal(t, "room_b", view.RoomId)
	assert.Equal(t, "Room B", view.Title)
}

func TestWorldHandler_Move_NoExit(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	_, err = h.Move("u1", world.West)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no exit")
}

func TestWorldHandler_MoveWithContext(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	result, err := h.MoveWithContext("u1", world.North)
	require.NoError(t, err)
	assert.Equal(t, "room_a", result.OldRoomID)
	assert.Equal(t, "room_b", result.View.RoomId)
}

func TestWorldHandler_Exits(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	exitList, err := h.Exits("u1")
	require.NoError(t, err)
	require.Len(t, exitList.Exits, 1)
	assert.Equal(t, "north", exitList.Exits[0].Direction)
	assert.Equal(t, "room_b", exitList.Exits[0].TargetRoomId)
}

func TestWorldHandler_RoomViewExcludesSelf(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)
	_, err = sessMgr.AddPlayer("u2", "Bob", "Bob", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Contains(t, view.Players, "Bob")
	assert.NotContains(t, view.Players, "Alice")
}

// testWorldAndSessionWithClock builds a WorldHandler that includes a GameClock
// fixed at startHour. The clock is not started so it will not advance.
func testWorldAndSessionWithClock(t *testing.T, startHour int32) (*WorldHandler, *world.Manager, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	clock := NewGameClock(startHour, time.Hour*24) // long tick — will not advance during test
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock)
	return h, worldMgr, sessMgr
}

func TestBuildRoomView_TimeOfDay_HourAndPeriodPopulated(t *testing.T) {
	const startHour int32 = 17 // Dusk
	h, _, sessMgr := testWorldAndSessionWithClock(t, startHour)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Equal(t, int32(17), view.Hour)
	assert.Equal(t, string(PeriodDusk), view.Period)
}

func TestBuildRoomView_DarkPeriod_OutdoorHidesExits(t *testing.T) {
	const startHour int32 = 0 // Midnight — dark
	worldMgr, sessMgr := testWorldAndSession(t)

	// Mark room_a as outdoor and give it an exit
	room, ok := worldMgr.GetRoom("room_a")
	require.True(t, ok)
	room.Properties = map[string]string{"outdoor": "true"}

	clock := NewGameClock(startHour, time.Hour*24)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Nil(t, view.Exits, "outdoor exits must be hidden during dark periods")
}

func TestBuildRoomView_LightPeriod_OutdoorShowsExits(t *testing.T) {
	const startHour int32 = 12 // Afternoon — light
	worldMgr, sessMgr := testWorldAndSession(t)

	room, ok := worldMgr.GetRoom("room_a")
	require.True(t, ok)
	room.Properties = map[string]string{"outdoor": "true"}

	clock := NewGameClock(startHour, time.Hour*24)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.NotNil(t, view.Exits, "outdoor exits must be visible during light periods")
	assert.NotEmpty(t, view.Exits)
}

func TestBuildRoomView_OutdoorFlavorText_Appended(t *testing.T) {
	const startHour int32 = 12 // Afternoon
	worldMgr, sessMgr := testWorldAndSession(t)

	room, ok := worldMgr.GetRoom("room_a")
	require.True(t, ok)
	originalDesc := room.Description
	room.Properties = map[string]string{"outdoor": "true"}

	clock := NewGameClock(startHour, time.Hour*24)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(view.Description, originalDesc), "description must begin with the room's original description")
	assert.Greater(t, len(view.Description), len(originalDesc), "description must have a non-empty flavor text suffix appended")
}

func TestBuildRoomView_IndoorNoFlavorText(t *testing.T) {
	const startHour int32 = 12 // Afternoon
	worldMgr, sessMgr := testWorldAndSession(t)

	room, ok := worldMgr.GetRoom("room_a")
	require.True(t, ok)
	originalDesc := room.Description
	// room_a already has empty Properties (no outdoor key) from testWorldAndSession

	clock := NewGameClock(startHour, time.Hour*24)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Equal(t, originalDesc, view.Description, "indoor rooms must have no flavor text appended")
}

func TestProperty_BuildRoomView_HourAlwaysInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		startHour := rapid.Int32Range(0, 23).Draw(rt, "startHour")
		worldMgr, sessMgr := testWorldAndSession(t)
		clock := NewGameClock(startHour, time.Hour*24)
		h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock)

		_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player", "", "", 0)
		require.NoError(rt, err)

		view, err := h.Look("u1")
		require.NoError(rt, err)
		assert.Equal(rt, startHour, view.Hour, "RoomView.Hour must equal the clock's startHour")
	})
}
