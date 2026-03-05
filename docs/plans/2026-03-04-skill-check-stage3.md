# Skill Check Stage 3 — Active Feat/Class Feature → Condition Mapping

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** When a player uses an active feat or class feature, apply a named condition to the player's session (carrying mechanical deltas like damage bonus, AC penalty); clear `encounter`-duration conditions when combat ends.

**Architecture:** `ConditionDef` gains a `DamageBonus` field and an `"encounter"` duration type. `Feat` and `ClassFeature` gain an optional `ConditionID` string. `handleUse` applies the condition to `sess.Conditions` in addition to returning the activation text. `CombatHandler` gains an `onCombatEndFn` callback; `grpc_service.go` registers it to call `ClearEncounter()` on all sessions in the room.

**Tech Stack:** Go, `pgregory.net/rapid`, existing `condition.ActiveSet`/`condition.Registry`, `ruleset.Feat`/`ruleset.ClassFeature`

---

## Background & Key Facts

- **`ConditionDef`** — `internal/game/condition/definition.go`. Fields: `ID`, `Name`, `Description`, `DurationType` (`"rounds"`|`"until_save"`|`"permanent"`), `MaxStacks`, `AttackPenalty`, `ACPenalty`, `SpeedPenalty`, `RestrictActions`, `LuaOnApply`, `LuaOnRemove`, `LuaOnTick`. **Missing:** `DamageBonus int`, `"encounter"` duration type.
- **`condition.ActiveSet`** — `internal/game/condition/active.go`. Methods: `Apply(uid, def, stacks, duration)`, `Remove`, `Has`, `All`. **Missing:** `ClearEncounter()` to remove all conditions with `duration_type == "encounter"`.
- **`condition.modifiers`** — `internal/game/condition/modifiers.go`. Functions: `AttackBonus`, `ACBonus`, `IsActionRestricted`, `StunnedAPReduction`. **Missing:** `DamageBonus(s *ActiveSet) int`.
- **`Feat`** — `internal/game/ruleset/feat.go`. Fields: `ID`, `Name`, `Category`, `Skill`, `Archetype`, `PF2E`, `Active`, `ActivateText`, `Description`. **Missing:** `ConditionID string`.
- **`ClassFeature`** — `internal/game/ruleset/class_feature.go`. Same pattern. **Missing:** `ConditionID string`.
- **`handleUse`** — `internal/gameserver/grpc_service.go` (around line 2641). Currently finds a matching active feat/class feature and returns its `ActivateText`. Must be extended to apply `ConditionID` condition when present.
- **`sess.Conditions *condition.ActiveSet`** — initialized at login. Available for condition application.
- **`s.condRegistry *condition.Registry`** — already on `GameServiceServer`. `Get(id string) (*ConditionDef, bool)`.
- **`CombatHandler`** — `internal/gameserver/combat_handler.go`. Combat ends when `resolveAndAdvanceLocked` detects no living NPCs or no living players. It calls `h.broadcastFn(roomID, events)` then `h.engine.EndCombat(roomID)`. **No callback exists yet for post-combat cleanup.**
- **`h.engine.EndCombat(roomID)`** — removes the combat record from the engine's map. Called in `combat_handler.go` after broadcasting the end event.
- **Penalty convention**: `AttackPenalty int` and `ACPenalty int` store positive values meaning "negative effect" (penalty). `DamageBonus int` stores a positive value meaning "positive effect" (bonus). `ACPenalty: 2` means -2 to AC; `DamageBonus: 2` means +2 to damage.
- **`DurationType: "encounter"`** — new value. Conditions with this type are cleared at the end of combat via `ClearEncounter()`. They persist in `sess.Conditions` indefinitely until `ClearEncounter()` is called.

---

## Task 1: Add `DamageBonus` to `ConditionDef` + modifier function

