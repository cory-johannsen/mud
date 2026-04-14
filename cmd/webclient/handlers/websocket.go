package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/cmd/webclient/eventbus"
	"github.com/cory-johannsen/mud/cmd/webclient/session"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// GameDialer opens a gRPC session stream.
type GameDialer interface {
	Session(ctx context.Context, opts ...grpc.CallOption) (gamev1.GameService_SessionClient, error)
}

// CharacterGetter loads a character by ID.
type CharacterGetter interface {
	GetByID(ctx context.Context, id int64) (*character.Character, error)
}

// AccountUsernameGetter resolves an account ID to a username.
type AccountUsernameGetter interface {
	GetUsernameByID(ctx context.Context, id int64) (string, error)
}

// wsClaims holds the JWT claims for a WebSocket session.
type wsClaims struct {
	AccountID   int64  `json:"account_id"`
	CharacterID int64  `json:"character_id"`
	Role        string `json:"role"`
	jwt.RegisteredClaims
}

// WSHandler handles GET /ws.
//
// Precondition: jwtSecret must be the same secret used when issuing tokens.
// Precondition: dialer must be a valid connected GameServiceClient (set at startup).
type WSHandler struct {
	jwtSecret     string
	dialer        GameDialer
	charGetter    CharacterGetter
	accountGetter AccountUsernameGetter    // may be nil; username falls back to synthetic "user_<id>"
	bus           *eventbus.EventBus       // may be nil; if set, server events are published
	registry      *ActiveCharacterRegistry // may be nil; if set, tracks active character sessions
	logger        *zap.Logger
}

// NewWSHandler creates a WSHandler.
//
// Precondition: jwtSecret must be non-empty; logger may be nil (falls back to zap.NewNop).
func NewWSHandler(jwtSecret string, dialer GameDialer, charGetter CharacterGetter) *WSHandler {
	return &WSHandler{
		jwtSecret:  jwtSecret,
		dialer:     dialer,
		charGetter: charGetter,
		logger:     zap.NewNop(),
	}
}

// WithAccountGetter attaches an AccountUsernameGetter so the WS handler can include the real
// account username in JoinWorldRequest (used in "who" lists and session management).
func (h *WSHandler) WithAccountGetter(ag AccountUsernameGetter) *WSHandler {
	h.accountGetter = ag
	return h
}

// WithEventBus attaches an EventBus; all received gRPC ServerEvents will be published.
func (h *WSHandler) WithEventBus(bus *eventbus.EventBus) *WSHandler {
	h.bus = bus
	return h
}

// WithLogger attaches a logger to the handler.
func (h *WSHandler) WithLogger(l *zap.Logger) *WSHandler {
	h.logger = l
	return h
}

// WithRegistry attaches an ActiveCharacterRegistry so the handler can track active sessions.
//
// Postcondition: Returns h for chaining.
func (h *WSHandler) WithRegistry(r *ActiveCharacterRegistry) *WSHandler {
	h.registry = r
	return h
}

// wsMessage is the JSON envelope for all WebSocket frames.
type wsMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// extractAndValidateJWT parses and validates a JWT from either the Authorization
// header or the ?token= query parameter.
//
// Precondition: r must not be nil; secret must be non-empty.
// Postcondition: Returns valid wsClaims or an error.
func extractAndValidateJWT(r *http.Request, secret string) (*wsClaims, error) {
	tokenStr := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tokenStr = strings.TrimPrefix(auth, "Bearer ")
	} else if q := r.URL.Query().Get("token"); q != "" {
		tokenStr = q
	}
	if tokenStr == "" {
		return nil, fmt.Errorf("no token provided")
	}
	claims := &wsClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	return claims, nil
}

