package combat_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestRoundTimer_Fires(t *testing.T) {
	var called atomic.Int32
	rt := combat.NewRoundTimer(20*time.Millisecond, func() {
		called.Add(1)
	})
	_ = rt
	time.Sleep(50 * time.Millisecond)
	if called.Load() != 1 {
		t.Fatalf("expected callback called once, got %d", called.Load())
	}
}

func TestRoundTimer_Stop_PreventsCallback(t *testing.T) {
	var called atomic.Int32
	rt := combat.NewRoundTimer(50*time.Millisecond, func() {
		called.Add(1)
	})
	rt.Stop()
	time.Sleep(80 * time.Millisecond)
	if called.Load() != 0 {
		t.Fatalf("expected callback not called, got %d", called.Load())
	}
}

func TestRoundTimer_Reset_ExtendsDeadline(t *testing.T) {
	var called atomic.Int32
	rt := combat.NewRoundTimer(30*time.Millisecond, func() {
		called.Add(1)
	})
	time.Sleep(15 * time.Millisecond)
	rt.Reset(30*time.Millisecond, func() {
		called.Add(1)
	})
	// At 35ms from start (15ms + 20ms), original would have fired but shouldn't have.
	time.Sleep(20 * time.Millisecond)
	if called.Load() != 0 {
		t.Fatalf("expected callback not called at 35ms, got %d", called.Load())
	}
	// Wait until 55ms from reset (15ms already elapsed + 40ms more = 55ms total)
	time.Sleep(25 * time.Millisecond)
	if called.Load() != 1 {
		t.Fatalf("expected callback called once at ~55ms, got %d", called.Load())
	}
}

func TestRoundTimer_StopIdempotent(t *testing.T) {
	rt := combat.NewRoundTimer(50*time.Millisecond, func() {})
	// Multiple Stop() calls must not panic
	rt.Stop()
	rt.Stop()
	rt.Stop()
}