**Files:**
- Modify: `internal/game/condition/definition.go`
- Modify: `internal/game/condition/modifiers.go`
- Test: `internal/game/condition/modifiers_test.go` (create if absent)

**Context:** `ConditionDef` only tracks penalties (attack, AC, speed). Active feats like Brutal Surge need to grant damage bonuses. Add `DamageBonus int` field and a `DamageBonus(s *ActiveSet) int` modifier function.

**Step 1: Write failing tests**

Create or open `internal/game/condition/modifiers_test.go`. Add:

```go
package condition_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/condition"
)

func TestDamageBonus_ZeroWhenNoConditions(t *testing.T) {
    s := condition.NewActiveSet()
    assert.Equal(t, 0, condition.DamageBonus(s))
}

func TestDamageBonus_AppliedCondition(t *testing.T) {
    s := condition.NewActiveSet()
    def := &condition.ConditionDef{
        ID:           "surge",
        Name:         "Surge",
        DamageBonus:  3,
        DurationType: "encounter",
        MaxStacks:    0,
    }
    err := s.Apply("uid1", def, 1, -1)
    assert.NoError(t, err)
    assert.Equal(t, 3, condition.DamageBonus(s))
}

func TestProperty_DamageBonus_NeverNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        bonus := rapid.IntRange(0, 10).Draw(rt, "bonus")
        s := condition.NewActiveSet()
        def := &condition.ConditionDef{
            ID: "test", Name: "Test", DamageBonus: bonus,
            DurationType: "permanent", MaxStacks: 0,
        }
        _ = s.Apply("uid", def, 1, -1)
        got := condition.DamageBonus(s)
        assert.GreaterOrEqual(t, got, 0, "DamageBonus should be non-negative")
    })
}
```

Run:
```bash
go test -run "TestDamageBonus|TestProperty_DamageBonus" -v ./internal/game/condition/...
```
Expected: FAIL — `ConditionDef.DamageBonus` undefined and `condition.DamageBonus` function undefined

**Step 2: Add `DamageBonus int` to `ConditionDef`**

In `internal/game/condition/definition.go`, add after `SpeedPenalty`:

```go
DamageBonus int `yaml:"damage_bonus"`
```

**Step 3: Add `DamageBonus` modifier function**

In `internal/game/condition/modifiers.go`, add:

```go
// DamageBonus returns the total damage bonus granted by all active conditions.
// Precondition: s must not be nil.
// Postcondition: returns >= 0.
func DamageBonus(s *ActiveSet) int {
    total := 0
    for _, ac := range s.All() {
        stacks := ac.Stacks
        if stacks < 1 {
            stacks = 1
        }
        total += ac.Def.DamageBonus * stacks
    }
    if total < 0 {
        total = 0
    }
    return total
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -run "TestDamageBonus|TestProperty_DamageBonus" -v ./internal/game/condition/...
```
Expected: PASS

**Step 5: Run full suite**

```bash
go test ./internal/game/condition/...
```
Expected: all pass

**Step 6: Commit**

```bash
git add internal/game/condition/definition.go internal/game/condition/modifiers.go internal/game/condition/modifiers_test.go
git commit -m "feat: add DamageBonus field to ConditionDef and DamageBonus modifier function"
```

---

## Task 2: Add `encounter` duration type + `ClearEncounter()` to `ActiveSet`

**Files:**
- Modify: `internal/game/condition/active.go`
- Test: `internal/game/condition/active_test.go` (create or add to existing)

**Context:** Active feat conditions like Brutal Surge should last only for one combat encounter. Add `ClearEncounter()` to remove all conditions with `DurationType == "encounter"`.

**Step 1: Write failing tests**

Find or create `internal/game/condition/active_test.go`. Add:

```go
func TestClearEncounter_RemovesEncounterConditions(t *testing.T) {
    s := condition.NewActiveSet()
    enc := &condition.ConditionDef{ID: "surge", Name: "Surge", DurationType: "encounter", MaxStacks: 0}
    perm := &condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0}
    _ = s.Apply("uid", enc, 1, -1)
    _ = s.Apply("uid", perm, 1, -1)
    assert.True(t, s.Has("surge"))
    assert.True(t, s.Has("prone"))

    s.ClearEncounter()

    assert.False(t, s.Has("surge"), "encounter condition should be cleared")
    assert.True(t, s.Has("prone"), "permanent condition should remain")
}

func TestClearEncounter_EmptySet_NoPanic(t *testing.T) {
    s := condition.NewActiveSet()
    assert.NotPanics(t, func() { s.ClearEncounter() })
}

func TestProperty_ClearEncounter_OnlyRemovesEncounterType(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        durType := rapid.SampledFrom([]string{"permanent", "rounds", "until_save", "encounter"}).Draw(rt, "dur")
        s := condition.NewActiveSet()
        def := &condition.ConditionDef{ID: "c1", Name: "C1", DurationType: durType, MaxStacks: 0}
        _ = s.Apply("uid", def, 1, -1)
        s.ClearEncounter()
        if durType == "encounter" {
            assert.False(t, s.Has("c1"), "encounter condition must be cleared")
        } else {
            assert.True(t, s.Has("c1"), "non-encounter condition must remain")
        }
    })
}
```

Run:
```bash
go test -run "TestClearEncounter|TestProperty_ClearEncounter" -v ./internal/game/condition/...
```
Expected: FAIL — `ClearEncounter` undefined

**Step 2: Add `ClearEncounter()` to `ActiveSet`**

In `internal/game/condition/active.go`, add:

```go
// ClearEncounter removes all conditions with DurationType == "encounter".
// Called at the end of combat to clear temporary combat-only conditions.
// Precondition: s must not be nil.
// Postcondition: all encounter-duration conditions are removed; other conditions unchanged.
func (s *ActiveSet) ClearEncounter() {
    for id, ac := range s.conditions {
        if ac.Def.DurationType == "encounter" {
            delete(s.conditions, id)
        }
    }
}
```

**Step 3: Run tests to verify they pass**

```bash
go test -run "TestClearEncounter|TestProperty_ClearEncounter" -v ./internal/game/condition/...
```
Expected: PASS

**Step 4: Run full suite**

```bash
go test ./internal/game/condition/...
```

**Step 5: Commit**

```bash
git add internal/game/condition/active.go internal/game/condition/active_test.go
git commit -m "feat: add encounter duration type and ClearEncounter() to ActiveSet"
```

---

## Task 3: Add `ConditionID` to `Feat` and `ClassFeature`; wire `handleUse`

**Files:**
- Modify: `internal/game/ruleset/feat.go`
- Modify: `internal/game/ruleset/class_feature.go`
- Modify: `internal/gameserver/grpc_service.go` (`handleUse`)
- Test: `internal/gameserver/grpc_service_test.go`

**Context:** `handleUse` currently returns only `ActivateText`. Extend it: if the feat/class feature has a `ConditionID`, look up the condition in `s.condRegistry` and apply it to `sess.Conditions` with stacks=1 and duration=-1 (stored indefinitely until `ClearEncounter()` removes it for encounter-type conditions, or until session ends for permanent ones).

**Step 1: Add `ConditionID string` to both structs**

In `internal/game/ruleset/feat.go`:
```go
ConditionID  string `yaml:"condition_id"`  // optional; non-empty means Use applies this condition
```

In `internal/game/ruleset/class_feature.go`:
```go
ConditionID  string `yaml:"condition_id"`  // optional; non-empty means Use applies this condition
```

**Step 2: Write failing test for `handleUse` with condition**

In `internal/gameserver/grpc_service_test.go`, add:

