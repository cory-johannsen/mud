package inventory_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// pistolDef returns a valid firearm WeaponDef for use in tests.
func pistolDef() *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:               "pistol-9mm",
		Name:             "9mm Pistol",
		DamageDice:       "2d6",
		DamageType:       "ballistic",
		RangeIncrement:   30,
		FiringModes:      []inventory.FiringMode{inventory.FiringModeSingle},
		MagazineCapacity: 15,
	}
}

// knifeDef returns a valid melee WeaponDef for use in tests.
func knifeDef() *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:         "knife-combat",
		Name:       "Combat Knife",
		DamageDice: "1d6",
		DamageType: "slashing",
		// RangeIncrement == 0 means melee; no FiringModes for melee.
	}
}

// TestLoadout_EquipPrimary verifies that equipping a pistol in the primary
// slot initialises a full magazine with capacity 15.
func TestLoadout_EquipPrimary(t *testing.T) {
	l := inventory.NewLoadout()
	if err := l.Equip(inventory.SlotPrimary, pistolDef()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	item := l.Primary()
	if item == nil {
		t.Fatal("expected non-nil EquippedItem in primary slot")
	}
	if item.Magazine == nil {
		t.Fatal("expected non-nil Magazine for firearm")
	}
	if item.Magazine.Loaded != 15 {
		t.Fatalf("expected Magazine.Loaded=15, got %d", item.Magazine.Loaded)
	}
}

// TestLoadout_EquipMelee_NoMagazine verifies that equipping a melee weapon
// results in a nil Magazine.
func TestLoadout_EquipMelee_NoMagazine(t *testing.T) {
	l := inventory.NewLoadout()
	if err := l.Equip(inventory.SlotPrimary, knifeDef()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	item := l.Primary()
	if item == nil {
		t.Fatal("expected non-nil EquippedItem")
	}
	if item.Magazine != nil {
		t.Fatalf("expected nil Magazine for melee weapon, got %+v", item.Magazine)
	}
}

// TestLoadout_Unequip_ClearsSlot verifies that Unequip removes the weapon
// from the slot so that Equipped returns nil.
func TestLoadout_Unequip_ClearsSlot(t *testing.T) {
	l := inventory.NewLoadout()
	if err := l.Equip(inventory.SlotPrimary, pistolDef()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Unequip(inventory.SlotPrimary)
	if l.Equipped(inventory.SlotPrimary) != nil {
		t.Fatal("expected nil after unequip")
	}
}

// TestLoadout_Equip_NilDefReturnsError verifies that Equip with a nil def
// returns a non-nil error.
func TestLoadout_Equip_NilDefReturnsError(t *testing.T) {
	l := inventory.NewLoadout()
	if err := l.Equip(inventory.SlotPrimary, nil); err == nil {
		t.Fatal("expected error when equipping nil def, got nil")
	}
}

// TestProperty_Loadout_MultipleEquip_OnlyLastRemains is a property-based test
// that equips the same slot N times and asserts only the last weapon remains.
func TestProperty_Loadout_MultipleEquip_OnlyLastRemains(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		l := inventory.NewLoadout()
		n := rapid.IntRange(1, 10).Draw(rt, "n")

		var lastID string
		for i := 0; i < n; i++ {
			id := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "id")
			def := &inventory.WeaponDef{
				ID:         id,
				Name:       "Test Weapon",
				DamageDice: "1d6",
				DamageType: "slashing",
			}
			if err := l.Equip(inventory.SlotPrimary, def); err != nil {
				continue
			}
			lastID = id
		}

		if lastID == "" {
			return // all iterations produced validation failures; skip
		}

		item := l.Equipped(inventory.SlotPrimary)
		if item == nil {
			rt.Fatalf("expected equipped item, got nil")
		}
		if item.Def.ID != lastID {
			rt.Fatalf("expected last def ID=%q, got %q", lastID, item.Def.ID)
		}
	})
}
