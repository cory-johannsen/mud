package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleInteract_ReturnsNonEmpty(t *testing.T) {
	got := command.HandleInteract()
	if got == "" {
		t.Error("HandleInteract() returned empty string")
	}
}
