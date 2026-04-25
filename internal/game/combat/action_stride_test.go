package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestActionStride_Cost_IsOneAP(t *testing.T) {
	assert.Equal(t, 1, combat.ActionStride.Cost())
}

func TestActionStride_String(t *testing.T) {
	assert.Equal(t, "stride", combat.ActionStride.String())
}

// makeStrideCombat2D creates a minimal two-combatant combat with GridX/GridY set.
// Player starts at (playerX, playerY); NPC at (npcX, npcY).
func makeStrideCombat2D(t *testing.T, playerX, playerY, npcX, npcY int) (*combat.Combat, *combat.Combatant, *combat.Combatant) {
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
	cbt, err := eng.StartCombat("room_a", []*combat.Combatant{player, npc}, reg, nil, "")
	require.NoError(t, err)
	return cbt, player, npc
}

// TestStride_TowardMovesCloserOnGrid verifies that striding "toward" reduces the
// Chebyshev distance between player and NPC by exactly 1 square (5 ft).
func TestStride_TowardMovesCloserOnGrid(t *testing.T) {
	src := fixedSrcDist{val: 1}
	// Player at (0,0), NPC at (5,5) — Chebyshev distance = 5 squares = 25 ft.
	cbt, player, npc := makeStrideCombat2D(t, 0, 0, 5, 5)

	distBefore := combat.CombatRange(*player, *npc)

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	_ = combat.ResolveRound(cbt, src, func(id string, hp int) {}, nil, 0)

	distAfter := combat.CombatRange(*player, *npc)
	assert.Less(t, distAfter, distBefore, "stride toward must reduce Chebyshev distance")
}

// TestStride_AwayMovesAwayOnGrid verifies that striding "away" does not reduce
// the Chebyshev distance between player and NPC.
func TestStride_AwayMovesAwayOnGrid(t *testing.T) {
	src := fixedSrcDist{val: 1}
	// Player at (3,3), NPC at (0,0) — Chebyshev distance = 3 squares = 15 ft.
	cbt, player, npc := makeStrideCombat2D(t, 3, 3, 0, 0)

	distBefore := combat.CombatRange(*player, *npc)

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	_ = combat.ResolveRound(cbt, src, func(id string, hp int) {}, nil, 0)

	distAfter := combat.CombatRange(*player, *npc)
	assert.GreaterOrEqual(t, distAfter, distBefore, "stride away must not reduce Chebyshev distance")
}

// TestStride_CompassDirections verifies each compass direction moves up to 5 squares
// (default 25 ft speed = 5 squares). Starting at (5,5), each cardinal/diagonal direction
// moves 5 cells, clamped at grid boundaries.
func TestStride_CompassDirections(t *testing.T) {
	tests := []struct {
		dir    string
		wantX  int
		wantY  int
	}{
		{"n", 5, 0},
		{"s", 5, 10},
		{"e", 10, 5},
		{"w", 0, 5},
		{"ne", 10, 0},
		{"nw", 0, 0},
		{"se", 10, 10},
		{"sw", 0, 10},
	}
	for _, tc := range tests {
		t.Run(tc.dir, func(t *testing.T) {
			src := fixedSrcDist{val: 1}
			// Player starts at (5,5); NPC far away so it doesn't block movement.
			cbt, player, _ := makeStrideCombat2D(t, 5, 5, 19, 19)

			_ = cbt.StartRound(3)
			_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: tc.dir})
			_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
			_ = combat.ResolveRound(cbt, src, func(id string, hp int) {}, nil, 0)

			assert.Equal(t, tc.wantX, player.GridX, "dir=%s: wrong GridX", tc.dir)
			assert.Equal(t, tc.wantY, player.GridY, "dir=%s: wrong GridY", tc.dir)
		})
	}
}

// TestProperty_Stride_GridBoundsRespected verifies that no direction places a combatant
// outside the 20×20 grid (GridX in [0,19], GridY in [0,19]).
func TestProperty_Stride_GridBoundsRespected(t *testing.T) {
	directions := []string{"n", "s", "e", "w", "ne", "nw", "se", "sw", "toward", "away"}
	rapid.Check(t, func(rt *rapid.T) {
		startX := rapid.IntRange(0, 19).Draw(rt, "startX")
		startY := rapid.IntRange(0, 19).Draw(rt, "startY")
		dir := directions[rapid.IntRange(0, len(directions)-1).Draw(rt, "dirIdx")]

		// NPC at the opposite corner to ensure toward/away have a valid opponent.
		npcX := 19 - startX
		npcY := 19 - startY
		if npcX == startX && npcY == startY {
			npcX = (startX + 10) % 20
			npcY = (startY + 10) % 20
		}

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

		player := &combat.Combatant{
			ID: "p1", Kind: combat.KindPlayer, Name: "Player",
			CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
			GridX: startX, GridY: startY,
		}
		npc := &combat.Combatant{
			ID: "n1", Kind: combat.KindNPC, Name: "NPC",
			CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
			GridX: npcX, GridY: npcY,
		}
		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room_prop", []*combat.Combatant{player, npc}, reg, nil, "")
		if err != nil {
			rt.Fatal(err)
		}

		src := fixedSrcDist{val: 1}
		_ = cbt.StartRound(3)
		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: dir})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
		_ = combat.ResolveRound(cbt, src, func(id string, hp int) {}, nil, 0)

		if player.GridX < 0 || player.GridX > 19 || player.GridY < 0 || player.GridY > 19 {
			rt.Fatalf("after stride %q from (%d,%d), player ended at (%d,%d) — outside 20x20 grid",
				dir, startX, startY, player.GridX, player.GridY)
		}
	})
}
