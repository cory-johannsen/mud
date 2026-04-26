# Targeting System in Combat — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace name-string targeting with a UID-keyed system covering single-target, sticky selection, single-enemy auto-target, and authored AoE template placement (burst, cone, line). Feats and techs declare a `targeting:` block in YAML. Validation runs at selection time (preview only) and enqueue time (rejecting). Resolution-time stale-target handling stays as-is (existing "swings at empty air" behaviour). The line-of-fire validation stage exists as a no-op in v1, ready to activate when the visibility / LoS follow-up ticket lands.

**Spec:** [docs/superpowers/specs/2026-04-22-targeting-system-in-combat.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-22-targeting-system-in-combat.md)

**Architecture:** Combat already has a 2D grid (`GridX/GridY` on `Combatant`, `GridWidth/GridHeight` on `Combat`). The new `targeting.go` module adds `TargetCategory`, `TargetingError`, `AoEShape`, `AoETemplate`, and the two pipelines (`ValidateSingleTarget`, `ValidateAoE`). `QueuedAction` gains `TargetUID` (authoritative) and `AoETemplate` (full template); `TargetX/TargetY` stay as deprecated back-compat fields. A new per-session `CombatTargeting` struct (held server-side per player per combat) carries `CurrentTargetUID` and `PendingTemplate`. Telnet adds a `target` command, AoE placement commands (`<verb> at X,Y` for burst, `<verb> <dir> <length>` for cone/line, plus `preview` / `confirm` / `cancel`). Web client adds click-to-select highlight, AoE placement-mode overlays, and a target panel. `GridCell` and `LineCells` are reused from #247; if #247 has not landed, this plan defines them minimally and the two implementations converge.

**Tech Stack:** Go, `pgregory.net/rapid` (property tests), `internal/game/combat/`, `internal/gameserver/`, `internal/frontend/handlers/`, web client (React/TypeScript).

**Prerequisite:** None hard. #247 (Cover bonuses) is a soft dep — `GridCell` / `LineCells` are reused; if #247 is unmerged at start, define them in `combat/grid.go` here and reconcile at merge time.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/combat/targeting.go` |
| Create | `internal/game/combat/targeting_test.go` |
| Create | `internal/game/combat/aoe_template.go` |
| Create | `internal/game/combat/aoe_template_test.go` |
| Create | `internal/gameserver/combat_targeting.go` |
| Create | `internal/gameserver/combat_targeting_test.go` |
| Create | `internal/gameserver/target_session_test.go` |
| Create | `internal/frontend/handlers/target_command.go` |
| Create | `internal/frontend/handlers/target_command_test.go` |
| Create | `internal/frontend/handlers/aoe_placement.go` |
| Create | `internal/frontend/handlers/aoe_placement_test.go` |
| Modify | `internal/game/combat/action.go` (add `TargetUID`, `AoETemplate`) |
| Modify | `internal/game/combat/round.go` (resolver prefers `AoETemplate` over `TargetX/Y`) |
| Modify | `internal/game/world/feat.go` / `tech.go` (load `targeting:` block) |
| Modify | `internal/game/world/feat_test.go` / `tech_test.go` (load validation) |
| Modify | `internal/gameserver/session.go` (own per-player `CombatTargeting`) |
| Modify | `internal/frontend/handlers/attack_command.go` / `use_command.go` / `strike_command.go` (sticky vs override) |
| Modify | `cmd/webclient/ui/src/combat/TargetPanel.tsx` (new component) |
| Modify | `cmd/webclient/ui/src/combat/CombatMap.tsx` (click-to-select + AoE overlays) |
| Modify | `cmd/webclient/ui/src/combat/AoEPlacement.tsx` (new component) |
| Modify | `cmd/webclient/ui/src/proto/combat.ts` (new WS messages) |
| Modify | `proto/combat/v1/combat.proto` (`SetTargetRequest`, `AoEPlacement` messages) |
| Modify | `docs/architecture/combat.md` (new Targeting section) |

---

### Task 1: Core targeting types — `TargetCategory`, `TargetingError`, `AoEShape`, `CompassDir`

**Files:**
- Create: `internal/game/combat/targeting.go`
- Create: `internal/game/combat/targeting_test.go`

- [ ] **Step 1: Failing tests for type identity and zero-values**

