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
		inventory.SlotHands,
		inventory.SlotLeftLeg,
		inventory.SlotRightLeg,
		inventory.SlotFeet,
	}
	if len(slots) != 8 {
		t.Fatalf("expected 8 armor slots, got %d", len(slots))
	}
}

func TestEquipment_AccessorySlotCount(t *testing.T) {
	slots := []inventory.AccessorySlot{
		inventory.SlotNeck,
		inventory.SlotLeftRing1,
		inventory.SlotLeftRing2,
		inventory.SlotLeftRing3,
		inventory.SlotLeftRing4,
		inventory.SlotLeftRing5,
		inventory.SlotRightRing1,
		inventory.SlotRightRing2,
		inventory.SlotRightRing3,
		inventory.SlotRightRing4,
		inventory.SlotRightRing5,
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
		{inventory.SlotHands, "hands"},
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
		{inventory.SlotLeftRing1, "left_ring_1"},
		{inventory.SlotLeftRing2, "left_ring_2"},
		{inventory.SlotLeftRing3, "left_ring_3"},
		{inventory.SlotLeftRing4, "left_ring_4"},
		{inventory.SlotLeftRing5, "left_ring_5"},
		{inventory.SlotRightRing1, "right_ring_1"},
		{inventory.SlotRightRing2, "right_ring_2"},
		{inventory.SlotRightRing3, "right_ring_3"},
		{inventory.SlotRightRing4, "right_ring_4"},
		{inventory.SlotRightRing5, "right_ring_5"},
	}
	for _, tc := range tests {
		if string(tc.slot) != tc.want {
			t.Errorf("slot %q: got %q, want %q", tc.slot, string(tc.slot), tc.want)
		}
	}
}

func TestEquipment_SlotDisplayName_KnownSlots(t *testing.T) {
	tests := []struct {
		slot string
		want string
	}{
		{"head", "Head"},
		{"left_arm", "Left Arm"},
		{"right_arm", "Right Arm"},
		{"torso", "Torso"},
		{"hands", "Hands"},
		{"left_leg", "Left Leg"},
		{"right_leg", "Right Leg"},
		{"feet", "Feet"},
		{"neck", "Neck"},
		{"left_ring_1", "Left Hand Ring 1"},
		{"left_ring_2", "Left Hand Ring 2"},
		{"left_ring_3", "Left Hand Ring 3"},
		{"left_ring_4", "Left Hand Ring 4"},
		{"left_ring_5", "Left Hand Ring 5"},
		{"right_ring_1", "Right Hand Ring 1"},
		{"right_ring_2", "Right Hand Ring 2"},
		{"right_ring_3", "Right Hand Ring 3"},
		{"right_ring_4", "Right Hand Ring 4"},
		{"right_ring_5", "Right Hand Ring 5"},
		{"main", "Main Hand"},
		{"off", "Off Hand"},
	}
	for _, tc := range tests {
		got := inventory.SlotDisplayName(tc.slot)
		if got != tc.want {
			t.Errorf("SlotDisplayName(%q) = %q, want %q", tc.slot, got, tc.want)
		}
	}
}

func TestEquipment_SlotDisplayName_UnknownSlotFallback(t *testing.T) {
	got := inventory.SlotDisplayName("unknown_slot_xyz")
	if got != "unknown_slot_xyz" {
		t.Errorf("expected fallback to raw slot, got %q", got)
	}
}

func TestProperty_Equipment_SlotDisplayName_NeverEmpty(t *testing.T) {
	knownSlots := []string{
		"head", "left_arm", "right_arm", "torso", "hands",
		"left_leg", "right_leg", "feet", "neck",
		"left_ring_1", "left_ring_2", "left_ring_3", "left_ring_4", "left_ring_5",
		"right_ring_1", "right_ring_2", "right_ring_3", "right_ring_4", "right_ring_5",
		"main", "off",
	}
	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, len(knownSlots)-1).Draw(rt, "idx")
		slot := knownSlots[idx]
		name := inventory.SlotDisplayName(slot)
		if name == "" {
			rt.Fatalf("SlotDisplayName(%q) returned empty string", slot)
		}
		if name == slot {
			rt.Fatalf("SlotDisplayName(%q) returned the raw slot key unchanged (expected a human label)", slot)
		}
	})
}

func TestProperty_Equipment_ArmorSlotsAreDistinct(t *testing.T) {
	allSlots := []inventory.ArmorSlot{
		inventory.SlotHead, inventory.SlotLeftArm, inventory.SlotRightArm,
		inventory.SlotTorso, inventory.SlotHands,
		inventory.SlotLeftLeg, inventory.SlotRightLeg, inventory.SlotFeet,
	}
	rapid.Check(t, func(rt *rapid.T) {
		i := rapid.IntRange(0, len(allSlots)-1).Draw(rt, "i")
		j := rapid.IntRange(0, len(allSlots)-1).Draw(rt, "j")
		if i == j {
			return
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
		inventory.SlotLeftRing1, inventory.SlotLeftRing2, inventory.SlotLeftRing3,
		inventory.SlotLeftRing4, inventory.SlotLeftRing5,
		inventory.SlotRightRing1, inventory.SlotRightRing2, inventory.SlotRightRing3,
		inventory.SlotRightRing4, inventory.SlotRightRing5,
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
