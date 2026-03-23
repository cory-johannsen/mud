package behavior_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc/behavior"
)

func TestThreatScore_BasicFormula(t *testing.T) {
	// party of 2 players avg level 5, npc level 3, avg hp 100%
	// score = (5-3) + (2-1)*2 - floor((1-1.0)*3) = 2+2-0 = 4
	players := []behavior.PlayerSnapshot{
		{Level: 5, CurrentHP: 10, MaxHP: 10},
		{Level: 5, CurrentHP: 10, MaxHP: 10},
	}
	got := behavior.ThreatScore(players, 3)
	if got != 4 {
		t.Fatalf("expected 4, got %d", got)
	}
}

func TestThreatScore_InjuredParty(t *testing.T) {
	// party of 1 player avg level 3, npc level 3, avg hp 50%
	// score = (3-3) + (1-1)*2 - floor((1-0.5)*3) = 0+0 - floor(1.5) = 0-1 = -1
	players := []behavior.PlayerSnapshot{
		{Level: 3, CurrentHP: 5, MaxHP: 10},
	}
	got := behavior.ThreatScore(players, 3)
	if got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
}

func TestThreatScore_EmptyPlayers_ReturnsZero(t *testing.T) {
	got := behavior.ThreatScore(nil, 5)
	if got != 0 {
		t.Fatalf("expected 0 for empty players, got %d", got)
	}
}

func TestProperty_ThreatScore_WoundedPartyReducesScore(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		npcLevel := rapid.IntRange(1, 20).Draw(rt, "npcLevel")
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		level := rapid.IntRange(1, 20).Draw(rt, "level")

		fullHP := make([]behavior.PlayerSnapshot, n)
		halfHP := make([]behavior.PlayerSnapshot, n)
		for i := range fullHP {
			fullHP[i] = behavior.PlayerSnapshot{Level: level, CurrentHP: 10, MaxHP: 10}
			halfHP[i] = behavior.PlayerSnapshot{Level: level, CurrentHP: 5, MaxHP: 10}
		}

		full := behavior.ThreatScore(fullHP, npcLevel)
		half := behavior.ThreatScore(halfHP, npcLevel)

		if half > full {
			rt.Fatalf("wounded party score %d exceeded full-hp score %d", half, full)
		}
	})
}
