package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"pgregory.net/rapid"
)

func TestItemDef_Validate_RejectsEmptyID(t *testing.T) {
	d := &inventory.ItemDef{
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestItemDef_Validate_RejectsEmptyKind(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty Kind, got nil")
	}
}

func TestItemDef_Validate_RejectsInvalidKind(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     "invalid",
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for invalid Kind, got nil")
	}
}

func TestItemDef_Validate_RejectsZeroMaxStack(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 0,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for MaxStack==0, got nil")
	}
}

func TestItemDef_Validate_RejectsNegativeWeight(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
		Weight:   -1.0,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for negative Weight, got nil")
	}
}

func TestItemDef_Validate_AcceptsMinimalJunk(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected no error for minimal junk, got: %v", err)
	}
}

func TestItemDef_Validate_AcceptsWeaponRef(t *testing.T) {
	d := &inventory.ItemDef{
		ID:        "sword_item",
		Name:      "Sword",
		Kind:      inventory.KindWeapon,
		MaxStack:  1,
		WeaponRef: "sword_def",
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected no error for weapon with ref, got: %v", err)
	}
}

func TestItemDef_Validate_RejectsWeaponWithoutRef(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "sword_item",
		Name:     "Sword",
		Kind:     inventory.KindWeapon,
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for weapon without WeaponRef, got nil")
	}
}

func TestItemDef_Validate_RejectsExplosiveWithoutRef(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "grenade_item",
		Name:     "Grenade",
		Kind:     inventory.KindExplosive,
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for explosive without ExplosiveRef, got nil")
	}
}

func TestItemDef_Validate_AcceptsStackable(t *testing.T) {
	d := &inventory.ItemDef{
		ID:        "ammo",
		Name:      "Ammo Box",
		Kind:      inventory.KindConsumable,
		MaxStack:  20,
		Stackable: true,
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected no error for stackable item, got: %v", err)
	}
}

func TestLoadItems_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	content := `id: scrap_metal
name: Scrap Metal
description: A piece of scrap metal.
kind: junk
weight: 0.5
stackable: true
max_stack: 10
value: 5
`
	if err := os.WriteFile(filepath.Join(dir, "scrap.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}
	// Also write a .yml file to verify both extensions are loaded.
	content2 := `id: bandage
name: Bandage
description: A simple bandage.
kind: consumable
weight: 0.1
stackable: true
max_stack: 5
value: 10
`
	if err := os.WriteFile(filepath.Join(dir, "bandage.yml"), []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}

	items, err := inventory.LoadItems(dir)
	if err != nil {
		t.Fatalf("LoadItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	byID := make(map[string]*inventory.ItemDef)
	for _, item := range items {
		byID[item.ID] = item
	}

	scrap, ok := byID["scrap_metal"]
	if !ok {
		t.Fatal("expected scrap_metal item")
	}
	if scrap.Name != "Scrap Metal" {
		t.Errorf("expected Name 'Scrap Metal', got %q", scrap.Name)
	}
	if scrap.Weight != 0.5 {
		t.Errorf("expected Weight 0.5, got %f", scrap.Weight)
	}
	if scrap.MaxStack != 10 {
		t.Errorf("expected MaxStack 10, got %d", scrap.MaxStack)
	}

	bandage, ok := byID["bandage"]
	if !ok {
		t.Fatal("expected bandage item")
	}
	if bandage.Kind != inventory.KindConsumable {
		t.Errorf("expected Kind 'consumable', got %q", bandage.Kind)
	}
}

func TestRegistry_Item_Lookup(t *testing.T) {
	r := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := r.RegisterItem(def); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := r.Item(def.ID)
	if !ok {
		t.Fatal("expected item to be found")
	}
	if got.ID != def.ID {
		t.Fatalf("expected ID=%q, got %q", def.ID, got.ID)
	}
}

func TestRegistry_RegisterItem_RejectsDuplicate(t *testing.T) {
	r := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := r.RegisterItem(def); err != nil {
		t.Fatalf("unexpected error on first register: %v", err)
	}
	if err := r.RegisterItem(def); err == nil {
		t.Fatal("expected collision error on second register, got nil")
	}
}

func TestRegistry_Item_NotFound(t *testing.T) {
	r := inventory.NewRegistry()
	_, ok := r.Item("does-not-exist")
	if ok {
		t.Fatal("expected ok==false for missing item")
	}
}

func TestProperty_ItemDef_ValidKind_AcceptsAll(t *testing.T) {
	kinds := []string{
		inventory.KindWeapon,
		inventory.KindExplosive,
		inventory.KindConsumable,
		inventory.KindJunk,
	}
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.SampledFrom(kinds).Draw(rt, "kind")
		d := &inventory.ItemDef{
			ID:       rapid.StringMatching(`[a-z][a-z0-9_]{2,19}`).Draw(rt, "id"),
			Name:     rapid.StringMatching(`[A-Z][a-zA-Z ]{2,29}`).Draw(rt, "name"),
			Kind:     kind,
			MaxStack: rapid.IntRange(1, 100).Draw(rt, "max_stack"),
			Weight:   rapid.Float64Range(0, 100).Draw(rt, "weight"),
		}
		if kind == inventory.KindWeapon {
			d.WeaponRef = "weapon_" + d.ID
		}
		if kind == inventory.KindExplosive {
			d.ExplosiveRef = "explosive_" + d.ID
		}
		if err := d.Validate(); err != nil {
			rt.Fatalf("expected valid ItemDef to pass validation, got: %v", err)
		}
	})
}
