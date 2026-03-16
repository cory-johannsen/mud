# Technology Grants Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend Jobs and Archetypes to define technology grants; assign and persist all four technology types (hardwired, prepared, spontaneous, innate) at character creation with an interactive prompt for player-choosable slots.

**Architecture:** New `TechnologyGrants` / `InnateGrant` types added to the `ruleset` package extend the existing Job/Archetype YAML model. Four new postgres repositories persist slot assignments. A new `assignTechnologies` function in `internal/gameserver/` handles character-creation interactive flow. Session loading at login follows the existing `loadClassFeatures` pattern.

**Tech Stack:** Go modules (`github.com/cory-johannsen/mud`), `gopkg.in/yaml.v3`, `pgx/v5`, `pgregory.net/rapid v1.2.0`, `github.com/stretchr/testify`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/game/technology/model.go` | Modify | Rename `UsageCantrip` → `UsageHardwired` |
| `internal/game/technology/model_test.go` | Modify | Update test for renamed constant |
| `internal/game/technology/testdata/*.yaml` | Modify | Update `usage_type: cantrip` → `hardwired` |
| `content/technologies/**/*.yaml` | Modify | Update seed content |
| `internal/game/ruleset/technology_grants.go` | Create | `TechnologyGrants`, `PreparedGrants`, `PreparedEntry`, `SpontaneousGrants`, `SpontaneousEntry`, `InnateGrant`, `Validate()` |
| `internal/game/ruleset/technology_grants_test.go` | Create | REQ-TG2–TG5, TG11, TG12 |
| `internal/game/ruleset/job.go` | Modify | Add `TechnologyGrants *TechnologyGrants` field |
| `internal/game/ruleset/archetype.go` | Modify | Add `InnateTechnologies []InnateGrant` field |
| `internal/game/session/technology.go` | Create | `PreparedSlot`, `InnateSlot` types |
| `internal/game/session/manager.go` | Modify | Add 4 new fields to `PlayerSession` |
| `migrations/025_character_technologies.up.sql` | Create | 4 new tables |
| `migrations/025_character_technologies.down.sql` | Create | Drop 4 tables |
| `internal/storage/postgres/character_hardwired_tech.go` | Create | `CharacterHardwiredTechRepository` |
| `internal/storage/postgres/character_prepared_tech.go` | Create | `CharacterPreparedTechRepository` |
| `internal/storage/postgres/character_spontaneous_tech.go` | Create | `CharacterSpontaneousTechRepository` |
| `internal/storage/postgres/character_innate_tech.go` | Create | `CharacterInnateTechRepository` |
| `internal/storage/postgres/character_technology_repos_test.go` | Create | Integration tests for all four tech repos |
| `internal/gameserver/technology_assignment.go` | Create | `assignTechnologies`, `loadTechnologies` |
| `internal/gameserver/technology_assignment_test.go` | Create | REQ-TG6–TG10 |
| `internal/gameserver/grpc_service.go` | Modify | Add 4 repo fields; call assign + load |
| `cmd/gameserver/main.go` | Modify | Construct repos; inject into NewGameServiceServer |
| `docs/requirements/FEATURES.md` | Modify | Add level-up item |

---

## Chunk 1: Model Changes

### Task 1: Rename cantrip → hardwired

**Files:**
- Modify: `internal/game/technology/model.go`
- Modify: `internal/game/technology/model_test.go`
- Modify: `internal/game/technology/testdata/` (any fixtures using `usage_type: cantrip`)
- Modify: `content/technologies/**/*.yaml` (any seed using `usage_type: cantrip`)

- [ ] **Step 1: Write the failing test**

In `internal/game/technology/model_test.go`, add:

```go
// REQ-TG1: UsageHardwired constant has value "hardwired"; "cantrip" is no longer valid
func TestUsageHardwired_ConstantValue(t *testing.T) {
    assert.Equal(t, technology.UsageType("hardwired"), technology.UsageHardwired)
}

func TestValidUsageTypes_NoCantripKey(t *testing.T) {
    def := validDef()
    def.UsageType = technology.UsageType("cantrip")
    err := def.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "usage_type")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud/.worktrees/technology-grants
go test ./internal/game/technology/... -run TestUsageHardwired -run TestValidUsageTypes_NoCantripKey -v 2>&1
```

Expected: compile error (`UsageHardwired` undefined) or test failure.

- [ ] **Step 3: Update model.go AND model_test.go atomically**

**Important:** Steps 3 and 4 (updating the constant and all references) must be done together before any `go build` or `go test`. Do not run tests between them.

In `internal/game/technology/model.go`, replace:
```go
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
```

With:
```go
const (
    UsageHardwired   UsageType = "hardwired"
    UsagePrepared    UsageType = "prepared"
    UsageSpontaneous UsageType = "spontaneous"
    UsageInnate      UsageType = "innate"
)

var validUsageTypes = map[UsageType]bool{
    UsageHardwired: true, UsagePrepared: true,
    UsageSpontaneous: true, UsageInnate: true,
}
```

In `internal/game/technology/model_test.go`, find all occurrences of `UsageCantrip` (in `validDef()`, any `SampledFrom` list, any other helpers) and replace with `UsageHardwired`.

- [ ] **Step 4: Update any testdata and content YAML files**

Search for `usage_type: cantrip` in testdata (no matches expected — existing fixtures use other types):
```bash
grep -r "usage_type: cantrip" /home/cjohannsen/src/mud/.worktrees/technology-grants/internal/game/technology/testdata/ 2>/dev/null || echo "none found"
grep -r "usage_type: cantrip" /home/cjohannsen/src/mud/.worktrees/technology-grants/content/ 2>/dev/null || echo "none found"
```

Update any found files to `usage_type: hardwired`. No matches is the expected result — the existing seed and testdata do not use `cantrip`.

- [ ] **Step 5: Run all technology tests**

```bash
go test ./internal/game/technology/... -v 2>&1 | tail -15
```

Expected: all PASS.

- [ ] **Step 6: Run full test suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|error" | head -10
```

Expected: no failures.

- [ ] **Step 7: Commit**

```bash
git add internal/game/technology/ content/technologies/
git commit -m "feat(technology): rename UsageCantrip -> UsageHardwired; update YAML value cantrip -> hardwired"
```

---

### Task 2: TechnologyGrants model in ruleset package

**Files:**
- Create: `internal/game/ruleset/technology_grants.go`
- Create: `internal/game/ruleset/technology_grants_test.go`
- Modify: `internal/game/ruleset/job.go`
- Modify: `internal/game/ruleset/archetype.go`

- [ ] **Step 1: Write failing tests**

Create `internal/game/ruleset/technology_grants_test.go`:

```go
package ruleset_test

import (
    "fmt"
    "os"
    "path/filepath"
    "testing"

    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gopkg.in/yaml.v3"
    "pgregory.net/rapid"
)

// REQ-TG2: Job with technology_grants.hardwired YAML round-trips
func TestJob_TechnologyGrants_Hardwired_RoundTrip(t *testing.T) {
    src := `
id: test_job
name: Test Job
archetype: aggressor
technology_grants:
  hardwired:
    - neural_shock
    - mind_spike
`
    var j ruleset.Job
    require.NoError(t, yaml.Unmarshal([]byte(src), &j))
    require.NotNil(t, j.TechnologyGrants)
    assert.Equal(t, []string{"neural_shock", "mind_spike"}, j.TechnologyGrants.Hardwired)
}

// REQ-TG3: Job with technology_grants.prepared YAML round-trips
func TestJob_TechnologyGrants_Prepared_RoundTrip(t *testing.T) {
    src := `
id: test_job
name: Test Job
archetype: aggressor
technology_grants:
  prepared:
    slots_by_level:
      1: 2
    fixed:
      - id: neural_shock
        level: 1
    pool:
      - id: mind_spike
        level: 1
      - id: arc_thought
        level: 1
