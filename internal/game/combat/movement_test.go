package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMovementCombat returns a Combat with a width×height grid and no terrain.
func newMovementCombat(width, height int) *combat.Combat {
	return &combat.Combat{
		GridWidth:  width,
		GridHeight: height,
	}
}

// addCombatant places a non-dead combatant at (x, y) on the grid and returns it.
func addCombatant(cbt *combat.Combat, id string, kind combat.Kind, x, y int) *combat.Combatant {
	c := &combat.Combatant{
		ID:        id,
		Kind:      kind,
		Name:      id,
		MaxHP:     20,
		CurrentHP: 20,
		GridX:     x,
		GridY:     y,
		SpeedFt:   25, // SpeedBudget == 5
	}
	cbt.Combatants = append(cbt.Combatants, c)
	return c
}

// containsCell reports whether the slice contains the given cell.
func containsCell(cells []combat.GridCell, want combat.GridCell) bool {
	for _, c := range cells {
		if c == want {
			return true
		}
	}
	return false
}

// ─── CandidateCells ──────────────────────────────────────────────────────────

func TestCandidateCells_IncludesCurrentCell(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 5)
	cells := combat.CandidateCells(cbt, npc)
	require.True(t, containsCell(cells, combat.GridCell{X: 5, Y: 5}),
		"current cell must always be a candidate (MOVE-9)")
}

func TestCandidateCells_ExcludesOccupiedAndOOB(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 0, 0)
	addCombatant(cbt, "p1", combat.KindPlayer, 0, 1)
	cells := combat.CandidateCells(cbt, npc)
	assert.False(t, containsCell(cells, combat.GridCell{X: 0, Y: 1}),
		"occupied cells must be excluded")
	assert.False(t, containsCell(cells, combat.GridCell{X: -1, Y: 0}),
		"out-of-bounds cells must be excluded")
}

func TestCandidateCells_ExcludesGreaterDifficultTerrain(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 5)
	cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
		{X: 6, Y: 5}: {X: 6, Y: 5, Type: combat.TerrainGreaterDifficult},
	}
	cells := combat.CandidateCells(cbt, npc)
	assert.False(t, containsCell(cells, combat.GridCell{X: 6, Y: 5}),
		"greater_difficult cells must be excluded (MOVE-14)")
}

func TestCandidateCells_BoundedByMovementBudget(t *testing.T) {
	cbt := newMovementCombat(40, 40)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 20, 20)
	maxRing := npc.SpeedBudget() * combat.MaxMovementAP
	cells := combat.CandidateCells(cbt, npc)
	for _, c := range cells {
		dx := c.X - 20
		if dx < 0 {
			dx = -dx
		}
		dy := c.Y - 20
		if dy < 0 {
			dy = -dy
		}
		ring := dx
		if dy > dx {
			ring = dy
		}
		require.LessOrEqual(t, ring, maxRing,
			"candidate cell %+v exceeds reachable Chebyshev ring %d", c, maxRing)
	}
}

// ─── RangeGoal ───────────────────────────────────────────────────────────────

func TestRangeGoal_MeleeMaxAtAdjacent(t *testing.T) {
	target := &combat.Combatant{GridX: 5, GridY: 5}
	score := combat.RangeGoal(combat.MoveContext{RangeIncrementFt: 0}, target,
		combat.GridCell{X: 4, Y: 5}, 20)
	assert.InDelta(t, 1.0, score, 0.001, "melee NPC adjacent to target should score 1.0")
}

func TestRangeGoal_RangedMaxAtIncrementDistance(t *testing.T) {
	target := &combat.Combatant{GridX: 10, GridY: 10}
	// 15 ft / 5 = 3 cells.
	score := combat.RangeGoal(combat.MoveContext{RangeIncrementFt: 15}, target,
		combat.GridCell{X: 7, Y: 10}, 20)
	assert.InDelta(t, 1.0, score, 0.001, "ranged NPC at increment distance should score 1.0")
}

