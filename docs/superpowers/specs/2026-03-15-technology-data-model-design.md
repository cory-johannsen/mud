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

`package technology` — files at `internal/game/technology/`.

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
//   damage      — Dice or Amount (at least one); DamageType
//   heal        — Dice or Amount (at least one)
//   drain       — Dice or Amount (at least one); Resource ("hp" | "ap")
//   condition   — ConditionID; Duration is optional (overrides parent duration if set)
//   skill_check — Skill; DC (must be > 0; not omitted even if zero — use explicit value)
//   movement    — Distance (> 0); Direction ("toward" | "away" | "teleport")
//   zone        — Radius (> 0)
//   summon      — NPCID; SummonRounds (> 0)
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
```

**Note on `DC`:** The `dc` field uses no `omitempty` tag so that a zero value is preserved in YAML round-trips. `Validate` requires `DC > 0` for `skill_check` effects.

### TechnologyDef

```go
// TechnologyDef defines a single technology — the game's analog of a PF2E spell.
//
// Precondition: ID, Name, Tradition, Level (1–10), UsageType, Range, Targets,
//   Duration, and at least one Effect must all be set.
// Postcondition: Validate() returns nil iff all required fields are present and valid.
type TechnologyDef struct {
    ID          string    `yaml:"id"`
    Name        string    `yaml:"name"`
    Description string    `yaml:"description,omitempty"` // optional; empty is allowed
    Tradition   Tradition `yaml:"tradition"`
    Level       int       `yaml:"level"`       // 1–10
    UsageType   UsageType `yaml:"usage_type"`  // cantrip | prepared | spontaneous | innate
    ActionCost  int       `yaml:"action_cost"` // AP cost in combat; 0 = free action
    Range       Range     `yaml:"range"`       // self | melee | ranged | zone
    Targets     Targets   `yaml:"targets"`     // single | all_enemies | all_allies | zone
    Duration    string    `yaml:"duration"`    // instant | rounds:N | encounter | permanent
    SaveType    string    `yaml:"save_type,omitempty"` // reflex | fortitude | will | grit | hustle | cool
    SaveDC      int       `yaml:"save_dc,omitempty"`   // base DC; scaled at resolution time

    // Effects is the ordered list of effects applied when the technology is used.
    Effects []TechEffect `yaml:"effects"`

    // AmpedLevel is the technology slot level at which the amped version activates.
    // Zero means the technology cannot be amped. When a player uses this technology
    // at slot level >= AmpedLevel, AmpedEffects replaces Effects.
    AmpedLevel int `yaml:"amped_level,omitempty"`

    // AmpedEffects replaces Effects when the technology is used at AmpedLevel or higher.
    AmpedEffects []TechEffect `yaml:"amped_effects,omitempty"`
}

// Validate returns an error if any required field is missing or invalid.
func (t *TechnologyDef) Validate() error
```

### Validation Rules

- `ID` must be non-empty.
- `Name` must be non-empty.
- `Tradition` must be one of the four defined constants.
- `Level` must be in [1, 10].
- `UsageType` must be one of the four defined constants.
- `Range` must be one of the four defined constants.
- `Targets` must be one of the four defined constants.
- `Duration` must be non-empty.
- `Effects` must have at least one entry.
- Each `TechEffect.Type` must be one of the nine defined constants.
- For `skill_check` effects: `Skill` must be non-empty and `DC` must be > 0.
- If `AmpedEffects` is non-empty, `AmpedLevel` must be > 0.
- If `AmpedLevel` > 0, `AmpedEffects` must be non-empty.
- If `SaveType` is non-empty, `SaveDC` must be > 0.

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
// Precondition: Load must be called with a valid, existing directory path.
// Postcondition: Load is fail-fast — it returns on the first invalid or unreadable
//   file, wrapping the file path in the error. All successfully loaded defs prior
//   to the error are discarded. An empty directory is not an error.
type Registry struct {
    byID        map[string]*TechnologyDef
    byTradition map[Tradition][]*TechnologyDef
    byLevel     map[int][]*TechnologyDef
    byUsage     map[UsageType][]*TechnologyDef
}

// Load walks dir recursively, parses all YAML files, validates each def,
// and returns a populated Registry. Returns an error on the first invalid
// file; the error message includes the file path.
func Load(dir string) (*Registry, error)

// Get returns the TechnologyDef for id, or (nil, false) if not found.
func (r *Registry) Get(id string) (*TechnologyDef, bool)

// All returns all loaded TechnologyDefs sorted by tradition ascending (lexicographic:
// bio_synthetic < fanatic_doctrine < neural < technical), then level ascending, then ID ascending.
func (r *Registry) All() []*TechnologyDef

// ByTradition returns all technologies of the given tradition,
// sorted by level ascending, then ID ascending.
func (r *Registry) ByTradition(t Tradition) []*TechnologyDef

// ByTraditionAndLevel returns all technologies of the given tradition at a specific level,
// sorted by ID ascending.
func (r *Registry) ByTraditionAndLevel(t Tradition, level int) []*TechnologyDef

// ByUsageType returns all technologies of the given usage type,
// sorted by tradition ascending (lexicographic: bio_synthetic < fanatic_doctrine < neural < technical),
// then level ascending, then ID ascending.
func (r *Registry) ByUsageType(u UsageType) []*TechnologyDef
```

