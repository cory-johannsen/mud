package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newSurrenderTestServer builds a GameServiceServer with a test world, session
// manager, NPC manager, and detained condition registry.
// The player is placed in "room_a" with WantedLevel 2 in zone "test".
//
// Precondition: t must be non-nil.
// Postcondition: Returns a configured server and the player UID.
func newSurrenderTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)
	// Override condRegistry to include detained.
	svc.condRegistry = makeDetainedConditionRegistry()

	uid := "surrender_u1"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "surrender_user", CharName: "SurrenderChar",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	// "room_a" is in zone "test" (see testWorldAndSession / world_handler_test.go).
	sess.WantedLevel["test"] = 2
	return svc, uid
}

// spawnSurrenderGuard spawns a guard NPC in "room_a" on the given server.
//
// Precondition: svc must be non-nil.
// Postcondition: Returns a spawned guard instance.
func spawnSurrenderGuard(t *testing.T, svc *GameServiceServer) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:      "test_surrender_guard",
		Name:    "Guard Brix",
		NPCType: "guard",
		Level:   3,
		MaxHP:   30,
		AC:      14,
		Guard: &npc.GuardConfig{
			WantedThreshold: 1,
			Bribeable:       false,
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	return inst
}

// TestSurrender_NoGuardInRoom verifies that surrendering when no guard is present
// returns an appropriate message.
//
// Precondition: no guard NPC in room; player WantedLevel["test"] == 2.
// Postcondition: event message contains "no one".
func TestSurrender_NoGuardInRoom(t *testing.T) {
	svc, uid := newSurrenderTestServer(t)

	evt, err := svc.handleSurrender(uid, &gamev1.SurrenderRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().Content, "no one")
}

// TestSurrender_WantedLevelZero verifies that surrendering when not wanted
// returns an appropriate message.
//
// Precondition: player WantedLevel["test"] == 0; guard in room.
// Postcondition: event message contains "not wanted".
func TestSurrender_WantedLevelZero(t *testing.T) {
	svc, uid := newSurrenderTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.WantedLevel["test"] = 0
	spawnSurrenderGuard(t, svc)

	evt, err := svc.handleSurrender(uid, &gamev1.SurrenderRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().Content, "not wanted")
}

// TestSurrender_SetsDetainedConditionAndTimer verifies that surrendering at
// WantedLevel 2 sets DetainedUntil ~1 minute from now and applies the detained
// condition.
//
// Precondition: guard in room; player WantedLevel["test"] == 2.
// Postcondition: sess.DetainedUntil is within 5 seconds of now+60s;
// sess.Conditions.Has("detained") is true; event message contains "surrender".
func TestSurrender_SetsDetainedConditionAndTimer(t *testing.T) {
	svc, uid := newSurrenderTestServer(t)
	spawnSurrenderGuard(t, svc)

	before := time.Now()
	evt, err := svc.handleSurrender(uid, &gamev1.SurrenderRequest{})
	after := time.Now()
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.Contains(t, evt.GetMessage().Content, "surrender")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	require.NotNil(t, sess.DetainedUntil, "DetainedUntil must be set after surrender")

	// WantedLevel 2 → 1 minute detention.
	expectedDur := time.Minute
	lowerBound := before.Add(expectedDur - 5*time.Second)
	upperBound := after.Add(expectedDur + 5*time.Second)
	assert.True(t, sess.DetainedUntil.After(lowerBound), "DetainedUntil too early: %v", sess.DetainedUntil)
	assert.True(t, sess.DetainedUntil.Before(upperBound), "DetainedUntil too late: %v", sess.DetainedUntil)

	require.NotNil(t, sess.Conditions)
	assert.True(t, sess.Conditions.Has("detained"), "detained condition must be applied")
}

