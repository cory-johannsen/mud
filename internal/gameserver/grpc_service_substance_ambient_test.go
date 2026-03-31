package gameserver_test

import (
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

// TestAmbientDoseTimer_60sIntervalEnforced verifies that ShouldApplyAmbientDose
// returns false for any elapsed time strictly less than 60 seconds.
//
// REQ-OCF-8: Ambient dose interval is 60 seconds.
func TestAmbientDoseTimer_60sIntervalEnforced(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		elapsed := rapid.IntRange(0, 59).Draw(t, "elapsed")
		lastDose := time.Now().Add(-time.Duration(elapsed) * time.Second)
		if gameserver.ShouldApplyAmbientDose(lastDose, time.Now()) {
			t.Fatalf("expected no ambient dose at %ds elapsed, got true", elapsed)
		}
	})
}

// TestAmbientDoseTimer_60sIntervalFires verifies that ShouldApplyAmbientDose
// returns true for any elapsed time >= 60 seconds.
//
// REQ-OCF-8: Ambient dose fires once per 60-second window.
func TestAmbientDoseTimer_60sIntervalFires(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		elapsed := rapid.IntRange(60, 3600).Draw(t, "elapsed")
		lastDose := time.Now().Add(-time.Duration(elapsed) * time.Second)
		if !gameserver.ShouldApplyAmbientDose(lastDose, time.Now()) {
			t.Fatalf("expected ambient dose at %ds elapsed, got false", elapsed)
		}
	})
}

// TestAmbientDoseTimer_ZeroValueAlwaysFires verifies that a zero LastAmbientDose
// always triggers a dose (first entry into an ambient room).
//
// REQ-OCF-8: Zero last-dose time means the player has never received an ambient dose.
func TestAmbientDoseTimer_ZeroValueAlwaysFires(t *testing.T) {
	var zero time.Time
	if !gameserver.ShouldApplyAmbientDose(zero, time.Now()) {
		t.Fatal("expected ambient dose for zero last-dose time, got false")
	}
}
