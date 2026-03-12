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

// newTumbleSvc builds a minimal GameServiceServer for handleTumble tests.
// npcMgr may be nil; combatHandler may be nil.
func newTumbleSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// newTumbleSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newTumbleSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleTumble_NoSession verifies that handleTumble returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_tbl_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleTumble_NoSession(t *testing.T) {
	svc, _ := newTumbleSvc(t, nil, nil, nil)
	event, err := svc.handleTumble("unknown_tbl_uid", &gamev1.TumbleRequest{Target: "bandit"})
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleTumble_NotInCombat verifies that handleTumble returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleTumble_NotInCombat(t *testing.T) {
	svc, sessMgr := newTumbleSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_tbl_nc",
		Username: "Rogue",
		CharName: "Rogue",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleTumble("u_tbl_nc", &gamev1.TumbleRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleTumble_EmptyTarget verifies that handleTumble returns an error event
// when no target is specified.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage: tumble".
func TestHandleTumble_EmptyTarget(t *testing.T) {
	svc, sessMgr := newTumbleSvc(t, nil, nil, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_tbl_et",
		Username: "Rogue",
		CharName: "Rogue",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleTumble("u_tbl_et", &gamev1.TumbleRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage: tumble")
}

// TestHandleTumble_TargetNotFound verifies that handleTumble returns an error event
// when the named target NPC is not in the player's room, and that AP is NOT spent.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "not found"; AP is not decremented.
func TestHandleTumble_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newTumbleSvcWithCombat(t, roller)

	const roomID = "room_tbl_tnf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-tbl-tnf", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_tbl_tnf", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_tbl_tnf", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_tbl_tnf")

	event, err := svc.handleTumble("u_tbl_tnf", &gamev1.TumbleRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "not found")

	apAfter := combatHandler.RemainingAP("u_tbl_tnf")
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when target is not found")
}

// TestHandleTumble_RollAboveDC_Success verifies that handleTumble returns a success message
// and the player's position increases by 5 ft when the acrobatics roll meets or exceeds Level+10 DC.
//
// Precondition: player in combat; NPC Level=1 → DC=11; dice returns 19 (roll=20, bonus=0, total=20 >= 11).
// Postcondition: message event containing "tumble through"; player position increased by 5.
func TestHandleTumble_RollAboveDC_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newTumbleSvcWithCombat(t, roller)

	const roomID = "room_tbl_ra"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-tbl-ra", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)

	sessRA, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_tbl_ra", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRA.Status = statusInCombat

	_, err = combatHandler.Attack("u_tbl_ra", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	combatant := cbt.GetCombatant("u_tbl_ra")
	require.NotNil(t, combatant)
	posBefore := combatant.Position

	event, err := svc.handleTumble("u_tbl_ra", &gamev1.TumbleRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful tumble")
	assert.Contains(t, msgEvt.Content, "tumble through")

	posAfter := combatant.Position
	assert.Equal(t, posBefore+5, posAfter, "player position must increase by 5 on success")
}

// TestHandleTumble_RollBelowDC_Failure verifies that handleTumble returns a failure message
// and the player's position does NOT change when the acrobatics roll is below Level+10 DC.
//
// Precondition: player in combat; NPC Level=5 → DC=15; dice returns 0 (roll=1, bonus=0, total=1 < 15).
// Postcondition: message event containing "reactive strike"; player position unchanged.
func TestHandleTumble_RollBelowDC_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newTumbleSvcWithCombat(t, roller)

	const roomID = "room_tbl_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-tbl-rb", Name: "Bandit", Level: 5, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)

	sessRB, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_tbl_rb", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRB.Status = statusInCombat

	_, err = combatHandler.Attack("u_tbl_rb", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	combatant := cbt.GetCombatant("u_tbl_rb")
	require.NotNil(t, combatant)
	posBefore := combatant.Position

	event, err := svc.handleTumble("u_tbl_rb", &gamev1.TumbleRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed tumble")
	assert.Contains(t, msgEvt.Content, "reactive strike")

	posAfter := combatant.Position
	assert.Equal(t, posBefore, posAfter, "player position must not change on failure")
}
