package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/command"
)

// TestHandleSelectTech_ReturnsRequest verifies HandleSelectTech returns a non-nil request.
func TestHandleSelectTech_ReturnsRequest(t *testing.T) {
	result, err := command.HandleSelectTech([]string{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, &command.SelectTechRequest{}, result)
}

// TestHandleSelectTech_ArgsIgnored verifies HandleSelectTech ignores all arguments.
func TestHandleSelectTech_ArgsIgnored(t *testing.T) {
	result1, err1 := command.HandleSelectTech(nil)
	require.NoError(t, err1)
	result2, err2 := command.HandleSelectTech([]string{"foo", "bar"})
	require.NoError(t, err2)
	assert.Equal(t, result1, result2)
}
