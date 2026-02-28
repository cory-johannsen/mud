package inventory_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestEquipment_New_Empty(t *testing.T) {
	e := inventory.NewEquipment()
	if e.Armor == nil {
		t.Fatal("expected non-nil Armor map")
	}
	if e.Accessories == nil {
		t.Fatal("expected non-nil Accessories map")
	}
	if len(e.Armor) != 0 {
		t.Fatalf("expected empty Armor, got %d entries", len(e.Armor))
	}
	if len(e.Accessories) != 0 {
		t.Fatalf("expected empty Accessories, got %d entries", len(e.Accessories))
	}
}

func TestEquipment_ArmorSlotCount(t *testing.T) {
	slots := []inventory.ArmorSlot{
		inventory.SlotHead,
		inventory.SlotLeftArm,
		inventory.SlotRightArm,
		inventory.SlotTorso,
		inventory.SlotLeftLeg,
		inventory.SlotRightLeg,
		inventory.SlotFeet,
	}
	if len(slots) != 7 {
		t.Fatalf("expected 7 armor slots, got %d", len(slots))
	}
}

func TestEquipment_AccessorySlotCount(t *testing.T) {
	slots := []inventory.AccessorySlot{
		inventory.SlotNeck,
		inventory.SlotRing1,
		inventory.SlotRing2,
		inventory.SlotRing3,
		inventory.SlotRing4,
		inventory.SlotRing5,
		inventory.SlotRing6,
		inventory.SlotRing7,
		inventory.SlotRing8,
		inventory.SlotRing9,
		inventory.SlotRing10,
	}
	if len(slots) != 11 {
		t.Fatalf("expected 11 accessory slots, got %d", len(slots))
	}
}

func TestEquipment_ArmorSlotValues(t *testing.T) {
	tests := []struct {
		slot inventory.ArmorSlot
		want string
	}{
		{inventory.SlotHead, "head"},
		{inventory.SlotLeftArm, "left_arm"},
		{inventory.SlotRightArm, "right_arm"},
		{inventory.SlotTorso, "torso"},
		{inventory.SlotLeftLeg, "left_leg"},
		{inventory.SlotRightLeg, "right_leg"},
		{inventory.SlotFeet, "feet"},
	}
	for _, tc := range tests {
		if string(tc.slot) != tc.want {
			t.Errorf("slot %q: got %q, want %q", tc.slot, string(tc.slot), tc.want)
		}
	}
}

func TestEquipment_AccessorySlotValues(t *testing.T) {
	tests := []struct {
		slot inventory.AccessorySlot
		want string
	}{
		{inventory.SlotNeck, "neck"},
		{inventory.SlotRing1, "ring_1"},
		{inventory.SlotRing2, "ring_2"},
		{inventory.SlotRing3, "ring_3"},
		{inventory.SlotRing4, "ring_4"},
		{inventory.SlotRing5, "ring_5"},
		{inventory.SlotRing6, "ring_6"},
		{inventory.SlotRing7, "ring_7"},
		{inventory.SlotRing8, "ring_8"},
		{inventory.SlotRing9, "ring_9"},
		{inventory.SlotRing10, "ring_10"},
	}
	for _, tc := range tests {
		if string(tc.slot) != tc.want {
			t.Errorf("slot %q: got %q, want %q", tc.slot, string(tc.slot), tc.want)
		}
	}
}

func TestProperty_Equipment_ArmorSlotsAreDistinct(t *testing.T) {
	allSlots := []inventory.ArmorSlot{
		inventory.SlotHead, inventory.SlotLeftArm, inventory.SlotRightArm,
		inventory.SlotTorso, inventory.SlotLeftLeg, inventory.SlotRightLeg,
		inventory.SlotFeet,
	}
	rapid.Check(t, func(rt *rapid.T) {
		// Draw two distinct indices and assert the corresponding slots have different string values.
		i := rapid.IntRange(0, len(allSlots)-1).Draw(rt, "i")
		j := rapid.IntRange(0, len(allSlots)-1).Draw(rt, "j")
		if i == j {
			return // same index is trivially the same slot — skip
		}
		if string(allSlots[i]) == string(allSlots[j]) {
			rt.Fatalf("armor slots at index %d (%q) and %d (%q) have the same string value",
				i, allSlots[i], j, allSlots[j])
		}
	})
}

func TestProperty_Equipment_AccessorySlotsAreDistinct(t *testing.T) {
	allSlots := []inventory.AccessorySlot{
		inventory.SlotNeck,
		inventory.SlotRing1, inventory.SlotRing2, inventory.SlotRing3,
		inventory.SlotRing4, inventory.SlotRing5, inventory.SlotRing6,
		inventory.SlotRing7, inventory.SlotRing8, inventory.SlotRing9,
		inventory.SlotRing10,
	}
	rapid.Check(t, func(rt *rapid.T) {
		i := rapid.IntRange(0, len(allSlots)-1).Draw(rt, "i")
		j := rapid.IntRange(0, len(allSlots)-1).Draw(rt, "j")
		if i == j {
			return
		}
		if string(allSlots[i]) == string(allSlots[j]) {
			rt.Fatalf("accessory slots at index %d (%q) and %d (%q) have the same string value",
				i, allSlots[i], j, allSlots[j])
		}
	})
}
