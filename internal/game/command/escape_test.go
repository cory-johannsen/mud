package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleEscape_NoArgs(t *testing.T) {
	req, err := command.HandleEscape(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleEscape_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleEscape([]string{"extra"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestPropertyHandleEscape_AlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleEscape(args)
		if err != nil {
			rt.Fatalf("HandleEscape must never return error, got: %v", err)
		}
		if req == nil {
			rt.Fatal("HandleEscape must never return nil request")
		}
	})
}
