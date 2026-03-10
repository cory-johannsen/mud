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

// newStepSvcWithCombat builds a GameServiceServer and associated helpers suitable
// for tests that need real in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newStepSvcWithCombat(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
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

// newStepSvc builds a minimal GameServiceServer for handleStep tests without combat.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc and sessMgr.
func newStepSvc(t *testing.T, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
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

// TestHandleStep_NotInCombat verifies that handleStep returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleStep_NotInCombat(t *testing.T) {
	svc, sessMgr := newStepSvc(t, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_step_nc",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_step_nc",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleStep("u_step_nc", &gamev1.StepRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleStep_TowardIncreasesPositionBy5 verifies that stepping toward increases
// the player's Position by 5.
//
// Precondition: player in combat at Position=0; direction=="toward".
// Postcondition: player.Position becomes 5.
func TestHandleStep_TowardIncreasesPositionBy5(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_tr"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-tr", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_step_tr", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_step_tr", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant("u_step_tr")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 0

	event, err := svc.handleStep("u_step_tr", &gamev1.StepRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "toward")

	assert.Equal(t, 5, playerCbt.Position, "player.Position should be 5 after stepping toward")
}

// TestHandleStep_AwayDecreasesPositionBy5 verifies that stepping away decreases
// the player's Position by 5.
//
// Precondition: player in combat at Position=25; direction=="away".
// Postcondition: player.Position becomes 20.
func TestHandleStep_AwayDecreasesPositionBy5(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_ai"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-ai", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_step_ai", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_step_ai", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant("u_step_ai")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 25

	event, err := svc.handleStep("u_step_ai", &gamev1.StepRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "away")

	assert.Equal(t, 20, playerCbt.Position, "player.Position should be 20 after stepping away from 25")
}

// TestHandleStep_AwayFlooredAtZero verifies that stepping away from Position=0
// does not go below zero.
//
// Precondition: player at Position=0; direction=="away".
// Postcondition: player.Position remains 0.
func TestHandleStep_AwayFlooredAtZero(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_fz"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-fz", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_step_fz", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_step_fz", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant("u_step_fz")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 0

	event, err := svc.handleStep("u_step_fz", &gamev1.StepRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	assert.Equal(t, 0, playerCbt.Position, "player.Position should remain 0 when already at floor")
}

// TestHandleStep_NoReactiveStrikes verifies that stepping does NOT produce any
// reactive strike narrative in the response, even when adjacent to an NPC.
//
// Precondition: player adjacent to NPC (dist <= 5); direction=="away".
// Postcondition: response message does NOT contain "reactive strike".
func TestHandleStep_NoReactiveStrikes(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_rs"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-rs", Name: "Goblin", Level: 1, MaxHP: 20, AC: 10, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_step_rs", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 30, MaxHP: 30, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_step_rs", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	// Place player at Position=5, NPC at Position=0 (adjacent, dist=5 <= 5).
	playerCbt := cbt.GetCombatant("u_step_rs")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 5

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.Position = 0
			break
		}
	}

	// Step away: player moves from 5 to 0. Even though adjacent, NO reactive strike fires.
	event, err := svc.handleStep("u_step_rs", &gamev1.StepRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.NotContains(t, msgEvt.Content, "reactive strike",
		"Step must never trigger reactive strikes")
}

// TestHandleStep_MessageContainsDistance verifies that the response message includes
// the distance to target after the step.
//
// Precondition: player at Position=0, NPC at Position=25; direction=="toward".
// Postcondition: message contains distance text.
func TestHandleStep_MessageContainsDistance(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_dist"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-dist", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_step_dist", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_step_dist", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant("u_step_dist")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 0

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.Position = 25
			break
		}
	}

	event, err := svc.handleStep("u_step_dist", &gamev1.StepRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	// After stepping toward from 0, player.Position = 5; NPC at 25; dist = 20.
	assert.Contains(t, msgEvt.Content, "Distance to target")
	assert.Contains(t, msgEvt.Content, "20 ft")
}
