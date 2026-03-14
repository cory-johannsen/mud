package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestHandleHeroPoint_NoArgs verifies that HandleHeroPoint with no args returns a usage error.
//
// Precondition: args is empty.
// Postcondition: result.Error contains "Usage:"; err is nil.
func TestHandleHeroPoint_NoArgs(t *testing.T) {
	result, err := HandleHeroPoint(nil)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "Usage:")
}

// TestHandleHeroPoint_InvalidSubcommand verifies that an unknown subcommand returns an error.
//
// Precondition: args[0] is not "reroll" or "stabilize".
// Postcondition: result.Error contains "unknown subcommand"; err is nil.
func TestHandleHeroPoint_InvalidSubcommand(t *testing.T) {
	result, err := HandleHeroPoint([]string{"badcmd"})
	require.NoError(t, err)
	assert.Contains(t, result.Error, "unknown subcommand")
}

// TestHandleHeroPoint_Reroll verifies that "reroll" subcommand is parsed correctly.
//
// Precondition: args[0] == "reroll".
// Postcondition: result.Subcommand == "reroll"; result.Error is empty; err is nil.
func TestHandleHeroPoint_Reroll(t *testing.T) {
	result, err := HandleHeroPoint([]string{"reroll"})
	require.NoError(t, err)
	assert.Equal(t, "reroll", result.Subcommand)
	assert.Empty(t, result.Error)
}

// TestHandleHeroPoint_Stabilize verifies that "stabilize" subcommand is parsed correctly.
//
// Precondition: args[0] == "stabilize".
// Postcondition: result.Subcommand == "stabilize"; result.Error is empty; err is nil.
func TestHandleHeroPoint_Stabilize(t *testing.T) {
	result, err := HandleHeroPoint([]string{"stabilize"})
	require.NoError(t, err)
	assert.Equal(t, "stabilize", result.Subcommand)
	assert.Empty(t, result.Error)
}

// TestProperty_HandleHeroPoint_ValidSubcmds verifies that valid subcommands always yield
// Subcommand matching the input and no error string.
//
// Precondition: args[0] is one of the valid subcommands.
// Postcondition: result.Subcommand == args[0]; result.Error is empty; err is nil.
func TestProperty_HandleHeroPoint_ValidSubcmds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sub := rapid.SampledFrom([]string{"reroll", "stabilize"}).Draw(rt, "sub")
		result, err := HandleHeroPoint([]string{sub})
		require.NoError(rt, err)
		assert.Equal(rt, sub, result.Subcommand)
		assert.Empty(rt, result.Error)
	})
}
