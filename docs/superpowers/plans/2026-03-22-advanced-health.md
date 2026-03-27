# Advanced Health Effects Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add substance effect system (drugs/alcohol/medicine/poison/toxin) with onset delay, addiction state machine, and 5-second ticker for effect progression.

**Architecture:** New SubstanceDef/SubstanceRegistry package under internal/game/substance/; PlayerSession gets SubstanceState (active effects, addiction level per substance); 5-second ticker in gameserver processes onset delays and applies/removes effects; addiction state machine with 5 states.

**Tech Stack:** Go, YAML substance definitions, existing condition/session packages, DB migration for substance state

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/game/substance/definition.go` | SubstanceDef, SubstanceEffect, SubstanceRegistry, LoadDirectory, Validate |
| Create | `internal/game/substance/definition_test.go` | TDD + property-based tests for SubstanceDef and Registry |
| Create | `internal/game/substance/active.go` | ActiveSubstance, SubstanceAddiction structs |
| Create | `internal/game/substance/active_test.go` | Unit tests for active substance structs |
| Create | `content/substances/jet.yaml` | Drug example substance |
| Create | `content/substances/cheap_whiskey.yaml` | Alcohol example substance |
| Create | `content/substances/stimpak.yaml` | Medicine example substance |
| Create | `content/substances/viper_venom.yaml` | Poison example substance |
| Modify | `internal/game/session/manager.go` | Add ActiveSubstances, AddictionState, SubstanceConditionRefs fields to PlayerSession |
| Modify | `internal/gameserver/grpc_service.go` | Inject SubstanceRegistry; add tickSubstances, applySubstanceDose, onSubstanceExpired, ApplySubstanceByID; extend 5s ticker; add consumable use handler |
| Create | `internal/gameserver/grpc_service_substance_test.go` | TDD tests for substance tick logic, dose resolution, addiction state machine |
| Modify | `internal/game/inventory/item.go` | Add SubstanceID string field (substance ID) and PoisonSubstanceID string field to ItemDef; update Validate |
| Modify | `internal/game/inventory/item_test.go` | Tests for new ItemDef fields |
| Modify | `internal/game/trap/template.go` | Add SubstanceID string field to TrapPayload |
| Modify | `internal/game/trap/payload.go` | Propagate SubstanceID in TriggerResult |
| Modify | `internal/game/trap/payload.go` | Add SubstanceID to TriggerResult struct |
| Modify | `internal/gameserver/grpc_service_trap.go` | Call ApplySubstanceByID when TriggerResult.SubstanceID is non-empty |
| Modify | `cmd/gameserver/wire_gen.go` | Inject SubstanceRegistry into ContentDeps and GameServiceServer |
| Modify | `cmd/gameserver/wire.go` | Add SubstanceRegistry provider |
| Modify | `internal/gameserver/gameserver.go` (ContentDeps) | Add SubstanceRegistry field to ContentDeps |
| Create | `internal/gameserver/grpc_service_substance_property_test.go` | pgregory.net/rapid property-based tests |

---

## Task 1: Define SubstanceDef, SubstanceEffect, SubstanceRegistry

**Files:**
- Create: `internal/game/substance/definition.go`
- Create: `internal/game/substance/definition_test.go`

**REQ coverage:** REQ-AH-0A, REQ-AH-0B, REQ-AH-0C, REQ-AH-0D, REQ-AH-1, REQ-AH-2, REQ-AH-4, REQ-AH-4A, REQ-AH-26, REQ-AH-27

- [ ] **Step 1: Write failing tests**

```go
// internal/game/substance/definition_test.go
package substance_test

import (
    "testing"
    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/substance"
)

