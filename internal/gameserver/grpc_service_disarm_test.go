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

// newDisarmSvc builds a minimal GameServiceServer for handleDisarm tests.
// npcMgr may be nil; combatHandler may be nil.
func newDisarmSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
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

// newDisarmSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newDisarmSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
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
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleDisarm_NotInCombat verifies that handleDisarm returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleDisarm_NotInCombat(t *testing.T) {
	svc, sessMgr := newDisarmSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_dis_nc",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleDisarm("u_dis_nc", &gamev1.DisarmRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleDisarm_NoTarget verifies that handleDisarm returns an error event
// when no target is specified.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage: disarm".
func TestHandleDisarm_NoTarget(t *testing.T) {
	svc, sessMgr := newDisarmSvc(t, nil, nil, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_dis_nt",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleDisarm("u_dis_nt", &gamev1.DisarmRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage: disarm")
}

// TestHandleDisarm_TargetNotFound verifies that handleDisarm returns an error event
// when the named target NPC is not in the player's room, and that AP is NOT spent.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "not found"; AP is not decremented.
func TestHandleDisarm_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDisarmSvcWithCombat(t, roller)

	const roomID = "room_dis_tnf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-dis-tnf", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_dis_tnf", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_dis_tnf", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_dis_tnf")

	event, err := svc.handleDisarm("u_dis_tnf", &gamev1.DisarmRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "not found")

	apAfter := combatHandler.RemainingAP("u_dis_tnf")
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when target is not found")
}

// TestHandleDisarm_RollFailure_NoArmChange verifies that handleDisarm returns a failure message
// and does not clear the NPC's WeaponID when the muscle roll falls below the DC.
//
// Precondition: player in combat; NPC Level=5 → DC=15; dice returns 0 (roll=1, bonus=0, total=1 < 15).
// Postcondition: message event containing "failure"; NPC WeaponID unchanged.
func TestHandleDisarm_RollFailure_NoArmChange(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDisarmSvcWithCombat(t, roller)

	const roomID = "room_dis_rf"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-dis-rf", Name: "Bandit", Level: 5, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)
	inst.WeaponID = "short_sword"

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_dis_rf", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_dis_rf", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleDisarm("u_dis_rf", &gamev1.DisarmRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed disarm")
	assert.Contains(t, msgEvt.Content, "failure")

	// NPC should still have its weapon.
	assert.Equal(t, "short_sword", inst.WeaponID, "NPC WeaponID must not be cleared on failure")
}

// TestHandleDisarm_RollSuccess_WeaponCleared verifies that handleDisarm returns a success message
// referencing the weapon and clears the NPC's WeaponID when the roll meets or exceeds the DC.
//
// Precondition: player in combat; NPC Level=1 → DC=11; dice returns 19 (roll=20, total=20 >= 11); NPC has WeaponID set.
// Postcondition: message event containing "success" and "clatters"; NPC WeaponID is empty after call.
func TestHandleDisarm_RollSuccess_WeaponCleared(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDisarmSvcWithCombat(t, roller)

	const roomID = "room_dis_rs"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-dis-rs", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)
	inst.WeaponID = "short_sword"

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_dis_rs", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_dis_rs", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleDisarm("u_dis_rs", &gamev1.DisarmRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful disarm")
	assert.Contains(t, msgEvt.Content, "success")
	assert.Contains(t, msgEvt.Content, "clatters")

	assert.Empty(t, inst.WeaponID, "NPC WeaponID must be cleared after successful disarm")
}

// TestHandleDisarm_RollSuccess_NPCNoWeapon verifies that handleDisarm returns a success message
// saying "had no weapon equipped" when the NPC has no WeaponID and the roll meets the DC.
//
// Precondition: player in combat; NPC Level=1 → DC=11; dice returns 19; NPC WeaponID == "".
// Postcondition: message event containing "had no weapon equipped".
func TestHandleDisarm_RollSuccess_NPCNoWeapon(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDisarmSvcWithCombat(t, roller)

	const roomID = "room_dis_nw"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "peasant-dis-nw", Name: "Peasant", Level: 1, MaxHP: 10, AC: 10, Perception: 1,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)
	// WeaponID is empty by default (no Weapon table in template).

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_dis_nw", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_dis_nw", "Peasant")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleDisarm("u_dis_nw", &gamev1.DisarmRequest{Target: "Peasant"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event when NPC has no weapon")
	assert.Contains(t, msgEvt.Content, "had no weapon equipped")
}
