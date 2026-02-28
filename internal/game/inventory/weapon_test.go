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
kind: one_handed
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
	if w.Kind != inventory.WeaponKindOneHanded {
		t.Errorf("expected Kind %q, got %q", inventory.WeaponKindOneHanded, w.Kind)
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

// TestWeaponDef_SupportsBurst_TrueWhenBurstPresent verifies SupportsBurst
// returns true when FiringModeBurst is in FiringModes.
func TestWeaponDef_SupportsBurst_TrueWhenBurstPresent(t *testing.T) {
	w := &inventory.WeaponDef{
		ID:               "smg",
		Name:             "SMG",
		DamageDice:       "1d6",
		DamageType:       "piercing",
		RangeIncrement:   20,
		ReloadActions:    1,
		MagazineCapacity: 30,
		FiringModes:      []inventory.FiringMode{inventory.FiringModeSingle, inventory.FiringModeBurst},
	}
	if !w.SupportsBurst() {
		t.Fatal("expected SupportsBurst=true, got false")
	}
}

// TestWeaponDef_SupportsBurst_FalseWhenAbsent verifies SupportsBurst returns
// false when FiringModeBurst is not in FiringModes.
func TestWeaponDef_SupportsBurst_FalseWhenAbsent(t *testing.T) {
	w := pistolDef()
	if w.SupportsBurst() {
		t.Fatal("expected SupportsBurst=false for single-mode pistol, got true")
	}
}

// TestWeaponDef_SupportsAutomatic_TrueWhenAutoPresent verifies
// SupportsAutomatic returns true when FiringModeAutomatic is in FiringModes.
func TestWeaponDef_SupportsAutomatic_TrueWhenAutoPresent(t *testing.T) {
	w := &inventory.WeaponDef{
		ID:               "lmg",
		Name:             "LMG",
		DamageDice:       "1d10",
		DamageType:       "piercing",
		RangeIncrement:   50,
		ReloadActions:    2,
		MagazineCapacity: 100,
		FiringModes:      []inventory.FiringMode{inventory.FiringModeSingle, inventory.FiringModeAutomatic},
	}
	if !w.SupportsAutomatic() {
		t.Fatal("expected SupportsAutomatic=true, got false")
	}
}

// TestWeaponDef_SupportsAutomatic_FalseWhenAbsent verifies SupportsAutomatic
// returns false when FiringModeAutomatic is not in FiringModes.
func TestWeaponDef_SupportsAutomatic_FalseWhenAbsent(t *testing.T) {
	w := pistolDef()
	if w.SupportsAutomatic() {
		t.Fatal("expected SupportsAutomatic=false for single-mode pistol, got true")
	}
}

func TestWeaponDef_Kind_DefaultEmpty(t *testing.T) {
	w := &inventory.WeaponDef{
		ID: "knife", Name: "Knife", DamageDice: "1d6", DamageType: "slashing",
	}
	if w.Kind != "" {
		t.Fatalf("expected empty Kind, got %q", w.Kind)
	}
}

func TestWeaponDef_Validate_KindNotRequired(t *testing.T) {
	w := &inventory.WeaponDef{
		ID: "knife", Name: "Knife", DamageDice: "1d6", DamageType: "slashing",
	}
	if err := w.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWeaponDef_IsOneHanded(t *testing.T) {
	w := &inventory.WeaponDef{
		ID: "pistol", Name: "Pistol", DamageDice: "2d6", DamageType: "ballistic",
		Kind: inventory.WeaponKindOneHanded,
	}
	if !w.IsOneHanded() {
		t.Fatal("expected IsOneHanded true")
	}
	if w.IsTwoHanded() {
		t.Fatal("expected IsTwoHanded false")
	}
	if w.IsShield() {
		t.Fatal("expected IsShield false")
	}
}

func TestWeaponDef_IsTwoHanded(t *testing.T) {
	w := &inventory.WeaponDef{
		ID:               "rifle",
		Name:             "Rifle",
		DamageDice:       "3d6",
		DamageType:       "ballistic",
		Kind:             inventory.WeaponKindTwoHanded,
		RangeIncrement:   100,
		FiringModes:      []inventory.FiringMode{inventory.FiringModeSingle},
		MagazineCapacity: 10,
	}
	if !w.IsTwoHanded() {
		t.Fatal("expected IsTwoHanded true")
	}
}

func TestWeaponDef_IsShield(t *testing.T) {
	w := &inventory.WeaponDef{
		ID: "buckler", Name: "Buckler", DamageDice: "1d4", DamageType: "bludgeoning",
		Kind: inventory.WeaponKindShield,
	}
	if !w.IsShield() {
		t.Fatal("expected IsShield true")
	}
}

func TestProperty_WeaponDef_WeaponKind_MutuallyExclusive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.SampledFrom([]inventory.WeaponKind{
			inventory.WeaponKindOneHanded,
			inventory.WeaponKindTwoHanded,
			inventory.WeaponKindShield,
			"", // zero value
		}).Draw(rt, "kind")
		w := &inventory.WeaponDef{
			ID: "test", Name: "Test", DamageDice: "1d6", DamageType: "slashing",
			Kind: kind,
		}
		oneHanded := w.IsOneHanded()
		twoHanded := w.IsTwoHanded()
		shield := w.IsShield()
		// At most one can be true
		count := 0
		if oneHanded {
			count++
		}
		if twoHanded {
			count++
		}
		if shield {
			count++
		}
		if count > 1 {
			rt.Fatalf("multiple kind predicates true for kind=%q", kind)
		}
		// Expected truth matches kind
		switch kind {
		case inventory.WeaponKindOneHanded:
			if !oneHanded {
				rt.Fatalf("IsOneHanded should be true for %q", kind)
			}
		case inventory.WeaponKindTwoHanded:
			if !twoHanded {
				rt.Fatalf("IsTwoHanded should be true for %q", kind)
			}
		case inventory.WeaponKindShield:
			if !shield {
				rt.Fatalf("IsShield should be true for %q", kind)
			}
		case "":
			if oneHanded || twoHanded || shield {
				rt.Fatalf("all predicates should be false for empty kind")
			}
		}
	})
}