func TestSubstanceDef_Validate_RejectsEmptyID(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsEmptyName(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsInvalidCategory(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsInvalidOnsetDelay(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsInvalidDuration(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsInvalidRecoveryDuration(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsAddictionChanceOutOfRange(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsOverdoseThresholdLessThan1(t *testing.T) { ... }
func TestSubstanceDef_Validate_RejectsMedicineAddictiveTrue(t *testing.T) { ... } // REQ-AH-26
func TestSubstanceDef_Validate_ValidDef_NoError(t *testing.T) { ... }
func TestSubstanceRegistry_Get_Found(t *testing.T) { ... }
func TestSubstanceRegistry_Get_NotFound(t *testing.T) { ... }
func TestSubstanceRegistry_LoadDirectory_ParsesYAML(t *testing.T) { ... }
func TestSubstanceRegistry_LoadDirectory_MissingDir_Error(t *testing.T) { ... }
func TestPropertySubstanceDef_ValidateNeverPanics(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        def := &substance.SubstanceDef{
            ID:               rapid.String().Draw(t, "id"),
            Name:             rapid.String().Draw(t, "name"),
            Category:         rapid.String().Draw(t, "category"),
            OnsetDelayStr:    rapid.String().Draw(t, "onset_delay"),
            DurationStr:      rapid.String().Draw(t, "duration"),
            RecoveryDurStr:   rapid.String().Draw(t, "recovery_duration"),
            AddictionChance:  rapid.Float64().Draw(t, "addiction_chance"),
            OverdoseThreshold: rapid.Int().Draw(t, "overdose_threshold"),
        }
        _ = def.Validate() // must not panic
    })
}
```

- [ ] **Step 2: Implement `internal/game/substance/definition.go`**

```go
package substance

import (
    "bytes"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "gopkg.in/yaml.v3"
)

// ValidCategories is the set of accepted substance category values.
var ValidCategories = map[string]bool{
    "drug": true, "alcohol": true, "medicine": true, "poison": true, "toxin": true,
}

// SubstanceEffect describes one effect applied at onset.
// Exactly one action field must be non-zero.
type SubstanceEffect struct {
    ApplyCondition  string   `yaml:"apply_condition,omitempty"`
    Stacks          int      `yaml:"stacks,omitempty"`
    RemoveCondition string   `yaml:"remove_condition,omitempty"`
    HPRegen         int      `yaml:"hp_regen,omitempty"`
    CureConditions  []string `yaml:"cure_conditions,omitempty"`
}

// SubstanceDef is the static definition of a substance loaded from YAML.
type SubstanceDef struct {
    ID                string            `yaml:"id"`
    Name              string            `yaml:"name"`
    Category          string            `yaml:"category"`
    OnsetDelayStr     string            `yaml:"onset_delay"`
    DurationStr       string            `yaml:"duration"`
    Effects           []SubstanceEffect `yaml:"effects"`
    RemoveOnExpire    []string          `yaml:"remove_on_expire"`
    Addictive         bool              `yaml:"addictive"`
    AddictionChance   float64           `yaml:"addiction_chance"`
    OverdoseThreshold int               `yaml:"overdose_threshold"`
    OverdoseCondition string            `yaml:"overdose_condition"`
    WithdrawalConditions []string       `yaml:"withdrawal_conditions"`
    RecoveryDurStr    string            `yaml:"recovery_duration"`

    // Parsed durations — populated by Validate().
    OnsetDelay      time.Duration
    Duration        time.Duration
    RecoveryDuration time.Duration
}

// Validate parses duration strings and checks all invariants.
// Postcondition: returns nil iff all fields are valid; sets OnsetDelay, Duration, RecoveryDuration.
func (d *SubstanceDef) Validate() error {
    var errs []error
    if d.ID == "" {
        errs = append(errs, errors.New("id must not be empty"))
    }
    if d.Name == "" {
        errs = append(errs, errors.New("name must not be empty"))
    }
    if !ValidCategories[d.Category] {
        errs = append(errs, fmt.Errorf("category must be one of drug|alcohol|medicine|poison|toxin, got %q", d.Category))
    }
    // REQ-AH-26: medicine may not be addictive.
    if d.Category == "medicine" && d.Addictive {
        errs = append(errs, errors.New("medicine substances must not be addictive"))
    }
    var err error
    if d.OnsetDelay, err = time.ParseDuration(d.OnsetDelayStr); err != nil {
        errs = append(errs, fmt.Errorf("invalid onset_delay %q: %w", d.OnsetDelayStr, err))
    }
    if d.Duration, err = time.ParseDuration(d.DurationStr); err != nil {
        errs = append(errs, fmt.Errorf("invalid duration %q: %w", d.DurationStr, err))
    }
    if d.RecoveryDuration, err = time.ParseDuration(d.RecoveryDurStr); err != nil {
        errs = append(errs, fmt.Errorf("invalid recovery_duration %q: %w", d.RecoveryDurStr, err))
    }
    if d.AddictionChance < 0.0 || d.AddictionChance > 1.0 {
        errs = append(errs, fmt.Errorf("addiction_chance must be in [0,1], got %v", d.AddictionChance))
    }
    if d.OverdoseThreshold < 1 {
        errs = append(errs, fmt.Errorf("overdose_threshold must be >= 1, got %d", d.OverdoseThreshold))
    }
    if len(errs) > 0 {
        return fmt.Errorf("substance %q validation failed: %v", d.ID, errs)
    }
    return nil
}

// Registry holds all known SubstanceDefs keyed by ID.
type Registry struct {
    defs map[string]*SubstanceDef
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
    return &Registry{defs: make(map[string]*SubstanceDef)}
}

// Get returns the SubstanceDef for id.
// Postcondition: Returns (def, true) if found, or (nil, false) otherwise.
func (r *Registry) Get(id string) (*SubstanceDef, bool) {
    d, ok := r.defs[id]
    return d, ok
}

// All returns a snapshot slice sorted by ID ascending.
// Postcondition: returned slice is sorted by ID ascending.
func (r *Registry) All() []*SubstanceDef {
    out := make([]*SubstanceDef, 0, len(r.defs))
    for _, d := range r.defs {
        out = append(out, d)
    }
    sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
    return out
}

// Register adds def to the registry.
// Precondition: def must not be nil and def.ID must not be empty.
func (r *Registry) Register(def *SubstanceDef) {
    if def == nil || def.ID == "" {
        return
    }
    r.defs[def.ID] = def
}

// LoadDirectory reads every *.yaml in dir, parses as SubstanceDef, validates, and returns a Registry.
// Precondition: dir must be a readable directory.
// Postcondition: Returns a non-nil Registry or error if any file fails to parse or validate.
func LoadDirectory(dir string) (*Registry, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, fmt.Errorf("reading substance dir %q: %w", dir, err)
    }
    reg := NewRegistry()
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
            continue
        }
        path := filepath.Join(dir, e.Name())
        data, err := os.ReadFile(path)
        if err != nil {
            return nil, fmt.Errorf("reading %q: %w", path, err)
        }
        var def SubstanceDef
        dec := yaml.NewDecoder(bytes.NewReader(data))
        dec.KnownFields(true)
        if err := dec.Decode(&def); err != nil {
            return nil, fmt.Errorf("parsing %q: %w", path, err)
        }
        if err := def.Validate(); err != nil {
            return nil, fmt.Errorf("invalid substance in %q: %w", path, err)
        }
        reg.Register(&def)
    }
    return reg, nil
}

// CrossValidate checks all condition ID references in every SubstanceDef against condIDs.
// REQ-AH-4A: any unknown condition ID causes a fatal startup error.
// Precondition: condIDs must be the complete set of known condition IDs.
// Postcondition: returns nil iff all referenced condition IDs are in condIDs.
func (r *Registry) CrossValidate(condIDs map[string]bool) error {
    var unknown []string
    for _, def := range r.defs {
        for _, eff := range def.Effects {
            if eff.ApplyCondition != "" && !condIDs[eff.ApplyCondition] {
                unknown = append(unknown, fmt.Sprintf("%s.effects.apply_condition:%s", def.ID, eff.ApplyCondition))
            }
            if eff.RemoveCondition != "" && !condIDs[eff.RemoveCondition] {
                unknown = append(unknown, fmt.Sprintf("%s.effects.remove_condition:%s", def.ID, eff.RemoveCondition))
            }
            for _, c := range eff.CureConditions {
                if !condIDs[c] {
                    unknown = append(unknown, fmt.Sprintf("%s.effects.cure_conditions:%s", def.ID, c))
                }
            }
        }
        for _, c := range def.RemoveOnExpire {
            if !condIDs[c] {
                unknown = append(unknown, fmt.Sprintf("%s.remove_on_expire:%s", def.ID, c))
            }
        }
        if def.OverdoseCondition != "" && !condIDs[def.OverdoseCondition] {
            unknown = append(unknown, fmt.Sprintf("%s.overdose_condition:%s", def.ID, def.OverdoseCondition))
        }
        for _, c := range def.WithdrawalConditions {
            if !condIDs[c] {
                unknown = append(unknown, fmt.Sprintf("%s.withdrawal_conditions:%s", def.ID, c))
            }
        }
    }
    if len(unknown) > 0 {
        sort.Strings(unknown)
        return fmt.Errorf("unknown condition IDs in substance definitions: %v", unknown)
    }
    return nil
}
```

- [ ] **Step 3: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/game/substance/...
  ```

---

## Task 2: Define ActiveSubstance and SubstanceAddiction structs

**Files:**
- Create: `internal/game/substance/active.go`
- Create: `internal/game/substance/active_test.go`

**REQ coverage:** REQ-AH-5, REQ-AH-6, REQ-AH-6A

- [ ] **Step 1: Write failing tests**

```go
// active_test.go
func TestActiveSubstance_ZeroValue(t *testing.T) {
    var a substance.ActiveSubstance
    assert.Equal(t, "", a.SubstanceID)
    assert.Equal(t, 0, a.DoseCount)
    assert.False(t, a.EffectsApplied)
}

func TestSubstanceAddiction_ZeroValue(t *testing.T) {
    var a substance.SubstanceAddiction
    assert.Equal(t, "", a.Status)
    assert.True(t, a.WithdrawalUntil.IsZero())
}
```

- [ ] **Step 2: Implement `internal/game/substance/active.go`**

```go
package substance

import "time"

// ActiveSubstance tracks one consumed substance entry in a player session.
type ActiveSubstance struct {
    SubstanceID    string
    DoseCount      int
    OnsetAt        time.Time
    ExpiresAt      time.Time
    EffectsApplied bool
}

// SubstanceAddiction tracks the addiction state for one substance per player session.
type SubstanceAddiction struct {
    // Status is "" (clean), "at_risk", "addicted", or "withdrawal".
    Status          string
    WithdrawalUntil time.Time
}
```

- [ ] **Step 3: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/game/substance/...
  ```

---

## Task 3: Add SubstanceState fields to PlayerSession

**Files:**
- Modify: `internal/game/session/manager.go`

> **Serialization Constraint:** quests and factions also add fields to PlayerSession. Implement these three features sequentially (not in parallel) to avoid conflicts on this struct.

**REQ coverage:** REQ-AH-5, REQ-AH-6, REQ-AH-6A, REQ-AH-7

- [ ] **Step 1: Write failing tests in `internal/game/session/manager_test.go`**

```go
func TestAddPlayer_SubstanceFieldsInitializedNil(t *testing.T) {
    m := session.NewManager()
    sess, err := m.AddPlayer(session.AddPlayerOptions{
        UID: "u1", Username: "bob", CharName: "Bob",
        RoomID: "r1", Role: "player", CharacterID: 1,
        CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    // REQ-AH-5: ActiveSubstances nil slice zero value
    assert.Nil(t, sess.ActiveSubstances)
    // REQ-AH-6: AddictionState nil map zero value
    assert.Nil(t, sess.AddictionState)
    // REQ-AH-6A: SubstanceConditionRefs nil map zero value
    assert.Nil(t, sess.SubstanceConditionRefs)
}
```

- [ ] **Step 2: Add fields to `PlayerSession` struct in `internal/game/session/manager.go`**

Add the following fields to the `PlayerSession` struct (after the existing session-only fields near the bottom of the struct, before `InitDone`):

```go
// ActiveSubstances tracks all currently active substance doses for this player.
// REQ-AH-5: nil slice is the zero value; initialized lazily on first dose.
// REQ-AH-7: session-only; cleared on disconnect.
ActiveSubstances []substance.ActiveSubstance
// AddictionState maps substance ID to the player's current addiction state for that substance.
// REQ-AH-6: nil map is the zero value; lazily initialized on first write.
// REQ-AH-7: session-only; cleared on disconnect.
AddictionState map[string]substance.SubstanceAddiction
// SubstanceConditionRefs counts how many active substances have applied each condition ID.
// REQ-AH-6A: nil map is the zero value; lazily initialized on first write.
// REQ-AH-7: session-only; cleared on disconnect.
SubstanceConditionRefs map[string]int
```

- [ ] **Step 3: Add import for `github.com/cory-johannsen/mud/internal/game/substance` to `manager.go`**

- [ ] **Step 4: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/game/session/...
  ```

---

## Task 4: Create example substance YAML content files

**Files:**
- Create: `content/substances/jet.yaml`
- Create: `content/substances/cheap_whiskey.yaml`
- Create: `content/substances/stimpak.yaml`
- Create: `content/substances/viper_venom.yaml`

**REQ coverage:** REQ-AH-1, REQ-AH-2

- [ ] **Step 1: Create `content/substances/jet.yaml`**

```yaml
id: jet
name: Jet
category: drug
onset_delay: "0s"
duration: "20m"
effects:
  - apply_condition: speed_boost
    stacks: 1
  - apply_condition: tunnel_vision
    stacks: 1
remove_on_expire:
  - speed_boost
  - tunnel_vision
addictive: true
addiction_chance: 0.20
overdose_threshold: 3
overdose_condition: stimulant_overdose
withdrawal_conditions:
  - fatigue
  - nausea
recovery_duration: "4h"
```

- [ ] **Step 2: Create `content/substances/cheap_whiskey.yaml`**

```yaml
id: cheap_whiskey
name: Cheap Whiskey
category: alcohol
onset_delay: "30s"
duration: "15m"
effects:
  - apply_condition: drunk
    stacks: 1
remove_on_expire:
  - drunk
addictive: false
addiction_chance: 0.0
overdose_threshold: 5
overdose_condition: alcohol_poisoning
withdrawal_conditions: []
recovery_duration: "0s"
```

- [ ] **Step 3: Create `content/substances/stimpak.yaml`**

```yaml
id: stimpak
name: Stimpak
category: medicine
onset_delay: "0s"
duration: "5m"
effects:
  - hp_regen: 2
remove_on_expire: []
addictive: false
addiction_chance: 0.0
overdose_threshold: 10
overdose_condition: stimulant_overdose
withdrawal_conditions: []
recovery_duration: "0s"
```

- [ ] **Step 4: Create `content/substances/viper_venom.yaml`**

```yaml
id: viper_venom
name: Viper Venom
category: poison
onset_delay: "10s"
duration: "3m"
effects:
  - apply_condition: poisoned
    stacks: 1
remove_on_expire:
  - poisoned
addictive: false
addiction_chance: 0.0
overdose_threshold: 1
overdose_condition: severe_poisoning
withdrawal_conditions: []
recovery_duration: "0s"
```

**Note:** The condition IDs referenced (speed_boost, tunnel_vision, drunk, poisoned, fatigue, nausea, stimulant_overdose, alcohol_poisoning, severe_poisoning) must be added to `content/conditions/` if not already present, or the cross-validation step in Task 8 will fail at startup. Add stub condition YAML files for any that are missing.

---

## Task 5: Add ItemDef.SubstanceID and ItemDef.PoisonSubstanceID fields

> **NOTE:** `ItemDef.Effect *ConsumableEffect` is added by equipment-mechanics. This feature uses the separate `SubstanceID string` field (YAML: `substance_id`) to avoid collision.

**Files:**
- Modify: `internal/game/inventory/item.go`
- Modify: `internal/game/inventory/item_test.go`

**REQ coverage:** REQ-AH-8, REQ-AH-21

- [ ] **Step 1: Write failing tests in `internal/game/inventory/item_test.go`**

```go
func TestItemDef_SubstanceID_Field_Stored(t *testing.T) {
    d := inventory.ItemDef{
        ID: "stimpak_item", Name: "Stimpak", Kind: inventory.KindConsumable,
        MaxStack: 10, SubstanceID: "stimpak",
    }
    err := d.Validate()
    assert.NoError(t, err)
    assert.Equal(t, "stimpak", d.SubstanceID)
}

func TestItemDef_PoisonSubstanceID_Field_Stored(t *testing.T) {
    d := inventory.ItemDef{
        ID: "poison_dagger", Name: "Poison Dagger", Kind: inventory.KindWeapon,
        MaxStack: 1, WeaponRef: "dagger", PoisonSubstanceID: "viper_venom",
    }
    err := d.Validate()
    assert.NoError(t, err)
    assert.Equal(t, "viper_venom", d.PoisonSubstanceID)
}
```

- [ ] **Step 2: Add fields to `ItemDef` struct in `internal/game/inventory/item.go`**

```go
// SubstanceID is the substance ID applied when this consumable item is used.
// Only meaningful when Kind == KindConsumable. Empty for non-substance consumables.
SubstanceID string `yaml:"substance_id,omitempty"`
// PoisonSubstanceID is the substance ID applied to a target on a successful weapon hit.
// Only meaningful when Kind == KindWeapon. Empty for non-poisoned weapons.
// REQ-AH-21: attack pipeline calls ApplySubstanceByID when this is non-empty.
PoisonSubstanceID string `yaml:"poison_substance_id,omitempty"`
```

- [ ] **Step 3: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/...
  ```

---

## Task 6: Add SubstanceID to TrapPayload and TriggerResult

**Files:**
- Modify: `internal/game/trap/template.go`
- Modify: `internal/game/trap/payload.go`

**REQ coverage:** REQ-AH-22

- [ ] **Step 1: Write failing tests**

```go
// internal/game/trap/payload_test.go — add:
func TestResolveTrigger_SubstanceID_Propagated(t *testing.T) {
    tmpl := &trap.TrapTemplate{
        ID: "poison_pit", Trigger: trap.TriggerEntry,
        Payload: &trap.TrapPayload{Type: "pit", Damage: "1d6", SubstanceID: "viper_venom"},
    }
    result, err := trap.ResolveTrigger(tmpl, "safe", nil)
    require.NoError(t, err)
    assert.Equal(t, "viper_venom", result.SubstanceID)
}
```

- [ ] **Step 2: Add `SubstanceID string` field to `TrapPayload` in `internal/game/trap/template.go`**

```go
SubstanceID string `yaml:"substance_id,omitempty"`
```

- [ ] **Step 3: Add `SubstanceID string` field to `TriggerResult` in `internal/game/trap/payload.go`**

```go
// SubstanceID is the substance applied on trap fire (empty = no substance).
// REQ-AH-22: caller must call ApplySubstanceByID when non-empty.
SubstanceID string
```

- [ ] **Step 4: Propagate `SubstanceID` from `TrapPayload` to `TriggerResult` in `resolvePayload` for all payload types** (mine, pit, bear_trap, trip_wire; skip honkeypot as it has no physical contact).

- [ ] **Step 5: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/game/trap/...
  ```

---

## Task 7: Inject SubstanceRegistry into GameServiceServer

**Files:**
- Modify: `internal/gameserver/gameserver.go` (ContentDeps struct — find by searching for `ContentDeps`)
- Modify: `cmd/gameserver/wire_gen.go`
- Modify: `cmd/gameserver/wire.go`

**REQ coverage:** REQ-AH-3

- [ ] **Step 1: Locate `ContentDeps` struct** — it is defined in `internal/gameserver/gameserver.go` or a nearby file. Grep for `ContentDeps` to find the exact file.

- [ ] **Step 2: Add `SubstanceRegistry *substance.Registry` field to `ContentDeps`**

- [ ] **Step 3: In `cmd/gameserver/wire_gen.go`, after the `conditionRegistry` block, add:**

```go
substancesDir := cfg.SubstancesDir
substanceRegistry, err := substance.LoadDirectory(substancesDir)
if err != nil {
    return nil, err
}
// Cross-validate substance condition references against condition registry.
condIDs := make(map[string]bool)
for _, def := range conditionRegistry.All() {
    condIDs[def.ID] = true
}
if cvErr := substanceRegistry.CrossValidate(condIDs); cvErr != nil {
    return nil, fmt.Errorf("substance cross-validation failed: %w", cvErr)
}
```

Then add `SubstanceRegistry: substanceRegistry` to the `ContentDeps{}` struct literal.

- [ ] **Step 4: Add `SubstancesDir string` field to `AppConfig` in `cmd/gameserver/wire.go`**

- [ ] **Step 5: Add `SubstancesDir` to the `AppConfig` population in `cmd/gameserver/main.go`**, reading from an environment variable `SUBSTANCES_DIR` with default `content/substances`.

- [ ] **Step 6: Run full build and confirm compilation succeeds**
  ```
  cd /home/cjohannsen/src/mud && go build ./...
  ```

---

## Task 8: Implement applySubstanceDose and the addiction state machine

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_substance_test.go`

**REQ coverage:** REQ-AH-8A, REQ-AH-9, REQ-AH-10, REQ-AH-11, REQ-AH-17, REQ-AH-18, REQ-AH-19, REQ-AH-20

- [ ] **Step 1: Write failing tests in `internal/gameserver/grpc_service_substance_test.go`**

```go
package gameserver_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"

    "github.com/cory-johannsen/mud/internal/game/substance"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/gameserver"
)

// REQ-AH-9: First dose creates ActiveSubstance entry with DoseCount=1.
func TestApplySubstanceDose_FirstDose_CreatesEntry(t *testing.T) { ... }

// REQ-AH-9: Second dose increments DoseCount and extends ExpiresAt.
func TestApplySubstanceDose_SecondDose_IncrementsAndExtends(t *testing.T) { ... }

// REQ-AH-10: DoseCount > overdose_threshold applies overdose_condition immediately.
func TestApplySubstanceDose_Overdose_AppliesCondition(t *testing.T) { ... }

// REQ-AH-10: Overdose message sent to player.
func TestApplySubstanceDose_Overdose_SendsMessage(t *testing.T) { ... }

// REQ-AH-11: First dose of addictive substance sets status to "at_risk".
func TestApplySubstanceDose_Addictive_FirstDose_SetsAtRisk(t *testing.T) { ... }

// REQ-AH-11: at_risk dose with rand < addiction_chance → "addicted" + message.
func TestApplySubstanceDose_AtRisk_Roll_SetsAddicted(t *testing.T) { ... }

// REQ-AH-17: Dose while in withdrawal resets WithdrawalUntil, removes withdrawal conditions, sets addicted.
func TestApplySubstanceDose_WhileWithdrawal_ResetsAndSetsAddicted(t *testing.T) { ... }

// REQ-AH-18: Dose while addicted re-rolls; on success sends "Your dependency deepens."; status stays addicted.
func TestApplySubstanceDose_WhileAddicted_ReRoll_SendsMessage(t *testing.T) { ... }

// REQ-AH-8A: Use handler blocks poison/toxin with "You can't use that directly."
func TestHandleConsumeItem_PoisonBlocked(t *testing.T) { ... }
func TestHandleConsumeItem_ToxinBlocked(t *testing.T) { ... }

// REQ-AH-20: ApplySubstanceByID calls applySubstanceDose directly (no category guard).
func TestApplySubstanceByID_PoisonApplied(t *testing.T) { ... }

// REQ-AH-20: ApplySubstanceByID returns error for unknown substance ID.
func TestApplySubstanceByID_UnknownID_Error(t *testing.T) { ... }

// REQ-AH-19: Addiction state is independent per substance ID.
func TestAddictionState_IndependentPerSubstance(t *testing.T) { ... }

// Property: applySubstanceDose never panics regardless of input shape.
func TestPropertyApplySubstanceDose_NeverPanics(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) { ... })
}
```

- [ ] **Step 2: Add `substanceReg *substance.Registry` field to `GameServiceServer`**

- [ ] **Step 3: Pass `SubstanceRegistry` from `ContentDeps` to `GameServiceServer` in `NewGameServiceServer`**

- [ ] **Step 4: Implement `applySubstanceDose(uid string, def *substance.SubstanceDef)` on `*GameServiceServer`**

Preconditions: sess must be found for uid; def must be non-nil.
Postconditions: ActiveSubstances contains an entry for def.ID with updated DoseCount/ExpiresAt; overdose condition applied if threshold exceeded; addiction state advanced per state machine.

```
func (s *GameServiceServer) applySubstanceDose(uid string, def *substance.SubstanceDef) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok { return }

    now := time.Now()

    // Find or create entry in ActiveSubstances.
    var entry *substance.ActiveSubstance
    for i := range sess.ActiveSubstances {
        if sess.ActiveSubstances[i].SubstanceID == def.ID {
            entry = &sess.ActiveSubstances[i]
            break
        }
    }
    if entry == nil {
        sess.ActiveSubstances = append(sess.ActiveSubstances, substance.ActiveSubstance{
            SubstanceID: def.ID,
            DoseCount:   1,
            OnsetAt:     now.Add(def.OnsetDelay),
            ExpiresAt:   now.Add(def.OnsetDelay + def.Duration),
        })
        entry = &sess.ActiveSubstances[len(sess.ActiveSubstances)-1]
    } else {
        entry.DoseCount++
        entry.ExpiresAt = now.Add(def.OnsetDelay + def.Duration)
    }

    // REQ-AH-10: overdose check.
    if entry.DoseCount > def.OverdoseThreshold && def.OverdoseCondition != "" {
        if condDef, ok := s.condRegistry.Get(def.OverdoseCondition); ok {
            _ = sess.Conditions.Apply(uid, condDef, 1, -1)
        }
        s.sendMessageToPlayer(uid, "You've taken too much. Your body reacts violently.")
    }

    // REQ-AH-11 / REQ-AH-17 / REQ-AH-18: addiction state machine.
    if def.Addictive {
        if sess.AddictionState == nil {
            sess.AddictionState = make(map[string]substance.SubstanceAddiction)
        }
        addict := sess.AddictionState[def.ID]
        switch addict.Status {
        case "":
            addict.Status = "at_risk"
        case "at_risk":
            if s.roller.Float64() < def.AddictionChance {
                addict.Status = "addicted"
                s.sendMessageToPlayer(uid, "You feel a gnawing need for more.")
            }
        case "withdrawal":
            // REQ-AH-17: suppress symptoms, reset timer.
            s.removeWithdrawalConditions(uid, sess, def)
            addict.WithdrawalUntil = now.Add(def.RecoveryDuration)
            addict.Status = "addicted"
        case "addicted":
            // REQ-AH-18: re-roll.
            if s.roller.Float64() < def.AddictionChance {
                s.sendMessageToPlayer(uid, "Your dependency deepens.")
            }
        }
        sess.AddictionState[def.ID] = addict
    }
}
```

- [ ] **Step 5: Implement `ApplySubstanceByID(uid, substanceID string) error`**

```go
// ApplySubstanceByID looks up substanceID and calls applySubstanceDose directly.
// REQ-AH-20: bypasses the use handler category guard.
// Postcondition: returns error if substanceID is not found in the registry.
func (s *GameServiceServer) ApplySubstanceByID(uid, substanceID string) error {
    def, ok := s.substanceReg.Get(substanceID)
    if !ok {
        return fmt.Errorf("substance %q not found", substanceID)
    }
    s.applySubstanceDose(uid, def)
    return nil
}
```

- [ ] **Step 6: Implement `handleConsumeItem(uid, itemInstanceID string)` on `*GameServiceServer`** — the handler for using a KindConsumable inventory item with a substance effect.

```go
func (s *GameServiceServer) handleConsumeItem(uid, itemInstanceID string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("Session not found."), nil
    }
    // Look up item in backpack.
    inst, found := sess.Backpack.FindInstance(itemInstanceID)
    if !found {
        return messageEvent("Item not found in your backpack."), nil
    }
    itemDef, defOK := s.invRegistry.GetItem(inst.ItemDefID)
    if !defOK || itemDef.SubstanceID == "" {
        return messageEvent("You can't use that."), nil
    }
    def, substOK := s.substanceReg.Get(itemDef.SubstanceID)
    if !substOK {
        return messageEvent("Unknown substance."), nil
    }
    // REQ-AH-8A: block poison and toxin from voluntary use.
    if def.Category == "poison" || def.Category == "toxin" {
        return messageEvent("You can't use that directly."), nil
    }
    s.applySubstanceDose(uid, def)
    // Remove one stack from backpack.
    sess.Backpack.RemoveOne(itemInstanceID)
    return messageEvent(fmt.Sprintf("You use the %s.", itemDef.Name)), nil
}
```

- [ ] **Step 7: Wire `handleConsumeItem` into the command dispatch loop** in `commandLoop` for an `ItemUseRequest` (or whichever proto message the frontend sends for consuming items from inventory; check proto definitions).

- [ ] **Step 8: Implement helper `removeWithdrawalConditions(uid string, sess *session.PlayerSession, def *substance.SubstanceDef)`**

- [ ] **Step 9: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/...
  ```

