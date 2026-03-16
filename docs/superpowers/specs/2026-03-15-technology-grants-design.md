# Technology Grants — Design Spec

**Date:** 2026-03-15

---

## Goal

Extend Jobs and Archetypes to define technology grants. At character creation, assign those technologies to the player's session and persist them. This sub-project covers data model extensions, DB schema, session fields, and the interactive character creation prompt. Technology use/resolution, slot rearrangement at rest, and level-up selection are out of scope.

---

## Context

The technology data model (`TechnologyDef`, `Registry`) is already implemented. This sub-project builds the assignment layer on top of it. It also renames `UsageCantrip` → `UsageHardwired` to match the game's cyberpunk setting.

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
- All YAML content files using `usage_type: cantrip` are updated to `usage_type: hardwired`.
- All testdata fixtures are updated.
- The `validUsageTypes` map is updated.

---

## Feature 2: Job YAML extension

### New `technology_grants` block on `Job`

```go
// TechnologyGrants defines all technology assignments a job provides at character creation.
type TechnologyGrants struct {
    Hardwired   []string               `yaml:"hardwired,omitempty"`   // tech IDs; unlimited use
    Prepared    *PreparedGrants        `yaml:"prepared,omitempty"`
    Spontaneous *SpontaneousGrants     `yaml:"spontaneous,omitempty"`
}

// PreparedGrants defines prepared technology slot allocation for a job.
type PreparedGrants struct {
    // SlotsByLevel maps technology level to number of prepared slots at that level.
    SlotsByLevel map[int]int `yaml:"slots_by_level"`
    // Fixed lists job-mandated prepared technologies (pre-fills slots; no player choice).
    Fixed []PreparedFixedGrant `yaml:"fixed,omitempty"`
    // Pool lists technologies the player may choose to fill remaining slots.
    Pool []PreparedPoolEntry `yaml:"pool,omitempty"`
}

// PreparedFixedGrant is a job-mandated prepared technology at a specific slot level.
type PreparedFixedGrant struct {
    ID    string `yaml:"id"`
    Level int    `yaml:"level"`
}

// PreparedPoolEntry is a technology available for player selection into a prepared slot.
type PreparedPoolEntry struct {
    ID    string `yaml:"id"`
    Level int    `yaml:"level"`
}

// SpontaneousGrants defines spontaneous technology slot allocation for a job.
type SpontaneousGrants struct {
    // KnownByLevel maps technology level to number of techs known at that level.
    KnownByLevel map[int]int `yaml:"known_by_level"`
    // UsesByLevel maps technology level to uses per day at that level.
    UsesByLevel map[int]int `yaml:"uses_by_level"`
    // Fixed lists job-mandated spontaneous technologies (always known; no player choice).
    Fixed []string `yaml:"fixed,omitempty"`
    // Pool lists technologies the player may choose to fill remaining known slots.
    Pool []string `yaml:"pool,omitempty"`
}
```

`TechnologyGrants` is added as a field on the existing `Job` struct:

```go
TechnologyGrants *TechnologyGrants `yaml:"technology_grants,omitempty"`
```

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
      - battle_fervor
    pool:
      - acid_spray
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

### Example Archetype YAML

```yaml
innate_technologies:
  - id: acid_spray
    uses_per_day: 1
```

---

## Feature 4: Session fields

### New types in `internal/game/session/`

```go
// PreparedSlot holds one prepared technology slot.
type PreparedSlot struct {
    TechID string
}

// SpontaneousSlot tracks a known spontaneous technology and its remaining daily uses.
type SpontaneousSlot struct {
    Level         int
    UsesRemaining int
}

// InnateSlot tracks an innate technology and its remaining daily uses.
// MaxUses == 0 means unlimited.
type InnateSlot struct {
    UsesRemaining int
    MaxUses       int
}
```

### New fields on `PlayerSession`

```go
HardwiredTechs   []string                   // tech IDs; unlimited use; set at creation
PreparedTechs    map[int][]*PreparedSlot     // slot level → ordered slots
SpontaneousTechs map[string]*SpontaneousSlot // tech_id → slot info
InnateTechs      map[string]*InnateSlot      // tech_id → slot info
```

All four fields are initialized to non-nil empty values in `Manager.AddPlayer()`.

---

## Feature 5: Database schema

Four new tables, added via a single migration:

```sql
CREATE TABLE character_hardwired_technologies (
    character_id TEXT NOT NULL,
    tech_id      TEXT NOT NULL,
    PRIMARY KEY (character_id, tech_id)
);

CREATE TABLE character_prepared_technologies (
    character_id TEXT NOT NULL,
    slot_level   INT  NOT NULL,
    slot_index   INT  NOT NULL,
    tech_id      TEXT NOT NULL,
    PRIMARY KEY (character_id, slot_level, slot_index)
);

CREATE TABLE character_spontaneous_technologies (
    character_id   TEXT NOT NULL,
    tech_id        TEXT NOT NULL,
    level          INT  NOT NULL,
    uses_remaining INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, tech_id)
);

CREATE TABLE character_innate_technologies (
    character_id   TEXT NOT NULL,
    tech_id        TEXT NOT NULL,
    uses_remaining INT  NOT NULL DEFAULT 0,
    max_uses       INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, tech_id)
);
```

