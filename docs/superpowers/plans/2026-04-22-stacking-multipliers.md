# Stacking Multipliers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the ad-hoc crit-×2 / weakness / resistance arithmetic scattered across `round.go` with a named-stage damage pipeline (`ResolveDamage`) that correctly combines multipliers per PF2E rules (extras become +1, not multiplicative), and emits a stage breakdown in combat narratives.

**Architecture:** New `internal/game/combat/damage.go` owns `DamageInput`, `DamageResult`, and `ResolveDamage` (pure). New `internal/game/combat/damage_input.go` owns `buildDamageInput` which assembles the input from call-site data. Five `EffectiveDamage()` call sites in `round.go` migrate to `buildDamageInput + ResolveDamage`. `TechEffect.Multiplier` is validated at load time. A `ShowDamageBreakdown bool` field is added to `PlayerSession` for opt-in verbose output.

**Tech Stack:** Go, `pgregory.net/rapid` (property tests), existing proto fields for breakdown text transport.

---

## File Map

**New files:**
- `internal/game/combat/damage.go` — `DamageInput`, `DamageResult`, `DamageStage`, `ResolveDamage`
- `internal/game/combat/damage_test.go` — property-based tests
- `internal/game/combat/damage_scenario_test.go` — PF2E regression anchors
- `internal/game/combat/damage_input.go` — `buildDamageInput` helper
- `internal/game/combat/round_damage_pipeline_test.go` — end-to-end pipeline tests
- `internal/game/combat/round_breakdown_narrative_test.go` — narrative / width tests
- `internal/game/technology/tech_multiplier_load_test.go` — multiplier validation tests

**Modified files:**
- `internal/game/combat/resolver.go` — deprecate `EffectiveDamage()` with comment
- `internal/game/combat/round.go` — migrate 5 EffectiveDamage call sites + explosive site; remove open-coded crit, weakness, resistance; add breakdown to narratives
- `internal/game/technology/model.go` — validate `TechEffect.Multiplier` in `Validate()`
- `internal/game/session/manager.go` — add `ShowDamageBreakdown bool` field to `PlayerSession`
- `internal/frontend/handlers/text_renderer.go` — `RenderDamageBreakdown()` helper
- `docs/architecture/combat.md` — "Damage pipeline" section

---

## Task 1: `damage.go` — core types and `ResolveDamage`

**Files:**
- Create: `internal/game/combat/damage.go`
- Create: `internal/game/combat/damage_test.go`

- [ ] **Step 1: Write the failing property-based tests**

```go
// internal/game/combat/damage_test.go
package combat_test

import (
    "fmt"
    "math"
    "testing"

    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/combat"
)

func TestResolveDamage_BaseOnly(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 8, Source: "attack"}},
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 8, r.Final)
    require.Len(t, r.Breakdown, 1)
    assert.Equal(t, combat.StageBase, r.Breakdown[0].Stage)
}

func TestResolveDamage_NegativeAdditivesClamped(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{
            {Label: "dice", Value: 5, Source: "attack"},
            {Label: "penalty", Value: -10, Source: "condition:weakened"},
        },
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 0, r.Final) // floored at 0
}

func TestResolveDamage_CritMultiplier(t *testing.T) {
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "dice", Value: 7, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "critical hit", Factor: 2.0, Source: "engine:crit"}},
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 14, r.Final)
}

func TestResolveDamage_TwoMultipliers_NotChained(t *testing.T) {
    // PF2E: ×2 + ×2 = ×3 (not ×4). MULT-2.
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{
            {Label: "critical hit", Factor: 2.0, Source: "engine:crit"},
            {Label: "vulnerable", Factor: 2.0, Source: "condition:vulnerable"},
        },
    }
    r := combat.ResolveDamage(in)
    // effective = 1 + (2-1) + (2-1) = 3
    assert.Equal(t, 30, r.Final)
}

func TestResolveDamage_ThreeMultipliers(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{
            {Label: "crit", Factor: 2.0, Source: "engine:crit"},
            {Label: "vuln", Factor: 2.0, Source: "cond:vuln"},
            {Label: "extra", Factor: 2.0, Source: "tech:amp"},
        },
    }
    r := combat.ResolveDamage(in)
    // effective = 1 + 1 + 1 + 1 = 4
    assert.Equal(t, 40, r.Final)
}

func TestResolveDamage_Halver(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
        Halvers:   []combat.DamageHalver{{Label: "basic save success", Source: "tech:fireball"}},
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 5, r.Final) // floor(10/2)
}

func TestResolveDamage_HalverIdempotent(t *testing.T) {
    // Multiple halvers = same as one. MULT-3.
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
        Halvers: []combat.DamageHalver{
            {Label: "save", Source: "tech:a"},
            {Label: "evasion", Source: "feat:b"},
        },
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 5, r.Final) // still just floor(10/2)
}

func TestResolveDamage_CritPlusHalver_NetOne(t *testing.T) {
    // PF2E: crit (×2) + basic save success (÷2) = net ×1. MULT-4.
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
        Halvers:     []combat.DamageHalver{{Label: "save", Source: "tech:fireball"}},
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 10, r.Final) // 10 × 2 = 20, then 20 / 2 = 10
}

func TestResolveDamage_Weakness(t *testing.T) {
    in := combat.DamageInput{
        Additives:  []combat.DamageAdditive{{Label: "dice", Value: 12, Source: "attack"}},
        DamageType: "fire",
        Weakness:   3,
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 15, r.Final) // 12 + 3
}

func TestResolveDamage_Resistance(t *testing.T) {
    in := combat.DamageInput{
        Additives:  []combat.DamageAdditive{{Label: "dice", Value: 12, Source: "attack"}},
        DamageType: "physical",
        Resistance: 5,
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 7, r.Final) // 12 - 5
}

func TestResolveDamage_ResistanceGreaterThanDamage_FloorsToZero(t *testing.T) {
    in := combat.DamageInput{
        Additives:  []combat.DamageAdditive{{Label: "dice", Value: 3, Source: "attack"}},
        DamageType: "physical",
        Resistance: 10,
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 0, r.Final)
}

func TestResolveDamage_FinalAlwaysNonNegative(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{
            {Label: "dice", Value: 1, Source: "attack"},
            {Label: "penalty", Value: -100, Source: "debuff"},
        },
    }
    r := combat.ResolveDamage(in)
    assert.GreaterOrEqual(t, r.Final, 0)
}

func TestResolveDamage_Pure(t *testing.T) {
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "dice", Value: 7, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
        Weakness:    2,
        Resistance:  1,
    }
    r1 := combat.ResolveDamage(in)
    r2 := combat.ResolveDamage(in)
    assert.Equal(t, r1.Final, r2.Final) // MULT-11
}

func TestResolveDamage_BreakdownContainsBase(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 5, Source: "attack"}},
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, combat.StageBase, r.Breakdown[0].Stage) // MULT-13
}

func TestResolveDamage_BreakdownOrder_MultiplierBeforeHalver(t *testing.T) {
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
        Halvers:     []combat.DamageHalver{{Label: "save", Source: "tech:a"}},
    }
    r := combat.ResolveDamage(in)
    stages := make([]combat.DamageStage, len(r.Breakdown))
    for i, s := range r.Breakdown {
        stages[i] = s.Stage
    }
    // Must appear in order: base, multiplier, halver
    multIdx, halvIdx := -1, -1
    for i, s := range stages {
        if s == combat.StageMultiplier { multIdx = i }
        if s == combat.StageHalver { halvIdx = i }
    }
    require.Greater(t, multIdx, -1)
    require.Greater(t, halvIdx, -1)
    assert.Less(t, multIdx, halvIdx) // MULT-1
}

func TestProperty_ResolveDamage_MultiplierCombination(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        base := rapid.IntRange(1, 100).Draw(rt, "base").(int)
        n := rapid.IntRange(1, 4).Draw(rt, "n").(int)
        var mults []combat.DamageMultiplier
        sumExtra := 0.0
        for i := 0; i < n; i++ {
            factor := rapid.Float64Range(1.01, 4.0).Draw(rt, fmt.Sprintf("factor%d", i)).(float64)
            mults = append(mults, combat.DamageMultiplier{Label: "m", Factor: factor, Source: "test"})
            sumExtra += factor - 1.0
        }
        in := combat.DamageInput{
            Additives:   []combat.DamageAdditive{{Label: "dice", Value: base, Source: "test"}},
            Multipliers: mults,
        }
        r := combat.ResolveDamage(in)
        effective := 1.0 + sumExtra
        expected := int(math.Floor(float64(base) * effective))
        assert.Equal(rt, expected, r.Final)
    })
}

func TestProperty_ResolveDamage_FinalNonNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        base := rapid.Int().Draw(rt, "base").(int)
        resist := rapid.IntRange(0, 200).Draw(rt, "resist").(int)
        in := combat.DamageInput{
            Additives:  []combat.DamageAdditive{{Label: "dice", Value: base, Source: "test"}},
            Resistance: resist,
        }
        r := combat.ResolveDamage(in)
        assert.GreaterOrEqual(rt, r.Final, 0)
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestResolveDamage|TestProperty_ResolveDamage" 2>&1 | head -20
```
Expected: `undefined: combat.DamageInput`.

