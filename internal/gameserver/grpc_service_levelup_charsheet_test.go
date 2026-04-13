package gameserver

// REQ-58-2: When a non-combat XP award (room discovery, skill check) causes a level-up,
// GameServiceServer MUST push a CharacterSheetView to the player's entity channel so that
// the web UI Stats tab reflects the pending stat boosts without requiring a relog.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newLevelUpCharSheetWorld creates a minimal two-room world for level-up charsheet tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a world.Manager and session.Manager with room_a → north → room_b.
func newLevelUpCharSheetWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test_lu",
		Name:        "Test LU",
		Description: "Level-up test zone",
		StartRoom:   "room_lu_a",
		Rooms: map[string]*world.Room{
			"room_lu_a": {
				ID:          "room_lu_a",
				ZoneID:      "test_lu",
				Title:       "Room A",
				Description: "Starting room.",
				Exits:       []world.Exit{{Direction: world.North, TargetRoom: "room_lu_b"}},
				Properties:  map[string]string{},
			},
			"room_lu_b": {
				ID:          "room_lu_b",
				ZoneID:      "test_lu",
				Title:       "Room B",
				Description: "Second room.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_lu_a"}},
				Properties:  map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

// newLevelUpCharSheetService builds a minimal GameServiceServer with xpSvc wired,
// ready to award room discovery XP.
//
// Precondition: worldMgr and sessMgr must be non-nil.
// Postcondition: Returns a GameServiceServer with xpSvc set.
func newLevelUpCharSheetService(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager) *GameServiceServer {
	t.Helper()
	logger := zaptest.NewLogger(t)
	wh := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	ch := NewChatHandler(sessMgr)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		wh, ch, logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	svc.SetXPService(xp.NewService(testXPConfig(), &grantXPProgressSaver{}))
	return svc
}

// drainEntityCharSheetViews reads all currently buffered messages from the player's entity
// channel and returns any CharacterSheetView payloads found.
//
// Precondition: sess must have a non-nil Entity.
// Postcondition: Returns all CharacterSheetView events from the buffer.
func drainEntityCharSheetViews(t *testing.T, sess *session.PlayerSession) []*gamev1.CharacterSheetView {
	t.Helper()
	var views []*gamev1.CharacterSheetView
	for {
		select {
		case data := <-sess.Entity.Events():
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				continue
			}
			if csv := evt.GetCharacterSheet(); csv != nil {
				views = append(views, csv)
			}
		default:
			return views
		}
	}
}

// TestHandleMove_RoomDiscoveryLevelUp_PushesCharacterSheetView verifies that when moving
// into an unexplored room causes a level-up (via room discovery XP), a CharacterSheetView
// event is pushed to the player's entity channel.
//
// REQ-58-2: non-combat level-up MUST push CharacterSheetView so pending boosts are visible.
//
// Precondition: Player starts with Experience at level-up threshold minus NewRoomXP (10),
// so one room discovery award crosses the level boundary.
// Postcondition: After handleMove, entity channel contains at least one CharacterSheetView.
func TestHandleMove_RoomDiscoveryLevelUp_PushesCharacterSheetView(t *testing.T) {
	worldMgr, sessMgr := newLevelUpCharSheetWorld(t)
	svc := newLevelUpCharSheetService(t, worldMgr, sessMgr)

	// Level 2 threshold: 2² × 100 = 400. Starting at 390 → one 10-XP room discovery
	// pushes Experience to 400, crossing the level boundary.
	cfg := testXPConfig()
	startXP := xp.XPToLevel(2, cfg.BaseXP) - cfg.Awards.NewRoomXP

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_lu_rd",
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      "room_lu_a",
		CurrentHP:   10,
		MaxHP:       10,
		Level:       1,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Experience = startXP

	// Mark room_lu_a as already explored; room_lu_b is undiscovered.
	sess.ExploredCache["test_lu"] = map[string]bool{"room_lu_a": true}
	sess.AutomapCache["test_lu"] = map[string]bool{"room_lu_a": true}

	// Move north into the unexplored room_lu_b, triggering room discovery XP + level-up.
	evt, moveErr := svc.handleMove("u_lu_rd", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, moveErr)
	require.NotNil(t, evt, "expected a room view response")

	sheets := drainEntityCharSheetViews(t, sess)
	assert.NotEmpty(t, sheets, "CharacterSheetView must be pushed when room discovery causes a level-up")
}

// TestProperty_HandleMove_RoomDiscoveryLevelUp_PushesCharacterSheetView is a property-based
// companion to the table test above.
//
// Property: for any starting level L in [1, 3], moving into an unexplored room that causes
// a level-up MUST always push a CharacterSheetView.
//
// REQ-58-2 (property): CharacterSheetView push is invariant over multiple starting levels.
func TestProperty_HandleMove_RoomDiscoveryLevelUp_PushesCharacterSheetView(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		startLevel := rapid.IntRange(1, 3).Draw(rt, "startLevel")

		worldMgr, sessMgr := newLevelUpCharSheetWorld(t)
		svc := newLevelUpCharSheetService(t, worldMgr, sessMgr)

		cfg := testXPConfig()
		// Start at exactly 1 XP below the next level threshold; one room discovery
		// crosses the boundary.
		startXP := xp.XPToLevel(startLevel+1, cfg.BaseXP) - cfg.Awards.NewRoomXP

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         "u_lu_prop",
			Username:    "testuser",
			CharName:    "Hero",
			CharacterID: int64(startLevel),
			RoomID:      "room_lu_a",
			CurrentHP:   10,
			MaxHP:       10,
			Level:       startLevel,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		require.NoError(rt, err)
		sess.Experience = startXP

		sess.ExploredCache["test_lu"] = map[string]bool{"room_lu_a": true}
		sess.AutomapCache["test_lu"] = map[string]bool{"room_lu_a": true}

		_, moveErr := svc.handleMove("u_lu_prop", &gamev1.MoveRequest{Direction: "north"})
		require.NoError(rt, moveErr)

		sheets := drainEntityCharSheetViews(t, sess)
		assert.NotEmpty(rt, sheets, "CharacterSheetView must be pushed when room discovery causes a level-up (startLevel=%d)", startLevel)
	})
}