// TestDetentionCompletion_DecrementsWantedLevel verifies that calling
// checkDetentionCompletion when DetainedUntil is in the past removes the
// detained condition, decrements WantedLevel, and sets DetentionGraceUntil.
//
// Precondition: sess.DetainedUntil is 1 second in the past; WantedLevel["test"] == 2;
// detained condition applied.
// Postcondition: sess.Conditions.Has("detained") is false; WantedLevel["test"] == 1;
// sess.DetentionGraceUntil is approximately 5 seconds from now.
func TestDetentionCompletion_DecrementsWantedLevel(t *testing.T) {
	svc, uid := newSurrenderTestServer(t)
	condReg := makeDetainedConditionRegistry()

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	// Place the player in a valid room so zone lookup succeeds.
	sess.RoomID = "room_a"
	sess.WantedLevel["test"] = 2

	// Apply detained condition directly.
	applyDetainedCondition(t, sess, condReg)
	require.True(t, sess.Conditions.Has("detained"))

	// Set DetainedUntil to the past.
	past := time.Now().Add(-1 * time.Second)
	sess.DetainedUntil = &past

	before := time.Now()
	svc.checkDetentionCompletion(sess)
	after := time.Now()

	assert.Nil(t, sess.DetainedUntil, "DetainedUntil must be cleared after completion")
	assert.False(t, sess.Conditions.Has("detained"), "detained condition must be removed")
	assert.Equal(t, 1, sess.WantedLevel["test"], "WantedLevel must decrement by 1")

	// DetentionGraceUntil should be ~5 seconds from now.
	assert.True(t, sess.DetentionGraceUntil.After(before.Add(4*time.Second)), "grace period too short: %v", sess.DetentionGraceUntil)
	assert.True(t, sess.DetentionGraceUntil.Before(after.Add(6*time.Second)), "grace period too long: %v", sess.DetentionGraceUntil)
}

// TestDetentionCompletion_OfflineReconnect verifies that detention completion
// fires correctly when the player re-enters the system with an expired timer,
// simulating an offline reconnect scenario.
//
// Precondition: sess.DetainedUntil is 10 minutes in the past; WantedLevel["test"] == 3.
// Postcondition: same as TestDetentionCompletion_DecrementsWantedLevel.
func TestDetentionCompletion_OfflineReconnect(t *testing.T) {
	svc, uid := newSurrenderTestServer(t)
	condReg := makeDetainedConditionRegistry()

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	sess.RoomID = "room_a"
	sess.WantedLevel["test"] = 3

	applyDetainedCondition(t, sess, condReg)
	require.True(t, sess.Conditions.Has("detained"))

	// Simulate a long-expired timer (reconnect after being offline).
	longPast := time.Now().Add(-10 * time.Minute)
	sess.DetainedUntil = &longPast

	svc.checkDetentionCompletion(sess)

	assert.Nil(t, sess.DetainedUntil)
	assert.False(t, sess.Conditions.Has("detained"))
	assert.Equal(t, 2, sess.WantedLevel["test"], "WantedLevel must decrement from 3 to 2")
	assert.True(t, sess.DetentionGraceUntil.After(time.Now()), "grace period must be in the future")
}

// TestDetentionCompletion_NoopWhenNotDetained verifies that
// checkDetentionCompletion is a no-op when DetainedUntil is nil.
//
// Precondition: sess.DetainedUntil == nil.
// Postcondition: WantedLevel unchanged; no panic.
func TestDetentionCompletion_NoopWhenNotDetained(t *testing.T) {
	svc, uid := newSurrenderTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	sess.WantedLevel["test"] = 2
	sess.DetainedUntil = nil

	svc.checkDetentionCompletion(sess)

	assert.Equal(t, 2, sess.WantedLevel["test"], "WantedLevel must not change")
}

// TestDetentionCompletion_StillActiveDoesNotComplete verifies that
// checkDetentionCompletion does not fire when DetainedUntil is in the future.
//
// Precondition: sess.DetainedUntil is 5 minutes in the future.
// Postcondition: detained condition remains; WantedLevel unchanged.
func TestDetentionCompletion_StillActiveDoesNotComplete(t *testing.T) {
	svc, uid := newSurrenderTestServer(t)
	condReg := makeDetainedConditionRegistry()

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	sess.RoomID = "room_a"
	sess.WantedLevel["test"] = 2
	applyDetainedCondition(t, sess, condReg)

	future := time.Now().Add(5 * time.Minute)
	sess.DetainedUntil = &future

	svc.checkDetentionCompletion(sess)

	require.NotNil(t, sess.DetainedUntil, "DetainedUntil must remain set")
	assert.True(t, sess.Conditions.Has("detained"), "detained condition must remain active")
	assert.Equal(t, 2, sess.WantedLevel["test"], "WantedLevel must not change")
}
