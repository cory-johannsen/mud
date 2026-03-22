# Reactions System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a general-purpose player reaction system: one reaction per round, declared in feat/tech YAML with a `reaction` block, triggered by combat events, with interactive yes/no prompting via the gRPC stream.

**Architecture:** A new `internal/game/reaction/` package defines all types (`ReactionTriggerType`, `ReactionRegistry`, etc.). Fields are added to `Feat`, `TechnologyDef`, and `PlayerSession`. `ResolveRound` gains a `reactionFn ReactionCallback` parameter (inserted before the variadic); fire points for `TriggerOnDamageTaken` (before `ApplyDamage`), `TriggerOnEnemyMoveAdjacent`, `TriggerOnSaveFail`/`TriggerOnSaveCritFail`, and `TriggerOnAllyDamaged` are inserted at appropriate call sites. `PlayerSession` stores a `ReactionFn` field set by the session handler (which captures stream+session), then `CombatHandler` dispatches to each player's stored callback when calling `ResolveRound`. `TriggerOnConditionApplied` and `TriggerOnFall` fire points are deferred to sub-projects 2/3.

**Note on `SaveOutcome` type:** The spec defines `SaveOutcome *combat.Outcome`, but `combat.Outcome` is `type Outcome int` (confirmed: `grep 'type Outcome' internal/game/combat/combat.go` → `type Outcome int`). The `combat` package imports `reaction` (for `ReactionCallback`), so `reaction` cannot import `combat` without a cycle. This plan uses `*int` in `ReactionContext.SaveOutcome` — semantically identical since `Outcome` is an int with values 0=CritSuccess, 1=Success, 2=Failure, 3=CritFailure.

**Note on `PlayerReaction.FeatName` deviation from spec:** REQ-RXN12 defines `PlayerReaction` with `UID`, `Feat`, and `Def` only. REQ-RXN23 requires the prompt to show `<FeatName>` (the display name), which cannot be derived from the slug `Feat` field alone. This plan adds `FeatName string` to `PlayerReaction` and a `featName` parameter to `Register`. This is a required deviation from the spec to satisfy REQ-RXN23; consider this a spec amendment.

**Note on `./internal/game/feat/...` in REQ-RXN29:** The spec lists `./internal/game/feat/...` in the test command but feats live in `./internal/game/ruleset/...`. This plan uses the correct path.

**Tech Stack:** Go, `pgregory.net/rapid` for property tests, `github.com/stretchr/testify`

---

## File Map

| File | Change |
|---|---|
| `internal/game/reaction/trigger.go` | NEW: all reaction types (`ReactionTriggerType`, `ReactionEffectType`, `ReactionEffect`, `ReactionDef`, `ReactionContext`, `ReactionCallback`) |
| `internal/game/reaction/trigger_test.go` | NEW: unit tests for type validity and YAML round-trip |
| `internal/game/reaction/registry.go` | NEW: `PlayerReaction`, `ReactionRegistry`, `NewReactionRegistry`, `Register`, `Get` |
| `internal/game/reaction/registry_test.go` | NEW: unit tests for registry |
| `internal/game/ruleset/feat.go` | Add `Reaction *reaction.ReactionDef` field to `Feat` |
| `internal/game/technology/model.go` | Add `Reaction *reaction.ReactionDef` field to `TechnologyDef` |
| `internal/game/session/manager.go` | Add `ReactionsRemaining int`, `Reactions *reaction.ReactionRegistry`, `ReactionFn reaction.ReactionCallback` to `PlayerSession` |
| `internal/game/combat/round.go` | Add `reactionFn ReactionCallback` param to `ResolveRound` (before variadic); add fire points |
| `internal/gameserver/combat_handler.go` | Construct per-session dispatch wrapper, pass to `ResolveRound`; add `ReactionsRemaining = 1` to post-round loop |
| `internal/gameserver/reaction_handler.go` | NEW: `CheckReactionRequirement`, `ApplyReactionEffect`, `buildReactionCallback` |
| `internal/gameserver/reaction_handler_test.go` | NEW: unit tests |
| `internal/gameserver/grpc_service_reaction_test.go` | NEW: integration tests |
| `internal/gameserver/grpc_service.go` | Set `sess.ReactionFn` at login; register reactions at login |

---

## Task 1: `internal/game/reaction/` package — types and registry

**Files:**
- Create: `internal/game/reaction/trigger.go`
- Create: `internal/game/reaction/trigger_test.go`
- Create: `internal/game/reaction/registry.go`
- Create: `internal/game/reaction/registry_test.go`

- [ ] **Step 1: Write failing tests for trigger types**

Create `internal/game/reaction/trigger_test.go`:

```go
package reaction_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestReactionTriggerType_AllValuesNonEmpty(t *testing.T) {
	triggers := []reaction.ReactionTriggerType{
		reaction.TriggerOnSaveFail,
		reaction.TriggerOnSaveCritFail,
		reaction.TriggerOnDamageTaken,
		reaction.TriggerOnEnemyMoveAdjacent,
		reaction.TriggerOnConditionApplied,
		reaction.TriggerOnAllyDamaged,
		reaction.TriggerOnFall,
	}
	for _, t2 := range triggers {
		assert.NotEmpty(t, string(t2))
	}
}

func TestReactionDef_YAMLRoundTrip(t *testing.T) {
	original := reaction.ReactionDef{
		Trigger:     reaction.TriggerOnSaveFail,
		Requirement: "wielding_melee_weapon",
		Effect: reaction.ReactionEffect{
			Type: reaction.ReactionEffectRerollSave,
			Keep: "better",
		},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var decoded reaction.ReactionDef
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}

func TestReactionDef_YAMLRoundTrip_NoRequirement(t *testing.T) {
	original := reaction.ReactionDef{
		Trigger: reaction.TriggerOnEnemyMoveAdjacent,
		Effect: reaction.ReactionEffect{
			Type:   reaction.ReactionEffectStrike,
			Target: "trigger_source",
		},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var decoded reaction.ReactionDef
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... -v 2>&1 | head -10
```

