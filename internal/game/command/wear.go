package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleWear processes the "wear <item_id> <slot>" command.
//
// Precondition: sess and reg must be non-nil; arg is the raw argument string.
// Postcondition: On success, moves the item from the backpack into the equipment slot and returns
// a confirmation message. On failure, returns a descriptive error message and state is unchanged.
func HandleWear(sess *session.PlayerSession, reg *inventory.Registry, arg string) string {
	parts := strings.Fields(strings.TrimSpace(arg))
	if len(parts) < 2 {
		return "Usage: wear <item_id> <slot>"
	}
	itemID := parts[0]
	slotStr := parts[1]

	// Validate that the slot is a legal armor slot.
	targetSlot := inventory.ArmorSlot(slotStr)
	if _, ok := inventory.ValidArmorSlots()[targetSlot]; !ok {
		return fmt.Sprintf("Unknown slot %q. Valid: head, torso, left_arm, right_arm, hands, left_leg, right_leg, feet.", slotStr)
	}

	// Locate the item in the backpack.
	instances := sess.Backpack.FindByItemDefID(itemID)
	if len(instances) == 0 {
		return fmt.Sprintf("Item %q not found in inventory.", itemID)
	}
	inst := instances[0]

	// Resolve the ItemDef.
	itemDef, ok := reg.Item(itemID)
	if !ok {
		return fmt.Sprintf("Item definition %q not found.", itemID)
	}
	if itemDef.Kind != inventory.KindArmor {
		return fmt.Sprintf("%q is not armor.", itemDef.Name)
	}
	if itemDef.ArmorRef == "" {
		return fmt.Sprintf("%q has no armor definition.", itemDef.Name)
	}

	// Resolve the ArmorDef.
	armorDef, ok := reg.Armor(itemDef.ArmorRef)
	if !ok {
		return fmt.Sprintf("Armor definition %q not found.", itemDef.ArmorRef)
	}

	// Validate that the armor's slot matches the requested slot.
	if armorDef.Slot != targetSlot {
		return fmt.Sprintf("%q must be worn on %s, not %s.", armorDef.Name, armorDef.Slot, targetSlot)
	}

	// Remove the item from the backpack before equipping.
	if err := sess.Backpack.Remove(inst.InstanceID, 1); err != nil {
		return fmt.Sprintf("Could not remove item from inventory: %v", err)
	}

	// If the slot is already occupied, return the previous item to the backpack.
	if prev := sess.Equipment.Armor[targetSlot]; prev != nil {
		// prev.ItemDefID holds the ArmorDef ID; find the corresponding ItemDef to add back.
		prevItemDef, ok := reg.ItemByArmorRef(prev.ItemDefID)
		if !ok {
			// Rollback: restore the item that was removed.
			_, _ = sess.Backpack.Add(itemID, 1, reg)
			return fmt.Sprintf("Cannot find item definition for %s to return to inventory.", prev.Name)
		}
		if _, err := sess.Backpack.Add(prevItemDef.ID, 1, reg); err != nil {
			// Rollback: restore the item that was removed.
			_, _ = sess.Backpack.Add(itemID, 1, reg)
			return fmt.Sprintf("Inventory full: cannot unequip previous %s.", prev.Name)
		}
	}

	// Store the ArmorDef ID in ItemDefID so ComputedDefenses can resolve it.
	sess.Equipment.Armor[targetSlot] = &inventory.SlottedItem{
		ItemDefID: armorDef.ID,
		Name:      armorDef.Name,
	}

	return fmt.Sprintf("Wore %s.", armorDef.Name)
}
