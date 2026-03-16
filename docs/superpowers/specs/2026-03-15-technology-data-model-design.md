# Technology Data Model — Design Spec

**Date:** 2026-03-15

---

## Goal

Introduce `TechnologyDef` as a new top-level type representing a technology (the game's analog of a PF2E spell). This sub-project covers only the data model, registry, YAML loader, and seed content. Slot tracking, preparation mechanics, and usage limits are out of scope — those are separate sub-projects.

---

## Context

Gunchete has no magic. PF2E spell traditions map to technology types:

| PF2E Tradition | Gunchete Tradition |
|----------------|--------------------|
| Arcane         | Technical          |
| Divine         | Fanatic Doctrine   |
| Occult         | Neural             |
| Primal         | Bio-Synthetic      |

Technologies are granted to players by jobs and feats (future sub-projects). This spec builds the foundation those systems will depend on.

---

## Feature 1: Data Model

### Package

`internal/game/technology/model.go`

### String-Typed Enums

```go
type Tradition  string
type UsageType  string
type Range      string
type Targets    string
type EffectType string

const (
    TraditionTechnical       Tradition = "technical"
    TraditionFanaticDoctrine Tradition = "fanatic_doctrine"
    TraditionNeural          Tradition = "neural"
    TraditionBioSynthetic    Tradition = "bio_synthetic"
)

const (
    UsageCantrip     UsageType = "cantrip"
    UsagePrepared    UsageType = "prepared"
    UsageSpontaneous UsageType = "spontaneous"
    UsageInnate      UsageType = "innate"
)

const (
    RangeSelf   Range = "self"
    RangeMelee  Range = "melee"
    RangeRanged Range = "ranged"
    RangeZone   Range = "zone"
)

const (
    TargetsSingle     Targets = "single"
    TargetsAllEnemies Targets = "all_enemies"
    TargetsAllAllies  Targets = "all_allies"
    TargetsZone       Targets = "zone"
)

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
```

### TechEffect

```go
// TechEffect is one effect within a technology, discriminated by Type.
// Only fields relevant to the given Type need be set; others are zero-valued.
//
// Type-to-required-fields mapping:
//   damage      — Dice or Amount; DamageType
//   heal        — Dice or Amount
//   drain       — Dice or Amount; Resource ("hp" | "ap")
//   condition   — ConditionID; Duration (optional override)
//   skill_check — Skill; DC
//   movement    — Distance; Direction ("toward" | "away" | "teleport")
//   zone        — Radius
//   summon      — NPCID; SummonRounds
//   utility     — UtilityType ("unlock" | "reveal" | "hack")
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

    // skill_check
    Skill string `yaml:"skill,omitempty"`
    DC    int    `yaml:"dc,omitempty"`

    // movement
    Distance  int    `yaml:"distance,omitempty"`  // feet
    Direction string `yaml:"direction,omitempty"` // toward | away | teleport

    // zone
    Radius int `yaml:"radius,omitempty"` // feet

    // summon
    NPCID        string `yaml:"npc_id,omitempty"`
    SummonRounds int    `yaml:"summon_rounds,omitempty"`

    // utility
    UtilityType string `yaml:"utility_type,omitempty"` // unlock | reveal | hack
}
```

### TechnologyDef

```go
// TechnologyDef defines a single technology — the game's analog of a PF2E spell.
//
// Precondition: ID, Name, Tradition, Level (1–10), UsageType, Range, Targets,
//   Duration, and at least one Effect must all be set.
// Postcondition: Validate() returns nil iff all required fields are valid.
type TechnologyDef struct {
    ID          string    `yaml:"id"`
    Name        string    `yaml:"name"`
    Description string    `yaml:"description"`
    Tradition   Tradition `yaml:"tradition"`
    Level       int       `yaml:"level"`       // 1–10
    UsageType   UsageType `yaml:"usage_type"`  // cantrip | prepared | spontaneous | innate
    ActionCost  int       `yaml:"action_cost"` // AP cost in combat; 0 = free
    Range       Range     `yaml:"range"`       // self | melee | ranged | zone
    Targets     Targets   `yaml:"targets"`     // single | all_enemies | all_allies | zone
    Duration    string    `yaml:"duration"`    // instant | rounds:N | encounter | permanent
    SaveType    string    `yaml:"save_type,omitempty"` // reflex | fortitude | will | grit | hustle | cool
    SaveDC      int       `yaml:"save_dc,omitempty"`   // base DC; scaled at resolution time

    // Effects is the ordered list of effects applied when the technology is used.
    Effects []TechEffect `yaml:"effects"`

    // AmpedLevel is the minimum technology level required to amp this technology.
    // Zero means the technology cannot be amped.
    AmpedLevel int `yaml:"amped_level,omitempty"`

    // AmpedEffects replaces Effects when the technology is used at AmpedLevel or higher.
    AmpedEffects []TechEffect `yaml:"amped_effects,omitempty"`
}

// Validate returns an error if any required field is missing or invalid.
func (t *TechnologyDef) Validate() error
```

### Validation Rules

- `ID`, `Name` must be non-empty.
- `Tradition` must be one of the four defined constants.
- `Level` must be in [1, 10].
- `UsageType` must be one of the four defined constants.
- `Range` must be one of the four defined constants.
- `Targets` must be one of the four defined constants.
- `Duration` must be non-empty.
- `Effects` must have at least one entry.
- Each `TechEffect.Type` must be one of the nine defined constants.
- If `AmpedEffects` is non-empty, `AmpedLevel` must be > 0.
- If `AmpedLevel` > 0, `AmpedEffects` must be non-empty.

---

## Feature 2: YAML Content Layout

```
content/technologies/
  technical/
    neural_shock.yaml
  fanatic_doctrine/
    battle_fervor.yaml
  neural/
    mind_spike.yaml
  bio_synthetic/
    acid_spray.yaml
```

The loader walks `content/technologies/` recursively. Subdirectory names are informational only; the `tradition` field in each YAML file is authoritative.

### Example YAML (`content/technologies/neural/mind_spike.yaml`)

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

---

## Feature 3: Registry

`internal/game/technology/registry.go`

```go
// Registry holds all loaded TechnologyDefs, indexed for fast lookup.
//
// Precondition: Load must be called with a valid directory path.
// Postcondition: All valid YAML files are loaded; invalid files return an error.
type Registry struct {
    byID        map[string]*TechnologyDef
    byTradition map[Tradition][]*TechnologyDef
    byLevel     map[int][]*TechnologyDef
    byUsage     map[UsageType][]*TechnologyDef
}

// Load walks dir recursively, parses all YAML files, validates each def,
// and returns a populated Registry.
func Load(dir string) (*Registry, error)

// Get returns the TechnologyDef for id, or (nil, false) if not found.
func (r *Registry) Get(id string) (*TechnologyDef, bool)

// ByTradition returns all technologies of the given tradition, sorted by level then ID.
func (r *Registry) ByTradition(t Tradition) []*TechnologyDef

// ByTraditionAndLevel returns all technologies of the given tradition at a specific level.
func (r *Registry) ByTraditionAndLevel(t Tradition, level int) []*TechnologyDef

// ByUsageType returns all technologies of the given usage type.
func (r *Registry) ByUsageType(u UsageType) []*TechnologyDef
```

The `Registry` is wired into `GameServiceServer` as a new field `techRegistry *technology.Registry` and loaded at startup alongside the existing registries (conditions, feats, class features, etc.).

---

## Feature 4: Seed Content

Four seed technology YAML files — one per tradition — are created as part of this task:

| File | Tradition | Level | Usage | Summary |
|------|-----------|-------|-------|---------|
| `content/technologies/technical/neural_shock.yaml` | technical | 1 | prepared | Single-target energy damage + stunned condition |
| `content/technologies/fanatic_doctrine/battle_fervor.yaml` | fanatic_doctrine | 1 | spontaneous | Self buff: damage bonus condition |
| `content/technologies/neural/mind_spike.yaml` | neural | 1 | spontaneous | Ranged damage + confused condition |
| `content/technologies/bio_synthetic/acid_spray.yaml` | bio_synthetic | 1 | prepared | Zone acid damage |

---

## Testing

All tests in `internal/game/technology/` using TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-T1**: `Validate` rejects unknown `Tradition` string.
- **REQ-T2**: `Validate` rejects `Level` < 1 or > 10.
- **REQ-T3**: `Validate` rejects empty `Effects` slice.
- **REQ-T4**: `Validate` rejects `AmpedEffects` non-empty with `AmpedLevel == 0`.
- **REQ-T5**: `Validate` rejects `AmpedLevel > 0` with empty `AmpedEffects`.
- **REQ-T6**: `Load` with a fixture directory containing one valid YAML per tradition returns a registry with all four traditions present.
- **REQ-T7**: `Get` by ID returns the correct `TechnologyDef`.
- **REQ-T8**: `ByTraditionAndLevel` returns results sorted by level then ID.
- **REQ-T9**: `Load` with a malformed YAML file returns an error; the file path is included in the error message.
- **REQ-T10** (property): For any `EffectType`, a `TechEffect` with that type marshals to YAML and unmarshals back with all set fields preserved.
- **REQ-T11** (property): For any combination of valid `Tradition`, `Level` [1–10], `UsageType`, `Range`, `Targets`, a `TechnologyDef` with those fields and one effect passes `Validate()` and round-trips through YAML without data loss.

---

## Constraints

- Technologies are **data only** in this sub-project — no resolution logic, no player assignment, no slot tracking.
- The `GameServiceServer` wiring adds the field and loads the registry at startup; no RPC handlers are added.
- Seed content covers one technology per tradition; it is not exhaustive.