```go
package combat_test

import (
    "testing"

    "github.com/cory-johannsen/gunchete/internal/game/combat"
)

func TestTargetCategory_KnownValues(t *testing.T) {
    cases := map[combat.TargetCategory]string{
        combat.TargetSelf:        "self",
        combat.TargetSingleAlly:  "single_ally",
        combat.TargetSingleEnemy: "single_enemy",
        combat.TargetSingleAny:   "single_any",
        combat.TargetAoEBurst:    "aoe_burst",
        combat.TargetAoECone:     "aoe_cone",
        combat.TargetAoELine:     "aoe_line",
    }
    for cat, s := range cases {
        if string(cat) != s {
            t.Fatalf("category %v: want %q got %q", cat, s, string(cat))
        }
    }
}

func TestTargetingError_OKIsZeroValue(t *testing.T) {
    var z combat.TargetingError
    if z != combat.TargetOK {
        t.Fatalf("zero TargetingError must equal TargetOK; got %v", z)
    }
}
```

- [ ] **Step 2: Implement type declarations** (TGT-2 enum membership) and ensure tests pass.

```go
type TargetCategory string

const (
    TargetSelf        TargetCategory = "self"
    TargetSingleAlly  TargetCategory = "single_ally"
    TargetSingleEnemy TargetCategory = "single_enemy"
    TargetSingleAny   TargetCategory = "single_any"
    TargetAoEBurst    TargetCategory = "aoe_burst"
    TargetAoECone     TargetCategory = "aoe_cone"
    TargetAoELine     TargetCategory = "aoe_line"
)

type TargetingError int

const (
    TargetOK TargetingError = iota
    ErrTargetMissing
    ErrTargetNotInCombat
    ErrTargetDead
    ErrOutOfRange
    ErrLineOfFireBlocked
    ErrWrongCategory
    ErrAoEShapeInvalid
)

type TargetingResult struct {
    Err    TargetingError
    Detail string
}

type AoEShape string

const (
    AoEBurst AoEShape = "burst"
    AoECone  AoEShape = "cone"
    AoELine  AoEShape = "line"
)

type CompassDir int

const (
    DirN CompassDir = iota
    DirNE
    DirE
    DirSE
    DirS
    DirSW
    DirW
    DirNW
)
```

- [ ] **Step 3:** `go test ./internal/game/combat/... -run Targeting`. All green.

---

### Task 2: `AoETemplate.Cells()` for burst, cone, line

**Files:**
- Create: `internal/game/combat/aoe_template.go`
- Create: `internal/game/combat/aoe_template_test.go`

- [ ] **Step 1: Failing property tests for `Cells()`**

```go
func TestProperty_Burst_ChebyshevRadius(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        radiusFt := rapid.IntRange(5, 30).Draw(t, "radius")
        cx := rapid.IntRange(2, 17).Draw(t, "cx")
        cy := rapid.IntRange(2, 17).Draw(t, "cy")
        tmpl := combat.AoETemplate{Shape: combat.AoEBurst, CenterX: cx, CenterY: cy, Radius: radiusFt}
        cells := tmpl.Cells()
        rad := radiusFt / 5
        for _, c := range cells {
            dx := abs(c.X - cx)
            dy := abs(c.Y - cy)
            if max(dx, dy) > rad {
                t.Fatalf("cell (%d,%d) outside chebyshev radius %d of (%d,%d)", c.X, c.Y, rad, cx, cy)
            }
        }
        // Coverage: count must be (2*rad+1)^2.
        if want := (2*rad + 1) * (2*rad + 1); len(cells) != want {
            t.Fatalf("burst cell count: want %d got %d", want, len(cells))
        }
    })
}

func TestProperty_Line_DirectionAndLength(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        dir := combat.CompassDir(rapid.IntRange(0, 7).Draw(t, "dir"))
        lengthFt := rapid.IntRange(5, 60).Draw(t, "len")
        tmpl := combat.AoETemplate{Shape: combat.AoELine, OriginX: 10, OriginY: 10, Direction: dir, Length: lengthFt}
        cells := tmpl.Cells()
        steps := lengthFt / 5
        if len(cells) > steps+1 {
            t.Fatalf("line cells exceeded length budget: got %d, max %d", len(cells), steps+1)
        }
    })
}

func TestCone_NorthCardinal_KnownPattern(t *testing.T) {
    tmpl := combat.AoETemplate{Shape: combat.AoECone, OriginX: 5, OriginY: 5, Direction: combat.DirN, Length: 15}
    cells := tmpl.Cells()
    // Expect a 90° square-fan cone of depth 3 (15 / 5).
    // Depth 1: 1 cell; depth 2: 3 cells; depth 3: 5 cells. Total 9.
    if len(cells) != 9 {
        t.Fatalf("north cone depth 3: want 9 cells got %d", len(cells))
    }
}
```

