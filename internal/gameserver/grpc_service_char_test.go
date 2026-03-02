package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"pgregory.net/rapid"
)

func TestHandleChar_HappyPath_ReturnsCharacterSheetView(t *testing.T) {
	mgr := session.NewManager()
	_, _ = mgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            "room1",
		CurrentHP:         15,
		MaxHP:             20,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the North",
		Class:             "Gunner",
		Level:             3,
	})

	s := &GameServiceServer{sessions: mgr}
	result, err := s.handleChar("uid1")
	if err != nil {
		t.Fatalf("handleChar returned unexpected error: %v", err)
	}
	cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
	if !ok {
		t.Fatalf("expected ServerEvent_CharacterSheet payload, got %T", result.Payload)
	}
	if cs.CharacterSheet.Name != "Hero" {
		t.Errorf("Name = %q, want %q", cs.CharacterSheet.Name, "Hero")
	}
	if cs.CharacterSheet.Level != 3 {
		t.Errorf("Level = %d, want 3", cs.CharacterSheet.Level)
	}
	if cs.CharacterSheet.CurrentHp != 15 {
		t.Errorf("CurrentHp = %d, want 15", cs.CharacterSheet.CurrentHp)
	}
	if cs.CharacterSheet.MaxHp != 20 {
		t.Errorf("MaxHp = %d, want 20", cs.CharacterSheet.MaxHp)
	}
}

func TestHandleChar_SessionNotFound_ReturnsErrorEvent(t *testing.T) {
	mgr := session.NewManager()
	s := &GameServiceServer{sessions: mgr}
	result, err := s.handleChar("nonexistent-uid")
	if err != nil {
		t.Fatalf("handleChar returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for missing session")
	}
	if _, ok := result.Payload.(*gamev1.ServerEvent_Error); !ok {
		t.Fatalf("expected ServerEvent_Error payload for missing session, got %T", result.Payload)
	}
}

func TestHandleChar_NilJobRegistry_DoesNotPanic_ReturnsJobAsClass(t *testing.T) {
	mgr := session.NewManager()
	_, _ = mgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Ranger",
		CharacterID:       1,
		RoomID:            "room1",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the West",
		Class:             "Scout",
		Level:             1,
	})

	// jobRegistry is explicitly nil — must not panic.
	s := &GameServiceServer{sessions: mgr, jobRegistry: nil}
	result, err := s.handleChar("uid1")
	if err != nil {
		t.Fatalf("handleChar returned unexpected error: %v", err)
	}
	cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
	if !ok {
		t.Fatalf("expected ServerEvent_CharacterSheet payload, got %T", result.Payload)
	}
	if cs.CharacterSheet.Job != "Scout" {
		t.Errorf("Job = %q, want %q (class fallback)", cs.CharacterSheet.Job, "Scout")
	}
}

func TestProperty_HandleChar_NilJobRegistry_AlwaysReturnsClassAsJob(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		className := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "className")
		charName := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "charName")

		mgr := session.NewManager()
		_, _ = mgr.AddPlayer(session.AddPlayerOptions{
			UID:               "uid1",
			Username:          "user1",
			CharName:          charName,
			CharacterID:       1,
			RoomID:            "room1",
			CurrentHP:         10,
			MaxHP:             10,
			Abilities:         character.AbilityScores{},
			Role:              "player",
			RegionDisplayName: "the East",
			Class:             className,
			Level:             1,
		})

		s := &GameServiceServer{sessions: mgr, jobRegistry: nil}
		result, err := s.handleChar("uid1")
		if err != nil {
			t.Fatalf("handleChar error: %v", err)
		}
		cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
		if !ok {
			t.Fatalf("expected CharacterSheet, got %T", result.Payload)
		}
		if cs.CharacterSheet.Job != className {
			t.Fatalf("Job = %q, want %q", cs.CharacterSheet.Job, className)
		}
		if cs.CharacterSheet.Name != charName {
			t.Fatalf("Name = %q, want %q", cs.CharacterSheet.Name, charName)
		}
	})
}
