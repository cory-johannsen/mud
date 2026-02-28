package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func testPoolEquipment(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to test DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestCharacterRepository_LoadWeaponPresets_EmptyByDefault(t *testing.T) {
	pool := testPoolEquipment(t)
	repo := pgstore.NewCharacterRepository(pool)
	ls, err := repo.LoadWeaponPresets(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ls.Presets) != 2 {
		t.Fatalf("expected 2 default presets, got %d", len(ls.Presets))
	}
	if ls.Active != 0 {
		t.Fatalf("expected Active=0, got %d", ls.Active)
	}
}

func TestCharacterRepository_LoadEquipment_EmptyByDefault(t *testing.T) {
	pool := testPoolEquipment(t)
	repo := pgstore.NewCharacterRepository(pool)
	eq, err := repo.LoadEquipment(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eq.Armor) != 0 || len(eq.Accessories) != 0 {
		t.Fatal("expected empty equipment for character ID 0")
	}
}
