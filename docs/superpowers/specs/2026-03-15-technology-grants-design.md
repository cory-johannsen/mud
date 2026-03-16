# Technology Grants — Design Spec

**Date:** 2026-03-15

---

## Goal

Extend Jobs and Archetypes to define technology grants. At character creation, assign those technologies to the player's session and persist them. This sub-project covers data model extensions, DB schema, session fields, and the interactive character creation prompt. Technology use/resolution, slot rearrangement at rest, use count decrement, and level-up selection are out of scope.

---

## Context

The technology data model (`TechnologyDef`, `Registry`) is already implemented and loaded from YAML. `TechnologyDef` records are not persisted in the database — only session slot assignments (which tech goes in which slot) are persisted. This sub-project builds the assignment layer on top of the existing registry.

---

## Feature 1: Rename cantrip → hardwired

### Constant rename

```go
// Before
UsageCantrip UsageType = "cantrip"

// After
UsageHardwired UsageType = "hardwired"
```

- The Go constant `UsageCantrip` is renamed to `UsageHardwired`.
- The YAML string value changes from `"cantrip"` to `"hardwired"`.
- The `validUsageTypes` map key is updated from `UsageCantrip` to `UsageHardwired`.
- All YAML content files using `usage_type: cantrip` are updated to `usage_type: hardwired`.
- All testdata fixtures are updated.
- No database migration is needed — technology definitions are loaded from YAML only.

---

## Feature 2: Job YAML extension

### New `technology_grants` block on `Job`

```go
// TechnologyGrants defines all technology assignments a job provides at character creation.
type TechnologyGrants struct {
    Hardwired   []string               `yaml:"hardwired,omitempty"`
    Prepared    *PreparedGrants        `yaml:"prepared,omitempty"`
    Spontaneous *SpontaneousGrants     `yaml:"spontaneous,omitempty"`
}

// PreparedGrants defines prepared technology slot allocation for a job.
type PreparedGrants struct {
    // SlotsByLevel maps technology level to number of prepared slots at that level.
    SlotsByLevel map[int]int `yaml:"slots_by_level"`
    // Fixed lists job-mandated prepared technologies. No player choice for these.
    Fixed []PreparedEntry `yaml:"fixed,omitempty"`
    // Pool lists technologies the player may choose to fill remaining slots.
    Pool []PreparedEntry `yaml:"pool,omitempty"`
}

// PreparedEntry is a technology at a specific level for a prepared slot.
type PreparedEntry struct {
    ID    string `yaml:"id"`
    Level int    `yaml:"level"`
}

// SpontaneousGrants defines spontaneous technology slot allocation for a job.
// Uses are tracked as a shared pool per level (PF2E-faithful): all known techs
// at a given level draw from the same daily use pool for that level.
type SpontaneousGrants struct {
    // KnownByLevel maps technology level to number of techs known at that level.
    KnownByLevel map[int]int `yaml:"known_by_level"`
    // UsesByLevel maps technology level to shared uses per day at that level.
    UsesByLevel map[int]int `yaml:"uses_by_level"`
    // Fixed lists job-mandated spontaneous technologies (always known; no player choice).
    Fixed []SpontaneousEntry `yaml:"fixed,omitempty"`
    // Pool lists technologies the player may choose to fill remaining known slots.
    Pool []SpontaneousEntry `yaml:"pool,omitempty"`
}

// SpontaneousEntry is a technology at a specific level for a spontaneous known slot.
type SpontaneousEntry struct {
    ID    string `yaml:"id"`
    Level int    `yaml:"level"`
}
```

`TechnologyGrants` is added as a field on the existing `Job` struct:

```go
TechnologyGrants *TechnologyGrants `yaml:"technology_grants,omitempty"`
```

### Validation

At YAML load time, `TechnologyGrants.Validate()` enforces:
- For each level in `PreparedGrants.SlotsByLevel`: `len(Fixed at level) + len(Pool at level) >= SlotsByLevel[level]`. If the pool is too small to fill all slots, return an error (fail-fast at load time, not at character creation).
- Same constraint for `SpontaneousGrants`: `len(Fixed at level) + len(Pool at level) >= KnownByLevel[level]`.

### Example Job YAML

