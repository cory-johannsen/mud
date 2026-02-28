package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestChatHandler_Say(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)

	evt, err := h.Say("u1", "hello world")
	require.NoError(t, err)
	assert.Equal(t, "Alice", evt.Sender)
	assert.Equal(t, "hello world", evt.Content)
	assert.Equal(t, gamev1.MessageType_MESSAGE_TYPE_SAY, evt.Type)
}

func TestChatHandler_Say_NotFound(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	_, err := h.Say("unknown", "hello")
	assert.Error(t, err)
}

func TestChatHandler_Emote(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)

	evt, err := h.Emote("u1", "waves")
	require.NoError(t, err)
	assert.Equal(t, "Alice", evt.Sender)
	assert.Equal(t, "waves", evt.Content)
	assert.Equal(t, gamev1.MessageType_MESSAGE_TYPE_EMOTE, evt.Type)
}

func TestChatHandler_Who(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	_, err := sessMgr.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)
	_, err = sessMgr.AddPlayer("u2", "Bob", "Bob", 0, "room_a", 10, "player")
	require.NoError(t, err)

	list, err := h.Who("u1")
	require.NoError(t, err)
	assert.Len(t, list.Players, 2)
	assert.Contains(t, list.Players, "Alice")
	assert.Contains(t, list.Players, "Bob")
}
