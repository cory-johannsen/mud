package gameserver

// Admin gRPC RPCs on GameServiceServer.
//
// Precondition: Caller is trusted at the network boundary — no role check is performed here.
// Postcondition: Each method satisfies its documented contract or returns a gRPC status error.

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// AdminListSessions (REQ-AGA-4) returns a snapshot of all active player sessions.
//
// Precondition: none.
// Postcondition: Sessions with CharacterID == 0 are omitted; all others are mapped to AdminSessionInfo.
func (s *GameServiceServer) AdminListSessions(_ context.Context, _ *gamev1.AdminListSessionsRequest) (*gamev1.AdminListSessionsResponse, error) {
	players := s.sessions.AllPlayers()
	infos := make([]*gamev1.AdminSessionInfo, 0, len(players))
	for _, sess := range players {
		if sess.CharacterID == 0 {
			continue
		}
		zone := ""
		if room, ok := s.world.GetRoom(sess.RoomID); ok {
			zone = room.ZoneID
		}
		infos = append(infos, &gamev1.AdminSessionInfo{
			CharId:     sess.CharacterID,
			PlayerName: sess.CharName,
			Level:      int32(sess.Level),
			RoomId:     sess.RoomID,
			Zone:       zone,
			CurrentHp:  int32(sess.CurrentHP),
			AccountId:  0, // not available in PlayerSession; web layer has it from JWT
		})
	}
	return &gamev1.AdminListSessionsResponse{Sessions: infos}, nil
}

// AdminKickPlayer (REQ-AGA-5) disconnects a player by character ID.
//
// Precondition: req.CharId must identify an online player.
// Postcondition: A Disconnected ServerEvent is pushed to the target; returns NotFound if player is not online.
func (s *GameServiceServer) AdminKickPlayer(_ context.Context, req *gamev1.AdminKickRequest) (*gamev1.AdminKickResponse, error) {
	target := s.sessions.GetPlayerByCharID(req.CharId)
	if target == nil {
		return nil, status.Errorf(codes.NotFound, "player not found")
	}
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Disconnected{
			Disconnected: &gamev1.Disconnected{
				Reason: fmt.Sprintf("%s has been kicked by an admin", target.CharName),
			},
		},
	}
	if data, err := proto.Marshal(evt); err == nil {
		_ = target.Entity.Push(data)
	}
	return &gamev1.AdminKickResponse{}, nil
}

// AdminMessagePlayer (REQ-AGA-6) sends a text message to a player by character ID.
//
// Precondition: req.CharId must identify an online player.
// Postcondition: A Message ServerEvent is pushed to the target; returns NotFound if player is not online.
func (s *GameServiceServer) AdminMessagePlayer(_ context.Context, req *gamev1.AdminMessageRequest) (*gamev1.AdminMessageResponse, error) {
	target := s.sessions.GetPlayerByCharID(req.CharId)
	if target == nil {
		return nil, status.Errorf(codes.NotFound, "player not found")
	}
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: req.Text,
				Type:    gamev1.MessageType_MESSAGE_TYPE_SAY,
			},
		},
	}
	if data, err := proto.Marshal(evt); err == nil {
		_ = target.Entity.Push(data)
	}
	return &gamev1.AdminMessageResponse{}, nil
}

// AdminTeleportPlayer (REQ-AGA-7) teleports a player to a specific room by character and room ID.
//
// Precondition: req.CharId must identify an online player; req.RoomId must identify a loaded room.
// Postcondition: Player is moved, location is persisted, departure/arrival events are broadcast,
// and a teleport message + room view are pushed to the target.
func (s *GameServiceServer) AdminTeleportPlayer(ctx context.Context, req *gamev1.AdminTeleportRequest) (*gamev1.AdminTeleportResponse, error) {
	target := s.sessions.GetPlayerByCharID(req.CharId)
	if target == nil {
		return nil, status.Errorf(codes.NotFound, "player not found")
	}
	targetRoom, ok := s.world.GetRoom(req.RoomId)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "room not found")
	}

	oldRoomID, err := s.sessions.MovePlayer(target.UID, targetRoom.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to move player: %v", err)
	}

	// Persist location immediately.
	if target.CharacterID > 0 && s.charSaver != nil {
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.charSaver.SaveState(saveCtx, target.CharacterID, targetRoom.ID, target.CurrentHP); err != nil {
			s.logger.Warn("persisting admin teleport location",
				zap.String("target", target.CharName),
				zap.Error(err),
			)
		}
	}

	// Broadcast departure from old room.
	s.broadcastRoomEvent(oldRoomID, target.UID, &gamev1.RoomEvent{
		Player: target.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})

	// Broadcast arrival in new room.
	s.broadcastRoomEvent(targetRoom.ID, target.UID, &gamev1.RoomEvent{
		Player: target.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})

	// Send message and room view to the target player.
	roomView := s.worldH.buildRoomView(target.UID, targetRoom)
	teleportMsg := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: fmt.Sprintf("You have been teleported to %s.", targetRoom.Title),
				Type:    gamev1.MessageType_MESSAGE_TYPE_SAY,
			},
		},
	}
	teleportView := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: roomView},
	}
	if data, err := proto.Marshal(teleportMsg); err == nil {
		_ = target.Entity.Push(data)
	}
	if data, err := proto.Marshal(teleportView); err == nil {
		_ = target.Entity.Push(data)
	}

	return &gamev1.AdminTeleportResponse{}, nil
}
