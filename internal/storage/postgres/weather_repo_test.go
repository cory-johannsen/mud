package postgres_test

import (
	"context"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func truncateWeatherEvents(t *testing.T) {
	t.Helper()
	db := testDB(t)
	_, err := db.Exec(context.Background(), `DELETE FROM weather_events`)
	if err != nil {
		t.Fatalf("truncateWeatherEvents: %v", err)
	}
}

func TestWeatherRepo_RoundTrip(t *testing.T) {
	db := testDB(t)
	truncateWeatherEvents(t)
	repo := postgres.NewWeatherRepo(db)
	ctx := context.Background()

	// Initially no active event
	ev, err := repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if ev != nil {
		t.Fatalf("expected nil active event, got %+v", ev)
	}

	// Start an event
	if err := repo.StartEvent(ctx, "blizzard", 500); err != nil {
		t.Fatalf("StartEvent: %v", err)
	}

	// Retrieve active event
	ev, err = repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if ev == nil {
		t.Fatal("expected active event, got nil")
	}
	if ev.WeatherType != "blizzard" || ev.EndTick != 500 {
		t.Errorf("unexpected event: %+v", ev)
	}

	// End the event (with cooldown)
	if err := repo.EndEvent(ctx, 600); err != nil {
		t.Fatalf("EndEvent: %v", err)
	}

	// No active event after ending
	ev, err = repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive after end: %v", err)
	}
	if ev != nil {
		t.Fatalf("expected nil after end, got %+v", ev)
	}

	// Cooldown is set
	endTick, found, err := repo.GetCooldownEnd(ctx)
	if err != nil {
		t.Fatalf("GetCooldownEnd: %v", err)
	}
	if !found {
		t.Fatal("expected cooldown to be found")
	}
	if endTick != 600 {
		t.Errorf("expected cooldown endTick=600, got %d", endTick)
	}

	// Clear expired
	if err := repo.ClearExpired(ctx); err != nil {
		t.Fatalf("ClearExpired: %v", err)
	}

	// Verify ClearExpired removed the cooldown row
	endTick2, found2, err2 := repo.GetCooldownEnd(ctx)
	if err2 != nil {
		t.Fatalf("GetCooldownEnd after clear: %v", err2)
	}
	if found2 {
		t.Errorf("expected no cooldown after ClearExpired, got endTick=%d", endTick2)
	}
}

func TestWeatherRepo_Property_RoundTrip(t *testing.T) {
	outerT := t
	rapid.Check(t, func(t *rapid.T) {
		db := testDB(outerT)
		repo := postgres.NewWeatherRepo(db)
		ctx := context.Background()

		// Truncate between rapid iterations to ensure clean state
		if _, err := db.Exec(ctx, `DELETE FROM weather_events`); err != nil {
			t.Fatalf("truncate weather_events: %v", err)
		}

		weatherType := rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "weatherType")
		endTick := rapid.Int64Range(1, 10000).Draw(t, "endTick")
		cooldownEnd := rapid.Int64Range(endTick+1, endTick+1000).Draw(t, "cooldownEnd")

		// Start → End → Clear lifecycle
		if err := repo.StartEvent(ctx, weatherType, endTick); err != nil {
			t.Fatalf("StartEvent: %v", err)
		}
		ev, err := repo.GetActive(ctx)
		if err != nil {
			t.Fatalf("GetActive: %v", err)
		}
		if ev == nil || ev.WeatherType != weatherType || ev.EndTick != endTick {
			t.Fatalf("GetActive mismatch: got %+v", ev)
		}
		if err := repo.EndEvent(ctx, cooldownEnd); err != nil {
			t.Fatalf("EndEvent: %v", err)
		}
		tick, found, err := repo.GetCooldownEnd(ctx)
		if err != nil {
			t.Fatalf("GetCooldownEnd: %v", err)
		}
		if !found || tick != cooldownEnd {
			t.Fatalf("GetCooldownEnd: found=%v tick=%d want %d", found, tick, cooldownEnd)
		}
		if err := repo.ClearExpired(ctx); err != nil {
			t.Fatalf("ClearExpired: %v", err)
		}
		_, found2, err := repo.GetCooldownEnd(ctx)
		if err != nil {
			t.Fatalf("GetCooldownEnd after clear: %v", err)
		}
		if found2 {
			t.Fatal("cooldown still found after ClearExpired")
		}
	})
}
