# Tech Effect Resolution Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement 4-tier PF2E save/attack effect resolution for all three tech activation paths (prepared, spontaneous, innate), restructure the TechEffect model to support tiered outcomes, and populate all 14 existing technology YAML files with real effect definitions.

**Architecture:** Add a `TieredEffects` struct to replace the flat `[]TechEffect` in `TechnologyDef`; wire a new `ResolveTechEffects` function into `handleUse` after each use is expended; use the existing `combat.ResolveSave` for save-based resolution and `src.Intn(20)+1` vs `target.AC` for attack-roll resolution. Target resolution is hybrid: in combat defaults to current room's NPC combatant, explicit `use <tech> <target>` for named targets, self/utility techs skip targeting entirely.

**Tech Stack:** Go 1.22+, `pgregory.net/rapid` for property-based tests, `github.com/stretchr/testify`, `gopkg.in/yaml.v3`, proto3 via `make proto`.

**Spec:** `docs/superpowers/specs/2026-03-18-tech-effect-resolution-design.md`
**PF2E Reference:** `docs/requirements/pf2e-import-reference.md`

---

## Key Existing APIs (read before implementing)

```go
// combat.ResolveSave — already implemented, takes "toughness"/"hustle"/"cool"
func ResolveSave(saveType string, combatant *Combatant, dc int, src Source) Outcome
// Outcome values: CritSuccess, Success, Failure, CritFailure (combat package constants)

// combat.OutcomeFor — 4-tier check
func OutcomeFor(roll, ac int) Outcome  // CritSuccess if roll >= ac+10; Success if >= ac; etc.

// condition.ActiveSet.Apply — apply a condition to a combatant
func (s *ActiveSet) Apply(uid string, def *ConditionDef, stacks, duration int) error
// duration -1 = permanent; duration N = N rounds

// dice.RollExpr — package-level, takes a Source
func RollExpr(expr string, src Source) (RollResult, error)
// RollResult.Total() int

// combat.Combat.Conditions — map[string]*condition.ActiveSet keyed by combatant ID
// Access: cbt.Conditions[target.ID].Apply(...)

// s.combatH.ActiveCombatForPlayer(uid) *combat.Combat — nil if not in combat
// s.dice.Src() dice.Source — satisfies combat.Source (Intn method)
// s.condRegistry *condition.Registry — Get(id) (*ConditionDef, bool)

// Combatant fields available: ID, Name, CurrentHP, AC, Position, GritMod, QuicknessMod, SavvyMod, ToughnessRank, HustleRank, CoolRank
```

---

## Chunk 1: Model + Conditions

### Task 1: TieredEffects model

**Files:**
- Modify: `internal/game/technology/model.go`
- Modify: `internal/game/technology/model_test.go`

- [ ] **Step 1: Write failing tests for new model**

Add to `internal/game/technology/model_test.go` (inside the existing `package technology_test`):

```go
// REQ-TER1: resolution:"save" without save_type rejected.
func TestValidate_REQ_TER1_SaveResolutionWithoutSaveType(t *testing.T) {
	d := validDef()
	d.Resolution = "save"
	d.SaveDC = 15
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save_type")
}

// REQ-TER2: resolution:"save" without save_dc rejected.
func TestValidate_REQ_TER2_SaveResolutionWithoutSaveDC(t *testing.T) {
	d := validDef()
	d.Resolution = "save"
	d.SaveType = "cool"
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save_dc")
}

// REQ-TER3: resolution:"none" accepts empty save_type and zero save_dc.
func TestValidate_REQ_TER3_NoneResolutionNoSaveRequired(t *testing.T) {
	d := validDef()
	d.Resolution = "none"
	err := d.Validate()
	require.NoError(t, err)
}

// REQ-TER4: resolution:"attack" with save_type set is rejected.
func TestValidate_REQ_TER4_AttackResolutionWithSaveTypeRejected(t *testing.T) {
	d := validDef()
	d.Resolution = "attack"
	d.SaveType = "cool"
	d.SaveDC = 15
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save_type")
}

// TieredEffects round-trip YAML test.
func TestTieredEffects_YAMLRoundTrip(t *testing.T) {
	input := `
resolution: save
save_type: cool
save_dc: 15
effects:
  on_failure:
    - type: damage
      dice: 2d4
      damage_type: neural
  on_crit_failure:
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
`
	var def TechnologyDef
	require.NoError(t, yaml.Unmarshal([]byte(input), &def))  // Note: use validDef base + overlay
	assert.Equal(t, "save", def.Resolution)
	assert.Equal(t, "cool", def.SaveType)
	assert.Equal(t, 15, def.SaveDC)
	assert.Len(t, def.Effects.OnFailure, 1)
	assert.Equal(t, EffectDamage, def.Effects.OnFailure[0].Type)
	assert.Len(t, def.Effects.OnCritFailure, 1)
	assert.Equal(t, EffectCondition, def.Effects.OnCritFailure[0].Type)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -run "TestValidate_REQ_TER|TestTieredEffects" -v 2>&1 | tail -20
```
Expected: FAIL — `Resolution` field doesn't exist yet.

- [ ] **Step 3: Add `TieredEffects` struct and update `TechnologyDef`**

In `internal/game/technology/model.go`, add after the existing `TechEffect` struct:

```go
// TieredEffects holds per-outcome effect lists for a technology.
// Only the tiers relevant to the tech's Resolution type need to be populated.
//
//	Save-based (resolution:"save"):   OnCritSuccess/OnSuccess/OnFailure/OnCritFailure
//	Attack-based (resolution:"attack"): OnMiss/OnHit/OnCritHit
//	No-roll (resolution:"none" or ""):  OnApply
type TieredEffects struct {
	// Save-based tiers
	OnCritSuccess []TechEffect `yaml:"on_crit_success,omitempty"`
	OnSuccess     []TechEffect `yaml:"on_success,omitempty"`
	OnFailure     []TechEffect `yaml:"on_failure,omitempty"`
	OnCritFailure []TechEffect `yaml:"on_crit_failure,omitempty"`
	// Attack-based tiers
	OnMiss    []TechEffect `yaml:"on_miss,omitempty"`
	OnHit     []TechEffect `yaml:"on_hit,omitempty"`
	OnCritHit []TechEffect `yaml:"on_crit_hit,omitempty"`
	// No-roll
	OnApply []TechEffect `yaml:"on_apply,omitempty"`
}

// AllEffects returns a flat slice of all TechEffect entries across all tiers.
// Used for validation to check all contained effects are structurally valid.
func (te TieredEffects) AllEffects() []TechEffect {
	var all []TechEffect
	all = append(all, te.OnCritSuccess...)
	all = append(all, te.OnSuccess...)
	all = append(all, te.OnFailure...)
	all = append(all, te.OnCritFailure...)
	all = append(all, te.OnMiss...)
	all = append(all, te.OnHit...)
	all = append(all, te.OnCritHit...)
	all = append(all, te.OnApply...)
	return all
}
```

In `TechnologyDef`, replace:
```go
// OLD — remove these two lines:
Effects      []TechEffect `yaml:"effects"`
AmpedEffects []TechEffect `yaml:"amped_effects,omitempty"`
```
with:
```go
Resolution   string        `yaml:"resolution,omitempty"`    // "save" | "attack" | "none"
Effects      TieredEffects `yaml:"effects,omitempty"`
AmpedEffects TieredEffects `yaml:"amped_effects,omitempty"`
```

- [ ] **Step 4: Update `Validate()` for new model**

