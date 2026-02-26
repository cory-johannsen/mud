package gameserver

import (
	"context"
	"sync"
	"time"
)

// ZoneTickManager runs a periodic tick for each registered zone.
// Each zone's tick callback is invoked sequentially within its own goroutine.
//
// Invariant: all callbacks are invoked at most once per tick interval.
type ZoneTickManager struct {
	interval time.Duration
	mu       sync.Mutex
	ticks    map[string]func()
}

// NewZoneTickManager returns a manager that fires ticks every interval.
//
// Precondition: interval must be > 0.
func NewZoneTickManager(interval time.Duration) *ZoneTickManager {
	if interval <= 0 {
		panic("gameserver.NewZoneTickManager: interval must be > 0")
	}
	return &ZoneTickManager{
		interval: interval,
		ticks:    make(map[string]func()),
	}
}

// RegisterTick registers a callback for zoneID. Replaces any existing callback.
func (z *ZoneTickManager) RegisterTick(zoneID string, fn func()) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.ticks[zoneID] = fn
}

// Unregister removes the tick callback for zoneID.
func (z *ZoneTickManager) Unregister(zoneID string) {
	z.mu.Lock()
	defer z.mu.Unlock()
	delete(z.ticks, zoneID)
}

// Start begins the tick loop. Runs until ctx is cancelled.
//
// Postcondition: all registered tick callbacks are invoked once per interval.
func (z *ZoneTickManager) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(z.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				z.mu.Lock()
				callbacks := make(map[string]func(), len(z.ticks))
				for k, v := range z.ticks {
					callbacks[k] = v
				}
				z.mu.Unlock()
				for _, fn := range callbacks {
					fn()
				}
			}
		}
	}()
}
