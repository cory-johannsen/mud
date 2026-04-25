package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestProperty_TerrainAt_AbsentCellIsNormal(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cbt := &combat.Combat{Terrain: nil, GridWidth: 10, GridHeight: 10}
		x := rapid.IntRange(0, 9).Draw(rt, "x")
		y := rapid.IntRange(0, 9).Draw(rt, "y")
		tc := cbt.TerrainAt(x, y)
		if tc.Type != combat.TerrainNormal {
			rt.Fatalf("absent cell (%d,%d): want normal got %v", x, y, tc.Type)
		}
	})
}

func TestProperty_EntryCost_GreaterDifficultImpassable(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
			{X: 3, Y: 3}: {X: 3, Y: 3, Type: combat.TerrainGreaterDifficult},
		}
		_, ok := cbt.EntryCost(3, 3)
		if ok {
			rt.Fatal("greater_difficult must be impassable (ok=false)")
		}
	})
}

func TestProperty_EntryCost_DifficultCostsTwo(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
			{X: 1, Y: 1}: {X: 1, Y: 1, Type: combat.TerrainDifficult},
		}
		cost, ok := cbt.EntryCost(1, 1)
		if !ok {
			rt.Fatal("difficult must be passable")
		}
		if cost != 2 {
			rt.Fatalf("difficult: want cost=2 got %d", cost)
		}
	})
}

func TestProperty_EntryCost_HazardousWithDifficultOverlayCostsTwo(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
			{X: 2, Y: 2}: {X: 2, Y: 2, Type: combat.TerrainHazardous, DifficultOverlay: true},
		}
		cost, ok := cbt.EntryCost(2, 2)
		if !ok {
			rt.Fatal("hazardous must be passable")
		}
		if cost != 2 {
			rt.Fatalf("hazardous+difficult: want cost=2 got %d", cost)
		}
	})
}

func TestProperty_EntryCost_NormalCostsOne(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cost, ok := cbt.EntryCost(0, 0)
		if !ok || cost != 1 {
			rt.Fatalf("normal: want (1, true) got (%d, %v)", cost, ok)
		}
	})
}

func TestProperty_SpeedBudget_EqualsSpeedSquares(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		c := &combat.Combatant{
			SpeedFt: rapid.IntRange(0, 100).Draw(rt, "speedFt"),
		}
		if c.SpeedBudget() != c.SpeedSquares() {
			rt.Fatalf("SpeedBudget %d != SpeedSquares %d for SpeedFt %d",
				c.SpeedBudget(), c.SpeedSquares(), c.SpeedFt)
		}
	})
}

func TestProperty_SpeedBudget_MinOne(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		c := &combat.Combatant{SpeedFt: 0}
		if c.SpeedBudget() < 1 {
			rt.Fatalf("SpeedBudget must be >= 1, got %d", c.SpeedBudget())
		}
	})
}
