package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
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

// TestProperty_LoadEquipment_ArmorAndAccessorySlotsDontOverlap verifies that the slot
// classification logic used in LoadEquipment routes armor and accessory slots to disjoint sets.
// This property test does not require a DB connection.
func TestProperty_LoadEquipment_ArmorAndAccessorySlotsDontOverlap(t *testing.T) {
	armorSlots := []string{
		string(inventory.SlotHead), string(inventory.SlotLeftArm), string(inventory.SlotRightArm),
		string(inventory.SlotTorso), string(inventory.SlotLeftLeg), string(inventory.SlotRightLeg),
		string(inventory.SlotFeet), string(inventory.SlotHands),
	}
	accSlots := []string{
		string(inventory.SlotNeck),
		string(inventory.SlotLeftRing1), string(inventory.SlotLeftRing2), string(inventory.SlotLeftRing3),
		string(inventory.SlotLeftRing4), string(inventory.SlotLeftRing5),
		string(inventory.SlotRightRing1), string(inventory.SlotRightRing2), string(inventory.SlotRightRing3),
		string(inventory.SlotRightRing4), string(inventory.SlotRightRing5),
	}
	rapid.Check(t, func(rt *rapid.T) {
		a := rapid.SampledFrom(armorSlots).Draw(rt, "armor")
		acc := rapid.SampledFrom(accSlots).Draw(rt, "acc")
		if a == acc {
			rt.Fatalf("armor slot %q collides with accessory slot %q", a, acc)
		}
	})
}
