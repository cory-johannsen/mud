package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleUse_ReturnsNonEmpty(t *testing.T) {
	got := command.HandleUse()
	if got == "" {
		t.Error("HandleUse() returned empty string")
	}
}
