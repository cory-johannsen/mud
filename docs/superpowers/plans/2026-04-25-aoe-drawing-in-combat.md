# AoE Drawing in Combat — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend AoE content from burst-only to burst / cone / line uniformly across content loaders, wire format, resolver, and both client UIs (telnet + web). Ship the placement / preview UX that today's bridge handler explicitly defers. Existing burst content keeps working unchanged.

**Spec:** [docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md)

**Architecture:** Today the combat code only understands burst — `combat.CombatantsInRadius` walks a Chebyshev square around a center cell. The plan introduces a `geometry` module with three pure helpers (`BurstCells`, `ConeCells`, `LineCells`), an `AoeTemplate` proto message, and an extension point (`PostFilterAffectedCells`) that #267 (visibility) will plug into without further resolver edits. The resolver replaces its inline `CombatantsInRadius` call with `affectedCells := geometry.CellsForTemplate(...) → PostFilterAffectedCells → CombatantsInCells`. Two new placement-mode state machines (one per client surface) drive the preview UX — telnet via `aim` / `face` / `confirm` / `cancel` commands, web via mouse-driven anchor / facing tracking. The web client mirrors geometry helpers for preview only; the server is authoritative and emits the resolved `cells` list on outbound combat events so client preview never drifts from server resolution.

**Tech Stack:** Go (`internal/game/combat/`, `internal/game/ruleset/`, `internal/game/technology/`, `internal/game/inventory/`, `internal/gameserver/`), `pgregory.net/rapid` for property tests, protobuf (`api/proto/game/v1/game.proto`), Bubble Tea telnet (`internal/frontend/telnet/`), React/TypeScript (`cmd/webclient/ui/src/`).

**Prerequisite:** None hard. #247 (Cover bonuses) is a soft dep — `Cell` / `GridCell` are reused. #249 (Targeting system) shares ideas but lands separately; the validation pipeline this plan adds composes with #249's enqueue-time validators if both are present.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/combat/geometry.go` |
| Create | `internal/game/combat/geometry_test.go` |
| Create | `internal/game/combat/testdata/rapid/TestProperty_BurstCellsBoundedChebyshev/` |
| Create | `internal/game/combat/testdata/rapid/TestProperty_ConeCellsAlignedToFacing/` |
| Create | `internal/game/combat/testdata/rapid/TestProperty_LineCellsThicknessAndLength/` |
| Modify | `internal/game/combat/aoe.go` (add `CombatantsInCells`; refactor `CombatantsInRadius` as wrapper) |
| Modify | `internal/game/combat/aoe_test.go` |
| Modify | `internal/game/ruleset/feat.go` (add `AoeShape`, `AoeLength`, `AoeWidth`) |
| Modify | `internal/game/ruleset/class_feature.go` |
| Modify | `internal/game/technology/model.go` |
| Modify | `internal/game/inventory/explosive.go` |
| Modify | `internal/game/ruleset/feat_test.go`, `class_feature_test.go`, etc. |
| Modify | `api/proto/game/v1/game.proto` (`AoeTemplate`, `UseRequest.template`) |
| Modify | `internal/gameserver/grpc_service.go` (resolver swap + `PostFilterAffectedCells`) |
| Modify | `internal/gameserver/grpc_service_aoe_test.go` (un-skip; cover burst/cone/line) |
| Create | `internal/frontend/telnet/aoe_placement.go` |
| Create | `internal/frontend/telnet/aoe_placement_test.go` |
| Modify | `internal/frontend/telnet/combat_grid.go` (overlay layer) |
| Modify | `internal/frontend/handlers/bridge_handlers.go` (drop deferred-UI marker; wire placement) |
| Create | `cmd/webclient/ui/src/game/aoe/AoePlacement.tsx` |
| Create | `cmd/webclient/ui/src/game/aoe/AoePlacement.test.tsx` |
| Create | `cmd/webclient/ui/src/game/aoe/geometry.ts` (mirror of Go helpers; preview only) |
| Create | `cmd/webclient/ui/src/game/aoe/geometry.test.ts` |
| Modify | `cmd/webclient/ui/src/game/panels/MapPanel.tsx` |
| Modify | `content/class_features.yaml` (explicit `aoe_shape: burst` on 9 entries) |
| Modify | one class feature, one tech, one explosive (cone/line exemplar — coordinate with user) |
| Modify | `docs/architecture/combat.md` (new AoE section) |

---

