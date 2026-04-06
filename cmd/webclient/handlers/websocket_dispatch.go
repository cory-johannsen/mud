package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	case command.HandlerHotbar:
		args := parsed.Args
		if len(args) == 0 {
			return &gamev1.ClientMessage{RequestId: reqID,
				Payload: &gamev1.ClientMessage_HotbarRequest{HotbarRequest: &gamev1.HotbarRequest{Action: "show"}}}, nil
		}
		if args[0] == "clear" {
			if len(args) < 2 {
				return nil, fmt.Errorf("usage: hotbar clear <slot>")
			}
			slot, err := strconv.Atoi(args[1])
			if err != nil {
				return nil, fmt.Errorf("invalid slot %q: must be a number 1-10", args[1])
			}
			return &gamev1.ClientMessage{RequestId: reqID,
				Payload: &gamev1.ClientMessage_HotbarRequest{HotbarRequest: &gamev1.HotbarRequest{Action: "clear", Slot: int32(slot)}}}, nil
		}
		slot, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, fmt.Errorf("invalid slot %q: must be a number 1-10", args[0])
		}
		text := strings.Join(args[1:], " ")
		if text == "" {
			return nil, fmt.Errorf("usage: hotbar <slot> <command text>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_HotbarRequest{HotbarRequest: &gamev1.HotbarRequest{Action: "set", Slot: int32(slot), Text: text}}}, nil

	// Combat actions
	case command.HandlerReload:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Reload{Reload: &gamev1.ReloadRequest{WeaponId: rawArgs}}}, nil
	case command.HandlerFireBurst:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_FireBurst{FireBurst: &gamev1.FireBurstRequest{Target: rawArgs}}}, nil
	case command.HandlerFireAuto:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_FireAutomatic{FireAutomatic: &gamev1.FireAutomaticRequest{Target: rawArgs}}}, nil
	case command.HandlerThrow:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Throw{Throw: &gamev1.ThrowRequest{ExplosiveId: rawArgs}}}, nil
	case command.HandlerRaiseShield:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_RaiseShield{RaiseShield: &gamev1.RaiseShieldRequest{}}}, nil
	case command.HandlerTakeCover:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_TakeCover{TakeCover: &gamev1.TakeCoverRequest{}}}, nil
	case command.HandlerUncover:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_UncoverRequest{UncoverRequest: &gamev1.UncoverRequest{}}}, nil
	case command.HandlerFirstAid:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_FirstAid{FirstAid: &gamev1.FirstAidRequest{}}}, nil
	case command.HandlerFeint:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Feint{Feint: &gamev1.FeintRequest{Target: rawArgs}}}, nil
	case command.HandlerDemoralize:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Demoralize{Demoralize: &gamev1.DemoralizeRequest{Target: rawArgs}}}, nil
	case command.HandlerSeduce:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SeduceRequest{SeduceRequest: &gamev1.SeduceRequest{Target: rawArgs}}}, nil
	case command.HandlerGrapple:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Grapple{Grapple: &gamev1.GrappleRequest{Target: rawArgs}}}, nil
	case command.HandlerTrip:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Trip{Trip: &gamev1.TripRequest{Target: rawArgs}}}, nil
	case command.HandlerDelay:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Delay{Delay: &gamev1.DelayRequest{}}}, nil
	case command.HandlerDisarm:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Disarm{Disarm: &gamev1.DisarmRequest{Target: rawArgs}}}, nil
	case command.HandlerDisarmTrap:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_DisarmTrap{DisarmTrap: &gamev1.DisarmTrapRequest{TrapName: rawArgs}}}, nil
	case command.HandlerDeployTrap:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_DeployTrap{DeployTrap: &gamev1.DeployTrapRequest{ItemName: rawArgs}}}, nil
	case command.HandlerReady:
		parts := strings.SplitN(rawArgs, " when ", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("usage: ready <action> when <trigger>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Ready{Ready: &gamev1.ReadyRequest{
				Action:  strings.TrimSpace(parts[0]),
				Trigger: strings.TrimSpace(parts[1]),
			}}}, nil
	case command.HandlerShove:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Shove{Shove: &gamev1.ShoveRequest{Target: rawArgs}}}, nil
	case command.HandlerTumble:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Tumble{Tumble: &gamev1.TumbleRequest{Target: rawArgs}}}, nil
	case command.HandlerSeek:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Seek{Seek: &gamev1.SeekRequest{}}}, nil
	case command.HandlerEscape:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Escape{Escape: &gamev1.EscapeRequest{}}}, nil
	case command.HandlerDivert:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Divert{Divert: &gamev1.DivertRequest{}}}, nil
	case command.HandlerCalm:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Calm{Calm: &gamev1.CalmRequest{}}}, nil
	case command.HandlerAid:
		aidTarget := ""
		if fields := strings.Fields(rawArgs); len(fields) > 0 {
			aidTarget = fields[0]
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Aid{Aid: &gamev1.AidRequest{Target: aidTarget}}}, nil
	case command.HandlerHide:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Hide{Hide: &gamev1.HideRequest{}}}, nil
	case command.HandlerSneak:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Sneak{Sneak: &gamev1.SneakRequest{}}}, nil
	case command.HandlerStride:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Stride{Stride: &gamev1.StrideRequest{}}}, nil
	case command.HandlerStep:
		stepDir := "toward"
		if rawArgs == "away" {
			stepDir = "away"
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Step{Step: &gamev1.StepRequest{Direction: stepDir}}}, nil
	case command.HandlerClimb:
		if arg == "" {
			return nil, fmt.Errorf("usage: climb <direction>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Climb{Climb: &gamev1.ClimbRequest{Direction: arg}}}, nil
	case command.HandlerSwim:
		if arg == "" {
			return nil, fmt.Errorf("usage: swim <direction>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Swim{Swim: &gamev1.SwimRequest{Direction: arg}}}, nil
	case command.HandlerMotive:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Motive{Motive: &gamev1.MotiveRequest{Target: arg}}}, nil
	case command.HandlerCombatDefault:
		combatAction, combatErr := command.HandleCombatDefault(parsed.Args)
		if combatErr != nil {
			return nil, combatErr
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_CombatDefault{CombatDefault: &gamev1.CombatDefaultRequest{Action: combatAction}}}, nil
	case command.HandlerAction:
		actionReq, actionErr := command.HandleAction(parsed.Args)
		if actionErr != nil {
			return nil, actionErr
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Action{Action: &gamev1.ActionRequest{Name: actionReq.Name, Target: actionReq.Target}}}, nil

	// Inventory / Equipment
	case command.HandlerEquip:
		equipParts := strings.SplitN(rawArgs, " ", 2)
		equipSlot := ""
		if len(equipParts) == 2 {
			equipSlot = strings.TrimSpace(equipParts[1])
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Equip{Equip: &gamev1.EquipRequest{WeaponId: strings.TrimSpace(equipParts[0]), Slot: equipSlot}}}, nil
	case command.HandlerUnequip:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Unequip{Unequip: &gamev1.UnequipRequest{Slot: rawArgs}}}, nil
	case command.HandlerLoadout:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Loadout{Loadout: &gamev1.LoadoutRequest{Arg: rawArgs}}}, nil
	case command.HandlerWear:
		if len(parsed.Args) < 2 {
			return nil, fmt.Errorf("usage: wear <item_id> <slot>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Wear{Wear: &gamev1.WearRequest{ItemId: parsed.Args[0], Slot: parsed.Args[1]}}}, nil
	case command.HandlerRemoveArmor:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_RemoveArmor{RemoveArmor: &gamev1.RemoveArmorRequest{Slot: rawArgs}}}, nil
	case command.HandlerGet:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_GetItem{GetItem: &gamev1.GetItemRequest{Target: rawArgs}}}, nil
	case command.HandlerDrop:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_DropItem{DropItem: &gamev1.DropItemRequest{Target: rawArgs}}}, nil
	case command.HandlerEquipment:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Equipment{Equipment: &gamev1.EquipmentRequest{}}}, nil
	case command.HandlerUseEquipment:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_UseEquipment{UseEquipment: &gamev1.UseEquipmentRequest{InstanceId: strings.TrimSpace(rawArgs)}}}, nil

	// General / character commands
	case command.HandlerUse:
		useParts := strings.Fields(rawArgs)
		var useFeatID, useTarget string
		if len(useParts) >= 1 {
			useFeatID = useParts[0]
		}
		if len(useParts) >= 2 {
			useTarget = useParts[1]
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_UseRequest{UseRequest: &gamev1.UseRequest{FeatId: useFeatID, Target: useTarget}}}, nil
	case command.HandlerInteract:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_InteractRequest{InteractRequest: &gamev1.InteractRequest{InstanceId: strings.TrimSpace(rawArgs)}}}, nil
	case command.HandlerBalance:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Balance{Balance: &gamev1.BalanceRequest{}}}, nil
	case command.HandlerClassFeatures:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ClassFeaturesRequest{ClassFeaturesRequest: &gamev1.ClassFeaturesRequest{}}}, nil
	case command.HandlerProficiencies:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ProficienciesRequest{ProficienciesRequest: &gamev1.ProficienciesRequest{}}}, nil
	case command.HandlerHeroPoint:
		heroResult, heroErr := command.HandleHeroPoint(parsed.Args)
		if heroErr != nil {
			return nil, heroErr
		}
		if heroResult.Error != "" {
			return nil, fmt.Errorf("%s", heroResult.Error)
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_HeroPoint{HeroPoint: &gamev1.HeroPointRequest{Subcommand: heroResult.Subcommand}}}, nil
	case command.HandlerLevelUp:
		levelResult := command.HandleLevelUp(rawArgs)
		if strings.HasPrefix(levelResult, "Usage:") || strings.HasPrefix(levelResult, "Unknown ability") {
			return nil, fmt.Errorf("%s", levelResult)
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_LevelUp{LevelUp: &gamev1.LevelUpRequest{Ability: levelResult}}}, nil
	case command.HandlerTrainSkill:
		skillID, trainErr := command.HandleTrainSkill(parsed.Args)
		if trainErr != nil {
			return nil, trainErr
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_TrainSkill{TrainSkill: &gamev1.TrainSkillRequest{SkillId: skillID}}}, nil
	case command.HandlerSelectTech:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SelectTech{SelectTech: &gamev1.SelectTechRequest{}}}, nil
	case command.HandlerDowntime:
		downtimeParts := strings.SplitN(strings.TrimSpace(rawArgs), " ", 2)
		downtimeSubcmd := ""
		downtimeArgs := ""
		if len(downtimeParts) >= 1 {
			downtimeSubcmd = strings.ToLower(downtimeParts[0])
		}
		if len(downtimeParts) >= 2 {
			downtimeArgs = downtimeParts[1]
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_DowntimeRequest{DowntimeRequest: &gamev1.DowntimeRequest{
				Subcommand: downtimeSubcmd,
				Args:       downtimeArgs,
			}}}, nil
	case command.HandlerExplore:
		exploreReq, exploreErr := command.HandleExplore(parsed.Args)
		if exploreErr != nil {
			return nil, exploreErr
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ExploreRequest{ExploreRequest: &gamev1.ExploreRequest{
				Mode:         exploreReq.Mode,
				ShadowTarget: exploreReq.ShadowTarget,
			}}}, nil
	case command.HandlerMaterials:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_MaterialsRequest{MaterialsRequest: &gamev1.MaterialsRequest{Category: strings.TrimSpace(rawArgs)}}}, nil
	case command.HandlerCraft:
		craftArgs := strings.Fields(rawArgs)
		if len(craftArgs) == 0 {
			return &gamev1.ClientMessage{RequestId: reqID,
				Payload: &gamev1.ClientMessage_CraftListRequest{CraftListRequest: &gamev1.CraftListRequest{}}}, nil
		}
		switch strings.ToLower(craftArgs[0]) {
		case "list":
			craftCategory := ""
			if len(craftArgs) > 1 {
				craftCategory = craftArgs[1]
			}
			return &gamev1.ClientMessage{RequestId: reqID,
				Payload: &gamev1.ClientMessage_CraftListRequest{CraftListRequest: &gamev1.CraftListRequest{Category: craftCategory}}}, nil
		case "confirm":
			return &gamev1.ClientMessage{RequestId: reqID,
				Payload: &gamev1.ClientMessage_CraftConfirmRequest{CraftConfirmRequest: &gamev1.CraftConfirmRequest{}}}, nil
		default:
			return &gamev1.ClientMessage{RequestId: reqID,
				Payload: &gamev1.ClientMessage_CraftRequest{CraftRequest: &gamev1.CraftRequest{RecipeId: strings.Join(craftArgs, " ")}}}, nil
		}
	case command.HandlerScavenge:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ScavengeRequest{ScavengeRequest: &gamev1.ScavengeRequest{}}}, nil
	case command.HandlerAffix:
		affixArgs := strings.Fields(rawArgs)
		if len(affixArgs) < 2 {
			return nil, fmt.Errorf("usage: affix <material> <item>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_AffixRequest{AffixRequest: &gamev1.AffixRequest{
				MaterialQuery: affixArgs[0],
				TargetQuery:   strings.Join(affixArgs[1:], " "),
			}}}, nil

	// Social / NPC commands
	case command.HandlerTalk:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Talk{Talk: &gamev1.TalkRequest{NpcName: rawArgs}}}, nil
	case command.HandlerHeal:
		healFields := strings.Fields(rawArgs)
		if len(healFields) >= 2 {
			if healAmount, healErr := strconv.Atoi(healFields[len(healFields)-1]); healErr == nil && healAmount > 0 {
				healNPC := strings.Join(healFields[:len(healFields)-1], " ")
				return &gamev1.ClientMessage{RequestId: reqID,
					Payload: &gamev1.ClientMessage_HealAmount{HealAmount: &gamev1.HealAmountRequest{NpcName: healNPC, Amount: int32(healAmount)}}}, nil
			}
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Heal{Heal: &gamev1.HealRequest{NpcName: rawArgs}}}, nil
	case command.HandlerBrowse:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Browse{Browse: &gamev1.BrowseRequest{NpcName: rawArgs}}}, nil
	case command.HandlerBuy:
		buyFields := strings.Fields(rawArgs)
		if len(buyFields) < 2 {
			return nil, fmt.Errorf("usage: buy <npc> <item> [quantity]")
		}
		buyQty := int32(1)
		if len(buyFields) >= 3 {
			if n, nErr := strconv.Atoi(buyFields[len(buyFields)-1]); nErr == nil && n > 0 {
				buyQty = int32(n)
				buyFields = buyFields[:len(buyFields)-1]
			}
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Buy{Buy: &gamev1.BuyRequest{NpcName: buyFields[0], ItemId: strings.Join(buyFields[1:], " "), Quantity: buyQty}}}, nil
	case command.HandlerSell:
		sellFields := strings.Fields(rawArgs)
		if len(sellFields) < 2 {
			return nil, fmt.Errorf("usage: sell <npc> <item> [quantity]")
		}
		sellQty := int32(1)
		if len(sellFields) >= 3 {
			if n, nErr := strconv.Atoi(sellFields[len(sellFields)-1]); nErr == nil && n > 0 {
				sellQty = int32(n)
				sellFields = sellFields[:len(sellFields)-1]
			}
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Sell{Sell: &gamev1.SellRequest{NpcName: sellFields[0], ItemId: strings.Join(sellFields[1:], " "), Quantity: sellQty}}}, nil
	case command.HandlerNegotiate:
		negFields := strings.Fields(rawArgs)
		if len(negFields) == 0 {
			return nil, fmt.Errorf("usage: negotiate <npc> [smooth_talk|grift]")
		}
		negSkill := ""
		negNPC := negFields[0]
		if len(negFields) >= 2 {
			negSkill = negFields[len(negFields)-1]
			negNPC = strings.Join(negFields[:len(negFields)-1], " ")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Negotiate{Negotiate: &gamev1.NegotiateRequest{NpcName: negNPC, Skill: negSkill}}}, nil
	case command.HandlerDeposit:
		depFields := strings.Fields(rawArgs)
		if len(depFields) < 2 {
			return nil, fmt.Errorf("usage: deposit <npc> <amount>")
		}
		depAmount, depErr := strconv.Atoi(depFields[len(depFields)-1])
		if depErr != nil || depAmount <= 0 {
			return nil, fmt.Errorf("usage: deposit <npc> <amount>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_StashDeposit{StashDeposit: &gamev1.StashDepositRequest{
				NpcName: strings.Join(depFields[:len(depFields)-1], " "),
				Amount:  int32(depAmount),
			}}}, nil
	case command.HandlerWithdraw:
		withFields := strings.Fields(rawArgs)
		if len(withFields) < 2 {
			return nil, fmt.Errorf("usage: withdraw <npc> <amount>")
		}
		withAmount, withErr := strconv.Atoi(withFields[len(withFields)-1])
		if withErr != nil || withAmount <= 0 {
			return nil, fmt.Errorf("usage: withdraw <npc> <amount>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_StashWithdraw{StashWithdraw: &gamev1.StashWithdrawRequest{
				NpcName: strings.Join(withFields[:len(withFields)-1], " "),
				Amount:  int32(withAmount),
			}}}, nil
	case command.HandlerStashBalance:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_StashBalance{StashBalance: &gamev1.StashBalanceRequest{NpcName: rawArgs}}}, nil
	case command.HandlerHire:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Hire{Hire: &gamev1.HireRequest{NpcName: rawArgs}}}, nil
	case command.HandlerDismiss:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Dismiss{Dismiss: &gamev1.DismissRequest{}}}, nil
	case command.HandlerTrainJob:
		trainFields := strings.Fields(rawArgs)
		if len(trainFields) < 1 {
			return nil, fmt.Errorf("usage: train <npc> [job]")
		}
		trainJobID := ""
		if len(trainFields) >= 2 {
			trainJobID = strings.Join(trainFields[1:], "_")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_TrainJob{TrainJob: &gamev1.TrainJobRequest{NpcName: trainFields[0], JobId: trainJobID}}}, nil
	case command.HandlerListJobs:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ListJobs{ListJobs: &gamev1.ListJobsRequest{}}}, nil
	case command.HandlerSetJob:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SetJob{SetJob: &gamev1.SetJobRequest{JobId: strings.ReplaceAll(strings.TrimSpace(rawArgs), " ", "_")}}}, nil
	case command.HandlerBribe:
		if len(parsed.Args) > 0 && strings.ToLower(parsed.Args[0]) == "confirm" {
			return &gamev1.ClientMessage{RequestId: reqID,
				Payload: &gamev1.ClientMessage_BribeConfirmRequest{BribeConfirmRequest: &gamev1.BribeConfirmRequest{}}}, nil
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_BribeRequest{BribeRequest: &gamev1.BribeRequest{NpcName: rawArgs}}}, nil
	case command.HandlerSurrender:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SurrenderRequest{SurrenderRequest: &gamev1.SurrenderRequest{}}}, nil
	case command.HandlerRelease:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ReleaseRequest{ReleaseRequest: &gamev1.ReleaseRequest{PlayerName: strings.TrimSpace(rawArgs)}}}, nil

	// Faction commands
	case command.HandlerFaction:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_FactionRequest{FactionRequest: &gamev1.FactionRequest{}}}, nil
	case command.HandlerFactionInfo:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_FactionInfoRequest{FactionInfoRequest: &gamev1.FactionInfoRequest{FactionId: strings.TrimSpace(rawArgs)}}}, nil
	case command.HandlerFactionStanding:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_FactionStandingRequest{FactionStandingRequest: &gamev1.FactionStandingRequest{}}}, nil
	case command.HandlerChangeRep:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ChangeRepRequest{ChangeRepRequest: &gamev1.ChangeRepRequest{FactionId: strings.TrimSpace(rawArgs)}}}, nil

	// Group commands
	case command.HandlerGroup:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Group{Group: &gamev1.GroupRequest{Args: rawArgs}}}, nil
	case command.HandlerInvite:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Invite{Invite: &gamev1.InviteRequest{Player: arg}}}, nil
	case command.HandlerAcceptGroup:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_AcceptGroup{AcceptGroup: &gamev1.AcceptGroupRequest{}}}, nil
	case command.HandlerDeclineGroup:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_DeclineGroup{DeclineGroup: &gamev1.DeclineGroupRequest{}}}, nil
	case command.HandlerUngroup:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Ungroup{Ungroup: &gamev1.UngroupRequest{}}}, nil
	case command.HandlerKick:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Kick{Kick: &gamev1.KickRequest{Player: arg}}}, nil
	case command.HandlerJoin:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Join{Join: &gamev1.JoinRequest{}}}, nil
	case command.HandlerDecline:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Decline{Decline: &gamev1.DeclineRequest{}}}, nil

	// System / character-switch commands
	case command.HandlerSwitch:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SwitchCharacter{SwitchCharacter: &gamev1.SwitchCharacterRequest{}}}, nil
	case command.HandlerHelp:
		// Help is rendered client-side; no server round-trip needed.
		return nil, nil

	// Admin commands
	case command.HandlerSetRole:
		if len(parsed.Args) < 2 {
			return nil, fmt.Errorf("usage: setrole <username> <role>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SetRole{SetRole: &gamev1.SetRoleRequest{
				TargetUsername: parsed.Args[0],
				Role:           parsed.Args[1],
			}}}, nil
	case command.HandlerTeleport:
		if len(parsed.Args) < 2 {
			return nil, fmt.Errorf("usage: teleport <character> <room_id>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Teleport{Teleport: &gamev1.TeleportRequest{
				TargetCharacter: parsed.Args[0],
				RoomId:          parsed.Args[1],
			}}}, nil
	case command.HandlerGrant:
		if len(parsed.Args) < 3 {
			return nil, fmt.Errorf("usage: grant <xp|money|heropoint> <character> <amount>")
		}
		grantType := strings.ToLower(parsed.Args[0])
		grantAmount := int32(1)
		if grantType != "heropoint" {
			n, nErr := strconv.Atoi(parsed.Args[len(parsed.Args)-1])
			if nErr != nil || n <= 0 {
				return nil, fmt.Errorf("usage: grant <xp|money|heropoint> <character> <amount>")
			}
			grantAmount = int32(n)
		}
		grantChar := strings.Join(parsed.Args[1:len(parsed.Args)-1], " ")
		if grantType == "heropoint" {
			grantChar = strings.Join(parsed.Args[1:], " ")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Grant{Grant: &gamev1.GrantRequest{
				GrantType: grantType,
				CharName:  grantChar,
				Amount:    grantAmount,
			}}}, nil

	// Editor commands
	case command.HandlerRoomEquip:
		parts := strings.Fields(rawArgs)
		roomEquipReq := &gamev1.RoomEquipRequest{}
		if len(parts) > 0 {
			roomEquipReq.SubCommand = parts[0]
		}
		if len(parts) > 1 {
			roomEquipReq.ItemId = parts[1]
		}
		if len(parts) > 2 {
			if n, nErr := strconv.Atoi(parts[2]); nErr == nil {
				roomEquipReq.MaxCount = int32(n)
			}
		}
		if len(parts) > 3 {
			roomEquipReq.Respawn = parts[3]
		}
		if len(parts) > 4 {
			roomEquipReq.Immovable = parts[4] == "true"
		}
		if len(parts) > 5 {
			roomEquipReq.Script = strings.Join(parts[5:], " ")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_RoomEquip{RoomEquip: roomEquipReq}}, nil
	case command.HandlerSummonItem:
		summonParsed := command.HandleSummonItem(rawArgs)
		summonParts := strings.Fields(summonParsed)
		if len(summonParts) != 2 {
			return nil, fmt.Errorf("usage: summon_item <item_id> [quantity]")
		}
		summonQty, _ := strconv.Atoi(summonParts[1])
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SummonItem{SummonItem: &gamev1.SummonItemRequest{
				ItemId:   summonParts[0],
				Quantity: int32(summonQty),
			}}}, nil
	case command.HandlerSpawnNPC:
		if len(parsed.Args) < 1 {
			return nil, fmt.Errorf("usage: spawnnpc <template_id> [room_id]")
		}
		spawnRoomID := ""
		if len(parsed.Args) >= 2 {
			spawnRoomID = parsed.Args[1]
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SpawnNpc{SpawnNpc: &gamev1.SpawnNPCRequest{
				TemplateId: parsed.Args[0],
				RoomId:     spawnRoomID,
			}}}, nil
	case command.HandlerAddRoom:
		if len(parsed.Args) < 3 {
			return nil, fmt.Errorf("usage: addroom <zone_id> <room_id> <title>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_AddRoom{AddRoom: &gamev1.AddRoomRequest{
				ZoneId: parsed.Args[0],
				RoomId: parsed.Args[1],
				Title:  strings.Join(parsed.Args[2:], " "),
			}}}, nil
	case command.HandlerAddLink:
		if len(parsed.Args) < 3 {
			return nil, fmt.Errorf("usage: addlink <from_room_id> <direction> <to_room_id>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_AddLink{AddLink: &gamev1.AddLinkRequest{
				FromRoomId: parsed.Args[0],
				Direction:  parsed.Args[1],
				ToRoomId:   parsed.Args[2],
			}}}, nil
	case command.HandlerRemoveLink:
		if len(parsed.Args) < 2 {
			return nil, fmt.Errorf("usage: removelink <room_id> <direction>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_RemoveLink{RemoveLink: &gamev1.RemoveLinkRequest{
				RoomId:    parsed.Args[0],
				Direction: parsed.Args[1],
			}}}, nil
	case command.HandlerSetRoom:
		if len(parsed.Args) < 2 {
			return nil, fmt.Errorf("usage: setroom <field> <value>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SetRoom{SetRoom: &gamev1.SetRoomRequest{
				Field: parsed.Args[0],
				Value: strings.Join(parsed.Args[1:], " "),
			}}}, nil
	case command.HandlerEditorCmds:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_EditorCmds{EditorCmds: &gamev1.EditorCmdsRequest{}}}, nil
	case command.HandlerSpawnChar:
		if len(parsed.Args) == 0 {
			return nil, fmt.Errorf("usage: spawn_char <name>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_SpawnCharRequest{SpawnCharRequest: &gamev1.SpawnCharRequest{
				Name: strings.Join(parsed.Args, " "),
			}}}, nil
	case command.HandlerDeleteChar:
		if len(parsed.Args) == 0 {
			return nil, fmt.Errorf("usage: delete_char <name>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_DeleteCharRequest{DeleteCharRequest: &gamev1.DeleteCharRequest{
				Name: strings.Join(parsed.Args, " "),
			}}}, nil

	// World / navigation
	case command.HandlerTravel:
		travelZone := strings.TrimSpace(rawArgs)
		if travelZone == "" {
			return nil, fmt.Errorf("usage: travel <zone name>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_Travel{Travel: &gamev1.TravelRequest{ZoneId: travelZone}}}, nil

	// Character creation
	case command.HandlerArchetypeSelection:
		if len(parsed.Args) == 0 {
			return nil, nil
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_ArchetypeSelection{ArchetypeSelection: &gamev1.ArchetypeSelectionRequest{
				ArchetypeId: parsed.Args[0],
			}}}, nil

	// Tab completion
	case command.HandlerTabComplete:
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_TabComplete{TabComplete: &gamev1.TabCompleteRequest{
				Prefix: strings.TrimSpace(rawArgs),
			}}}, nil

	// Heal amount (server-side dispatch target, arguments: <npc> <amount>)
	case command.HandlerHealAmount:
		healFields := strings.Fields(rawArgs)
		if len(healFields) < 2 {
			return nil, fmt.Errorf("usage: heal_amount <npc> <amount>")
		}
		healAmt, healErr := strconv.Atoi(healFields[len(healFields)-1])
		if healErr != nil || healAmt <= 0 {
			return nil, fmt.Errorf("usage: heal_amount <npc> <amount>")
		}
		return &gamev1.ClientMessage{RequestId: reqID,
			Payload: &gamev1.ClientMessage_HealAmount{HealAmount: &gamev1.HealAmountRequest{
				NpcName: strings.Join(healFields[:len(healFields)-1], " "),
				Amount:  int32(healAmt),
			}}}, nil

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
		"LoadoutRequest":        func() proto.Message { return &gamev1.LoadoutRequest{} },
		"RestRequest":           func() proto.Message { return &gamev1.RestRequest{} },
		"HotbarRequest":         func() proto.Message { return &gamev1.HotbarRequest{} },
		"UseEquipmentRequest":   func() proto.Message { return &gamev1.UseEquipmentRequest{} },
		"BrowseRequest":         func() proto.Message { return &gamev1.BrowseRequest{} },
		"BuyRequest":             func() proto.Message { return &gamev1.BuyRequest{} },
		"HealRequest":            func() proto.Message { return &gamev1.HealRequest{} },
		"TrainJobRequest":        func() proto.Message { return &gamev1.TrainJobRequest{} },
		"TravelRequest":          func() proto.Message { return &gamev1.TravelRequest{} },
		"StashDepositRequest":    func() proto.Message { return &gamev1.StashDepositRequest{} },
		"StashWithdrawRequest":   func() proto.Message { return &gamev1.StashWithdrawRequest{} },
		"StashBalanceRequest":    func() proto.Message { return &gamev1.StashBalanceRequest{} },
		"TakeCoverRequest":       func() proto.Message { return &gamev1.TakeCoverRequest{} },
		"UncoverRequest":         func() proto.Message { return &gamev1.UncoverRequest{} },
		"EquipRequest":           func() proto.Message { return &gamev1.EquipRequest{} },
		"WearRequest":            func() proto.Message { return &gamev1.WearRequest{} },
		"LevelUpRequest":         func() proto.Message { return &gamev1.LevelUpRequest{} },
		"TrainSkillRequest":      func() proto.Message { return &gamev1.TrainSkillRequest{} },
		"JobGrantsRequest":       func() proto.Message { return &gamev1.JobGrantsRequest{} },
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
	case *gamev1.LoadoutRequest:
		cm.Payload = &gamev1.ClientMessage_Loadout{Loadout: m}
	case *gamev1.RestRequest:
		cm.Payload = &gamev1.ClientMessage_Rest{Rest: m}
	case *gamev1.HotbarRequest:
		cm.Payload = &gamev1.ClientMessage_HotbarRequest{HotbarRequest: m}
	case *gamev1.UseEquipmentRequest:
		cm.Payload = &gamev1.ClientMessage_UseEquipment{UseEquipment: m}
	case *gamev1.BrowseRequest:
		cm.Payload = &gamev1.ClientMessage_Browse{Browse: m}
	case *gamev1.BuyRequest:
		cm.Payload = &gamev1.ClientMessage_Buy{Buy: m}
	case *gamev1.HealRequest:
		cm.Payload = &gamev1.ClientMessage_Heal{Heal: m}
	case *gamev1.TrainJobRequest:
		cm.Payload = &gamev1.ClientMessage_TrainJob{TrainJob: m}
	case *gamev1.TravelRequest:
		cm.Payload = &gamev1.ClientMessage_Travel{Travel: m}
	case *gamev1.StashDepositRequest:
		cm.Payload = &gamev1.ClientMessage_StashDeposit{StashDeposit: m}
	case *gamev1.StashWithdrawRequest:
		cm.Payload = &gamev1.ClientMessage_StashWithdraw{StashWithdraw: m}
	case *gamev1.StashBalanceRequest:
		cm.Payload = &gamev1.ClientMessage_StashBalance{StashBalance: m}
	case *gamev1.TakeCoverRequest:
		cm.Payload = &gamev1.ClientMessage_TakeCover{TakeCover: m}
	case *gamev1.UncoverRequest:
		cm.Payload = &gamev1.ClientMessage_UncoverRequest{UncoverRequest: m}
	case *gamev1.EquipRequest:
		cm.Payload = &gamev1.ClientMessage_Equip{Equip: m}
	case *gamev1.WearRequest:
		cm.Payload = &gamev1.ClientMessage_Wear{Wear: m}
	case *gamev1.LevelUpRequest:
		cm.Payload = &gamev1.ClientMessage_LevelUp{LevelUp: m}
	case *gamev1.TrainSkillRequest:
		cm.Payload = &gamev1.ClientMessage_TrainSkill{TrainSkill: m}
	case *gamev1.JobGrantsRequest:
		cm.Payload = &gamev1.ClientMessage_JobGrantsRequest{JobGrantsRequest: m}
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