`
    var j ruleset.Job
    require.NoError(t, yaml.Unmarshal([]byte(src), &j))
    require.NotNil(t, j.TechnologyGrants)
    require.NotNil(t, j.TechnologyGrants.Prepared)
    assert.Equal(t, 2, j.TechnologyGrants.Prepared.SlotsByLevel[1])
    assert.Equal(t, "neural_shock", j.TechnologyGrants.Prepared.Fixed[0].ID)
    assert.Equal(t, 1, j.TechnologyGrants.Prepared.Fixed[0].Level)
    assert.Len(t, j.TechnologyGrants.Prepared.Pool, 2)
}

// REQ-TG4: Job with technology_grants.spontaneous YAML round-trips
func TestJob_TechnologyGrants_Spontaneous_RoundTrip(t *testing.T) {
    src := `
id: test_job
name: Test Job
archetype: aggressor
technology_grants:
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
      - id: neural_shock
        level: 1
`
    var j ruleset.Job
    require.NoError(t, yaml.Unmarshal([]byte(src), &j))
    require.NotNil(t, j.TechnologyGrants)
    require.NotNil(t, j.TechnologyGrants.Spontaneous)
    assert.Equal(t, 2, j.TechnologyGrants.Spontaneous.KnownByLevel[1])
    assert.Equal(t, 4, j.TechnologyGrants.Spontaneous.UsesByLevel[1])
    assert.Equal(t, "battle_fervor", j.TechnologyGrants.Spontaneous.Fixed[0].ID)
    assert.Len(t, j.TechnologyGrants.Spontaneous.Pool, 2)
}

// REQ-TG5: Archetype with innate_technologies YAML round-trips
func TestArchetype_InnateTechnologies_RoundTrip(t *testing.T) {
    src := `
id: test_archetype
name: Test Archetype
innate_technologies:
  - id: acid_spray
    uses_per_day: 1
  - id: neural_shock
    uses_per_day: 0
`
    var a ruleset.Archetype
    require.NoError(t, yaml.Unmarshal([]byte(src), &a))
    require.Len(t, a.InnateTechnologies, 2)
    assert.Equal(t, "acid_spray", a.InnateTechnologies[0].ID)
    assert.Equal(t, 1, a.InnateTechnologies[0].UsesPerDay)
    assert.Equal(t, "neural_shock", a.InnateTechnologies[1].ID)
    assert.Equal(t, 0, a.InnateTechnologies[1].UsesPerDay)
}

// REQ-TG12: Validate returns error when pool+fixed < slots at any level
func TestTechnologyGrants_Validate_PoolTooSmall(t *testing.T) {
    grants := &ruleset.TechnologyGrants{
        Prepared: &ruleset.PreparedGrants{
            SlotsByLevel: map[int]int{1: 3},
            Fixed:        []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
            Pool:         []ruleset.PreparedEntry{{ID: "mind_spike", Level: 1}},
            // 1 fixed + 1 pool = 2, but 3 slots required
        },
    }
    err := grants.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "level 1")
}

func TestTechnologyGrants_Validate_SufficientPool(t *testing.T) {
    grants := &ruleset.TechnologyGrants{
        Prepared: &ruleset.PreparedGrants{
            SlotsByLevel: map[int]int{1: 2},
            Fixed:        []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
            Pool:         []ruleset.PreparedEntry{{ID: "mind_spike", Level: 1}},
        },
    }
    require.NoError(t, grants.Validate())
}

func TestTechnologyGrants_Validate_Spontaneous_PoolTooSmall(t *testing.T) {
    grants := &ruleset.TechnologyGrants{
        Spontaneous: &ruleset.SpontaneousGrants{
            KnownByLevel: map[int]int{1: 3},
            UsesByLevel:  map[int]int{1: 4},
            Fixed:        []ruleset.SpontaneousEntry{{ID: "battle_fervor", Level: 1}},
            Pool:         []ruleset.SpontaneousEntry{{ID: "acid_spray", Level: 1}},
        },
    }
    err := grants.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "level 1")
}

// REQ-TG11 (property): TechnologyGrants YAML round-trip preserves all fields
func TestProperty_TechnologyGrants_YAMLRoundTrip(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        numFixed := rapid.IntRange(0, 3).Draw(rt, "numFixed")
        numPool := rapid.IntRange(0, 3).Draw(rt, "numPool")
        slots := numFixed + numPool
        if slots == 0 {
            slots = 1
            numPool = 1
        }

        prepFixed := make([]ruleset.PreparedEntry, numFixed)
        for i := range prepFixed {
            prepFixed[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("tech_%d", i), Level: 1}
        }
        prepPool := make([]ruleset.PreparedEntry, numPool)
        for i := range prepPool {
            prepPool[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("pool_%d", i), Level: 1}
        }

        numSpontFixed := rapid.IntRange(0, 3).Draw(rt, "numSpontFixed")
        numSpontPool := rapid.IntRange(0, 3).Draw(rt, "numSpontPool")
        known := numSpontFixed + numSpontPool
        if known == 0 {
            known = 1
            numSpontPool = 1
        }
        spontFixed := make([]ruleset.SpontaneousEntry, numSpontFixed)
        for i := range spontFixed {
            spontFixed[i] = ruleset.SpontaneousEntry{ID: fmt.Sprintf("spont_tech_%d", i), Level: 1}
        }
        spontPool := make([]ruleset.SpontaneousEntry, numSpontPool)
        for i := range spontPool {
            spontPool[i] = ruleset.SpontaneousEntry{ID: fmt.Sprintf("spont_pool_%d", i), Level: 1}
        }

        g := &ruleset.TechnologyGrants{
            Hardwired: []string{"hw_tech"},
            Prepared: &ruleset.PreparedGrants{
                SlotsByLevel: map[int]int{1: slots},
                Fixed:        prepFixed,
                Pool:         prepPool,
            },
            Spontaneous: &ruleset.SpontaneousGrants{
                KnownByLevel: map[int]int{1: known},
                UsesByLevel:  map[int]int{1: 4},
                Fixed:        spontFixed,
                Pool:         spontPool,
            },
        }
        data, err := yaml.Marshal(g)
        require.NoError(rt, err)
        var got ruleset.TechnologyGrants
        require.NoError(rt, yaml.Unmarshal(data, &got))
        assert.Equal(rt, g.Hardwired, got.Hardwired)
        require.NotNil(rt, got.Prepared)
        assert.Equal(rt, g.Prepared.SlotsByLevel, got.Prepared.SlotsByLevel)
        assert.Equal(rt, g.Prepared.Fixed, got.Prepared.Fixed)
        assert.Equal(rt, g.Prepared.Pool, got.Prepared.Pool)
        require.NotNil(rt, got.Spontaneous)
        assert.Equal(rt, g.Spontaneous.KnownByLevel, got.Spontaneous.KnownByLevel)
        assert.Equal(rt, g.Spontaneous.UsesByLevel, got.Spontaneous.UsesByLevel)
        assert.Equal(rt, g.Spontaneous.Fixed, got.Spontaneous.Fixed)
        assert.Equal(rt, g.Spontaneous.Pool, got.Spontaneous.Pool)
    })
}

// REQ-TG12 (load-time): LoadJobs returns error when technology_grants pool is insufficient
func TestLoadJobs_RejectsInvalidTechnologyGrants(t *testing.T) {
    dir := t.TempDir()
    content := `
id: bad_job
name: Bad Job
archetype: aggressor
technology_grants:
  prepared:
    slots_by_level:
      1: 3
    fixed:
      - id: neural_shock
        level: 1
    pool:
      - id: mind_spike
        level: 1
`
    require.NoError(t, os.WriteFile(filepath.Join(dir, "bad_job.yaml"), []byte(content), 0644))
    _, err := ruleset.LoadJobs(dir)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "level 1")
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/game/ruleset/... -run TestJob_TechnologyGrants -run TestArchetype_Innate -run TestTechnologyGrants -v 2>&1 | head -10
```

Expected: compile error (types not yet defined).

- [ ] **Step 3: Create technology_grants.go**

Create `internal/game/ruleset/technology_grants.go`:

```go
package ruleset

import "fmt"

// TechnologyGrants defines all technology assignments a job provides at character creation.
//
// Precondition: nil TechnologyGrants is valid (job grants no technologies).
// Postcondition: Validate() returns nil iff pool+fixed entries are sufficient for all slots.
type TechnologyGrants struct {
    Hardwired   []string           `yaml:"hardwired,omitempty"`
    Prepared    *PreparedGrants    `yaml:"prepared,omitempty"`
    Spontaneous *SpontaneousGrants `yaml:"spontaneous,omitempty"`
}

// PreparedGrants defines prepared technology slot allocation for a job.
type PreparedGrants struct {
    // SlotsByLevel maps technology level to number of prepared slots at that level.
    SlotsByLevel map[int]int     `yaml:"slots_by_level"`
    // Fixed lists job-mandated prepared technologies (pre-fills slots; no player choice).
    Fixed        []PreparedEntry `yaml:"fixed,omitempty"`
    // Pool lists technologies the player may choose to fill remaining slots.
    Pool         []PreparedEntry `yaml:"pool,omitempty"`
}

// PreparedEntry is a technology at a specific level for a prepared slot.
type PreparedEntry struct {
    ID    string `yaml:"id"`
    Level int    `yaml:"level"`
}

// SpontaneousGrants defines spontaneous technology known-slot allocation for a job.
// Uses are a shared daily pool per level (PF2E-faithful).
type SpontaneousGrants struct {
    // KnownByLevel maps technology level to number of techs known at that level.
    KnownByLevel map[int]int        `yaml:"known_by_level"`
    // UsesByLevel maps technology level to shared daily uses at that level.
    UsesByLevel  map[int]int        `yaml:"uses_by_level"`
    // Fixed lists job-mandated spontaneous technologies (always known; no player choice).
    Fixed        []SpontaneousEntry `yaml:"fixed,omitempty"`
    // Pool lists technologies the player may choose to fill remaining known slots.
    Pool         []SpontaneousEntry `yaml:"pool,omitempty"`
}

// SpontaneousEntry is a technology at a specific level for a spontaneous known slot.
type SpontaneousEntry struct {
    ID    string `yaml:"id"`
    Level int    `yaml:"level"`
}

// InnateGrant defines a single innate technology granted by an archetype.
type InnateGrant struct {
    ID         string `yaml:"id"`
    UsesPerDay int    `yaml:"uses_per_day"` // 0 = unlimited
}

// Validate returns an error if pool+fixed entries are insufficient to fill any slot level.
// Precondition: g is not nil.
func (g *TechnologyGrants) Validate() error {
    if g.Prepared != nil {
        for lvl, slots := range g.Prepared.SlotsByLevel {
            fixed := countEntriesAtLevel(preparedToLeveled(g.Prepared.Fixed), lvl)
            pool := countEntriesAtLevel(preparedToLeveled(g.Prepared.Pool), lvl)
            if fixed+pool < slots {
                return fmt.Errorf("prepared: level %d requires %d slots but only %d fixed+pool entries available", lvl, slots, fixed+pool)
            }
        }
    }
    if g.Spontaneous != nil {
        for lvl, known := range g.Spontaneous.KnownByLevel {
            fixed := countEntriesAtLevel(spontaneousToLeveled(g.Spontaneous.Fixed), lvl)
            pool := countEntriesAtLevel(spontaneousToLeveled(g.Spontaneous.Pool), lvl)
            if fixed+pool < known {
                return fmt.Errorf("spontaneous: level %d requires %d known but only %d fixed+pool entries available", lvl, known, fixed+pool)
            }
        }
    }
    return nil
}

type leveledEntry struct{ level int }

func preparedToLeveled(entries []PreparedEntry) []leveledEntry {
    out := make([]leveledEntry, len(entries))
    for i, e := range entries {
        out[i] = leveledEntry{e.Level}
    }
    return out
}

func spontaneousToLeveled(entries []SpontaneousEntry) []leveledEntry {
    out := make([]leveledEntry, len(entries))
    for i, e := range entries {
        out[i] = leveledEntry{e.Level}
    }
    return out
}

func countEntriesAtLevel(entries []leveledEntry, level int) int {
    n := 0
    for _, e := range entries {
        if e.level == level {
            n++
        }
    }
    return n
}
```

- [ ] **Step 4: Add TechnologyGrants field to Job**

In `internal/game/ruleset/job.go`, add the field after `ClassFeatureGrants`:

```go
TechnologyGrants *TechnologyGrants `yaml:"technology_grants,omitempty"`
```

- [ ] **Step 5: Add InnateTechnologies field to Archetype**

In `internal/game/ruleset/archetype.go`, add the field after `AbilityBoosts`:

```go
InnateTechnologies []InnateGrant `yaml:"innate_technologies,omitempty"`
```

- [ ] **Step 6: Wire Validate() into LoadJobs (fail-fast at load time)**

In `internal/game/ruleset/job.go`, find the `LoadJobs` function. After unmarshaling each job, call `Validate()` on `TechnologyGrants` if non-nil and return an error if it fails. The pattern should follow the existing fail-fast structure in LoadJobs.

Add the following after the YAML unmarshal of each job:

```go
if job.TechnologyGrants != nil {
    if err := job.TechnologyGrants.Validate(); err != nil {
        return nil, fmt.Errorf("job %q technology_grants: %w", job.ID, err)
    }
}
```

The test for this (`TestLoadJobs_RejectsInvalidTechnologyGrants`) was already written in Step 1 and is already failing. Running Step 7 will verify this test now passes.

- [ ] **Step 7: Run tests**

```bash
go test ./internal/game/ruleset/... -v 2>&1 | tail -15
```

Expected: all PASS including the new technology grants tests.

- [ ] **Step 8: Run full suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|error" | head -10
```

Expected: no failures.

- [ ] **Step 9: Commit**

```bash
git add internal/game/ruleset/technology_grants.go internal/game/ruleset/technology_grants_test.go internal/game/ruleset/job.go internal/game/ruleset/archetype.go
git commit -m "feat(ruleset): TechnologyGrants model; add to Job and Archetype YAML structs; fail-fast validation in LoadJobs"
```

---

## Chunk 2: Session + DB + Repositories

### Task 3: Session types and PlayerSession fields

**Files:**
- Create: `internal/game/session/technology.go`
- Modify: `internal/game/session/manager.go`

- [ ] **Step 1: Write failing test**

Create `internal/game/session/technology_test.go`:

```go
package session_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/stretchr/testify/assert"
)

func TestPreparedSlot_ZeroValue(t *testing.T) {
    var s session.PreparedSlot
    assert.Equal(t, "", s.TechID)
}

func TestInnateSlot_ZeroMaxUses_MeansUnlimited(t *testing.T) {
    s := session.InnateSlot{MaxUses: 0}
    assert.Equal(t, 0, s.MaxUses) // 0 = unlimited per spec
}

func TestPlayerSession_TechFields_NilUntilLoaded(t *testing.T) {
    sess := &session.PlayerSession{}
    assert.Nil(t, sess.HardwiredTechs)
    assert.Nil(t, sess.PreparedTechs)
    assert.Nil(t, sess.SpontaneousTechs)
    assert.Nil(t, sess.InnateTechs)
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/game/session/... -run TestPreparedSlot -run TestInnateSlot -run TestPlayerSession_TechFields -v 2>&1 | head -10
```

Expected: compile error (types undefined).

- [ ] **Step 3: Create session/technology.go**

```go
// internal/game/session/technology.go
package session

// PreparedSlot holds one prepared technology slot.
type PreparedSlot struct {
    TechID string
}

// InnateSlot tracks an innate technology granted by an archetype.
// MaxUses == 0 means unlimited.
type InnateSlot struct {
    MaxUses int
}
```

- [ ] **Step 4: Add fields to PlayerSession**

In `internal/game/session/manager.go`, find the `PlayerSession` struct and add four fields after `FeatureChoices`:

```go
// Technology slots — nil until loaded from DB at login.
HardwiredTechs   []string                   // tech IDs; unlimited use
PreparedTechs    map[int][]*PreparedSlot     // slot level → ordered slots
SpontaneousTechs map[int][]string            // tech level → known tech IDs
InnateTechs      map[string]*InnateSlot      // tech_id → innate slot info
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/game/session/... -v 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/game/session/technology.go internal/game/session/technology_test.go internal/game/session/manager.go
git commit -m "feat(session): add technology slot types and PlayerSession fields"
```

---

### Task 4: Database migration

**Files:**
- Create: `migrations/025_character_technologies.up.sql`
- Create: `migrations/025_character_technologies.down.sql`

- [ ] **Step 1: Create up migration**

Create `migrations/025_character_technologies.up.sql`:

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

- [ ] **Step 2: Create down migration**

Create `migrations/025_character_technologies.down.sql`:

```sql
DROP TABLE IF EXISTS character_innate_technologies;
DROP TABLE IF EXISTS character_spontaneous_technologies;
DROP TABLE IF EXISTS character_prepared_technologies;
DROP TABLE IF EXISTS character_hardwired_technologies;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/025_character_technologies.up.sql migrations/025_character_technologies.down.sql
git commit -m "feat(db): migration 025 — character technology slot tables"
```

---

### Task 5: Repository implementations

**Files:**
- Create: `internal/storage/postgres/character_technology_repos_test.go`
- Create: `internal/storage/postgres/character_hardwired_tech.go`
- Create: `internal/storage/postgres/character_prepared_tech.go`
- Create: `internal/storage/postgres/character_spontaneous_tech.go`
- Create: `internal/storage/postgres/character_innate_tech.go`

- [ ] **Step 1: Write failing integration tests**

Create `internal/storage/postgres/character_technology_repos_test.go`:

```go
package postgres_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"

    pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
    "github.com/cory-johannsen/mud/internal/game/session"
)

