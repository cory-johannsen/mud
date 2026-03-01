package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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

// TestRegistry_RegisterExplosive_Lookup verifies that a registered
// ExplosiveDef can be retrieved by ID.
func TestRegistry_RegisterExplosive_Lookup(t *testing.T) {
	r := inventory.NewRegistry()
	e := &inventory.ExplosiveDef{
		ID:         "frag-grenade",
		Name:       "Frag Grenade",
		DamageDice: "3d6",
		DamageType: "piercing",
		AreaType:   inventory.AreaTypeBurst,
		Fuse:       inventory.FuseDelayed,
	}
	if err := r.RegisterExplosive(e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := r.Explosive(e.ID)
	if got == nil {
		t.Fatal("expected non-nil ExplosiveDef, got nil")
	}
	if got.ID != e.ID {
		t.Fatalf("expected ID=%q, got %q", e.ID, got.ID)
	}
}

// TestRegistry_RegisterExplosive_CollisionError verifies that registering two
// ExplosiveDefs with the same ID returns an error on the second registration.
func TestRegistry_RegisterExplosive_CollisionError(t *testing.T) {
	r := inventory.NewRegistry()
	e := &inventory.ExplosiveDef{
		ID:         "frag-grenade",
		Name:       "Frag Grenade",
		DamageDice: "3d6",
		DamageType: "piercing",
		AreaType:   inventory.AreaTypeBurst,
		Fuse:       inventory.FuseDelayed,
	}
	if err := r.RegisterExplosive(e); err != nil {
		t.Fatalf("unexpected error on first register: %v", err)
	}
	if err := r.RegisterExplosive(e); err == nil {
		t.Fatal("expected collision error on second register, got nil")
	}
}

func TestRegistry_RegisterArmor_And_Lookup(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID: "test_helm", Name: "Test Helm", Slot: inventory.SlotHead, Group: "composite",
	}
	require.NoError(t, reg.RegisterArmor(def))
	got, ok := reg.Armor("test_helm")
	require.True(t, ok)
	assert.Equal(t, def, got)
}

func TestRegistry_Armor_Unknown_ReturnsFalse(t *testing.T) {
	reg := inventory.NewRegistry()
	_, ok := reg.Armor("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_RegisterArmor_DuplicateReturnsError(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{ID: "dup", Name: "Dup", Slot: inventory.SlotHead, Group: "leather"}
	require.NoError(t, reg.RegisterArmor(def))
	assert.Error(t, reg.RegisterArmor(def))
}

func TestRegistry_AllArmors_ReturnsAll(t *testing.T) {
	reg := inventory.NewRegistry()
	slots := map[string]inventory.ArmorSlot{
		"helm":  inventory.SlotHead,
		"vest":  inventory.SlotTorso,
		"boots": inventory.SlotFeet,
	}
	for id, slot := range slots {
		require.NoError(t, reg.RegisterArmor(&inventory.ArmorDef{
			ID: id, Name: id, Slot: slot, Group: "leather",
		}))
	}
	all := reg.AllArmors()
	assert.Len(t, all, 3)
}

func TestProperty_Registry_ArmorRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		id := rapid.StringMatching(`[a-z][a-z0-9_]{1,15}`).Draw(rt, "id")
		def := &inventory.ArmorDef{
			ID: id, Name: "Test", Slot: inventory.SlotTorso, Group: "leather",
		}
		require.NoError(t, reg.RegisterArmor(def))
		got, ok := reg.Armor(id)
		assert.True(t, ok)
		assert.Equal(t, def, got)
	})
}