---

## Task 9: Implement tickSubstances and onSubstanceExpired

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_substance_test.go`

**REQ coverage:** REQ-AH-12, REQ-AH-13, REQ-AH-14, REQ-AH-15, REQ-AH-16, REQ-AH-24, REQ-AH-25

- [ ] **Step 1: Write failing tests**

```go
// REQ-AH-13: tickSubstances fires onset when OnsetAt is past.
func TestTickSubstances_Onset_AppliesEffects(t *testing.T) { ... }

// REQ-AH-13: tickSubstances sends "kicks in" message on onset.
func TestTickSubstances_Onset_SendsKicksInMessage(t *testing.T) { ... }

// REQ-AH-13: tickSubstances increments SubstanceConditionRefs on onset.
func TestTickSubstances_Onset_IncrementsRefs(t *testing.T) { ... }

// REQ-AH-13: tickSubstances fires expiry, decrements refs, removes zero-ref conditions.
func TestTickSubstances_Expiry_RemovesZeroRefConditions(t *testing.T) { ... }

// REQ-AH-13: expiry does not remove condition when another substance still holds a ref.
func TestTickSubstances_Expiry_SharedCondition_NotRemoved(t *testing.T) { ... }

// REQ-AH-14: hp_regen applies per tick, clamped to MaxHP, no message.
func TestTickSubstances_HPRegen_AppliedPerTick(t *testing.T) { ... }
func TestTickSubstances_HPRegen_ClampedToMaxHP(t *testing.T) { ... }
func TestTickSubstances_HPRegen_NoMessageSent(t *testing.T) { ... }

