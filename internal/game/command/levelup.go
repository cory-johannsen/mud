package command

import (
	"fmt"
	"strings"
)

// ValidAbilities is the set of ability names accepted by HandleLevelUp.
var ValidAbilities = []string{"brutality", "quickness", "grit", "reasoning", "savvy", "flair"}

// validAbilities is the O(1) lookup set built from ValidAbilities.
var validAbilities = func() map[string]bool {
	m := make(map[string]bool, len(ValidAbilities))
	for _, a := range ValidAbilities {
		m[a] = true
	}
	return m
}()

var abilityList = strings.Join(ValidAbilities, ", ")

// HandleLevelUp parses and validates a levelup command argument.
//
// Precondition: rawArgs is the raw argument string after the command name (may be empty).
// Postcondition: returns the normalized (lowercase) ability name on valid input;
// returns a usage string if rawArgs is empty;
// returns an error string if the ability name is unknown.
func HandleLevelUp(rawArgs string) string {
	ability := strings.ToLower(strings.TrimSpace(rawArgs))
	if ability == "" {
		return fmt.Sprintf("Usage: levelup <ability>\nValid abilities: %s", abilityList)
	}
	if !validAbilities[ability] {
		return fmt.Sprintf("Unknown ability '%s'. Valid: %s", ability, abilityList)
	}
	return ability
}
