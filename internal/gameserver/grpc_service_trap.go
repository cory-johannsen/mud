package gameserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// pushMessage marshals a text message and pushes it to the player's entity channel.
// Silently drops if entity is nil, marshalling fails, or the channel is full.
func pushMessage(sess *session.PlayerSession, msg string) {
	if sess.Entity == nil {
		return
	}
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: msg,
			},
		},
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		return
	}
	_ = sess.Entity.Push(data)
}

// checkEntryTraps fires all armed entry-trigger traps in room for the given player,
// and performs Search-mode trap detection (REQ-TR-3/4/5/6/7).
//
// Precondition: uid and sess must be non-nil; room must be the room the player just entered.
// Postcondition: Fires all matching armed traps; marks detected traps in TrapManager.
func (s *GameServiceServer) checkEntryTraps(uid string, sess *session.PlayerSession, room *world.Room) {
	if s.trapMgr == nil || s.trapTemplates == nil {
		return
	}
	zone, zoneOK := s.world.GetZone(room.ZoneID)
	if !zoneOK {
		return
	}
	dangerLevel := room.DangerLevel
	if dangerLevel == "" {
		dangerLevel = zone.DangerLevel
	}

	// Enumerate ALL trap instances for this room (both room-level and equipment-level)
	// via TrapsForRoom, which iterates the TrapManager's internal map by prefix.
	allInstanceIDs := s.trapMgr.TrapsForRoom(zone.ID, room.ID)

	// Determine whether this trap applies to the current player.
	// TriggerRegion (honkeypot): only fire if the player's home region is targeted.
	// TriggerEntry: always fire on room entry.

	// Fire armed entry-trigger and region-trigger (Honkeypot) traps.
	for _, instanceID := range allInstanceIDs {
		state, ok := s.trapMgr.GetTrap(instanceID)
		if !ok || !state.Armed {
			continue
		}
		tmpl, ok := s.trapTemplates[state.TemplateID]
		if !ok {
			continue
		}
		switch tmpl.Trigger {
		case trap.TriggerEntry:
			// Always fire on room entry.
		case trap.TriggerRegion:
			// REQ-TR-1: Only fire Honkeypot if TriggerAction=="entry" and player region is targeted.
			if tmpl.TriggerAction != "entry" || !isTrapTargeted(sess.Region, tmpl.TargetRegions) {
				continue
			}
		default:
			continue
		}
		s.fireTrap(uid, sess, tmpl, instanceID, dangerLevel, false)
	}

	// Search mode detection (REQ-TR-3/4/5/6/7).
	if sess.ExploreMode == session.ExploreModeCaseIt {
		s.detectTrapsInRoom(uid, sess, room, zone.ID, dangerLevel)
	}
}

