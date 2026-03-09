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

// newSneakSvc builds a minimal GameServiceServer for handleSneak tests.
func newSneakSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
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
		nil,
	)
	return svc, sessMgr
}

// newSneakSvcWithCombat builds a GameServiceServer with real combat state and condition registry.
func newSneakSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeTestConditionRegistry()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleSneak_NoSession verifies handleSneak returns error when session is missing.
func TestHandleSneak_NoSession(t *testing.T) {
	svc, _ := newSneakSvc(t, nil, nil, nil)
	event, err := svc.handleSneak("unknown_sneak_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleSneak_NotInCombat verifies handleSneak returns error event when not in combat.
func TestHandleSneak_NotInCombat(t *testing.T) {
	svc, sessMgr := newSneakSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_snk_nc", Username: "Rogue", CharName: "Rogue", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleSneak("u_snk_nc")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleSneak_NotHidden verifies handleSneak returns error event when player is not hidden.
func TestHandleSneak_NotHidden(t *testing.T) {
	svc, sessMgr := newSneakSvc(t, nil, nil, nil)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_snk_nh", Username: "Rogue", CharName: "Rogue", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	event, err := svc.handleSneak("u_snk_nh")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "must be hidden to sneak")
}

// TestHandleSneak_RollBelow_RemovesHidden verifies that a failed sneak roll removes hidden.
func TestHandleSneak_RollBelow_RemovesHidden(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // roll=1, bonus=0, total=1 < DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newSneakSvcWithCombat(t, roller)
	condReg := makeTestConditionRegistry()

	const roomID = "room_snk_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "watcher-snk-rb", Name: "Watcher", Level: 1, MaxHP: 20, AC: 13, Perception: 12,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_snk_rb", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	// Apply hidden condition manually.
	hiddenDef, ok := condReg.Get("hidden")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, hiddenDef, 1, -1))
	require.True(t, sess.Conditions.Has("hidden"))

	_, err = combatHandler.Attack("u_snk_rb", "Watcher")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Set combatant hidden.
	require.NoError(t, combatHandler.SetCombatantHidden("u_snk_rb", true))

	event, err := svc.handleSneak("u_snk_rb")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "failure")

	// hidden condition must be removed.
	assert.False(t, sess.Conditions.Has("hidden"), "hidden condition must be removed on sneak failure")

	// combatant Hidden must be false.
	combatant, ok := combatHandler.GetCombatant("u_snk_rb", "u_snk_rb")
	require.True(t, ok)
	assert.False(t, combatant.Hidden)
}

// TestHandleSneak_RollAbove_MaintainsHidden verifies that a successful sneak roll keeps hidden.
func TestHandleSneak_RollAbove_MaintainsHidden(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20, bonus=0, total=20 >= DC=10
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newSneakSvcWithCombat(t, roller)
	condReg := makeTestConditionRegistry()

	const roomID = "room_snk_ra"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "spy-snk-ra", Name: "Spy", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_snk_ra", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	// Apply hidden condition manually.
	hiddenDef, ok := condReg.Get("hidden")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, hiddenDef, 1, -1))
	require.True(t, sess.Conditions.Has("hidden"))

	_, err = combatHandler.Attack("u_snk_ra", "Spy")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Set combatant hidden.
	require.NoError(t, combatHandler.SetCombatantHidden("u_snk_ra", true))

	event, err := svc.handleSneak("u_snk_ra")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "success")

	// hidden condition must still be present.
	assert.True(t, sess.Conditions.Has("hidden"), "hidden condition must remain on sneak success")

	// combatant Hidden must still be true.
	combatant, ok := combatHandler.GetCombatant("u_snk_ra", "u_snk_ra")
	require.True(t, ok)
	assert.True(t, combatant.Hidden)
}
