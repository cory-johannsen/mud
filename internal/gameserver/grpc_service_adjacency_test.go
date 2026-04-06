package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAdjacencyCombat spawns an NPC and adds a player in roomID, starts combat,
// cancels the timer, then manually places combatants at the given positions.
//
// Precondition: h, npcMgr, sessMgr must be non-nil; all ID/name strings non-empty.
// Postcondition: Active combat exists in roomID with player at (playerX, playerY)
// and NPC at (npcX, npcY); timer is cancelled.
func setupAdjacencyCombat(
	t *testing.T,
	h *CombatHandler,
	npcMgr *npc.Manager,
	sessMgr *session.Manager,
	roomID, playerUID, npcName string,
	playerX, playerY, npcX, npcY int,
) {
	t.Helper()
	_, err := npcMgr.Spawn(&npc.Template{
		ID: npcName + "-tmpl", Name: npcName, Level: 1, MaxHP: 100, AC: 15, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       playerUID,
		Username:  playerUID,
		CharName:  playerUID,
		Role:      "player",
		RoomID:    roomID,
		CurrentHP: 50,
		MaxHP:     50,
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = h.Attack(playerUID, npcName)
	require.NoError(t, err)
	h.cancelTimer(roomID)

	cbt, ok := h.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant(playerUID)
	require.NotNil(t, playerCbt)
	playerCbt.GridX = playerX
	playerCbt.GridY = playerY

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.GridX = npcX
			c.GridY = npcY
			break
		}
	}
}

// TestHandleStride_Toward_StopsWhenAdjacentToNPC verifies that handleStride "toward"
// does not move the player when already adjacent (Chebyshev ≤ 5 ft) to the target NPC.
//
// Precondition: player at (18,10), NPC at (19,10); distance = 5 ft (adjacent).
// Postcondition: player position is unchanged after stride toward.
func TestHandleStride_Toward_StopsWhenAdjacentToNPC(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)
	const roomID = "room_adj_str"
	setupAdjacencyCombat(t, combatHandler, npcMgr, sessMgr, roomID, "u_adj_str", "Guard", 18, 10, 19, 10)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	playerCbt := cbt.GetCombatant("u_adj_str")
	require.NotNil(t, playerCbt)

	event, err := svc.handleStride("u_adj_str", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)

	assert.Equal(t, 18, playerCbt.GridX, "player must not move when already adjacent to NPC")
	assert.Equal(t, 10, playerCbt.GridY, "player must not move when already adjacent to NPC")
}

// TestHandleStride_Toward_StopsAtAdjacency verifies that handleStride "toward" stops
// exactly when reaching adjacency, even if the player has remaining speed.
//
// Precondition: player at (14,10), NPC at (19,10); distance = 25 ft (5 cells).
// Speed = 25 ft (5 cells). Player should stop at (18,10) — adjacent to NPC at (19,10).
// Postcondition: player at (18,10) after stride toward; not at (19,10).
func TestHandleStride_Toward_StopsAtAdjacency(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)
	const roomID = "room_adj_stop"
	setupAdjacencyCombat(t, combatHandler, npcMgr, sessMgr, roomID, "u_adj_stop", "Guard2", 14, 10, 19, 10)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	playerCbt := cbt.GetCombatant("u_adj_stop")
	require.NotNil(t, playerCbt)

	event, err := svc.handleStride("u_adj_stop", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)

	// 5 cells to close, but stops when adjacent (18,10), not on NPC's cell (19,10).
	assert.Equal(t, 18, playerCbt.GridX, "player must stop adjacent to NPC, not on its cell")
	assert.Equal(t, 10, playerCbt.GridY, "player Y must not change (same row)")
	// Verify no overlap.
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			assert.False(t, playerCbt.GridX == c.GridX && playerCbt.GridY == c.GridY,
				"player and NPC must never share a cell")
		}
	}
}

// TestHandleStride_NoOverlap_CompassDirection verifies that handleStride in a compass
// direction stops before entering a cell occupied by another living combatant.
//
// Precondition: player at (17,10), NPC at (18,10); player strides "e" (east), 5 steps.
// First step would land on (18,10) = NPC's cell.
// Postcondition: player position unchanged (movement blocked on first step).
func TestHandleStride_NoOverlap_CompassDirection(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)
	const roomID = "room_noo_str"
	setupAdjacencyCombat(t, combatHandler, npcMgr, sessMgr, roomID, "u_noo_str", "Guard3", 17, 10, 18, 10)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	playerCbt := cbt.GetCombatant("u_noo_str")
	require.NotNil(t, playerCbt)

	event, err := svc.handleStride("u_noo_str", &gamev1.StrideRequest{Direction: "e"})
	require.NoError(t, err)
	require.NotNil(t, event)

	assert.Equal(t, 17, playerCbt.GridX, "player must not move onto NPC's occupied cell")
	assert.Equal(t, 10, playerCbt.GridY, "player Y must not change")
}

// TestHandleStep_Toward_StopsWhenAdjacentToNPC verifies that handleStep "toward"
// does not move the player when already adjacent (Chebyshev ≤ 5 ft) to the target.
//
// Precondition: player at (18,10), NPC at (19,10); distance = 5 ft (adjacent).
// Postcondition: player position is unchanged after step toward.
func TestHandleStep_Toward_StopsWhenAdjacentToNPC(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)
	const roomID = "room_adj_stp"
	setupAdjacencyCombat(t, combatHandler, npcMgr, sessMgr, roomID, "u_adj_stp", "Guard4", 18, 10, 19, 10)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	playerCbt := cbt.GetCombatant("u_adj_stp")
	require.NotNil(t, playerCbt)

	event, err := svc.handleStep("u_adj_stp", &gamev1.StepRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)

	assert.Equal(t, 18, playerCbt.GridX, "player must not step onto NPC's cell when already adjacent")
	assert.Equal(t, 10, playerCbt.GridY, "player Y must not change")
}

// TestHandleStep_NoOverlap_CompassDirection verifies that handleStep in a compass direction
// is blocked when the destination cell is occupied by a living combatant.
//
// Precondition: player at (17,10), NPC at (18,10); player steps "e" → would land on NPC.
// Postcondition: player position is unchanged.
func TestHandleStep_NoOverlap_CompassDirection(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)
	const roomID = "room_noo_stp"
	setupAdjacencyCombat(t, combatHandler, npcMgr, sessMgr, roomID, "u_noo_stp", "Guard5", 17, 10, 18, 10)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	playerCbt := cbt.GetCombatant("u_noo_stp")
	require.NotNil(t, playerCbt)

	event, err := svc.handleStep("u_noo_stp", &gamev1.StepRequest{Direction: "e"})
	require.NoError(t, err)
	require.NotNil(t, event)

	assert.Equal(t, 17, playerCbt.GridX, "player must not step onto NPC's occupied cell")
	assert.Equal(t, 10, playerCbt.GridY, "player Y must not change")
}
