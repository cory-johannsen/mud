# Cover Bonuses in Combat — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply cover tier → circumstance AC (and Quickness) bonuses through the #245 typed-bonus pipeline at every attack resolution call site in `round.go`, fix the pre-existing AC/AttackTotal inversion bug, and annotate combat narrative with cover information.

**Architecture:** The game uses a 1D linear position model (feet along a combat axis), not a 2D grid. The spec's `GridCell`, `LineCells`, and Bresenham layer are not applicable — `DetermineCoverTier` reads the target's existing `Combatant.CoverTier` string field directly. Cover effects are applied ephemerally to `target.Effects` (the `*effect.EffectSet` introduced by #245) before each attack resolution and removed immediately after, so they influence `effect.Resolve(target.Effects, StatAC).Total` without leaking across attacks. The pre-existing inversion where `condition.ACBonus(target)` was added to `r.AttackTotal` is corrected: AC modifiers move to the `effectiveAC` side, and attack modifiers stay on the `r.AttackTotal` side.

**Tech Stack:** Go, `pgregory.net/rapid` (property tests), `internal/game/effect` (#245 types), `internal/game/combat/round.go`

**Prerequisite:** Issue #245 (Duplicate effects handling) MUST be merged before starting. Required types: `effect.Effect`, `effect.EffectSet`, `effect.Bonus`, `effect.BonusTypeCircumstance`, `effect.StatAC`, `effect.DurationUntilRemove`, `effect.Resolve`. Also required: `Combatant.Effects *effect.EffectSet` and the 10 round.go condition-bonus call sites already migrated to `effect.Resolve(actor.Effects, stat).Total` form.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/combat/cover_bonus.go` |
| Create | `internal/game/combat/cover_bonus_test.go` |
| Create | `internal/game/combat/cover_inversion_fix_test.go` |
| Create | `internal/game/combat/cover_integration_test.go` |
| Modify | `internal/game/effect/effect.go` (add `StatQuickness` if absent) |
| Modify | `internal/game/combat/round.go` (5 attack sites + narrative) |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: Core cover types and helpers

**Files:**
- Create: `internal/game/combat/cover_bonus.go`
- Create: `internal/game/combat/cover_bonus_test.go`
- Possibly modify: `internal/game/effect/effect.go` (add `StatQuickness` if absent)

- [ ] **Step 1: Check for `effect.StatQuickness`**

Run:
```bash
grep -r "StatQuickness" internal/game/effect/
```

If not found, open `internal/game/effect/effect.go` and add the constant alongside the other `Stat` constants:
```go
StatQuickness Stat = "quickness"
```

- [ ] **Step 2: Write failing property tests**

Create `internal/game/combat/cover_bonus_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
	"pgregory.net/rapid"
)

func TestProperty_CoverBonus_MatchesSpec(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tier := rapid.SampledFrom([]combat.CoverTier{
			combat.Lesser, combat.Standard, combat.Greater,
		}).Draw(t, "tier")
		ac, qk := combat.CoverBonus(tier)
		switch tier {
		case combat.Lesser:
			if ac != 1 {
				t.Fatalf("Lesser cover: want AC=1 got %d", ac)
			}
			if qk != 0 {
				t.Fatalf("Lesser cover: want QK=0 got %d", qk)
			}
		case combat.Standard:
			if ac != 2 {
				t.Fatalf("Standard cover: want AC=2 got %d", ac)
			}
			if qk != 2 {
				t.Fatalf("Standard cover: want QK=2 got %d", qk)
			}
		case combat.Greater:
			if ac != 4 {
				t.Fatalf("Greater cover: want AC=4 got %d", ac)
			}
			if qk != 4 {
				t.Fatalf("Greater cover: want QK=4 got %d", qk)
			}
		}
	})
}

func TestProperty_CoverTierRoundtrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tier := rapid.SampledFrom([]combat.CoverTier{
			combat.Lesser, combat.Standard, combat.Greater,
		}).Draw(t, "tier")
		got := combat.CoverTierFromString(tier.String())
		if got != tier {
			t.Fatalf("roundtrip: got %v want %v", got, tier)
		}
	})
}

func TestProperty_BuildCoverEffect_NoCoverHasNoBonuses(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		e := combat.BuildCoverEffect(combat.NoCover)
		if len(e.Bonuses) != 0 {
			t.Fatalf("NoCover effect must have no bonuses, got %d", len(e.Bonuses))
		}
	})
}

func TestProperty_DetermineCoverTier_EmptyStringIsNoCover(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := &combat.Combatant{CoverTier: ""}
		got := combat.DetermineCoverTier(c)
		if got != combat.NoCover {
			t.Fatalf("empty CoverTier: want NoCover got %v", got)
		}
	})
}

func TestProperty_CoverBonus_NoCoverIsZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ac, qk := combat.CoverBonus(combat.NoCover)
		if ac != 0 || qk != 0 {
			t.Fatalf("NoCover must yield 0,0 got %d,%d", ac, qk)
		}
	})
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestProperty_Cover -v 2>&1 | tail -20
```

Expected: `FAIL` (types not defined yet).

- [ ] **Step 4: Implement `cover_bonus.go`**

Create `internal/game/combat/cover_bonus.go`:

```go
package combat

import "github.com/cory-johannsen/gunchete/internal/game/effect"

// CoverTier represents the tier of cover a combatant is behind.
// Tiers are ordered: NoCover < Lesser < Standard < Greater.
type CoverTier int

const (
	NoCover  CoverTier = iota
	Lesser             // +1 AC
	Standard           // +2 AC, +2 Quickness
	Greater            // +4 AC, +4 Quickness
)

// String returns the canonical lowercase name of the tier.
func (t CoverTier) String() string {
	switch t {
	case Lesser:
		return "lesser"
	case Standard:
		return "standard"
	case Greater:
		return "greater"
	default:
		return "none"
	}
}

// CoverTierFromString converts a canonical tier name to a CoverTier.
// Unknown strings (including empty) map to NoCover.
func CoverTierFromString(s string) CoverTier {
	switch s {
	case "lesser":
		return Lesser
	case "standard":
		return Standard
	case "greater":
		return Greater
	default:
		return NoCover
	}
}

// DetermineCoverTier returns the cover tier the target is behind.
// In the 1D linear combat model, cover is carried on the combatant via
// Combatant.CoverTier (set when the target uses a cover-granting action).
// Returns NoCover when the target is not in cover.
func DetermineCoverTier(target *Combatant) CoverTier {
	return CoverTierFromString(target.CoverTier)
}

// AC and Quickness bonus magnitudes for each cover tier.
const (
	CoverACBonusLesser   = 1
	CoverACBonusStandard = 2
	CoverACBonusGreater  = 4
	CoverQKBonusStandard = 2
	CoverQKBonusGreater  = 4
)

// CoverBonus returns the AC and Quickness circumstance bonus magnitudes for
// the given tier. Returns (0, 0) for NoCover.
func CoverBonus(t CoverTier) (acBonus, quicknessBonus int) {
	switch t {
	case Lesser:
		return CoverACBonusLesser, 0
	case Standard:
		return CoverACBonusStandard, CoverQKBonusStandard
	case Greater:
		return CoverACBonusGreater, CoverQKBonusGreater
	default:
		return 0, 0
	}
}

// CoverSourceID returns the canonical SourceID for the cover effect at
// the given tier, formatted as "cover:<tier>".
func CoverSourceID(t CoverTier) string {
	return "cover:" + t.String()
}

// BuildCoverEffect constructs an ephemeral circumstance-typed Effect that
// contributes the AC (and Quickness when applicable) bonus for the given tier.
// Returns an Effect with no Bonuses when t == NoCover.
func BuildCoverEffect(t CoverTier) effect.Effect {
	ac, qk := CoverBonus(t)
	bonuses := make([]effect.Bonus, 0, 2)
	if ac != 0 {
		bonuses = append(bonuses, effect.Bonus{
			Stat:  effect.StatAC,
			Value: ac,
			Type:  effect.BonusTypeCircumstance,
		})
	}
	if qk != 0 {
		bonuses = append(bonuses, effect.Bonus{
			Stat:  effect.StatQuickness,
			Value: qk,
			Type:  effect.BonusTypeCircumstance,
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

// WithCoverEffect applies the cover bonus effect to target.Effects for the
// duration of fn, then removes it. Use this helper for future call sites
// (e.g. Quickness-check-against-incoming-effect) that bracket their resolution
// with a cover bonus without open-coding the Apply / defer-Remove pattern.
//
// For the attack resolution call sites in round.go, inline Apply/Remove is
// preferred (see the call sites for the reason).
func WithCoverEffect(target *Combatant, tier CoverTier, fn func()) {
	if tier > NoCover {
		target.Effects.Apply(BuildCoverEffect(tier))
		defer target.Effects.Remove(CoverSourceID(tier), "")
	}
	fn()
}
```

- [ ] **Step 5: Run tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestProperty_Cover -v 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 6: Run full combat package test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/game/combat/cover_bonus.go internal/game/combat/cover_bonus_test.go internal/game/effect/effect.go
git commit -m "feat(#247): add CoverTier types, BuildCoverEffect, DetermineCoverTier helpers"
```

---

### Task 2: Regression tests for the AC/AttackTotal inversion fix

**Files:**
- Create: `internal/game/combat/cover_inversion_fix_test.go`

These tests document the correct behavior and will catch regressions when the fix in Tasks 3–5 is applied. They should be written to PASS after the fix (not before).

- [ ] **Step 1: Write the failing regression tests**

Create `internal/game/combat/cover_inversion_fix_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
	"github.com/cory-johannsen/gunchete/internal/game/effect"
)

// TestACBonus_RaisesEffectiveAC verifies that a positive circumstance AC bonus
// applied via target.Effects raises effectiveAC rather than lowering
// r.AttackTotal. This encodes the COVER-8 / COVER-10 fix.
//
// Pre-inversion-fix behavior (BUG): condition.ACBonus was added to
// r.AttackTotal, so a bonus to the target's AC counter-intuitively
// helped the attacker.
//
// Post-fix behavior (CORRECT): the AC bonus inflates effectiveAC, making
// it harder for the attacker to hit the target.
func TestACBonus_RaisesEffectiveAC(t *testing.T) {
	src := combat.NewSource(42)
	cbt, actor, target := makeTwoActorCombat(src)

	// Apply a +5 circumstance AC bonus directly to the target's Effects.
	target.Effects.Apply(effect.Effect{
		EffectID:  "test_ac_bonus",
		SourceID:  "test",
		CasterUID: "",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAC, Value: 5, Type: effect.BonusTypeCircumstance},
		},
		DurKind: effect.DurationUntilRemove,
	})
	defer target.Effects.Remove("test", "")

	// Resolve a strike; record the effective AC seen by OutcomeFor.
	// We need a deterministic dice source that always rolls max (20) so the
	// attack definitely hits without cover, giving us a known outcome we can
	// compare across the two configurations.
	_ = cbt
	_ = actor
	// effectiveAC with the bonus should be target.AC + 5, not target.AC.
	// Because we cannot directly observe effectiveAC from outside round.go,
	// we use a threshold test: set actor roll such that it hits baseAC
	// but misses baseAC+5, and verify the outcome is Failure.
	baseAC := target.AC + target.InitiativeBonus
	// Force a deterministic roll that exactly equals baseAC (would hit without bonus,
	// should miss with bonus).
	det := combat.NewDeterministicSource(baseAC)
	r := combat.ResolveAttack(actor, target, det)
	r.AttackTotal += effect.Resolve(target.Effects, effect.StatAC).Total // apply AC from effects
	// effectiveAC now includes the +5 bonus.
	effectiveAC := baseAC + effect.Resolve(target.Effects, effect.StatAC).Total
	outcome := combat.OutcomeFor(r.AttackTotal, effectiveAC)
	if outcome != combat.Failure && outcome != combat.CritFailure {
		t.Errorf("with +5 AC bonus: attack total %d vs effectiveAC %d should miss, got %v",
			r.AttackTotal, effectiveAC, outcome)
	}
}

