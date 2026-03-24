package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// bridgeContext carries all inputs a bridge handler needs.
type bridgeContext struct {
	reqID          string
	cmd            *command.Command
	parsed         command.ParseResult
	conn           *telnet.Conn
	charName       string
	role           string
	stream         gamev1.GameService_SessionClient
	helpFn         func()        // called by bridgeHelp to render help output
	promptFn       func() string // called to build the current colored prompt
	travelResolver func(zoneName string) (zoneID string, errMsg string) // nil if not available; errMsg non-empty signals failure
}

// bridgeResult is returned by every bridge handler.
// msg is the ClientMessage to send (nil if nothing to send).
// done is true when the handler dealt with output locally and the loop should continue.
// quit is true when the handler has completed a clean disconnect and commandLoop should return nil.
// switchCharacter is true when the handler signals gameBridge to return ErrSwitchCharacter.
// enterMapMode is true when the handler requests the command loop to enter map mode.
// mapView is the map view type ("zone" or "world") when enterMapMode is true.
type bridgeResult struct {
	msg             *gamev1.ClientMessage
	done            bool
	quit            bool
	switchCharacter bool
	enterMapMode    bool
	mapView         string
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
	command.HandlerMove:               bridgeMove,
	command.HandlerLook:               bridgeLook,
	command.HandlerExits:              bridgeExits,
	command.HandlerSay:                bridgeSay,
	command.HandlerEmote:              bridgeEmote,
	command.HandlerWho:                bridgeWho,
	command.HandlerQuit:               bridgeQuit,
	command.HandlerSwitch:             bridgeSwitch,
	command.HandlerHelp:               bridgeHelp,
	command.HandlerExamine:            bridgeExamine,
	command.HandlerAttack:             bridgeAttack,
	command.HandlerFlee:               bridgeFlee,
	command.HandlerPass:               bridgePass,
	command.HandlerStrike:             bridgeStrike,
	command.HandlerStatus:             bridgeStatus,
	command.HandlerEquip:              bridgeEquip,
	command.HandlerReload:             bridgeReload,
	command.HandlerFireBurst:          bridgeFireBurst,
	command.HandlerFireAuto:           bridgeFireAuto,
	command.HandlerThrow:              bridgeThrow,
	command.HandlerInventory:          bridgeInventory,
	command.HandlerGet:                bridgeGet,
	command.HandlerDrop:               bridgeDrop,
	command.HandlerBalance:            bridgeBalance,
	command.HandlerSetRole:            bridgeSetRole,
	command.HandlerTeleport:           bridgeTeleport,
	command.HandlerLoadout:            bridgeLoadout,
	command.HandlerUnequip:            bridgeUnequip,
	command.HandlerEquipment:          bridgeEquipment,
	command.HandlerWear:               bridgeWear,
	command.HandlerRemoveArmor:        bridgeRemoveArmor,
	command.HandlerChar:               bridgeChar,
	command.HandlerArchetypeSelection: bridgeArchetypeSelection,
	command.HandlerUseEquipment:       bridgeUseEquipment,
	command.HandlerRoomEquip:          bridgeRoomEquip,
	command.HandlerMap:                bridgeMap,
	command.HandlerTravel:             bridgeTravel,
	command.HandlerSkills:             bridgeSkills,
	command.HandlerFeats:              bridgeFeats,
	command.HandlerClassFeatures:      bridgeClassFeatures,
	command.HandlerInteract:           bridgeInteract,
	command.HandlerUse:                bridgeUse,
	command.HandlerSummonItem:         bridgeSummonItem,
	command.HandlerProficiencies:      bridgeProficiencies,
	command.HandlerLevelUp:            bridgeLevelUp,
	command.HandlerCombatDefault:      bridgeCombatDefault,
	command.HandlerTrainSkill:         bridgeTrainSkill,
	command.HandlerAction:             bridgeAction,
	command.HandlerRaiseShield:        bridgeRaiseShield,
	command.HandlerTakeCover:          bridgeTakeCover,
	command.HandlerFirstAid:           bridgeFirstAid,
	command.HandlerFeint:              bridgeFeint,
	command.HandlerDemoralize:         bridgeDemoralize,
	command.HandlerGrapple:            bridgeGrapple,
	command.HandlerTrip:               bridgeTrip,
	command.HandlerDelay:              bridgeDelay,
	command.HandlerDisarm:             bridgeDisarm,
	command.HandlerDisarmTrap:         bridgeDisarmTrap,
	command.HandlerDeployTrap:         bridgeDeployTrap,
	command.HandlerReady:              bridgeReady,
	command.HandlerClimb:              bridgeClimb,
	command.HandlerStride:             bridgeStride,
	command.HandlerHide:               bridgeHide,
	command.HandlerSneak:              bridgeSneak,
	command.HandlerSwim:               bridgeSwim,
	command.HandlerDivert:             bridgeDivert,
	command.HandlerEscape:             bridgeEscape,
	command.HandlerGrant:              bridgeGrant,
	command.HandlerShove:              bridgeShove,
	command.HandlerStep:               bridgeStep,
	command.HandlerTumble:             bridgeTumble,
	command.HandlerSeek:               bridgeSeek,
	command.HandlerMotive:             bridgeMotive,
	command.HandlerCalm:               bridgeCalm,
	command.HandlerHeroPoint:          bridgeHeroPoint,
	command.HandlerJoin:               bridgeJoin,
	command.HandlerDecline:            bridgeDecline,
	command.HandlerGroup:              bridgeGroup,
	command.HandlerInvite:             bridgeInvite,
	command.HandlerAcceptGroup:        bridgeAcceptGroup,
	command.HandlerDeclineGroup:       bridgeDeclineGroup,
	command.HandlerUngroup:            bridgeUngroup,
	command.HandlerKick:               bridgeKick,
	command.HandlerRest:               bridgeRest,
	command.HandlerSelectTech:         bridgeSelectTech,
	command.HandlerAid:                bridgeAid,
	command.HandlerTalk:               bridgeTalk,
	command.HandlerBribe:              bridgeBribe,
	command.HandlerSurrender:          bridgeSurrender,
	command.HandlerRelease:            bridgeRelease,
	command.HandlerFaction:            bridgeFaction,
	command.HandlerFactionInfo:        bridgeFactionInfo,
	command.HandlerFactionStanding:    bridgeFactionStanding,
	command.HandlerChangeRep:          bridgeChangeRep,
	command.HandlerHeal:               bridgeHeal,
	command.HandlerBrowse:             bridgeBrowse,
	command.HandlerBuy:                bridgeBuy,
	command.HandlerSell:               bridgeSell,
	command.HandlerNegotiate:          bridgeNegotiate,
	command.HandlerDeposit:            bridgeDeposit,
	command.HandlerWithdraw:           bridgeWithdraw,
	command.HandlerStashBalance:       bridgeStashBalance,
	command.HandlerHire:               bridgeHire,
	command.HandlerDismiss:            bridgeDismiss,
	command.HandlerTrainJob:           bridgeTrainJob,
	command.HandlerListJobs:           bridgeListJobs,
	command.HandlerSetJob:             bridgeSetJob,
	command.HandlerSpawnNPC:           bridgeSpawnNPC,
	command.HandlerAddRoom:            bridgeAddRoom,
	command.HandlerAddLink:            bridgeAddLink,
	command.HandlerRemoveLink:         bridgeRemoveLink,
	command.HandlerSetRoom:            bridgeSetRoom,
	command.HandlerEditorCmds:         bridgeEditorCmds,
	command.HandlerTabComplete:        bridgeTabComplete,
}

