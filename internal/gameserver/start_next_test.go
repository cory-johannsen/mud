package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueuePopper implements DowntimeQueueRepo for testing.
// It returns entries from a pre-populated slice, one per PopHead call.
//
// Precondition: entries may be nil (empty queue).
// Postcondition: PopHead removes and returns the first unread entry, or nil if exhausted.
type mockQueuePopper struct {
	entries []*postgres.QueueEntry
	index   int
}

func (m *mockQueuePopper) PopHead(_ context.Context, _ int64) (*postgres.QueueEntry, error) {
	if m.index >= len(m.entries) {
		return nil, nil
	}
	e := m.entries[m.index]
	m.index++
	return e, nil
}

func (m *mockQueuePopper) Enqueue(_ context.Context, _ int64, _, _ string) error { return nil }
func (m *mockQueuePopper) ListQueue(_ context.Context, _ int64) ([]postgres.QueueEntry, error) {
	return nil, nil
}
func (m *mockQueuePopper) RemoveAt(_ context.Context, _ int64, _ int) error { return nil }
func (m *mockQueuePopper) Clear(_ context.Context, _ int64) error           { return nil }

// newStartNextServer creates a minimal GameServiceServer for startNext tests.
//
// Precondition: uid is non-empty; queueEntries may be nil.
// Postcondition: Returns a non-nil server and player session in a safe room.
func newStartNextServer(t *testing.T, uid string, queueEntries []*postgres.QueueEntry) (*GameServiceServer, *session.PlayerSession) {
	t.Helper()
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := session.NewManager()
	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "test_user",
		CharName:    "TestHero",
		CharacterID: 42,
		RoomID:      "room1",
		CurrentHP:   10,
		MaxHP:       10,
		Role:        "player",
		Level:       1,
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{}

	s := &GameServiceServer{
		sessions:         sMgr,
		world:            wMgr,
		downtimeQueueRepo: &mockQueuePopper{entries: queueEntries},
	}
	return s, sess
}

// TestStartNext_EmptyQueue_NoActivity verifies that startNext with an empty queue
// does not start any activity and leaves DowntimeBusy == false.
//
// Precondition: empty queue (PopHead returns nil); sess.DowntimeBusy == false.
// Postcondition: sess.DowntimeBusy == false.
func TestStartNext_EmptyQueue_NoActivity(t *testing.T) {
	s, sess := newStartNextServer(t, "uid1", nil)
	s.startNext("uid1")
	assert.False(t, sess.DowntimeBusy)
}

// TestStartNext_ValidActivity_StartsIt verifies that startNext with a valid earn_creds
// entry in a safe room starts the activity and sets DowntimeBusy = true.
//
// Precondition: queue has earn_creds entry; room has "safe" tag.
// Postcondition: sess.DowntimeBusy == true; sess.DowntimeActivityID == "earn_creds".
func TestStartNext_ValidActivity_StartsIt(t *testing.T) {
	entry := &postgres.QueueEntry{ActivityID: "earn_creds", ActivityArgs: ""}
	s, sess := newStartNextServer(t, "uid2", []*postgres.QueueEntry{entry})
	s.startNext("uid2")
	assert.True(t, sess.DowntimeBusy)
	assert.Equal(t, "earn_creds", sess.DowntimeActivityID)
	assert.True(t, sess.DowntimeCompletesAt.After(time.Now()))
}

// TestStartNext_UnknownActivity_SkipsToEmpty verifies that an unknown activity ID
// is skipped (recursive call) and terminates on empty queue.
//
// Precondition: queue has one entry with unknown activity ID, then empty.
// Postcondition: sess.DowntimeBusy == false (nothing started).
func TestStartNext_UnknownActivity_SkipsToEmpty(t *testing.T) {
	entry := &postgres.QueueEntry{ActivityID: "nonexistent_activity_xyz", ActivityArgs: ""}
	s, sess := newStartNextServer(t, "uid3", []*postgres.QueueEntry{entry})
	s.startNext("uid3")
	assert.False(t, sess.DowntimeBusy)
}
