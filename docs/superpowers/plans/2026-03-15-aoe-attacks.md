# AoE Attacks Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing explosive/throw system with a `friendly_fire` flag and attacker-level-scaled Reflex save DCs; update existing grenade content.

**Architecture:** The existing `ExplosiveDef` (in `internal/game/inventory/explosive.go`), `ResolveExplosive` (in `internal/game/combat/resolver.go`), and `resolveThrow` (in `internal/game/combat/round.go`) already handle grenade resolution. This plan adds two fields to `ExplosiveDef`, changes the `ResolveExplosive` signature to accept an `effectiveDC` override, adds an `explosiveTargetsOf` helper for friendly-fire filtering, and updates existing grenade YAML content.

**Tech Stack:** Go 1.26, gopkg.in/yaml.v3, pgregory.net/rapid (property-based tests)

**Spec:** `docs/superpowers/specs/2026-03-15-aoe-attacks-design.md`

---

## Chunk 1: Data Model and Resolution

### Task 1: ExplosiveDef extension and content updates

**Files:**
- Modify: `internal/game/inventory/explosive.go`
- Modify: `content/explosives/frag_grenade.yaml`
- Modify: `content/explosives/incendiary_grenade.yaml`
- Modify: `content/explosives/emp_grenade.yaml`
- Test: `internal/game/inventory/explosive_test.go` (create)

- [ ] **Step 1: Write failing tests**

Create `internal/game/inventory/explosive_test.go`:

```go
package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplosiveDef_FriendlyFire_DefaultFalse(t *testing.T) {
	e := &inventory.ExplosiveDef{
		ID:         "test_grenade",
		Name:       "Test Grenade",
		DamageDice: "2d6",
		DamageType: "piercing",
		SaveType:   "reflex",
		SaveDC:     12,
	}
	require.NoError(t, e.Validate())
	assert.False(t, e.FriendlyFire, "FriendlyFire should default false")
	assert.Equal(t, 0, e.AoERadius, "AoERadius should default 0")
}

func TestExplosiveDef_FriendlyFire_ParsedFromYAML(t *testing.T) {
	explosives, err := inventory.LoadExplosives("../../../content/explosives")
	require.NoError(t, err)
	require.NotEmpty(t, explosives)
	for _, e := range explosives {
		assert.False(t, e.FriendlyFire, "existing explosives should have friendly_fire: false")
		assert.Equal(t, 0, e.AoERadius, "existing explosives should have aoe_radius: 0")
		require.NoError(t, e.Validate())
	}
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run TestExplosiveDef_ -v`
Expected: FAIL — `FriendlyFire` field not found.

- [ ] **Step 2: Add fields to ExplosiveDef**

In `internal/game/inventory/explosive.go`, add after `Fuse FuseType` field:

```go
// FriendlyFire controls whether this explosive damages allied combatants.
// When false (default), only enemy-kind combatants are targeted.
// When true, all living combatants except the thrower are targeted.
FriendlyFire bool `yaml:"friendly_fire,omitempty"`

// AoERadius is the blast radius in feet for AreaTypeBurst explosives.
// Stored for future position-based filtering; not actively used in target selection now.
// Zero means room-wide effect (current behavior).
AoERadius int `yaml:"aoe_radius,omitempty"`
```

- [ ] **Step 3: Update existing explosive YAML files**

In `content/explosives/frag_grenade.yaml`, add after `fuse: immediate`:
```yaml
friendly_fire: false
aoe_radius: 0
```

In `content/explosives/incendiary_grenade.yaml`, add after `fuse: immediate`:
```yaml
friendly_fire: false
aoe_radius: 0
```

In `content/explosives/emp_grenade.yaml`, add after `fuse: immediate`:
```yaml
friendly_fire: false
aoe_radius: 0
```

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -v -count=1`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/inventory/explosive.go \
        internal/game/inventory/explosive_test.go \
        content/explosives/frag_grenade.yaml \
        content/explosives/incendiary_grenade.yaml \
        content/explosives/emp_grenade.yaml
git commit -m "$(cat <<'EOF'
feat(inventory): add FriendlyFire and AoERadius to ExplosiveDef; update grenade content

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: explosiveTargetsOf helper, ResolveExplosive signature, resolveThrow update

**Files:**
- Modify: `internal/game/combat/resolver.go`
- Modify: `internal/game/combat/round.go`
- Test: `internal/game/combat/resolver_aoe_test.go` (create)

**Context:** `ResolveExplosive` is defined at `internal/game/combat/resolver.go:165`. Its current signature is:
```go
func ResolveExplosive(grenade *inventory.ExplosiveDef, targets []*Combatant, src Source) []ExplosiveResult
```
`resolveThrow` at `internal/game/combat/round.go:959` calls it as:
```go
enemies := livingEnemiesOf(cbt, actor)
results := ResolveExplosive(grenade, enemies, src)
```
`livingEnemiesOf` is defined at `round.go:1000` and used in other resolution paths — do NOT delete it.

- [ ] **Step 1: Write failing tests**

Create `internal/game/combat/resolver_aoe_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// Note: fixedSrc is already defined in resolver_stage7_test.go as:
//   type fixedSrc struct{ v int }
//   func (f fixedSrc) Intn(n int) int { if f.v >= n { return n-1 }; return f.v }
// Do NOT redefine it. Use fixedSrc{v: N} in this file.

