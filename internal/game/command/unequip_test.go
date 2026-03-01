package command_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// TestHandleUnequip_MainHand_Success verifies that unequipping from main hand
// returns the weapon name in the result and clears the slot.
func TestHandleUnequip_MainHand_Success(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.ActivePreset().EquipMainHand(pistolWeaponDef())

	result := command.HandleUnequip(sess, "main")

	if !strings.Contains(result, "Unequipped") {
		t.Errorf("expected 'Unequipped' in result, got: %q", result)
	}
	if !strings.Contains(result, "9mm Pistol") {
		t.Errorf("expected weapon name in result, got: %q", result)
	}
	if !strings.Contains(strings.ToLower(result), "main") {
		t.Errorf("expected 'main' in result, got: %q", result)
	}
	if sess.LoadoutSet.ActivePreset().MainHand != nil {
		t.Error("expected MainHand to be nil after unequip")
	}
}

// TestHandleUnequip_OffHand_Success verifies that unequipping from the off hand
// clears the slot and names the weapon in the response.
func TestHandleUnequip_OffHand_Success(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.ActivePreset().EquipOffHand(pistolWeaponDef())

	result := command.HandleUnequip(sess, "off")

	if !strings.Contains(result, "Unequipped") {
		t.Errorf("expected 'Unequipped' in result, got: %q", result)
	}
	if !strings.Contains(result, "9mm Pistol") {
		t.Errorf("expected weapon name in result, got: %q", result)
	}
	if !strings.Contains(strings.ToLower(result), "off") {
		t.Errorf("expected 'off' in result, got: %q", result)
	}
	if sess.LoadoutSet.ActivePreset().OffHand != nil {
		t.Error("expected OffHand to be nil after unequip")
	}
}

// TestHandleUnequip_MainHand_Empty verifies that unequipping an empty main hand
// slot returns an informative message.
func TestHandleUnequip_MainHand_Empty(t *testing.T) {
	sess := newTestSessionWithBackpack()
	// MainHand is already empty.

	result := command.HandleUnequip(sess, "main")

	if !strings.Contains(strings.ToLower(result), "nothing equipped") {
		t.Errorf("expected 'nothing equipped' for empty main hand, got: %q", result)
	}
}

// TestHandleUnequip_OffHand_Empty verifies that unequipping an empty off hand
// slot returns an informative message.
func TestHandleUnequip_OffHand_Empty(t *testing.T) {
	sess := newTestSessionWithBackpack()

	result := command.HandleUnequip(sess, "off")

	if !strings.Contains(strings.ToLower(result), "nothing equipped") {
		t.Errorf("expected 'nothing equipped' for empty off hand, got: %q", result)
	}
}

// TestHandleUnequip_ArmorSlot_AlwaysEmpty verifies that armor slots always report
// nothing equipped (armor equip is not implemented yet).
func TestHandleUnequip_ArmorSlot_AlwaysEmpty(t *testing.T) {
	armorSlots := []string{"head", "torso", "left_arm", "right_arm", "left_leg", "right_leg", "feet"}
	for _, slot := range armorSlots {
		t.Run(slot, func(t *testing.T) {
			sess := newTestSessionWithBackpack()
			result := command.HandleUnequip(sess, slot)
			if !strings.Contains(strings.ToLower(result), "nothing equipped") {
				t.Errorf("slot %q: expected 'nothing equipped', got: %q", slot, result)
			}
			if !strings.Contains(result, slot) {
				t.Errorf("slot %q: expected slot name in result, got: %q", slot, result)
			}
		})
	}
}

// TestHandleUnequip_AccessorySlot_AlwaysEmpty verifies that accessory slots always
// report nothing equipped.
func TestHandleUnequip_AccessorySlot_AlwaysEmpty(t *testing.T) {
	accessorySlots := []string{
		"neck",
		"left_ring_1", "left_ring_2", "left_ring_3", "left_ring_4", "left_ring_5",
		"right_ring_1", "right_ring_2", "right_ring_3", "right_ring_4", "right_ring_5",
	}
	for _, slot := range accessorySlots {
		t.Run(slot, func(t *testing.T) {
			sess := newTestSessionWithBackpack()
			result := command.HandleUnequip(sess, slot)
			if !strings.Contains(strings.ToLower(result), "nothing equipped") {
				t.Errorf("slot %q: expected 'nothing equipped', got: %q", slot, result)
			}
			if !strings.Contains(result, slot) {
				t.Errorf("slot %q: expected slot name in result, got: %q", slot, result)
			}
		})
	}
}

