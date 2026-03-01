package command_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleSwitch_ReturnsEmptyString(t *testing.T) {
	result := command.HandleSwitch([]string{})
	if result != "" {
		t.Errorf("HandleSwitch() = %q, want empty string", result)
	}
}

func TestProperty_HandleSwitch_AlwaysEmptyString(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(t, "args")
		result := command.HandleSwitch(args)
		if result != "" {
			t.Fatalf("HandleSwitch(%v) = %q, want empty string", args, result)
		}
	})
}
