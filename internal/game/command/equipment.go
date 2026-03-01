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
	sb.WriteString(fmt.Sprintf("  %-21s %s\n", neckLabel, formatSlottedItem(eq.Accessories[inventory.SlotNeck])))
	for _, slot := range displayLeftRingSlots {
		label := inventory.SlotDisplayName(string(slot)) + ":"
		sb.WriteString(fmt.Sprintf("  %-21s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
	}
	for _, slot := range displayRightRingSlots {
		label := inventory.SlotDisplayName(string(slot)) + ":"
		sb.WriteString(fmt.Sprintf("  %-21s %s\n", label, formatSlottedItem(eq.Accessories[slot])))
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
