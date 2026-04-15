package gameserver

import (
	"testing"

	"go.uber.org/zap"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// makeMinimalServer returns a GameServiceServer with only the sessions Manager
// populated — sufficient for cleanupPlayer entity-guard testing.
func makeMinimalServer(t *testing.T) *GameServiceServer {
	t.Helper()
	return &GameServiceServer{
		sessions: session.NewManager(),
		logger:   zap.NewNop(),
	}
}

// addTestSession registers a player with the given uid and returns the session.
func addTestSession(t *testing.T, mgr *session.Manager, uid string) *session.PlayerSession {
	t.Helper()
	sess, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: "user_" + uid,
		CharName: "Char_" + uid,
		RoomID:   "room-1",
		Role:     "player",
	})
	if err != nil {
		t.Fatalf("AddPlayer(%q): %v", uid, err)
	}
	return sess
}

// TestCleanupPlayer_StaleCleanupDoesNotEvictReconnect verifies that when a
// reconnect evicts Session A and Session B is registered under the same uid,
// Session A's deferred cleanupPlayer call does NOT remove Session B from the
// session registry.
//
// REQ-CP-1: cleanupPlayer MUST be a no-op when the session in the registry has
// a different entity than the one captured at session-start time.
func TestCleanupPlayer_StaleCleanupDoesNotEvictReconnect(t *testing.T) {
	s := makeMinimalServer(t)
	const uid = "28"

	// Session A connects.
	sessA := addTestSession(t, s.sessions, uid)
	entityA := sessA.Entity

	// Session B reconnects — AddPlayer evicts Session A and creates a new entity.
	sessB := addTestSession(t, s.sessions, uid)

	// Verify Session B is registered.
	if got, ok := s.sessions.GetPlayer(uid); !ok || got.Entity != sessB.Entity {
		t.Fatal("expected Session B in registry before cleanup")
	}

	// Session A's stale cleanupPlayer fires (entity = entityA).
	// It must NOT remove Session B.
	s.cleanupPlayer(uid, "user_"+uid, entityA)

	// Session B must still be in the registry.
	got, ok := s.sessions.GetPlayer(uid)
	if !ok {
		t.Fatal("Session B was incorrectly removed by stale cleanupPlayer from Session A")
	}
	if got.Entity != sessB.Entity {
		t.Fatal("session in registry is not Session B after stale cleanup")
	}

	// Cleanup: close Session B entity to avoid goroutine leaks.
	_ = entityA.Close()
	_ = sessB.Entity.Close()
}

// TestCleanupPlayer_OwnSessionIsRemoved verifies that when no reconnect has
// occurred, cleanupPlayer correctly removes the player from the registry.
//
// REQ-CP-2: cleanupPlayer MUST remove the session when the entity matches.
func TestCleanupPlayer_OwnSessionIsRemoved(t *testing.T) {
	s := makeMinimalServer(t)
	const uid = "42"

	sess := addTestSession(t, s.sessions, uid)
	entity := sess.Entity

	s.cleanupPlayer(uid, "user_"+uid, entity)

	_, ok := s.sessions.GetPlayer(uid)
	if ok {
		t.Fatal("expected session to be removed by cleanupPlayer, but it still exists")
	}
}

// TestProperty_CleanupPlayer_StaleNeverEvictsLatestSession verifies the guard
// for arbitrary uid values: a stale cleanupPlayer with an old entity must never
// remove the current session registered for the same uid.
//
// Precondition: Sessions are created with valid uid/username/room/role.
// Postcondition: After stale cleanup, the latest session remains accessible.
func TestProperty_CleanupPlayer_StaleNeverEvictsLatestSession(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := makeMinimalServer(t)
		uid := rapid.StringMatching(`[a-z]{1,8}`).Draw(rt, "uid")

		// Session A.
		sessA := addTestSession(t, s.sessions, uid)
		entityA := sessA.Entity

		// Session B evicts Session A.
		sessB := addTestSession(t, s.sessions, uid)

		// Stale cleanup from Session A.
		s.cleanupPlayer(uid, "user_"+uid, entityA)

		// Session B must survive.
		got, ok := s.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatalf("stale cleanupPlayer removed Session B for uid %q", uid)
		}
		if got.Entity != sessB.Entity {
			rt.Fatalf("session in registry is not Session B after stale cleanup for uid %q", uid)
		}

		// Cleanup.
		_ = entityA.Close()
		_ = sessB.Entity.Close()
	})
}

