package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMoveToSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for handleMoveTo tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler sharing the same sessMgr.
func newMoveToSvcWithCombat(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	return newStrideSvcWithCombat(t)
}

// startCombatForMoveTo initiates combat between a player (at playerX, playerY) and an NPC,
// then cancels the round timer so tests have deterministic control.
//
// Precondition: svc, sessMgr, npcMgr, and combatHandler must be non-nil; roomID non-empty.
// Postcondition: Player session status is statusInCombat; combat is active in roomID.
func startCombatForMoveTo(
	t *testing.T,
	sessMgr *session.Manager,
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	roomID, playerUID string,
	playerX, playerY int,
) (*session.PlayerSession, *combat.Combat) {
	t.Helper()

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "npc-moveto-" + roomID, Name: "Target-" + roomID, Level: 1, MaxHP: 1000, AC: 30, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       playerUID,
		Username:  "Hero",
		CharName:  "Hero",
		RoomID:    roomID,
		CurrentHP: 100,
		MaxHP:     100,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack(playerUID, "Target-"+roomID)
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant(playerUID)
	require.NotNil(t, playerCbt)
	playerCbt.GridX = playerX
	playerCbt.GridY = playerY

	return sess, cbt
}

// TestHandleMoveTo_WithinOneStride_Costs1AP verifies that moving to a target within
// strideCells distance deducts exactly 1 movement AP and updates the player's position.
//
// Precondition: player at (0,0); target at (0,3); strideCells = 25/5 = 5.
// Postcondition: player at (0,3); movement AP spent = 1.
func TestHandleMoveTo_WithinOneStride_Costs1AP(t *testing.T) {
	const roomID = "room_mt_1ap"
	const uid = "u_mt_1ap"

	svc, sessMgr, npcMgr, combatHandler := newMoveToSvcWithCombat(t)
	_, cbt := startCombatForMoveTo(t, sessMgr, npcMgr, combatHandler, roomID, uid, 0, 0)

	event, err := svc.handleMoveTo(uid, &gamev1.MoveToRequest{TargetX: 0, TargetY: 3})
	require.NoError(t, err)
	require.NotNil(t, event)

	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event, not an error")
	assert.Contains(t, msgEvt.Content, "0, 3")

	playerCbt := cbt.GetCombatant(uid)
	require.NotNil(t, playerCbt)
	assert.Equal(t, 0, playerCbt.GridX, "GridX should remain 0")
	assert.Equal(t, 3, playerCbt.GridY, "GridY should be 3")

	q, qOK := cbt.ActionQueues[uid]
	require.True(t, qOK)
	assert.Equal(t, 1, q.MovementAPSpent(), "exactly 1 movement AP should have been spent")
}

// TestHandleMoveTo_WithinTwoStrides_Costs2AP verifies that moving to a target between
// strideCells and 2*strideCells distance deducts exactly 2 movement AP.
//
// Precondition: player at (0,0); target at (0,8); strideCells = 5; dist = 8 > 5 → cost 2.
// Postcondition: player at (0,8); movement AP spent = 2.
func TestHandleMoveTo_WithinTwoStrides_Costs2AP(t *testing.T) {
	const roomID = "room_mt_2ap"
	const uid = "u_mt_2ap"

	svc, sessMgr, npcMgr, combatHandler := newMoveToSvcWithCombat(t)
	_, cbt := startCombatForMoveTo(t, sessMgr, npcMgr, combatHandler, roomID, uid, 0, 0)

	event, err := svc.handleMoveTo(uid, &gamev1.MoveToRequest{TargetX: 0, TargetY: 8})
	require.NoError(t, err)
	require.NotNil(t, event)

	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event, not an error")
	assert.Contains(t, msgEvt.Content, "0, 8")

	playerCbt := cbt.GetCombatant(uid)
	require.NotNil(t, playerCbt)
	assert.Equal(t, 0, playerCbt.GridX, "GridX should remain 0")
	assert.Equal(t, 8, playerCbt.GridY, "GridY should be 8")

	q, qOK := cbt.ActionQueues[uid]
	require.True(t, qOK)
	assert.Equal(t, 2, q.MovementAPSpent(), "exactly 2 movement AP should have been spent")
}

// TestHandleMoveTo_OutOfRange_ReturnsError verifies that requesting a target beyond
// 2*strideCells returns an error event without modifying position.
//
// Precondition: player at (0,0); target at (0,15); strideCells = 5; max reach = 10.
// Postcondition: error event returned; player position unchanged at (0,0).
func TestHandleMoveTo_OutOfRange_ReturnsError(t *testing.T) {
	const roomID = "room_mt_oor"
	const uid = "u_mt_oor"

	svc, sessMgr, npcMgr, combatHandler := newMoveToSvcWithCombat(t)
	_, cbt := startCombatForMoveTo(t, sessMgr, npcMgr, combatHandler, roomID, uid, 0, 0)

	event, err := svc.handleMoveTo(uid, &gamev1.MoveToRequest{TargetX: 0, TargetY: 15})
	require.NoError(t, err)
	require.NotNil(t, event)

	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event for out-of-range target")
	assert.Contains(t, errEvt.Message, "too far")

	playerCbt := cbt.GetCombatant(uid)
	require.NotNil(t, playerCbt)
	assert.Equal(t, 0, playerCbt.GridX, "GridX should remain 0 on error")
	assert.Equal(t, 0, playerCbt.GridY, "GridY should remain 0 on error")
}

// TestHandleMoveTo_AlreadyAtTarget_ReturnsMessage verifies that requesting the current
// cell returns a "already at that location" message without spending any AP.
//
// Precondition: player at (3,4); target at (3,4).
// Postcondition: message event with "already at that location"; no AP spent.
func TestHandleMoveTo_AlreadyAtTarget_ReturnsMessage(t *testing.T) {
	const roomID = "room_mt_same"
	const uid = "u_mt_same"

	svc, sessMgr, npcMgr, combatHandler := newMoveToSvcWithCombat(t)
	_, cbt := startCombatForMoveTo(t, sessMgr, npcMgr, combatHandler, roomID, uid, 3, 4)

	event, err := svc.handleMoveTo(uid, &gamev1.MoveToRequest{TargetX: 3, TargetY: 4})
	require.NoError(t, err)
	require.NotNil(t, event)

	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event, not an error")
	assert.Contains(t, msgEvt.Content, "already at that location")

	q, qOK := cbt.ActionQueues[uid]
	require.True(t, qOK)
	assert.Equal(t, 0, q.MovementAPSpent(), "no movement AP should be spent when already at target")
}