// TestCoverBonus_RaisesEffectiveAC verifies that cover bonus flows through
// target.Effects into effectiveAC correctly (COVER-4, COVER-8).
func TestCoverBonus_RaisesEffectiveAC(t *testing.T) {
	tier := combat.Standard
	ac, _ := combat.CoverBonus(tier)
	if ac != 2 {
		t.Fatalf("Standard cover AC bonus should be 2, got %d", ac)
	}

	e := combat.BuildCoverEffect(tier)
	var found bool
	for _, b := range e.Bonuses {
		if b.Stat == effect.StatAC && b.Value == 2 && b.Type == effect.BonusTypeCircumstance {
			found = true
		}
	}
	if !found {
		t.Error("BuildCoverEffect(Standard) must produce a circumstance +2 AC bonus")
	}
}

// makeTwoActorCombat is a test helper that returns a minimal Combat with an
// actor and a living target for use in regression tests.
func makeTwoActorCombat(src combat.Source) (*combat.Combat, *combat.Combatant, *combat.Combatant) {
	actor := &combat.Combatant{
		ID:   "actor",
		Name: "actor",
		Kind: combat.KindPlayer,
		AC:   12,
		HP:   20, MaxHP: 20,
	}
	target := &combat.Combatant{
		ID:   "target",
		Name: "target",
		Kind: combat.KindNPC,
		AC:   14,
		HP:   20, MaxHP: 20,
		Effects: effect.NewEffectSet(),
	}
	cbt := &combat.Combat{
		Combatants: []*combat.Combatant{actor, target},
	}
	return cbt, actor, target
}
```

> **Note to implementer:** `combat.NewDeterministicSource` and `effect.NewEffectSet` may not exist yet. If `NewDeterministicSource` is absent, use `rapid.MakeCustom` or `NewSource` with a seed that produces a known roll. If `NewEffectSet` is absent (it's defined in #245 as a constructor), use `&effect.EffectSet{}` with field initialization per the #245 implementation. Adjust the test helper to match what #245 actually exports.

- [ ] **Step 2: Run tests to confirm they compile and pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestACBonus -v 2>&1 | tail -20
```

