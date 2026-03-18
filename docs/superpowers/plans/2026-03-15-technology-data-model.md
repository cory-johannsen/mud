# Technology Data Model Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `TechnologyDef` as a new top-level data type with YAML loader, registry, seed content, and `GameServiceServer` wiring — data model only, no resolution or slot tracking.

**Architecture:** A `technology` package at `internal/game/technology/` holds model, enums, validation, and registry. The registry follows the `condition.Registry` pattern: fail-fast `Load(dir)`, `KnownFields(true)` YAML decoder, sorted `All()`. `GameServiceServer` gets a `techRegistry *technology.Registry` field, loaded at startup from `content/technologies/`; a missing directory is non-fatal.

**Tech Stack:** Go modules (`github.com/cory-johannsen/mud`), `gopkg.in/yaml.v3`, `pgregory.net/rapid v1.2.0`, `github.com/stretchr/testify`

---

## Chunk 1: Model and Validation

### Task 1: Enums and TechEffect — failing tests

**Files:**
- Create: `internal/game/technology/model.go`
- Create: `internal/game/technology/model_test.go`

- [ ] **Step 1: Write the failing enum + validate tests**

```go
// internal/game/technology/model_test.go
package technology_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/technology"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"
)

// REQ-T1: Validate rejects unknown Tradition string
func TestValidate_UnknownTradition(t *testing.T) {
    def := validDef()
    def.Tradition = technology.Tradition("unknown")
    err := def.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "tradition")
}

// REQ-T2: Validate rejects Level < 1 or > 10
func TestValidate_LevelOutOfRange(t *testing.T) {
    for _, lvl := range []int{0, -1, 11, 100} {
        def := validDef()
        def.Level = lvl
        err := def.Validate()
        require.Error(t, err, "level=%d", lvl)
        assert.Contains(t, err.Error(), "level")
    }
}

// REQ-T3: Validate rejects empty Effects
func TestValidate_EmptyEffects(t *testing.T) {
    def := validDef()
    def.Effects = nil
    err := def.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "effect")
}

// REQ-T4: Validate rejects AmpedEffects non-empty with AmpedLevel == 0
func TestValidate_AmpedEffectsWithoutAmpedLevel(t *testing.T) {
    def := validDef()
    def.AmpedLevel = 0
    def.AmpedEffects = []technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "energy"}}
    err := def.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "amped")
}

// REQ-T5: Validate rejects AmpedLevel > 0 with empty AmpedEffects
func TestValidate_AmpedLevelWithoutAmpedEffects(t *testing.T) {
    def := validDef()
    def.AmpedLevel = 3
    def.AmpedEffects = nil
    err := def.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "amped")
}

// REQ-T6: Validate rejects skill_check effect with DC == 0
func TestValidate_SkillCheckEffectZeroDC(t *testing.T) {
    def := validDef()
    def.Effects = []technology.TechEffect{{Type: technology.EffectSkillCheck, Skill: "perception", DC: 0}}
    err := def.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "dc")
}

// REQ-T15: Validate rejects non-empty SaveType with SaveDC == 0
func TestValidate_SaveTypeWithoutSaveDC(t *testing.T) {
    def := validDef()
    def.SaveType = "reflex"
    def.SaveDC = 0
    err := def.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "save_dc")
}

// REQ-T16: condition effect with no Duration is valid
func TestValidate_ConditionEffectNoDurationIsValid(t *testing.T) {
    def := validDef()
    def.Effects = []technology.TechEffect{{Type: technology.EffectCondition, ConditionID: "stunned"}}
    err := def.Validate()
    require.NoError(t, err)
}

// REQ-T13 (property): TechEffect with valid required fields round-trips through YAML
func TestProperty_TechEffect_YAMLRoundTrip(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        effectType := rapid.SampledFrom([]technology.EffectType{
            technology.EffectDamage,
            technology.EffectHeal,
            technology.EffectCondition,
            technology.EffectSkillCheck,
            technology.EffectMovement,
            technology.EffectZone,
            technology.EffectSummon,
            technology.EffectUtility,
            technology.EffectDrain,
        }).Draw(rt, "effectType").(technology.EffectType)

        effect := minimalEffect(effectType)
        data, err := yaml.Marshal(effect)
        require.NoError(rt, err)
        var got technology.TechEffect
        require.NoError(rt, yaml.Unmarshal(data, &got))
        assert.Equal(rt, effect.Type, got.Type)
    })
}

// REQ-T14 (property): TechnologyDef with valid fields passes Validate and round-trips YAML
func TestProperty_TechnologyDef_YAMLRoundTrip(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        tradition := rapid.SampledFrom([]technology.Tradition{
            technology.TraditionTechnical,
            technology.TraditionFanaticDoctrine,
            technology.TraditionNeural,
            technology.TraditionBioSynthetic,
        }).Draw(rt, "tradition").(technology.Tradition)

        level := rapid.IntRange(1, 10).Draw(rt, "level").(int)
        usageType := rapid.SampledFrom([]technology.UsageType{
            technology.UsageCantrip,
            technology.UsagePrepared,
            technology.UsageSpontaneous,
            technology.UsageInnate,
        }).Draw(rt, "usageType").(technology.UsageType)
        rng := rapid.SampledFrom([]technology.Range{
            technology.RangeSelf,
            technology.RangeMelee,
            technology.RangeRanged,
            technology.RangeZone,
        }).Draw(rt, "range").(technology.Range)
        targets := rapid.SampledFrom([]technology.Targets{
            technology.TargetsSingle,
            technology.TargetsAllEnemies,
            technology.TargetsAllAllies,
            technology.TargetsZone,
        }).Draw(rt, "targets").(technology.Targets)

        def := &technology.TechnologyDef{
            ID:        "test_tech",
            Name:      "Test Tech",
            Tradition: tradition,
            Level:     level,
            UsageType: usageType,
            Range:     rng,
            Targets:   targets,
            Duration:  "instant",
            Effects:   []technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "energy"}},
        }
        require.NoError(rt, def.Validate())

        data, err := yaml.Marshal(def)
        require.NoError(rt, err)
        var got technology.TechnologyDef
        require.NoError(rt, yaml.Unmarshal(data, &got))
        assert.Equal(rt, def.Tradition, got.Tradition)
        assert.Equal(rt, def.Level, got.Level)
    })
}

// helpers
func validDef() *technology.TechnologyDef {
    return &technology.TechnologyDef{
        ID:        "test",
        Name:      "Test",
        Tradition: technology.TraditionTechnical,
        Level:     1,
        UsageType: technology.UsagePrepared,
        Range:     technology.RangeRanged,
        Targets:   technology.TargetsSingle,
        Duration:  "instant",
        Effects:   []technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "energy"}},
    }
}

func minimalEffect(t technology.EffectType) technology.TechEffect {
    switch t {
    case technology.EffectDamage:
        return technology.TechEffect{Type: t, Dice: "1d6", DamageType: "energy"}
    case technology.EffectHeal:
        return technology.TechEffect{Type: t, Dice: "1d6"}
    case technology.EffectDrain:
        return technology.TechEffect{Type: t, Dice: "1d4", Resource: "hp"}
    case technology.EffectCondition:
        return technology.TechEffect{Type: t, ConditionID: "stunned"}
    case technology.EffectSkillCheck:
        return technology.TechEffect{Type: t, Skill: "perception", DC: 15}
    case technology.EffectMovement:
        return technology.TechEffect{Type: t, Distance: 10, Direction: "away"}
    case technology.EffectZone:
        return technology.TechEffect{Type: t, Radius: 10}
    case technology.EffectSummon:
        return technology.TechEffect{Type: t, NPCID: "drone", SummonRounds: 3}
    case technology.EffectUtility:
        return technology.TechEffect{Type: t, UtilityType: "hack"}
    default:
        return technology.TechEffect{Type: t}
    }
}
```

