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

// newShoveSvc builds a minimal GameServiceServer for handleShove tests.
// npcMgr may be nil; combatHandler may be nil.
func newShoveSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
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

// newShoveSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newShoveSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
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

// TestHandleShove_NoSession verifies that handleShove returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_shv_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleShove_NoSession(t *testing.T) {
	svc, _ := newShoveSvc(t, nil, nil, nil)
	event, err := svc.handleShove("unknown_shv_uid", &gamev1.ShoveRequest{Target: "bandit"})
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleShove_NotInCombat verifies that handleShove returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleShove_NotInCombat(t *testing.T) {
	svc, sessMgr := newShoveSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_shv_nc",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleShove("u_shv_nc", &gamev1.ShoveRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleShove_EmptyTarget verifies that handleShove returns an error event
// when no target is specified.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage: shove".
func TestHandleShove_EmptyTarget(t *testing.T) {
	svc, sessMgr := newShoveSvc(t, nil, nil, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_shv_et",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleShove("u_shv_et", &gamev1.ShoveRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage: shove")
}

// TestHandleShove_TargetNotFound verifies that handleShove returns an error event
// when the named target NPC is not in the player's room, and that AP is NOT spent.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "not found"; AP is not decremented.
func TestHandleShove_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newShoveSvcWithCombat(t, roller)

	const roomID = "room_shv_tnf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-shv-tnf", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_shv_tnf", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_shv_tnf", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_shv_tnf")

	event, err := svc.handleShove("u_shv_tnf", &gamev1.ShoveRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "not found")

	apAfter := combatHandler.RemainingAP("u_shv_tnf")
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when target is not found")
}

// TestHandleShove_Failure verifies that handleShove returns a failure message
// when the athletics roll total is below the target's Toughness DC.
//
// Precondition: player in combat; NPC Level=5 → DC=15 (Toughness DC: Level=5, Brutality=10, rank=untrained → 10+5+0+0=15); dice returns 0 (roll=1, bonus=0, total=1 < 15).
// Postcondition: message event containing "failure"; NPC position unchanged.
func TestHandleShove_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newShoveSvcWithCombat(t, roller)

	const roomID = "room_shv_fail"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-shv-fail", Name: "Bandit", Level: 5, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessFail, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_shv_fail", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessFail.Status = statusInCombat

	_, err = combatHandler.Attack("u_shv_fail", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Record position before
	npcCbt, ok := combatHandler.GetCombatant("u_shv_fail", inst.ID)
	require.True(t, ok)
	posBefore := npcCbt.Position

	event, err := svc.handleShove("u_shv_fail", &gamev1.ShoveRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed shove")
	assert.Contains(t, msgEvt.Content, "fail")

	// NPC position must not change
	assert.Equal(t, posBefore, npcCbt.Position, "NPC position must not change on failure")
}

// TestHandleShove_Success verifies that handleShove pushes the NPC 5ft on a standard success.
//
// Precondition: player in combat; NPC Level=1 → DC=11; dice returns 9 (roll=10, bonus=0, total=10 < 11 fails).
// Use level=1 NPC, dice=10 for total 11 which exactly meets DC=11.
func TestHandleShove_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// NPC Level=1, DC=11; dice returns 10 → roll=11, total=11 >= 11, but < 11+10=21 (not crit)
	src := &fixedDiceSource{val: 10}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newShoveSvcWithCombat(t, roller)

	const roomID = "room_shv_succ"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-shv-succ", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessSucc, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_shv_succ", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessSucc.Status = statusInCombat

	_, err = combatHandler.Attack("u_shv_succ", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	npcCbt, ok := combatHandler.GetCombatant("u_shv_succ", inst.ID)
	require.True(t, ok)
	posBefore := npcCbt.Position

	event, err := svc.handleShove("u_shv_succ", &gamev1.ShoveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful shove")
	assert.Contains(t, msgEvt.Content, "5 ft")

	// NPC must be pushed 5ft away from player
	assert.Equal(t, posBefore+5, npcCbt.Position, "NPC must be pushed 5ft on success")
}

// TestHandleShove_CriticalSuccess verifies that handleShove pushes the NPC 10ft on a critical success.
//
// Precondition: player in combat; NPC Level=1 → DC=11 (Toughness DC: Level=1, Brutality=10, rank=untrained → 10+1+0+0=11); dice returns 19 (roll=20, total=20 >= 21? No, 20 < 21).
// Use Level=1, DC=11; dice=19 → roll=20, total=20 < 21 so not crit. Use Level=1, DC=11; dice returns enough.
// beat DC by 10+: total >= DC+10=21, so need dice val 20 (roll=21). fixedDiceSource val=20.
func TestHandleShove_CriticalSuccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// NPC Level=1, DC=11; need total >= 21 for crit; fixedDiceSource.val=20 → roll=21, total=21 >= 21 (crit)
	src := &fixedDiceSource{val: 20}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newShoveSvcWithCombat(t, roller)

	const roomID = "room_shv_crit"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-shv-crit", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessCrit, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_shv_crit", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessCrit.Status = statusInCombat

	_, err = combatHandler.Attack("u_shv_crit", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	npcCbt, ok := combatHandler.GetCombatant("u_shv_crit", inst.ID)
	require.True(t, ok)
	posBefore := npcCbt.Position

	event, err := svc.handleShove("u_shv_crit", &gamev1.ShoveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on critical shove")
	assert.Contains(t, msgEvt.Content, "10 ft")

	// NPC must be pushed 10ft away from player
	assert.Equal(t, posBefore+10, npcCbt.Position, "NPC must be pushed 10ft on critical success")
}