Replace the effects validation block in `Validate()`. Remove the old block:
```go
// OLD — remove this:
if len(t.Effects) == 0 {
    return fmt.Errorf("effects must have at least one entry")
}
for i, e := range t.Effects {
    if err := validateEffect(e, i); err != nil {
        return err
    }
}
if len(t.AmpedEffects) > 0 && t.AmpedLevel == 0 {
    return fmt.Errorf("amped_level must be > 0 when amped_effects is non-empty")
}
if t.AmpedLevel > 0 && len(t.AmpedEffects) == 0 {
    return fmt.Errorf("amped_effects must be non-empty when amped_level > 0")
}
for i, e := range t.AmpedEffects {
    if !validEffectTypes[e.Type] {
        return fmt.Errorf("amped_effects[%d]: unknown type %q", i, e.Type)
    }
    if e.Type == EffectSkillCheck {
        if e.Skill == "" {
            return fmt.Errorf("amped_effects[%d]: skill_check effect requires skill", i)
        }
        if e.DC == 0 {
            return fmt.Errorf("amped_effects[%d]: skill_check effect requires dc > 0", i)
        }
    }
}
if t.SaveType != "" && t.SaveDC == 0 {
    return fmt.Errorf("save_dc must be > 0 when save_type is set")
}
```

Replace with:
```go
// Validate Resolution/SaveType/SaveDC consistency.
switch t.Resolution {
case "", "none":
    if t.SaveType != "" {
        return fmt.Errorf("save_type must be empty when resolution is %q", t.Resolution)
    }
    if t.SaveDC != 0 {
        return fmt.Errorf("save_dc must be 0 when resolution is %q", t.Resolution)
    }
case "save":
    if t.SaveType == "" {
        return fmt.Errorf("save_type must be set when resolution is \"save\"")
    }
    if t.SaveDC == 0 {
        return fmt.Errorf("save_dc must be > 0 when resolution is \"save\"")
    }
case "attack":
    if t.SaveType != "" {
        return fmt.Errorf("save_type must be empty when resolution is \"attack\"")
    }
default:
    return fmt.Errorf("unknown resolution %q", t.Resolution)
}
// Validate all effects in all tiers.
for i, e := range t.Effects.AllEffects() {
    if err := validateEffect(e, i); err != nil {
        return err
    }
}
if t.AmpedLevel > 0 && len(t.AmpedEffects.AllEffects()) == 0 {
    return fmt.Errorf("amped_effects must have at least one effect when amped_level > 0")
}
for i, e := range t.AmpedEffects.AllEffects() {
    if err := validateEffect(e, i); err != nil {
        return fmt.Errorf("amped_effects[%d]: %w", i, err)
    }
}
```

**Also update `validDef()` in model_test.go** — the helper uses `Effects: []TechEffect{...}` which no longer compiles. Change it to:

```go
func validDef() *TechnologyDef {
	return &TechnologyDef{
		ID:        "test-tech",
		Name:      "Test Technology",
		Tradition: TraditionTechnical,
		Level:     1,
		UsageType: UsageHardwired,
		Range:     RangeSelf,
		Targets:   TargetsSingle,
		Duration:  "instant",
		Resolution: "none",
		Effects: TieredEffects{
			OnApply: []TechEffect{minimalEffect(EffectUtility)},
		},
	}
}
```

Also update the `minimalEffect` helper — it's used unchanged, no edits needed there.

Also, scan the rest of `model_test.go` for any test that directly sets `Effects: []TechEffect{...}` and update each to `Effects: TieredEffects{OnApply: []TechEffect{...}}`. Run a quick grep first:

```bash
grep -n "Effects:" /home/cjohannsen/src/mud/internal/game/technology/model_test.go
```

Update each occurrence similarly.