// fireTrap applies a trap's payload to the given player (and optionally all room occupants for AoE).
// If disarmFailure is true, AoE traps target ONLY the disarming player — not all room occupants (REQ-TR-13).
//
// Precondition: tmpl and instanceID must reference a valid armed trap.
// Postcondition: Trap state is updated (disarmed); affected players receive damage/conditions.
func (s *GameServiceServer) fireTrap(uid string, sess *session.PlayerSession, tmpl *trap.TrapTemplate, instanceID, dangerLevel string, disarmFailure bool) {
	result, err := trap.ResolveTrigger(tmpl, dangerLevel, s.trapTemplates)
	if err != nil {
		s.logger.Warn("fireTrap: ResolveTrigger failed", zap.String("template", tmpl.ID), zap.Error(err))
		return
	}

	// applyToPlayer applies damage and conditions to a single player session and sends a message.
	applyToPlayer := func(target *session.PlayerSession, msg string) {
		if result.DamageDice != "" && s.dice != nil {
			rolled, rollErr := s.dice.RollExpr(result.DamageDice)
			if rollErr == nil {
				total := rolled.Total()
				for _, dt := range result.DamageTypes {
					resistance := target.Resistances[dt]
					dmg := total - resistance
					if dmg < 0 {
						dmg = 0
					}
					target.CurrentHP -= dmg
				}
				msg += fmt.Sprintf(" You take %s damage!", result.DamageDice)
			}
		}
		if result.ConditionID != "" {
			if target.Conditions != nil && s.condRegistry != nil {
				if condDef, condOK := s.condRegistry.Get(result.ConditionID); condOK {
					_ = target.Conditions.Apply(target.UID, condDef, 1, -1)
				}
			}
			msg += fmt.Sprintf(" You are now %s!", result.ConditionID)
		}
		pushMessage(target, msg)
	}

	// Apply to triggering player.
	applyToPlayer(sess, result.Narrative)
	s.checkNonCombatDeath(uid, sess)
	// REQ-AH-22: apply substance if trap payload has a substance_id.
	if result.SubstanceID != "" {
		_ = s.ApplySubstanceByID(uid, result.SubstanceID)
	}

	// If AoE and NOT a disarm failure, apply to all other players in the room (REQ-TR-13).
	if result.AoE && !disarmFailure {
		for _, other := range s.sessions.AllPlayers() {
			if other.UID == uid || other.RoomID != sess.RoomID {
				continue
			}
			applyToPlayer(other, fmt.Sprintf("A %s goes off nearby!", tmpl.Name))
			s.checkNonCombatDeath(other.UID, other)
			// REQ-AH-22: apply substance to AoE targets as well.
			if result.SubstanceID != "" {
				_ = s.ApplySubstanceByID(other.UID, result.SubstanceID)
			}
		}
	}

	// Update trap state: disarm after firing.
	s.trapMgr.Disarm(instanceID) // sets Armed=false; one_shot stays in map but disarmed.
}

// detectTrapsInRoom rolls secret Perception checks for the player against each armed trap.
// Uses TrapsForRoom to cover both room-level and equipment-level traps (REQ-TR-4).
// REQ-TR-3/4/5/6/7.
func (s *GameServiceServer) detectTrapsInRoom(uid string, sess *session.PlayerSession, room *world.Room, zoneID, dangerLevel string) {
	if s.trapMgr == nil {
		return
	}
	// Perception modifier: Savvy ability modifier.
	perceptionMod := combat.AbilityMod(sess.Abilities.Savvy)
	allInstanceIDs := s.trapMgr.TrapsForRoom(zoneID, room.ID)
	for _, instanceID := range allInstanceIDs {
		state, ok := s.trapMgr.GetTrap(instanceID)
		if !ok || !state.Armed {
			continue
		}
		tmpl, ok := s.trapTemplates[state.TemplateID]
		if !ok {
			continue
		}
		// REQ-TR-7: Exclude Honkeypots for non-targeted players.
		if tmpl.Trigger == trap.TriggerRegion {
			if !isTrapTargeted(sess.Region, tmpl.TargetRegions) {
				continue
			}
		}
		scaling := trap.ScalingFor(tmpl, dangerLevel)
		dc := tmpl.StealthDC + scaling.StealthDCBonus
		var roll int
		if s.dice != nil {
			rollResult, rollErr := s.dice.RollExpr("1d20")
			if rollErr == nil {
				roll = rollResult.Total()
			}
		}
		total := roll + perceptionMod
		if total >= dc {
			// REQ-TR-5: Mark detected, reveal name and location.
			s.trapMgr.MarkDetected(uid, instanceID)
			location := "the room"
			if strings.Contains(instanceID, "/equip/") {
				parts := strings.SplitN(instanceID, "/equip/", 2)
				if len(parts) == 2 {
					location = parts[1]
				}
			}
			pushMessage(sess, fmt.Sprintf("You notice a %s near %s!", tmpl.Name, location))
		}
		// REQ-TR-6: On failure, no message.
	}
}

// isTrapTargeted returns true if playerRegion is in targetRegions.
func isTrapTargeted(playerRegion string, targetRegions []string) bool {
	for _, r := range targetRegions {
		if r == playerRegion {
			return true
		}
	}
	return false
}