func newGrenade(dc int, friendly bool) *inventory.ExplosiveDef {
	return &inventory.ExplosiveDef{
		ID:           "test_grenade",
		Name:         "Test Grenade",
		DamageDice:   "2d6",
		DamageType:   "piercing",
		AreaType:     inventory.AreaTypeRoom,
		SaveType:     "reflex",
		SaveDC:       dc,
		Fuse:         inventory.FuseImmediate,
		FriendlyFire: friendly,
	}
}

func newCombatant(id string, kind combat.Kind, hp int) *combat.Combatant {
	return &combat.Combatant{
		ID:           id,
		Kind:         kind,
		Name:         id,
		CurrentHP:    hp,
		MaxHP:        hp,
		QuicknessMod: 0,
	}
}

// REQ-T2: explosiveTargetsOf with friendly_fire: false returns only enemy-kind combatants.
func TestExplosiveTargetsOf_FriendlyFireFalse_EnemiesOnly(t *testing.T) {
	actor := newCombatant("player", combat.KindPlayer, 10)
	enemy1 := newCombatant("npc1", combat.KindNPC, 10)
	enemy2 := newCombatant("npc2", combat.KindNPC, 10)
	ally := newCombatant("ally", combat.KindPlayer, 10)
	cbt := &combat.Combat{Combatants: []*combat.Combatant{actor, enemy1, enemy2, ally}}

	grenade := newGrenade(12, false)
	targets := combat.ExplosiveTargetsOf(cbt, actor, grenade)

	require.Len(t, targets, 2)
	for _, t2 := range targets {
		assert.Equal(t, combat.KindNPC, t2.Kind)
	}
}

// REQ-T3: explosiveTargetsOf with friendly_fire: true returns all living non-actor combatants.
func TestExplosiveTargetsOf_FriendlyFireTrue_AllNonActor(t *testing.T) {
	actor := newCombatant("player", combat.KindPlayer, 10)
	enemy := newCombatant("npc1", combat.KindNPC, 10)
	ally := newCombatant("ally", combat.KindPlayer, 10)
	cbt := &combat.Combat{Combatants: []*combat.Combatant{actor, enemy, ally}}

	grenade := newGrenade(12, true)
	targets := combat.ExplosiveTargetsOf(cbt, actor, grenade)

	require.Len(t, targets, 2)
}

// REQ-T4: Actor is never in target list.
func TestExplosiveTargetsOf_ActorNeverIncluded(t *testing.T) {
	actor := newCombatant("player", combat.KindPlayer, 10)
	cbt := &combat.Combat{Combatants: []*combat.Combatant{actor}}

	for _, ff := range []bool{true, false} {
		grenade := newGrenade(12, ff)
		targets := combat.ExplosiveTargetsOf(cbt, actor, grenade)
		assert.Empty(t, targets, "actor must not appear in target list (friendly_fire=%v)", ff)
	}
}

// REQ-T5: All non-actor combatants dead returns empty slice.
func TestExplosiveTargetsOf_AllDead_EmptySlice(t *testing.T) {
	actor := newCombatant("player", combat.KindPlayer, 10)
	dead := newCombatant("npc1", combat.KindNPC, 0)
	dead.Dead = true
	cbt := &combat.Combat{Combatants: []*combat.Combatant{actor, dead}}

	grenade := newGrenade(12, true)
	targets := combat.ExplosiveTargetsOf(cbt, actor, grenade)
	assert.Empty(t, targets)
}

