package gameserver

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// releaseDC returns the skill check DC for the release command based on the zone danger level.
//
// Precondition: dangerLevel is a string from the zone/room DangerLevel field.
// Postcondition: Returns a positive int DC.
// REQ-WC-16a: DC uses room's danger level at time of attempt.
func releaseDC(dangerLevel string) int {
	switch dangerLevel {
	case "safe":
		return 12
	case "sketchy":
		return 16
	case "dangerous":
		return 20
	case "all_out_war":
		return 24
	default:
		return 16
	}
}

// findDetainedTargetInRoom returns the *session.PlayerSession whose CharName
// matches targetNameLower (case-insensitive) among players in roomID, or nil.
//
// Precondition: s.sessions is non-nil; roomID and targetNameLower are non-empty.
// Postcondition: Returns the first matching session, or nil if not found.
func (s *GameServiceServer) findDetainedTargetInRoom(roomID, targetNameLower string) *session.PlayerSession {
	for _, p := range s.sessions.PlayersInRoomDetails(roomID) {
		if strings.ToLower(p.CharName) == targetNameLower {
			return p
		}
	}
	return nil
}

// handleRelease processes the release <player> command.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// REQ-WC-15: successful release does NOT modify the target's WantedLevel.
// REQ-WC-16: any player may attempt a release (no ally restriction).
// REQ-WC-16a: DC is derived from the releaser's current room danger level.
func (s *GameServiceServer) handleRelease(uid string, req *gamev1.ReleaseRequest) (*gamev1.ServerEvent, error) {
	// 1. Get releaser session.
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	targetName := strings.TrimSpace(req.GetPlayerName())
	if targetName == "" {
		return messageEvent("Usage: release <player>"), nil
	}

	// 2. Find target in the same room (case-insensitive).
	target := s.findDetainedTargetInRoom(sess.RoomID, strings.ToLower(targetName))
	if target == nil {
		return messageEvent(fmt.Sprintf("%s is not here.", targetName)), nil
	}

	// 3. Verify target has the detained condition.
	if target.Conditions == nil || !target.Conditions.Has("detained") {
		return messageEvent(fmt.Sprintf("%s is not detained.", targetName)), nil
	}

	// 4. Get zone danger level from the releaser's current room/zone.
	dangerLevel := ""
	if room, roomOK := s.world.GetRoom(sess.RoomID); roomOK {
		dangerLevel = room.DangerLevel
		if dangerLevel == "" {
			if zone, zoneOK := s.world.GetZone(room.ZoneID); zoneOK {
				dangerLevel = zone.DangerLevel
			}
		}
	}

	// 5. Compute DC (REQ-WC-16a).
	dc := releaseDC(dangerLevel)

	// 6. Skill check: use higher of Grift or Ghosting.
	griftBonus := skillRankBonus(sess.Skills["grift"])
	ghostingBonus := skillRankBonus(sess.Skills["ghosting"])
	bonus := griftBonus
	skillName := "grift"
	if ghostingBonus > griftBonus {
		bonus = ghostingBonus
		skillName = "ghosting"
	}

	// Roll 1d20 matching the pattern used by other out-of-combat skill checks.
	roll := 1
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	}
	total := roll + bonus

	// 7. Success.
	if total >= dc {
		// Remove detained condition and clear DetainedUntil.
		target.Conditions.Remove(target.UID, "detained")
		target.DetainedUntil = nil

		// Persist cleared detention.
		if s.detainedUntilRepo != nil {
			ctx := context.Background()
			if err := s.detainedUntilRepo.UpdateDetainedUntil(ctx, target.CharacterID, nil); err != nil {
				s.logger.Warn("handleRelease: UpdateDetainedUntil(nil) failed",
					zap.String("uid", uid),
					zap.String("target", targetName),
					zap.Error(err),
				)
			}
		}

		// REQ-WC-15: do NOT modify target's WantedLevel.

		// Refresh room views so observers see the "(detained)" annotation removed.
		s.pushRoomViewToAllInRoom(sess.RoomID)

		return messageEvent(fmt.Sprintf(
			"You slip the restraints off %s. %s is freed! (%s DC %d: rolled %d+%d=%d)",
			targetName, targetName, skillName, dc, roll, bonus, total,
		)), nil
	}

	// 8. Failure: leave detained condition in place.
	return messageEvent(fmt.Sprintf(
		"You fumble with the restraints and fail to free %s. (%s DC %d: rolled %d+%d=%d)",
		targetName, skillName, dc, roll, bonus, total,
	)), nil
}
