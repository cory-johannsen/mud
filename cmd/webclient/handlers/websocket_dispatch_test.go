package handlers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/internal/game/command"
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