// TestHandleUnequip_UnknownSlot_ReturnsError verifies that an unknown slot name
// returns a helpful error listing valid slots.
func TestHandleUnequip_UnknownSlot_ReturnsError(t *testing.T) {
	sess := newTestSessionWithBackpack()

	result := command.HandleUnequip(sess, "inventory")

	if !strings.Contains(strings.ToLower(result), "unknown slot") {
		t.Errorf("expected 'unknown slot' error, got: %q", result)
	}
	if !strings.Contains(strings.ToLower(result), "valid slots") {
		t.Errorf("expected 'valid slots' listing in error, got: %q", result)
	}
}

// TestHandleUnequip_EmptySlotArg_ReturnsError verifies that an empty slot argument
// returns an unknown slot error.
func TestHandleUnequip_EmptySlotArg_ReturnsError(t *testing.T) {
	sess := newTestSessionWithBackpack()

	result := command.HandleUnequip(sess, "")

	if !strings.Contains(strings.ToLower(result), "unknown slot") {
		t.Errorf("expected 'unknown slot' for empty arg, got: %q", result)
	}
}

// TestProperty_HandleUnequip_ValidSlotsNeverReturnUnknown is a property-based test
// verifying that every valid slot string never produces an "unknown slot" response.
func TestProperty_HandleUnequip_ValidSlotsNeverReturnUnknown(t *testing.T) {
	validSlots := []string{
		"main", "off",
		"head", "torso", "left_arm", "right_arm", "hands", "left_leg", "right_leg", "feet",
		"neck",
		"left_ring_1", "left_ring_2", "left_ring_3", "left_ring_4", "left_ring_5",
		"right_ring_1", "right_ring_2", "right_ring_3", "right_ring_4", "right_ring_5",
	}

	rapid.Check(t, func(rt *rapid.T) {
		slot := rapid.SampledFrom(validSlots).Draw(rt, "slot")
		sess := newTestSessionWithBackpack()

		result := command.HandleUnequip(sess, slot)

		if strings.Contains(strings.ToLower(result), "unknown slot") {
			rt.Fatalf("valid slot %q returned 'unknown slot': %q", slot, result)
		}
	})
}

// TestHandleUnequip_CaseSensitive verifies that slot names are matched case-sensitively
// so that "MAIN" is rejected as unknown.
func TestHandleUnequip_CaseSensitive(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.ActivePreset().EquipMainHand(pistolWeaponDef())

	result := command.HandleUnequip(sess, "MAIN")

	if !strings.Contains(strings.ToLower(result), "unknown slot") {
		t.Errorf("expected 'unknown slot' for 'MAIN' (case-sensitive), got: %q", result)
	}
}

// TestHandleUnequip_WhitespaceTrimmed verifies that leading/trailing whitespace in
// the slot argument is trimmed before matching.
func TestHandleUnequip_WhitespaceTrimmed(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.ActivePreset().EquipMainHand(pistolWeaponDef())

	result := command.HandleUnequip(sess, "  main  ")

	if !strings.Contains(result, "Unequipped") {
		t.Errorf("expected success after trimming whitespace, got: %q", result)
	}
}

// TestHandleUnequip_ArmorSlotMessage verifies the exact message format for armor slots.
func TestHandleUnequip_ArmorSlotMessage(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleUnequip(sess, "head")

	expected := "Nothing equipped in slot head."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// newTestSessionEquipped returns a session with a pistol equipped in the main hand,
// for use in tests requiring a pre-equipped state.
func newTestSessionEquipped() *inventory.WeaponPreset {
	preset := inventory.NewWeaponPreset()
	_ = preset.EquipMainHand(pistolWeaponDef())
	return preset
}

func TestHandleUnequip_HandsSlotAccepted(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleUnequip(sess, "hands")
	if strings.Contains(result, "Unknown slot") {
		t.Errorf("expected hands slot to be valid, got: %s", result)
	}
}

func TestHandleUnequip_LeftRightRingSlotsAccepted(t *testing.T) {
	sess := newTestSessionWithBackpack()
	newSlots := []string{
		"left_ring_1", "left_ring_2", "left_ring_3", "left_ring_4", "left_ring_5",
		"right_ring_1", "right_ring_2", "right_ring_3", "right_ring_4", "right_ring_5",
	}
	for _, slot := range newSlots {
		result := command.HandleUnequip(sess, slot)
		if strings.Contains(result, "Unknown slot") {
			t.Errorf("expected slot %q to be valid, got: %s", slot, result)
		}
	}
}

func TestHandleUnequip_OldRingSlotsRejected(t *testing.T) {
	sess := newTestSessionWithBackpack()
	oldSlots := []string{
		"ring_1", "ring_2", "ring_3", "ring_4", "ring_5",
		"ring_6", "ring_7", "ring_8", "ring_9", "ring_10",
	}
	for _, slot := range oldSlots {
		result := command.HandleUnequip(sess, slot)
		if !strings.Contains(result, "Unknown slot") {
			t.Errorf("expected old slot %q to be rejected, got: %s", slot, result)
		}
	}
}