- [ ] **Step 3: Write `damage.go`**

```go
// internal/game/combat/damage.go
package combat

import "math"

// DamageStage identifies a stage in the damage resolution pipeline (MULT-1).
type DamageStage string

const (
    StageBase       DamageStage = "base"
    StageMultiplier DamageStage = "multiplier"
    StageHalver     DamageStage = "halver"
    StageWeakness   DamageStage = "weakness"
    StageResistance DamageStage = "resistance"
    StageFloor      DamageStage = "floor"
)

// DamageAdditive is a flat contributor summed into the base damage.
// Value may be negative (e.g. a condition penalty).
type DamageAdditive struct {
    Label  string
    Value  int
    Source string
}

// DamageMultiplier is a ×Factor source. Factor must be > 1.0 (MULT-8).
type DamageMultiplier struct {
    Label  string
    Factor float64
    Source string
}

// DamageHalver is a boolean flag; any present triggers one halving after multipliers (MULT-3).
type DamageHalver struct {
    Label  string
    Source string
}

// DamageInput holds everything ResolveDamage needs — no RNG, no state.
type DamageInput struct {
    Additives   []DamageAdditive
    Multipliers []DamageMultiplier
    Halvers     []DamageHalver
    DamageType  string // may be empty
    Weakness    int    // pre-resolved flat weakness for DamageType; 0 if none
    Resistance  int    // pre-resolved flat resistance for DamageType; 0 if none
}

// DamageBreakdownStep records one stage's contribution to the final result.
type DamageBreakdownStep struct {
    Stage   DamageStage
    Before  int
    Delta   int    // signed change produced by this stage
    After   int
    Detail  string
    Sources []string
}

// DamageResult is the output of ResolveDamage.
type DamageResult struct {
    Final     int
    Breakdown []DamageBreakdownStep
}

// ResolveDamage runs the six-stage damage pipeline and returns the final damage
// and an ordered breakdown.
//
// Precondition: every in.Multipliers[i].Factor > 1.0.
// Postcondition: Final >= 0; Breakdown[0].Stage == StageBase (MULT-13).
// Pure: no RNG, no state mutation, no I/O (MULT-11).
func ResolveDamage(in DamageInput) DamageResult {
    var steps []DamageBreakdownStep

    // Stage 1: Base accumulation.
    base := 0
    var addSources []string
    for _, a := range in.Additives {
        base += a.Value
        addSources = append(addSources, a.Source)
    }
    steps = append(steps, DamageBreakdownStep{
        Stage:   StageBase,
        Before:  0,
        Delta:   base,
        After:   base,
        Detail:  "flat additives",
        Sources: addSources,
    })
    cur := base

    // Stage 2: Multiplier combination (MULT-2).
    if len(in.Multipliers) > 0 {
        sumExtra := 0.0
        var mSources []string
        for _, m := range in.Multipliers {
            sumExtra += m.Factor - 1.0
            mSources = append(mSources, m.Source)
        }
        effective := 1.0 + sumExtra
        afterMult := int(math.Floor(float64(cur) * effective))
        steps = append(steps, DamageBreakdownStep{
            Stage:   StageMultiplier,
            Before:  cur,
            Delta:   afterMult - cur,
            After:   afterMult,
            Detail:  fmt.Sprintf("×%.0f effective (from %d source(s))", effective, len(in.Multipliers)),
            Sources: mSources,
        })
        cur = afterMult
    }

    // Stage 3: Halver application (MULT-3, MULT-4).
    if len(in.Halvers) > 0 {
        afterHalve := int(math.Floor(float64(cur) / 2))
        var hSources []string
        for _, h := range in.Halvers {
            hSources = append(hSources, h.Source)
        }
        steps = append(steps, DamageBreakdownStep{
            Stage:   StageHalver,
            Before:  cur,
            Delta:   afterHalve - cur,
            After:   afterHalve,
            Detail:  in.Halvers[0].Label, // first halver label for display
            Sources: hSources,
        })
        cur = afterHalve
    }

    // Stage 4: Weakness (MULT-5).
    if in.Weakness > 0 {
        afterWeak := cur + in.Weakness
        steps = append(steps, DamageBreakdownStep{
            Stage:   StageWeakness,
            Before:  cur,
            Delta:   in.Weakness,
            After:   afterWeak,
            Detail:  fmt.Sprintf("+%d (%s weakness)", in.Weakness, in.DamageType),
            Sources: []string{"target:weakness"},
        })
        cur = afterWeak
    }

    // Stage 5: Resistance (MULT-6).
    if in.Resistance > 0 {
        afterRes := cur - in.Resistance
        steps = append(steps, DamageBreakdownStep{
            Stage:   StageResistance,
            Before:  cur,
            Delta:   -in.Resistance,
            After:   afterRes,
            Detail:  fmt.Sprintf("-%d (%s resistance)", in.Resistance, in.DamageType),
            Sources: []string{"target:resistance"},
        })
        cur = afterRes
    }

    // Stage 6: Floor at 0 (MULT-7).
    if cur < 0 {
        steps = append(steps, DamageBreakdownStep{
            Stage:  StageFloor,
            Before: cur,
            Delta:  -cur,
            After:  0,
            Detail: "floored at 0",
        })
        cur = 0
    }

    return DamageResult{Final: cur, Breakdown: steps}
}
```

