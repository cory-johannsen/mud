package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleDivert_NoArgs(t *testing.T) {
	req, err := command.HandleDivert(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleDivert_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleDivert([]string{"extra"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestPropertyHandleDivert_AlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleDivert(args)
		if err != nil {
			rt.Fatalf("HandleDivert must never return error, got: %v", err)
		}
		if req == nil {
			rt.Fatal("HandleDivert must never return nil request")
		}
	})
}