// REQ-AH-15: onSubstanceExpired triggers withdrawal when addicted.
func TestOnSubstanceExpired_Addicted_SetsWithdrawal(t *testing.T) { ... }
func TestOnSubstanceExpired_Addicted_AppliesWithdrawalConditions(t *testing.T) { ... }
func TestOnSubstanceExpired_Addicted_SendsWithdrawalMessage(t *testing.T) { ... }

// REQ-AH-16: tickSubstances clears withdrawal after WithdrawalUntil, sends recovery message.
func TestTickSubstances_WithdrawalExpiry_SetsClean(t *testing.T) { ... }
func TestTickSubstances_WithdrawalExpiry_SendsRecoveryMessage(t *testing.T) { ... }
func TestTickSubstances_WithdrawalExpiry_RemovesWithdrawalConditions(t *testing.T) { ... }

// REQ-AH-24: cure_conditions removes listed conditions immediately on onset.
func TestTickSubstances_Onset_CureConditions_RemovedImmediately(t *testing.T) { ... }

// Property: tickSubstances never panics regardless of session state.
func TestPropertyTickSubstances_NeverPanics(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) { ... })
}
```

- [ ] **Step 2: Implement `tickSubstances(uid string)` on `*GameServiceServer`**

```go
// tickSubstances processes onset and expiry for all active substances in uid's session.
// REQ-AH-12: called by the 5-second ticker goroutine.
// REQ-AH-13: fires onset (EffectsApplied=false, time.Now().After(OnsetAt)) and
//            expiry (EffectsApplied=true, time.Now().After(ExpiresAt)).
// REQ-AH-14: applies hp_regen per tick for active medicine substances.
// REQ-AH-16: checks withdrawal expiry for all substances.
func (s *GameServiceServer) tickSubstances(uid string) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok { return }
    if s.substanceReg == nil { return }

    now := time.Now()

    // Check withdrawal expiry for each substance in AddictionState.
    // REQ-AH-16.
    for substID, addict := range sess.AddictionState {
        if addict.Status == "withdrawal" && now.After(addict.WithdrawalUntil) {
            def, defOK := s.substanceReg.Get(substID)
            if defOK {
                s.removeWithdrawalConditions(uid, sess, def)
            }
            addict.Status = ""
            addict.WithdrawalUntil = time.Time{}
            sess.AddictionState[substID] = addict
            s.sendMessageToPlayer(uid, "You feel like yourself again.")
        }
    }

    // Process active substance entries (onset, expiry, hp_regen).
    var remaining []substance.ActiveSubstance
    for i := range sess.ActiveSubstances {
        entry := &sess.ActiveSubstances[i]
        def, defOK := s.substanceReg.Get(entry.SubstanceID)
        if !defOK {
            remaining = append(remaining, *entry)
            continue
        }

        // REQ-AH-13: Onset.
        if !entry.EffectsApplied && now.After(entry.OnsetAt) {
            s.applySubstanceEffectsOnOnset(uid, sess, def)
            entry.EffectsApplied = true
            s.sendMessageToPlayer(uid, fmt.Sprintf("The %s kicks in.", def.Name))
        }

        // REQ-AH-14: HP regen while active.
        if entry.EffectsApplied && now.Before(entry.ExpiresAt) {
            for _, eff := range def.Effects {
                if eff.HPRegen > 0 {
                    sess.CurrentHP += eff.HPRegen
                    if sess.CurrentHP > sess.MaxHP {
                        sess.CurrentHP = sess.MaxHP
                    }
                }
            }
        }

        // REQ-AH-13: Expiry.
        if entry.EffectsApplied && now.After(entry.ExpiresAt) {
            s.applySubstanceExpiry(uid, sess, def)
            s.onSubstanceExpired(uid, def)
            continue // do not keep in remaining
        }

        remaining = append(remaining, *entry)
    }
    sess.ActiveSubstances = remaining
}
```

- [ ] **Step 3: Implement `applySubstanceEffectsOnOnset(uid string, sess *session.PlayerSession, def *substance.SubstanceDef)`**

Iterates `def.Effects` and for each effect:
- `ApplyCondition`: calls `sess.Conditions.Apply(uid, condDef, eff.Stacks, -1)`, increments `SubstanceConditionRefs[condID]`. (REQ-AH-13)
- `RemoveCondition`: calls `sess.Conditions.Remove(uid, eff.RemoveCondition)`. (REQ-AH-0B)
- `CureConditions`: for each condID, decrements `SubstanceConditionRefs[condID]`; removes from `sess.Conditions` if ref drops to 0. (REQ-AH-24)
- `HPRegen`: handled per tick in tickSubstances, not at onset.

- [ ] **Step 4: Implement `applySubstanceExpiry(uid string, sess *session.PlayerSession, def *substance.SubstanceDef)`**

For each condID in `def.RemoveOnExpire`:
- Decrement `SubstanceConditionRefs[condID]`.
- If ref count is 0, call `sess.Conditions.Remove(uid, condID)`.

- [ ] **Step 5: Implement `onSubstanceExpired(uid string, def *substance.SubstanceDef)`**

```go
// onSubstanceExpired handles post-expiry logic: triggers withdrawal if the player is addicted.
// REQ-AH-15.
func (s *GameServiceServer) onSubstanceExpired(uid string, def *substance.SubstanceDef) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok { return }
    if sess.AddictionState == nil { return }
    addict := sess.AddictionState[def.ID]
    if addict.Status != "addicted" { return }

    addict.Status = "withdrawal"
    addict.WithdrawalUntil = time.Now().Add(def.RecoveryDuration)
    sess.AddictionState[def.ID] = addict

    // Apply withdrawal conditions.
    if sess.SubstanceConditionRefs == nil {
        sess.SubstanceConditionRefs = make(map[string]int)
    }
    for _, condID := range def.WithdrawalConditions {
        if condDef, condOK := s.condRegistry.Get(condID); condOK {
            _ = sess.Conditions.Apply(uid, condDef, 1, -1)
            sess.SubstanceConditionRefs[condID]++
        }
    }
    s.sendMessageToPlayer(uid, "You feel sick without your fix.")
}
```

- [ ] **Step 6: Extend the 5-second ticker goroutine in `Session()` to also call `tickSubstances`**

In `internal/gameserver/grpc_service.go`, the existing ticker goroutine (lines ~1260–1272):

```go
case <-roomRefreshTicker.C:
    if sess, ok := s.sessions.GetPlayer(uid); ok {
        s.pushRoomViewToAllInRoom(sess.RoomID)
    }
    s.tickSubstances(uid) // REQ-AH-12