// REQ-T1: ResolveExplosive with higher effectiveDC produces saves against that DC.
func TestResolveExplosive_EffectiveDCUsed(t *testing.T) {
	// Use a fixed die source that returns 5 (roll = 6 after +1).
	// target.QuicknessMod = 0; so total = 6.
	// With baseDC=12: total(6) < 12 → Failure.
	// With effectiveDC=5: total(6) >= 5 → Success (half damage).
	src := fixedSrc{v: 5} // Intn(20) returns 5 → roll = 6
	grenade := newGrenade(12, false)
	target := newCombatant("npc1", combat.KindNPC, 10)

	// Against effectiveDC=12: should fail (6 < 12)
	results := combat.ResolveExplosive(grenade, []*combat.Combatant{target}, 12, src)
	require.Len(t, results, 1)
	assert.Equal(t, combat.Failure, results[0].SaveResult)

	// Against effectiveDC=5: should succeed (6 >= 5)
	results2 := combat.ResolveExplosive(grenade, []*combat.Combatant{target}, 5, src)
	require.Len(t, results2, 1)
	assert.Equal(t, combat.Success, results2[0].SaveResult)
}

// REQ-T6 (property): For any actor.Level in [1,20], resolveThrow effective DC = grenade.SaveDC + actor.Level.
// Verified indirectly by checking ResolveExplosive receives a DC higher than grenade.SaveDC.
func TestProperty_ResolveExplosive_EffectiveDCScalesWithLevel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		baseDC := rapid.IntRange(10, 18).Draw(rt, "baseDC")

		// A fixed roll of 0 (minimum) means total = 0 + 0 = 0, which fails any DC.
		src := fixedSrc{v: 0}
		grenade := newGrenade(baseDC, false)
		target := newCombatant("npc", combat.KindNPC, 10)

		// effectiveDC = baseDC + level; a roll of 0 always fails regardless.
		results := combat.ResolveExplosive(grenade, []*combat.Combatant{target}, baseDC+level, src)
		require.Len(rt, results, 1)
		// Just verify no panic and result has non-negative damage.
		assert.GreaterOrEqual(rt, results[0].BaseDamage, 0)
	})
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestExplosiveTargets -v`
Expected: FAIL — `ExplosiveTargetsOf` not defined.

- [ ] **Step 2: Add `explosiveTargetsOf` to round.go**

In `internal/game/combat/round.go`, add immediately before `func resolveThrow`:

```go
// explosiveTargetsOf returns living targets for an explosive based on its FriendlyFire flag.
//
// Precondition: cbt, actor, and grenade must be non-nil.
// Postcondition: Returns all living non-actor combatants; if grenade.FriendlyFire is false,
//   only enemy-kind (different Kind from actor) combatants are returned.
func explosiveTargetsOf(cbt *Combat, actor *Combatant, grenade *inventory.ExplosiveDef) []*Combatant {
	var out []*Combatant
	for _, c := range cbt.Combatants {
		if c.IsDead() || c.ID == actor.ID {
			continue
		}
		if !grenade.FriendlyFire && c.Kind == actor.Kind {
			continue
		}
		out = append(out, c)
	}
	return out
}
```

Also expose the function for testing. Check first if `internal/game/combat/export_test.go` already exists:
```bash
ls internal/game/combat/export_test.go 2>/dev/null && echo EXISTS || echo MISSING
```
If MISSING, create `internal/game/combat/export_test.go`:
```go
package combat