```go
func TestHandleUse_AppliesConditionWhenConditionIDSet(t *testing.T) {
    // Build a server with:
    // - a condRegistry containing "surge_active" condition
    // - a session with a feat that has Active=true, ConditionID="surge_active"
    // - Call handleUse(uid, "brutal_surge") or however the feat is identified
    // Assert sess.Conditions.Has("surge_active") == true after the call
    // Assert the response contains the ActivateText
}
```

Look at existing `handleUse` tests in the file to understand setup. Use the `fixedDiceSource` pattern if dice are needed.

Run:
```bash
go test -run TestHandleUse_AppliesConditionWhenConditionIDSet -v ./internal/gameserver/...
```
Expected: FAIL

**Step 3: Extend `handleUse`**

In `internal/gameserver/grpc_service.go`, find `handleUse`. After finding the matching feat/class feature and before returning the response, add:

```go
// Apply condition if the feat/feature has one.
condID := "" // set from found feat or class feature
// ... (already have the matched item)
if condID != "" && sess.Conditions != nil && s.condRegistry != nil {
    if def, ok := s.condRegistry.Get(condID); ok {
        if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil {
            s.logger.Warn("failed to apply feat condition",
                zap.String("condition_id", condID),
                zap.Error(err),
            )
        }
    } else {
        s.logger.Warn("feat condition not found in registry",
            zap.String("condition_id", condID),
        )
    }
}
```

The actual `condID` value comes from `feat.ConditionID` or `classFeature.ConditionID` depending on which matched.

**Step 4: Add property test (SWENG-5a)**

```go
func TestProperty_HandleUse_ConditionIDEmpty_NoConditionApplied(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        // Feat with ConditionID="" should not apply any condition
        // Build server + session, call handleUse, assert Conditions is still empty
    })
}
```

**Step 5: Run full suite**

```bash
go test ./internal/game/ruleset/... ./internal/gameserver/...
```
Expected: all pass

**Step 6: Commit**

```bash
git add internal/game/ruleset/feat.go internal/game/ruleset/class_feature.go internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go
git commit -m "feat: add ConditionID to Feat/ClassFeature; handleUse applies condition when ConditionID set"
```

---

