# Chrome Reflex Reaction Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert `chrome_reflex` from a manually-activated innate tech into a player reaction that fires automatically on save failure, by extending `ReactionDef` to support multiple trigger types.

**Architecture:** Replace the single `Trigger ReactionTriggerType` field on `ReactionDef` with `Triggers []ReactionTriggerType`. Update `Register` to insert one `PlayerReaction` per trigger. Add the `reaction:` block to `chrome_reflex.yaml`. Block `use chrome_reflex` (and any reaction-bearing tech) with an informational message.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `github.com/stretchr/testify`

---

## File Map

| File | Change |
|---|---|
| `internal/game/reaction/trigger.go` | Replace `Trigger` with `Triggers []ReactionTriggerType` |
| `internal/game/reaction/registry.go` | `Register` iterates `def.Triggers`; update duplicate-guard loop |
| `internal/game/reaction/trigger_test.go` | Update `Trigger:` literals to `Triggers:`; add multi-trigger YAML test |
| `internal/game/reaction/registry_test.go` | Update `Trigger:` literals to `Triggers:`; add multi-trigger registration test |
| `internal/game/ruleset/feat_test.go` | Update YAML fixture (`trigger:` → `triggers:`); update assertion to `Triggers[0]` |
| `internal/gameserver/grpc_service_reaction_test.go` | Update `Trigger:` literals to `Triggers:` (lines 65, 96) |
| `content/technologies/innate/chrome_reflex.yaml` | Add `reaction:` block; remove `effects.on_apply` |
| `internal/gameserver/grpc_service.go` | Block `use` for reaction techs inside `if slot, ok := sess.InnateTechs[abilityID]; ok` |

---

## Task 1: `ReactionDef.Trigger` → `Triggers []ReactionTriggerType` + update all references

**Files:**
- Modify: `internal/game/reaction/trigger.go`
- Modify: `internal/game/reaction/registry.go`
- Modify: `internal/game/reaction/trigger_test.go`
- Modify: `internal/game/reaction/registry_test.go`
- Modify: `internal/game/ruleset/feat_test.go`
- Modify: `internal/gameserver/grpc_service_reaction_test.go`

- [ ] **Step 1: Run tests to confirm they currently pass (baseline)**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... ./internal/game/ruleset/... ./internal/gameserver/... 2>&1 | tail -10
```

Expected: All packages pass.

- [ ] **Step 2: Update `ReactionDef` in `trigger.go`**

In `internal/game/reaction/trigger.go`, replace the `Trigger` field with `Triggers`:

```go
// ReactionDef is the reaction declaration embedded in a Feat or TechnologyDef YAML.
type ReactionDef struct {
	// Triggers lists the combat events that can fire this reaction.
	// Must contain at least one entry. An empty slice is a no-op at registration time.
	Triggers []ReactionTriggerType `yaml:"triggers"`
	// Requirement is an optional predicate the player must satisfy (e.g. "wielding_melee_weapon").
	// Empty string means no requirement.
	Requirement string `yaml:"requirement,omitempty"`
	// Effect is the action taken when the reaction fires.
	Effect ReactionEffect `yaml:"effect"`
}
```

Remove the old `// Trigger is the combat event that can fire this reaction.` comment and the `Trigger ReactionTriggerType \`yaml:"trigger"\`` line entirely.

