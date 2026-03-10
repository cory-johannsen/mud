package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleDisarm_WithTarget(t *testing.T) {
	req, err := command.HandleDisarm([]string{"Ganger"})
	require.NoError(t, err)
	assert.Equal(t, "Ganger", req.Target)
}

func TestHandleDisarm_NoTarget(t *testing.T) {
	req, err := command.HandleDisarm([]string{})
	require.NoError(t, err)
	assert.Empty(t, req.Target)
}

func TestHandleDisarm_MultiWordTarget_TakesFirstWord(t *testing.T) {
	req, err := command.HandleDisarm([]string{"Ganger", "A"})
	require.NoError(t, err)
	assert.Equal(t, "Ganger", req.Target)
}