## Task 4: Clear encounter conditions on combat end

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`
- Test: `internal/gameserver/combat_handler_test.go` (create or add to existing)

**Context:** When combat ends, all conditions with `DurationType == "encounter"` must be cleared from every participating player's session. Add an `onCombatEndFn func(roomID string)` callback to `CombatHandler` that `grpc_service.go` registers to call `sess.Conditions.ClearEncounter()` for all sessions in the room.

**Step 1: Read `CombatHandler` struct**

Read `internal/gameserver/combat_handler.go`. Find the struct definition and `NewCombatHandler` constructor. Find where `h.engine.EndCombat(roomID)` is called.

**Step 2: Add `onCombatEndFn` to `CombatHandler`**

In the `CombatHandler` struct, add:

```go
onCombatEndFn func(roomID string) // optional; called after combat ends
```

In `resolveAndAdvanceLocked` (or wherever `EndCombat` is called), after `h.engine.EndCombat(roomID)`, add:

```go
if h.onCombatEndFn != nil {
    h.onCombatEndFn(roomID)
}
```

Add a setter method:

```go
// SetOnCombatEnd registers a callback invoked after each combat ends.
// Precondition: fn may be nil (no-op).
func (h *CombatHandler) SetOnCombatEnd(fn func(roomID string)) {
    h.onCombatEndFn = fn
}
```

**Step 3: Write failing test**

In `internal/gameserver/combat_handler_test.go` (create if absent), add:

```go
func TestCombatHandler_OnCombatEndCallback_Called(t *testing.T) {
    // Build a minimal CombatHandler
    // Register a callback that records the roomID
    // Simulate combat end (trigger resolveAndAdvanceLocked reaching the end condition)
    // Assert callback was called with the correct roomID
}
```

This test may be complex. If the combat handler is hard to unit-test directly, test the integration: verify that when a combat ends in a full-server test, the callback fires.

**Step 4: Register callback in `grpc_service.go`**

In `NewGameServiceServer` (or wherever `CombatHandler` is initialized), after creating the combat handler, register the cleanup callback:

```go
s.combatH.SetOnCombatEnd(func(roomID string) {
    // Clear encounter conditions from all sessions in the room.
    room, ok := s.world.GetRoom(roomID)
    if !ok {
        return
    }
    for _, uid := range room.ActivePlayerUIDs() { // find the correct method to get player UIDs in a room
        sess, ok := s.sessions.GetPlayer(uid)
        if !ok {
            continue
        }
        sess.Conditions.ClearEncounter()
    }
})
```

Find the correct method to enumerate players in a room. Look at `session.Manager` — it may have `GetPlayersInRoom(roomID string)` or similar. Check how `broadcastFn` works to understand how room players are enumerated.

**Step 5: Add property test (SWENG-5a)**

```go
func TestProperty_ClearEncounter_CalledForAllSessionsInRoom(t *testing.T) {
    // Property: after combat end callback fires, no encounter conditions remain
    // on any session that was in the combat room
    rapid.Check(t, func(rt *rapid.T) {
        // Build sessions with encounter conditions, fire the callback, assert cleared
    })
}
```

**Step 6: Run full suite**

```bash
go test ./internal/gameserver/... ./internal/game/condition/...
```
Expected: all pass

**Step 7: Commit**

```bash
git add internal/gameserver/combat_handler.go internal/gameserver/grpc_service.go internal/gameserver/combat_handler_test.go
git commit -m "feat: add onCombatEnd callback to CombatHandler; clear encounter conditions after combat"
```

---

## Task 5: Sample content + FEATURES.md + deploy

**Files:**
- Create: `content/conditions/brutal_surge_active.yaml`
- Modify: `content/feats.yaml` — add `condition_id: brutal_surge_active` to the `brutal_surge` feat
- Modify: `content/class_features.yaml` — add `condition_id` to one active class feature
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Create `brutal_surge_active` condition**

Create `content/conditions/brutal_surge_active.yaml`:

```yaml
id: brutal_surge_active
name: Brutal Surge Active
description: |
  You are in a combat frenzy. Your strikes hit harder but your guard is down.
  +2 melee damage, -2 AC for the duration of this encounter.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 2
damage_bonus: 2
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 2: Wire `brutal_surge` feat to `brutal_surge_active` condition**

Read `content/feats.yaml`. Find the `brutal_surge` feat (or whichever active feat is most appropriate for a damage/AC trade-off). Add:

```yaml
condition_id: brutal_surge_active
```

If `brutal_surge` doesn't exist, find any active feat and add the `condition_id` to it.

**Step 3: Wire one active class feature**

Read `content/class_features.yaml`. Find an active class feature (one with `active: true`). Add a `condition_id` to it, creating a corresponding condition YAML file as needed.

**Step 4: Mark Stage 3 complete in FEATURES.md**

In `docs/requirements/FEATURES.md`, mark the Stage 3 subitems complete:

```markdown
- [x] **Active feat/class feature mechanics — Stage 3**
  - [x] Active feats/class features map to a named condition via `condition_id` in YAML
  - [x] `use <feat>` applies the condition (with `damage_bonus`, `ac_penalty`, etc.) for encounter or timed duration
  - [x] Condition cleared on combat end or timer expiry
```

**Step 5: Build + test**

```bash
make build-gameserver 2>&1 | tail -5
go test ./internal/game/... ./internal/gameserver/... ./internal/frontend/...
```
All pass.

**Step 6: Commit**

```bash
git add content/ docs/requirements/FEATURES.md
git commit -m "feat: Stage 3 sample content (brutal_surge_active condition, feat wiring); FEATURES.md"
```

**Step 7: Deploy**

```bash
make k8s-redeploy 2>&1 | tail -10
```

Report Helm revision number.
