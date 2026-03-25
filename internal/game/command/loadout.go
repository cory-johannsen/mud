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
// Precondition: sess must not be nil; sess.LoadoutSet must not be nil; reg may be nil (disables affix display).
// Postcondition: Returns a string message describing the result of the command.
func HandleLoadout(sess *session.PlayerSession, arg string, reg *inventory.Registry) string {
	ls := sess.LoadoutSet

	arg = strings.TrimSpace(arg)
	if arg == "" {
		return renderLoadoutSet(ls, reg)
	}

	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(ls.Presets) {
		return fmt.Sprintf("Invalid preset: %q. Use a number between 1 and %d.", arg, len(ls.Presets))
	}

	idx := n - 1 // convert 1-based to 0-based

	// Capture active index before swap to detect no-op.
	wasActive := ls.Active
	if err := ls.Swap(idx); err != nil {
		if strings.Contains(err.Error(), "already swapped") {
			return "You have already swapped loadouts this round."
		}
		return fmt.Sprintf("Invalid preset %d: must be 1-%d.", n, len(ls.Presets))
	}
	if wasActive == idx {
		return fmt.Sprintf("Preset %d is already active.", n)
	}

	return fmt.Sprintf("Switched to preset %d.", n)
}

// renderLoadoutSet formats all presets for display, marking the active one.
//
// Precondition: ls must not be nil; reg may be nil (disables affix sub-list display).
// Postcondition: Returns a multi-line string with one section per preset.
func renderLoadoutSet(ls *inventory.LoadoutSet, reg *inventory.Registry) string {
	var sb strings.Builder
	for i, preset := range ls.Presets {
		label := fmt.Sprintf("Preset %d", i+1)
		if i == ls.Active {
			label += " [active]"
		}
		sb.WriteString(label + ":\n")
		sb.WriteString("  Main: " + formatEquippedWeaponWithAffixes(preset.MainHand, reg) + "\n")
		sb.WriteString("  Off:  " + formatEquippedWeaponWithAffixes(preset.OffHand, reg) + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// formatEquippedWeapon returns a human-readable description of an equipped weapon slot.
// Applies rarity color (REQ-EM-4) and modifier prefix (REQ-EM-18/19/20).
//
// Precondition: ew may be nil (represents an empty slot).
// Postcondition: Returns "empty" when ew is nil, otherwise the (colored, prefixed) weapon name with optional ammo.
func formatEquippedWeapon(ew *inventory.EquippedWeapon) string {
	return formatEquippedWeaponWithAffixes(ew, nil)
}

// formatEquippedWeaponWithAffixes returns a human-readable description of an equipped weapon slot,
// optionally showing upgrade slot count and affixed material sub-list when reg is non-nil.
//
// Precondition: ew may be nil (represents an empty slot); reg may be nil (disables affix display).
// Postcondition: Returns "empty" when ew is nil, otherwise the (colored, prefixed) weapon name
// with optional ammo, upgrade slot counter, and material sub-list.
func formatEquippedWeaponWithAffixes(ew *inventory.EquippedWeapon, reg *inventory.Registry) string {
	if ew == nil {
		return "empty"
	}
	displayName := ew.Def.Name
	// REQ-EM-18/19/20: prefix modifier.
	switch ew.Modifier {
	case "tuned":
		displayName = "Tuned " + displayName
	case "defective":
		displayName = "Defective " + displayName
	case "cursed":
		displayName = "Cursed " + displayName
	}
	// REQ-EM-4: apply rarity color.
	displayName = inventory.RarityColoredName(ew.Def.Rarity, displayName)

	var sb strings.Builder
	if ew.Magazine != nil {
		sb.WriteString(fmt.Sprintf("%s [%d/%d]", displayName, ew.Magazine.Loaded, ew.Magazine.Capacity))
	} else {
		sb.WriteString(displayName)
	}
	// Upgrade slot counter.
	if ew.Def.UpgradeSlots > 0 {
		sb.WriteString(fmt.Sprintf(" [%d/%d slots]", len(ew.AffixedMaterials), ew.Def.UpgradeSlots))
	}
	// Affixed material sub-list.
	if reg != nil {
		for _, entry := range ew.AffixedMaterials {
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) == 2 {
				if def, ok := reg.Material(parts[0], parts[1]); ok {
					sb.WriteString(fmt.Sprintf("\n    ↳ %s (%s)", def.Name, def.GradeName))
				}
			}
		}
	}
	return sb.String()
}
