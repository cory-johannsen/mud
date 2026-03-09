package command

import (
	"fmt"
	"strconv"
	"strings"
)

// HandleGrant validates and passes through a grant command.
// Valid subcommands are "xp" and "money".
// Returns a passthrough string "grant <type> <charname> <amount>" on success,
// or a usage string on invalid input.
//
// Precondition: args is the raw argument string following the "grant" command word.
// Postcondition: Returns a passthrough string or usage string; never empty.
func HandleGrant(args string) string {
	parts := strings.Fields(args)
	if len(parts) < 3 {
		return grantUsage()
	}
	subcommand := strings.ToLower(parts[0])
	if subcommand != "xp" && subcommand != "money" {
		return grantUsage()
	}
	charname := parts[1]
	amountStr := parts[2]
	amount, err := strconv.Atoi(amountStr)
	if err != nil || amount <= 0 {
		return grantUsage()
	}
	return fmt.Sprintf("grant %s %s %d", subcommand, charname, amount)
}

// grantUsage returns the canonical usage string for the grant command.
func grantUsage() string {
	return "Usage: grant <xp|money> <charname> <amount>"
}
