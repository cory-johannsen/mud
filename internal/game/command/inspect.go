package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleInspect processes the "inspect" command.
// arg is an item def ID (e.g. "tactical_knife") from the player's backpack or equipment.
//
// Precondition: sess must not be nil; reg must not be nil.
// Postcondition: Returns a one-line description of the item's name, description, and
// weapon stats when applicable.
func HandleInspect(sess *session.PlayerSession, reg *inventory.Registry, arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "Usage: inspect <item_id>"
	}

	itemDef, ok := reg.Item(arg)
	if !ok {
		return fmt.Sprintf("inspect: %q not found in item registry", arg)
	}

	base := fmt.Sprintf("%s: %s", itemDef.Name, itemDef.Description)

	if itemDef.Kind == inventory.KindWeapon && itemDef.WeaponRef != "" {
		if weaponDef := reg.Weapon(itemDef.WeaponRef); weaponDef != nil {
			base += fmt.Sprintf(" [weapon, damage: %s %s]", weaponDef.DamageDice, weaponDef.DamageType)
		}
	}

	return base
}
