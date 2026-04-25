# Smarter NPC Movement in Combat — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the binary "stride toward / stride away" NPC heuristic with a per-NPC tactical destination chooser that scores candidate cells against four weighted goals — `range`, `cover`, `spread`, `terrain` — and routes the chosen cell through the existing `ActionStride` machinery. HTN domains keep ownership of NPCs that explicitly plan movement; the new step fills the gap when the planner does not.

**Spec:** [docs/superpowers/specs/2026-04-24-smarter-npc-movement-in-combat.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-24-smarter-npc-movement-in-combat.md)

**Architecture:** A new `chooseMoveDestination` function lives in `internal/gameserver/combat_handler_move.go`, returning a `*Cell` (or `nil` for "no move"). The legacy `npcMovementStrideLocked` becomes a thin wrapper that calls it and converts a destination into the existing `"toward"` / `"away"` direction string for backward compatibility. Each goal is a pure function `Cell → float64`; the total score is a weighted sum (`wRange=1.0`, `wCover=0.7`, `wSpread=0.4`, `wTerrain=0.6` defaults). Per-NPC weight overrides ride on a new `CombatStrategy.MoveWeights` block on `NPCTemplate`. Wounded modifiers (`wCover` doubles at or below `WoundedHPPct`), flee modifiers (sign-flip on `rangeGoal` at or below `FleeHPPct` or above `CourageThreshold`), and an HTN action (`move_to_position`) round out the system. Single sources of truth are reused — cover tier from #247, terrain type from #248 (no-op default until #248 lands), AP/speed from `combat.MaxMovementAP`/`SpeedBudget`.

**Tech Stack:** Go, `pgregory.net/rapid` (property tests), `internal/gameserver/`, `internal/game/ai/`, `internal/game/npc/`, existing combat packages.

**Prerequisite:** None hard. #247 (cover) and #248 (terrain) are soft deps — `coverGoal` returns `0.0` until #247 ships its tier function; `terrainGoal` returns `1.0` uniformly until #248 lands. Both are wired now so those tickets plug in without further AI changes.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/gameserver/combat_handler_move.go` |
| Create | `internal/gameserver/combat_handler_move_test.go` |
| Create | `internal/gameserver/testdata/rapid/TestChooseMoveDestination_Property/` |
| Modify | `internal/gameserver/combat_handler.go` (`npcMovementStrideLocked` thin wrapper; HTN short-circuit) |
| Modify | `internal/game/npc/template.go` (`CombatStrategy.MoveWeights`, `WoundedHPPct`) |
| Modify | `internal/game/ai/domain.go` (`move_to_position` action shape) |
| Modify | `internal/game/ai/build_state.go` (`WorldState.MoveScores`) |
| Modify | `internal/gameserver/grpc_service_stride_test.go` (sanity guard regression — no change to assertions) |
| Modify | `docs/architecture/combat.md` (NPC movement section) |

---

### Task 1: `Cell` type + `candidateCells` enumeration

**Files:**
- Create: `internal/gameserver/combat_handler_move.go`
- Create: `internal/gameserver/combat_handler_move_test.go`

- [ ] **Step 1: Failing tests** for `candidateCells`:

```go
func TestCandidateCells_IncludesCurrentCell(t *testing.T) {
    cbt := newCombatWithGrid(20, 20)
    npc := newCombatantAt(cbt, "n1", 5, 5, true)
    cells := candidateCells(cbt, npc)
    require.Contains(t, cells, Cell{X: 5, Y: 5}, "current cell must always be a candidate (MOVE-9)")
}

func TestCandidateCells_ExcludesOccupiedAndOOB(t *testing.T) {
    cbt := newCombatWithGrid(20, 20)
    npc := newCombatantAt(cbt, "n1", 0, 0, true)
    other := newCombatantAt(cbt, "p1", 0, 1, true)
    cells := candidateCells(cbt, npc)
    require.NotContains(t, cells, Cell{X: 0, Y: 1}, "occupied cells excluded (MOVE-7)")
    require.NotContains(t, cells, Cell{X: -1, Y: 0}, "OOB excluded (MOVE-8)")
    _ = other
}

func TestCandidateCells_BoundedByMovementAPAndSpeed(t *testing.T) {
    cbt := newCombatWithGrid(20, 20)
    npc := newCombatantAt(cbt, "n1", 10, 10, true)
    npc.UsedAP = 0
    speed := npc.SpeedSquares() // or SpeedBudget() once #248 lands
    maxRing := combat.MaxMovementAP * speed
    cells := candidateCells(cbt, npc)
    for _, c := range cells {
        if max(abs(c.X-10), abs(c.Y-10)) > maxRing {
            t.Fatalf("cell %+v exceeds reachable Chebyshev ring %d", c, maxRing)
        }
    }
}
```

