package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// validUnequipSlots lists all slot names accepted by HandleUnequip, in display order.
var validUnequipSlots = []string{
	"main", "off",
	"head", "torso", "left_arm", "right_arm", "left_leg", "right_leg", "feet",
	"neck",
	"ring_1", "ring_2", "ring_3", "ring_4", "ring_5",
	"ring_6", "ring_7", "ring_8", "ring_9", "ring_10",
}

// validUnequipSlotSet is the fast-lookup set derived from validUnequipSlots.
var validUnequipSlotSet = func() map[string]bool {
	m := make(map[string]bool, len(validUnequipSlots))
	for _, s := range validUnequipSlots {
		m[s] = true
	}
	return m
}()

// HandleUnequip processes the "unequip" command.
// arg must be one of the valid slot names.
//
// Precondition: sess must not be nil; sess.LoadoutSet and sess.Equipment must not be nil.
// Postcondition: On success, the named slot is cleared and a confirmation is returned.
// Unknown slots return an error listing all valid slot names.
func HandleUnequip(sess *session.PlayerSession, arg string) string {
	slot := strings.TrimSpace(arg)

	if !validUnequipSlotSet[slot] {
		return fmt.Sprintf(
			"Unknown slot %q. Valid slots: %s",
			slot,
			strings.Join(validUnequipSlots, ", "),
		)
	}

	preset := sess.LoadoutSet.ActivePreset()

	switch slot {
	case "main":
		if preset.MainHand == nil {
			return "Nothing equipped in main hand."
		}
		name := preset.MainHand.Def.Name
		preset.UnequipMainHand()
		return fmt.Sprintf("Unequipped %s from main hand.", name)

	case "off":
		if preset.OffHand == nil {
			return "Nothing equipped in off hand."
		}
		name := preset.OffHand.Def.Name
		preset.UnequipOffHand()
		return fmt.Sprintf("Unequipped %s from off hand.", name)

	default:
		// Armor and accessory slots: always empty until feature #4.
		return fmt.Sprintf("Nothing equipped in slot %s.", slot)
	}
}
