package danger_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/danger"
)

func TestCanInitiateCombat_Table(t *testing.T) {
	tests := []struct {
		level     danger.DangerLevel
		initiator string
		want      bool
	}{
		{danger.Safe, "player", false},
		{danger.Safe, "npc", false},
		{danger.Sketchy, "player", true},
		{danger.Sketchy, "npc", false},
		{danger.Dangerous, "player", true},
		{danger.Dangerous, "npc", true},
		{danger.AllOutWar, "player", true},
		{danger.AllOutWar, "npc", true},
	}
	for _, tc := range tests {
		t.Run(string(tc.level)+"/"+tc.initiator, func(t *testing.T) {
			got := danger.CanInitiateCombat(tc.level, tc.initiator)
			if got != tc.want {
				t.Errorf("CanInitiateCombat(%q, %q) = %v; want %v", tc.level, tc.initiator, got, tc.want)
			}
		})
	}
}

func TestCanInitiateCombat_Property(t *testing.T) {
	levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
	initiators := []string{"player", "npc"}

	rapid.Check(t, func(t *rapid.T) {
		level := rapid.SampledFrom(levels).Draw(t, "level")
		initiator := rapid.SampledFrom(initiators).Draw(t, "initiator")
		result := danger.CanInitiateCombat(level, initiator)

		// Safe: no one may initiate
		if level == danger.Safe && result {
			t.Fatalf("CanInitiateCombat(Safe, %q) = true; want false", initiator)
		}
		// Sketchy: npc may never initiate
		if level == danger.Sketchy && initiator == "npc" && result {
			t.Fatalf("CanInitiateCombat(Sketchy, npc) = true; want false")
		}
		// Dangerous/AllOutWar: everyone may initiate
		if (level == danger.Dangerous || level == danger.AllOutWar) && !result {
			t.Fatalf("CanInitiateCombat(%q, %q) = false; want true", level, initiator)
		}
	})
}
