package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestHandleStride_NoArgs_DefaultsToToward(t *testing.T) {
	req, err := command.HandleStride([]string{})
	require.NoError(t, err)
	assert.Equal(t, "toward", req.Direction)
}

func TestHandleStride_Toward(t *testing.T) {
	req, err := command.HandleStride([]string{"toward"})
	require.NoError(t, err)
	assert.Equal(t, "toward", req.Direction)
}

func TestHandleStride_Away(t *testing.T) {
	req, err := command.HandleStride([]string{"away"})
	require.NoError(t, err)
	assert.Equal(t, "away", req.Direction)
}

func TestHandleStride_UnknownArg_DefaultsToToward(t *testing.T) {
	req, err := command.HandleStride([]string{"sideways"})
	require.NoError(t, err)
	assert.Equal(t, "toward", req.Direction)
}

func TestHandleStride_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		args := rapid.SliceOfN(rapid.String(), 0, 3).Draw(t, "args")
		req, err := command.HandleStride(args)
		require.NoError(t, err)
		require.NotNil(t, req)
		assert.True(t, req.Direction == "toward" || req.Direction == "away",
			"direction must be toward or away, got %q", req.Direction)
	})
}