- [ ] **Step 2: Implement** `candidateCells` using `combat.MaxMovementAP * speed` as the Chebyshev radius bound, then filtering through `CellOccupied` / `CellBlocked` and grid bounds. Always include the current cell (MOVE-9).

```go
type Cell = combat.Cell // reuse the geometry Cell type

func candidateCells(cbt *combat.Combat, c *combat.Combatant) []Cell {
    speed := movementSpeed(c) // SpeedBudget() once #248 lands; SpeedSquares() until then
    apLeft := combat.MaxMovementAP - movementAPUsedThisRound(c)
    if apLeft < 0 {
        apLeft = 0
    }
    radius := apLeft * speed
    out := []Cell{{X: c.GridX, Y: c.GridY}}
    for dx := -radius; dx <= radius; dx++ {
        for dy := -radius; dy <= radius; dy++ {
            if dx == 0 && dy == 0 {
                continue
            }
            x, y := c.GridX+dx, c.GridY+dy
            if x < 0 || y < 0 || x >= cbt.GridWidth || y >= cbt.GridHeight {
                continue
            }
            if combat.CellBlocked(cbt, c.ID, x, y) {
                continue
            }
            // greater_difficult exclusion is handled in terrainGoal — passable check here.
            if !cbt.IsPassable(x, y) {
                continue
            }
            out = append(out, Cell{X: x, Y: y})
        }
    }
    return out
}
```

`cbt.IsPassable` is a thin shim that returns `true` until #248's terrain layer lands; afterwards it consults `EntryCost`'s impassable bit (MOVE-14 last sentence).

- [ ] **Step 3:** All three tests pass.

---

### Task 2: Goal functions — `rangeGoal`, `coverGoal`, `spreadGoal`, `terrainGoal`

**Files:**
- Modify: `internal/gameserver/combat_handler_move.go`
- Modify: `internal/gameserver/combat_handler_move_test.go`

- [ ] **Step 1: Failing tests** for each goal (boundaries + monotonicity):

```go
func TestRangeGoal_MeleeMaxAtAdjacent(t *testing.T) {
    npc := meleeNPC()       // RangeIncrement = 0
    target := combatantAt(5, 5)
    score := rangeGoal(npc, target, Cell{X: 4, Y: 5}) // adjacent
    require.InDelta(t, 1.0, score, 0.001)
}

func TestRangeGoal_RangedMaxAtIncrementDistance(t *testing.T) {
    npc := rangedNPC(15)    // RangeIncrement = 15 ft = 3 cells
    target := combatantAt(10, 10)
    score := rangeGoal(npc, target, Cell{X: 7, Y: 10})
    require.InDelta(t, 1.0, score, 0.001)
}

func TestRangeGoal_RangedNeverPrefersPointBlank(t *testing.T) {
    npc := rangedNPC(15)
    target := combatantAt(10, 10)
    here := rangeGoal(npc, target, Cell{X: 9, Y: 10})  // 5 ft
    farther := rangeGoal(npc, target, Cell{X: 7, Y: 10}) // 15 ft
    require.Less(t, here, farther, "ranged NPC must never prefer cells inside 5 ft (MOVE-11 last clause)")
}

func TestCoverGoal_DisabledByStrategy(t *testing.T) {
    npc := meleeNPC()
    npc.Strategy.UseCover = false
    target := combatantAt(5, 5)
    require.InDelta(t, 0.0, coverGoal(npc, target, Cell{X: 4, Y: 5}, fakeCoverTier), 0.001)
}

func TestSpreadGoal_AdjacentAllyZero(t *testing.T) {
    npc := combatantAt(5, 5)
    npc.FactionID = "thugs"
    ally := combatantAt(5, 6)
    ally.FactionID = "thugs"
    require.InDelta(t, 0.0, spreadGoal(npc, []*combat.Combatant{ally}, Cell{X: 5, Y: 5}), 0.001)
}

func TestTerrainGoal_NormalUntilTerrainLayerLands(t *testing.T) {
    cbt := newCombatWithGrid(20, 20) // no terrain populated
    require.InDelta(t, 1.0, terrainGoal(cbt, Cell{X: 3, Y: 3}), 0.001)
}
```

