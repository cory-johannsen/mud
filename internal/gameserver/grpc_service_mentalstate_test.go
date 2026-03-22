package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newCombatSvcWithMentalMgr builds a full GameServiceServer with a real MentalStateManager.
// Uses the same construction pattern as newDisarmSvcWithCombat.
//
// Precondition: t must be non-nil; mentalMgr must be non-nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newCombatSvcWithMentalMgr(t *testing.T, mentalMgr *mentalstate.Manager) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, (*dice.Roller)(nil),
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, mentalMgr,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
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
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHPThreshold_FearTrigger verifies that checkHPThresholdFear sets Fear Uneasy at ≤25% HP.
//
// Precondition: player CurrentHP/MaxHP <= 0.25.
// Postcondition: mentalMgr.CurrentSeverity(uid, TrackFear) == SeverityMild.
func TestHPThreshold_FearTrigger(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	_, sessMgr, _, combatH := newCombatSvcWithMentalMgr(t, mentalMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_fear", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_fear")
	require.True(t, ok)
	sess.MaxHP = 10
	sess.CurrentHP = 2 // 20% — at threshold

	combatH.checkHPThresholdFear("u_fear")

	assert.Equal(t, mentalstate.SeverityMild, mentalMgr.CurrentSeverity("u_fear", mentalstate.TrackFear))
}

// TestHPThreshold_NoTriggerAboveThreshold verifies no Fear trigger above 25% HP.
//
// Precondition: player CurrentHP/MaxHP > 0.25.
// Postcondition: mentalMgr.CurrentSeverity(uid, TrackFear) == SeverityNone.
func TestHPThreshold_NoTriggerAboveThreshold(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	_, sessMgr, _, combatH := newCombatSvcWithMentalMgr(t, mentalMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_fear2", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_fear2")
	require.True(t, ok)
	sess.MaxHP = 10
	sess.CurrentHP = 4 // 40% — above threshold

	combatH.checkHPThresholdFear("u_fear2")

	assert.Equal(t, mentalstate.SeverityNone, mentalMgr.CurrentSeverity("u_fear2", mentalstate.TrackFear))
}

// TestHPThreshold_ExactlyAtThreshold verifies Fear triggers at exactly 25% HP.
//
// Precondition: player CurrentHP/MaxHP == 0.25.
// Postcondition: mentalMgr.CurrentSeverity(uid, TrackFear) == SeverityMild.
func TestHPThreshold_ExactlyAtThreshold(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	_, sessMgr, _, combatH := newCombatSvcWithMentalMgr(t, mentalMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_fear3", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_fear3")
	require.True(t, ok)
	sess.MaxHP = 8
	sess.CurrentHP = 2 // exactly 25%

	combatH.checkHPThresholdFear("u_fear3")

	assert.Equal(t, mentalstate.SeverityMild, mentalMgr.CurrentSeverity("u_fear3", mentalstate.TrackFear))
}

// TestHPThreshold_NoManagerNoOp verifies no panic when mentalStateMgr is nil.
//
// Precondition: combatH.mentalStateMgr == nil.
// Postcondition: no panic; function returns without error.
func TestHPThreshold_NoManagerNoOp(t *testing.T) {
	_, sessMgr, _, combatH := newCombatSvcWithMentalMgr(t, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_fear4", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_fear4")
	require.True(t, ok)
	sess.MaxHP = 10
	sess.CurrentHP = 1

	// Must not panic.
	combatH.checkHPThresholdFear("u_fear4")
}
