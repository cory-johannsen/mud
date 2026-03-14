package command

// HeroPointResult is the parsed form of the heropoint command.
type HeroPointResult struct {
	// Subcommand is one of "reroll" or "stabilize". Empty when Error is set.
	Subcommand string
	// Error is a usage or validation message to display to the player.
	Error string
}

// HandleHeroPoint parses the heropoint command arguments.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a HeroPointResult with either a valid Subcommand or a non-empty Error; err is always nil.
func HandleHeroPoint(args []string) (HeroPointResult, error) {
	if len(args) == 0 {
		return HeroPointResult{Error: "Usage: heropoint <reroll|stabilize>"}, nil
	}
	switch args[0] {
	case "reroll":
		return HeroPointResult{Subcommand: "reroll"}, nil
	case "stabilize":
		return HeroPointResult{Subcommand: "stabilize"}, nil
	default:
		return HeroPointResult{Error: "unknown subcommand '" + args[0] + "': use 'reroll' or 'stabilize'"}, nil
	}
}
