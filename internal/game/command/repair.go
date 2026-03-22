package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// RepairSession provides the player-session view needed by HandleRepair.
// It wraps *session.PlayerSession so callers do not need a separate type in tests.
//
// Precondition: Session must not be nil.
type RepairSession struct {
	Session *session.PlayerSession
}

// repairTarget is a matched item to repair: either a weapon slot or an armor slot.
type repairTarget struct {
	// weapon is set when the item is in the active weapon loadout.
	weapon *inventory.EquippedWeapon
	// armorSlot / armorItem are set when the item is an equipped armor piece.
	armorSlot inventory.ArmorSlot
	armorItem *inventory.SlottedItem
	// name is the display name for user messages.
	name string
	// maxDurability is the maximum durability for the item's rarity.
	maxDurability int
}

// findRepairTarget searches the active weapon loadout and all armor slots for an
// item whose Def ID, SlottedItem.ItemDefID, or name matches query (case-insensitive).
//
// Precondition: sess must not be nil.
// Postcondition: Returns a non-nil *repairTarget if found, nil otherwise.
func findRepairTarget(sess *session.PlayerSession, query string) *repairTarget {
	preset := sess.LoadoutSet.ActivePreset()
	if preset != nil {
		for _, ew := range []*inventory.EquippedWeapon{preset.MainHand, preset.OffHand} {
			if ew == nil || ew.Def == nil {
				continue
			}
			if ew.Def.ID == query || strings.EqualFold(ew.Def.Name, query) {
				maxDur := 0
				if rd, ok := inventory.LookupRarity(ew.Def.Rarity); ok {
					maxDur = rd.MaxDurability
				}
				return &repairTarget{weapon: ew, name: ew.Def.Name, maxDurability: maxDur}
			}
		}
	}
	if sess.Equipment != nil {
		for slot, si := range sess.Equipment.Armor {
			if si == nil {
				continue
			}
			if si.ItemDefID == query || strings.EqualFold(si.Name, query) {
				maxDur := 0
				if rd, ok := inventory.LookupRarity(si.Rarity); ok {
					maxDur = rd.MaxDurability
				}
				return &repairTarget{armorSlot: slot, armorItem: si, name: si.Name, maxDurability: maxDur}
			}
		}
	}
	return nil
}

// HandleRepair processes the "repair <item>" command (REQ-EM-13/14/15).
//
// Precondition: rs, reg, rng must be non-nil; query must be a non-empty string.
// Postcondition: On success, consumes one repair_kit, restores 1d6 durability,
// and returns a confirmation. On failure, returns a descriptive error string and
// state is unchanged.
func HandleRepair(rs *RepairSession, reg *inventory.Registry, query string, rng inventory.Roller) string {
	sess := rs.Session

	// REQ-EM-13: require a repair_kit in the backpack.
	kits := sess.Backpack.FindByItemDefID("repair_kit")
	if len(kits) == 0 {
		return "You need a repair kit to field-repair equipment."
	}

	// Find the target item.
	target := findRepairTarget(sess, query)
	if target == nil {
		return fmt.Sprintf("Item %q not found in your equipped gear.", query)
	}

	// Determine current and max durability.
	var curDur int
	if target.weapon != nil {
		curDur = target.weapon.Durability
	} else {
		curDur = target.armorItem.Durability
	}

	if curDur >= target.maxDurability && target.maxDurability > 0 {
		return fmt.Sprintf("%s is already at full durability.", target.name)
	}

	// REQ-EM-13: consume the repair_kit before calling RepairField.
	if err := sess.Backpack.Remove(kits[0].InstanceID, 1); err != nil {
		return fmt.Sprintf("Could not consume repair kit: %v", err)
	}

	// Build a temporary ItemInstance for RepairField.
	inst := &inventory.ItemInstance{
		Durability:    curDur,
		MaxDurability: target.maxDurability,
	}
	restored := inventory.RepairField(inst, rng)

	// Write the restored durability back to the equipped item.
	if target.weapon != nil {
		target.weapon.Durability = inst.Durability
	} else {
		target.armorItem.Durability = inst.Durability
	}

	return fmt.Sprintf("Repaired %s: restored %d durability (now %d/%d).",
		target.name, restored, inst.Durability, target.maxDurability)
}
