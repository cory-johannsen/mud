package danger_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/danger"
)

// alwaysRoller returns a fixed value for all Roll calls.
type alwaysRoller struct{ val int }

func (r alwaysRoller) Roll(_ int) int { return r.val }

func TestRollRoomTrap_SafeAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		if danger.RollRoomTrap(danger.Safe, nil, rng) {
			t.Fatal("RollRoomTrap(Safe) returned true; want false always")
		}
	})
}

func TestRollCoverTrap_SafeAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		if danger.RollCoverTrap(danger.Safe, nil, rng) {
			t.Fatal("RollCoverTrap(Safe) returned true; want false always")
		}
	})
}

func TestRollRoomTrap_ZeroRollTriggersWhenPctPositive(t *testing.T) {
	// Roll(100)=0 < any positive pct → true
	rng := alwaysRoller{val: 0}
	levels := []danger.DangerLevel{danger.Dangerous, danger.AllOutWar}
	for _, lvl := range levels {
		if !danger.RollRoomTrap(lvl, nil, rng) {
			t.Errorf("RollRoomTrap(%q, nil, roll=0): want true (pct>0), got false", lvl)
		}
	}
}

func TestRollCoverTrap_ZeroRollTriggersWhenPctPositive(t *testing.T) {
	rng := alwaysRoller{val: 0}
	levels := []danger.DangerLevel{danger.Sketchy, danger.Dangerous, danger.AllOutWar}
	for _, lvl := range levels {
		if !danger.RollCoverTrap(lvl, nil, rng) {
			t.Errorf("RollCoverTrap(%q, nil, roll=0): want true (pct>0), got false", lvl)
		}
	}
}

func TestRollRoomTrap_HighRollNeverTriggers(t *testing.T) {
	// Roll(100)=99 >= any pct < 100 → false
	rng := alwaysRoller{val: 99}
	levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
	for _, lvl := range levels {
		if danger.RollRoomTrap(lvl, nil, rng) {
			t.Errorf("RollRoomTrap(%q, nil, roll=99): want false (no pct==100), got true", lvl)
		}
	}
}

func TestRollRoomTrap_OverrideZeroAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		override := 0
		levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
		lvl := rapid.SampledFrom(levels).Draw(t, "level")
		if danger.RollRoomTrap(lvl, &override, rng) {
			t.Fatalf("RollRoomTrap(%q, &0, roll=%d): want false always, got true", lvl, roll)
		}
	})
}

func TestRollCoverTrap_OverrideZeroAlwaysFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roll := rapid.IntRange(0, 99).Draw(t, "roll")
		rng := alwaysRoller{val: roll}
		override := 0
		levels := []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar}
		lvl := rapid.SampledFrom(levels).Draw(t, "level")
		if danger.RollCoverTrap(lvl, &override, rng) {
			t.Fatalf("RollCoverTrap(%q, &0, roll=%d): want false always, got true", lvl, roll)
		}
	})
}

func TestRollRoomTrap_OverrideNonNilUsed(t *testing.T) {
	// Override of 50: roll=49 → true, roll=50 → false
	rng49 := alwaysRoller{val: 49}
	rng50 := alwaysRoller{val: 50}
	override := 50
	// Use Safe level (default 0%) to confirm override is used
	if !danger.RollRoomTrap(danger.Safe, &override, rng49) {
		t.Error("RollRoomTrap(Safe, &50, roll=49): want true (override used)")
	}
	if danger.RollRoomTrap(danger.Safe, &override, rng50) {
		t.Error("RollRoomTrap(Safe, &50, roll=50): want false")
	}
}