// ServeHTTP implements http.Handler for GET /ws.
//
// Postcondition: Upgrades to WebSocket on valid JWT; returns 401 otherwise.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, err := extractAndValidateJWT(r, h.jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	characterID := claims.CharacterID
	if characterID == 0 {
		http.Error(w, `{"error":"character_id claim missing"}`, http.StatusUnauthorized)
		return
	}

	char, err := h.charGetter.GetByID(r.Context(), characterID)
	if err != nil {
		http.Error(w, `{"error":"character not found"}`, http.StatusUnauthorized)
		return
	}

	// Verify ownership.
	if char.AccountID != claims.AccountID {
		http.Error(w, `{"error":"character not owned by account"}`, http.StatusUnauthorized)
		return
	}

	wsConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}
	h.logger.Info("websocket connected", zap.Int64("character_id", characterID))

	// Register the character as active; deregister when the WebSocket session ends.
	if h.registry != nil {
		h.registry.Register(char.ID)
		defer h.registry.Deregister(char.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())

	stream, err := h.dialer.Session(ctx)
	if err != nil {
		cancel()
		_ = wsConn.Close()
		h.logger.Error("failed to open gRPC session stream", zap.Error(err))
		return
	}

	// Resolve account username; fall back to synthetic value if getter is unavailable.
	username := fmt.Sprintf("user_%d", claims.AccountID)
	if h.accountGetter != nil {
		if u, err := h.accountGetter.GetUsernameByID(r.Context(), claims.AccountID); err == nil && u != "" {
			username = u
		}
	}

	// Send JoinWorldRequest with all required fields.
	joinMsg := &gamev1.ClientMessage{
		RequestId: "join-0",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:           fmt.Sprintf("%d", char.ID),
				Username:      username,
				CharacterId:   char.ID,
				CharacterName: char.Name,
				CurrentHp:     int32(char.CurrentHP),
				Location:      char.Location,
				Role:          claims.Role,
				RegionDisplay: char.Region,
				Class:         char.Class,
				Level:         int32(char.Level),
				Archetype:     char.Team,
			},
		},
	}
	if err := stream.Send(joinMsg); err != nil {
		cancel()
		_ = wsConn.Close()
		h.logger.Error("failed to send JoinWorldRequest", zap.Error(err))
		return
	}

	sess := session.New(ctx, cancel, wsConn, stream)
	sess.Run()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer sess.Close(websocket.CloseNormalClosure, "")
		h.wsToGRPC(ctx, wsConn, stream)
	}()

	go func() {
		defer wg.Done()
		defer sess.Close(websocket.CloseGoingAway, "server disconnected")
		h.grpcToWS(ctx, stream, wsConn)
	}()

	wg.Wait()
	sess.Wait()
}

// wsToGRPC reads JSON frames from the WebSocket and forwards them as ClientMessage protos.
func (h *WSHandler) wsToGRPC(ctx context.Context, wsConn *websocket.Conn, stream gamev1.GameService_SessionClient) {
	registry := command.DefaultRegistry()
	requestID := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, data, err := wsConn.ReadMessage()
		if err != nil {
			return
		}
		var env wsMessage
		if err := json.Unmarshal(data, &env); err != nil {
			h.logger.Warn("invalid ws message envelope", zap.Error(err))
			continue
		}
		requestID++
		reqID := fmt.Sprintf("ws-%d", requestID)

		msg, err := dispatchWSMessage(env, reqID, registry)
		if err != nil {
			h.logger.Warn("dispatch failed", zap.String("type", env.Type), zap.Error(err))
			continue
		}
		if msg == nil {
			continue
		}
		if err := stream.Send(msg); err != nil {
			return
		}
	}
}

