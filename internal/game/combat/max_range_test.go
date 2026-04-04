package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeMaxRangeCombat builds a two-combatant combat with GridX/GridY positions.
func makeMaxRangeCombat(t *testing.T, playerX, playerY, npcX, npcY int) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	player := &combat.Combatant{
		ID: "player1", Name: "Player", Kind: combat.KindPlayer,
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		GridX: playerX, GridY: playerY,
	}
	npc := &combat.Combatant{
		ID: "npc1", Name: "Bandit", Kind: combat.KindNPC,
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		GridX: npcX, GridY: npcY,
	}
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_max", []*combat.Combatant{player, npc}, reg, nil, "")
	require.NoError(t, err)
	return cbt
}

func resolveMaxRangeRound(cbt *combat.Combat) {
	src := fixedSrcDist{val: 1}
	combat.ResolveRound(cbt, src, func(string, int) {}, nil)
}

// TestMaxCombatRange_Constant verifies the constant is 100.
func TestMaxCombatRange_Constant(t *testing.T) {
	assert.Equal(t, 100, combat.MaxCombatRange)
}

// TestStride_Away_GridBoundClamped verifies that an NPC striding away from a
// player at the corner cannot move beyond the 10×10 grid boundary.
func TestStride_Away_GridBoundClamped(t *testing.T) {
	// NPC at (9,9) — top-right corner of the 10x10 grid. Player at (0,0).
	// Striding "away" would try to increase both X and Y, but they're already at max.
	cbt := makeMaxRangeCombat(t, 0, 0, 9, 9)
	_ = cbt.StartRound(3)
	err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	require.NoError(t, err)
	_ = cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionPass})

	resolveMaxRangeRound(cbt)

	npc := cbt.GetCombatant("npc1")
	require.NotNil(t, npc)
	assert.GreaterOrEqual(t, npc.GridX, 0, "GridX must be >= 0")
	assert.LessOrEqual(t, npc.GridX, 9, "GridX must be <= 9")
	assert.GreaterOrEqual(t, npc.GridY, 0, "GridY must be >= 0")
	assert.LessOrEqual(t, npc.GridY, 9, "GridY must be <= 9")
}

// TestStride_Away_BelowMaxRange_NotClamped verifies that striding away within
// grid boundaries proceeds normally.
func TestStride_Away_BelowMaxRange_NotClamped(t *testing.T) {
	// NPC at (5,5), player at (0,0) — stride "away" moves NPC to (6,6).
	cbt := makeMaxRangeCombat(t, 0, 0, 5, 5)
	_ = cbt.StartRound(3)
	err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	require.NoError(t, err)
	_ = cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionPass})

	resolveMaxRangeRound(cbt)

	npc := cbt.GetCombatant("npc1")
	require.NotNil(t, npc)
	assert.Equal(t, 6, npc.GridX, "NPC should move to GridX=6 after stride away")
	assert.Equal(t, 6, npc.GridY, "NPC should move to GridY=6 after stride away")
}

// TestStride_Away_MaxGridDistance_StaysInBounds verifies that an NPC at maximum
// grid distance stays in bounds after striding away.
func TestStride_Away_MaxGridDistance_StaysInBounds(t *testing.T) {
	// NPC at (8,8), player at (0,0). Max Chebyshev distance = 9 squares = 45 ft.
	// Stride away increases both X and Y by 1 — clamped to (9,9).
	cbt := makeMaxRangeCombat(t, 0, 0, 8, 8)
	_ = cbt.StartRound(3)
	err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	require.NoError(t, err)
	_ = cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionPass})

	resolveMaxRangeRound(cbt)

	npc := cbt.GetCombatant("npc1")
	require.NotNil(t, npc)
	player := cbt.GetCombatant("player1")
	require.NotNil(t, player)
	dist := combat.CombatRange(*npc, *player)
	maxGridDist := 9 * 5 // 9 squares × 5 ft
	assert.LessOrEqual(t, dist, maxGridDist,
		"NPC distance from player must not exceed max grid distance (%d ft)", maxGridDist)
	assert.GreaterOrEqual(t, npc.GridX, 0)
	assert.LessOrEqual(t, npc.GridX, 9)
	assert.GreaterOrEqual(t, npc.GridY, 0)
	assert.LessOrEqual(t, npc.GridY, 9)
}
