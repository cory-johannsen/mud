package gameserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findChipDocInRoom looks up a chip_doc NPC by name in the given room.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on any failure.
func (s *GameServiceServer) findChipDocInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "chip_doc" {
		return nil, fmt.Sprintf("%s is not a chip doc.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// findCursedEquippedItem searches the equipment for a cursed item matching nameQuery.
//
// Precondition: equip is non-nil; nameQuery may be empty (matches any cursed item).
// Postcondition: Returns the slot key, slotted item, and whether it is an armor slot;
// returns ("", nil, false) if no match is found.
func findCursedEquippedItem(equip *inventory.Equipment, nameQuery string) (slotKey string, slotted *inventory.SlottedItem, isArmor bool) {
	for slot, item := range equip.Armor {
		if item == nil {
			continue
		}
		if item.Modifier != "cursed" {
			continue
		}
		if nameQuery == "" || strings.Contains(strings.ToLower(item.Name), strings.ToLower(nameQuery)) {
			return string(slot), item, true
		}
	}
	for slot, item := range equip.Accessories {
		if item == nil {
			continue
		}
		if item.Modifier != "cursed" {
			continue
		}
		if nameQuery == "" || strings.Contains(strings.ToLower(item.Name), strings.ToLower(nameQuery)) {
			return string(slot), item, false
		}
	}
	return "", nil, false
}

// handleUncurse processes an uncurse request from a player at a chip_doc NPC.
//
// Precondition: uid is a valid session UID; req is non-nil.
// Postcondition: Returns a server event describing the outcome; never returns a non-nil error.
func (s *GameServiceServer) handleUncurse(uid string, req *gamev1.UncurseRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	inst, errMsg := s.findChipDocInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}

	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}

	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.ChipDoc == nil {
		return messageEvent("This chip doc has no configuration."), nil
	}
	cfg := tmpl.ChipDoc

	if sess.Equipment == nil {
		return messageEvent("You have no equipped items."), nil
	}

	slotKey, slotted, isArmor := findCursedEquippedItem(sess.Equipment, req.GetItemName())
	if slotted == nil {
		return messageEvent(fmt.Sprintf("You have no cursed item named %q equipped.", req.GetItemName())), nil
	}

	if sess.Currency < cfg.RemovalCost {
		return messageEvent(fmt.Sprintf(
			"The chip doc requires %d credits for the removal procedure, but you only have %d credits.",
			cfg.RemovalCost, sess.Currency,
		)), nil
	}

	sess.Currency -= cfg.RemovalCost

	var roll int
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	} else {
		roll = 10
	}

	savvyMod := abilityModFrom(sess.Abilities.Savvy)

	rank := ""
	if sess.Skills != nil {
		rank = sess.Skills["rigging"]
	}

	total := roll + savvyMod + skillcheck.ProficiencyBonus(rank)
	outcome := skillcheck.OutcomeFor(total, cfg.CheckDC)

	switch outcome {
	case skillcheck.CritSuccess, skillcheck.Success:
		// Move item to backpack as defective; clear equipment slot.
		slotted.Modifier = "defective"

		if s.invRegistry != nil {
			itemDefID := slotted.ItemDefID
			if itemDef, ok2 := s.invRegistry.ItemByArmorRef(slotted.ItemDefID); ok2 {
				itemDefID = itemDef.ID
			}
			added, addErr := sess.Backpack.Add(itemDefID, 1, s.invRegistry)
			if addErr == nil && added != nil {
				added.Modifier = "defective"
			}
		} else {
			// No registry available; add item instance directly.
			added := sess.Backpack.AddInstance(inventory.ItemInstance{
				InstanceID: slotted.InstanceID,
				ItemDefID:  slotted.ItemDefID,
				Quantity:   1,
				Modifier:   "defective",
			})
			_ = added
		}

		if isArmor {
			sess.Equipment.Armor[inventory.ArmorSlot(slotKey)] = nil
		} else {
			sess.Equipment.Accessories[inventory.AccessorySlot(slotKey)] = nil
		}

		if s.charSaver != nil && sess.CharacterID > 0 {
			ctx := context.Background()
			_ = s.charSaver.SaveEquipment(ctx, sess.CharacterID, sess.Equipment)
			invItems := backpackToInventoryItems(sess.Backpack)
			_ = s.charSaver.SaveInventory(ctx, sess.CharacterID, invItems)
		}

		return messageEvent(fmt.Sprintf(
			"The chip doc carefully removes the cursed chip. The %s is now defective but no longer bound to you.",
			slotted.Name,
		)), nil

	case skillcheck.Failure:
		return messageEvent(fmt.Sprintf(
			"The chip doc's tools slip and the curse holds firm. You've lost %d credits and the %s remains cursed.",
			cfg.RemovalCost, slotted.Name,
		)), nil

	default: // CritFailure
		if s.condRegistry != nil {
			if def, ok2 := s.condRegistry.Get("fatigue"); ok2 {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
			}
		}
		return messageEvent(fmt.Sprintf(
			"The chip doc fails catastrophically! You are left staggered and fatigued. You've lost %d credits and the %s remains cursed.",
			cfg.RemovalCost, slotted.Name,
		)), nil
	}
}
