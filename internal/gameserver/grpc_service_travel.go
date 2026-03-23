package gameserver

import (
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleTravel fast-travels the player to the named zone's start room.
//
// Preconditions (checked in order):
//  1. req.ZoneId must exist in the world model.
//  2. Player must have discovered the target zone (AutomapCache entry non-empty).
//  3. Player must not be in combat.
//  4. Player must have no non-zero WantedLevel entries.
//  5. Player must not already be in the target zone.
//
// Postcondition: On success, sess.RoomID is set to targetZone.StartRoom and a
// confirmation MessageEvent is returned.
func (s *GameServiceServer) handleTravel(uid string, req *gamev1.TravelRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Precondition 1: zone must exist.
	targetZone, ok := s.world.GetZone(req.GetZoneId())
	if !ok {
		return messageEvent("That zone does not exist."), nil
	}

	// Precondition 2: zone must be discovered.
	if len(sess.AutomapCache[req.GetZoneId()]) == 0 {
		return messageEvent("You don't know how to get there."), nil
	}

	// Precondition 3: no active combat.
	if sess.Status == statusInCombat {
		return messageEvent("You can't travel while in combat."), nil
	}

	// Precondition 4: no wanted levels.
	for _, level := range sess.WantedLevel {
		if level != 0 {
			return messageEvent("You can't travel while Wanted."), nil
		}
	}

	// Precondition 5: not already in target zone.
	currentRoom, ok := s.world.GetRoom(sess.RoomID)
	if ok && currentRoom.ZoneID == req.GetZoneId() {
		return messageEvent("You're already there."), nil
	}

	// Relocate to target zone's start room.
	startRoom, ok := s.world.GetRoom(targetZone.StartRoom)
	if !ok {
		return messageEvent("That zone is unreachable right now."), nil
	}
	sess.RoomID = startRoom.ID

	// Build confirmation message.
	travelMsg := fmt.Sprintf("You make your way to %s.", targetZone.Name)
	return messageEvent(travelMsg), nil
}