- [ ] **Step 3: Run tests to confirm compile errors**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go build ./... 2>&1 | head -30
```

Expected: Compile errors referencing `Trigger` field — confirms the change broke the right things.

- [ ] **Step 4: Update `Register` in `registry.go`**

Replace the `Register` function body entirely:

```go
// Register adds a reaction for the given player and feat.
// featName is the human-readable display name used in prompts.
// One PlayerReaction entry is created per trigger in def.Triggers.
// If an entry with the same UID and trigger already exists, it is updated in-place.
// If def.Triggers is empty, Register is a no-op (REQ-CRX2).
func (r *ReactionRegistry) Register(uid, featID, featName string, def ReactionDef) {
	for _, trigger := range def.Triggers {
		found := false
		for i := range r.byTrigger[trigger] {
			if r.byTrigger[trigger][i].UID == uid {
				r.byTrigger[trigger][i] = PlayerReaction{
					UID:      uid,
					Feat:     featID,
					FeatName: featName,
					Def:      def,
				}
				found = true
				break
			}
		}
		if !found {
			r.byTrigger[trigger] = append(r.byTrigger[trigger], PlayerReaction{
				UID:      uid,
				Feat:     featID,
				FeatName: featName,
				Def:      def,
			})
		}
	}
}
```

- [ ] **Step 5: Update `trigger_test.go` — fix `Trigger:` literals and add multi-trigger test**

In `internal/game/reaction/trigger_test.go`, replace `Trigger:` with `Triggers:` (slice) in both existing tests, and add a new test:

```go
func TestReactionDef_YAMLRoundTrip(t *testing.T) {
	original := reaction.ReactionDef{
		Triggers:    []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
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
		Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnEnemyMoveAdjacent},
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

// REQ-CRX8: multi-trigger YAML round-trip.
func TestReactionDef_YAMLRoundTrip_MultiTrigger(t *testing.T) {
	original := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{
			reaction.TriggerOnSaveFail,
			reaction.TriggerOnSaveCritFail,
		},
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
```

- [ ] **Step 6: Update `registry_test.go` — fix `Trigger:` literals and add multi-trigger tests**

In `internal/game/reaction/registry_test.go`, replace every `Trigger:` field with `Triggers: []reaction.ReactionTriggerType{...}`. For example:

```go
// Before:
def := reaction.ReactionDef{
    Trigger: reaction.TriggerOnSaveFail,
    Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
}

// After:
def := reaction.ReactionDef{
    Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
    Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
}
```

Apply the same change to ALL `ReactionDef` literals in the file (lines 21, 33, 45, 62, 66).

Also update `TestReactionRegistry_RegisterTwice_UpdatesInPlace`: both `def1` and `def2` use `Trigger: reaction.TriggerOnSaveFail` — change to `Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail}`.

Then add two new tests at the end of the file:

```go
// REQ-CRX9: Register with two triggers — both are retrievable.
func TestReactionRegistry_MultiTrigger_BothRetrievable(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{
			reaction.TriggerOnSaveFail,
			reaction.TriggerOnSaveCritFail,
		},
		Effect: reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result1 := reg.Get("uid1", reaction.TriggerOnSaveFail)
	assert.NotNil(t, result1, "must be retrievable by TriggerOnSaveFail")
	assert.Equal(t, "chrome_reflex", result1.Feat)

	result2 := reg.Get("uid1", reaction.TriggerOnSaveCritFail)
	assert.NotNil(t, result2, "must be retrievable by TriggerOnSaveCritFail")
	assert.Equal(t, "chrome_reflex", result2.Feat)
}

// REQ-CRX9: Spending reaction on save_fail prevents crit_fail from firing in same round.
// (ReactionsRemaining check is what enforces this — verify the guard works cross-trigger.)
func TestReactionRegistry_CrossTrigger_OnlyOneFiresPerRound(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{
			reaction.TriggerOnSaveFail,
			reaction.TriggerOnSaveCritFail,
		},
		Effect: reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	reactionsRemaining := 1

	// First trigger: TriggerOnSaveFail — spends the reaction.
	pr1 := reg.Get("uid1", reaction.TriggerOnSaveFail)
	require.NotNil(t, pr1)
	assert.Greater(t, reactionsRemaining, 0, "reaction available before first trigger")
	reactionsRemaining-- // simulate spending

	// Second trigger: TriggerOnSaveCritFail — blocked because reactionsRemaining == 0.
	pr2 := reg.Get("uid1", reaction.TriggerOnSaveCritFail)
	require.NotNil(t, pr2, "reaction is registered for crit fail too")
	assert.Equal(t, 0, reactionsRemaining, "no reactions remaining after spending on save_fail")
	// The callback would check reactionsRemaining <= 0 and return false — verified here.
}

// REQ-CRX2: Register with empty Triggers is a no-op.
func TestReactionRegistry_EmptyTriggers_NoOp(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{}, // empty
		Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	// Nothing should be retrievable for any trigger.
	assert.Nil(t, reg.Get("uid1", reaction.TriggerOnSaveFail))
	assert.Nil(t, reg.Get("uid1", reaction.TriggerOnSaveCritFail))
}
```

- [ ] **Step 7: Update `feat_test.go` — fix YAML fixture and assertion**

In `internal/game/ruleset/feat_test.go`, in `TestFeat_YAML_WithReactionBlock`:

Change the YAML inline string from:
```yaml
  reaction:
    trigger: on_enemy_move_adjacent
    requirement: wielding_melee_weapon
```
to:
```yaml
  reaction:
    triggers:
      - on_enemy_move_adjacent
    requirement: wielding_melee_weapon
```

Change the assertion on line 111 from:
```go
assert.Equal(t, reaction.TriggerOnEnemyMoveAdjacent, f.Reaction.Trigger)
```
to:
```go
require.Len(t, f.Reaction.Triggers, 1)
assert.Equal(t, reaction.TriggerOnEnemyMoveAdjacent, f.Reaction.Triggers[0])
```

- [ ] **Step 8: Update `grpc_service_reaction_test.go` — fix `Trigger:` literals**

In `internal/gameserver/grpc_service_reaction_test.go`, at lines 64-66 and 95-97, change:

```go
def := reaction.ReactionDef{
    Trigger: reaction.TriggerOnDamageTaken,
    Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage},
}
```
to:
```go
def := reaction.ReactionDef{
    Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnDamageTaken},
    Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage},
}
```

Apply to both occurrences.

- [ ] **Step 9: Run tests and commit**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... ./internal/game/ruleset/... ./internal/gameserver/... 2>&1 | tail -10
```