// --- Hardwired repo ---

func TestCharacterHardwiredTechRepo_GetAll_EmptyForNew(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterHardwiredTechRepository(sharedPool)

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Empty(t, got)
}

func TestCharacterHardwiredTechRepo_SetAll_And_GetAll(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterHardwiredTechRepository(sharedPool)

    require.NoError(t, repo.SetAll(ctx, ch.ID, []string{"neural_shock", "mind_spike"}))

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.ElementsMatch(t, []string{"neural_shock", "mind_spike"}, got)
}

func TestPropertyCharacterHardwiredTechRepo_RoundTrip(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    repo := pgstore.NewCharacterHardwiredTechRepository(sharedPool)

    rapid.Check(t, func(rt *rapid.T) {
        ch := createTestCharacter(t, charRepo, ctx)
        n := rapid.IntRange(1, 5).Draw(rt, "n")
        ids := make([]string, n)
        for i := range ids {
            ids[i] = rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
        }
        if err := repo.SetAll(ctx, ch.ID, ids); err != nil {
            rt.Fatalf("SetAll: %v", err)
        }
        got, err := repo.GetAll(ctx, ch.ID)
        if err != nil {
            rt.Fatalf("GetAll: %v", err)
        }
        assert.ElementsMatch(rt, ids, got)
    })
}

// --- Prepared repo ---

func TestCharacterPreparedTechRepo_GetAll_EmptyForNew(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Empty(t, got)
}

func TestCharacterPreparedTechRepo_Set_And_GetAll(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

    require.NoError(t, repo.Set(ctx, ch.ID, 1, 0, "neural_shock"))
    require.NoError(t, repo.Set(ctx, ch.ID, 1, 1, "mind_spike"))

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    require.Len(t, got[1], 2)
    assert.Equal(t, "neural_shock", got[1][0].TechID)
    assert.Equal(t, "mind_spike", got[1][1].TechID)
}

func TestCharacterPreparedTechRepo_DeleteAll(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

    require.NoError(t, repo.Set(ctx, ch.ID, 1, 0, "neural_shock"))
    require.NoError(t, repo.DeleteAll(ctx, ch.ID))
    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Empty(t, got)
}

// --- Spontaneous repo ---

func TestCharacterSpontaneousTechRepo_GetAll_EmptyForNew(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterSpontaneousTechRepository(sharedPool)

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Empty(t, got)
}

func TestCharacterSpontaneousTechRepo_Add_And_GetAll(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterSpontaneousTechRepository(sharedPool)

    require.NoError(t, repo.Add(ctx, ch.ID, "battle_fervor", 1))
    require.NoError(t, repo.Add(ctx, ch.ID, "acid_spray", 1))

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.ElementsMatch(t, []string{"battle_fervor", "acid_spray"}, got[1])
}

func TestPropertyCharacterSpontaneousTechRepo_RoundTrip(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    repo := pgstore.NewCharacterSpontaneousTechRepository(sharedPool)

    rapid.Check(t, func(rt *rapid.T) {
        ch := createTestCharacter(t, charRepo, ctx)
        techID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
        level := rapid.IntRange(1, 10).Draw(rt, "level")
        if err := repo.Add(ctx, ch.ID, techID, level); err != nil {
            rt.Fatalf("Add: %v", err)
        }
        got, err := repo.GetAll(ctx, ch.ID)
        if err != nil {
            rt.Fatalf("GetAll: %v", err)
        }
        assert.Contains(rt, got[level], techID)
    })
}

func TestPropertyCharacterPreparedTechRepo_RoundTrip(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

    rapid.Check(t, func(rt *rapid.T) {
        ch := createTestCharacter(t, charRepo, ctx)
        level := rapid.IntRange(1, 10).Draw(rt, "level")
        index := rapid.IntRange(0, 4).Draw(rt, "index")
        techID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
        if err := repo.Set(ctx, ch.ID, level, index, techID); err != nil {
            rt.Fatalf("Set: %v", err)
        }
        got, err := repo.GetAll(ctx, ch.ID)
        if err != nil {
            rt.Fatalf("GetAll: %v", err)
        }
        require.True(rt, len(got[level]) > index, "slot not found at level %d index %d", level, index)
        assert.Equal(rt, techID, got[level][index].TechID)
    })
}

// --- Innate repo ---

func TestCharacterInnateTechRepo_GetAll_EmptyForNew(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterInnateTechRepository(sharedPool)

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Empty(t, got)
}

func TestCharacterInnateTechRepo_Set_And_GetAll(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, charRepo, ctx)
    repo := pgstore.NewCharacterInnateTechRepository(sharedPool)

    require.NoError(t, repo.Set(ctx, ch.ID, "acid_spray", 3))
    require.NoError(t, repo.Set(ctx, ch.ID, "neural_shock", 0))

    got, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Equal(t, &session.InnateSlot{MaxUses: 3}, got["acid_spray"])
    assert.Equal(t, &session.InnateSlot{MaxUses: 0}, got["neural_shock"])
}

func TestPropertyCharacterInnateTechRepo_RoundTrip(t *testing.T) {
    ctx := context.Background()
    charRepo := pgstore.NewCharacterRepository(sharedPool)
    repo := pgstore.NewCharacterInnateTechRepository(sharedPool)

    rapid.Check(t, func(rt *rapid.T) {
        ch := createTestCharacter(t, charRepo, ctx)
        techID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
        maxUses := rapid.IntRange(0, 10).Draw(rt, "maxUses")
        if err := repo.Set(ctx, ch.ID, techID, maxUses); err != nil {
            rt.Fatalf("Set: %v", err)
        }
        got, err := repo.GetAll(ctx, ch.ID)
        if err != nil {
            rt.Fatalf("GetAll: %v", err)
        }
        require.Contains(rt, got, techID)
        assert.Equal(rt, maxUses, got[techID].MaxUses)
    })
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/storage/postgres/... -v 2>&1 | head -15
```

Expected: compile error (types undefined).

- [ ] **Step 3: Create CharacterHardwiredTechRepository**

Create `internal/storage/postgres/character_hardwired_tech.go`:

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

// CharacterHardwiredTechRepository persists hardwired technology assignments.
type CharacterHardwiredTechRepository struct {
    db *pgxpool.Pool
}

func NewCharacterHardwiredTechRepository(db *pgxpool.Pool) *CharacterHardwiredTechRepository {
    return &CharacterHardwiredTechRepository{db: db}
}

// GetAll returns all hardwired tech IDs for the character.
// Precondition: characterID > 0.
func (r *CharacterHardwiredTechRepository) GetAll(ctx context.Context, characterID int64) ([]string, error) {
    rows, err := r.db.Query(ctx,
        `SELECT tech_id FROM character_hardwired_technologies WHERE character_id = $1 ORDER BY tech_id`,
        characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("CharacterHardwiredTechRepository.GetAll: %w", err)
    }
    defer rows.Close()
    var ids []string
    for rows.Next() {
        var id string
        if err := rows.Scan(&id); err != nil {
            return nil, fmt.Errorf("CharacterHardwiredTechRepository.GetAll scan: %w", err)
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}

// SetAll replaces all hardwired tech assignments for the character (transactional).
// Precondition: characterID > 0.
func (r *CharacterHardwiredTechRepository) SetAll(ctx context.Context, characterID int64, techIDs []string) error {
    tx, err := r.db.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx) //nolint:errcheck
    if _, err := tx.Exec(ctx,
        `DELETE FROM character_hardwired_technologies WHERE character_id = $1`, characterID,
    ); err != nil {
        return fmt.Errorf("deleting hardwired technologies: %w", err)
    }
    for _, id := range techIDs {
        if _, err := tx.Exec(ctx,
            `INSERT INTO character_hardwired_technologies (character_id, tech_id) VALUES ($1, $2)`,
            characterID, id,
        ); err != nil {
            return fmt.Errorf("inserting hardwired tech %s: %w", id, err)
        }
    }
    return tx.Commit(ctx)
}
```

- [ ] **Step 4: Create CharacterPreparedTechRepository**

Create `internal/storage/postgres/character_prepared_tech.go`:

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/cory-johannsen/mud/internal/game/session"
)

