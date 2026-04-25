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
	combat.ResolveRound(cbt, src, func(string, int) {}, nil, 0)
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

// TestCellBlockedByCover_TrueWhenCoverAtCell verifies GH #227: a cell with a
// cover object is reported blocked.
func TestCellBlockedByCover_TrueWhenCoverAtCell(t *testing.T) {
	cbt, _, _ := makeAdjacentStrideCombat(t, 0, 10, 10, 10)
	cbt.CoverObjects = []combat.CoverObject{
		{EquipmentID: "crate-1", Tier: "standard", GridX: 5, GridY: 10},
	}
	assert.True(t, combat.CellBlockedByCover(cbt, 5, 10),
		"cell containing cover must be reported blocked")
	assert.False(t, combat.CellBlockedByCover(cbt, 6, 10),
		"cell not containing cover must not be reported blocked")
}

// TestCellBlocked_CombinesCombatantAndCover verifies GH #227: CellBlocked
// composes the combatant-occupancy check with the cover-occupancy check.
func TestCellBlocked_CombinesCombatantAndCover(t *testing.T) {
	cbt, _, _ := makeAdjacentStrideCombat(t, 0, 10, 10, 10)
	cbt.CoverObjects = []combat.CoverObject{
		{EquipmentID: "crate-1", Tier: "standard", GridX: 5, GridY: 10},
	}
	// NPC at (10,10).
	assert.True(t, combat.CellBlocked(cbt, "p1", 10, 10),
		"cell with opposing combatant must block")
	assert.True(t, combat.CellBlocked(cbt, "p1", 5, 10),
		"cell with cover must block")
	assert.False(t, combat.CellBlocked(cbt, "p1", 2, 10),
		"empty cell must not block")
}

// TestStride_Cover_BlocksNPCMovement verifies GH #227: an NPC striding toward
// the player through a cover-occupied cell stops at the cover and does not
// pass through.
//
// Precondition: player at (0,10), NPC at (9,10), cover at (5,10).
// Postcondition: NPC ends at (6,10) — one cell east of the cover, having
// stopped when it encountered the blocking tile.
func TestStride_Cover_BlocksNPCMovement(t *testing.T) {
	cbt, _, npc := makeAdjacentStrideCombat(t, 0, 10, 9, 10)
	cbt.CoverObjects = []combat.CoverObject{
		{EquipmentID: "barrier-1", Tier: "standard", GridX: 5, GridY: 10},
	}

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"})
	resolveAdjacentRound(cbt)

	assert.Equal(t, 6, npc.GridX,
		"NPC must stop one cell east of the cover at (5,10) — got GridX=%d", npc.GridX)
	assert.Equal(t, 10, npc.GridY)
}

// TestStride_Cover_DestroyedTileIsTraversable verifies GH #227: after the
// cover object is removed from cbt.CoverObjects (simulating destruction),
// the NPC can pass through the previously-blocked cell.
func TestStride_Cover_DestroyedTileIsTraversable(t *testing.T) {
	cbt, _, npc := makeAdjacentStrideCombat(t, 0, 10, 9, 10)
	cbt.CoverObjects = []combat.CoverObject{
		{EquipmentID: "barrier-1", Tier: "standard", GridX: 5, GridY: 10},
	}
	// Simulate destruction: drop the cover.
	cbt.CoverObjects = nil

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"})
	resolveAdjacentRound(cbt)

	// With speed 5, NPC at (9,10) striding toward player at (0,10) walks
	// 5 cells west → (4,10). Nothing blocks now that cover is gone.
	assert.Equal(t, 4, npc.GridX,
		"NPC must traverse the previously-blocked cell once cover is destroyed")
	assert.Equal(t, 10, npc.GridY)
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
