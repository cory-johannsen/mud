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
	inventory.SlotLeftLeg,
	inventory.SlotRightLeg,
	inventory.SlotFeet,
}

// displayAccessorySlots is the ordered list of accessory slots shown by HandleEquipment.
// Only ring_1 through ring_5 are shown per the display specification.
var displayAccessorySlots = []inventory.AccessorySlot{
	inventory.SlotNeck,
	inventory.SlotRing1,
	inventory.SlotRing2,
	inventory.SlotRing3,
	inventory.SlotRing4,
	inventory.SlotRing5,
}

// HandleEquipment displays the complete equipment state for the player's session.
//
// Precondition: sess must not be nil; sess.LoadoutSet and sess.Equipment must not be nil.
// Postcondition: Returns a formatted multi-section string showing all weapon presets,
// all 7 armor slots, and neck + ring_1..ring_5 accessory slots.
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
		sb.WriteString("  Main: " + formatEquippedWeapon(preset.MainHand) + "\n")
		sb.WriteString("  Off:  " + formatEquippedWeapon(preset.OffHand) + "\n")
	}

	// === Armor ===
	sb.WriteString("\n=== Armor ===\n")
	eq := sess.Equipment
	for _, slot := range displayArmorSlots {
		item := eq.Armor[slot]
		sb.WriteString(fmt.Sprintf("  %-11s %s\n", string(slot)+":", formatSlottedItem(item)))
	}

	// === Accessories ===
	sb.WriteString("\n=== Accessories ===\n")
	for _, slot := range displayAccessorySlots {
		item := eq.Accessories[slot]
		sb.WriteString(fmt.Sprintf("  %-8s %s\n", string(slot)+":", formatSlottedItem(item)))
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