### Task 1: Geometry helpers — `BurstCells`, `ConeCells`, `LineCells`, `FacingFrom`

**Files:**
- Create: `internal/game/combat/geometry.go`
- Create: `internal/game/combat/geometry_test.go`
- Create: rapid testdata directories listed in the file map.

- [ ] **Step 1: Failing property tests** for each helper.

```go
package combat_test

import (
    "testing"

    "github.com/cory-johannsen/gunchete/internal/game/combat"
    "pgregory.net/rapid"
)

func TestProperty_BurstCellsBoundedChebyshev(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cx := rapid.IntRange(-50, 50).Draw(t, "cx")
        cy := rapid.IntRange(-50, 50).Draw(t, "cy")
        radiusFt := rapid.IntRange(5, 60).Draw(t, "radiusFt")
        cells := combat.BurstCells(combat.Cell{X: cx, Y: cy}, radiusFt)
        rad := radiusFt / 5
        for _, c := range cells {
            dx := abs(c.X - cx)
            dy := abs(c.Y - cy)
            if max(dx, dy) > rad {
                t.Fatalf("cell %+v outside Chebyshev radius %d of (%d,%d)", c, rad, cx, cy)
            }
        }
        if want := (2*rad + 1) * (2*rad + 1); len(cells) != want {
            t.Fatalf("burst count: want %d got %d", want, len(cells))
        }
    })
}

func TestProperty_ConeCellsAlignedToFacing(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        dir := combat.Direction(rapid.IntRange(0, 7).Draw(t, "dir"))
        lengthFt := rapid.IntRange(5, 60).Draw(t, "lenFt")
        apex := combat.Cell{X: 0, Y: 0}
        cells := combat.ConeCells(apex, dir, lengthFt)
        for _, c := range cells {
            if c == apex {
                t.Fatalf("apex must be excluded (AOE-10); got %+v", c)
            }
            // Within Chebyshev range
            d := max(abs(c.X), abs(c.Y))
            if d > lengthFt/5 {
                t.Fatalf("cone cell %+v exceeds cone length %d (cells)", c, lengthFt/5)
            }
        }
    })
}

func TestProperty_LineCellsThicknessAndLength(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        dir := combat.Direction(rapid.IntRange(0, 7).Draw(t, "dir"))
        lengthFt := rapid.IntRange(5, 50).Draw(t, "lenFt")
        widthFt := rapid.IntRange(5, 25).Draw(t, "widthFt")
        cells := combat.LineCells(combat.Cell{X: 0, Y: 0}, dir, lengthFt, widthFt)
        wantLen := (lengthFt / 5) * (widthFt / 5)
        if len(cells) > wantLen+1 {
            t.Fatalf("line cells exceeded length*width budget: got %d, max %d", len(cells), wantLen+1)
        }
    })
}
```

- [ ] **Step 2: Implement helpers**.

```go
type Cell struct{ X, Y int }

type Direction int

const (
    DirN Direction = iota
    DirNE
    DirE
    DirSE
    DirS
    DirSW
    DirW
    DirNW
)

func BurstCells(center Cell, radiusFt int) []Cell {
    rad := radiusFt / 5
    out := make([]Cell, 0, (2*rad+1)*(2*rad+1))
    for dx := -rad; dx <= rad; dx++ {
        for dy := -rad; dy <= rad; dy++ {
            out = append(out, Cell{X: center.X + dx, Y: center.Y + dy})
        }
    }
    return out
}

func ConeCells(apex Cell, facing Direction, lengthFt int) []Cell {
    depth := lengthFt / 5
    out := make([]Cell, 0)
    fdx, fdy := facingDelta(facing)
    for d := 1; d <= depth; d++ {
        for w := -d; w <= d; w++ {
            cx, cy := coneCellAt(apex, facing, d, w, fdx, fdy)
            out = append(out, Cell{X: cx, Y: cy})
        }
    }
    return out
}

func LineCells(origin Cell, facing Direction, lengthFt, widthFt int) []Cell {
    depth := lengthFt / 5
    halfW := (widthFt / 5) / 2
    out := make([]Cell, 0, depth*(2*halfW+1))
    for d := 1; d <= depth; d++ {
        for w := -halfW; w <= halfW; w++ {
            cx, cy := lineCellAt(origin, facing, d, w)
            out = append(out, Cell{X: cx, Y: cy})
        }
    }
    return out
}

func FacingFrom(from, to Cell) Direction {
    dx := to.X - from.X
    dy := to.Y - from.Y
    // Round to nearest octant; axial wins ties.
    return facingFromDelta(dx, dy)
}
```

