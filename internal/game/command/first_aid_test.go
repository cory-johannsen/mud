package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleFirstAid_NoArgs(t *testing.T) {
	req, err := command.HandleFirstAid(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleFirstAid_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleFirstAid([]string{"extra", "args"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestPropertyHandleFirstAid_AlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleFirstAid(args)
		if err != nil {
			rt.Fatalf("HandleFirstAid must never return error, got: %v", err)
		}
		if req == nil {
			rt.Fatal("HandleFirstAid must never return nil request")
		}
	})
}
