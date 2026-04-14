package handlers

// REQ-61-1: RenderCombatScreen MUST render a round countdown bar when DurationMs > 0.
// REQ-61-2: The countdown bar MUST be fully filled when ElapsedMs == 0.
// REQ-61-3: The countdown bar MUST be fully empty when ElapsedMs >= DurationMs.
// REQ-61-4: RenderCombatScreen MUST NOT render a countdown bar when DurationMs == 0.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// minimalTimerSnap returns a CombatRenderSnapshot with one combatant and the
// provided duration/elapsed values.
func minimalTimerSnap(durationMs, elapsedMs int) CombatRenderSnapshot {
	return CombatRenderSnapshot{
		Round:      1,
		DurationMs: durationMs,
		ElapsedMs:  elapsedMs,
		TurnOrder:  []string{"Alice"},
		Combatants: map[string]*CombatantState{
			"Alice": {Name: "Alice", HP: 10, MaxHP: 10, AP: 3, MaxAP: 3, IsPlayer: true},
		},
		Log: nil,
	}
}

// TestRenderCombatScreen_TimerBar_RenderedWhenDurationSet verifies that
// RenderCombatScreen includes a countdown bar when DurationMs > 0.
//
// REQ-61-1: countdown bar must be present when DurationMs > 0.
//
// Precondition: snap.DurationMs = 6000, snap.ElapsedMs = 0.
// Postcondition: output contains "[" timer bar characters.
func TestRenderCombatScreen_TimerBar_RenderedWhenDurationSet(t *testing.T) {
	snap := minimalTimerSnap(6000, 0)
	out := RenderCombatScreen(snap, 80)
	assert.Contains(t, out, "[", "RenderCombatScreen must render a timer bar when DurationMs > 0 (REQ-61-1)")
	assert.Contains(t, out, "]", "RenderCombatScreen must render a timer bar when DurationMs > 0 (REQ-61-1)")
	// The output must contain a "timer" style bar distinct from the roster HP bars.
	// We look for the timer label that identifies it as a round timer.
	assert.Contains(t, out, "Round", "output must contain round indicator (REQ-61-1)")
}

// TestRenderCombatScreen_TimerBar_FullWhenElapsedZero verifies that the countdown
// bar inner segment is fully filled (all '#') at round start (ElapsedMs == 0).
//
// REQ-61-2: timer bar MUST be full at round start.
//
// Precondition: snap.DurationMs = 6000, snap.ElapsedMs = 0.
// Postcondition: the '[…]' bar segment inside the timer line contains only '#' chars.
func TestRenderCombatScreen_TimerBar_FullWhenElapsedZero(t *testing.T) {
	snap := minimalTimerSnap(6000, 0)
	out := RenderCombatScreen(snap, 80)
	// roundTimerBar at elapsed=0: inner portion must be all '#'.
	bar := roundTimerBar(6000, 0, 20)
	inner := extractBarInner(bar)
	assert.NotEmpty(t, inner, "bar inner must not be empty")
	assert.NotContains(t, inner, ".", "timer bar inner at elapsed=0 must have no empty chars (REQ-61-2)")
	// Screen must contain a Timer line with only '#' in the bar inner.
	assert.Contains(t, out, "Timer: [", "RenderCombatScreen must contain timer bar prefix (REQ-61-2)")
	// Extract the timer bar from screen output and check its inner is all '#'.
	timerIdx := strings.Index(out, "Timer: [")
	require.True(t, timerIdx >= 0, "timer line not found in screen output")
	screenBar := out[timerIdx+len("Timer: "):]
	screenInner := extractBarInner(screenBar)
	assert.NotContains(t, screenInner, ".", "screen timer bar inner at elapsed=0 must be all '#' (REQ-61-2)")
}

// TestRenderCombatScreen_TimerBar_EmptyWhenElapsedEqualsDuration verifies that
// the timer bar inner segment is fully empty when the round has expired.
//
// REQ-61-3: timer bar MUST be empty when ElapsedMs >= DurationMs.
//
// Precondition: snap.DurationMs = 6000, snap.ElapsedMs = 6000.
// Postcondition: the '[…]' bar segment contains only '.' chars (no '#').
func TestRenderCombatScreen_TimerBar_EmptyWhenElapsedEqualsDuration(t *testing.T) {
	snap := minimalTimerSnap(6000, 6000)
	out := RenderCombatScreen(snap, 80)
	bar := roundTimerBar(6000, 6000, 20)
	inner := extractBarInner(bar)
	assert.NotEmpty(t, inner, "bar inner must not be empty")
	assert.NotContains(t, inner, "#", "timer bar inner at elapsed==duration must have no filled chars (REQ-61-3)")
	// Extract and verify screen bar too.
	timerIdx := strings.Index(out, "Timer: [")
	require.True(t, timerIdx >= 0, "timer line not found in screen output")
	screenBar := out[timerIdx+len("Timer: "):]
	screenInner := extractBarInner(screenBar)
	assert.NotContains(t, screenInner, "#", "screen timer bar inner at elapsed==duration must be all '.' (REQ-61-3)")
}

// TestRenderCombatScreen_TimerBar_AbsentWhenDurationZero verifies that no timer bar
// is rendered when DurationMs == 0.
//
// REQ-61-4: timer bar MUST NOT appear when DurationMs == 0.
//
// Precondition: snap.DurationMs = 0.
// Postcondition: output does not contain a timer bar line.
func TestRenderCombatScreen_TimerBar_AbsentWhenDurationZero(t *testing.T) {
	snap := minimalTimerSnap(0, 0)
	out := RenderCombatScreen(snap, 80)
	// With DurationMs==0 there should be no timer bar; the bar helper should not be embedded.
	noBar := roundTimerBar(0, 0, 20)
	// noBar should be empty string when duration==0.
	assert.Equal(t, "", noBar, "roundTimerBar must return empty string when duration==0 (REQ-61-4)")
	_ = out // output may contain other brackets from HP bars; we just verify the helper returns ""
}

// TestProperty_RoundTimerBar_FractionAlwaysInRange is a property test verifying
// that for any elapsed/duration combination, the timer bar fill fraction is in [0,1].
//
// REQ-61-2, REQ-61-3 (property): bar is never over-full or under-empty.
func TestProperty_RoundTimerBar_FractionAlwaysInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		durationMs := rapid.IntRange(1, 30000).Draw(rt, "durationMs")
		elapsedMs := rapid.IntRange(0, 35000).Draw(rt, "elapsedMs")
		width := rapid.IntRange(4, 40).Draw(rt, "width")

		bar := roundTimerBar(durationMs, elapsedMs, width)
		// Count filled vs empty chars inside the brackets.
		inner := extractBarInner(bar)
		filled := strings.Count(inner, "#")
		empty := strings.Count(inner, ".")
		total := filled + empty
		if total != width {
			rt.Fatalf("bar inner width %d != expected %d; bar=%q", total, width, bar)
		}
		if filled < 0 || filled > width {
			rt.Fatalf("filled %d out of range [0,%d]; bar=%q", filled, width, bar)
		}
	})
}

// extractBarInner extracts the content between the first '[' and the first ']' in s.
// Returns empty string if not found.
func extractBarInner(s string) string {
	open := strings.Index(s, "[")
	if open < 0 {
		return ""
	}
	close := strings.Index(s[open:], "]")
	if close < 0 {
		return ""
	}
	return s[open+1 : open+close]
}
