package handlers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestDispatchWSMessage_CommandText_Move(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "north"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-1", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	move := msg.GetMove()
	require.NotNil(t, move)
	assert.Equal(t, "north", move.Direction)
}

func TestDispatchWSMessage_CommandText_Say(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "say Hello world"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-2", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	say := msg.GetSay()
	require.NotNil(t, say)
	assert.Equal(t, "Hello world", say.Message)
}

func TestDispatchWSMessage_DirectProto_MoveRequest(t *testing.T) {
	env := handlers.WSMessageForTest("MoveRequest", map[string]string{"direction": "south"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-3", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "south", msg.GetMove().GetDirection())
}

func TestDispatchWSMessage_UnknownType_ReturnsError(t *testing.T) {
	env := handlers.WSMessageForTest("BogusRequest", map[string]string{})
	registry := command.DefaultRegistry()
	_, err := handlers.DispatchWSMessageForTest(env, "req-4", registry)
	assert.Error(t, err)
}

// ── serverEventInner: UseResponse ────────────────────────────────────────────

func TestServerEventInner_UseResponse_Message(t *testing.T) {
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_UseResponse{
			UseResponse: &gamev1.UseResponse{Message: "You strike with Power Strike!"},
		},
	}
	inner, name := handlers.ServerEventInnerForTest(event)
	require.NotNil(t, inner, "UseResponse must not be dropped")
	assert.Equal(t, "UseResponse", name)
	ur, ok := inner.(*gamev1.UseResponse)
	require.True(t, ok, "inner must be *gamev1.UseResponse")
	assert.Equal(t, "You strike with Power Strike!", ur.GetMessage())
}

func TestServerEventInner_UseResponse_Choices(t *testing.T) {
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_UseResponse{
			UseResponse: &gamev1.UseResponse{
				Choices: []*gamev1.FeatEntry{
					{FeatId: "power_strike", Name: "Power Strike"},
				},
			},
		},
	}
	inner, name := handlers.ServerEventInnerForTest(event)
	require.NotNil(t, inner, "UseResponse with choices must not be dropped")
	assert.Equal(t, "UseResponse", name)
}