- [ ] **Step 2: Implement `Cells()`** for each shape:

```go
type AoETemplate struct {
    Shape   AoEShape
    CenterX, CenterY int
    Radius           int
    OriginX, OriginY int
    Direction        CompassDir
    Length           int
}

func (t AoETemplate) Cells() []GridCell {
    switch t.Shape {
    case AoEBurst:
        return burstCells(t.CenterX, t.CenterY, t.Radius/5)
    case AoECone:
        return coneCells(t.OriginX, t.OriginY, t.Direction, t.Length/5)
    case AoELine:
        return lineCells(t.OriginX, t.OriginY, t.Direction, t.Length/5)
    }
    return nil
}
```

- Burst: Chebyshev disc.
- Line: Bresenham step in direction, capped at `len`.
- Cone: PF2E square-fan — at depth `d`, `2d-1` cells perpendicular to facing, centered on the cone axis. Eight cardinal+ordinal directions handled via a small lookup of `(dx, dy)` pairs.

- [ ] **Step 3:** `go test ./internal/game/combat/... -run AoE` green. Hand-trace one example for each direction in cone tests.

---

### Task 3: `QueuedAction` extension — `TargetUID` and `AoETemplate`

**Files:**
- Modify: `internal/game/combat/action.go`
- Modify: `internal/game/combat/round.go`

- [ ] **Step 1:** Add fields, leaving `TargetX/TargetY` in place per TGT-19.

```go
type QueuedAction struct {
    Type        ActionType
    Target      string
    TargetUID   string       // NEW
    Direction   string
    WeaponID    string
    ExplosiveID string
    AbilityID   string
    AbilityCost int
    TargetX     int32        // deprecated
    TargetY     int32        // deprecated
    AoETemplate *AoETemplate // NEW
}
```

- [ ] **Step 2: Update resolver to prefer `AoETemplate` when present**, falling back to a single-cell burst built from `TargetX/Y` for legacy call sites. Test:

```go
func TestResolver_PrefersAoETemplateOverLegacyTargetXY(t *testing.T) {
    qa := combat.QueuedAction{
        TargetX: 1, TargetY: 1,
        AoETemplate: &combat.AoETemplate{Shape: combat.AoEBurst, CenterX: 5, CenterY: 5, Radius: 10},
    }
    cells := combat.ResolveTemplateCells(qa)
    if cells[0].X != 3 && cells[len(cells)-1].X != 7 { // sanity that template wins
        t.Fatalf("resolver did not honour template; cells=%v", cells)
    }
}
```

`ResolveTemplateCells` is a thin internal helper used by the resolver to pick the cell list deterministically.

- [ ] **Step 3:** Existing combat tests still pass.

---

### Task 4: `ValidateSingleTarget` pipeline

**Files:**
- Modify: `internal/game/combat/targeting.go`
- Modify: `internal/game/combat/targeting_test.go`

- [ ] **Step 1: Failing table tests** for each error stage in order.

```go
func TestValidateSingleTarget_StageOrder(t *testing.T) {
    cbt, actor, ally, enemy := setupTwoVTwo()
    cases := []struct {
        name        string
        targetUID   string
        category    combat.TargetCategory
        maxRangeFt  int
        wantErr     combat.TargetingError
    }{
        {"missing", "no-such", combat.TargetSingleEnemy, 30, combat.ErrTargetMissing},
        {"dead", deadEnemyUID(cbt), combat.TargetSingleEnemy, 30, combat.ErrTargetDead},
        {"wrong category — ally as enemy", ally.UID, combat.TargetSingleEnemy, 30, combat.ErrWrongCategory},
        {"out of range", farEnemyUID(cbt), combat.TargetSingleEnemy, 5, combat.ErrOutOfRange},
        {"self with maxRange 5 — accepts self distance 0", actor.UID, combat.TargetSelf, 5, combat.TargetOK},
        {"valid enemy", enemy.UID, combat.TargetSingleEnemy, 30, combat.TargetOK},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            res := combat.ValidateSingleTarget(cbt, actor, tc.targetUID, tc.category, tc.maxRangeFt, false)
            if res.Err != tc.wantErr {
                t.Fatalf("want %v got %v (%s)", tc.wantErr, res.Err, res.Detail)
            }
        })
    }
}
```

