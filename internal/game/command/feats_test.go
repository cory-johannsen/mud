package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleFeats_ReturnsNonEmpty(t *testing.T) {
	got := command.HandleFeats()
	if got == "" {
		t.Error("HandleFeats() returned empty string")
	}
}
