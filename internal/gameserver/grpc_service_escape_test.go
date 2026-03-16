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

// newEscapeSvc builds a minimal GameServiceServer for handleEscape tests.
func newEscapeSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
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
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// newEscapeSvcWithCombat builds a GameServiceServer with real combat state and condition registry.
func newEscapeSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
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
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleEscape_NoSession verifies handleEscape returns error when session is missing.
func TestHandleEscape_NoSession(t *testing.T) {
	svc, _ := newEscapeSvc(t, nil, nil, nil)
	event, err := svc.handleEscape("unknown_escape_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleEscape_NotInCombat verifies handleEscape returns error event when not in combat.
func TestHandleEscape_NotInCombat(t *testing.T) {
	svc, sessMgr := newEscapeSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_esc_nc", Username: "Fighter", CharName: "Fighter", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleEscape("u_esc_nc")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleEscape_NotGrabbed verifies handleEscape returns error event when player is not grabbed.
func TestHandleEscape_NotGrabbed(t *testing.T) {
	svc, sessMgr := newEscapeSvc(t, nil, nil, nil)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_esc_ng", Username: "Fighter", CharName: "Fighter", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	event, err := svc.handleEscape("u_esc_ng")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "not grabbed")
}

// TestHandleEscape_SpendAPFail verifies handleEscape returns error event when AP is insufficient.
func TestHandleEscape_SpendAPFail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newEscapeSvcWithCombat(t, roller)
	condReg := makeTestConditionRegistry()

	const roomID = "room_esc_ap"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "brute-esc-ap", Name: "Brute", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_esc_ap", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	// Apply grabbed condition.
	grabbedDef, ok := condReg.Get("grabbed")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

	_, err = combatHandler.Attack("u_esc_ap", "Brute")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Drain all AP.
	for combatHandler.RemainingAP("u_esc_ap") > 0 {
		_ = combatHandler.SpendAP("u_esc_ap", 1)
	}

	event, err := svc.handleEscape("u_esc_ap")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
}

// TestHandleEscape_RollBelow_StillGrabbed verifies that a failed escape roll keeps grabbed.
func TestHandleEscape_RollBelow_StillGrabbed(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// roll=1, bonus=0, total=1; DC=15 (no grabber NPC set, uses default DC).
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newEscapeSvcWithCombat(t, roller)
	condReg := makeTestConditionRegistry()

	const roomID = "room_esc_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ogre-esc-rb", Name: "Ogre", Level: 1, MaxHP: 30, AC: 14, Perception: 3,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_esc_rb", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	// Apply grabbed condition.
	grabbedDef, ok := condReg.Get("grabbed")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

	_, err = combatHandler.Attack("u_esc_rb", "Ogre")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleEscape("u_esc_rb")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "failure")

	// grabbed condition must still be present.
	assert.True(t, sess.Conditions.Has("grabbed"), "grabbed condition must remain on escape failure")
}

// TestHandleEscape_RollAbove_Success verifies that a successful escape roll removes grabbed.
func TestHandleEscape_RollAbove_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// roll=20, bonus=0, total=20 >= DC=15 (default no-grabber DC).
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newEscapeSvcWithCombat(t, roller)
	condReg := makeTestConditionRegistry()

	const roomID = "room_esc_ra"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "troll-esc-ra", Name: "Troll", Level: 1, MaxHP: 30, AC: 14, Perception: 3,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_esc_ra", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	// Apply grabbed condition.
	grabbedDef, ok := condReg.Get("grabbed")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

	_, err = combatHandler.Attack("u_esc_ra", "Troll")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleEscape("u_esc_ra")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "success")

	// grabbed condition must be removed.
	assert.False(t, sess.Conditions.Has("grabbed"), "grabbed condition must be removed on escape success")
	// GrabberID must be cleared.
	assert.Equal(t, "", sess.GrabberID)
}