// writeErrorPrompt writes a red error message and re-issues the prompt, returning done=true.
// Precondition: bctx must be non-nil with a valid conn and charName; msg must be non-empty.
// Postcondition: writes msg in red and the prompt, then returns done=true with nil error.
func writeErrorPrompt(bctx *bridgeContext, msg string) (bridgeResult, error) {
	colored := telnet.Colorize(telnet.Red, msg)
	if bctx.conn != nil {
		if bctx.conn.IsSplitScreen() {
			_ = bctx.conn.WriteConsole(colored)
			_ = bctx.conn.WritePromptSplit(bctx.promptFn())
		} else {
			_ = bctx.conn.WriteLine(colored)
			_ = bctx.conn.WritePrompt(bctx.promptFn())
		}
	}
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
	goodbye := telnet.Colorize(telnet.Cyan, "The rain swallows your footsteps. Goodbye.")
	if bctx.conn.IsSplitScreen() {
		_ = bctx.conn.WriteConsole(goodbye)
	} else {
		_ = bctx.conn.WriteLine(goodbye)
	}
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
		if bctx.conn.IsSplitScreen() {
			_ = bctx.conn.WriteConsole("Usage: setrole <username> <role>")
			_ = bctx.conn.WritePromptSplit(bctx.promptFn())
		} else {
			_ = bctx.conn.WriteLine("Usage: setrole <username> <role>")
			_ = bctx.conn.WritePrompt(bctx.promptFn())
		}
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

// bridgeWear builds a WearRequest to equip an armor item from inventory into a body slot.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if fewer than 2 args are present, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a WearRequest.
func bridgeWear(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) < 2 {
		return writeErrorPrompt(bctx, "Usage: wear <item_id> <slot>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Wear{Wear: &gamev1.WearRequest{
			ItemId: bctx.parsed.Args[0],
			Slot:   bctx.parsed.Args[1],
		}},
	}}, nil
}

// bridgeChar builds a CharacterSheetRequest to display the player's full character sheet.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a CharacterSheetRequest; done is false.
func bridgeChar(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_CharSheet{CharSheet: &gamev1.CharacterSheetRequest{}},
	}}, nil
}

// bridgeArchetypeSelection sends an ArchetypeSelectionRequest to the game server.
// Archetype selection occurs during character creation flow; this bridge satisfies CMD-5 wiring.
//
// Precondition: bctx must be non-nil.
// Postcondition: Returns a ClientMessage wrapping ArchetypeSelectionRequest, or done=true if no args.
func bridgeArchetypeSelection(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) == 0 {
		return bridgeResult{done: true}, nil
	}
	return bridgeResult{
		msg: &gamev1.ClientMessage{
			RequestId: bctx.reqID,
			Payload: &gamev1.ClientMessage_ArchetypeSelection{
				ArchetypeSelection: &gamev1.ArchetypeSelectionRequest{
					ArchetypeId: bctx.parsed.Args[0],
				},
			},
		},
	}, nil
}

// bridgeUseEquipment builds a UseEquipmentRequest for the named item instance ID.
//
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a UseEquipmentRequest.
func bridgeUseEquipment(bctx *bridgeContext) (bridgeResult, error) {
	instanceID := strings.TrimSpace(bctx.parsed.RawArgs)
	if instanceID == "" {
		return writeErrorPrompt(bctx, "Usage: use <instance_id>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_UseEquipment{UseEquipment: &gamev1.UseEquipmentRequest{InstanceId: instanceID}},
	}}, nil
}