```

- [ ] **Step 7: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/...
  ```

---

## Task 10: Wire poison substances into the attack pipeline

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

**REQ coverage:** REQ-AH-21

- [ ] **Step 1: Write failing test**

```go
// internal/gameserver/combat_handler_test.go — add:
func TestAttack_PoisonedWeapon_AppliesSubstanceOnHit(t *testing.T) {
    // Build a weapon with PoisonSubstanceID set.
    // Ensure attacker has the weapon equipped.
    // Mock substanceReg to confirm ApplySubstanceByID is called.
    // Assert: after a successful hit, ApplySubstanceByID was called with correct substanceID.
}
```

- [ ] **Step 2: Locate the hit-resolution path in `combat_handler.go`** (near `func (h *CombatHandler) Attack`). Find the code that applies damage on a confirmed hit.

- [ ] **Step 3: After dealing damage on a successful hit, check the attacker's equipped weapon's `PoisonSubstanceID`**

```go
// REQ-AH-21: poisoned weapon applies substance on hit.
if equippedWeapon != nil && equippedWeapon.PoisonSubstanceID != "" {
    if err := h.substanceSvc.ApplySubstanceByID(targetUID, equippedWeapon.PoisonSubstanceID); err != nil {
        h.logger.Warn("ApplySubstanceByID failed", zap.Error(err))
    }
}
```

