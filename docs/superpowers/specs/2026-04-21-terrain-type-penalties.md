# Terrain Type Penalties in Combat

**Issue:** [#248 Feature: Terrain type penalties in combat](https://github.com/cory-johannsen/mud/issues/248)
**Status:** Spec
**Date:** 2026-04-21
**Depends on:** #246 Stacking multipliers (merges first)

## 1. Summary

Add a per-cell terrain layer to the combat grid. Four terrain types in v1: `normal`, `difficult`, `greater_difficult`, and `hazardous`. `ActionStride` consumes a speed budget per step at a per-cell entry cost; `greater_difficult` is impassable. Hazardous cells carry a hazard descriptor (referenced from the room's existing `HazardDef` list or declared inline) and fire damage / conditions on entry or at round start via the #246 `ResolveDamage` pipeline. Terrain is authored in room YAML (two equivalent shapes accepted), rendered beneath combatants on both telnet and web maps, and static for the duration of a combat.

Dynamic terrain, zone-level defaults, region authoring, polygon shapes, and authoring-tool UI are all explicit non-goals for v1.

## 2. Requirements

- TERRAIN-1: The combat grid MUST support a per-cell terrain layer with types `normal`, `difficult`, `greater_difficult`, and `hazardous`.
- TERRAIN-2: Cells not explicitly declared MUST default to `normal`.
- TERRAIN-3: Room YAML MUST accept either an explicit grid (`combat_terrain.grid` with a `legend`) or a default-plus-overrides form (`combat_terrain.default` + `combat_terrain.cells`); declaring both in the same room MUST be a load error.
- TERRAIN-4: A cell with `type: hazardous` MUST carry exactly one of `hazard_id` (reference) or `hazard` (inline); a non-hazardous cell carrying either MUST be a load error.
- TERRAIN-5: `type: greater_difficult` MUST NOT combine with `difficult` or with any hazard; such combinations MUST be load errors.
- TERRAIN-6: `ActionStride` MUST consume a speed budget per step, with per-cell cost: `normal = 1`, `difficult = 2`, `hazardous = 1` (`+1` if `difficult` overlay), `greater_difficult = impassable`.
- TERRAIN-7: Stride MUST terminate when the next step would exceed the remaining speed budget or enter an impassable cell, without advancing into that cell.
- TERRAIN-8: `SpeedSquares()` MUST remain as a deprecated wrapper that delegates to `SpeedBudget()`; in-package call sites MUST migrate to `SpeedBudget()`.
- TERRAIN-9: Entering a `hazardous` cell MUST fire any `on_enter` hazard on that cell for the entering combatant.
- TERRAIN-10: At `Combat.StartRound`, each living combatant occupying a `hazardous` cell MUST have any `round_start` hazard on that cell fired.
- TERRAIN-11: Hazard damage MUST flow through the #246 `ResolveDamage` pipeline.
- TERRAIN-12: A combatant placed on a `hazardous` cell at combat start MUST fire the cell's `on_enter` hazard once; `round_start` hazards MUST NOT double-fire on the same round.
- TERRAIN-13: Forced movement (future features) onto a `greater_difficult` cell MUST be refused at the mover layer; no implicit bypass.
- TERRAIN-14: The combat map renderer MUST draw terrain beneath combatants and cover.
- TERRAIN-15: The telnet renderer MUST provide a `terrain` command that prints the legend.
- TERRAIN-16: The web client MUST tint hazardous cells with an icon and tooltip describing the hazard.
- TERRAIN-17: A stride that ends early due to terrain MUST emit a narrative line explaining the stop (e.g. "stops at the rubble").
- TERRAIN-18: A stride that cannot begin because the first step is unaffordable or impassable MUST emit an informational narrative line.
- TERRAIN-19: A hazard inline definition MUST satisfy `world.HazardDef.Validate`; a referenced `hazard_id` MUST resolve against the active room's `Hazards` list at combat start; unresolved ids MUST emit an error log and skip the hazard.
- TERRAIN-20: Terrain MUST be static for the duration of a combat; dynamic terrain mutation is out of scope for v1.
- TERRAIN-21: Implementation of #248 MUST NOT start before #246 (Stacking multipliers) is merged.

## 3. Architecture

### 3.1 Package layout

- `internal/game/combat/terrain.go` (new) — `TerrainType`, `TerrainCell`, `CellHazard`; `Combat.Terrain` map; `TerrainAt`; `EntryCost`.
- `internal/game/combat/round.go` — `ActionStride` rewritten to use the speed budget; `StartRound` gains a terrain-hazard walk; `applyCellHazard` helper.
- `internal/game/world/model.go` — `Room` gains `CombatTerrain *CombatTerrainConfig`; the loader understands both authoring shapes.
- `internal/game/world/terrain_load.go` (new) — parses `combat_terrain:` YAML block; normalises to a single in-memory representation.
- Telnet renderer and web client get a terrain layer drawn beneath combatants.

### 3.2 Lifecycle

1. Room YAML load: `combat_terrain:` block parses into a canonical representation on the `Room`.
2. Combat start: the engine reads the room's terrain config and populates `Combat.Terrain` sparsely. Combatants placed on hazardous cells trigger `on_enter` hazards once at combat start.
3. During combat: `ActionStride` consumes speed budget per cell; hazards fire on entry and at round start.
4. Combat end: terrain state is discarded (non-persistent).

### 3.3 Dependencies

- **#246 (Stacking multipliers).** Hazard damage routes through `ResolveDamage` from #246. Implementation gated on #246 merging first.
- **#245 (Typed bonuses).** Not directly required. Terrain grants no typed bonuses in v1; `difficult` is a movement-cost feature, not an AC/skill bonus.

### 3.4 No unification with room-wide `Hazards`

Room-wide `HazardDef` entries in `Room.Hazards` keep their current on-enter / round-start semantics for zone exploration and non-grid hazards. Cell hazards are a distinct construct; they may *reference* a room-level hazard by id for authoring convenience, but the two systems do not merge.

## 4. Data Model

### 4.1 Enum

```go
type TerrainType string

const (
    TerrainNormal           TerrainType = "normal"
    TerrainDifficult        TerrainType = "difficult"
    TerrainGreaterDifficult TerrainType = "greater_difficult"
    TerrainHazardous        TerrainType = "hazardous"
)
```

### 4.2 `TerrainCell`

```go
type TerrainCell struct {
    X, Y             int
    Type             TerrainType
    // DifficultOverlay, when true on a TerrainHazardous cell, makes the cell
    // also cost 2 to enter. Ignored on other types.
    DifficultOverlay bool
    // Hazard is populated iff Type == TerrainHazardous.
    Hazard *CellHazard
}

type CellHazard struct {
    // Exactly one of these is set.
    HazardID string            // reference to room.Hazards[].ID
    Inline   *world.HazardDef  // inline per-cell declaration
}
```

Entry cost:

- `TerrainNormal` → 1
- `TerrainDifficult` → 2
- `TerrainHazardous` → 1 (or 2 if `DifficultOverlay`)
- `TerrainGreaterDifficult` → impassable

### 4.3 Combat-level storage

```go
type Combat struct {
    // ... existing fields ...
    Terrain map[GridCell]*TerrainCell // sparse; cells absent -> normal
}

// TerrainAt returns the cell at (x, y), or a zero-value TerrainNormal if absent.
func (c *Combat) TerrainAt(x, y int) TerrainCell

// EntryCost returns the movement cost to enter (x, y); ok=false if impassable.
func (c *Combat) EntryCost(x, y int) (cost int, ok bool)
```

`GridCell{X, Y int}` is the shared grid type introduced in #247.

### 4.4 Authoring shape A — explicit grid

```yaml
combat_terrain:
  grid:
    - "....................."
    - "....####............."
    - "....####...H........."
    - "....................."
  legend:
    ".": normal
    "#": difficult
    "X": greater_difficult
    "H": { type: hazardous, hazard_id: fire_vent }
    "h": { type: hazardous, difficult: true, hazard_id: flaming_rubble }
```

- Rows are indexed 0..height-1; columns 0..width-1.
- Legend maps single-character codes to terrain descriptors.
- A descriptor is either a bare type string or an object with `type`, optional `difficult`, optional `hazard_id` (reference), or optional `hazard` (inline).
- Default glyphs `.`, `#`, `X`, `H` are assumed if the legend omits them. Legend entries override defaults.
- Grid dimensions MUST match the combat grid size; mismatch is a load error.

### 4.5 Authoring shape B — default + overrides

```yaml
combat_terrain:
  default: normal
  cells:
    - { x: 4, y: 1, type: difficult }
    - { x: 5, y: 1, type: difficult }
    - { x: 4, y: 2, type: greater_difficult }
    - { x: 11, y: 2, type: hazardous, hazard_id: fire_vent }
    - { x: 12, y: 2, type: hazardous, difficult: true, hazard:
        id: flaming_rubble
        trigger: on_enter
        damage_expr: "1d6"
        damage_type: fire
        message: "flames lick at you"
      }
```

- `default` is optional; defaults to `normal` when absent.
- Each `cells[]` entry: `x`, `y`, `type`, optional `difficult`, optional `hazard_id` (reference), optional `hazard` (inline). Exactly one of `hazard_id` / `hazard` for hazardous cells; neither permitted for non-hazardous.
- Duplicate `(x, y)` entries are a load error.

### 4.6 Loader normalisation

The loader accepts either shape and produces a `map[GridCell]*TerrainCell`. Mixed shapes in one file, mismatched grid dimensions, duplicate cells, greater_difficult combined with hazard or difficult, and hazardous cells without a hazard are all load errors with clear messages.

A `CellHazard.HazardID` is resolved against `Room.Hazards` at combat start. Unknown ids emit a one-off error log and skip the hazard (the cell remains a visual hazardous tile but never fires).

### 4.7 Persistence

No database schema change. Terrain is derived from the room's authored YAML each time a combat starts.

## 5. Resolution

### 5.1 `SpeedBudget`

```go
// SpeedBudget returns this combatant's stride movement budget in speed points.
// Default is 5. Stride consumes budget by per-cell EntryCost.
func (c *Combatant) SpeedBudget() int

// SpeedSquares is retained as a deprecated alias over SpeedBudget for external
// callers. In-package callers migrate to SpeedBudget.
func (c *Combatant) SpeedSquares() int // deprecated
```

### 5.2 `ActionStride` rewrite

```go
budget := actor.SpeedBudget()
for budget > 0 {
    dx, dy := CompassDelta(dir, actor, opponent)
    if dx == 0 && dy == 0 {
        break
    }
    newX := clamp(actor.GridX + dx, 0, width-1)
    newY := clamp(actor.GridY + dy, 0, height-1)
    if newX == actor.GridX && newY == actor.GridY {
        break // grid edge
    }
    if CellBlocked(cbt, actor.ID, newX, newY) {
        break // occupant / cover
    }
    cost, passable := cbt.EntryCost(newX, newY)
    if !passable || cost > budget {
        break // greater_difficult or insufficient budget
    }
    budget -= cost
    actor.GridX = newX
    actor.GridY = newY
    if tc := cbt.TerrainAt(newX, newY); tc.Type == TerrainHazardous && tc.Hazard != nil {
        applyCellHazard(cbt, actor, tc, "on_enter", src)
    }
    // existing reaction/adjacency hooks
}
```

- `REQ-STRIDE-STOP` adjacency stop is preserved (check at loop top).
- `REQ-STRIDE-NOOVERLAP` is preserved via `CellBlocked`.
- A stride that terminates on terrain emits the appropriate narrative (see §6).

### 5.3 Hazard application

```go
func applyCellHazard(cbt *Combat, victim *Combatant, tc TerrainCell, trigger string, src Source) {
    def := resolveHazardDef(cbt, tc.Hazard)
    if def == nil || def.Trigger != trigger {
        return
    }
    if def.DamageExpr != "" {
        base := rollDamageExpr(def.DamageExpr, src)
        in := DamageInput{
            Additives:  []DamageAdditive{{Label: def.ID, Value: base, Source: "hazard:" + def.ID}},
            DamageType: def.DamageType,
            Weakness:   victim.Weaknesses[def.DamageType],
            Resistance: victim.Resistances[def.DamageType],
        }
        result := ResolveDamage(in)
        victim.ApplyDamage(result.Final)
        emit RoundEvent{Narrative: renderHazardNarrative(def, victim, result), ...}
    }
    if def.ConditionID != "" && condReg.Has(def.ConditionID) {
        cbt.Conditions[victim.ID].Apply(victim.ID, condReg.Get(def.ConditionID), 1, -1)
    }
}
```

`resolveHazardDef` returns `tc.Hazard.Inline` when non-nil, otherwise looks up `tc.Hazard.HazardID` in the active room's `Hazards`. Unknown ids skip.

### 5.4 Round-start hazard hook

In `Combat.StartRound`, after condition ticks and before resetting action queues:

```go
for _, c := range cbt.Combatants {
    if c.IsDead() {
        continue
    }
    if tc := cbt.TerrainAt(c.GridX, c.GridY); tc.Type == TerrainHazardous && tc.Hazard != nil {
        applyCellHazard(cbt, c, tc, "round_start", src)
    }
}
```

### 5.5 Combat-start placement

When a combatant is added to combat on a hazardous cell, fire the `on_enter` hazard once at combat start. The `round_start` hook in the first round does NOT re-fire hazards for combatants that just entered (tracked via an ephemeral "entered this round" flag cleared at first `round_start` tick).

### 5.6 `CellBlocked` interaction

`CellBlocked` continues to check occupancy and cover. Terrain blocking is handled in the stride loop via `EntryCost`'s impassable return value. Keeping concerns separate leaves `CellBlocked` reusable for placement, targeting, and other features.

### 5.7 Reaction interactions

- `TriggerOnEnemyMoveAdjacent` continues to fire per step regardless of terrain cost.
- Hazardous-cell entry that deals damage fires `TriggerOnDamageTaken` via the existing `ApplyDamage` path — reactions like Shield Block can trigger on a fire-vent step.
- `TriggerOnEnemyEntersRoom` is room-scoped; terrain is cell-scoped; they do not interact.

### 5.8 Edge cases

- Zero-budget or impassable first step: stride is a no-op; informational narrative emitted (TERRAIN-18).
- Forced movement onto greater_difficult: refused at the mover layer (TERRAIN-13).
- Dying combatant at round start: does not fire further hazards; fires the on-damage reactions that brought them down normally.

## 6. Rendering

### 6.1 Telnet — glyphs

| Type                  | Glyph  | Style                         |
|-----------------------|--------|-------------------------------|
| normal                | `·`    | dim gray                      |
| difficult             | `▒`    | yellow-brown                  |
| greater_difficult     | `▓`    | dark gray                     |
| hazardous             | `!`    | red                           |
| hazardous + difficult | `!`    | red on yellow-brown           |

ASCII fallbacks for non-Unicode capability: `.`, `+`, `#`, `!`. Capability detected via the existing per-session render-capability flag. Terrain draws beneath combatant / cover glyphs.

### 6.2 Telnet — `terrain` command

```
Combat map legend:
  ·  normal          — 1 speed to enter
  ▒  difficult       — 2 speed to enter
  ▓  impassable
  !  hazardous       — damage on entry or round start
```

Static text, not per-map. Per-hazard details appear in the narrative when hazards fire, not in the legend.

### 6.3 Telnet — stride narrative

- Partial stride: `Kira strides 3 squares through the rubble and stops.`
- Refused first step: `Kira tries to move but the wall of debris blocks the way.`
- Hazard fire: `Kira steps into the fire vent! (1d6 fire → 4 damage)` — hazard damage reuses the #246 breakdown format when non-trivial.

### 6.4 Web — tinted cells

Per-type CSS tint classes on cells: `terrain-normal`, `terrain-difficult`, `terrain-greater-difficult`, `terrain-hazardous`. Hazardous cells carry a corner icon and a hover tooltip naming the hazard and its effect. Combatants / cover render above terrain.

Terrain layer ships in the existing combat-start WS payload — no new message type; terrain is static for the combat's duration.

### 6.5 Web — legend panel

Info button opens a dismissible panel with the legend (same text as §6.2).

### 6.6 Web — stride preview

The existing stride preview UI shows the path cell-by-cell with cumulative speed cost. Difficult cells are marked `×2`; greater_difficult cells show a stop indicator.

### 6.7 Rendering non-goals

- No animated hazards.
- No per-hazard custom glyphs (all hazardous cells use `!`).
- No per-session color customization beyond existing palette capabilities.
- No new zoom / scroll functionality.

## 7. Testing

Per SWENG-5 / SWENG-5a, TDD with property-based tests where appropriate.

- `internal/game/world/terrain_load_test.go` — load validation (both shapes; all error cases enumerated in §4).
- `internal/game/combat/terrain_test.go` — `TerrainAt` / `EntryCost` correctness for each type and overlay.
- `internal/game/combat/round_stride_terrain_test.go` — property + scripted:
  - All-normal stride moves exactly `SpeedBudget()` cells.
  - Half-difficult stride moves half as far.
  - Greater_difficult blocks at cell before.
  - Zero-movement stride emits informational narrative.
  - Hazardous cells fire `on_enter` per step.
- `internal/game/combat/round_hazard_test.go` — scripted:
  - `round_start` hazard fires per occupant.
  - Hazard damage routes through `ResolveDamage` (weakness/resistance applied; breakdown populated).
  - Hazard condition lands in `ActiveSet`.
  - Dying combatant does not fire further hazards.
  - Combat-start placement fires `on_enter` once; `round_start` does not double-fire in round 1.
- `internal/game/combat/terrain_render_test.go` — telnet golden: terrain glyphs emitted beneath combatant glyphs; legend command output matches.
- Web: component tests for per-cell tinting, hazardous icon presence, tooltip content, stride-preview cost markers.

## 8. Documentation

- `docs/architecture/combat.md` — new "Terrain" subsection hosting `TERRAIN-N` requirements, the type/cost table, and a worked authoring example for each shape.
- `docs/architecture/combat.md` — stride description updated to reference `SpeedBudget`.
- Content-authoring documentation — both `combat_terrain:` shapes with examples; hazard referencing vs inline; enumerated load-error cases.

## 9. Non-Goals (v1)

- No dynamic terrain.
- No terrain mutation by abilities.
- No custom terrain glyphs / icons per hazard beyond the single `!` marker.
- No zone-level default terrain propagating across rooms.
- No region / polygon authoring shape.
- No per-cell movement cost beyond the 1 / 2 / ∞ ladder.
- No directional terrain.
- No terrain-granted cover, concealment, or AC bonuses (cover objects from #247 remain the only AC surface).
- No authoring tool UI.
- No climbing / swimming actions that reveal greater_difficult as "climbable with effort."

## 10. Open Questions for the Planner

- Exact location of `TerrainCell` / `CellHazard` types — single-file `terrain.go` in `combat/`, or split.
- Whether the web stride-preview update is bundled into this spec's implementation or split to a follow-up ticket.
- Telnet glyph choice when the terminal does not support Unicode block shades — spec proposes ASCII fallbacks; planner confirms against the existing capability checks.
- Whether the `terrain` command is in-combat only or also in exploration mode (where the concept doesn't apply).
