package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// mockDowntimeRepo is a test double for CharacterDowntimeRepository.
//
// Precondition: state may be nil (simulates no persisted downtime row).
// Postcondition: Load returns state and loadErr; Save/Clear record calls.
type mockDowntimeRepo struct {
	state    *postgres.DowntimeState
	loadErr  error
	cleared  bool
	saved    *postgres.DowntimeState
}

func (m *mockDowntimeRepo) Save(_ context.Context, _ int64, state postgres.DowntimeState) error {
	m.saved = &state
	return nil
}

func (m *mockDowntimeRepo) Load(_ context.Context, _ int64) (*postgres.DowntimeState, error) {
	return m.state, m.loadErr
}

func (m *mockDowntimeRepo) Clear(_ context.Context, _ int64) error {
	m.cleared = true
	return nil
}

// newReconnectSession creates a fresh PlayerSession with the given characterID.
//
// Precondition: characterID > 0.
// Postcondition: Returns (uid, *PlayerSession) where sess.CharacterID == characterID.
func newReconnectSession(t *testing.T, characterID int64) (string, *session.PlayerSession) {
	t.Helper()
	uid := "uid_reconnect"
	sMgr := session.NewManager()
	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "reconnect_user",
		CharName:    "ReconnectHero",
		CharacterID: characterID,
		RoomID:      "room1",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	return uid, sess
}

// TestDowntimeReconnect_ResumesIfNotElapsed verifies that when a player reconnects with an
// active downtime activity that has not yet elapsed, the session busy state is restored.
//
// Precondition: downtimeRepo returns a DowntimeState with CompletesAt in the future.
// Postcondition: sess.DowntimeBusy==true and sess.DowntimeActivityID matches persisted state.
func TestDowntimeReconnect_ResumesIfNotElapsed(t *testing.T) {
	uid, sess := newReconnectSession(t, 1)

	repo := &mockDowntimeRepo{
		state: &postgres.DowntimeState{
			ActivityID:  "earn_creds",
			CompletesAt: time.Now().Add(5 * time.Minute),
			RoomID:      "room1",
		},
	}

	s := &GameServiceServer{downtimeRepo: repo}
	s.restoreDowntimeState(context.Background(), uid, sess, 1)

	assert.True(t, sess.DowntimeBusy, "sess.DowntimeBusy must be true when activity has not elapsed")
	assert.Equal(t, "earn_creds", sess.DowntimeActivityID)
	assert.False(t, repo.cleared, "repo must not be cleared for a non-elapsed activity")
}

// TestDowntimeReconnect_ResolvesIfElapsed verifies that when a player reconnects with an
// active downtime activity that already elapsed, it is resolved immediately.
//
// Precondition: downtimeRepo returns a DowntimeState with CompletesAt in the past.
// Postcondition: sess.DowntimeBusy==false after resolution; repo.cleared==true.
func TestDowntimeReconnect_ResolvesIfElapsed(t *testing.T) {
	uid, sess := newReconnectSession(t, 2)

	repo := &mockDowntimeRepo{
		state: &postgres.DowntimeState{
			ActivityID:  "earn_creds",
			CompletesAt: time.Now().Add(-1 * time.Second),
			RoomID:      "room1",
		},
	}

	s := &GameServiceServer{downtimeRepo: repo}
	s.restoreDowntimeState(context.Background(), uid, sess, 2)

	assert.False(t, sess.DowntimeBusy, "sess.DowntimeBusy must be false after resolving elapsed activity")
	assert.Empty(t, sess.DowntimeActivityID, "DowntimeActivityID must be cleared after resolution")
	assert.True(t, repo.cleared, "repo must be cleared after resolving elapsed activity")
}

// TestDowntimeReconnect_NilRepo_NoOp verifies that restoreDowntimeState is a no-op when
// downtimeRepo is nil.
//
// Precondition: downtimeRepo is nil.
// Postcondition: sess.DowntimeBusy remains false; no panic.
func TestDowntimeReconnect_NilRepo_NoOp(t *testing.T) {
	uid, sess := newReconnectSession(t, 3)

	s := &GameServiceServer{downtimeRepo: nil}
	s.restoreDowntimeState(context.Background(), uid, sess, 3)

	assert.False(t, sess.DowntimeBusy)
	assert.Empty(t, sess.DowntimeActivityID)
}

