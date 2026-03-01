package gameserver

import (
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/game/session"
	"pgregory.net/rapid"
)

func TestHandleExamine_PlayerTarget_ReturnsCharacterInfo(t *testing.T) {
	mgr := session.NewManager()
	// Add examiner in room1
	_, _ = mgr.AddPlayer("uid1", "user1", "Hero", 1, "room1", 10, "player", "the Northeast", "Gunner", 3)
	// Add target in same room
	_, _ = mgr.AddPlayer("uid2", "user2", "Villain", 2, "room1", 8, "player", "the Midwest", "Machete", 2)

	s := &GameServiceServer{sessions: mgr}
	result, err := s.handleExamine("uid1", &gamev1.ExamineRequest{Target: "Villain"})
	if err != nil {
		t.Fatalf("handleExamine error: %v", err)
	}
	ci, ok := result.Payload.(*gamev1.ServerEvent_CharacterInfo)
	if !ok {
		t.Fatalf("expected CharacterInfo payload, got %T", result.Payload)
	}
	if ci.CharacterInfo.Name != "Villain" {
		t.Errorf("Name = %q, want %q", ci.CharacterInfo.Name, "Villain")
	}
	if ci.CharacterInfo.Region != "the Midwest" {
		t.Errorf("Region = %q, want %q", ci.CharacterInfo.Region, "the Midwest")
	}
}

func TestHandleExamine_PlayerInDifferentRoom_FallsBackToNPC(t *testing.T) {
	// Players in different rooms should not match — this is handled by the NPC fallback
	// Since there's no NPC handler set up, the fallback will error — that's acceptable for this test
	mgr := session.NewManager()
	_, _ = mgr.AddPlayer("uid1", "user1", "Hero", 1, "room1", 10, "player", "the Northeast", "Gunner", 3)
	_, _ = mgr.AddPlayer("uid2", "user2", "Villain", 2, "room2", 8, "player", "the Midwest", "Machete", 2)

	s := &GameServiceServer{sessions: mgr}
	// Villain is in a different room — should not return CharacterInfo, will try NPC path
	result, err := s.handleExamine("uid1", &gamev1.ExamineRequest{Target: "Villain"})
	// We expect either an error (no NPC handler) or an NpcView — NOT CharacterInfo
	if err == nil {
		if _, ok := result.Payload.(*gamev1.ServerEvent_CharacterInfo); ok {
			t.Error("should not return CharacterInfo for player in different room")
		}
	}
	// An error is acceptable here since npcH is nil
}

func TestProperty_HandleExamine_PlayerTargetSameRoom_AlwaysCharacterInfo(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		mgr := session.NewManager()
		regionDisplay := rapid.StringMatching(`[A-Za-z ]+`).Draw(t, "regionDisplay")
		targetName := rapid.StringMatching(`[A-Za-z]+`).Draw(t, "targetName")
		_, _ = mgr.AddPlayer("uid1", "user1", "Hero", 1, "room1", 10, "player", "the Northeast", "Gunner", 3)
		_, _ = mgr.AddPlayer("uid2", "user2", targetName, 2, "room1", 8, "player", regionDisplay, "Machete", 2)

		s := &GameServiceServer{sessions: mgr}
		result, err := s.handleExamine("uid1", &gamev1.ExamineRequest{Target: targetName})
		if err != nil {
			t.Fatalf("handleExamine error: %v", err)
		}
		ci, ok := result.Payload.(*gamev1.ServerEvent_CharacterInfo)
		if !ok {
			t.Fatalf("expected CharacterInfo, got %T", result.Payload)
		}
		if ci.CharacterInfo.Region != regionDisplay {
			t.Fatalf("Region = %q, want %q", ci.CharacterInfo.Region, regionDisplay)
		}
	})
}
