package gameserver

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"strings"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/scripting"

	lua "github.com/yuin/gopher-lua"
)

// errQuit is returned by handleQuit to signal the command loop to stop cleanly.
var errQuit = fmt.Errorf("quit")

// AccountAdmin provides account lookup and role mutation for in-game admin commands.
//
// Precondition: Implementations must be safe for concurrent use.
type AccountAdmin interface {
	GetAccountByUsername(ctx context.Context, username string) (AccountInfo, error)
	SetAccountRole(ctx context.Context, accountID int64, role string) error
}

// AccountInfo is a minimal view of an account used by AccountAdmin.
type AccountInfo struct {
	ID       int64
	Username string
	Role     string
}

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
	npcMgr     *npc.Manager
	combatH    *CombatHandler
	scriptMgr  *scripting.Manager
	respawnMgr  *npc.RespawnManager
	floorMgr    *inventory.FloorManager
	invRegistry  *inventory.Registry
	accountAdmin AccountAdmin
	logger       *zap.Logger
}

// NewGameServiceServer creates a GameServiceServer with the given dependencies.
//
// Precondition: worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, diceRoller, and logger must be non-nil.
// charSaver may be nil (character state will not be persisted on disconnect).
// respawnMgr may be nil (respawn functionality will be disabled).
// floorMgr may be nil (inventory get/drop will return errors).
// invRegistry may be nil (item name resolution will fall back to ItemDefID).
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
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	scriptMgr *scripting.Manager,
	respawnMgr *npc.RespawnManager,
	floorMgr *inventory.FloorManager,
	invRegistry *inventory.Registry,
	accountAdmin AccountAdmin,
) *GameServiceServer {
	return &GameServiceServer{
		world:       worldMgr,
		sessions:    sessMgr,
		commands:    cmdRegistry,
		worldH:      worldHandler,
		chatH:       chatHandler,
		charSaver:   charSaver,
		dice:        diceRoller,
		npcH:        npcHandler,
		npcMgr:      npcMgr,
		combatH:     combatHandler,
		scriptMgr:   scriptMgr,
		respawnMgr:  respawnMgr,
		floorMgr:    floorMgr,
		invRegistry:  invRegistry,
		accountAdmin: accountAdmin,
		logger:       logger,
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

	// Step 2: Create player session in saved location (or global start room)
	var spawnRoom *world.Room
	if loc := joinReq.Location; loc != "" {
		if r, ok := s.world.GetRoom(loc); ok {
			spawnRoom = r
		}
	}
	if spawnRoom == nil {
		spawnRoom = s.world.StartRoom()
	}
	if spawnRoom == nil {
		return fmt.Errorf("no start room configured")
	}

	role := joinReq.Role
	if role == "" {
		role = "player"
	}

	sess, err := s.sessions.AddPlayer(uid, username, charName, characterID, spawnRoom.ID, currentHP, role)
	if err != nil {
		return fmt.Errorf("adding player: %w", err)
	}
	defer s.cleanupPlayer(uid, username)

	// Broadcast arrival to other players in the room
	s.broadcastRoomEvent(spawnRoom.ID, uid, &gamev1.RoomEvent{
		Player: charName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})

	// Send initial room view
	roomView := s.worldH.buildRoomView(uid, spawnRoom)
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

		// status is a personal query: all ConditionEvents are sent directly on the stream.
		if _, ok := msg.Payload.(*gamev1.ClientMessage_Status); ok {
			if err := s.handleStatus(uid, msg.RequestId, stream); err != nil {
				errEvt := &gamev1.ServerEvent{
					RequestId: msg.RequestId,
					Payload: &gamev1.ServerEvent_Error{
						Error: &gamev1.ErrorEvent{Message: err.Error()},
					},
				}
				if sendErr := stream.Send(errEvt); sendErr != nil {
					return fmt.Errorf("sending error: %w", sendErr)
				}
			}
			continue
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
	case *gamev1.ClientMessage_Equip:
		return s.handleEquip(uid, p.Equip)
	case *gamev1.ClientMessage_Reload:
		return s.handleReload(uid, p.Reload)
	case *gamev1.ClientMessage_FireBurst:
		return s.handleFireBurst(uid, p.FireBurst)
	case *gamev1.ClientMessage_FireAutomatic:
		return s.handleFireAutomatic(uid, p.FireAutomatic)
	case *gamev1.ClientMessage_Throw:
		return s.handleThrow(uid, p.Throw)
	case *gamev1.ClientMessage_InventoryReq:
		return s.handleInventory(uid)
	case *gamev1.ClientMessage_GetItem:
		return s.handleGetItem(uid, p.GetItem.Target)
	case *gamev1.ClientMessage_DropItem:
		return s.handleDropItem(uid, p.DropItem.Target)
	case *gamev1.ClientMessage_Balance:
		return s.handleBalance(uid)
	case *gamev1.ClientMessage_SetRole:
		return s.handleSetRole(uid, p.SetRole)
	case *gamev1.ClientMessage_Teleport:
		return s.handleTeleport(uid, p.Teleport)
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

	if s.scriptMgr != nil {
		if oldRoom, ok := s.world.GetRoom(result.OldRoomID); ok {
			s.scriptMgr.CallHook(oldRoom.ZoneID, "on_exit", //nolint:errcheck
				lua.LString(uid),
				lua.LString(result.OldRoomID),
				lua.LString(result.View.RoomId),
			)
		}
		if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
			s.scriptMgr.CallHook(newRoom.ZoneID, "on_enter", //nolint:errcheck
				lua.LString(uid),
				lua.LString(result.View.RoomId),
				lua.LString(result.OldRoomID),
			)
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: result.View},
	}, nil
}

func (s *GameServiceServer) handleLook(uid string) (*gamev1.ServerEvent, error) {
	view, err := s.worldH.Look(uid)
	if err != nil {
		return nil, err
	}
	if s.scriptMgr != nil {
		if sess, ok := s.sessions.GetPlayer(uid); ok {
			if room, ok := s.world.GetRoom(sess.RoomID); ok {
				s.scriptMgr.CallHook(room.ZoneID, "on_look", //nolint:errcheck
					lua.LString(uid),
					lua.LString(room.ID),
				)
			}
		}
	}

	// Append floor items to the room view.
	//
	// Precondition: view is a fully constructed RoomView from worldH.Look.
	// Postcondition: view.FloorItems contains all items on the floor of the
	// player's current room; item names are resolved from the registry when
	// available, falling back to the raw ItemDefID.
	if sess, ok := s.sessions.GetPlayer(uid); ok && s.floorMgr != nil {
		floorItems := s.floorMgr.ItemsInRoom(sess.RoomID)
		for _, fi := range floorItems {
			name := fi.ItemDefID
			if s.invRegistry != nil {
				if def, ok := s.invRegistry.Item(fi.ItemDefID); ok {
					name = def.Name
				}
			}
			view.FloorItems = append(view.FloorItems, &gamev1.FloorItem{
				InstanceId: fi.InstanceID,
				Name:       name,
				Quantity:   int32(fi.Quantity),
			})
		}
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

// handleStatus sends the active conditions for uid directly on stream.
// One ConditionEvent is sent per active condition. If no conditions are active,
// a single empty sentinel ConditionEvent is sent to signal "no conditions".
// All events are sent only to the requesting player — no room broadcast occurs.
//
// Precondition: uid must be a valid connected player; stream must be non-nil.
// Postcondition: One or more ConditionEvents are sent on stream; returns nil on success.
func (s *GameServiceServer) handleStatus(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
	conds, err := s.combatH.Status(uid)
	if err != nil {
		return err
	}
	if len(conds) == 0 {
		// Send empty sentinel so the client knows no conditions are active.
		return stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_ConditionEvent{
				ConditionEvent: &gamev1.ConditionEvent{},
			},
		})
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}
	for _, ac := range conds {
		if err := stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_ConditionEvent{
				ConditionEvent: &gamev1.ConditionEvent{
					TargetUid:     uid,
					TargetName:    sess.CharName,
					ConditionId:   ac.Def.ID,
					ConditionName: ac.Def.Name,
					Stacks:        int32(ac.Stacks),
					Applied:       true,
				},
			},
		}); err != nil {
			return fmt.Errorf("sending condition event: %w", err)
		}
	}
	return nil
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