func TestRangeGoal_RangedNeverPrefersPointBlank(t *testing.T) {
	target := &combat.Combatant{GridX: 10, GridY: 10}
	near := combat.RangeGoal(combat.MoveContext{RangeIncrementFt: 15}, target,
		combat.GridCell{X: 9, Y: 10}, 20)
	farther := combat.RangeGoal(combat.MoveContext{RangeIncrementFt: 15}, target,
		combat.GridCell{X: 7, Y: 10}, 20)
	assert.Less(t, near, farther,
		"ranged NPC must never prefer point-blank over its preferred range")
}

func TestRangeGoal_NilTargetReturnsZero(t *testing.T) {
	score := combat.RangeGoal(combat.MoveContext{}, nil, combat.GridCell{X: 0, Y: 0}, 10)
	assert.Equal(t, 0.0, score)
}

// ─── CoverGoal ───────────────────────────────────────────────────────────────

func TestCoverGoal_DisabledByStrategy(t *testing.T) {
	ctx := combat.MoveContext{
		UseCover:    false,
		CoverTierAt: func(combat.GridCell) string { return combat.CoverTierGreater },
	}
	assert.Equal(t, 0.0, combat.CoverGoal(ctx, combat.GridCell{X: 0, Y: 0}))
}

func TestCoverGoal_NilTierFnReturnsZero(t *testing.T) {
	ctx := combat.MoveContext{UseCover: true}
	assert.Equal(t, 0.0, combat.CoverGoal(ctx, combat.GridCell{X: 0, Y: 0}))
}

func TestCoverGoal_TierMonotone(t *testing.T) {
	mk := func(tier string) combat.MoveContext {
		return combat.MoveContext{
			UseCover:    true,
			CoverTierAt: func(combat.GridCell) string { return tier },
		}
	}
	c := combat.GridCell{X: 1, Y: 1}
	none := combat.CoverGoal(mk(combat.CoverTierNone), c)
	lesser := combat.CoverGoal(mk(combat.CoverTierLesser), c)
	standard := combat.CoverGoal(mk(combat.CoverTierStandard), c)
	greater := combat.CoverGoal(mk(combat.CoverTierGreater), c)
	assert.Less(t, none, lesser)
	assert.Less(t, lesser, standard)
	assert.Less(t, standard, greater)
	assert.InDelta(t, 1.0, greater, 0.001)
}

// ─── SpreadGoal ──────────────────────────────────────────────────────────────

func TestSpreadGoal_NoAlliesFullScore(t *testing.T) {
	actor := &combat.Combatant{ID: "n1", GridX: 5, GridY: 5}
	score := combat.SpreadGoal(combat.MoveContext{}, actor, combat.GridCell{X: 5, Y: 5})
	assert.InDelta(t, 1.0, score, 0.001)
}

func TestSpreadGoal_AdjacentAllyLowScore(t *testing.T) {
	actor := &combat.Combatant{ID: "n1", GridX: 5, GridY: 5, FactionID: "thugs", MaxHP: 10, CurrentHP: 10, Kind: combat.KindNPC}
	ally := &combat.Combatant{ID: "n2", GridX: 5, GridY: 6, FactionID: "thugs", MaxHP: 10, CurrentHP: 10, Kind: combat.KindNPC}
	ctx := combat.MoveContext{Allies: []*combat.Combatant{ally}}
	atActor := combat.SpreadGoal(ctx, actor, combat.GridCell{X: 5, Y: 5})
	farAway := combat.SpreadGoal(ctx, actor, combat.GridCell{X: 5, Y: 11})
	assert.Less(t, atActor, farAway,
		"a cell adjacent to an ally must score lower than a cell 6 cells away")
}

