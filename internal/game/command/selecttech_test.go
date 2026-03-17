package command_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

// TestHandleSelectTech_ReturnsRequest verifies HandleSelectTech returns a non-nil request.
func TestHandleSelectTech_ReturnsRequest(t *testing.T) {
	result, err := command.HandleSelectTech([]string{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, &command.SelectTechRequest{}, result)
}

// TestPropertyHandleSelectTech_AnyArgs_ReturnsNonNilResult verifies that HandleSelectTech
// always returns a non-nil result for any args slice (SWENG-5a).
func TestPropertyHandleSelectTech_AnyArgs_ReturnsNonNilResult(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nArgs := rapid.IntRange(0, 5).Draw(rt, "nArgs")
		args := make([]string, nArgs)
		for i := range args {
			args[i] = rapid.StringMatching(`[a-z]{1,10}`).Draw(rt, fmt.Sprintf("arg_%d", i))
		}
		result, err := command.HandleSelectTech(args)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestHandleSelectTech_ArgsIgnored verifies HandleSelectTech ignores all arguments.
func TestHandleSelectTech_ArgsIgnored(t *testing.T) {
	result1, err1 := command.HandleSelectTech(nil)
	require.NoError(t, err1)
	result2, err2 := command.HandleSelectTech([]string{"foo", "bar"})
	require.NoError(t, err2)
	assert.Equal(t, result1, result2)
}