- [ ] **Step 2: Implement** the six-stage pipeline, short-circuiting on the first failure (TGT-6). Stage 6 (line-of-fire) returns `TargetOK` unconditionally in v1 (TGT-21).

- [ ] **Step 3:** Detail strings: `"out of range (30 ft, max 25 ft)"`, `"target is dead"`, etc. Cover in tests.

---

### Task 5: `ValidateAoE` pipeline

**Files:**
- Modify: `internal/game/combat/targeting.go`
- Modify: `internal/game/combat/targeting_test.go`

- [ ] **Step 1: Failing tests** for shape-consistency, anchor, params, origin range, grid bounds.

```go
func TestValidateAoE_AnchorMustEqualActorCell(t *testing.T) {
    cbt, actor := setupActor(5, 5)
    tmpl := combat.AoETemplate{Shape: combat.AoEBurst, CenterX: 7, CenterY: 7, Radius: 10}
    res := combat.ValidateAoE(cbt, actor, tmpl, combat.TargetAoEBurst, 30, true /* anchored */)
    if res.Err != combat.ErrAoEShapeInvalid {
        t.Fatalf("anchored mismatch must be ErrAoEShapeInvalid; got %v (%s)", res.Err, res.Detail)
    }
}

func TestValidateAoE_GridBoundsOnOrigin(t *testing.T) {
    cbt, actor := setupActor(5, 5)
    tmpl := combat.AoETemplate{Shape: combat.AoELine, OriginX: -1, OriginY: 5, Direction: combat.DirE, Length: 10}
    res := combat.ValidateAoE(cbt, actor, tmpl, combat.TargetAoELine, 30, false)
    if res.Err != combat.ErrAoEShapeInvalid {
        t.Fatalf("origin out of grid must be ErrAoEShapeInvalid")
    }
}
```

- [ ] **Step 2: Implement** the six-stage AoE pipeline matching spec §5.2, including the no-op LoF stage on origin.

- [ ] **Step 3:** Verify that out-of-grid template *cells* are still allowed (clipped at resolution, not rejected at validation).

---

### Task 6: Per-session `CombatTargeting` + auto-target rule

**Files:**
- Create: `internal/gameserver/combat_targeting.go`
- Create: `internal/gameserver/combat_targeting_test.go`
- Modify: `internal/gameserver/session.go`

- [ ] **Step 1: Failing scripted tests** for the auto-target rule across 0/1/2 enemy states (TGT-8, TGT-9), target death, target flee, `target clear`, second-enemy-arrival.

```go
func TestAutoTarget_SingleEnemy_Sticks(t *testing.T) {
    s := newSessionWithCombat(t, 1 /*enemies*/)
    s.OnCombatStart()
    if got := s.Targeting().CurrentTargetUID; got == "" {
        t.Fatal("auto-target did not populate with single enemy")
    }
    s.AddEnemy() // second enemy enters
    if got := s.Targeting().CurrentTargetUID; got != s.firstEnemyUID() {
        t.Fatal("second enemy must not override sticky target")
    }
}

func TestAutoTarget_TargetDeath_Reevaluates(t *testing.T) {
    s := newSessionWithCombat(t, 2)
    s.SetTarget(s.enemyAtIndex(0))
    s.KillEnemy(s.enemyAtIndex(0))
    if got := s.Targeting().CurrentTargetUID; got != s.enemyAtIndex(1).UID {
        t.Fatalf("after death with 1 enemy left, auto-target should pick survivor; got %q", got)
    }
}

func TestAutoTarget_ClearInSingleEnemyCombat_ImmediatelyReselects(t *testing.T) {
    s := newSessionWithCombat(t, 1)
    s.OnCombatStart()
    s.ClearTarget()
    if got := s.Targeting().CurrentTargetUID; got == "" {
        t.Fatal("clear in single-enemy combat must immediately re-auto-target")
    }
}
```

