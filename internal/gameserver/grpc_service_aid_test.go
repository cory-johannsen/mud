package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

func newAidSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
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
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

func newAidSvc(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
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
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleAid_NotInCombat verifies that Aid returns an informational message when player is not in combat.
//
// Precondition: Player "aid_ooc" exists but is not in combat (status != statusInCombat).
// Postcondition: No error; event message contains "only valid in combat".
func TestHandleAid_NotInCombat(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr := newAidSvc(t, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "aid_ooc", Username: "AidOOC", CharName: "AidOOC", Role: "player",
		RoomID: "room_aid_ooc", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	ev, err := svc.handleAid("aid_ooc", &gamev1.AidRequest{Target: "Bob"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "only valid in combat")
}

// TestHandleAid_EmptyTarget verifies that Aid returns an informational message when no ally name is given.
//
// Precondition: Player "aid_empty" is in combat (status == statusInCombat); Target is empty string.
// Postcondition: No error; event message contains "specify an ally name".
func TestHandleAid_EmptyTarget(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr := newAidSvc(t, roller)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "aid_empty", Username: "AidEmpty", CharName: "AidEmpty", Role: "player",
		RoomID: "room_aid_empty", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	ev, err := svc.handleAid("aid_empty", &gamev1.AidRequest{Target: ""})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "specify an ally name")
}

// TestHandleAid_NilCombatHandler verifies that Aid returns an error event when combatH is nil.
//
// Precondition: Player "aid_nocombat" is statusInCombat; svc.combatH is nil.
// Postcondition: No error; event contains "Combat handler unavailable".
func TestHandleAid_NilCombatHandler(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	// Use the no-combatH variant of the service.
	svc, sessMgr := newAidSvc(t, roller)
	svc.combatH = nil
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "aid_nocombat", Username: "NoCombat", CharName: "NoCombat", Role: "player",
		RoomID: "room_aid_nocombat", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	ev, err := svc.handleAid("aid_nocombat", &gamev1.AidRequest{Target: "Bob"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetError().GetMessage(), "Combat handler unavailable")
}

// TestHandleAid_SelfTarget verifies that Aid returns an error message when the player targets themselves.
//
// Precondition: Player "aid_self" is in active combat (attacked an NPC); Target matches own CharName.
// Postcondition: No error; event message contains "cannot aid yourself".
func TestHandleAid_SelfTarget(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, npcMgr, combatHandler := newAidSvcWithCombat(t, roller)

	_, spawnErr := npcMgr.Spawn(&npc.Template{
		ID: "self-guard", Name: "SelfGuard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "room_aid_self")
	require.NoError(t, spawnErr)

	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "aid_self", Username: "aid_self", CharName: "aid_self", Role: "player",
		RoomID: "room_aid_self", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, addErr)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, attackErr := combatHandler.Attack("aid_self", "SelfGuard")
	require.NoError(t, attackErr)
	combatHandler.cancelTimer("room_aid_self")

	ev, err := svc.handleAid("aid_self", &gamev1.AidRequest{Target: "aid_self"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "cannot aid yourself")
}

// TestHandleAid_UnknownAlly verifies that Aid returns an error message when the ally is not in combat.
//
// Precondition: Player "aid_unk" is in active combat; Target "Ghost" is not a combatant.
// Postcondition: No error; event message contains "no living ally".
func TestHandleAid_UnknownAlly(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, npcMgr, combatHandler := newAidSvcWithCombat(t, roller)

	_, spawnErr := npcMgr.Spawn(&npc.Template{
		ID: "unk-guard", Name: "UnkGuard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "room_aid_unk")
	require.NoError(t, spawnErr)

	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "aid_unk", Username: "aid_unk", CharName: "aid_unk", Role: "player",
		RoomID: "room_aid_unk", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, addErr)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, attackErr := combatHandler.Attack("aid_unk", "UnkGuard")
	require.NoError(t, attackErr)
	combatHandler.cancelTimer("room_aid_unk")

	ev, err := svc.handleAid("aid_unk", &gamev1.AidRequest{Target: "Ghost"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "no living ally")
}
