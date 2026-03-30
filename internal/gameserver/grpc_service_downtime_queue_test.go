package gameserver

import (
	"context"
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDowntimeQueueRepo implements DowntimeQueueRepo for queue subcommand tests.
// It maintains an in-memory slice of QueueEntry values with full CRUD semantics.
//
// Precondition: entries may be nil (empty queue).
// Postcondition: all operations maintain 1-based contiguous position ordering.
type mockDowntimeQueueRepo struct {
	entries []postgres.QueueEntry
}

func (m *mockDowntimeQueueRepo) Enqueue(_ context.Context, _ int64, activityID, activityArgs string) error {
	m.entries = append(m.entries, postgres.QueueEntry{
		Position:     len(m.entries) + 1,
		ActivityID:   activityID,
		ActivityArgs: activityArgs,
	})
	return nil
}

func (m *mockDowntimeQueueRepo) ListQueue(_ context.Context, _ int64) ([]postgres.QueueEntry, error) {
	return m.entries, nil
}

func (m *mockDowntimeQueueRepo) RemoveAt(_ context.Context, _ int64, pos int) error {
	for i, e := range m.entries {
		if e.Position == pos {
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			for j := range m.entries {
				m.entries[j].Position = j + 1
			}
			return nil
		}
	}
	return nil
}

func (m *mockDowntimeQueueRepo) Clear(_ context.Context, _ int64) error {
	m.entries = nil
	return nil
}

func (m *mockDowntimeQueueRepo) PopHead(_ context.Context, _ int64) (*postgres.QueueEntry, error) {
	if len(m.entries) == 0 {
		return nil, nil
	}
	head := m.entries[0]
	m.entries = m.entries[1:]
	for i := range m.entries {
		m.entries[i].Position = i + 1
	}
	return &head, nil
}

// newQueueTestServer creates a minimal GameServiceServer for queue subcommand tests.
//
// Precondition: uid is non-empty; mock is non-nil.
// Postcondition: Returns a non-nil server and player session.
func newQueueTestServer(t *testing.T, uid string, mock *mockDowntimeQueueRepo) (*GameServiceServer, *session.PlayerSession) {
	t.Helper()
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := session.NewManager()
	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "test",
		CharName:    "TestHero",
		CharacterID: 42,
		RoomID:      "room1",
		CurrentHP:   10,
		MaxHP:       10,
		Role:        "player",
		Level:       1,
	})
	require.NoError(t, err)
	sess.DowntimeQueueLimit = 5
	sess.Skills = map[string]string{}
	s := &GameServiceServer{
		sessions:          sMgr,
		world:             wMgr,
		downtimeQueueRepo: mock,
	}
	return s, sess
}

// eventText extracts the text from a *gamev1.ServerEvent via GetMessage().GetContent().
func eventText(evt *gamev1.ServerEvent) string {
	if evt == nil {
		return ""
	}
	if msg := evt.GetMessage(); msg != nil {
		return msg.GetContent()
	}
	return ""
}

// TestHandleDowntime_QueueAdd_Success verifies that a valid alias queues an activity
// when the queue is below the limit.
//
// Precondition: sess.DowntimeQueueLimit=5; queue empty; args="earn".
// Postcondition: response contains "Queued"; queue has 1 entry.
func TestHandleDowntime_QueueAdd_Success(t *testing.T) {
	mock := &mockDowntimeQueueRepo{}
	s, _ := newQueueTestServer(t, "uid_qa_ok", mock)

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "earn"}
	evt, err := s.handleDowntime("uid_qa_ok", req)

	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(eventText(evt)), "queued")
	assert.Len(t, mock.entries, 1)
	assert.Equal(t, "earn_creds", mock.entries[0].ActivityID)
}

// TestHandleDowntime_QueueAdd_AtLimit_Fails verifies that adding to a full queue
// returns a "full" error message without enqueuing.
//
// Precondition: sess.DowntimeQueueLimit=1; queue has 1 entry; args="earn".
// Postcondition: response contains "full"; queue remains at 1 entry.
func TestHandleDowntime_QueueAdd_AtLimit_Fails(t *testing.T) {
	mock := &mockDowntimeQueueRepo{
		entries: []postgres.QueueEntry{
			{Position: 1, ActivityID: "earn_creds", ActivityArgs: ""},
		},
	}
	s, sess := newQueueTestServer(t, "uid_qa_limit", mock)
	sess.DowntimeQueueLimit = 1

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "earn"}
	evt, err := s.handleDowntime("uid_qa_limit", req)

	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(eventText(evt)), "full")
	assert.Len(t, mock.entries, 1)
}