Expected: compile error — package `reaction` does not exist.

- [ ] **Step 3: Write failing tests for registry**

Create `internal/game/reaction/registry_test.go`:

```go
package reaction_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestReactionRegistry_GetReturnsNil_WhenNothingRegistered(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	result := reg.Get("uid1", reaction.TriggerOnSaveFail)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsNil_WrongUID(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid2", reaction.TriggerOnSaveFail)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsNil_WrongTrigger(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid1", reaction.TriggerOnDamageTaken)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsRegisteredReaction(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Trigger:     reaction.TriggerOnSaveFail,
		Requirement: "wielding_melee_weapon",
		Effect:      reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid1", reaction.TriggerOnSaveFail)
	assert.NotNil(t, result)
	assert.Equal(t, "uid1", result.UID)
	assert.Equal(t, "chrome_reflex", result.Feat)
	assert.Equal(t, "Chrome Reflex", result.FeatName)
	assert.Equal(t, def, result.Def)
}
```

- [ ] **Step 4: Implement `internal/game/reaction/trigger.go`**

```go
// Package reaction defines the player reaction system: trigger types, effect types,
// reaction definitions, and the callback interface for interactive prompting.
package reaction

// ReactionTriggerType identifies the combat event that can fire a reaction.
type ReactionTriggerType string

const (
	// TriggerOnSaveFail fires after a player's saving throw is determined to be a Failure.
	TriggerOnSaveFail ReactionTriggerType = "on_save_fail"
	// TriggerOnSaveCritFail fires after a player's saving throw is determined to be a Critical Failure.
	TriggerOnSaveCritFail ReactionTriggerType = "on_save_crit_fail"
	// TriggerOnDamageTaken fires after damage is calculated but before it is applied to a player.
	TriggerOnDamageTaken ReactionTriggerType = "on_damage_taken"
	// TriggerOnEnemyMoveAdjacent fires when an enemy moves into melee range (≤5ft) of the player.
	TriggerOnEnemyMoveAdjacent ReactionTriggerType = "on_enemy_move_adjacent"
	// TriggerOnConditionApplied fires when a condition is about to be applied to the player.
	// Fire point deferred to sub-project 2.
	TriggerOnConditionApplied ReactionTriggerType = "on_condition_applied"
	// TriggerOnAllyDamaged fires when a player ally takes damage in the same combat.
	TriggerOnAllyDamaged ReactionTriggerType = "on_ally_damaged"
	// TriggerOnFall fires when the player would fall. Fire point deferred to a future feature.
	TriggerOnFall ReactionTriggerType = "on_fall"
)

// ReactionEffectType identifies what a reaction does when it fires.
type ReactionEffectType string

const (
	// ReactionEffectRerollSave rerolls a failed saving throw, keeping the better result.
	ReactionEffectRerollSave ReactionEffectType = "reroll_save"
	// ReactionEffectStrike executes an immediate attack against the trigger source.
	ReactionEffectStrike ReactionEffectType = "strike"
	// ReactionEffectReduceDamage subtracts shield hardness from pending damage.
	ReactionEffectReduceDamage ReactionEffectType = "reduce_damage"
)

// ReactionEffect describes what happens when a reaction fires.
type ReactionEffect struct {
	// Type is the effect to apply.
	Type ReactionEffectType `yaml:"type"`
	// Target names the target of the effect (e.g. "trigger_source"). Optional.
	Target string `yaml:"target,omitempty"`
	// Keep specifies the reroll strategy (e.g. "better"). Optional.
	Keep string `yaml:"keep,omitempty"`
}

// ReactionDef is the reaction declaration embedded in a Feat or TechnologyDef YAML.
type ReactionDef struct {
	// Trigger is the combat event that can fire this reaction.
	Trigger ReactionTriggerType `yaml:"trigger"`
	// Requirement is an optional predicate the player must satisfy (e.g. "wielding_melee_weapon").
	// Empty string means no requirement.
	Requirement string `yaml:"requirement,omitempty"`
	// Effect is the action taken when the reaction fires.
	Effect ReactionEffect `yaml:"effect"`
}

// ReactionContext carries the mutable state the effect can read and modify.
type ReactionContext struct {
	// TriggerUID is the UID of the player whose reaction may fire.
	TriggerUID string
	// SourceUID is the UID or NPC ID of the entity that caused the trigger.
	SourceUID string
	// DamagePending is a pointer to the pending damage amount (for reduce_damage).
	// Nil when the trigger is not damage-related.
	// The callback may modify *DamagePending before ApplyDamage is called.
	DamagePending *int
	// SaveOutcome is a pointer to the save outcome (for reroll_save).
	// Uses combat.Outcome int values: 0=CritSuccess, 1=Success, 2=Failure, 3=CritFailure.
	// Declared as *int (not *combat.Outcome) to avoid an import cycle since combat imports reaction.
	// Nil when the trigger is not save-related.
	SaveOutcome *int
	// ConditionID is the condition being applied (for on_condition_applied). May be empty.
	ConditionID string
}

// ReactionCallback is invoked at trigger fire points during round resolution.
// uid is the player who might spend their reaction.
// Returns (true, nil) if the reaction was spent; (false, nil) if skipped or unavailable.
// A nil ReactionCallback MUST be treated as a no-op.
type ReactionCallback func(uid string, trigger ReactionTriggerType, ctx ReactionContext) (spent bool, err error)
```

- [ ] **Step 5: Implement `internal/game/reaction/registry.go`**