// fireConsumableTrapOnCombatant applies a consumable trap's payload to a single combat.Combatant.
// Handles both player and NPC targets. Does NOT call trapMgr.Disarm — caller is responsible.
//
// Precondition: target, tmpl must be non-nil; instanceID and dangerLevel must be non-empty.
// Postcondition: Damage applied and message sent; trap state unchanged (caller disarms).
func (s *GameServiceServer) fireConsumableTrapOnCombatant(
	target *combat.Combatant,
	tmpl *trap.TrapTemplate,
	instanceID, dangerLevel, roomID string,
) {
	result, err := trap.ResolveTrigger(tmpl, dangerLevel, s.trapTemplates)
	if err != nil {
		s.logger.Warn("consumable trap ResolveTrigger failed", zap.String("templateID", tmpl.ID), zap.Error(err))
		return
	}
	dmg := 0
	if result.DamageDice != "" && s.dice != nil {
		rolled, rollErr := s.dice.RollExpr(result.DamageDice)
		if rollErr == nil {
			dmg = rolled.Total()
		}
		if dmg < 0 {
			dmg = 0
		}
	}
	target.ApplyDamage(dmg)

	// Player target: apply condition + substance + send personal message.
	if sess, ok := s.sessions.GetPlayer(target.ID); ok {
		if result.ConditionID != "" && s.condRegistry != nil && sess.Conditions != nil {
			if condDef, condOk := s.condRegistry.Get(result.ConditionID); condOk {
				_ = sess.Conditions.Apply(target.ID, condDef, 1, -1)
			}
		}
		// REQ-AH-22: apply substance from consumable trap payload.
		if result.SubstanceID != "" {
			if err := s.ApplySubstanceByID(target.ID, result.SubstanceID); err != nil && s.logger != nil {
				s.logger.Warn("consumable trap ApplySubstanceByID failed", zap.Error(err))
			}
		}
		pushMessage(sess, fmt.Sprintf("A %s triggers on you! (%d damage)", tmpl.Name, dmg))
		return
	}

	// NPC target: broadcast to all players in the same room.
	for _, p := range s.sessions.AllPlayers() {
		if p.RoomID == roomID {
			pushMessage(p, fmt.Sprintf("A %s catches %s! (%d damage)", tmpl.Name, target.Name, dmg))
		}
	}
}

// checkConsumableTraps checks all armed consumable traps in the room against the moving combatant.
// Returns early if the combatant is not found in active combat.
//
// Precondition: s.trapMgr and s.trapTemplates must be non-nil (checked internally).
// Postcondition: All consumable traps within trigger range fire against the mover (or blast targets); disarmed after firing.
func (s *GameServiceServer) checkConsumableTraps(roomID, movedCombatantID string) {
	if s.trapMgr == nil || s.trapTemplates == nil {
		return
	}
	room, ok := s.world.GetRoom(roomID)
	if !ok {
		return
	}
	zone, ok := s.world.GetZone(room.ZoneID)
	if !ok {
		return
	}
	dangerLevel := room.DangerLevel
	if dangerLevel == "" {
		dangerLevel = zone.DangerLevel
	}

	// Find the moving combatant. If not in active combat, return early.
	if s.combatH == nil {
		return
	}
	combatants := s.combatH.CombatantsInRoom(roomID)
	var mover *combat.Combatant
	for _, c := range combatants {
		if c.ID == movedCombatantID {
			mover = c
			break
		}
	}
	if mover == nil {
		return
	}
	movedPos := mover.GridX * 5

	instanceIDs := s.trapMgr.TrapsForRoom(zone.ID, roomID)
	for _, instanceID := range instanceIDs {
		inst, ok := s.trapMgr.GetTrap(instanceID)
		if !ok || !inst.Armed || !inst.IsConsumable {
			continue
		}
		tmpl, ok := s.trapTemplates[inst.TemplateID]
		if !ok {
			continue
		}

		dist := movedPos - inst.DeployPosition
		if dist < 0 {
			dist = -dist
		}
		if dist > trap.EffectiveTriggerRange(tmpl) {
			continue
		}

		// Trap fires. Multiple overlapping traps all fire independently.
		if tmpl.BlastRadiusFt == 0 {
			s.fireConsumableTrapOnCombatant(mover, tmpl, instanceID, dangerLevel, roomID)
		} else {
			for _, c := range combatants {
				d := c.GridX*5 - inst.DeployPosition
				if d < 0 {
					d = -d
				}
				if d <= tmpl.BlastRadiusFt {
					s.fireConsumableTrapOnCombatant(c, tmpl, instanceID, dangerLevel, roomID)
				}
			}
		}
		s.trapMgr.Disarm(instanceID) // always one-shot (REQ-CTR-10)
	}
}