// bridgeRemoveArmor builds a RemoveArmorRequest for the named armor slot.
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
//
//	otherwise returns a non-nil msg containing a RemoveArmorRequest.
func bridgeRemoveArmor(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: remove <slot>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_RemoveArmor{RemoveArmor: &gamev1.RemoveArmorRequest{Slot: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeMap builds a MapRequest to retrieve the automap.
// If the argument is "world", requests the world view and enters map mode.
// Otherwise requests the zone view and enters map mode.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a MapRequest and enterMapMode=true.
func bridgeMap(bctx *bridgeContext) (bridgeResult, error) {
	view := "zone"
	arg := strings.TrimSpace(strings.ToLower(bctx.parsed.RawArgs))
	if arg == "world" {
		view = "world"
	}
	return bridgeResult{
		enterMapMode: true,
		mapView:      view,
		msg: &gamev1.ClientMessage{
			RequestId: bctx.reqID,
			Payload:   &gamev1.ClientMessage_Map{Map: &gamev1.MapRequest{View: view}},
		},
	}, nil
}

// bridgeTravel sends a TravelRequest to fast-travel to a named zone.
//
// Precondition: bctx.parsed.RawArgs is the zone name argument (may be empty).
// Postcondition: Returns done=true with local error message on validation failure,
// or msg non-nil to send TravelRequest.
func bridgeTravel(bctx *bridgeContext) (bridgeResult, error) {
	arg := strings.TrimSpace(bctx.parsed.RawArgs)
	if arg == "" {
		usageMsg := "Usage: travel <zone name>"
		if bctx.conn.IsSplitScreen() {
			_ = bctx.conn.WriteConsole(usageMsg)
			_ = bctx.conn.WritePromptSplit(bctx.promptFn())
		} else {
			_ = bctx.conn.WriteLine(usageMsg)
			_ = bctx.conn.WritePrompt(bctx.promptFn())
		}
		return bridgeResult{done: true}, nil
	}
	if bctx.travelResolver != nil {
		zoneID, errMsg := bctx.travelResolver(arg)
		if errMsg != "" {
			if bctx.conn.IsSplitScreen() {
				_ = bctx.conn.WriteConsole(errMsg)
				_ = bctx.conn.WritePromptSplit(bctx.promptFn())
			} else {
				_ = bctx.conn.WriteLine(errMsg)
				_ = bctx.conn.WritePrompt(bctx.promptFn())
			}
			return bridgeResult{done: true}, nil
		}
		return bridgeResult{msg: &gamev1.ClientMessage{
			RequestId: bctx.reqID,
			Payload:   &gamev1.ClientMessage_Travel{Travel: &gamev1.TravelRequest{ZoneId: zoneID}},
		}}, nil
	}
	// Fallback: send with the raw arg as zone_id (server will reject if wrong).
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Travel{Travel: &gamev1.TravelRequest{ZoneId: arg}},
	}}, nil
}

// bridgeSkills builds a SkillsRequest to retrieve skill proficiencies.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a SkillsRequest; done is false.
func bridgeSkills(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_SkillsRequest{SkillsRequest: &gamev1.SkillsRequest{}},
	}}, nil
}

// bridgeFeats builds a FeatsRequest to retrieve feat list.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a FeatsRequest; done is false.
func bridgeFeats(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FeatsRequest{FeatsRequest: &gamev1.FeatsRequest{}},
	}}, nil
}

// bridgeClassFeatures builds a ClassFeaturesRequest to retrieve class feature list.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a ClassFeaturesRequest; done is false.
func bridgeClassFeatures(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_ClassFeaturesRequest{ClassFeaturesRequest: &gamev1.ClassFeaturesRequest{}},
	}}, nil
}

// bridgeProficiencies builds a ProficienciesRequest to retrieve armor/weapon proficiency list.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a ProficienciesRequest; done is false.
func bridgeProficiencies(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_ProficienciesRequest{ProficienciesRequest: &gamev1.ProficienciesRequest{}},
	}}, nil
}

// bridgeInteract builds an InteractRequest for the named item instance ID.
//
// Precondition: bctx must be non-nil with a valid reqID and Args.
// Postcondition: if RawArgs is empty, writes usage error and returns done=true;
// otherwise returns a non-nil msg containing an InteractRequest.
func bridgeInteract(bctx *bridgeContext) (bridgeResult, error) {
	instanceID := strings.TrimSpace(bctx.parsed.RawArgs)
	if instanceID == "" {
		return writeErrorPrompt(bctx, "Usage: interact <item>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_InteractRequest{InteractRequest: &gamev1.InteractRequest{InstanceId: instanceID}},
	}}, nil
}

// bridgeUse builds a UseRequest for tech/feat activation.
// args format: "<abilityID> [target]"
// If no ability name given, sends empty feat_id to trigger listing.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a UseRequest with parsed feat_id and optional target.
func bridgeUse(bctx *bridgeContext) (bridgeResult, error) {
	parts := strings.Fields(bctx.parsed.RawArgs)
	var featID, target string
	if len(parts) >= 1 {
		featID = parts[0]
	}
	if len(parts) >= 2 {
		target = parts[1]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_UseRequest{
			UseRequest: &gamev1.UseRequest{FeatId: featID, Target: target},
		},
	}}, nil
}

// bridgeSummonItem builds a SummonItemRequest, delegating arg validation to HandleSummonItem.
//
// Precondition: bctx must be non-nil with a valid conn, reqID, and parsed.Args.
// Postcondition: if HandleSummonItem returns a Usage string, writes it and returns done=true;
// otherwise returns a non-nil msg containing a SummonItemRequest.
func bridgeSummonItem(bctx *bridgeContext) (bridgeResult, error) {
	parsed := command.HandleSummonItem(strings.Join(bctx.parsed.Args, " "))
	if strings.HasPrefix(parsed, "Usage:") {
		return writeErrorPrompt(bctx, parsed)
	}
	parts := strings.Fields(parsed)
	if len(parts) != 2 {
		return writeErrorPrompt(bctx, "Usage: summon_item <item_id> [quantity]")
	}
	itemID := parts[0]
	qty, _ := strconv.Atoi(parts[1]) // safe: HandleSummonItem already validated
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_SummonItem{SummonItem: &gamev1.SummonItemRequest{
			ItemId:   itemID,
			Quantity: int32(qty),
		}},
	}}, nil
}

// bridgeLevelUp builds a LevelUpRequest, delegating validation to HandleLevelUp.
//
// Precondition: bctx must be non-nil with a valid conn, reqID, and parsed.RawArgs.
// Postcondition: if HandleLevelUp returns a usage or error string, writes it and returns done=true;
// otherwise returns a non-nil msg containing a LevelUpRequest.
func bridgeLevelUp(bctx *bridgeContext) (bridgeResult, error) {
	result := command.HandleLevelUp(bctx.parsed.RawArgs)
	if strings.HasPrefix(result, "Usage:") || strings.HasPrefix(result, "Unknown ability") {
		return writeErrorPrompt(bctx, result)
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_LevelUp{LevelUp: &gamev1.LevelUpRequest{Ability: result}},
	}}, nil
}

