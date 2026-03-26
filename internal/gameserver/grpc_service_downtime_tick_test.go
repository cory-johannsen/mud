package gameserver

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addBusyPlayer adds a player with DowntimeBusy=true and DowntimeActivityID set to "earn_creds"
// to the provided session manager, and returns the PlayerSession.
//
// Precondition: uid and roomID are non-empty.
// Postcondition: GetPlayer(uid).DowntimeBusy == true.
func addBusyPlayer(t *testing.T, sMgr *session.Manager, uid, roomID string, completesAt time.Time) *session.PlayerSession {
	t.Helper()
	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "BusyUser",
		CharName:    "BusyHero",
		CharacterID: 99,
		RoomID:      roomID,
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.DowntimeBusy = true
	sess.DowntimeActivityID = "earn_creds"
	sess.DowntimeCompletesAt = completesAt
	return sess
}

// TestCheckCompletion_ResolvesElapsedActivity verifies that checkDowntimeCompletion
// clears the busy state when the activity completion time has already elapsed.
//
// Precondition: sess.DowntimeBusy==true; sess.DowntimeCompletesAt is in the past.
// Postcondition: sess.DowntimeBusy==false after call.
func TestCheckCompletion_ResolvesElapsedActivity(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := session.NewManager()

	past := time.Now().Add(-1 * time.Second)
	addBusyPlayer(t, sMgr, "uid_elapsed", "room1", past)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	s.checkDowntimeCompletion("uid_elapsed")

	sess, ok := sMgr.GetPlayer("uid_elapsed")
	require.True(t, ok)
	assert.False(t, sess.DowntimeBusy, "DowntimeBusy must be cleared after elapsed activity")
}

// TestCheckCompletion_DoesNotResolveIfNotElapsed verifies that checkDowntimeCompletion
// does NOT clear the busy state when the activity has not yet elapsed.
//
// Precondition: sess.DowntimeBusy==true; sess.DowntimeCompletesAt is in the future.
// Postcondition: sess.DowntimeBusy remains true.
func TestCheckCompletion_DoesNotResolveIfNotElapsed(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := session.NewManager()

	future := time.Now().Add(10 * time.Minute)
	addBusyPlayer(t, sMgr, "uid_future", "room1", future)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	s.checkDowntimeCompletion("uid_future")

	sess, ok := sMgr.GetPlayer("uid_future")
	require.True(t, ok)
	assert.True(t, sess.DowntimeBusy, "DowntimeBusy must remain true when activity has not elapsed")
}

// TestCheckCompletion_NoopIfNotBusy verifies that checkDowntimeCompletion is a no-op
// when the player is not currently engaged in a downtime activity.
//
// Precondition: sess.DowntimeBusy==false.
// Postcondition: No panic; DowntimeBusy remains false.
func TestCheckCompletion_NoopIfNotBusy(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid_idle", "room1")

	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	require.NotPanics(t, func() {
		s.checkDowntimeCompletion("uid_idle")
	})

	sess, ok := sMgr.GetPlayer("uid_idle")
	require.True(t, ok)
	assert.False(t, sess.DowntimeBusy, "DowntimeBusy must remain false for non-busy player")
}