Add import block at top of test file:
```go
import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/technology"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gopkg.in/yaml.v3"
    "pgregory.net/rapid"
)
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud/.worktrees/aoe-and-zone-effects
go test ./internal/game/technology/... 2>&1 | head -20
```

Expected: compile error — package does not exist yet

- [ ] **Step 3: Implement model.go**

```go
// internal/game/technology/model.go
package technology

import "fmt"

// Tradition represents the four technology traditions, analogous to PF2E spell traditions.
type Tradition string

// UsageType describes how a technology is prepared and cast.
type UsageType string

// Range describes the reach of a technology.
type Range string

// Targets describes the targeting pattern of a technology.
type Targets string

// EffectType discriminates TechEffect variants.
type EffectType string

const (
    TraditionTechnical       Tradition = "technical"
    TraditionFanaticDoctrine Tradition = "fanatic_doctrine"
    TraditionNeural          Tradition = "neural"
    TraditionBioSynthetic    Tradition = "bio_synthetic"
)

var validTraditions = map[Tradition]bool{
    TraditionTechnical: true, TraditionFanaticDoctrine: true,
    TraditionNeural: true, TraditionBioSynthetic: true,
}

const (
    UsageCantrip     UsageType = "cantrip"
    UsagePrepared    UsageType = "prepared"
    UsageSpontaneous UsageType = "spontaneous"
    UsageInnate      UsageType = "innate"
)

var validUsageTypes = map[UsageType]bool{
    UsageCantrip: true, UsagePrepared: true,
    UsageSpontaneous: true, UsageInnate: true,
}

const (
    RangeSelf   Range = "self"
    RangeMelee  Range = "melee"
    RangeRanged Range = "ranged"
    RangeZone   Range = "zone"
)

var validRanges = map[Range]bool{
    RangeSelf: true, RangeMelee: true, RangeRanged: true, RangeZone: true,
}

const (
    TargetsSingle     Targets = "single"
    TargetsAllEnemies Targets = "all_enemies"
    TargetsAllAllies  Targets = "all_allies"
    TargetsZone       Targets = "zone"
)

var validTargets = map[Targets]bool{
    TargetsSingle: true, TargetsAllEnemies: true,
    TargetsAllAllies: true, TargetsZone: true,
}

const (
    EffectDamage     EffectType = "damage"
    EffectHeal       EffectType = "heal"
    EffectCondition  EffectType = "condition"
    EffectSkillCheck EffectType = "skill_check"
    EffectMovement   EffectType = "movement"
    EffectZone       EffectType = "zone"
    EffectSummon     EffectType = "summon"
    EffectUtility    EffectType = "utility"
    EffectDrain      EffectType = "drain"
)

var validEffectTypes = map[EffectType]bool{
    EffectDamage: true, EffectHeal: true, EffectCondition: true,
    EffectSkillCheck: true, EffectMovement: true, EffectZone: true,
    EffectSummon: true, EffectUtility: true, EffectDrain: true,
}

// TechEffect is one effect within a technology, discriminated by Type.
// Only fields relevant to the given Type need be set; others are zero-valued.
//
// Type-to-required-fields mapping:
//
//	damage      — Dice or Amount (at least one); DamageType
//	heal        — Dice or Amount (at least one)
//	drain       — Dice or Amount (at least one); Resource ("hp" | "ap")
//	condition   — ConditionID; Duration is optional (overrides parent duration if set)
//	skill_check — Skill; DC (must be > 0; not omitted even if zero — use explicit value)
//	movement    — Distance (> 0); Direction ("toward" | "away" | "teleport")
//	zone        — Radius (> 0)
//	summon      — NPCID; SummonRounds (> 0)
//	utility     — UtilityType ("unlock" | "reveal" | "hack")
type TechEffect struct {
    Type EffectType `yaml:"type"`

    // damage / heal / drain
    Dice       string `yaml:"dice,omitempty"`
    DamageType string `yaml:"damage_type,omitempty"`
    Amount     int    `yaml:"amount,omitempty"`
    Resource   string `yaml:"resource,omitempty"` // drain: "hp" | "ap"

    // condition
    ConditionID string `yaml:"condition_id,omitempty"`
    Duration    string `yaml:"duration,omitempty"` // overrides parent duration if set

    // skill_check — DC must be > 0; yaml tag does NOT use omitempty so zero is preserved
    Skill string `yaml:"skill,omitempty"`
    DC    int    `yaml:"dc"`

    // movement
    Distance  int    `yaml:"distance,omitempty"`  // feet; > 0
    Direction string `yaml:"direction,omitempty"` // toward | away | teleport

    // zone
    Radius int `yaml:"radius,omitempty"` // feet; > 0

    // summon
    NPCID        string `yaml:"npc_id,omitempty"`
    SummonRounds int    `yaml:"summon_rounds,omitempty"` // > 0

    // utility
    UtilityType string `yaml:"utility_type,omitempty"` // unlock | reveal | hack
}

// TechnologyDef defines a single technology — the game's analog of a PF2E spell.
//
// Precondition: ID, Name, Tradition, Level (1–10), UsageType, Range, Targets,
//
//	Duration, and at least one Effect must all be set.
//
// Postcondition: Validate() returns nil iff all required fields are present and valid.
type TechnologyDef struct {
    ID          string    `yaml:"id"`
    Name        string    `yaml:"name"`
    Description string    `yaml:"description,omitempty"`
    Tradition   Tradition `yaml:"tradition"`
    Level       int       `yaml:"level"`
    UsageType   UsageType `yaml:"usage_type"`
    ActionCost  int       `yaml:"action_cost"`
    Range       Range     `yaml:"range"`
    Targets     Targets   `yaml:"targets"`
    Duration    string    `yaml:"duration"`
    SaveType    string    `yaml:"save_type,omitempty"`
    SaveDC      int       `yaml:"save_dc,omitempty"`
    Effects     []TechEffect `yaml:"effects"`
    AmpedLevel  int          `yaml:"amped_level,omitempty"`
    AmpedEffects []TechEffect `yaml:"amped_effects,omitempty"`
}

// Validate returns an error if any required field is missing or invalid.
// Precondition: t is not nil.
// Postcondition: returns nil iff all required fields are present and valid.
func (t *TechnologyDef) Validate() error {
    if t.ID == "" {
        return fmt.Errorf("id must not be empty")
    }
    if t.Name == "" {
        return fmt.Errorf("name must not be empty")
    }
    if !validTraditions[t.Tradition] {
        return fmt.Errorf("unknown tradition %q", t.Tradition)
    }
    if t.Level < 1 || t.Level > 10 {
        return fmt.Errorf("level %d out of range [1,10]", t.Level)
    }
    if !validUsageTypes[t.UsageType] {
        return fmt.Errorf("unknown usage_type %q", t.UsageType)
    }
    if !validRanges[t.Range] {
        return fmt.Errorf("unknown range %q", t.Range)
    }
    if !validTargets[t.Targets] {
        return fmt.Errorf("unknown targets %q", t.Targets)
    }
    if t.Duration == "" {
        return fmt.Errorf("duration must not be empty")
    }
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
    if t.SaveType != "" && t.SaveDC == 0 {
        return fmt.Errorf("save_dc must be > 0 when save_type is set")
    }
    return nil
}

func validateEffect(e TechEffect, idx int) error {
    if !validEffectTypes[e.Type] {
        return fmt.Errorf("effects[%d]: unknown type %q", idx, e.Type)
    }
    if e.Type == EffectSkillCheck {
        if e.Skill == "" {
            return fmt.Errorf("effects[%d]: skill_check effect requires skill", idx)
        }
        if e.DC == 0 {
            return fmt.Errorf("effects[%d]: skill_check effect requires dc > 0", idx)
        }
    }
    return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/game/technology/... -v -run TestValidate 2>&1
go test ./internal/game/technology/... -v -run TestProperty 2>&1
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/technology/model.go internal/game/technology/model_test.go
git commit -m "feat(technology): TechnologyDef model, enums, and Validate"
```

