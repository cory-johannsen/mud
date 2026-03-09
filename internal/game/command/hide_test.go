package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleHide_NoArgs(t *testing.T) {
	req, err := command.HandleHide(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleHide_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleHide([]string{"extra"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestPropertyHandleHide_AlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleHide(args)
		if err != nil {
			rt.Fatalf("HandleHide must never return error, got: %v", err)
		}
		if req == nil {
			rt.Fatal("HandleHide must never return nil request")
		}
	})
}
