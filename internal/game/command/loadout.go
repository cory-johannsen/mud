package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleLoadout handles the "loadout" command.
// With no argument it displays all presets with an [active] marker on the active one.
// With a 1-based integer argument it attempts to swap to that preset.
//
// Precondition: sess must not be nil; sess.LoadoutSet must not be nil.
// Postcondition: Returns a string message describing the result of the command.
func HandleLoadout(sess *session.PlayerSession, arg string) string {
	ls := sess.LoadoutSet

	arg = strings.TrimSpace(arg)
	if arg == "" {
		return renderLoadoutSet(ls)
	}

	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(ls.Presets) {
		return fmt.Sprintf("Invalid preset: %q. Use a number between 1 and %d.", arg, len(ls.Presets))
	}

	idx := n - 1 // convert 1-based to 0-based

	if err := ls.Swap(idx); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already") {
			return "You have already swapped loadouts this round."
		}
		return fmt.Sprintf("Invalid preset index: %s", msg)
	}

	if ls.Active != idx {
		// Swap was a no-op (same preset selected); SwappedThisRound was not set.
		return fmt.Sprintf("Preset %d is already active.", n)
	}

	return fmt.Sprintf("Switched to preset %d.", n)
}

// renderLoadoutSet formats all presets for display, marking the active one.
//
// Precondition: ls must not be nil.
// Postcondition: Returns a multi-line string with one section per preset.
func renderLoadoutSet(ls *inventory.LoadoutSet) string {
	var sb strings.Builder
	for i, preset := range ls.Presets {
		label := fmt.Sprintf("Preset %d", i+1)
		if i == ls.Active {
			label += " [active]"
		}
		sb.WriteString(label + ":\n")
		sb.WriteString("  Main: " + formatEquippedWeapon(preset.MainHand) + "\n")
		sb.WriteString("  Off:  " + formatEquippedWeapon(preset.OffHand) + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// formatEquippedWeapon returns a human-readable description of an equipped weapon slot.
//
// Precondition: ew may be nil (represents an empty slot).
// Postcondition: Returns "empty" when ew is nil, otherwise the weapon name with optional ammo.
func formatEquippedWeapon(ew *inventory.EquippedWeapon) string {
	if ew == nil {
		return "empty"
	}
	if ew.Magazine != nil {
		return fmt.Sprintf("%s [%d/%d]", ew.Def.Name, ew.Magazine.Loaded, ew.Magazine.Capacity)
	}
	return ew.Def.Name
}