// TestDowntimeReconnect_NoRow_NoOp verifies that restoreDowntimeState is a no-op when
// the repository returns nil (no persisted row).
//
// Precondition: downtimeRepo.Load returns (nil, nil).
// Postcondition: sess.DowntimeBusy remains false; no panic.
func TestDowntimeReconnect_NoRow_NoOp(t *testing.T) {
	uid, sess := newReconnectSession(t, 4)

	repo := &mockDowntimeRepo{state: nil}

	s := &GameServiceServer{downtimeRepo: repo}
	s.restoreDowntimeState(context.Background(), uid, sess, 4)

	assert.False(t, sess.DowntimeBusy)
	assert.Empty(t, sess.DowntimeActivityID)
}

// TestDowntimeReconnect_ZeroCharacterID_NoOp verifies that restoreDowntimeState is a no-op
// when characterID is 0.
//
// Precondition: characterID == 0.
// Postcondition: sess.DowntimeBusy remains false; repo.Load is never called.
func TestDowntimeReconnect_ZeroCharacterID_NoOp(t *testing.T) {
	uid, sess := newReconnectSession(t, 5)

	repo := &mockDowntimeRepo{
		state: &postgres.DowntimeState{
			ActivityID:  "earn_creds",
			CompletesAt: time.Now().Add(5 * time.Minute),
		},
	}

	s := &GameServiceServer{downtimeRepo: repo}
	s.restoreDowntimeState(context.Background(), uid, sess, 0)

	assert.False(t, sess.DowntimeBusy, "characterID==0 must result in a no-op")
}

// TestPropertyDowntimeReconnect_ElapsedAlwaysResolved is a property test verifying that any
// activity with a CompletesAt in the past is always resolved on reconnect.
//
// Precondition: CompletesAt is always in the past.
// Postcondition: sess.DowntimeBusy is always false after restoreDowntimeState.
func TestPropertyDowntimeReconnect_ElapsedAlwaysResolved(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		characterID := rapid.Int64Range(1, 1000).Draw(rt, "characterID")
		offsetSecs := rapid.Int64Range(1, 3600).Draw(rt, "offsetSecs")

		uid, sess := newReconnectSession(t, characterID)

		repo := &mockDowntimeRepo{
			state: &postgres.DowntimeState{
				ActivityID:  "earn_creds",
				CompletesAt: time.Now().Add(-time.Duration(offsetSecs) * time.Second),
				RoomID:      "room1",
			},
		}

		s := &GameServiceServer{downtimeRepo: repo}
		s.restoreDowntimeState(context.Background(), uid, sess, characterID)

		assert.False(rt, sess.DowntimeBusy, "elapsed activity must always result in DowntimeBusy==false")
	})
}

// TestPropertyDowntimeReconnect_NotElapsedAlwaysRestored is a property test verifying that any
// activity with a CompletesAt in the future is always restored on reconnect.
//
// Precondition: CompletesAt is always in the future.
// Postcondition: sess.DowntimeBusy is always true after restoreDowntimeState.
func TestPropertyDowntimeReconnect_NotElapsedAlwaysRestored(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		characterID := rapid.Int64Range(1, 1000).Draw(rt, "characterID")
		offsetSecs := rapid.Int64Range(1, 3600).Draw(rt, "offsetSecs")

		uid, sess := newReconnectSession(t, characterID)

		repo := &mockDowntimeRepo{
			state: &postgres.DowntimeState{
				ActivityID:  "earn_creds",
				CompletesAt: time.Now().Add(time.Duration(offsetSecs) * time.Second),
				RoomID:      "room1",
			},
		}

		s := &GameServiceServer{downtimeRepo: repo}
		s.restoreDowntimeState(context.Background(), uid, sess, characterID)

		assert.True(rt, sess.DowntimeBusy, "non-elapsed activity must always result in DowntimeBusy==true")
		assert.Equal(rt, "earn_creds", sess.DowntimeActivityID)
	})
}
