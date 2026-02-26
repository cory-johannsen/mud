package gameserver_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestZoneTickManager_StartsAndStops(t *testing.T) {
	zm := gameserver.NewZoneTickManager(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	zm.Start(ctx)
	time.Sleep(120 * time.Millisecond)
	cancel()
	// Should not block or panic after cancel
}

func TestZoneTickManager_TickCallbackInvoked(t *testing.T) {
	zm := gameserver.NewZoneTickManager(20 * time.Millisecond)
	called := make(chan struct{}, 1)
	zm.RegisterTick("zone1", func() {
		select {
		case called <- struct{}{}:
		default:
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	zm.Start(ctx)
	select {
	case <-called:
		// success
	case <-ctx.Done():
		t.Fatal("tick callback not invoked within timeout")
	}
}

func TestZoneTickManager_UnregisterStopsCallback(t *testing.T) {
	zm := gameserver.NewZoneTickManager(20 * time.Millisecond)
	var count atomic.Int64
	zm.RegisterTick("z1", func() { count.Add(1) })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	zm.Start(ctx)
	time.Sleep(60 * time.Millisecond)
	zm.Unregister("z1")
	countAfterUnregister := count.Load()
	time.Sleep(60 * time.Millisecond)
	if count.Load() > countAfterUnregister+1 {
		t.Fatalf("tick continued after unregister: before=%d after=%d", countAfterUnregister, count.Load())
	}
}