// CharacterPreparedTechRepository persists prepared technology slot assignments.
type CharacterPreparedTechRepository struct {
    db *pgxpool.Pool
}

func NewCharacterPreparedTechRepository(db *pgxpool.Pool) *CharacterPreparedTechRepository {
    return &CharacterPreparedTechRepository{db: db}
}

// GetAll returns prepared slot assignments keyed by slot level.
// Precondition: characterID > 0.
func (r *CharacterPreparedTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]*session.PreparedSlot, error) {
    rows, err := r.db.Query(ctx,
        `SELECT slot_level, slot_index, tech_id
         FROM character_prepared_technologies
         WHERE character_id = $1
         ORDER BY slot_level, slot_index`,
        characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("CharacterPreparedTechRepository.GetAll: %w", err)
    }
    defer rows.Close()
    result := make(map[int][]*session.PreparedSlot)
    for rows.Next() {
        var level, index int
        var techID string
        if err := rows.Scan(&level, &index, &techID); err != nil {
            return nil, fmt.Errorf("CharacterPreparedTechRepository.GetAll scan: %w", err)
        }
        // Grow slice to fit index if needed
        for len(result[level]) <= index {
            result[level] = append(result[level], nil)
        }
        result[level][index] = &session.PreparedSlot{TechID: techID}
    }
    return result, rows.Err()
}

// Set upserts a single prepared slot.
// Precondition: characterID > 0; level > 0; index >= 0; techID not empty.
func (r *CharacterPreparedTechRepository) Set(ctx context.Context, characterID int64, level, index int, techID string) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO character_prepared_technologies (character_id, slot_level, slot_index, tech_id)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (character_id, slot_level, slot_index) DO UPDATE SET tech_id = EXCLUDED.tech_id`,
        characterID, level, index, techID,
    )
    if err != nil {
        return fmt.Errorf("CharacterPreparedTechRepository.Set: %w", err)
    }
    return nil
}

// DeleteAll removes all prepared slot assignments for the character.
func (r *CharacterPreparedTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
    _, err := r.db.Exec(ctx,
        `DELETE FROM character_prepared_technologies WHERE character_id = $1`, characterID,
    )
    return err
}
```

- [ ] **Step 5: Create CharacterSpontaneousTechRepository**

Create `internal/storage/postgres/character_spontaneous_tech.go`:

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

// CharacterSpontaneousTechRepository persists spontaneous technology known-slot assignments.
type CharacterSpontaneousTechRepository struct {
    db *pgxpool.Pool
}

func NewCharacterSpontaneousTechRepository(db *pgxpool.Pool) *CharacterSpontaneousTechRepository {
    return &CharacterSpontaneousTechRepository{db: db}
}

// GetAll returns known spontaneous techs keyed by tech level.
// Precondition: characterID > 0.
func (r *CharacterSpontaneousTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]string, error) {
    rows, err := r.db.Query(ctx,
        `SELECT tech_id, level
         FROM character_spontaneous_technologies
         WHERE character_id = $1
         ORDER BY level, tech_id`,
        characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("CharacterSpontaneousTechRepository.GetAll: %w", err)
    }
    defer rows.Close()
    result := make(map[int][]string)
    for rows.Next() {
        var techID string
        var level int
        if err := rows.Scan(&techID, &level); err != nil {
            return nil, fmt.Errorf("CharacterSpontaneousTechRepository.GetAll scan: %w", err)
        }
        result[level] = append(result[level], techID)
    }
    return result, rows.Err()
}

// Add inserts a known spontaneous tech.
// Precondition: characterID > 0; techID not empty; level > 0.
func (r *CharacterSpontaneousTechRepository) Add(ctx context.Context, characterID int64, techID string, level int) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO character_spontaneous_technologies (character_id, tech_id, level)
         VALUES ($1, $2, $3)
         ON CONFLICT (character_id, tech_id) DO NOTHING`,
        characterID, techID, level,
    )
    if err != nil {
        return fmt.Errorf("CharacterSpontaneousTechRepository.Add: %w", err)
    }
    return nil
}

// DeleteAll removes all spontaneous tech assignments for the character.
func (r *CharacterSpontaneousTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
    _, err := r.db.Exec(ctx,
        `DELETE FROM character_spontaneous_technologies WHERE character_id = $1`, characterID,
    )
    return err
}
```

- [ ] **Step 6: Create CharacterInnateTechRepository**

Create `internal/storage/postgres/character_innate_tech.go`:

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/cory-johannsen/mud/internal/game/session"
)

// CharacterInnateTechRepository persists innate technology assignments.
type CharacterInnateTechRepository struct {
    db *pgxpool.Pool
}

func NewCharacterInnateTechRepository(db *pgxpool.Pool) *CharacterInnateTechRepository {
    return &CharacterInnateTechRepository{db: db}
}

// GetAll returns innate tech assignments keyed by tech ID.
// Precondition: characterID > 0.
func (r *CharacterInnateTechRepository) GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error) {
    rows, err := r.db.Query(ctx,
        `SELECT tech_id, max_uses
         FROM character_innate_technologies
         WHERE character_id = $1`,
        characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("CharacterInnateTechRepository.GetAll: %w", err)
    }
    defer rows.Close()
    result := make(map[string]*session.InnateSlot)
    for rows.Next() {
        var techID string
        var maxUses int
        if err := rows.Scan(&techID, &maxUses); err != nil {
            return nil, fmt.Errorf("CharacterInnateTechRepository.GetAll scan: %w", err)
        }
        result[techID] = &session.InnateSlot{MaxUses: maxUses}
    }
    return result, rows.Err()
}

// Set upserts an innate tech assignment.
// Precondition: characterID > 0; techID not empty.
func (r *CharacterInnateTechRepository) Set(ctx context.Context, characterID int64, techID string, maxUses int) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO character_innate_technologies (character_id, tech_id, max_uses)
         VALUES ($1, $2, $3)
         ON CONFLICT (character_id, tech_id) DO UPDATE SET max_uses = EXCLUDED.max_uses`,
        characterID, techID, maxUses,
    )
    if err != nil {
        return fmt.Errorf("CharacterInnateTechRepository.Set: %w", err)
    }
    return nil
}

// DeleteAll removes all innate tech assignments for the character.
func (r *CharacterInnateTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
    _, err := r.db.Exec(ctx,
        `DELETE FROM character_innate_technologies WHERE character_id = $1`, characterID,
    )
    return err
}
```

- [ ] **Step 7: Run integration tests**

