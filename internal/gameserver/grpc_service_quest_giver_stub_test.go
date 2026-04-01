package gameserver

import (
	"context"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestHandleQuestGiver_ReturnsStubMessage(t *testing.T) {
	msg, err := HandleQuestGiverInteract(context.Background(), "juggalo_quest_giver", "player1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantSubstr = "time isn't right yet"
	if !strings.Contains(msg, wantSubstr) {
		t.Fatalf("expected message containing %q, got %q", wantSubstr, msg)
	}
}

func TestHandleQuestGiver_AllFactionQuestGivers_ReturnStub(t *testing.T) {
	givers := []string{
		"juggalo_quest_giver",
		"tweaker_quest_giver",
		"wook_quest_giver",
	}
	for _, g := range givers {
		msg, err := HandleQuestGiverInteract(context.Background(), g, "player1")
		if err != nil {
			t.Fatalf("giver %q: unexpected error: %v", g, err)
		}
		if msg == "" {
			t.Fatalf("giver %q: expected non-empty message", g)
		}
	}
}

func TestProperty_HandleQuestGiver_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		giverID := rapid.String().Draw(rt, "giver_id")
		playerID := rapid.String().Draw(rt, "player_id")
		_, _ = HandleQuestGiverInteract(context.Background(), giverID, playerID)
	})
}
