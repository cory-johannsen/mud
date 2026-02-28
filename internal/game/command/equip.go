package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleEquip processes the "equip" command.
// arg is expected to be "<itemDefID> <slot>" where slot is "main" or "off".
//
// Precondition: sess must not be nil; sess.LoadoutSet, sess.Backpack must not be nil; reg must not be nil.
// Postcondition: On success, removes the item from the backpack, equips it in the named slot, and
// returns a confirmation. On failure, the backpack and preset state are unchanged.
func HandleEquip(sess *session.PlayerSession, reg *inventory.Registry, arg string) string {
	arg = strings.TrimSpace(arg)

	// Split into at most two tokens: itemDefID and optional slot.
	parts := strings.Fields(arg)
	if len(parts) == 0 {
		return "Usage: equip <item_id> <main|off>"
	}

	itemDefID := parts[0]

	// Require explicit slot for weapons.
	if len(parts) < 2 {
		return "specify main or off"
	}
	slot := parts[1]
	if slot != "main" && slot != "off" {
		return "specify main or off"
	}

	// Locate the item in the backpack.
	instances := sess.Backpack.FindByItemDefID(itemDefID)
	if len(instances) == 0 {
		return fmt.Sprintf("%s: not found in your pack", itemDefID)
	}
	inst := instances[0]

	// Resolve the ItemDef to get the WeaponRef.
	itemDef, ok := reg.Item(itemDefID)
	if !ok {
		return fmt.Sprintf("%s: not found in your pack", itemDefID)
	}
	if itemDef.Kind != inventory.KindWeapon {
		return fmt.Sprintf("%s is not a weapon", itemDef.Name)
	}

	// Resolve the WeaponDef.
	weaponDef := reg.Weapon(itemDef.WeaponRef)
	if weaponDef == nil {
		return fmt.Sprintf("%s: weapon definition not found", itemDef.Name)
	}

	// Attempt to equip in the requested slot.
	preset := sess.LoadoutSet.ActivePreset()
	var equipErr error
	var slotLabel string
	switch slot {
	case "main":
		equipErr = preset.EquipMainHand(weaponDef)
		slotLabel = "main"
	case "off":
		equipErr = preset.EquipOffHand(weaponDef)
		slotLabel = "off"
	}
	if equipErr != nil {
		return equipErr.Error()
	}

	// Remove from backpack only after a successful equip.
	if err := sess.Backpack.Remove(inst.InstanceID, 1); err != nil {
		// Revert the equip to maintain consistency.
		switch slot {
		case "main":
			preset.UnequipMainHand()
		case "off":
			preset.UnequipOffHand()
		}
		return fmt.Sprintf("failed to remove item from pack: %v", err)
	}

	return fmt.Sprintf("Equipped %s in %s hand.", weaponDef.Name, slotLabel)
}
