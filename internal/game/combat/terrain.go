package combat

import "github.com/cory-johannsen/mud/internal/game/world"

// GridCell is a 2D grid coordinate (column, row).
type GridCell struct {
	X, Y int
}

// TerrainType enumerates the four terrain types supported in v1.
type TerrainType string

const (
	TerrainNormal           TerrainType = "normal"
	TerrainDifficult        TerrainType = "difficult"
	TerrainGreaterDifficult TerrainType = "greater_difficult"
	TerrainHazardous        TerrainType = "hazardous"
)

// TerrainCell is one cell in the combat grid's terrain layer.
type TerrainCell struct {
	X, Y int
	Type TerrainType
	// DifficultOverlay, when true on a TerrainHazardous cell, makes the cell
	// cost 2 movement to enter instead of 1. Ignored on other types.
	DifficultOverlay bool
	// Hazard is populated iff Type == TerrainHazardous.
	Hazard *CellHazard
}

// CellHazard is the resolved hazard descriptor for a hazardous cell.
type CellHazard struct {
	HazardID string
	Def      *world.HazardDef
}

// TerrainAt returns the TerrainCell at (x, y). Absent cells map to TerrainNormal.
//
// Postcondition: returned cell Type is always a valid TerrainType.
func (c *Combat) TerrainAt(x, y int) TerrainCell {
	if c.Terrain == nil {
		return TerrainCell{X: x, Y: y, Type: TerrainNormal}
	}
	if tc, ok := c.Terrain[GridCell{X: x, Y: y}]; ok {
		return *tc
	}
	return TerrainCell{X: x, Y: y, Type: TerrainNormal}
}

// EntryCost returns the speed budget cost to enter cell (x, y).
// ok=false means the cell is impassable (TerrainGreaterDifficult).
//
//   - TerrainNormal: 1
//   - TerrainDifficult: 2
//   - TerrainHazardous: 1 (or 2 if DifficultOverlay)
//   - TerrainGreaterDifficult: impassable (ok=false)
func (c *Combat) EntryCost(x, y int) (cost int, ok bool) {
	tc := c.TerrainAt(x, y)
	switch tc.Type {
	case TerrainGreaterDifficult:
		return 0, false
	case TerrainDifficult:
		return 2, true
	case TerrainHazardous:
		if tc.DifficultOverlay {
			return 2, true
		}
		return 1, true
	default:
		return 1, true
	}
}
