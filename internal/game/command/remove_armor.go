package command

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleRemoveArmor processes the "remove <slot>" command, unequipping armor from the given slot.
//
// Precondition: sess and reg must be non-nil; arg is the requested slot name string.
// Postcondition: On success, the item is moved from the armor slot to the backpack and the slot is
// set to nil, and the returned string contains "Removed". On failure, the slot is unchanged and the
// returned string describes the error.
func HandleRemoveArmor(sess *session.PlayerSession, reg *inventory.Registry, arg string) string {
	slot := inventory.ArmorSlot(arg)
	if _, ok := inventory.ValidArmorSlots()[slot]; !ok {
		return fmt.Sprintf("Unknown slot %q. Valid slots: head, torso, left_arm, right_arm, hands, left_leg, right_leg, feet.", arg)
	}

	slotted := sess.Equipment.Armor[slot]
	if slotted == nil {
		return fmt.Sprintf("You're wearing nothing on your %s.", slot)
	}

	// Determine the ItemDef ID to return to the backpack.
	// ItemByArmorRef resolves armorDefID -> ItemDef; if the registry contains the mapping
	// we use the canonical item ID, otherwise fall back to the stored ItemDefID.
	itemDefID := slotted.ItemDefID
	if itemDef, ok := reg.ItemByArmorRef(slotted.ItemDefID); ok {
		itemDefID = itemDef.ID
	}

	if _, err := sess.Backpack.Add(itemDefID, 1, reg); err != nil {
		return fmt.Sprintf("Cannot remove %s: inventory full.", slotted.Name)
	}

	sess.Equipment.Armor[slot] = nil
	return fmt.Sprintf("Removed %s.", slotted.Name)
}
