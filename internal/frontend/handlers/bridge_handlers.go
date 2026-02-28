package handlers

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// bridgeContext carries all inputs a bridge handler needs.
type bridgeContext struct {
	reqID    string
	cmd      *command.Command
	parsed   command.ParseResult
	conn     *telnet.Conn
	charName string
	role     string
	stream   gamev1.GameService_SessionClient
}

// bridgeResult is returned by every bridge handler.
// msg is the ClientMessage to send (nil if nothing to send).
// done is true when the handler dealt with output locally and the loop should continue.
// quit is true when the handler has completed a clean disconnect and commandLoop should return nil.
type bridgeResult struct {
	msg  *gamev1.ClientMessage
	done bool
	quit bool
}

// bridgeHandlerFunc is the signature for all bridge dispatch functions.
type bridgeHandlerFunc func(bctx *bridgeContext) (bridgeResult, error)

// BridgeHandlers returns the map from Handler constant to bridge function.
// Exported so TestAllCommandHandlersAreWired can verify completeness.
func BridgeHandlers() map[string]bridgeHandlerFunc {
	return bridgeHandlerMap
}

// bridgeHandlerMap is the single source of truth for frontend command dispatch.
// To add a new command: add a Handler constant to commands.go AND add an entry here.
var bridgeHandlerMap = map[string]bridgeHandlerFunc{
	command.HandlerMove:      bridgeMove,
	command.HandlerLook:      bridgeLook,
	command.HandlerExits:     bridgeExits,
	command.HandlerSay:       bridgeSay,
	command.HandlerEmote:     bridgeEmote,
	command.HandlerWho:       bridgeWho,
	command.HandlerQuit:      bridgeQuit,
	command.HandlerHelp:      bridgeHelp,
	command.HandlerExamine:   bridgeExamine,
	command.HandlerAttack:    bridgeAttack,
	command.HandlerFlee:      bridgeFlee,
	command.HandlerPass:      bridgePass,
	command.HandlerStrike:    bridgeStrike,
	command.HandlerStatus:    bridgeStatus,
	command.HandlerEquip:     bridgeEquip,
	command.HandlerReload:    bridgeReload,
	command.HandlerFireBurst: bridgeFireBurst,
	command.HandlerFireAuto:  bridgeFireAuto,
	command.HandlerThrow:     bridgeThrow,
	command.HandlerInventory: bridgeInventory,
	command.HandlerGet:       bridgeGet,
	command.HandlerDrop:      bridgeDrop,
	command.HandlerBalance:   bridgeBalance,
	command.HandlerSetRole:   bridgeSetRole,
	command.HandlerTeleport:  bridgeTeleport,
	command.HandlerLoadout:   bridgeLoadout,
	command.HandlerUnequip:   bridgeUnequip,
	command.HandlerEquipment: bridgeEquipment,
}

// writeErrorPrompt writes a red error message and re-issues the prompt, returning done=true.
func writeErrorPrompt(bctx *bridgeContext, msg string) (bridgeResult, error) {
	_ = bctx.conn.WriteLine(telnet.Colorize(telnet.Red, msg))
	_ = bctx.conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", bctx.charName))
	return bridgeResult{done: true}, nil
}

func bridgeMove(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: buildMoveMessage(bctx.reqID, bctx.cmd.Name)}, nil

}

func bridgeLook(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}},
	}}, nil
}

func bridgeExits(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Exits{Exits: &gamev1.ExitsRequest{}},
	}}, nil
}

func bridgeSay(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Say what?")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Say{Say: &gamev1.SayRequest{Message: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeEmote(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Emote what?")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Emote{Emote: &gamev1.EmoteRequest{Action: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeWho(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Who{Who: &gamev1.WhoRequest{}},
	}}, nil
}

