# Agile Weapon Trait + MAP Reduction — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the existing `agile` weapon trait into MAP computation. When the wielder's currently-active weapon has the trait, MAP becomes `-4 / -8` instead of `-5 / -10`. Annotate every MAP-affected attack in the combat log with the penalty value, ordinal, and contributing traits. The trait registry from #253 is the single source of truth.

**Spec:** [docs/superpowers/specs/2026-04-25-agile-and-map.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-agile-and-map.md) (PR [#289](https://github.com/cory-johannsen/mud/pull/289))

**Architecture:** Three small surgical changes. (1) `traits.Agile = "agile"` is added to the registry from #253 with a `MAPReductionSteps int = 1` `Behavior` field. (2) `mapPenaltyFor(attacksMade int)` is refactored to `mapPenaltyFor(attacksMade int, weapon *inventory.WeaponDef) int` and consults the registry: each step reduces both `-5` and `-10` by `5` (clamped at zero). All callers are updated to pass `attacker.EquippedWeapon()`. (3) `attackNarrative` emits a `[MAP -X: <ordinal> attack<, agile?>]` suffix on every attack with `MAP > 0`, listing every contributing trait id. The wielder's current weapon at attack-resolution time decides the penalty (AGILE-8) — weapon swap mid-turn picks up the new trait set.

**Tech Stack:** Go (`internal/game/inventory/traits/`, `internal/game/combat/`).

**Prerequisite:** #253 (Move trait) carries the trait registry. This plan adds `Agile` to the same registry. If #253 has not landed yet, this plan creates the registry alongside the `Move` and `Agile` constants and the `Behavior` struct fields both specs need.

**Note on spec PR**: Spec is on PR #289, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/game/inventory/traits/registry.go` (`traits.Agile`; `Behavior.MAPReductionSteps`) |
| Modify | `internal/game/inventory/traits/registry_test.go` |
| Modify | `internal/game/combat/round.go` (`mapPenaltyFor` signature + agile branch; `attackNarrative` annotation) |
| Create | `internal/game/combat/round_agile_map_test.go` |
| Modify | `internal/game/combat/round_test.go` (any tests pinning the old signature) |
| Modify | `internal/gameserver/combat_handler.go` (no expected callers; verify) |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: Registry — `traits.Agile` + `MAPReductionSteps`

**Files:**
- Modify: `internal/game/inventory/traits/registry.go`
- Modify: `internal/game/inventory/traits/registry_test.go`

- [ ] **Step 1: Failing tests** (AGILE-1, AGILE-2, AGILE-3):

```go
func TestRegistry_AgileBehavior(t *testing.T) {
    r := traits.DefaultRegistry()
    b := r.Behavior(traits.Agile)
    require.NotNil(t, b)
    require.Equal(t, "Agile", b.DisplayName)
    require.Equal(t, 1, b.MAPReductionSteps)
}

func TestRegistry_BothMoveAndAgilePresent(t *testing.T) {
    r := traits.DefaultRegistry()
    require.NotNil(t, r.Behavior(traits.Mobile))
    require.NotNil(t, r.Behavior(traits.Agile))
}

func TestWeaponDef_HasTrait_Agile(t *testing.T) {
    w := &inventory.WeaponDef{Traits: []string{"agile"}}
    require.True(t, w.HasTrait(traits.Agile))
    require.False(t, w.HasTrait(traits.Mobile))
}
```

- [ ] **Step 2: Implement**:

```go
const Agile = "agile"

type Behavior struct {
    // ... existing from #253 ...
    MAPReductionSteps int
}

// In DefaultRegistry():
Agile: {
    ID:                Agile,
    DisplayName:       "Agile",
    Description:       "Reduces multiple-attack penalty by one step.",
    MAPReductionSteps: 1,
},
```

`HasTrait` from #253 already canonicalises trait ids; no new helper needed.

---

### Task 2: `mapPenaltyFor` weapon-aware refactor

**Files:**
- Modify: `internal/game/combat/round.go`
- Create: `internal/game/combat/round_agile_map_test.go`
- Modify: `internal/game/combat/round_test.go` (any tests using the old signature)

- [ ] **Step 1: Failing tests** (AGILE-4..8, AGILE-13, AGILE-14, AGILE-15):

```go
func TestMapPenaltyFor_NonAgileDefault(t *testing.T) {
    require.Equal(t, 0,   combat.MapPenaltyFor(1, nil))
    require.Equal(t, -5,  combat.MapPenaltyFor(2, nil))
    require.Equal(t, -10, combat.MapPenaltyFor(3, nil))
    require.Equal(t, -10, combat.MapPenaltyFor(5, nil))
}

func TestMapPenaltyFor_AgileSecondAttackMinus4(t *testing.T) {
    w := agileWeaponDef(t)
    require.Equal(t, 0,  combat.MapPenaltyFor(1, w))
    require.Equal(t, -4, combat.MapPenaltyFor(2, w))
    require.Equal(t, -8, combat.MapPenaltyFor(3, w))
}

func TestMapPenaltyFor_TwoStepReduction_FutureProof(t *testing.T) {
    w := weaponWithMAPReductionSteps(t, 2) // synthetic, future-proof check
    require.Equal(t, -3, combat.MapPenaltyFor(2, w))
    require.Equal(t, -6, combat.MapPenaltyFor(3, w))
}

func TestMapPenaltyFor_StepsClampAtZero(t *testing.T) {
    w := weaponWithMAPReductionSteps(t, 5)
    require.Equal(t, 0, combat.MapPenaltyFor(2, w))
    require.Equal(t, 0, combat.MapPenaltyFor(3, w))
}

func TestResolveRound_CrossAction_AgileMAP_SecondAttackAt4(t *testing.T) {
    cbt := buildCombatWithAgileWielder(t)
    queueAttack(cbt, attacker, target)
    queueAttack(cbt, attacker, target)
    combat.ResolveRound(cbt)
    require.Equal(t, -4, secondAttackMapPenalty(cbt))
}

func TestResolveRound_StrikeAgile_SecondAttackAt4(t *testing.T) { ... }

func TestResolveRound_AgileMAP_ResetsAtRoundStart(t *testing.T) {
    cbt := buildCombatWithAgileWielder(t)
    queueAttack(cbt, attacker, target)
    queueAttack(cbt, attacker, target)
    combat.ResolveRound(cbt)
    cbt.StartRound()
    queueAttack(cbt, attacker, target)
    combat.ResolveRound(cbt)
    require.Equal(t, 0, latestAttackMapPenalty(cbt))
}

func TestWeaponSwapMidTurn_PerAttackTimeReadingApplies(t *testing.T) {
    cbt := buildCombat(t, withAttacker(equipped: "non_agile_pistol"))
    queueAttack(cbt, attacker, target)
    swapToAgile := func(c *combat.Combat) { attacker.EquipWeapon("agile_blade") }
    queueAttackThen(cbt, attacker, target, swapToAgile)
    queueAttack(cbt, attacker, target) // third attack with agile blade
    combat.ResolveRound(cbt)

    require.Equal(t, 0,  attackPenalty(cbt, 1)) // first
    require.Equal(t, -4, attackPenalty(cbt, 2)) // second after swap → agile -4
    require.Equal(t, -8, attackPenalty(cbt, 3)) // third with agile → -8
}
```

- [ ] **Step 2: Implement**:

```go
func MapPenaltyFor(attacksMade int, weapon *inventory.WeaponDef) int {
    base := 0
    switch {
    case attacksMade <= 1:
        return 0
    case attacksMade == 2:
        base = -5
    default:
        base = -10
    }
    steps := 0
    if weapon != nil {
        // Sum reduction across all traits the weapon carries.
        for _, t := range weapon.Traits {
            if b := traits.DefaultRegistry().Behavior(t); b != nil {
                steps += b.MAPReductionSteps
            }
        }
    }
    reduced := base + 5*steps
    if reduced > 0 { reduced = 0 } // clamp
    return reduced
}
```

- [ ] **Step 3: Update all call sites** in `round.go` to pass `attacker.EquippedWeapon().Def()` (or `nil` for unarmed). The function name keeps the lowercase `mapPenaltyFor` if it stays package-private; export to `MapPenaltyFor` only if other packages need it (the spec keeps it package-local).

- [ ] **Step 4: Existing tests** at `round_conditions_test.go:143-221` and `round_test.go` use weapons without the agile trait, so the default `-5 / -10` continues to apply. Verify they pass after the signature change.

---

### Task 3: Combat log annotation — `[MAP -X: <ordinal> attack<, traits>]`

**Files:**
- Modify: `internal/game/combat/round.go` (`attackNarrative`)
- Modify: `internal/game/combat/round_agile_map_test.go`

- [ ] **Step 1: Failing tests** (AGILE-9, AGILE-10, AGILE-11, AGILE-16):

```go
func TestAttackNarrative_NoMAPAnnotationOnFirstAttack(t *testing.T) {
    out := attackNarrativeOf(t, attacksMade: 1, weapon: nil)
    require.NotContains(t, out, "[MAP")
}

func TestAttackNarrative_DefaultMAPAnnotation(t *testing.T) {
    out := attackNarrativeOf(t, attacksMade: 2, weapon: nil)
    require.Contains(t, out, "[MAP -5: 2nd attack]")
}

func TestAttackNarrative_AgileMAPAnnotation(t *testing.T) {
    w := agileWeaponDef(t)
    out := attackNarrativeOf(t, attacksMade: 2, weapon: w)
    require.Contains(t, out, "[MAP -4: 2nd attack, agile]")
}

func TestAttackNarrative_ThirdAttackOrdinal(t *testing.T) {
    out := attackNarrativeOf(t, attacksMade: 3, weapon: nil)
    require.Contains(t, out, "[MAP -10: 3rd attack]")
}

func TestAttackNarrative_FutureMultipleTraitsListed(t *testing.T) {
    // AGILE-Q2: when multiple MAP-reducing traits exist, all are named.
    w := weaponWithTraits(t, []string{"agile", "future_nimble_with_map_steps_1"})
    out := attackNarrativeOf(t, attacksMade: 2, weapon: w)
    require.Contains(t, out, "[MAP -3: 2nd attack, agile, future_nimble_with_map_steps_1]")
}
```

- [ ] **Step 2: Implement** the annotation in `attackNarrative`:

```go
func attackNarrative(...) string {
    base := existingNarrative(...)
    if penalty == 0 { return base }
    ord := ordinalSuffix(attacksMade) // "2nd", "3rd", ...
    var traitNames []string
    if weapon != nil {
        for _, t := range weapon.Traits {
            if b := traits.DefaultRegistry().Behavior(t); b != nil && b.MAPReductionSteps > 0 {
                traitNames = append(traitNames, t)
            }
        }
    }
    annotation := fmt.Sprintf("[MAP %d: %s attack", penalty, ord)
    if len(traitNames) > 0 {
        annotation += ", " + strings.Join(traitNames, ", ")
    }
    annotation += "]"
    return base + " " + annotation
}
```

- [ ] **Step 3:** The annotation is appended to the existing `1d20 (N) ±mod = total vs AC X` breakdown line so players see it inline (AGILE-10).

---

### Task 4: Architecture documentation update

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Extend the existing "Weapon Traits" section** (added by #253) with a subsection on the `agile` trait. Document:
  - The MAP reduction rule (`-5/-10` → `-4/-8`).
  - The wielder's-current-weapon-decides rule (AGILE-8).
  - The combat-log annotation format with examples.
  - The future-proofing of `MAPReductionSteps`.

- [ ] **Step 2: Cross-link** to `internal/game/inventory/traits/registry.go`, the spec, and the predecessor #253.

---

## Verification

```
go test ./...
```

Additional sanity:

- `go vet ./...` clean.
- Telnet smoke test: equip a non-agile weapon, attack twice in one round, verify `[MAP -5: 2nd attack]` annotation; equip an agile weapon (e.g., `combat_knife`), attack twice, verify `[MAP -4: 2nd attack, agile]`; attack a third time, verify `[MAP -8: 3rd attack, agile]`.

---

## Rollout / Open Questions Resolved at Plan Time

- **AGILE-Q1**: Off-hand attack uses the off-hand weapon's traits. The wielder's *attacking* weapon at attack-resolution time decides.
- **AGILE-Q2**: All MAP-reducing traits are listed in the annotation (comma-separated). v1 only has `agile`.
- **AGILE-Q3**: `mapPenaltyFor` signature changes in one PR. Only `round.go` consumes it.
- **AGILE-Q4**: Registry validation warns on unknown trait ids; no warning fires for stock content because the registry is built before content loads.
- **AGILE-Q5**: Maneuvers (Trip / Disarm / Shove from #252) stay at default MAP in v1. Folded in when the skill-action resolver wires up MAP-counting.

## Non-Goals Reaffirmed

Per spec §2.2:

- No other weapon traits (`forceful` / `sweep` / `backswing` / etc.).
- No change to `AttacksMadeThisRound` counter semantics.
- No per-weapon MAP profiles authored directly in YAML.
- No asymmetric MAP across weapon-swap.
- No MAP-modifying feats.
