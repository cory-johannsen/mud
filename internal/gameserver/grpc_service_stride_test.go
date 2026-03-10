package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newStrideSvc builds a minimal GameServiceServer for handleStride tests without combat.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc and sessMgr.
func newStrideSvc(t *testing.T, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr
}

// newStrideSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newStrideSvcWithCombat(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	src := &fixedDiceSource{val: 10}
	roller := dice.NewLoggedRoller(src, logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleStride_NotInCombat verifies that handleStride returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleStride_NotInCombat(t *testing.T) {
	svc, sessMgr := newStrideSvc(t, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_str_nc",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_str_nc",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleStride("u_str_nc", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleStride_TowardReducesDistance verifies that striding toward reduces distance by 25,
// and the result is clamped at minimum 5.
//
// Precondition: player in combat at distance 25; direction == "toward".
// Postcondition: distance becomes 5 (25-25=0, clamped to 5).
func TestHandleStride_TowardReducesDistance(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_tr"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-tr", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_tr", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_tr", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	cbt.SetDistance(25)

	event, err := svc.handleStride("u_str_tr", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "toward")

	assert.Equal(t, 5, cbt.Distance, "distance should be clamped to 5 (25-25=0, min=5)")
}

// TestHandleStride_AwayIncreasesDistance verifies that striding away increases distance by 25.
//
// Precondition: player in combat at distance 25; direction == "away".
// Postcondition: distance becomes 50.
func TestHandleStride_AwayIncreasesDistance(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_ai"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-ai", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_ai", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_ai", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	cbt.SetDistance(25)

	event, err := svc.handleStride("u_str_ai", &gamev1.StrideRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "away")

	assert.Equal(t, 50, cbt.Distance, "distance should be 50 after striding away from 25")
}

// TestHandleStride_ClampAtMinimum verifies that distance is clamped at 5 when already at minimum.
//
// Precondition: player in combat at distance 5; direction == "toward".
// Postcondition: distance remains 5.
func TestHandleStride_ClampAtMinimum(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_cm"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-cm", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_cm", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_cm", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	cbt.SetDistance(5)

	event, err := svc.handleStride("u_str_cm", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	assert.Equal(t, 5, cbt.Distance, "distance should remain 5 when already at minimum")
}

// TestHandleStride_ClampAtMaximum verifies that distance is clamped at 100 when already at maximum.
//
// Precondition: player in combat at distance 100; direction == "away".
// Postcondition: distance remains 100.
func TestHandleStride_ClampAtMaximum(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_cx"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-cx", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_cx", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_cx", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	cbt.SetDistance(100)

	event, err := svc.handleStride("u_str_cx", &gamev1.StrideRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	assert.Equal(t, 100, cbt.Distance, "distance should remain 100 when already at maximum")
}