// bridgeCombatDefault validates and sends a CombatDefaultRequest to set the player's default combat action.
//
// Precondition: bctx must be non-nil with a valid conn and reqID.
// Postcondition: if HandleCombatDefault returns an error, writes usage error and returns done=true;
// otherwise returns a non-nil msg containing a CombatDefaultRequest.
func bridgeCombatDefault(bctx *bridgeContext) (bridgeResult, error) {
	action, err := command.HandleCombatDefault(bctx.parsed.Args)
	if err != nil {
		return writeErrorPrompt(bctx, err.Error())
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_CombatDefault{CombatDefault: &gamev1.CombatDefaultRequest{Action: action}},
	}}, nil
}

// bridgeTrainSkill validates and sends a TrainSkillRequest.
//
// Precondition: bctx must be non-nil with a valid conn, reqID, and parsed.Args.
// Postcondition: if HandleTrainSkill returns an error, writes usage error and returns done=true;
// otherwise returns a non-nil msg containing a TrainSkillRequest.
func bridgeTrainSkill(bctx *bridgeContext) (bridgeResult, error) {
	skillID, err := command.HandleTrainSkill(bctx.parsed.Args)
	if err != nil {
		return writeErrorPrompt(bctx, err.Error())
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_TrainSkill{TrainSkill: &gamev1.TrainSkillRequest{SkillId: skillID}},
	}}, nil
}

// bridgeAction validates and sends an ActionRequest for a named archetype or job action.
//
// Precondition: bctx must be non-nil with a valid conn, reqID, and parsed.Args.
// Postcondition: if HandleAction returns an error, writes usage error and returns done=true;
// otherwise returns a non-nil msg containing an ActionRequest.
func bridgeAction(bctx *bridgeContext) (bridgeResult, error) {
	req, err := command.HandleAction(bctx.parsed.Args)
	if err != nil {
		return writeErrorPrompt(bctx, err.Error())
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Action{Action: &gamev1.ActionRequest{
			Name:   req.Name,
			Target: req.Target,
		}},
	}}, nil
}

// bridgeRoomEquip builds a RoomEquipRequest from parsed subcommand arguments.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: Returns a non-nil msg containing a RoomEquipRequest populated
// from the parsed args; done is false.
func bridgeRoomEquip(bctx *bridgeContext) (bridgeResult, error) {
	parts := strings.Fields(bctx.parsed.RawArgs)
	req := &gamev1.RoomEquipRequest{}
	if len(parts) > 0 {
		req.SubCommand = parts[0]
	}
	if len(parts) > 1 {
		req.ItemId = parts[1]
	}
	if len(parts) > 2 {
		if n, err := strconv.Atoi(parts[2]); err == nil {
			req.MaxCount = int32(n)
		}
	}
	if len(parts) > 3 {
		req.Respawn = parts[3]
	}
	if len(parts) > 4 {
		req.Immovable = parts[4] == "true"
	}
	if len(parts) > 5 {
		req.Script = strings.Join(parts[5:], " ")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_RoomEquip{RoomEquip: req},
	}}, nil
}

// bridgeRaiseShield builds a RaiseShieldRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a RaiseShieldRequest; done is false.
func bridgeRaiseShield(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_RaiseShield{RaiseShield: &gamev1.RaiseShieldRequest{}},
	}}, nil
}

// bridgeTakeCover builds a TakeCoverRequest.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a TakeCoverRequest; done is false.
func bridgeTakeCover(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_TakeCover{TakeCover: &gamev1.TakeCoverRequest{}},
	}}, nil
}

// bridgeFirstAid builds a FirstAidRequest.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a FirstAidRequest; done is false.
func bridgeFirstAid(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FirstAid{FirstAid: &gamev1.FirstAidRequest{}},
	}}, nil
}

// bridgeFeint builds a FeintRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a FeintRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeFeint(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: feint <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Feint{Feint: &gamev1.FeintRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeDemoralize builds a DemoralizeRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a DemoralizeRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeDemoralize(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: demoralize <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Demoralize{Demoralize: &gamev1.DemoralizeRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeGrapple builds a GrappleRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a GrappleRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeGrapple(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: grapple <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Grapple{Grapple: &gamev1.GrappleRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeTrip builds a TripRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a TripRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeTrip(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: trip <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Trip{Trip: &gamev1.TripRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeClimb builds a ClimbRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a ClimbRequest; done is false.
func bridgeClimb(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) == 0 {
		return writeErrorPrompt(bctx, "Usage: climb <direction>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Climb{Climb: &gamev1.ClimbRequest{
			Direction: bctx.parsed.Args[0],
		}},
	}}, nil
}

// bridgeDisarm builds a DisarmRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a DisarmRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeDisarm(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: disarm <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Disarm{Disarm: &gamev1.DisarmRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeDisarmTrap builds a DisarmTrapRequest with the trap name from the command argument.
//
// Precondition: bctx.parsed.RawArgs must be the trap name.
// Postcondition: Returns a ClientMessage with DisarmTrap payload when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeDisarmTrap(bctx *bridgeContext) (bridgeResult, error) {
	trapName := strings.TrimSpace(bctx.parsed.RawArgs)
	if trapName == "" {
		return writeErrorPrompt(bctx, "Usage: disarm_trap <trap name>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_DisarmTrap{DisarmTrap: &gamev1.DisarmTrapRequest{TrapName: trapName}},
	}}, nil
}

// bridgeDeployTrap builds a DeployTrapRequest with the item name from the command argument.
//
// Precondition: bctx.parsed.RawArgs must be the item name.
// Postcondition: Returns a ClientMessage with DeployTrap payload when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeDeployTrap(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: deploy <item>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_DeployTrap{DeployTrap: &gamev1.DeployTrapRequest{ItemName: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeReady builds a ReadyRequest from "<action> when <trigger>" format.
//
// Precondition: bctx.parsed.RawArgs must contain "<action> when <trigger>".
// Postcondition: Returns a ClientMessage with Ready payload when args are valid;
// otherwise returns done=true with a usage error event.
func bridgeReady(bctx *bridgeContext) (bridgeResult, error) {
	args := strings.TrimSpace(bctx.parsed.RawArgs)
	parts := strings.SplitN(args, " when ", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return writeErrorPrompt(bctx, "Usage: ready <action> when <trigger>\n  actions: strike, step, shield\n  triggers: enters, attacks, ally")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Ready{Ready: &gamev1.ReadyRequest{
			Action:  strings.TrimSpace(parts[0]),
			Trigger: strings.TrimSpace(parts[1]),
		}},
	}}, nil
}

