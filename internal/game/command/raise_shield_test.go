package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleRaiseShield_NoArgs(t *testing.T) {
	req, err := command.HandleRaiseShield(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleRaiseShield_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleRaiseShield([]string{"extra"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}