- [ ] **Step 2: Implement** `CombatTargeting` struct, lifecycle hooks (`OnCombatStart`, `OnTargetDeath`, `OnTargetFlee`, `OnTargetLeftCombat`, `OnCombatEnd`, `Clear`), and the auto-target re-evaluation that fires when `CurrentTargetUID` becomes empty mid-combat.

```go
type CombatTargeting struct {
    CurrentTargetUID string
    PendingTemplate  *combat.AoETemplate
    LastAoECoords    *combat.GridCell
}
```

- [ ] **Step 3:** Per-session-per-combat scope; never leaks across combats (TGT-20). Verify in test that two parallel sessions do not see each other's selections.

---

### Task 7: Telnet `target` command — list / set / clear / disambiguator

**Files:**
- Create: `internal/frontend/handlers/target_command.go`
- Create: `internal/frontend/handlers/target_command_test.go`

- [ ] **Step 1: Failing tests** covering:
  - `target` with no arg → list with current selection (single-enemy header `Auto-targeted: ...`, multi-enemy table) (TGT-10).
  - `target <name>` → exact / prefix / `#N` disambiguator (TGT-11).
  - `target <N>` → list-index resolution.
  - `target clear` → unsets, auto-target re-fires.
  - Invalid pick → reason surfaced inline (TGT-15).
  - Ambiguous prefix → list-of-options reply.

```go
func TestTargetCommand_AmbiguousNameListsOptions(t *testing.T) {
    h := newHandler(t, "Thug#1", "Thug#2")
    out := h.Run("target thug")
    require.Contains(t, out, "1) Thug#1")
    require.Contains(t, out, "2) Thug#2")
    require.NotContains(t, out, "Current target:")
}

func TestTargetCommand_SetByDisambiguator(t *testing.T) {
    h := newHandler(t, "Thug#1", "Thug#2")
    h.Run("target thug#2")
    require.Equal(t, h.Session.Targeting().CurrentTargetUID, h.UIDOf("Thug#2"))
}
```

- [ ] **Step 2: Implement** the command. Disambiguator generation is deterministic — duplicate display names sort by spawn order and get `#1`, `#2`, … suffixes (consistent with narrative output) (TGT-11).

- [ ] **Step 3: Wire** invalid-target reason printer using `TargetingResult.Detail`.

---

### Task 8: Telnet attack/strike/use — sticky vs override

**Files:**
- Modify: `internal/frontend/handlers/attack_command.go`
- Modify: `internal/frontend/handlers/strike_command.go`
- Modify: `internal/frontend/handlers/use_command.go`
- Modify: `internal/frontend/handlers/target_command_test.go` (add cross-tests)

- [ ] **Step 1: Failing tests** for:
  - `attack` with empty selection in multi-enemy combat → `Select a target first: target <name>` (TGT-12).
  - `attack <name>` → enqueues against `<name>`'s UID; sticky unchanged (TGT-12).
  - `attack` with sticky → enqueues against sticky UID.
  - `use <ability> on <name>` → override; sticky unchanged.

- [ ] **Step 2: Implement** sticky vs override resolution:
  1. Inline arg present → resolve to UID, validate, enqueue; **do not** mutate `CurrentTargetUID`.
  2. Inline arg absent → use `CurrentTargetUID`; if empty, error.

- [ ] **Step 3:** `QueuedAction.TargetUID` always populated on enqueue. `Target` (display name) populated for narrative.

---

### Task 9: Telnet AoE placement — `preview` / `confirm` / `cancel`

**Files:**
- Create: `internal/frontend/handlers/aoe_placement.go`
- Create: `internal/frontend/handlers/aoe_placement_test.go`

- [ ] **Step 1: Failing tests** for:
  - `throw grenade at 7,4` → builds `PendingTemplate` (burst at (7,4)).
  - `use overload cone north 30` → builds cone template (TGT-13).
  - `use laser_sweep line east 60` → builds line template.
  - `preview` → ASCII grid render with `@` actor, `*` AoE cells, marginal labels.
  - `preview` with allies in area → `Allies in area: X, Y` warning line.
  - `confirm` → enqueues `QueuedAction` with template.
  - `cancel` → drops `PendingTemplate`.
  - Out-of-range origin → preview shows error; confirm rejected (TGT-5 enqueue-time gate).

- [ ] **Step 2: Implement** parser + ASCII renderer + handler bindings. Reuse `AoETemplate.Cells()` from Task 2.

