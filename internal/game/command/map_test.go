package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestHandleMap_ReturnsNonEmpty(t *testing.T) {
	result := command.HandleMap()
	require.NotEmpty(t, result)
}

func TestProperty_HandleMap_NeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		result := command.HandleMap()
		if result == "" {
			t.Fatal("HandleMap returned empty string")
		}
	})
}
