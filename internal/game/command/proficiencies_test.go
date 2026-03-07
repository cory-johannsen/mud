package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"pgregory.net/rapid"
)

func TestHandleProficiencies_ReturnsEmptyString(t *testing.T) {
	result := command.HandleProficiencies()
	// HandleProficiencies is a client-side no-op; the actual data comes from the server.
	// It must return an empty string (not an error value).
	if result != "" {
		t.Errorf("HandleProficiencies must return empty string, got %q", result)
	}
}

func TestHandleProficiencies_PropertyAlwaysReturnsString(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		result := command.HandleProficiencies()
		// The function must always return a valid string (never panics).
		_ = result
	})
}