```bash
go test ./internal/storage/postgres/... -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/storage/postgres/character_technology_repos_test.go \
        internal/storage/postgres/character_hardwired_tech.go \
        internal/storage/postgres/character_prepared_tech.go \
        internal/storage/postgres/character_spontaneous_tech.go \
        internal/storage/postgres/character_innate_tech.go
git commit -m "feat(postgres): character technology slot repositories (hardwired, prepared, spontaneous, innate)"
```

---

## Chunk 3: Assignment Logic and Wiring

### Task 6: assignTechnologies and loadTechnologies

**Files:**
- Create: `internal/gameserver/technology_assignment.go`
- Create: `internal/gameserver/technology_assignment_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/technology_assignment_test.go`:

```go
package gameserver_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/gameserver"
)

// --- fakes ---

type fakeHardwiredRepo struct{ stored []string }
func (r *fakeHardwiredRepo) GetAll(_ context.Context, _ int64) ([]string, error) { return r.stored, nil }
func (r *fakeHardwiredRepo) SetAll(_ context.Context, _ int64, ids []string) error { r.stored = ids; return nil }

type fakePreparedRepo struct{ slots map[int][]*session.PreparedSlot }
func (r *fakePreparedRepo) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) { return r.slots, nil }
func (r *fakePreparedRepo) Set(_ context.Context, _ int64, level, index int, techID string) error {
    if r.slots == nil { r.slots = make(map[int][]*session.PreparedSlot) }
    for len(r.slots[level]) <= index { r.slots[level] = append(r.slots[level], nil) }
    r.slots[level][index] = &session.PreparedSlot{TechID: techID}
    return nil
}
func (r *fakePreparedRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }

type fakeSpontaneousRepo struct{ techs map[int][]string }
func (r *fakeSpontaneousRepo) GetAll(_ context.Context, _ int64) (map[int][]string, error) { return r.techs, nil }
func (r *fakeSpontaneousRepo) Add(_ context.Context, _ int64, techID string, level int) error {
    if r.techs == nil { r.techs = make(map[int][]string) }
    r.techs[level] = append(r.techs[level], techID)
    return nil
}
func (r *fakeSpontaneousRepo) DeleteAll(_ context.Context, _ int64) error { r.techs = nil; return nil }

type fakeInnateRepo struct{ slots map[string]*session.InnateSlot }
func (r *fakeInnateRepo) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) { return r.slots, nil }
func (r *fakeInnateRepo) Set(_ context.Context, _ int64, techID string, maxUses int) error {
    if r.slots == nil { r.slots = make(map[string]*session.InnateSlot) }
    r.slots[techID] = &session.InnateSlot{MaxUses: maxUses}
    return nil
}
func (r *fakeInnateRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }

// noPrompt returns the first option automatically (for testing auto-assign paths)
func noPrompt(options []string) (string, error) {
    if len(options) == 0 { return "", nil }
    return options[0], nil
}

// REQ-TG6: assignTechnologies with full job+archetype populates all four session fields
func TestAssignTechnologies_FullJob(t *testing.T) {
    sess := &session.PlayerSession{}
    job := &ruleset.Job{
        TechnologyGrants: &ruleset.TechnologyGrants{
            Hardwired: []string{"hw_tech"},
            Prepared: &ruleset.PreparedGrants{
                SlotsByLevel: map[int]int{1: 2},
                Fixed:        []ruleset.PreparedEntry{{ID: "fixed_tech", Level: 1}},
                Pool:         []ruleset.PreparedEntry{{ID: "pool_tech", Level: 1}},
            },
            Spontaneous: &ruleset.SpontaneousGrants{
                KnownByLevel: map[int]int{1: 2},
                UsesByLevel:  map[int]int{1: 4},
                Fixed:        []ruleset.SpontaneousEntry{{ID: "spon_fixed", Level: 1}},
                Pool:         []ruleset.SpontaneousEntry{{ID: "spon_pool", Level: 1}},
            },
        },
    }
    archetype := &ruleset.Archetype{
        InnateTechnologies: []ruleset.InnateGrant{
            {ID: "innate_tech", UsesPerDay: 2},
        },
    }

    hwRepo := &fakeHardwiredRepo{}
    prepRepo := &fakePreparedRepo{}
    spontRepo := &fakeSpontaneousRepo{}
    innateRepo := &fakeInnateRepo{}

    err := gameserver.AssignTechnologies(context.Background(), sess, 1, job, archetype,
        nil, noPrompt, hwRepo, prepRepo, spontRepo, innateRepo)
    require.NoError(t, err)

    // Hardwired
    assert.Equal(t, []string{"hw_tech"}, sess.HardwiredTechs)
    // Prepared: 1 fixed + 1 auto-assigned from pool (pool size == open slots)
    require.Len(t, sess.PreparedTechs[1], 2)
    assert.Equal(t, "fixed_tech", sess.PreparedTechs[1][0].TechID)
    assert.Equal(t, "pool_tech", sess.PreparedTechs[1][1].TechID)
    // Spontaneous
    require.Len(t, sess.SpontaneousTechs[1], 2)
    // Innate
    require.NotNil(t, sess.InnateTechs["innate_tech"])
    assert.Equal(t, 2, sess.InnateTechs["innate_tech"].MaxUses)
}

// REQ-TG7: nil TechnologyGrants is a no-op
func TestAssignTechnologies_NilGrants(t *testing.T) {
    sess := &session.PlayerSession{}
    job := &ruleset.Job{TechnologyGrants: nil}
    archetype := &ruleset.Archetype{}

    err := gameserver.AssignTechnologies(context.Background(), sess, 1, job, archetype,
        nil, noPrompt, &fakeHardwiredRepo{}, &fakePreparedRepo{}, &fakeSpontaneousRepo{}, &fakeInnateRepo{})
    require.NoError(t, err)

    assert.Nil(t, sess.HardwiredTechs)
    assert.Nil(t, sess.PreparedTechs)
    assert.Nil(t, sess.SpontaneousTechs)
    assert.Nil(t, sess.InnateTechs)
}

// REQ-TG8: auto-assign prepared when pool == open slots (no prompt)
func TestAssignTechnologies_PreparedAutoAssign(t *testing.T) {
    sess := &session.PlayerSession{}
    promptCalled := false
    job := &ruleset.Job{
        TechnologyGrants: &ruleset.TechnologyGrants{
            Prepared: &ruleset.PreparedGrants{
                SlotsByLevel: map[int]int{1: 1},
                Pool:         []ruleset.PreparedEntry{{ID: "auto_tech", Level: 1}},
            },
        },
    }
    archetype := &ruleset.Archetype{}
    promptFn := func(options []string) (string, error) {
        promptCalled = true
        return options[0], nil
    }

    require.NoError(t, gameserver.AssignTechnologies(context.Background(), sess, 1, job, archetype,
        nil, promptFn, &fakeHardwiredRepo{}, &fakePreparedRepo{}, &fakeSpontaneousRepo{}, &fakeInnateRepo{}))

    assert.False(t, promptCalled, "prompt should not be called when pool == open slots")
    assert.Equal(t, "auto_tech", sess.PreparedTechs[1][0].TechID)
}

// REQ-TG9: auto-assign spontaneous when pool == open slots (no prompt)
func TestAssignTechnologies_SpontaneousAutoAssign(t *testing.T) {
    sess := &session.PlayerSession{}
    promptCalled := false
    job := &ruleset.Job{
        TechnologyGrants: &ruleset.TechnologyGrants{
            Spontaneous: &ruleset.SpontaneousGrants{
                KnownByLevel: map[int]int{1: 1},
                UsesByLevel:  map[int]int{1: 4},
                Pool:         []ruleset.SpontaneousEntry{{ID: "auto_spon", Level: 1}},
            },
        },
    }
    archetype := &ruleset.Archetype{}
    promptFn := func(options []string) (string, error) {
        promptCalled = true
        return options[0], nil
    }

    require.NoError(t, gameserver.AssignTechnologies(context.Background(), sess, 1, job, archetype,
        nil, promptFn, &fakeHardwiredRepo{}, &fakePreparedRepo{}, &fakeSpontaneousRepo{}, &fakeInnateRepo{}))

    assert.False(t, promptCalled)
    assert.Contains(t, sess.SpontaneousTechs[1], "auto_spon")
}

// REQ-TG10: loadTechnologies populates all four session fields from repos
func TestLoadTechnologies(t *testing.T) {
    hwRepo := &fakeHardwiredRepo{stored: []string{"hw_tech"}}
    prepRepo := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
        1: {{TechID: "prep_tech"}},
    }}
    spontRepo := &fakeSpontaneousRepo{techs: map[int][]string{1: {"spon_tech"}}}
    innateRepo := &fakeInnateRepo{slots: map[string]*session.InnateSlot{
        "innate_tech": {MaxUses: 1},
    }}

    sess := &session.PlayerSession{}
    require.NoError(t, gameserver.LoadTechnologies(context.Background(), sess, 1, hwRepo, prepRepo, spontRepo, innateRepo))

    assert.Equal(t, []string{"hw_tech"}, sess.HardwiredTechs)
    require.NotNil(t, sess.PreparedTechs[1])
    assert.Equal(t, "prep_tech", sess.PreparedTechs[1][0].TechID)
    assert.Contains(t, sess.SpontaneousTechs[1], "spon_tech")
    assert.Equal(t, 1, sess.InnateTechs["innate_tech"].MaxUses)
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/gameserver/... -run TestAssignTechnologies -run TestLoadTechnologies -v 2>&1 | head -15
```

