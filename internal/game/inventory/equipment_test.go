package inventory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestComputedDefenses_NoArmor(t *testing.T) {
	reg := inventory.NewRegistry()
	eq := inventory.NewEquipment()
	stats := eq.ComputedDefenses(reg, 3)
	assert.Equal(t, 0, stats.ACBonus)
	assert.Equal(t, 3, stats.EffectiveDex) // no cap = full dex
	assert.Equal(t, 0, stats.CheckPenalty)
	assert.Equal(t, 0, stats.SpeedPenalty)
	assert.Equal(t, 0, stats.StrengthReq)
}

func TestComputedDefenses_SingleSlot(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID: "vest", Name: "Vest", Slot: inventory.SlotTorso, Group: "composite",
		ACBonus: 3, DexCap: 2, CheckPenalty: -1, SpeedPenalty: 0, StrengthReq: 14,
	}
	require.NoError(t, reg.RegisterArmor(def))
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "vest", Name: "Vest"}

	stats := eq.ComputedDefenses(reg, 4) // dex 4, cap is 2
	assert.Equal(t, 3, stats.ACBonus)
	assert.Equal(t, 2, stats.EffectiveDex) // capped at 2
	assert.Equal(t, -1, stats.CheckPenalty)
	assert.Equal(t, 0, stats.SpeedPenalty)
	assert.Equal(t, 14, stats.StrengthReq)
}

func TestComputedDefenses_MultiSlot_SumsPenalties(t *testing.T) {
	reg := inventory.NewRegistry()
	head := &inventory.ArmorDef{ID: "helm", Name: "Helm", Slot: inventory.SlotHead, Group: "composite", ACBonus: 1, DexCap: 3, CheckPenalty: -1}
	torso := &inventory.ArmorDef{ID: "plate", Name: "Plate", Slot: inventory.SlotTorso, Group: "plate", ACBonus: 4, DexCap: 1, CheckPenalty: -2}
	require.NoError(t, reg.RegisterArmor(head))
	require.NoError(t, reg.RegisterArmor(torso))
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotHead] = &inventory.SlottedItem{ItemDefID: "helm", Name: "Helm"}
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "plate", Name: "Plate"}

	stats := eq.ComputedDefenses(reg, 5)
	assert.Equal(t, 5, stats.ACBonus)       // 1+4
	assert.Equal(t, 1, stats.EffectiveDex)  // min(5, 3, 1) = 1
	assert.Equal(t, -3, stats.CheckPenalty) // -1 + -2
}

func TestComputedDefenses_DexModBelowCap_UsesActualDex(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{ID: "light", Name: "Light", Slot: inventory.SlotTorso, Group: "leather", ACBonus: 1, DexCap: 5}
	require.NoError(t, reg.RegisterArmor(def))
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "light", Name: "Light"}

	stats := eq.ComputedDefenses(reg, 2) // dex 2, cap 5 — dex wins
	assert.Equal(t, 2, stats.EffectiveDex)
}

func TestComputedDefenses_UnknownArmorDef_Skipped(t *testing.T) {
	reg := inventory.NewRegistry()
	eq := inventory.NewEquipment()
	// Slot has an item but def is not in registry — should be skipped silently
	eq.Armor[inventory.SlotHead] = &inventory.SlottedItem{ItemDefID: "unknown_def", Name: "Unknown"}
	stats := eq.ComputedDefenses(reg, 3)
	assert.Equal(t, 0, stats.ACBonus)
	assert.Equal(t, 3, stats.EffectiveDex)
}

func TestProperty_ComputedDefenses_ACBonusEqualsSum(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		slots := []inventory.ArmorSlot{
			inventory.SlotHead, inventory.SlotTorso, inventory.SlotHands, inventory.SlotFeet,
		}
		reg := inventory.NewRegistry()
		eq := inventory.NewEquipment()
		totalAC := 0
		for _, slot := range slots {
			ac := rapid.IntRange(0, 6).Draw(rt, "ac")
			totalAC += ac
			id := string(slot) + "_test"
			def := &inventory.ArmorDef{
				ID: id, Name: id, Slot: slot,
				ACBonus: ac, DexCap: 10, Group: "leather",
			}
			require.NoError(rt, reg.RegisterArmor(def))
			eq.Armor[slot] = &inventory.SlottedItem{ItemDefID: id, Name: id}
		}
		stats := eq.ComputedDefenses(reg, 5)
		assert.Equal(rt, totalAC, stats.ACBonus)
	})
}
