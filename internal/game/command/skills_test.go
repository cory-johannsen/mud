package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleSkills_ReturnsNonEmpty(t *testing.T) {
	result := command.HandleSkills()
	if result == "" {
		t.Error("HandleSkills must return a non-empty string")
	}
}
