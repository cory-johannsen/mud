package command

import (
	"slices"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestRegisterShortcuts_NoDuplicates(t *testing.T) {
	features := []*ruleset.ClassFeature{
		{ID: "surge", Active: true, Shortcut: "surge", Name: "Brutal Surge"},
		{ID: "patch", Active: true, Shortcut: "patch", Name: "Patch Job"},
	}
	base := BuiltinCommands()
	cmds := RegisterShortcuts(features, base)
	// Verify no duplicate command names
	names := map[string]int{}
	for _, c := range cmds {
		names[c.Name]++
	}
	for name, count := range names {
		if count > 1 {
			t.Errorf("duplicate command name %q appears %d times", name, count)
		}
	}
	// Verify shortcuts are added
	found := map[string]bool{}
	for _, c := range cmds {
		found[c.Name] = true
	}
	if !found["surge"] {
		t.Error("shortcut 'surge' not found in commands")
	}
	if !found["patch"] {
		t.Error("shortcut 'patch' not found in commands")
	}
}

func TestRegisterShortcuts_SkipsInactiveAndEmptyShortcut(t *testing.T) {
	features := []*ruleset.ClassFeature{
		{ID: "passive_feat", Active: false, Shortcut: "passive"},
		{ID: "no_shortcut", Active: true, Shortcut: ""},
		{ID: "active_with_shortcut", Active: true, Shortcut: "myaction", Name: "My Action"},
	}
	base := BuiltinCommands()
	cmds := RegisterShortcuts(features, base)
	found := map[string]bool{}
	for _, c := range cmds {
		found[c.Name] = true
	}
	if found["passive"] {
		t.Error("inactive feature shortcut 'passive' should not be registered")
	}
	if found[""] {
		t.Error("empty shortcut should not be registered")
	}
	if !found["myaction"] {
		t.Error("active feature shortcut 'myaction' not found")
	}
}

func TestRegisterShortcuts_PanicOnCollision(t *testing.T) {
	features := []*ruleset.ClassFeature{
		{ID: "fake_attack", Active: true, Shortcut: "attack", Name: "Fake Attack"}, // "attack" already exists
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on shortcut collision, got none")
		}
	}()
	RegisterShortcuts(features, BuiltinCommands())
}

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