`facingDelta`, `coneCellAt`, `lineCellAt`, `facingFromDelta` are private helpers; the cone variant uses the PF2E square-fan: at depth `d`, `2d+1` cells perpendicular to the facing axis. The line variant thickens perpendicular to the facing axis by `widthFt/5` total cells (centered).

- [ ] **Step 3:** `go test ./internal/game/combat/... -run Geometry` green. Hand-verify a single golden vector for each direction.

- [ ] **Step 4:** Confirm AOE-10 (apex/origin excluded) and AOE-11 (no cover/wall consultation) by inspection — these helpers take only `Cell`, `Direction`, and integer feet, so they cannot consult occlusion.

---

### Task 2: `CombatantsInCells` + retrofit `CombatantsInRadius`

**Files:**
- Modify: `internal/game/combat/aoe.go`
- Modify: `internal/game/combat/aoe_test.go`

- [ ] **Step 1: Failing tests** for `CombatantsInCells` returning only living combatants whose cell is in the input set; for `CombatantsInRadius` continuing to behave identically (regression).

```go
func TestCombatantsInCells_FiltersDeadAndOffSet(t *testing.T) {
    cbt := newCombatWithGrid(20, 20)
    a := newCombatantAt(cbt, "a", 5, 5, true /*alive*/)
    b := newCombatantAt(cbt, "b", 6, 6, false)
    c := newCombatantAt(cbt, "c", 9, 9, true)
    got := combat.CombatantsInCells(cbt, []combat.Cell{{X: 5, Y: 5}, {X: 6, Y: 6}})
    if len(got) != 1 || got[0] != a {
        t.Fatalf("want [a]; got %v", got)
    }
    _ = b; _ = c
}

func TestCombatantsInRadius_RegressionAfterRefactor(t *testing.T) {
    cbt := newCombatWithGrid(20, 20)
    newCombatantAt(cbt, "a", 5, 5, true)
    newCombatantAt(cbt, "b", 5, 6, true)
    got := combat.CombatantsInRadius(cbt, 5, 5, 10 /*ft*/)
    if len(got) != 2 {
        t.Fatalf("want 2 in burst@10ft from (5,5); got %d", len(got))
    }
}
```

- [ ] **Step 2: Implement**:

```go
func CombatantsInCells(cbt *Combat, cells []Cell) []*Combatant {
    inSet := make(map[Cell]struct{}, len(cells))
    for _, c := range cells {
        inSet[c] = struct{}{}
    }
    out := make([]*Combatant, 0)
    for _, c := range cbt.Combatants {
        if c.IsDead() {
            continue
        }
        if _, ok := inSet[Cell{X: c.GridX, Y: c.GridY}]; ok {
            out = append(out, c)
        }
    }
    return out
}

// CombatantsInRadius is retained as a thin wrapper for back-compat (AOE-19).
func CombatantsInRadius(cbt *Combat, cx, cy, radiusFt int) []*Combatant {
    return CombatantsInCells(cbt, BurstCells(Cell{X: cx, Y: cy}, radiusFt))
}
```

- [ ] **Step 3:** Run all existing combat tests; nothing should change behaviour.

---

### Task 3: Content model — `AoeShape`, `AoeLength`, `AoeWidth` on four types

**Files:**
- Modify: `internal/game/ruleset/feat.go` and `feat_test.go`
- Modify: `internal/game/ruleset/class_feature.go` and `class_feature_test.go`
- Modify: `internal/game/technology/model.go` and corresponding test
- Modify: `internal/game/inventory/explosive.go` and corresponding test

- [ ] **Step 1: Failing tests** for:
  - Loading a YAML with `aoe_shape: cone` + `aoe_length: 30` succeeds.
  - Loading `aoe_shape: cone` with no `aoe_length` → load error (AOE-5).
  - Loading `aoe_shape: line` with no `aoe_width` → defaults to 5 (AOE-3).
  - Loading `aoe_shape: burst` with `aoe_length` set → load error (AOE-6).
  - Loading legacy `aoe_radius: 10` (no `aoe_shape`) → treats as burst (AOE-4).

```go
func TestFeatLoad_DefaultShapeIsBurstWhenAoeRadiusPresent(t *testing.T) {
    f := mustLoadFeat(t, `
