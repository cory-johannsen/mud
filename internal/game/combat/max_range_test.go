package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeMaxRangeCombat builds a two-combatant combat with the given positions.
func makeMaxRangeCombat(t *testing.T, playerPos, npcPos int) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	player := &combat.Combatant{
		ID: "player1", Name: "Player", Kind: combat.KindPlayer,
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		Position: playerPos,
	}
	npc := &combat.Combatant{
		ID: "npc1", Name: "Bandit", Kind: combat.KindNPC,
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		Position: npcPos,
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

// TestStride_Away_CappedAtMaxRange verifies that an NPC striding away
// cannot exceed MaxCombatRange from the player.
func TestStride_Away_CappedAtMaxRange(t *testing.T) {
	// NPC is at 95, player at 0 — 25ft stride away would reach 120, capped at 100.
	cbt := makeMaxRangeCombat(t, 0, 95)
	_ = cbt.StartRound(3)
	err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	require.NoError(t, err)
	_ = cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionPass})

	resolveMaxRangeRound(cbt)

	npc := cbt.GetCombatant("npc1")
	require.NotNil(t, npc)
	player := cbt.GetCombatant("player1")
	require.NotNil(t, player)
	dist := combat.PosDist(npc.Position, player.Position)
	assert.LessOrEqual(t, dist, combat.MaxCombatRange,
		"NPC position %d should be capped so distance <= 100ft", npc.Position)
}

// TestStride_Away_BelowMaxRange_NotCapped verifies that striding away within
// MaxCombatRange proceeds normally.
func TestStride_Away_BelowMaxRange_NotCapped(t *testing.T) {
	// NPC at 50, player at 0 — 25ft stride away reaches 75, well within 100.
	cbt := makeMaxRangeCombat(t, 0, 50)
	_ = cbt.StartRound(3)
	err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	require.NoError(t, err)
	_ = cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionPass})

	resolveMaxRangeRound(cbt)

	npc := cbt.GetCombatant("npc1")
	require.NotNil(t, npc)
	assert.Equal(t, 75, npc.Position)
}

// TestStride_Away_ExactlyAtMaxRange_NoFurtherMovement verifies that an NPC
// already at MaxCombatRange cannot stride further away.
func TestStride_Away_ExactlyAtMaxRange_NoFurtherMovement(t *testing.T) {
	// NPC at 100, player at 0 — already at max range.
	cbt := makeMaxRangeCombat(t, 0, 100)
	_ = cbt.StartRound(3)
	err := cbt.QueueAction("npc1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	require.NoError(t, err)
	_ = cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionPass})

	resolveMaxRangeRound(cbt)

	npc := cbt.GetCombatant("npc1")
	require.NotNil(t, npc)
	player := cbt.GetCombatant("player1")
	require.NotNil(t, player)
	dist := combat.PosDist(npc.Position, player.Position)
	assert.LessOrEqual(t, dist, combat.MaxCombatRange,
		"NPC at max range should not move further away")
}
