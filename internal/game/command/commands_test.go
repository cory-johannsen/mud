package command

import (
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
		}
	}
	if !found {
		t.Error("HandlerAction not found in BuiltinCommands")
	}
}
