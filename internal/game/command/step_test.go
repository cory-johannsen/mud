package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestHandleStep_NoArgs_DefaultsToToward(t *testing.T) {
	req, err := command.HandleStep([]string{})
	require.NoError(t, err)
	assert.Equal(t, "toward", req.Direction)
}

func TestHandleStep_Toward(t *testing.T) {
	req, err := command.HandleStep([]string{"toward"})
	require.NoError(t, err)
	assert.Equal(t, "toward", req.Direction)
}

func TestHandleStep_Away(t *testing.T) {
	req, err := command.HandleStep([]string{"away"})
	require.NoError(t, err)
	assert.Equal(t, "away", req.Direction)
}

func TestHandleStep_UnknownArg_DefaultsToToward(t *testing.T) {
	req, err := command.HandleStep([]string{"sideways"})
	require.NoError(t, err)
	assert.Equal(t, "toward", req.Direction)
}

func TestHandleStep_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		args := rapid.SliceOfN(rapid.String(), 0, 3).Draw(t, "args")
		req, err := command.HandleStep(args)
		require.NoError(t, err)
		require.NotNil(t, req)
		assert.True(t, req.Direction == "toward" || req.Direction == "away",
			"direction must be toward or away, got %q", req.Direction)
	})
}
