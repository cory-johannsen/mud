package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// buildClientMessageFromText converts a raw text command to a ClientMessage.
func buildClientMessageFromText(reqID, text string, registry *command.Registry) (*gamev1.ClientMessage, error) {
	parsed := command.Parse(strings.TrimSpace(text))
	if parsed.Command == "" {
		return nil, nil
	}
	cmd, ok := registry.Resolve(parsed.Command)
	if !ok {
		return buildMoveClientMessage(reqID, parsed.Command), nil
	}
	bctx := &webBridgeContext{reqID: reqID, cmd: cmd, parsed: parsed}
	return buildMessageFromCommand(bctx)
}

func buildMoveClientMessage(reqID, direction string) *gamev1.ClientMessage {
	return &gamev1.ClientMessage{
		RequestId: reqID,
		Payload: &gamev1.ClientMessage_Move{
			Move: &gamev1.MoveRequest{Direction: direction},
		},
	}
}

type webBridgeContext struct {
	reqID  string
	cmd    *command.Command
	parsed command.ParseResult
}

func buildMessageFromCommand(bctx *webBridgeContext) (*gamev1.ClientMessage, error) {
	reqID := bctx.reqID
	parsed := bctx.parsed
	arg := ""
	if len(parsed.Args) > 0 {
		arg = parsed.Args[0]
	}
	rawArgs := parsed.RawArgs

	switch bctx.cmd.Handler {
	case command.HandlerMove:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Move{Move: &gamev1.MoveRequest{Direction: parsed.Command}}}, nil
	case command.HandlerLook:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}}}, nil
	case command.HandlerSay:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Say{Say: &gamev1.SayRequest{Message: rawArgs}}}, nil
	case command.HandlerEmote:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Emote{Emote: &gamev1.EmoteRequest{Action: rawArgs}}}, nil
	case command.HandlerExits:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Exits{Exits: &gamev1.ExitsRequest{}}}, nil
	case command.HandlerWho:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Who{Who: &gamev1.WhoRequest{}}}, nil
	case command.HandlerQuit:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}}}, nil
	case command.HandlerExamine:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Examine{Examine: &gamev1.ExamineRequest{Target: rawArgs}}}, nil
	case command.HandlerAttack:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Attack{Attack: &gamev1.AttackRequest{Target: arg}}}, nil
	case command.HandlerFlee:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Flee{Flee: &gamev1.FleeRequest{}}}, nil
	case command.HandlerPass:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}}}, nil
	case command.HandlerStrike:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Strike{Strike: &gamev1.StrikeRequest{Target: arg}}}, nil
	case command.HandlerStatus:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}}}, nil
	case command.HandlerInventory:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_InventoryReq{InventoryReq: &gamev1.InventoryRequest{}}}, nil
	case command.HandlerMap:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Map{Map: &gamev1.MapRequest{}}}, nil
	case command.HandlerSkills:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SkillsRequest{SkillsRequest: &gamev1.SkillsRequest{}}}, nil
	case command.HandlerFeats:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_FeatsRequest{FeatsRequest: &gamev1.FeatsRequest{}}}, nil
	case command.HandlerChar:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_CharSheet{CharSheet: &gamev1.CharacterSheetRequest{}}}, nil
	case command.HandlerRest:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Rest{Rest: &gamev1.RestRequest{}}}, nil
	default:
		return nil, fmt.Errorf("handler %q not supported in web client dispatch", bctx.cmd.Handler)
	}
}

func protoMessageByName(name string) (proto.Message, error) {
	typeMap := map[string]func() proto.Message{
		"MoveRequest":           func() proto.Message { return &gamev1.MoveRequest{} },
		"LookRequest":           func() proto.Message { return &gamev1.LookRequest{} },
		"SayRequest":            func() proto.Message { return &gamev1.SayRequest{} },
		"EmoteRequest":          func() proto.Message { return &gamev1.EmoteRequest{} },
		"AttackRequest":         func() proto.Message { return &gamev1.AttackRequest{} },
		"FleeRequest":           func() proto.Message { return &gamev1.FleeRequest{} },
		"ExamineRequest":        func() proto.Message { return &gamev1.ExamineRequest{} },
		"ExitsRequest":          func() proto.Message { return &gamev1.ExitsRequest{} },
		"WhoRequest":            func() proto.Message { return &gamev1.WhoRequest{} },
		"QuitRequest":           func() proto.Message { return &gamev1.QuitRequest{} },
		"PassRequest":           func() proto.Message { return &gamev1.PassRequest{} },
		"StrikeRequest":         func() proto.Message { return &gamev1.StrikeRequest{} },
		"StatusRequest":         func() proto.Message { return &gamev1.StatusRequest{} },
		"InventoryRequest":      func() proto.Message { return &gamev1.InventoryRequest{} },
		"MapRequest":            func() proto.Message { return &gamev1.MapRequest{} },
		"SkillsRequest":         func() proto.Message { return &gamev1.SkillsRequest{} },
		"FeatsRequest":          func() proto.Message { return &gamev1.FeatsRequest{} },
		"CharacterSheetRequest": func() proto.Message { return &gamev1.CharacterSheetRequest{} },
		"RestRequest":           func() proto.Message { return &gamev1.RestRequest{} },
		"HotbarRequest":         func() proto.Message { return &gamev1.HotbarRequest{} },
	}
	factory, ok := typeMap[name]
	if !ok {
		return nil, fmt.Errorf("unknown proto message name: %q", name)
	}
	return factory(), nil
}