func bridgeQuit(bctx *bridgeContext) (bridgeResult, error) {
	_ = bctx.conn.WriteLine(telnet.Colorize(telnet.Cyan, "The rain swallows your footsteps. Goodbye."))
	msg := &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}},
	}
	_ = bctx.stream.Send(msg)
	return bridgeResult{quit: true}, nil
}

// bridgeHelp signals that help is handled locally by commandLoop (no server round-trip).
func bridgeHelp(_ *bridgeContext) (bridgeResult, error) {
	return bridgeResult{done: true}, nil
}

func bridgeExamine(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: examine <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Examine{Examine: &gamev1.ExamineRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeAttack(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: attack <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Attack{Attack: &gamev1.AttackRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeFlee(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Flee{Flee: &gamev1.FleeRequest{}},
	}}, nil
}

func bridgePass(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}},
	}}, nil
}

func bridgeStrike(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: strike <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Strike{Strike: &gamev1.StrikeRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeStatus(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}},
	}}, nil
}

func bridgeEquip(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: equip <weapon_id> [slot]")
	}
	parts := strings.SplitN(bctx.parsed.RawArgs, " ", 2)
	slot := ""
	if len(parts) == 2 {
		slot = strings.TrimSpace(parts[1])
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Equip{Equip: &gamev1.EquipRequest{WeaponId: strings.TrimSpace(parts[0]), Slot: slot}},
	}}, nil
}

func bridgeReload(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Reload{Reload: &gamev1.ReloadRequest{WeaponId: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeFireBurst(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: burst <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FireBurst{FireBurst: &gamev1.FireBurstRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeFireAuto(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: auto <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FireAutomatic{FireAutomatic: &gamev1.FireAutomaticRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeThrow(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: throw <explosive_id>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Throw{Throw: &gamev1.ThrowRequest{ExplosiveId: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeInventory(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_InventoryReq{InventoryReq: &gamev1.InventoryRequest{}},
	}}, nil
}

func bridgeGet(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: get <item>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_GetItem{GetItem: &gamev1.GetItemRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeDrop(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: drop <item>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_DropItem{DropItem: &gamev1.DropItemRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeBalance(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Balance{Balance: &gamev1.BalanceRequest{}},
	}}, nil
}

func bridgeSetRole(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) < 2 {
		_ = bctx.conn.WriteLine("Usage: setrole <username> <role>")
		_ = bctx.conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", bctx.charName))
		return bridgeResult{done: true}, nil
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_SetRole{SetRole: &gamev1.SetRoleRequest{
			TargetUsername: bctx.parsed.Args[0],
			Role:           bctx.parsed.Args[1],
		}},
	}}, nil
}

func bridgeTeleport(bctx *bridgeContext) (bridgeResult, error) {
	targetChar := strings.TrimSpace(bctx.parsed.RawArgs)
	if targetChar == "" {
		_ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Character name: "))
		line, err := bctx.conn.ReadLine()
		if err != nil {
			return bridgeResult{}, fmt.Errorf("reading teleport target: %w", err)
		}
		targetChar = strings.TrimSpace(line)
	}
	if targetChar == "" {
		return writeErrorPrompt(bctx, "Character name cannot be empty.")
	}
	_ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Room ID: "))
	line, err := bctx.conn.ReadLine()
	if err != nil {
		return bridgeResult{}, fmt.Errorf("reading teleport room: %w", err)
	}
	roomID := strings.TrimSpace(line)
	if roomID == "" {
		return writeErrorPrompt(bctx, "Room ID cannot be empty.")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Teleport{Teleport: &gamev1.TeleportRequest{
			TargetCharacter: targetChar,
			RoomId:          roomID,
		}},
	}}, nil
}

func bridgeLoadout(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Loadout{Loadout: &gamev1.LoadoutRequest{Arg: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeUnequip(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: unequip <slot>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Unequip{Unequip: &gamev1.UnequipRequest{Slot: bctx.parsed.RawArgs}},
	}}, nil
}

func bridgeEquipment(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Equipment{Equipment: &gamev1.EquipmentRequest{}},
	}}, nil
}