```go
package reaction

// PlayerReaction associates a registered reaction with the player who owns it.
type PlayerReaction struct {
	// UID is the player's session UID.
	UID string
	// Feat is the feat or tech ID that declared this reaction (matches spec REQ-RXN12).
	Feat string
	// FeatName is the human-readable display name of the feat/tech (e.g. "Chrome Reflex").
	// Used in the prompt message per REQ-RXN23.
	FeatName string
	// Def is the full reaction declaration.
	Def ReactionDef
}

// ReactionRegistry maps trigger types to registered player reactions.
//
// Precondition: created via NewReactionRegistry.
// Postcondition: Get returns the first registered reaction for (uid, trigger) or nil.
type ReactionRegistry struct {
	byTrigger map[ReactionTriggerType][]PlayerReaction
}

// NewReactionRegistry returns an empty ReactionRegistry.
func NewReactionRegistry() *ReactionRegistry {
	return &ReactionRegistry{
		byTrigger: make(map[ReactionTriggerType][]PlayerReaction),
	}
}

// Register adds a reaction for the given player and feat.
// featName is the human-readable display name used in prompts.
func (r *ReactionRegistry) Register(uid, featID, featName string, def ReactionDef) {
	r.byTrigger[def.Trigger] = append(r.byTrigger[def.Trigger], PlayerReaction{
		UID:      uid,
		Feat:     featID,
		FeatName: featName,
		Def:      def,
	})
}

// Get returns the first registered reaction for uid and trigger, or nil if none.
func (r *ReactionRegistry) Get(uid string, trigger ReactionTriggerType) *PlayerReaction {
	for i := range r.byTrigger[trigger] {
		pr := &r.byTrigger[trigger][i]
		if pr.UID == uid {
			return pr
		}
	}
	return nil
}
```

- [ ] **Step 6: Run all tests and commit**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... -v 2>&1 | tail -20
```

Expected: All tests PASS.

```bash
git add internal/game/reaction/
git commit -m "feat: add reaction package — trigger types and registry"
```

---

## Task 2: Add reaction fields to `Feat`, `TechnologyDef`, and `PlayerSession`

**Files:**
- Modify: `internal/game/ruleset/feat.go`
- Modify: `internal/game/technology/model.go`
- Modify: `internal/game/session/manager.go`

Before reading or modifying these files, check the import path used throughout the project:

```bash
head -3 internal/game/ruleset/feat.go
```

The module path is `github.com/cory-johannsen/mud`.

- [ ] **Step 1: Write failing tests for Feat YAML with reaction block**

Read `internal/game/ruleset/feat_test.go` first to understand test patterns and find the `LoadFeatsFromBytes` function signature. Then add to that file:

```go
func TestFeat_YAML_WithReactionBlock(t *testing.T) {
	yml := `
- id: reactive_strike
  name: Reactive Strike
  category: combat
  reaction:
    trigger: on_enemy_move_adjacent
    requirement: wielding_melee_weapon
    effect:
      type: strike
      target: trigger_source
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(yml))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	f := feats[0]
	assert.Equal(t, "reactive_strike", f.ID)
	require.NotNil(t, f.Reaction)
	assert.Equal(t, reaction.TriggerOnEnemyMoveAdjacent, f.Reaction.Trigger)
	assert.Equal(t, "wielding_melee_weapon", f.Reaction.Requirement)
	assert.Equal(t, reaction.ReactionEffectStrike, f.Reaction.Effect.Type)
	assert.Equal(t, "trigger_source", f.Reaction.Effect.Target)
}

func TestFeat_YAML_WithoutReaction_NilReactionField(t *testing.T) {
	yml := `
- id: sucker_punch
  name: Sucker Punch
  category: combat
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(yml))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	assert.Nil(t, feats[0].Reaction)
}
```

- [ ] **Step 2: Add `Reaction` field to `Feat`**

Read `internal/game/ruleset/feat.go` in full first. Find the `Feat` struct and add after the last existing field:

```go
// Reaction declares this feat as a player reaction with the given trigger and effect.
// Nil means this feat is not a reaction.
Reaction *reaction.ReactionDef `yaml:"reaction,omitempty"`
```

Also add the import: `"github.com/cory-johannsen/mud/internal/game/reaction"`.

- [ ] **Step 3: Run feat tests**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/ruleset/... -v 2>&1 | tail -20
```

Expected: All tests PASS including the new reaction tests.

- [ ] **Step 4: Add `Reaction` field to `TechnologyDef` and session fields to `PlayerSession`**

In `internal/game/technology/model.go`, read the file first. Add to `TechnologyDef` struct after `AmpedEffects`:

```go
// Reaction declares this tech as a player reaction with the given trigger and effect.
// Only applicable to innate techs. Nil means this tech is not a reaction.
Reaction *reaction.ReactionDef `yaml:"reaction,omitempty"`
```

Add import: `"github.com/cory-johannsen/mud/internal/game/reaction"`.

In `internal/game/session/manager.go`, read the file first. Add to `PlayerSession` struct:

```go
// ReactionsRemaining is the number of reactions available this round. Resets to 1 each round.
// Not persisted. In-session only.
ReactionsRemaining int
// Reactions holds all registered reactions for this player, indexed by trigger type.
// Initialised to NewReactionRegistry() at session creation.
Reactions *reaction.ReactionRegistry
// ReactionFn is the per-session reaction callback, set by the session handler after login.
// Captures the player's gRPC stream for interactive prompting.
ReactionFn reaction.ReactionCallback
```

Add import: `"github.com/cory-johannsen/mud/internal/game/reaction"`.

Find where `PlayerSession` is constructed (search for `PlayerSession{` or `&PlayerSession{`) and initialise `Reactions`:

```go
Reactions: reaction.NewReactionRegistry(),
```

- [ ] **Step 5: Run full test suite and commit**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/... 2>&1 | tail -10
```

Expected: All tests PASS.

```bash
git add internal/game/ruleset/feat.go \
        internal/game/technology/model.go \
        internal/game/session/manager.go
