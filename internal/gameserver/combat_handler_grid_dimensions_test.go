package gameserver_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeGridDimHandler builds a CombatHandler for grid-dimension tests and wires a
// RoundStartEvent capture callback via SetRoundStartBroadcastFn.
//
// Precondition: t must be non-nil.
// Postcondition: Returns handler, npcMgr, sessMgr, and a function that returns all
// captured RoundStartEvents under a mutex.
func makeGridDimHandler(
	t *testing.T,
) (*gameserver.CombatHandler, *npc.Manager, *session.Manager, func() []*gamev1.RoundStartEvent) {
	t.Helper()

	broadcastFn, _ := makeBroadcastCapture()
	h, npcMgr, sessMgr := makePositionBroadcastHandler(t, broadcastFn)

	var mu sync.Mutex
	var captured []*gamev1.RoundStartEvent

	h.SetRoundStartBroadcastFn(func(_ string, evt *gamev1.RoundStartEvent) {
		mu.Lock()
		defer mu.Unlock()
		captured = append(captured, evt)
	})

	get := func() []*gamev1.RoundStartEvent {
		mu.Lock()
		defer mu.Unlock()
		result := make([]*gamev1.RoundStartEvent, len(captured))
		copy(result, captured)
		return result
	}

	return h, npcMgr, sessMgr, get
}

// TestRoundStartEvent_GridDimensions_PopulatedOnAttack verifies that the RoundStartEvent
// broadcast during combat initiation carries the correct GridWidth and GridHeight values.
//
// REQ-PA-1b: RoundStartEvent MUST carry the grid width and height of the combat arena.
//
// Precondition: NPC survives; player initiates combat via Attack.
// Postcondition: At least one RoundStartEvent is captured with GridWidth = 20 and GridHeight = 20
// (the engine default defined in internal/game/combat/engine.go).
func TestRoundStartEvent_GridDimensions_PopulatedOnAttack(t *testing.T) {
	h, npcMgr, sessMgr, getRoundStartEvents := makeGridDimHandler(t)

	const roomID = "room-grid-dim"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-grid", Name: "GridGuard", Level: 1, MaxHP: 1000, AC: 30, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "player-grid",
		Username:  "Hero",
		CharName:  "Hero",
		RoomID:    roomID,
		CurrentHP: 1000,
		MaxHP:     1000,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	equipTestPistol(t, h, "player-grid")

	_, err = h.Attack("player-grid", "GridGuard")
	require.NoError(t, err)

	// Wait up to 2 seconds for at least one RoundStartEvent to be captured.
	deadline := time.Now().Add(2 * time.Second)
	var evts []*gamev1.RoundStartEvent
	for time.Now().Before(deadline) {
		evts = getRoundStartEvents()
		if len(evts) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	require.NotEmpty(t, evts, "expected at least one RoundStartEvent to be broadcast")

	for _, evt := range evts {
		assert.Equal(t, int32(20), evt.GridWidth, "RoundStartEvent.GridWidth must equal the engine default of 20")
		assert.Equal(t, int32(20), evt.GridHeight, "RoundStartEvent.GridHeight must equal the engine default of 20")
	}
}
