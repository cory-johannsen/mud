package gameserver

import (
	"fmt"
	"strings"
	"time"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/trap"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
)

// handleDeployTrap processes a deploy_trap command.
//
// Precondition: uid refers to an existing player session; req.ItemName is non-empty.
// Postcondition: on success, 1 item is removed from backpack and a consumable trap is armed in the player's room.
func (s *GameServiceServer) handleDeployTrap(uid string, req *gamev1.DeployTrapRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}

	// AP check first: in-combat deploys cost 1 AP.
	inCombat := sess.Status == statusInCombat
	if inCombat {
		if s.combatH.RemainingAP(uid) < 1 {
			return messageEvent("Not enough AP to deploy a trap."), nil
		}
		if err := s.combatH.SpendAP(uid, 1); err != nil {
			return messageEvent("Not enough AP to deploy a trap."), nil
		}
	}

	// Find item by name (case-insensitive).
	var foundInstanceID string
	var foundDef *inventory.ItemDef
	for _, inst := range sess.Backpack.Items() {
		def, defOk := s.invRegistry.Item(inst.ItemDefID)
		if !defOk {
			continue
		}
		if strings.EqualFold(def.Name, req.ItemName) {
			foundInstanceID = inst.InstanceID
			foundDef = def
			break
		}
	}
	if foundInstanceID == "" {
		return messageEvent(fmt.Sprintf("You don't have a %s.", req.ItemName)), nil
	}
	if foundDef.Kind != inventory.KindTrap {
		return messageEvent(fmt.Sprintf("You can't deploy that.")), nil
	}

	tmpl, ok := s.trapTemplates[foundDef.TrapTemplateRef]
	if !ok {
		s.logger.Error("missing trap template for deployable item",
			zap.String("item_id", foundDef.ID),
			zap.String("trap_template_ref", foundDef.TrapTemplateRef),
		)
		return messageEvent("That trap is broken — contact an admin."), nil
	}

	// Remove 1 from backpack.
	if err := sess.Backpack.Remove(foundInstanceID, 1); err != nil {
		return messageEvent(fmt.Sprintf("Failed to remove %s from inventory.", req.ItemName)), nil
	}

	// Determine deploy position.
	deployPos := 0
	if inCombat {
		deployPos = s.combatH.CombatantPosition(sess.RoomID, uid)
	}

	// Resolve zone ID for the instance ID.
	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return messageEvent("You are not in a valid room."), nil
	}
	zone, ok := s.world.GetZone(room.ZoneID)
	if !ok {
		return messageEvent("You are not in a valid zone."), nil
	}

	instanceID := trap.TrapInstanceID(
		zone.ID, sess.RoomID, trap.TrapKindConsumable,
		fmt.Sprintf("%d", time.Now().UnixNano()),
	)
	if err := s.trapMgr.AddConsumableTrap(instanceID, tmpl, deployPos); err != nil {
		s.logger.Error("failed to add consumable trap", zap.Error(err))
		return messageEvent("Failed to arm trap."), nil
	}

	if inCombat {
		return messageEvent(fmt.Sprintf("You arm a %s at your position.", foundDef.Name)), nil
	}
	return messageEvent(fmt.Sprintf("You arm a %s here.", foundDef.Name)), nil
}
