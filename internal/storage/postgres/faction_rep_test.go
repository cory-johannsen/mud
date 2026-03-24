package postgres_test

import (
	"context"
	"testing"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestFactionRepository_SaveAndLoad(t *testing.T) {
	db := testDB(t)
	repo := pgstore.NewFactionRepRepository(db)
	ctx := context.Background()

	charRepo := NewCharacterRepository(db)
	charID := createTestCharacter(t, charRepo, ctx).ID

	// LoadRep on empty returns empty map.
	rep, err := repo.LoadRep(ctx, charID)
	if err != nil {
		t.Fatalf("LoadRep: %v", err)
	}
	if len(rep) != 0 {
		t.Fatalf("expected empty map, got %v", rep)
	}

	// Save and reload.
	if err := repo.SaveRep(ctx, charID, "gun", 150); err != nil {
		t.Fatalf("SaveRep: %v", err)
	}
	rep, err = repo.LoadRep(ctx, charID)
	if err != nil {
		t.Fatalf("LoadRep after save: %v", err)
	}
	if rep["gun"] != 150 {
		t.Fatalf("expected rep[gun]=150, got %d", rep["gun"])
	}

	// Update (upsert) existing entry.
	if err := repo.SaveRep(ctx, charID, "gun", 300); err != nil {
		t.Fatalf("SaveRep update: %v", err)
	}
	rep, err = repo.LoadRep(ctx, charID)
	if err != nil {
		t.Fatalf("LoadRep after update: %v", err)
	}
	if rep["gun"] != 300 {
		t.Fatalf("expected rep[gun]=300, got %d", rep["gun"])
	}
}
