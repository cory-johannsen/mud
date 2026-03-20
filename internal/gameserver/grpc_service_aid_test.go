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

func newAidSvc(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager) {
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
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
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
