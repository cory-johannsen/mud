package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/command"
)

// makeBridgeContext builds a minimal bridgeContext for unit-testing bridge handlers.
// reqID is the request identifier; rawArgs is the unparsed argument string after the command.
func makeBridgeContext(reqID, rawArgs string) *bridgeContext {
	return &bridgeContext{
		reqID: reqID,
		parsed: command.ParseResult{
			RawArgs: rawArgs,
		},
	}
}

// TestBridgeReady_ValidArgs verifies that bridgeReady parses "strike when enters"
// into Action="strike" and Trigger="enters".
func TestBridgeReady_ValidArgs(t *testing.T) {
	bctx := makeBridgeContext("req1", "strike when enters")
	result, err := bridgeReady(bctx)
	require.NoError(t, err)
	msg := result.msg.GetReady()
	require.NotNil(t, msg)
	assert.Equal(t, "strike", msg.GetAction())
	assert.Equal(t, "enters", msg.GetTrigger())
}


// TestBridgeReady_NoWhenKeyword verifies that bridgeReady returns done=true (error path)
// when the args don't contain " when ".
func TestBridgeReady_NoWhenKeyword(t *testing.T) {
	bctx := makeBridgeContext("req1", "strike")
	result, err := bridgeReady(bctx)
	require.NoError(t, err)
	assert.True(t, result.done, "missing 'when' keyword must result in error prompt (done=true)")
	assert.Nil(t, result.msg, "no message should be returned on error path")
}

// TestBridgeUse_WithTarget verifies that bridgeUse populates both feat_id and target
// when two tokens are provided in RawArgs.
func TestBridgeUse_WithTarget(t *testing.T) {
	bctx := makeBridgeContext("req1", "mind_spike goblin")
	result, err := bridgeUse(bctx)
	require.NoError(t, err)
	msg := result.msg.GetUseRequest()
	require.NotNil(t, msg)
	assert.Equal(t, "mind_spike", msg.GetFeatId())
	assert.Equal(t, "goblin", msg.GetTarget())
}

// TestBridgeUse_NoTarget verifies that bridgeUse leaves target empty when only
// the feat ID is provided.
func TestBridgeUse_NoTarget(t *testing.T) {
	bctx := makeBridgeContext("req1", "mind_spike")
	result, err := bridgeUse(bctx)
	require.NoError(t, err)
	msg := result.msg.GetUseRequest()
	require.NotNil(t, msg)
	assert.Equal(t, "mind_spike", msg.GetFeatId())
	assert.Equal(t, "", msg.GetTarget())
}

// TestBridgeTabComplete_BuildsTabCompleteRequest verifies that bridgeTabComplete
// wraps the raw args as a TabCompleteRequest with the correct Prefix.
func TestBridgeTabComplete_BuildsTabCompleteRequest(t *testing.T) {
	bctx := makeBridgeContext("req-tc", "use med")
	result, err := bridgeTabComplete(bctx)
	require.NoError(t, err)
	require.NotNil(t, result.msg)
	msg := result.msg.GetTabComplete()
	require.NotNil(t, msg)
	assert.Equal(t, "use med", msg.GetPrefix())
	assert.Equal(t, "req-tc", result.msg.GetRequestId())
}