```yaml
technology_grants:
  hardwired:
    - neural_shock
  prepared:
    slots_by_level:
      1: 2
      2: 1
    fixed:
      - id: neural_shock
        level: 1
    pool:
      - id: mind_spike
        level: 1
      - id: arc_thought
        level: 1
  spontaneous:
    known_by_level:
      1: 2
    uses_by_level:
      1: 4
    fixed:
      - id: battle_fervor
        level: 1
    pool:
      - id: acid_spray
        level: 1
```

---

## Feature 3: Archetype YAML extension

### New `innate_technologies` field on `Archetype`

```go
// InnateGrant defines a single innate technology granted by an archetype.
type InnateGrant struct {
    ID         string `yaml:"id"`
    UsesPerDay int    `yaml:"uses_per_day"` // 0 = unlimited
}
```

`InnateGrant` slice added to the existing `Archetype` struct:

```go
InnateTechnologies []InnateGrant `yaml:"innate_technologies,omitempty"`
```

---

## Feature 4: Session fields

### New types in `internal/game/session/`

```go
// PreparedSlot holds one prepared technology slot.
type PreparedSlot struct {
    TechID string
}

// InnateSlot tracks an innate technology.
// MaxUses == 0 means unlimited. UsesRemaining is set at creation; decrement is future scope.
type InnateSlot struct {
    MaxUses int // 0 = unlimited
}
```

Spontaneous technologies are tracked as a map of known tech IDs per level.
Uses are a shared daily pool per level; use tracking is future scope (session only stores which techs are known).

### New fields on `PlayerSession`

```go
HardwiredTechs   []string                // tech IDs; unlimited use
PreparedTechs    map[int][]*PreparedSlot  // slot level → ordered slots
SpontaneousTechs map[int][]string         // tech level → known tech IDs (shared use pool per level)
InnateTechs      map[string]*InnateSlot   // tech_id → innate slot info
```

These fields follow the existing nil-until-loaded pattern: they are `nil` in `Manager.AddPlayer()` and populated during login loading (same as `PassiveFeats`, `FeatureChoices`).

---

## Feature 5: Database schema

Four new tables added via a single migration. `character_id` is `int64` consistent with existing tables (e.g., `character_class_features`).

```sql
CREATE TABLE character_hardwired_technologies (
    character_id BIGINT NOT NULL,
    tech_id      TEXT   NOT NULL,
    PRIMARY KEY (character_id, tech_id)
);

CREATE TABLE character_prepared_technologies (
    character_id BIGINT NOT NULL,
    slot_level   INT    NOT NULL,
    slot_index   INT    NOT NULL,
    tech_id      TEXT   NOT NULL,
    PRIMARY KEY (character_id, slot_level, slot_index)
);

CREATE TABLE character_spontaneous_technologies (
    character_id BIGINT NOT NULL,
    tech_id      TEXT   NOT NULL,
    level        INT    NOT NULL,
    PRIMARY KEY (character_id, tech_id)
);

CREATE TABLE character_innate_technologies (
    character_id BIGINT NOT NULL,
    tech_id      TEXT   NOT NULL,
    max_uses     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, tech_id)
);
```

### Repository types

Four concrete structs following the existing pattern in `internal/storage/postgres/` (concrete struct, not interface, consistent with `CharacterClassFeaturesRepository`):

```go
type CharacterHardwiredTechRepository struct { db *pgxpool.Pool }
func (r *CharacterHardwiredTechRepository) GetAll(ctx context.Context, characterID int64) ([]string, error)
func (r *CharacterHardwiredTechRepository) SetAll(ctx context.Context, characterID int64, techIDs []string) error

type CharacterPreparedTechRepository struct { db *pgxpool.Pool }
func (r *CharacterPreparedTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]*session.PreparedSlot, error)
func (r *CharacterPreparedTechRepository) Set(ctx context.Context, characterID int64, level, index int, techID string) error
func (r *CharacterPreparedTechRepository) DeleteAll(ctx context.Context, characterID int64) error

type CharacterSpontaneousTechRepository struct { db *pgxpool.Pool }
func (r *CharacterSpontaneousTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]string, error)
func (r *CharacterSpontaneousTechRepository) Add(ctx context.Context, characterID int64, techID string, level int) error
func (r *CharacterSpontaneousTechRepository) DeleteAll(ctx context.Context, characterID int64) error

type CharacterInnateTechRepository struct { db *pgxpool.Pool }
func (r *CharacterInnateTechRepository) GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error)
func (r *CharacterInnateTechRepository) Set(ctx context.Context, characterID int64, techID string, maxUses int) error
func (r *CharacterInnateTechRepository) DeleteAll(ctx context.Context, characterID int64) error
```

