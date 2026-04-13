package gameserver

// REQ-57-1: tickNPCIdle MUST NOT allow a hostile NPC to initiate combat
// in a room whose effective danger_level is "safe".

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newSafeRoomWorld creates a minimal two-room world:
//   - room_safe  : danger_level "safe"
//   - room_danger: danger_level "dangerous"
//
// Both are in a zone with danger_level "sketchy".
func newSafeRoomWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test_sr",
		Name:        "Test Safe Room",
		Description: "Zone for safe room tests",
		DangerLevel: "sketchy",
		StartRoom:   "room_safe",
		Rooms: map[string]*world.Room{
			"room_safe": {
				ID:          "room_safe",
				ZoneID:      "test_sr",
				Title:       "Safe Room",
				Description: "Should never allow NPC-initiated combat.",
				DangerLevel: "safe",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_danger"}},
				Properties:  map[string]string{},
			},
			"room_danger": {
				ID:          "room_danger",
				ZoneID:      "test_sr",
				Title:       "Dangerous Room",
				Description: "NPCs may initiate combat here.",
				DangerLevel: "dangerous",
				Exits:       []world.Exit{{Direction: world.North, TargetRoom: "room_safe"}},
				Properties:  map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

// makeSafeRoomCombatHandler creates a CombatHandler that shares the given sessMgr,
// so players registered there are visible to the combat handler.
func makeSafeRoomCombatHandler(t *testing.T, sessMgr *session.Manager) *CombatHandler {
	t.Helper()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), zap.NewNop())
	return NewCombatHandler(
		combat.NewEngine(), npc.NewManager(), sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		200*time.Millisecond, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)
}

// newSafeRoomService builds a GameServiceServer for safe-room NPC tests.
// It uses the provided sessMgr so the caller can share it with the combat handler.
func newSafeRoomService(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager) *GameServiceServer {
	t.Helper()
	logger := zaptest.NewLogger(t)
	npcManager := npc.NewManager()
	wh := NewWorldHandler(worldMgr, sessMgr, npcManager, nil, nil, nil)
	ch := NewChatHandler(sessMgr)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		wh, ch, logger,
		nil, nil, nil, npcManager, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc
}

// makeHostileNPCInRoom creates a hostile combat NPC instance in the given room.
// The NPC is created directly (not via npcMgr.Spawn) so no manager is needed.
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns a non-nil *npc.Instance with Disposition=="hostile", RoomID==roomID.
func makeHostileNPCInRoom(roomID string, id string, courage int) *npc.Instance {
	return &npc.Instance{
		ID:               id,
		TemplateID:       id,
		RoomID:           roomID,
		NPCType:          "combat",
		Disposition:      "hostile",
		CourageThreshold: courage,
		Level:            2,
		MaxHP:            20,
		CurrentHP:        20,
	}
}

// TestTickNPCIdle_SafeRoom_HostileNPCDoesNotInitiateCombat verifies that a hostile
// combat NPC whose room has danger_level "safe" does NOT initiate combat against a
// player, even when the NPC would otherwise engage (courage threshold exceeded).
//
// REQ-57-1: NPCs MUST NOT initiate combat in rooms with effective danger_level "safe".
//
// Precondition: hostile NPC (courage=100) and player are both in room_safe.
// Postcondition: no combat is active in room_safe after tickNPCIdle.
func TestTickNPCIdle_SafeRoom_HostileNPCDoesNotInitiateCombat(t *testing.T) {
	worldMgr, sessMgr := newSafeRoomWorld(t)
	svc := newSafeRoomService(t, worldMgr, sessMgr)
	svc.combatH = makeSafeRoomCombatHandler(t, sessMgr)

	inst := makeHostileNPCInRoom("room_safe", "sr_grunt", 100)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_sr_player",
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      "room_safe",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	svc.tickNPCIdle(inst, "test_sr", nil)

	assert.False(t, svc.combatH.IsInCombat(inst.ID),
		"hostile NPC must NOT initiate combat in a safe room")
}

// TestTickNPCIdle_DangerousRoom_HostileNPCInitiatesCombat is the regression companion:
// a hostile NPC in a dangerous room MUST initiate combat as before.
//
// Precondition: hostile NPC (courage=100) and player are in room_danger.
// Postcondition: combat is active for NPC after tickNPCIdle.
func TestTickNPCIdle_DangerousRoom_HostileNPCInitiatesCombat(t *testing.T) {
	worldMgr, sessMgr := newSafeRoomWorld(t)
	svc := newSafeRoomService(t, worldMgr, sessMgr)
	svc.combatH = makeSafeRoomCombatHandler(t, sessMgr)

	inst := makeHostileNPCInRoom("room_danger", "dr_grunt", 100)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_dr_player",
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 2,
		RoomID:      "room_danger",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	svc.tickNPCIdle(inst, "test_sr", nil)

	assert.True(t, svc.combatH.IsInCombat(inst.ID),
		"hostile NPC MUST initiate combat in a dangerous room")
}

// TestProperty_TickNPCIdle_SafeRoom_NeverInitiatesCombat verifies that for any
// hostile NPC courage level [0, 100], safe rooms always prevent NPC combat initiation.
//
// REQ-57-1 (property): safe-room protection is invariant over NPC courage levels.
func TestProperty_TickNPCIdle_SafeRoom_NeverInitiatesCombat(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		courage := rapid.IntRange(0, 100).Draw(rt, "courage")

		worldMgr, sessMgr := newSafeRoomWorld(t)
		svc := newSafeRoomService(t, worldMgr, sessMgr)
		svc.combatH = makeSafeRoomCombatHandler(t, sessMgr)

		inst := makeHostileNPCInRoom("room_safe", "prop_grunt", courage)

		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         "u_prop_safe",
			Username:    "testuser",
			CharName:    "Hero",
			CharacterID: 3,
			RoomID:      "room_safe",
			CurrentHP:   10,
			MaxHP:       10,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		require.NoError(rt, err)

		svc.tickNPCIdle(inst, "test_sr", nil)

		assert.False(rt, svc.combatH.IsInCombat(inst.ID),
			"hostile NPC (courage=%d) must NOT initiate combat in a safe room", courage)
	})
}
