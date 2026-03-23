package gameserver

import (
	"context"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// handleTravel fast-travels the player to the StartRoom of the target zone.
//
// Preconditions are checked in order (REQ-WM-17..21):
//  1. req.ZoneId must exist in the world model.
//  2. Player must have discovered the target zone (AutomapCache entry non-empty).
//  3. Player must not be in combat.
//  4. Player must have no non-zero WantedLevel entries.
//  5. Player must not already be in the target zone.
//
// Postcondition: On success, the travel message is pushed to the player's entity channel
// (REQ-WM-23), all normal room-entry hooks fire (REQ-WM-24), and a RoomView ServerEvent
// is returned. On failure, a MessageEvent is returned with an appropriate error string.
func (s *GameServiceServer) handleTravel(uid string, req *gamev1.TravelRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// REQ-WM-17: zone must exist (first check).
	targetZone, ok := s.world.GetZone(req.GetZoneId())
	if !ok {
		return messageEvent("That zone does not exist."), nil
	}

	// REQ-WM-18: zone must be discovered.
	if len(sess.AutomapCache[req.GetZoneId()]) == 0 {
		return messageEvent("You don't know how to get there."), nil
	}

	// REQ-WM-19: must not be in combat.
	if sess.Status == statusInCombat {
		return messageEvent("You can't travel while in combat."), nil
	}

	// REQ-WM-20: must not be Wanted.
	for _, level := range sess.WantedLevel {
		if level != 0 {
			return messageEvent("You can't travel while Wanted."), nil
		}
	}

	// REQ-WM-21: must not already be in target zone.
	if currentRoom, ok := s.world.GetRoom(sess.RoomID); ok {
		if currentRoom.ZoneID == targetZone.ID {
			return messageEvent("You're already there."), nil
		}
	}

	// REQ-WM-22: relocate to targetZone.StartRoom.
	destRoom, ok := s.world.GetRoom(targetZone.StartRoom)
	if !ok {
		return messageEvent("That zone is unreachable right now."), nil
	}

	oldRoomID, err := s.sessions.MovePlayer(uid, destRoom.ID)
	if err != nil {
		return nil, fmt.Errorf("handleTravel MovePlayer: %w", err)
	}

	// REQ-WM-23: push travel message to player's entity channel.
	travelMsg := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: fmt.Sprintf("You make your way to %s.", targetZone.Name),
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if sess.Entity != nil {
		if data, marshalErr := proto.Marshal(travelMsg); marshalErr == nil {
			if pushErr := sess.Entity.Push(data); pushErr != nil {
				s.logger.Warn("pushing travel message", zap.String("uid", uid), zap.Error(pushErr))
			}
		}
	}

	// REQ-WM-24: fire all normal room-entry hooks.
	s.broadcastRoomEvent(oldRoomID, uid, &gamev1.RoomEvent{
		Player: sess.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})
	s.broadcastRoomEvent(destRoom.ID, uid, &gamev1.RoomEvent{
		Player: sess.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})
	s.triggerPassiveTechsForRoom(destRoom.ID)

	if s.scriptMgr != nil {
		if oldRoom, ok := s.world.GetRoom(oldRoomID); ok {
			s.scriptMgr.CallHook(oldRoom.ZoneID, "on_exit", //nolint:errcheck
				lua.LString(uid),
				lua.LString(oldRoomID),
				lua.LString(destRoom.ID),
			)
		}
		s.scriptMgr.CallHook(destRoom.ZoneID, "on_enter", //nolint:errcheck
			lua.LString(uid),
			lua.LString(destRoom.ID),
			lua.LString(oldRoomID),
		)
	}

	// Record automap discovery for the destination room.
	if sess.AutomapCache[destRoom.ZoneID] == nil {
		sess.AutomapCache[destRoom.ZoneID] = make(map[string]bool)
	}
	if !sess.AutomapCache[destRoom.ZoneID][destRoom.ID] {
		sess.AutomapCache[destRoom.ZoneID][destRoom.ID] = true
		if s.automapRepo != nil {
			if err := s.automapRepo.Insert(context.Background(), sess.CharacterID, destRoom.ZoneID, destRoom.ID); err != nil {
				s.logger.Warn("persisting travel room discovery", zap.Error(err))
			}
		}
	}

	// Apply room effects, skill checks, entry traps, wanted-level guard check.
	s.applyRoomEffectsOnEntry(sess, uid, destRoom, time.Now().Unix())
	if msgs := s.applyRoomSkillChecks(uid, destRoom); len(msgs) > 0 {
		for _, msg := range msgs {
			msgEvt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: msg},
				},
			}
			if data, marshalErr := proto.Marshal(msgEvt); marshalErr == nil && sess.Entity != nil {
				_ = sess.Entity.Push(data)
			}
		}
	}
	s.checkEntryTraps(uid, sess, destRoom)
	if !time.Now().Before(sess.DetentionGraceUntil) {
		wantedLevel := sess.WantedLevel[destRoom.ZoneID]
		if wantedLevel >= 2 && s.combatH != nil {
			s.combatH.InitiateGuardCombat(uid, destRoom.ZoneID, wantedLevel)
		}
	}
	s.notifyCombatJoinIfEligible(sess, destRoom.ID)
	s.clearNegotiateState(sess)

	// Build and return room view.
	if s.worldH == nil {
		// No world handler available (e.g. in unit tests); return the travel confirmation message.
		return travelMsg, nil
	}
	roomView := s.worldH.buildRoomView(uid, destRoom)
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: roomView},
	}, nil
}