### Repository interfaces

Four interfaces following the existing repository pattern in `internal/storage/postgres/`:

```go
type CharacterHardwiredTechRepository interface {
    GetAll(ctx context.Context, characterID string) ([]string, error)
    Add(ctx context.Context, characterID, techID string) error
    DeleteAll(ctx context.Context, characterID string) error
}

type CharacterPreparedTechRepository interface {
    GetAll(ctx context.Context, characterID string) (map[int][]*session.PreparedSlot, error)
    Set(ctx context.Context, characterID string, level, index int, techID string) error
    DeleteAll(ctx context.Context, characterID string) error
}

type CharacterSpontaneousTechRepository interface {
    GetAll(ctx context.Context, characterID string) (map[string]*session.SpontaneousSlot, error)
    Set(ctx context.Context, characterID, techID string, level, usesRemaining int) error
    DeleteAll(ctx context.Context, characterID string) error
}

type CharacterInnateTechRepository interface {
    GetAll(ctx context.Context, characterID string) (map[string]*session.InnateSlot, error)
    Set(ctx context.Context, characterID, techID string, usesRemaining, maxUses int) error
    DeleteAll(ctx context.Context, characterID string) error
}
```

---

## Feature 6: Character creation flow

### Assignment logic

The assignment function `assignTechnologies(sess, uid, job, archetype, techRegistry)` runs after ability boost selection during character creation:

1. **Hardwired** — copy `job.TechnologyGrants.Hardwired` directly into `sess.HardwiredTechs`; persist via `CharacterHardwiredTechRepository.Add`.
2. **Innate** — copy `archetype.InnateTechnologies` into `sess.InnateTechs`; persist via `CharacterInnateTechRepository.Set`.
3. **Prepared** — for each level in `SlotsByLevel`:
   - Pre-fill slots from `Fixed` (job-mandated, no prompt).
   - Count remaining open slots.
   - If `len(pool_entries_at_level) == remaining_open`: auto-assign without prompt.
   - Otherwise: present numbered list prompt, player selects to fill each remaining slot.
   - Persist all slots via `CharacterPreparedTechRepository.Set`.
4. **Spontaneous** — for each level in `KnownByLevel`:
   - Pre-fill from `Fixed` (always known, no prompt).
   - Count remaining open known slots.
   - If pool exactly fills remaining: auto-assign.
   - Otherwise: numbered list prompt.
   - Initialize `UsesRemaining = UsesByLevel[level]`.
   - Persist via `CharacterSpontaneousTechRepository.Set`.

### Prompt format

```
Choose a level-1 prepared technology (1 remaining):
1. Mind Spike — A focused neural disruption that scrambles a target's cognition.
2. Arc Thought — ...
>
```

### Loading at login

After loading class features, load all four technology tables and populate `PlayerSession` fields. Follows the same pattern as `loadClassFeatures()` in `grpc_service.go`.

### GameServiceServer wiring

Four new repository fields added to `GameServiceServer` and `NewGameServiceServer`, following the existing repository injection pattern. Injected from `cmd/gameserver/main.go`.

---

## Feature 7: FEATURES.md update

Add to the Technology section:

```
- [ ] Level-up technology selection — player chooses new technologies when levelling up (prepared/spontaneous pool expands; player selects additions interactively)
```

---

## Testing

All tests in the relevant packages (`package technology_test`, `package session_test`, `package ruleset_test`) using TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-TG1**: `UsageHardwired` constant has value `"hardwired"`; `UsageCantrip` no longer exists.
- **REQ-TG2**: `Job` with `technology_grants.hardwired` YAML round-trips without data loss.
- **REQ-TG3**: `Job` with `technology_grants.prepared` YAML round-trips without data loss.
- **REQ-TG4**: `Job` with `technology_grants.spontaneous` YAML round-trips without data loss.
- **REQ-TG5**: `Archetype` with `innate_technologies` YAML round-trips without data loss.
- **REQ-TG6**: `assignTechnologies` with a fully-specified job and archetype populates all four session fields correctly.
- **REQ-TG7**: `assignTechnologies` with a job that has no `technology_grants` leaves all session fields as empty (not nil).
- **REQ-TG8**: `assignTechnologies` auto-assigns prepared pool entries when pool size equals open slots (no prompt).
- **REQ-TG9**: `assignTechnologies` auto-assigns spontaneous pool entries when pool size equals open slots (no prompt).
- **REQ-TG10**: Loading session at login from repository populates all four session fields correctly.
- **REQ-TG11** (property): For any valid `TechnologyGrants`, YAML marshal/unmarshal round-trip preserves all fields.

---

## Constraints

- Technology use/resolution is out of scope.
- Slot rearrangement at rest is out of scope.
- Use count decrement is out of scope (UsesRemaining is set at creation but not decremented here).
- Long-rest reset is out of scope.
- Level-up selection is out of scope (tracked in FEATURES.md as a future item).
