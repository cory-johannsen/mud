package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTravel_CommandRegistered verifies that "travel" is in the default registry.
func TestTravel_CommandRegistered(t *testing.T) {
	r := DefaultRegistry()
	cmd, ok := r.Resolve("travel")
	require.True(t, ok, "travel command must be registered")
	require.Equal(t, HandlerTravel, cmd.Handler)
}

// TestTravel_CategoryWorld verifies that the travel command is in CategoryWorld.
func TestTravel_CategoryWorld(t *testing.T) {
	r := DefaultRegistry()
	byCategory := r.CommandsByCategory()
	worldCmds := byCategory[CategoryWorld]
	found := false
	for _, cmd := range worldCmds {
		if cmd.Name == "travel" {
			found = true
			break
		}
	}
	require.True(t, found, "travel must be in CategoryWorld")
}
