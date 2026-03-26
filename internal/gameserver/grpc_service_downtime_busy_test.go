package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestDowntimeBusy_BlocksMove verifies that a player with DowntimeBusy=true cannot move
// to another room.
//
// Precondition: sess.DowntimeBusy is true; player is in room_a.
// Postcondition: handleMove returns a non-nil event without a RoomView; player remains in room_a.
func TestDowntimeBusy_BlocksMove(t *testing.T) {
	worldMgr, sessMgr := newNormalTerrainWorld(t)
	svc := newMoveTestService(t, worldMgr, sessMgr)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_downtime_move",
		Username:    "BusyPlayer",
		CharName:    "BusyPlayer",
		CharacterID: 20,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.DowntimeBusy = true

	evt, err := svc.handleMove("u_downtime_move", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Must not be a RoomView — player should not have moved.
	assert.Nil(t, evt.GetRoomView(), "downtime-busy player must not receive a RoomView on move")

	// Player's room must be unchanged.
	updatedSess, ok := sessMgr.GetPlayer("u_downtime_move")
	require.True(t, ok)
	assert.Equal(t, "room_a", updatedSess.RoomID, "downtime-busy player room must not change")

	// The message must mention the busy state.
	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a message event when blocked by downtime busy")
	assert.Contains(t, msg.Content, "busy with a downtime activity")
}

// TestPropertyDowntimeBusy_AlwaysBlocksMove is a property test verifying that a player with
// DowntimeBusy=true can never move to another room regardless of direction.
//
// Precondition: sess.DowntimeBusy is true.
// Postcondition: For all valid move attempts, the player's room is never changed.
func TestPropertyDowntimeBusy_AlwaysBlocksMove(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		worldMgr, sessMgr := newNormalTerrainWorld(t)
		svc := newMoveTestService(t, worldMgr, sessMgr)

		uid := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid")

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "DowntimeProp",
			CharName:    "DowntimeProp",
			CharacterID: 98,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       10,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		require.NoError(rt, err)
		sess.DowntimeBusy = true

		evt, err := svc.handleMove(uid, &gamev1.MoveRequest{Direction: "north"})
		require.NoError(rt, err)
		require.NotNil(rt, evt)

		assert.Nil(rt, evt.GetRoomView(), "downtime-busy player must never receive a RoomView")

		updatedSess, ok := sessMgr.GetPlayer(uid)
		require.True(rt, ok)
		assert.Equal(rt, "room_a", updatedSess.RoomID, "downtime-busy player room must not change")
	})
}

// TestDowntimeBusy_AllowsLook verifies that a player with DowntimeBusy=true can still use the
// look command.
//
// Precondition: sess.DowntimeBusy is true.
// Postcondition: handleLook returns a non-nil event with a RoomView.
func TestDowntimeBusy_AllowsLook(t *testing.T) {
	worldMgr, sessMgr := newNormalTerrainWorld(t)
	svc := newMoveTestService(t, worldMgr, sessMgr)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_downtime_look",
		Username:    "BusyLooker",
		CharName:    "BusyLooker",
		CharacterID: 21,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.DowntimeBusy = true

	evt, err := svc.handleLook("u_downtime_look")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Look must succeed and return a RoomView.
	assert.NotNil(t, evt.GetRoomView(), "downtime-busy player must still receive a RoomView on look")
}

// TestDowntimeBusy_AllowsDowntimeStatus verifies that a player with DowntimeBusy=true can still
// use the downtime status subcommand.
//
// Precondition: sess.DowntimeBusy is true, sess.DowntimeActivityID is set.
// Postcondition: handleDowntime returns a non-nil event without blocking.
func TestDowntimeBusy_AllowsDowntimeStatus(t *testing.T) {
	worldMgr, sessMgr := newNormalTerrainWorld(t)
	svc := newMoveTestService(t, worldMgr, sessMgr)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_downtime_status",
		Username:    "BusyStatus",
		CharName:    "BusyStatus",
		CharacterID: 22,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.DowntimeBusy = true
	sess.DowntimeActivityID = "earn_creds"

	evt, err := svc.handleDowntime("u_downtime_status", &gamev1.DowntimeRequest{Subcommand: "status"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	// The downtime status handler must not return an error about being busy.
	if msg := evt.GetMessage(); msg != nil {
		assert.NotContains(t, msg.Content, "cannot", "downtime status must not be blocked")
	}
}