func TestSpreadGoal_DifferentFactionIgnored(t *testing.T) {
	actor := &combat.Combatant{ID: "n1", GridX: 5, GridY: 5, FactionID: "thugs", MaxHP: 10, CurrentHP: 10, Kind: combat.KindNPC}
	other := &combat.Combatant{ID: "n2", GridX: 5, GridY: 6, FactionID: "guards", MaxHP: 10, CurrentHP: 10, Kind: combat.KindNPC}
	ctx := combat.MoveContext{Allies: []*combat.Combatant{other}}
	score := combat.SpreadGoal(ctx, actor, combat.GridCell{X: 5, Y: 5})
	assert.InDelta(t, 1.0, score, 0.001, "non-faction combatants must not affect spread")
}

// ─── TerrainGoal ─────────────────────────────────────────────────────────────

func TestTerrainGoal_NormalUntilTerrainPopulated(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	assert.InDelta(t, 1.0, combat.TerrainGoal(cbt, combat.GridCell{X: 3, Y: 3}), 0.001)
}

func TestTerrainGoal_DifficultHalfScore(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
		{X: 3, Y: 3}: {X: 3, Y: 3, Type: combat.TerrainDifficult},
	}
	assert.InDelta(t, 0.5, combat.TerrainGoal(cbt, combat.GridCell{X: 3, Y: 3}), 0.001)
}

func TestTerrainGoal_HazardousZeroScore(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
		{X: 3, Y: 3}: {X: 3, Y: 3, Type: combat.TerrainHazardous},
	}
	assert.InDelta(t, 0.0, combat.TerrainGoal(cbt, combat.GridCell{X: 3, Y: 3}), 0.001)
}

// ─── ChooseMoveDestination ───────────────────────────────────────────────────

func TestChooseMoveDestination_MeleeClosesDistance(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 0, 10)
	pl := addCombatant(cbt, "p1", combat.KindPlayer, 19, 10)
	dest := combat.ChooseMoveDestination(cbt, npc, pl, combat.MoveContext{RangeIncrementFt: 0}, false)
	require.NotNil(t, dest, "melee NPC far from target should choose to move")
	// New cell should reduce Chebyshev distance to player.
	oldDist := chebyshev(combat.GridCell{X: npc.GridX, Y: npc.GridY},
		combat.GridCell{X: pl.GridX, Y: pl.GridY})
	newDist := chebyshev(*dest, combat.GridCell{X: pl.GridX, Y: pl.GridY})
	assert.Less(t, newDist, oldDist, "melee NPC should close distance to target")
}

func TestChooseMoveDestination_RangedSeeksIncrementDistance(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 8, 10) // 1 cell from player
	pl := addCombatant(cbt, "p1", combat.KindPlayer, 9, 10)
	dest := combat.ChooseMoveDestination(cbt, npc, pl,
		combat.MoveContext{RangeIncrementFt: 30}, false)
	require.NotNil(t, dest, "ranged NPC at point-blank should move")
	// Should move farther from the player, not closer.
	oldDist := chebyshev(combat.GridCell{X: npc.GridX, Y: npc.GridY},
		combat.GridCell{X: pl.GridX, Y: pl.GridY})
	newDist := chebyshev(*dest, combat.GridCell{X: pl.GridX, Y: pl.GridY})
	assert.Greater(t, newDist, oldDist, "ranged NPC should retreat from point-blank")
}

func TestChooseMoveDestination_NilTargetReturnsNil(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 5)
	dest := combat.ChooseMoveDestination(cbt, npc, nil, combat.MoveContext{}, false)
	assert.Nil(t, dest)
}