id: bomb
aoe_radius: 10
`)
    if f.AoeShape != ruleset.AoeShapeBurst {
        t.Fatalf("expected AoeShapeBurst default; got %v", f.AoeShape)
    }
}

func TestFeatLoad_ConeRequiresLength(t *testing.T) {
    _, err := loadFeat(`
id: cone_feat
aoe_shape: cone
`)
    if err == nil || !strings.Contains(err.Error(), "aoe_length") {
        t.Fatalf("expected aoe_length validation error; got %v", err)
    }
}
```

- [ ] **Step 2: Add fields and validation** to all four types. Centralise validation in a helper:

```go
// In internal/game/ruleset/aoe_validation.go (or shared package).
func ValidateAoeFields(shape AoeShape, radiusFt, lengthFt, widthFt int) error
```

- [ ] **Step 3:** All four types call the shared validator from their `Validate()` methods.

---

### Task 4: Wire format — `AoeTemplate` proto + `UseRequest.template`

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: generated Go bindings via `make proto`
- Modify: TS bindings under `cmd/webclient/ui/src/proto/`

- [ ] **Step 1: Add the message** (AOE-12).

```proto
message AoeTemplate {
  enum Shape {
    SHAPE_UNSPECIFIED = 0;
    SHAPE_BURST = 1;
    SHAPE_CONE = 2;
    SHAPE_LINE = 3;
  }
  enum Direction {
    DIR_UNSPECIFIED = 0;
    DIR_N  = 1;
    DIR_NE = 2;
    DIR_E  = 3;
    DIR_SE = 4;
    DIR_S  = 5;
    DIR_SW = 6;
    DIR_W  = 7;
    DIR_NW = 8;
  }
  message Cell { int32 x = 1; int32 y = 2; }

  Shape     shape    = 1;
  int32     anchor_x = 2;
  int32     anchor_y = 3;
  Direction facing   = 4;
  repeated Cell cells = 5;  // server-populated on outbound; ignored on inbound
}
```

- [ ] **Step 2: Add `template` to `UseRequest`** (AOE-13). Keep `target_x` / `target_y` as-is for back-compat (AOE-16).

- [ ] **Step 3: Failing handler tests**:
  - `UseRequest` for cone/line content with no `template` → `InvalidArgument` "AoE template required".
  - `UseRequest` for burst content with `target_x/y` and no `template` → resolves correctly (back-compat).
  - Outbound combat event includes the resolved `cells` list (AOE-12).

- [ ] **Step 4: Implement** the validator hook in the gRPC entry point and ensure the resolver reads `template` first, falls back to `target_x/y` for burst.

---

### Task 5: Resolver swap — `CellsForTemplate` + `PostFilterAffectedCells`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_aoe_test.go` (un-skip; cover all three shapes)

- [ ] **Step 1: Add the helpers** to a new internal file (e.g. `internal/gameserver/aoe_resolver.go`):

```go
func CellsForTemplate(t *gamev1.AoeTemplate, content AoeContent) []combat.Cell {
    switch t.Shape {
    case gamev1.AoeTemplate_SHAPE_BURST:
        return combat.BurstCells(combat.Cell{X: int(t.AnchorX), Y: int(t.AnchorY)}, content.AoeRadius())
    case gamev1.AoeTemplate_SHAPE_CONE:
        return combat.ConeCells(combat.Cell{X: int(t.AnchorX), Y: int(t.AnchorY)}, dirFromProto(t.Facing), content.AoeLength())
    case gamev1.AoeTemplate_SHAPE_LINE:
        return combat.LineCells(combat.Cell{X: int(t.AnchorX), Y: int(t.AnchorY)}, dirFromProto(t.Facing), content.AoeLength(), content.AoeWidth())
    }
    return nil
}

// PostFilterAffectedCells is the seam #267 (visibility) plugs into.
// v1 implementation: identity (AOE-21).
func PostFilterAffectedCells(cells []combat.Cell, ctx ResolveContext) []combat.Cell {
    return cells
}
```

`AoeContent` is a small interface satisfied by `FeatDef`, `ClassFeatureDef`, `TechnologyDef`, and `Explosive` exposing `AoeRadius()`, `AoeLength()`, `AoeWidth()`.

- [ ] **Step 2: Replace the inline `CombatantsInRadius` call** at `grpc_service.go:8573` (REQ-AOE-2) with:

```go
cells := CellsForTemplate(template, content)
cells = PostFilterAffectedCells(cells, ctx)
affected := combat.CombatantsInCells(cbt, cells)
// existing effect application loop
```

