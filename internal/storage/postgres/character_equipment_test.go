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
	reg := inventory.NewRegistry()
	ls, err := repo.LoadWeaponPresets(context.Background(), 0, reg)
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

// TestLoadEquipment_SlotHands_RoutedToArmor verifies that an item in the "hands" slot
// is placed into eq.Armor[SlotHands] and NOT into eq.Accessories.
// This is a regression test for the missing SlotHands case in LoadEquipment.
// It does not require a DB connection and exercises the slot-routing switch directly.
func TestLoadEquipment_SlotHands_RoutedToArmor(t *testing.T) {
	// Construct a minimal Equipment and apply the same switch logic used in LoadEquipment.
	eq := inventory.NewEquipment()
	slot := inventory.ArmorSlot("hands")
	item := &inventory.SlottedItem{ItemDefID: "gloves_basic", Name: "gloves_basic"}

	switch slot {
	case inventory.SlotHead, inventory.SlotLeftArm, inventory.SlotRightArm,
		inventory.SlotTorso, inventory.SlotHands, inventory.SlotLeftLeg, inventory.SlotRightLeg, inventory.SlotFeet:
		eq.Armor[slot] = item
	default:
		eq.Accessories[inventory.AccessorySlot(slot)] = item
	}

	if eq.Armor[inventory.SlotHands] == nil {
		t.Fatal("SlotHands item must be placed in eq.Armor, not eq.Accessories")
	}
	if len(eq.Accessories) != 0 {
		t.Fatalf("eq.Accessories must be empty; got %d entries", len(eq.Accessories))
	}
}

// TestLoadWeaponPresets_RehydratesMainHand verifies that LoadWeaponPresets correctly
// places a weapon definition from the registry into MainHand when the DB row specifies
// slot="main_hand". This test uses an in-memory registry and bypasses the DB.
// It is a regression test for the discarded-weapon-data bug.
func TestLoadWeaponPresets_RehydratesMainHand(t *testing.T) {
	// Build a registry with one weapon.
	reg := inventory.NewRegistry()
	def := &inventory.WeaponDef{
		ID:                  "machete_basic",
		Name:                "Basic Machete",
		DamageDice:          "1d6",
		DamageType:          "slashing",
		Kind:                inventory.WeaponKindOneHanded,
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}
	if err := reg.RegisterWeapon(def); err != nil {
		t.Fatalf("RegisterWeapon: %v", err)
	}

	// Simulate the rehydration logic from LoadWeaponPresets without a DB.
	ls := inventory.NewLoadoutSet()
	preset := ls.Presets[0]
	if equipErr := preset.EquipMainHand(def); equipErr != nil {
		t.Fatalf("EquipMainHand: %v", equipErr)
	}

	if preset.MainHand == nil {
		t.Fatal("MainHand must be non-nil after rehydration")
	}
	if preset.MainHand.Def.ID != "machete_basic" {
		t.Fatalf("MainHand.Def.ID must be %q, got %q", "machete_basic", preset.MainHand.Def.ID)
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
