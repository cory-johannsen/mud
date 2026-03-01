package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"pgregory.net/rapid"
)

// TestHandleSwitch_ReturnsDisconnectedWithReason verifies that handleSwitch returns
// a Disconnected event with a non-empty reason and errQuit for a known player.
//
// Precondition: A player session exists for the given uid.
// Postcondition: Returns errQuit and a Disconnected payload with non-empty Reason.
func TestHandleSwitch_ReturnsDisconnectedWithReason(t *testing.T) {
	mgr := session.NewManager()
	_, _ = mgr.AddPlayer("uid1", "user1", "Hero", 1, "room1", 10, "player", "the Northeast", "Gunner", 3)
	s := &GameServiceServer{sessions: mgr}

	result, err := s.handleSwitch("uid1")
	if err != errQuit {
		t.Fatalf("expected errQuit, got %v", err)
	}
	d := result.GetDisconnected()
	if d == nil {
		t.Fatal("expected Disconnected payload")
	}
	if d.Reason == "" {
		t.Error("Reason must be non-empty")
	}
}

// TestHandleSwitch_UnknownUID_StillReturnsDisconnected verifies that handleSwitch returns
// errQuit and a Disconnected payload even when the uid is not in any session.
//
// Precondition: No player session exists for the given uid.
// Postcondition: Returns errQuit and a Disconnected payload.
func TestHandleSwitch_UnknownUID_StillReturnsDisconnected(t *testing.T) {
	mgr := session.NewManager()
	s := &GameServiceServer{sessions: mgr}

	result, err := s.handleSwitch("nobody")
	if err != errQuit {
		t.Fatalf("expected errQuit, got %v", err)
	}
	if result.GetDisconnected() == nil {
		t.Fatal("expected Disconnected payload")
	}
}

// TestProperty_HandleSwitch_AlwaysReturnsErrQuit is a property-based test verifying that
// handleSwitch always returns errQuit regardless of the uid or character name provided.
//
// Precondition: A session manager exists; uid and charName are arbitrary non-empty strings.
// Postcondition: handleSwitch always returns errQuit.
func TestProperty_HandleSwitch_AlwaysReturnsErrQuit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		mgr := session.NewManager()
		uid := rapid.StringMatching(`[a-z0-9]+`).Draw(t, "uid")
		charName := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "charName")
		_, _ = mgr.AddPlayer(uid, "user", charName, 1, "room1", 10, "player", "the Northeast", "Gunner", 1)
		s := &GameServiceServer{sessions: mgr}

		_, err := s.handleSwitch(uid)
		if err != errQuit {
			t.Fatalf("handleSwitch must return errQuit, got %v", err)
		}
	})
}
