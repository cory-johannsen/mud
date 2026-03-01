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
	helpFn   func() // called by bridgeHelp to render help output
}

// bridgeResult is returned by every bridge handler.
// msg is the ClientMessage to send (nil if nothing to send).
// done is true when the handler dealt with output locally and the loop should continue.
// quit is true when the handler has completed a clean disconnect and commandLoop should return nil.
// switchCharacter is true when the handler signals gameBridge to return ErrSwitchCharacter.
type bridgeResult struct {
	msg             *gamev1.ClientMessage
	done            bool
	quit            bool
	switchCharacter bool
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
	command.HandlerSwitch:    bridgeSwitch,
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
// Precondition: bctx must be non-nil with a valid conn and charName; msg must be non-empty.
// Postcondition: writes msg in red and the prompt, then returns done=true with nil error.
func writeErrorPrompt(bctx *bridgeContext, msg string) (bridgeResult, error) {
	_ = bctx.conn.WriteLine(telnet.Colorize(telnet.Red, msg))
	_ = bctx.conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", bctx.charName))
	return bridgeResult{done: true}, nil
}

// bridgeMove builds a MoveRequest for the named direction.
// Precondition: bctx must be non-nil with a valid reqID and cmd.Name.
// Postcondition: returns a non-nil msg containing a MoveRequest; done is false.
func bridgeMove(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: buildMoveMessage(bctx.reqID, bctx.cmd.Name)}, nil

}

// bridgeLook builds a LookRequest for the current room.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: returns a non-nil msg containing a LookRequest; done is false.
func bridgeLook(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}},
	}}, nil
}

// bridgeExits builds an ExitsRequest for the current room.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an ExitsRequest; done is false.
func bridgeExits(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Exits{Exits: &gamev1.ExitsRequest{}},
	}}, nil
}

// bridgeSay builds a SayRequest.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a SayRequest.
func bridgeSay(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Say what?")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Say{Say: &gamev1.SayRequest{Message: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeEmote builds an EmoteRequest.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing an EmoteRequest.
func bridgeEmote(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Emote what?")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Emote{Emote: &gamev1.EmoteRequest{Action: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeWho builds a WhoRequest to list online players.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a WhoRequest; done is false.
func bridgeWho(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Who{Who: &gamev1.WhoRequest{}},
	}}, nil
}

// bridgeQuit sends a farewell message and a QuitRequest, then signals a clean disconnect.
// Precondition: bctx must be non-nil with a valid conn, stream, and reqID.
// Postcondition: writes goodbye text, sends QuitRequest on the stream, and returns quit=true.
func bridgeQuit(bctx *bridgeContext) (bridgeResult, error) {
	_ = bctx.conn.WriteLine(telnet.Colorize(telnet.Cyan, "The rain swallows your footsteps. Goodbye."))
	msg := &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}},
	}
	_ = bctx.stream.Send(msg)
	return bridgeResult{quit: true}, nil
}

// bridgeSwitch sends a SwitchCharacterRequest and signals the command loop to return
// to the character selection screen.
//
// Precondition: bctx must be non-nil.
// Postcondition: Returns switchCharacter=true so gameBridge returns ErrSwitchCharacter.
func bridgeSwitch(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{
		msg: &gamev1.ClientMessage{
			RequestId: bctx.reqID,
			Payload:   &gamev1.ClientMessage_SwitchCharacter{SwitchCharacter: &gamev1.SwitchCharacterRequest{}},
		},
		switchCharacter: true,
	}, nil
}

// bridgeHelp renders in-game help by invoking bctx.helpFn.
// Precondition: bctx must be non-nil; helpFn may be nil (in which case no output is produced).
// Postcondition: returns done=true so commandLoop continues without a server round-trip.
func bridgeHelp(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.helpFn != nil {
		bctx.helpFn()
	}
	return bridgeResult{done: true}, nil
}

// bridgeExamine builds an ExamineRequest for the named target.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing an ExamineRequest.
func bridgeExamine(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: examine <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Examine{Examine: &gamev1.ExamineRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeAttack builds an AttackRequest for the named target.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing an AttackRequest.
func bridgeAttack(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: attack <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Attack{Attack: &gamev1.AttackRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeFlee builds a FleeRequest to escape combat.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a FleeRequest; done is false.
func bridgeFlee(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Flee{Flee: &gamev1.FleeRequest{}},
	}}, nil
}

// bridgePass builds a PassRequest to skip a combat round.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a PassRequest; done is false.
func bridgePass(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}},
	}}, nil
}

// bridgeStrike builds a StrikeRequest for the named target.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a StrikeRequest.
func bridgeStrike(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: strike <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Strike{Strike: &gamev1.StrikeRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeStatus builds a StatusRequest to display the player's character status.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a StatusRequest; done is false.
func bridgeStatus(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}},
	}}, nil
}

// bridgeEquip builds an EquipRequest with an optional slot argument.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing an EquipRequest.
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

// bridgeReload builds a ReloadRequest for the specified weapon.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a ReloadRequest; done is false.
func bridgeReload(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Reload{Reload: &gamev1.ReloadRequest{WeaponId: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeFireBurst builds a FireBurstRequest for the named target.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a FireBurstRequest.
func bridgeFireBurst(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: burst <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FireBurst{FireBurst: &gamev1.FireBurstRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeFireAuto builds a FireAutomaticRequest for the named target.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a FireAutomaticRequest.
func bridgeFireAuto(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: auto <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FireAutomatic{FireAutomatic: &gamev1.FireAutomaticRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeThrow builds a ThrowRequest for the named explosive.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a ThrowRequest.
func bridgeThrow(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: throw <explosive_id>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Throw{Throw: &gamev1.ThrowRequest{ExplosiveId: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeInventory builds an InventoryRequest to display the player's inventory.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an InventoryRequest; done is false.
func bridgeInventory(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_InventoryReq{InventoryReq: &gamev1.InventoryRequest{}},
	}}, nil
}

// bridgeGet builds a GetItemRequest for the named item.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a GetItemRequest.
func bridgeGet(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: get <item>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_GetItem{GetItem: &gamev1.GetItemRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeDrop builds a DropItemRequest for the named item.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a DropItemRequest.
func bridgeDrop(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: drop <item>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_DropItem{DropItem: &gamev1.DropItemRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeBalance builds a BalanceRequest to query the player's currency balance.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a BalanceRequest; done is false.
func bridgeBalance(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Balance{Balance: &gamev1.BalanceRequest{}},
	}}, nil
}

// bridgeSetRole builds a SetRoleRequest for the target username.
// Precondition: bctx must be non-nil with a valid conn and reqID; caller must hold admin role.
// Postcondition: if fewer than 2 args are present, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a SetRoleRequest.
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

// bridgeTeleport builds a TeleportRequest, prompting for character name and room ID if needed.
// Precondition: bctx must be non-nil with a valid conn and reqID; caller must hold admin role.
// Postcondition: if either character name or room ID is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a TeleportRequest.
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

// bridgeLoadout builds a LoadoutRequest, passing any optional arg to the server.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a LoadoutRequest; done is false.
func bridgeLoadout(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Loadout{Loadout: &gamev1.LoadoutRequest{Arg: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeUnequip builds an UnequipRequest for the named equipment slot.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing an UnequipRequest.
func bridgeUnequip(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: unequip <slot>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Unequip{Unequip: &gamev1.UnequipRequest{Slot: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeEquipment builds an EquipmentRequest to display all equipped items.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an EquipmentRequest; done is false.
func bridgeEquipment(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Equipment{Equipment: &gamev1.EquipmentRequest{}},
	}}, nil
}
