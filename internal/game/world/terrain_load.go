package world

import (
	"fmt"
	"strings"
)

// CellKey is the world-package equivalent of combat.GridCell, used as a map key.
type CellKey struct{ X, Y int }

// ResolvedTerrainCell is the normalised per-cell result.
type ResolvedTerrainCell struct {
	Type             string
	DifficultOverlay bool
	Hazard           *ResolvedCellHazard
}

// ResolvedCellHazard carries the resolved hazard for a hazardous cell.
type ResolvedCellHazard struct {
	HazardID string
	Def      *HazardDef
}

// LoadCombatTerrain normalises a CombatTerrainConfig into a sparse map.
// Only non-normal cells are present in the result.
func LoadCombatTerrain(cfg *CombatTerrainConfig, gridWidth, gridHeight int, hazards []HazardDef) (map[CellKey]*ResolvedTerrainCell, error) {
	if cfg == nil {
		return nil, fmt.Errorf("terrain config must not be nil")
	}
	hasGrid := len(cfg.Grid) > 0
	hasCells := len(cfg.Cells) > 0
	if hasGrid && hasCells {
		return nil, fmt.Errorf("combat_terrain: cannot declare both grid and cells in the same room (TERRAIN-3)")
	}
	hazardMap := make(map[string]*HazardDef, len(hazards))
	for i := range hazards {
		hazardMap[hazards[i].ID] = &hazards[i]
	}
	if hasGrid {
		return loadShapeA(cfg, gridWidth, gridHeight, hazardMap)
	}
	return loadShapeB(cfg, hazardMap)
}

var defaultLegend = map[string]TerrainLegendEntry{
	".": {Type: "normal"},
	"#": {Type: "difficult"},
	"X": {Type: "greater_difficult"},
	"H": {Type: "hazardous"},
}

func loadShapeA(cfg *CombatTerrainConfig, gridWidth, gridHeight int, hazards map[string]*HazardDef) (map[CellKey]*ResolvedTerrainCell, error) {
	if len(cfg.Grid) != gridHeight {
		return nil, fmt.Errorf("combat_terrain.grid: %d rows declared but grid height is %d", len(cfg.Grid), gridHeight)
	}
	legend := make(map[string]TerrainLegendEntry, len(defaultLegend)+len(cfg.Legend))
	for k, v := range defaultLegend {
		legend[k] = v
	}
	for k, v := range cfg.Legend {
		legend[k] = v
	}
	result := make(map[CellKey]*ResolvedTerrainCell)
	for y, row := range cfg.Grid {
		runes := []rune(row)
		if len(runes) != gridWidth {
			return nil, fmt.Errorf("combat_terrain.grid: row %d has %d columns, expected %d", y, len(runes), gridWidth)
		}
		for x, ch := range runes {
			glyph := string(ch)
			entry, ok := legend[glyph]
			if !ok {
				return nil, fmt.Errorf("combat_terrain.grid: unknown glyph %q at (%d,%d) — add it to the legend", glyph, x, y)
			}
			if entry.Type == "normal" {
				continue
			}
			tc, err := resolveCell(entry.Type, entry.Difficult, entry.HazardID, entry.Hazard, hazards)
			if err != nil {
				return nil, fmt.Errorf("combat_terrain.grid row %d col %d: %w", y, x, err)
			}
			result[CellKey{X: x, Y: y}] = tc
		}
	}
	return result, nil
}

func loadShapeB(cfg *CombatTerrainConfig, hazards map[string]*HazardDef) (map[CellKey]*ResolvedTerrainCell, error) {
	result := make(map[CellKey]*ResolvedTerrainCell)
	seen := make(map[CellKey]bool)
	for _, cd := range cfg.Cells {
		key := CellKey{X: cd.X, Y: cd.Y}
		if seen[key] {
			return nil, fmt.Errorf("combat_terrain.cells: duplicate entry for (%d,%d) (TERRAIN-3)", cd.X, cd.Y)
		}
		seen[key] = true
		if cd.Type == "normal" {
			continue
		}
		tc, err := resolveCell(cd.Type, cd.Difficult, cd.HazardID, cd.Hazard, hazards)
		if err != nil {
			return nil, fmt.Errorf("combat_terrain.cells (%d,%d): %w", cd.X, cd.Y, err)
		}
		result[key] = tc
	}
	return result, nil
}

func resolveCell(typ string, difficult bool, hazardID string, inlineHazard *HazardDef, hazards map[string]*HazardDef) (*ResolvedTerrainCell, error) {
	tc := &ResolvedTerrainCell{Type: typ}
	switch typ {
	case "greater_difficult":
		if hazardID != "" || inlineHazard != nil {
			return nil, fmt.Errorf("greater_difficult must not carry a hazard (TERRAIN-5)")
		}
		if difficult {
			return nil, fmt.Errorf("greater_difficult must not combine with difficult (TERRAIN-5)")
		}
	case "hazardous":
		if hazardID == "" && inlineHazard == nil {
			return nil, fmt.Errorf("hazardous cell must declare hazard_id or hazard (TERRAIN-4)")
		}
		if hazardID != "" && inlineHazard != nil {
			return nil, fmt.Errorf("hazardous cell may not declare both hazard_id and inline hazard (TERRAIN-4)")
		}
		tc.DifficultOverlay = difficult
		rch := &ResolvedCellHazard{HazardID: hazardID}
		if inlineHazard != nil {
			if err := inlineHazard.Validate(); err != nil {
				return nil, fmt.Errorf("inline hazard invalid: %w", err)
			}
			rch.Def = inlineHazard
		} else if def, ok := hazards[hazardID]; ok {
			rch.Def = def
		}
		tc.Hazard = rch
	case "difficult", "normal":
		if hazardID != "" || inlineHazard != nil {
			return nil, fmt.Errorf("non-hazardous cell type %q must not carry a hazard (TERRAIN-4)", typ)
		}
	default:
		return nil, fmt.Errorf("unknown terrain type %q; valid: normal difficult greater_difficult hazardous", typ)
	}
	tc.Type = strings.TrimSpace(typ)
	return tc, nil
}