// errorEvent builds a ServerEvent carrying an ErrorEvent with the given message.
//
// Precondition: msg must be non-empty.
// Postcondition: Returns a non-nil ServerEvent.
func errorEvent(msg string) *gamev1.ServerEvent {
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Error{
			Error: &gamev1.ErrorEvent{Message: msg},
		},
	}
}

// messageEvent builds a ServerEvent carrying a MessageEvent with the given text.
//
// Precondition: text must be non-empty.
// Postcondition: Returns a non-nil ServerEvent.
func messageEvent(text string) *gamev1.ServerEvent {
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: text},
		},
	}
}

func (s *GameServiceServer) handleEquip(uid string, req *gamev1.EquipRequest) (*gamev1.ServerEvent, error) {
	_, err := s.combatH.Equip(uid, req.GetWeaponId(), req.GetSlot())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	return messageEvent("Equipped " + req.GetWeaponId() + "."), nil
}

func (s *GameServiceServer) handleReload(uid string, req *gamev1.ReloadRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Reload(uid)
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

func (s *GameServiceServer) handleFireBurst(uid string, req *gamev1.FireBurstRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.FireBurst(uid, req.GetTarget())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

func (s *GameServiceServer) handleFireAutomatic(uid string, req *gamev1.FireAutomaticRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.FireAutomatic(uid, req.GetTarget())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

func (s *GameServiceServer) handleThrow(uid string, req *gamev1.ThrowRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Throw(uid, req.GetExplosiveId())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

// StartZoneTicks starts per-zone periodic tick goroutines.
//
// Precondition: ctx must not be nil; zm and aiReg must not be nil.
// Postcondition: zone tick goroutines are running until ctx is cancelled.
func (s *GameServiceServer) StartZoneTicks(ctx context.Context, zm *ZoneTickManager, aiReg *ai.Registry) {
	for _, zone := range s.world.AllZones() {
		zoneID := zone.ID
		zm.RegisterTick(zoneID, func() {
			s.tickZone(zoneID, aiReg)
		})
	}
	zm.Start(ctx)
}

// tickZone executes one game tick for the given zone, advancing NPC AI and
// draining the respawn queue.
//
// Precondition: zoneID must be a valid zone identifier loaded in worldMgr.
func (s *GameServiceServer) tickZone(zoneID string, aiReg *ai.Registry) {
	for _, zone := range s.world.AllZones() {
		if zone.ID != zoneID {
			continue
		}
		if s.npcH == nil {
			break
		}
		for _, room := range zone.Rooms {
			for _, inst := range s.npcH.InstancesInRoom(room.ID) {
				if s.combatH != nil && s.combatH.IsInCombat(inst.ID) {
					continue
				}
				s.tickNPCIdle(inst, zoneID, aiReg)
			}
		}
	}
	if s.respawnMgr != nil {
		s.respawnMgr.Tick(time.Now(), s.npcMgr)
	}
}

// tickNPCIdle evaluates idle/patrol behavior for a non-combat NPC.
func (s *GameServiceServer) tickNPCIdle(inst *npc.Instance, zoneID string, aiReg *ai.Registry) {
	if inst.AIDomain == "" || aiReg == nil {
		return
	}
	planner, ok := aiReg.PlannerFor(inst.AIDomain)
	if !ok {
		return
	}
	ws := &ai.WorldState{
		NPC: &ai.NPCState{
			UID:        inst.ID,
			Name:       inst.Name,
			Kind:       "npc",
			HP:         inst.CurrentHP,
			MaxHP:      inst.MaxHP,
			Perception: inst.Perception,
			ZoneID:     zoneID,
			RoomID:     inst.RoomID,
		},
		Room:       &ai.RoomState{ID: inst.RoomID, ZoneID: zoneID},
		Combatants: nil,
	}
	actions, err := planner.Plan(ws)
	if err != nil || len(actions) == 0 {
		return
	}
	for _, a := range actions {
		switch a.Action {
		case "move_random":
			s.npcPatrolRandom(inst)
		default:
			// idle/pass: no-op
		}
	}
}

// npcPatrolRandom moves the NPC to a random visible exit.
func (s *GameServiceServer) npcPatrolRandom(inst *npc.Instance) {
	room, ok := s.world.GetRoom(inst.RoomID)
	if !ok || len(room.Exits) == 0 {
		return
	}
	exits := room.VisibleExits()
	if len(exits) == 0 {
		return
	}
	idx := rand.Intn(len(exits))
	_ = s.npcH.MoveNPC(inst.ID, exits[idx].TargetRoom)
}

// handleInventory sends the player's backpack contents and currency.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a ServerEvent containing InventoryView, or an error event.
func (s *GameServiceServer) handleInventory(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	var items []*gamev1.InventoryItem
	for _, inst := range sess.Backpack.Items() {
		name := inst.ItemDefID
		kind := ""
		weight := 0.0
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(inst.ItemDefID); ok {
				name = def.Name
				kind = def.Kind
				weight = def.Weight
			}
		}
		items = append(items, &gamev1.InventoryItem{
			InstanceId: inst.InstanceID,
			Name:       name,
			Kind:       kind,
			Quantity:   int32(inst.Quantity),
			Weight:     weight * float64(inst.Quantity),
		})
	}
	var totalWeight float64
	if s.invRegistry != nil {
		totalWeight = sess.Backpack.TotalWeight(s.invRegistry)
	}
	view := &gamev1.InventoryView{
		Items:       items,
		UsedSlots:   int32(sess.Backpack.UsedSlots()),
		MaxSlots:    int32(sess.Backpack.MaxSlots),
		TotalWeight: totalWeight,
		MaxWeight:   sess.Backpack.MaxWeight,
		Currency:    inventory.FormatRounds(sess.Currency),
		TotalRounds: int32(sess.Currency),
	}
	return &gamev1.ServerEvent{Payload: &gamev1.ServerEvent_InventoryView{InventoryView: view}}, nil
}

// handleBalance sends the player's currency breakdown.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a ServerEvent containing a message with the currency string.
func (s *GameServiceServer) handleBalance(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	return messageEvent(fmt.Sprintf("Currency: %s", inventory.FormatRounds(sess.Currency))), nil
}

// handleSetRole changes a target account's privilege level.
//
// Precondition: uid must be a valid connected player with admin role.
// Postcondition: Returns a success message or an error event.
func (s *GameServiceServer) handleSetRole(uid string, req *gamev1.SetRoleRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if sess.Role != "admin" {
		return errorEvent("permission denied: admin role required"), nil
	}
	if s.accountAdmin == nil {
		return errorEvent("account administration not available"), nil
	}
	if req.TargetUsername == "" || req.Role == "" {
		return errorEvent("usage: setrole <username> <role>"), nil
	}

	target, err := s.accountAdmin.GetAccountByUsername(context.Background(), req.TargetUsername)
	if err != nil {
		return errorEvent(fmt.Sprintf("account %q not found", req.TargetUsername)), nil
	}
	if err := s.accountAdmin.SetAccountRole(context.Background(), target.ID, req.Role); err != nil {
		return errorEvent(fmt.Sprintf("failed to set role: %v", err)), nil
	}

	return messageEvent(fmt.Sprintf("Set role for %s (#%d): %s -> %s",
		target.Username, target.ID, target.Role, req.Role)), nil
}

// handleTeleport moves a target player to a specific room by ID.
//
// Precondition: uid must be a valid connected player with admin role.
// Postcondition: Target player is moved, location is persisted, target receives a message and room view.
func (s *GameServiceServer) handleTeleport(uid string, req *gamev1.TeleportRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if sess.Role != "admin" {
		return errorEvent("permission denied: admin role required"), nil
	}
	if req.TargetCharacter == "" || req.RoomId == "" {
		return errorEvent("usage: teleport <character> <room_id>"), nil
	}

	targetRoom, ok := s.world.GetRoom(req.RoomId)
	if !ok {
		return errorEvent(fmt.Sprintf("room %q not found", req.RoomId)), nil
	}

	target, ok := s.sessions.GetPlayerByCharName(req.TargetCharacter)
	if !ok {
		return errorEvent(fmt.Sprintf("player %q not online", req.TargetCharacter)), nil
	}

	oldRoomID, err := s.sessions.MovePlayer(target.UID, targetRoom.ID)
	if err != nil {
		return errorEvent(fmt.Sprintf("failed to move player: %v", err)), nil
	}

	// Persist location immediately.
	if target.CharacterID > 0 && s.charSaver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.charSaver.SaveState(ctx, target.CharacterID, targetRoom.ID, target.CurrentHP); err != nil {
			s.logger.Warn("persisting teleport location",
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

	return messageEvent(fmt.Sprintf("Teleported %s to %s (%s)",
		target.CharName, targetRoom.Title, targetRoom.ID)), nil
}

// handleGetItem picks up an item from the room floor into the player's backpack.
//
// Precondition: uid must be a valid connected player; target is the item name or "all".
// Postcondition: On success the item is moved from floor to backpack; returns a message event.
func (s *GameServiceServer) handleGetItem(uid, target string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if s.floorMgr == nil {
		return errorEvent("floor system not available"), nil
	}
	if target == "all" {
		items := s.floorMgr.PickupAll(sess.RoomID)
		if len(items) == 0 {
			return messageEvent("There is nothing here to pick up."), nil
		}
		picked := 0
		for _, item := range items {
			_, err := sess.Backpack.Add(item.ItemDefID, item.Quantity, s.invRegistry)
			if err != nil {
				s.floorMgr.Drop(sess.RoomID, item)
				continue
			}
			picked++
		}
		return messageEvent(fmt.Sprintf("Picked up %d item(s).", picked)), nil
	}

	floorItems := s.floorMgr.ItemsInRoom(sess.RoomID)
	for _, item := range floorItems {
		name := item.ItemDefID
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(item.ItemDefID); ok {
				name = def.Name
			}
		}
		if strings.EqualFold(name, target) || strings.EqualFold(item.ItemDefID, target) {
			picked, ok := s.floorMgr.Pickup(sess.RoomID, item.InstanceID)
			if !ok {
				return errorEvent("Item is no longer there."), nil
			}
			_, err := sess.Backpack.Add(picked.ItemDefID, picked.Quantity, s.invRegistry)
			if err != nil {
				s.floorMgr.Drop(sess.RoomID, picked)
				return errorEvent(fmt.Sprintf("Cannot pick up: %v", err)), nil
			}
			return messageEvent(fmt.Sprintf("You pick up %s.", name)), nil
		}
	}
	return messageEvent("You don't see that here."), nil
}

// handleDropItem drops an item from the player's backpack to the room floor.
//
// Precondition: uid must be a valid connected player; target is the item name.
// Postcondition: On success the item is moved from backpack to floor; returns a message event.
func (s *GameServiceServer) handleDropItem(uid, target string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if s.floorMgr == nil {
		return errorEvent("floor system not available"), nil
	}
	for _, inst := range sess.Backpack.Items() {
		name := inst.ItemDefID
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(inst.ItemDefID); ok {
				name = def.Name
			}
		}
		if strings.EqualFold(name, target) || strings.EqualFold(inst.ItemDefID, target) {
			if err := sess.Backpack.Remove(inst.InstanceID, inst.Quantity); err != nil {
				return errorEvent(fmt.Sprintf("Cannot drop: %v", err)), nil
			}
			s.floorMgr.Drop(sess.RoomID, inst)
			return messageEvent(fmt.Sprintf("You drop %s.", name)), nil
		}
	}
	return messageEvent("You don't have that."), nil
}