// bridgeShove builds a ShoveRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a ShoveRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeShove(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: shove <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Shove{Shove: &gamev1.ShoveRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeTumble builds a TumbleRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a TumbleRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeTumble(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: tumble <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Tumble{Tumble: &gamev1.TumbleRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeStep builds a StepRequest with the direction ("toward" or "away").
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a StepRequest; direction is "toward" or "away".
func bridgeStep(bctx *bridgeContext) (bridgeResult, error) {
	dir := "toward"
	if bctx.parsed.RawArgs == "away" {
		dir = "away"
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Step{Step: &gamev1.StepRequest{Direction: dir}},
	}}, nil
}

// bridgeStride builds a StrideRequest with the direction ("toward" or "away").
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a StrideRequest; direction is "toward" or "away".
func bridgeStride(bctx *bridgeContext) (bridgeResult, error) {
	dir := "toward"
	if bctx.parsed.RawArgs == "away" {
		dir = "away"
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Stride{Stride: &gamev1.StrideRequest{Direction: dir}},
	}}, nil
}

// bridgeHide builds a HideRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a HideRequest; done is false.
func bridgeHide(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Hide{Hide: &gamev1.HideRequest{}},
	}}, nil
}

// bridgeSneak builds a SneakRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a SneakRequest; done is false.
func bridgeSneak(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Sneak{Sneak: &gamev1.SneakRequest{}},
	}}, nil
}

// bridgeSeek builds a SeekRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a SeekRequest.
func bridgeSeek(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Seek{Seek: &gamev1.SeekRequest{}},
	}}, nil
}

// bridgeCalm sends a CalmRequest with no arguments.
//
// Postcondition: Always returns a non-nil msg containing a CalmRequest.
func bridgeCalm(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Calm{Calm: &gamev1.CalmRequest{}},
	}}, nil
}

// bridgeHeroPoint parses the heropoint subcommand and sends a HeroPointRequest.
//
// Precondition: bctx must be non-nil with a valid reqID and parsed.Args.
// Postcondition: Returns a non-nil msg containing a HeroPointRequest on success;
// returns done=true with usage error on invalid subcommand.
func bridgeHeroPoint(bctx *bridgeContext) (bridgeResult, error) {
	result, err := command.HandleHeroPoint(bctx.parsed.Args)
	if err != nil {
		return bridgeResult{}, err
	}
	if result.Error != "" {
		return writeErrorPrompt(bctx, result.Error)
	}
	return bridgeResult{
		msg: &gamev1.ClientMessage{
			RequestId: bctx.reqID,
			Payload: &gamev1.ClientMessage_HeroPoint{
				HeroPoint: &gamev1.HeroPointRequest{
					Subcommand: result.Subcommand,
				},
			},
		},
	}, nil
}

// bridgeMotive builds a MotiveRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a MotiveRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeMotive(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) == 0 {
		return writeErrorPrompt(bctx, "Usage: motive <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Motive{Motive: &gamev1.MotiveRequest{Target: bctx.parsed.Args[0]}},
	}}, nil
}

// bridgeSwim builds a SwimRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a SwimRequest; done is false.
func bridgeSwim(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) == 0 {
		return writeErrorPrompt(bctx, "Usage: swim <direction>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Swim{Swim: &gamev1.SwimRequest{
			Direction: bctx.parsed.Args[0],
		}},
	}}, nil
}

// bridgeDivert builds a DivertRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a DivertRequest; done is false.
func bridgeDivert(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Divert{Divert: &gamev1.DivertRequest{}},
	}}, nil
}

// bridgeEscape builds an EscapeRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an EscapeRequest; done is false.
func bridgeEscape(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Escape{Escape: &gamev1.EscapeRequest{}},
	}}, nil
}

// bridgeGrant builds a GrantRequest, prompting for grant type, character name, and amount if missing.
// Character name is read as a full line to support multiword names.
//
// Precondition: bctx must be non-nil with a valid conn and reqID; caller must hold editor/admin role.
// Postcondition: Returns a non-nil msg containing a GrantRequest, or writes an error and returns done=true.
func bridgeGrant(bctx *bridgeContext) (bridgeResult, error) {
	args := bctx.parsed.Args

	// Resolve grant type from first arg or prompt.
	var grantType string
	if len(args) > 0 {
		grantType = strings.ToLower(args[0])
	}
	if grantType != "xp" && grantType != "money" && grantType != "heropoint" {
		_ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Grant type (xp/money/heropoint): "))
		line, err := bctx.conn.ReadLine()
		if err != nil {
			return bridgeResult{}, fmt.Errorf("reading grant type: %w", err)
		}
		grantType = strings.ToLower(strings.TrimSpace(line))
		if grantType != "xp" && grantType != "money" && grantType != "heropoint" {
			return writeErrorPrompt(bctx, "Grant type must be 'xp', 'money', or 'heropoint'.")
		}
	}

	// Resolve character name from remaining args or prompt.
	// Always prompt when no args follow the type so multiword names are supported via readline.
	var charName string
	if len(args) > 2 {
		// args[1..n-1] are the name; args[n] is the amount if it parses as an int.
		last := args[len(args)-1]
		if _, aErr := strconv.Atoi(last); aErr == nil && len(args) > 2 {
			charName = strings.Join(args[1:len(args)-1], " ")
		} else {
			charName = strings.Join(args[1:], " ")
		}
	}
	if charName == "" {
		_ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Character name: "))
		line, err := bctx.conn.ReadLine()
		if err != nil {
			return bridgeResult{}, fmt.Errorf("reading character name: %w", err)
		}
		charName = strings.TrimSpace(line)
	}
	if charName == "" {
		return writeErrorPrompt(bctx, "Character name cannot be empty.")
	}

	// Resolve amount: heropoint grants always use amount=1; others prompt or parse from args.
	var amount int
	if grantType == "heropoint" {
		amount = 1
	} else {
		var amountStr string
		if len(args) > 2 {
			last := args[len(args)-1]
			if _, aErr := strconv.Atoi(last); aErr == nil {
				amountStr = last
			}
		}
		if amountStr == "" {
			_ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Amount: "))
			line, err := bctx.conn.ReadLine()
			if err != nil {
				return bridgeResult{}, fmt.Errorf("reading amount: %w", err)
			}
			amountStr = strings.TrimSpace(line)
		}
		var convErr error
		amount, convErr = strconv.Atoi(amountStr)
		if convErr != nil || amount <= 0 {
			return writeErrorPrompt(bctx, "Amount must be a positive integer.")
		}
	}

	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Grant{Grant: &gamev1.GrantRequest{
			GrantType: grantType,
			CharName:  charName,
			Amount:    int32(amount),
		}},
	}}, nil
}