func TestChooseMoveDestination_HazardousNeverChosen(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 5)
	pl := addCombatant(cbt, "p1", combat.KindPlayer, 19, 5)
	// Surround the optimal "+x" route with hazardous terrain so the chooser
	// must avoid it. Place hazardous cells at all (x, 5) for x in [6..15].
	cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{}
	for x := 6; x <= 15; x++ {
		cbt.Terrain[combat.GridCell{X: x, Y: 5}] = &combat.TerrainCell{X: x, Y: 5, Type: combat.TerrainHazardous}
	}
	dest := combat.ChooseMoveDestination(cbt, npc, pl, combat.MoveContext{}, false)
	require.NotNil(t, dest)
	tc := cbt.TerrainAt(dest.X, dest.Y)
	assert.NotEqual(t, combat.TerrainHazardous, tc.Type,
		"hazardous terrain must never be chosen as a destination")
}

func TestChooseMoveDestination_FleeReversesRangeGoal(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 10)
	pl := addCombatant(cbt, "p1", combat.KindPlayer, 6, 10) // adjacent
	// Without fleeing, melee NPC stays adjacent (no improvement available).
	// With flipRange=true, the NPC seeks distance.
	dest := combat.ChooseMoveDestination(cbt, npc, pl,
		combat.MoveContext{RangeIncrementFt: 0}, true)
	require.NotNil(t, dest, "fleeing NPC adjacent to enemy should move")
	oldDist := chebyshev(combat.GridCell{X: npc.GridX, Y: npc.GridY},
		combat.GridCell{X: pl.GridX, Y: pl.GridY})
	newDist := chebyshev(*dest, combat.GridCell{X: pl.GridX, Y: pl.GridY})
	assert.Greater(t, newDist, oldDist, "fleeing NPC should increase distance to enemy")
}

func TestChooseMoveDestination_EpsilonStayPut(t *testing.T) {
	// When the actor is already at an optimal position (melee adjacent to
	// target on open terrain with no allies), chooseMoveDestination should
	// return nil rather than picking a tied cell.
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 10)
	addCombatant(cbt, "p1", combat.KindPlayer, 6, 10)
	dest := combat.ChooseMoveDestination(cbt, npc, cbt.Combatants[1],
		combat.MoveContext{RangeIncrementFt: 0}, false)
	assert.Nil(t, dest, "melee NPC already adjacent should stay put (MOVE-20)")
}

// chebyshev mirrors the package-private helper for use in tests.
func chebyshev(a, b combat.GridCell) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// ─── Property tests ──────────────────────────────────────────────────────────

func TestProperty_ChooseMoveDestination_Determinism(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 5)
	pl := addCombatant(cbt, "p1", combat.KindPlayer, 12, 12)
	ctx := combat.MoveContext{RangeIncrementFt: 30}
	a := combat.ChooseMoveDestination(cbt, npc, pl, ctx, false)
	b := combat.ChooseMoveDestination(cbt, npc, pl, ctx, false)
	require.Equal(t, a, b, "same state must produce same destination")
}

func TestProperty_ChooseMoveDestination_BoundedByCandidates(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 5)
	pl := addCombatant(cbt, "p1", combat.KindPlayer, 12, 12)
	candidates := combat.CandidateCells(cbt, npc)
	d := combat.ChooseMoveDestination(cbt, npc, pl, combat.MoveContext{}, false)
	if d == nil {
		return
	}
	assert.True(t, containsCell(candidates, *d),
		"chosen destination must be in the candidate set")
}

func TestProperty_ChooseMoveDestination_StrideBudgetRespected(t *testing.T) {
	cbt := newMovementCombat(20, 20)
	npc := addCombatant(cbt, "n1", combat.KindNPC, 5, 5)
	pl := addCombatant(cbt, "p1", combat.KindPlayer, 19, 19)
	d := combat.ChooseMoveDestination(cbt, npc, pl, combat.MoveContext{}, false)
	if d == nil {
		return
	}
	dist := chebyshev(combat.GridCell{X: npc.GridX, Y: npc.GridY}, *d)
	maxReach := npc.SpeedBudget() * combat.MaxMovementAP
	assert.LessOrEqual(t, dist, maxReach,
		"destination Chebyshev distance must fit inside total stride budget")
}