- [ ] **Step 2: Implement** each goal per the spec:

```go
func rangeGoal(c *combat.Combatant, target *combat.Combatant, cell Cell) float64 {
    eff := 1 // melee
    if c.RangeIncrementFt > 0 {
        eff = c.RangeIncrementFt / 5
    }
    dist := chebyshev(cell, Cell{X: target.GridX, Y: target.GridY})
    if c.RangeIncrementFt > 0 && dist <= 1 {
        // Ranged NPCs never prefer point-blank.
        return 0.0
    }
    delta := abs(dist - eff)
    worst := maxGridDistance(c)
    return clamp01(1.0 - float64(delta)/float64(worst))
}

func coverGoal(c *combat.Combatant, target *combat.Combatant, cell Cell, tierFn coverTierFn) float64 {
    if !c.Strategy.UseCover {
        return 0.0
    }
    switch tierFn(cell, target) {
    case combat.CoverNone:    return 0.0
    case combat.CoverLesser:  return 0.33
    case combat.CoverStandard:return 0.66
    case combat.CoverGreater: return 1.0
    }
    return 0.0
}

func spreadGoal(c *combat.Combatant, allies []*combat.Combatant, cell Cell) float64 {
    nearest := math.MaxInt32
    for _, a := range allies {
        if a == c || a.IsDead() || a.FactionID != c.FactionID {
            continue
        }
        d := chebyshev(cell, Cell{X: a.GridX, Y: a.GridY})
        if d < nearest {
            nearest = d
        }
    }
    if nearest == math.MaxInt32 {
        return 1.0 // no allies
    }
    cap := max(c.RangeIncrementFt/5, 6) // 6 cells = 30 ft
    return clamp01(float64(nearest) / float64(cap))
}

func terrainGoal(cbt *combat.Combat, cell Cell) float64 {
    tt := cbt.TerrainAt(cell.X, cell.Y) // returns Normal until #248 lands
    switch tt.Type {
    case combat.TerrainNormal:    return 1.0
    case combat.TerrainDifficult: return 0.5
    case combat.TerrainHazardous: return 0.0
    }
    return 1.0
}
```

`tierFn` is a small interface so #247's positional cover model can be injected; until #247 ships, a constant-`CoverNone` stub is used.

- [ ] **Step 3:** All goal tests green.

---

### Task 3: Scoring + selection + `chooseMoveDestination`

**Files:**
- Modify: `internal/gameserver/combat_handler_move.go`
- Modify: `internal/gameserver/combat_handler_move_test.go`
- Modify: `internal/game/npc/template.go` (`CombatStrategy.MoveWeights`, `WoundedHPPct`)

- [ ] **Step 1: Add `MoveWeights` and `WoundedHPPct`** to `CombatStrategy` with defaults from MOVE-16 / MOVE-18.

