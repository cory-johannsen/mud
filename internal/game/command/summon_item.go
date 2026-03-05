package command

import (
	"fmt"
	"strconv"
	"strings"
)

// HandleSummonItem parses and validates summon_item command arguments.
// Precondition: args is the raw argument string after the command name (may be empty).
// Postcondition: returns "itemID qty" (space-separated) on valid input,
// or the usage string on invalid input.
func HandleSummonItem(args string) string {
	const usage = "Usage: summon_item <item_id> [quantity]"
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return usage
	}
	itemID := parts[0]
	if len(parts) > 2 {
		return usage
	}
	qty := 1
	if len(parts) >= 2 {
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 1 {
			return usage
		}
		qty = n
	}
	return fmt.Sprintf("%s %d", itemID, qty)
}
