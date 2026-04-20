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

// newDivertSvc builds a minimal GameServiceServer for handleDivert tests.
func newDivertSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := newTestGameServiceServer(
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
		nil, nil,
	)
	return svc, sessMgr
}

// newDivertSvcWithCombat builds a GameServiceServer with real combat state and condition registry.
func newDivertSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeTestConditionRegistry()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
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
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleDivert_NoSession verifies handleDivert returns error when session is missing.
func TestHandleDivert_NoSession(t *testing.T) {
	svc, _ := newDivertSvc(t, nil, nil, nil)
	event, err := svc.handleDivert("unknown_divert_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleDivert_NotInCombat verifies handleDivert returns error event when not in combat.
func TestHandleDivert_NotInCombat(t *testing.T) {
	svc, sessMgr := newDivertSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_div_nc", Username: "Rogue", CharName: "Rogue", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleDivert("u_div_nc")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleDivert_SpendAPFail verifies handleDivert returns error event when AP is insufficient.
func TestHandleDivert_SpendAPFail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDivertSvcWithCombat(t, roller)

	const roomID = "room_div_ap"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "thug-div-ap", Name: "Thug", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_div_ap", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_div_ap", "Thug")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Drain all AP.
	for combatHandler.RemainingAP("u_div_ap") > 0 {
		_ = combatHandler.SpendAP("u_div_ap", 1)
	}

	event, err := svc.handleDivert("u_div_ap")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
}

// TestHandleDivert_RollBelow_Failure verifies that a low roll does not apply hidden.
func TestHandleDivert_RollBelow_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // roll=1, bonus=0, total=1 < DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDivertSvcWithCombat(t, roller)

	const roomID = "room_div_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "lookout-div-rb", Name: "Lookout", Level: 1, MaxHP: 20, AC: 13, Awareness: 12,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_div_rb", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_div_rb", "Lookout")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleDivert("u_div_rb")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "failure")

	// hidden condition must NOT be present.
	assert.False(t, sess.Conditions.Has("hidden"), "hidden condition must not be applied on failure")
}

// TestHandleDivert_RollAbove_Success verifies that a high roll applies hidden.
func TestHandleDivert_RollAbove_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20, bonus=0, total=20 >= DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDivertSvcWithCombat(t, roller)

	const roomID = "room_div_ra"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "sentry-div-ra", Name: "Sentry", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_div_ra", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_div_ra", "Sentry")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleDivert("u_div_ra")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "success")

	// hidden condition must be present.
	assert.True(t, sess.Conditions.Has("hidden"), "hidden condition must be applied on success")

	// combatant Hidden must be true.
	combatant, ok := combatHandler.GetCombatant("u_div_ra", "u_div_ra")
	require.True(t, ok)
	assert.True(t, combatant.Hidden)
}