// bridgeDelay builds a DelayRequest message with no arguments.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a DelayRequest.
func bridgeDelay(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Delay{Delay: &gamev1.DelayRequest{}},
	}}, nil
}

// bridgeJoin builds a JoinRequest message with no arguments.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a JoinRequest.
func bridgeJoin(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Join{Join: &gamev1.JoinRequest{}},
	}}, nil
}

// bridgeDecline builds a DeclineRequest message with no arguments.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a DeclineRequest.
func bridgeDecline(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Decline{Decline: &gamev1.DeclineRequest{}},
	}}, nil
}

// bridgeGroup builds a GroupRequest message with optional args.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a GroupRequest.
func bridgeGroup(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Group{Group: &gamev1.GroupRequest{Args: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeInvite builds an InviteRequest message with the target player name.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an InviteRequest.
func bridgeInvite(bctx *bridgeContext) (bridgeResult, error) {
	player := ""
	if len(bctx.parsed.Args) > 0 {
		player = bctx.parsed.Args[0]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Invite{Invite: &gamev1.InviteRequest{Player: player}},
	}}, nil
}

// bridgeAcceptGroup builds an AcceptGroupRequest message with no arguments.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an AcceptGroupRequest.
func bridgeAcceptGroup(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_AcceptGroup{AcceptGroup: &gamev1.AcceptGroupRequest{}},
	}}, nil
}

// bridgeDeclineGroup builds a DeclineGroupRequest message with no arguments.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a DeclineGroupRequest.
func bridgeDeclineGroup(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_DeclineGroup{DeclineGroup: &gamev1.DeclineGroupRequest{}},
	}}, nil
}

// bridgeUngroup builds an UngroupRequest message with no arguments.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an UngroupRequest.
func bridgeUngroup(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Ungroup{Ungroup: &gamev1.UngroupRequest{}},
	}}, nil
}

// bridgeKick builds a KickRequest message with the target player name.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a KickRequest.
func bridgeKick(bctx *bridgeContext) (bridgeResult, error) {
	player := ""
	if len(bctx.parsed.Args) > 0 {
		player = bctx.parsed.Args[0]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Kick{Kick: &gamev1.KickRequest{Player: player}},
	}}, nil
}

// bridgeSelectTech builds a SelectTechRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a SelectTechRequest; done is false.
func bridgeSelectTech(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_SelectTech{SelectTech: &gamev1.SelectTechRequest{}},
	}}, nil
}

// bridgeRest builds a RestRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a RestRequest; done is false.
func bridgeRest(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Rest{Rest: &gamev1.RestRequest{}},
	}}, nil
}

// bridgeAid builds an AidRequest targeting the first whitespace-delimited token.
// If no token is present, target is empty string (server will reject with helpful message).
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an AidRequest; done is false.
func bridgeAid(bctx *bridgeContext) (bridgeResult, error) {
	target := ""
	if fields := strings.Fields(bctx.parsed.RawArgs); len(fields) > 0 {
		target = fields[0]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Aid{Aid: &gamev1.AidRequest{Target: target}},
	}}, nil
}

