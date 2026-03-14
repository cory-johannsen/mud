package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newMoveJoinSvc reuses newJoinSvc — a minimal service for move/join trigger tests.
func newMoveJoinSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	svc, sessMgr, _ := newJoinSvc(t)
	return svc, sessMgr
}

// REQ-T1: notifyCombatJoinIfEligible sets PendingCombatJoin when room has active combat.
func TestNotifyCombatJoinIfEligible_ActiveCombat_SetsPendingJoin(t *testing.T) {
	svc, sessMgr := newMoveJoinSvc(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_movejoin1",
		Username: "Mover1",
		CharName: "Mover1",
		RoomID:   "room-1",
		Role:     "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)

	npc1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8}
	_, err = svc.combatH.engine.StartCombat("room-1", []*combat.Combatant{npc1},
		makeTestConditionRegistry(), nil, "")
	require.NoError(t, err)

	svc.notifyCombatJoinIfEligible(sess, "room-1")

	assert.Equal(t, "room-1", sess.PendingCombatJoin,
		"PendingCombatJoin must be set to the room with active combat")
}

// REQ-T13: Player already in combat → notifyCombatJoinIfEligible does nothing.
func TestNotifyCombatJoinIfEligible_PlayerAlreadyInCombat_NoChange(t *testing.T) {
	svc, sessMgr := newMoveJoinSvc(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_movejoin2",
		Username: "Mover2",
		CharName: "Mover2",
		RoomID:   "room-2",
		Role:     "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)
	sess.Status = statusInCombat

	npc1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8}
	_, err = svc.combatH.engine.StartCombat("room-2", []*combat.Combatant{npc1},
		makeTestConditionRegistry(), nil, "")
	require.NoError(t, err)

	svc.notifyCombatJoinIfEligible(sess, "room-2")

	assert.Equal(t, "", sess.PendingCombatJoin, "Player already in combat must not receive join prompt")
}

// REQ-T1 (no combat): notifyCombatJoinIfEligible does nothing when room has no active combat.
func TestNotifyCombatJoinIfEligible_NoCombat_NoChange(t *testing.T) {
	svc, sessMgr := newMoveJoinSvc(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_movejoin3",
		Username: "Mover3",
		CharName: "Mover3",
		RoomID:   "empty-room",
		Role:     "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)

	svc.notifyCombatJoinIfEligible(sess, "empty-room")

	assert.Equal(t, "", sess.PendingCombatJoin, "No combat in room — PendingCombatJoin must remain empty")
}

// REQ-T-PROP: clearPendingJoinForRoom always leaves no session with PendingCombatJoin == roomID.
func TestProperty_ClearPendingJoin_AlwaysClearsMatchingRoom(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newMoveJoinSvc(t)
		roomID := rapid.StringMatching(`[a-z][a-z0-9-]{0,8}`).Draw(rt, "roomID")
		otherRoom := rapid.StringMatching(`[A-Z][A-Z0-9]{0,8}`).Draw(rt, "otherRoom")
		n := rapid.IntRange(0, 5).Draw(rt, "n")
		for i := 0; i < n; i++ {
			uid := rapid.StringMatching(`u[0-9]{4}`).Draw(rt, "uid")
			sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
				UID:      uid,
				Username: uid,
				CharName: uid,
				RoomID:   "room-prop",
				Role:     "player",
			})
			if err != nil || sess == nil {
				continue
			}
			if i%2 == 0 {
				sess.PendingCombatJoin = roomID
			} else {
				sess.PendingCombatJoin = otherRoom
			}
		}

		svc.clearPendingJoinForRoom(roomID)

		for _, sess := range sessMgr.AllPlayers() {
			require.NotEqual(rt, roomID, sess.PendingCombatJoin,
				"clearPendingJoinForRoom must clear all sessions pending for this room")
		}
	})
}
