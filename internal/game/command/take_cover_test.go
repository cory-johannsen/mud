package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleTakeCover_NoArgs(t *testing.T) {
	req, err := command.HandleTakeCover(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleTakeCover_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleTakeCover([]string{"extra"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestPropertyHandleTakeCover_AlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleTakeCover(args)
		if err != nil {
			rt.Fatalf("HandleTakeCover must never return error, got: %v", err)
		}
		if req == nil {
			rt.Fatal("HandleTakeCover must never return nil request")
		}
	})
}