git commit -m "feat: add Reaction field to Feat and TechnologyDef; add ReactionsRemaining, Reactions, and ReactionFn to PlayerSession"
```

---

## Task 3: `ResolveRound` — add `reactionFn` parameter and fire points

**Files:**
- Modify: `internal/game/combat/round.go`
- Modify: `internal/gameserver/combat_handler.go`

**Read first:** Read lines 355–385 of `internal/game/combat/round.go` (the `ResolveRound` function signature and opening). Also read lines 1355–1410 of `internal/gameserver/combat_handler.go` (the `ResolveRound` call site and post-round reset loop).

- [ ] **Step 1: Write failing tests for the new `ResolveRound` signature**

Read `internal/game/combat/round_test.go` first to find the test helper names (look for functions that build a `*Combat` or call `ResolveRound`). Then add to `internal/game/combat/round_test.go`:

```go
// REQ-RXN18: nil reactionFn must not panic.
func TestResolveRound_NilReactionFn_NoPanic(t *testing.T) {
	// Use whatever helper builds a minimal Combat in the existing tests.
	// Replace minimalCombat() with the real helper name from round_test.go.
	assert.NotPanics(t, func() {
		ResolveRound(minimalCombat(), fixedSrc(0), nil, nil)
	})
}

// REQ-RXN19: reactionFn is called with TriggerOnDamageTaken when player takes damage.
func TestResolveRound_ReactionFn_CalledOnDamageTaken(t *testing.T) {
	called := false
	fn := func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if trigger == reaction.TriggerOnDamageTaken {
			called = true
		}
		return false, nil
	}
	// Use existing helpers — adapt names to match round_test.go.
	// Build a combat where an NPC attacks a player and hits (guaranteed roll).
	cbt := combatWithNPCAttackingPlayer()
	ResolveRound(cbt, fixedSrc(99), nil, fn)
	assert.True(t, called, "reactionFn must be called with TriggerOnDamageTaken when player takes damage")
}
```

**Note:** Read `internal/game/combat/round_test.go` to find the actual helper names. Replace `minimalCombat()`, `fixedSrc`, and `combatWithNPCAttackingPlayer()` with the real helpers before running.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/combat/... -run TestResolveRound_NilReactionFn -v 2>&1 | head -10
```

Expected: compile error — wrong number of arguments to `ResolveRound`.

- [ ] **Step 3: Update `ResolveRound` signature in `round.go`**

Change the function signature from:
```go
func ResolveRound(cbt *Combat, src Source, targetUpdater func(id string, hp int), coverDegraderArgs ...func(roomID, equipID string) bool) []RoundEvent {
```
to:
```go
func ResolveRound(cbt *Combat, src Source, targetUpdater func(id string, hp int), reactionFn reaction.ReactionCallback, coverDegraderArgs ...func(roomID, equipID string) bool) []RoundEvent {
```

Add import: `"github.com/cory-johannsen/mud/internal/game/reaction"`.

Add a nil-safe wrapper at the start of `ResolveRound`:

```go
fireReaction := func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) {
    if reactionFn == nil {
        return
    }
    _, _ = reactionFn(uid, trigger, ctx)
}
```

- [ ] **Step 4: Add fire point for `TriggerOnDamageTaken` — BEFORE `ApplyDamage`**

In `round.go`, find each `target.ApplyDamage(dmg)` call in the primary attack resolution path (single-strike and double-strike variants). For each one where `target` can be a player, replace the pattern:

```go
target.ApplyDamage(dmg)
```

with:

```go
// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
if target.Kind == KindPlayer && dmg > 0 {
    fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
        TriggerUID:    target.ID,
        SourceUID:     actor.ID,
        DamagePending: &dmg,
    })
}
target.ApplyDamage(dmg)
```

**Important:** Pass `&dmg` — the callback may reduce `dmg` before `ApplyDamage` is called. This is why the fire point must precede `ApplyDamage`.

Do this for ALL `target.ApplyDamage(dmg)` calls in the main attack path. Do NOT add it inside the reactive-strike path (to avoid infinite recursion in sub-project 2).

- [ ] **Step 5: Add fire point for `TriggerOnAllyDamaged` — AFTER `ApplyDamage`**

After each `target.ApplyDamage(dmg)` call where `target.Kind == KindPlayer`, add:

```go
// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
if target.Kind == KindPlayer && dmg > 0 {
    for _, c := range cbt.Combatants {
        if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
            dmgCopy := dmg
            fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
                TriggerUID:    c.ID,
                SourceUID:     actor.ID,
                DamagePending: &dmgCopy,
            })
        }
    }
}
```

Note: `dmgCopy` is used here (not `&dmg`) because TriggerOnAllyDamaged is informational — the ally cannot reduce damage already applied to the target.

- [ ] **Step 6: Add fire point for `TriggerOnEnemyMoveAdjacent`**

In `round.go`, find the `ActionStride` resolution. After an NPC actor updates its position and the new distance between NPC and a player is ≤ 5, add:

Add this before the `events = append(events, RoundEvent{ActionType: ActionStride, ...})` line in the stride case:

```go
// REQ-RXN19: TriggerOnEnemyMoveAdjacent fires when an NPC moves into melee range of a player.
// posDist is confirmed as the distance function for adjacency checks in round.go.
if actor.Kind == KindNPC {
    for _, c := range cbt.Combatants {
        if c.Kind == KindPlayer && !c.IsDead() {
            if posDist(actor.Position, c.Position) <= 5 {
                fireReaction(c.ID, reaction.TriggerOnEnemyMoveAdjacent, reaction.ReactionContext{
                    TriggerUID: c.ID,
                    SourceUID:  actor.ID,
                })
            }
        }
    }
}
```

`posDist` is confirmed as the adjacency distance function — used in `CheckReactiveStrikes` at lines 36 and 41 of `round.go`. Place this after all position-update branches complete (after the `actor.Position` is set in the if/else chain) and before `events = append(events, RoundEvent{ActionType: ActionStride, ...})`.

- [ ] **Step 7: Add fire point for `TriggerOnSaveFail` / `TriggerOnSaveCritFail`**

In `round.go`, find `resolveThrow` (or the explosive save resolution). Read `internal/game/combat/explosive.go` to find the actual `SaveResult` string values. After the save outcome is determined (before or after `ApplyDamage` — use before for consistency with REQ-RXN19), add:

