package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// TestHydrateEquipmentNames_SetsNameFromRegistry verifies that hydrateEquipmentNames
// replaces the ItemDefID placeholder with the ArmorDef.Name from the registry.
//
// Precondition: Equipment has a SlottedItem with Name set to ItemDefID; registry has a matching ArmorDef.
// Postcondition: SlottedItem.Name equals the ArmorDef.Name from the registry.
func TestHydrateEquipmentNames_SetsNameFromRegistry(t *testing.T) {
	reg := inventory.NewRegistry()
	err := reg.RegisterArmor(&inventory.ArmorDef{
		ID:   "tactical_boots",
		Name: "Tactical Boots",
		Slot: inventory.SlotFeet,
	})
	if err != nil {
		t.Fatalf("RegisterArmor: %v", err)
	}

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotFeet] = &inventory.SlottedItem{ItemDefID: "tactical_boots", Name: "tactical_boots"}

	hydrateEquipmentNames(eq, reg)

	got := eq.Armor[inventory.SlotFeet].Name
	if got != "Tactical Boots" {
		t.Errorf("expected Name=%q, got %q", "Tactical Boots", got)
	}
}

// TestHydrateEquipmentNames_UnknownIDUnchanged verifies that a SlottedItem with an
// unregistered ItemDefID is left unchanged.
//
// Precondition: Equipment has a SlottedItem whose ItemDefID is not in the registry.
// Postcondition: SlottedItem.Name remains equal to the original placeholder value.
func TestHydrateEquipmentNames_UnknownIDUnchanged(t *testing.T) {
	reg := inventory.NewRegistry()
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotHead] = &inventory.SlottedItem{ItemDefID: "unknown_helmet", Name: "unknown_helmet"}

	hydrateEquipmentNames(eq, reg)

	got := eq.Armor[inventory.SlotHead].Name
	if got != "unknown_helmet" {
		t.Errorf("expected Name unchanged %q, got %q", "unknown_helmet", got)
	}
}

// TestHydrateEquipmentNames_NilSafe verifies that nil equipment or registry does not panic.
//
// Precondition: eq or reg may be nil.
// Postcondition: Function returns without panic.
func TestHydrateEquipmentNames_NilSafe(t *testing.T) {
	reg := inventory.NewRegistry()
	eq := inventory.NewEquipment()

	// Neither panics.
	hydrateEquipmentNames(nil, reg)
	hydrateEquipmentNames(eq, nil)
}

// TestHydrateEquipmentNames_MultipleSlots verifies that all populated armor slots are hydrated.
//
// Precondition: Equipment has items in multiple armor slots; registry contains definitions for all.
// Postcondition: All populated slots have Name updated to their ArmorDef.Name.
func TestHydrateEquipmentNames_MultipleSlots(t *testing.T) {
	reg := inventory.NewRegistry()
	defs := []inventory.ArmorDef{
		{ID: "iron_helm", Name: "Iron Helm", Slot: inventory.SlotHead},
		{ID: "chain_torso", Name: "Chain Hauberk", Slot: inventory.SlotTorso},
	}
	for i := range defs {
		if err := reg.RegisterArmor(&defs[i]); err != nil {
			t.Fatalf("RegisterArmor: %v", err)
		}
	}

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotHead] = &inventory.SlottedItem{ItemDefID: "iron_helm", Name: "iron_helm"}
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "chain_torso", Name: "chain_torso"}

	hydrateEquipmentNames(eq, reg)

	cases := map[inventory.ArmorSlot]string{
		inventory.SlotHead:  "Iron Helm",
		inventory.SlotTorso: "Chain Hauberk",
	}
	for slot, want := range cases {
		if got := eq.Armor[slot].Name; got != want {
			t.Errorf("slot %s: expected Name=%q, got %q", slot, want, got)
		}
	}
}