Expected: compile error (functions not yet defined).

- [ ] **Step 3: Create technology_assignment.go**

Create `internal/gameserver/technology_assignment.go`:

```go
package gameserver

import (
    "context"

    "github.com/cory-johannsen/mud/internal/game/ruleset"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/technology"
)

// TechPromptFn is a function that presents options to the player and returns the chosen ID.
// Used for interactive slot selection during character creation.
type TechPromptFn func(options []string) (string, error)

// HardwiredTechRepo is the interface required by AssignTechnologies and LoadTechnologies.
type HardwiredTechRepo interface {
    GetAll(ctx context.Context, characterID int64) ([]string, error)
    SetAll(ctx context.Context, characterID int64, techIDs []string) error
}

// PreparedTechRepo is the interface required by AssignTechnologies and LoadTechnologies.
type PreparedTechRepo interface {
    GetAll(ctx context.Context, characterID int64) (map[int][]*session.PreparedSlot, error)
    Set(ctx context.Context, characterID int64, level, index int, techID string) error
    DeleteAll(ctx context.Context, characterID int64) error
}

// SpontaneousTechRepo is the interface required by AssignTechnologies and LoadTechnologies.
type SpontaneousTechRepo interface {
    GetAll(ctx context.Context, characterID int64) (map[int][]string, error)
    Add(ctx context.Context, characterID int64, techID string, level int) error
    DeleteAll(ctx context.Context, characterID int64) error
}

// InnateTechRepo is the interface required by AssignTechnologies and LoadTechnologies.
type InnateTechRepo interface {
    GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error)
    Set(ctx context.Context, characterID int64, techID string, maxUses int) error
    DeleteAll(ctx context.Context, characterID int64) error
}

// AssignTechnologies assigns all technologies from job and archetype grants to sess,
// persisting each assignment via the provided repositories. promptFn is called when the
// player must choose from a pool larger than the remaining open slots; techReg is used
// to look up tech names for display (may be nil, in which case IDs are shown instead).
//
// Precondition: sess is not nil; characterID > 0.
// Postcondition: all four session technology fields are populated; all assignments persisted.
func AssignTechnologies(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    job *ruleset.Job,
    archetype *ruleset.Archetype,
    techReg *technology.Registry,
    promptFn TechPromptFn,
    hwRepo HardwiredTechRepo,
    prepRepo PreparedTechRepo,
    spontRepo SpontaneousTechRepo,
    innateRepo InnateTechRepo,
) error {
    if job == nil || job.TechnologyGrants == nil {
        // No grants — leave session fields nil (consistent with nil-until-loaded pattern)
        return nil
    }
    grants := job.TechnologyGrants

    // 1. Hardwired
    if len(grants.Hardwired) > 0 {
        sess.HardwiredTechs = make([]string, len(grants.Hardwired))
        copy(sess.HardwiredTechs, grants.Hardwired)
        if err := hwRepo.SetAll(ctx, characterID, sess.HardwiredTechs); err != nil {
            return err
        }
    }

    // 2. Innate (from archetype)
    if archetype != nil && len(archetype.InnateTechnologies) > 0 {
        sess.InnateTechs = make(map[string]*session.InnateSlot)
        for _, grant := range archetype.InnateTechnologies {
            sess.InnateTechs[grant.ID] = &session.InnateSlot{MaxUses: grant.UsesPerDay}
            if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
                return err
            }
        }
    }

    // 3. Prepared
    if grants.Prepared != nil {
        sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
        for level, totalSlots := range grants.Prepared.SlotsByLevel {
            slots := make([]*session.PreparedSlot, 0, totalSlots)

            // Pre-fill fixed
            for _, entry := range grants.Prepared.Fixed {
                if entry.Level == level {
                    slots = append(slots, &session.PreparedSlot{TechID: entry.ID})
                }
            }

            // Fill remaining from pool
            poolAtLevel := filterPreparedPool(grants.Prepared.Pool, level)
            open := totalSlots - len(slots)
            if open > 0 {
                chosen, err := fillFromPool(poolAtLevel, open, techReg, promptFn)
                if err != nil {
                    return err
                }
                for _, id := range chosen {
                    slots = append(slots, &session.PreparedSlot{TechID: id})
                }
            }

            sess.PreparedTechs[level] = slots
            for idx, slot := range slots {
                if err := prepRepo.Set(ctx, characterID, level, idx, slot.TechID); err != nil {
                    return err
                }
            }
        }
    }

    // 4. Spontaneous
    if grants.Spontaneous != nil {
        sess.SpontaneousTechs = make(map[int][]string)
        for level, totalKnown := range grants.Spontaneous.KnownByLevel {
            known := make([]string, 0, totalKnown)

            // Pre-fill fixed
            for _, entry := range grants.Spontaneous.Fixed {
                if entry.Level == level {
                    known = append(known, entry.ID)
                }
            }

            // Fill remaining from pool
            poolAtLevel := filterSpontaneousPool(grants.Spontaneous.Pool, level)
            open := totalKnown - len(known)
            if open > 0 {
                chosen, err := fillFromPool(poolAtLevel, open, techReg, promptFn)
                if err != nil {
                    return err
                }
                known = append(known, chosen...)
            }

            sess.SpontaneousTechs[level] = known
            for _, id := range known {
                if err := spontRepo.Add(ctx, characterID, id, level); err != nil {
                    return err
                }
            }
        }
    }

    return nil
}

// LoadTechnologies loads all four technology slot types from their repos into sess.
//
// Precondition: sess is not nil; characterID > 0.
func LoadTechnologies(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    hwRepo HardwiredTechRepo,
    prepRepo PreparedTechRepo,
    spontRepo SpontaneousTechRepo,
    innateRepo InnateTechRepo,
) error {
    hw, err := hwRepo.GetAll(ctx, characterID)
    if err != nil {
        return err
    }
    sess.HardwiredTechs = hw

    prep, err := prepRepo.GetAll(ctx, characterID)
    if err != nil {
        return err
    }
    sess.PreparedTechs = prep

    spont, err := spontRepo.GetAll(ctx, characterID)
    if err != nil {
        return err
    }
    sess.SpontaneousTechs = spont

    innate, err := innateRepo.GetAll(ctx, characterID)
    if err != nil {
        return err
    }
    sess.InnateTechs = innate

    return nil
}

// fillFromPool selects `open` tech IDs from the pool.
// If len(pool) == open, auto-assigns without prompting.
// If len(pool) > open, calls promptFn for each selection.
func fillFromPool(pool []string, open int, techReg *technology.Registry, promptFn TechPromptFn) ([]string, error) {
    if len(pool) == 0 || open == 0 {
        return nil, nil
    }
    if len(pool) == open {
        return pool, nil
    }
    // Need to prompt
    chosen := make([]string, 0, open)
    remaining := make([]string, len(pool))
    copy(remaining, pool)

    for i := 0; i < open; i++ {
        // Build display options (ID or "Name — Description" if registry available)
        options := make([]string, len(remaining))
        for j, id := range remaining {
            if techReg != nil {
                if def, ok := techReg.Get(id); ok {
                    options[j] = def.ID + " — " + def.Description
                } else {
                    options[j] = id
                }
            } else {
                options[j] = id
            }
        }

        selected, err := promptFn(options)
        if err != nil {
            return chosen, err
        }
        // Extract ID from display string (before " — ")
        techID := selected
        for _, id := range remaining {
            display := id
            if techReg != nil {
                if def, ok := techReg.Get(id); ok {
                    display = def.ID + " — " + def.Description
                }
            }
            if display == selected {
                techID = id
                break
            }
        }
        chosen = append(chosen, techID)
        // Remove chosen from remaining
        next := remaining[:0]
        for _, id := range remaining {
            if id != techID {
                next = append(next, id)
            }
        }
        remaining = next
    }
    return chosen, nil
}

func filterPreparedPool(pool []ruleset.PreparedEntry, level int) []string {
    var out []string
    for _, e := range pool {
        if e.Level == level {
            out = append(out, e.ID)
        }
    }
    return out
}

func filterSpontaneousPool(pool []ruleset.SpontaneousEntry, level int) []string {
    var out []string
    for _, e := range pool {
        if e.Level == level {
            out = append(out, e.ID)
        }
    }
    return out
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/gameserver/... -run TestAssignTechnologies -run TestLoadTechnologies -v 2>&1
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|error" | head -10
```

