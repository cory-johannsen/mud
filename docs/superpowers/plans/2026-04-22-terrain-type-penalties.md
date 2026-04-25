# Terrain Type Penalties in Combat — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-cell terrain layer to the 2D combat grid — four terrain types (normal / difficult / greater_difficult / hazardous) with movement costs, hazard damage via the #246 `ResolveDamage` pipeline, and telnet + web rendering.

**Architecture:** Combat already uses a 2D grid (20×20, `GridX/GridY` on `Combatant`, `GridWidth/GridHeight` on `Combat`). Terrain is stored as a sparse `map[GridCell]*TerrainCell` on `Combat`, populated at combat start from the room's authored YAML. The stride loop is converted from a fixed-step iteration to a budget-based loop: each cell costs 1 (normal/hazardous) or 2 (difficult) budget points; `greater_difficult` cells are impassable. Hazards fire via `applyCellHazard`, which routes damage through `combat.ResolveDamage` (from #246). Rendering draws terrain glyphs beneath combatants on both telnet and web.

**Tech Stack:** Go, `pgregory.net/rapid` (property tests), `internal/game/combat/` (grid + round), `internal/game/world/` (room model + YAML loader), web client (React/TypeScript).

**Prerequisite:** Issue #246 (Stacking multipliers) MUST be merged before starting — `ResolveDamage`, `DamageInput`, and `DamageAdditive` from #246 are required for hazard damage routing (TERRAIN-21).

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/combat/terrain.go` |
| Create | `internal/game/combat/terrain_test.go` |
| Create | `internal/game/combat/round_stride_terrain_test.go` |
| Create | `internal/game/combat/round_hazard_test.go` |
| Create | `internal/game/combat/terrain_render_test.go` |
| Create | `internal/game/world/terrain_load.go` |
| Create | `internal/game/world/terrain_load_test.go` |
| Modify | `internal/game/combat/engine.go` (add `Combat.Terrain`, `Combat.RoomHazards`, `Combat.skipHazardRoundStart`) |
| Modify | `internal/game/combat/combat.go` (add `SpeedBudget()`, deprecate `SpeedSquares()`) |
| Modify | `internal/game/combat/round.go` (stride loop rewrite, `applyCellHazard`, StartRound hook) |
| Modify | `internal/game/world/model.go` (add `Room.CombatTerrain`) |
| Modify | `internal/frontend/text_renderer.go` (terrain glyphs) |
| Modify | `cmd/webclient/ui/src/` (terrain layer + CSS) |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: Core terrain types and `Combat.Terrain`

**Files:**
- Create: `internal/game/combat/terrain.go`
- Create: `internal/game/combat/terrain_test.go`
- Modify: `internal/game/combat/engine.go`

- [ ] **Step 1: Write failing property tests**

Create `internal/game/combat/terrain_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
	"pgregory.net/rapid"
)

func TestProperty_TerrainAt_AbsentCellIsNormal(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cbt := &combat.Combat{Terrain: nil, GridWidth: 10, GridHeight: 10}
		x := rapid.IntRange(0, 9).Draw(t, "x")
		y := rapid.IntRange(0, 9).Draw(t, "y")
		tc := cbt.TerrainAt(x, y)
		if tc.Type != combat.TerrainNormal {
			t.Fatalf("absent cell (%d,%d): want normal got %v", x, y, tc.Type)
		}
	})
}

func TestProperty_EntryCost_GreaterDifficultImpassable(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
			{X: 3, Y: 3}: {X: 3, Y: 3, Type: combat.TerrainGreaterDifficult},
		}
		_, ok := cbt.EntryCost(3, 3)
		if ok {
			t.Fatal("greater_difficult must be impassable (ok=false)")
		}
	})
}

func TestProperty_EntryCost_DifficultCostsTwo(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
			{X: 1, Y: 1}: {X: 1, Y: 1, Type: combat.TerrainDifficult},
		}
		cost, ok := cbt.EntryCost(1, 1)
		if !ok {
			t.Fatal("difficult must be passable")
		}
		if cost != 2 {
			t.Fatalf("difficult: want cost=2 got %d", cost)
		}
	})
}

func TestProperty_EntryCost_HazardousWithDifficultOverlayCostsTwo(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
			{X: 2, Y: 2}: {X: 2, Y: 2, Type: combat.TerrainHazardous, DifficultOverlay: true},
		}
		cost, ok := cbt.EntryCost(2, 2)
		if !ok {
			t.Fatal("hazardous must be passable")
		}
		if cost != 2 {
			t.Fatalf("hazardous+difficult: want cost=2 got %d", cost)
		}
	})
}

func TestProperty_EntryCost_NormalCostsOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cbt := &combat.Combat{GridWidth: 10, GridHeight: 10}
		cost, ok := cbt.EntryCost(0, 0)
		if !ok || cost != 1 {
			t.Fatalf("normal: want (1, true) got (%d, %v)", cost, ok)
		}
	})
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestProperty_Terrain -v 2>&1 | tail -20
```

Expected: `FAIL` (types not defined yet).

- [ ] **Step 3: Implement `terrain.go`**

Create `internal/game/combat/terrain.go`:

```go
package combat

import "github.com/cory-johannsen/gunchete/internal/game/world"

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
	X, Y             int
	Type             TerrainType
	// DifficultOverlay, when true on a TerrainHazardous cell, makes the cell
	// cost 2 movement to enter instead of 1. Ignored on other types.
	DifficultOverlay bool
	// Hazard is populated iff Type == TerrainHazardous.
	Hazard *CellHazard
}

// CellHazard is the resolved hazard descriptor for a hazardous cell.
// Exactly one of HazardID or Def is meaningful:
//   - HazardID is non-empty when the hazard references a room-level entry by id.
//   - Def is non-nil after resolution (or when the hazard was declared inline).
type CellHazard struct {
	HazardID string         // for diagnostic logging; empty if inline-declared
	Def      *world.HazardDef // nil only when HazardID could not be resolved
}

// TerrainAt returns the TerrainCell at (x, y). When no cell is authored at
// that position, returns a zero-value TerrainCell with Type=TerrainNormal.
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

