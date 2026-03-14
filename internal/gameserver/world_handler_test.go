package gameserver

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
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
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Equal(t, "room_a", view.RoomId)
	assert.Equal(t, "Room A", view.Title)
	assert.Contains(t, view.Description, "first room")
}

func TestWorldHandler_Look_NotFound(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)

	_, err := h.Look("nonexistent")
	assert.Error(t, err)
}

func TestWorldHandler_Move(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)

	view, err := h.Move("u1", world.North)
	require.NoError(t, err)
	assert.Equal(t, "room_b", view.RoomId)
	assert.Equal(t, "Room B", view.Title)
}

func TestWorldHandler_Move_NoExit(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)

	_, err = h.Move("u1", world.West)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no exit")
}

func TestWorldHandler_MoveWithContext(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)

	result, err := h.MoveWithContext("u1", world.North)
	require.NoError(t, err)
	assert.Equal(t, "room_a", result.OldRoomID)
	assert.Equal(t, "room_b", result.View.RoomId)
}

func TestWorldHandler_Exits(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)

	exitList, err := h.Exits("u1")
	require.NoError(t, err)
	require.Len(t, exitList.Exits, 1)
	assert.Equal(t, "north", exitList.Exits[0].Direction)
	assert.Equal(t, "room_b", exitList.Exits[0].TargetRoomId)
}

func TestWorldHandler_RoomViewExcludesSelf(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u2",
		Username:          "Bob",
		CharName:          "Bob",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
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
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock, nil, nil)
	return h, worldMgr, sessMgr
}

func TestBuildRoomView_TimeOfDay_HourAndPeriodPopulated(t *testing.T) {
	const startHour int32 = 17 // Dusk
	h, _, sessMgr := testWorldAndSessionWithClock(t, startHour)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
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
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
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
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
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
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
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
	h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)

	view, err := h.Look("u1")
	require.NoError(t, err)
	assert.Equal(t, originalDesc, view.Description, "indoor rooms must have no flavor text appended")
}

// TestBuildRoomView_NpcConditions_NoCombat verifies REQ-T5: when there is no active
// combat the NpcInfo.Conditions slice is empty for every NPC in the room.
//
// Precondition: NPC is alive in room; no active combat.
// Postcondition: NpcInfo.Conditions is nil or empty.
func TestBuildRoomView_NpcConditions_NoCombat(t *testing.T) {
	worldMgr, _ := testWorldAndSession(t)
	combatH := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	// Share the combat handler's session manager so player lookups resolve correctly.
	h := NewWorldHandler(worldMgr, combatH.sessions, combatH.npcMgr, nil, nil, nil)
	h.SetCombatHandler(combatH)

	const roomID = "room_a"
	spawnTestNPC(t, combatH.npcMgr, roomID)
	addTestPlayer(t, combatH.sessions, "u_t5", roomID)

	view, err := h.Look("u_t5")
	require.NoError(t, err)
	require.Len(t, view.Npcs, 1, "expected exactly one NPC in room")
	assert.Empty(t, view.Npcs[0].Conditions, "REQ-T5: NpcInfo.Conditions must be empty when no active combat")
}

// TestBuildRoomView_NpcConditions_WithCombat verifies REQ-T6: when an NPC has active
// conditions in combat, NpcInfo.Conditions contains the condition display names.
//
// Precondition: Active combat with NPC; "grabbed" condition applied to NPC.
// Postcondition: NpcInfo.Conditions contains "Grabbed".
func TestBuildRoomView_NpcConditions_WithCombat(t *testing.T) {
	worldMgr, _ := testWorldAndSession(t)
	combatH := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	// Share the combat handler's session manager so player lookups resolve correctly.
	h := NewWorldHandler(worldMgr, combatH.sessions, combatH.npcMgr, nil, nil, nil)
	h.SetCombatHandler(combatH)

	const roomID = "room_a"
	const uid = "u_t6"
	inst := spawnTestNPC(t, combatH.npcMgr, roomID)
	addTestPlayer(t, combatH.sessions, uid, roomID)

	_, err := combatH.Attack(uid, "Goblin")
	require.NoError(t, err)
	combatH.cancelTimer(roomID)

	err = combatH.ApplyCombatCondition(uid, inst.ID, "grabbed")
	require.NoError(t, err)

	view, err := h.Look(uid)
	require.NoError(t, err)
	require.Len(t, view.Npcs, 1, "expected exactly one NPC in room")
	assert.Contains(t, view.Npcs[0].Conditions, "Grabbed", "REQ-T6: NpcInfo.Conditions must contain 'Grabbed' when condition is active")
}

func TestProperty_BuildRoomView_HourAlwaysInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		startHour := rapid.Int32Range(0, 23).Draw(rt, "startHour")
		worldMgr, sessMgr := testWorldAndSession(t)
		clock := NewGameClock(startHour, time.Hour*24)
		h := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), clock, nil, nil)

		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:               "u1",
			Username:          "Alice",
			CharName:          "Alice",
			CharacterID:       0,
			RoomID:            "room_a",
			CurrentHP:         10,
			MaxHP:             0,
			Abilities:         character.AbilityScores{},
			Role:              "player",
			RegionDisplayName: "",
			Class:             "",
			Level:             0,
		})
		require.NoError(rt, err)

		view, err := h.Look("u1")
		require.NoError(rt, err)
		assert.Equal(rt, startHour, view.Hour, "RoomView.Hour must equal the clock's startHour")
	})
}
