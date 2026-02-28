package command_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// TestHandleEquipment_EmptyState verifies that an empty equipment state displays
// all section headers and "empty" for every slot.
func TestHandleEquipment_EmptyState(t *testing.T) {
	sess := newTestSessionWithBackpack()

	result := command.HandleEquipment(sess)

	if !strings.Contains(result, "=== Weapons ===") {
		t.Errorf("expected '=== Weapons ===' header, got:\n%s", result)
	}
	if !strings.Contains(result, "=== Armor ===") {
		t.Errorf("expected '=== Armor ===' header, got:\n%s", result)
	}
	if !strings.Contains(result, "=== Accessories ===") {
		t.Errorf("expected '=== Accessories ===' header, got:\n%s", result)
	}
}

// TestHandleEquipment_ShowsBothPresets verifies that both presets are displayed with
// their preset numbers.
func TestHandleEquipment_ShowsBothPresets(t *testing.T) {
	sess := newTestSessionWithBackpack()

	result := command.HandleEquipment(sess)

	if !strings.Contains(result, "Preset 1") {
		t.Errorf("expected 'Preset 1', got:\n%s", result)
	}
	if !strings.Contains(result, "Preset 2") {
		t.Errorf("expected 'Preset 2', got:\n%s", result)
	}
}

// TestHandleEquipment_ActivePresetMarked verifies that the active preset is labelled [active].
func TestHandleEquipment_ActivePresetMarked(t *testing.T) {
	sess := newTestSessionWithBackpack()

	result := command.HandleEquipment(sess)

	count := strings.Count(result, "[active]")
	if count != 1 {
		t.Errorf("expected exactly 1 [active] marker, got %d in:\n%s", count, result)
	}
}

// TestHandleEquipment_WeaponDisplayedWithAmmo verifies that a firearm in the main hand
// is displayed with its [loaded/capacity] ammo count.
func TestHandleEquipment_WeaponDisplayedWithAmmo(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.Presets[0].EquipMainHand(pistolWeaponDef())
	sess.LoadoutSet.Presets[0].MainHand.Magazine.Loaded = 12

	result := command.HandleEquipment(sess)

	if !strings.Contains(result, "9mm Pistol") {
		t.Errorf("expected weapon name, got:\n%s", result)
	}
	if !strings.Contains(result, "[12/15]") {
		t.Errorf("expected ammo [12/15], got:\n%s", result)
	}
}

// TestHandleEquipment_ArmorSlotsAll verifies that all 7 armor slots appear.
func TestHandleEquipment_ArmorSlotsAll(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	armorSlots := []string{"head", "torso", "left_arm", "right_arm", "left_leg", "right_leg", "feet"}
	for _, slot := range armorSlots {
		if !strings.Contains(result, slot) {
			t.Errorf("expected armor slot %q in output:\n%s", slot, result)
		}
	}
}

// TestHandleEquipment_AccessorySlotsRing1Through5 verifies that ring_1 through ring_5
// are displayed (the required 5-ring display).
func TestHandleEquipment_AccessorySlotsRing1Through5(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	for i := 1; i <= 5; i++ {
		slot := "ring_" + string(rune('0'+i))
		if !strings.Contains(result, slot) {
			t.Errorf("expected %q in output:\n%s", slot, result)
		}
	}
}

// TestHandleEquipment_Ring6ThroughRing10NotDisplayed verifies that ring_6 through ring_10
// are not shown in the display (only 5 rings shown per spec).
func TestHandleEquipment_Ring6ThroughRing10NotDisplayed(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	for i := 6; i <= 10; i++ {
		slot := "ring_" + string(rune('0'+i))
		if strings.Contains(result, slot) {
			t.Errorf("did not expect %q in output (only ring_1..5 shown):\n%s", slot, result)
		}
	}
}

// TestHandleEquipment_NeckSlotDisplayed verifies that the neck accessory slot appears.
func TestHandleEquipment_NeckSlotDisplayed(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	if !strings.Contains(result, "neck") {
		t.Errorf("expected 'neck' in output:\n%s", result)
	}
}

// TestHandleEquipment_EmptyWeaponSlots verifies that both presets show "empty" for
// main and off hand when no weapons are equipped.
func TestHandleEquipment_EmptyWeaponSlots(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	// 2 presets × 2 slots = 4 "empty" occurrences minimum in weapon section.
	count := strings.Count(result, "empty")
	if count < 4 {
		t.Errorf("expected at least 4 'empty' occurrences, got %d:\n%s", count, result)
	}
}

// TestHandleEquipment_SecondPresetEquipped verifies that a weapon equipped in
// the non-active preset appears in the output.
func TestHandleEquipment_SecondPresetEquipped(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.Presets[1].EquipMainHand(pistolWeaponDef())

	result := command.HandleEquipment(sess)

	if !strings.Contains(result, "9mm Pistol") {
		t.Errorf("expected weapon in preset 2 to appear, got:\n%s", result)
	}
}

// TestProperty_HandleEquipment_AlwaysHasAllSections is a property-based test verifying
// that regardless of which preset is active, all three sections always appear.
func TestProperty_HandleEquipment_AlwaysHasAllSections(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := &session.PlayerSession{
			UID:        "test-uid",
			CharName:   "Tester",
			LoadoutSet: inventory.NewLoadoutSet(),
			Equipment:  inventory.NewEquipment(),
			Backpack:   inventory.NewBackpack(20, 100.0),
		}
		activeIdx := rapid.IntRange(0, len(sess.LoadoutSet.Presets)-1).Draw(rt, "activeIdx")
		sess.LoadoutSet.Active = activeIdx

		result := command.HandleEquipment(sess)

		sections := []string{"=== Weapons ===", "=== Armor ===", "=== Accessories ==="}
		for _, section := range sections {
			if !strings.Contains(result, section) {
				rt.Fatalf("missing section %q for activeIdx=%d:\n%s", section, activeIdx, result)
			}
		}

		// Exactly one [active] marker must appear.
		if strings.Count(result, "[active]") != 1 {
			rt.Fatalf("expected exactly 1 [active] marker:\n%s", result)
		}
	})
}
