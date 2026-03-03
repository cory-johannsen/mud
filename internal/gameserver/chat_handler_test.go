package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestChatHandler_Say(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
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

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
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

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u1",
		Username:          "Alice",
		CharName:          "Alice",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u2",
		Username:          "Bob",
		CharName:          "Bob",
		CharacterID:       0,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             0,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             0,
	})
	require.NoError(t, err)

	list, err := h.Who("u1")
	require.NoError(t, err)
	require.Len(t, list.Players, 2)
	names := []string{list.Players[0].Name, list.Players[1].Name}
	assert.ElementsMatch(t, []string{"Alice", "Bob"}, names)
}

func TestChatHandler_Who_NotFound(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	list, err := h.Who("nonexistent-uid")
	assert.Nil(t, list)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-uid")
}

func TestChatHandler_Who_PopulatesPlayerInfo(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u1", Username: "Alice", CharName: "Alice",
		CharacterID: 1, RoomID: "room_a",
		CurrentHP:   8, MaxHP: 10,
		Abilities:   character.AbilityScores{}, Role: "player",
		Class:       "striker_gun", Level: 3,
	})
	require.NoError(t, err)

	list, err := h.Who("u1")
	require.NoError(t, err)
	require.Len(t, list.Players, 1)
	p := list.Players[0]
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, int32(3), p.Level)
	assert.Equal(t, "striker_gun", p.Job)
	assert.Equal(t, "Lightly Wounded", p.HealthLabel)
	assert.Equal(t, gamev1.CombatStatus_COMBAT_STATUS_IDLE, p.Status)
}
