// Package command provides the command registry, parser, and built-in command definitions.
package command

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// Categories for organizing commands.
const (
	CategoryMovement      = "movement"
	CategoryWorld         = "world"
	CategoryCombat        = "combat"
	CategoryCommunication = "communication"
	CategorySystem        = "system"
	CategoryAdmin         = "admin"
	CategoryCharacter     = "character"
	// CategoryHidden marks commands that are internal flow steps and must not appear in help.
	CategoryHidden = "hidden"
	// CategoryEditor marks commands available to editor and admin roles.
	CategoryEditor = "Editor"
)

// Handler identifiers mapping commands to gRPC message types.
const (
	HandlerMove               = "move"
	HandlerLook               = "look"
	HandlerExits              = "exits"
	HandlerSay                = "say"
	HandlerEmote              = "emote"
	HandlerWho                = "who"
	HandlerQuit               = "quit"
	HandlerHelp               = "help"
	HandlerExamine            = "examine"
	HandlerAttack             = "attack"
	HandlerFlee               = "flee"
	HandlerPass               = "pass"
	HandlerStrike             = "strike"
	HandlerStatus             = "status"
	HandlerEquip              = "equip"
	HandlerReload             = "reload"
	HandlerFireBurst          = "burst"
	HandlerFireAuto           = "auto"
	HandlerThrow              = "throw"
	HandlerInventory          = "inventory"
	HandlerGet                = "get"
	HandlerDrop               = "drop"
	HandlerBalance            = "balance"
	HandlerSetRole            = "setrole"
	HandlerTeleport           = "teleport"
	HandlerLoadout            = "loadout"
	HandlerUnequip            = "unequip"
	HandlerEquipment          = "equipment"
	HandlerSwitch             = "switch"
	HandlerWear               = "wear"
	HandlerRemoveArmor        = "remove"
	HandlerChar               = "char"
	HandlerArchetypeSelection = "archetype_selection"
	HandlerUseEquipment       = "use_equipment"
	HandlerRoomEquip          = "room_equip"
	HandlerMap                = "map"
	HandlerTravel             = "travel"
	HandlerSkills             = "skills"
	HandlerFeats              = "feats"
	HandlerClassFeatures      = "class_features"
	HandlerInteract           = "interact"
	HandlerUse                = "use"
	HandlerSummonItem         = "summon_item"
	HandlerProficiencies      = "proficiencies"
	HandlerLevelUp            = "levelup"
	HandlerCombatDefault      = "combat_default"
	HandlerTrainSkill         = "trainskill"
	HandlerAction             = "action"
	HandlerRaiseShield        = "raise_shield"
	HandlerTakeCover          = "take_cover"
	HandlerFirstAid           = "first_aid"
	HandlerFeint              = "feint"
	HandlerDemoralize         = "demoralize"
	HandlerGrapple            = "grapple"
	HandlerTrip               = "trip"
	HandlerDelay              = "delay"
	HandlerDisarm             = "disarm"
	HandlerDisarmTrap         = "disarm_trap"
	HandlerDeployTrap         = "deploy_trap"
	HandlerStride             = "stride"
	HandlerHide               = "hide"
	HandlerSneak              = "sneak"
	HandlerDivert             = "divert"
	HandlerEscape             = "escape"
	HandlerGrant              = "grant"
	HandlerShove              = "shove"
	HandlerStep               = "step"
	HandlerTumble             = "tumble"
	HandlerSeek               = "seek"
	HandlerMotive             = "motive"
	HandlerClimb              = "climb"
	HandlerSwim               = "swim"
	HandlerCalm               = "calm"
	HandlerHeroPoint          = "heropoint"
	HandlerJoin               = "join"
	HandlerDecline            = "decline"
	HandlerGroup              = "group"
	HandlerInvite             = "invite"
	HandlerAcceptGroup        = "acceptgroup"
	HandlerDeclineGroup       = "declinegroup"
	HandlerUngroup            = "ungroup"
	HandlerKick               = "kick"
	HandlerRest               = "rest"
	HandlerSelectTech         = "selecttech"
	HandlerAid                = "aid"
	HandlerReady              = "ready"
	HandlerTalk               = "talk"
	HandlerHeal               = "heal"
	HandlerHealAmount         = "heal_amount"
	HandlerBrowse             = "browse"
	HandlerBuy                = "buy"
	HandlerSell               = "sell"
	HandlerNegotiate          = "negotiate"
	HandlerDeposit            = "deposit"
	HandlerWithdraw           = "withdraw"
	HandlerStashBalance       = "stash_balance"
	HandlerHire               = "hire"
	HandlerDismiss            = "dismiss"
	HandlerTrainJob           = "train_job"
	HandlerListJobs           = "list_jobs"
	HandlerSetJob             = "setjob"
	HandlerBribe              = "bribe"
	HandlerSurrender          = "surrender"
	HandlerRelease            = "release"
	HandlerSpawnNPC           = "spawn_npc"
	HandlerAddRoom            = "add_room"
	HandlerAddLink            = "add_link"
	HandlerRemoveLink         = "remove_link"
	HandlerSetRoom            = "set_room"
	HandlerEditorCmds         = "ecmds"
)

