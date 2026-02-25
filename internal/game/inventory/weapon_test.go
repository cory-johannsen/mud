package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"pgregory.net/rapid"
)

func TestWeaponDef_Validate_RejectsEmpty(t *testing.T) {
	w := &inventory.WeaponDef{}
	if err := w.Validate(); err == nil {
		t.Fatal("expected error for empty WeaponDef, got nil")
	}
}

func TestWeaponDef_Validate_AcceptsMinimal(t *testing.T) {
	w := &inventory.WeaponDef{
		ID:         "test_sword",
		Name:       "Test Sword",
		DamageDice: "1d6",
		DamageType: "slashing",
	}
	if err := w.Validate(); err != nil {
		t.Fatalf("expected no error for minimal melee WeaponDef, got: %v", err)
	}
}

func TestWeaponDef_Validate_FirearmRequiresMagazine(t *testing.T) {
	w := &inventory.WeaponDef{
		ID:               "test_pistol",
		Name:             "Test Pistol",
		DamageDice:       "1d6",
		DamageType:       "piercing",
		RangeIncrement:   30,
		FiringModes:      []inventory.FiringMode{inventory.FiringModeSingle},
		MagazineCapacity: 0,
	}
	if err := w.Validate(); err == nil {
		t.Fatal("expected error for firearm with MagazineCapacity==0, got nil")
	}
}

func TestLoadWeapons_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	content := `id: test_pistol
name: Test Pistol
damage_dice: 1d8
damage_type: piercing
range_increment: 30
reload_actions: 1
magazine_capacity: 10
firing_modes: [single]
traits: [concealable]
`
	if err := os.WriteFile(filepath.Join(dir, "test_pistol.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}

	weapons, err := inventory.LoadWeapons(dir)
	if err != nil {
		t.Fatalf("LoadWeapons failed: %v", err)
	}
	if len(weapons) != 1 {
		t.Fatalf("expected 1 weapon, got %d", len(weapons))
	}
	w := weapons[0]
	if w.ID != "test_pistol" {
		t.Errorf("expected ID 'test_pistol', got %q", w.ID)
	}
	if w.Name != "Test Pistol" {
		t.Errorf("expected Name 'Test Pistol', got %q", w.Name)
	}
	if w.DamageDice != "1d8" {
		t.Errorf("expected DamageDice '1d8', got %q", w.DamageDice)
	}
	if w.MagazineCapacity != 10 {
		t.Errorf("expected MagazineCapacity 10, got %d", w.MagazineCapacity)
	}
	if len(w.Traits) != 1 || w.Traits[0] != "concealable" {
		t.Errorf("unexpected traits: %v", w.Traits)
	}
}

func TestProperty_WeaponDef_RangeIncrementNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ri := rapid.IntRange(0, 1000).Draw(rt, "range_increment")
		w := &inventory.WeaponDef{
			ID:             "prop_weapon",
			Name:           "Prop Weapon",
			DamageDice:     "1d6",
			DamageType:     "piercing",
			RangeIncrement: ri,
		}
		if w.RangeIncrement < 0 {
			rt.Fatal("RangeIncrement must be non-negative")
		}
		if ri == 0 && !w.IsMelee() {
			rt.Fatal("weapon with RangeIncrement==0 must be melee")
		}
		if ri > 0 && w.IsMelee() {
			rt.Fatal("weapon with RangeIncrement>0 must not be melee")
		}
	})
}
