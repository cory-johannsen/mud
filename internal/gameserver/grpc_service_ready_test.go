package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerSession_ReadiedFields_DefaultEmpty(t *testing.T) {
	mgr := session.NewManager()
	_, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1_user", CharName: "u1_char",
		CharacterID: 1, RoomID: "r1",
		CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{},
		Role: "player", Level: 1,
	})
	require.NoError(t, err)
	sess, ok := mgr.GetPlayer("u1")
	require.True(t, ok)
	assert.Equal(t, "", sess.ReadiedTrigger, "ReadiedTrigger must default to empty")
	assert.Equal(t, "", sess.ReadiedAction, "ReadiedAction must default to empty")
}

func TestReadyAction_ClearReadiedAction_ClearsFields(t *testing.T) {
	mgr := session.NewManager()
	_, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1_user", CharName: "u1_char",
		CharacterID: 1, RoomID: "r1",
		CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{},
		Role: "player", Level: 1,
	})
	require.NoError(t, err)
	sess, ok := mgr.GetPlayer("u1")
	require.True(t, ok)
	sess.ReadiedTrigger = "enemy_enters"
	sess.ReadiedAction = "strike"

	clearReadiedAction(sess)

	assert.Equal(t, "", sess.ReadiedTrigger)
	assert.Equal(t, "", sess.ReadiedAction)
}
