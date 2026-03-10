package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleSeek_NoArgs(t *testing.T) {
	req, err := command.HandleSeek(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleSeek_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleSeek([]string{"ignored"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestPropertyHandleSeek_AlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleSeek(args)
		if err != nil {
			rt.Fatalf("HandleSeek must never return error, got: %v", err)
		}
		if req == nil {
			rt.Fatal("HandleSeek must never return nil request")
		}
	})
}