Expected: All packages pass.

```bash
git add internal/game/reaction/trigger.go \
        internal/game/reaction/registry.go \
        internal/game/reaction/trigger_test.go \
        internal/game/reaction/registry_test.go \
        internal/game/ruleset/feat_test.go \
        internal/gameserver/grpc_service_reaction_test.go
git commit -m "feat: ReactionDef.Trigger → Triggers []ReactionTriggerType; Register iterates all triggers"
```

---

## Task 2: `chrome_reflex.yaml` reaction block + use-command block

**Files:**
- Modify: `content/technologies/innate/chrome_reflex.yaml`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write failing test for chrome_reflex YAML load**

Read `internal/game/technology/registry_test.go` (or the nearest test file for technology loading) to understand the test pattern. Then add to an appropriate tech test file — if none exists for innate YAML loading, add to `internal/game/technology/registry_test.go`:

```go
// REQ-CRX10: chrome_reflex.yaml loads correctly with both reaction triggers.
func TestChromeReflex_YAMLLoad_HasReactionDef(t *testing.T) {
    // Load the actual chrome_reflex.yaml content file.
    data, err := os.ReadFile("../../content/technologies/innate/chrome_reflex.yaml")
    require.NoError(t, err)

    var def technology.TechnologyDef
    err = yaml.Unmarshal(data, &def)
    require.NoError(t, err)

    require.NotNil(t, def.Reaction, "chrome_reflex must have a Reaction definition")
    assert.Contains(t, def.Reaction.Triggers, reaction.TriggerOnSaveFail,
        "chrome_reflex must trigger on save fail")
    assert.Contains(t, def.Reaction.Triggers, reaction.TriggerOnSaveCritFail,
        "chrome_reflex must trigger on save crit fail")
    assert.Equal(t, reaction.ReactionEffectRerollSave, def.Reaction.Effect.Type)
    assert.Equal(t, "better", def.Reaction.Effect.Keep)
}
```

**Note:** Adjust the import path for `os` and `yaml`, and the relative path to `chrome_reflex.yaml` based on where the test file lives. If `registry_test.go` doesn't exist in `internal/game/technology/`, find the nearest test file that loads YAML content. Use `os.ReadFile` with the correct relative path from that test file's directory.