// bridgeTalk builds a TalkRequest for the named NPC.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a TalkRequest; done is false.
func bridgeTalk(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Talk{Talk: &gamev1.TalkRequest{NpcName: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeHeal builds a HealRequest or HealAmountRequest depending on whether an amount is given.
// Syntax: heal <npc> or heal <npc> <amount>
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeHeal(bctx *bridgeContext) (bridgeResult, error) {
	fields := strings.Fields(bctx.parsed.RawArgs)
	if len(fields) >= 2 {
		amount, err := strconv.Atoi(fields[len(fields)-1])
		if err == nil && amount > 0 {
			npcName := strings.Join(fields[:len(fields)-1], " ")
			return bridgeResult{msg: &gamev1.ClientMessage{
				RequestId: bctx.reqID,
				Payload:   &gamev1.ClientMessage_HealAmount{HealAmount: &gamev1.HealAmountRequest{NpcName: npcName, Amount: int32(amount)}},
			}}, nil
		}
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Heal{Heal: &gamev1.HealRequest{NpcName: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeBrowse builds a BrowseRequest for the named merchant NPC.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a BrowseRequest; done is false.
func bridgeBrowse(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Browse{Browse: &gamev1.BrowseRequest{NpcName: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeBuy builds a BuyRequest.
// Syntax: buy <npc> <item_id> [qty]
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeBuy(bctx *bridgeContext) (bridgeResult, error) {
	fields := strings.Fields(bctx.parsed.RawArgs)
	if len(fields) < 2 {
		return writeErrorPrompt(bctx, "Usage: buy <npc> <item> [quantity]")
	}
	qty := int32(1)
	if len(fields) >= 3 {
		if n, err := strconv.Atoi(fields[len(fields)-1]); err == nil && n > 0 {
			qty = int32(n)
			fields = fields[:len(fields)-1]
		}
	}
	npcName := fields[0]
	itemID := strings.Join(fields[1:], " ")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Buy{Buy: &gamev1.BuyRequest{NpcName: npcName, ItemId: itemID, Quantity: qty}},
	}}, nil
}

// bridgeSell builds a SellRequest.
// Syntax: sell <npc> <item_id> [qty]
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeSell(bctx *bridgeContext) (bridgeResult, error) {
	fields := strings.Fields(bctx.parsed.RawArgs)
	if len(fields) < 2 {
		return writeErrorPrompt(bctx, "Usage: sell <npc> <item> [quantity]")
	}
	qty := int32(1)
	if len(fields) >= 3 {
		if n, err := strconv.Atoi(fields[len(fields)-1]); err == nil && n > 0 {
			qty = int32(n)
			fields = fields[:len(fields)-1]
		}
	}
	npcName := fields[0]
	itemID := strings.Join(fields[1:], " ")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Sell{Sell: &gamev1.SellRequest{NpcName: npcName, ItemId: itemID, Quantity: qty}},
	}}, nil
}

// bridgeNegotiate builds a NegotiateRequest.
// Syntax: negotiate <npc> [smooth_talk|grift]
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeNegotiate(bctx *bridgeContext) (bridgeResult, error) {
	fields := strings.Fields(bctx.parsed.RawArgs)
	if len(fields) == 0 {
		return writeErrorPrompt(bctx, "Usage: negotiate <npc> [smooth_talk|grift]")
	}
	skill := ""
	npcName := fields[0]
	if len(fields) >= 2 {
		skill = fields[len(fields)-1]
		npcName = strings.Join(fields[:len(fields)-1], " ")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Negotiate{Negotiate: &gamev1.NegotiateRequest{NpcName: npcName, Skill: skill}},
	}}, nil
}

// bridgeDeposit builds a StashDepositRequest.
// Syntax: deposit <npc> <amount>
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeDeposit(bctx *bridgeContext) (bridgeResult, error) {
	fields := strings.Fields(bctx.parsed.RawArgs)
	if len(fields) < 2 {
		return writeErrorPrompt(bctx, "Usage: deposit <npc> <amount>")
	}
	amount, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil || amount <= 0 {
		return writeErrorPrompt(bctx, "Usage: deposit <npc> <amount>")
	}
	npcName := strings.Join(fields[:len(fields)-1], " ")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_StashDeposit{StashDeposit: &gamev1.StashDepositRequest{NpcName: npcName, Amount: int32(amount)}},
	}}, nil
}

// bridgeWithdraw builds a StashWithdrawRequest.
// Syntax: withdraw <npc> <amount>
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeWithdraw(bctx *bridgeContext) (bridgeResult, error) {
	fields := strings.Fields(bctx.parsed.RawArgs)
	if len(fields) < 2 {
		return writeErrorPrompt(bctx, "Usage: withdraw <npc> <amount>")
	}
	amount, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil || amount <= 0 {
		return writeErrorPrompt(bctx, "Usage: withdraw <npc> <amount>")
	}
	npcName := strings.Join(fields[:len(fields)-1], " ")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_StashWithdraw{StashWithdraw: &gamev1.StashWithdrawRequest{NpcName: npcName, Amount: int32(amount)}},
	}}, nil
}

// bridgeStashBalance builds a StashBalanceRequest.
// Syntax: stash <npc>
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeStashBalance(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_StashBalance{StashBalance: &gamev1.StashBalanceRequest{NpcName: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeHire builds a HireRequest for the named hireling NPC.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeHire(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Hire{Hire: &gamev1.HireRequest{NpcName: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeDismiss builds a DismissRequest.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeDismiss(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Dismiss{Dismiss: &gamev1.DismissRequest{}},
	}}, nil
}

// bridgeTrainJob builds a TrainJobRequest.
// Syntax: train <npc> <job>
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeTrainJob(bctx *bridgeContext) (bridgeResult, error) {
	fields := strings.Fields(bctx.parsed.RawArgs)
	if len(fields) < 2 {
		return writeErrorPrompt(bctx, "Usage: train <npc> <job>")
	}
	npcName := fields[0]
	jobID := strings.Join(fields[1:], "_")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_TrainJob{TrainJob: &gamev1.TrainJobRequest{NpcName: npcName, JobId: jobID}},
	}}, nil
}

// bridgeListJobs builds a ListJobsRequest.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeListJobs(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_ListJobs{ListJobs: &gamev1.ListJobsRequest{}},
	}}, nil
}

// bridgeSetJob builds a SetJobRequest.
// Syntax: setjob <job>
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg; done is false.
func bridgeSetJob(bctx *bridgeContext) (bridgeResult, error) {
	jobID := strings.ReplaceAll(strings.TrimSpace(bctx.parsed.RawArgs), " ", "_")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_SetJob{SetJob: &gamev1.SetJobRequest{JobId: jobID}},
	}}, nil
}

// bridgeBribe builds a BribeRequest or BribeConfirmRequest depending on whether the
// first argument is "confirm".
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a BribeConfirmRequest when the first
// arg is "confirm"; otherwise returns a BribeRequest with an optional NPC name.
func bridgeBribe(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) > 0 && strings.ToLower(bctx.parsed.Args[0]) == "confirm" {
		return bridgeResult{msg: &gamev1.ClientMessage{
			RequestId: bctx.reqID,
			Payload:   &gamev1.ClientMessage_BribeConfirmRequest{BribeConfirmRequest: &gamev1.BribeConfirmRequest{}},
		}}, nil
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_BribeRequest{BribeRequest: &gamev1.BribeRequest{NpcName: bctx.parsed.RawArgs}},
	}}, nil
}

// bridgeSurrender builds a SurrenderRequest.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a SurrenderRequest; done is false.
func bridgeSurrender(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_SurrenderRequest{SurrenderRequest: &gamev1.SurrenderRequest{}},
	}}, nil
}

// bridgeRelease builds a ReleaseRequest for the named detained player.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: if RawArgs is empty, writes a usage error and returns done=true;
// otherwise returns a non-nil msg containing a ReleaseRequest.
func bridgeRelease(bctx *bridgeContext) (bridgeResult, error) {
	playerName := strings.TrimSpace(bctx.parsed.RawArgs)
	if playerName == "" {
		return writeErrorPrompt(bctx, "Usage: release <player>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_ReleaseRequest{ReleaseRequest: &gamev1.ReleaseRequest{PlayerName: playerName}},
	}}, nil
}