Where `h.substanceSvc` is a new interface field on `CombatHandler`:

```go
// SubstanceService is the interface CombatHandler needs for substance application.
type SubstanceService interface {
    ApplySubstanceByID(uid, substanceID string) error
}
```

- [ ] **Step 4: Add `SubstanceSvc SubstanceService` field to `CombatHandler` and populate it in `NewCombatHandlerProvider`**

- [ ] **Step 5: Implement `GameServiceServer` as satisfying `SubstanceService` (it already has `ApplySubstanceByID`)**

- [ ] **Step 6: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/...
  ```

---

## Task 11: Wire substance into trap trigger pipeline

**Files:**
- Modify: `internal/gameserver/grpc_service_trap.go`

**REQ coverage:** REQ-AH-22

- [ ] **Step 1: Write failing test**

```go
// internal/gameserver/grpc_service_trap_internal_test.go — add:
func TestTrapTrigger_SubstanceID_NonEmpty_CallsApplySubstanceByID(t *testing.T) {
    // Set up a trap with Payload.SubstanceID = "viper_venom".
    // Trigger the trap against a player.
    // Assert ApplySubstanceByID called with correct uid and substanceID.
}
```

- [ ] **Step 2: In `grpc_service_trap.go`, locate the trap fire handler** (where `ResolveTrigger` result is acted on — either in `fireConsumableTrapOnCombatant` or the world-trap firing path).

- [ ] **Step 3: After applying damage/conditions from TriggerResult, add:**

```go
// REQ-AH-22: substance application for poisoned traps.
if result.SubstanceID != "" {
    if err := s.ApplySubstanceByID(targetUID, result.SubstanceID); err != nil {
        s.logger.Warn("trap ApplySubstanceByID failed", zap.Error(err))
    }
}
```

- [ ] **Step 4: Run tests and confirm they pass**
  ```
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/...
  ```

---

## Task 12: Full test suite pass

- [ ] **Step 1: Run the full test suite**
  ```
  cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -30
  ```

- [ ] **Step 2: Fix any failures until the suite is 100% green**

- [ ] **Step 3: Build the full binary to confirm no compilation errors**
  ```
  cd /home/cjohannsen/src/mud && go build ./...
  ```

- [ ] **Step 4: Mark `advanced-health` as complete in `docs/features/index.yaml`**

---

## Requirement Coverage Matrix

| Requirement | Task(s) |
|-------------|---------|
| REQ-AH-0A | 1 |
| REQ-AH-0B | 1, 9 |
| REQ-AH-0C | 1, 9 |
| REQ-AH-0D | 1, 9 |
| REQ-AH-1  | 1, 4 |
| REQ-AH-2  | 1, 4 |
| REQ-AH-3  | 7 |
| REQ-AH-4  | 1 |
| REQ-AH-4A | 7 |
| REQ-AH-5  | 2, 3 |
| REQ-AH-6  | 2, 3 |
| REQ-AH-6A | 2, 3 |
| REQ-AH-7  | 3 |
| REQ-AH-8  | 8 |
| REQ-AH-8A | 8 |
| REQ-AH-9  | 8 |
| REQ-AH-10 | 8 |
| REQ-AH-11 | 8 |
| REQ-AH-12 | 9 |
| REQ-AH-13 | 9 |
| REQ-AH-14 | 9 |
| REQ-AH-15 | 9 |
| REQ-AH-16 | 9 |
| REQ-AH-17 | 8 |
| REQ-AH-18 | 8 |
| REQ-AH-19 | 8 |
| REQ-AH-20 | 8 |
| REQ-AH-21 | 5, 10 |
| REQ-AH-22 | 6, 11 |
| REQ-AH-24 | 9 |
| REQ-AH-25 | 9 |
| REQ-AH-26 | 1 |
| REQ-AH-27 | 1 (no-op validation) |
