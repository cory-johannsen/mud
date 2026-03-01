# Hand Armor Slot + Ring Rename + Human-Readable Display Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `hands` armor slot, rename ring slots to `left_ring_1`–`left_ring_5` / `right_ring_1`–`right_ring_5`, and display all slot names with human-readable labels.

**Architecture:** Add `SlotHands` to `ArmorSlot` constants and rename all ring `AccessorySlot` constants in `equipment.go`. Add a `SlotDisplayName(slot string) string` function mapping every slot identifier to its label. Update the equipment display command to call `SlotDisplayName()` and show the new ring groups. Update `validUnequipSlots` in `unequip.go` to match.

**Tech Stack:** Go, `pgregory.net/rapid` (property-based testing)

---

## Context for Implementer

### Key files
- `internal/game/inventory/equipment.go` — defines `ArmorSlot`, `AccessorySlot` constants, `Equipment` struct, `NewEquipment()`
- `internal/game/inventory/equipment_test.go` — tests for the above
- `internal/game/command/equipment.go` — `HandleEquipment()` display function
- `internal/game/command/equipment_test.go` — tests for `HandleEquipment()`
- `internal/game/command/unequip.go` — `validUnequipSlots` list + `HandleUnequip()`
- `internal/game/command/unequip_test.go` — tests for `HandleUnequip()`

### Current slot constants (to be changed)
```go
// ArmorSlot — 7 slots
SlotHead, SlotLeftArm, SlotRightArm, SlotTorso, SlotLeftLeg, SlotRightLeg, SlotFeet

// AccessorySlot — 11 slots
SlotNeck, SlotRing1..SlotRing10  // values: "ring_1".."ring_10"
```

### After this plan
```go
// ArmorSlot — 8 slots (hands added)
SlotHead, SlotLeftArm, SlotRightArm, SlotTorso, SlotHands, SlotLeftLeg, SlotRightLeg, SlotFeet

// AccessorySlot — 11 slots (ring names changed)
SlotNeck,
SlotLeftRing1..SlotLeftRing5,   // "left_ring_1".."left_ring_5"
SlotRightRing1..SlotRightRing5  // "right_ring_1".."right_ring_5"
```

### Run tests with
```bash
go test ./internal/game/inventory/... ./internal/game/command/... -v
```

---

## Task 1: Update slot constants and add SlotDisplayName

**Files:**
- Modify: `internal/game/inventory/equipment.go`
- Modify: `internal/game/inventory/equipment_test.go`

**Step 1: Write the failing tests first**

Replace `internal/game/inventory/equipment_test.go` with the following complete file. These tests will fail until the implementation is updated:

```go
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
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/game/inventory/... -v -run TestEquipment_ArmorSlotCount
```

Expected: FAIL with `undefined: inventory.SlotHands` or `expected 8 armor slots, got 7`

**Step 3: Update `internal/game/inventory/equipment.go`**

Replace the entire file with:

```go
package inventory

// ArmorSlot identifies a body-armor equipment slot.
type ArmorSlot string

const (
	// SlotHead is the head armor slot.
	SlotHead ArmorSlot = "head"
	// SlotLeftArm is the left-arm armor slot.
	SlotLeftArm ArmorSlot = "left_arm"
	// SlotRightArm is the right-arm armor slot.
	SlotRightArm ArmorSlot = "right_arm"
	// SlotTorso is the torso armor slot.
	SlotTorso ArmorSlot = "torso"
	// SlotHands is the hands armor slot (covers both hands).
	SlotHands ArmorSlot = "hands"
	// SlotLeftLeg is the left-leg armor slot.
	SlotLeftLeg ArmorSlot = "left_leg"
	// SlotRightLeg is the right-leg armor slot.
	SlotRightLeg ArmorSlot = "right_leg"
	// SlotFeet is the feet armor slot.
	SlotFeet ArmorSlot = "feet"
)

// AccessorySlot identifies an accessory equipment slot.
type AccessorySlot string

const (
	// SlotNeck is the neck accessory slot.
	SlotNeck AccessorySlot = "neck"
	// SlotLeftRing1 through SlotLeftRing5 are the left-hand ring slots.
	SlotLeftRing1 AccessorySlot = "left_ring_1"
	SlotLeftRing2 AccessorySlot = "left_ring_2"
	SlotLeftRing3 AccessorySlot = "left_ring_3"
	SlotLeftRing4 AccessorySlot = "left_ring_4"
	SlotLeftRing5 AccessorySlot = "left_ring_5"
	// SlotRightRing1 through SlotRightRing5 are the right-hand ring slots.
	SlotRightRing1 AccessorySlot = "right_ring_1"
	SlotRightRing2 AccessorySlot = "right_ring_2"
	SlotRightRing3 AccessorySlot = "right_ring_3"
	SlotRightRing4 AccessorySlot = "right_ring_4"
	SlotRightRing5 AccessorySlot = "right_ring_5"
)

// slotDisplayNames maps every slot identifier to its human-readable label.
var slotDisplayNames = map[string]string{
	"head":      "Head",
	"left_arm":  "Left Arm",
	"right_arm": "Right Arm",
	"torso":     "Torso",
	"hands":     "Hands",
	"left_leg":  "Left Leg",
	"right_leg": "Right Leg",
	"feet":      "Feet",
	"neck":      "Neck",
	"left_ring_1":  "Left Hand Ring 1",
	"left_ring_2":  "Left Hand Ring 2",
	"left_ring_3":  "Left Hand Ring 3",
	"left_ring_4":  "Left Hand Ring 4",
	"left_ring_5":  "Left Hand Ring 5",
	"right_ring_1": "Right Hand Ring 1",
	"right_ring_2": "Right Hand Ring 2",
	"right_ring_3": "Right Hand Ring 3",
	"right_ring_4": "Right Hand Ring 4",
	"right_ring_5": "Right Hand Ring 5",
	"main": "Main Hand",
	"off":  "Off Hand",
}

// SlotDisplayName returns the human-readable label for a slot identifier.
//
// Precondition: slot is a non-empty string.
// Postcondition: returns the registered label, or slot itself if not found.
func SlotDisplayName(slot string) string {
	if label, ok := slotDisplayNames[slot]; ok {
		return label
	}
	return slot
}

// SlottedItem records an item occupying any equipment slot (armor or accessory).
type SlottedItem struct {
	// ItemDefID is the unique item definition identifier.
	ItemDefID string
	// Name is the display name shown to the player.
	Name string
}

// Equipment holds all armor and accessory slots for a character.
// These slots are shared across all weapon presets.
type Equipment struct {
	// Armor maps each ArmorSlot to the item equipped there, or nil when empty.
	Armor map[ArmorSlot]*SlottedItem
	// Accessories maps each AccessorySlot to the item equipped there, or nil when empty.
	Accessories map[AccessorySlot]*SlottedItem
}

// NewEquipment returns an empty Equipment with initialised maps.
//
// Postcondition: Armor and Accessories are non-nil, empty maps.
func NewEquipment() *Equipment {
	return &Equipment{
		Armor:       make(map[ArmorSlot]*SlottedItem),
		Accessories: make(map[AccessorySlot]*SlottedItem),
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/game/inventory/... -v
```

Expected: all tests PASS

**Step 5: Commit**

```bash
git add internal/game/inventory/equipment.go internal/game/inventory/equipment_test.go
git commit -m "feat: add SlotHands, rename ring slots left/right, add SlotDisplayName"
```

---

## Task 2: Update equipment display command

**Files:**
- Modify: `internal/game/command/equipment.go`
- Modify: `internal/game/command/equipment_test.go`

**Step 1: Write the failing tests**

Open `internal/game/command/equipment_test.go` and add these tests (keep all existing tests, add these new ones):