---

### Task 2: Registry — failing tests

**Files:**
- Create: `internal/game/technology/registry.go`
- Create: `internal/game/technology/registry_test.go`
- Create: `internal/game/technology/testdata/` (fixture YAMLs)

- [ ] **Step 1: Create fixture YAML files for tests**

Create directory `internal/game/technology/testdata/`:

`internal/game/technology/testdata/technical_basic.yaml`:
```yaml
id: neural_shock_fixture
name: Neural Shock Fixture
tradition: technical
level: 2
usage_type: prepared
action_cost: 2
range: ranged
targets: single
duration: instant
effects:
  - type: damage
    dice: 2d6
    damage_type: energy
```

`internal/game/technology/testdata/neural_basic.yaml`:
```yaml
id: mind_spike_fixture
name: Mind Spike Fixture
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
effects:
  - type: condition
    condition_id: confused
```

`internal/game/technology/testdata/fanatic_basic.yaml`:
```yaml
id: battle_fervor_fixture
name: Battle Fervor Fixture
tradition: fanatic_doctrine
level: 1
usage_type: spontaneous
action_cost: 1
range: self
targets: single
duration: rounds:3
effects:
  - type: condition
    condition_id: battle_fervor_active
```

`internal/game/technology/testdata/bio_basic.yaml`:
```yaml
id: acid_spray_fixture
name: Acid Spray Fixture
tradition: bio_synthetic
level: 1
usage_type: prepared
action_cost: 2
range: zone
targets: all_enemies
duration: instant
effects:
  - type: damage
    dice: 1d6
    damage_type: acid
```

