package session_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerSession_ZoneEffectCooldowns_NilByDefault(t *testing.T) {
	mgr := session.NewManager()
	sess, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1", CharName: "Alice", RoomID: "room1",
		CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	assert.Nil(t, sess.ZoneEffectCooldowns, "ZoneEffectCooldowns should be nil by default")
}

func TestPlayerSession_ZoneEffectCooldowns_LazyInit(t *testing.T) {
	mgr := session.NewManager()
	sess, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1", CharName: "Alice", RoomID: "room1",
		CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Should not panic on lazy init.
	if sess.ZoneEffectCooldowns == nil {
		sess.ZoneEffectCooldowns = make(map[string]int64)
	}
	sess.ZoneEffectCooldowns["room1:despair"] = 3

	assert.Equal(t, int64(3), sess.ZoneEffectCooldowns["room1:despair"])
}
