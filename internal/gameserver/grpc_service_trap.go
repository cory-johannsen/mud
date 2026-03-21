package gameserver

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/combat"
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

	// Fire armed entry-trigger traps.
	for _, instanceID := range allInstanceIDs {
		state, ok := s.trapMgr.GetTrap(instanceID)
		if !ok || !state.Armed {
			continue
		}
		tmpl, ok := s.trapTemplates[state.TemplateID]
		if !ok || tmpl.Trigger != trap.TriggerEntry {
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

	// If AoE and NOT a disarm failure, apply to all other players in the room (REQ-TR-13).
	if result.AoE && !disarmFailure {
		for _, other := range s.sessions.AllPlayers() {
			if other.UID == uid || other.RoomID != sess.RoomID {
				continue
			}
			applyToPlayer(other, fmt.Sprintf("A %s goes off nearby!", tmpl.Name))
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
