package gameserver

import (
	"context"
	"fmt"
	"io"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// GameServiceServer implements the gRPC GameService with bidirectional streaming.
type GameServiceServer struct {
	gamev1.UnimplementedGameServiceServer
	world    *world.Manager
	sessions *session.Manager
	commands *command.Registry
	worldH   *WorldHandler
	chatH    *ChatHandler
	logger   *zap.Logger
}

// NewGameServiceServer creates a GameServiceServer with the given dependencies.
//
// Precondition: All parameters must be non-nil.
func NewGameServiceServer(
	worldMgr *world.Manager,
	sessMgr *session.Manager,
	cmdRegistry *command.Registry,
	worldHandler *WorldHandler,
	chatHandler *ChatHandler,
	logger *zap.Logger,
) *GameServiceServer {
	return &GameServiceServer{
		world:    worldMgr,
		sessions: sessMgr,
		commands: cmdRegistry,
		worldH:   worldHandler,
		chatH:    chatHandler,
		logger:   logger,
	}
}

// Session implements the bidirectional streaming RPC.
// Flow:
//  1. Wait for JoinWorldRequest
//  2. Create player session, place in start room
//  3. Spawn goroutine to forward entity events to gRPC stream
//  4. Main loop: read ClientMessage, dispatch, send response
//  5. On disconnect: clean up session
func (s *GameServiceServer) Session(stream gamev1.GameService_SessionServer) error {
	// Step 1: Wait for JoinWorldRequest
	firstMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving join request: %w", err)
	}

	joinReq := firstMsg.GetJoinWorld()
	if joinReq == nil {
		return fmt.Errorf("first message must be JoinWorldRequest")
	}

	uid := joinReq.Uid
	username := joinReq.Username

	s.logger.Info("player joining world",
		zap.String("uid", uid),
		zap.String("username", username),
	)

	// Step 2: Create player session in start room
	startRoom := s.world.StartRoom()
	if startRoom == nil {
		return fmt.Errorf("no start room configured")
	}

	sess, err := s.sessions.AddPlayer(uid, username, startRoom.ID)
	if err != nil {
		return fmt.Errorf("adding player: %w", err)
	}
	defer s.cleanupPlayer(uid, username)

	// Broadcast arrival to other players in the room
	s.broadcastRoomEvent(startRoom.ID, uid, &gamev1.RoomEvent{
		Player: username,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})

	// Send initial room view
	roomView := s.worldH.buildRoomView(uid, startRoom)
	if err := stream.Send(&gamev1.ServerEvent{
		RequestId: firstMsg.RequestId,
		Payload:   &gamev1.ServerEvent_RoomView{RoomView: roomView},
	}); err != nil {
		return fmt.Errorf("sending initial room view: %w", err)
	}

	// Step 3: Spawn goroutine to forward entity events to stream
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.forwardEvents(ctx, sess.Entity, stream)
	}()

	// Step 4: Main command loop
	err = s.commandLoop(ctx, uid, stream)

	// Step 5: Cleanup happens via defer
	cancel()
	wg.Wait()

	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

// commandLoop processes incoming ClientMessages until the stream ends.
func (s *GameServiceServer) commandLoop(ctx context.Context, uid string, stream gamev1.GameService_SessionServer) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			return io.EOF
		}
		if err != nil {
			return fmt.Errorf("receiving message: %w", err)
		}

		resp, err := s.dispatch(uid, msg)
		if err != nil {
			// Send error event
			errEvt := &gamev1.ServerEvent{
				RequestId: msg.RequestId,
				Payload: &gamev1.ServerEvent_Error{
					Error: &gamev1.ErrorEvent{Message: err.Error()},
				},
			}
			if sendErr := stream.Send(errEvt); sendErr != nil {
				return fmt.Errorf("sending error: %w", sendErr)
			}
			continue
		}

		if resp != nil {
			resp.RequestId = msg.RequestId
			if err := stream.Send(resp); err != nil {
				return fmt.Errorf("sending response: %w", err)
			}
		}
	}
}

// dispatch routes a ClientMessage to the appropriate handler.
func (s *GameServiceServer) dispatch(uid string, msg *gamev1.ClientMessage) (*gamev1.ServerEvent, error) {
	switch p := msg.Payload.(type) {
	case *gamev1.ClientMessage_Move:
		return s.handleMove(uid, p.Move)
	case *gamev1.ClientMessage_Look:
		return s.handleLook(uid)
	case *gamev1.ClientMessage_Exits:
		return s.handleExits(uid)
	case *gamev1.ClientMessage_Say:
		return s.handleSay(uid, p.Say)
	case *gamev1.ClientMessage_Emote:
		return s.handleEmote(uid, p.Emote)
	case *gamev1.ClientMessage_Who:
		return s.handleWho(uid)
	case *gamev1.ClientMessage_Quit:
		return s.handleQuit(uid)
	default:
		return nil, fmt.Errorf("unknown message type")
	}
}

