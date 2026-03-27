package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newDifficultTerrainWorld creates a world where room_b has a terrain_mud Effect
// (MoveAPCost > 0) and room_a connects to it via north.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a world.Manager, session.Manager, and condition.Registry with terrain_mud registered.
func newDifficultTerrainWorld(t *testing.T) (*world.Manager, *session.Manager, *condition.Registry) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID:           "terrain_mud",
		Name:         "Mud",
		DurationType: "permanent",
		MoveAPCost:   1,
	})
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
				Exits:       []world.Exit{{Direction: world.North, TargetRoom: "room_b"}},
				Properties:  map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "The second room.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_a"}},
				Effects:     []world.RoomEffect{{Track: "terrain_mud"}},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager(), reg
}

// newNormalTerrainWorld creates a world where room_b has no terrain property.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a world.Manager and session.Manager with a two-room zone.
func newNormalTerrainWorld(t *testing.T) (*world.Manager, *session.Manager) {
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
				Exits:       []world.Exit{{Direction: world.North, TargetRoom: "room_b"}},
				Properties:  map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "The second room.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_a"}},
				Properties:  map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

// newMoveTestService builds a minimal GameServiceServer using the given world and session managers.
//
// Precondition: worldMgr and sessMgr must be non-nil.
// Postcondition: Returns a GameServiceServer ready for handleMove tests.
func newMoveTestService(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager, condReg *condition.Registry) *GameServiceServer {
	t.Helper()
	logger := zaptest.NewLogger(t)
	wh := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	ch := NewChatHandler(sessMgr)
	return newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		wh, ch, logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
}

// drainEntityMessages reads all currently buffered messages from the player's entity channel
// and returns their decoded MessageEvent contents.
//
// Precondition: sess must have a non-nil Entity.
// Postcondition: Returns all content strings from MessageEvent payloads in the buffer.
func drainEntityMessages(t *testing.T, sess *session.PlayerSession) []string {
	t.Helper()
	var contents []string
	for {
		select {
		case data := <-sess.Entity.Events():
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				continue
			}
			if msg := evt.GetMessage(); msg != nil {
				contents = append(contents, msg.Content)
			}
		default:
			return contents
		}
	}
}

// TestHandleMove_DifficultTerrain_MessageSentWithoutFeat verifies that moving into a difficult
// terrain room without zone_awareness pushes a flavor message to the player's entity.
//
// Precondition: Destination room has Properties["terrain"]="difficult"; player lacks zone_awareness.
// Postcondition: Player's entity channel contains a message about difficult terrain.
func TestHandleMove_DifficultTerrain_MessageSentWithoutFeat(t *testing.T) {
	worldMgr, sessMgr, reg := newDifficultTerrainWorld(t)
	svc := newMoveTestService(t, worldMgr, sessMgr, reg)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_difficult",
		Username:    "Tester",
		CharName:    "Tester",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	// PassiveFeats is nil by default (no zone_awareness).
	sess.PassiveFeats = map[string]bool{}

	evt, err := svc.handleMove("u_difficult", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "room_b", evt.GetRoomView().RoomId)

	msgs := drainEntityMessages(t, sess)
	require.NotEmpty(t, msgs, "expected a terrain message pushed to entity")
	found := false
	for _, m := range msgs {
		if m != "" && containsSubstring(m, "mud") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one message mentioning 'mud', got: %v", msgs)
}

// TestHandleMove_DifficultTerrain_NoMessageWithFeat verifies that moving into a difficult
// terrain room with zone_awareness does NOT push any flavor message.
//
// Precondition: Destination room has Properties["terrain"]="difficult"; player has zone_awareness=true.
// Postcondition: Player's entity channel contains no difficult terrain message.
func TestHandleMove_DifficultTerrain_NoMessageWithFeat(t *testing.T) {
	worldMgr, sessMgr, reg := newDifficultTerrainWorld(t)
	svc := newMoveTestService(t, worldMgr, sessMgr, reg)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_aware",
		Username:    "Aware",
		CharName:    "Aware",
		CharacterID: 2,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.PassiveFeats = map[string]bool{"zone_awareness": true}

	evt, err := svc.handleMove("u_aware", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "room_b", evt.GetRoomView().RoomId)

	msgs := drainEntityMessages(t, sess)
	for _, m := range msgs {
		assert.NotContains(t, m, "mud",
			"zone_awareness player should not receive a terrain message")
	}
}

// TestHandleMove_NoDifficultTerrain_NoMessage verifies that moving into a room with no terrain
// property does NOT push a difficult terrain message regardless of feat status.
//
// Precondition: Destination room has no "terrain" property.
// Postcondition: Player's entity channel contains no difficult terrain message.
func TestHandleMove_NoDifficultTerrain_NoMessage(t *testing.T) {
	worldMgr, sessMgr := newNormalTerrainWorld(t)
	svc := newMoveTestService(t, worldMgr, sessMgr, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_normal",
		Username:    "Normal",
		CharName:    "Normal",
		CharacterID: 3,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.PassiveFeats = map[string]bool{}

	evt, err := svc.handleMove("u_normal", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "room_b", evt.GetRoomView().RoomId)

	msgs := drainEntityMessages(t, sess)
	for _, m := range msgs {
		assert.NotContains(t, m, "terrain",
			"normal terrain room should not produce a terrain message")
	}
}

// TestPropertyHandleMove_ZoneAwareness_NeverReceivesTerrainMessage is a property test verifying
// that any player with zone_awareness never receives a difficult terrain message.
//
// Precondition: Room has terrain=difficult; player has zone_awareness=true.
// Postcondition: For all valid moves, no difficult terrain message is pushed to the player.
func TestPropertyHandleMove_ZoneAwareness_NeverReceivesTerrainMessage(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		worldMgr, sessMgr, reg := newDifficultTerrainWorld(t)
		svc := newMoveTestService(t, worldMgr, sessMgr, reg)

		uid := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid")

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "PropUser",
			CharName:    "PropUser",
			CharacterID: 99,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       10,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		require.NoError(rt, err)
		sess.PassiveFeats = map[string]bool{"zone_awareness": true}

		_, _ = svc.handleMove(uid, &gamev1.MoveRequest{Direction: "north"})

		msgs := drainEntityMessages(t, sess)
		for _, m := range msgs {
			assert.NotContains(rt, m, "mud",
				"zone_awareness player should never receive terrain message")
		}
	})
}