// checkEnemyEntersReadyTrigger fires the TriggerOnEnemyEntersRoom fire point for all players
// in the same room who have ReadiedTrigger == "enemy_enters" and whose mover is an NPC (enemy).
//
// Precondition: movedCombatantID must be non-empty; s.combatH and s.sessions must be non-nil.
// Postcondition: ReactionFn is invoked for each player with a matching readied trigger; no-op if mover is a player.
func (s *GameServiceServer) checkEnemyEntersReadyTrigger(roomID, movedCombatantID string) {
	if s.combatH == nil {
		return
	}
	combatants := s.combatH.CombatantsInRoom(roomID)
	if len(combatants) == 0 {
		return
	}

	// Confirm the mover is an NPC (enemy). If the mover is a player, no trigger fires.
	moverIsEnemy := false
	for _, c := range combatants {
		if c.ID == movedCombatantID && c.Kind == combat.KindNPC {
			moverIsEnemy = true
			break
		}
	}
	if !moverIsEnemy {
		return
	}

	// Fire the trigger for every player session in the room with a matching readied trigger.
	rctx := reaction.ReactionContext{}
	for _, p := range s.sessions.AllPlayers() {
		if p.RoomID != roomID || p.ReadiedTrigger != "enemy_enters" || p.ReactionFn == nil {
			continue
		}
		// TODO(#244 Task 9): thread the resolver's ctx and filtered candidates instead of Background/nil.
		_, _, _ = p.ReactionFn(context.Background(), p.UID, reaction.TriggerOnEnemyEntersRoom, rctx, nil)
	}
}

// WireConsumableTrapTrigger connects the combat movement callback to consumable trap checking
// and to the enemy_enters ready-action fire point (REQ-RA-7).
//
// Precondition: s.combatH must be non-nil; call after NewGameServiceServer.
// Postcondition: CombatHandler will invoke checkConsumableTraps and checkEnemyEntersReadyTrigger
//
//	on every combatant movement event.
func (s *GameServiceServer) WireConsumableTrapTrigger() {
	if s.combatH == nil {
		return
	}
	s.combatH.SetOnCombatantMoved(func(roomID, movedCombatantID string) {
		s.checkConsumableTraps(roomID, movedCombatantID)
		s.checkEnemyEntersReadyTrigger(roomID, movedCombatantID)
	})
}

// checkPressurePlateTraps fires all armed pressure_plate traps in room for the given player.
// Only called during combat (caller's responsibility to check).
//
// Precondition: s.trapMgr and s.trapTemplates must be non-nil (caller checks).
// Postcondition: All armed pressure_plate traps in room are fired against player.
func (s *GameServiceServer) checkPressurePlateTraps(uid string, sess *session.PlayerSession, room *world.Room) {
	if s.trapMgr == nil || s.trapTemplates == nil {
		return
	}
	// Only fire during combat — callers must ensure this, but guard defensively.
	if sess.Status != statusInCombat {
		return
	}
	zone, ok := s.world.GetZone(room.ZoneID)
	if !ok {
		return
	}
	dangerLevel := room.DangerLevel
	if dangerLevel == "" {
		dangerLevel = zone.DangerLevel
	}
	for i := range room.Equipment {
		eq := &room.Equipment[i]
		if eq.TrapTemplate == "" {
			continue
		}
		tmpl, ok := s.trapTemplates[eq.TrapTemplate]
		if !ok || tmpl.Trigger != trap.TriggerPressurePlate {
			continue
		}
		instanceID := trap.TrapInstanceID(zone.ID, room.ID, "equip", eq.Description)
		state, ok := s.trapMgr.GetTrap(instanceID)
		if !ok || !state.Armed {
			continue
		}
		s.fireTrap(uid, sess, tmpl, instanceID, dangerLevel, false)
	}
}

