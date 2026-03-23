package behavior_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc/behavior"
)

func TestParseHours_Range(t *testing.T) {
	hours, err := behavior.ParseHours("6-18")
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 13 {
		t.Fatalf("expected 13 hours (6..18), got %d", len(hours))
	}
	if hours[0] != 6 || hours[len(hours)-1] != 18 {
		t.Fatalf("expected 6..18, got %d..%d", hours[0], hours[len(hours)-1])
	}
}

func TestParseHours_WrapMidnight(t *testing.T) {
	hours, err := behavior.ParseHours("19-5")
	if err != nil {
		t.Fatal(err)
	}
	// 19,20,21,22,23,0,1,2,3,4,5 = 11 hours
	if len(hours) != 11 {
		t.Fatalf("expected 11 hours (19..23 + 0..5), got %d", len(hours))
	}
}

func TestParseHours_CommaSeparated(t *testing.T) {
	hours, err := behavior.ParseHours("8,12,20")
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 3 {
		t.Fatalf("expected 3 hours, got %d", len(hours))
	}
}

func TestParseHours_Invalid(t *testing.T) {
	_, err := behavior.ParseHours("abc")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestActiveEntry_MatchesRange(t *testing.T) {
	entries := []behavior.ScheduleEntry{
		{Hours: "6-18", PreferredRoom: "market", BehaviorMode: "patrol"},
		{Hours: "19-5", PreferredRoom: "barracks", BehaviorMode: "idle"},
	}
	e := behavior.ActiveEntry(entries, 12)
	if e == nil || e.PreferredRoom != "market" {
		t.Fatal("expected market entry at hour 12")
	}
	e = behavior.ActiveEntry(entries, 22)
	if e == nil || e.PreferredRoom != "barracks" {
		t.Fatal("expected barracks entry at hour 22")
	}
	e = behavior.ActiveEntry(entries, 3)
	if e == nil || e.PreferredRoom != "barracks" {
		t.Fatal("expected barracks entry at hour 3 (wrap)")
	}
}

func TestActiveEntry_NoMatch(t *testing.T) {
	entries := []behavior.ScheduleEntry{
		{Hours: "8,12", PreferredRoom: "office", BehaviorMode: "idle"},
	}
	e := behavior.ActiveEntry(entries, 10)
	if e != nil {
		t.Fatal("expected nil at hour 10")
	}
}

func TestProperty_ActiveEntry_NilWhenEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		e := behavior.ActiveEntry(nil, hour)
		if e != nil {
			rt.Fatalf("expected nil for empty schedule at hour %d", hour)
		}
	})
}