`internal/game/technology/testdata/neural_level2.yaml`:
```yaml
id: arc_thought_fixture
name: Arc Thought Fixture
tradition: neural
level: 2
usage_type: innate
action_cost: 1
range: ranged
targets: single
duration: instant
effects:
  - type: damage
    dice: 1d8
    damage_type: neural
```

- [ ] **Step 2: Write failing registry tests**

```go
// internal/game/technology/registry_test.go
package technology_test

import (
    "path/filepath"
    "runtime"
    "testing"

    "github.com/cory-johannsen/mud/internal/game/technology"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func testdataDir() string {
    _, file, _, _ := runtime.Caller(0)
    return filepath.Join(filepath.Dir(file), "testdata")
}

// REQ-T7: Load with one valid YAML per tradition returns registry with all four traditions
func TestLoad_AllFourTraditions(t *testing.T) {
    reg, err := technology.Load(testdataDir())
    require.NoError(t, err)
    for _, trad := range []technology.Tradition{
        technology.TraditionTechnical,
        technology.TraditionFanaticDoctrine,
        technology.TraditionNeural,
        technology.TraditionBioSynthetic,
    } {
        defs := reg.ByTradition(trad)
        assert.NotEmpty(t, defs, "expected at least one def for tradition %s", trad)
    }
}

// REQ-T8: Get by ID returns correct def; unknown ID returns (nil, false)
func TestGet_ByID(t *testing.T) {
    reg, err := technology.Load(testdataDir())
    require.NoError(t, err)

    def, ok := reg.Get("neural_shock_fixture")
    require.True(t, ok)
    assert.Equal(t, "neural_shock_fixture", def.ID)
    assert.Equal(t, technology.TraditionTechnical, def.Tradition)

    _, ok = reg.Get("does_not_exist")
    assert.False(t, ok)
}

// REQ-T9: ByTradition returns results sorted by level ascending then ID ascending
func TestByTradition_SortOrder(t *testing.T) {
    reg, err := technology.Load(testdataDir())
    require.NoError(t, err)

    defs := reg.ByTradition(technology.TraditionNeural)
    require.GreaterOrEqual(t, len(defs), 2)
    for i := 1; i < len(defs); i++ {
        prev, cur := defs[i-1], defs[i]
        if prev.Level == cur.Level {
            assert.LessOrEqual(t, prev.ID, cur.ID, "IDs should be sorted ascending at same level")
        } else {
            assert.Less(t, prev.Level, cur.Level, "levels should be sorted ascending")
        }
    }
}

// REQ-T10: ByTraditionAndLevel returns only matching defs sorted by ID
func TestByTraditionAndLevel(t *testing.T) {
    reg, err := technology.Load(testdataDir())
    require.NoError(t, err)

    defs := reg.ByTraditionAndLevel(technology.TraditionNeural, 1)
    for _, d := range defs {
        assert.Equal(t, technology.TraditionNeural, d.Tradition)
        assert.Equal(t, 1, d.Level)
    }
    // level 2 neural should not appear
    for _, d := range defs {
        assert.NotEqual(t, 2, d.Level)
    }
}

// REQ-T11: ByUsageType returns all matching defs across traditions sorted tradition > level > ID
func TestByUsageType(t *testing.T) {
    reg, err := technology.Load(testdataDir())
    require.NoError(t, err)

    defs := reg.ByUsageType(technology.UsageSpontaneous)
    for _, d := range defs {
        assert.Equal(t, technology.UsageSpontaneous, d.UsageType)
    }
    // Verify sort: tradition ascending (lex), then level, then ID
    for i := 1; i < len(defs); i++ {
        prev, cur := defs[i-1], defs[i]
        tradCmp := string(prev.Tradition) <= string(cur.Tradition)
        if string(prev.Tradition) == string(cur.Tradition) {
            if prev.Level == cur.Level {
                assert.LessOrEqual(t, prev.ID, cur.ID)
            } else {
                assert.LessOrEqual(t, prev.Level, cur.Level)
            }
        } else {
            assert.True(t, tradCmp)
        }
    }
}

// REQ-T12: Load with malformed YAML returns error containing file path
func TestLoad_MalformedYAML(t *testing.T) {
    dir := t.TempDir()
    // write a file with an unknown field (KnownFields strict mode)
    err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("id: test\nunknown_field: oops\n"), 0o644)
    require.NoError(t, err)
    _, err = technology.Load(dir)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "bad.yaml")
}
```