// WireCoverCrossfireTrap registers a callback on the CombatHandler that fires armed
// traps on cover equipment when an attack misses due to cover (crossfire mechanic).
//
// Precondition: s.combatH must be non-nil; call after NewGameServiceServer.
// Postcondition: CombatHandler will fire an armed trap on cover equipment when
//   an attack misses due to cover and the cover item has TrapTemplate set.
func (s *GameServiceServer) WireCoverCrossfireTrap() {
	if s.combatH == nil {
		return
	}
	s.combatH.SetOnCoverHit(func(roomID, attackerID, coverEquipID string) {
		if s.trapMgr == nil || s.trapTemplates == nil {
			return
		}
		// Only fire for player attackers (NPCs can't have PlayerSession).
		attackerSess, ok := s.sessions.GetPlayer(attackerID)
		if !ok {
			return
		}
		room, ok := s.world.GetRoom(roomID)
		if !ok {
			return
		}
		zone, ok := s.world.GetZone(room.ZoneID)
		if !ok {
			return
		}
		dangerLevel := room.DangerLevel
		if dangerLevel == "" {
			dangerLevel = zone.DangerLevel
		}
		for i := range room.Equipment {
			eq := &room.Equipment[i]
			if eq.ItemID != coverEquipID || eq.TrapTemplate == "" {
				continue
			}
			tmpl, ok := s.trapTemplates[eq.TrapTemplate]
			if !ok {
				continue
			}
			instanceID := trap.TrapInstanceID(zone.ID, roomID, "equip", eq.Description)
			state, ok := s.trapMgr.GetTrap(instanceID)
			if !ok || !state.Armed {
				continue
			}
			// Cover crossfire: fire against the attacker only (not AoE).
			s.fireTrap(attackerID, attackerSess, tmpl, instanceID, dangerLevel, false)
			break
		}
	})
}

// checkInteractionTrap fires an armed interaction-trigger trap on the given equipment item.
//
// Precondition: s.trapMgr and s.trapTemplates must be non-nil; room and uid must be valid.
// Postcondition: Fires the trap if armed and triggered by interaction.
func (s *GameServiceServer) checkInteractionTrap(uid string, sess *session.PlayerSession, room *world.Room, equipItemID string) {
	if s.trapMgr == nil || s.trapTemplates == nil {
		return
	}
	zone, ok := s.world.GetZone(room.ZoneID)
	if !ok {
		return
	}
	dangerLevel := room.DangerLevel
	if dangerLevel == "" {
		dangerLevel = zone.DangerLevel
	}
	for i := range room.Equipment {
		eq := &room.Equipment[i]
		if eq.ItemID != equipItemID || eq.TrapTemplate == "" {
			continue
		}
		tmpl, ok := s.trapTemplates[eq.TrapTemplate]
		if !ok || tmpl.Trigger != trap.TriggerInteraction {
			continue
		}
		instanceID := trap.TrapInstanceID(zone.ID, room.ID, "equip", eq.Description)
		state, stateOk := s.trapMgr.GetTrap(instanceID)
		if !stateOk || !state.Armed {
			continue
		}
		s.fireTrap(uid, sess, tmpl, instanceID, dangerLevel, false)
		break
	}
}

