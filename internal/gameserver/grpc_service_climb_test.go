package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

func newClimbWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID: "test", Name: "Test", Description: "Test zone.", StartRoom: "room_ground",
		Rooms: map[string]*world.Room{
			"room_ground": {
				ID: "room_ground", ZoneID: "test", Title: "Ground Floor",
				Description: "A ground level area.", Terrain: "wall",
				Exits: []world.Exit{
					{Direction: world.Up, TargetRoom: "room_top", ClimbDC: 15, Height: 20},
					{Direction: world.North, TargetRoom: "room_b"},
				},
				Properties: map[string]string{},
			},
			"room_top": {
				ID: "room_top", ZoneID: "test", Title: "Top",
				Description: "The top of the wall.",
				Exits:       []world.Exit{{Direction: world.Down, TargetRoom: "room_ground"}},
				Properties:  map[string]string{},
			},
			"room_b": {
				ID: "room_b", ZoneID: "test", Title: "Room B", Description: "A nearby room.",
				Exits:      []world.Exit{{Direction: world.South, TargetRoom: "room_ground"}},
				Properties: map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return worldMgr, session.NewManager()
}

func newClimbSvc(t *testing.T, src dice.Source) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := newClimbWorld(t)
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(src, logger)
	npcMgr := npc.NewManager()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// narrativeContent extracts the narrative/content text from a ServerEvent.
func narrativeContent(ev *gamev1.ServerEvent) string {
	if ev == nil {
		return ""
	}
	if m := ev.GetMessage(); m != nil {
		return m.GetContent()
	}
	return ""
}

// TestHandleClimb_NoDirection verifies that climb with empty direction returns usage error.
func TestHandleClimb_NoDirection(t *testing.T) {
	svc, sessMgr := newClimbSvc(t, &fixedDiceSource{val: 10})
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	ev, err := svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: ""})
	require.NoError(t, err)
	assert.Contains(t, narrativeContent(ev), "direction")
}

// TestHandleClimb_NoClimbableExit verifies message when direction has no climbable exit.
func TestHandleClimb_NoClimbableExit(t *testing.T) {
	svc, sessMgr := newClimbSvc(t, &fixedDiceSource{val: 10})
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	ev, err := svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: "south"})
	require.NoError(t, err)
	assert.Contains(t, narrativeContent(ev), "nothing to climb")
}

// TestHandleClimb_Success verifies player moves to destination on high roll.
func TestHandleClimb_Success(t *testing.T) {
	svc, sessMgr := newClimbSvc(t, &fixedDiceSource{val: 18})
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	_, err = svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: "up"})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok)
	assert.Equal(t, "room_top", sess.RoomID)
}

// TestHandleClimb_CritFailure_FallDamage verifies height-based fall damage on critical failure.
func TestHandleClimb_CritFailure_FallDamage(t *testing.T) {
	// val=0: Intn(n) always returns 0, so each die face = 0+1 = 1.
	// d20 roll = 1, DC=15 → 1 < 15-10=5 → CritFailure.
	// height=20 → numDice=2, 2d6 with val=0 → each die=1 → total dmg=2; HP = 10-2 = 8.
	svc, sessMgr := newClimbSvc(t, &fixedDiceSource{val: 0})
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	_, err = svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: "up"})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok)
	// height=20 → 2 dice × 1 each = 2 damage; HP = 10-2 = 8
	assert.Equal(t, 8, sess.CurrentHP)
}

// TestProperty_FallDamage_HeightRange verifies fall damage formula for all heights in [0,100].
func TestProperty_FallDamage_HeightRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		height := rapid.IntRange(0, 100).Draw(rt, "height")
		// dc in [12,30] guarantees roll=1 is CritFailure (1 < dc-10 when dc >= 12).
		dc := rapid.IntRange(12, 30).Draw(rt, "dc")

		// val=0: Intn(n) always returns 0, so each die face = 0+1 = 1.
		// d20 roll = 1. For any DC >= 2, 1 < DC-10 when DC >= 12, and otherwise Failure.
		// To guarantee CritFailure for all dc in [2,30]: dc-10 range is [-8,20].
		// When dc <= 11, roll=1 >= dc-10 (e.g. dc=2: 1 >= -8 → Failure, not CritFailure).
		// We restrict dc to [12,30] in the property to guarantee CritFailure.
		src := &fixedDiceSource{val: 0}
		zone := &world.Zone{
			ID: "t", Name: "T", Description: "Test zone.", StartRoom: "r",
			Rooms: map[string]*world.Room{
				"r": {
					ID: "r", ZoneID: "t", Title: "Start", Description: "A room.",
					Exits:      []world.Exit{{Direction: world.Up, TargetRoom: "r2", ClimbDC: dc, Height: height}},
					Properties: map[string]string{},
				},
				"r2": {ID: "r2", ZoneID: "t", Title: "Top", Description: "Top room.", Properties: map[string]string{}},
			},
		}
		wm, err := world.NewManager([]*world.Zone{zone})
		require.NoError(rt, err)
		sm := session.NewManager()
		logger := zaptest.NewLogger(rt)
		roller := dice.NewLoggedRoller(src, logger)
		npcMgr2 := npc.NewManager()
		svc2 := NewGameServiceServer(
			wm, sm,
			command.DefaultRegistry(),
			NewWorldHandler(wm, sm, npcMgr2, nil, nil, nil),
			NewChatHandler(sm),
			logger,
			nil, roller, nil, npcMgr2, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil,
			nil, nil,
		)
		_, addErr := sm.AddPlayer(session.AddPlayerOptions{
			UID: "u", Username: "u", CharName: "u", Role: "player",
			RoomID: "r", CurrentHP: 1000, MaxHP: 1000,
		})
		require.NoError(rt, addErr)
		_, handleErr := svc2.handleClimb("u", &gamev1.ClimbRequest{Direction: "up"})
		require.NoError(rt, handleErr)
		sess, ok := sm.GetPlayer("u")
		require.True(rt, ok)
		expectedDice := height / 10
		if expectedDice < 1 {
			expectedDice = 1
		}
		// With val=0, each die = Intn(6)+1 = 1; total damage = expectedDice * 1.
		assert.Equal(rt, 1000-(expectedDice*1), sess.CurrentHP)
	})
}
