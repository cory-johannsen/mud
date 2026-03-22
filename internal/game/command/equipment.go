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

// equipSlotLine formats a single slot row as "  <label padded to labelW> <item>".
//
// Precondition: labelW > 0; item is the slot display value.
// Postcondition: returns a string with no trailing newline.
func equipSlotLine(label, item string, labelW int) string {
	return fmt.Sprintf("  %-*s %s", labelW, label, item)
}

// zipCols zips two string slices into a two-column layout.
// Each row is: left padded to leftW chars + " | " + right.
//
// Precondition: leftW > 0; all strings in left and right are ASCII-only
// (byte length == visible width). If any left[i] is longer than leftW, the
// separator will still appear but column alignment will be broken for that row.
// Postcondition: returns a string with one \n-terminated row per max(len(left), len(right)).
func zipCols(left, right []string, leftW int) string {
	var sb strings.Builder
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		pad := leftW - len(l)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(l)
		sb.WriteString(strings.Repeat(" ", pad))
		sb.WriteString(" | ")
		sb.WriteString(r)
		sb.WriteString("\n")
	}
	return sb.String()
}

// HandleEquipment displays the complete equipment state for the player's session.
//
// When width >= 60, armor and accessory slots are rendered in two side-by-side columns.
// When width < 60, all slots are rendered in a single column (existing behaviour).
//
// Precondition: sess must not be nil; sess.LoadoutSet and sess.Equipment must not be nil.
// Postcondition: Returns a formatted multi-section string showing all weapon presets
// and all armor and accessory slots.
func HandleEquipment(sess *session.PlayerSession, width int) string {
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

	eq := sess.Equipment

	if width >= 60 {
		// Two-column layout: armor left, accessories right.
		// leftW is the padded width of a left-column slot line.
		const armorLabelW = 12
		const accLabelW = 21
		// leftW = 2 (indent) + armorLabelW (12) + 1 (space) + max expected item name (~20).
		// Must be >= the longest left-column line to keep columns aligned.
		// At 36, items up to 21 chars fit cleanly; longer names still appear but break alignment.
		const leftW = 36

		var leftLines []string
		for _, slot := range displayArmorSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			leftLines = append(leftLines, equipSlotLine(label, formatSlottedItem(eq.Armor[slot]), armorLabelW))
		}

		var rightLines []string
		neckLabel := inventory.SlotDisplayName("neck") + ":"
		rightLines = append(rightLines, equipSlotLine(neckLabel, formatSlottedItem(eq.Accessories[inventory.SlotNeck]), accLabelW))
		for _, slot := range displayLeftRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			rightLines = append(rightLines, equipSlotLine(label, formatSlottedItem(eq.Accessories[slot]), accLabelW))
		}
		for _, slot := range displayRightRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			rightLines = append(rightLines, equipSlotLine(label, formatSlottedItem(eq.Accessories[slot]), accLabelW))
		}

		sb.WriteString("\n")
		sb.WriteString(zipCols(leftLines, rightLines, leftW))
	} else {
		// Single-column layout (original behaviour).
		sb.WriteString("\n=== Armor ===\n")
		for _, slot := range displayArmorSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			sb.WriteString(fmt.Sprintf("  %-12s %s\n", label, formatSlottedItem(eq.Armor[slot])))
		}

		sb.WriteString("\n=== Accessories ===\n")
		neckLabel := inventory.SlotDisplayName("neck") + ":"
		sb.WriteString(fmt.Sprintf("  %-21s %s\n", neckLabel, formatSlottedItem(eq.Accessories[inventory.SlotNeck])))
		for _, slot := range displayLeftRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			sb.WriteString(fmt.Sprintf("  %-21s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
		}
		for _, slot := range displayRightRingSlots {
			label := inventory.SlotDisplayName(string(slot)) + ":"
			sb.WriteString(fmt.Sprintf("  %-21s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// formatSlottedItem returns a human-readable description of a slotted armor or accessory item.
// Applies modifier prefix (REQ-EM-18/19/20) and rarity color (REQ-EM-4).
//
// Precondition: item may be nil (represents an empty slot).
// Postcondition: Returns "empty" when item is nil, otherwise the (prefixed, colored) item display name.
func formatSlottedItem(item *inventory.SlottedItem) string {
	if item == nil {
		return "empty"
	}
	displayName := item.Name
	// REQ-EM-18/19/20: prefix modifier.
	switch item.Modifier {
	case "tuned":
		displayName = "Tuned " + displayName
	case "defective":
		displayName = "Defective " + displayName
	case "cursed":
		displayName = "Cursed " + displayName
	}
	// REQ-EM-4: apply rarity color (no-op when Rarity is empty).
	if item.Rarity != "" {
		displayName = inventory.RarityColoredName(item.Rarity, displayName)
	}
	return displayName
}
