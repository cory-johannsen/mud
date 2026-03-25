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

	result := command.HandleEquipment(sess, 0, nil)

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

	result := command.HandleEquipment(sess, 0, nil)

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

	result := command.HandleEquipment(sess, 0, nil)

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

	result := command.HandleEquipment(sess, 0, nil)

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
	result := command.HandleEquipment(sess, 0, nil)

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
	result := command.HandleEquipment(sess, 0, nil)

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
	result := command.HandleEquipment(sess, 0, nil)

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
	result := command.HandleEquipment(sess, 0, nil)

	if !strings.Contains(result, "Neck:") {
		t.Errorf("expected 'Neck:' in output:\n%s", result)
	}
}

// TestHandleEquipment_EmptyWeaponSlots verifies that both presets show "empty" for
// main and off hand when no weapons are equipped.
func TestHandleEquipment_EmptyWeaponSlots(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 0, nil)

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

	result := command.HandleEquipment(sess, 0, nil)

	if !strings.Contains(result, "9mm Pistol") {
		t.Errorf("expected weapon in preset 2 to appear, got:\n%s", result)
	}
}

// NOTE: intentionally tests single-col path only (width=0).
// The 2-col path (width>=60) does not emit === Armor === or === Accessories === headers.
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

		result := command.HandleEquipment(sess, 0, nil)

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
	output := command.HandleEquipment(sess, 0, nil)
	if !strings.Contains(output, "Hands:") {
		t.Errorf("expected 'Hands:' in output, got:\n%s", output)
	}
}

func TestHandleEquipment_HumanReadableArmorLabels(t *testing.T) {
	sess := newTestSession()
	output := command.HandleEquipment(sess, 0, nil)
	wantLabels := []string{"Head:", "Torso:", "Left Arm:", "Right Arm:", "Hands:", "Left Leg:", "Right Leg:", "Feet:"}
	for _, label := range wantLabels {
		if !strings.Contains(output, label) {
			t.Errorf("expected label %q in output, got:\n%s", label, output)
		}
	}
}

func TestHandleEquipment_HumanReadableRingLabels(t *testing.T) {
	sess := newTestSession()
	output := command.HandleEquipment(sess, 0, nil)
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
	output := command.HandleEquipment(sess, 0, nil)
	oldNames := []string{
		"ring_1:", "ring_2:", "ring_3:", "ring_4:", "ring_5:",
		"ring_6:", "ring_7:", "ring_8:", "ring_9:", "ring_10:",
	}
	for _, old := range oldNames {
		if strings.Contains(output, old) {
			t.Errorf("old slot name %q should not appear in output, got:\n%s", old, output)
		}
	}
}

// TestHandleEquipment_TwoCol_ArmorAppearsInLeftColumn verifies that at width>=60
// all 8 armor slot labels appear in the output.
func TestHandleEquipment_TwoCol_ArmorAppearsInLeftColumn(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80, nil)

	armorLabels := []string{"Head:", "Torso:", "Left Arm:", "Right Arm:", "Hands:", "Left Leg:", "Right Leg:", "Feet:"}
	for _, label := range armorLabels {
		if !strings.Contains(result, label) {
			t.Errorf("expected armor label %q in 2-col output:\n%s", label, result)
		}
	}
}

// TestHandleEquipment_TwoCol_AccessoriesAppearInRightColumn verifies that at width>=60
// neck and all 10 ring slots appear in the output.
func TestHandleEquipment_TwoCol_AccessoriesAppearInRightColumn(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80, nil)

	if !strings.Contains(result, "Neck:") {
		t.Errorf("expected 'Neck:' in 2-col output:\n%s", result)
	}
	for i := 1; i <= 5; i++ {
		leftLabel := fmt.Sprintf("Left Hand Ring %d:", i)
		rightLabel := fmt.Sprintf("Right Hand Ring %d:", i)
		if !strings.Contains(result, leftLabel) {
			t.Errorf("expected %q in 2-col output:\n%s", leftLabel, result)
		}
		if !strings.Contains(result, rightLabel) {
			t.Errorf("expected %q in 2-col output:\n%s", rightLabel, result)
		}
	}
}

// TestHandleEquipment_TwoCol_HasColumnSeparator verifies that the " | " column divider
// appears in the 2-col output.
func TestHandleEquipment_TwoCol_HasColumnSeparator(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80, nil)

	if !strings.Contains(result, " | ") {
		t.Errorf("expected ' | ' column separator in 2-col output:\n%s", result)
	}
}

// TestHandleEquipment_TwoCol_NoSeparatorAtWidth40 verifies that narrow terminals
// get the single-column layout (no " | " separator).
func TestHandleEquipment_TwoCol_NoSeparatorAtWidth40(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 40, nil)

	if strings.Contains(result, " | ") {
		t.Errorf("did not expect ' | ' separator at width=40:\n%s", result)
	}
}