func wrapProtoAsClientMessage(reqID, typeName string, msg proto.Message) (*gamev1.ClientMessage, error) {
	cm := &gamev1.ClientMessage{RequestId: reqID}
	switch m := msg.(type) {
	case *gamev1.MoveRequest:
		cm.Payload = &gamev1.ClientMessage_Move{Move: m}
	case *gamev1.LookRequest:
		cm.Payload = &gamev1.ClientMessage_Look{Look: m}
	case *gamev1.SayRequest:
		cm.Payload = &gamev1.ClientMessage_Say{Say: m}
	case *gamev1.EmoteRequest:
		cm.Payload = &gamev1.ClientMessage_Emote{Emote: m}
	case *gamev1.AttackRequest:
		cm.Payload = &gamev1.ClientMessage_Attack{Attack: m}
	case *gamev1.FleeRequest:
		cm.Payload = &gamev1.ClientMessage_Flee{Flee: m}
	case *gamev1.ExamineRequest:
		cm.Payload = &gamev1.ClientMessage_Examine{Examine: m}
	case *gamev1.ExitsRequest:
		cm.Payload = &gamev1.ClientMessage_Exits{Exits: m}
	case *gamev1.WhoRequest:
		cm.Payload = &gamev1.ClientMessage_Who{Who: m}
	case *gamev1.QuitRequest:
		cm.Payload = &gamev1.ClientMessage_Quit{Quit: m}
	case *gamev1.PassRequest:
		cm.Payload = &gamev1.ClientMessage_Pass{Pass: m}
	case *gamev1.StrikeRequest:
		cm.Payload = &gamev1.ClientMessage_Strike{Strike: m}
	case *gamev1.StatusRequest:
		cm.Payload = &gamev1.ClientMessage_Status{Status: m}
	case *gamev1.InventoryRequest:
		cm.Payload = &gamev1.ClientMessage_InventoryReq{InventoryReq: m}
	case *gamev1.MapRequest:
		cm.Payload = &gamev1.ClientMessage_Map{Map: m}
	case *gamev1.SkillsRequest:
		cm.Payload = &gamev1.ClientMessage_SkillsRequest{SkillsRequest: m}
	case *gamev1.FeatsRequest:
		cm.Payload = &gamev1.ClientMessage_FeatsRequest{FeatsRequest: m}
	case *gamev1.CharacterSheetRequest:
		cm.Payload = &gamev1.ClientMessage_CharSheet{CharSheet: m}
	case *gamev1.RestRequest:
		cm.Payload = &gamev1.ClientMessage_Rest{Rest: m}
	case *gamev1.HotbarRequest:
		cm.Payload = &gamev1.ClientMessage_HotbarRequest{HotbarRequest: m}
	default:
		return nil, fmt.Errorf("no ClientMessage oneof for type %q", typeName)
	}
	return cm, nil
}

// dispatchWSMessage converts a wsMessage envelope into a ClientMessage proto (REQ-WC-30).
//
// Precondition: env.Type must be a known message type or "CommandText".
// Postcondition: Returns a ClientMessage or nil (empty command), or an error for unknown types.
func dispatchWSMessage(env wsMessage, reqID string, registry *command.Registry) (*gamev1.ClientMessage, error) {
	if env.Type == "CommandText" {
		var body struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(env.Payload, &body); err != nil {
			return nil, fmt.Errorf("parsing CommandText payload: %w", err)
		}
		return buildClientMessageFromText(reqID, body.Text, registry)
	}
	msg, err := protoMessageByName(env.Type)
	if err != nil {
		return nil, fmt.Errorf("unknown message type %q: %w", env.Type, err)
	}
	if err := protojson.Unmarshal(env.Payload, msg); err != nil {
		return nil, fmt.Errorf("unmarshalling %q: %w", env.Type, err)
	}
	return wrapProtoAsClientMessage(reqID, env.Type, msg)
}
