package gameserver

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"pgregory.net/rapid"
)

// TestHandleArchetypeSelection_UnknownSession verifies that handleArchetypeSelection
// returns an error when no session exists for the given uid.
//
// Precondition: No player session exists for the given uid.
// Postcondition: Returns a non-nil error containing "session not found".
func TestHandleArchetypeSelection_UnknownSession(t *testing.T) {
	mgr := session.NewManager()
	s := &GameServiceServer{sessions: mgr}

	result, err := s.handleArchetypeSelection("unknown-uid", &gamev1.ArchetypeSelectionRequest{ArchetypeId: "aggressor"})
	if err == nil {
		t.Fatal("expected error for unknown session, got nil")
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Errorf("error %q does not contain %q", err.Error(), "session not found")
	}
	if result != nil {
		t.Errorf("expected nil result for unknown session, got %v", result)
	}
}

// TestHandleArchetypeSelection_KnownSession verifies that handleArchetypeSelection
// returns an empty ServerEvent and no error when a session exists.
//
// Precondition: A player session exists for the given uid.
// Postcondition: Returns a non-nil empty ServerEvent and nil error.
func TestHandleArchetypeSelection_KnownSession(t *testing.T) {
	mgr := session.NewManager()
	_, _ = mgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            "room1",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the North",
		Class:             "Aggressor",
		Level:             1,
	})
	s := &GameServiceServer{sessions: mgr}

	result, err := s.handleArchetypeSelection("uid1", &gamev1.ArchetypeSelectionRequest{ArchetypeId: "aggressor"})
	if err != nil {
		t.Fatalf("handleArchetypeSelection returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ServerEvent, got nil")
	}
}

// TestProperty_HandleArchetypeSelection_NeverPanics is a property-based test verifying
// that handleArchetypeSelection never panics regardless of the archetype ID provided.
//
// Precondition: A session manager exists; archetypeID is an arbitrary string.
// Postcondition: handleArchetypeSelection never panics.
func TestProperty_HandleArchetypeSelection_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		archetypeID := rapid.String().Draw(rt, "archetype_id")
		mgr := session.NewManager()
		s := &GameServiceServer{sessions: mgr}

		// Must not panic regardless of input.
		defer func() {
			if r := recover(); r != nil {
				rt.Fatalf("handleArchetypeSelection panicked: %v", r)
			}
		}()
		_, _ = s.handleArchetypeSelection("unknown-uid", &gamev1.ArchetypeSelectionRequest{ArchetypeId: archetypeID})
	})
}