Add `"os"` to the import block.

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/game/technology/... -v -run TestLoad -run TestGet -run TestBy 2>&1
```

Expected: compile error (registry.go not yet created)

- [ ] **Step 4: Implement registry.go**

```go
// internal/game/technology/registry.go
package technology

import (
    "bytes"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "gopkg.in/yaml.v3"
)

// Registry holds all loaded TechnologyDefs, indexed for fast lookup.
//
// Precondition: Load must be called with a valid, existing directory path.
// Postcondition: Load is fail-fast — it returns on the first invalid or unreadable
//
//	file, wrapping the file path in the error. All successfully loaded defs prior
//	to the error are discarded. An empty directory is not an error.
type Registry struct {
    byID        map[string]*TechnologyDef
    byTradition map[Tradition][]*TechnologyDef
    byLevel     map[int][]*TechnologyDef
    byUsage     map[UsageType][]*TechnologyDef
}

// Load walks dir recursively, parses all YAML files, validates each def,
// and returns a populated Registry. Returns an error on the first invalid
// file; the error message includes the file path.
func Load(dir string) (*Registry, error) {
    r := &Registry{
        byID:        make(map[string]*TechnologyDef),
        byTradition: make(map[Tradition][]*TechnologyDef),
        byLevel:     make(map[int][]*TechnologyDef),
        byUsage:     make(map[UsageType][]*TechnologyDef),
    }
    err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return fmt.Errorf("walking %q: %w", path, err)
        }
        if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
            return nil
        }
        data, err := os.ReadFile(path)
        if err != nil {
            return fmt.Errorf("reading %q: %w", path, err)
        }
        var def TechnologyDef
        dec := yaml.NewDecoder(bytes.NewReader(data))
        dec.KnownFields(true)
        if err := dec.Decode(&def); err != nil {
            return fmt.Errorf("parsing %q: %w", path, err)
        }
        if err := def.Validate(); err != nil {
            return fmt.Errorf("validating %q: %w", path, err)
        }
        r.byID[def.ID] = &def
        r.byTradition[def.Tradition] = append(r.byTradition[def.Tradition], &def)
        r.byLevel[def.Level] = append(r.byLevel[def.Level], &def)
        r.byUsage[def.UsageType] = append(r.byUsage[def.UsageType], &def)
        return nil
    })
    if err != nil {
        return nil, err
    }
    return r, nil
}