Note: add `"fmt"` import to `damage.go`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestResolveDamage|TestProperty_ResolveDamage" -v 2>&1 | tail -30
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/damage.go internal/game/combat/damage_test.go && git commit -m "feat(damage): ResolveDamage pipeline — named stages, PF2E multiplier stacking (#246)"
```

---

## Task 2: PF2E scenario regression anchors

**Files:**
- Create: `internal/game/combat/damage_scenario_test.go`

- [ ] **Step 1: Write and run the scenario tests**

```go
// internal/game/combat/damage_scenario_test.go
package combat_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/combat"
)

// Scenario: crit alone = ×2.
func TestScenario_CritAlone(t *testing.T) {
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "1d8", Value: 5, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "critical hit", Factor: 2.0, Source: "engine:crit"}},
    }
    assert.Equal(t, 10, combat.ResolveDamage(in).Final)
}

// Scenario: crit + vulnerable (both ×2) = ×3. PF2E core.
func TestScenario_CritPlusVulnerable(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "1d8", Value: 5, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{
            {Label: "critical hit", Factor: 2.0, Source: "engine:crit"},
            {Label: "vulnerable fire", Factor: 2.0, Source: "condition:vulnerable"},
        },
    }
    assert.Equal(t, 15, combat.ResolveDamage(in).Final) // 5 × 3
}

// Scenario: crit + vulnerable + hypothetical third ×2 = ×4.
func TestScenario_ThreeDoubling(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "1d6", Value: 4, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{
            {Label: "crit", Factor: 2.0, Source: "engine:crit"},
            {Label: "vuln", Factor: 2.0, Source: "cond:vuln"},
            {Label: "amp", Factor: 2.0, Source: "tech:amp"},
        },
    }
    assert.Equal(t, 16, combat.ResolveDamage(in).Final) // 4 × 4
}

// Scenario: crit + basic save success halver = net ×1.
func TestScenario_CritPlusHalver(t *testing.T) {
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "dice", Value: 8, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
        Halvers:     []combat.DamageHalver{{Label: "basic save success", Source: "tech:fireball"}},
    }
    assert.Equal(t, 8, combat.ResolveDamage(in).Final) // 8×2=16, 16÷2=8
}

// Scenario: weakness 5 + resistance 10 + base 12 = 7. Per spec §7.
func TestScenario_WeaknessAndResistance(t *testing.T) {
    in := combat.DamageInput{
        Additives:  []combat.DamageAdditive{{Label: "dice", Value: 12, Source: "attack"}},
        DamageType: "fire",
        Weakness:   5,
        Resistance: 10,
    }
    // 12 + 5 (weakness) - 10 (resistance) = 7
    assert.Equal(t, 7, combat.ResolveDamage(in).Final)
}

// Scenario: resistance greater than total — floor at 0.
func TestScenario_ResistanceGreaterThanDamage(t *testing.T) {
    in := combat.DamageInput{
        Additives:  []combat.DamageAdditive{{Label: "dice", Value: 3, Source: "attack"}},
        DamageType: "fire",
        Resistance: 10,
    }
    assert.Equal(t, 0, combat.ResolveDamage(in).Final)
}

// Scenario: empty multipliers / halvers — base passthrough.
func TestScenario_EmptyModifiers(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 6, Source: "attack"}},
    }
    r := combat.ResolveDamage(in)
    assert.Equal(t, 6, r.Final)
    assert.Len(t, r.Breakdown, 1) // only StageBase
    assert.Equal(t, combat.StageBase, r.Breakdown[0].Stage)
}

// Scenario: breakdown length for non-trivial inputs — more than 1 stage.
func TestScenario_NonTrivialBreakdownLength(t *testing.T) {
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "dice", Value: 5, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
        Weakness:    3,
    }
    r := combat.ResolveDamage(in)
    assert.Greater(t, len(r.Breakdown), 1) // MULT-13: non-trivial stages present
}
```

- [ ] **Step 2: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestScenario_" -v 2>&1 | tail -30
```
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/damage_scenario_test.go && git commit -m "test(damage): PF2E scenario regression anchors for ResolveDamage (#246)"
```

---

## Task 3: `TechEffect.Multiplier` validation at load time

**Files:**
- Modify: `internal/game/technology/model.go` — update `TechEffect.Validate()` or `TechnologyDef.Validate()`
- Create: `internal/game/technology/tech_multiplier_load_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/game/technology/tech_multiplier_load_test.go
package technology_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/technology"
)

func TestTechEffect_Multiplier_ZeroPointFive_IsHalver(t *testing.T) {
    // 0.5 is the only legal fractional value (MULT-9).
    e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 0.5}
    err := e.ValidateMultiplier()
    assert.NoError(t, err)
    assert.True(t, e.IsHalver())
    assert.False(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_TwoPointZero_IsMultiplier(t *testing.T) {
    e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 2.0}
    err := e.ValidateMultiplier()
    assert.NoError(t, err)
    assert.False(t, e.IsHalver())
    assert.True(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_PointThree_IsLoadError(t *testing.T) {
    // In (0,1) and not 0.5 → load error (MULT-10).
    e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 0.3}
    err := e.ValidateMultiplier()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "illegal fractional")
}

