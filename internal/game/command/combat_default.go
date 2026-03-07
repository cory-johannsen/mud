package command

import (
	"fmt"
	"strings"
)

var validCombatActionsSet = map[string]bool{
	"attack": true,
	"strike": true,
	"bash":   true,
	"dodge":  true,
	"parry":  true,
	"cast":   true,
	"pass":   true,
	"flee":   true,
}

// ValidCombatActions is exported so tests and other packages can reference the full list.
var ValidCombatActions = []string{"attack", "strike", "bash", "dodge", "parry", "cast", "pass", "flee"}

// HandleCombatDefault validates and normalizes the requested default combat action.
//
// Precondition: args must be non-nil.
// Postcondition: returns the normalized action string and nil on success;
// returns an empty string and non-nil error if args is empty or the action is invalid.
func HandleCombatDefault(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: combat_default <action>  (actions: %s)", strings.Join(ValidCombatActions, ", "))
	}
	action := strings.ToLower(strings.TrimSpace(args[0]))
	if !validCombatActionsSet[action] {
		return "", fmt.Errorf("invalid action %q; valid: %s", action, strings.Join(ValidCombatActions, ", "))
	}
	return action, nil
}