// Command defines a player-invocable command.
type Command struct {
	// Name is the canonical command name.
	Name string
	// Aliases are alternate names for this command.
	Aliases []string
	// Help is the short help text displayed to players.
	Help string
	// Category groups the command (movement, world, communication, system).
	Category string
	// Handler maps to the gRPC message type or local handler.
	Handler string
}

// BuiltinCommands returns all built-in commands for the game.
func BuiltinCommands() []Command {
	return []Command{
		// Movement commands
		{Name: "north", Aliases: []string{"n"}, Help: "Move north", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "south", Aliases: []string{"s"}, Help: "Move south", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "east", Aliases: []string{"e"}, Help: "Move east", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "west", Aliases: []string{"w"}, Help: "Move west", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "northeast", Aliases: []string{"ne"}, Help: "Move northeast", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "northwest", Aliases: []string{"nw"}, Help: "Move northwest", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "southeast", Aliases: []string{"se"}, Help: "Move southeast", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "southwest", Aliases: []string{"sw"}, Help: "Move southwest", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "up", Aliases: []string{"u"}, Help: "Move up", Category: CategoryMovement, Handler: HandlerMove},
		{Name: "down", Aliases: []string{"d"}, Help: "Move down", Category: CategoryMovement, Handler: HandlerMove},

		// World commands
		{Name: "look", Aliases: []string{"l"}, Help: "Look around the current room", Category: CategoryWorld, Handler: HandlerLook},
		{Name: "exits", Aliases: nil, Help: "List available exits", Category: CategoryWorld, Handler: HandlerExits},
		{Name: "examine", Aliases: []string{"ex"}, Help: "Examine an NPC or object in the room", Category: CategoryWorld, Handler: HandlerExamine},
		{Name: "attack", Aliases: []string{"att", "kill"}, Help: "Attack a target", Category: CategoryWorld, Handler: HandlerAttack},
		{Name: "flee", Aliases: []string{"run"}, Help: "Attempt to flee combat", Category: CategoryWorld, Handler: HandlerFlee},
		{Name: "pass", Aliases: []string{"p"}, Help: "Forfeit remaining action points this round.", Category: CategoryCombat, Handler: HandlerPass},
		{Name: "strike", Aliases: []string{"st"}, Help: "Full attack routine (2 AP, two hits with MAP) against target.", Category: CategoryCombat, Handler: HandlerStrike},
		{Name: "status", Aliases: []string{"cond"}, Help: "Show your active conditions.", Category: CategoryCombat, Handler: HandlerStatus},
		{Name: "equip", Aliases: []string{"eq"}, Help: "Equip a weapon (equip <weapon_id> [slot])", Category: CategoryCombat, Handler: HandlerEquip},
		{Name: "loadout", Aliases: []string{"lo", "prep", "kit"}, Help: "Display or swap weapon presets (loadout [1|2])", Category: CategoryCombat, Handler: HandlerLoadout},
		{Name: "unequip", Aliases: []string{"ueq"}, Help: "Unequip an item from a slot (unequip <slot>)", Category: CategoryCombat, Handler: HandlerUnequip},
		{Name: "equipment", Aliases: []string{"gear"}, Help: "Show all equipped items", Category: CategoryCombat, Handler: HandlerEquipment},
		{Name: "reload", Aliases: []string{"rl"}, Help: "Reload equipped weapon (1 AP)", Category: CategoryCombat, Handler: HandlerReload},
		{Name: "burst", Aliases: []string{"bf"}, Help: "Burst fire at target (2 AP, 2 attacks)", Category: CategoryCombat, Handler: HandlerFireBurst},
		{Name: "auto", Aliases: []string{"af"}, Help: "Automatic fire at all enemies (3 AP)", Category: CategoryCombat, Handler: HandlerFireAuto},
		{Name: "throw", Aliases: []string{"gr"}, Help: "Throw an explosive at current room", Category: CategoryCombat, Handler: HandlerThrow},
		{Name: "inventory", Aliases: []string{"inv", "i"}, Help: "Show backpack contents and currency", Category: CategoryWorld, Handler: HandlerInventory},
		{Name: "get", Aliases: []string{"take"}, Help: "Pick up item from room floor", Category: CategoryWorld, Handler: HandlerGet},
		{Name: "drop", Aliases: nil, Help: "Drop an item from your backpack", Category: CategoryWorld, Handler: HandlerDrop},
		{Name: "balance", Aliases: []string{"bal"}, Help: "Show your currency (Rounds/Clips/Crates)", Category: CategoryWorld, Handler: HandlerBalance},

		// NPC interaction commands
		{Name: "talk", Aliases: nil, Help: "Talk to a quest giver NPC (talk <npc>)", Category: CategoryWorld, Handler: HandlerTalk},
		{Name: "heal", Aliases: nil, Help: "Ask a healer to fully restore your HP (heal <npc>) or a specific amount (heal <npc> <amount>)", Category: CategoryWorld, Handler: HandlerHeal},
		{Name: "browse", Aliases: nil, Help: "Browse a merchant's inventory (browse <npc>)", Category: CategoryWorld, Handler: HandlerBrowse},
		{Name: "buy", Aliases: nil, Help: "Buy an item from a merchant (buy <npc> <item> [qty])", Category: CategoryWorld, Handler: HandlerBuy},
		{Name: "sell", Aliases: nil, Help: "Sell an item to a merchant (sell <npc> <item> [qty])", Category: CategoryWorld, Handler: HandlerSell},
		{Name: "negotiate", Aliases: []string{"neg"}, Help: "Negotiate prices with a merchant (negotiate <npc> [smooth_talk|grift])", Category: CategoryWorld, Handler: HandlerNegotiate},
		{Name: "deposit", Aliases: nil, Help: "Deposit credits with a banker (deposit <npc> <amount>)", Category: CategoryWorld, Handler: HandlerDeposit},
		{Name: "withdraw", Aliases: nil, Help: "Withdraw credits from a banker (withdraw <npc> <amount>)", Category: CategoryWorld, Handler: HandlerWithdraw},
		{Name: "stash", Aliases: []string{"stashbal"}, Help: "Check your stash balance at a banker (stash <npc>)", Category: CategoryWorld, Handler: HandlerStashBalance},
		{Name: "hire", Aliases: nil, Help: "Hire a hireling NPC (hire <npc>)", Category: CategoryWorld, Handler: HandlerHire},
		{Name: "dismiss", Aliases: nil, Help: "Dismiss your current hireling", Category: CategoryWorld, Handler: HandlerDismiss},
		{Name: "train", Aliases: nil, Help: "Train a job with a job trainer NPC (train <npc> <job>)", Category: CategoryWorld, Handler: HandlerTrainJob},
		{Name: "jobs", Aliases: nil, Help: "List your current jobs", Category: CategoryCharacter, Handler: HandlerListJobs},
		{Name: "setjob", Aliases: nil, Help: "Set your active job (setjob <job>)", Category: CategoryCharacter, Handler: HandlerSetJob},
		{Name: "bribe", Aliases: nil, Help: "Bribe law enforcement to reduce wanted level (bribe [npc]) or confirm a pending bribe (bribe confirm)", Category: CategoryWorld, Handler: HandlerBribe},
		{Name: "surrender", Aliases: nil, Help: "Surrender to law enforcement in the current room", Category: CategoryWorld, Handler: HandlerSurrender},
		{Name: "release", Aliases: nil, Help: "Release a detained player (release <player>)", Category: CategoryWorld, Handler: HandlerRelease},

		// Communication commands
		{Name: "say", Aliases: nil, Help: "Say something to the room", Category: CategoryCommunication, Handler: HandlerSay},
		{Name: "emote", Aliases: []string{"em"}, Help: "Perform an emote action", Category: CategoryCommunication, Handler: HandlerEmote},

		// System commands
		{Name: "who", Aliases: nil, Help: "List players in the room", Category: CategorySystem, Handler: HandlerWho},
		{Name: "quit", Aliases: []string{"exit"}, Help: "Disconnect from the game", Category: CategorySystem, Handler: HandlerQuit},
		{Name: "switch", Aliases: nil, Help: "Switch to a different character without disconnecting.", Category: CategorySystem, Handler: HandlerSwitch},
		{Name: "help", Aliases: []string{"?"}, Help: "Show available commands", Category: CategorySystem, Handler: HandlerHelp},

		// Armor commands
		{Name: "wear", Aliases: nil, Help: "Equip a piece of armor from your inventory (wear <item_id> <slot>)", Category: CategoryCombat, Handler: HandlerWear},
		{Name: "remove", Aliases: []string{"rem"}, Help: "Remove a piece of armor and return it to inventory (remove <slot>)", Category: CategoryCombat, Handler: HandlerRemoveArmor},

		{Name: "char", Aliases: []string{"sheet"}, Help: "Display your character sheet", Category: CategoryWorld, Handler: HandlerChar},

		{Name: "archetype_selection", Aliases: nil, Help: "Select archetype during character creation", Category: CategoryHidden, Handler: HandlerArchetypeSelection},
		{Name: "use", Aliases: nil, Help: "Activate an active feat. With no argument, shows a list.", Category: CategoryWorld, Handler: HandlerUse},
		{Name: "interact", Aliases: []string{"int"}, Help: "Interact with an item in the room.", Category: CategoryWorld, Handler: HandlerInteract},

		// Admin commands
		{Name: "setrole", Aliases: nil, Help: "Set a player's role (admin only)", Category: CategoryAdmin, Handler: HandlerSetRole},
		{Name: "teleport", Aliases: []string{"tp"}, Help: "Teleport a player to a room (admin only)", Category: CategoryAdmin, Handler: HandlerTeleport},
		{Name: "roomequip", Aliases: nil, Help: "Manage room equipment (editor)", Category: CategoryEditor, Handler: HandlerRoomEquip},
		{Name: "grant", Aliases: nil, Help: "Grant XP or money to a player (editor)", Category: CategoryEditor, Handler: HandlerGrant},

		// Editor commands (REQ-EC-9,18,21,23,25,27)
		{Name: "spawnnpc", Aliases: nil, Help: "Spawn an NPC from template into a room", Category: CategoryEditor, Handler: HandlerSpawnNPC},
		{Name: "addroom", Aliases: nil, Help: "Add a new room to a zone", Category: CategoryEditor, Handler: HandlerAddRoom},
		{Name: "addlink", Aliases: nil, Help: "Add a bidirectional exit between two rooms", Category: CategoryEditor, Handler: HandlerAddLink},
		{Name: "removelink", Aliases: nil, Help: "Remove a directional exit from a room", Category: CategoryEditor, Handler: HandlerRemoveLink},
		{Name: "setroom", Aliases: nil, Help: "Set a field on the current room", Category: CategoryEditor, Handler: HandlerSetRoom},
		{Name: "ecmds", Aliases: nil, Help: "List all editor commands", Category: CategoryEditor, Handler: HandlerEditorCmds},

		{Name: "map", Aliases: nil, Help: "Display your automap for the current zone", Category: CategoryWorld, Handler: HandlerMap},
		{Name: "travel", Aliases: nil, Help: "Fast travel to a discovered zone (travel <zone name>)", Category: CategoryWorld, Handler: HandlerTravel},
		{Name: "skills", Aliases: []string{"sk"}, Help: "Display your skill proficiencies.", Category: CategoryWorld, Handler: HandlerSkills},
		{Name: "feats", Aliases: []string{"ft"}, Help: "Display your feats.", Category: CategoryWorld, Handler: HandlerFeats},
		{Name: HandlerClassFeatures, Aliases: []string{"cf"}, Help: "List your class features", Category: CategoryCharacter, Handler: HandlerClassFeatures},
		{Name: "proficiencies", Aliases: []string{"prof"}, Help: "Display your armor and weapon proficiencies.", Category: CategoryCharacter, Handler: HandlerProficiencies},

		{Name: "summon_item", Aliases: nil, Help: "Summon an item into the current room (editor+)", Category: CategoryEditor, Handler: HandlerSummonItem},
		{Name: "levelup", Aliases: []string{"lu"}, Help: "Assign a pending ability boost to the named ability", Category: CategoryCharacter, Handler: HandlerLevelUp},
		{Name: "combat_default", Aliases: []string{"cd"}, Help: "Set your default combat action (attack/strike/bash/dodge/parry/cast/pass/flee)", Category: CategoryCombat, Handler: HandlerCombatDefault},
		{Name: "trainskill", Aliases: []string{"ts"}, Help: "Advance a skill proficiency rank using a pending skill increase", Category: CategoryCharacter, Handler: HandlerTrainSkill},
		{Name: "action", Aliases: []string{"act"}, Help: "Activate an archetype or job action. Usage: action [name] [target]", Category: CategoryCombat, Handler: HandlerAction},
		{Name: "raise", Aliases: []string{"rs"}, Help: "Raise your shield (+2 AC until start of next turn). Requires a shield in the off-hand slot.", Category: CategoryCombat, Handler: HandlerRaiseShield},
		{Name: "cover", Aliases: []string{"tc"}, Help: "Take cover (+2 AC for the encounter). Costs 1 AP in combat.", Category: CategoryCombat, Handler: HandlerTakeCover},
		{Name: "aid", Aliases: []string{"fa"}, Help: "Aid an ally (DC 20 check; crit +3, success +2, fail 0, crit fail -1 to ally attack). Costs 2 AP.", Category: CategoryCombat, Handler: HandlerAid},
		{Name: "feint", Aliases: nil, Help: "Feint against a target (grift vs Perception DC; success applies flat_footed -2 AC for 1 round). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerFeint},
		{Name: "demoralize", Aliases: []string{"dem"}, Help: "Demoralize a target (smooth_talk vs Level+10 DC; success applies -1 AC and -1 attack for the encounter). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerDemoralize},
		{Name: "grapple", Aliases: []string{"grp"}, Help: "Grapple a target (muscle vs Level+10 DC; success applies grabbed condition, target is -2 AC for encounter). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerGrapple},
		{Name: "trip", Aliases: []string{"trp"}, Help: "Trip a target (muscle vs Level+10 DC; success applies prone, -2 attack for encounter). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerTrip},
		{Name: "disarm", Aliases: []string{"dsm"}, Help: "Disarm a target (muscle vs Level+10 DC; success removes NPC weapon and drops it to the floor). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerDisarm},
		{Name: "disarm_trap", Aliases: []string{"dt"}, Help: "Disarm a detected trap by name. Must have previously detected the trap (Search mode). Uses Thievery.", Category: CategoryCombat, Handler: HandlerDisarmTrap},
		{Name: "deploy_trap", Aliases: []string{"deploy"}, Help: "deploy <item> — arm a trap item at your current position (1 AP in combat)", Category: CategoryCombat, Handler: HandlerDeployTrap},
		{Name: "stride", Aliases: []string{"str"}, Help: "Move toward or away from your target (stride [toward|away]; 1 AP; changes distance by 25ft). Combat only.", Category: CategoryCombat, Handler: HandlerStride},
		{Name: "hide", Aliases: nil, Help: "Attempt to hide (stealth vs highest NPC Perception DC; success applies hidden condition). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerHide},
		{Name: "sneak", Aliases: nil, Help: "Move while hidden (stealth vs highest NPC Perception DC; fail removes hidden). Requires hidden condition. Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerSneak},
		{Name: "divert", Aliases: []string{"div"}, Help: "Create a diversion (grift vs highest NPC Perception DC; success applies hidden condition). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerDivert},
		{Name: "escape", Aliases: []string{"esc"}, Help: "Escape from grabbed condition (max muscle/acrobatics vs DC; success removes grabbed). Requires grabbed. Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerEscape},
		{Name: "shove", Aliases: nil, Help: "Shove a target, pushing them back 5 ft (10 ft on critical success). Requires Athletics check vs target level+10.", Category: CategoryCombat, Handler: HandlerShove},
		{Name: "step", Handler: HandlerStep, Help: "Step 5 ft toward or away from your target. Does not trigger Reactive Strikes.", Category: CategoryCombat},
		{Name: "tumble", Handler: HandlerTumble, Help: "Attempt to tumble through an enemy's space. Acrobatics vs Level+10. Success: 5ft move. Failure: blocked + Reactive Strike.", Category: CategoryCombat},
		{Name: "seek", Handler: HandlerSeek, Help: "Scan for hidden enemies (Perception vs NPC Stealth DC; reveals hidden NPCs for 1 round). Combat only, costs 1 AP.", Category: CategoryCombat},
		{Name: "motive", Aliases: []string{"mot"}, Help: "Read an NPC's intentions (awareness vs Hustle DC; success reveals HP tier in combat). Costs 1 AP in combat.", Category: CategoryCombat, Handler: HandlerMotive},
		{Name: "climb", Aliases: []string{"cl"}, Help: "Climb a climbable surface (muscle vs DC; costs 2 AP in combat).", Category: CategoryMovement, Handler: HandlerClimb},
		{Name: "swim", Aliases: []string{"sm"}, Help: "Swim through water or surface when submerged (muscle vs DC; costs 2 AP in combat).", Category: CategoryMovement, Handler: HandlerSwim},
		{Name: "calm", Help: "Attempt to calm your worst active mental state (Grit check; costs all AP in combat).", Category: CategoryCombat, Handler: HandlerCalm},
		{Name: "heropoint", Aliases: []string{"hp"}, Help: "Spend a hero point (heropoint reroll | heropoint stabilize)", Category: CategoryCharacter, Handler: HandlerHeroPoint},
		{Name: "delay", Aliases: []string{"dl"}, Help: "Bank remaining AP (up to 2) for next round at cost of -2 AC. Combat only.", Category: CategoryCombat, Handler: HandlerDelay},
		{Name: "join", Help: "Join active combat in the current room.", Category: CategoryCombat, Handler: HandlerJoin},
		{Name: "decline", Help: "Decline to join active combat.", Category: CategoryCombat, Handler: HandlerDecline},
		{Name: "group", Help: "Create a group or show group info. 'group' with no args shows current group.", Category: CategoryCommunication, Handler: HandlerGroup},
		{Name: "invite", Help: "Invite a player to your group.", Category: CategoryCommunication, Handler: HandlerInvite},
		{Name: "accept", Help: "Accept a pending group invitation.", Category: CategoryCommunication, Handler: HandlerAcceptGroup},
		{Name: "gdecline", Help: "Decline a pending group invitation.", Category: CategoryCommunication, Handler: HandlerDeclineGroup},
		{Name: "ungroup", Help: "Leave your group. Leaders disband the group for all members.", Category: CategoryCommunication, Handler: HandlerUngroup},
		{Name: "kick", Help: "Kick a player from your group (leader only).", Category: CategoryCommunication, Handler: HandlerKick},
		{Name: "rest", Help: "Rest to rearrange your prepared technology slots.", Category: CategoryCharacter, Handler: HandlerRest},
		{Name: "selecttech", Help: "Select pending technology upgrades from levelling up.", Category: CategoryCharacter, Handler: HandlerSelectTech},
		{Name: "ready", Aliases: []string{"rdy"}, Help: "ready <action> when <trigger> — ready a reaction (2 AP); actions: strike/step/shield; triggers: enters/attacks/ally", Category: CategoryCombat, Handler: HandlerReady},
	}
}

// RegisterShortcuts builds shortcut Command entries for active class features and
// appends them to the existing commands slice. Panics at startup if any shortcut
// collides with an existing command name or alias.
//
// Precondition: features must be non-nil; existing must be the current command slice.
// Postcondition: Returns extended slice with one entry per non-empty active shortcut;
// panics on collision.
func RegisterShortcuts(features []*ruleset.ClassFeature, existing []Command) []Command {
	names := make(map[string]bool, len(existing))
	for _, c := range existing {
		names[c.Name] = true
		for _, alias := range c.Aliases {
			names[alias] = true
		}
	}
	for _, f := range features {
		if !f.Active || f.Shortcut == "" {
			continue
		}
		if names[f.Shortcut] {
			panic(fmt.Sprintf("command.RegisterShortcuts: shortcut %q (feature %s) collides with existing command", f.Shortcut, f.ID))
		}
		names[f.Shortcut] = true
		existing = append(existing, Command{
			Name:     f.Shortcut,
			Handler:  HandlerAction,
			Help:     fmt.Sprintf("Shortcut for action %s.", f.Name),
			Category: CategoryCombat,
		})
	}
	return existing
}

// IsMovementCommand reports whether the command name is a movement direction.
func IsMovementCommand(name string) bool {
	switch name {
	case "north", "south", "east", "west",
		"northeast", "northwest", "southeast", "southwest",
		"up", "down":
		return true
	default:
		return false
	}
}
