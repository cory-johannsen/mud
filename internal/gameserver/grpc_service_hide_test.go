package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newHideSvc builds a minimal GameServiceServer for handleHide tests.
func newHideSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
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
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
	)
	return svc, sessMgr
}

// newHideSvcWithCombat builds a GameServiceServer with real combat state and condition registry.
func newHideSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeTestConditionRegistry()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleHide_NoSession verifies handleHide returns error when session is missing.
func TestHandleHide_NoSession(t *testing.T) {
	svc, _ := newHideSvc(t, nil, nil, nil)
	event, err := svc.handleHide("unknown_hide_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleHide_NotInCombat verifies handleHide returns error event when not in combat.
func TestHandleHide_NotInCombat(t *testing.T) {
	svc, sessMgr := newHideSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_hide_nc", Username: "Rogue", CharName: "Rogue", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleHide("u_hide_nc")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleHide_SpendAPFail verifies handleHide returns error event when AP is insufficient.
func TestHandleHide_SpendAPFail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newHideSvcWithCombat(t, roller)

	const roomID = "room_hide_ap"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-hide-ap", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_hide_ap", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_hide_ap", "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Drain all AP.
	for combatHandler.RemainingAP("u_hide_ap") > 0 {
		_ = combatHandler.SpendAP("u_hide_ap", 1)
	}

	event, err := svc.handleHide("u_hide_ap")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
}

// TestHandleHide_RollBelow_Failure verifies that a low roll does not apply hidden.
func TestHandleHide_RollBelow_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // roll=1, bonus=0, total=1 < DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newHideSvcWithCombat(t, roller)

	const roomID = "room_hide_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "scout-hide-rb", Name: "Scout", Level: 1, MaxHP: 20, AC: 13, Awareness: 12,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_hide_rb", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_hide_rb", "Scout")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleHide("u_hide_rb")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "failure")

	// hidden condition must NOT be present.
	assert.False(t, sess.Conditions.Has("hidden"), "hidden condition must not be applied on failure")

	// combatant Hidden must be false.
	combatant, ok := combatHandler.GetCombatant("u_hide_rb", "u_hide_rb")
	require.True(t, ok)
	assert.False(t, combatant.Hidden)
}

// TestHandleHide_RollAbove_Success verifies that a high roll applies hidden.
func TestHandleHide_RollAbove_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20, bonus=0, total=20 >= DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newHideSvcWithCombat(t, roller)

	const roomID = "room_hide_ra"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "patrol-hide-ra", Name: "Patrol", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_hide_ra", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_hide_ra", "Patrol")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleHide("u_hide_ra")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "success")

	// hidden condition must be present.
	assert.True(t, sess.Conditions.Has("hidden"), "hidden condition must be applied on success")

	// combatant Hidden must be true.
	combatant, ok := combatHandler.GetCombatant("u_hide_ra", "u_hide_ra")
	require.True(t, ok)
	assert.True(t, combatant.Hidden)
}