```go
type MoveWeights struct {
    RangeWeight   float64
    CoverWeight   float64
    SpreadWeight  float64
    TerrainWeight float64
}

func (w MoveWeights) WithDefaults() MoveWeights {
    if w.RangeWeight   == 0 { w.RangeWeight   = 1.0 }
    if w.CoverWeight   == 0 { w.CoverWeight   = 0.7 }
    if w.SpreadWeight  == 0 { w.SpreadWeight  = 0.4 }
    if w.TerrainWeight == 0 { w.TerrainWeight = 0.6 }
    return w
}

type CombatStrategy struct {
    // ... existing fields ...
    MoveWeights      MoveWeights
    WoundedHPPct     int  // default 50 (zero treated as 50)
    FleeHPPct        int  // existing
    CourageThreshold int  // existing
    UseCover         bool // existing
}
```

- [ ] **Step 2: Implement scoring + selection**:

```go
func chooseMoveDestination(cbt *combat.Combat, c *combat.Combatant) *Cell {
    target := currentTarget(cbt, c)
    if target == nil {
        return nil
    }
    candidates := candidateCells(cbt, c)
    if len(candidates) <= 1 {
        return nil // MOVE-10
    }

    weights := c.Strategy.MoveWeights.WithDefaults()
    if hpPct(c) <= effectiveWoundedHPPct(c) {
        weights.CoverWeight *= 2 // MOVE-18
    }
    flipRange := hpPct(c) <= c.Strategy.FleeHPPct ||
        combatThreat(cbt, c) > c.Strategy.CourageThreshold // MOVE-25, MOVE-26

    allies := factionAllies(cbt, c)
    bestScore := math.Inf(-1)
    var best Cell
    for _, cell := range candidates {
        rg := rangeGoal(c, target, cell)
        if flipRange {
            rg = 1.0 - rg
        }
        score := weights.RangeWeight   * rg +
                 weights.CoverWeight   * coverGoal(c, target, cell, coverTier) +
                 weights.SpreadWeight  * spreadGoal(c, allies, cell) +
                 weights.TerrainWeight * terrainGoal(cbt, cell)
        if score > bestScore || (score == bestScore && tieBreak(cell, best)) {
            bestScore = score
            best = cell
        }
    }

    here := Cell{X: c.GridX, Y: c.GridY}
    hereScore := scoreOf(here, ...) // recompute or cache
    if best == here || (bestScore - hereScore) < 0.01 {
        return nil // MOVE-20 epsilon-stay
    }
    return &best
}
```

`tieBreak(a, b)` returns `true` if `a` should win when `score(a) == score(b)` — `(GridY, GridX)` ascending (MOVE-19).

- [ ] **Step 3: Failing scenario tests** mirroring the spec MOVE-32 list:

```go
func TestChoose_MeleeNPC_PrefersCoverCellAtPlusOne(t *testing.T) { ... }
func TestChoose_RangedNPC_MovesToRangeIncrement(t *testing.T) { ... }
func TestChoose_WoundedNPC_FleesButHugsCover(t *testing.T) { ... }
func TestChoose_ThreeAlliesAdjacent_OneSpreads(t *testing.T) { ... }
func TestChoose_HazardousCell_NeverChosen(t *testing.T) { ... }
func TestChoose_GreaterDifficult_ExcludedFromCandidates(t *testing.T) { ... }
```

Each builds a small fixture combat, calls `chooseMoveDestination`, and asserts the returned cell satisfies the spec's predicate. All green before continuing.

---

### Task 4: Property tests under `testdata/rapid/`

**Files:**
- Create: `internal/gameserver/testdata/rapid/TestChooseMoveDestination_Property/`
- Modify: `internal/gameserver/combat_handler_move_test.go`

- [ ] **Step 1: Property tests** per MOVE-31:

```go
func TestProperty_ChooseMoveDestination_Determinism(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cbt := buildArbitraryCombat(t)
        npc := pickArbitraryNPC(t, cbt)
        a := chooseMoveDestination(cbt, npc)
        b := chooseMoveDestination(cbt, npc)
        require.Equal(t, a, b, "same state must produce same destination")
    })
}

func TestProperty_ChooseMoveDestination_BoundedByCandidateSet(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cbt := buildArbitraryCombat(t)
        npc := pickArbitraryNPC(t, cbt)
        cells := candidateCells(cbt, npc)
        d := chooseMoveDestination(cbt, npc)
        if d == nil {
            return
        }
        require.Contains(t, cells, *d)
    })
}

func TestProperty_ChooseMoveDestination_StrideBudget(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cbt := buildArbitraryCombat(t)
        npc := pickArbitraryNPC(t, cbt)
        d := chooseMoveDestination(cbt, npc)
        if d == nil {
            return
        }
        steps := stridesNeeded(npc, *d) // chebyshev / SpeedBudget
        require.LessOrEqual(t, steps, combat.MaxMovementAP)
    })
}

func TestProperty_SpreadMonotonicity(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Two cells equal on (range, cover, terrain); one adjacent to ally, one not.
        // The ally-free cell must score >= the adjacent-to-ally cell.
        ...
    })
}
```

- [ ] **Step 2:** Verify rapid recordings are checked into `testdata/rapid/`. Empty failures captured automatically.

---

### Task 5: Stride conversion + `npcMovementStrideLocked` wrapper

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

- [ ] **Step 1: Failing test** that pins legacy direction-only behavior under the new code path:

```go
func TestNPCAutoStride_MeleeNPC_ClosesDistance(t *testing.T) {
    // Existing test from grpc_service_stride_test.go:337 — must still pass.
    // Adapted assertion: when chooseMoveDestination returns a destination one cell
    // closer to the target, the wrapper returns "toward".
    ...
}
```

- [ ] **Step 2: Convert `npcMovementStrideLocked` to a wrapper**:

```go
func (s *Service) npcMovementStrideLocked(cbt *combat.Combat, c *combat.Combatant) string {
    dest := chooseMoveDestination(cbt, c)
    if dest == nil {
        return ""
    }
    return directionFromTo(Cell{X: c.GridX, Y: c.GridY}, *dest)
}

func directionFromTo(a, b Cell) string {
    dx := sign(b.X - a.X)
    dy := sign(b.Y - a.Y)
    // Map to existing string set used by the legacy direction path.
    // For this wrapper, "toward"/"away" suffice for the legacy stride loop because
    // the direct destination is consumed by the new path (Step 3).
    if dx == 0 && dy == 0 {
        return ""
    }
    if towardTarget(a, b /*relative to current target*/) {
        return "toward"
    }
    return "away"
}
```

- [ ] **Step 3: Add multi-stride decomposition** (MOVE-21, MOVE-22, MOVE-23):

```go
func (s *Service) queueStridesToDestination(cbt *combat.Combat, c *combat.Combatant, dest Cell) {
    speed := movementSpeed(c)
    apLeft := combat.MaxMovementAP - movementAPUsedThisRound(c)
    cur := Cell{X: c.GridX, Y: c.GridY}
    for ap := 0; ap < apLeft; ap++ {
        if cur == dest {
            break
        }
        // One stride moves up to `speed` cells along Chebyshev path.
        next := stepToward(cur, dest, speed, cbt) // respects CellBlocked, terrain
        if next == cur {
            break
        }
        s.enqueueStrideAction(c, directionFromTo(cur, next))
        cur = next
    }
}
```

- [ ] **Step 4:** Existing tests `TestNPCAutoStride_MeleeNPC_ClosesDistance` and `TestNPCAutoStride_RangedNPC_DoesNotStride` pass without modification (MOVE-30).

---

### Task 6: HTN plumbing — `move_to_position` action + `WorldState.MoveScores`

**Files:**
- Modify: `internal/game/ai/domain.go`
- Modify: `internal/game/ai/build_state.go`
- Modify: `internal/gameserver/combat_handler.go` (HTN short-circuit per MOVE-3, MOVE-28)

- [ ] **Step 1: Failing tests**:
  - HTN domain returning `move_to_position{x: 7, y: 4}` short-circuits `chooseMoveDestination` (MOVE-3, MOVE-28).
  - HTN domain returning no movement at all → `chooseMoveDestination` runs (MOVE-4).
  - `WorldState.MoveScores` is populated for the NPC's candidate cells; values match the per-goal weighted sum.