- [ ] **Step 3: Un-skip `grpc_service_aoe_test.go`** and add coverage for one cone, one line, and one burst path. Verify outbound event carries the populated `cells` list.

- [ ] **Step 4:** Confirm AOE-20 — empty intersection still consumes AP and emits a "hit nothing" narrative line. Add a scripted test.

---

### Task 6: Telnet placement mode — `aim` / `face` / `confirm` / `cancel`

**Files:**
- Create: `internal/frontend/telnet/aoe_placement.go`
- Create: `internal/frontend/telnet/aoe_placement_test.go`
- Modify: `internal/frontend/telnet/combat_grid.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Failing tests** for:
  - Invoking a cone-shaped tech enters placement mode (suppresses normal input — AOE-22).
  - `aim 7 4` updates anchor; subsequent render shows `+` overlay on affected cells (AOE-23).
  - `face ne` updates facing for cone/line; ignored for burst.
  - `confirm` dispatches the gRPC request with the current template.
  - `cancel` exits placement; no AP charged.
  - Burst content with legacy `target_x`/`target_y` macro skips placement mode (AOE-26).
  - 60s of inactivity auto-`cancel`s (AOE-27).
  - Legend line shows `[AoE: cone (9 cells, 2 targets)] aim/face/confirm/cancel` (AOE-25).
  - During placement, combat-affecting commands (`move`, `attack`) are rejected with the "you are placing an area" message (AOE-Q4).

- [ ] **Step 2: Implement** the placement-mode struct held on the per-session combat state:

```go
type AoePlacement struct {
    Shape    AoeShape
    Anchor   combat.Cell
    Facing   combat.Direction
    Active   bool
    StartedAt time.Time
}
```

- The bridge handler that previously deferred placement (`bridge_handlers.go:900`) now activates `AoePlacement` for non-burst content and lets the player's next input land in `aoe_placement.go`'s parser.
- The 60s timeout is enforced by a per-session timer reset on each placement input; on fire, emits `cancel`.

- [ ] **Step 3: Render overlay** in `combat_grid.go`: while placement is active, the cell renderer overlays `+` glyphs on `geometry.CellsForTemplate(...)`. Combatants in those cells render in inverse video. Render does NOT mutate combat state (AOE-23).

---

### Task 7: Web placement mode — `AoePlacement` component + map integration

**Files:**
- Create: `cmd/webclient/ui/src/game/aoe/AoePlacement.tsx`
- Create: `cmd/webclient/ui/src/game/aoe/AoePlacement.test.tsx`
- Create: `cmd/webclient/ui/src/game/aoe/geometry.ts`
- Create: `cmd/webclient/ui/src/game/aoe/geometry.test.ts`
- Modify: `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

- [ ] **Step 1: Failing tests** for:
  - Clicking an AoE-bearing action button enters placement state (AOE-28).
  - Mouse motion updates anchor (burst/line) or facing (cone via `FacingFrom`) (AOE-29).
  - Cell highlight is 30% opacity, tinted by shape (red / amber / blue) (AOE-29).
  - Confirm button + Enter key submit the template (AOE-30).
  - X button + Esc cancel without submitting (AOE-30).
  - Triple-click on a combatant for a burst-only action skips placement (AOE-32).
  - Geometry mirror produces identical cell sets to a fixed Go-generated golden vector for `burst@2`, `cone@30ft N`, `line@25ftx5ft E`.

- [ ] **Step 2: Implement** the geometry mirror in TypeScript. Generate golden vectors by adding a small Go helper in step 1 above (e.g. `internal/game/combat/geometry_golden_test.go` writing fixtures to `cmd/webclient/ui/src/game/aoe/__golden__/*.json`).

```ts
export function burstCells(center: Cell, radiusFt: number): Cell[];
export function coneCells(apex: Cell, facing: Direction, lengthFt: number): Cell[];
export function lineCells(origin: Cell, facing: Direction, lengthFt: number, widthFt: number): Cell[];
export function facingFrom(from: Cell, to: Cell): Direction;
```

- [ ] **Step 3: Integrate** with `MapPanel.tsx`:

```ts
const [aoePlacement, setAoePlacement] = useState<AoePlacement | null>(null);
// when an AoE action button is clicked → setAoePlacement({ shape, anchor: caster, facing: DirN, cells: ... });
// while aoePlacement !== null, mouse motion updates anchor or facing;
// confirm → dispatch UseRequest with template; clear placement;
// Esc / cancel → clear placement.
```