- [ ] **Step 5: Run model tests (no YAML loading)**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -run "TestValidate" -v 2>&1 | tail -30
```
Expected: All `TestValidate_*` tests pass. Registry/YAML loading tests may fail — that's expected until Task 6 updates the YAML files. Do NOT run `go test ./...` yet.

- [ ] **Step 6: Commit**

```bash
git add internal/game/technology/model.go internal/game/technology/model_test.go
git commit -m "feat(technology): TieredEffects model; Resolution/SaveType/SaveDC validation"
```

---

### Task 2: New condition YAML files

**Files:**
- Create: `content/conditions/slowed.yaml`
- Create: `content/conditions/immobilized.yaml`
- Create: `content/conditions/blinded.yaml`
- Create: `content/conditions/fleeing.yaml`

- [ ] **Step 1: Write failing test**

Add to `internal/game/condition/definition_test.go` (create if absent, package `condition_test`):

```go
package condition_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// REQ-COND1: New conditions load without error.
func TestNewConditions_LoadFromDirectory(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)

	for _, id := range []string{"slowed", "immobilized", "blinded", "fleeing"} {
		def, ok := reg.Get(id)
		require.True(t, ok, "condition %q not found", id)
		assert.NotEmpty(t, def.Name, "condition %q has empty name", id)
		assert.NotEmpty(t, def.Description, "condition %q has empty description", id)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/condition/... -run "TestNewConditions" -v 2>&1 | tail -10
```
Expected: FAIL — condition files don't exist yet.

- [ ] **Step 3: Create condition YAML files**

`content/conditions/slowed.yaml`:
```yaml
id: slowed
name: Slowed
description: You lose actions each round due to impaired reactions. Slowed N means you lose N AP at the start of your turn.
duration_type: rounds
max_stacks: 2
ap_reduction: 1
```
*(Note: `ap_reduction` is the field name per `condition.ConditionDef` — it maps `APReduction int yaml:"ap_reduction"`. Each stack multiplies this: Slowed 2 = 2×1 = 2 AP lost. The combat engine reads `stacks * APReduction` at round start.)*

`content/conditions/immobilized.yaml`:
```yaml
id: immobilized
name: Immobilized
description: You cannot move. Your speed is reduced to zero and Stride actions automatically fail.
duration_type: rounds
max_stacks: 1
speed_penalty: 999
restrict_actions:
  - stride
```

`content/conditions/blinded.yaml`:
```yaml
id: blinded
name: Blinded
description: You cannot see. You take a -4 penalty to all attack rolls and all targets are treated as hidden.
duration_type: rounds
max_stacks: 1
attack_penalty: 4
```

`content/conditions/fleeing.yaml`:
```yaml
id: fleeing
name: Fleeing
description: You must use all your actions to move away from the source of your fear. You cannot willingly approach the source.
duration_type: rounds
max_stacks: 1
forced_action: flee
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/condition/... -run "TestNewConditions" -v 2>&1 | tail -10
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add content/conditions/slowed.yaml content/conditions/immobilized.yaml \
        content/conditions/blinded.yaml content/conditions/fleeing.yaml \
        internal/game/condition/definition_test.go
git commit -m "feat(conditions): add slowed, immobilized, blinded, fleeing"
```

---

## Chunk 2: Proto + Resolver

### Task 3: Proto UseRequest target field + bridge handler

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Write failing test for target parsing**

Add to `internal/frontend/handlers/bridge_handlers_test.go` (find the existing test file for bridge handlers — grep for `TestBridgeUse` or similar):

```bash
grep -rn "bridgeUse\|TestBridge.*Use\|BridgeUse" /home/cjohannsen/src/mud/internal/frontend/handlers/ | head -5
```

Add in the handlers test file:
```go
// TestBridgeUse_WithTarget passes target arg in UseRequest.
func TestBridgeUse_WithTarget(t *testing.T) {
	bctx := makeBridgeContext("req1", "mind_spike goblin")
	result, err := bridgeUse(bctx)
	require.NoError(t, err)
	msg := result.msg.GetUseRequest()
	require.NotNil(t, msg)
	assert.Equal(t, "mind_spike", msg.GetFeatId())
	assert.Equal(t, "goblin", msg.GetTarget())
}

// TestBridgeUse_NoTarget leaves target empty.
func TestBridgeUse_NoTarget(t *testing.T) {
	bctx := makeBridgeContext("req1", "mind_spike")
	result, err := bridgeUse(bctx)
	require.NoError(t, err)
	msg := result.msg.GetUseRequest()
	require.NotNil(t, msg)
	assert.Equal(t, "mind_spike", msg.GetFeatId())
	assert.Equal(t, "", msg.GetTarget())
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... -run "TestBridgeUse_With\|TestBridgeUse_No" -v 2>&1 | tail -10
```
Expected: FAIL — `GetTarget()` doesn't exist yet.

- [ ] **Step 3: Update proto**

In `api/proto/game/v1/game.proto`, find `UseRequest` and add `target`:
```protobuf
// UseRequest asks the server to activate an active feat or technology.
// An empty feat_id causes the server to return the list of available active abilities.
// target is optional; empty string means "use combat default or self".
message UseRequest {
  string feat_id = 1;
  string target  = 2;
}
```

- [ ] **Step 4: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
make proto 2>&1 | tail -5
```
Expected: exits 0, `internal/gameserver/gamev1/game.pb.go` updated.

- [ ] **Step 5: Update `bridgeUse`**

In `internal/frontend/handlers/bridge_handlers.go`, replace the existing `bridgeUse`:

```go
// bridgeUse builds a UseRequest for tech/feat activation.
// args format: "<abilityID> [target]"
// If no ability name given, sends empty feat_id to trigger listing.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a UseRequest with parsed feat_id and optional target.
func bridgeUse(bctx *bridgeContext) (bridgeResult, error) {
	parts := strings.Fields(bctx.parsed.RawArgs)
	var featID, target string
	if len(parts) >= 1 {
		featID = parts[0]
	}
	if len(parts) >= 2 {
		target = parts[1]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_UseRequest{
			UseRequest: &gamev1.UseRequest{FeatId: featID, Target: target},
		},
	}}, nil
}
```

- [ ] **Step 6: Run bridge handler tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... -v 2>&1 | tail -20
```
Expected: all pass including new tests.

- [ ] **Step 7: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/frontend/handlers/bridge_handlers_test.go
git commit -m "feat(proto): add target field to UseRequest; update bridgeUse to parse target arg"
```

---

### Task 4: ResolveTechEffects function

**Files:**
- Create: `internal/gameserver/tech_effect_resolver.go`
- Create: `internal/gameserver/tech_effect_resolver_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/tech_effect_resolver_test.go` (package `gameserver`):

```go
package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// deterministicSrc always returns the fixed value for Intn.
type deterministicSrc struct{ val int }

func (d *deterministicSrc) Intn(n int) int {
	if d.val >= n {
		return n - 1
	}
	return d.val
}

// makeSaveTech builds a minimal save-based TechnologyDef for tests.
func makeSaveTech(saveType string, onFailure, onCritFailure []technology.TechEffect) *technology.TechnologyDef {
	return &technology.TechnologyDef{
		ID:         "test-save-tech",
		Name:       "Test Save Tech",
		Resolution: "save",
		SaveType:   saveType,
		SaveDC:     15,
		Effects: technology.TieredEffects{
			OnFailure:     onFailure,
			OnCritFailure: onCritFailure,
		},
	}
}

// makeAttackTech builds a minimal attack-based TechnologyDef for tests.
func makeAttackTech(onHit, onCritHit []technology.TechEffect) *technology.TechnologyDef {
	return &technology.TechnologyDef{
		ID:         "test-attack-tech",
		Name:       "Test Attack Tech",
		Tradition:  technology.TraditionNeural,
		Resolution: "attack",
		Effects: technology.TieredEffects{
			OnHit:     onHit,
			OnCritHit: onCritHit,
		},
	}
}

// makeTarget builds a minimal Combatant for tests.
func makeTarget(name string, currentHP, maxHP, ac int) *combat.Combatant {
	return &combat.Combatant{
		ID:        name,
		Name:      name,
		CurrentHP: currentHP,
		MaxHP:     maxHP,
		AC:        ac,
		Level:     1,
		// Save mods zero → easy to predict outcomes
		ToughnessRank: "untrained",
		HustleRank:    "untrained",
		CoolRank:      "untrained",
	}
}

// REQ-TER5: OnFailure effects applied when save returns Failure.
// src.Intn(20) returns 0 → roll=1; total=1+0+0=1 vs DC=15 → Failure
func TestResolveTechEffects_REQ_TER5_SaveFailureAppliesOnFailure(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1", CurrentHP: 20, MaxHP: 20}
	target := makeTarget("npc1", 30, 30, 12)
	tech := makeSaveTech("cool", []technology.TechEffect{
		{Type: technology.EffectDamage, Dice: "1d6", DamageType: "neural"},
	}, nil)
	src := &deterministicSrc{val: 0} // roll=1, fails DC=15

	msgs := ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	require.NotEmpty(t, msgs)
	assert.Less(t, target.CurrentHP, 30, "expected damage applied on failure")
}

// REQ-TER6: OnCritSuccess tier (empty) — no effects applied on crit success.
// src.Intn(20) returns 19 → roll=20; total=20+0+0=20 vs DC=15 → roll+10=20 >= 15+10=25? No.
// Actually need 25+ for crit success with DC=15. Use val=19 → roll=20; 20 >= 15 → Success (not crit).
// For CritSuccess we need total >= DC+10=25. With 0 mods: roll must be 25, impossible on d20.
// Use DC=5: roll=20 >= 5+10=15 → CritSuccess.
func TestResolveTechEffects_REQ_TER6_CritSuccessNoEffects(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1", CurrentHP: 20, MaxHP: 20}
	target := makeTarget("npc1", 30, 30, 12)
	tech := &technology.TechnologyDef{
		ID:         "test",
		Resolution: "save",
		SaveType:   "cool",
		SaveDC:     5,
		Effects: technology.TieredEffects{
			OnCritSuccess: nil, // intentionally empty
			OnFailure: []technology.TechEffect{
				{Type: technology.EffectDamage, Dice: "1d6", DamageType: "neural"},
			},
		},
	}
	src := &deterministicSrc{val: 19} // roll=20 vs DC=5 → total=20 >= 5+10=15 → CritSuccess

	before := target.CurrentHP
	msgs := ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.Equal(t, before, target.CurrentHP, "no damage on crit success")
	require.NotEmpty(t, msgs)
}

// REQ-TER7: Damage effect — target.CurrentHP decreases; never below 0.
func TestResolveTechEffects_REQ_TER7_DamageReducesHP(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	target := makeTarget("npc1", 5, 30, 1) // AC=1 → easy hit
	tech := makeAttackTech(
		[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "acid"}},
		nil,
	)
	src := &deterministicSrc{val: 10} // roll=11 vs AC=1 → hit; dice val=10 → d6 cap at 5→ rolls 5

	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.GreaterOrEqual(t, target.CurrentHP, 0, "HP never below 0")
	assert.Less(t, target.CurrentHP, 5, "HP should be reduced")
}

// REQ-TER8: Heal effect — sess.CurrentHP increases; never above MaxHP.
func TestResolveTechEffects_REQ_TER8_HealIncreasesHP(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1", CurrentHP: 10, MaxHP: 20}
	tech := &technology.TechnologyDef{
		ID:         "nanite",
		Resolution: "none",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{
				{Type: technology.EffectHeal, Dice: "1d8", Amount: 8},
			},
		},
	}
	src := &deterministicSrc{val: 7} // d8 → max 8

	ResolveTechEffects(sess, tech, nil, nil, nil, src)

	assert.LessOrEqual(t, sess.CurrentHP, sess.MaxHP, "HP never above MaxHP")
	assert.Greater(t, sess.CurrentHP, 10, "HP should have increased")
}

// REQ-TER10: Movement effect — target.Position increases when direction is "away".
func TestResolveTechEffects_REQ_TER10_MovementPushesTarget(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	target := makeTarget("npc1", 30, 30, 1) // easy hit
	target.Position = 25
	tech := makeAttackTech(
		[]technology.TechEffect{
			{Type: technology.EffectMovement, Distance: 5, Direction: "away"},
		},
		nil,
	)
	src := &deterministicSrc{val: 10} // hit

	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.Equal(t, 30, target.Position, "target pushed 5 ft away from 25 → 30")
}

// REQ-TER10b: Position floored at 0 when pushed below zero.
func TestResolveTechEffects_REQ_TER10b_PositionFloored(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	target := makeTarget("npc1", 30, 30, 1)
	target.Position = 2
	tech := makeAttackTech(
		[]technology.TechEffect{
			{Type: technology.EffectMovement, Distance: 10, Direction: "away"},
		},
		nil,
	)
	// player is at 0; target is at 2; "away" from player means increasing position
	src := &deterministicSrc{val: 10}

	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.GreaterOrEqual(t, target.Position, 0, "Position must not be negative")
}

// REQ-TER11: Attack tech — OnHit effects applied on hit; no effects on miss.
func TestResolveTechEffects_REQ_TER11_AttackMissNoEffects(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1", Level: 1}
	target := makeTarget("npc1", 30, 30, 25) // high AC
	tech := makeAttackTech(
		[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "acid"}},
		nil,
	)
	src := &deterministicSrc{val: 0} // roll=1 vs AC=25 → miss

	before := target.CurrentHP
	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.Equal(t, before, target.CurrentHP, "no damage on miss")
}

// REQ-TER12 (property): For save-based tech, CritSuccess tier never applies on Failure/CritFailure.
func TestProperty_REQ_TER12_CritSuccessTierNotAppliedOnFailure(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := &session.PlayerSession{UID: "p1"}
		hp := rapid.IntRange(10, 50).Draw(rt, "hp")
		target := makeTarget("npc1", hp, hp, 12)
		// Always-fail src: roll=0+1=1 vs DC=30 → always Failure
		src := &deterministicSrc{val: 0}
		tech := &technology.TechnologyDef{
			ID:         "test",
			Resolution: "save",
			SaveType:   "cool",
			SaveDC:     30,
			Effects: technology.TieredEffects{
				OnCritSuccess: []technology.TechEffect{
					{Type: technology.EffectDamage, Dice: "10d10", DamageType: "neural"},
				},
				OnFailure: []technology.TechEffect{
					{Type: technology.EffectDamage, Dice: "1d4", DamageType: "neural"},
				},
			},
		}
		before := target.CurrentHP
		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)
		// Should apply 1d4 (OnFailure), not 10d10 (OnCritSuccess)
		dmg := before - target.CurrentHP
		assert.LessOrEqual(rt, dmg, 4, "only OnFailure 1d4 should apply, not OnCritSuccess 10d10")
	})
}

// REQ-TER13 (property): Damage output always within dice bounds.
func TestProperty_REQ_TER13_DamageWithinDiceBounds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dieSize := rapid.IntRange(1, 12).Draw(rt, "die_size")
		numDice := rapid.IntRange(1, 4).Draw(rt, "num_dice")
		flat := rapid.IntRange(0, 10).Draw(rt, "flat")
		expr := fmt.Sprintf("%dd%d", numDice, dieSize)

		sess := &session.PlayerSession{UID: "p1"}
		target := makeTarget("npc1", 1000, 1000, 1) // easy hit, high HP
		tech := makeAttackTech(
			[]technology.TechEffect{{Type: technology.EffectDamage, Dice: expr, Amount: flat, DamageType: "acid"}},
			nil,
		)
		src := &deterministicSrc{val: 10} // always hit
		before := target.CurrentHP

		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

		dmg := before - target.CurrentHP
		minDmg := numDice + flat       // min: all 1s
		maxDmg := numDice*dieSize + flat // max: all max
		assert.GreaterOrEqual(rt, dmg, minDmg, "damage at least minimum")
		assert.LessOrEqual(rt, dmg, maxDmg, "damage at most maximum")
	})
}

// REQ-TER14 (property): target.CurrentHP never goes negative.
func TestProperty_REQ_TER14_HPNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		initialHP := rapid.IntRange(1, 20).Draw(rt, "hp")
		target := makeTarget("npc1", initialHP, initialHP, 1)
		sess := &session.PlayerSession{UID: "p1"}
		tech := makeAttackTech(
			[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "4d6", DamageType: "neural"}},
			nil,
		)
		src := &deterministicSrc{val: 15}

		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)
		assert.GreaterOrEqual(rt, target.CurrentHP, 0, "HP must not go negative")
	})
}

// REQ-TER21: Area-targeting tech applies effects to every target in the slice.
func TestResolveTechEffects_REQ_TER21_AreaTargetingAppliesToAll(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	targets := []*combat.Combatant{
		makeTarget("npc1", 30, 30, 1),
		makeTarget("npc2", 30, 30, 1),
		makeTarget("npc3", 30, 30, 1),
	}
	tech := &technology.TechnologyDef{
		ID:         "terror_broadcast",
		Resolution: "save",
		SaveType:   "cool",
		SaveDC:     30, // always fail with 0 mods
		Effects: technology.TieredEffects{
			OnFailure: []technology.TechEffect{
				{Type: technology.EffectDamage, Dice: "1d4", DamageType: "neural"},
			},
		},
	}
	src := &deterministicSrc{val: 0} // all fail

	msgs := ResolveTechEffects(sess, tech, targets, nil, nil, src)

	for _, tgt := range targets {
		assert.Less(t, tgt.CurrentHP, 30, "all targets should take damage")
	}
	assert.GreaterOrEqual(t, len(msgs), 3, "one message per target")
}

// REQ-TER22 (property): Area-targeting with N enemies produces N messages.
func TestProperty_REQ_TER22_AreaMessagesEqualTargetCount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n_targets")
		targets := make([]*combat.Combatant, n)
		for i := range targets {
			targets[i] = makeTarget(fmt.Sprintf("npc%d", i), 100, 100, 1)
		}
		sess := &session.PlayerSession{UID: "p1"}
		tech := &technology.TechnologyDef{
			ID:         "area_tech",
			Resolution: "save",
			SaveType:   "cool",
			SaveDC:     30,
			Effects: technology.TieredEffects{
				OnFailure: []technology.TechEffect{
					{Type: technology.EffectDamage, Dice: "1d4", DamageType: "neural"},
				},
			},
		}
		src := &deterministicSrc{val: 0}
		msgs := ResolveTechEffects(sess, tech, targets, nil, nil, src)
		assert.Equal(rt, n, len(msgs))
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestResolveTechEffects|TestProperty_REQ_TER" -v 2>&1 | tail -10
```
Expected: FAIL — `ResolveTechEffects` undefined.

- [ ] **Step 3: Implement `ResolveTechEffects`**

Create `internal/gameserver/tech_effect_resolver.go` (package `gameserver`):

```go
package gameserver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// ResolveTechEffects resolves all effects for a tech activation and returns a slice of
// human-readable result messages (one per target). Does not expend uses — the caller
// has already done that.
//
// Preconditions:
//   - sess must be non-nil
//   - tech must be non-nil
//   - targets is empty for self/utility/no-roll; one entry for single; all enemies for area
//   - cbt may be nil when not in combat; condition effects are silently skipped when nil
//   - condRegistry may be nil; condition effects are silently skipped when nil
//   - src must be non-nil (satisfies combat.Source: Intn method)
//
// Postconditions:
//   - Returns at least one message
//   - target.CurrentHP and sess.CurrentHP never go below 0
//   - sess.CurrentHP never exceeds sess.MaxHP
func ResolveTechEffects(
	sess         *session.PlayerSession,
	tech         *technology.TechnologyDef,
	targets      []*combat.Combatant,
	cbt          *combat.Combat,
	condRegistry *condition.Registry,
	src          combat.Source,
) []string {
	if len(targets) == 0 {
		return applyEffects(sess, tech.Effects.OnApply, nil, cbt, condRegistry, src)
	}

	var msgs []string
	for _, target := range targets {
		var tier []technology.TechEffect
		var label string

		switch tech.Resolution {
		case "save":
			outcome := combat.ResolveSave(tech.SaveType, target, tech.SaveDC, src)
			tier, label = selectSaveTier(tech.Effects, outcome, target.Name)
		case "attack":
			outcome := resolveAttackRoll(sess, tech, target, src)
			tier, label = selectAttackTier(tech.Effects, outcome, target.Name)
		default: // "none" or ""
			tier = tech.Effects.OnApply
			label = ""
		}

		effectMsgs := applyEffects(sess, tier, target, cbt, condRegistry, src)
		for _, m := range effectMsgs {
			if label != "" {
				msgs = append(msgs, label+m)
			} else {
				msgs = append(msgs, m)
			}
		}
	}
	if len(msgs) == 0 {
		msgs = append(msgs, "Nothing happens.")
	}
	return msgs
}

// selectSaveTier returns the effect tier and a prefix label for a save outcome.
func selectSaveTier(effects technology.TieredEffects, outcome combat.Outcome, targetName string) ([]technology.TechEffect, string) {
	switch outcome {
	case combat.CritSuccess:
		return effects.OnCritSuccess, fmt.Sprintf("%s critically succeeds: ", targetName)
	case combat.Success:
		return effects.OnSuccess, fmt.Sprintf("%s succeeds: ", targetName)
	case combat.Failure:
		return effects.OnFailure, fmt.Sprintf("%s fails: ", targetName)
	case combat.CritFailure:
		return effects.OnCritFailure, fmt.Sprintf("%s critically fails: ", targetName)
	default:
		return nil, ""
	}
}

// selectAttackTier returns the effect tier and a prefix label for an attack outcome.
func selectAttackTier(effects technology.TieredEffects, outcome combat.Outcome, targetName string) ([]technology.TechEffect, string) {
	switch outcome {
	case combat.CritSuccess:
		return effects.OnCritHit, fmt.Sprintf("Critical hit on %s: ", targetName)
	case combat.Success:
		return effects.OnHit, fmt.Sprintf("Hit %s: ", targetName)
	default:
		return effects.OnMiss, fmt.Sprintf("Missed %s.", targetName)
	}
}

// resolveAttackRoll resolves an attack roll for tech vs target.
// Returns CritSuccess (crit hit), Success (hit), or Failure (miss).
func resolveAttackRoll(sess *session.PlayerSession, tech *technology.TechnologyDef, target *combat.Combatant, src combat.Source) combat.Outcome {
	roll := src.Intn(20) + 1
	total := roll + techAttackMod(sess, tech)
	return combat.OutcomeFor(total, target.AC)
}

// techAttackMod returns the tech attack bonus for the given session and tech tradition.
// Formula: Level/2 + primary ability modifier.
// Tradition → primary ability: neural→Savvy, bio_synthetic→Grit, technical→Quickness.
func techAttackMod(sess *session.PlayerSession, tech *technology.TechnologyDef) int {
	if sess == nil {
		return 0
	}
	levelBonus := sess.Level / 2
	var abilityMod int
	switch tech.Tradition {
	case technology.TraditionNeural:
		abilityMod = abilityModifier(sess.Abilities.Savvy)
	case technology.TraditionBioSynthetic:
		abilityMod = abilityModifier(sess.Abilities.Grit)
	default: // TraditionTechnical, TraditionFanaticDoctrine
		abilityMod = abilityModifier(sess.Abilities.Quickness)
	}
	return levelBonus + abilityMod
}

// abilityModifier returns the PF2E ability modifier for a score.
// Formula: (score - 10) / 2, rounded down.
func abilityModifier(score int) int {
	return (score - 10) / 2
}

// applyEffects applies a slice of TechEffects and returns result messages.
// target may be nil for self/heal/utility effects.
func applyEffects(
	sess         *session.PlayerSession,
	effects      []technology.TechEffect,
	target       *combat.Combatant,
	cbt          *combat.Combat,
	condRegistry *condition.Registry,
	src          combat.Source,
) []string {
	var msgs []string
	for _, e := range effects {
		msg := applyEffect(sess, e, target, cbt, condRegistry, src)
		if msg != "" {
			msgs = append(msgs, msg)
		}
	}
	if len(msgs) == 0 {
		msgs = append(msgs, "No effect.")
	}
	return msgs
}

// applyEffect applies a single TechEffect and returns a description message.
func applyEffect(
	sess         *session.PlayerSession,
	e            technology.TechEffect,
	target       *combat.Combatant,
	cbt          *combat.Combat,
	condRegistry *condition.Registry,
	src          combat.Source,
) string {
	switch e.Type {
	case technology.EffectDamage:
		if target == nil {
			return ""
		}
		dmg := rollAmount(e.Dice, e.Amount, src)
		target.CurrentHP -= dmg
		if target.CurrentHP < 0 {
			target.CurrentHP = 0
		}
		return fmt.Sprintf("%d %s damage.", dmg, e.DamageType)

	case technology.EffectHeal:
		heal := rollAmount(e.Dice, e.Amount, src)
		sess.CurrentHP += heal
		if sess.CurrentHP > sess.MaxHP {
			sess.CurrentHP = sess.MaxHP
		}
		return fmt.Sprintf("Healed %d HP.", heal)

	case technology.EffectCondition:
		if condRegistry == nil || cbt == nil {
			return "" // silently skip — no condition system available
		}
		def, ok := condRegistry.Get(e.ConditionID)
		if !ok {
			return fmt.Sprintf("Unknown condition %q.", e.ConditionID)
		}
		stacks := e.Value
		if stacks == 0 {
			stacks = 1
		}
		dur := parseDuration(e.Duration)
		uid := ""
		var applySet *condition.ActiveSet
		if target != nil {
			uid = target.ID
			applySet = cbt.Conditions[target.ID]
		} else {
			uid = sess.UID
			applySet = cbt.Conditions[sess.UID]
		}
		if applySet == nil {
			return ""
		}
		if err := applySet.Apply(uid, def, stacks, dur); err != nil {
			return fmt.Sprintf("Failed to apply %s.", e.ConditionID)
		}
		return fmt.Sprintf("%s %d applied.", e.ConditionID, stacks)

	case technology.EffectMovement:
		if target == nil {
			return ""
		}
		// "away" = increase distance from player (player is at 0, target > 0).
		// "toward" = decrease distance.
		if e.Direction == "away" {
			target.Position += e.Distance
		} else if e.Direction == "toward" {
			target.Position -= e.Distance
			if target.Position < 0 {
				target.Position = 0
			}
		}
		return fmt.Sprintf("Pushed %d feet %s.", e.Distance, e.Direction)

	case technology.EffectUtility:
		if e.Description != "" {
			return e.Description
		}
		return ""

	default:
		return ""
	}
}

// rollAmount rolls dice expression and adds flat amount.
// Returns the flat amount if dice expression is empty.
func rollAmount(expr string, flat int, src combat.Source) int {
	if expr == "" {
		return flat
	}
	result, err := dice.RollExpr(expr, src)
	if err != nil {
		return flat
	}
	return result.Total() + flat
}

// parseDuration converts a duration string to rounds.
// "rounds:N" → N; "minutes:N" → N*10; "instant" or "" → 1.
func parseDuration(s string) int {
	if s == "" || s == "instant" {
		return 1
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 1
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n <= 0 {
		return 1
	}
	switch parts[0] {
	case "rounds":
		return n
	case "minutes":
		return n * 10
	default:
		return 1
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestResolveTechEffects|TestProperty_REQ_TER" -v 2>&1 | tail -30
```
Expected: All pass.

- [ ] **Step 5: Verify compilation**

```bash
cd /home/cjohannsen/src/mud
go build ./... 2>&1 | head -20
```
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/tech_effect_resolver.go internal/gameserver/tech_effect_resolver_test.go
git commit -m "feat(gameserver): ResolveTechEffects — 4-tier save/attack/none resolution (REQ-TER5–14, 21–22)"
```

---

## Chunk 3: handleUse wiring

### Task 5: handleUse target resolution + effect wiring

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_use_target_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_use_target_test.go` (package `gameserver`):

```go
package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// REQ-TER15: handleUse with targetID="" out of combat, damage tech → "Specify a target".
func TestHandleUse_REQ_TER15_NoCombatDamageTechNoTarget(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	// Wire a minimal tech registry so the tech can be found
	reg := technology.NewRegistry()
	reg.Register(&technology.TechnologyDef{
		ID:         "acid_spit",
		Name:       "Acid Spit",
		Tradition:  technology.TraditionBioSynthetic,
		Level:      1,
		UsageType:  technology.UsageInnate,
		Range:      technology.RangeRanged,
		Targets:    technology.TargetsSingle,
		Duration:   "instant",
		Resolution: "attack",
		Effects: technology.TieredEffects{
			OnHit: []technology.TechEffect{
				{Type: technology.EffectDamage, Dice: "1d6", DamageType: "acid"},
			},
		},
	})
	svc.techRegistry = reg

	uid := "p-ter15"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 1},
	}

	// No combat active → targetID="" → should ask for a target
	evt, err := svc.handleUse(uid, "acid_spit", "")
	require.NoError(t, err)
	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "target", "should prompt for a target when not in combat")
}

// REQ-TER16: handleUse with targetID set but not in combat → "You are not in combat."
func TestHandleUse_REQ_TER16_NotInCombatWithTarget(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	reg := technology.NewRegistry()
	reg.Register(&technology.TechnologyDef{
		ID: "acid_spit", Name: "Acid Spit",
		Tradition: technology.TraditionBioSynthetic, Level: 1,
		UsageType: technology.UsageInnate, Range: technology.RangeRanged,
		Targets: technology.TargetsSingle, Duration: "instant",
		Resolution: "attack",
		Effects: technology.TieredEffects{
			OnHit: []technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "acid"}},
		},
	})
	svc.techRegistry = reg

	uid := "p-ter16"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 1},
	}

	evt, err := svc.handleUse(uid, "acid_spit", "goblin")
	require.NoError(t, err)
	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "not in combat")
}

// REQ-TER17: Self-targeting tech (resolution:"none") resolves without error out of combat.
func TestHandleUse_REQ_TER17_SelfTargetOutOfCombat(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	reg := technology.NewRegistry()
	reg.Register(&technology.TechnologyDef{
		ID: "nanite_infusion", Name: "Nanite Infusion",
		Tradition: technology.TraditionBioSynthetic, Level: 1,
		UsageType: technology.UsageInnate, Range: technology.RangeSelf,
		Targets: technology.TargetsSelf, Duration: "instant",
		Resolution: "none",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{
				{Type: technology.EffectHeal, Dice: "1d8", Amount: 8},
			},
		},
	})
	svc.techRegistry = reg

	uid := "p-ter17"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"nanite_infusion": {MaxUses: 1, UsesRemaining: 1},
	}
	sess.CurrentHP = 5
	sess.MaxHP = 20

	evt, err := svc.handleUse(uid, "nanite_infusion", "")
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Greater(t, sess.CurrentHP, 5, "HP should have increased from heal")
}
```

*(Note: REQ-TER18 — in-combat default target — is best verified by integration; skip unit test for it as it requires combat engine wiring. The behavior is documented in the spec.)*

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleUse_REQ_TER" -v 2>&1 | tail -10
```
Expected: FAIL — `handleUse` still takes 2 args.

- [ ] **Step 3: Update `handleUse` signature and add target resolution**

In `internal/gameserver/grpc_service.go`:

**3a.** Change the function signature from:
```go
func (s *GameServiceServer) handleUse(uid, abilityID string) (*gamev1.ServerEvent, error) {
```
to:
```go
// handleUse activates an active feat, class feature, or technology.
// targetID is optional (empty = use combat default or self).
// Postcondition: Returns a ServerEvent with UseResponse or MessageEvent.
func (s *GameServiceServer) handleUse(uid, abilityID, targetID string) (*gamev1.ServerEvent, error) {
```

**3b.** Find where `handleUse` is called from the dispatch switch. It will look something like:
```go
case *gamev1.ClientMessage_UseRequest:
    evt, err = s.handleUse(uid, msg.UseRequest.GetFeatId())
```
Update to:
```go
case *gamev1.ClientMessage_UseRequest:
    evt, err = s.handleUse(uid, msg.UseRequest.GetFeatId(), msg.UseRequest.GetTarget())
```

**3c.** Find all other internal callers of `handleUse` (grep for `handleUse(`). Update each to pass `""` as the third argument.

```bash
grep -n "handleUse(" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -10
```

**3d.** In `handleUse`, after the guard check (`if s.characterFeatsRepo == nil && ...`) and before the `abilityID == ""` check, add target resolution for the tech paths. Find the section **after all feat/class feature activation** (where prepared/spontaneous/innate tech activation happens — look for `// Prepared tech activation` or `// Activate the named ability`). After the prepared/spontaneous/innate block identifies the tech, add this target resolution block:

```go
// resolveUseTarget returns the target Combatant for the given tech and targetID,
// or returns an error message if targeting is invalid.
// Returns (nil, "") for self/utility/no-roll techs (no target needed).
func (s *GameServiceServer) resolveUseTarget(uid, targetID string, tech *technology.TechnologyDef) (*combat.Combatant, string) {
    // Self-targeting and no-roll techs never need a target.
    if tech.Targets == technology.TargetsSelf || tech.Resolution == "" || tech.Resolution == "none" {
        return nil, ""
    }
    cbt := s.combatH.ActiveCombatForPlayer(uid)
    if cbt == nil {
        // Not in combat.
        if targetID != "" {
            return nil, "You are not in combat."
        }
        return nil, "Specify a target: use <tech> <target>"
    }
    // In combat — find target.
    if targetID == "" {
        // Default: first living NPC combatant in combat.
        for _, c := range cbt.Combatants() {
            if c.Kind == combat.KindNPC && !c.IsDead() {
                return c, ""
            }
        }
        return nil, "No valid target in combat."
    }
    // Named target — case-insensitive prefix match.
    for _, c := range cbt.Combatants() {
        if c.Kind == combat.KindNPC && !c.IsDead() &&
            strings.HasPrefix(strings.ToLower(c.Name), strings.ToLower(targetID)) {
            return c, ""
        }
    }
    return nil, fmt.Sprintf("No combatant named %q in this fight.", targetID)
}
```

Add this as a method on `*GameServiceServer` just below the `handleUse` function.

**Note:** `cbt.Combatants()` — check the actual method name on `*combat.Combat` for iterating combatants:
```bash
grep -n "func.*Combatants\|func.*Combatant\b" /home/cjohannsen/src/mud/internal/game/combat/engine.go | head -5
```
Use the correct method or field. If it's a slice field, use direct iteration.

**3e.** In the tech activation paths (prepared, spontaneous, innate), after the slot is identified but before use is expended:

```go
// After identifying the tech def but before expending use:
if s.techRegistry != nil {
    techDef, ok := s.techRegistry.Get(techID)
    if ok {
        target, errMsg := s.resolveUseTarget(uid, targetID, techDef)
        if errMsg != "" {
            return messageEvent(errMsg), nil
        }
        // Expend use (existing code runs here)
        // ...
        // After expending: resolve effects
        var techTargets []*combat.Combatant
        if techDef.Targets == technology.TargetsAllEnemies {
            cbt := s.combatH.ActiveCombatForPlayer(uid)
            if cbt != nil {
                for _, c := range cbt.Combatants() { // use correct method
                    if c.Kind == combat.KindNPC && !c.IsDead() {
                        techTargets = append(techTargets, c)
                    }
                }
            }
        } else if target != nil {
            techTargets = []*combat.Combatant{target}
        }
        msgs := ResolveTechEffects(sess, techDef, techTargets, s.combatH.ActiveCombatForPlayer(uid), s.condRegistry, s.dice.Src())
        resultText := strings.Join(msgs, "\n")
        return messageEvent(resultText), nil
    }
}
```

**Important:** Insert this block at each of the three tech activation paths (prepared, spontaneous, innate). The exact location depends on the current code structure. Look for the return statement after `"You activate <techID>."` in each path and replace it with the above.

- [ ] **Step 4: Update existing handleUse tests that call `handleUse` with 2 args**

Find all test files that call `svc.handleUse(uid, ...)` with 2 args:

```bash
grep -rn "handleUse(uid," /home/cjohannsen/src/mud/internal/gameserver/ | grep -v "_test.go:" | head -5
grep -rn "\.handleUse(" /home/cjohannsen/src/mud/internal/gameserver/ | grep "_test.go" | head -10
```

Update each call to pass `""` as the third argument. These are in:
- `grpc_service_selecttech_test.go`
- `grpc_service_innate_test.go`
- `grpc_service_innate_property_test.go`
- Any other test files that call `handleUse`

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleUse_REQ_TER|TestHandleUse_Innate|TestHandleUse_InnateActivation|TestHandleUse_InnateExhausted|TestHandleUse_InnateUnlimited|TestHandleUse_InnateList|TestHandleUse_SelectTech" -v 2>&1 | tail -30
```
Expected: all target tests pass; existing tests still pass.

- [ ] **Step 6: Run full gameserver tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -v 2>&1 | grep -E "^(PASS|FAIL|---)" | tail -30
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_use_target_test.go
git commit -m "feat(gameserver): handleUse gains targetID; hybrid target resolution; ResolveTechEffects wired in (REQ-TER15–18)"
```

---

## Chunk 4: Content + Final

### Task 6: Update all 14 technology YAML files

**Files:**
- Modify: `content/technologies/neural/mind_spike.yaml`
- Modify: `content/technologies/neural/neural_static.yaml`
- Modify: `content/technologies/neural/synaptic_surge.yaml`
- Modify: `content/technologies/innate/blackout_pulse.yaml`
- Modify: `content/technologies/innate/arc_lights.yaml`
- Modify: `content/technologies/innate/pressure_burst.yaml`
- Modify: `content/technologies/innate/nanite_infusion.yaml`
- Modify: `content/technologies/innate/atmospheric_surge.yaml`
- Modify: `content/technologies/innate/viscous_spray.yaml`
- Modify: `content/technologies/innate/chrome_reflex.yaml`
- Modify: `content/technologies/innate/seismic_sense.yaml`
- Modify: `content/technologies/innate/moisture_reclaim.yaml`
- Modify: `content/technologies/innate/terror_broadcast.yaml`
- Modify: `content/technologies/innate/acid_spit.yaml`
- Modify: `internal/game/technology/registry_test.go`

- [ ] **Step 1: Write failing registry tests**

Add to `internal/game/technology/registry_test.go`:

```go
// REQ-TER19: All 14 tech YAML files load without error after model change.
func TestRegistry_REQ_TER19_AllTechsLoadAfterModelChange(t *testing.T) {
	dirs := []string{
		"../../../content/technologies/neural",
		"../../../content/technologies/innate",
	}
	for _, dir := range dirs {
		reg, err := technology.Load(dir)
		require.NoError(t, err, "failed to load directory %q", dir)
		assert.Greater(t, reg.Count(), 0, "expected at least one tech in %q", dir)
	}
}

// REQ-TER20: Each tech with resolution:"save" has save_type and save_dc > 0.
func TestRegistry_REQ_TER20_SaveResolutionHasSaveTypeAndDC(t *testing.T) {
	dirs := []string{
		"../../../content/technologies/neural",
		"../../../content/technologies/innate",
	}
	for _, dir := range dirs {
		reg, err := technology.Load(dir)
		require.NoError(t, err)
		for _, tech := range reg.All() {
			if tech.Resolution == "save" {
				assert.NotEmpty(t, tech.SaveType, "tech %q: save resolution requires save_type", tech.ID)
				assert.Greater(t, tech.SaveDC, 0, "tech %q: save resolution requires save_dc > 0", tech.ID)
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -run "TestRegistry_REQ_TER19|TestRegistry_REQ_TER20" -v 2>&1 | tail -20
```
Expected: FAIL — YAML files still use old flat format.

- [ ] **Step 3: Write all 14 YAML files**

Replace the full content of each file as follows. The `id`, `name`, `description`, `tradition`, `level`, `usage_type`, `action_cost`, `range`, `duration` fields from the current YAML are preserved. `save_type`, `save_dc`, and the old flat `effects:`/`amped_effects:` blocks are replaced with the new tiered format.

**`content/technologies/neural/mind_spike.yaml`:**
```yaml
id: mind_spike
name: Mind Spike
description: A focused neural disruption that scrambles a target's cognition.
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
resolution: save
save_type: cool
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: damage
      dice: 1d3
      damage_type: mental
  on_failure:
    - type: damage
      dice: 1d6
      damage_type: mental
  on_crit_failure:
    - type: damage
      dice: 1d6
      damage_type: mental
    - type: condition
      condition_id: stunned
      value: 1
      duration: rounds:1
amped_level: 3
amped_effects:
  on_crit_success: []
  on_success:
    - type: damage
      dice: 1d6
      damage_type: mental
  on_failure:
    - type: damage
      dice: 2d6
      damage_type: mental
  on_crit_failure:
    - type: damage
      dice: 2d6
      damage_type: mental
    - type: condition
      condition_id: stunned
      value: 1
      duration: rounds:1
```

**`content/technologies/neural/neural_static.yaml`:**
```yaml
id: neural_static
name: Neural Static
description: Floods a target's sensory nerves with dissonant white-noise, slowing their reactions.
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
resolution: save
save_type: hustle
save_dc: 15
effects:
  on_crit_success: []
  on_success: []
  on_failure:
    - type: condition
      condition_id: slowed
      value: 1
      duration: rounds:1
  on_crit_failure:
    - type: condition
      condition_id: slowed
      value: 2
      duration: rounds:1
amped_level: 3
amped_effects:
  on_crit_success: []
  on_success: []
  on_failure:
    - type: condition
      condition_id: slowed
      value: 1
      duration: rounds:2
  on_crit_failure:
    - type: condition
      condition_id: slowed
      value: 2
      duration: rounds:2
```

**`content/technologies/neural/synaptic_surge.yaml`:**
```yaml
id: synaptic_surge
name: Synaptic Surge
description: Overwhelms a target's nervous system with a burst of pain signals.
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
resolution: save
save_type: cool
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: frightened
      value: 1
      duration: rounds:1
  on_failure:
    - type: damage
      dice: 2d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
  on_crit_failure:
    - type: damage
      dice: 4d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 3
      duration: rounds:1
amped_level: 3
amped_effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: frightened
      value: 1
      duration: rounds:1
  on_failure:
    - type: damage
      dice: 4d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
  on_crit_failure:
    - type: damage
      dice: 6d4
      damage_type: neural
    - type: condition
      condition_id: frightened
      value: 3
      duration: rounds:1
```

**`content/technologies/innate/blackout_pulse.yaml`:**
```yaml
id: blackout_pulse
name: Blackout Pulse
description: Emits a localized EM burst that kills electronic lighting and blinds optical sensors in a small radius.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: zone
targets: all_enemies
duration: rounds:1
resolution: none
effects:
  on_apply:
    - type: condition
      condition_id: blinded
      value: 1
      duration: rounds:1
```

**`content/technologies/innate/arc_lights.yaml`:**
```yaml
id: arc_lights
name: Arc Lights
description: Projects three hovering electromagnetic arc-light drones that illuminate and disorient.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: zone
targets: all_enemies
duration: minutes:1
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Three hovering arc-light drones illuminate the area, shedding bright light in a wide radius."
```

**`content/technologies/innate/pressure_burst.yaml`:**
```yaml
id: pressure_burst
name: Pressure Burst
description: A pneumatic compression rig vents a focused blast that shoves targets and shatters brittle obstacles.
tradition: technical
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: instant
resolution: attack
effects:
  on_miss: []
  on_hit:
    - type: damage
      dice: 3d6
      damage_type: bludgeoning
    - type: movement
      distance: 5
      direction: away
  on_crit_hit:
    - type: damage
      dice: 6d6
      damage_type: bludgeoning
    - type: movement
      distance: 10
      direction: away
```

**`content/technologies/innate/nanite_infusion.yaml`:**
```yaml
id: nanite_infusion
name: Nanite Infusion
description: Releases a cloud of salvaged medical nanites that accelerate tissue repair in a touched target.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: self
targets: self
duration: instant
resolution: none
effects:
  on_apply:
    - type: heal
      dice: 1d8
      amount: 8
```

**`content/technologies/innate/atmospheric_surge.yaml`:**
```yaml
id: atmospheric_surge
name: Atmospheric Surge
description: A wrist-mounted atmospheric compressor discharges a powerful wind blast that scatters enemies.
tradition: technical
level: 1
usage_type: innate
action_cost: 2
range: zone
targets: all_enemies
duration: instant
resolution: save
save_type: toughness
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: utility
      description: "The surge buffets you but you hold your ground."
  on_failure:
    - type: condition
      condition_id: prone
      value: 1
      duration: rounds:1
  on_crit_failure:
    - type: damage
      dice: 2d6
      damage_type: bludgeoning
    - type: condition
      condition_id: prone
      value: 1
      duration: rounds:1
    - type: movement
      distance: 30
      direction: away
```

**`content/technologies/innate/viscous_spray.yaml`:**
```yaml
id: viscous_spray
name: Viscous Spray
description: A bio-synthetic adhesive secretion coats a target's joints and limbs, restraining movement.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: instant
resolution: save
save_type: hustle
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: slowed
      value: 1
      duration: rounds:1
  on_failure:
    - type: condition
      condition_id: immobilized
      value: 1
      duration: rounds:1
  on_crit_failure:
    - type: condition
      condition_id: immobilized
      value: 1
      duration: rounds:2
```

**`content/technologies/innate/chrome_reflex.yaml`:**
```yaml
id: chrome_reflex
name: Chrome Reflex
description: A neural-augmented reflex burst that overrides the nervous system and forces a second attempt at a failed saving throw.
tradition: neural
level: 1
usage_type: innate
action_cost: 1
range: self
targets: self
duration: instant
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Your chrome-augmented nervous system fires. You feel a momentary surge of neural clarity."
```

**`content/technologies/innate/seismic_sense.yaml`:**
```yaml
id: seismic_sense
name: Seismic Sense
description: Bone-conduction implants detect ground vibrations, revealing the movement of creatures through floors and walls.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: zone
targets: self
duration: rounds:1
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Your bone-conduction implants detect ground vibrations. You sense the movement of all creatures in the room through the floor."
```

**`content/technologies/innate/moisture_reclaim.yaml`:**
```yaml
id: moisture_reclaim
name: Moisture Reclaim
description: Atmospheric condensation filters extract potable water from ambient humidity.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 1
range: self
targets: self
duration: instant
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Your condensation filters extract 2 gallons of potable water from the ambient air."
```

**`content/technologies/innate/terror_broadcast.yaml`:**
```yaml
id: terror_broadcast
name: Terror Broadcast
description: A subdermal transmitter floods nearby targets with a fear-inducing neural frequency.
tradition: neural
level: 1
usage_type: innate
action_cost: 2
range: zone
targets: all_enemies
duration: rounds:1
resolution: save
save_type: cool
save_dc: 15
effects:
  on_crit_success: []
  on_success:
    - type: condition
      condition_id: frightened
      value: 1
      duration: rounds:1
  on_failure:
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
  on_crit_failure:
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
    - type: condition
      condition_id: fleeing
      value: 1
      duration: rounds:1
```

**`content/technologies/innate/acid_spit.yaml`:**
```yaml
id: acid_spit
name: Acid Spit
description: A bio-synthetic gland secretes pressurized corrosive fluid at a single target.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: instant
resolution: attack
effects:
  on_miss: []
  on_hit:
    - type: damage
      dice: 1d6
      damage_type: acid
  on_crit_hit:
    - type: damage
      dice: 2d6
      damage_type: acid
```

- [ ] **Step 4: Check for `TechEffect.Description` field**

The `TechEffect` struct currently has `UtilityType string` for utility effects, but the spec uses `Description string`. Check if `Description` exists:

```bash
grep -n "Description\|description" /home/cjohannsen/src/mud/internal/game/technology/model.go
```

If `Description` is not in `TechEffect`, add it:
```go
// utility
UtilityType string `yaml:"utility_type,omitempty"` // unlock | reveal | hack
Description string `yaml:"description,omitempty"`  // human-readable text for utility effects
```

Then update `tech_effect_resolver.go` in `applyEffect` `EffectUtility` case to use `e.Description`.

- [ ] **Step 5: Run YAML loading tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -run "TestRegistry_REQ_TER19|TestRegistry_REQ_TER20" -v 2>&1 | tail -20
```
Expected: PASS.

- [ ] **Step 6: Run full tech package tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -v 2>&1 | grep -E "^(PASS|FAIL|---)" | tail -20
```
Expected: all PASS. If pre-existing tests relied on `len(t.Effects) > 0` patterns, they'll surface here — fix them to use the new `TieredEffects` structure.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud
go test ./... 2>&1 | grep -E "^(PASS|FAIL|ok|---)" | tail -30
```
Expected: all PASS. Fix any remaining compilation errors from the model change.

- [ ] **Step 8: Commit**

```bash
git add content/technologies/neural/ content/technologies/innate/ \
        internal/game/technology/registry_test.go \
        internal/game/technology/model.go
git commit -m "feat(content): tiered effects for all 14 technologies; REQ-TER19-20 (REQ-CONTENT1)"
```

---

### Task 7: Update FEATURES.md and final verification

**Files:**
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Mark items complete in FEATURES.md**

In `docs/requirements/FEATURES.md`, change:
```markdown
        - [ ] Prepared tech effect resolution — activating a tech applies its game effect (damage, condition, etc.) (Sub-project: Tech Effect Resolution)
```
to:
```markdown
        - [x] Prepared tech effect resolution — activating a tech applies its game effect (damage, condition, etc.) (Sub-project: Tech Effect Resolution; REQ-TER1–22)
```

Similarly mark complete:
```markdown
      - [ ] Spontaneous tech effect resolution — activating a spontaneous tech applies its game effect (Sub-project: Tech Effect Resolution)
```
→
```markdown
      - [x] Spontaneous tech effect resolution — activating a spontaneous tech applies its game effect (Sub-project: Tech Effect Resolution; REQ-TER1–22)
```

And:
```markdown
        - [ ] Innate tech effect resolution — activating an innate tech applies its game effect (damage, condition, etc.) (Sub-project: Tech Effect Resolution)
```
→
```markdown
        - [x] Innate tech effect resolution — activating an innate tech applies its game effect (damage, condition, etc.) (Sub-project: Tech Effect Resolution; REQ-TER1–22)
```

- [ ] **Step 2: Final full test suite run**

```bash
cd /home/cjohannsen/src/mud
go test ./... 2>&1 | grep -E "^(PASS|FAIL|ok\s)" | tee /tmp/final_test_results.txt
grep "FAIL" /tmp/final_test_results.txt || echo "All tests passed."
```
Expected: "All tests passed."

- [ ] **Step 3: Verify build**

```bash
cd /home/cjohannsen/src/mud
go build ./... 2>&1
```
Expected: no output (clean build).

- [ ] **Step 4: Final commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "feat(tech-effect-resolution): mark prepared/spontaneous/innate effect resolution complete (REQ-TER1–22, REQ-COND1)"
```