Expected: no failures.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
git commit -m "feat(gameserver): AssignTechnologies and LoadTechnologies"
```

---

### Task 7: GameServiceServer wiring and FEATURES.md

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Add repo fields to GameServiceServer**

In `internal/gameserver/grpc_service.go`, add four fields to the `GameServiceServer` struct after `techRegistry`:

```go
hardwiredTechRepo   HardwiredTechRepo
preparedTechRepo    PreparedTechRepo
spontaneousTechRepo SpontaneousTechRepo
innateTechRepo      InnateTechRepo
```

- [ ] **Step 2: Add parameters to NewGameServiceServer**

Add four parameters to `NewGameServiceServer` after `techRegistry *technology.Registry`:

```go
hardwiredTechRepo   HardwiredTechRepo,
preparedTechRepo    PreparedTechRepo,
spontaneousTechRepo SpontaneousTechRepo,
innateTechRepo      InnateTechRepo,
```

Wire them in the struct literal:

```go
hardwiredTechRepo:   hardwiredTechRepo,
preparedTechRepo:    preparedTechRepo,
spontaneousTechRepo: spontaneousTechRepo,
innateTechRepo:      innateTechRepo,
```

- [ ] **Step 3: Add nil args to all test call sites**

Search for all test files calling `NewGameServiceServer`:

```bash
grep -rn "NewGameServiceServer" internal/gameserver/ --include="*_test.go" -l
```

For each file, search for the `NewGameServiceServer(` call site and locate the `techRegistry` argument (search for `techReg` or `nil` at the position where the tech registry was added in the previous worktree commit). Insert four `nil` arguments immediately after the `techRegistry` argument and before `loadoutsDir`. Verify the build passes before committing.

The pattern to find and replace: look for lines with `techReg,` or `nil, // techRegistry` followed by `*loadoutsDir` or a string. Insert `nil, nil, nil, nil,` between them.

**Important:** After modifying, build to confirm before running tests:

```bash
go build ./internal/gameserver/... 2>&1
```

- [ ] **Step 4: Wire loadTechnologies into session startup**

In `internal/gameserver/grpc_service.go`, find the `loadClassFeatures` call (or the block that populates `PassiveFeats`). After the `FeatureChoices` initialization block, add a call to `LoadTechnologies`:

```go
if s.hardwiredTechRepo != nil {
    if techErr := LoadTechnologies(stream.Context(), sess, characterID,
        s.hardwiredTechRepo, s.preparedTechRepo, s.spontaneousTechRepo, s.innateTechRepo,
    ); techErr != nil {
        s.logger.Warn("loading technologies", zap.Int64("character_id", characterID), zap.Error(techErr))
    }
}
```

- [ ] **Step 5: Wire assignTechnologies into character creation**

In `internal/gameserver/grpc_service.go`, find the block that calls `charAbilityBoostsRepo` / ability boost prompting. After that block completes, add technology assignment for new characters (characters with no existing technology slots):

```go
// Assign technologies at character creation (only if no slots yet assigned)
if s.hardwiredTechRepo != nil && s.jobRegistry != nil {
    existingHW, hwCheckErr := s.hardwiredTechRepo.GetAll(stream.Context(), characterID)
    if hwCheckErr == nil && len(existingHW) == 0 {
        if job, ok := s.jobRegistry.Job(sess.Class); ok {
            archetype := s.archetypes[job.Archetype]
            promptFn := func(options []string) (string, error) {
                // Build FeatureChoices-style prompt using promptFeatureChoice
                choices := &ruleset.FeatureChoices{
                    Prompt:  "Choose a technology:",
                    Options: options,
                    Key:     "tech_choice",
                }
                return s.promptFeatureChoice(stream, "tech_choice", choices)
            }
            if assignErr := AssignTechnologies(stream.Context(), sess, characterID,
                job, archetype, s.techRegistry, promptFn,
                s.hardwiredTechRepo, s.preparedTechRepo, s.spontaneousTechRepo, s.innateTechRepo,
            ); assignErr != nil {
                s.logger.Warn("assigning technologies", zap.Int64("character_id", characterID), zap.Error(assignErr))
            }
        }
    }
}
```

- [ ] **Step 6: Update cmd/gameserver/main.go**

After the `techReg` loading block, construct the four repositories and inject them into `NewGameServiceServer`:

```go
hardwiredTechRepo := postgres.NewCharacterHardwiredTechRepository(pool.DB())
preparedTechRepo := postgres.NewCharacterPreparedTechRepository(pool.DB())
spontaneousTechRepo := postgres.NewCharacterSpontaneousTechRepository(pool.DB())
innateTechRepo := postgres.NewCharacterInnateTechRepository(pool.DB())
```

Add these four to the `NewGameServiceServer` call after `techReg`:

```go
hardwiredTechRepo, preparedTechRepo, spontaneousTechRepo, innateTechRepo,
```

- [ ] **Step 7: Build and test**

```bash
go build ./... 2>&1
go test ./... 2>&1 | grep -E "FAIL|error" | head -10
```

Expected: build succeeds, no test failures.

- [ ] **Step 8: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, find the Technology section and add after the existing `[ ]` items:

```
    - [ ] Level-up technology selection — player chooses new technologies when levelling up (prepared/spontaneous pool expands; player selects additions interactively)
```

- [ ] **Step 9: Commit**

```bash
git add internal/gameserver/grpc_service.go cmd/gameserver/main.go docs/requirements/FEATURES.md
git commit -m "feat(gameserver): wire technology repos into GameServiceServer; assign and load technologies at creation/login"
```

---

### Task 8: Final verification

- [ ] **Step 1: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all packages pass, 0 failures.

- [ ] **Step 2: Build binary**

```bash
go build ./cmd/gameserver/... 2>&1
```

Expected: no errors.

- [ ] **Step 3: Verify new --flags appear**

```bash
go run ./cmd/gameserver/... --help 2>&1 | grep tech
```

Expected: `--tech-content-dir` flag visible (already present).

- [ ] **Step 4: Done**

All requirements satisfied:
- REQ-TG1: cantrip → hardwired rename (Task 1)
- REQ-TG2–TG5: YAML round-trip tests (Task 2)
- REQ-TG6–TG10: assignment + loading tests (Task 6)
- REQ-TG11: property round-trip test (Task 2)
- REQ-TG12: pool validation test (Task 2)
- DB migration: 4 tables (Task 4)
- Repos: 4 implementations (Task 5)
- Session fields: 4 new fields (Task 3)
- GameServiceServer: wired + login loading + creation assignment (Task 7)
- FEATURES.md: level-up item added (Task 7)