- [ ] **Step 3:** Ally warning: scan template cells for combatants whose `Kind` is allied to the actor; surface their disambiguated names. Never block confirm (TGT-17).

---

### Task 10: Feat / tech YAML loader — `targeting:` block

**Files:**
- Modify: `internal/game/world/feat.go`
- Modify: `internal/game/world/tech.go`
- Modify: `internal/game/world/feat_test.go`
- Modify: `internal/game/world/tech_test.go`

- [ ] **Step 1: Failing tests** for:
  - All seven categories load.
  - AoE category without matching shape param → load error (TGT-3).
  - `anchored_to_actor: true` on `aoe_cone` → load error (TGT-4).
  - Missing `targeting:` block on attack-style action → defaults to `single_enemy` with one-shot warn log (per spec §4.2).
  - `max_range: 0` interpreted as touch/self.

```go
func TestFeatLoad_AoEBurstWithoutRadius_Errors(t *testing.T) {
    _, err := world.LoadFeatFromYAML([]byte(`
id: bomb
targeting:
  category: aoe_burst
  max_range: 30
`))
    if err == nil || !strings.Contains(err.Error(), "burst_radius") {
        t.Fatalf("expected burst_radius load error; got %v", err)
    }
}
```

- [ ] **Step 2: Add `Targeting *TargetingSpec` field** to `FeatDef` / `TechDef`:

```go
type TargetingSpec struct {
    Category           combat.TargetCategory
    MaxRangeFt         int
    RequiresLineOfFire bool
    BurstRadius        int
    ConeLength         int
    LineLength         int
    AnchoredToActor    bool
}
```

- [ ] **Step 3:** Loader validates per the rules. Warn-once on default fallback per fallback type (use a `sync.Once` keyed by category).

---

### Task 11: Enqueue-time validation wiring

**Files:**
- Modify: `internal/gameserver/combat_targeting.go`
- Modify: `internal/frontend/handlers/attack_command.go` / `strike_command.go` / `use_command.go`

- [ ] **Step 1: Failing tests** for:
  - Selection-time validation: result is preview-only; no AP deducted; action not enqueued (TGT-5).
  - Enqueue-time re-validation: on failure, action dropped; AP NOT deducted; reason returned to caller.
  - Successful enqueue writes `TargetUID` (and `AoETemplate` if AoE).

- [ ] **Step 2: Implement** the two-phase validation hooks. Single phase function:

```go
func (ct *CombatTargeting) ResolveAndValidate(
    cbt *combat.Combat,
    actor *combat.Combatant,
    spec *world.TargetingSpec,
    inlineUID string, // empty → use sticky
) (resolvedUID string, template *combat.AoETemplate, res combat.TargetingResult)
```

- [ ] **Step 3:** Caller drops the action on `res.Err != TargetOK` and surfaces `res.Detail`.

---

### Task 12: Web — proto + `set_target` and `aoe_placement` WS messages

**Files:**
- Modify: `proto/combat/v1/combat.proto`
- Modify: `cmd/webclient/ui/src/proto/combat.ts`
- Modify: `internal/gameserver/grpc/combat_handlers.go`

- [ ] **Step 1:** Define proto messages:

```proto
message SetTargetRequest {
  string combat_id = 1;
  string target_uid = 2;  // empty → clear
}

message SetTargetResponse {
  bool   ok       = 1;
  uint32 err_code = 2;    // 0 = ok; matches TargetingError
  string detail   = 3;
}

message AoePlacement {
  string  combat_id = 1;
  string  shape     = 2;   // "burst" | "cone" | "line"
  int32   center_x  = 3;
  int32   center_y  = 4;
  int32   origin_x  = 5;
  int32   origin_y  = 6;
  int32   direction = 7;   // CompassDir
  int32   length    = 8;
  int32   radius    = 9;
  bool    confirmed = 10;
}
```

- [ ] **Step 2: Failing handler tests** in Go for each message kind. Successful selection updates `CombatTargeting`; failed selection returns the typed error code + detail.

- [ ] **Step 3:** TS bindings auto-generated from proto.

---

### Task 13: Web — `TargetPanel` + click-to-select

**Files:**
- Create: `cmd/webclient/ui/src/combat/TargetPanel.tsx`
- Create: `cmd/webclient/ui/src/combat/TargetPanel.test.tsx`
- Modify: `cmd/webclient/ui/src/combat/CombatMap.tsx`

