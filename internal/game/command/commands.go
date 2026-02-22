// Package command provides the command registry, parser, and built-in command definitions.
package command

// Categories for organizing commands.
const (
	CategoryMovement      = "movement"
	CategoryWorld         = "world"
	CategoryCommunication = "communication"
	CategorySystem        = "system"
)

// Handler identifiers mapping commands to gRPC message types.
const (
	HandlerMove  = "move"
	HandlerLook  = "look"
	HandlerExits = "exits"
	HandlerSay   = "say"
	HandlerEmote = "emote"
	HandlerWho   = "who"
	HandlerQuit  = "quit"
	HandlerHelp    = "help"
	HandlerExamine = "examine"
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

		// Communication commands
		{Name: "say", Aliases: nil, Help: "Say something to the room", Category: CategoryCommunication, Handler: HandlerSay},
		{Name: "emote", Aliases: []string{"em"}, Help: "Perform an emote action", Category: CategoryCommunication, Handler: HandlerEmote},

		// System commands
		{Name: "who", Aliases: nil, Help: "List players in the room", Category: CategorySystem, Handler: HandlerWho},
		{Name: "quit", Aliases: []string{"exit"}, Help: "Disconnect from the game", Category: CategorySystem, Handler: HandlerQuit},
		{Name: "help", Aliases: []string{"?"}, Help: "Show available commands", Category: CategorySystem, Handler: HandlerHelp},
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
