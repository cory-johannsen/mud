package session_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPlayer_JobsInitialized(t *testing.T) {
	mgr := session.NewManager()
	_, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "a", CharName: "A", RoomID: "r1",
		CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess, ok := mgr.GetPlayer("u1")
	require.True(t, ok)
	assert.NotNil(t, sess.Jobs, "Jobs map must be initialized on AddPlayer")
	assert.Equal(t, "", sess.ActiveJobID, "ActiveJobID must default to empty string")
}

func TestPlayerSession_Jobs_TrackMultipleJobs(t *testing.T) {
	mgr := session.NewManager()
	_, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u2", Username: "b", CharName: "B", RoomID: "r1",
		CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess, _ := mgr.GetPlayer("u2")
	sess.Jobs["scavenger"] = 1
	sess.Jobs["infiltrator"] = 3
	assert.Equal(t, 2, len(sess.Jobs))
	assert.Equal(t, 1, sess.Jobs["scavenger"])
	assert.Equal(t, 3, sess.Jobs["infiltrator"])
}
