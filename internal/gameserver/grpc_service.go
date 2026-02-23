package gameserver

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// errQuit is returned by handleQuit to signal the command loop to stop cleanly.
var errQuit = fmt.Errorf("quit")

// CharacterSaver persists character state after a session ends.
//
// Precondition: id must be > 0; location must be a valid room ID.
// Postcondition: Returns nil on success or a non-nil error on failure.
type CharacterSaver interface {
	SaveState(ctx context.Context, id int64, location string, currentHP int) error
}

// GameServiceServer implements the gRPC GameService with bidirectional streaming.
type GameServiceServer struct {
	gamev1.UnimplementedGameServiceServer
	world      *world.Manager
	sessions   *session.Manager
	commands   *command.Registry
	worldH     *WorldHandler
	chatH      *ChatHandler
	charSaver  CharacterSaver
	dice       *dice.Roller
	npcH       *NPCHandler
	combatH    *CombatHandler
	logger     *zap.Logger
}

// NewGameServiceServer creates a GameServiceServer with the given dependencies.
//
// Precondition: worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, diceRoller, and logger must be non-nil.
// charSaver may be nil (character state will not be persisted on disconnect).
// Postcondition: Returns a fully initialised GameServiceServer.
func NewGameServiceServer(
	worldMgr *world.Manager,
	sessMgr *session.Manager,
	cmdRegistry *command.Registry,
	worldHandler *WorldHandler,
	chatHandler *ChatHandler,
	logger *zap.Logger,
	charSaver CharacterSaver,
	diceRoller *dice.Roller,
	npcHandler *NPCHandler,
	combatHandler *CombatHandler,
) *GameServiceServer {
	return &GameServiceServer{
		world:     worldMgr,
		sessions:  sessMgr,
		commands:  cmdRegistry,
		worldH:    worldHandler,
		chatH:     chatHandler,
		charSaver: charSaver,
		dice:      diceRoller,
		npcH:      npcHandler,
		combatH:   combatHandler,
		logger:    logger,
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
	charName := joinReq.CharacterName
	if charName == "" {
		charName = username // fallback for backward compat
	}
	characterID := joinReq.CharacterId
	currentHP := int(joinReq.CurrentHp)

	s.logger.Info("player joining world",
		zap.String("uid", uid),
		zap.String("username", username),
		zap.String("char_name", charName),
		zap.Int64("character_id", characterID),
	)

	// Step 2: Create player session in start room
	startRoom := s.world.StartRoom()
	if startRoom == nil {
		return fmt.Errorf("no start room configured")
	}

	sess, err := s.sessions.AddPlayer(uid, username, charName, characterID, startRoom.ID, currentHP)
	if err != nil {
		return fmt.Errorf("adding player: %w", err)
	}
	defer s.cleanupPlayer(uid, username)

	// Broadcast arrival to other players in the room
	s.broadcastRoomEvent(startRoom.ID, uid, &gamev1.RoomEvent{
		Player: charName,
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
		if err == errQuit {
			// Send Disconnected event then exit cleanly.
			if resp != nil {
				resp.RequestId = msg.RequestId
				_ = stream.Send(resp)
			}
			return nil
		}
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
	case *gamev1.ClientMessage_Examine:
		return s.handleExamine(uid, p.Examine)
	case *gamev1.ClientMessage_Attack:
		return s.handleAttack(uid, p.Attack)
	case *gamev1.ClientMessage_Flee:
		return s.handleFlee(uid)
	case *gamev1.ClientMessage_Pass:
		return s.handlePass(uid)
	case *gamev1.ClientMessage_Strike:
		return s.handleStrike(uid, p.Strike)
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

	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q session not found after move", uid)
	}

	// Broadcast departure from old room
	s.broadcastRoomEvent(result.OldRoomID, uid, &gamev1.RoomEvent{
		Player:    sess.CharName,
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
		Direction: string(dir),
	})

	// Broadcast arrival in new room
	s.broadcastRoomEvent(result.View.RoomId, uid, &gamev1.RoomEvent{
		Player:    sess.CharName,
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

	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q session not found", uid)
	}
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

	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q session not found", uid)
	}
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
		reason = fmt.Sprintf("%s has quit", sess.CharName)
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Disconnected{
			Disconnected: &gamev1.Disconnected{Reason: reason},
		},
	}, errQuit
}


func (s *GameServiceServer) handleExamine(uid string, req *gamev1.ExamineRequest) (*gamev1.ServerEvent, error) {
	view, err := s.npcH.Examine(uid, req.Target)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_NpcView{NpcView: view},
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

func (s *GameServiceServer) handleAttack(uid string, req *gamev1.AttackRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Attack(uid, req.Target)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	// Broadcast all events except the first to room (first is returned directly to player).
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) handleFlee(uid string) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Flee(uid)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		s.broadcastCombatEvent(sess.RoomID, uid, events[0])
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) broadcastCombatEvent(roomID, excludeUID string, evt *gamev1.CombatEvent) {
	s.broadcastToRoom(roomID, excludeUID, &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: evt},
	})
}

// BroadcastCombatEvents sends combat events to all players in the room.
// Called by CombatHandler's round timer callback.
//
// Postcondition: Each event is delivered to all sessions in roomID.
func (s *GameServiceServer) BroadcastCombatEvents(roomID string, events []*gamev1.CombatEvent) {
	for _, evt := range events {
		s.broadcastCombatEvent(roomID, "", evt)
	}
}

func (s *GameServiceServer) handlePass(uid string) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Pass(uid)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) handleStrike(uid string, req *gamev1.StrikeRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Strike(uid, req.GetTarget())
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

// cleanupPlayer removes a player from the session manager, persists character state, and broadcasts departure.
//
// Precondition: uid must be non-empty.
// Postcondition: Player is removed from all tracking; character state is saved if charSaver is configured.
func (s *GameServiceServer) cleanupPlayer(uid, username string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	roomID := sess.RoomID
	characterID := sess.CharacterID
	currentHP := sess.CurrentHP
	charName := sess.CharName

	if err := s.sessions.RemovePlayer(uid); err != nil {
		s.logger.Warn("removing player on cleanup", zap.String("uid", uid), zap.Error(err))
	}

	// Persist character state on disconnect.
	if characterID > 0 && s.charSaver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.charSaver.SaveState(ctx, characterID, roomID, currentHP); err != nil {
			s.logger.Warn("saving character state on disconnect",
				zap.String("uid", uid),
				zap.Int64("character_id", characterID),
				zap.Error(err),
			)
		} else {
			s.logger.Info("character state saved",
				zap.Int64("character_id", characterID),
				zap.String("room", roomID),
			)
		}
	}

	s.broadcastRoomEvent(roomID, uid, &gamev1.RoomEvent{
		Player: charName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})

	s.logger.Info("player disconnected",
		zap.String("uid", uid),
		zap.String("username", username),
		zap.Int64("character_id", characterID),
	)
}
