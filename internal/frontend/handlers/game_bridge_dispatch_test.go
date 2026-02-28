package handlers_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/cory-johannsen/mud/internal/game/command"
)

// TestAllCommandHandlersAreWired asserts that every Handler constant
// registered in BuiltinCommands has a corresponding entry in the bridge
// dispatch map. Adding a new command to commands.go MUST be accompanied
// by a bridge handler entry or this test fails.
//
// Precondition: none.
// Postcondition: every cmd.Handler in BuiltinCommands() is a key in BridgeHandlers().
func TestAllCommandHandlersAreWired(t *testing.T) {
	registered := handlers.BridgeHandlers()
	for _, cmd := range command.BuiltinCommands() {
		if _, ok := registered[cmd.Handler]; !ok {
			t.Errorf("handler %q is in BuiltinCommands() but missing from BridgeHandlers() — add it to bridge_handlers.go", cmd.Handler)
		}
	}
}
