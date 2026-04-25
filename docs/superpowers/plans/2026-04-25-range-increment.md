# Range Increment Penalties — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the range-increment cap from 4× to 6× per PF2E. Extract the math to a single `combat.RangePenalty(distanceFt, increment) (penalty, beyondMax)` helper. Annotate every range-affected attack in the combat log with the penalty value, distance, and increment. Existing melee paths (`RangeIncrement == 0`) remain untouched.

**Spec:** [docs/superpowers/specs/2026-04-25-range-increment.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-range-increment.md) (PR [#290](https://github.com/cory-johannsen/mud/pull/290))

**Architecture:** Three small surgical changes. (1) New pure helper `combat.RangePenalty` in `internal/game/combat/range.go` returning `(penalty, beyondMax)`; property tests cover monotonicity, boundary precision, and the cap. (2) `resolver.go:106-107` replaces its inline calc with a `RangePenalty` call; on `beyondMax == true` the attack short-circuits with the "out of range" narrative; the `4 *` cutoff at `round.go:869` is removed. (3) `attackNarrative` emits a `[Range -X: <dist>ft / increment <inc>ft]` annotation alongside the MAP / step annotations from #260 / #262 — fixed annotation order `[MAP …][Range …][Step bumped …]` — and a distinct `*** OUT OF RANGE ***` line skips the d20 breakdown when applicable.

**Tech Stack:** Go (`internal/game/combat/`), `pgregory.net/rapid` for property tests.

**Prerequisite:** None hard. Sister annotation specs #260 (Step) and #262 (MAP) — annotation order is locked in this plan; whichever lands last must verify the order matches.

**Note on spec PR**: Spec is on PR #290, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/combat/range.go` (`RangePenalty` helper) |
| Create | `internal/game/combat/range_test.go` |
| Create | `internal/game/combat/range_increment_test.go` (scenario coverage) |
| Create | `internal/game/combat/testdata/rapid/TestRangePenalty_Property/` |
| Modify | `internal/game/combat/resolver.go` (replace inline calc) |
| Modify | `internal/game/combat/round.go` (remove `4×` cutoff; integrate annotation) |
| Modify | `internal/game/combat/round_test.go` and `max_range_test.go` (update pinned cap from 4× to 6×) |
| Optional | `cmd/webclient/ui/src/combat/AttackButton.tsx` (RANGE-14 chip) |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: `RangePenalty` helper + property tests

**Files:**
- Create: `internal/game/combat/range.go`
- Create: `internal/game/combat/range_test.go`
- Create: `internal/game/combat/testdata/rapid/TestRangePenalty_Property/`

- [ ] **Step 1: Failing tests** (RANGE-1..3, RANGE-12):

```go
func TestRangePenalty_FirstIncrementNoPenalty(t *testing.T) {
    p, beyond := combat.RangePenalty(60, 60)
    require.Equal(t, 0, p)
    require.False(t, beyond)
}

func TestRangePenalty_PerIncrementSteps(t *testing.T) {
    inc := 60
    cases := []struct{ distFt, wantPenalty int }{
        {1,    0},          // first increment
        {60,   0},          // boundary inside first
        {61,   -2},         // second
        {120,  -2},
        {121,  -4},
        {180,  -4},
        {181,  -6},
        {240,  -6},
        {241,  -8},
        {300,  -8},
        {301,  -10},
        {360,  -10},
    }
    for _, c := range cases {
        p, beyond := combat.RangePenalty(c.distFt, inc)
        require.Equal(t, c.wantPenalty, p, "distance %d", c.distFt)
        require.False(t, beyond)
    }
}

func TestRangePenalty_BeyondMaxReturnsBeyondTrue(t *testing.T) {
    p, beyond := combat.RangePenalty(361, 60)
    require.Equal(t, 0, p)
    require.True(t, beyond)
}

func TestRangePenalty_MeleeIncrementZeroReturnsNoPenalty(t *testing.T) {
    p, beyond := combat.RangePenalty(50, 0)
    require.Equal(t, 0, p)
    require.False(t, beyond)
}

func TestRangePenalty_BoundaryPrecision(t *testing.T) {
    // distance == increment is in increment 1 (no penalty).
    p, _ := combat.RangePenalty(60, 60)
    require.Equal(t, 0, p)
    // distance == increment + 1 is in increment 2 (-2).
    p, _ = combat.RangePenalty(61, 60)
    require.Equal(t, -2, p)
}
```

- [ ] **Step 2: Property test** (RANGE-3):

```go
func TestProperty_RangePenalty_Monotonic(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        inc := rapid.IntRange(1, 200).Draw(t, "inc")
        a := rapid.IntRange(1, 6*inc).Draw(t, "a")
        b := rapid.IntRange(a, 6*inc).Draw(t, "b")
        pa, _ := combat.RangePenalty(a, inc)
        pb, _ := combat.RangePenalty(b, inc)
        require.GreaterOrEqual(t, pa, pb, "longer distance must produce equal or more negative penalty")
    })
}

func TestProperty_RangePenalty_StepBoundary(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        inc := rapid.IntRange(1, 100).Draw(t, "inc")
        k := rapid.IntRange(1, 6).Draw(t, "k")
        // distance == k*inc → still in increment k (zero-indexed: k-1 increments past first).
        p, beyond := combat.RangePenalty(k*inc, inc)
        require.False(t, beyond)
        require.Equal(t, -2*(k-1), p)
    })
}

func TestProperty_RangePenalty_BeyondAtSixPlusOne(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        inc := rapid.IntRange(1, 100).Draw(t, "inc")
        _, beyond := combat.RangePenalty(6*inc+1, inc)
        require.True(t, beyond)
    })
}
```

- [ ] **Step 3: Implement**:

```go
package combat

func RangePenalty(distanceFt, incrementFt int) (penalty int, beyondMax bool) {
    if incrementFt <= 0 || distanceFt <= incrementFt {
        return 0, false
    }
    if distanceFt > 6*incrementFt {
        return 0, true
    }
    // Compute increment index. With distanceFt > incrementFt and ≤ 6*incrementFt,
    // index is ceil(distanceFt / incrementFt), giving 2..6.
    index := (distanceFt + incrementFt - 1) / incrementFt
    return -2 * (index - 1), false
}
```

- [ ] **Step 4:** All tests green; rapid recordings checked into `testdata/rapid/`.

---

### Task 2: Resolver integration — replace inline calc + remove `4×` cutoff

**Files:**
- Modify: `internal/game/combat/resolver.go`
- Modify: `internal/game/combat/round.go`
- Create: `internal/game/combat/range_increment_test.go`

- [ ] **Step 1: Failing scenario tests** (RANGE-4..7, RANGE-12):

```go
func TestResolveAttack_FirstIncrementNoRangePenalty(t *testing.T) {
    cbt := buildCombat(t)
    a, target := setupRangedShot(t, cbt, distanceCells: 12, weaponIncrement: 60) // 60 ft
    res := combat.ResolveAttack(cbt, a, target, weaponNamed(a, "carbine"))
    require.Equal(t, 0, res.RangePenalty)
}

func TestResolveAttack_FifthIncrementMinus8(t *testing.T) {
    cbt := buildCombat(t)
    a, target := setupRangedShot(t, cbt, distanceCells: 49, weaponIncrement: 60) // 245 ft
    res := combat.ResolveAttack(cbt, a, target, weaponNamed(a, "carbine"))
    require.Equal(t, -8, res.RangePenalty)
}

func TestResolveAttack_BeyondSixthAutoMiss(t *testing.T) {
    cbt := buildCombat(t)
    a, target := setupRangedShot(t, cbt, distanceCells: 73, weaponIncrement: 60) // 365 ft > 360 ft cap
    res := combat.ResolveAttack(cbt, a, target, weaponNamed(a, "carbine"))
    require.Equal(t, combat.OutcomeMiss, res.Outcome)
    require.True(t, res.OutOfRange)
}

func TestResolveAttack_MeleeIncrementZeroSkipsRangePath(t *testing.T) {
    cbt := buildCombat(t)
    a, target := setupMeleeShot(t, cbt) // weapon with RangeIncrement = 0
    res := combat.ResolveAttack(cbt, a, target, weaponNamed(a, "club"))
    require.Equal(t, 0, res.RangePenalty)
    require.False(t, res.OutOfRange)
}

func TestResolveAttack_DamageNotAffectedByRange(t *testing.T) {
    cbt := buildCombat(t)
    nearRes := resolveAtIncrement(t, 1)
    farRes  := resolveAtIncrement(t, 5)
    require.Equal(t, nearRes.Damage, farRes.Damage)
}
```

- [ ] **Step 2: Replace inline calc** at `resolver.go:106-107`:

```go
penalty, beyondMax := combat.RangePenalty(distance, weapon.RangeIncrement)
if beyondMax {
    return AttackResult{Outcome: OutcomeMiss, OutOfRange: true}
}
attackTotal += penalty
result.RangePenalty = penalty
```

- [ ] **Step 3: Remove the `4×` cutoff** at `round.go:869`. The `beyondMax` branch is now the only path.

- [ ] **Step 4: Update existing range tests** at `max_range_test.go` from the 4× cap to the 6× cap. Each updated test gets a `// PF2E alignment per RANGE-11` comment marking the change.

- [ ] **Step 5:** `WeaponDef.RangeIncrement == 0` (melee / unarmed) skips the range path entirely (RANGE-6 / RANGE-12 last case).

---

### Task 3: Combat log annotation — `[Range -X: <dist>ft / increment <inc>ft]`

**Files:**
- Modify: `internal/game/combat/round.go` (`attackNarrative`)
- Modify: `internal/game/combat/range_increment_test.go`

- [ ] **Step 1: Failing tests** (RANGE-8..10, RANGE-13):

```go
func TestAttackNarrative_RangePenaltyAppears(t *testing.T) {
    out := attackNarrativeOf(t, distanceFt: 245, incrementFt: 60)
    require.Contains(t, out, "[Range -8: 245ft / increment 60ft]")
}

func TestAttackNarrative_NoRangeAnnotationOnFirstIncrement(t *testing.T) {
    out := attackNarrativeOf(t, distanceFt: 60, incrementFt: 60)
    require.NotContains(t, out, "[Range")
}

func TestAttackNarrative_OutOfRangeNarrativeReplacesD20(t *testing.T) {
    out := attackNarrativeOf(t, distanceFt: 365, incrementFt: 60)
    require.Contains(t, out, "*** OUT OF RANGE ***")
    require.NotContains(t, out, "vs AC")
    require.Contains(t, out, "365ft / max 360ft")
}

func TestAttackNarrative_AnnotationOrder_MAP_Range_Step(t *testing.T) {
    // RANGE-10: fixed order [MAP …][Range …][Step bumped …].
    out := attackNarrativeOfFull(t, mapPenalty: -5, rangePenalty: -4, stepBumped: "Failure → Success", naturalRoll: 20)
    mapIdx   := strings.Index(out, "[MAP ")
    rangeIdx := strings.Index(out, "[Range ")
    stepIdx  := strings.Index(out, "Natural 20")
    require.Less(t, mapIdx, rangeIdx)
    require.Less(t, rangeIdx, stepIdx)
}
```

- [ ] **Step 2: Implement** the annotation in `attackNarrative`:

```go
func attackNarrative(...) string {
    base := existingNarrative(...)
    if outOfRange {
        return fmt.Sprintf("*** OUT OF RANGE *** %s fires at %s (%dft / max %dft).", attacker.Name, target.Name, distanceFt, 6*incrementFt)
    }
    if mapPenalty != 0 {
        base += " " + mapAnnotation(mapPenalty, attacksMade, weapon)
    }
    if rangePenalty != 0 {
        base += fmt.Sprintf(" [Range %d: %dft / increment %dft]", rangePenalty, distanceFt, incrementFt)
    }
    if stepAdjusted {
        base += " " + stepAnnotation(naturalRoll, prev, next)
    }
    return base
}
```

The annotation order is fixed: `[MAP …][Range …][Step bumped …]` (RANGE-10).

- [ ] **Step 3:** OOR narrative entirely replaces the d20 breakdown (RANGE-9).

---

### Task 4: Optional — web `AttackButton` range chip

**Files:**
- Modify: `cmd/webclient/ui/src/combat/AttackButton.tsx` (or equivalent)

- [ ] **Step 1: Decision.** Per RANGE-14 SHOULD-not-MUST. Implement only if time allows.

- [ ] **Step 2 (if implementing): Failing component tests**:

```ts
test("AttackButton shows yellow chip at second increment", () => {
  render(<AttackButton distanceFt={120} weaponIncrementFt={60} />);
  expect(screen.getByText("-2")).toHaveClass("range-chip-yellow");
});

test("AttackButton shows orange chip at third increment", () => {
  render(<AttackButton distanceFt={181} weaponIncrementFt={60} />);
  expect(screen.getByText("-6")).toHaveClass("range-chip-orange");
});

test("AttackButton shows red chip at sixth increment", () => {
  render(<AttackButton distanceFt={360} weaponIncrementFt={60} />);
  expect(screen.getByText("-10")).toHaveClass("range-chip-red");
});

test("AttackButton hides chip at first increment", () => {
  render(<AttackButton distanceFt={60} weaponIncrementFt={60} />);
  expect(screen.queryByText(/-/)).toBeNull();
});
```

- [ ] **Step 3: Implement** the chip with thresholds: `-2` yellow, `-6` orange, `-10` red.

---

### Task 5: Architecture documentation update

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Range Increments" section** documenting:
  - The 6× cap (PF2E aligned).
  - The `RangePenalty` helper signature and contract.
  - The annotation format and the fixed order with MAP / step annotations.
  - The OOR narrative format.
  - Open question resolutions (RANGE-Q1..Q5).
  - Cross-link to spec, plan, and #251 MOVE-11 follow-on.

- [ ] **Step 2: Verify** GitHub markdown preview.

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- Telnet smoke test: ranged attack at distance ≤ first increment (no annotation); attack at second increment (yellow chip equivalent + annotation `[Range -2: ...]`); attack at sixth increment (`[Range -10: ...]`); attack beyond cap (`*** OUT OF RANGE ***`).

---

## Rollout / Open Questions Resolved at Plan Time

- **RANGE-Q1**: PF2E formulation `−2 × (incrementIndex - 1)` as captured in RANGE-1.
- **RANGE-Q2**: Boundary precision — `distance == increment` is in increment 1 (no penalty); `distance == increment + 1` is in increment 2 (`-2`). Verified in tests.
- **RANGE-Q3**: Web chip thresholds: -2 yellow, -6 orange, -10 red. Optional task.
- **RANGE-Q4**: NPCs share the resolver path; existing NPC tests pinning specific roll outcomes get updated when the 6× cap correction surfaces.
- **RANGE-Q5**: NPC `rangeGoal` integration (#251 MOVE-11) is RANGE-F1 — out of scope here.

## Non-Goals Reaffirmed

Per spec §2.2:

- No volumetric / 3D range.
- No per-environment range modifiers.
- No range-affecting weapon traits.
- No splash / AoE range-increment math.
- No "near max range" UI hint beyond the optional chip.