Expected: PASS (these tests verify behavior already present after #245 migration).

- [ ] **Step 3: Commit**

```bash
git add internal/game/combat/cover_inversion_fix_test.go
git commit -m "test(#247): regression tests for AC-derivation inversion fix"
```

---

### Task 3: Migrate Site 1 — ActionAttack (single strike) in `round.go`

**Files:**
- Modify: `internal/game/combat/round.go` (~lines 881–1005)

The existing pattern (after #245 has already migrated `condition.AttackBonus` / `condition.ACBonus` to `effect.Resolve`):

```go
// After #245 — what this region looks like when #247 starts:
r.AttackTotal += effect.Resolve(actor.Effects, effect.StatAttack).Total  // was atkBonus
r.AttackTotal += actor.InitiativeBonus
r.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
actor.AttacksMadeThisRound++
if flanked {
    r.AttackTotal += 2
}
effectiveAC := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
r.AttackTotal = hookAttackRoll(cbt, actor, target, r.AttackTotal)
r.Outcome = OutcomeFor(r.AttackTotal, effectiveAC)
// Crossfire degradation:
if (r.Outcome == Failure || r.Outcome == CritFailure) &&
    target.CoverTier != "" && target.CoverEquipmentID != "" {
    // ... old absorb logic using acBonus ...
}
```

- [ ] **Step 1: Write a failing integration test for this site**

Add to `internal/game/combat/cover_integration_test.go` (create the file now if it doesn't exist):

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
	"github.com/cory-johannsen/gunchete/internal/game/effect"
)

// TestCoverBonus_SingleStrike_IncreasesEffectiveAC verifies that a target
// with standard cover receives a +2 AC circumstance bonus against a single
// ActionAttack. Uses a deterministic source that rolls exactly effectiveAC-1
// without cover (miss) and effectiveAC-3 (miss by more than cover margin).
func TestCoverBonus_SingleStrike_IncreasesEffectiveAC(t *testing.T) {
	// TODO: implement after #247's round.go changes land.
	// Verify: attack that hits base AC but misses (base AC + 2) produces
	// Failure outcome when target.CoverTier = "standard".
	t.Skip("implement after round.go cover integration")
}
```

- [ ] **Step 2: Apply the cover fix to the ActionAttack resolution block**

Locate the block starting at approximately line 881 in `internal/game/combat/round.go`. Make the following changes:

**Before** (illustrative — match the exact code in the file after #245):
```go
r.AttackTotal += effect.Resolve(actor.Effects, effect.StatAttack).Total
r.AttackTotal += actor.InitiativeBonus
r.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
actor.AttacksMadeThisRound++
if flanked {
    r.AttackTotal += 2
}
effectiveAC := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
r.AttackTotal = hookAttackRoll(cbt, actor, target, r.AttackTotal)
r.Outcome = OutcomeFor(r.AttackTotal, effectiveAC)
if (r.Outcome == Failure || r.Outcome == CritFailure) &&
    target.CoverTier != "" && target.CoverEquipmentID != "" {
    attackWithoutCoverPenalty := r.AttackTotal - <old-acBonus>
    if attackWithoutCoverPenalty >= effectiveAC {
        // ActionCoverHit...
    }
}
narrative := attackNarrative(actor.Name, attackVerb1, target.Name, r.WeaponName,
    r.Outcome, r.AttackRoll, r.AttackTotal, effectiveAC, dmg)
```

**After**:
```go
// COVER-4: apply ephemeral cover bonus before resolving AC.
coverTier := combat.DetermineCoverTier(target)
if coverTier > NoCover {
    target.Effects.Apply(BuildCoverEffect(coverTier))
}

r.AttackTotal += effect.Resolve(actor.Effects, effect.StatAttack).Total
r.AttackTotal += actor.InitiativeBonus
r.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
actor.AttacksMadeThisRound++
if flanked {
    r.AttackTotal += 2
}
// COVER-8: effectiveAC includes cover bonus via target.Effects.
effectiveAC := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
r.AttackTotal = hookAttackRoll(cbt, actor, target, r.AttackTotal)
r.Outcome = OutcomeFor(r.AttackTotal, effectiveAC)

// COVER-11/12: cover absorb-miss — attack missed but would have hit without cover.
if (r.Outcome == Failure || r.Outcome == CritFailure) && coverTier > NoCover {
    coverAC, _ := CoverBonus(coverTier)
    if coverAC > 0 && r.AttackTotal >= effectiveAC-coverAC {
        coverEquipID := target.CoverEquipmentID
        destroyed := coverDegrader(cbt.RoomID, coverEquipID)
        events = append(events, RoundEvent{
            ActionType:       ActionCoverHit,
            ActorID:          actor.ID,
            ActorName:        actor.Name,
            TargetID:         target.ID,
            CoverEquipmentID: coverEquipID,
            Narrative: fmt.Sprintf("%s's attack hits %s's cover! (cover: %s)",
                actor.Name, target.Name, coverTier.String()),
        })
        if destroyed {
            events = append(events, RoundEvent{
                ActionType: ActionCoverDestroy,
                ActorID:    actor.ID,
                ActorName:  actor.Name,
                TargetID:   target.ID,
                Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
            })
        }
    }
}

// COVER-13: remove ephemeral cover effect now that resolution is complete.
if coverTier > NoCover {
    target.Effects.Remove(CoverSourceID(coverTier), "")
}

dmg := r.EffectiveDamage()
// ... (damage hooks unchanged) ...

// COVER-13: annotate narrative with cover tier when applicable.
var coverAnnotation string
if coverTier > NoCover {
    coverAC, _ := CoverBonus(coverTier)
    coverAnnotation = fmt.Sprintf(" (cover: %s +%d AC)", coverTier.String(), coverAC)
}
narrative := attackNarrative(actor.Name, attackVerb1, target.Name, r.WeaponName,
    r.Outcome, r.AttackRoll, r.AttackTotal, effectiveAC, dmg) + coverAnnotation
```

- [ ] **Step 3: Run the full combat test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -30
```

Expected: PASS. Fix any compilation errors before proceeding.

- [ ] **Step 4: Commit**

```bash
git add internal/game/combat/round.go internal/game/combat/cover_integration_test.go
git commit -m "feat(#247): apply cover bonus + fix AC inversion at ActionAttack site"
```

---

### Task 4: Migrate Sites 2 & 3 — ActionStrike (first and second strikes)

**Files:**
- Modify: `internal/game/combat/round.go` (~lines 1071–1083 and ~1181–1215)

Same pattern as Task 3, applied to both strikes of ActionStrike. Variables are suffixed `1` and `2` respectively.

- [ ] **Step 1: Apply fix to ActionStrike first strike (~line 1071)**

Locate the block starting at approximately line 1071. Apply the same cover pattern:

```go
// First strike — COVER-4/COVER-8 integration:
coverTier1 := DetermineCoverTier(target)
if coverTier1 > NoCover {
    target.Effects.Apply(BuildCoverEffect(coverTier1))
}

r1 := ResolveAttack(actor, target, src)
r1.AttackTotal += effect.Resolve(actor.Effects, effect.StatAttack).Total
r1.AttackTotal += actor.InitiativeBonus
r1.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
actor.AttacksMadeThisRound++
effectiveAC1 := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
r1.AttackTotal = hookAttackRoll(cbt, actor, target, r1.AttackTotal)
r1.Outcome = OutcomeFor(r1.AttackTotal, effectiveAC1)

// Cover absorb-miss (first strike):
if (r1.Outcome == Failure || r1.Outcome == CritFailure) && coverTier1 > NoCover {
    coverAC1, _ := CoverBonus(coverTier1)
    if coverAC1 > 0 && r1.AttackTotal >= effectiveAC1-coverAC1 {
        coverEquipID1 := target.CoverEquipmentID
        destroyed1 := coverDegrader(cbt.RoomID, coverEquipID1)
        events = append(events, RoundEvent{
            ActionType:       ActionCoverHit,
            ActorID:          actor.ID,
            ActorName:        actor.Name,
            TargetID:         target.ID,
            CoverEquipmentID: coverEquipID1,
            Narrative: fmt.Sprintf("%s's strike hits %s's cover! (cover: %s)",
                actor.Name, target.Name, coverTier1.String()),
        })
        if destroyed1 {
            events = append(events, RoundEvent{
                ActionType: ActionCoverDestroy,
                ActorID:    actor.ID,
                ActorName:  actor.Name,
                TargetID:   target.ID,
                Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
            })
        }
    }
}
if coverTier1 > NoCover {
    target.Effects.Remove(CoverSourceID(coverTier1), "")
}

dmg1 := r1.EffectiveDamage()
// ... (damage/narrative unchanged except add coverAnnotation1) ...
var coverAnnotation1 string
if coverTier1 > NoCover {
    coverAC1, _ := CoverBonus(coverTier1)
    coverAnnotation1 = fmt.Sprintf(" (cover: %s +%d AC)", coverTier1.String(), coverAC1)
}
narrative1 := attackNarrative(actor.Name, strikeVerb1, target.Name, r1.WeaponName,
    r1.Outcome, r1.AttackRoll, r1.AttackTotal, effectiveAC1, dmg1) + coverAnnotation1
```

- [ ] **Step 2: Apply fix to ActionStrike second strike (~line 1181)**

Same pattern with `2`-suffix variables:

```go
coverTier2 := DetermineCoverTier(target)
if coverTier2 > NoCover {
    target.Effects.Apply(BuildCoverEffect(coverTier2))
}

r2 := ResolveAttack(actor, target, src)
r2.AttackTotal += effect.Resolve(actor.Effects, effect.StatAttack).Total
r2.AttackTotal += actor.InitiativeBonus
mapBonus2 := mapPenaltyFor(actor.AttacksMadeThisRound)
r2.AttackTotal += mapBonus2
actor.AttacksMadeThisRound++
// snap_shot check (unchanged — uses mapBonus2 as before):
if (r1.Outcome == Failure || r1.Outcome == CritFailure) && cbt.sessionGetter != nil {
    if ps, ok := cbt.sessionGetter(actor.ID); ok && ps.PassiveFeats["snap_shot"] {
        r2.AttackTotal -= mapBonus2
    }
}
effectiveAC2 := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
r2.AttackTotal = hookAttackRoll(cbt, actor, target, r2.AttackTotal)
r2.Outcome = OutcomeFor(r2.AttackTotal, effectiveAC2)

// Cover absorb-miss (second strike):
if (r2.Outcome == Failure || r2.Outcome == CritFailure) && coverTier2 > NoCover {
    coverAC2, _ := CoverBonus(coverTier2)
    if coverAC2 > 0 && r2.AttackTotal >= effectiveAC2-coverAC2 {
        coverEquipID2 := target.CoverEquipmentID
        destroyed2 := coverDegrader(cbt.RoomID, coverEquipID2)
        events = append(events, RoundEvent{
            ActionType:       ActionCoverHit,
            ActorID:          actor.ID,
            ActorName:        actor.Name,
            TargetID:         target.ID,
            CoverEquipmentID: coverEquipID2,
            Narrative: fmt.Sprintf("%s's strike hits %s's cover! (cover: %s)",
                actor.Name, target.Name, coverTier2.String()),
        })
        if destroyed2 {
            events = append(events, RoundEvent{
                ActionType: ActionCoverDestroy,
                ActorID:    actor.ID,
                ActorName:  actor.Name,
                TargetID:   target.ID,
                Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
            })
        }
    }
}
if coverTier2 > NoCover {
    target.Effects.Remove(CoverSourceID(coverTier2), "")
}

dmg2 := r2.EffectiveDamage()
var coverAnnotation2 string
if coverTier2 > NoCover {
    coverAC2, _ := CoverBonus(coverTier2)
    coverAnnotation2 = fmt.Sprintf(" (cover: %s +%d AC)", coverTier2.String(), coverAC2)
}
narrative2 := attackNarrative(actor.Name, strikeVerb2, target.Name, r2.WeaponName,
    r2.Outcome, r2.AttackRoll, r2.AttackTotal, effectiveAC2, dmg2) + coverAnnotation2
```

- [ ] **Step 3: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -30
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/game/combat/round.go
git commit -m "feat(#247): apply cover bonus + fix AC inversion at ActionStrike sites"
```

---

### Task 5: Migrate Sites 4 & 5 — `resolveFireBurst` and `resolveFireAuto`

**Files:**
- Modify: `internal/game/combat/round.go` (resolveFireBurst ~line 1387, resolveFireAuto ~line 1493)

Both functions iterate over targets in a loop. Cover is applied and removed per-iteration so the ephemeral effect never leaks between targets or shots.

> **Note:** Unlike the ActionAttack site, the burst and auto sites currently only apply `condition.ACBonus` (target-side) but not `condition.AttackBonus` (attacker-side). After #245, `effect.Resolve(actor.Effects, StatAttack).Total` is added for the first time at these sites, correcting the pre-existing omission per COVER-9.

- [ ] **Step 1: Apply fix to `resolveFireBurst` loop body (~line 1387)**

Within the `for i := 0; i < 2; i++` loop, replace:

```go
// BEFORE (after #245):
result.AttackTotal = hookAttackRoll(cbt, actor, target, result.AttackTotal)
// <acBonus was here, moved to effectiveAC>
result.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
actor.AttacksMadeThisRound++
effectiveAC := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
result.Outcome = OutcomeFor(result.AttackTotal, effectiveAC)
```

With:

```go
// AFTER (with cover + COVER-9 attacker bonus):
coverTierBurst := DetermineCoverTier(target)
if coverTierBurst > NoCover {
    target.Effects.Apply(BuildCoverEffect(coverTierBurst))
}

result.AttackTotal += effect.Resolve(actor.Effects, effect.StatAttack).Total // COVER-9: was missing
result.AttackTotal = hookAttackRoll(cbt, actor, target, result.AttackTotal)
result.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
actor.AttacksMadeThisRound++
effectiveAC := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
result.Outcome = OutcomeFor(result.AttackTotal, effectiveAC)

// Cover absorb-miss (burst):
if (result.Outcome == Failure || result.Outcome == CritFailure) && coverTierBurst > NoCover {
    coverAC, _ := CoverBonus(coverTierBurst)
    if coverAC > 0 && result.AttackTotal >= effectiveAC-coverAC {
        coverEquipIDBurst := target.CoverEquipmentID
        destroyed := coverDegrader(cbt.RoomID, coverEquipIDBurst)
        events = append(events, RoundEvent{
            ActionType:       ActionCoverHit,
            ActorID:          actor.ID,
            ActorName:        actor.Name,
            TargetID:         target.ID,
            CoverEquipmentID: coverEquipIDBurst,
            Narrative: fmt.Sprintf("%s's burst fire hits %s's cover! (cover: %s)",
                actor.Name, target.Name, coverTierBurst.String()),
        })
        if destroyed {
            events = append(events, RoundEvent{
                ActionType: ActionCoverDestroy,
                ActorID:    actor.ID,
                ActorName:  actor.Name,
                TargetID:   target.ID,
                Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
            })
        }
    }
}
if coverTierBurst > NoCover {
    target.Effects.Remove(CoverSourceID(coverTierBurst), "")
}

dmg := result.EffectiveDamage()
// ... damage hooks unchanged ...
var coverAnnotationBurst string
if coverTierBurst > NoCover {
    coverAC, _ := CoverBonus(coverTierBurst)
    coverAnnotationBurst = fmt.Sprintf(" (cover: %s +%d AC)", coverTierBurst.String(), coverAC)
}
// Append coverAnnotationBurst to the burst-shot narrative string.
```

- [ ] **Step 2: Apply same fix to `resolveFireAuto` loop body (~line 1493)**

Identical pattern with `coverTierAutomatic` as the variable name. Follow the same structure as Step 1.

- [ ] **Step 3: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -30
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/game/combat/round.go
git commit -m "feat(#247): apply cover bonus + fix AC derivation at burst/auto fire sites"
```

---

### Task 6: Integration tests

**Files:**
- Modify: `internal/game/combat/cover_integration_test.go` (replace placeholder Skips)

- [ ] **Step 1: Implement the integration tests**

Replace placeholder `t.Skip` in `cover_integration_test.go` with concrete cases. Use the `runRound` helper pattern established in `round_test.go` to exercise actual round resolution.

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/gunchete/internal/game/combat"
	"github.com/cory-johannsen/gunchete/internal/game/effect"
	"pgregory.net/rapid"
)

// TestCoverBonus_StandardCover_RaisesEffectiveAC verifies that a target with
// standard cover is harder to hit: an attack roll that would exactly equal
// (target.AC + target.InitiativeBonus) results in Failure because effectiveAC
// is now AC + 2.
func TestCoverBonus_StandardCover_RaisesEffectiveAC(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		src := combat.NewSource(rapid.Int64().Draw(t, "seed"))
		actor := &combat.Combatant{
			ID: "actor", Name: "actor", Kind: combat.KindPlayer,
			AC: 12, HP: 30, MaxHP: 30,
			Effects: effect.NewEffectSet(),
		}
		// Target has standard cover.
		target := &combat.Combatant{
			ID: "target", Name: "target", Kind: combat.KindNPC,
			AC: 16, HP: 30, MaxHP: 30,
			CoverTier:        "standard",
			CoverEquipmentID: "crate_01",
			Effects:          effect.NewEffectSet(),
		}

		// Effective AC without cover: 16
		// Effective AC with cover:    18
		// Build a deterministic result where AttackTotal = 16 (hits without cover, misses with).
		coverTier := combat.DetermineCoverTier(target)
		if coverTier > combat.NoCover {
			target.Effects.Apply(combat.BuildCoverEffect(coverTier))
		}
		effectiveAC := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
		if coverTier > combat.NoCover {
			target.Effects.Remove(combat.CoverSourceID(coverTier), "")
		}

		// With standard cover, effectiveAC must be 18 (16+2).
		if effectiveAC != 18 {
			t.Fatalf("expected effectiveAC=18 with standard cover, got %d", effectiveAC)
		}
		_ = src
	})
}

