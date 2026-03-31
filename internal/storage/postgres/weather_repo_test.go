package postgres_test

import (
	"context"
	"testing"

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
}
