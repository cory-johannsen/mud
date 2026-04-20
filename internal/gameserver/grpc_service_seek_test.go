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

// newSeekSvcWithCombat builds a GameServiceServer with real combat state for handleSeek tests.
func newSeekSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
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

// TestHandleSeek_NoSession verifies handleSeek returns error when session is missing.
func TestHandleSeek_NoSession(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
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
	event, err := svc.handleSeek("unknown_seek_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleSeek_NotInCombat verifies handleSeek returns error event when not in combat.
func TestHandleSeek_NotInCombat(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
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
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_seek_nc", Username: "Scout", CharName: "Scout", RoomID: "room_seek_nc", Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleSeek("u_seek_nc")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleSeek_NoHiddenNPCs verifies handleSeek returns "no hidden threats" when no NPC is hidden.
func TestHandleSeek_NoHiddenNPCs(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20, total=20 >= DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newSeekSvcWithCombat(t, roller)

	const roomID = "room_seek_noh"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-seek-noh", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_seek_noh", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_seek_noh", "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// NPC combatant is not hidden — no hidden threats.
	event, err := svc.handleSeek("u_seek_noh")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "no hidden threats")
}

// TestHandleSeek_Success verifies handleSeek reveals a hidden NPC combatant on a successful roll.
func TestHandleSeek_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20, total=20 >= DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newSeekSvcWithCombat(t, roller)

	const roomID = "room_seek_suc"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "shadow-seek-suc", Name: "Shadow", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_seek_suc", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_seek_suc", "Shadow")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Mark the NPC combatant as hidden.
	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	var npcCombatant *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.Hidden = true
			npcCombatant = c
			break
		}
	}
	require.NotNil(t, npcCombatant, "NPC combatant must exist")

	event, err := svc.handleSeek("u_seek_suc")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "success")
	assert.Contains(t, msgEvt.Content, "Shadow")

	// RevealedUntilRound must be set to cbt.Round+1.
	assert.Equal(t, cbt.Round+1, npcCombatant.RevealedUntilRound)
}

// TestHandleSeek_Failure verifies handleSeek does not reveal hidden NPCs on a failed roll.
func TestHandleSeek_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // roll=1, total=1 < DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newSeekSvcWithCombat(t, roller)

	const roomID = "room_seek_fail"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "lurker-seek-fail", Name: "Lurker", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_seek_fail", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_seek_fail", "Lurker")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Mark the NPC combatant as hidden.
	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	var npcCombatant *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.Hidden = true
			npcCombatant = c
			break
		}
	}
	require.NotNil(t, npcCombatant, "NPC combatant must exist")

	event, err := svc.handleSeek("u_seek_fail")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "find nothing")

	// Hidden must remain true; RevealedUntilRound must remain 0.
	assert.True(t, npcCombatant.Hidden)
	assert.Equal(t, 0, npcCombatant.RevealedUntilRound)
}