// serverEventInner unwraps a ServerEvent oneof and returns the inner proto.Message
// and its short type name (e.g. "RoomView", "MessageEvent").
// Returns nil, "" if the payload is unrecognised or nil.
func serverEventInner(event *gamev1.ServerEvent) (proto.Message, string) {
	switch p := event.Payload.(type) {
	case *gamev1.ServerEvent_RoomView:
		return p.RoomView, "RoomView"
	case *gamev1.ServerEvent_Message:
		return p.Message, "MessageEvent"
	case *gamev1.ServerEvent_RoomEvent:
		return p.RoomEvent, "RoomEvent"
	case *gamev1.ServerEvent_CharacterInfo:
		return p.CharacterInfo, "CharacterInfo"
	case *gamev1.ServerEvent_CharacterSheet:
		return p.CharacterSheet, "CharacterSheetView"
	case *gamev1.ServerEvent_InventoryView:
		return p.InventoryView, "InventoryView"
	case *gamev1.ServerEvent_Map:
		return p.Map, "MapResponse"
	case *gamev1.ServerEvent_CombatEvent:
		return p.CombatEvent, "CombatEvent"
	case *gamev1.ServerEvent_RoundStart:
		return p.RoundStart, "RoundStartEvent"
	case *gamev1.ServerEvent_RoundEnd:
		return p.RoundEnd, "RoundEndEvent"
	case *gamev1.ServerEvent_Error:
		return p.Error, "ErrorEvent"
	case *gamev1.ServerEvent_HpUpdate:
		return p.HpUpdate, "HpUpdate"
	case *gamev1.ServerEvent_Disconnected:
		return p.Disconnected, "Disconnected"
	case *gamev1.ServerEvent_PlayerList:
		return p.PlayerList, "PlayerList"
	case *gamev1.ServerEvent_ExitList:
		return p.ExitList, "ExitList"
	case *gamev1.ServerEvent_NpcView:
		return p.NpcView, "NpcView"
	case *gamev1.ServerEvent_ConditionEvent:
		return p.ConditionEvent, "ConditionEvent"
	case *gamev1.ServerEvent_TimeOfDay:
		return p.TimeOfDay, "TimeOfDay"
	case *gamev1.ServerEvent_HotbarUpdate:
		return p.HotbarUpdate, "HotbarUpdate"
	case *gamev1.ServerEvent_UseResponse:
		return p.UseResponse, "UseResponse"
	case *gamev1.ServerEvent_ShopView:
		return p.ShopView, "ShopView"
	case *gamev1.ServerEvent_HealerView:
		return p.HealerView, "HealerView"
	case *gamev1.ServerEvent_TrainerView:
		return p.TrainerView, "TrainerView"
	case *gamev1.ServerEvent_FixerView:
		return p.FixerView, "FixerView"
	case *gamev1.ServerEvent_LoadoutView:
		return p.LoadoutView, "LoadoutView"
	case *gamev1.ServerEvent_JobGrantsResponse:
		return p.JobGrantsResponse, "JobGrantsResponse"
	case *gamev1.ServerEvent_RestView:
		return p.RestView, "RestView"
	case *gamev1.ServerEvent_Weather:
		return p.Weather, "WeatherEvent"
	case *gamev1.ServerEvent_QuestGiverView:
		return p.QuestGiverView, "QuestGiverView"
	case *gamev1.ServerEvent_QuestLogView:
		return p.QuestLogView, "QuestLogView"
	case *gamev1.ServerEvent_QuestComplete:
		return p.QuestComplete, "QuestCompleteEvent"
	case *gamev1.ServerEvent_ApUpdate:
		return p.ApUpdate, "APUpdateEvent"
	default:
		return nil, ""
	}
}

// serverEventEncodedChoice checks if a MessageEvent carries a sentinel-encoded
// feature choice prompt and extracts the JSON bytes and type name if so.
//
// Precondition: event must not be nil.
// Postcondition: Returns non-nil bytes and "FeatureChoicePrompt" when the MessageEvent
// content begins with the "\x00choice\x00" sentinel; returns nil, "" otherwise.
func serverEventEncodedChoice(event *gamev1.ServerEvent) (json.RawMessage, string) {
	msg, ok := event.Payload.(*gamev1.ServerEvent_Message)
	if !ok || msg.Message == nil {
		return nil, ""
	}
	const sentinel = "\x00choice\x00"
	if !strings.HasPrefix(msg.Message.Content, sentinel) {
		return nil, ""
	}
	jsonStr := msg.Message.Content[len(sentinel):]
	return json.RawMessage(jsonStr), "FeatureChoicePrompt"
}