// bridgeSpawnNPC builds a SpawnNPCRequest. Usage: spawnnpc <template_id> [room_id]
// Precondition: bctx must be non-nil; caller must have editor or admin role.
// Postcondition: returns a SpawnNPCRequest with the given template and optional room.
func bridgeSpawnNPC(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) < 1 {
		return writeErrorPrompt(bctx, "Usage: spawnnpc <template_id> [room_id]")
	}
	templateID := bctx.parsed.Args[0]
	roomID := ""
	if len(bctx.parsed.Args) >= 2 {
		roomID = bctx.parsed.Args[1]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_SpawnNpc{SpawnNpc: &gamev1.SpawnNPCRequest{TemplateId: templateID, RoomId: roomID}},
	}}, nil
}

// bridgeAddRoom builds an AddRoomRequest. Usage: addroom <zone_id> <room_id> <title...>
// Precondition: bctx must be non-nil; caller must have editor or admin role.
// Postcondition: returns an AddRoomRequest with the given zone, room ID, and title.
func bridgeAddRoom(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) < 3 {
		return writeErrorPrompt(bctx, "Usage: addroom <zone_id> <room_id> <title>")
	}
	zoneID := bctx.parsed.Args[0]
	roomID := bctx.parsed.Args[1]
	title := strings.Join(bctx.parsed.Args[2:], " ")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_AddRoom{AddRoom: &gamev1.AddRoomRequest{ZoneId: zoneID, RoomId: roomID, Title: title}},
	}}, nil
}

// bridgeAddLink builds an AddLinkRequest. Usage: addlink <from_room> <direction> <to_room>
// Precondition: bctx must be non-nil; caller must have editor or admin role.
// Postcondition: returns an AddLinkRequest with the given rooms and direction.
func bridgeAddLink(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) < 3 {
		return writeErrorPrompt(bctx, "Usage: addlink <from_room_id> <direction> <to_room_id>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_AddLink{AddLink: &gamev1.AddLinkRequest{
			FromRoomId: bctx.parsed.Args[0],
			Direction:  bctx.parsed.Args[1],
			ToRoomId:   bctx.parsed.Args[2],
		}},
	}}, nil
}

// bridgeRemoveLink builds a RemoveLinkRequest. Usage: removelink <room_id> <direction>
// Precondition: bctx must be non-nil; caller must have editor or admin role.
// Postcondition: returns a RemoveLinkRequest for the given room and direction.
func bridgeRemoveLink(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) < 2 {
		return writeErrorPrompt(bctx, "Usage: removelink <room_id> <direction>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_RemoveLink{RemoveLink: &gamev1.RemoveLinkRequest{
			RoomId:    bctx.parsed.Args[0],
			Direction: bctx.parsed.Args[1],
		}},
	}}, nil
}

// bridgeSetRoom builds a SetRoomRequest. Usage: setroom <field> <value...>
// Precondition: bctx must be non-nil; caller must have editor or admin role.
// Postcondition: returns a SetRoomRequest for the current room with the given field and value.
func bridgeSetRoom(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) < 2 {
		return writeErrorPrompt(bctx, "Usage: setroom <field> <value>  (fields: title, description, danger_level)")
	}
	field := bctx.parsed.Args[0]
	value := strings.Join(bctx.parsed.Args[1:], " ")
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_SetRoom{SetRoom: &gamev1.SetRoomRequest{Field: field, Value: value}},
	}}, nil
}

// bridgeEditorCmds builds an EditorCmdsRequest. Usage: ecmds
// Precondition: bctx must be non-nil; caller must have editor or admin role.
// Postcondition: returns an EditorCmdsRequest.
func bridgeEditorCmds(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_EditorCmds{EditorCmds: &gamev1.EditorCmdsRequest{}},
	}}, nil
}

// bridgeFaction builds a FactionRequest asking the server for the player's
// current faction, tier, rep, and perks.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a FactionRequest; done is false.
func bridgeFaction(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FactionRequest{FactionRequest: &gamev1.FactionRequest{}},
	}}, nil
}

// bridgeFactionInfo builds a FactionInfoRequest asking the server for public
// information about the faction identified by args[0].
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: if RawArgs is empty, writes a usage error and returns done=true;
// otherwise returns a non-nil msg containing a FactionInfoRequest.
func bridgeFactionInfo(bctx *bridgeContext) (bridgeResult, error) {
	factionID := strings.TrimSpace(bctx.parsed.RawArgs)
	if factionID == "" {
		return writeErrorPrompt(bctx, "Usage: faction_info <faction_id>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FactionInfoRequest{FactionInfoRequest: &gamev1.FactionInfoRequest{FactionId: factionID}},
	}}, nil
}

// bridgeFactionStanding builds a FactionStandingRequest asking the server for
// the player's standing in all tracked factions.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a FactionStandingRequest; done is false.
func bridgeFactionStanding(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FactionStandingRequest{FactionStandingRequest: &gamev1.FactionStandingRequest{}},
	}}, nil
}

// bridgeChangeRep builds a ChangeRepRequest asking a Fixer NPC to improve the
// player's faction standing for currency. The faction_id is taken from RawArgs.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: if RawArgs is empty, writes a usage error and returns done=true;
// otherwise returns a non-nil msg containing a ChangeRepRequest.
func bridgeChangeRep(bctx *bridgeContext) (bridgeResult, error) {
	factionID := strings.TrimSpace(bctx.parsed.RawArgs)
	if factionID == "" {
		return writeErrorPrompt(bctx, "Usage: change_rep <faction_id>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_ChangeRepRequest{ChangeRepRequest: &gamev1.ChangeRepRequest{FactionId: factionID}},
	}}, nil
}

// bridgeTabComplete builds a TabCompleteRequest from the raw input prefix.
// Precondition: bctx must be non-nil with a valid reqID; parsed.RawArgs contains the current input prefix.
// Postcondition: returns a non-nil msg containing a TabCompleteRequest with Prefix set; done is false.
func bridgeTabComplete(bctx *bridgeContext) (bridgeResult, error) {
	prefix := strings.TrimSpace(bctx.parsed.RawArgs)
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_TabComplete{
			TabComplete: &gamev1.TabCompleteRequest{Prefix: prefix},
		},
	}}, nil
}
