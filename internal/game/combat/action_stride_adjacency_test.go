package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeAdjacentStrideCombat builds a two-combatant combat with explicit GridX/GridY positions.
func makeAdjacentStrideCombat(t *testing.T, playerX, playerY, npcX, npcY int) (*combat.Combat, *combat.Combatant, *combat.Combatant) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	player := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		GridX: playerX, GridY: playerY,
	}
	npc := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		GridX: npcX, GridY: npcY,
	}
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_adj", []*combat.Combatant{player, npc}, reg, nil, "")
	require.NoError(t, err)
	return cbt, player, npc
}

func resolveAdjacentRound(cbt *combat.Combat) {
	src := fixedSrcDist{val: 1}
	combat.ResolveRound(cbt, src, func(string, int) {}, nil)
}

// TestStride_Toward_StopsWhenAdjacent verifies that striding "toward" an opponent
// who is already adjacent (Chebyshev distance = 1 square = 5 ft) results in no movement.
//
// Precondition: player at (5,10), NPC at (6,10); Chebyshev distance = 1 = 5 ft (adjacent).
// Postcondition: player position is unchanged after stride toward.
func TestStride_Toward_StopsWhenAdjacent(t *testing.T) {
	cbt, player, _ := makeAdjacentStrideCombat(t, 5, 10, 6, 10)

	beforeX := player.GridX
	beforeY := player.GridY

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	resolveAdjacentRound(cbt)

	assert.Equal(t, beforeX, player.GridX, "player must not move when already adjacent to opponent")
	assert.Equal(t, beforeY, player.GridY, "player must not move when already adjacent to opponent")
}

// TestStride_Toward_MovesToAdjacentWhenTwoCellsAway verifies that striding "toward" from
// 2 cells (10 ft) away moves the player exactly 1 cell to reach adjacency (5 ft).
//
// Precondition: player at (3,10), NPC at (5,10); Chebyshev distance = 2 = 10 ft.
// Postcondition: player at (4,10) — adjacent to NPC at (5,10).
func TestStride_Toward_MovesToAdjacentWhenTwoCellsAway(t *testing.T) {
	cbt, player, _ := makeAdjacentStrideCombat(t, 3, 10, 5, 10)

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	resolveAdjacentRound(cbt)

	assert.Equal(t, 4, player.GridX, "player must stop 1 cell from NPC (adjacent)")
	assert.Equal(t, 10, player.GridY, "player Y must not change (same row)")
}

// TestStride_Toward_NPC_StopsWhenAdjacent verifies that an NPC striding "toward" a player
// who is already adjacent does not move onto the player's cell.
//
// Precondition: player at (5,10), NPC at (6,10); distance = 5 ft (adjacent).
// Postcondition: NPC position is unchanged.
func TestStride_Toward_NPC_StopsWhenAdjacent(t *testing.T) {
	cbt, player, npc := makeAdjacentStrideCombat(t, 5, 10, 6, 10)

	beforeNPCX := npc.GridX
	beforeNPCY := npc.GridY

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"})
	resolveAdjacentRound(cbt)

	assert.Equal(t, beforeNPCX, npc.GridX, "NPC must not move onto player's cell when already adjacent")
	assert.Equal(t, beforeNPCY, npc.GridY, "NPC must not move when already adjacent")
	// Verify no overlap.
	assert.False(t, player.GridX == npc.GridX && player.GridY == npc.GridY,
		"player and NPC must never share a cell")
}

// TestStride_NoOverlap_CompassDirection verifies that a player striding via a compass
// direction cannot land on a cell occupied by another living combatant.
//
// Precondition: player at (4,10), NPC at (5,10); player strides "e" (east) → destination (5,10) = NPC's cell.
// Postcondition: player position is unchanged (movement blocked).
func TestStride_NoOverlap_CompassDirection(t *testing.T) {
	cbt, player, _ := makeAdjacentStrideCombat(t, 4, 10, 5, 10)

	beforeX := player.GridX
	beforeY := player.GridY

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "e"})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	resolveAdjacentRound(cbt)

	assert.Equal(t, beforeX, player.GridX, "player must not move onto an occupied cell")
	assert.Equal(t, beforeY, player.GridY, "player must not move onto an occupied cell")
}

// TestCellOccupied_TrueWhenOtherCombatantPresent verifies that CellOccupied returns true
// when a living combatant other than actorID occupies the given cell.
//
// Precondition: combat with player at (0,10) and NPC at (5,10).
// Postcondition: CellOccupied returns true for NPC's cell from player's perspective.
func TestCellOccupied_TrueWhenOtherCombatantPresent(t *testing.T) {
	cbt, _, _ := makeAdjacentStrideCombat(t, 0, 10, 5, 10)
	assert.True(t, combat.CellOccupied(cbt, "p1", 5, 10), "NPC's cell must be reported occupied")
}

// TestCellOccupied_FalseForActorOwnCell verifies that CellOccupied returns false for the
// actor's own cell (not counting self as an obstacle).
//
// Precondition: combat with player at (0,10).
// Postcondition: CellOccupied returns false for player's own position.
func TestCellOccupied_FalseForActorOwnCell(t *testing.T) {
	cbt, _, _ := makeAdjacentStrideCombat(t, 0, 10, 5, 10)
	assert.False(t, combat.CellOccupied(cbt, "p1", 0, 10), "actor's own cell must not be reported occupied")
}

// TestCellOccupied_FalseForEmptyCell verifies that CellOccupied returns false for a cell
// with no living combatants.
//
// Precondition: combat with player at (0,10) and NPC at (5,10).
// Postcondition: CellOccupied returns false for an empty cell.
func TestCellOccupied_FalseForEmptyCell(t *testing.T) {
	cbt, _, _ := makeAdjacentStrideCombat(t, 0, 10, 5, 10)
	assert.False(t, combat.CellOccupied(cbt, "p1", 10, 10), "empty cell must not be reported occupied")
}

// TestCellOccupied_DeadCombatantDoesNotBlock verifies that a dead combatant's cell is NOT
// reported as occupied (dead bodies don't block movement).
//
// Precondition: combat with player at (0,10) and dead NPC at (5,10).
// Postcondition: CellOccupied returns false for the dead NPC's cell.
func TestCellOccupied_DeadCombatantDoesNotBlock(t *testing.T) {
	cbt, _, npc := makeAdjacentStrideCombat(t, 0, 10, 5, 10)
	npc.CurrentHP = 0 // mark dead
	assert.False(t, combat.CellOccupied(cbt, "p1", 5, 10), "dead combatant must not block cell")
}
