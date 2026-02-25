package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// TestRegistry_RegisterWeapon_Lookup verifies that a registered WeaponDef can
// be retrieved by ID.
func TestRegistry_RegisterWeapon_Lookup(t *testing.T) {
	r := inventory.NewRegistry()
	def := pistolDef()
	if err := r.RegisterWeapon(def); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := r.Weapon(def.ID)
	if got == nil {
		t.Fatal("expected non-nil WeaponDef, got nil")
	}
	if got.ID != def.ID {
		t.Fatalf("expected ID=%q, got %q", def.ID, got.ID)
	}
}

// TestRegistry_RegisterWeapon_CollisionError verifies that registering two
// WeaponDefs with the same ID returns an error on the second registration.
func TestRegistry_RegisterWeapon_CollisionError(t *testing.T) {
	r := inventory.NewRegistry()
	def := pistolDef()
	if err := r.RegisterWeapon(def); err != nil {
		t.Fatalf("unexpected error on first register: %v", err)
	}
	if err := r.RegisterWeapon(def); err == nil {
		t.Fatal("expected collision error on second register, got nil")
	}
}

// TestRegistry_Explosive_NotFound_ReturnsNil verifies that looking up an
// unregistered explosive ID returns nil.
func TestRegistry_Explosive_NotFound_ReturnsNil(t *testing.T) {
	r := inventory.NewRegistry()
	if got := r.Explosive("does-not-exist"); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// TestRegistry_AllWeapons_ReturnsAll verifies that after registering three
// WeaponDefs, AllWeapons returns a slice of length 3.
func TestRegistry_AllWeapons_ReturnsAll(t *testing.T) {
	r := inventory.NewRegistry()
	defs := []*inventory.WeaponDef{
		{ID: "w1", Name: "Weapon 1", DamageDice: "1d4", DamageType: "bludgeoning"},
		{ID: "w2", Name: "Weapon 2", DamageDice: "1d4", DamageType: "bludgeoning"},
		{ID: "w3", Name: "Weapon 3", DamageDice: "1d4", DamageType: "bludgeoning"},
	}
	for _, d := range defs {
		if err := r.RegisterWeapon(d); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	all := r.AllWeapons()
	if len(all) != 3 {
		t.Fatalf("expected 3 weapons, got %d", len(all))
	}
}