// Get returns the TechnologyDef for id, or (nil, false) if not found.
func (r *Registry) Get(id string) (*TechnologyDef, bool) {
    d, ok := r.byID[id]
    return d, ok
}

// All returns all loaded TechnologyDefs sorted by tradition ascending (lexicographic:
// bio_synthetic < fanatic_doctrine < neural < technical), then level ascending, then ID ascending.
func (r *Registry) All() []*TechnologyDef {
    out := make([]*TechnologyDef, 0, len(r.byID))
    for _, d := range r.byID {
        out = append(out, d)
    }
    sortTechDefs(out)
    return out
}

// ByTradition returns all technologies of the given tradition,
// sorted by level ascending, then ID ascending.
func (r *Registry) ByTradition(t Tradition) []*TechnologyDef {
    out := make([]*TechnologyDef, len(r.byTradition[t]))
    copy(out, r.byTradition[t])
    sort.Slice(out, func(i, j int) bool {
        if out[i].Level != out[j].Level {
            return out[i].Level < out[j].Level
        }
        return out[i].ID < out[j].ID
    })
    return out
}

// ByTraditionAndLevel returns all technologies of the given tradition at a specific level,
// sorted by ID ascending.
func (r *Registry) ByTraditionAndLevel(t Tradition, level int) []*TechnologyDef {
    var out []*TechnologyDef
    for _, d := range r.byTradition[t] {
        if d.Level == level {
            out = append(out, d)
        }
    }
    sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
    return out
}