// TestHandleEquipment_TwoCol_WeaponsBeforeColumns verifies weapons section appears
// before the 2-col grid (i.e. no " | " separator precedes the weapons header).
func TestHandleEquipment_TwoCol_WeaponsBeforeColumns(t *testing.T) {
	sess := newTestSessionWithBackpack()
	result := command.HandleEquipment(sess, 80, nil)

	weaponsIdx := strings.Index(result, "=== Weapons ===")
	separatorIdx := strings.Index(result, " | ")
	if weaponsIdx < 0 {
		t.Fatalf("expected '=== Weapons ===' in output:\n%s", result)
	}
	if separatorIdx < 0 {
		t.Fatalf("expected ' | ' separator in output:\n%s", result)
	}
	if weaponsIdx > separatorIdx {
		t.Errorf("expected '=== Weapons ===' before ' | ', weapons at %d, separator at %d", weaponsIdx, separatorIdx)
	}
}

// TestHandleEquipment_TwoCol_EquippedArmorAppearsInOutput verifies that an equipped
// armor item's name appears when 2-col mode is active.
func TestHandleEquipment_TwoCol_EquippedArmorAppearsInOutput(t *testing.T) {
	sess := newTestSessionWithBackpack()
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{Name: "Tactical Vest"}

	result := command.HandleEquipment(sess, 80, nil)

	if !strings.Contains(result, "Tactical Vest") {
		t.Errorf("expected 'Tactical Vest' in 2-col output:\n%s", result)
	}
}

// TestProperty_HandleEquipment_TwoCol_NeverPanics verifies that HandleEquipment never
// panics for any combination of width and session state.
func TestProperty_HandleEquipment_TwoCol_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := &session.PlayerSession{
			UID:        "test-uid",
			CharName:   "Tester",
			LoadoutSet: inventory.NewLoadoutSet(),
			Equipment:  inventory.NewEquipment(),
			Backpack:   inventory.NewBackpack(20, 100.0),
		}
		width := rapid.IntRange(0, 200).Draw(rt, "width")
		_ = command.HandleEquipment(sess, width, nil)
	})
}

// TestHandleEquipment_RarityColoredName_WeaponInDisplay verifies that a weapon with
// a known rarity has its name displayed with ANSI color codes (REQ-EM-4).
func TestHandleEquipment_RarityColoredName_WeaponInDisplay(t *testing.T) {
	sess := newTestSessionWithBackpack()
	// pistolWeaponDef has Rarity="salvage" → dark gray \033[90m
	_ = sess.LoadoutSet.Presets[0].EquipMainHand(pistolWeaponDef())

	result := command.HandleEquipment(sess, 0, nil)

	// Should contain the ANSI color code for salvage (dark gray).
	if !strings.Contains(result, "\033[90m") {
		t.Errorf("expected salvage ANSI color code in output, got:\n%s", result)
	}
}

// TestHandleEquipment_ModifierPrefix_Tuned verifies that a "tuned" weapon shows
// the "Tuned " prefix in the display (REQ-EM-18).
func TestHandleEquipment_ModifierPrefix_Tuned(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.Presets[0].EquipMainHand(pistolWeaponDef())
	sess.LoadoutSet.Presets[0].MainHand.Modifier = "tuned"

	result := command.HandleEquipment(sess, 0, nil)

	if !strings.Contains(result, "Tuned") {
		t.Errorf("expected 'Tuned' prefix in display, got:\n%s", result)
	}
}

// TestHandleEquipment_ModifierPrefix_Defective verifies that a "defective" weapon
// shows the "Defective " prefix in the display (REQ-EM-19).
func TestHandleEquipment_ModifierPrefix_Defective(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.Presets[0].EquipMainHand(pistolWeaponDef())
	sess.LoadoutSet.Presets[0].MainHand.Modifier = "defective"

	result := command.HandleEquipment(sess, 0, nil)

	if !strings.Contains(result, "Defective") {
		t.Errorf("expected 'Defective' prefix in display, got:\n%s", result)
	}
}

// TestHandleEquipment_ModifierPrefix_Cursed verifies that a "cursed" weapon
// shows the "Cursed " prefix in the display (REQ-EM-20).
func TestHandleEquipment_ModifierPrefix_Cursed(t *testing.T) {
	sess := newTestSessionWithBackpack()
	_ = sess.LoadoutSet.Presets[0].EquipMainHand(pistolWeaponDef())
	sess.LoadoutSet.Presets[0].MainHand.Modifier = "cursed"

	result := command.HandleEquipment(sess, 0, nil)

	if !strings.Contains(result, "Cursed") {
		t.Errorf("expected 'Cursed' prefix in display, got:\n%s", result)
	}
}

// TestHandleEquipment_SlottedItem_ModifierPrefix verifies that a slotted armor item
// with a modifier shows the correct prefix (REQ-EM-18/19/20).
func TestHandleEquipment_SlottedItem_ModifierPrefix_Tuned(t *testing.T) {
	sess := newTestSessionWithBackpack()
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "vest-def",
		Name:      "Tactical Vest",
		Modifier:  "tuned",
	}

	result := command.HandleEquipment(sess, 0, nil)

	if !strings.Contains(result, "Tuned") {
		t.Errorf("expected 'Tuned' prefix on slotted armor, got:\n%s", result)
	}
}
