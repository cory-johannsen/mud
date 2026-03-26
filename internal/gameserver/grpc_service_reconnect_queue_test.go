package gameserver

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWorldWithUnsafeRoom creates a world.Manager with a room that has NO "safe" tag.
//
// Precondition: zoneID and roomID must be non-empty strings.
// Postcondition: The room at roomID has an empty Properties map.
func newWorldWithUnsafeRoom(zoneID, roomID string) *world.Manager {
	r := &world.Room{
		ID:          roomID,
		ZoneID:      zoneID,
		Title:       "Unsafe Room",
		Description: "A room with no safe tag.",
		MapX:        0,
		MapY:        0,
		Properties:  map[string]string{},
	}
	z := &world.Zone{
		ID:        zoneID,
		Name:      "Test Zone",
		StartRoom: roomID,
		Rooms:     map[string]*world.Room{roomID: r},
	}
	mgr, err := world.NewManager([]*world.Zone{z})
	if err != nil {
		panic("newWorldWithUnsafeRoom: " + err.Error())
	}
	return mgr
}

// newOfflineQueueSession creates a session.Manager with a single player for offline queue tests.
//
// Precondition: uid and roomID are non-empty.
// Postcondition: Returns (*session.Manager, *session.PlayerSession) with Skills initialized.
func newOfflineQueueSession(t *testing.T, uid, roomID string) (*session.Manager, *session.PlayerSession) {
	t.Helper()
	sMgr := session.NewManager()
	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "reconnect_user",
		CharName:    "ReconnectHero",
		CharacterID: 42,
		RoomID:      roomID,
		CurrentHP:   10,
		MaxHP:       10,
		Role:        "player",
		Level:       1,
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{}
	return sMgr, sess
}

// TestResolveOfflineQueue_ResolvesElapsedActivities verifies that resolveOfflineQueue
// resolves all queued activities whose hypothetical completion time has already elapsed.
//
// Precondition: sess.DowntimeBusy=false (active already resolved); queue has earn_creds and
//   patch_up; cursor = 3 hours ago; both take <= 30 min so both have elapsed.
// Postcondition: both activities resolved; sess.DowntimeBusy == false; queue empty.
func TestResolveOfflineQueue_ResolvesElapsedActivities(t *testing.T) {
	uid := "uid_offline_resolve"
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	mock := &mockDowntimeQueueRepo{
		entries: []postgres.QueueEntry{
			{Position: 1, ActivityID: "earn_creds", ActivityArgs: ""},
			{Position: 2, ActivityID: "subsist", ActivityArgs: ""},
		},
	}
	sMgr, sess := newOfflineQueueSession(t, uid, "room1")
	sess.DowntimeBusy = false

	s := &GameServiceServer{
		sessions:          sMgr,
		world:             wMgr,
		downtimeQueueRepo: mock,
	}

	// Both earn_creds (6 min) and subsist (2 min) fit easily within 3 hours.
	threeHoursAgo := time.Now().Add(-3 * time.Hour)
	s.resolveOfflineQueue(uid, sess, threeHoursAgo)

	assert.False(t, sess.DowntimeBusy, "no future activity should remain after both elapsed")
	assert.Empty(t, mock.entries, "queue must be empty after all entries consumed")
}

// TestResolveOfflineQueue_SkipsInvalidRoom verifies that resolveOfflineQueue skips
// an activity that requires a safe room when the player's room has no safe tag.
//
// Precondition: room has no "safe" tag; queue has earn_creds (requires safe); cursor = 2 hours ago.
// Postcondition: earn_creds skipped (not resolved); sess.DowntimeBusy == false.
func TestResolveOfflineQueue_SkipsInvalidRoom(t *testing.T) {
	uid := "uid_offline_unsafe"
	wMgr := newWorldWithUnsafeRoom("zone1", "room2")
	mock := &mockDowntimeQueueRepo{
		entries: []postgres.QueueEntry{
			{Position: 1, ActivityID: "earn_creds", ActivityArgs: ""},
		},
	}
	sMgr, sess := newOfflineQueueSession(t, uid, "room2")
	sess.DowntimeBusy = false

	s := &GameServiceServer{
		sessions:          sMgr,
		world:             wMgr,
		downtimeQueueRepo: mock,
	}

	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	s.resolveOfflineQueue(uid, sess, twoHoursAgo)

	assert.False(t, sess.DowntimeBusy, "no activity should be started when room is invalid")
}

// TestResolveOfflineQueue_StartsNextFutureActivity verifies that resolveOfflineQueue
// resolves an elapsed activity and then starts the next queued activity as active
// when that next activity has not yet elapsed.
//
// Precondition: safe room; cursor = 1 hour ago; queue has earn_creds (30 min, elapsed) then
//   subsist (30 min, would start 30 min ago and end now — future relative to cursor+30min).
//   We use cursor = 31 minutes ago so earn_creds elapsed, and subsist's hypothetical end
//   is 1 minute in the future.
// Postcondition: earn_creds resolved; sess.DowntimeBusy == true; sess.DowntimeActivityID == "subsist".
func TestResolveOfflineQueue_StartsNextFutureActivity(t *testing.T) {
	uid := "uid_offline_startnext"
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	mock := &mockDowntimeQueueRepo{
		entries: []postgres.QueueEntry{
			{Position: 1, ActivityID: "earn_creds", ActivityArgs: ""},
			{Position: 2, ActivityID: "subsist", ActivityArgs: ""},
		},
	}
	sMgr, sess := newOfflineQueueSession(t, uid, "room1")
	sess.DowntimeBusy = false

	s := &GameServiceServer{
		sessions:          sMgr,
		world:             wMgr,
		downtimeQueueRepo: mock,
	}

	// earn_creds has DurationMinutes=6. subsist has DurationMinutes=2.
	// cursor = 7 minutes ago => earn_creds hypothetical end = 1 min ago (elapsed).
	// After resolving earn_creds, cursor advances to 1 min ago.
	// subsist hypothetical end = 1 min ago + 2 min = 1 min in the future (not elapsed).
	cursor := time.Now().Add(-7 * time.Minute)
	s.resolveOfflineQueue(uid, sess, cursor)

	assert.True(t, sess.DowntimeBusy, "subsist should be started as active activity")
	assert.Equal(t, "subsist", sess.DowntimeActivityID)
}