The triggering action button stays visibly "armed" (CSS class) until placement resolves (AOE-30).

- [ ] **Step 4:** Tests pass; manual smoke check confirms preview tracks server resolution exactly for the three sample shapes.

---

### Task 8: Content migration — explicit `aoe_shape: burst` on existing entries + cone/line exemplars

**Files:**
- Modify: `content/class_features.yaml` (9 existing entries)
- Modify: one class feature, one technology, one explosive (cone/line — coordinate with user)
- Modify: `internal/gameserver/grpc_service_aoe_test.go` (un-skip; covers exemplars)

- [ ] **Step 1: Add `aoe_shape: burst`** to the nine existing entries at `content/class_features.yaml` lines 77, 93, 380, 663, 725, 754, 850, 953, 969 (AOE-33). Semantics unchanged by AOE-4; this is documentation clarity.

- [ ] **Step 2: Coordinate with user** on which class feature, tech, and explosive to migrate to cone or line (AOE-34). Suggested defaults pending user input:
  - Class feature: a cone-style breath / sweep ability if one exists.
  - Tech: a directional emission (e.g. an arc weapon) — line.
  - Explosive: keep as burst per AOE-Q1 recommendation.

- [ ] **Step 3:** Re-enable `grpc_service_aoe_test.go` and add coverage for the three migrated content items end-to-end (AOE-35).

- [ ] **Step 4:** Targeted regression — the nine pre-existing burst features resolve identically before / after the migration. Add a small fixture-driven test that loads each, runs `CellsForTemplate`, and asserts the result matches a golden cell set generated from `CombatantsInRadius` against a fixed combat state.

---

### Task 9: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a new "Area-of-Effect" section** documenting:
  - All 35 `AOE-N` requirements with one-line summaries.
  - The `AoeTemplate` proto message and the back-compat behaviour for `target_x` / `target_y`.
  - The geometry helpers and where they live.
  - The resolver pipeline: `CellsForTemplate → PostFilterAffectedCells → CombatantsInCells → effect application`.
  - The `PostFilterAffectedCells` extension point and its planned use by #267 (visibility).
  - The placement-mode UX contracts on telnet and web.
  - The content migration policy (AOE-33 / AOE-34) and the validation rules (AOE-5 / AOE-6).
  - Open question resolutions (cone variant chosen, explosives stay burst-only in v1).

- [ ] **Step 2: Cross-link** to the spec, to `internal/game/combat/geometry.go`, to the proto, and to the placement UX components.

- [ ] **Step 3: Verify** the doc renders correctly in GitHub markdown preview. No broken anchors.

---

## Verification

Per SWENG-6, the full test suite MUST pass before commit / PR:

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly with no diff (no drift from the proto edit).
- Telnet smoke test: pick a cone-shape exemplar tech; verify placement mode activates, `aim` and `face` update the overlay, `confirm` resolves with the expected combatants, `cancel` exits with no AP charge, idle 60s auto-cancels.
- Web smoke test: pick a line exemplar; verify mouse-driven anchor/facing tracking, the three colour tints, Enter/Esc bindings, the confirm-while-armed visual.

---

## Rollout / Open Questions Resolved at Plan Time

- **AOE-Q1**: Explosives stay burst-only in v1. The migration step adds `aoe_shape: burst` to existing explosive content for clarity but introduces no cone/line explosives.
- **AOE-Q2**: Cone is the 90° octant wedge — square-fan on the Chebyshev grid. Avoids fractional-angle math; matches our 8-direction facing model.
- **AOE-Q3**: Explosives keep going through the legacy `target_x` / `target_y` path until a content reason forces migration. The `template` field is accepted on the throw RPC for forward compatibility but defaults to a synthetic burst built from `target_x` / `target_y` when omitted.
- **AOE-Q4**: During telnet placement mode, chat commands pass through; combat-affecting commands (`move`, `use`, `attack`, `strike`) are rejected with `you are placing an area; confirm or cancel first`.

## Non-Goals Reaffirmed

Per spec §2.2, this plan does NOT cover:

- Visibility / LoS / occlusion for AoE cells (#267; wired via `PostFilterAffectedCells` no-op).
- Friendly-fire toggles, exclusion masks, or "save for half" mechanics.
- Multi-stage / over-time templates.
- Authoring tools for new AoE content.
- Diagonal-cost geometry (Chebyshev preserved).
- Animated AoE previews on the web client.