func (s *GameServiceServer) handleMove(uid string, req *gamev1.MoveRequest) (*gamev1.ServerEvent, error) {
	dir := world.Direction(req.Direction)

	result, err := s.worldH.MoveWithContext(uid, dir)
	if err != nil {
		return nil, err
	}

	sess, _ := s.sessions.GetPlayer(uid)

	// Broadcast departure from old room
	s.broadcastRoomEvent(result.OldRoomID, uid, &gamev1.RoomEvent{
		Player:    sess.Username,
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
		Direction: string(dir),
	})

	// Broadcast arrival in new room
	s.broadcastRoomEvent(result.View.RoomId, uid, &gamev1.RoomEvent{
		Player:    sess.Username,
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
		Direction: string(dir.Opposite()),
	})

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: result.View},
	}, nil
}

func (s *GameServiceServer) handleLook(uid string) (*gamev1.ServerEvent, error) {
	view, err := s.worldH.Look(uid)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: view},
	}, nil
}

func (s *GameServiceServer) handleExits(uid string) (*gamev1.ServerEvent, error) {
	exitList, err := s.worldH.Exits(uid)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_ExitList{ExitList: exitList},
	}, nil
}

func (s *GameServiceServer) handleSay(uid string, req *gamev1.SayRequest) (*gamev1.ServerEvent, error) {
	msgEvt, err := s.chatH.Say(uid, req.Message)
	if err != nil {
		return nil, err
	}

	sess, _ := s.sessions.GetPlayer(uid)
	s.broadcastMessage(sess.RoomID, uid, msgEvt)

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{Message: msgEvt},
	}, nil
}

func (s *GameServiceServer) handleEmote(uid string, req *gamev1.EmoteRequest) (*gamev1.ServerEvent, error) {
	msgEvt, err := s.chatH.Emote(uid, req.Action)
	if err != nil {
		return nil, err
	}

	sess, _ := s.sessions.GetPlayer(uid)
	s.broadcastMessage(sess.RoomID, uid, msgEvt)

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{Message: msgEvt},
	}, nil
}

func (s *GameServiceServer) handleWho(uid string) (*gamev1.ServerEvent, error) {
	playerList, err := s.chatH.Who(uid)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_PlayerList{PlayerList: playerList},
	}, nil
}

func (s *GameServiceServer) handleQuit(uid string) (*gamev1.ServerEvent, error) {
	sess, _ := s.sessions.GetPlayer(uid)
	reason := "Goodbye"
	if sess != nil {
		reason = fmt.Sprintf("%s has quit", sess.Username)
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Disconnected{
			Disconnected: &gamev1.Disconnected{Reason: reason},
		},
	}, nil
}

// broadcastRoomEvent sends a RoomEvent to all players in a room except the excluded UID.
func (s *GameServiceServer) broadcastRoomEvent(roomID, excludeUID string, evt *gamev1.RoomEvent) {
	serverEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomEvent{RoomEvent: evt},
	}
	s.broadcastToRoom(roomID, excludeUID, serverEvt)
}

// broadcastMessage sends a MessageEvent to all players in a room except the sender.
func (s *GameServiceServer) broadcastMessage(roomID, excludeUID string, evt *gamev1.MessageEvent) {
	serverEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{Message: evt},
	}
	s.broadcastToRoom(roomID, excludeUID, serverEvt)
}

// broadcastToRoom serializes a ServerEvent and pushes it to all BridgeEntities
// in the room, excluding the specified UID.
func (s *GameServiceServer) broadcastToRoom(roomID, excludeUID string, evt *gamev1.ServerEvent) {
	data, err := proto.Marshal(evt)
	if err != nil {
		s.logger.Error("marshaling broadcast event", zap.Error(err))
		return
	}

	uids := s.sessions.PlayerUIDsInRoom(roomID)
	for _, uid := range uids {
		if uid == excludeUID {
			continue
		}
		sess, ok := s.sessions.GetPlayer(uid)
		if !ok {
			continue
		}
		if err := sess.Entity.Push(data); err != nil {
			s.logger.Warn("push to entity failed",
				zap.String("uid", uid),
				zap.Error(err),
			)
		}
	}
}

// forwardEvents reads from the BridgeEntity events channel and sends
// deserialized ServerEvents to the gRPC stream.
func (s *GameServiceServer) forwardEvents(ctx context.Context, entity *session.BridgeEntity, stream gamev1.GameService_SessionServer) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-entity.Events():
			if !ok {
				return
			}
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				s.logger.Error("unmarshaling event from entity", zap.Error(err))
				continue
			}
			if err := stream.Send(&evt); err != nil {
				s.logger.Debug("forward event send failed", zap.Error(err))
				return
			}
		}
	}
}

// cleanupPlayer removes a player from the session manager and broadcasts departure.
func (s *GameServiceServer) cleanupPlayer(uid, username string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	roomID := sess.RoomID

	if err := s.sessions.RemovePlayer(uid); err != nil {
		s.logger.Warn("removing player on cleanup", zap.String("uid", uid), zap.Error(err))
	}

	s.broadcastRoomEvent(roomID, uid, &gamev1.RoomEvent{
		Player: username,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})

	s.logger.Info("player disconnected",
		zap.String("uid", uid),
		zap.String("username", username),
	)
}
