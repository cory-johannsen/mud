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
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
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
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
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

// TestHandleStep_TowardDecreasesDistanceBy5 verifies that stepping toward decreases
// the Chebyshev distance to the NPC by one grid cell (5ft).
//
// Precondition: player in combat at GridX=0, GridY=0; NPC at GridX=5, GridY=9; direction=="toward".
// Postcondition: player GridX increases by 1 (moves toward NPC column), distance decreases.
func TestHandleStep_TowardDecreasesDistanceBy5(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_tr"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-tr", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
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
	// Attack initializes player to GridX=0, GridY=0 and NPC to GridX=5, GridY=9.
	playerCbt.GridX = 0
	playerCbt.GridY = 0

	var npcCbt *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			npcCbt = c
			break
		}
	}
	require.NotNil(t, npcCbt)
	npcCbt.GridX = 5
	npcCbt.GridY = 9

	distBefore := combat.CombatRange(*playerCbt, *npcCbt)

	event, err := svc.handleStep("u_step_tr", &gamev1.StepRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "toward")

	distAfter := combat.CombatRange(*playerCbt, *npcCbt)
	assert.Less(t, distAfter, distBefore, "player should be closer to NPC after stepping toward")
	assert.Equal(t, distBefore-5, distAfter, "distance should decrease by exactly 5ft (1 grid cell)")
}

// TestHandleStep_AwayIncreasesDistanceBy5 verifies that stepping away increases
// the Chebyshev distance to the NPC by one grid cell (5ft).
//
// Precondition: player in combat at GridX=4, GridY=0; NPC at GridX=5, GridY=0; direction=="away".
// Postcondition: player GridX decreases by 1 (moves away from NPC), distance increases.
func TestHandleStep_AwayIncreasesDistanceBy5(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_ai"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-ai", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
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
	playerCbt.GridX = 4
	playerCbt.GridY = 0

	var npcCbt *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			npcCbt = c
			break
		}
	}
	require.NotNil(t, npcCbt)
	npcCbt.GridX = 5
	npcCbt.GridY = 0

	distBefore := combat.CombatRange(*playerCbt, *npcCbt)

	event, err := svc.handleStep("u_step_ai", &gamev1.StepRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "away")

	distAfter := combat.CombatRange(*playerCbt, *npcCbt)
	assert.Greater(t, distAfter, distBefore, "player should be farther from NPC after stepping away")
	assert.Equal(t, distBefore+5, distAfter, "distance should increase by exactly 5ft (1 grid cell)")
}

// TestHandleStep_AwayClampedAtGridBoundary verifies that stepping away when already
// at the grid boundary (GridX=0) does not move the player off the grid.
//
// Precondition: player at GridX=0, GridY=0; NPC at GridX=5, GridY=0; direction=="away".
// Postcondition: player GridX remains 0 (clamped at boundary).
func TestHandleStep_AwayClampedAtGridBoundary(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_fz"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-fz", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
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
	playerCbt.GridX = 0
	playerCbt.GridY = 0

	var npcCbt *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			npcCbt = c
			break
		}
	}
	require.NotNil(t, npcCbt)
	npcCbt.GridX = 5
	npcCbt.GridY = 0

	event, err := svc.handleStep("u_step_fz", &gamev1.StepRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	assert.Equal(t, 0, playerCbt.GridX, "player GridX should remain 0 when already at left boundary")
}

// TestHandleStep_NoReactiveStrikes verifies that stepping does NOT produce any
// reactive strike narrative in the response, even when adjacent to an NPC.
//
// Precondition: player adjacent to NPC (Chebyshev dist == 5ft); direction=="away".
// Postcondition: response message does NOT contain "reactive strike".
func TestHandleStep_NoReactiveStrikes(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_rs"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-rs", Name: "Goblin", Level: 1, MaxHP: 20, AC: 10, Awareness: 2,
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

	// Place player at GridX=1, GridY=0; NPC at GridX=2, GridY=0 (adjacent, dist=5ft).
	playerCbt := cbt.GetCombatant("u_step_rs")
	require.NotNil(t, playerCbt)
	playerCbt.GridX = 1
	playerCbt.GridY = 0

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.GridX = 2
			c.GridY = 0
			break
		}
	}

	// Step away: player moves from GridX=1 to GridX=0. Even though adjacent, NO reactive strike fires.
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
// Precondition: player at GridX=0, GridY=0; NPC at GridX=5, GridY=0 (25ft); direction=="toward".
// Postcondition: message contains "Distance to target" and the updated distance.
func TestHandleStep_MessageContainsDistance(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStepSvcWithCombat(t)

	const roomID = "room_step_dist"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-step-dist", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
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
	playerCbt.GridX = 0
	playerCbt.GridY = 0

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			// Place NPC 5 cells away on same row: dist = 5*5 = 25ft.
			c.GridX = 5
			c.GridY = 0
			break
		}
	}

	event, err := svc.handleStep("u_step_dist", &gamev1.StepRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	// After stepping toward from GridX=0, player moves to GridX=1; NPC at GridX=5; dist = 4*5 = 20ft.
	assert.Contains(t, msgEvt.Content, "Distance to target")
	assert.Contains(t, msgEvt.Content, "20 ft")
}