```go
func TestHandleEquipment_HandsSlotDisplayed(t *testing.T) {
	sess := newTestSession()
	output := HandleEquipment(sess)
	if !strings.Contains(output, "Hands:") {
		t.Errorf("expected 'Hands:' in output, got:\n%s", output)
	}
}

func TestHandleEquipment_HumanReadableArmorLabels(t *testing.T) {
	sess := newTestSession()
	output := HandleEquipment(sess)
	wantLabels := []string{"Head:", "Torso:", "Left Arm:", "Right Arm:", "Hands:", "Left Leg:", "Right Leg:", "Feet:"}
	for _, label := range wantLabels {
		if !strings.Contains(output, label) {
			t.Errorf("expected label %q in output, got:\n%s", label, output)
		}
	}
}

func TestHandleEquipment_HumanReadableRingLabels(t *testing.T) {
	sess := newTestSession()
	output := HandleEquipment(sess)
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
	output := HandleEquipment(sess)
	oldNames := []string{"ring_1:", "ring_2:", "ring_3:", "ring_4:", "ring_5:"}
	for _, old := range oldNames {
		if strings.Contains(output, old) {
			t.Errorf("old slot name %q should not appear in output, got:\n%s", old, output)
		}
	}
}
```

**Step 2: Run to verify they fail**

```bash
go test ./internal/game/command/... -v -run TestHandleEquipment_HandsSlotDisplayed
```

Expected: FAIL (Hands slot not in output yet)

**Step 3: Update `internal/game/command/equipment.go`**

Replace the entire file with:

```go
package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// displayArmorSlots is the ordered list of armor slots shown by HandleEquipment.
var displayArmorSlots = []inventory.ArmorSlot{
	inventory.SlotHead,
	inventory.SlotTorso,
	inventory.SlotLeftArm,
	inventory.SlotRightArm,
	inventory.SlotHands,
	inventory.SlotLeftLeg,
	inventory.SlotRightLeg,
	inventory.SlotFeet,
}

// displayLeftRingSlots is the ordered list of left-hand ring slots shown by HandleEquipment.
var displayLeftRingSlots = []inventory.AccessorySlot{
	inventory.SlotLeftRing1,
	inventory.SlotLeftRing2,
	inventory.SlotLeftRing3,
	inventory.SlotLeftRing4,
	inventory.SlotLeftRing5,
}

// displayRightRingSlots is the ordered list of right-hand ring slots shown by HandleEquipment.
var displayRightRingSlots = []inventory.AccessorySlot{
	inventory.SlotRightRing1,
	inventory.SlotRightRing2,
	inventory.SlotRightRing3,
	inventory.SlotRightRing4,
	inventory.SlotRightRing5,
}

// HandleEquipment displays the complete equipment state for the player's session.
//
// Precondition: sess must not be nil; sess.LoadoutSet and sess.Equipment must not be nil.
// Postcondition: Returns a formatted multi-section string showing all weapon presets,
// all 8 armor slots, and neck + left/right ring accessory slots with human-readable labels.
func HandleEquipment(sess *session.PlayerSession) string {
	var sb strings.Builder

	// === Weapons ===
	sb.WriteString("=== Weapons ===\n")
	ls := sess.LoadoutSet
	for i, preset := range ls.Presets {
		label := fmt.Sprintf("Preset %d", i+1)
		if i == ls.Active {
			label += " [active]"
		}
		sb.WriteString(label + ":\n")
		sb.WriteString("  " + inventory.SlotDisplayName("main") + ": " + formatEquippedWeapon(preset.MainHand) + "\n")
		sb.WriteString("  " + inventory.SlotDisplayName("off") + ":  " + formatEquippedWeapon(preset.OffHand) + "\n")
	}

	// === Armor ===
	sb.WriteString("\n=== Armor ===\n")
	eq := sess.Equipment
	for _, slot := range displayArmorSlots {
		label := inventory.SlotDisplayName(string(slot)) + ":"
		item := eq.Armor[slot]
		sb.WriteString(fmt.Sprintf("  %-12s %s\n", label, formatSlottedItem(item)))
	}

	// === Accessories ===
	sb.WriteString("\n=== Accessories ===\n")
	neckLabel := inventory.SlotDisplayName("neck") + ":"
	sb.WriteString(fmt.Sprintf("  %-20s %s\n", neckLabel, formatSlottedItem(eq.Accessories[inventory.SlotNeck])))
	for _, slot := range displayLeftRingSlots {
		label := inventory.SlotDisplayName(string(slot)) + ":"
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
	}
	for _, slot := range displayRightRingSlots {
		label := inventory.SlotDisplayName(string(slot)) + ":"
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// formatSlottedItem returns a human-readable description of a slotted armor or accessory item.
//
// Precondition: item may be nil (represents an empty slot).
// Postcondition: Returns "empty" when item is nil, otherwise the item's display name.
func formatSlottedItem(item *inventory.SlottedItem) string {
	if item == nil {
		return "empty"
	}
	return item.Name
}
```

