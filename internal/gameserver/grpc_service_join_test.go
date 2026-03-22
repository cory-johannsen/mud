package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

// newJoinSvc mirrors newGrappleSvcWithCombat — uses the same constructor args.
func newJoinSvc(t *testing.T) (*GameServiceServer, *session.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, nil,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
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
	return svc, sessMgr, combatHandler
}

// REQ-T6: handleJoin when PendingCombatJoin == "" returns "No combat to join."
func TestHandleJoin_NoPendingCombatJoin_ReturnsNoJoinMessage(t *testing.T) {
	svc, sessMgr, _ := newJoinSvc(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_join_nopend",
		Username: "Joiner",
		CharName: "Joiner",
		RoomID:   "room-1",
		Role:     "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)

	resp, err := svc.handleJoin("u_join_nopend", &gamev1.JoinRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	msgEvt := resp.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "No combat to join")
}

// REQ-T3: handleDecline when PendingCombatJoin == "" returns "Nothing to decline."
func TestHandleDecline_NoPendingCombatJoin_ReturnsNothingToDecline(t *testing.T) {
	svc, sessMgr, _ := newJoinSvc(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_dec_nopend",
		Username: "Decliner",
		CharName: "Decliner",
		RoomID:   "room-1",
		Role:     "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)

	resp, err := svc.handleDecline("u_dec_nopend", &gamev1.DeclineRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	msgEvt := resp.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "Nothing to decline")
}

// REQ-T3 (full): handleDecline when PendingCombatJoin != "" clears it and returns watch message.
func TestHandleDecline_WithPendingJoin_ClearsAndReturnsWatchMessage(t *testing.T) {
	svc, sessMgr, _ := newJoinSvc(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_dec_pend",
		Username: "Decliner",
		CharName: "Decliner",
		RoomID:   "room-1",
		Role:     "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)
	sess.PendingCombatJoin = "room-1"

	resp, err := svc.handleDecline("u_dec_pend", &gamev1.DeclineRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	msgEvt := resp.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "stay back")
	assert.Equal(t, "", sess.PendingCombatJoin, "PendingCombatJoin must be cleared")
}

// REQ-T2: handleJoin with valid PendingCombatJoin joins combat, sets status, clears pending.
func TestHandleJoin_WithPendingJoin_JoinsCombatAndClearsField(t *testing.T) {
	svc, sessMgr, combatHandler := newJoinSvc(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_join_pend",
		Username:  "Joiner",
		CharName:  "Joiner",
		RoomID:    "room-2",
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)

	// Start a combat in "room-2" with one NPC.
	npc1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8}
	_, err = combatHandler.engine.StartCombat("room-2", []*combat.Combatant{npc1},
		makeTestConditionRegistry(), nil, "")
	require.NoError(t, err)

	sess.PendingCombatJoin = "room-2"

	resp, err := svc.handleJoin("u_join_pend", &gamev1.JoinRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "", sess.PendingCombatJoin, "PendingCombatJoin must be cleared after join")
	assert.Equal(t, statusInCombat, sess.Status, "Status must be statusInCombat after join")
}

// REQ-T-PROP: handleDecline always clears PendingCombatJoin regardless of the room ID.
func TestProperty_HandleDecline_AlwaysClearsPending(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr, _ := newJoinSvc(t)
		uid := rapid.StringMatching(`u-[a-z0-9]+`).Draw(rt, "uid")
		roomID := rapid.StringMatching(`room-[a-z0-9]+`).Draw(rt, "roomID")

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:      uid,
			Username: uid,
			CharName: uid,
			RoomID:   roomID,
			Role:     "player",
		})
		if err != nil {
			// uid collision across rapid runs — skip this draw
			rt.Skip()
		}
		require.NotNil(rt, sess)
		sess.PendingCombatJoin = roomID

		_, err = svc.handleDecline(uid, &gamev1.DeclineRequest{})
		require.NoError(rt, err)
		require.Equal(rt, "", sess.PendingCombatJoin,
			"PendingCombatJoin must be empty after decline for any room ID")
	})
}
