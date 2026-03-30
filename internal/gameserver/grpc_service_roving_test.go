package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// newRovingTestService constructs a minimal GameServiceServer for onRovingMove tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a GameServiceServer with the standard two-room test world (room_a→north→room_b).
func newRovingTestService(t *testing.T) (*GameServiceServer, *npc.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	logger := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc, npcMgr
}

// TestRovingMove_NotifiesPlayersInOriginRoom verifies that a player in the origin room
// receives a "<name> leaves to the north." message when an NPC moves north from room_a to room_b.
//
// Precondition: Player is in room_a; NPC moves from room_a to room_b (north exit).
// Postcondition: Player's entity channel contains a "leaves to the north." message.
func TestRovingMove_NotifiesPlayersInOriginRoom(t *testing.T) {
	svc, npcMgr := newRovingTestService(t)
	_, sessMgr := testWorldAndSession(t)
	// Use the svc's internal sessMgr (injected via newTestGameServiceServer).
	sessMgr = svc.sessions

	tmpl := &npc.Template{
		ID:    "wolf",
		Name:  "Wolf",
		MaxHP: 20,
	}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	player, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "roving-player-origin",
		Username: "Origin",
		CharName: "Origin",
		RoomID:   "room_a",
		MaxHP:    10,
		CurrentHP: 10,
		Role:     "player",
	})
	require.NoError(t, err)

	svc.onRovingMove(inst.ID, "room_a", "room_b")

	msgs := drainEntityMessages(t, player)
	require.NotEmpty(t, msgs, "expected a leave message pushed to player in origin room")
	found := false
	for _, m := range msgs {
		if containsSubstring(m, "leaves to the north") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'leaves to the north' message, got: %v", msgs)
}

// TestRovingMove_NotifiesPlayersInDestinationRoom verifies that a player in the destination room
// receives a "<name> arrives from the south." message when an NPC moves from room_a to room_b.
//
// Precondition: Player is in room_b; NPC moves from room_a to room_b (north exit).
// Postcondition: Player's entity channel contains an "arrives from the south." message.
func TestRovingMove_NotifiesPlayersInDestinationRoom(t *testing.T) {
	svc, npcMgr := newRovingTestService(t)
	sessMgr := svc.sessions

	tmpl := &npc.Template{
		ID:    "wolf",
		Name:  "Wolf",
		MaxHP: 20,
	}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	player, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "roving-player-dest",
		Username: "Dest",
		CharName: "Dest",
		RoomID:   "room_b",
		MaxHP:    10,
		CurrentHP: 10,
		Role:     "player",
	})
	require.NoError(t, err)

	svc.onRovingMove(inst.ID, "room_a", "room_b")

	msgs := drainEntityMessages(t, player)
	require.NotEmpty(t, msgs, "expected an arrive message pushed to player in destination room")
	found := false
	for _, m := range msgs {
		if containsSubstring(m, "arrives from the south") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'arrives from the south' message, got: %v", msgs)
}

// TestRovingMove_NoNotificationWhenRoomEmpty verifies that onRovingMove does not panic
// when neither room contains any players.
//
// Precondition: No players are in room_a or room_b.
// Postcondition: onRovingMove completes without panicking.
func TestRovingMove_NoNotificationWhenRoomEmpty(t *testing.T) {
	svc, npcMgr := newRovingTestService(t)

	tmpl := &npc.Template{
		ID:    "wolf",
		Name:  "Wolf",
		MaxHP: 20,
	}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	// Must not panic.
	assert.NotPanics(t, func() {
		svc.onRovingMove(inst.ID, "room_a", "room_b")
	})
}
