// Package command provides the command registry, parser, and built-in command definitions.
package command

// Categories for organizing commands.
const (
	CategoryMovement      = "movement"
	CategoryWorld         = "world"
	CategoryCombat        = "combat"
	CategoryCommunication = "communication"
	CategorySystem        = "system"
	CategoryAdmin         = "admin"
)

// Handler identifiers mapping commands to gRPC message types.
const (
	HandlerMove      = "move"
	HandlerLook      = "look"
	HandlerExits     = "exits"
	HandlerSay       = "say"
	HandlerEmote     = "emote"
	HandlerWho       = "who"
	HandlerQuit      = "quit"
	HandlerHelp      = "help"
	HandlerExamine   = "examine"
	HandlerAttack    = "attack"
	HandlerFlee      = "flee"
	HandlerPass      = "pass"
	HandlerStrike    = "strike"
	HandlerStatus    = "status"
	HandlerEquip     = "equip"
	HandlerReload    = "reload"
	HandlerFireBurst = "burst"
	HandlerFireAuto  = "auto"
	HandlerThrow     = "throw"
	HandlerInventory = "inventory"
	HandlerGet       = "get"
	HandlerDrop      = "drop"
	HandlerBalance   = "balance"
	HandlerSetRole   = "setrole"
	HandlerTeleport  = "teleport"
	HandlerLoadout   = "loadout"
	HandlerUnequip   = "unequip"
	HandlerEquipment = "equipment"
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
		{Name: "loadout", Aliases: []string{"lo"}, Help: "Display or swap weapon presets (loadout [1|2])", Category: CategoryCombat, Handler: HandlerLoadout},
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

		// Communication commands
		{Name: "say", Aliases: nil, Help: "Say something to the room", Category: CategoryCommunication, Handler: HandlerSay},
		{Name: "emote", Aliases: []string{"em"}, Help: "Perform an emote action", Category: CategoryCommunication, Handler: HandlerEmote},

		// System commands
		{Name: "who", Aliases: nil, Help: "List players in the room", Category: CategorySystem, Handler: HandlerWho},
		{Name: "quit", Aliases: []string{"exit"}, Help: "Disconnect from the game", Category: CategorySystem, Handler: HandlerQuit},
		{Name: "help", Aliases: []string{"?"}, Help: "Show available commands", Category: CategorySystem, Handler: HandlerHelp},

		// Admin commands
		{Name: "setrole", Aliases: nil, Help: "Set a player's role (admin only)", Category: CategoryAdmin, Handler: HandlerSetRole},
		{Name: "teleport", Aliases: []string{"tp"}, Help: "Teleport a player to a room (admin only)", Category: CategoryAdmin, Handler: HandlerTeleport},
	}
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