// containsSubstring is a helper that checks if s contains substr (case-sensitive).
//
// Precondition: none.
// Postcondition: Returns true if substr appears within s.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestHandleMove_BlockedWhileInCombat verifies that a player in combat cannot change rooms.
//
// Precondition: Player's Status is statusInCombat.
// Postcondition: handleMove returns a non-nil event with an error/message and the player's
// room is unchanged.
func TestHandleMove_BlockedWhileInCombat(t *testing.T) {
	worldMgr, sessMgr := newNormalTerrainWorld(t)
	svc := newMoveTestService(t, worldMgr, sessMgr, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_combat",
		Username:    "Fighter",
		CharName:    "Fighter",
		CharacterID: 10,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	evt, err := svc.handleMove("u_combat", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Must not be a RoomView — player should not have moved.
	assert.Nil(t, evt.GetRoomView(), "player in combat must not receive a RoomView on move")

	// Player's room must be unchanged.
	updatedSess, ok := sessMgr.GetPlayer("u_combat")
	require.True(t, ok)
	assert.Equal(t, "room_a", updatedSess.RoomID, "player room must not change while in combat")
}

// TestPropertyHandleMove_InCombat_AlwaysBlocked is a property test verifying that
// a player with statusInCombat can never move to another room.
//
// Precondition: Player's Status is statusInCombat.
// Postcondition: For all valid move attempts, the player's room is never changed.
func TestPropertyHandleMove_InCombat_AlwaysBlocked(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		worldMgr, sessMgr := newNormalTerrainWorld(t)
		svc := newMoveTestService(t, worldMgr, sessMgr, nil)

		uid := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid")

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "CombatProp",
			CharName:    "CombatProp",
			CharacterID: 99,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       10,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat

		evt, err := svc.handleMove(uid, &gamev1.MoveRequest{Direction: "north"})
		require.NoError(rt, err)
		require.NotNil(rt, evt)

		assert.Nil(rt, evt.GetRoomView(), "in-combat player must never receive a RoomView")

		updatedSess, ok := sessMgr.GetPlayer(uid)
		require.True(rt, ok)
		assert.Equal(rt, "room_a", updatedSess.RoomID, "in-combat player room must not change")
	})
}
