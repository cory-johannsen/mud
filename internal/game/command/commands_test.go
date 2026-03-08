package command

import (
	"slices"
	"testing"
)

func TestHandlerAction_InBuiltinCommands(t *testing.T) {
	cmds := BuiltinCommands()
	var found bool
	for _, c := range cmds {
		if c.Handler == HandlerAction {
			found = true
			if c.Name != "action" {
				t.Errorf("action command name: got %q, want %q", c.Name, "action")
			}
			if !slices.Contains(c.Aliases, "act") {
				t.Errorf("action command aliases: %v does not contain \"act\"", c.Aliases)
			}
		}
	}
	if !found {
		t.Error("HandlerAction not found in BuiltinCommands")
	}
}