### Wiring into GameServiceServer

`internal/gameserver/grpc_service.go` — add field to `GameServiceServer` struct:

```go
techRegistry *technology.Registry
```

In `NewGameServiceServer`, load the registry from `content/technologies/` (resolved relative to the binary's working directory, consistent with how other content is loaded). A missing `content/technologies/` directory is treated as an empty registry (not a fatal error) to allow the server to start before any content is authored.

---

## Feature 4: Seed Content

Four seed technology YAML files — one per tradition — are created as part of this task. All use level 1.

### `content/technologies/technical/neural_shock.yaml`
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

### `content/technologies/fanatic_doctrine/battle_fervor.yaml`
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

### `content/technologies/neural/mind_spike.yaml`
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

### `content/technologies/bio_synthetic/acid_spray.yaml`
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

**Note:** `stunned` and `battle_fervor_active` condition IDs referenced by seed content do not need to exist in `content/conditions/` for this sub-project — the registry validates `TechEffect.Type` and field presence, not that referenced condition IDs resolve. Condition ID resolution is a concern of the resolution engine (future sub-project).

---

## Testing

All tests in `internal/game/technology/` (`package technology_test`) using TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-T1**: `Validate` rejects unknown `Tradition` string.
- **REQ-T2**: `Validate` rejects `Level` < 1 or > 10.
- **REQ-T3**: `Validate` rejects empty `Effects` slice.
- **REQ-T4**: `Validate` rejects `AmpedEffects` non-empty with `AmpedLevel == 0`.
- **REQ-T5**: `Validate` rejects `AmpedLevel > 0` with empty `AmpedEffects`.
- **REQ-T6**: `Validate` rejects `skill_check` effect with `DC == 0`.
- **REQ-T7**: `Load` with a fixture directory containing one valid YAML per tradition returns a registry with all four traditions present via `ByTradition`.
- **REQ-T8**: `Get` by ID returns the correct `TechnologyDef`; `Get` for unknown ID returns `(nil, false)`.
- **REQ-T9**: `ByTradition` returns results sorted by level ascending, then ID ascending.
- **REQ-T10**: `ByTraditionAndLevel` returns only defs matching both tradition and level; results sorted by ID ascending.
- **REQ-T11**: `ByUsageType` returns all defs matching the usage type across all traditions; results sorted by tradition, level, ID.
- **REQ-T12**: `Load` with a malformed YAML file returns an error containing the file path; no registry returned.
- **REQ-T13** (property): For any `EffectType`, a `TechEffect` with that type and valid required fields marshals to YAML and unmarshals back with all set fields preserved.
- **REQ-T14** (property): For any combination of valid `Tradition`, `Level` [1–10], `UsageType`, `Range`, `Targets`, a `TechnologyDef` with those fields, non-empty `Duration`, and one valid effect passes `Validate()` and round-trips through YAML without data loss.
- **REQ-T15**: `Validate` rejects a `TechnologyDef` with non-empty `SaveType` and `SaveDC == 0`.
- **REQ-T16**: A `condition` effect with no `Duration` field (duration omitted) is valid; the parent `TechnologyDef.Duration` serves as the fallback. `Validate` accepts such a def; `Load` loads it without error.

---

## Constraints

- Technologies are **data only** in this sub-project — no resolution logic, no player assignment, no slot tracking.
- `GameServiceServer` wiring adds the field and loads the registry at startup; no RPC handlers are added.
- A missing `content/technologies/` directory is not a fatal error — the server starts with an empty registry.
- Seed content covers one technology per tradition; it is not exhaustive.
- Referenced condition IDs (e.g. `stunned`, `battle_fervor_active`) are not validated against `content/conditions/` in this sub-project.