- [ ] **Step 2: Add the action shape** to the HTN domain DSL:

```go
type ActionMoveToPosition struct {
    X, Y int
}
```

The HTN compiler picks this up and the planner can return it inside a plan step.

- [ ] **Step 3: Add `MoveScores`** to `WorldState`:

```go
type WorldState struct {
    // ... existing fields ...
    MoveScores map[Cell]float64
}
```

Populated once per NPC turn in `build_state.go` (call `chooseMoveDestination` lite — score-only — and stash the map).

- [ ] **Step 4: HTN short-circuit** at the planner-result handling site in `combat_handler.go`: if the plan contains `ActionStride{Direction: ...}` or `ActionMoveToPosition`, do NOT call `chooseMoveDestination`. Otherwise call it as today.

---

### Task 7: NPC template loader — `MoveWeights` YAML

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/template_test.go`

- [ ] **Step 1: Failing tests** for:
  - YAML loads `combat_strategy.move_weights: { range: 1.5, cover: 0.0 }` into `CombatStrategy.MoveWeights`.
  - Missing fields default per MOVE-16.
  - `combat_strategy.wounded_hp_pct: 30` loads correctly; missing → defaults to 50.

- [ ] **Step 2: Implement** the YAML mapping. Validation: weights MUST be `>= 0`; `WoundedHPPct` MUST be `[0, 100]`.

- [ ] **Step 3: Documentation** in the NPC content authoring doc (existing `docs/architecture/npcs.md` or wherever combat-strategy lives) describing the new fields with worked examples.

---

### Task 8: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "NPC Movement" section** documenting:
  - All 32 `MOVE-N` requirements with one-line summaries.
  - The decision flow (HTN → planner-destination → `chooseMoveDestination` → stride loop).
  - Each goal's score formula and bounds.
  - The default weights and the wounded / flee / courage modifiers.
  - The `move_to_position` HTN action and the `WorldState.MoveScores` debugging affordance.
  - The integration seams for #247 (cover) and #248 (terrain) — they plug into `coverGoal` / `terrainGoal` with no further AI churn.
  - Open question resolutions (single-target cover, allies-only spread, no AoE-reactive movement in v1).

- [ ] **Step 2: Cross-link** to `combat_handler_move.go`, the spec, and the related cover / terrain / AoE plans.

- [ ] **Step 3:** Verify the doc renders correctly in GitHub markdown preview.

---

## Verification

Per SWENG-6, the full test suite MUST pass before commit / PR:

```
go test ./...
```

Additional sanity:

- `go vet ./...` clean.
- Stock encounter playthrough on a stock zone shows visibly different positioning vs. main: at least one NPC sidesteps into cover or away from clustered allies during a 4-round combat (Acceptance bullet from the spec).
- Profile at default weights on a 20×20 grid with 8 NPCs per round shows decision time well under round-tick budget.

---

## Rollout / Open Questions Resolved at Plan Time

- **MOVE-Q1**: `coverGoal` evaluates against the current threat target only. Multi-enemy cover is too coupled to plan quality and would dominate the score in messy fights.
- **MOVE-Q2**: `spreadGoal` considers allies only in v1. Cover objects already factor into the candidate set via `CellBlocked`.
- **MOVE-Q3**: Bosses (`Tier == "boss"`) get `wSpread = 0.0` via a one-line override at the boss handler, not a new template field.
- **MOVE-Q4**: AoE-reactive movement is deferred. It crosses into reaction-economy territory (#244) and deserves its own design.
- **MOVE-Q5**: No pre-computation in v1. Profile first; revisit only if hotspots surface.

## Non-Goals Reaffirmed

Per spec §2.2:

- No A* / multi-cell pathfinding beyond per-cell stride.
- No multi-turn lookahead or learning.
- No faction coordination / squad maneuvers.
- No player-visible AI tuning UI.
- No HTN replacement.
- No reactive movement on enemy turns.
