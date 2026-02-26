package inventory_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// TestMagazine_NewMagazine_FullyLoaded verifies that NewMagazine initialises
// Loaded == Capacity == 15.
func TestMagazine_NewMagazine_FullyLoaded(t *testing.T) {
	m := inventory.NewMagazine("pistol-9mm", 15)
	if m.Loaded != 15 {
		t.Fatalf("expected Loaded=15, got %d", m.Loaded)
	}
	if m.Capacity != 15 {
		t.Fatalf("expected Capacity=15, got %d", m.Capacity)
	}
}

// TestMagazine_Consume_DecreasesLoaded verifies that Consume(1) moves Loaded
// from 15 to 14.
func TestMagazine_Consume_DecreasesLoaded(t *testing.T) {
	m := inventory.NewMagazine("pistol-9mm", 15)
	if err := m.Consume(1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Loaded != 14 {
		t.Fatalf("expected Loaded=14, got %d", m.Loaded)
	}
}

// TestMagazine_Consume_FailsWhenEmpty verifies that Consume returns an error
// when the magazine is empty.
func TestMagazine_Consume_FailsWhenEmpty(t *testing.T) {
	m := inventory.NewMagazine("pistol-9mm", 1)
	if err := m.Consume(1); err != nil {
		t.Fatalf("unexpected error draining: %v", err)
	}
	if err := m.Consume(1); err == nil {
		t.Fatal("expected error consuming from empty magazine, got nil")
	}
}

// TestMagazine_Reload_RestoresToCapacity verifies that after consuming 10
// rounds from a 15-round magazine, Reload restores Loaded to 15.
func TestMagazine_Reload_RestoresToCapacity(t *testing.T) {
	m := inventory.NewMagazine("pistol-9mm", 15)
	if err := m.Consume(10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m.Reload()
	if m.Loaded != 15 {
		t.Fatalf("expected Loaded=15 after reload, got %d", m.Loaded)
	}
}

// TestMagazine_Consume_PanicsOnZeroN verifies that Consume(0) panics.
func TestMagazine_Consume_PanicsOnZeroN(t *testing.T) {
	m := inventory.NewMagazine("pistol-9mm", 15)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on Consume(0), got none")
		}
	}()
	_ = m.Consume(0) //nolint:errcheck
}

// TestProperty_Magazine_LoadedNeverExceedsCapacity is a property-based test
// that asserts Loaded ∈ [0, Capacity] for arbitrary consume/reload sequences.
func TestProperty_Magazine_LoadedNeverExceedsCapacity(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		capacity := rapid.IntRange(1, 30).Draw(rt, "capacity")
		m := inventory.NewMagazine("test-weapon", capacity)

		consume := rapid.IntRange(0, capacity).Draw(rt, "consume")
		if consume > 0 {
			_ = m.Consume(consume)
		}
		m.Reload()

		if m.Loaded < 0 || m.Loaded > m.Capacity {
			rt.Fatalf("Loaded=%d out of range [0, %d]", m.Loaded, m.Capacity)
		}
	})
}

// TestMagazine_IsEmpty_FalseWhenLoaded verifies IsEmpty returns false for a
// newly created magazine.
func TestMagazine_IsEmpty_FalseWhenLoaded(t *testing.T) {
	m := inventory.NewMagazine("pistol-9mm", 5)
	if m.IsEmpty() {
		t.Fatal("expected IsEmpty=false for fully loaded magazine, got true")
	}
}

// TestMagazine_IsEmpty_TrueWhenDrained verifies IsEmpty returns true after all
// rounds are consumed.
func TestMagazine_IsEmpty_TrueWhenDrained(t *testing.T) {
	m := inventory.NewMagazine("pistol-9mm", 3)
	if err := m.Consume(3); err != nil {
		t.Fatalf("unexpected error draining: %v", err)
	}
	if !m.IsEmpty() {
		t.Fatal("expected IsEmpty=true after draining all rounds, got false")
	}
}

// TestMagazine_NewMagazine_PanicsOnZeroCapacity verifies NewMagazine panics
// when capacity is zero.
func TestMagazine_NewMagazine_PanicsOnZeroCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on capacity=0, got none")
		}
	}()
	_ = inventory.NewMagazine("pistol-9mm", 0)
}