func TestTechEffect_Multiplier_OnePointZero_IsNoOp(t *testing.T) {
    // 1.0 or unset: no contribution.
    e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 1.0}
    err := e.ValidateMultiplier()
    assert.NoError(t, err)
    assert.False(t, e.IsHalver())
    assert.False(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_Unset_IsNoOp(t *testing.T) {
    e := technology.TechEffect{Type: technology.EffectDamage}
    err := e.ValidateMultiplier()
    assert.NoError(t, err)
    assert.False(t, e.IsHalver())
    assert.False(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_NegativeValue_IsLoadError(t *testing.T) {
    e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: -0.5}
    err := e.ValidateMultiplier()
    require.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -run "TestTechEffect_Multiplier" 2>&1 | head -20
```

- [ ] **Step 3: Add `ValidateMultiplier`, `IsHalver`, `IsMultiplier` to `TechEffect` in `model.go`**

```go
// Add to internal/game/technology/model.go (in the TechEffect methods section):
import (
    "fmt"
    "math"
)

// ValidateMultiplier checks that the Multiplier field is a legal value.
// Legal values: 0 (unset/no-op), 0.5 (halver), 1.0 (no-op), or > 1.0 (multiplier).
// Values in (0, 1) other than 0.5 are a load-time error (MULT-10).
// Precondition: none.
// Postcondition: returns non-nil error iff Multiplier is illegal.
func (e TechEffect) ValidateMultiplier() error {
    m := e.Multiplier
    if m == 0 || m == 1.0 || m == 0.5 {
        return nil
    }
    if m > 1.0 {
        return nil
    }
    if m < 0 {
        return fmt.Errorf("tech effect: negative multiplier %v is not permitted", m)
    }
    // m is in (0, 1) and not 0.5
    return fmt.Errorf("tech effect: illegal fractional multiplier %v (only 0.5 permitted)", m)
}

// IsHalver returns true iff Multiplier == 0.5 (converts to a halver stage). (MULT-9)
func (e TechEffect) IsHalver() bool {
    return math.Abs(e.Multiplier - 0.5) < 1e-9
}

// IsMultiplier returns true iff Multiplier > 1.0 (feeds the multiplier bucket). (MULT-8)
func (e TechEffect) IsMultiplier() bool {
    return e.Multiplier > 1.0
}
```

Also call `ValidateMultiplier()` inside `TechnologyDef.Validate()` for every effect in all TieredEffects:

```go
// In TechnologyDef.Validate(), add a loop over all effects:
for _, eff := range d.Effects.AllEffects() {
    if err := eff.ValidateMultiplier(); err != nil {
        return fmt.Errorf("tech %q: %w", d.ID, err)
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -v 2>&1 | tail -20
```

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/technology/ && git commit -m "feat(damage): TechEffect.Multiplier validation — halver at 0.5, error on illegal fractions (#246)"
```

---

## Task 4: `buildDamageInput` helper

**Files:**
- Create: `internal/game/combat/damage_input.go`

The helper assembles `DamageInput` from the data available at each round.go damage call site. It is NOT pure (it reads from the combat state) but does not roll dice; dice results are passed in as pre-rolled values.

- [ ] **Step 1: Write the failing test**

```go
// internal/game/combat/damage_input_test.go
package combat_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
)

func TestBuildDamageInput_CritAddsMultiplier(t *testing.T) {
    actor := &combat.Combatant{ID: "player", Level: 1, StrMod: 2, WeaponBonus: 0}
    target := &combat.Combatant{ID: "npc", Resistances: map[string]int{}, Weaknesses: map[string]int{}}
    r := combat.AttackResult{
        Outcome:    combat.CritSuccess,
        BaseDamage: 5,
        DamageType: "slashing",
    }
    opts := combat.BuildDamageOpts{
        Actor:             actor,
        Target:            target,
        AttackResult:      r,
        ConditionDmgBonus: 0,
        ExtraDiceRolled:   0,
    }
    di := combat.BuildDamageInput(opts)
    require.Len(t, di.Multipliers, 1)
    assert.Equal(t, 2.0, di.Multipliers[0].Factor)
}

func TestBuildDamageInput_MissProducesZeroBase(t *testing.T) {
    actor := &combat.Combatant{ID: "player", Level: 1, StrMod: 1}
    target := &combat.Combatant{ID: "npc", Resistances: map[string]int{}, Weaknesses: map[string]int{}}
    r := combat.AttackResult{Outcome: combat.Failure, BaseDamage: 0}
    opts := combat.BuildDamageOpts{Actor: actor, Target: target, AttackResult: r}
    di := combat.BuildDamageInput(opts)
    total := 0
    for _, a := range di.Additives {
        total += a.Value
    }
    assert.LessOrEqual(t, total, 0) // miss has no positive base
}

func TestBuildDamageInput_WeaknessFromTarget(t *testing.T) {
    actor := &combat.Combatant{ID: "player", Level: 1}
    target := &combat.Combatant{
        ID:          "npc",
        Weaknesses:  map[string]int{"fire": 5},
        Resistances: map[string]int{},
    }
    r := combat.AttackResult{Outcome: combat.Success, BaseDamage: 8, DamageType: "fire"}
    opts := combat.BuildDamageOpts{Actor: actor, Target: target, AttackResult: r}
    di := combat.BuildDamageInput(opts)
    assert.Equal(t, 5, di.Weakness)
    assert.Equal(t, 0, di.Resistance)
}

func TestBuildDamageInput_ResistanceFromTarget(t *testing.T) {
    actor := &combat.Combatant{ID: "player", Level: 1}
    target := &combat.Combatant{
        ID:          "npc",
        Weaknesses:  map[string]int{},
        Resistances: map[string]int{"physical": 3},
    }
    r := combat.AttackResult{Outcome: combat.Success, BaseDamage: 10, DamageType: "physical"}
    opts := combat.BuildDamageOpts{Actor: actor, Target: target, AttackResult: r}
    di := combat.BuildDamageInput(opts)
    assert.Equal(t, 3, di.Resistance)
    assert.Equal(t, 0, di.Weakness)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestBuildDamageInput" 2>&1 | head -10
```

- [ ] **Step 3: Implement `damage_input.go`**

```go
// internal/game/combat/damage_input.go
package combat

// BuildDamageOpts carries the inputs for assembling a DamageInput at a call site.
type BuildDamageOpts struct {
    Actor             *Combatant
    Target            *Combatant
    AttackResult      AttackResult
    ConditionDmgBonus int // from condition.DamageBonus(...)
    WeaponModBonus    int // from weaponModifierDamageBonus(actor)
    ExtraDiceRolled   int // pre-rolled extra weapon dice value (caller rolls with src)
    PassiveFeatBonus  int // from applyPassiveFeats return value
    Halvers           []DamageHalver // tech halvers (e.g. from save success)
}

// BuildDamageInput assembles a DamageInput from round.go call-site data.
// On a miss (Failure or CritFailure), BaseDamage is 0 from the resolver,
// producing a zero-damage input which ResolveDamage correctly resolves to 0.
//
// Precondition: opts.Actor, opts.Target, opts.AttackResult must be set.
// Postcondition: returns a fully populated DamageInput.
func BuildDamageInput(opts BuildDamageOpts) DamageInput {
    r := opts.AttackResult
    actor := opts.Actor
    target := opts.Target

    var additives []DamageAdditive
    // Base damage (already includes dice + STR/DEX mod + weapon bonus from resolver).
    additives = append(additives, DamageAdditive{
        Label:  "dice + mods",
        Value:  r.BaseDamage,
        Source: "attack:base",
    })
    // Weapon modifier bonus (tuned/defective/cursed).
    if opts.WeaponModBonus != 0 {
        additives = append(additives, DamageAdditive{
            Label:  "weapon modifier",
            Value:  opts.WeaponModBonus,
            Source: "item:weapon_modifier",
        })
    }
    // Condition damage bonus.
    if opts.ConditionDmgBonus != 0 {
        additives = append(additives, DamageAdditive{
            Label:  "condition bonus",
            Value:  opts.ConditionDmgBonus,
            Source: "condition:damage",
        })
    }
    // Extra weapon dice (pre-rolled by caller, actor adds them as additive).
    if opts.ExtraDiceRolled != 0 {
        additives = append(additives, DamageAdditive{
            Label:  "extra weapon dice",
            Value:  opts.ExtraDiceRolled,
            Source: "feat:extra_dice",
        })
    }
    // Passive feat bonus (pre-computed by caller via applyPassiveFeats).
    if opts.PassiveFeatBonus != 0 {
        additives = append(additives, DamageAdditive{
            Label:  "passive feat",
            Value:  opts.PassiveFeatBonus,
            Source: "feat:passive",
        })
    }

    // Crit multiplier (MULT-2).
    var multipliers []DamageMultiplier
    if r.Outcome == CritSuccess {
        multipliers = append(multipliers, DamageMultiplier{
            Label:  "critical hit",
            Factor: 2.0,
            Source: "engine:crit",
        })
    }

    // Tech halvers passed in by caller.
    halvers := opts.Halvers

    // Weakness and resistance pre-resolved from target (MULT-5, MULT-6).
    weakness := 0
    resistance := 0
    if r.DamageType != "" && target != nil {
        if target.Weaknesses != nil {
            weakness = target.Weaknesses[r.DamageType]
        }
        if target.Resistances != nil {
            resistance = target.Resistances[r.DamageType]
        }
    }
    _ = actor // actor available for future passive-feat expansion

    return DamageInput{
        Additives:   additives,
        Multipliers: multipliers,
        Halvers:     halvers,
        DamageType:  r.DamageType,
        Weakness:    weakness,
        Resistance:  resistance,
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestBuildDamageInput" -v 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/damage_input.go internal/game/combat/damage_input_test.go && git commit -m "feat(damage): BuildDamageInput helper for round.go call sites (#246)"
```

---

## Task 5: Deprecate `EffectiveDamage()` and migrate 5 round.go damage sites

**Files:**
- Modify: `internal/game/combat/resolver.go` — add deprecation comment
- Modify: `internal/game/combat/round.go` — replace 5 EffectiveDamage call sites
- Create: `internal/game/combat/round_damage_pipeline_test.go`

The five sites to migrate (identified in exploration):
- Line 785: Single attack (`ActionAttack`)
- Line 948: Strike first attack (`ActionStrike` first)
- Line 1066: Strike second attack (`ActionStrike` second)
- Line 1258: Burst fire (`ActionFireBurst`)
- Line 1364: Auto fire (`ActionFireAutomatic`)

Also migrate the explosive site in `ResolveExplosive` (resolver.go) — save-outcome-scaled damage becomes `DamageInput` construction.

- [ ] **Step 1: Write end-to-end pipeline test**

```go
// internal/game/combat/round_damage_pipeline_test.go
package combat_test

import (
    "math/rand"
    "testing"

    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/dice"
)

// TestResolveRound_DamageNonNegative verifies that damage applied via the pipeline
// is always >= 0 regardless of conditions or resistances.
func TestProperty_ResolveRound_DamageNonNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        seed := rapid.Int64().Draw(rt, "seed").(int64)
        src := dice.NewRandSource(rand.NewSource(seed))

        resist := rapid.IntRange(0, 50).Draw(rt, "resist").(int)
        attacker := &combat.Combatant{
            ID: "player", Kind: combat.KindPlayer,
            Name: "Test", Level: 1, MaxHP: 20, CurrentHP: 20,
            AC: 10, StrMod: 1,
            Resistances: map[string]int{},
            Weaknesses:  map[string]int{},
            Effects:     nil,
        }
        target := &combat.Combatant{
            ID: "npc1", Kind: combat.KindNPC,
            Name: "Goon", Level: 1, MaxHP: 30, CurrentHP: 30,
            AC: 15, StrMod: 0,
            Resistances: map[string]int{"slashing": resist},
            Weaknesses:  map[string]int{},
        }
        cbt := &combat.Combat{
            RoomID:     "room1",
            Combatants: []*combat.Combatant{attacker, target},
            Conditions: map[string]*condition.ActiveSet{
                "player": condition.NewActiveSet(),
                "npc1":   condition.NewActiveSet(),
            },
            ActionQueues: map[string]*combat.ActionQueue{
                "player": {Actions: []combat.QueuedAction{{Type: combat.ActionAttack, Target: "Goon"}}},
                "npc1":   {Actions: []combat.QueuedAction{{Type: combat.ActionPass}}},
            },
        }
        events := combat.ResolveRound(cbt, src, func(id string, hp int) {}, nil)
        for _, ev := range events {
            if ev.AttackResult != nil {
                assert.GreaterOrEqual(rt, ev.AttackResult.BaseDamage, 0)
            }
        }
        // HP must never go below 0
        for _, c := range cbt.Combatants {
            assert.GreaterOrEqual(rt, c.CurrentHP, 0)
        }
    })
}
```

- [ ] **Step 2: Run test to see baseline**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestProperty_ResolveRound_DamageNonNegative" -v 2>&1 | tail -10
```

- [ ] **Step 3: Deprecate `EffectiveDamage()` in `resolver.go`**

Add a comment above the method:

```go
// EffectiveDamage returns the damage after applying the crit multiplier.
//
// Deprecated: use BuildDamageInput + ResolveDamage for all in-package call sites.
// Retained for external callers. (MULT-17)
func (r AttackResult) EffectiveDamage() int {
```

- [ ] **Step 4: Migrate the single attack site (line ~785)**

In `round.go`, find the block:
```go
dmg := r.EffectiveDamage()
dmg += weaponModifierDamageBonus(actor)
dmg += condition.DamageBonus(cbt.Conditions[actor.ID])
// ... extra dice logic ...
if r.Outcome == CritSuccess {
    extraDmg *= 2
}
dmg += extraDmg
dmg += applyPassiveFeats(cbt, actor, target, dmg, src)
dmg = hookDamageRoll(cbt, actor, target, dmg)
dmg, rwAnnotations := applyResistanceWeakness(target, r.DamageType, dmg)
```

Replace with:
```go
// Roll extra weapon dice (caller rolls; crit-doubling handled by pipeline multiplier).
var extraDiceRolled int
if extraDice := condition.ExtraWeaponDice(cbt.Conditions[actor.ID]); extraDice > 0 && (r.Outcome == CritSuccess || r.Outcome == Success) {
    dieSides := 6
    if mainHandDef != nil && mainHandDef.DamageDice != "" {
        if expr, parseErr := dice.Parse(mainHandDef.DamageDice); parseErr == nil {
            dieSides = expr.Sides
        }
    }
    for i := 0; i < extraDice; i++ {
        extraDiceRolled += src.Intn(dieSides) + 1
    }
}
// Compute passive feat bonus (pre-pipeline; not multiplied separately).
passiveFeatBonus := applyPassiveFeats(cbt, actor, target, 0, src)
// Build and resolve the damage pipeline.
di := BuildDamageInput(BuildDamageOpts{
    Actor:             actor,
    Target:            target,
    AttackResult:      r,
    ConditionDmgBonus: condition.DamageBonus(cbt.Conditions[actor.ID]),
    WeaponModBonus:    weaponModifierDamageBonus(actor),
    ExtraDiceRolled:   extraDiceRolled,
    PassiveFeatBonus:  passiveFeatBonus,
})
dmgResult := ResolveDamage(di)
dmg := hookDamageRoll(cbt, actor, target, dmgResult.Final) // Lua hook on final scalar
var rwAnnotations []string // populated from breakdown for narrative
if len(dmgResult.Breakdown) > 1 {
    rwAnnotations = append(rwAnnotations, FormatBreakdownInline(dmgResult.Breakdown))
}
```

Apply the same transformation pattern to the other 4 sites (Strike first, Strike second, Burst fire, Auto fire). The Burst and Auto fire sites currently only call `EffectiveDamage()` without weapon mods or condition bonuses — use zero for those opts fields.

- [ ] **Step 5: Remove `applyResistanceWeakness` call sites that have been replaced**

The `applyResistanceWeakness` function is now superseded by the pipeline's weakness/resistance stages. Remove the call and the `rwAnnotations` variable from all migrated sites. Replace the narrative annotation logic with `FormatBreakdownInline` (from Task 6).

Note: do NOT delete `applyResistanceWeakness` yet — verify it has no remaining callers first:
```bash
grep -n "applyResistanceWeakness" /home/cjohannsen/src/mud/internal/game/combat/round.go
```
Once all sites are migrated, remove the function.

- [ ] **Step 6: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v 2>&1 | tail -30
```

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/resolver.go internal/game/combat/round.go internal/game/combat/round_damage_pipeline_test.go && git commit -m "feat(damage): migrate 5 round.go damage sites to ResolveDamage pipeline; deprecate EffectiveDamage (#246)"
```

---

## Task 6: Breakdown narrative rendering

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go` — `FormatBreakdownInline`, `RenderDamageBreakdown`
- Create: `internal/game/combat/round_breakdown_narrative_test.go`

Note: `FormatBreakdownInline` is needed by Task 5 Step 4 but tested in this task. Implement them in the correct order: write and export the format functions first (in `damage.go` or a new `damage_format.go`), then use them in round.go.

- [ ] **Step 1: Write narrative tests**

```go
// internal/game/combat/round_breakdown_narrative_test.go
package combat_test

import (
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/combat"
)

func TestFormatBreakdownInline_TrivialOnlyBase_NoLine(t *testing.T) {
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{{Label: "dice", Value: 6, Source: "attack"}},
    }
    r := combat.ResolveDamage(in)
    line := combat.FormatBreakdownInline(r.Breakdown)
    // MULT-14: no breakdown line when only StageBase present
    assert.Empty(t, line)
}

func TestFormatBreakdownInline_CritIncluded(t *testing.T) {
    in := combat.DamageInput{
        Additives:   []combat.DamageAdditive{{Label: "1d8", Value: 5, Source: "attack"}},
        Multipliers: []combat.DamageMultiplier{{Label: "critical hit", Factor: 2.0, Source: "engine:crit"}},
    }
    r := combat.ResolveDamage(in)
    line := combat.FormatBreakdownInline(r.Breakdown)
    // MULT-14: breakdown line when non-trivial
    assert.NotEmpty(t, line)
    assert.Contains(t, line, "×2")
}

func TestFormatBreakdownInline_WeaknessAndResistance(t *testing.T) {
    in := combat.DamageInput{
        Additives:  []combat.DamageAdditive{{Label: "dice", Value: 14, Source: "attack"}},
        DamageType: "fire",
        Weakness:   3,
        Resistance: 10,
    }
    r := combat.ResolveDamage(in)
    line := combat.FormatBreakdownInline(r.Breakdown)
    assert.Contains(t, line, "weakness")
    assert.Contains(t, line, "resistance")
}

func TestFormatBreakdownInline_WidthWrapping(t *testing.T) {
    // A long breakdown must not produce lines wider than 80 chars.
    in := combat.DamageInput{
        Additives: []combat.DamageAdditive{
            {Label: "1d8", Value: 5, Source: "attack"},
            {Label: "STR mod", Value: 3, Source: "stat:str"},
            {Label: "passive feat add", Value: 2, Source: "feat:brutal"},
        },
        Multipliers: []combat.DamageMultiplier{
            {Label: "critical hit", Factor: 2.0, Source: "engine:crit"},
            {Label: "vulnerable fire", Factor: 2.0, Source: "cond:vuln"},
        },
        Halvers:    []combat.DamageHalver{{Label: "basic save", Source: "tech:fireball"}},
        DamageType: "fire",
        Weakness:   5,
        Resistance: 3,
    }
    r := combat.ResolveDamage(in)
    line := combat.FormatBreakdownInline(r.Breakdown)
    for _, l := range strings.Split(line, "\n") {
        assert.LessOrEqual(t, len(l), 80, "line too wide: %q", l)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestFormatBreakdownInline" 2>&1 | head -10
```

- [ ] **Step 3: Implement `FormatBreakdownInline` in `damage.go`**

Add to `internal/game/combat/damage.go`:

```go
// FormatBreakdownInline renders the breakdown as a single indented line appended
// to the combat narrative. Returns "" when breakdown has only StageBase (trivial). (MULT-14)
func FormatBreakdownInline(steps []DamageBreakdownStep) string {
    if len(steps) <= 1 {
        return "" // only base — no annotation needed
    }
    var parts []string
    for _, step := range steps {
        switch step.Stage {
        case StageBase:
            parts = append(parts, fmt.Sprintf("base %d", step.After))
        case StageMultiplier:
            parts = append(parts, fmt.Sprintf("×%.0f [%s] = %d", 1.0+float64(step.Delta)/float64(step.Before), step.Detail, step.After))
        case StageHalver:
            parts = append(parts, fmt.Sprintf("halved [%s] = %d", step.Detail, step.After))
        case StageWeakness:
            parts = append(parts, fmt.Sprintf("weakness +%d = %d", step.Delta, step.After))
        case StageResistance:
            parts = append(parts, fmt.Sprintf("resistance %d = %d", step.Delta, step.After))
        case StageFloor:
            parts = append(parts, "floored = 0")
        }
    }
    line := "  (" + strings.Join(parts, " → ") + ")"
    // Width-wrap at → boundaries to stay within 80 cols.
    return wrapAtArrow(line, 80)
}

// wrapAtArrow wraps a breakdown line at " → " boundaries so no segment exceeds maxWidth.
func wrapAtArrow(line string, maxWidth int) string {
    if len(line) <= maxWidth {
        return line
    }
    segments := strings.Split(line, " → ")
    var sb strings.Builder
    cur := ""
    for i, seg := range segments {
        candidate := cur
        if i > 0 {
            candidate += " → "
        }
        candidate += seg
        if len(candidate) > maxWidth && cur != "" {
            sb.WriteString(cur + "\n")
            cur = "    → " + seg // continuation indent
        } else {
            cur = candidate
        }
    }
    sb.WriteString(cur)
    return sb.String()
}
```

Add `"strings"` import to `damage.go`.

- [ ] **Step 4: Implement `RenderDamageBreakdown` in `text_renderer.go` for verbose form**

Add to `internal/frontend/handlers/text_renderer.go`:

```go
// RenderDamageBreakdown renders the full verbose breakdown block (MULT-15).
// Only emitted to observers with ShowDamageBreakdown enabled.
func RenderDamageBreakdown(steps []combat.DamageBreakdownStep) string {
    if len(steps) == 0 {
        return ""
    }
    var sb strings.Builder
    for _, step := range steps {
        switch step.Stage {
        case combat.StageBase:
            sb.WriteString(fmt.Sprintf("  base:       %+d\n", step.After))
        case combat.StageMultiplier:
            sb.WriteString(fmt.Sprintf("  multiplier: %s\n", step.Detail))
            sb.WriteString(fmt.Sprintf("  after:      %d\n", step.After))
        case combat.StageHalver:
            sb.WriteString(fmt.Sprintf("  halver:     %s = %d\n", step.Detail, step.After))
        case combat.StageWeakness:
            sb.WriteString(fmt.Sprintf("  weakness:   %s\n", step.Detail))
        case combat.StageResistance:
            sb.WriteString(fmt.Sprintf("  resistance: %s\n", step.Detail))
        case combat.StageFloor:
            sb.WriteString(fmt.Sprintf("  final:      0 (floored)\n"))
        }
    }
    if steps[len(steps)-1].Stage != combat.StageFloor {
        last := steps[len(steps)-1]
        sb.WriteString(fmt.Sprintf("  final:      %d\n", last.After))
    }
    return sb.String()
}
```

- [ ] **Step 5: Run all narrative tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestFormatBreakdownInline" -v 2>&1 | tail -20
```

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/damage.go internal/frontend/handlers/text_renderer.go internal/game/combat/round_breakdown_narrative_test.go && git commit -m "feat(damage): FormatBreakdownInline and RenderDamageBreakdown narrative helpers (#246)"
```

---

## Task 7: `show_damage_breakdown` preference and verbose block delivery

**Files:**
- Modify: `internal/game/session/manager.go` — add `ShowDamageBreakdown bool` to `PlayerSession`
- Modify: `internal/gameserver/combat_handler.go` — emit verbose block to breakdown-enabled observers
- Modify: `internal/game/combat/round.go` — include breakdown string in `RoundEvent`

- [ ] **Step 1: Write failing test for pref field**

```go
// internal/game/session/session_breakdown_test.go
package session_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/session"
)

func TestPlayerSession_ShowDamageBreakdown_DefaultFalse(t *testing.T) {
    sess := &session.PlayerSession{}
    assert.False(t, sess.ShowDamageBreakdown)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/session/... -run "TestPlayerSession_ShowDamageBreakdown" 2>&1 | head -10
```

- [ ] **Step 3: Add `ShowDamageBreakdown` to `PlayerSession`**

In `internal/game/session/manager.go`, add to `PlayerSession` struct (after `BankedAP`):

```go
// ShowDamageBreakdown controls whether the player receives verbose per-stage
// damage breakdowns for attacks they deal or receive.
ShowDamageBreakdown bool
```

- [ ] **Step 4: Add `DamageBreakdown` field to `RoundEvent`**

In `internal/game/combat/round.go`, add to the `RoundEvent` struct:

```go
// DamageBreakdown is the formatted verbose breakdown (MULT-13/15).
// Only populated when the damage pipeline had non-trivial stages.
DamageBreakdown string
```

In the round.go damage sites, after computing `dmgResult`:

```go
roundEv.DamageBreakdown = combat.FormatBreakdownInline(dmgResult.Breakdown)
```

- [ ] **Step 5: Deliver verbose block in gameserver**

In `internal/gameserver/combat_handler.go`, where combat narrative events are delivered to players, check `ShowDamageBreakdown` and append the verbose block:

```go
// After building the narrative string for a damage event:
if ev.DamageBreakdown != "" {
    // Deliver inline form to all observers.
    narrative += "\n" + ev.DamageBreakdown

    // Deliver verbose form only to ShowDamageBreakdown-enabled observers.
    if observerSess != nil && observerSess.ShowDamageBreakdown {
        narrative += "\n" + handlers.RenderDamageBreakdown(ev.BreakdownSteps)
    }
}
```

Note: The `RoundEvent` carries `DamageBreakdown string` (inline text). For the verbose form, the breakdown steps need to be available on the event. Add `BreakdownSteps []DamageBreakdownStep` to `RoundEvent` as well, populated alongside `DamageBreakdown`:

```go
// In RoundEvent struct:
BreakdownSteps []DamageBreakdownStep
```

Populate in round.go:
```go
roundEv.BreakdownSteps = dmgResult.Breakdown
```

- [ ] **Step 6: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/session/... ./internal/game/combat/... ./internal/gameserver/... -v 2>&1 | tail -30
```

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/session/ internal/game/combat/round.go internal/gameserver/ && git commit -m "feat(damage): ShowDamageBreakdown pref; verbose breakdown delivery to enabled observers (#246)"
```

---

## Task 8: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add "Damage pipeline" section**

Find the appropriate place in `docs/architecture/combat.md` (after the round resolution section) and insert:

```markdown
## Damage Pipeline (MULT Requirements)

All direct-damage computation flows through `combat.ResolveDamage(DamageInput) DamageResult`
in `internal/game/combat/damage.go`. This function is pure: no RNG, no mutation, no I/O.

### Stages (executed in order — MULT-1)

| Stage | Rule |
|-------|------|
| **Base** | Sum all `DamageAdditive.Value` entries (may be negative) |
| **Multiplier** | `effective = 1 + Σ(Factor_i − 1)`; apply `base × effective` (MULT-2) |
| **Halver** | `floor(after_mult / 2)`; multiple halvers collapse to one (MULT-3, MULT-4) |
| **Weakness** | `+ target.Weaknesses[damageType]` (MULT-5) |
| **Resistance** | `− target.Resistances[damageType]` (MULT-6) |
| **Floor** | `max(0, …)` (MULT-7) |

### Multiplier stacking rule (MULT-2)

PF2E: multiple ×M sources combine as `effective = 1 + Σ(M_i − 1)`.

| Sources | Effective factor |
|---------|-----------------|
| Crit (×2) alone | ×2 |
| Crit (×2) + Vulnerable (×2) | ×3 |
| Three ×2 sources | ×4 |

Multiplicative chaining (×4 from two ×2s) is **not** permitted.

### Halver semantics (MULT-9)

`TechEffect.Multiplier == 0.5` is interpreted as a halver (not a 0.5× multiplier).
Any other value in `(0, 1)` except `0.5` is rejected at YAML load time (MULT-10).

### buildDamageInput (MULT-12)

Every damage-producing call site in `round.go` assembles its input via
`BuildDamageInput(BuildDamageOpts)` in `damage_input.go`. Open-coded crit-doubling,
weakness, and resistance arithmetic are removed; those stages live only in the pipeline.

### Reaction compatibility (MULT-16)

`TriggerOnDamageTaken` callbacks receive and may mutate `DamagePending *int` — the final
post-pipeline scalar — before `ApplyDamage`. They do not see the breakdown in v1.

### Breakdown output (MULT-13, MULT-14, MULT-15)

- `DamageResult.Breakdown` contains one entry per non-trivial stage in canonical order.
- `StageBase` is always present.
- `FormatBreakdownInline(steps)` returns `""` for trivial (base-only) inputs; otherwise
  an indented inline string wrapping at `→` boundaries within 80 cols.
- `RenderDamageBreakdown(steps)` returns the verbose multi-line form.
- Players with `SessionManager.ShowDamageBreakdown` receive the verbose block.
```

- [ ] **Step 2: Commit**

```bash
cd /home/cjohannsen/src/mud && git add docs/architecture/combat.md && git commit -m "docs: damage pipeline architecture — MULT requirements, stage table, multiplier stacking (#246)"
```

---

## Self-Review

**Spec coverage check:**

| Requirement | Covered by |
|-------------|-----------|
| MULT-1: stages in order base→mult→halve→weak→resist→floor | Task 1 `ResolveDamage` |
| MULT-2: extra multipliers as +1, not chained | Task 1 `ResolveDamage` / `sumExtra` |
| MULT-3: multiple halvers collapse to one | Task 1 `if len(in.Halvers) > 0` |
| MULT-4: halver after multiplier | Task 1 stage ordering |
| MULT-5: weakness after halver | Task 1 stage 4 |
| MULT-6: resistance after weakness | Task 1 stage 5 |
| MULT-7: floor at 0 | Task 1 stage 6 |
| MULT-8: `Factor > 1.0` for multipliers | Task 4 `BuildDamageInput` / validation note |
| MULT-9: `Multiplier == 0.5` → halver | Task 3 `IsHalver()` |
| MULT-10: fractional ≠ 0.5 → load error | Task 3 `ValidateMultiplier()` |
| MULT-11: `ResolveDamage` is pure | Task 1 (no RNG/mutation/IO) |
| MULT-12: all round.go sites via `buildDamageInput` | Task 5 migration |
| MULT-13: breakdown has one entry per non-trivial stage; StageBase always present | Task 1 |
| MULT-14: inline line iff `len(Breakdown) > 1` | Task 6 `FormatBreakdownInline` |
| MULT-15: verbose block for `show_damage_breakdown` observers | Task 7 |
| MULT-16: reaction callbacks see final scalar only | Task 5 (hookDamageRoll/reaction remain post-pipeline) |
| MULT-17: `EffectiveDamage()` deprecated but retained | Task 5 deprecation comment |

**Placeholder scan:** No TBDs or "implement later" phrases.

**Type consistency:** `DamageBreakdownStep.Stage` uses `DamageStage` constants throughout Tasks 1-7. `BuildDamageOpts` and `BuildDamageInput` match between Tasks 4 and 5.

**Open questions resolved (spec §10):**
- `buildDamageInput` location: `internal/game/combat/damage_input.go`
- Web client breakdown: inline muted text under damage event (serialized into `RoundEvent.DamageBreakdown` string, no new WS message type)
- `show_damage_breakdown` persistence: `PlayerSession.ShowDamageBreakdown bool` field (session-scoped, not DB-persisted in v1)
- Verbose block: emitted once per damage event per breakdown-enabled observer