**Step 4: Run all equipment tests**

```bash
go test ./internal/game/command/... -v -run TestHandleEquipment
```

Expected: all PASS

**Step 5: Commit**

```bash
git add internal/game/command/equipment.go internal/game/command/equipment_test.go
git commit -m "feat: update equipment display with hands slot and human-readable labels"
```

---

## Task 3: Update unequip slot list

**Files:**
- Modify: `internal/game/command/unequip.go`
- Modify: `internal/game/command/unequip_test.go`

**Step 1: Write the failing tests**

Open `internal/game/command/unequip_test.go` and add these tests (keep existing tests):

```go
func TestHandleUnequip_HandsSlotAccepted(t *testing.T) {
	sess := newTestSessionEquipped()
	result := HandleUnequip(sess, "hands")
	if strings.Contains(result, "Unknown slot") {
		t.Errorf("expected hands slot to be valid, got: %s", result)
	}
}

func TestHandleUnequip_LeftRightRingSlotsAccepted(t *testing.T) {
	sess := newTestSessionEquipped()
	newSlots := []string{
		"left_ring_1", "left_ring_2", "left_ring_3", "left_ring_4", "left_ring_5",
		"right_ring_1", "right_ring_2", "right_ring_3", "right_ring_4", "right_ring_5",
	}
	for _, slot := range newSlots {
		result := HandleUnequip(sess, slot)
		if strings.Contains(result, "Unknown slot") {
			t.Errorf("expected slot %q to be valid, got: %s", slot, result)
		}
	}
}

func TestHandleUnequip_OldRingSlotsRejected(t *testing.T) {
	sess := newTestSessionEquipped()
	oldSlots := []string{
		"ring_1", "ring_2", "ring_3", "ring_4", "ring_5",
		"ring_6", "ring_7", "ring_8", "ring_9", "ring_10",
	}
	for _, slot := range oldSlots {
		result := HandleUnequip(sess, slot)
		if !strings.Contains(result, "Unknown slot") {
			t.Errorf("expected old slot %q to be rejected, got: %s", slot, result)
		}
	}
}
```

**Step 2: Run to verify they fail**

```bash
go test ./internal/game/command/... -v -run TestHandleUnequip_HandsSlotAccepted
```

Expected: FAIL (`hands` not in valid slots yet)

**Step 3: Update `validUnequipSlots` in `internal/game/command/unequip.go`**

Change only the `validUnequipSlots` variable (lines 11–17). Replace with:

```go
var validUnequipSlots = []string{
	"main", "off",
	"head", "torso", "left_arm", "right_arm", "hands", "left_leg", "right_leg", "feet",
	"neck",
	"left_ring_1", "left_ring_2", "left_ring_3", "left_ring_4", "left_ring_5",
	"right_ring_1", "right_ring_2", "right_ring_3", "right_ring_4", "right_ring_5",
}
```

**Step 4: Run all unequip tests**

```bash
go test ./internal/game/command/... -v -run TestHandleUnequip
```

Expected: all PASS

**Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all PASS

**Step 6: Commit**

```bash
git add internal/game/command/unequip.go internal/game/command/unequip_test.go
git commit -m "feat: update unequip slot list with hands and left/right ring slots"
```

---

## Task 4: Update FEATURES.md

**Files:**
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Mark feature complete**

Change:
```
- [ ] The player should have a hand armor slot. The player should have 5 rings on each hand and manage them separately.  The equipment display should use human readable names for the item slots.
```
To:
```
- [x] The player should have a hand armor slot. The player should have 5 rings on each hand and manage them separately.  The equipment display should use human readable names for the item slots.
```

**Step 2: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "feat: mark hand armor slot + ring rename complete in FEATURES.md"
```
