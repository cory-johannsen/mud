package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleSneak_NoArgs(t *testing.T) {
	req, err := command.HandleSneak(nil)
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestHandleSneak_WithArgs_Ignored(t *testing.T) {
	req, err := command.HandleSneak([]string{"extra"})
	require.NoError(t, err)
	assert.NotNil(t, req)
}

func TestPropertyHandleSneak_AlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleSneak(args)
		if err != nil {
			rt.Fatalf("HandleSneak must never return error, got: %v", err)
		}
		if req == nil {
			rt.Fatal("HandleSneak must never return nil request")
		}
	})
}
