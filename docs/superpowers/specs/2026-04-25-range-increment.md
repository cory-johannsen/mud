---
title: Range Increment Penalties for Ranged Weapons
issue: https://github.com/cory-johannsen/mud/issues/263
date: 2026-04-25
status: spec
prefix: RANGE
depends_on: []
related:
  - "#262 Agile + MAP (sister attack-roll annotation)"
  - "#260 Natural 1/20 (sister attack-roll annotation)"
  - "#259 Bonus types (combat log breakdown surface)"
---

# Range Increment Penalties for Ranged Weapons

## 1. Summary

Range increment computation is already partially implemented:

- `WeaponDef.RangeIncrement int` declares the per-weapon increment in feet (`internal/game/inventory/weapon.go`).
- `combat.CombatRange(a, b)` returns Chebyshev distance × 5 ft (`combat.go:285-298`).
- `resolver.go:106-107` computes `rangePenalty := rangeIncrements * 2` and subtracts it from the attack total.
- `round.go:869` short-circuits to an auto-miss "extreme range" narrative when distance > **4** × RangeIncrement.

The issue text and PF2E both specify a **6-increment cap** (max range = 6 × increment, with each increment past the first applying −2). The current code caps at 4 × increment, which is more restrictive than the rules. There is also no combat-log annotation for the range penalty (the player's roll is silently lower without explanation), no dedicated tests pinning the increment math, and the `attackNarrative` function does not surface the range-penalty component.

This spec ships:

1. Correct the extreme-range cutoff from `4 ×` to `6 ×` (PF2E and issue alignment).
2. Add a `[Range -2 × N: <range>ft / increment <inc>ft]` combat-log annotation alongside the existing MAP and natural-1/20 annotations.
3. Add explicit boundary tests for first increment (no penalty), second increment (-2), increments 3–6 (-4 / -6 / -8 / -10), and beyond-6th (auto-miss).
4. Expose a `combat.RangePenalty(distanceFt int, increment int) (penalty int, beyondMax bool)` helper so the calculation has a single home.
5. Ensure unarmed / melee attacks (range increment 0) are unaffected by the new helper.

## 2. Goals & Non-Goals

### 2.1 Goals

- RANGE-G1: Ranged attacks within the first range increment have no penalty.
- RANGE-G2: Each range increment beyond the first applies a cumulative −2 penalty to the attack roll, capped at six increments per PF2E.
- RANGE-G3: Attacks beyond the sixth increment auto-miss with a clear "out of range" narrative.
- RANGE-G4: The combat-log breakdown shows the range-increment penalty value and source.
- RANGE-G5: A single `combat.RangePenalty` helper function owns the math; no duplicated logic across resolver and renderer.
- RANGE-G6: Existing tests pass; `WeaponDef.RangeIncrement == 0` (melee / unarmed) still bypasses the range path.

### 2.2 Non-Goals

- RANGE-NG1: Volumetric / 3D range. v1 stays Chebyshev × 5 ft.
- RANGE-NG2: Per-environment range modifiers (wind, low gravity).
- RANGE-NG3: Special weapon traits that change increment behavior (e.g., a hypothetical `precision` trait that reduces the per-increment penalty). Out of scope until a trait warrants it.
- RANGE-NG4: Splash / area-effect range increments (existing AoE per #250 is positional, not increment-based).
- RANGE-NG5: A "warning when attacking near max range" UI hint. The annotation is sufficient.

## 3. Glossary

- **Range increment**: a per-weapon distance in feet (`WeaponDef.RangeIncrement`).
- **Increment index**: the integer value `floor((distanceFt - 1) / increment) + 1`, so distance 0–increment is index 1 (no penalty), increment+1 to 2×increment is index 2 (−2 penalty), and so on.
- **Maximum range**: `6 × RangeIncrement` ft. Attacks at distances strictly greater than this auto-miss.

## 4. Requirements

### 4.1 Helper Function

- RANGE-1: A new function `combat.RangePenalty(distanceFt int, increment int) (penalty int, beyondMax bool)` MUST be added, with the rules:
  - `increment <= 0` → return `(0, false)` (melee / unarmed).
  - `distanceFt <= increment` → return `(0, false)` (within first increment).
  - `distanceFt <= 6 * increment` → return `(-2 * (incrementIndex - 1), false)` where `incrementIndex = ceil(distanceFt / increment)`.
  - `distanceFt > 6 * increment` → return `(0, true)` (beyond max).
- RANGE-2: The function MUST be a pure function (no globals, no allocations except scalar return).
- RANGE-3: Property tests under `internal/game/combat/testdata/rapid/TestRangePenalty_Property/` MUST verify:
  - Monotonicity: penalty grows (becomes more negative) as distance grows, until the cap.
  - Step boundaries: `distanceFt == k * increment` is index `k`.
  - Beyond-max: `distanceFt == 6 * increment + 1` returns `beyondMax: true`.

### 4.2 Resolver Integration

- RANGE-4: `resolver.go:106-107` MUST replace its inline computation with a call to `combat.RangePenalty(distance, weapon.RangeIncrement)`. When `beyondMax == true`, the attack MUST short-circuit to an auto-miss outcome ("out of range") regardless of the d20 roll.
- RANGE-5: `round.go:869` (current `4 *` cutoff) MUST be removed in favor of the `beyondMax` branch from RANGE-4. Single source of truth for the cap.
- RANGE-6: When `weapon.RangeIncrement == 0` (melee / unarmed), the resolver MUST skip the range path entirely. No regression in melee tests.
- RANGE-7: Damage application MUST be unchanged — range only affects the to-hit roll, not damage.

### 4.3 Combat Log Annotation

- RANGE-8: `attackNarrative` MUST emit a `[Range <-2|-4|-6|-8|-10>: <distance>ft / increment <inc>ft]` suffix when the range penalty is non-zero. Examples:
  - `[Range -2: 65ft / increment 60ft]`
  - `[Range -10: 295ft / increment 60ft]`
- RANGE-9: When the attack is "out of range" (beyondMax), the narrative MUST emit `*** OUT OF RANGE *** <attacker> fires at <target> (<distance>ft / max <maxFt>ft).` and skip the d20 / vs AC line entirely.
- RANGE-10: The annotation MUST appear inline with the existing `1d20 (N) ±mod = total vs AC X` breakdown so the player sees range alongside MAP and crit-step annotations from #260 and #262. Annotation order: `[MAP …][Range …][Step bumped …]`.

### 4.4 Tests

- RANGE-11: Existing tests in `internal/game/combat/max_range_test.go` MUST be updated to expect the new 6 × cap rather than the prior 4 × cap. Each updated test MUST carry a comment marking the change as PF2E alignment.
- RANGE-12: New unit tests in `internal/game/combat/range_increment_test.go` MUST cover:
  - First increment (distance ≤ increment): penalty `0`.
  - Each increment from 2 through 6: `-2 / -4 / -6 / -8 / -10` respectively.
  - Beyond-6th: auto-miss (no roll).
  - Boundary precision: `distance == increment` is in increment 1; `distance == increment + 1` is in increment 2.
  - Melee weapon (RangeIncrement = 0): skips range path.
- RANGE-13: A scenario test MUST exercise the combat log annotation format from RANGE-8 / RANGE-9 for both within-range and beyond-max cases.

### 4.5 Optional UI Hint

- RANGE-14: The web combat panel SHOULD show a small range-marker on the action bar's attack button when hovering a target outside the first range increment (e.g., a yellow `-2` chip for second increment, escalating to red at the cap). Telnet has no equivalent; the annotation in the combat log is the single feedback channel there. This requirement is SHOULD, not MUST, so the implementer may defer it without violating acceptance.

## 5. Architecture

### 5.1 Where the new code lives

```
internal/game/combat/
  range.go                      # NEW: RangePenalty pure helper
  range_test.go                 # NEW: unit + property tests
  resolver.go                   # existing; replace inline calc with RangePenalty call
  round.go                      # existing; remove the 4× cutoff at :869
  range_increment_test.go       # NEW: scenario coverage
  testdata/rapid/TestRangePenalty_Property/   # NEW
```

### 5.2 Attack flow

```
ResolveRound → attack:
  weapon = lookupWeapon(attacker.WeaponID)
  if weapon.RangeIncrement > 0:
      distance = CombatRange(attacker, target)  # Chebyshev × 5 ft
      penalty, beyondMax = RangePenalty(distance, weapon.RangeIncrement)
      if beyondMax:
          emit "*** OUT OF RANGE ***" narrative; skip roll
          return
      attackTotal += penalty
  resolve as normal
  attackNarrative emits "[Range <penalty>: <distance>ft / increment <inc>ft]" when penalty != 0
```

### 5.3 Single sources of truth

- Range math: `combat.RangePenalty` only.
- Annotation format: `attackNarrative` only.
- Distance: `combat.CombatRange` only.

## 6. Open Questions

- RANGE-Q1: PF2E's exact rule says "each range increment past the first" applies a `−2` *cumulative* penalty. The phrasing implies `-2 × (incrementIndex - 1)` which matches RANGE-1. Recommendation: stick with that formulation.
- RANGE-Q2: When a weapon's range increment is `5` ft (essentially melee with throwing) — does the helper correctly produce `0` at distance 5 ft and `-2` at 6 ft? Yes per RANGE-1; verify in tests (RANGE-12 boundary case).
- RANGE-Q3: The optional UI hint (RANGE-14) — should the chip color match the magnitude of the penalty (yellow → orange → red)? Recommendation: yes, with thresholds at -2 (yellow), -6 (orange), -10 (red). Defer if implementer is short on time.
- RANGE-Q4: Should NPCs whose `RangeIncrement` is set on their template weapon also feel the penalty in their attack rolls? Recommendation: yes — the resolver path is shared. NPC behavior tests will need updates if the existing NPC tests pin a specific roll outcome that changes under the corrected 6 × cap.
- RANGE-Q5: Once #251 (smarter NPC movement) lands, the `rangeGoal` (MOVE-11) prefers cells at the NPC's effective range. Does that goal need to know about the penalty curve to avoid sitting at increment 5–6 instead of increment 1? Recommendation: yes — extend MOVE-11 in a follow-on to compute "effective sweet spot" as the cell with the lowest range penalty *and* high cover score; out of scope here, capture as RANGE-F1.

## 7. Acceptance

- [ ] All existing combat tests pass after the cap correction (with comment-marked updates where 4× was pinned).
- [ ] New `RangePenalty` property tests pass.
- [ ] First-increment shot has no penalty; sixth-increment shot has `-10`; beyond-sixth auto-misses.
- [ ] Combat log annotation appears inline with MAP / step annotations.
- [ ] Melee weapon (RangeIncrement = 0) attacks behave identically to before.

## 8. Out-of-Scope Follow-Ons

- RANGE-F1: NPC movement integration with the range curve (per RANGE-Q5).
- RANGE-F2: Volumetric / 3D range modeling.
- RANGE-F3: Per-environment range modifiers.
- RANGE-F4: Range-affecting weapon traits.
- RANGE-F5: Splash / AoE range-increment math.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/263
- Existing inline calc: `internal/game/combat/resolver.go:106-107`
- Existing 4× cap (to remove): `internal/game/combat/round.go:869`
- Distance helper: `internal/game/combat/combat.go:285-298` (`CombatRange`)
- Weapon model: `internal/game/inventory/weapon.go` (`RangeIncrement`)
- Existing range tests: `internal/game/combat/max_range_test.go`
- Combat narrative emitter: `internal/game/combat/round.go:163-199` (`attackNarrative`)
- Sister annotation specs: `docs/superpowers/specs/2026-04-25-agile-and-map.md` AGILE-9, `docs/superpowers/specs/2026-04-25-natural-1-and-20.md` CRIT-17
- NPC movement integration target: `docs/superpowers/specs/2026-04-24-smarter-npc-movement-in-combat.md` MOVE-11
