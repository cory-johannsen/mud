# Cover Bonuses in Combat

**Issue:** [#247 Feature: Cover bonuses in combat](https://github.com/cory-johannsen/mud/issues/247)
**Status:** Spec
**Date:** 2026-04-21
**Depends on:** #245 Duplicate effects handling (merges first)

## 1. Summary

Add grid-derived cover bonuses to combat. For every attack-resolution event the engine walks a Bresenham line from attacker to target and inspects every strictly-intervening cell for a `CoverObject`; the highest tier encountered contributes a `circumstance`-typed bonus to the target's AC (and Quickness) via #245's typed-bonus pipeline. The bonus is ephemeral — applied for the duration of the single attack resolution and removed immediately after so that it does not leak into subsequent attacks.

Cover tier-to-bonus mapping:

| Tier     | AC  | Quickness |
|----------|-----|-----------|
| Lesser   | +1  | 0         |
| Standard | +2  | +2        |
| Greater  | +4  | +4        |

The spec also performs a small but important cleanup: the existing attack resolver confuses "AC bonus on the target" with "attack bonus on the attacker", adding `condition.ACBonus(target)` to the attacker's `AttackTotal`. Cover bonuses cannot paper over that inversion — they require the correct target-side derivation of effective AC. The cleanup goes through every attack call site in `round.go`.

Take Cover action and total-cover / line-block are explicit non-goals here; a follow-up ticket tracks Take Cover.

## 2. Requirements

- COVER-1: Cover tier MUST be computed per attack-resolution event by walking the Bresenham line from the attacker's cell to the target's cell and inspecting each strictly-intervening cell for a `CoverObject`.
- COVER-2: When multiple cover objects lie on the line, the cover routine MUST return the single highest tier encountered (`greater > standard > lesser`); no additive stacking.
- COVER-3: Cover tier-to-bonus mapping MUST be: `lesser = +1 AC`; `standard = +2 AC, +2 Quickness`; `greater = +4 AC, +4 Quickness`.
- COVER-4: Cover bonuses MUST be applied as `circumstance`-typed bonuses on the target's `effect.EffectSet` for the duration of the attack resolution only, and removed before the next resolution begins.
- COVER-5: Cover effects MUST use `SourceID = "cover:<tier>"` and `CasterUID = ""` so #245's dedup and tie-break rules treat them deterministically.
- COVER-6: When attacker and target occupy the same cell or adjacent cells, `DetermineCoverTier` MUST return `NoCover`.
- COVER-7: Cover cells MUST NOT block the line itself; cover is a bonus, not a hard block. (Total cover / full line-block is a future feature.)
- COVER-8: Attack resolution MUST derive effective AC as `target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, StatAC).Total` at every attack call site.
- COVER-9: Attack resolution MUST derive the attacker's effective attack total as `r.AttackTotal + effect.Resolve(actor.Effects, StatAttack).Total` at every attack call site.
- COVER-10: The pre-existing inversion where `condition.ACBonus(target)` was added to `AttackTotal` MUST be removed; every attack call site MUST use the COVER-8 / COVER-9 formulas.
- COVER-11: Cover-absorb-miss logic MUST use the cover AC bonus (not `condition.ACBonus`) to determine whether the shot would have hit without cover.
- COVER-12: When cover absorbs a shot, the degraded cover object MUST be the cover object closest to the attacker on the line.
- COVER-13: Combat narrative MUST annotate attacks that received cover bonuses with `(cover: <tier> +<n> AC)`; absorb-miss events MUST annotate `(cover: <tier>)`.
- COVER-14: Quickness-keyed checks rolled against an incoming effect MUST benefit from the cover bonus via the same `effect.EffectSet` application pattern; the engine MUST provide a `WithCoverEffect` helper so future call sites do not open-code the apply/defer/remove pattern.
- COVER-15: `LineCells` MUST be deterministic and use classic Bresenham (major-axis step, no corner-clip).
- COVER-16: Implementation of #247 MUST NOT start before #245 is merged, because cover bonuses depend on the typed-bonus pipeline.

## 3. Architecture

### 3.1 Module layout

New files in `internal/game/combat/`:

- `grid_line.go` — `GridCell` and `LineCells`. Reusable for any future grid-ray consumer (line-of-sight blocking, AoE templates, visibility).
- `cover_bonus.go` — `CoverTier` enum, `CoverBonus` mapping, `DetermineCoverTier`, `BuildCoverEffect`, `CoverSourceID`, and the `WithCoverEffect` helper.

Existing files touched:

- `internal/game/combat/round.go` — every attack resolution site.
- `internal/game/combat/engine.go` — no structural change; `CoverObject` already carries `Tier`.

### 3.2 Dependency on #245

#247 implementation starts after #245 merges. Cover contributes to `effect.EffectSet` as `circumstance`-typed bonuses; #245's `Resolve` rule is the sole aggregator for AC and Quickness. Until #245 lands, #247 has no place to deposit the bonus.

### 3.3 Resolver flow at each attack site

```go
coverTier := DetermineCoverTier(cbt, actor, target)
if coverTier > NoCover {
    target.Effects.Apply(BuildCoverEffect(coverTier))
}
defer func() {
    if coverTier > NoCover {
        target.Effects.Remove(effectKey{
            SourceID: CoverSourceID(coverTier), CasterUID: "",
        })
    }
}()

ac   := effect.Resolve(target.Effects, effect.StatAC).Total
atk  := effect.Resolve(actor.Effects,  effect.StatAttack).Total
effectiveAC  := target.AC + target.InitiativeBonus + ac
effectiveAtk := r.AttackTotal + atk
r.AttackTotal = effectiveAtk
r.Outcome     = OutcomeFor(effectiveAtk, effectiveAC)
```

The `defer` closes per-target. Multi-target actions (burst, automatic fire) compute the cover tier independently per target; the ephemeral effect never leaks across targets.

### 3.4 AC/attack-total cleanup (the inversion fix)

Today `round.go` adds `condition.ACBonus(target)` into `r.AttackTotal`, which inverts the intended sign when the condition is a genuine AC buff. Cover is the first true circumstance AC buff in the engine, so the inversion cannot remain.

Cleanup touches every attack site (single strike, MAP second strike, burst per-target, automatic per-target, throw) and consistently replaces the legacy `acBonus` thread with `effect.Resolve(...).Total` on the correct side per COVER-8 / COVER-9. The legacy `condition.ACBonus`/`condition.AttackBonus` wrappers already delegate to `effect.Resolve` after #245, so the math flows through a single canonical pipeline.

### 3.5 Cover-absorb-miss rewrite

The existing "attack missed but would have hit without cover" logic in `round.go` at lines 758, 924, 1042, 1231, 1337 is rewritten to use the cover AC bonus directly:

```go
if r.Outcome == Failure || r.Outcome == CritFailure {
    coverAC, _ := CoverBonus(coverTier)
    if coverAC > 0 && effectiveAtk >= effectiveAC - coverAC {
        // Attack would have hit without cover — cover absorbs the shot.
        degraded := closestCoverOnLine(cbt, actor, target)
        emit ActionCoverHit{CoverEquipmentID: degraded.EquipmentID}
        degradeCover(degraded)
    }
}
```

`closestCoverOnLine` returns the first strictly-intervening cover cell encountered walking from the attacker's cell toward the target's. This is independent of which tier "won" the AC contest — it's the physical "shot hit the cover in front of me" model.

### 3.6 Quickness-check integration (forward-compatible)

The engine has no Quickness-check-against-incoming-effect call site today. The spec does not add one; it exposes `WithCoverEffect(cbt, attacker, target, func())` so future call sites (explosive-dodge, burst-evade, AoE saves) can bracket their check with the same apply/defer/remove pattern without open-coding it:

```go
combat.WithCoverEffect(cbt, attacker, target, func() {
    // Roll Quickness check; effect.Resolve(target.Effects, StatQuickness)
    // naturally includes the cover bonus.
})
```

## 4. Line Algorithm and Tier Resolution

### 4.1 Bresenham variant

Classic Bresenham (major-axis step, with occasional diagonal step). The produced path is 8-connected (diagonal transitions are legal steps) and never emits a cell that the line only grazes at a corner — grazes go to one neighbour cell deterministically, not both.

```go
type GridCell struct{ X, Y int }

// LineCells returns the ordered list of grid cells the straight line from
// (ax, ay) to (bx, by) traverses. Endpoints are INCLUDED.
//
// Precondition: any integer coordinates are legal.
// Postcondition: returns a non-empty slice; first cell == (ax, ay);
// last cell == (bx, by); no cell appears twice.
func LineCells(ax, ay, bx, by int) []GridCell
```

### 4.2 Cover tier enum

```go
type CoverTier int

const (
    NoCover  CoverTier = iota
    Lesser
    Standard
    Greater
)

func (t CoverTier) String() string // "none" | "lesser" | "standard" | "greater"
```

### 4.3 Tier → bonus mapping

```go
const (
    CoverACBonusLesser     = 1
    CoverACBonusStandard   = 2
    CoverACBonusGreater    = 4
    CoverQKBonusStandard   = 2
    CoverQKBonusGreater    = 4
)

func CoverBonus(t CoverTier) (acBonus, quicknessBonus int) {
    switch t {
    case Lesser:   return CoverACBonusLesser, 0
    case Standard: return CoverACBonusStandard, CoverQKBonusStandard
    case Greater:  return CoverACBonusGreater,  CoverQKBonusGreater
    default:       return 0, 0
    }
}
```

### 4.4 `DetermineCoverTier`

```go
func DetermineCoverTier(cbt *Combat, attacker, target *Combatant) CoverTier {
    cells := LineCells(attacker.GridX, attacker.GridY, target.GridX, target.GridY)
    if len(cells) <= 2 { // same cell or adjacent
        return NoCover
    }
    best := NoCover
    for _, c := range cells[1 : len(cells)-1] {
        for _, obj := range cbt.CoverObjects {
            if obj.GridX == c.X && obj.GridY == c.Y {
                if t := tierFromString(obj.Tier); t > best {
                    best = t
                }
            }
        }
    }
    return best
}
```

`tierFromString` maps `"lesser" | "standard" | "greater"` to the enum; unknown strings degrade to `NoCover` and are logged once per process lifetime.

### 4.5 Ephemeral effect construction

```go
func CoverSourceID(t CoverTier) string { return "cover:" + t.String() }

func BuildCoverEffect(t CoverTier) effect.Effect {
    ac, qk := CoverBonus(t)
    bonuses := []effect.Bonus{}
    if ac != 0 {
        bonuses = append(bonuses, effect.Bonus{
            Stat: effect.StatAC, Value: ac, Type: effect.BonusTypeCircumstance,
        })
    }
    if qk != 0 {
        bonuses = append(bonuses, effect.Bonus{
            Stat: effect.StatQuickness, Value: qk, Type: effect.BonusTypeCircumstance,
        })
    }
    return effect.Effect{
        EffectID:   "cover_" + t.String(),
        SourceID:   CoverSourceID(t),
        CasterUID:  "",
        Bonuses:    bonuses,
        DurKind:    effect.DurationUntilRemove,
        Annotation: "cover (" + t.String() + ")",
    }
}
```

### 4.6 Performance

`DetermineCoverTier` is O(cells × cover_objects). Typical combats have ≤ 3 cover objects on short lines; no indexing is required for v1.

## 5. Narrative

- Attacks that received a cover bonus append `(cover: <tier> +<n> AC)` to the attack narrative:
  ```
  Kira slashes the thug for 0 damage. (miss, DC 18) (cover: standard +2 AC)
  ```
- Cover-absorb-miss events emit the existing `ActionCoverHit` narrative plus a tier annotation:
  ```
  The sniper fires at you but the crate absorbs the shot! (cover: standard)
  ```
- When both cover-absorb and cover-contributed-to-miss would apply, the absorb message wins (it is the more informative of the two).

## 6. Edge-Case Policy

- Cover on an endpoint cell: ignored. Only strictly-intervening cells count.
- Melee: adjacent cells have no intervening cells; `DetermineCoverTier` returns `NoCover`. No special-casing required.
- Same cell: defensive `NoCover` return.
- Corner-clip: Bresenham's chosen variant does not emit corner-only touches; no cover bonus from a cell the line only grazes.
- Multiple covers on the line: highest tier wins for AC contribution; closest-to-attacker cover takes any absorb-miss degradation hit.
- Total cover / fully line-blocking cover: out of scope.

## 7. Testing

Per SWENG-5 / SWENG-5a, TDD with property-based tests where appropriate.

- `internal/game/combat/grid_line_test.go` — property tests:
  - Endpoints always present.
  - Reversing start/end yields the reversed cell list.
  - Horizontal, vertical, and 45° diagonals yield the expected traversals.
  - No cell appears twice.
- `internal/game/combat/cover_bonus_test.go` — property tests:
  - Tier → bonus mapping matches COVER-3 exactly.
  - `BuildCoverEffect(NoCover)` yields no bonuses.
  - Highest-tier-wins across shuffled input orderings.
  - Same-cell and adjacent-cell returns `NoCover`.
- `internal/game/combat/cover_integration_test.go` — end-to-end:
  - Single target + one standard cover on the line → +2 AC, narrative annotated.
  - Burst fire against two targets where only one has cover — per-target independent, no leakage.
  - Attack miss within cover AC band → `ActionCoverHit` on the closest-to-attacker cover.
  - Attack miss outside the cover AC band → no absorb event.
  - Two covers on the line (lesser + standard) → target gets +2 AC; closest-to-attacker object takes any absorb hit.
- `internal/game/combat/cover_inversion_fix_test.go` — regression golden tests verifying the attacker-side / target-side AC derivation is now correct; a condition granting a positive AC bonus to the target raises the target's effective AC instead of the attacker's roll. Docstrings call out the intentional behavior change.

## 8. Documentation

- `docs/architecture/combat.md` — new "Cover" subsection hosting `COVER-N` requirements, the tier-to-bonus table, and a short note on the AC-derivation cleanup.
- `docs/architecture/combat.md` — narrative section updated with the cover annotation format.
- Content-authoring documentation — document that `CoverObject.Tier` values `"lesser" | "standard" | "greater"` are authoritative and that the authoring tool for room equipment is the primary surface for assigning tier.

## 9. Non-Goals (v1)

- No Take Cover action. Separate follow-up ticket.
- No total cover / full line-block.
- No directional cover (a cover object that blocks only attacks from specific directions).
- No cover-derived concealment (miss chance). Cover gives AC, not miss %.
- No visibility / line-of-sight system. Cover is AC-only.
- No Quickness-check call site added by this spec; the integration surface is prepared and documented.
- No grid-adjacent "behind cover" state — the Bresenham ray handles it.

## 10. Open Questions for the Planner

- Exact location of `LineCells` — standalone file `grid_line.go` in `internal/game/combat/`, or a new `internal/game/combat/grid/` subpackage.
- Whether the web client surfaces cover tier on the combat grid UI before the attack fires (preview) or only in the narrative after the fact.
- Whether the cover-absorb degradation event should record the cover tier explicitly (it already records the equipment-id).

## 11. Follow-On Work

- **Take Cover action (`ActionTakeCover`):** elevates current cover one tier (`lesser → standard`, `standard → greater`) until the player moves or attacks. Requires a new `ActionType`, cost, condition, and UX. Tracked by the follow-up ticket created alongside this spec.