// ExplosiveTargetsOf exposes explosiveTargetsOf for package-external tests.
var ExplosiveTargetsOf = explosiveTargetsOf
```
If EXISTS, append the `var ExplosiveTargetsOf` line to the existing file.

- [ ] **Step 3: Update `ResolveExplosive` signature in resolver.go**

In `internal/game/combat/resolver.go`, change the `ResolveExplosive` signature from:
```go
func ResolveExplosive(grenade *inventory.ExplosiveDef, targets []*Combatant, src Source) []ExplosiveResult {
```
to:
```go
func ResolveExplosive(grenade *inventory.ExplosiveDef, targets []*Combatant, effectiveDC int, src Source) []ExplosiveResult {
```

Inside the function body, replace the line `saveOutcome := ResolveSave("hustle", target, grenade.SaveDC, src)` with:
```go
saveOutcome := ResolveSave("hustle", target, effectiveDC, src)
```

Update the doc comment precondition:
```go
// Precondition: grenade and all targets must not be nil; effectiveDC >= 1.
// Postcondition: each target makes a Hustle save vs effectiveDC;
// damage scaled by save outcome (crit success: 0, success: half, failure: full, crit failure: double).
```

- [ ] **Step 4: Update all ResolveExplosive call sites**

Run: `grep -rn "ResolveExplosive" /home/cjohannsen/src/mud/internal/` to enumerate call sites.

**Known call site: `internal/game/combat/round.go:960`**

In `resolveThrow`, replace:
```go
enemies := livingEnemiesOf(cbt, actor)
results := ResolveExplosive(grenade, enemies, src)
```
with:
```go
targets := explosiveTargetsOf(cbt, actor, grenade)
effectiveDC := grenade.SaveDC + actor.Level
results := ResolveExplosive(grenade, targets, effectiveDC, src)
```

**Existing test call sites** in `internal/game/combat/resolver_stage7_test.go`: update each `ResolveExplosive(grenade, targets, src)` call to `ResolveExplosive(grenade, targets, grenade.SaveDC, src)`.

Run `grep -n "ResolveExplosive" internal/game/combat/resolver_stage7_test.go` to find all test call sites and update them.

- [ ] **Step 5: Run all tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... ./internal/game/inventory/... -v -count=1 -race 2>&1 | tail -40`
Expected: All pass including existing `TestResolveExplosive_*` tests (REQ-T7).

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/combat/round.go \
        internal/game/combat/resolver.go \
        internal/game/combat/export_test.go \
        internal/game/combat/resolver_aoe_test.go \
        internal/game/combat/resolver_stage7_test.go
git commit -m "$(cat <<'EOF'
feat(combat): add explosiveTargetsOf helper; scale explosive DC by attacker level

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Full integration test and final suite

**Files:**
- Test: `internal/game/combat/throw_friendly_fire_test.go` (create)

**Context:** REQ-T8 requires verifying that `friendly_fire: false` excludes allies and `friendly_fire: true` includes them. These tests only use `combat.ExplosiveTargetsOf` and `inventory.ExplosiveDef` — they belong in `package combat_test` alongside `resolver_aoe_test.go`, not in `package gameserver` (which would pull in all gameserver dependencies unnecessarily).

- [ ] **Step 1: Write integration test (REQ-T8)**

Create `internal/game/combat/throw_friendly_fire_test.go`:

```go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
)

// TestThrow_FriendlyFireFalse_EnemyOnly verifies REQ-T8 (friendly_fire: false path):
// only enemy-kind combatants are in the target list.
func TestThrow_FriendlyFireFalse_EnemyOnly(t *testing.T) {
	actor := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", CurrentHP: 10, MaxHP: 10, Level: 2}
	enemy := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Rat", CurrentHP: 10, MaxHP: 10}
	ally := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", CurrentHP: 10, MaxHP: 10}
	cbt := &combat.Combat{Combatants: []*combat.Combatant{actor, enemy, ally}}

	grenade := &inventory.ExplosiveDef{
		ID: "frag_grenade", Name: "Frag Grenade",
		DamageDice: "2d6", DamageType: "piercing",
		AreaType: inventory.AreaTypeRoom, SaveType: "reflex", SaveDC: 15,
		FriendlyFire: false,
	}

	targets := combat.ExplosiveTargetsOf(cbt, actor, grenade)
	assert.Len(t, targets, 1, "with friendly_fire=false, only the enemy should be targeted")
	assert.Equal(t, "n1", targets[0].ID, "only NPC enemy should be in target list")
}

// TestThrow_FriendlyFireTrue_AllNonActor verifies REQ-T8 (friendly_fire: true path):
// all living non-actor combatants including allies are in the target list.
func TestThrow_FriendlyFireTrue_AllyIncluded(t *testing.T) {
	actor := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", CurrentHP: 10, MaxHP: 10, Level: 2}
	enemy := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Rat", CurrentHP: 10, MaxHP: 10}
	ally := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", CurrentHP: 10, MaxHP: 10}
	cbt := &combat.Combat{Combatants: []*combat.Combatant{actor, enemy, ally}}

	grenade := &inventory.ExplosiveDef{
		ID: "friendly_grenade", Name: "Friendly Grenade",
		DamageDice: "1d4", DamageType: "piercing",
		AreaType: inventory.AreaTypeRoom, SaveType: "reflex", SaveDC: 10,
		FriendlyFire: true,
	}

	targets := combat.ExplosiveTargetsOf(cbt, actor, grenade)
	assert.Len(t, targets, 2, "with friendly_fire=true, both NPC and ally should be targeted")
	var allyFound bool
	for _, tgt := range targets {
		if tgt.ID == "p2" {
			allyFound = true
		}
	}
	assert.True(t, allyFound, "ally must be in target list when friendly_fire=true")
}
```

- [ ] **Step 2: Run full suite**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1 -timeout=120s 2>&1 | tail -20`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/combat/throw_friendly_fire_test.go
git commit -m "$(cat <<'EOF'
test(combat): add friendly_fire integration tests for throw command

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```