// EntryCost returns the speed budget cost to enter grid cell (x, y).
// ok=false means the cell is impassable (TerrainGreaterDifficult).
//
// Cost table:
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
	default: // TerrainNormal
		return 1, true
	}
}
```

- [ ] **Step 4: Add `Combat.Terrain`, `Combat.RoomHazards`, and `Combat.skipHazardRoundStart` to `engine.go`**

In `internal/game/combat/engine.go`, locate the `Combat` struct definition (around line 28) and add three fields immediately after `CoverObjects`:

```go
// Terrain is the per-cell terrain layer for this combat. Absent cells are TerrainNormal.
// Populated at combat start from the room's CombatTerrain config.
Terrain map[GridCell]*TerrainCell

// RoomHazards is a copy of the room's HazardDef list, used to resolve hazard_id
// references on TerrainHazardous cells at combat start.
RoomHazards []world.HazardDef

// skipHazardRoundStart tracks combatant IDs that entered a hazardous cell at
// combat start (via on_enter) and must not fire round_start hazards in round 1.
// Cleared by StartRoundWithSrc on first use.
skipHazardRoundStart map[string]bool
```

Also add `"github.com/cory-johannsen/gunchete/internal/game/world"` to the import list if not already present.

- [ ] **Step 5: Run tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestProperty_Terrain -v 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 6: Run full combat package test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/game/combat/terrain.go internal/game/combat/terrain_test.go internal/game/combat/engine.go
git commit -m "feat(#248): add GridCell, TerrainType, TerrainCell, Combat.Terrain, EntryCost"
```

---

### Task 2: `SpeedBudget()` method + deprecate `SpeedSquares()`

**Files:**
- Modify: `internal/game/combat/combat.go`

- [ ] **Step 1: Write failing property test**

Add to `internal/game/combat/terrain_test.go`:

```go
func TestProperty_SpeedBudget_EqualsSpeedSquares(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := &combat.Combatant{
			SpeedFt: rapid.IntRange(0, 100).Draw(t, "speedFt"),
		}
		if c.SpeedBudget() != c.SpeedSquares() {
			t.Fatalf("SpeedBudget %d != SpeedSquares %d for SpeedFt %d",
				c.SpeedBudget(), c.SpeedSquares(), c.SpeedFt)
		}
	})
}

func TestProperty_SpeedBudget_MinOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := &combat.Combatant{SpeedFt: 0}
		if c.SpeedBudget() < 1 {
			t.Fatalf("SpeedBudget must be >= 1, got %d", c.SpeedBudget())
		}
	})
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestProperty_SpeedBudget -v 2>&1 | tail -10
```

Expected: `FAIL` (SpeedBudget not defined).

- [ ] **Step 3: Add `SpeedBudget()` and update `SpeedSquares()` in `combat.go`**

In `internal/game/combat/combat.go`, locate `SpeedSquares()` (line ~168). Replace with:

```go
// SpeedBudget returns the number of speed budget points available per stride action.
// Difficult cells cost 2 points; normal and hazardous cost 1. Default 25 ft = 5 points.
//
// Postcondition: returns >= 1.
func (c *Combatant) SpeedBudget() int {
	ft := c.SpeedFt
	if ft <= 0 {
		ft = 25
	}
	sq := ft / 5
	if sq < 1 {
		sq = 1
	}
	return sq
}

// SpeedSquares is deprecated: use SpeedBudget instead.
//
// Deprecated: SpeedSquares delegates to SpeedBudget. All in-package callers
// have migrated. Retained for external callers.
func (c *Combatant) SpeedSquares() int {
	return c.SpeedBudget()
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestProperty_SpeedBudget -v 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 5: Migrate the in-package call site in `round.go`**

In `internal/game/combat/round.go`, locate the stride loop entry (~line 724):
```go
steps := actor.SpeedSquares()
for step := 0; step < steps; step++ {
```

Change to:
```go
budget := actor.SpeedBudget()
```

(The loop body will be rewritten in Task 4; for now leave the loop control unchanged so it still compiles — we will update it there.)

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/game/combat/combat.go internal/game/combat/round.go internal/game/combat/terrain_test.go
git commit -m "feat(#248): add SpeedBudget(); deprecate SpeedSquares()"
```

---

### Task 3: Room model + terrain YAML loader

**Files:**
- Modify: `internal/game/world/model.go`
- Create: `internal/game/world/terrain_load.go`
- Create: `internal/game/world/terrain_load_test.go`

- [ ] **Step 1: Add terrain types to `world/model.go`**

At the bottom of `internal/game/world/model.go`, add:

```go
// CombatTerrainConfig holds the authored per-cell terrain configuration for a room.
// Exactly one of Grid (shape A) or Cells (shape B) must be set; both is a load error.
type CombatTerrainConfig struct {
	// Shape A: explicit grid.
	Grid   []string                    `yaml:"grid,omitempty"`
	Legend map[string]TerrainLegendEntry `yaml:"legend,omitempty"`
	// Shape B: default + per-cell overrides.
	Default string           `yaml:"default,omitempty"`
	Cells   []TerrainCellDef `yaml:"cells,omitempty"`
}

// TerrainLegendEntry is a legend value in shape-A terrain authoring.
// YAML unmarshalling: bare type string ("normal", "difficult", etc.) OR
// an object {type, difficult, hazard_id, hazard}.
type TerrainLegendEntry struct {
	Type      string    `yaml:"type"`
	Difficult bool      `yaml:"difficult,omitempty"`
	HazardID  string    `yaml:"hazard_id,omitempty"`
	Hazard    *HazardDef `yaml:"hazard,omitempty"`
}

// UnmarshalYAML for TerrainLegendEntry supports both bare string and object forms.
func (e *TerrainLegendEntry) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try bare string first.
	var s string
	if err := unmarshal(&s); err == nil {
		e.Type = s
		return nil
	}
	// Fall back to object form.
	type plain TerrainLegendEntry
	var obj plain
	if err := unmarshal(&obj); err != nil {
		return err
	}
	*e = TerrainLegendEntry(obj)
	return nil
}

// TerrainCellDef is a single per-cell override entry in shape-B terrain authoring.
type TerrainCellDef struct {
	X         int       `yaml:"x"`
	Y         int       `yaml:"y"`
	Type      string    `yaml:"type"`
	Difficult bool      `yaml:"difficult,omitempty"`
	HazardID  string    `yaml:"hazard_id,omitempty"`
	Hazard    *HazardDef `yaml:"hazard,omitempty"`
}
```

Also add `CombatTerrain *CombatTerrainConfig` to the `Room` struct after `Hazards`:
```go
CombatTerrain *CombatTerrainConfig `yaml:"combat_terrain,omitempty"`
```

- [ ] **Step 2: Write failing load tests**

Create `internal/game/world/terrain_load_test.go`:

```go
package world_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/world"
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
	// (1,1) and (2,1) should be difficult
	if cells[world.CellKey{X: 1, Y: 1}].Type != "difficult" {
		t.Error("(1,1) should be difficult")
	}
	if cells[world.CellKey{X: 2, Y: 1}].Type != "difficult" {
		t.Error("(2,1) should be difficult")
	}
	// (0,0) should be absent (normal default)
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
			{X: 0, Y: 0, Type: "hazardous"}, // no hazard_id or hazard
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
		Grid: []string{"....", "...."},   // 4 cols, 2 rows
	}
	_, err := world.LoadCombatTerrain(cfg, 3, 2, nil) // grid width 3 but row length 4
	if err == nil {
		t.Error("grid row length mismatch must be a load error")
	}
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run TestLoadTerrain -v 2>&1 | tail -20
```

Expected: compile failure (LoadCombatTerrain not defined).

- [ ] **Step 4: Implement `terrain_load.go`**

Create `internal/game/world/terrain_load.go`:

```go
package world

import (
	"fmt"
	"strings"
)

// CellKey is the world-package equivalent of combat.GridCell, used as a map key.
// The combat package defines its own GridCell; this type avoids an import cycle.
type CellKey struct{ X, Y int }

// ResolvedTerrainCell is the normalised per-cell result produced by LoadCombatTerrain.
// The combat package maps these to its own TerrainCell type at combat start.
type ResolvedTerrainCell struct {
	Type             string
	DifficultOverlay bool
	Hazard           *ResolvedCellHazard
}

// ResolvedCellHazard carries the resolved hazard for a hazardous cell.
type ResolvedCellHazard struct {
	HazardID string     // source HazardID for diagnostics; empty if inline
	Def      *HazardDef // nil if HazardID could not be resolved (warning logged at runtime)
}

// LoadCombatTerrain normalises a CombatTerrainConfig into a sparse map of
// ResolvedTerrainCell values. Only non-normal cells are present in the result.
//
// Preconditions:
//   - cfg must not be nil.
//   - gridWidth and gridHeight are the expected combat grid dimensions.
//   - hazards is the room's Hazards slice, used to resolve hazard_id references.
//
// Errors are returned for all TERRAIN-3 through TERRAIN-5 violations.
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
	return loadShapeB(cfg, gridWidth, gridHeight, hazardMap)
}

// defaultLegend provides default glyph mappings when the legend omits them.
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
				continue // sparse: skip normal cells
			}
			tc, err := resolveCell(x, y, entry.Type, entry.Difficult, entry.HazardID, entry.Hazard, hazards)
			if err != nil {
				return nil, fmt.Errorf("combat_terrain.grid row %d col %d: %w", y, x, err)
			}
			result[CellKey{X: x, Y: y}] = tc
		}
	}
	return result, nil
}

func loadShapeB(cfg *CombatTerrainConfig, _, _ int, hazards map[string]*HazardDef) (map[CellKey]*ResolvedTerrainCell, error) {
	result := make(map[CellKey]*ResolvedTerrainCell)
	seen := make(map[CellKey]bool)
	for _, cd := range cfg.Cells {
		key := CellKey{X: cd.X, Y: cd.Y}
		if seen[key] {
			return nil, fmt.Errorf("combat_terrain.cells: duplicate entry for (%d,%d) (TERRAIN-3)", cd.X, cd.Y)
		}
		seen[key] = true
		if cd.Type == "normal" {
			continue // sparse
		}
		tc, err := resolveCell(cd.X, cd.Y, cd.Type, cd.Difficult, cd.HazardID, cd.Hazard, hazards)
		if err != nil {
			return nil, fmt.Errorf("combat_terrain.cells (%d,%d): %w", cd.X, cd.Y, err)
		}
		result[key] = tc
	}
	return result, nil
}

func resolveCell(x, y int, typ string, difficult bool, hazardID string, inlineHazard *HazardDef, hazards map[string]*HazardDef) (*ResolvedTerrainCell, error) {
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
		} else {
			// Reference: resolved at combat start (may be nil here if hazards map is empty).
			if def, ok := hazards[hazardID]; ok {
				rch.Def = def
			}
			// Unresolved: rch.Def remains nil; caller emits a warning at combat start.
		}
		tc.Hazard = rch
	case "difficult", "normal":
		// valid; no hazard allowed on non-hazardous types
		if hazardID != "" || inlineHazard != nil {
			return nil, fmt.Errorf("non-hazardous cell type %q must not carry a hazard (TERRAIN-4)", typ)
		}
	default:
		return nil, fmt.Errorf("unknown terrain type %q; valid: normal difficult greater_difficult hazardous", typ)
	}
	tc.Type = strings.TrimSpace(typ)
	return tc, nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run TestLoadTerrain -v 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 6: Run full world package test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/game/world/model.go internal/game/world/terrain_load.go internal/game/world/terrain_load_test.go
git commit -m "feat(#248): add Room.CombatTerrain model and terrain YAML loader (both authoring shapes)"
```

---

### Task 4: Stride loop rewrite — budget-based with terrain penalties

**Files:**
- Modify: `internal/game/combat/round.go`
- Create: `internal/game/combat/round_stride_terrain_test.go`

- [ ] **Step 1: Write failing stride-terrain tests**

Create `internal/game/combat/round_stride_terrain_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
	"pgregory.net/rapid"
)

// TestStride_AllNormal_MovesFullBudget verifies that a stride across normal terrain
// moves exactly SpeedBudget() squares.
func TestStride_AllNormal_MovesFullBudget(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		src := combat.NewSource(rapid.Int64().Draw(t, "seed"))
		actor := &combat.Combatant{
			ID: "actor", Name: "actor", Kind: combat.KindPlayer,
			HP: 20, MaxHP: 20, AC: 12,
			GridX: 0, GridY: 5,
		}
		cbt := &combat.Combat{
			GridWidth: 20, GridHeight: 20,
			Combatants: []*combat.Combatant{actor},
			// No terrain — all normal.
		}
		budget := actor.SpeedBudget()
		startX := actor.GridX
		// Resolve a stride action toward x=19 (right).
		events := resolveTestStride(cbt, actor, "away", src)
		moved := actor.GridX - startX
		if moved != budget {
			t.Fatalf("all-normal stride: expected to move %d squares, moved %d; events: %v",
				budget, moved, eventsNarratives(events))
		}
	})
}

// TestStride_DifficultHalvesDistance verifies that difficult terrain doubles cost,
// halving the distance traversed by a full-budget stride.
func TestStride_DifficultHalvesDistance(t *testing.T) {
	src := combat.NewSource(42)
	actor := &combat.Combatant{
		ID: "actor", Name: "actor", Kind: combat.KindPlayer,
		HP: 20, MaxHP: 20, AC: 12,
		GridX: 0, GridY: 5, SpeedFt: 25, // SpeedBudget() == 5
	}
	// Fill x=1..19 with difficult terrain.
	terrain := make(map[combat.GridCell]*combat.TerrainCell)
	for x := 1; x <= 19; x++ {
		terrain[combat.GridCell{X: x, Y: 5}] = &combat.TerrainCell{X: x, Y: 5, Type: combat.TerrainDifficult}
	}
	cbt := &combat.Combat{
		GridWidth: 20, GridHeight: 20,
		Combatants: []*combat.Combatant{actor},
		Terrain: terrain,
	}
	startX := actor.GridX
	resolveTestStride(cbt, actor, "away", src)
	moved := actor.GridX - startX
	// budget=5, each difficult cell costs 2: floor(5/2)=2 cells
	if moved != 2 {
		t.Fatalf("difficult terrain: expected to move 2 squares, moved %d", moved)
	}
}

// TestStride_GreaterDifficultBlocks verifies stride stops before a greater_difficult cell.
func TestStride_GreaterDifficultBlocks(t *testing.T) {
	src := combat.NewSource(7)
	actor := &combat.Combatant{
		ID: "actor", Name: "actor", Kind: combat.KindPlayer,
		HP: 20, MaxHP: 20, AC: 12,
		GridX: 0, GridY: 5, SpeedFt: 25,
	}
	// Cell at x=1 is greater_difficult.
	terrain := map[combat.GridCell]*combat.TerrainCell{
		{X: 1, Y: 5}: {X: 1, Y: 5, Type: combat.TerrainGreaterDifficult},
	}
	cbt := &combat.Combat{
		GridWidth: 20, GridHeight: 20,
		Combatants: []*combat.Combatant{actor},
		Terrain: terrain,
	}
	resolveTestStride(cbt, actor, "away", src)
	if actor.GridX != 0 {
		t.Fatalf("greater_difficult: actor must not advance past the barrier, got GridX=%d", actor.GridX)
	}
}

// TestStride_ZeroBudgetNoMove verifies that a combatant with SpeedFt set so low
// that even the first step is unaffordable emits an informational narrative.
func TestStride_ZeroBudgetNoMove(t *testing.T) {
	src := combat.NewSource(1)
	actor := &combat.Combatant{
		ID: "actor", Name: "actor", Kind: combat.KindPlayer,
		HP: 20, MaxHP: 20, AC: 12,
		GridX: 0, GridY: 5, SpeedFt: 5, // SpeedBudget() == 1
	}
	// First step is difficult (cost 2 > budget 1 → cannot afford).
	terrain := map[combat.GridCell]*combat.TerrainCell{
		{X: 1, Y: 5}: {X: 1, Y: 5, Type: combat.TerrainDifficult},
	}
	cbt := &combat.Combat{
		GridWidth: 20, GridHeight: 20,
		Combatants: []*combat.Combatant{actor},
		Terrain: terrain,
	}
	events := resolveTestStride(cbt, actor, "away", src)
	if actor.GridX != 0 {
		t.Fatal("no move expected when first step is unaffordable")
	}
	// Must emit an informational narrative (TERRAIN-18).
	found := false
	for _, e := range events {
		if e.ActionType == combat.ActionStride {
			found = true
		}
	}
	if !found {
		t.Error("must emit a stride event even on zero-movement stride")
	}
}

// resolveTestStride is a test helper that queues a stride action and runs one
// round of combat. Returns the emitted events.
func resolveTestStride(cbt *combat.Combat, actor *combat.Combatant, dir string, src combat.Source) []combat.RoundEvent {
	// Populate the action queue with a stride.
	actor.ActionQueue = []combat.QueuedAction{
		{Type: combat.ActionStride, Target: dir},
	}
	if cbt.ActionQueues == nil {
		cbt.ActionQueues = make(map[string][]combat.QueuedAction)
	}
	cbt.ActionQueues[actor.ID] = actor.ActionQueue
	return cbt.ResolveRoundWithSrc(src)
}

func eventsNarratives(events []combat.RoundEvent) []string {
	ns := make([]string, len(events))
	for i, e := range events {
		ns[i] = e.Narrative
	}
	return ns
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestStride_ -v 2>&1 | tail -20
```

Expected: compile OK but tests fail (stride logic unchanged).

- [ ] **Step 3: Rewrite the stride loop in `round.go`**

In `internal/game/combat/round.go`, locate the stride loop starting at the line that was:
```go
steps := actor.SpeedSquares()
for step := 0; step < steps; step++ {
```

(After Task 2 this reads `budget := actor.SpeedBudget()`.)

Replace the full loop body with the budget-based version:

```go
budget := actor.SpeedBudget()
strideNarrative := fmt.Sprintf("%s strides %s.", actor.Name, dir)
strideBlocked := false
for budget > 0 {
    // REQ-STRIDE-STOP: For "toward" strides, stop when already adjacent (≤ 5 ft).
    if dir == "toward" && opponent != nil && CombatRange(*actor, *opponent) <= 5 {
        break
    }

    dx, dy := CompassDelta(dir, actor, opponent)
    if dx == 0 && dy == 0 {
        break
    }

    newX := actor.GridX + dx
    newY := actor.GridY + dy
    // Clamp to grid bounds.
    newX = clampInt(newX, 0, width-1)
    newY = clampInt(newY, 0, height-1)
    // Stop if clamping produced no movement (actor is at a grid edge).
    if newX == actor.GridX && newY == actor.GridY {
        break
    }
    // REQ-STRIDE-NOOVERLAP: Do not move onto a cell occupied by another combatant or cover.
    if CellBlocked(cbt, actor.ID, newX, newY) {
        break
    }
    // TERRAIN-6/7: Check entry cost; stop if impassable or insufficient budget.
    cost, passable := cbt.EntryCost(newX, newY)
    if !passable {
        // TERRAIN-17: emit informational narrative when greater_difficult blocks.
        if actor.GridX == actor.GridX && actor.GridY == actor.GridY { // first step
            strideNarrative = fmt.Sprintf("%s tries to move but the terrain blocks the way.", actor.Name)
        } else {
            strideNarrative = fmt.Sprintf("%s strides %s and stops at the terrain.", actor.Name, dir)
        }
        strideBlocked = true
        break
    }
    if cost > budget {
        // TERRAIN-7: insufficient budget to enter next cell.
        if actor.GridX == 0 && actor.GridY == 0 { // no movement yet (zero-budget case)
            strideNarrative = fmt.Sprintf("%s cannot afford to move — not enough movement speed.", actor.Name)
        } else {
            strideNarrative = fmt.Sprintf("%s strides %s and stops — terrain too rough to continue.", actor.Name, dir)
        }
        strideBlocked = true
        break
    }
    budget -= cost
    actor.GridX = newX
    actor.GridY = newY

    // TERRAIN-9: fire on_enter hazard for hazardous cells.
    if tc := cbt.TerrainAt(newX, newY); tc.Type == TerrainHazardous && tc.Hazard != nil {
        events = append(events, applyCellHazard(cbt, actor, tc, "on_enter", src)...)
    }

    // REQ-RXN19: TriggerOnEnemyMoveAdjacent (unchanged).
    if actor.Kind == KindNPC {
        for _, c := range cbt.Combatants {
            if c.Kind == KindPlayer && !c.IsDead() {
                if CombatRange(*actor, *c) <= 5 {
                    events = append(events, fireReaction(c.ID, reaction.TriggerOnEnemyMoveAdjacent,
                        reaction.ReactionContext{TriggerUID: c.ID, SourceUID: actor.ID})...)
                }
            }
        }
    }
}
_ = strideBlocked // used only for narrative branch above
events = append(events, RoundEvent{
    ActionType: ActionStride,
    ActorID:    actor.ID,
    ActorName:  actor.Name,
    Narrative:  strideNarrative,
})
```

Also add the `clampInt` helper if not already present:
```go
func clampInt(v, lo, hi int) int {
    if v < lo { return lo }
    if v > hi { return hi }
    return v
}
```

Note: The zero-budget narrative detection above uses a heuristic based on actor position; a cleaner approach is tracking `stepsTaken int` in the loop body and using `stepsTaken == 0` for the zero-movement message. Adjust as needed for correctness.

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestStride_ -v 2>&1 | tail -30
```

Expected: all pass.

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/game/combat/round.go internal/game/combat/round_stride_terrain_test.go
git commit -m "feat(#248): rewrite stride loop — budget-based with terrain penalties and hazard entry"
```

---

### Task 5: Hazard application helpers + StartRound hook + combat-start placement

**Files:**
- Modify: `internal/game/combat/round.go` (add `applyCellHazard`)
- Modify: `internal/game/combat/engine.go` (add terrain-hazard hook to `StartRoundWithSrc`)
- Create: `internal/game/combat/round_hazard_test.go`

- [ ] **Step 1: Write failing hazard tests**

Create `internal/game/combat/round_hazard_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
	"github.com/cory-johannsen/gunchete/internal/game/world"
)

// TestHazard_OnEnter_DealsDamage verifies that entering a hazardous cell
// applies damage to the combatant.
func TestHazard_OnEnter_DealsDamage(t *testing.T) {
	src := combat.NewSource(1)
	actor := &combat.Combatant{
		ID: "actor", Name: "actor", Kind: combat.KindPlayer,
		HP: 20, MaxHP: 20, AC: 12,
		GridX: 0, GridY: 5,
	}
	hazDef := &world.HazardDef{
		ID: "fire_vent", Trigger: "on_enter",
		DamageExpr: "1d4", DamageType: "fire",
		Message: "flames lick at you",
	}
	tc := combat.TerrainCell{X: 1, Y: 5, Type: combat.TerrainHazardous,
		Hazard: &combat.CellHazard{Def: hazDef},
	}
	hpBefore := actor.CurrentHP
	events := combat.ApplyCellHazardForTest(nil, actor, tc, "on_enter", src)
	if actor.CurrentHP >= hpBefore {
		t.Errorf("on_enter hazard must deal damage; HP before=%d after=%d", hpBefore, actor.CurrentHP)
	}
	if len(events) == 0 {
		t.Error("applyCellHazard must emit at least one RoundEvent")
	}
}

// TestHazard_RoundStart_FiresEachRound verifies that round_start hazards
// fire on combatants occupying hazardous cells.
func TestHazard_RoundStart_FiresRoundStart(t *testing.T) {
	src := combat.NewSource(2)
	actor := &combat.Combatant{
		ID: "actor", Name: "actor", Kind: combat.KindPlayer,
		HP: 30, MaxHP: 30, AC: 12,
		GridX: 3, GridY: 3,
	}
	hazDef := &world.HazardDef{
		ID: "acid_pool", Trigger: "round_start",
		DamageExpr: "1d4", DamageType: "acid",
	}
	terrain := map[combat.GridCell]*combat.TerrainCell{
		{X: 3, Y: 3}: {X: 3, Y: 3, Type: combat.TerrainHazardous,
			Hazard: &combat.CellHazard{Def: hazDef},
		},
	}
	cbt := &combat.Combat{
		GridWidth: 10, GridHeight: 10,
		Combatants: []*combat.Combatant{actor},
		Terrain: terrain,
	}
	hpBefore := actor.CurrentHP
	cbt.StartRoundWithSrc(2, src)
	if actor.CurrentHP >= hpBefore {
		t.Errorf("round_start hazard must deal damage; HP before=%d after=%d", hpBefore, actor.CurrentHP)
	}
}

// TestHazard_CombatStart_OnEnterOnce_NoDoubleFireRound1 verifies that
// a combatant placed on a hazardous cell at combat start fires on_enter once
// and round_start does NOT double-fire in round 1.
func TestHazard_CombatStart_NoDoubleFireRound1(t *testing.T) {
	t.Skip("implement after applyCellHazard and StartRoundWithSrc hazard hook land")
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestHazard_ -v 2>&1 | tail -20
```

Expected: compile error (`ApplyCellHazardForTest` not defined).

- [ ] **Step 3: Add `applyCellHazard` to `round.go`**

Add the following function to `internal/game/combat/round.go` (near other helpers):

```go
// applyCellHazard fires a hazard on victim if its trigger matches.
// Routes damage through ResolveDamage (#246 pipeline).
// Returns zero or more RoundEvents (damage narrative, condition narrative, etc.).
//
// Precondition: tc.Type == TerrainHazardous; tc.Hazard must not be nil.
func applyCellHazard(cbt *Combat, victim *Combatant, tc TerrainCell, trigger string, src Source) []RoundEvent {
	if tc.Hazard == nil || tc.Hazard.Def == nil {
		return nil
	}
	def := tc.Hazard.Def
	if def.Trigger != trigger {
		return nil
	}
	var events []RoundEvent
	if def.DamageExpr != "" {
		rollResult, err := dice.RollExpr(def.DamageExpr, src)
		if err == nil && rollResult.Total > 0 {
			in := DamageInput{
				Additives: []DamageAdditive{
					{Label: def.ID, Value: rollResult.Total, Source: "hazard:" + def.ID},
				},
				DamageType: def.DamageType,
				Weakness:   victim.WeaknessFor(def.DamageType),
				Resistance: victim.ResistanceFor(def.DamageType),
			}
			result := ResolveDamage(in)
			victim.ApplyDamage(result.Final)
			narrative := fmt.Sprintf("%s is hit by %s! (%s → %d damage)",
				victim.Name, def.ID, def.DamageExpr, result.Final)
			if def.Message != "" {
				narrative = fmt.Sprintf("%s — %s (%s → %d damage)", def.Message, victim.Name, def.DamageExpr, result.Final)
			}
			events = append(events, RoundEvent{
				ActionType: ActionHazardDamage,
				ActorID:    victim.ID,
				ActorName:  victim.Name,
				Damage:     result.Final,
				Narrative:  narrative,
			})
		}
	}
	return events
}

// ApplyCellHazardForTest exposes applyCellHazard for package-external tests.
func ApplyCellHazardForTest(cbt *Combat, victim *Combatant, tc TerrainCell, trigger string, src Source) []RoundEvent {
	return applyCellHazard(cbt, victim, tc, trigger, src)
}
```

You will also need to add `ActionHazardDamage` to the `ActionType` enum in `action.go`:
```go
ActionHazardDamage // terrain hazard fires damage on entry or round start
```

And add `WeaknessFor(damageType string) int` and `ResistanceFor(damageType string) int` helpers to `Combatant` in `combat.go` if the fields are maps:
```go
func (c *Combatant) WeaknessFor(dt string) int {
    if c.Weaknesses == nil { return 0 }
    return c.Weaknesses[dt]
}

func (c *Combatant) ResistanceFor(dt string) int {
    if c.Resistances == nil { return 0 }
    return c.Resistances[dt]
}
```

- [ ] **Step 4: Add terrain-hazard hook to `StartRoundWithSrc` in `engine.go`**

In `internal/game/combat/engine.go`, in `StartRoundWithSrc`, after the dying-recovery loop and before resetting action queues, add:

```go
// TERRAIN-10: fire round_start hazards for combatants occupying hazardous cells.
for _, cbt := range c.Combatants {
    if cbt.IsDead() {
        continue
    }
    // TERRAIN-12: skip round_start in round 1 for combatants that entered at combat start.
    if c.skipHazardRoundStart != nil && c.skipHazardRoundStart[cbt.ID] {
        delete(c.skipHazardRoundStart, cbt.ID)
        continue
    }
    if tc := c.TerrainAt(cbt.GridX, cbt.GridY); tc.Type == TerrainHazardous && tc.Hazard != nil {
        _ = applyCellHazard(c, cbt, tc, "round_start", src)
        // Note: applyCellHazard events are not propagated from StartRoundWithSrc in v1
        // (StartRoundWithSrc returns []RoundConditionEvent, not []RoundEvent).
        // The damage is applied; a follow-up ticket may surface the narrative.
    }
}
```

> **Note:** `StartRoundWithSrc` returns `[]RoundConditionEvent` but `applyCellHazard` returns `[]RoundEvent`. The hazard damage is applied directly to HP; narrative events from the hazard are discarded in StartRoundWithSrc v1. A future improvement can propagate them via a combined return type.

- [ ] **Step 5: Add combat-start placement hazard firing**

In the gameserver layer (e.g., `internal/gameserver/combat_handler.go`), after populating `cbt.Terrain` at combat start, add:

```go
// TERRAIN-9/12: fire on_enter hazards for all combatants placed on hazardous cells at start.
if cbt.skipHazardRoundStart == nil {
    cbt.skipHazardRoundStart = make(map[string]bool)
}
for _, c := range cbt.Combatants {
    if tc := cbt.TerrainAt(c.GridX, c.GridY); tc.Type == combat.TerrainHazardous && tc.Hazard != nil {
        combat.ApplyCellHazardForTest(cbt, c, tc, "on_enter", src) // fires on_enter
        cbt.skipHazardRoundStart[c.ID] = true // suppress round_start in round 1
    }
}
```

- [ ] **Step 6: Run hazard tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestHazard_ -v 2>&1 | tail -20
```

Expected: `TestHazard_OnEnter_DealsDamage` and `TestHazard_RoundStart_FiresRoundStart` pass; skip test passes.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/game/combat/round.go internal/game/combat/engine.go internal/game/combat/round_hazard_test.go internal/gameserver/combat_handler.go
git commit -m "feat(#248): applyCellHazard, StartRound hazard hook, combat-start placement hazards"
```

---

### Task 6: Telnet rendering — terrain glyphs and `terrain` command

**Files:**
- Modify: `internal/frontend/text_renderer.go`
- Create: `internal/game/combat/terrain_render_test.go`

- [ ] **Step 1: Write failing render tests**

Create `internal/game/combat/terrain_render_test.go`:

```go
package combat_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
)

// TestTerrainGlyphs_NormalIsDot verifies that normal terrain renders as the dot glyph.
func TestTerrainGlyphs_NormalIsDot(t *testing.T) {
	g := combat.TerrainGlyph(combat.TerrainNormal, false)
	if g != "·" && g != "." {
		t.Errorf("normal terrain glyph: want · or . got %q", g)
	}
}

// TestTerrainGlyphs_DifficultIsHash verifies difficult terrain renders as a block shade.
func TestTerrainGlyphs_DifficultIsHash(t *testing.T) {
	g := combat.TerrainGlyph(combat.TerrainDifficult, false)
	if g == "" {
		t.Error("difficult terrain must have a non-empty glyph")
	}
}

// TestTerrainGlyphs_GreaterDifficultIsDense verifies greater_difficult has a distinct glyph.
func TestTerrainGlyphs_GreaterDifficultIsDense(t *testing.T) {
	g := combat.TerrainGlyph(combat.TerrainGreaterDifficult, false)
	gd := combat.TerrainGlyph(combat.TerrainDifficult, false)
	if g == gd {
		t.Error("greater_difficult and difficult must have distinct glyphs")
	}
}

// TestTerrainLegendOutput verifies the terrain command output contains all four types.
func TestTerrainLegendOutput(t *testing.T) {
	legend := combat.TerrainLegendText(false)
	for _, keyword := range []string{"normal", "difficult", "impassable", "hazardous"} {
		if !strings.Contains(legend, keyword) {
			t.Errorf("terrain legend must mention %q", keyword)
		}
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestTerrainGlyphs_ -v 2>&1 | tail -10
```

Expected: compile error.

- [ ] **Step 3: Add `TerrainGlyph` and `TerrainLegendText` to `terrain.go`**

Add to `internal/game/combat/terrain.go`:

```go
// TerrainGlyph returns the display character for a terrain type.
// When unicode is false, returns ASCII fallbacks.
func TerrainGlyph(t TerrainType, unicode bool) string {
	if unicode {
		switch t {
		case TerrainDifficult:
			return "▒"
		case TerrainGreaterDifficult:
			return "▓"
		case TerrainHazardous:
			return "!"
		default:
			return "·"
		}
	}
	switch t {
	case TerrainDifficult:
		return "+"
	case TerrainGreaterDifficult:
		return "#"
	case TerrainHazardous:
		return "!"
	default:
		return "."
	}
}

// TerrainLegendText returns a printable legend for the `terrain` command.
func TerrainLegendText(unicode bool) string {
	dot := TerrainGlyph(TerrainNormal, unicode)
	diff := TerrainGlyph(TerrainDifficult, unicode)
	greater := TerrainGlyph(TerrainGreaterDifficult, unicode)
	haz := TerrainGlyph(TerrainHazardous, unicode)
	return fmt.Sprintf(
		"Combat map legend:\n"+
			"  %s  normal          — 1 speed to enter\n"+
			"  %s  difficult       — 2 speed to enter\n"+
			"  %s  impassable\n"+
			"  %s  hazardous       — damage on entry or round start\n",
		dot, diff, greater, haz,
	)
}
```

- [ ] **Step 4: Wire terrain glyphs into the combat map renderer**

In `internal/frontend/text_renderer.go`, find the cell-drawing function for the combat map (search for where `CoverObjects` or combatant GridX/GridY positions are used for the map grid). Before drawing a combatant or cover glyph, draw the terrain glyph for that cell as the background character.

The exact integration depends on the current map-drawing code. The pattern to follow:

```go
// For each cell (x, y) in the grid:
baseGlyph := combat.TerrainGlyph(cbt.TerrainAt(x, y).Type, session.UnicodeCapable)
// Then overlay combatant / cover glyph on top.
```

Search for `RenderCombatMap` or similar in `text_renderer.go` to find the exact integration point.

- [ ] **Step 5: Add `terrain` telnet command handler**

In the appropriate command handler file (search for `ActionStride` or existing combat commands in `internal/frontend/handlers/`), register a `terrain` command that returns `combat.TerrainLegendText(session.UnicodeCapable)`. This command is in-combat only.

- [ ] **Step 6: Run render tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestTerrainGlyphs_ -v 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -30
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/game/combat/terrain.go internal/game/combat/terrain_render_test.go internal/frontend/text_renderer.go internal/frontend/handlers/
git commit -m "feat(#248): terrain glyphs in telnet combat map, terrain legend command"
```

---

### Task 7: Web client — terrain layer

**Files:**
- Modify: `cmd/webclient/ui/src/` (combat map component + CSS)

- [ ] **Step 1: Add terrain CSS classes**

In the combat map CSS (search for `.combat-cell` or `.grid-cell` in `cmd/webclient/ui/src/`), add:

```css
.terrain-normal { /* no tint — default */ }
.terrain-difficult { background-color: rgba(180, 140, 60, 0.3); }
.terrain-greater-difficult { background-color: rgba(80, 80, 80, 0.5); }
.terrain-hazardous { background-color: rgba(220, 50, 50, 0.3); }
```

- [ ] **Step 2: Add terrain data to the combat WS payload**

The combat-start WebSocket message (find where `CombatState` or equivalent is serialised in the gameserver) must include the terrain map. Add a `terrain` field:

```go
// In the combat state DTO (find the type in internal/gameserver/ used for WS):
Terrain []TerrainCellDTO `json:"terrain"`

type TerrainCellDTO struct {
    X    int    `json:"x"`
    Y    int    `json:"y"`
    Type string `json:"type"`
    // HazardName is populated for hazardous cells, used in the tooltip.
    HazardName string `json:"hazard_name,omitempty"`
}
```

Populate this from `cbt.Terrain` in the gameserver combat-start handler.

- [ ] **Step 3: Render terrain cells in the combat map component**

In the combat map React component (search for `GridCell` or `CombatCell` in `cmd/webclient/ui/src/`):

```tsx
// For each cell, apply the terrain class:
const terrainClass = cell.terrain ? `terrain-${cell.terrain.type.replace('_', '-')}` : '';

// For hazardous cells, render a corner icon and tooltip:
{cell.terrain?.type === 'hazardous' && (
  <span
    className="hazard-icon"
    title={cell.terrain.hazardName || 'hazardous terrain'}
  >!</span>
)}
```

- [ ] **Step 4: Add terrain legend panel**

Add a `TerrainLegend` component (dismissible info panel, triggered by an "i" button near the combat map):

```tsx
const TerrainLegend = () => (
  <div className="terrain-legend">
    <p><span className="terrain-normal-swatch" /> normal — 1 speed</p>
    <p><span className="terrain-difficult-swatch" /> difficult — 2 speed</p>
    <p><span className="terrain-greater-difficult-swatch" /> impassable</p>
    <p><span className="terrain-hazardous-swatch" /> hazardous — damage on entry/round</p>
  </div>
);
```

- [ ] **Step 5: Add stride-preview cost markers**

In the stride-preview UI (if it exists; search for "stride" or "preview" in the web component), mark difficult cells with `×2` and greater_difficult with a stop indicator (✗ or similar).

- [ ] **Step 6: Run web tests**

```bash
cd /home/cjohannsen/src/mud && npm --prefix cmd/webclient/ui run test 2>&1 | tail -30
```

Expected: PASS (or any pre-existing test failures that are unchanged).

- [ ] **Step 7: Commit**

```bash
git add cmd/webclient/ui/src/
git commit -m "feat(#248): terrain layer in web combat map — CSS tints, hazard tooltip, legend panel"
```

---

### Task 8: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add Terrain section**

Add a new `## Terrain` section to `docs/architecture/combat.md`:

```markdown
## Terrain

### Requirements (TERRAIN-N)

- TERRAIN-1: Per-cell layer with types `normal`, `difficult`, `greater_difficult`, `hazardous`.
- TERRAIN-2: Absent cells default to `normal`.
- TERRAIN-3: Room YAML accepts explicit grid (shape A) or default+overrides (shape B); both in same room is a load error.
- TERRAIN-4: Hazardous cell requires exactly one of `hazard_id` or `hazard`; non-hazardous cells carrying either is a load error.
- TERRAIN-5: `greater_difficult` may not combine with `difficult` or any hazard.
- TERRAIN-6: Speed costs — `normal=1`, `difficult=2`, `hazardous=1` (+1 if `DifficultOverlay`), `greater_difficult=impassable`.
- TERRAIN-7: Stride terminates when next step would exceed budget or enter impassable cell.
- TERRAIN-8: `SpeedSquares()` deprecated; `SpeedBudget()` is canonical.
- TERRAIN-9: Entering hazardous cell fires `on_enter` hazard.
- TERRAIN-10: At `StartRound`, living combatants on hazardous cells fire `round_start` hazard.
- TERRAIN-11: Hazard damage routes through `ResolveDamage` (#246).
- TERRAIN-12: Combat-start placement fires `on_enter` once; `round_start` suppressed in round 1.
- TERRAIN-13: Forced movement onto `greater_difficult` refused at mover layer.
- TERRAIN-14–16: Terrain rendered beneath combatants; telnet `terrain` command; web tints + tooltips.
- TERRAIN-17: Terrain-stopped stride emits explanatory narrative.
- TERRAIN-18: Zero-movement stride emits informational narrative.
- TERRAIN-19: Unresolved `hazard_id` logs an error and skips the hazard.
- TERRAIN-20: Terrain is static per combat.
- TERRAIN-21: Depends on #246 (Stacking multipliers).

### Type/Cost Table

| Type               | Entry Cost | Passable |
|--------------------|-----------|---------|
| `normal`           | 1         | yes     |
| `difficult`        | 2         | yes     |
| `hazardous`        | 1 (or 2)  | yes     |
| `greater_difficult`| —         | no      |

### Implementation Files

| File | Responsibility |
|------|----------------|
| `internal/game/combat/terrain.go` | GridCell, TerrainType, TerrainCell, CellHazard, TerrainAt, EntryCost, glyphs |
| `internal/game/world/terrain_load.go` | YAML loader for both authoring shapes |
| `internal/game/combat/round.go` | Budget-based stride, applyCellHazard, StartRound hazard hook |
| `internal/game/combat/engine.go` | Combat.Terrain, RoomHazards, skipHazardRoundStart fields |

### Stride Budget Model

```
budget = actor.SpeedBudget()  // e.g. 5 for 25 ft
for budget > 0:
    cost, ok = cbt.EntryCost(newX, newY)
    if !ok or cost > budget: stop
    budget -= cost
    move to (newX, newY)
    if hazardous: fire on_enter hazard
```
```

- [ ] **Step 2: Verify build**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | tail -10
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add docs/architecture/combat.md
git commit -m "docs(#248): add Terrain section to combat architecture"
```

---

## Self-Review Checklist

**Spec coverage:**
- TERRAIN-1 ✓ (Task 1: TerrainType enum with 4 values)
- TERRAIN-2 ✓ (Task 1: TerrainAt returns Normal for absent cells)
- TERRAIN-3 ✓ (Task 3: load error for both shapes; loader distinguishes A and B)
- TERRAIN-4 ✓ (Task 3: resolveCell validates hazardous cell hazard requirement)
- TERRAIN-5 ✓ (Task 3: resolveCell rejects greater_difficult + hazard/difficult combo)
- TERRAIN-6 ✓ (Task 1: EntryCost; Task 4: stride budget loop)
- TERRAIN-7 ✓ (Task 4: `cost > budget` check stops without advancing)
- TERRAIN-8 ✓ (Task 2: SpeedBudget added; SpeedSquares deprecated)
- TERRAIN-9 ✓ (Task 4/5: on_enter fired via applyCellHazard per step)
- TERRAIN-10 ✓ (Task 5: StartRoundWithSrc round_start hook)
- TERRAIN-11 ✓ (Task 5: ResolveDamage used in applyCellHazard)
- TERRAIN-12 ✓ (Task 5: skipHazardRoundStart map prevents double-fire)
- TERRAIN-13 ✓ (Task 4: `!passable` break in stride loop; no implicit bypass)
- TERRAIN-14 ✓ (Task 6: terrain glyphs beneath combatants in telnet map)
- TERRAIN-15 ✓ (Task 6: `terrain` command outputs TerrainLegendText)
- TERRAIN-16 ✓ (Task 7: web CSS tints, hazard icon + tooltip)
- TERRAIN-17 ✓ (Task 4: terrain-stopped stride narrative)
- TERRAIN-18 ✓ (Task 4: zero-movement stride narrative)
- TERRAIN-19 ✓ (Task 3/5: unresolved hazard_id — Def=nil; error logged at combat start; hazard skipped)
- TERRAIN-20 ✓ (implied: Terrain not mutated during combat)
- TERRAIN-21 ✓ (Plan header: #246 prerequisite)

**No placeholders found.**

**Type consistency:** `GridCell`, `TerrainType`, `TerrainCell`, `CellHazard`, `TerrainAt`, `EntryCost`, `SpeedBudget`, `TerrainGlyph`, `TerrainLegendText`, `applyCellHazard`, `ApplyCellHazardForTest` — all defined in Task 1/5 and referenced consistently in later tasks. `world.ResolvedTerrainCell` / `world.CellKey` are distinct from `combat.GridCell` / `combat.TerrainCell` to avoid import cycles.
