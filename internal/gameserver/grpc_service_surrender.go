package gameserver

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// detentionDuration maps a wanted level to a real-time detention duration.
// Per spec: 1 in-game hour = 1 real minute.
// WL1 = 30s, WL2 = 1m, WL3 = 3m, WL4 = 8m.
//
// Precondition: wantedLevel is in [1, 4]; values outside that range fall back to 30 seconds.
// Postcondition: Returns a positive, non-zero duration.
func detentionDuration(wantedLevel int) time.Duration {
	switch wantedLevel {
	case 1:
		return 30 * time.Second
	case 2:
		return 1 * time.Minute
	case 3:
		return 3 * time.Minute
	case 4:
		return 8 * time.Minute
	default:
		return 30 * time.Second
	}
}

// handleSurrender processes the surrender command.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sets sess.DetainedUntil, applies the detained condition, persists to DB.
func (s *GameServiceServer) handleSurrender(uid string, req *gamev1.SurrenderRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	// Resolve current room and zone.
	room, roomOK := s.world.GetRoom(sess.RoomID)
	if !roomOK {
		return messageEvent("current room not found"), nil
	}
	zoneID := room.ZoneID

	// REQ-WC-13: player must be wanted.
	wantedLevel := sess.WantedLevel[zoneID]
	if wantedLevel == 0 {
		return messageEvent("You are not wanted here."), nil
	}

	// REQ-WC-12: a guard NPC must be present in the room.
	instances := s.npcMgr.InstancesInRoom(sess.RoomID)
	hasGuard := false
	for _, inst := range instances {
		if inst.NPCType == "guard" {
			hasGuard = true
			break
		}
	}
	if !hasGuard {
		return messageEvent("There is no one here to surrender to."), nil
	}

	// Compute detention expiry.
	dur := detentionDuration(wantedLevel)
	sess.DetainedUntil = new(time.Now().Add(dur))

	// Apply detained condition.
	if s.condRegistry != nil {
		if def, ok := s.condRegistry.Get("detained"); ok {
			if sess.Conditions == nil {
				sess.Conditions = condition.NewActiveSet()
			}
			sess.Conditions.Apply(sess.UID, def, 1, -1) //nolint:errcheck
		}
	}

	// Persist detention expiry.
	if s.detainedUntilRepo != nil {
		ctx := context.Background()
		if err := s.detainedUntilRepo.UpdateDetainedUntil(ctx, sess.CharacterID, sess.DetainedUntil); err != nil {
			s.logger.Warn("handleSurrender: UpdateDetainedUntil failed",
				zap.String("uid", uid),
				zap.Error(err),
			)
		}
	}

	return messageEvent(fmt.Sprintf(
		"You surrender. You will be detained for %s.",
		formatDetentionDuration(dur),
	)), nil
}

// checkDetentionCompletion checks whether a player's detention has expired and completes it.
// Called at command dispatch, player login, and regen tick.
//
// Precondition: sess must not be nil.
// REQ-WC-14b: detention is complete when DetainedUntil is in the past.
// REQ-WC-14c: sets a 5-second DetentionGraceUntil window after completion.
// Postcondition: if expired — DetainedUntil cleared, detained condition removed,
// WantedLevel decremented, DetentionGraceUntil set 5 seconds from now.
func (s *GameServiceServer) checkDetentionCompletion(sess *session.PlayerSession) {
	if sess.DetainedUntil == nil {
		return
	}
	if time.Now().Before(*sess.DetainedUntil) {
		return
	}

	// Detention expired.
	sess.DetainedUntil = nil

	// Remove detained condition.
	if sess.Conditions != nil {
		sess.Conditions.Remove(sess.UID, "detained")
	}

	// Resolve zone and decrement WantedLevel.
	zoneID := ""
	if room, ok := s.world.GetRoom(sess.RoomID); ok {
		zoneID = room.ZoneID
	}
	if zoneID != "" && sess.WantedLevel[zoneID] > 0 {
		sess.WantedLevel[zoneID]--
		newLevel := sess.WantedLevel[zoneID]

		// Persist wanted level.
		ctx := context.Background()
		if s.wantedRepo != nil {
			if err := s.wantedRepo.Upsert(ctx, sess.CharacterID, zoneID, newLevel); err != nil {
				s.logger.Warn("checkDetentionCompletion: Upsert wanted level failed",
					zap.String("uid", sess.UID),
					zap.Error(err),
				)
			}
		}
	}

	// Persist cleared detention.
	if s.detainedUntilRepo != nil {
		ctx := context.Background()
		if err := s.detainedUntilRepo.UpdateDetainedUntil(ctx, sess.CharacterID, nil); err != nil {
			s.logger.Warn("checkDetentionCompletion: UpdateDetainedUntil(nil) failed",
				zap.String("uid", sess.UID),
				zap.Error(err),
			)
		}
	}

	// Set grace window.
	sess.DetentionGraceUntil = time.Now().Add(5 * time.Second)

	// Notify the player.
	msg := "Your detention is complete. You are free to go."
	msgEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: msg,
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if data, err := proto.Marshal(msgEvt); err == nil {
		_ = sess.Entity.Push(data)
	}

	// Update room views so "(detained)" annotation is removed.
	s.pushRoomViewToAllInRoom(sess.RoomID)
}

// formatDetentionDuration returns a human-readable string for the given duration.
//
// Precondition: d must be positive.
// Postcondition: Returns a non-empty string.
func formatDetentionDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d == time.Minute {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", int(d.Minutes()))
}
