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
	"pgregory.net/rapid"
)

func newDelaySvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
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

// setupDelayPlayer spawns a Guard NPC in roomID, adds a player, marks them in combat,
// calls Attack to register the combat in the engine, and cancels the auto-advance timer.
//
// Precondition: npcMgr and sessMgr are non-nil.
// Postcondition: player session exists with statusInCombat; combat registered in engine with ActionQueues initialised.
func setupDelayPlayer(t testing.TB, uid, roomID, npcName string, sessMgr *session.Manager, npcMgr *npc.Manager, combatHandler *CombatHandler) *session.PlayerSession {
	t.Helper()
	_, err := npcMgr.Spawn(&npc.Template{
		ID: uid + "-guard", Name: npcName, Level: 1, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)
	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, Role: "player",
		RoomID: roomID, CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, addErr)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, attackErr := combatHandler.Attack(uid, npcName)
	require.NoError(t, attackErr)
	combatHandler.cancelTimer(roomID)
	return sess
}

// TestHandleDelay_OutOfCombat_Fails verifies delay returns an informational event outside combat.
//
// Precondition: Player "dl_ooc" exists but is not in combat.
// Postcondition: Event narrative contains "cannot delay outside"; no error.
func TestHandleDelay_OutOfCombat_Fails(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, _, _ := newDelaySvcWithCombat(t, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "dl_ooc", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	ev, err := svc.handleDelay("dl_ooc", &gamev1.DelayRequest{})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "cannot delay outside")
}

// TestHandleDelay_InCombat_BanksAP verifies AP banking after delay.
//
// Precondition: Player "dl_bank" in combat; Attack queued (costs 1 AP), leaving 2 remaining.
// Delay costs 1 AP. Postcondition: BankedAP = min(2-1, 2) = 1; RemainingAP = 0.
func TestHandleDelay_InCombat_BanksAP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, npcMgr, combatHandler := newDelaySvcWithCombat(t, roller)
	sess := setupDelayPlayer(t, "dl_bank", "room_dl_bank", "Guard", sessMgr, npcMgr, combatHandler)
	remaining := combatHandler.RemainingAP("dl_bank")
	expected := remaining - 1
	if expected > 2 {
		expected = 2
	}
	_, err := svc.handleDelay("dl_bank", &gamev1.DelayRequest{})
	require.NoError(t, err)
	assert.Equal(t, expected, sess.BankedAP)
	assert.Equal(t, 0, combatHandler.RemainingAP("dl_bank"))
}

// TestHandleDelay_InCombat_AppliesACPenalty verifies -2 ACMod on player combatant after delay.
//
// Precondition: Player "dl_ac" in combat with default AP.
// Postcondition: Player combatant ACMod = -2 after delay.
func TestHandleDelay_InCombat_AppliesACPenalty(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, npcMgr, combatHandler := newDelaySvcWithCombat(t, roller)
	setupDelayPlayer(t, "dl_ac", "room_dl_ac", "Sniper", sessMgr, npcMgr, combatHandler)
	_, err := svc.handleDelay("dl_ac", &gamev1.DelayRequest{})
	require.NoError(t, err)
	acMod := combatHandler.PlayerACMod("dl_ac")
	assert.Equal(t, -2, acMod)
}

// TestProperty_BankedAP_Formula verifies BankedAP = min(remainingAP - 1, 2) for all achievable remaining AP values.
//
// The formula is tested by spending AP down to a target value from whatever the engine provides after
// the initial Attack action. startingAP is drawn from [1, current] to ensure we only spend (not add) AP.
// Precondition: Player in active combat; remaining AP reduced to startingAP.
// Postcondition: sess.BankedAP == min(startingAP-1, 2); RemainingAP == 0.
func TestProperty_BankedAP_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		logger := zaptest.NewLogger(t)
		roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
		svc, sessMgr, npcMgr, combatHandler := newDelaySvcWithCombat(t, roller)

		const uid = "dl_prop"
		const roomID = "room_dl_prop"
		_, spawnErr := npcMgr.Spawn(&npc.Template{
			ID: uid + "-guard", Name: "Bandit", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
		}, roomID)
		require.NoError(rt, spawnErr)
		sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: uid, Role: "player",
			RoomID: roomID, CurrentHP: 10, MaxHP: 10,
		})
		require.NoError(rt, addErr)
		sess.Conditions = condition.NewActiveSet()
		sess.Status = statusInCombat
		_, attackErr := combatHandler.Attack(uid, "Bandit")
		require.NoError(rt, attackErr)
		combatHandler.cancelTimer(roomID)

		// Draw startingAP in [1, current] to simulate spending down only.
		current := combatHandler.RemainingAP(uid)
		if current < 1 {
			rt.Skip("no AP available after Attack")
		}
		startingAP := rapid.IntRange(1, current).Draw(rt, "startingAP")
		if diff := current - startingAP; diff > 0 {
			_ = combatHandler.SpendAP(uid, diff)
		}

		_, err := svc.handleDelay(uid, &gamev1.DelayRequest{})
		require.NoError(rt, err)
		expected := startingAP - 1
		if expected > 2 {
			expected = 2
		}
		assert.Equal(rt, expected, sess.BankedAP)
		assert.Equal(rt, 0, combatHandler.RemainingAP(uid))
	})
}