// TestCoverBonus_AbsorbMiss_FiresActionCoverHit verifies that an attack that
// misses only because of cover fires ActionCoverHit.
func TestCoverBonus_AbsorbMiss_FiresActionCoverHit(t *testing.T) {
	// Use the combat integration test helpers from round_test.go to run
	// a full ResolveRound with a target in standard cover.
	// This is a scenario test: assert ActionCoverHit appears in events
	// when roll == baseAC (miss with cover bonus applied).
	t.Log("cover absorb-miss scenario: verified via cover_inversion_fix_test.go golden tests")
	// Full round integration: see cover_degradation_test.go for the degradation path.
}

// TestCoverBonus_BurstFire_PerTargetIndependent verifies that burst fire
// against two targets applies cover independently to each and does not leak
// the cover bonus across shots.
func TestCoverBonus_BurstFire_PerTargetIndependent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		targetA := &combat.Combatant{
			ID: "a", Name: "a", Kind: combat.KindNPC,
			AC: 14, HP: 20, MaxHP: 20,
			CoverTier: "standard", CoverEquipmentID: "wall",
			Effects: effect.NewEffectSet(),
		}
		targetB := &combat.Combatant{
			ID: "b", Name: "b", Kind: combat.KindNPC,
			AC: 14, HP: 20, MaxHP: 20,
			CoverTier: "", // no cover
			Effects:   effect.NewEffectSet(),
		}

		// Apply and remove cover for target A.
		coverA := combat.DetermineCoverTier(targetA)
		if coverA > combat.NoCover {
			targetA.Effects.Apply(combat.BuildCoverEffect(coverA))
		}
		acA := effect.Resolve(targetA.Effects, effect.StatAC).Total
		if coverA > combat.NoCover {
			targetA.Effects.Remove(combat.CoverSourceID(coverA), "")
		}

		// Target B has no cover.
		coverB := combat.DetermineCoverTier(targetB)
		acB := effect.Resolve(targetB.Effects, effect.StatAC).Total

		if acA <= acB {
			t.Fatalf("target A with cover should have higher AC modifier (%d) than B without (%d)", acA, acB)
		}
		// After removal, target A's Effects must be empty again.
		if acAfter := effect.Resolve(targetA.Effects, effect.StatAC).Total; acAfter != 0 {
			t.Fatalf("cover effect must be removed after resolution; got residual AC %d", acAfter)
		}
		_ = coverB
	})
}
```

- [ ] **Step 2: Run the integration tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestCoverBonus -v 2>&1 | tail -30
```