// serverEventEncodedLoadout checks if a MessageEvent carries a sentinel-encoded
// LoadoutView payload and extracts the JSON bytes and type name if so.
//
// Precondition: event must not be nil.
// Postcondition: Returns non-nil bytes and "LoadoutView" when the MessageEvent content
// begins with the "\x00loadout\x00" sentinel; returns nil, "" otherwise.
func serverEventEncodedLoadout(event *gamev1.ServerEvent) (json.RawMessage, string) {
	msg, ok := event.Payload.(*gamev1.ServerEvent_Message)
	if !ok || msg.Message == nil {
		return nil, ""
	}
	const sentinel = "\x00loadout\x00"
	if !strings.HasPrefix(msg.Message.Content, sentinel) {
		return nil, ""
	}
	jsonStr := msg.Message.Content[len(sentinel):]
	return json.RawMessage(jsonStr), "LoadoutView"
}

// grpcToWS reads ServerEvent protos from the gRPC stream and writes JSON frames to the WS.
// If h.bus is non-nil, each event is also published to the EventBus for SSE fan-out.
func (h *WSHandler) grpcToWS(ctx context.Context, stream gamev1.GameService_SessionClient, wsConn *websocket.Conn) {
	// EmitUnpopulated: true is required so that zero-value int32 fields (e.g. GridX=0, GridY=0)
	// are included in the JSON output. Without this, combatants at row 0 or column 0
	// have their coordinates silently dropped, making them invisible on the battle map.
	marshaler := protojson.MarshalOptions{EmitUnpopulated: true}
	for {
		event, err := stream.Recv()
		if err != nil {
			h.logger.Info("grpcToWS: stream.Recv error", zap.Error(err))
			return
		}
		// Handle sentinel-encoded FeatureChoicePrompt carried inside a MessageEvent.
		if rawPayload, msgName := serverEventEncodedChoice(event); rawPayload != nil {
			env := wsMessage{Type: msgName, Payload: rawPayload}
			frame, err := json.Marshal(env)
			if err != nil {
				h.logger.Error("failed to marshal FeatureChoicePrompt ws envelope", zap.Error(err))
				continue
			}
			if h.bus != nil {
				h.bus.Publish(eventbus.Event{Type: msgName, Payload: rawPayload, Time: time.Now()})
			}
			if err := wsConn.WriteMessage(websocket.TextMessage, frame); err != nil {
				h.logger.Info("grpcToWS: WebSocket write error", zap.String("type", msgName), zap.Error(err))
				return
			}
			continue
		}
		// Handle sentinel-encoded LoadoutView carried inside a MessageEvent.
		if rawPayload, msgName := serverEventEncodedLoadout(event); rawPayload != nil {
			env := wsMessage{Type: msgName, Payload: rawPayload}
			frame, err := json.Marshal(env)
			if err != nil {
				h.logger.Error("failed to marshal LoadoutView ws envelope", zap.Error(err))
				continue
			}
			if h.bus != nil {
				h.bus.Publish(eventbus.Event{Type: msgName, Payload: rawPayload, Time: time.Now()})
			}
			if err := wsConn.WriteMessage(websocket.TextMessage, frame); err != nil {
				h.logger.Info("grpcToWS: WebSocket write error", zap.String("type", msgName), zap.Error(err))
				return
			}
			continue
		}
		inner, msgName := serverEventInner(event)
		if inner == nil {
			h.logger.Warn("unrecognised ServerEvent payload; skipping")
			continue
		}
		payload, err := marshaler.Marshal(inner)
		if err != nil {
			h.logger.Error("failed to marshal ServerEvent inner", zap.String("type", msgName), zap.Error(err))
			continue
		}
		rawPayload := json.RawMessage(payload)

		// Fan-out to SSE subscribers via EventBus.
		if h.bus != nil {
			h.bus.Publish(eventbus.Event{
				Type:    msgName,
				Payload: rawPayload,
				Time:    time.Now(),
			})
		}

		env := wsMessage{
			Type:    msgName,
			Payload: rawPayload,
		}
		frame, err := json.Marshal(env)
		if err != nil {
			h.logger.Error("failed to marshal ws envelope", zap.Error(err))
			continue
		}
		if err := wsConn.WriteMessage(websocket.TextMessage, frame); err != nil {
			h.logger.Info("grpcToWS: WebSocket write error", zap.String("type", msgName), zap.Error(err))
			return
		}
	}
}