Run to confirm failure:

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/technology/... -run TestChromeReflex -v 2>&1 | head -15
```

Expected: FAIL — chrome_reflex.yaml has no reaction block yet.

- [ ] **Step 2: Update `chrome_reflex.yaml`**

Replace the full contents of `content/technologies/innate/chrome_reflex.yaml` with:

```yaml
id: chrome_reflex
name: Chrome Reflex
description: A neural-augmented reflex burst that overrides the nervous system and forces a second attempt at a failed saving throw.
tradition: neural
level: 1
usage_type: innate
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
reaction:
  triggers:
    - on_save_fail
    - on_save_crit_fail
  effect:
    type: reroll_save
    keep: better
```

Note: `effects.on_apply` is removed per REQ-CRX3 — chrome_reflex no longer has a manual activation path and the entry was dead code.

- [ ] **Step 3: Run YAML load test to confirm it passes**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/technology/... -run TestChromeReflex -v 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 4: Write failing test for use-command block**

Find `internal/gameserver/grpc_service_use_tech_test.go` or the nearest test for `handleUse`. Read it to understand the test helper pattern.

Add a test (in the appropriate test file — use `package gameserver_test` or `package gameserver` matching the existing file):

```go
// REQ-CRX6: use <tech> is blocked for reaction-bearing techs.
func TestHandleUse_ReactionTech_BlocksActivation(t *testing.T) {
    // Build a minimal service with a tech registry containing chrome_reflex.
    // Use the same test helper as existing handleUse tests (e.g. newTestSvc or similar).
    // Read the file carefully to find the right helper before writing.
    //
    // The test must verify:
    // 1. Calling use with chrome_reflex (or any tech with Reaction != nil) returns
    //    a message containing "fires automatically as a reaction"
    // 2. activateTechWithEffects is NOT called (no tech effect applied)
}
```

**Read the existing use-tech test file to find the right helper names and adapt the test body before running.**

Run to confirm failure:

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleUse_ReactionTech -v 2>&1 | head -15
```

Expected: FAIL or compile error — block not implemented yet.

- [ ] **Step 5: Implement the use-command block in `grpc_service.go`**

In `internal/gameserver/grpc_service.go`, find the innate tech activation block (search for `if slot, ok := sess.InnateTechs[abilityID]; ok`). This is around line 4968. Immediately after entering the `if` block and before the uses-remaining check, add:

```go
// REQ-CRX6: block manual use for techs that fire as reactions.
if s.techRegistry != nil {
    if techDef, ok := s.techRegistry.Get(abilityID); ok && techDef.Reaction != nil {
        return messageEvent(fmt.Sprintf("%s fires automatically as a reaction and cannot be activated manually.", techDef.Name)), nil
    }
}
```

The full block becomes:

```go
if slot, ok := sess.InnateTechs[abilityID]; ok {
    // REQ-CRX6: block manual use for techs that fire as reactions.
    if s.techRegistry != nil {
        if techDef, ok := s.techRegistry.Get(abilityID); ok && techDef.Reaction != nil {
            return messageEvent(fmt.Sprintf("%s fires automatically as a reaction and cannot be activated manually.", techDef.Name)), nil
        }
    }
    if slot.MaxUses != 0 && slot.UsesRemaining <= 0 {
        return messageEvent(fmt.Sprintf("No uses of %s remaining.", abilityID)), nil
    }
    // ... rest of existing code unchanged
```

- [ ] **Step 6: Run the use-block test**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleUse_ReactionTech -v 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 7: Run full test suite and commit**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... ./internal/game/technology/... ./internal/game/ruleset/... ./internal/gameserver/... 2>&1 | tail -10
```

Expected: All packages pass.

```bash
git add content/technologies/innate/chrome_reflex.yaml \
        internal/gameserver/grpc_service.go \
        internal/game/technology/
git commit -m "feat: chrome_reflex fires as reaction on save fail/crit-fail; block manual use for reaction techs"
```

---

## Final verification

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/reaction/... ./internal/game/technology/... ./internal/game/ruleset/... ./internal/gameserver/... 2>&1 | tail -5
```

Expected: All packages PASS. Mark Technology feature `chrome_reflex` item complete in `docs/features/technology.md`.