// ByUsageType returns all technologies of the given usage type,
// sorted by tradition ascending (lexicographic: bio_synthetic < fanatic_doctrine < neural < technical),
// then level ascending, then ID ascending.
func (r *Registry) ByUsageType(u UsageType) []*TechnologyDef {
    out := make([]*TechnologyDef, len(r.byUsage[u]))
    copy(out, r.byUsage[u])
    sortTechDefs(out)
    return out
}

func sortTechDefs(defs []*TechnologyDef) {
    sort.Slice(defs, func(i, j int) bool {
        ti, tj := string(defs[i].Tradition), string(defs[j].Tradition)
        if ti != tj {
            return ti < tj
        }
        if defs[i].Level != defs[j].Level {
            return defs[i].Level < defs[j].Level
        }
        return defs[i].ID < defs[j].ID
    })
}
```

- [ ] **Step 5: Run all technology tests**

```bash
go test ./internal/game/technology/... -v 2>&1
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/game/technology/registry.go internal/game/technology/registry_test.go internal/game/technology/testdata/
git commit -m "feat(technology): Registry with Load, Get, All, ByTradition, ByTraditionAndLevel, ByUsageType"
```

---

## Chunk 2: Seed Content and GameServiceServer Wiring

### Task 3: Seed YAML content

**Files:**
- Create: `content/technologies/technical/neural_shock.yaml`
- Create: `content/technologies/fanatic_doctrine/battle_fervor.yaml`
- Create: `content/technologies/neural/mind_spike.yaml`
- Create: `content/technologies/bio_synthetic/acid_spray.yaml`

- [ ] **Step 1: Create seed content files**

`content/technologies/technical/neural_shock.yaml`:
```yaml
id: neural_shock
name: Neural Shock
description: A targeted electromagnetic pulse that disrupts the target's nervous system.
tradition: technical
level: 1
usage_type: prepared
action_cost: 2
range: ranged
targets: single
duration: instant
save_type: reflex
save_dc: 15
effects:
  - type: damage
    dice: 2d6
    damage_type: energy
  - type: condition
    condition_id: stunned
    duration: rounds:1
```

`content/technologies/fanatic_doctrine/battle_fervor.yaml`:
```yaml
id: battle_fervor
name: Battle Fervor
description: A surge of doctrinal conviction that sharpens combat focus.
tradition: fanatic_doctrine
level: 1
usage_type: spontaneous
action_cost: 1
range: self
targets: single
duration: rounds:3
effects:
  - type: condition
    condition_id: battle_fervor_active
```

`content/technologies/neural/mind_spike.yaml`:
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
save_type: will
save_dc: 15
effects:
  - type: damage
    dice: 2d6
    damage_type: neural
  - type: condition
    condition_id: confused
    duration: rounds:1
amped_level: 3
amped_effects:
  - type: damage
    dice: 4d6
    damage_type: neural
  - type: condition
    condition_id: confused
    duration: rounds:2
```

`content/technologies/bio_synthetic/acid_spray.yaml`:
```yaml
id: acid_spray
name: Acid Spray
description: A bio-synthetic secretion that coats all nearby enemies in corrosive fluid.
tradition: bio_synthetic
level: 1
usage_type: prepared
action_cost: 2
range: zone
targets: all_enemies
duration: instant
save_type: reflex
save_dc: 14
effects:
  - type: damage
    dice: 1d6
    damage_type: acid
  - type: zone
    radius: 10
```

- [ ] **Step 2: Verify seed files load via the registry**

```bash
cd /home/cjohannsen/src/mud/.worktrees/aoe-and-zone-effects
go run -v ./cmd/gameserver/... --help 2>&1 | head -5
# OR write a quick smoke test:
cat > /tmp/smoke_tech.go << 'EOF'
//go:build ignore
package main

import (
    "fmt"
    "github.com/cory-johannsen/mud/internal/game/technology"
)

func main() {
    reg, err := technology.Load("content/technologies")
    if err != nil {
        panic(err)
    }
    for _, d := range reg.All() {
        fmt.Printf("%s (%s L%d)\n", d.ID, d.Tradition, d.Level)
    }
}
EOF
go run /tmp/smoke_tech.go
```

