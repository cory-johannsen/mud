package command_test

import (
	"fmt"
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

// TestHandleEquipment_ArmorSlotsAll verifies that all 8 armor slots appear using human-readable labels.
func TestHandleEquipment_ArmorSlotsAll(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	armorLabels := []string{"Head:", "Torso:", "Left Arm:", "Right Arm:", "Hands:", "Left Leg:", "Right Leg:", "Feet:"}
	for _, label := range armorLabels {
		if !strings.Contains(result, label) {
			t.Errorf("expected armor label %q in output:\n%s", label, result)
		}
	}
}

// TestHandleEquipment_AccessorySlotsRing1Through5 verifies that left/right hand ring 1 through 5
// are displayed using human-readable labels.
func TestHandleEquipment_AccessorySlotsRing1Through5(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	for i := 1; i <= 5; i++ {
		leftLabel := fmt.Sprintf("Left Hand Ring %d:", i)
		rightLabel := fmt.Sprintf("Right Hand Ring %d:", i)
		if !strings.Contains(result, leftLabel) {
			t.Errorf("expected %q in output:\n%s", leftLabel, result)
		}
		if !strings.Contains(result, rightLabel) {
			t.Errorf("expected %q in output:\n%s", rightLabel, result)
		}
	}
}

// TestHandleEquipment_Ring6ThroughRing10NotDisplayed verifies that left/right ring 6 through 10
// are not shown in the display (only 5 rings per hand shown per spec).
func TestHandleEquipment_Ring6ThroughRing10NotDisplayed(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	for i := 6; i <= 10; i++ {
		leftLabel := fmt.Sprintf("Left Hand Ring %d:", i)
		rightLabel := fmt.Sprintf("Right Hand Ring %d:", i)
		if strings.Contains(result, leftLabel) {
			t.Errorf("did not expect %q in output:\n%s", leftLabel, result)
		}
		if strings.Contains(result, rightLabel) {
			t.Errorf("did not expect %q in output:\n%s", rightLabel, result)
		}
	}
}

// TestHandleEquipment_NeckSlotDisplayed verifies that the neck accessory slot appears.
func TestHandleEquipment_NeckSlotDisplayed(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess)

	if !strings.Contains(result, "Neck:") {
		t.Errorf("expected 'Neck:' in output:\n%s", result)
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

func TestHandleEquipment_HandsSlotDisplayed(t *testing.T) {
	sess := newTestSession()
	output := command.HandleEquipment(sess)
	if !strings.Contains(output, "Hands:") {
		t.Errorf("expected 'Hands:' in output, got:\n%s", output)
	}
}

func TestHandleEquipment_HumanReadableArmorLabels(t *testing.T) {
	sess := newTestSession()
	output := command.HandleEquipment(sess)
	wantLabels := []string{"Head:", "Torso:", "Left Arm:", "Right Arm:", "Hands:", "Left Leg:", "Right Leg:", "Feet:"}
	for _, label := range wantLabels {
		if !strings.Contains(output, label) {
			t.Errorf("expected label %q in output, got:\n%s", label, output)
		}
	}
}

func TestHandleEquipment_HumanReadableRingLabels(t *testing.T) {
	sess := newTestSession()
	output := command.HandleEquipment(sess)
	wantLabels := []string{
		"Left Hand Ring 1:", "Left Hand Ring 2:", "Left Hand Ring 3:",
		"Left Hand Ring 4:", "Left Hand Ring 5:",
		"Right Hand Ring 1:", "Right Hand Ring 2:", "Right Hand Ring 3:",
		"Right Hand Ring 4:", "Right Hand Ring 5:",
	}
	for _, label := range wantLabels {
		if !strings.Contains(output, label) {
			t.Errorf("expected label %q in output, got:\n%s", label, output)
		}
	}
}

func TestHandleEquipment_OldRingNamesNotDisplayed(t *testing.T) {
	sess := newTestSession()
	output := command.HandleEquipment(sess)
	oldNames := []string{"ring_1:", "ring_2:", "ring_3:", "ring_4:", "ring_5:"}
	for _, old := range oldNames {
		if strings.Contains(output, old) {
			t.Errorf("old slot name %q should not appear in output, got:\n%s", old, output)
		}
	}
}
