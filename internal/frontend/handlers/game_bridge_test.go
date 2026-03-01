package handlers_test

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
)

func TestBuildPrompt_Format(t *testing.T) {
	got := handlers.BuildPrompt("Thorald", "Dusk", "17:00", 45, 60)
	// Must end with "> "
	if !strings.HasSuffix(got, "> ") {
		t.Errorf("prompt must end with '> ', got %q", got)
	}
	// Must contain character name
	if !strings.Contains(got, "Thorald") {
		t.Errorf("prompt must contain character name, got %q", got)
	}
	// Must contain period
	if !strings.Contains(got, "Dusk") {
		t.Errorf("prompt must contain period, got %q", got)
	}
	// Must contain hour
	if !strings.Contains(got, "17:00") {
		t.Errorf("prompt must contain hour, got %q", got)
	}
}

func TestBuildPrompt_HealthColors(t *testing.T) {
	// Full health >= 75% — contains hp fraction
	got := handlers.BuildPrompt("Thorald", "Dusk", "17:00", 60, 60)
	if !strings.Contains(got, "60/60hp") {
		t.Errorf("expected 60/60hp in prompt, got %q", got)
	}
	// Wounded ~40%
	got = handlers.BuildPrompt("Thorald", "Morning", "09:00", 24, 60)
	if !strings.Contains(got, "24/60hp") {
		t.Errorf("expected 24/60hp in prompt, got %q", got)
	}
	// Critical <40%
	got = handlers.BuildPrompt("Thorald", "Night", "22:00", 10, 60)
	if !strings.Contains(got, "10/60hp") {
		t.Errorf("expected 10/60hp in prompt, got %q", got)
	}
}

func TestBuildPrompt_AllPeriods(t *testing.T) {
	periods := []string{
		"Midnight", "Late Night", "Dawn", "Morning",
		"Afternoon", "Dusk", "Evening", "Night",
	}
	for _, p := range periods {
		got := handlers.BuildPrompt("X", p, "00:00", 10, 10)
		if got == "" {
			t.Errorf("BuildPrompt returned empty for period %q", p)
		}
		if !strings.HasSuffix(got, "> ") {
			t.Errorf("prompt for period %q must end with '> '", p)
		}
	}
}