// TestHandleDowntime_QueueAdd_InvalidAlias_Fails verifies that an unknown activity alias
// returns an "Unknown" message.
//
// Precondition: queue empty; args="notanactivity".
// Postcondition: response contains "Unknown"; queue remains empty.
func TestHandleDowntime_QueueAdd_InvalidAlias_Fails(t *testing.T) {
	mock := &mockDowntimeQueueRepo{}
	s, _ := newQueueTestServer(t, "uid_qa_inv", mock)

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "notanactivity"}
	evt, err := s.handleDowntime("uid_qa_inv", req)

	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(eventText(evt)), "unknown")
	assert.Len(t, mock.entries, 0)
}

// TestHandleDowntime_QueueList_Empty verifies that listing an empty queue returns
// a message containing "empty".
//
// Precondition: queue empty; args="list".
// Postcondition: response contains "empty".
func TestHandleDowntime_QueueList_Empty(t *testing.T) {
	mock := &mockDowntimeQueueRepo{}
	s, _ := newQueueTestServer(t, "uid_ql_empty", mock)

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "list"}
	evt, err := s.handleDowntime("uid_ql_empty", req)

	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(eventText(evt)), "empty")
}

// TestHandleDowntime_QueueList_ShowsEntries verifies that listing a non-empty queue
// includes the activity names.
//
// Precondition: queue has earn_creds and subsist entries; args="list".
// Postcondition: response contains "Earn Creds" and "Subsist".
func TestHandleDowntime_QueueList_ShowsEntries(t *testing.T) {
	mock := &mockDowntimeQueueRepo{
		entries: []postgres.QueueEntry{
			{Position: 1, ActivityID: "earn_creds", ActivityArgs: ""},
			{Position: 2, ActivityID: "subsist", ActivityArgs: ""},
		},
	}
	s, _ := newQueueTestServer(t, "uid_ql_entries", mock)

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "list"}
	evt, err := s.handleDowntime("uid_ql_entries", req)

	require.NoError(t, err)
	text := eventText(evt)
	assert.Contains(t, text, "Earn Creds")
	assert.Contains(t, text, "Subsist")
}

// TestHandleDowntime_QueueRemove_Success verifies that removing a valid position
// removes the entry and the queue reindexes.
//
// Precondition: queue has 3 entries; args="remove 2".
// Postcondition: queue has 2 entries; positions are 1 and 2.
func TestHandleDowntime_QueueRemove_Success(t *testing.T) {
	mock := &mockDowntimeQueueRepo{
		entries: []postgres.QueueEntry{
			{Position: 1, ActivityID: "earn_creds"},
			{Position: 2, ActivityID: "subsist"},
			{Position: 3, ActivityID: "patch_up"},
		},
	}
	s, _ := newQueueTestServer(t, "uid_qr_ok", mock)

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "remove 2"}
	evt, err := s.handleDowntime("uid_qr_ok", req)

	require.NoError(t, err)
	assert.Contains(t, eventText(evt), "removed")
	require.Len(t, mock.entries, 2)
	assert.Equal(t, 1, mock.entries[0].Position)
	assert.Equal(t, "earn_creds", mock.entries[0].ActivityID)
	assert.Equal(t, 2, mock.entries[1].Position)
	assert.Equal(t, "patch_up", mock.entries[1].ActivityID)
}

// TestHandleDowntime_QueueRemove_InvalidPosition verifies that a position < 1
// returns a usage message.
//
// Precondition: args="remove 0".
// Postcondition: response contains "Usage".
func TestHandleDowntime_QueueRemove_InvalidPosition(t *testing.T) {
	mock := &mockDowntimeQueueRepo{}
	s, _ := newQueueTestServer(t, "uid_qr_inv", mock)

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "remove 0"}
	evt, err := s.handleDowntime("uid_qr_inv", req)

	require.NoError(t, err)
	assert.Contains(t, eventText(evt), "Usage")
}

// TestHandleDowntime_QueueClear_OnlyClearsQueue verifies that the clear subcommand
// empties the queue without affecting the active activity (REQ-DTQ-16).
//
// Precondition: queue has 2 entries; sess.DowntimeBusy=true; args="clear".
// Postcondition: queue is empty; sess.DowntimeBusy remains true.
func TestHandleDowntime_QueueClear_OnlyClearsQueue(t *testing.T) {
	mock := &mockDowntimeQueueRepo{
		entries: []postgres.QueueEntry{
			{Position: 1, ActivityID: "earn_creds", ActivityArgs: ""},
			{Position: 2, ActivityID: "subsist", ActivityArgs: ""},
		},
	}
	s, sess := newQueueTestServer(t, "uid_qc_clear", mock)
	sess.DowntimeBusy = true
	sess.DowntimeActivityID = "recalibrate"

	req := &gamev1.DowntimeRequest{Subcommand: "queue", Args: "clear"}
	evt, err := s.handleDowntime("uid_qc_clear", req)

	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(eventText(evt)), "cleared")
	assert.Len(t, mock.entries, 0, "queue must be empty after clear")
	assert.True(t, sess.DowntimeBusy, "active activity must not be cancelled by queue clear (REQ-DTQ-16)")
}