- [ ] **Step 1: Failing component tests** for:
  - Hover valid combatant → green outline, distance + HP tooltip.
  - Hover invalid combatant → red outline, reason tooltip (TGT-15).
  - Click valid → `set_target` message dispatched; selected outline persists (TGT-14).
  - Click invalid → no message, transient toast with reason.
  - Auto-selected target → muted `auto-selected` subheader on panel.
  - Ability buttons disabled when no valid target / placement exists (§7.7).

- [ ] **Step 2: Implement** `TargetPanel` reading from `CombatTargeting` state in the combat slice. Hover state is local; click dispatches a thunk that issues the WS message and waits for the response before flipping the selected style.

- [ ] **Step 3:** Confirm AAA accessibility: text fallbacks for color-only highlights (§7.8).

---

### Task 14: Web — AoE placement overlays

**Files:**
- Create: `cmd/webclient/ui/src/combat/AoEPlacement.tsx`
- Create: `cmd/webclient/ui/src/combat/AoEPlacement.test.tsx`
- Modify: `cmd/webclient/ui/src/combat/CombatMap.tsx`

- [ ] **Step 1: Failing tests** for:
  - Burst: hover circle overlay; click confirms.
  - Cone: hover fan from actor toward hover direction; click confirms.
  - Line: hover line from actor to hover; click confirms.
  - Ally-in-area → confirm modal blocks until user accepts.
  - `Esc` cancels placement.

- [ ] **Step 2: Implement** placement-mode reducer. Direction snap for cone/line: nearest of eight compass directions to the cursor angle.

- [ ] **Step 3:** Reuse server-side `AoETemplate.Cells()` semantics on the client by reimplementing the trio (`burstCells`, `coneCells`, `lineCells`) in TS — verify equivalence via golden vectors generated from Go tests.

---

### Task 15: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Targeting" section** that:
  - Lists all 21 `TGT-N` requirements with a one-line description.
  - Documents the category taxonomy.
  - Documents the validation pipeline order (single + AoE) and the `TargetingError` codes.
  - Documents the auto-target rule.
  - Documents the AoE template geometry (burst Chebyshev, cone square-fan, line Bresenham).
  - Notes the deprecated `TargetX/TargetY` fields and the `AoETemplate` replacement.
  - Notes the no-op LoF stage and points at the visibility / LoS follow-up.

- [ ] **Step 2: Cross-link** to `proto/combat/v1/combat.proto`, `internal/game/combat/targeting.go`, and the YAML examples for feat/tech `targeting:` blocks.

- [ ] **Step 3:** Verify the doc renders correctly in GitHub markdown preview. No broken anchors.

---

## Verification

Per SWENG-6, the full test suite MUST pass before commit / PR:

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- Proto regeneration (`make proto`) runs cleanly with no diff after re-generation.
- Telnet smoke test: enter combat with a single enemy, verify `target` shows `Auto-targeted:`; kill enemy, verify selection cleared; spawn one new enemy, verify auto-target re-fires.
- Web smoke test: enter combat with two enemies, hover/click each, verify red/green outlines, verify ability buttons gate on selection. Trigger an AoE: verify placement-mode overlays for each shape.

---

## Rollout / Open Questions Resolved at Plan Time

- **`CombatTargeting` lives on the per-session struct, scoped per active combat** — when a player is in multiple combats sequentially, state resets on `OnCombatStart`. Cross-combat leaks blocked at type level (per-session map keyed by combat ID).
- **Cone variant: PF2E square-fan**, depth `Length / 5`, width `2d - 1` at depth `d`, axis along `Direction`. Confirmed in Task 2 unit tests.
- **`#N` disambiguator scope: combat-only** for v1; room listings keep their existing `look` rendering. Re-evaluate when narrative parity becomes an issue.
- **Proto: extend existing `combat.proto`** with the two new message types rather than introducing a new service.

## Non-Goals Reaffirmed

Per spec §10, this plan does NOT cover:

- Visibility / LoS / fog of war (follow-up).
- Smart-target keybindings beyond existing.
- Saved target preferences across combats.
- Per-target combat log filters.
- Pre-attack outcome previews.
- Dynamic `Kind`-switching for charmed combatants.
- Cross-room targeting.
- Predictive pathing for AoE placement.
- Alias resolvers (`last_attacker`, `nearest_enemy`, `lowest_hp`).
