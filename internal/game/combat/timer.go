package combat

import (
	"sync"
	"time"
)

// RoundTimer fires a callback after a configurable duration unless stopped.
// It is safe for concurrent use.
type RoundTimer struct {
	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
}

// NewRoundTimer creates and starts a timer that calls onFire after duration.
// onFire is called in a separate goroutine.
//
// Precondition: duration > 0; onFire must not be nil.
// Postcondition: Returns a running RoundTimer; onFire will be called unless Stop is called first.
func NewRoundTimer(duration time.Duration, onFire func()) *RoundTimer {
	rt := &RoundTimer{}
	rt.timer = time.AfterFunc(duration, func() {
		rt.mu.Lock()
		stopped := rt.stopped
		rt.mu.Unlock()
		if !stopped {
			onFire()
		}
	})
	return rt
}

// Reset cancels the current timer and starts a new one with the provided duration and callback.
//
// Precondition: duration > 0; onFire must not be nil.
// Postcondition: onFire will be called after duration from now unless Stop is called first.
func (rt *RoundTimer) Reset(duration time.Duration, onFire func()) {
	rt.mu.Lock()
	rt.stopped = false
	rt.timer.Stop()
	rt.mu.Unlock()

	newTimer := time.AfterFunc(duration, func() {
		rt.mu.Lock()
		s := rt.stopped
		rt.mu.Unlock()
		if !s {
			onFire()
		}
	})

	rt.mu.Lock()
	rt.timer = newTimer
	rt.mu.Unlock()
}

// Stop prevents the callback from firing. Safe to call multiple times.
//
// Postcondition: onFire will not be called after Stop returns.
func (rt *RoundTimer) Stop() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.stopped = true
	rt.timer.Stop()
}