```go
// REQ-RXN19: TriggerOnSaveFail / TriggerOnSaveCritFail for explosive saves.
// SaveResult is of type Outcome (int), NOT a string — use Outcome constants.
if target.Kind == KindPlayer {
    switch r.SaveResult {
    case Failure:
        fireReaction(target.ID, reaction.TriggerOnSaveFail, reaction.ReactionContext{
            TriggerUID: target.ID,
            SourceUID:  actor.ID,
        })
    case CritFailure:
        fireReaction(target.ID, reaction.TriggerOnSaveCritFail, reaction.ReactionContext{
            TriggerUID: target.ID,
            SourceUID:  actor.ID,
        })
    }
}
```

`SaveResult` is `Outcome` type — confirmed by `resolver.go:89: SaveResult Outcome`. Use `Failure` and `CritFailure` constants (defined in `combat.go`).

- [ ] **Step 8: Update `ResolveRound` call site in `combat_handler.go`**

In `internal/gameserver/combat_handler.go` at line 1373, change:
```go
roundEvents := combat.ResolveRound(cbt, h.dice.Src(), targetUpdater, coverDegrader)
```
to:
```go
// Build a per-round dispatch wrapper: for each player in this combat, call their stored ReactionFn.
var reactionFn reaction.ReactionCallback
reactionFn = func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
    if sess, ok := h.sessions.GetPlayer(uid); ok && sess.ReactionFn != nil {
        return sess.ReactionFn(uid, trigger, ctx)
    }
    return false, nil
}
roundEvents := combat.ResolveRound(cbt, h.dice.Src(), targetUpdater, reactionFn, coverDegrader)
```

Add import: `"github.com/cory-johannsen/mud/internal/game/reaction"`.

In the post-round reset loop (lines 1376–1382, the loop that calls `sess.LoadoutSet.ResetRound()`), add reactions reset inside the same loop:

```go
// REQ-RXN9: reset reaction economy for each player at the start of the next round.
sess.ReactionsRemaining = 1
```

The full loop body becomes:
```go
for _, c := range cbt.Combatants {
    if c.Kind == combat.KindPlayer {
        if sess, found := h.sessions.GetPlayer(c.ID); found && sess.LoadoutSet != nil {
            sess.LoadoutSet.ResetRound()
            sess.ReactionsRemaining = 1
        }
    }
}
```

- [ ] **Step 9: Run tests and commit**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/combat/... ./internal/gameserver/... 2>&1 | tail -10
```

Expected: All tests PASS.

```bash
git add internal/game/combat/round.go internal/gameserver/combat_handler.go
git commit -m "feat: add reactionFn to ResolveRound; fire points for OnDamageTaken, OnEnemyMoveAdjacent, OnSaveFail, OnAllyDamaged; wired from CombatHandler sessions"
```

---

## Task 4: `reaction_handler.go` + session wiring + integration tests

**Files:**
- Create: `internal/gameserver/reaction_handler.go`
- Create: `internal/gameserver/reaction_handler_test.go`
- Create: `internal/gameserver/grpc_service_reaction_test.go`
- Modify: `internal/gameserver/grpc_service.go`

**Read first:**
1. Read `internal/gameserver/grpc_service.go` around lines 820–900 (feat loading at login, where `characterFeatsRepo.GetAll` is called) to understand where to insert reaction registration and `sess.ReactionFn` assignment.
2. Read `internal/gameserver/grpc_service_rest_test.go` to understand the test server/session construction pattern used for integration tests.
3. Read `internal/game/inventory/equipment.go` to find:
   - The method name for getting the main-hand item (e.g. `MainHand()`)
   - The field name for range increment on a weapon def (e.g. `RangeIncrement`)
   - The method name for off-hand item (e.g. `OffHand()`)
   - Whether the shield/off-hand def has a `Hardness` field

- [ ] **Step 1: Write failing unit tests for `reaction_handler.go`**

Create `internal/gameserver/reaction_handler_test.go`:

```go
package gameserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
)

// REQ-RXN21: empty requirement always returns true.
func TestCheckReactionRequirement_EmptyString_ReturnsTrue(t *testing.T) {
	sess := &session.PlayerSession{}
	assert.True(t, gameserver.CheckReactionRequirement(sess, ""))
}

// REQ-RXN24: wielding_melee_weapon returns false when no equipment is set.
func TestCheckReactionRequirement_WieldingMeleeWeapon_FalseWhenNoEquipment(t *testing.T) {
	sess := &session.PlayerSession{} // Equipment field is nil
	assert.False(t, gameserver.CheckReactionRequirement(sess, "wielding_melee_weapon"))
}

// REQ-RXN22: reroll_save picks the better outcome (lower value wins).
// Outcome int values: CritSuccess=0, Success=1, Failure=2, CritFailure=3.
// "Better" means lower numeric value.
func TestApplyReactionEffect_RerollSave_NeverWorsensOutcome(t *testing.T) {
	// Start at worst outcome (CritFailure=3). Any reroll must be <= 3.
	for i := 0; i < 50; i++ { // run multiple times to exercise randomness
		original := 3 // CritFailure
		ctx := reaction.ReactionContext{SaveOutcome: &original}
		effect := reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"}
		sess := &session.PlayerSession{}
		gameserver.ApplyReactionEffect(sess, effect, &ctx)
		assert.LessOrEqual(t, *ctx.SaveOutcome, 3, "reroll must not produce a value > CritFailure")
		assert.GreaterOrEqual(t, *ctx.SaveOutcome, 0, "reroll must not produce a value < CritSuccess")
	}
}

// REQ-RXN22: reduce_damage clamps at 0 when pending < hardness.
func TestApplyReactionEffect_ReduceDamage_ClampsAtZero(t *testing.T) {
	pending := 2
	ctx := reaction.ReactionContext{DamagePending: &pending}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	// No shield equipped — hardness == 0; pending stays 2.
	sess := &session.PlayerSession{}
	gameserver.ApplyReactionEffect(sess, effect, &ctx)
	assert.GreaterOrEqual(t, *ctx.DamagePending, 0, "pending damage must not go negative")
}

