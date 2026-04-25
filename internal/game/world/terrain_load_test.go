package world_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
)

func TestLoadTerrainShapeA_ExplicitGrid(t *testing.T) {
	cfg := &world.CombatTerrainConfig{
		Grid: []string{
			"....",
			".##.",
			"....",
		},
		Legend: map[string]world.TerrainLegendEntry{
			".": {Type: "normal"},
			"#": {Type: "difficult"},
		},
	}
	cells, err := world.LoadCombatTerrain(cfg, 4, 3, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cells[world.CellKey{X: 1, Y: 1}].Type != "difficult" {
		t.Error("(1,1) should be difficult")
	}
	if cells[world.CellKey{X: 2, Y: 1}].Type != "difficult" {
		t.Error("(2,1) should be difficult")
	}
	if _, ok := cells[world.CellKey{X: 0, Y: 0}]; ok {
		t.Error("(0,0) should be absent (normal, not stored)")
	}
}

func TestLoadTerrainShapeB_DefaultPlusOverrides(t *testing.T) {
	cfg := &world.CombatTerrainConfig{
		Default: "normal",
		Cells: []world.TerrainCellDef{
			{X: 2, Y: 2, Type: "difficult"},
			{X: 3, Y: 3, Type: "hazardous", HazardID: "fire_vent"},
		},
	}
	hazards := []world.HazardDef{{ID: "fire_vent", Trigger: "on_enter", DamageExpr: "1d6", DamageType: "fire"}}
	cells, err := world.LoadCombatTerrain(cfg, 10, 10, hazards)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cells[world.CellKey{X: 2, Y: 2}].Type != "difficult" {
		t.Error("(2,2) should be difficult")
	}
	h := cells[world.CellKey{X: 3, Y: 3}]
	if h.Type != "hazardous" {
		t.Errorf("(3,3) should be hazardous, got %v", h.Type)
	}
	if h.Hazard == nil || h.Hazard.Def == nil {
		t.Error("(3,3) hazard must be resolved to HazardDef")
	}
}

func TestLoadTerrain_BothShapesError(t *testing.T) {
	cfg := &world.CombatTerrainConfig{
		Grid:  []string{"."},
		Cells: []world.TerrainCellDef{{X: 0, Y: 0, Type: "difficult"}},
	}
	_, err := world.LoadCombatTerrain(cfg, 1, 1, nil)
	if err == nil {
		t.Error("declaring both grid and cells must be a load error")
	}
}

func TestLoadTerrain_HazardousWithoutHazardError(t *testing.T) {
	cfg := &world.CombatTerrainConfig{
		Cells: []world.TerrainCellDef{
			{X: 0, Y: 0, Type: "hazardous"},
		},
	}
	_, err := world.LoadCombatTerrain(cfg, 10, 10, nil)
	if err == nil {
		t.Error("hazardous cell without a hazard must be a load error")
	}
}

func TestLoadTerrain_GreaterDifficultWithHazardError(t *testing.T) {
	cfg := &world.CombatTerrainConfig{
		Cells: []world.TerrainCellDef{
			{X: 0, Y: 0, Type: "greater_difficult", HazardID: "fire_vent"},
		},
	}
	_, err := world.LoadCombatTerrain(cfg, 10, 10, nil)
	if err == nil {
		t.Error("greater_difficult with hazard must be a load error")
	}
}

func TestLoadTerrain_DuplicateCellError(t *testing.T) {
	cfg := &world.CombatTerrainConfig{
		Cells: []world.TerrainCellDef{
			{X: 1, Y: 1, Type: "difficult"},
			{X: 1, Y: 1, Type: "normal"},
		},
	}
	_, err := world.LoadCombatTerrain(cfg, 10, 10, nil)
	if err == nil {
		t.Error("duplicate (x,y) entries must be a load error")
	}
}

func TestLoadTerrain_GridDimensionMismatchError(t *testing.T) {
	cfg := &world.CombatTerrainConfig{
		Grid: []string{"....", "...."},
	}
	_, err := world.LoadCombatTerrain(cfg, 3, 2, nil)
	if err == nil {
		t.Error("grid row length mismatch must be a load error")
	}
}
