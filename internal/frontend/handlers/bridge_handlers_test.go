package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
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

// TestBridgeLook_NoArgs verifies that bare "look" sends a LookRequest to the server.
func TestBridgeLook_NoArgs(t *testing.T) {
	bctx := makeBridgeContext("req1", "")
	result, err := bridgeLook(bctx)
	require.NoError(t, err)
	assert.False(t, result.done, "bare look should send server request")
	require.NotNil(t, result.msg)
	assert.NotNil(t, result.msg.GetLook())
}

// TestBridgeLook_Direction verifies that "look north" returns a local message
// describing the exit, without sending a server request.
//
// Precondition: roomViewFn returns a RoomView with a north exit to "Town Square".
// Postcondition: result.done is true (handled locally), msg is nil,
// and the console output contains the target room name.
func TestBridgeLook_Direction(t *testing.T) {
	bctx := makeBridgeContext("req1", "north")
	bctx.parsed.Args = []string{"north"}
	bctx.roomViewFn = func() *gamev1.RoomView {
		return &gamev1.RoomView{
			Exits: []*gamev1.ExitInfo{
				{Direction: "north", TargetTitle: "Town Square"},
				{Direction: "east", TargetTitle: "Market", Locked: true},
			},
		}
	}
	result, err := bridgeLook(bctx)
	require.NoError(t, err)
	assert.True(t, result.done, "look <direction> should be handled locally")
	assert.Nil(t, result.msg, "no server message for look <direction>")
	assert.Contains(t, result.consoleMsg, "Town Square")
}

// TestBridgeLook_Direction_Locked verifies that "look east" at a locked exit
// indicates the exit is locked.
func TestBridgeLook_Direction_Locked(t *testing.T) {
	bctx := makeBridgeContext("req1", "east")
	bctx.parsed.Args = []string{"east"}
	bctx.roomViewFn = func() *gamev1.RoomView {
		return &gamev1.RoomView{
			Exits: []*gamev1.ExitInfo{
				{Direction: "east", TargetTitle: "Market", Locked: true},
			},
		}
	}
	result, err := bridgeLook(bctx)
	require.NoError(t, err)
	assert.True(t, result.done)
	assert.Contains(t, result.consoleMsg, "locked")
}

// TestBridgeLook_Direction_NoExit verifies that "look south" with no south exit
// returns a "nothing" message.
func TestBridgeLook_Direction_NoExit(t *testing.T) {
	bctx := makeBridgeContext("req1", "south")
	bctx.parsed.Args = []string{"south"}
	bctx.roomViewFn = func() *gamev1.RoomView {
		return &gamev1.RoomView{
			Exits: []*gamev1.ExitInfo{
				{Direction: "north", TargetTitle: "Town Square"},
			},
		}
	}
	result, err := bridgeLook(bctx)
	require.NoError(t, err)
	assert.True(t, result.done)
	assert.Contains(t, result.consoleMsg, "nothing")
}

// TestBridgeLook_Direction_Alias verifies that "look n" resolves the alias "n" to "north".
func TestBridgeLook_Direction_Alias(t *testing.T) {
	bctx := makeBridgeContext("req1", "n")
	bctx.parsed.Args = []string{"n"}
	bctx.roomViewFn = func() *gamev1.RoomView {
		return &gamev1.RoomView{
			Exits: []*gamev1.ExitInfo{
				{Direction: "north", TargetTitle: "Town Square"},
			},
		}
	}
	result, err := bridgeLook(bctx)
	require.NoError(t, err)
	assert.True(t, result.done)
	assert.Contains(t, result.consoleMsg, "Town Square")
}
