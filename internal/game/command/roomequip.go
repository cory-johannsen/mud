package command

import "strings"

// HandleRoomEquip returns a plain-text acknowledgment for the roomequip command.
// Actual CRUD executes server-side; this function validates the subcommand client-side.
//
// Precondition: args may be any string.
// Postcondition: Returns a non-empty human-readable string.
func HandleRoomEquip(args string) string {
	if args == "" {
		return roomEquipUsage()
	}
	switch firstWord(args) {
	case "add", "remove", "list", "modify":
		return "roomequip " + args
	default:
		return roomEquipUsage()
	}
}

// roomEquipUsage returns the canonical usage string for the roomequip command.
func roomEquipUsage() string {
	return "Usage: roomequip <add|remove|list|modify> [item_id] [max_count] [respawn] [immovable] [script]"
}

// firstWord returns the first whitespace-delimited word in s.
// If s contains no whitespace, firstWord returns s unchanged.
func firstWord(s string) string {
	if idx := strings.IndexByte(s, ' '); idx >= 0 {
		return s[:idx]
	}
	return s
}
