package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/require"
)

func TestHandleExplore_NoArgs(t *testing.T) {
	req, err := command.HandleExplore(nil)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, "", req.Mode)
	require.Equal(t, "", req.ShadowTarget)
}

func TestHandleExplore_Off(t *testing.T) {
	req, err := command.HandleExplore([]string{"off"})
	require.NoError(t, err)
	require.Equal(t, "off", req.Mode)
}

func TestHandleExplore_SimpleMode(t *testing.T) {
	modes := []string{"lay_low", "hold_ground", "active_sensors", "case_it", "run_point", "poke_around"}
	for _, m := range modes {
		req, err := command.HandleExplore([]string{m})
		require.NoError(t, err, "mode: %s", m)
		require.Equal(t, m, req.Mode, "mode: %s", m)
		require.Equal(t, "", req.ShadowTarget)
	}
}

func TestHandleExplore_Shadow_WithTarget(t *testing.T) {
	req, err := command.HandleExplore([]string{"shadow", "Alice"})
	require.NoError(t, err)
	require.Equal(t, "shadow", req.Mode)
	require.Equal(t, "Alice", req.ShadowTarget)
}

func TestHandleExplore_Shadow_NoTarget(t *testing.T) {
	req, err := command.HandleExplore([]string{"shadow"})
	require.NoError(t, err)
	require.Equal(t, "shadow", req.Mode)
	require.Equal(t, "", req.ShadowTarget)
}

func TestHandleExplore_CaseInsensitive(t *testing.T) {
	req, err := command.HandleExplore([]string{"LAY_LOW"})
	require.NoError(t, err)
	require.Equal(t, "lay_low", req.Mode)
}