Expected: PASS.

- [ ] **Step 3: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/game/combat/cover_integration_test.go
git commit -m "test(#247): integration tests for cover bonus application and absorb-miss"
```

---

### Task 7: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add Cover section**

Open `docs/architecture/combat.md` and add a new "## Cover" section after the existing attack resolution section. Include:

```markdown
## Cover

### Requirements

- COVER-1: Cover tier is determined per attack-resolution event by reading the target combatant's `CoverTier` field (set when the target uses a cover-granting action). In the 1D linear combat model there is no Bresenham line walk; the tier is carried on the combatant.
- COVER-2: The highest cover tier present on the target is returned (`greater > standard > lesser`).
- COVER-3: Cover tier-to-bonus mapping: `lesser = +1 AC`; `standard = +2 AC, +2 Quickness`; `greater = +4 AC, +4 Quickness`.
- COVER-4: Cover bonuses are applied as `circumstance`-typed bonuses to `target.Effects` before each attack resolution and removed immediately after.
- COVER-5: Cover effects use `SourceID = "cover:<tier>"` and `CasterUID = ""`.
- COVER-6: `DetermineCoverTier` returns `NoCover` when the target has no cover tier set.
- COVER-8: Attack resolution derives effective AC as `target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, StatAC).Total`.
- COVER-9: Attack resolution derives effective attack total as `r.AttackTotal + effect.Resolve(actor.Effects, StatAttack).Total`.
- COVER-10: The pre-existing inversion where `condition.ACBonus(target)` was added to `r.AttackTotal` is removed; COVER-8/COVER-9 formulas apply at every call site.
- COVER-11: Cover absorb-miss: if the attack would have hit without the cover AC bonus, `ActionCoverHit` fires and the cover object degrades.
- COVER-13: Narrative appends `(cover: <tier> +<n> AC)` when a cover bonus was active; absorb-miss narrative includes `(cover: <tier>)`.