func TestProperty_BuildPrompt_AlwaysEndsWithPromptSuffix(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z]{1,20}`).Draw(rt, "name")
		period := rapid.SampledFrom([]string{
			"Midnight", "Late Night", "Dawn", "Morning",
			"Afternoon", "Dusk", "Evening", "Night",
		}).Draw(rt, "period")
		hour := rapid.StringMatching(`[0-9]{2}:[0-9]{2}`).Draw(rt, "hour")
		maxHP := rapid.Int32Range(1, 1000).Draw(rt, "maxHP")
		currentHP := rapid.Int32Range(0, maxHP).Draw(rt, "currentHP")

		got := handlers.BuildPrompt(name, period, hour, currentHP, maxHP)
		if !strings.HasSuffix(got, "> ") {
			rt.Errorf("BuildPrompt must end with '> ', got %q", got)
		}
		if !strings.Contains(got, name) {
			rt.Errorf("BuildPrompt must contain name %q, got %q", name, got)
		}
	})
}

// TestIdleMonitor_WarningAfterIdleTimeout verifies that the idle monitor sends
// a warning callback after the idle timeout and a disconnect callback after
// the grace period.
func TestIdleMonitor_WarningAfterIdleTimeout(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().UnixNano())

	warningCalled := make(chan struct{}, 1)
	disconnectCalled := make(chan struct{}, 1)

	idleTimeout := 100 * time.Millisecond
	gracePeriod := 50 * time.Millisecond
	tickInterval := 20 * time.Millisecond

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  idleTimeout,
		GracePeriod:  gracePeriod,
		TickInterval: tickInterval,
		OnWarning: func() {
			select {
			case warningCalled <- struct{}{}:
			default:
			}
		},
		OnDisconnect: func() {
			select {
			case disconnectCalled <- struct{}{}:
			default:
			}
		},
	})
	defer stop()

	select {
	case <-warningCalled:
		// good
	case <-time.After(idleTimeout + 3*tickInterval):
		t.Fatal("warning not called within expected time")
	}

	select {
	case <-disconnectCalled:
		// good
	case <-time.After(gracePeriod + 3*tickInterval):
		t.Fatal("disconnect not called within expected time after warning")
	}
}

// TestIdleMonitor_InputResetsTimer verifies that input before the idle timeout
// prevents the warning from being sent.
func TestIdleMonitor_InputResetsTimer(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().UnixNano())

	warningCalled := make(chan struct{}, 1)

	idleTimeout := 150 * time.Millisecond
	tickInterval := 20 * time.Millisecond

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  idleTimeout,
		GracePeriod:  500 * time.Millisecond,
		TickInterval: tickInterval,
		OnWarning: func() {
			select {
			case warningCalled <- struct{}{}:
			default:
			}
		},
		OnDisconnect: func() {},
	})
	defer stop()

	// Simulate input every 50ms — well within the 150ms idle timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.After(idleTimeout + 3*tickInterval)
		for {
			select {
			case <-ticker.C:
				lastInput.Store(time.Now().UnixNano())
			case <-deadline:
				return
			}
		}
	}()
	<-done

	select {
	case <-warningCalled:
		t.Fatal("warning should not have been called while player was active")
	default:
		// good — no warning
	}
}

// TestIdleMonitor_StopPreventsCallbacks verifies that calling the stop function
// prevents any callbacks from firing.
func TestIdleMonitor_StopPreventsCallbacks(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().UnixNano()) // NOT idle — goroutine cannot fire yet

	warned := atomic.Bool{}
	disconnected := atomic.Bool{}

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  10 * time.Millisecond,
		GracePeriod:  10 * time.Millisecond,
		TickInterval: 5 * time.Millisecond,
		OnWarning:    func() { warned.Store(true) },
		OnDisconnect: func() { disconnected.Store(true) },
	})

	// Stop before the idle timeout can elapse
	stop()

	// Now make it look idle — but goroutine is already stopped
	lastInput.Store(time.Now().Add(-10 * time.Second).UnixNano())

	// Wait long enough that a running goroutine would have fired
	time.Sleep(100 * time.Millisecond)

	if warned.Load() {
		t.Error("expected no warning after stop, but OnWarning was called")
	}
	if disconnected.Load() {
		t.Error("expected no disconnect after stop, but OnDisconnect was called")
	}
}

// TestIdleMonitor_WarningOnlyOnce verifies that the warning callback is called
// exactly once even if the monitor keeps ticking.
func TestIdleMonitor_WarningOnlyOnce(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().Add(-10 * time.Second).UnixNano()) // already idle

	var warningCount atomic.Int64

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  10 * time.Millisecond,
		GracePeriod:  200 * time.Millisecond, // long grace so monitor keeps ticking
		TickInterval: 5 * time.Millisecond,
		OnWarning: func() {
			warningCount.Add(1)
		},
		OnDisconnect: func() {},
	})
	defer stop()

	time.Sleep(100 * time.Millisecond)

	if n := warningCount.Load(); n != 1 {
		t.Fatalf("expected warning called exactly once, got %d", n)
	}
}

// TestProperty_IdleMonitor_ActivePlayerNeverDisconnected verifies that
// a player who inputs at least once per half-idle-timeout is never disconnected.
func TestProperty_IdleMonitor_ActivePlayerNeverDisconnected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		var lastInput atomic.Int64
		lastInput.Store(time.Now().UnixNano())

		disconnected := make(chan struct{}, 1)

		idleTimeout := 200 * time.Millisecond
		tickInterval := 20 * time.Millisecond

		stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
			LastInput:    &lastInput,
			IdleTimeout:  idleTimeout,
			GracePeriod:  50 * time.Millisecond,
			TickInterval: tickInterval,
			OnWarning:    func() {},
			OnDisconnect: func() {
				select {
				case disconnected <- struct{}{}:
				default:
				}
			},
		})
		defer stop()

		// Simulate input every 80ms (well within 200ms idle timeout)
		inputCount := rapid.IntRange(3, 8).Draw(rt, "inputCount")
		for i := 0; i < inputCount; i++ {
			time.Sleep(80 * time.Millisecond)
			lastInput.Store(time.Now().UnixNano())
		}

		select {
		case <-disconnected:
			rt.Fatal("active player should never be disconnected")
		default:
			// good
		}
	})
}
