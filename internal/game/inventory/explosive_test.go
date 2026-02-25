package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"

	"pgregory.net/rapid"
)

func TestExplosiveDef_Validate_RejectsEmpty(t *testing.T) {
	e := &inventory.ExplosiveDef{}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty ExplosiveDef, got nil")
	}
}

func TestExplosiveDef_Validate_AcceptsMinimal(t *testing.T) {
	e := &inventory.ExplosiveDef{
		ID:         "test_grenade",
		Name:       "Test Grenade",
		DamageDice: "2d6",
		DamageType: "piercing",
		SaveType:   "reflex",
		SaveDC:     12,
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("expected no error for minimal ExplosiveDef, got: %v", err)
	}
}

func TestLoadExplosives_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	content := `id: frag_grenade
name: Frag Grenade
damage_dice: 4d6
damage_type: piercing
area_type: room
save_type: reflex
save_dc: 15
fuse: immediate
traits: [thrown, limited_use]
`
	if err := os.WriteFile(filepath.Join(dir, "frag_grenade.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}

	explosives, err := inventory.LoadExplosives(dir)
	if err != nil {
		t.Fatalf("LoadExplosives failed: %v", err)
	}
	if len(explosives) != 1 {
		t.Fatalf("expected 1 explosive, got %d", len(explosives))
	}
	e := explosives[0]
	if e.ID != "frag_grenade" {
		t.Errorf("expected ID 'frag_grenade', got %q", e.ID)
	}
	if e.SaveDC != 15 {
		t.Errorf("expected SaveDC 15, got %d", e.SaveDC)
	}
	if e.Fuse != inventory.FuseImmediate {
		t.Errorf("expected Fuse 'immediate', got %q", e.Fuse)
	}
	if len(e.Traits) != 2 {
		t.Errorf("expected 2 traits, got %d", len(e.Traits))
	}
}

func TestProperty_ExplosiveDef_SaveDCPositiveAfterValidation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dc := rapid.IntRange(1, 30).Draw(rt, "save_dc")
		e := &inventory.ExplosiveDef{
			ID:         "e",
			Name:       "E",
			DamageDice: "1d6",
			DamageType: "piercing",
			AreaType:   inventory.AreaTypeRoom,
			SaveType:   "reflex",
			SaveDC:     dc,
			Fuse:       inventory.FuseImmediate,
		}
		if err := e.Validate(); err != nil {
			rt.Fatalf("valid explosive failed validation: %v", err)
		}
		if e.SaveDC <= 0 {
			rt.Fatal("SaveDC must be positive after successful validation")
		}
	})
}

func TestProperty_ExplosiveDef_ZeroSaveDCAlwaysInvalid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		e := &inventory.ExplosiveDef{
			ID:         "e",
			Name:       "E",
			DamageDice: "1d6",
			DamageType: "piercing",
			AreaType:   inventory.AreaTypeRoom,
			SaveType:   "reflex",
			SaveDC:     0,
			Fuse:       inventory.FuseImmediate,
		}
		if err := e.Validate(); err == nil {
			rt.Fatal("zero SaveDC must always fail validation")
		}
	})
}
