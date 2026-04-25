# Visibility / Line-of-Sight System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Light up the no-op extension points reserved by #249 (TGT-21), #250 (AOE-21), and #254 (DETECT-NG1) with a real LoS implementation. Add a per-cell opacity layer on `Combat`, a `WallObject` first-class entity, an `occludes`/`partial` flag on `CoverObject`, environmental occluders (smoke / fog / darkness) with durations, a pure `combat.LineOfSight` Bresenham walker returning `Clear` / `Concealed` / `Blocked`, and the wired-up validation + UI surfacing on telnet and web.

**Spec:** [docs/superpowers/specs/2026-04-25-visibility-line-of-sight.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-visibility-line-of-sight.md) (PR [#294](https://github.com/cory-johannsen/mud/pull/294))

**Architecture:** Three layers. (1) Data — `Combat.OpacityLayer` (per-cell opacity), `Combat.Environment []*EnvironmentalEffect` (active occluder effects with durations), populated at `StartCombat` from the room's walls + cover objects' `occludes`/`partial` flags + initial environment. (2) Algorithm — `combat.LineOfSight(layer, from, to) Result` walks Bresenham cells exclusive of endpoints; opaque → `Blocked`, translucent → `Concealed`, else `Clear`; adjacent and self short-circuit to `Clear` per LOS-7/8. Pure function, property-tested for symmetry and opacity-monotonicity. (3) Integration — three consumers all call the single function: targeting validation (`ValidateSingleTarget` / `ValidateAoE` line-of-fire stages), AoE `PostFilterAffectedCells` post-filter, and detection-state `OcclusionProvider`. UI surfacing: telnet target lists omit blocked targets and annotate concealed; web grays out blocked + adds tooltip "no line of sight"; AoE preview dims post-filtered cells.

**Tech Stack:** Go (`internal/game/combat/`, `internal/game/world/`, `internal/game/detection/`, `internal/gameserver/`), `pgregory.net/rapid` for property tests, telnet, React/TypeScript (`cmd/webclient/ui/src/game/panels/`).

**Prerequisite:** #249 (Targeting) MUST merge first — TGT-21's no-op stage is what this plan replaces. #254 (Detection states) is a soft dep — when the new `OcclusionProvider` returns `Blocked` / `Concealed`, the detection layer's transition rules consume the result. #250 (AoE) is a soft dep — the `PostFilterAffectedCells` hook is the integration point.

**Note on spec PR**: Spec is on PR #294, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/combat/opacity.go` (`OpacityLayer`, `Opacity` enum) |
| Create | `internal/game/combat/opacity_test.go` |
| Create | `internal/game/combat/los.go` (`LineOfSight` Bresenham walker) |
| Create | `internal/game/combat/los_test.go` |
| Create | `internal/game/combat/testdata/rapid/TestLineOfSight_Property/` |
| Create | `internal/game/combat/environment.go` (`EnvironmentalEffect` runtime + tick) |
| Create | `internal/game/combat/environment_test.go` |
| Modify | `internal/game/combat/combat.go` (`Combat.OpacityLayer`, `Combat.Environment`) |
| Modify | `internal/game/combat/engine.go` (`StartCombat` populates layer; round-tick decrements env effects) |
| Modify | `internal/game/world/model.go` (`WallObject` schema; `CoverObject.Occludes`, `Partial`) |
| Modify | `internal/game/world/model_test.go` |
| Modify | `internal/game/detection/occlusion.go` (real impl replacing the v1 no-op) |
| Modify | `internal/game/detection/occlusion_test.go` |
| Modify | `internal/game/skillaction/target.go` (line-of-fire stage from #252 if applicable) |
| Modify | `internal/game/combat/targeting.go` (TGT-21 LoF stage from #249) |
| Modify | `internal/gameserver/aoe_resolver.go` (`PostFilterAffectedCells` impl from #250) |
| Modify | `internal/gameserver/grpc_service.go` (`look <direction>` command per LOS-19) |
| Modify | `cmd/webclient/ui/src/combat/TargetPanel.tsx` (gray-out blocked / concealed annotation) |
| Modify | `cmd/webclient/ui/src/combat/TargetPanel.test.tsx` |
| Modify | `cmd/webclient/ui/src/combat/MapPanel.tsx` (AoE preview dimming) |
| Create | `content/environment/smoke_grenade.yaml`, `fog_bank.yaml`, `darkness_zone.yaml` |
| Modify | `internal/frontend/handlers/look_command.go` (or create) — `look <direction>` |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: `OpacityLayer` + `Opacity` enum

**Files:**
- Create: `internal/game/combat/opacity.go`
- Create: `internal/game/combat/opacity_test.go`

- [ ] **Step 1: Failing tests** (LOS-1):

```go
func TestOpacityLayer_DefaultsTransparent(t *testing.T) {
    l := combat.NewOpacityLayer(20, 20)
    require.Equal(t, combat.OpacityTransparent, l.At(5, 5))
}

func TestOpacityLayer_SetGet(t *testing.T) {
    l := combat.NewOpacityLayer(20, 20)
    l.Set(3, 3, combat.OpacityOpaque)
    require.Equal(t, combat.OpacityOpaque, l.At(3, 3))
    require.Equal(t, combat.OpacityTransparent, l.At(3, 4))
}

func TestOpacityLayer_OutOfBoundsTransparent(t *testing.T) {
    l := combat.NewOpacityLayer(10, 10)
    require.Equal(t, combat.OpacityTransparent, l.At(-1, 5))
    require.Equal(t, combat.OpacityTransparent, l.At(15, 5))
}

func TestOpacityLayer_MaxOpacityCompose(t *testing.T) {
    // LOS-Q6: overlapping effects take the most opaque.
    require.Equal(t, combat.OpacityOpaque,
        combat.MaxOpacity(combat.OpacityOpaque, combat.OpacityTranslucent))
    require.Equal(t, combat.OpacityTranslucent,
        combat.MaxOpacity(combat.OpacityTranslucent, combat.OpacityTransparent))
}
```

- [ ] **Step 2: Implement**:

```go
type Opacity int

const (
    OpacityTransparent Opacity = iota
    OpacityTranslucent
    OpacityOpaque
)

type OpacityLayer struct {
    width, height int
    cells         []Opacity // row-major
}

func NewOpacityLayer(w, h int) *OpacityLayer {
    return &OpacityLayer{width: w, height: h, cells: make([]Opacity, w*h)}
}

func (l *OpacityLayer) At(x, y int) Opacity {
    if x < 0 || y < 0 || x >= l.width || y >= l.height {
        return OpacityTransparent
    }
    return l.cells[y*l.width+x]
}

func (l *OpacityLayer) Set(x, y int, op Opacity) {
    if x < 0 || y < 0 || x >= l.width || y >= l.height {
        return
    }
    l.cells[y*l.width+x] = op
}

func MaxOpacity(a, b Opacity) Opacity {
    if a > b { return a }
    return b
}
```

---

### Task 2: `LineOfSight` Bresenham walker + property tests

**Files:**
- Create: `internal/game/combat/los.go`
- Create: `internal/game/combat/los_test.go`
- Create: `internal/game/combat/testdata/rapid/TestLineOfSight_Property/`

- [ ] **Step 1: Failing tests** (LOS-5..8, LOS-25):

```go
func TestLineOfSight_SelfIsClear(t *testing.T) {
    l := combat.NewOpacityLayer(20, 20)
    l.Set(5, 5, combat.OpacityOpaque)
    require.Equal(t, combat.LoSClear, combat.LineOfSight(l, combat.Cell{X: 5, Y: 5}, combat.Cell{X: 5, Y: 5}))
}

func TestLineOfSight_AdjacentClearEvenThroughOpaque(t *testing.T) {
    // LOS-8: adjacent always clear.
    l := combat.NewOpacityLayer(20, 20)
    l.Set(5, 5, combat.OpacityOpaque)
    require.Equal(t, combat.LoSClear, combat.LineOfSight(l, combat.Cell{X: 4, Y: 5}, combat.Cell{X: 5, Y: 5}))
}

func TestLineOfSight_OpaqueOnPathBlocks(t *testing.T) {
    l := combat.NewOpacityLayer(20, 20)
    l.Set(5, 5, combat.OpacityOpaque) // between (3,5) and (7,5)
    require.Equal(t, combat.LoSBlocked, combat.LineOfSight(l, combat.Cell{X: 3, Y: 5}, combat.Cell{X: 7, Y: 5}))
}

func TestLineOfSight_TranslucentOnPathConceals(t *testing.T) {
    l := combat.NewOpacityLayer(20, 20)
    l.Set(5, 5, combat.OpacityTranslucent)
    require.Equal(t, combat.LoSConcealed, combat.LineOfSight(l, combat.Cell{X: 3, Y: 5}, combat.Cell{X: 7, Y: 5}))
}

func TestLineOfSight_OpaqueBeatsTranslucent(t *testing.T) {
    l := combat.NewOpacityLayer(20, 20)
    l.Set(4, 5, combat.OpacityTranslucent)
    l.Set(6, 5, combat.OpacityOpaque)
    require.Equal(t, combat.LoSBlocked, combat.LineOfSight(l, combat.Cell{X: 3, Y: 5}, combat.Cell{X: 7, Y: 5}))
}

func TestLineOfSight_DiagonalPath(t *testing.T) {
    l := combat.NewOpacityLayer(20, 20)
    l.Set(5, 5, combat.OpacityOpaque)
    require.Equal(t, combat.LoSBlocked, combat.LineOfSight(l, combat.Cell{X: 3, Y: 3}, combat.Cell{X: 7, Y: 7}))
}
```

- [ ] **Step 2: Implement** the Bresenham walker:

```go
type LoSResult int

const (
    LoSClear LoSResult = iota
    LoSConcealed
    LoSBlocked
)

func LineOfSight(layer *OpacityLayer, from, to Cell) LoSResult {
    if from == to {
        return LoSClear
    }
    dx, dy := abs(to.X-from.X), abs(to.Y-from.Y)
    if max(dx, dy) <= 1 {
        return LoSClear // adjacent
    }
    cells := bresenhamCells(from, to) // exclusive endpoints
    worst := LoSClear
    for _, c := range cells {
        switch layer.At(c.X, c.Y) {
        case OpacityOpaque:
            return LoSBlocked
        case OpacityTranslucent:
            if worst == LoSClear {
                worst = LoSConcealed
            }
        }
    }
    return worst
}
```

- [ ] **Step 3: Property tests** (LOS-6):

```go
func TestProperty_LoS_Symmetry(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        l := arbitraryLayer(t, 10, 10)
        a := arbitraryCell(t, 10, 10)
        b := arbitraryCell(t, 10, 10)
        require.Equal(t,
            combat.LineOfSight(l, a, b),
            combat.LineOfSight(l, b, a),
            "LoS must be symmetric (LOS-6)")
    })
}

func TestProperty_LoS_OpacityMonotonic(t *testing.T) {
    // Adding an opaque cell never makes the result Clear-er.
    rapid.Check(t, func(t *rapid.T) {
        l := arbitraryLayer(t, 10, 10)
        a, b := arbitraryNonAdjacentCells(t, 10, 10)
        before := combat.LineOfSight(l, a, b)
        addCell := arbitraryCell(t, 10, 10)
        l.Set(addCell.X, addCell.Y, combat.OpacityOpaque)
        after := combat.LineOfSight(l, a, b)
        require.GreaterOrEqual(t, after, before, "monotonic in opacity")
    })
}

func TestProperty_LoS_Idempotent(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        l := arbitraryLayer(t, 10, 10)
        a, b := arbitraryNonAdjacentCells(t, 10, 10)
        once := combat.LineOfSight(l, a, b)
        twice := combat.LineOfSight(l, a, b)
        require.Equal(t, once, twice)
    })
}
```

---

### Task 3: World schema — `WallObject` + `CoverObject` flags

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/model_test.go`

- [ ] **Step 1: Failing tests** (LOS-2, LOS-3):

```go
func TestLoadRoom_WallObjectsParse(t *testing.T) {
    r, _ := world.LoadRoom([]byte(`
id: warehouse
walls:
  - id: north_wall
    cells: [{x: 0, y: 5}, {x: 1, y: 5}, {x: 2, y: 5}]
    destructible: false
`))
    require.Len(t, r.Walls, 1)
    require.Equal(t, "north_wall", r.Walls[0].ID)
    require.Equal(t, 3, len(r.Walls[0].Cells))
    require.False(t, r.Walls[0].Destructible)
}

func TestLoadRoom_DestructibleWallRequiresHP(t *testing.T) {
    _, err := world.LoadRoom([]byte(`
id: warehouse
walls:
  - id: north_wall
    cells: [{x: 0, y: 5}]
    destructible: true
`))
    require.Error(t, err)
    require.Contains(t, err.Error(), "destructible: true requires hp")
}

func TestLoadRoom_CoverObjectOccludesPartial(t *testing.T) {
    r, _ := world.LoadRoom([]byte(`
id: alley
cover_objects:
  - id: smoke_screen
    cells: [{x: 5, y: 5}]
    tier: lesser
    occludes: true
    partial: true
`))
    require.True(t, r.CoverObjects[0].Occludes)
    require.True(t, r.CoverObjects[0].Partial)
}

func TestLoadRoom_LegacyCoverObjectsDefaultNoOcclude(t *testing.T) {
    r, _ := world.LoadRoom(legacyCoverYAML)
    require.False(t, r.CoverObjects[0].Occludes, "back-compat: existing cover stays Transparent")
}
```

- [ ] **Step 2: Implement**:

```go
type WallObject struct {
    ID           string
    Cells        []Cell
    Destructible bool
    HP           int
}

func (w *WallObject) Validate() error {
    if w.Destructible && w.HP <= 0 {
        return fmt.Errorf("wall %s: destructible: true requires hp > 0", w.ID)
    }
    return nil
}

type CoverObject struct {
    // ... existing ...
    Occludes bool
    Partial  bool
}

type Room struct {
    // ... existing ...
    Walls []*WallObject
}
```

---

### Task 4: `StartCombat` populates `OpacityLayer`

**Files:**
- Modify: `internal/game/combat/engine.go`
- Modify: `internal/game/combat/combat.go` (add `Combat.OpacityLayer`, `Combat.Environment`)
- Modify: `internal/game/combat/engine_test.go`

- [ ] **Step 1: Failing tests** (LOS-1, LOS-2, LOS-3):

```go
func TestStartCombat_PopulatesOpacityFromWalls(t *testing.T) {
    cbt, _ := startCombatWithRoom(t, roomWithWalls(t, []world.Cell{{X: 0, Y: 5}, {X: 1, Y: 5}}), 1, 1)
    require.Equal(t, combat.OpacityOpaque, cbt.OpacityLayer.At(0, 5))
    require.Equal(t, combat.OpacityOpaque, cbt.OpacityLayer.At(1, 5))
    require.Equal(t, combat.OpacityTransparent, cbt.OpacityLayer.At(2, 5))
}

func TestStartCombat_PopulatesOpacityFromOccludingCover(t *testing.T) {
    room := roomWithCover(t, world.CoverObject{ID: "smoke_screen", Cells: []world.Cell{{X: 5, Y: 5}}, Occludes: true, Partial: true})
    cbt, _ := startCombatWithRoom(t, room, 1, 1)
    require.Equal(t, combat.OpacityTranslucent, cbt.OpacityLayer.At(5, 5))
}

func TestStartCombat_NonOccludingCoverLeavesTransparent(t *testing.T) {
    room := roomWithCover(t, world.CoverObject{ID: "crate", Cells: []world.Cell{{X: 5, Y: 5}}}) // no Occludes
    cbt, _ := startCombatWithRoom(t, room, 1, 1)
    require.Equal(t, combat.OpacityTransparent, cbt.OpacityLayer.At(5, 5))
}

func TestStartCombat_OutOfBoundsWallCellLogsWarning(t *testing.T) {
    room := roomWithWalls(t, []world.Cell{{X: 99, Y: 99}})
    cbt, log := startCombatWithRoom(t, room, 1, 1)
    require.Contains(t, log.AllLines(), "wall cell (99,99) out of bounds")
    _ = cbt
}
```

- [ ] **Step 2: Implement** in `StartCombat`:

```go
cbt.OpacityLayer = NewOpacityLayer(cbt.GridWidth, cbt.GridHeight)
cbt.Environment = []*EnvironmentalEffect{}

for _, w := range room.Walls {
    for _, c := range w.Cells {
        if !cellInBounds(c, cbt) {
            log.Warn().Msgf("wall cell (%d,%d) out of bounds", c.X, c.Y)
            continue
        }
        cbt.OpacityLayer.Set(c.X, c.Y, OpacityOpaque)
    }
}
for _, co := range room.CoverObjects {
    if !co.Occludes {
        continue
    }
    op := OpacityOpaque
    if co.Partial {
        op = OpacityTranslucent
    }
    for _, c := range co.Cells {
        cbt.OpacityLayer.Set(c.X, c.Y, MaxOpacity(cbt.OpacityLayer.At(c.X, c.Y), op))
    }
}
```

---

### Task 5: Environmental effects — runtime + content + tick

**Files:**
- Create: `internal/game/combat/environment.go`
- Create: `internal/game/combat/environment_test.go`
- Create: `content/environment/smoke_grenade.yaml`, `fog_bank.yaml`, `darkness_zone.yaml`

- [ ] **Step 1: Failing tests** (LOS-4, LOS-20..23, LOS-Q6):

```go
func TestEnvironmentalEffect_AppliesOpacityToCells(t *testing.T) {
    cbt := newCombatWithLayer(20, 20)
    e := combat.NewEnvironmentalEffect("smoke", []combat.Cell{{X: 5, Y: 5}, {X: 6, Y: 5}}, combat.OpacityTranslucent, 5)
    cbt.AddEnvironmental(e)
    require.Equal(t, combat.OpacityTranslucent, cbt.OpacityLayer.At(5, 5))
    require.Equal(t, combat.OpacityTranslucent, cbt.OpacityLayer.At(6, 5))
}

func TestEnvironmentalEffect_TickDecrementsAndExpires(t *testing.T) {
    cbt := newCombatWithLayer(20, 20)
    cbt.AddEnvironmental(combat.NewEnvironmentalEffect("smoke", []combat.Cell{{X: 5, Y: 5}}, combat.OpacityTranslucent, 1))
    require.Equal(t, combat.OpacityTranslucent, cbt.OpacityLayer.At(5, 5))
    cbt.TickEnvironmentals()
    require.Equal(t, combat.OpacityTransparent, cbt.OpacityLayer.At(5, 5), "expired effect clears its cells")
    require.Empty(t, cbt.Environment)
}

func TestEnvironmentalEffect_OverlappingTakesMostOpaque(t *testing.T) {
    // LOS-Q6.
    cbt := newCombatWithLayer(20, 20)
    cbt.AddEnvironmental(combat.NewEnvironmentalEffect("smoke", []combat.Cell{{X: 5, Y: 5}}, combat.OpacityTranslucent, 5))
    cbt.AddEnvironmental(combat.NewEnvironmentalEffect("darkness", []combat.Cell{{X: 5, Y: 5}}, combat.OpacityOpaque, 5))
    require.Equal(t, combat.OpacityOpaque, cbt.OpacityLayer.At(5, 5))
}

func TestEnvironmentalEffect_ExpiryRevertsToBaseline(t *testing.T) {
    // Wall is opaque; smoke is translucent on the same cell. When smoke expires, cell stays opaque (wall).
    cbt := newCombatWithLayerAndWall(20, 20, combat.Cell{X: 5, Y: 5})
    cbt.AddEnvironmental(combat.NewEnvironmentalEffect("smoke", []combat.Cell{{X: 5, Y: 5}}, combat.OpacityTranslucent, 1))
    cbt.TickEnvironmentals()
    require.Equal(t, combat.OpacityOpaque, cbt.OpacityLayer.At(5, 5), "wall persists after smoke expires")
}
```

- [ ] **Step 2: Implement**:

```go
type EnvironmentalEffect struct {
    ID              string
    Cells           []Cell
    Opacity         Opacity
    DurationRounds  int
}

type Combat struct {
    // ... existing ...
    OpacityLayer *OpacityLayer
    Environment  []*EnvironmentalEffect
    baselineOpacity *OpacityLayer // captured at StartCombat from walls + cover; never mutated
}

func (c *Combat) AddEnvironmental(e *EnvironmentalEffect) {
    c.Environment = append(c.Environment, e)
    c.recomputeOpacity()
}

func (c *Combat) TickEnvironmentals() {
    surviving := c.Environment[:0]
    for _, e := range c.Environment {
        e.DurationRounds--
        if e.DurationRounds > 0 {
            surviving = append(surviving, e)
        }
    }
    c.Environment = surviving
    c.recomputeOpacity()
}

func (c *Combat) recomputeOpacity() {
    // Reset to baseline (walls + occluding cover), then layer environmentals on top with MaxOpacity.
    *c.OpacityLayer = *c.baselineOpacity.Clone()
    for _, e := range c.Environment {
        for _, cell := range e.Cells {
            cur := c.OpacityLayer.At(cell.X, cell.Y)
            c.OpacityLayer.Set(cell.X, cell.Y, MaxOpacity(cur, e.Opacity))
        }
    }
}
```

`baselineOpacity` is captured at the end of `StartCombat`'s wall/cover population (Task 4) so environmentals never permanently overwrite the room's intrinsic opacity.

- [ ] **Step 3: Round tick integration** — `Combat.StartRound` (or end-of-round) calls `TickEnvironmentals()` after condition ticks.

- [ ] **Step 4: Author** the three exemplars (LOS-21):

```yaml
# content/environment/smoke_grenade.yaml
id: smoke_grenade
display_name: Smoke Grenade
description: A burst of dense smoke obscuring vision for several rounds.
cells_relative_to: { kind: anchor }
shape:
  kind: burst
  radius: 10  # 2 cells
opacity: translucent
duration_rounds: 5
```

(Plus `fog_bank.yaml` — translucent, 10 rounds, 5×5; `darkness_zone.yaml` — opaque, 20 rounds, 7×7.)

- [ ] **Step 5: Loader** for `content/environment/*.yaml` that produces a `Definition` template; the runtime `EnvironmentalEffect` instance is created at apply time with the anchor cell substituted.

---

### Task 6: `OcclusionProvider` real implementation (replaces #254 v1 no-op)

**Files:**
- Modify: `internal/game/detection/occlusion.go`
- Modify: `internal/game/detection/occlusion_test.go`

- [ ] **Step 1: Failing tests** (LOS-9, LOS-10, LOS-11):

```go
func TestOcclusionProvider_BlockedBetweenWallSeparatedCombatants(t *testing.T) {
    cbt, observer, target := setupWithWallBetween(t)
    p := combat.NewOcclusionProvider(cbt)
    require.Equal(t, detection.LoSBlocked, p.LineOfSight(observer.UID, target.UID))
}

func TestOcclusionProvider_ConcealedThroughTranslucentCover(t *testing.T) {
    cbt, observer, target := setupWithTranslucentCoverBetween(t)
    p := combat.NewOcclusionProvider(cbt)
    require.Equal(t, detection.LoSConcealed, p.LineOfSight(observer.UID, target.UID))
}

func TestOcclusionProvider_ClearWhenNoOccluders(t *testing.T) {
    cbt, observer, target := setupOpenField(t)
    p := combat.NewOcclusionProvider(cbt)
    require.Equal(t, detection.LoSClear, p.LineOfSight(observer.UID, target.UID))
}
```

- [ ] **Step 2: Implement**:

```go
package combat

func NewOcclusionProvider(cbt *Combat) detection.OcclusionProvider {
    return &occProvider{cbt: cbt}
}

type occProvider struct{ cbt *Combat }

func (p *occProvider) LineOfSight(observerUID, targetUID string) detection.LoSResult {
    obs := p.cbt.ByUID(observerUID)
    tgt := p.cbt.ByUID(targetUID)
    if obs == nil || tgt == nil {
        return detection.LoSClear // unknown → conservative; detection layer decides
    }
    res := LineOfSight(p.cbt.OpacityLayer, obs.Cell(), tgt.Cell())
    return translateLoS(res)
}
```

- [ ] **Step 3:** The transition rules from #254 (LOS-10, LOS-11) live in the detection layer; this plan only emits the `Result`. Verify `Combat.Start()` uses `NewOcclusionProvider(cbt)` instead of the old no-op when initialising `DetectionStates`.

---

### Task 7: Targeting validation — TGT-21 LoF stage

**Files:**
- Modify: `internal/game/combat/targeting.go`
- Modify: `internal/game/combat/targeting_test.go`

- [ ] **Step 1: Failing tests** (LOS-12, LOS-13):

```go
func TestValidateSingleTarget_LoFBlocked_Rejects(t *testing.T) {
    cbt, attacker, target := setupWithWallBetween(t)
    res := combat.ValidateSingleTarget(cbt, attacker, target.UID, combat.TargetSingleEnemy, 60, true /* requiresLoF */)
    require.Equal(t, combat.ErrLineOfFireBlocked, res.Err)
    require.Contains(t, res.Detail, "no line of sight")
}

func TestValidateSingleTarget_LoFConcealed_AllowsWithWarning(t *testing.T) {
    cbt, attacker, target := setupWithTranslucentCoverBetween(t)
    res := combat.ValidateSingleTarget(cbt, attacker, target.UID, combat.TargetSingleEnemy, 60, true)
    require.Equal(t, combat.TargetOK, res.Err)
    require.True(t, res.Concealed, "concealed warning bit set")
}

func TestValidateSingleTarget_LoFClear_NoWarning(t *testing.T) {
    cbt, attacker, target := setupOpenField(t)
    res := combat.ValidateSingleTarget(cbt, attacker, target.UID, combat.TargetSingleEnemy, 60, true)
    require.Equal(t, combat.TargetOK, res.Err)
    require.False(t, res.Concealed)
}

func TestValidateAoE_OriginBlocked_Rejects(t *testing.T) {
    cbt, attacker := setupWithWallAt(t, combat.Cell{X: 5, Y: 5})
    tmpl := combat.AoETemplate{Shape: combat.AoEBurst, CenterX: 9, CenterY: 5, Radius: 10}
    res := combat.ValidateAoE(cbt, attacker, tmpl, combat.TargetAoEBurst, 60, false)
    require.Equal(t, combat.ErrLineOfFireBlocked, res.Err)
}
```

- [ ] **Step 2: Replace TGT-21 no-op** with a real LoF check:

```go
// In ValidateSingleTarget after the existing 5 stages:
if requiresLineOfFire {
    res := LineOfSight(cbt.OpacityLayer, attacker.Cell(), target.Cell())
    switch res {
    case LoSBlocked:
        return TargetingResult{Err: ErrLineOfFireBlocked, Detail: "no line of sight"}
    case LoSConcealed:
        // allow with warning bit
        return TargetingResult{Err: TargetOK, Concealed: true}
    }
}
return TargetingResult{Err: TargetOK}
```

`TargetingResult.Concealed` is a new bit; tests in the targeting suite cover it.

- [ ] **Step 3: AoE validation** — same check applied to the template's anchor / origin cell.

---

### Task 8: `PostFilterAffectedCells` real implementation

**Files:**
- Modify: `internal/gameserver/aoe_resolver.go` (per #250 AOE-21)
- Modify: `internal/gameserver/grpc_service_aoe_test.go`

- [ ] **Step 1: Failing tests** (LOS-14):

```go
func TestPostFilterAffectedCells_DropsBlockedCells(t *testing.T) {
    cbt, attacker := setupWithWallBetween(t, attackerAt: combat.Cell{X: 0, Y: 5}, wallAt: combat.Cell{X: 5, Y: 5})
    cells := []combat.Cell{{X: 1, Y: 5}, {X: 4, Y: 5}, {X: 6, Y: 5}, {X: 9, Y: 5}}
    out := combat.PostFilterAffectedCells(cells, combat.ResolveContext{Cbt: cbt, AttackerCell: attacker.Cell()})
    require.Equal(t, []combat.Cell{{X: 1, Y: 5}, {X: 4, Y: 5}}, out, "post-wall cells dropped")
}

func TestPostFilterAffectedCells_KeepsConcealedCells(t *testing.T) {
    cbt, attacker := setupWithTranslucentCoverBetween(t, attackerAt: combat.Cell{X: 0, Y: 5}, smokeAt: combat.Cell{X: 5, Y: 5})
    cells := []combat.Cell{{X: 4, Y: 5}, {X: 6, Y: 5}, {X: 9, Y: 5}}
    out := combat.PostFilterAffectedCells(cells, combat.ResolveContext{Cbt: cbt, AttackerCell: attacker.Cell()})
    require.ElementsMatch(t, cells, out, "concealed cells stay (LOS-14)")
}
```

- [ ] **Step 2: Implement** — replace #250 v1 identity with the LoS-aware filter:

```go
func PostFilterAffectedCells(cells []combat.Cell, ctx combat.ResolveContext) []combat.Cell {
    out := cells[:0]
    for _, c := range cells {
        if combat.LineOfSight(ctx.Cbt.OpacityLayer, ctx.AttackerCell, c) != combat.LoSBlocked {
            out = append(out, c)
        }
    }
    return out
}
```

---

### Task 9: UI surfacing — telnet target list + web target picker + AoE preview dim

**Files:**
- Modify: `cmd/webclient/ui/src/combat/TargetPanel.tsx`
- Modify: `cmd/webclient/ui/src/combat/TargetPanel.test.tsx`
- Modify: `cmd/webclient/ui/src/combat/MapPanel.tsx` (AoE preview dimming)
- Modify: `internal/frontend/handlers/target_command.go` (omit blocked + concealed annotation)

- [ ] **Step 1: Failing telnet tests** (LOS-15, LOS-17):

```go
func TestTargetCommand_OmitsBlockedTargets(t *testing.T) {
    h := newHandlerWithLoS(t, withBlocked: "thug_behind_wall", withClear: "open_thug")
    out := h.Run("target")
    require.NotContains(t, out, "thug_behind_wall")
    require.Contains(t, out, "open_thug")
}

func TestTargetCommand_AnnotatesConcealedTargets(t *testing.T) {
    h := newHandlerWithLoS(t, withConcealed: "smoky_thug", withClear: "open_thug")
    out := h.Run("target")
    require.Contains(t, out, "smoky_thug (concealed)")
    require.Contains(t, out, "open_thug")
    require.NotContains(t, out, "open_thug (concealed)")
}
```

- [ ] **Step 2: Failing web tests** (LOS-16, LOS-17, LOS-18):

```ts
test("TargetPanel grays out targets without line of sight", () => {
  render(<TargetPanel combatants={[{ uid: "thug", losStatus: "blocked", name: "Thug" }]} />);
  const row = screen.getByText("Thug").closest("li")!;
  expect(row).toHaveClass("disabled");
});

test("TargetPanel shows tooltip 'no line of sight' on blocked target click", () => {
  render(<TargetPanel combatants={[{ uid: "thug", losStatus: "blocked", name: "Thug" }]} />);
  fireEvent.mouseOver(screen.getByText("Thug"));
  expect(screen.getByText(/no line of sight/i)).toBeVisible();
});

test("TargetPanel annotates concealed targets", () => {
  render(<TargetPanel combatants={[{ uid: "thug", losStatus: "concealed", name: "Thug" }]} />);
  expect(screen.getByText(/Thug.*concealed/)).toBeVisible();
});

test("MapPanel AoE preview dims post-filtered cells", () => {
  render(<MapPanel aoePreview={{ cells: [{x:1,y:1},{x:5,y:1}], filteredOut: [{x:5,y:1}] }} />);
  const cell51 = screen.getByTestId("aoe-cell-5-1");
  expect(cell51).toHaveClass("aoe-cell-dimmed");
  const cell11 = screen.getByTestId("aoe-cell-1-1");
  expect(cell11).not.toHaveClass("aoe-cell-dimmed");
});
```

- [ ] **Step 3: Implement** the three surfaces. The wire payload (`CombatantView`) gains a `los_status` field (`clear` / `concealed` / `blocked`) populated server-side per (player, combatant) pair from the `OcclusionProvider`.

---

### Task 10: `look <direction>` command

**Files:**
- Create: `internal/frontend/handlers/look_command.go` (or extend existing)
- Create: `internal/frontend/handlers/look_command_test.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Failing tests** (LOS-19):

```go
func TestLookDirection_ReportsClearCells(t *testing.T) {
    h := newHandlerInRoomWithEastClearTo(t, 10)
    out := h.Run("look east")
    require.Contains(t, out, "east: clear out to 10 cells")
}

func TestLookDirection_TerminatesAtFirstOpaque(t *testing.T) {
    h := newHandlerInRoomWithWallAt(t, dx: 4, dy: 0)
    out := h.Run("look east")
    require.Contains(t, out, "east: wall at 4 cells")
}

func TestLookDirection_NotesConcealedCells(t *testing.T) {
    h := newHandlerInRoomWithSmokeAt(t, dx: 3, dy: 0)
    out := h.Run("look east")
    require.Contains(t, out, "east: concealed (smoke) at 3 cells")
}

func TestLookDirection_RejectsUnknownDirection(t *testing.T) {
    h := newHandlerInOpenRoom(t)
    out := h.Run("look forward")
    require.Contains(t, out, "unknown direction")
}
```

- [ ] **Step 2: Implement**. The command walks cells from the player's position along the named compass octant, emitting one summary line. Uses `combat.LineOfSight` cell-by-cell or directly walks the layer.

---

### Task 11: Architecture documentation update

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Line of Sight" section** documenting:
  - The opacity model (`Transparent` / `Translucent` / `Opaque`).
  - The `WallObject` schema and `CoverObject.Occludes` / `Partial` flags.
  - The `LineOfSight` algorithm (Bresenham, exclusive endpoints, adjacent + self short-circuit).
  - The `OcclusionProvider` shape and how the detection layer (#254) consumes it.
  - The `EnvironmentalEffect` runtime + content authoring shape.
  - The integration points: `ValidateSingleTarget` / `ValidateAoE` LoF stages and the AoE `PostFilterAffectedCells` post-filter.
  - The UI surfacing contract (gray-out blocked, annotate concealed, dim AoE-filtered).
  - Open question resolutions (LOS-Q1..Q6).

- [ ] **Step 2: Cross-link** spec, plan, the predecessor specs (#247 / #249 / #250 / #254), and the no-op interface that this plan replaces.

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- Telnet smoke test: enter a room with a wall between attacker and defender; verify `target` omits the defender; move around the wall; verify they appear; throw a smoke grenade; verify `(concealed)` annotation; wait 5 rounds; verify the smoke clears.
- Web smoke test: same scenarios; verify the gray-out + tooltip + concealed annotation; AoE preview dims cells past a wall.
- `look east` in an open field reports clear; through smoke reports concealed; through a wall terminates.

---

## Rollout / Open Questions Resolved at Plan Time

- **LOS-Q1**: Walls are binary — opaque until destroyed, transparent when destroyed. Stepwise behaviour is LOS-F8.
- **LOS-Q2**: Single representative line (cell-center to cell-center). Multi-cell creatures are out of scope.
- **LOS-Q3**: Symmetric — your own smoke obscures you to outside observers and obscures your view outward.
- **LOS-Q4**: Seek consults LoS via the OcclusionProvider; through-wall Seek fails.
- **LOS-Q5**: `look <direction>` ships in v1.
- **LOS-Q6**: Overlapping environmental effects take the most opaque per cell.

## Non-Goals Reaffirmed

Per spec §2.2:

- No concealment miss-chance roll (#254 DETECT-7 owns it).
- No darkvision / nightvision / blindsight / scent / tremorsense (LOS-F2).
- No first-class light-source modelling.
- No persistent fog-of-war beyond effect duration.
- No volumetric / 3D occlusion.
- No authoring UI for walls (LOS-F6).
- No LoS-aware weapons (e.g., guided missiles).