Expected: four lines printed (acid_spray, battle_fervor, mind_spike, neural_shock)

- [ ] **Step 3: Commit seed content**

```bash
git add content/technologies/
git commit -m "feat(technology): seed content — one technology per tradition"
```

---

### Task 4: Wire techRegistry into GameServiceServer

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`

- [ ] **Step 1: Read the existing GameServiceServer struct and NewGameServiceServer signature**

Read `internal/gameserver/grpc_service.go` lines around the struct definition and `NewGameServiceServer` to understand the existing wiring pattern. Confirm the `condRegistry` field and how it is loaded.

- [ ] **Step 2: Add techRegistry field to GameServiceServer struct**

In `internal/gameserver/grpc_service.go`, add the field after `condRegistry`:

```go
techRegistry *technology.Registry
```

Also add the import:
```go
"github.com/cory-johannsen/mud/internal/game/technology"
```

- [ ] **Step 3: Add techRegistry parameter to NewGameServiceServer**

Add parameter after `condRegistry *condition.Registry`:
```go
techRegistry *technology.Registry,
```

Wire into the struct literal:
```go
techRegistry: techRegistry,
```

- [ ] **Step 4: Update cmd/gameserver/main.go to load techRegistry**

Find where `condRegistry` is loaded in `main.go`. Add analogous loading for tech registry. A missing `content/technologies/` directory must be non-fatal (empty registry instead of fatal error).

```go
techReg, err := technology.Load(*techContentDir)
if err != nil && !os.IsNotExist(errors.Unwrap(err)) {
    // If the directory simply doesn't exist, use empty registry
    log.Printf("WARN: technology content dir %q: %v — starting with empty tech registry", *techContentDir, err)
    techReg = &technology.Registry{}  // but Registry must export a constructor for this
}
```

**Note:** `Registry` as written has unexported fields and no exported empty constructor. Add `NewRegistry()` to `registry.go`:

```go
// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
    return &Registry{
        byID:        make(map[string]*TechnologyDef),
        byTradition: make(map[Tradition][]*TechnologyDef),
        byLevel:     make(map[int][]*TechnologyDef),
        byUsage:     make(map[UsageType][]*TechnologyDef),
    }
}
```

Then in `main.go`:

```go
techReg, err := technology.Load(*techContentDir)
if err != nil {
    if os.IsNotExist(pathErr(err)) {
        log.Printf("WARN: technology content dir %q not found — starting with empty tech registry", *techContentDir)
        techReg = technology.NewRegistry()
    } else {
        log.Fatalf("loading technology content: %v", err)
    }
}
```

Add a helper inline or use `errors.As` with `*fs.PathError`:

```go
func pathErr(err error) error {
    var pe *os.PathError
    if errors.As(err, &pe) {
        return pe.Err
    }
    return err
}
```

Look at how `*techContentDir` flag is defined — follow the same pattern as `condContentDir`. Add a flag:

```go
techContentDir = flag.String("tech-content-dir", "content/technologies", "path to technology YAML content")
```

Pass `techReg` to `NewGameServiceServer`.

- [ ] **Step 5: Build and run tests**

```bash
go build ./... 2>&1
go test ./internal/gameserver/... 2>&1 | tail -5
go test ./internal/game/technology/... 2>&1
```

Expected: all PASS, no compile errors

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/game/technology/registry.go cmd/gameserver/main.go
git commit -m "feat(technology): wire techRegistry into GameServiceServer; add NewRegistry(); add --tech-content-dir flag"
```

---

### Task 5: Final verification

- [ ] **Step 1: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all PASS, 0 failures

- [ ] **Step 2: Verify seed content loads at server startup**

```bash
go run ./cmd/gameserver/... --help 2>&1 | grep tech
```

Expected: `--tech-content-dir` flag visible

- [ ] **Step 3: Done**

All 16 REQ-T requirements are satisfied:
- REQ-T1 through REQ-T6, REQ-T15, REQ-T16: model_test.go (Task 1)
- REQ-T7 through REQ-T12: registry_test.go (Task 2)
- REQ-T13, REQ-T14: property tests in model_test.go (Task 1)
- Seed content: four YAML files (Task 3)
- GameServiceServer wiring: techRegistry field + flag (Task 4)