// handleDisarmTrap processes a disarm_trap command.
//
// Precondition: uid must identify a connected player; req.TrapName must be non-empty.
// Postcondition: Applies Thievery check vs disable_dc + scaling; on success disarms the trap;
//   on failure by 5+, fires trap against disarming player only (REQ-TR-13).
func (s *GameServiceServer) handleDisarmTrap(uid string, req *gamev1.DisarmTrapRequest) (*gamev1.ServerEvent, error) {
	if s.trapMgr == nil {
		return messageEvent("Traps are not active in this world."), nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("room %q not found", sess.RoomID)
	}
	zone, ok := s.world.GetZone(room.ZoneID)
	if !ok {
		return nil, fmt.Errorf("zone for room %q not found", sess.RoomID)
	}
	dangerLevel := room.DangerLevel
	if dangerLevel == "" {
		dangerLevel = zone.DangerLevel
	}

	instanceID, tmpl := s.findDetectedTrap(uid, sess, room, zone.ID, req.TrapName)
	if instanceID == "" {
		return messageEvent(fmt.Sprintf("You don't see any trap called %q here.", req.TrapName)), nil
	}

	// Thievery skill check.
	scaling := trap.ScalingFor(tmpl, dangerLevel)
	dc := tmpl.DisableDC + scaling.DisableDCBonus
	abilityScore := s.abilityScoreForSkill(sess, "thievery")
	amod := abilityModFrom(abilityScore)
	roll := 10 // fallback when no dice configured
	if s.dice != nil {
		result, rollErr := s.dice.RollExpr("1d20")
		if rollErr == nil {
			roll = result.Total()
		}
	}
	total := roll + amod

	if total >= dc {
		s.trapMgr.Disarm(instanceID)
		return messageEvent(fmt.Sprintf("You successfully disarm the %s.", tmpl.Name)), nil
	}
	if total <= dc-5 {
		// Failure by 5+: fire trap against disarming player only (REQ-TR-13).
		s.fireTrap(uid, sess, tmpl, instanceID, dangerLevel, true)
		return messageEvent(fmt.Sprintf("You fumble the disarm attempt — the %s fires!", tmpl.Name)), nil
	}
	return messageEvent(fmt.Sprintf("You fail to disarm the %s. You may try again.", tmpl.Name)), nil
}

// findDetectedTrap searches the current room for a detected trap matching name.
// Returns (instanceID, template) or ("", nil) if not found.
//
// Precondition: sess.RoomID must be valid; uid must have previously detected the trap.
// Postcondition: Returns the instance ID and template of the matching detected trap, or ("", nil).
func (s *GameServiceServer) findDetectedTrap(uid string, sess *session.PlayerSession, room *world.Room, zoneID, trapName string) (string, *trap.TrapTemplate) {
	lowerName := strings.ToLower(trapName)
	for i := range room.Equipment {
		eq := &room.Equipment[i]
		if eq.TrapTemplate == "" {
			continue
		}
		tmpl, ok := s.trapTemplates[eq.TrapTemplate]
		if !ok {
			continue
		}
		instanceID := trap.TrapInstanceID(zoneID, room.ID, "equip", eq.Description)
		if !s.trapMgr.IsDetected(uid, instanceID) {
			continue
		}
		candidate := strings.ToLower(tmpl.Name)
		withLocation := candidate + " near " + strings.ToLower(eq.Description)
		if lowerName == candidate || lowerName == withLocation {
			return instanceID, tmpl
		}
	}
	// Also search room-level traps (position="room").
	for _, rtc := range room.Traps {
		if rtc.Position != "room" {
			continue
		}
		tmpl, ok := s.trapTemplates[rtc.TemplateID]
		if !ok {
			continue
		}
		instanceID := trap.TrapInstanceID(zoneID, room.ID, "room", rtc.TemplateID)
		if !s.trapMgr.IsDetected(uid, instanceID) {
			continue
		}
		candidate := strings.ToLower(tmpl.Name)
		if lowerName == candidate {
			return instanceID, tmpl
		}
	}
	// Also search consumable traps armed via TrapManager (REQ-CTR-12).
	for _, instanceID := range s.trapMgr.TrapsForRoom(zoneID, room.ID) {
		inst, ok := s.trapMgr.GetTrap(instanceID)
		// IsConsumable guard also prevents re-matching world/equipment traps
		// that happen to be registered in the TrapManager (those have IsConsumable=false).
		if !ok || !inst.Armed || !inst.IsConsumable {
			continue
		}
		if !s.trapMgr.IsDetected(uid, instanceID) {
			continue
		}
		tmpl, ok := s.trapTemplates[inst.TemplateID]
		if !ok {
			continue
		}
		candidate := strings.ToLower(tmpl.Name)
		if lowerName == candidate {
			return instanceID, tmpl
		}
	}
	return "", nil
}