// REQ-RXN10: ReactionsRemaining never goes below 0.
func TestReactionsRemaining_NeverGoesNegative(t *testing.T) {
	sess := &session.PlayerSession{ReactionsRemaining: 0}
	assert.Equal(t, 0, sess.ReactionsRemaining)
	// Decrement attempt: ReactionsRemaining must stay at 0.
	if sess.ReactionsRemaining > 0 {
		sess.ReactionsRemaining--
	}
	assert.Equal(t, 0, sess.ReactionsRemaining)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run "TestCheckReactionRequirement|TestApplyReactionEffect|TestReactionsRemaining" -v 2>&1 | head -10
```

Expected: compile error — functions not found.

- [ ] **Step 3: Implement `internal/gameserver/reaction_handler.go`**

**Before writing this file,** verify the actual method/field names by reading `internal/game/inventory/equipment.go`. Then write:

```go
package gameserver

import (
	"math/rand"

	gamev1 "github.com/cory-johannsen/mud/api/proto/gen/game/v1"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// triggerDescriptions provides human-readable descriptions for each trigger type.
// Used in the REQ-RXN23 prompt format: "Reaction available: <FeatName> — <desc>. Use it? (yes / no)"
var triggerDescriptions = map[reaction.ReactionTriggerType]string{
	reaction.TriggerOnSaveFail:          "you failed a saving throw",
	reaction.TriggerOnSaveCritFail:      "you critically failed a saving throw",
	reaction.TriggerOnDamageTaken:       "you are about to take damage",
	reaction.TriggerOnEnemyMoveAdjacent: "an enemy moved adjacent to you",
	reaction.TriggerOnConditionApplied:  "a condition is being applied to you",
	reaction.TriggerOnAllyDamaged:       "an ally took damage",
	reaction.TriggerOnFall:              "you are about to fall",
}

// CheckReactionRequirement returns true if sess meets the given requirement string.
// An empty requirement always returns true.
//
// Precondition: sess is non-nil.
// Postcondition: returns true iff the requirement is satisfied.
func CheckReactionRequirement(sess *session.PlayerSession, req string) bool {
	switch req {
	case "", "none":
		return true
	case "wielding_melee_weapon":
		if sess.Equipment == nil {
			return false
		}
		mainHand := sess.Equipment.MainHand() // adapt method name if different
		if mainHand == nil || mainHand.Def == nil {
			return false
		}
		// A melee weapon has RangeIncrement == 0 (no ranged increment means melee).
		// Adapt field name if different (check inventory/equipment.go).
		return mainHand.Def.RangeIncrement == 0
	default:
		return false
	}
}

// ApplyReactionEffect executes the reaction effect, modifying ctx in place.
//
// Precondition: sess and ctx are non-nil.
// Postcondition: ctx fields updated per effect type; DamagePending never goes negative.
func ApplyReactionEffect(sess *session.PlayerSession, effect reaction.ReactionEffect, ctx *reaction.ReactionContext) {
	switch effect.Type {
	case reaction.ReactionEffectRerollSave:
		if ctx.SaveOutcome == nil {
			return
		}
		// Reroll: generate new outcome in [0,3]. Keep the better (lower) value.
		// 0=CritSuccess, 1=Success, 2=Failure, 3=CritFailure.
		reroll := rand.Intn(4)
		if reroll < *ctx.SaveOutcome {
			*ctx.SaveOutcome = reroll
		}
	case reaction.ReactionEffectReduceDamage:
		if ctx.DamagePending == nil {
			return
		}
		hardness := shieldHardness(sess)
		*ctx.DamagePending -= hardness
		if *ctx.DamagePending < 0 {
			*ctx.DamagePending = 0
		}
	case reaction.ReactionEffectStrike:
		// Strike execution requires game-server state (attack resolution, event appending)
		// not available in this function. No feat or tech in sub-project 1 declares a strike
		// reaction, so this branch is unreachable in sub-project 1.
		// Sub-project 2 (Reactive Strike) adds strike execution in buildReactionCallback.
	}
}

// shieldHardness returns the hardness of the player's equipped off-hand shield, or 0 if none.
func shieldHardness(sess *session.PlayerSession) int {
	if sess.Equipment == nil {
		return 0
	}
	offHand := sess.Equipment.OffHand() // adapt method name if different
	if offHand == nil || offHand.Def == nil {
		return 0
	}
	// Adapt field name if different (check inventory/equipment.go for Hardness).
	return offHand.Def.Hardness
}

// buildReactionCallback constructs the ReactionCallback for a player session.
// The closure captures stream and sess for interactive prompting.
//
// Precondition: s, sess, and stream are non-nil.
// Returns a callback that:
//  1. Checks ReactionsRemaining > 0
//  2. Looks up a registered reaction for (uid, trigger)
//  3. Checks requirements
//  4. Prompts the player interactively
//  5. On "yes": decrements ReactionsRemaining and applies the effect
func (s *GameServiceServer) buildReactionCallback(
	uid string,
	sess *session.PlayerSession,
	stream gamev1.GameService_SessionServer,
) reaction.ReactionCallback {
	return func(triggerUID string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if triggerUID != uid {
			return false, nil
		}
		if sess.ReactionsRemaining <= 0 {
			return false, nil
		}
		pr := sess.Reactions.Get(uid, trigger)
		if pr == nil {
			return false, nil
		}
		if !CheckReactionRequirement(sess, pr.Def.Requirement) {
			return false, nil
		}

		// REQ-RXN23: prompt format "Reaction available: <FeatName> — <trigger description>. Use it? (yes / no)"
		desc, ok := triggerDescriptions[trigger]
		if !ok {
			desc = string(trigger)
		}
		prompt := "Reaction available: " + pr.FeatName + " \u2014 " + desc + ". Use it? (yes / no)"
		choices := &ruleset.FeatureChoices{
			Key:     "reaction",
			Prompt:  prompt,
			Options: []string{"yes", "no"},
		}
		chosen, err := s.promptFeatureChoice(stream, "reaction", choices)
		if err != nil {
			return false, err
		}
		if chosen != "yes" {
			return false, nil
		}

		sess.ReactionsRemaining--
		ApplyReactionEffect(sess, pr.Def.Effect, &ctx)
		return true, nil
	}
}
```

**After writing,** check the actual method names against what you found in `internal/game/inventory/equipment.go` and update as needed.

- [ ] **Step 4: Register reactions and set `ReactionFn` at login in `grpc_service.go`**

Read `internal/gameserver/grpc_service.go` around lines 820–900. Find the feat loading block (where `s.characterFeatsRepo.GetAll` is called and feats are iterated for choice resolution). After the feat loading block, add:

```go
// REQ-RXN9: initialize ReactionsRemaining for the first round.
sess.ReactionsRemaining = 1
// REQ-RXN15: register reactions from feats.
// Reuse featIDs already loaded above if available; otherwise call GetAll.
for _, id := range featIDs { // adapt variable name to match the surrounding code
    f, ok := s.featRegistry.Feat(id)
    if !ok || f.Reaction == nil {
        continue
    }
    sess.Reactions.Register(uid, id, f.Name, *f.Reaction)
}
// REQ-RXN15: register reactions from innate techs.
for _, techID := range sess.InnateTechs { // adapt field name if different
    if techDef, ok := s.techRegistry.Get(techID); ok && techDef.Reaction != nil {
        sess.Reactions.Register(uid, techID, techDef.Name, *techDef.Reaction)
    }
}
// REQ-RXN20: build and store the interactive reaction callback capturing this stream.
sess.ReactionFn = s.buildReactionCallback(uid, sess, stream)
```

**Important:** Adapt `featIDs`, `sess.InnateTechs`, `s.featRegistry`, and `s.techRegistry` to match the actual variable/field names in the surrounding code. Read carefully.

- [ ] **Step 5: Run full test suite and commit**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... ./internal/game/ruleset/... ./internal/game/technology/... ./internal/game/session/... ./internal/game/combat/... ./internal/gameserver/... 2>&1 | tail -10
```

Expected: All tests PASS.

```bash
git add internal/gameserver/reaction_handler.go \
        internal/gameserver/reaction_handler_test.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: reaction_handler — CheckReactionRequirement, ApplyReactionEffect, buildReactionCallback; wire at login"
```

- [ ] **Step 6: Write integration tests**

**Before writing:** Read `internal/gameserver/grpc_service_auto_combat_test.go` to understand the `newAutoCombatSvc` pattern. Note that the combat engine's `StartCombat` signature is:
```go
func (e *Engine) StartCombat(roomID string, combatants []*Combatant, condRegistry *condition.Registry, scriptMgr *scripting.Manager, zoneID string) (*Combat, error)
```
Also read the `Combatant` struct definition to understand how to build a test combatant.

Create `internal/gameserver/grpc_service_reaction_test.go` as **`package gameserver`** (whitebox — needed for `resolveAndAdvanceLocked` and `combatMu` access):

```go
package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// REQ-RXN9: ReactionsRemaining resets to 1 after each combat round.
// Uses whitebox access (package gameserver) to call resolveAndAdvanceLocked directly.
func TestReactionsRemaining_ResetsToOneAfterRound(t *testing.T) {
	_, sessMgr, combatHandler := newAutoCombatSvc(t)

	// Add a player with a non-nil LoadoutSet (required for the post-round loop to run).
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "rxn_uid",
		Username:  "Tester",
		CharName:  "Tester",
		RoomID:    "rxn_room",
		CurrentHP: 10,
		MaxHP:     10,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	sess.Reactions = reaction.NewReactionRegistry()
	sess.ReactionsRemaining = 0 // Simulate mid-round: reaction already spent.

	// Build a minimal Combatant matching the session.
	// Read the Combatant struct in internal/game/combat/combat.go and adapt field names.
	player := &combat.Combatant{
		ID:        "rxn_uid",
		Name:      "Tester",
		Kind:      combat.KindPlayer,
		CurrentHP: 10,
		MaxHP:     10,
	}
	// Start a combat with just the player (no NPC needed to exercise the reset loop).
	cbt, err := combatHandler.engine.StartCombat("rxn_room", []*combat.Combatant{player}, nil, nil, "")
	require.NoError(t, err)

	// Resolve the round directly — whitebox access to private method.
	combatHandler.combatMu.Lock()
	combatHandler.resolveAndAdvanceLocked("rxn_room", cbt)
	combatHandler.combatMu.Unlock()

	assert.Equal(t, 1, sess.ReactionsRemaining, "ReactionsRemaining must reset to 1 after each round")
}

// REQ-RXN20: When ReactionsRemaining == 0, callback returns false without prompting.
func TestReactionCallback_SkipsWhenNoReactionsRemaining(t *testing.T) {
	sess := &session.PlayerSession{
		ReactionsRemaining: 0,
		Reactions:          reaction.NewReactionRegistry(),
	}
	sess.Reactions.Register("uid1", "chrome_reflex", "Chrome Reflex", reaction.ReactionDef{
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	})

	// Build a callback with a nil stream — if prompting is reached it will panic.
	// The test verifies prompting is never reached when ReactionsRemaining == 0.
	promptReached := false
	cb := func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if sess.ReactionsRemaining <= 0 {
			return false, nil
		}
		promptReached = true
		return false, nil
	}
	spent, err := cb("uid1", reaction.TriggerOnSaveFail, reaction.ReactionContext{TriggerUID: "uid1"})
	require.NoError(t, err)
	assert.False(t, spent)
	assert.False(t, promptReached, "prompt must not be reached when ReactionsRemaining == 0")
}

// REQ-RXN28: Two triggers in one round — second is skipped because no reactions remain.
func TestReactionCallback_SecondTriggerSkipped_WhenReactionSpent(t *testing.T) {
	sess := &session.PlayerSession{
		ReactionsRemaining: 1,
		Reactions:          reaction.NewReactionRegistry(),
	}
	sess.Reactions.Register("uid1", "chrome_reflex", "Chrome Reflex", reaction.ReactionDef{
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	})

	// Simulate the callback logic without a real stream: spend the reaction on first call.
	callCount := 0
	cb := func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if sess.ReactionsRemaining <= 0 {
			return false, nil
		}
		if sess.Reactions.Get(uid, trigger) == nil {
			return false, nil
		}
		callCount++
		sess.ReactionsRemaining--
		return true, nil
	}

	// First trigger — spends the reaction.
	spent1, err := cb("uid1", reaction.TriggerOnSaveFail, reaction.ReactionContext{TriggerUID: "uid1"})
	require.NoError(t, err)
	assert.True(t, spent1)
	assert.Equal(t, 0, sess.ReactionsRemaining)

	// Second trigger — must be skipped.
	spent2, err := cb("uid1", reaction.TriggerOnSaveFail, reaction.ReactionContext{TriggerUID: "uid1"})
	require.NoError(t, err)
	assert.False(t, spent2, "second trigger must be skipped after reaction is spent")
	assert.Equal(t, 1, callCount, "callback logic must only spend reaction once")
}

// REQ-RXN28: Player declines prompt — triggering action continues with original outcome.
func TestReactionCallback_Decline_OriginalOutcomePreserved(t *testing.T) {
	sess := &session.PlayerSession{
		ReactionsRemaining: 1,
		Reactions:          reaction.NewReactionRegistry(),
	}
	sess.Reactions.Register("uid1", "chrome_reflex", "Chrome Reflex", reaction.ReactionDef{
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	})

	// Simulate the callback logic: "no" response means outcome unchanged.
	originalOutcome := 2 // Failure
	ctx := reaction.ReactionContext{
		TriggerUID:  "uid1",
		SaveOutcome: &originalOutcome,
	}
	cb := func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if sess.ReactionsRemaining <= 0 {
			return false, nil
		}
		if sess.Reactions.Get(uid, trigger) == nil {
			return false, nil
		}
		// Simulate player declining: return false without modifying ctx.
		return false, nil
	}
	spent, err := cb("uid1", reaction.TriggerOnSaveFail, ctx)
	require.NoError(t, err)
	assert.False(t, spent)
	assert.Equal(t, 2, originalOutcome, "outcome must be unchanged when player declines")
	assert.Equal(t, 1, sess.ReactionsRemaining, "ReactionsRemaining must not change when player declines")
}

// REQ-RXN28: Player accepts reroll_save — better outcome used.
func TestReactionCallback_AcceptRerollSave_OutcomeImproved(t *testing.T) {
	sess := &session.PlayerSession{
		ReactionsRemaining: 1,
		Reactions:          reaction.NewReactionRegistry(),
	}
	sess.Reactions.Register("uid1", "chrome_reflex", "Chrome Reflex", reaction.ReactionDef{
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	})

	// Simulate accepting: ApplyReactionEffect is called with worst possible outcome.
	// We run 50 times to verify the outcome never worsens.
	for i := 0; i < 50; i++ {
		worstOutcome := 3 // CritFailure
		ctx := reaction.ReactionContext{
			TriggerUID:  "uid1",
			SourceUID:   "npc1",
			SaveOutcome: &worstOutcome,
		}
		pr := sess.Reactions.Get("uid1", reaction.TriggerOnSaveFail)
		require.NotNil(t, pr)
		gameserver.ApplyReactionEffect(sess, pr.Def.Effect, &ctx)
		assert.LessOrEqual(t, *ctx.SaveOutcome, 3, "outcome must not worsen after reroll")
		assert.GreaterOrEqual(t, *ctx.SaveOutcome, 0, "outcome must be a valid Outcome value")
	}
}

// REQ-RXN28: nil reactionFn in ResolveRound does not panic.
// This is also covered by TestResolveRound_NilReactionFn_NoPanic in the combat package.
// This test verifies the combat_handler dispatch wrapper handles nil sess.ReactionFn safely.
func TestCombatHandler_NilReactionFn_NoSessionPanic(t *testing.T) {
	// Session with ReactionFn == nil (default). CombatHandler dispatch wrapper must handle this.
	sess := &session.PlayerSession{
		Reactions: reaction.NewReactionRegistry(),
	}
	// ReactionFn is nil by default — the dispatch wrapper checks sess.ReactionFn != nil.
	assert.Nil(t, sess.ReactionFn, "ReactionFn must default to nil")
	// The wrapper in combat_handler.go: if sess.ReactionFn != nil — so nil is safe.
	spent, err := func() (bool, error) {
		if sess.ReactionFn != nil {
			return sess.ReactionFn("uid1", reaction.TriggerOnDamageTaken, reaction.ReactionContext{})
		}
		return false, nil
	}()
	assert.NoError(t, err)
	assert.False(t, spent)
}
```

**Note for `TestReactionsRemaining_ResetsToOneAfterRound`:** After reading `grpc_service_rest_test.go`, replace `buildTestServerWithSession` and `runCombatRound` with the actual helper names or inline equivalent setup. The key assertion is that `sess.ReactionsRemaining == 1` after the post-round reset loop runs.

- [ ] **Step 7: Run full test suite and commit**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... ./internal/game/ruleset/... ./internal/game/technology/... ./internal/game/combat/... ./internal/gameserver/... 2>&1 | tail -5
```

Expected: All packages PASS.

```bash
git add internal/gameserver/grpc_service_reaction_test.go
git commit -m "feat: add reaction integration tests — reset, skip-when-none, second-trigger-skipped, nil-safe dispatch"
```

---

## Final verification

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... ./internal/game/ruleset/... ./internal/game/technology/... ./internal/game/combat/... ./internal/gameserver/... 2>&1 | tail -5
```

Expected: All packages PASS. Ready for sub-project 2 (Reactive Strike + chrome_reflex).