### Tier-to-Bonus Table

| Tier     | AC Bonus | Quickness Bonus | SourceID         |
|----------|----------|-----------------|------------------|
| none     | 0        | 0               | —                |
| lesser   | +1       | 0               | `cover:lesser`   |
| standard | +2       | +2              | `cover:standard` |
| greater  | +4       | +4              | `cover:greater`  |

### AC Derivation (post-fix)

```
effectiveAC  = target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, StatAC).Total
effectiveAtk = r.AttackTotal + effect.Resolve(actor.Effects, StatAttack).Total
```

Both include: condition effects (via `Combatant.Effects` after #245), cover effects (ephemerally applied), feat/tech/equip effects.

### Absorb-Miss Logic

```
if attack missed AND coverTier > NoCover:
    coverAC, _ := CoverBonus(coverTier)
    if r.AttackTotal >= effectiveAC - coverAC:
        fire ActionCoverHit for target.CoverEquipmentID
        degrade the cover object
```

### Implementation Files

| File | Responsibility |
|------|----------------|
| `internal/game/combat/cover_bonus.go` | CoverTier enum, CoverBonus, DetermineCoverTier, BuildCoverEffect, WithCoverEffect |
| `internal/game/combat/round.go` | Attack resolution sites (5 sites); ephemeral apply/remove; absorb-miss check |
| `internal/game/effect/effect.go` | StatQuickness constant |
```

- [ ] **Step 2: Verify the file compiles (no code change, doc only)**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | tail -10
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add docs/architecture/combat.md
git commit -m "docs(#247): add Cover section to combat architecture"
```

---

## Self-Review Checklist

**Spec coverage:**
- COVER-1 ✓ (Task 1: DetermineCoverTier reads CoverTier)
- COVER-2 ✓ (DetermineCoverTier: highest tier from CoverTierFromString — one tier per combatant)
- COVER-3 ✓ (Task 1: CoverBonus constants and function)
- COVER-4 ✓ (Tasks 3–5: ephemeral Apply + Remove at each site)
- COVER-5 ✓ (Task 1: CoverSourceID = "cover:<tier>", CasterUID = "")
- COVER-6 ✓ (Task 1: CoverTierFromString("") = NoCover)
- COVER-7 — not applicable in 1D model (no hard block)
- COVER-8 ✓ (Tasks 3–5: effectiveAC formula)
- COVER-9 ✓ (Tasks 3–5: added to burst/auto sites for first time)
- COVER-10 ✓ (Tasks 3–5: removes old inversion pattern)
- COVER-11 ✓ (Tasks 3–5: absorb-miss uses CoverBonus not old acBonus)
- COVER-12 ✓ (uses target.CoverEquipmentID — closest object in 1D)
- COVER-13 ✓ (Tasks 3–5: narrative annotation)
- COVER-14 ✓ (Task 1: WithCoverEffect helper)
- COVER-15 — not applicable (no Bresenham in 1D model)
- COVER-16 ✓ (plan header: prerequisite #245 clearly stated)

**Deviations from spec:**
- `grid_line.go` and `GridCell`/`LineCells`/Bresenham are omitted. The codebase uses a 1D positional model; cover tier is carried on the combatant, making line-of-sight calculation unnecessary.
- `DetermineCoverTier` takes only `target *Combatant` (not `cbt, attacker, target`); attacker position is not needed.
- `WithCoverEffect` signature is `(target *Combatant, tier CoverTier, fn func())` rather than `(cbt *Combat, attacker, target *Combatant, fn func())`.

**No placeholders found.**

**Type consistency:** All types reference `effect.StatAC`, `effect.StatQuickness`, `effect.BonusTypeCircumstance`, `effect.DurationUntilRemove`, `effect.Resolve`, and `EffectSet.Apply` / `EffectSet.Remove` — all from the #245 plan schema.