---

## Feature 6: Character creation flow

### Assignment function

`assignTechnologies(ctx, sess, uid, job, archetype, techRegistry, hardwiredRepo, preparedRepo, spontaneousRepo, innateRepo)` runs after ability boost selection during character creation. `techRegistry` is used to look up tech names and descriptions for the interactive prompt.

1. **Hardwired** — copy `job.TechnologyGrants.Hardwired` into `sess.HardwiredTechs`; persist via `hardwiredRepo.SetAll`.
2. **Innate** — for each `InnateGrant` in `archetype.InnateTechnologies`: add to `sess.InnateTechs`; persist via `innateRepo.Set`.
3. **Prepared** — for each level in `SlotsByLevel`:
   - Pre-fill slots from `Fixed` at this level (no prompt).
   - Count open = `SlotsByLevel[level] - len(fixed at level)`.
   - Collect pool entries at this level.
   - If `len(pool at level) == open`: auto-assign without prompt.
   - If `len(pool at level) > open`: present numbered list, player selects until `open` slots filled.
   - Persist each slot via `preparedRepo.Set`.
4. **Spontaneous** — for each level in `KnownByLevel`:
   - Pre-fill from `Fixed` at this level (no prompt).
   - Count open = `KnownByLevel[level] - len(fixed at level)`.
   - Collect pool entries at this level.
   - If `len(pool at level) == open`: auto-assign without prompt.
   - If `len(pool at level) > open`: numbered list prompt.
   - Add each known tech via `spontaneousRepo.Add`.
5. If `job.TechnologyGrants` is nil, skip all steps silently. Session fields remain nil (populated at login from DB, which will return empty results).

### Prompt format

```
Choose a level-1 prepared technology (1 remaining):
1. Mind Spike — A focused neural disruption that scrambles a target's cognition.
2. Arc Thought — ...
>
```

### Loading at login

After loading class features (`loadClassFeatures`), a new `loadTechnologies` function loads all four technology tables by `CharacterID` and populates `sess.HardwiredTechs`, `sess.PreparedTechs`, `sess.SpontaneousTechs`, `sess.InnateTechs`.

### GameServiceServer wiring

Four new repository fields added to `GameServiceServer` and `NewGameServiceServer`, injected from `cmd/gameserver/main.go`. Follows the existing repository injection pattern.

---

## Feature 7: FEATURES.md update

Add to the Technology section:

```
- [ ] Level-up technology selection — player chooses new technologies when levelling up (prepared/spontaneous pool expands; player selects additions interactively)
```

---

## Testing

All tests using TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-TG1**: `UsageHardwired` constant has value `"hardwired"`; the `validUsageTypes` map no longer contains `"cantrip"`.
- **REQ-TG2**: `Job` with `technology_grants.hardwired` YAML round-trips without data loss.
- **REQ-TG3**: `Job` with `technology_grants.prepared` YAML round-trips without data loss.
- **REQ-TG4**: `Job` with `technology_grants.spontaneous` YAML round-trips without data loss.
- **REQ-TG5**: `Archetype` with `innate_technologies` YAML round-trips without data loss.
- **REQ-TG6**: `assignTechnologies` with a fully-specified job and archetype populates all four session fields correctly.
- **REQ-TG7**: `assignTechnologies` with `job.TechnologyGrants == nil` leaves all session fields nil (no-op).
- **REQ-TG8**: `assignTechnologies` auto-assigns prepared pool entries when `len(pool at level) == open slots` (no prompt).
- **REQ-TG9**: `assignTechnologies` auto-assigns spontaneous pool entries when `len(pool at level) == open slots` (no prompt).
- **REQ-TG10**: Loading session at login from repository populates all four session fields correctly.
- **REQ-TG11** (property): For any valid `TechnologyGrants`, YAML marshal/unmarshal round-trip preserves all fields.
- **REQ-TG12**: `TechnologyGrants.Validate()` returns an error when pool + fixed entries are insufficient to fill `SlotsByLevel` or `KnownByLevel` for any level.

---

## Constraints

- Technology use/resolution is out of scope.
- Slot rearrangement at rest is out of scope.
- Use count decrement is out of scope.
- Long-rest reset is out of scope.
- Level-up selection is out of scope (tracked in FEATURES.md as a future item).
- `TechnologyDef` records are not persisted in the database; only slot assignments are.
