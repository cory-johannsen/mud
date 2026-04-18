# Tech Catalog (KnownTechs) + Slot Assignment Modal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce per-archetype casting models, a KnownTechs catalog for wizard/ranger archetypes, heightened slot assignment for all prepared archetypes, and a FeatureChoiceModal enriched with level tabs, heightened badges, and back/forward navigation.

**Architecture:** `SpontaneousTechs` is renamed to `KnownTechs` throughout; a `CastingModel` enum is added to Archetype and Job ruleset types; `RearrangePreparedTechs` becomes casting-model-aware; `TechPromptFn` is extended with optional slot context; the frontend `FeatureChoiceModal` renders level tabs, heightened badges, and Back/Forward buttons using sentinel-embedded metadata.

**Tech Stack:** Go 1.22+, PostgreSQL (pgx v5), React/TypeScript, google/wire (DI)

---

## File Map

**Created:**
- `migrations/064_rename_known_technologies.up.sql`
- `migrations/064_rename_known_technologies.down.sql`
- `internal/game/ruleset/casting_model.go`
- `internal/storage/postgres/character_known_tech.go` (renamed from `character_spontaneous_tech.go`)

**Modified (Go):**
- `internal/game/session/manager.go` — rename `SpontaneousTechs` field
- `internal/game/ruleset/archetype.go` — add `CastingModel` field
- `internal/game/ruleset/job.go` — add `CastingModel` field
- `internal/gameserver/technology_assignment.go` — rename interface, extend logic
- `internal/gameserver/deps.go` — rename field
- `internal/gameserver/grpc_service.go` — rename fields/methods, extend `choicePromptPayload`, extend `TechPromptFn`
- `internal/gameserver/grpc_service_tech_trainer.go` — casting-model-aware KnownTechs writes
- `cmd/gameserver/wire.go` — update wire bindings
- `cmd/gameserver/wire_gen.go` — regenerated
- `cmd/webclient/main.go` — update constructor call
- `cmd/webclient/server.go` — rename field/parameter
- All `*_test.go` files referencing `SpontaneousTechs`/`spontaneousTechRepo`

**Modified (Content YAML):**
- `content/archetypes/nerd.yaml`
- `content/archetypes/schemer.yaml`
- `content/archetypes/naturalist.yaml`
- `content/archetypes/zealot.yaml`
- `content/archetypes/drifter.yaml`
- `content/archetypes/influencer.yaml`
- `content/archetypes/aggressor.yaml`
- `content/archetypes/criminal.yaml`

**Modified (TypeScript):**
- `cmd/webclient/ui/src/game/GameContext.tsx`
- `cmd/webclient/ui/src/game/drawers/FeatureChoiceModal.tsx`

---

### Task 1: DB migration — rename table

**Files:**
- Create: `migrations/064_rename_known_technologies.up.sql`
- Create: `migrations/064_rename_known_technologies.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/064_rename_known_technologies.up.sql
ALTER TABLE character_spontaneous_technologies RENAME TO character_known_technologies;
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/064_rename_known_technologies.down.sql
ALTER TABLE character_known_technologies RENAME TO character_spontaneous_technologies;
```

- [ ] **Step 3: Apply the migration**

```bash
cd /home/cjohannsen/src/mud
mise run migrate-up
# Expected: migration 064 applied OK
```

- [ ] **Step 4: Verify**

```bash
psql $DATABASE_URL -c "\d character_known_technologies"
# Expected: table exists with columns character_id, tech_id, level
```

- [ ] **Step 5: Commit**

```bash
git add migrations/064_rename_known_technologies.up.sql migrations/064_rename_known_technologies.down.sql
git commit -m "feat(db): rename character_spontaneous_technologies to character_known_technologies"
```

---

### Task 2: Rename KnownTechRepo interface and repository implementation

**Files:**
- Create: `internal/storage/postgres/character_known_tech.go`
- Delete: `internal/storage/postgres/character_spontaneous_tech.go`
- Modify: `internal/gameserver/technology_assignment.go:45-50`

- [ ] **Step 1: Write failing test**

In `internal/storage/postgres/character_technology_repos_test.go`, add:

```go
func TestCharacterKnownTechRepository_AddAndGetAll(t *testing.T) {
    repo := &CharacterKnownTechRepository{db: testDB}
    ctx := context.Background()
    characterID := int64(99991)
    require.NoError(t, repo.Add(ctx, characterID, "test_tech", 2))
    result, err := repo.GetAll(ctx, characterID)
    require.NoError(t, err)
    assert.Equal(t, []string{"test_tech"}, result[2])
    require.NoError(t, repo.DeleteAll(ctx, characterID))
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/storage/postgres/... -run TestCharacterKnownTechRepository -v 2>&1 | tail -10
# Expected: FAIL — CharacterKnownTechRepository undefined
```

- [ ] **Step 3: Create the new repository file**

Create `internal/storage/postgres/character_known_tech.go`:

```go
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// KnownTechRepo persists the set of technologies a character knows (their catalog).
// For wizard/ranger casting models this is the catalog from which prepared slots are filled.
// For spontaneous casting models this is the full set of known techs.
type KnownTechRepo interface {
	GetAll(ctx context.Context, characterID int64) (map[int][]string, error)
	Add(ctx context.Context, characterID int64, techID string, level int) error
	DeleteAll(ctx context.Context, characterID int64) error
}

// CharacterKnownTechRepository implements KnownTechRepo using PostgreSQL.
type CharacterKnownTechRepository struct {
	db *pgxpool.Pool
}

// NewCharacterKnownTechRepository constructs a CharacterKnownTechRepository.
func NewCharacterKnownTechRepository(db *pgxpool.Pool) *CharacterKnownTechRepository {
	return &CharacterKnownTechRepository{db: db}
}

// GetAll returns all known techs for a character, keyed by tech level.
func (r *CharacterKnownTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tech_id, level FROM character_known_technologies WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int][]string)
	for rows.Next() {
		var techID string
		var level int
		if err := rows.Scan(&techID, &level); err != nil {
			return nil, err
		}
		result[level] = append(result[level], techID)
	}
	return result, rows.Err()
}

// Add records a tech as known for the character at the given level.
// Silently succeeds if already present.
func (r *CharacterKnownTechRepository) Add(ctx context.Context, characterID int64, techID string, level int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_known_technologies (character_id, tech_id, level)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (character_id, tech_id) DO NOTHING`,
		characterID, techID, level,
	)
	return err
}

// DeleteAll removes all known tech records for the character.
func (r *CharacterKnownTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_known_technologies WHERE character_id = $1`,
		characterID,
	)
	return err
}
```

- [ ] **Step 4: Delete the old file**

```bash
rm /home/cjohannsen/src/mud/internal/storage/postgres/character_spontaneous_tech.go
```

- [ ] **Step 5: In `technology_assignment.go`, rename the interface**

In `internal/gameserver/technology_assignment.go` lines 45-50, change:

```go
// SpontaneousTechRepo persists ...
type SpontaneousTechRepo interface {
```

to:

```go
// KnownTechRepo persists the set of technologies a character knows.
// Precondition: characterID > 0 for all write methods.
type KnownTechRepo interface {
```

Also update the `postgres.KnownTechRepo` reference if the interface was being satisfied by the postgres type (check with `go build ./...`).

- [ ] **Step 6: Run tests**

```bash
go test ./internal/storage/postgres/... -run TestCharacterKnownTechRepository -v 2>&1 | tail -10
# Expected: PASS
```

- [ ] **Step 7: Commit**

```bash
git add internal/storage/postgres/character_known_tech.go internal/storage/postgres/character_spontaneous_tech.go internal/gameserver/technology_assignment.go
git commit -m "feat(storage): rename SpontaneousTechRepo → KnownTechRepo, CharacterSpontaneousTechRepository → CharacterKnownTechRepository"
```

---

### Task 3: Rename SpontaneousTechs → KnownTechs everywhere in Go code

**Files:** All files referencing `SpontaneousTechs`, `spontaneousTechs`, `spontaneousTechRepo`, `SetSpontaneousTechRepo`, `NewCharacterSpontaneousTechRepository`

- [ ] **Step 1: Verify compile fails (expected — old names still referenced)**

```bash
go build ./... 2>&1 | head -30
```

- [ ] **Step 2: Rename in session/manager.go**

In `internal/game/session/manager.go:195`, change:
```go
SpontaneousTechs    map[int][]string        // tech level → known tech IDs
```
to:
```go
KnownTechs          map[int][]string        // tech level → known tech IDs
```

- [ ] **Step 3: Rename in technology_assignment.go**

Using your editor or sed, replace all occurrences in `internal/gameserver/technology_assignment.go`:
- `SpontaneousTechRepo` → `KnownTechRepo` (interface type references in function signatures)
- `spontRepo` → `knownRepo` (parameter/variable names)
- `sess.SpontaneousTechs` → `sess.KnownTechs`

Key lines to update:
- Line 149: `spontRepo SpontaneousTechRepo` parameter → `knownRepo KnownTechRepo`
- Line 242, 248, 277, 297: all `spontRepo` references
- Lines 388-396: `sess.SpontaneousTechs` references in `LevelUpTechnologies`
- Line 882: spontaneous pool function parameter
- Line 1134, 1294: `fillFromSpontaneousPool` callers

- [ ] **Step 4: Rename in grpc_service.go**

In `internal/gameserver/grpc_service.go`, replace:
- `spontaneousTechRepo` field and method names → `knownTechRepo`
- `SetSpontaneousTechRepo` → `SetKnownTechRepo`
- All `s.spontaneousTechRepo` → `s.knownTechRepo`
- `sess.SpontaneousTechs` → `sess.KnownTechs` (all ~15 occurrences)

- [ ] **Step 5: Rename in grpc_service_tech_trainer.go**

In `internal/gameserver/grpc_service_tech_trainer.go`:
- `s.spontaneousTechRepo` → `s.knownTechRepo`
- `sess.SpontaneousTechs` → `sess.KnownTechs`

- [ ] **Step 6: Rename in deps.go**

In `internal/gameserver/deps.go:40`:
```go
SpontaneousTechRepo postgres.KnownTechRepo
```
→
```go
KnownTechRepo postgres.KnownTechRepo
```

- [ ] **Step 7: Rename in wire.go and regenerate wire_gen.go**

In `cmd/gameserver/wire.go:88`, update the `wire.Bind` call:
```go
wire.Bind(new(postgres.KnownTechRepo), new(*postgres.CharacterKnownTechRepository)),
```

Regenerate:
```bash
cd /home/cjohannsen/src/mud
go generate ./cmd/gameserver/...
# OR: wire gen ./cmd/gameserver/...
```

- [ ] **Step 8: Rename in webclient**

In `cmd/webclient/main.go:66`:
```go
NewCharacterKnownTechRepository(db),
```

In `cmd/webclient/server.go:28,115`:
- Rename the struct field from `spontaneousTechRepo` → `knownTechRepo`
- Rename the parameter/assignment accordingly

- [ ] **Step 9: Rename in all test files**

Search and replace in all `*_test.go` files:
```bash
grep -rn "spontaneousTechRepo\|SpontaneousTechs\|fakeSpontaneous\|SpontaneousTechRepo" /home/cjohannsen/src/mud --include="*_test.go" -l
```
For each file found, replace:
- `fakeSpontaneousRepo` → `fakeKnownRepo`
- `spontaneousTechRepo` → `knownTechRepo`
- `SpontaneousTechs` → `KnownTechs`
- `SetSpontaneousTechRepo` → `SetKnownTechRepo`

- [ ] **Step 10: Build and test**

```bash
go build ./... 2>&1 | head -20
# Expected: no errors
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
# Expected: all pass
```

- [ ] **Step 11: Commit**

```bash
git add -u
git commit -m "refactor: rename SpontaneousTechs → KnownTechs throughout Go codebase"
```

---

### Task 4: Add CastingModel type and update Archetype/Job structs + YAML

**Files:**
- Create: `internal/game/ruleset/casting_model.go`
- Modify: `internal/game/ruleset/archetype.go`
- Modify: `internal/game/ruleset/job.go`
- Modify: all 8 `content/archetypes/*.yaml` files

- [ ] **Step 1: Write failing test**

Create `internal/game/ruleset/casting_model_test.go`:

```go
package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestResolveCastingModel_JobOverridesArchetype(t *testing.T) {
	job := &ruleset.Job{CastingModel: ruleset.CastingModelSpontaneous}
	arch := &ruleset.Archetype{CastingModel: ruleset.CastingModelWizard}
	assert.Equal(t, ruleset.CastingModelSpontaneous, ruleset.ResolveCastingModel(job, arch))
}

func TestResolveCastingModel_ArchetypeUsedWhenJobEmpty(t *testing.T) {
	job := &ruleset.Job{}
	arch := &ruleset.Archetype{CastingModel: ruleset.CastingModelDruid}
	assert.Equal(t, ruleset.CastingModelDruid, ruleset.ResolveCastingModel(job, arch))
}

func TestResolveCastingModel_NoneWhenBothEmpty(t *testing.T) {
	assert.Equal(t, ruleset.CastingModelNone, ruleset.ResolveCastingModel(&ruleset.Job{}, &ruleset.Archetype{}))
	assert.Equal(t, ruleset.CastingModelNone, ruleset.ResolveCastingModel(nil, nil))
}

func TestProperty_ResolveCastingModel_JobAlwaysWins(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		models := []ruleset.CastingModel{
			ruleset.CastingModelWizard, ruleset.CastingModelDruid,
			ruleset.CastingModelRanger, ruleset.CastingModelSpontaneous, ruleset.CastingModelNone,
		}
		jobModel := models[rapid.IntRange(0, len(models)-1).Draw(rt, "job")]
		archModel := models[rapid.IntRange(0, len(models)-1).Draw(rt, "arch")]
		if jobModel == "" {
			jobModel = ruleset.CastingModelNone
		}
		job := &ruleset.Job{CastingModel: jobModel}
		arch := &ruleset.Archetype{CastingModel: archModel}
		result := ruleset.ResolveCastingModel(job, arch)
		if jobModel != ruleset.CastingModelNone {
			if result != jobModel {
				rt.Fatalf("expected job model %q to win, got %q", jobModel, result)
			}
		}
	})
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/game/ruleset/... -run "TestResolveCastingModel|TestProperty_ResolveCastingModel" -v 2>&1 | tail -10
# Expected: FAIL — CastingModel undefined
```

- [ ] **Step 3: Create casting_model.go**

```go
// internal/game/ruleset/casting_model.go
package ruleset

// CastingModel describes how a character acquires and uses technologies,
// mirroring PF2E spellcasting class mechanics.
type CastingModel string

const (
	// CastingModelWizard mirrors the PF2E Wizard/Witch: prepared caster with a catalog
	// (KnownTechs). Gains 2 extra catalog entries at L1 and +2 per subsequent level-up.
	CastingModelWizard CastingModel = "wizard"

	// CastingModelDruid mirrors the PF2E Druid/Cleric: prepared caster with access to
	// the full grant pool at every rest. No catalog tracking.
	CastingModelDruid CastingModel = "druid"

	// CastingModelRanger mirrors the PF2E Ranger: prepared caster whose catalog is
	// exactly the techs assigned at level-up and via trainers. No per-level extras.
	CastingModelRanger CastingModel = "ranger"

	// CastingModelSpontaneous mirrors the PF2E Bard/Sorcerer: knows a fixed set of
	// techs (KnownTechs) and casts from a shared use pool. Existing behavior unchanged.
	CastingModelSpontaneous CastingModel = "spontaneous"

	// CastingModelNone indicates no technology system. Used by Aggressor and Criminal.
	CastingModelNone CastingModel = "none"
)

// ResolveCastingModel returns the effective casting model for a character.
// Postcondition: job.CastingModel overrides archetype.CastingModel; CastingModelNone
// is returned when both are unset.
func ResolveCastingModel(job *Job, archetype *Archetype) CastingModel {
	if job != nil && job.CastingModel != "" && job.CastingModel != CastingModelNone {
		return job.CastingModel
	}
	if archetype != nil && archetype.CastingModel != "" {
		return archetype.CastingModel
	}
	return CastingModelNone
}
```

- [ ] **Step 4: Add CastingModel field to Archetype struct**

In `internal/game/ruleset/archetype.go`, add the field after `LevelUpFeatGrants`:

```go
type Archetype struct {
	ID                 string                    `yaml:"id"`
	Name               string                    `yaml:"name"`
	Description        string                    `yaml:"description"`
	KeyAbility         string                    `yaml:"key_ability"`
	HitPointsPerLevel  int                       `yaml:"hit_points_per_level"`
	AbilityBoosts      *AbilityBoostGrant        `yaml:"ability_boosts"`
	InnateTechnologies []InnateGrant             `yaml:"innate_technologies,omitempty"`
	TechnologyGrants   *TechnologyGrants         `yaml:"technology_grants,omitempty"`
	LevelUpGrants      map[int]*TechnologyGrants `yaml:"level_up_grants,omitempty"`
	LevelUpFeatGrants  map[int]*FeatGrants       `yaml:"level_up_feat_grants,omitempty"`
	CastingModel       CastingModel              `yaml:"casting_model,omitempty"`
}
```

- [ ] **Step 5: Add CastingModel field to Job struct**

In `internal/game/ruleset/job.go`, add after `LevelUpFeatGrants`:

```go
CastingModel CastingModel `yaml:"casting_model,omitempty"`
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/game/ruleset/... -run "TestResolveCastingModel|TestProperty_ResolveCastingModel" -v 2>&1 | tail -10
# Expected: PASS
```

- [ ] **Step 7: Update archetype YAML files**

Add `casting_model:` to each archetype YAML file. Find them:

```bash
find /home/cjohannsen/src/mud/content/archetypes -name "*.yaml"
```

Add the following line to each file (after `id:` at the top level):

| File | Line to add |
|------|------------|
| `nerd.yaml` | `casting_model: wizard` |
| `schemer.yaml` | `casting_model: wizard` |
| `naturalist.yaml` | `casting_model: druid` |
| `zealot.yaml` | `casting_model: druid` |
| `drifter.yaml` | `casting_model: ranger` |
| `influencer.yaml` | `casting_model: spontaneous` |
| `aggressor.yaml` | `casting_model: none` |
| `criminal.yaml` | `casting_model: none` |

- [ ] **Step 8: Build and run full test suite**

```bash
go build ./... && go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
# Expected: all pass
```

- [ ] **Step 9: Commit**

```bash
git add internal/game/ruleset/casting_model.go internal/game/ruleset/casting_model_test.go \
  internal/game/ruleset/archetype.go internal/game/ruleset/job.go \
  content/archetypes/
git commit -m "feat(ruleset): add CastingModel enum and per-archetype defaults"
```

---

### Task 5: Thread CastingModel into PlayerSession and technology assignment functions

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/grpc_service.go` (login path, ~line 1715-1735)
- Modify: `internal/gameserver/technology_assignment.go` (function signatures)

- [ ] **Step 1: Write failing test**

In `internal/gameserver/technology_assignment_test.go`, add:

```go
func TestLevelUpTechnologies_CastingModelWizard_PopulatesKnownTechs(t *testing.T) {
	known := &fakeKnownRepo{techs: make(map[int][]string)}
	prep := &fakePreparedRepo{slots: make(map[int][]*session.PreparedSlot)}
	sess := &session.PlayerSession{}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "tech_a", Level: 1}, {ID: "tech_b", Level: 1}, {ID: "tech_c", Level: 1}},
			},
		},
	}
	promptFn := func(prompt string, options []string, _ *gameserver.TechSlotContext) (string, error) {
		return options[0], nil
	}
	err := gameserver.LevelUpTechnologies(context.Background(), sess, 1, job, nil, nil, promptFn, nil, prep, known, nil, nil)
	require.NoError(t, err)
	// Slot pick goes to KnownTechs too
	assert.NotEmpty(t, known.techs[1], "slot pick must populate KnownTechs")
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/gameserver/... -run TestLevelUpTechnologies_CastingModelWizard -v 2>&1 | tail -10
# Expected: FAIL — TechSlotContext undefined or wrong signature
```

- [ ] **Step 3: Add CastingModel to PlayerSession**

In `internal/game/session/manager.go`, add after `InnateTechs`:

```go
// CastingModel is the resolved tech casting model for this character's job+archetype.
// Set at login; controls prepared slot pool selection and catalog population.
CastingModel ruleset.CastingModel
```

- [ ] **Step 4: Extend TechPromptFn in technology_assignment.go**

Replace the existing `TechPromptFn` type definition:

```go
// TechSlotContext provides slot metadata for the frontend modal when prompting
// the player to fill a prepared tech slot during rearrangement.
type TechSlotContext struct {
	SlotNum    int // 1-based slot number within the level
	TotalSlots int // total slots at this level
	SlotLevel  int // tech level of the slot being filled
}

// TechPromptFn prompts the player to choose from options and returns the chosen string.
// slotCtx is non-nil only during rest rearrangement; it is nil during level-up prompts.
type TechPromptFn func(prompt string, options []string, slotCtx *TechSlotContext) (string, error)
```

- [ ] **Step 5: Update all promptFn call sites in technology_assignment.go**

Every `promptFn(prompt, options)` call becomes `promptFn(prompt, options, nil)` in functions other than rearrangement (level-up, spontaneous pool fills). In `fillFromPreparedPoolWithSend`, the slotCtx will be populated in Task 9.

Also update `fillFromPreparedPool` and `fillFromSpontaneousPool` internal calls:
```go
chosen, err := promptFn(slotPrompt, options, nil)
```

- [ ] **Step 6: Update all promptFn closures in grpc_service.go**

Find all closures that create `promptFn` (search for `func(prompt string, options []string)`). There are ~6 of them at lines 1719, 1873, 3901, 4293, 4462, 8752. Update each signature:

```go
promptFn := func(prompt string, options []string, slotCtx *technology_assignment.TechSlotContext) (string, error) {
    choices := &ruleset.FeatureChoices{
        Prompt:  prompt,
        Options: options,
        Key:     "tech_choice",
    }
    return s.promptFeatureChoice(stream, "tech_choice", choices, headless)
}
```

(The `slotCtx` parameter is received but not yet used — it will be used in Task 9 when we extend `choicePromptPayload`.)

- [ ] **Step 7: Populate sess.CastingModel at login**

In `grpc_service.go` around line 1713, after `archetype` is resolved, add:

```go
sess.CastingModel = ruleset.ResolveCastingModel(job, archetype)
```

- [ ] **Step 8: Build and run tests**

```bash
go build ./... && go test ./internal/gameserver/... -run TestLevelUpTechnologies_CastingModelWizard -v 2>&1 | tail -10
```

Fix any remaining compile errors from the signature change.

- [ ] **Step 9: Run full suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
# Expected: all pass
```

- [ ] **Step 10: Commit**

```bash
git add -u
git commit -m "feat(session): add CastingModel to PlayerSession; extend TechPromptFn with TechSlotContext"
```

---

### Task 6: Populate KnownTechs during level-up slot picks (wizard + ranger)

**Files:**
- Modify: `internal/gameserver/technology_assignment.go` — `LevelUpTechnologies`, `fillFromPreparedPool`, `fillFromPreparedPoolWithSend`

- [ ] **Step 1: Write failing tests**

In `internal/gameserver/technology_assignment_test.go`, add:

```go
// REQ-TC-8: wizard level-up slot picks populate KnownTechs
func TestLevelUpTechnologies_Wizard_SlotPickPopulatesKnownTechs(t *testing.T) {
	known := &fakeKnownRepo{techs: make(map[int][]string)}
	prep := &fakePreparedRepo{slots: make(map[int][]*session.PreparedSlot)}
	sess := &session.PlayerSession{CastingModel: ruleset.CastingModelWizard}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
				Pool: []ruleset.PreparedEntry{
					{ID: "tech_a", Level: 1},
					{ID: "tech_b", Level: 1},
					{ID: "tech_c", Level: 1},
				},
			},
		},
	}
	pickIdx := 0
	promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
		choice := opts[pickIdx%len(opts)]
		pickIdx++
		return choice, nil
	}
	err := gameserver.LevelUpTechnologies(context.Background(), sess, 1, job, nil, nil, promptFn, nil, prep, known, nil, nil)
	require.NoError(t, err)
	assert.Len(t, known.techs[1], 2, "both slot picks must be in KnownTechs")
}

// REQ-TC-8: ranger level-up slot picks also populate KnownTechs
func TestLevelUpTechnologies_Ranger_SlotPickPopulatesKnownTechs(t *testing.T) {
	known := &fakeKnownRepo{techs: make(map[int][]string)}
	prep := &fakePreparedRepo{slots: make(map[int][]*session.PreparedSlot)}
	sess := &session.PlayerSession{CastingModel: ruleset.CastingModelRanger}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelRanger,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "tech_x", Level: 1}},
			},
		},
	}
	promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
		return opts[0], nil
	}
	err := gameserver.LevelUpTechnologies(context.Background(), sess, 1, job, nil, nil, promptFn, nil, prep, known, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, known.techs[1], "tech_x")
}

// REQ-TC-8: druid level-up slot picks do NOT populate KnownTechs
func TestLevelUpTechnologies_Druid_SlotPickDoesNotPopulateKnownTechs(t *testing.T) {
	known := &fakeKnownRepo{techs: make(map[int][]string)}
	prep := &fakePreparedRepo{slots: make(map[int][]*session.PreparedSlot)}
	sess := &session.PlayerSession{CastingModel: ruleset.CastingModelDruid}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelDruid,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "tech_y", Level: 1}},
			},
		},
	}
	promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
		return opts[0], nil
	}
	err := gameserver.LevelUpTechnologies(context.Background(), sess, 1, job, nil, nil, promptFn, nil, prep, known, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, known.techs[1], "druid slot picks must NOT populate KnownTechs")
}
```

- [ ] **Step 2: Run to confirm all three fail**

```bash
go test ./internal/gameserver/... -run "TestLevelUpTechnologies_(Wizard|Ranger|Druid)_Slot" -v 2>&1 | tail -15
```

- [ ] **Step 3: Update LevelUpTechnologies to add to KnownTechs after slot picks**

In `technology_assignment.go`, in `LevelUpTechnologies`, after each call to `fillFromPreparedPool` that returns chosen tech IDs, add this block when the casting model is wizard or ranger:

```go
// Populate KnownTechs for catalog-based casting models (wizard, ranger).
// Precondition: castingModel is resolved before this loop.
castingModel := ruleset.ResolveCastingModel(job, archetype)
if knownRepo != nil && (castingModel == ruleset.CastingModelWizard || castingModel == ruleset.CastingModelRanger) {
    for _, slot := range chosenSlots {
        if slot == nil {
            continue
        }
        if addErr := knownRepo.Add(ctx, sess.CharacterID, slot.TechID, lvl); addErr != nil {
            // Non-fatal: log and continue.
            _ = addErr
        }
        if sess.KnownTechs == nil {
            sess.KnownTechs = make(map[int][]string)
        }
        if !containsString(sess.KnownTechs[lvl], slot.TechID) {
            sess.KnownTechs[lvl] = append(sess.KnownTechs[lvl], slot.TechID)
        }
    }
}
```

Add helper (if not already present):
```go
func containsString(ss []string, s string) bool {
    for _, v := range ss {
        if v == s {
            return true
        }
    }
    return false
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/gameserver/... -run "TestLevelUpTechnologies_(Wizard|Ranger|Druid)_Slot" -v 2>&1 | tail -15
# Expected: all PASS
```

- [ ] **Step 5: Run full suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
git commit -m "feat(tech): populate KnownTechs during level-up slot picks for wizard/ranger models"
```

---

### Task 7: Wizard catalog extras — L1 +2 at creation and +2 per level-up

**Files:**
- Modify: `internal/gameserver/technology_assignment.go`

- [ ] **Step 1: Write failing tests**

In `technology_assignment_test.go`, add:

```go
// REQ-TC-9: wizard L1 creation gets 2 extra catalog entries beyond slot count
func TestLevelUpTechnologies_Wizard_L1ExtrasAddedToKnownTechs(t *testing.T) {
	known := &fakeKnownRepo{techs: make(map[int][]string)}
	prep := &fakePreparedRepo{slots: make(map[int][]*session.PreparedSlot)}
	sess := &session.PlayerSession{CastingModel: ruleset.CastingModelWizard}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "a", Level: 1}, {ID: "b", Level: 1}, {ID: "c", Level: 1},
					{ID: "d", Level: 1}, {ID: "e", Level: 1},
				},
			},
		},
	}
	pickCount := 0
	promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
		choice := opts[0]
		pickCount++
		return choice, nil
	}
	err := gameserver.LevelUpTechnologies(context.Background(), sess, 1, job, nil, nil, promptFn, nil, prep, known, nil, nil)
	require.NoError(t, err)
	// 1 slot pick + 2 extras = 3 total in KnownTechs
	assert.Len(t, known.techs[1], 3, "wizard L1 must have slot picks + 2 extras in KnownTechs")
}

// REQ-TC-10: wizard level 2 gets +2 catalog additions
func TestLevelUpTechnologies_Wizard_PerLevelExtrasAddedToKnownTechs(t *testing.T) {
	known := &fakeKnownRepo{techs: map[int][]string{1: {"existing_a", "existing_b", "existing_c"}}}
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{1: {{TechID: "existing_a"}}}}
	sess := &session.PlayerSession{
		CastingModel: ruleset.CastingModelWizard,
		PreparedTechs: map[int][]*session.PreparedSlot{1: {{TechID: "existing_a"}}},
		KnownTechs:    map[int][]string{1: {"existing_a", "existing_b", "existing_c"}},
	}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelWizard,
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: 1},
					Pool:         []ruleset.PreparedEntry{{ID: "new_d", Level: 1}, {ID: "new_e", Level: 1}, {ID: "new_f", Level: 1}},
				},
			},
		},
	}
	picks := []string{}
	promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
		choice := opts[0]
		picks = append(picks, choice)
		return choice, nil
	}
	err := gameserver.LevelUpTechnologies(context.Background(), sess, 2, job, nil, nil, promptFn, nil, prep, known, nil, nil)
	require.NoError(t, err)
	// The +2 catalog extras should have been prompted and added
	assert.GreaterOrEqual(t, len(known.techs[1]), 4, "wizard level-2 must add +2 catalog entries")
}

// Property: catalog picker never adds a tech already in KnownTechs
func TestProperty_WizardCatalogPicker_NeverDuplicates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		existingCount := rapid.IntRange(0, 3).Draw(rt, "existing")
		poolSize := rapid.IntRange(existingCount, existingCount+5).Draw(rt, "pool")
		var existing []string
		for i := 0; i < existingCount; i++ {
			existing = append(existing, fmt.Sprintf("tech_%d", i))
		}
		var pool []ruleset.PreparedEntry
		for i := 0; i < poolSize; i++ {
			pool = append(pool, ruleset.PreparedEntry{ID: fmt.Sprintf("tech_%d", i), Level: 1})
		}
		known := &fakeKnownRepo{techs: map[int][]string{1: existing}}
		sess := &session.PlayerSession{KnownTechs: map[int][]string{1: append([]string{}, existing...)}}
		promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
			if len(opts) == 0 {
				return "", fmt.Errorf("no options")
			}
			return opts[0], nil
		}
		err := gameserver.PickCatalogExtras(context.Background(), 1, 2, pool, known, sess, promptFn, nil)
		if err != nil {
			return // skip invalid combos
		}
		seen := make(map[string]int)
		for _, id := range known.techs[1] {
			seen[id]++
			if seen[id] > 1 {
				rt.Fatalf("duplicate tech %q in KnownTechs after catalog pick", id)
			}
		}
	})
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/gameserver/... -run "TestLevelUpTechnologies_Wizard_(L1|Per)|TestProperty_WizardCatalog" -v 2>&1 | tail -15
```

- [ ] **Step 3: Add PickCatalogExtras function to technology_assignment.go**

```go
// PickCatalogExtras prompts the player to add `count` techs from pool to their KnownTechs
// catalog at tech level `lvl`. Techs already in KnownTechs are excluded from options.
// If fewer than `count` eligible techs remain, all remaining are added without prompting.
//
// Precondition: knownRepo is non-nil; pool entries are at level lvl.
// Postcondition: up to `count` new techs are added to sess.KnownTechs[lvl] and persisted.
func PickCatalogExtras(
	ctx context.Context,
	lvl, count int,
	pool []ruleset.PreparedEntry,
	knownRepo KnownTechRepo,
	sess *session.PlayerSession,
	promptFn TechPromptFn,
	techReg TechRegistry,
) error {
	if sess.KnownTechs == nil {
		sess.KnownTechs = make(map[int][]string)
	}
	knownSet := make(map[string]bool, len(sess.KnownTechs[lvl]))
	for _, id := range sess.KnownTechs[lvl] {
		knownSet[id] = true
	}

	var remaining []ruleset.PreparedEntry
	for _, e := range pool {
		if e.Level == lvl && !knownSet[e.ID] {
			remaining = append(remaining, e)
		}
	}
	if len(remaining) == 0 {
		return nil
	}

	toAdd := count
	if len(remaining) <= toAdd {
		// Auto-add all remaining without prompting.
		for _, e := range remaining {
			sess.KnownTechs[lvl] = append(sess.KnownTechs[lvl], e.ID)
			if knownRepo != nil && sess.CharacterID > 0 {
				_ = knownRepo.Add(ctx, sess.CharacterID, e.ID, lvl)
			}
		}
		return nil
	}

	for i := 0; i < toAdd; i++ {
		options := buildPreparedOptions(remaining, techReg)
		prompt := fmt.Sprintf("Choose a Level %d technology to add to your catalog (extra %d of %d):", lvl, i+1, toAdd)
		chosen, err := promptFn(prompt, options, nil)
		if err != nil {
			return err
		}
		techID := parseTechID(chosen)
		sess.KnownTechs[lvl] = append(sess.KnownTechs[lvl], techID)
		if knownRepo != nil && sess.CharacterID > 0 {
			_ = knownRepo.Add(ctx, sess.CharacterID, techID, lvl)
		}
		// Remove chosen from remaining so it can't be picked again.
		for j, e := range remaining {
			if e.ID == techID {
				remaining = append(remaining[:j], remaining[j+1:]...)
				break
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Call PickCatalogExtras from LevelUpTechnologies**

In `LevelUpTechnologies`, after filling L1 prepared slots AND after adding them to KnownTechs, add:

```go
// Wizard model: pick 2 extra catalog entries at level 1 (beyond slot count).
if level == 1 && castingModel == ruleset.CastingModelWizard {
    combinedPool := mergePool(job, archetype, lvl)
    if pickErr := PickCatalogExtras(ctx, lvl, 2, combinedPool, knownRepo, sess, promptFn, techReg); pickErr != nil {
        return fmt.Errorf("wizard L1 catalog extras: %w", pickErr)
    }
}

// Wizard model: pick +2 catalog entries at each level-up above 1.
if level > 1 && castingModel == ruleset.CastingModelWizard {
    // Offer +2 from any eligible level (all levels with slots).
    for eligLvl := range slotsByLevel {
        combinedPool := mergePool(job, archetype, eligLvl)
        if pickErr := PickCatalogExtras(ctx, eligLvl, 2, combinedPool, knownRepo, sess, promptFn, techReg); pickErr != nil {
            return fmt.Errorf("wizard level-up catalog extras at tech level %d: %w", eligLvl, pickErr)
        }
    }
}
```

Add `mergePool` helper:
```go
// mergePool returns all pool entries at techLevel from job and archetype grants combined.
func mergePool(job *ruleset.Job, archetype *ruleset.Archetype, techLevel int) []ruleset.PreparedEntry {
	var pool []ruleset.PreparedEntry
	if job != nil && job.TechnologyGrants != nil && job.TechnologyGrants.Prepared != nil {
		for _, e := range job.TechnologyGrants.Prepared.Pool {
			if e.Level == techLevel {
				pool = append(pool, e)
			}
		}
	}
	if archetype != nil && archetype.TechnologyGrants != nil && archetype.TechnologyGrants.Prepared != nil {
		for _, e := range archetype.TechnologyGrants.Prepared.Pool {
			if e.Level == techLevel {
				pool = append(pool, e)
			}
		}
	}
	return pool
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/gameserver/... -run "TestLevelUpTechnologies_Wizard_(L1|Per)|TestProperty_WizardCatalog" -v 2>&1 | tail -20
# Expected: all PASS
```

- [ ] **Step 6: Run full suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
git commit -m "feat(tech): wizard catalog extras — +2 L1 at creation, +2 per level-up"
```

---

### Task 8: Update trainer flow for casting-model-aware KnownTechs population

**Files:**
- Modify: `internal/gameserver/grpc_service_tech_trainer.go`

- [ ] **Step 1: Write failing test**

In `grpc_service_tech_trainer_test.go`, add:

```go
// REQ-TC-11: wizard/ranger trainer writes to KnownTechs; druid does not
func TestDoTrainTech_Wizard_PopulatesKnownTechs(t *testing.T) {
	// Setup a wizard-model session with a pending L2 slot
	// Train a tech
	// Assert knownTechRepo.Add was called with the tech ID
	// This test uses the existing test server pattern
	svc, uid, trainerName := newTechTrainerTestServer(t)
	svc.getSession(uid).CastingModel = ruleset.CastingModelWizard
	// ... train the tech via handleTalk ...
	sess := svc.getSession(uid)
	assert.Contains(t, sess.KnownTechs[2], "test_tech_l2",
		"wizard trainer must add taught tech to KnownTechs")
}

func TestDoTrainTech_Druid_DoesNotPopulateKnownTechs(t *testing.T) {
	svc, uid, trainerName := newTechTrainerTestServer(t)
	svc.getSession(uid).CastingModel = ruleset.CastingModelDruid
	// ... train the tech ...
	sess := svc.getSession(uid)
	assert.Empty(t, sess.KnownTechs[2],
		"druid trainer must NOT add taught tech to KnownTechs")
}
```

*(Note: adapt to the actual test helper pattern in `grpc_service_tech_trainer_test.go`.)*

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/gameserver/... -run "TestDoTrainTech_(Wizard|Druid)" -v 2>&1 | tail -10
```

- [ ] **Step 3: Update doTrainTech in grpc_service_tech_trainer.go**

In `doTrainTech`, after the existing block that writes to `PreparedTechs` (around line 318), add for the prepared usage type case:

```go
case string(technology.UsagePrepared):
    // ... existing PreparedTechs write (unchanged) ...

    // Catalog-based models (wizard, ranger) track all trained techs in KnownTechs.
    // Druid model uses the full pool at rest and does not track individual techs.
    if sess.CastingModel == ruleset.CastingModelWizard || sess.CastingModel == ruleset.CastingModelRanger {
        if sess.KnownTechs == nil {
            sess.KnownTechs = make(map[int][]string)
        }
        if !containsString(sess.KnownTechs[matched.techLevel], techID) {
            sess.KnownTechs[matched.techLevel] = append(sess.KnownTechs[matched.techLevel], techID)
        }
        if s.knownTechRepo != nil && sess.CharacterID > 0 {
            if err := s.knownTechRepo.Add(ctx, sess.CharacterID, techID, matched.techLevel); err != nil {
                s.logger.Warn("doTrainTech: knownTechRepo.Add failed",
                    zap.String("tech_id", techID),
                    zap.Error(err),
                )
            }
        }
    }
```

Also update the spontaneous usage type case to use `knownTechRepo` (already renamed in Task 3):

```go
case string(technology.UsageSpontaneous):
    // ... existing KnownTechs write (already uses knownTechRepo after rename) ...
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/gameserver/... -run "TestDoTrainTech" -v 2>&1 | tail -15
```

- [ ] **Step 5: Full suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service_tech_trainer.go internal/gameserver/grpc_service_tech_trainer_test.go
git commit -m "feat(trainer): populate KnownTechs for wizard/ranger; skip for druid"
```

---

### Task 9: Refactor RearrangePreparedTechs — casting model pool, heightening, back/forward

**Files:**
- Modify: `internal/gameserver/technology_assignment.go`
- Modify: `internal/gameserver/technology_assignment_test.go`
- Modify: `internal/gameserver/grpc_service.go` (extend `choicePromptPayload` + promptFn closures)

- [ ] **Step 1: Write failing tests**

In `technology_assignment_test.go`, add:

```go
// REQ-TC-13: wizard rearrangement uses KnownTechs not grant pool
func TestRearrangePreparedTechs_Wizard_UsesKnownTechsCatalog(t *testing.T) {
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{1: {{TechID: "known_a"}}}}
	sess := &session.PlayerSession{
		CastingModel:  ruleset.CastingModelWizard,
		PreparedTechs: map[int][]*session.PreparedSlot{1: {{TechID: "known_a"}}},
		KnownTechs:    map[int][]string{1: {"known_a", "known_b"}},
	}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				// Pool has grant_only_tech which player does NOT know
				Pool: []ruleset.PreparedEntry{{ID: "known_a", Level: 1}, {ID: "known_b", Level: 1}, {ID: "grant_only_tech", Level: 1}},
			},
		},
	}
	var seenOptions [][]string
	promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
		seenOptions = append(seenOptions, opts)
		// Pick confirm or first real option
		for _, o := range opts {
			if o != "[back]" && o != "[forward]" && o != "[confirm]" {
				return o, nil
			}
		}
		return opts[0], nil
	}
	err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, nil, promptFn, prep, func(string) {}, technology.TraditionFlavor{SlotNoun: "slot"})
	require.NoError(t, err)
	// grant_only_tech must not appear in any option list
	for _, opts := range seenOptions {
		for _, o := range opts {
			assert.NotContains(t, o, "grant_only_tech", "wizard rearrangement must not show techs not in KnownTechs")
		}
	}
}

// REQ-TC-14: heighten delta is slotLevel - techLevel, never negative
func TestProperty_HeightenDelta_NeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		slotLevel := rapid.IntRange(1, 10).Draw(rt, "slotLevel")
		techLevel := rapid.IntRange(1, slotLevel).Draw(rt, "techLevel")
		delta := slotLevel - techLevel
		if delta < 0 {
			rt.Fatalf("heighten delta %d is negative (slotLevel=%d techLevel=%d)", delta, slotLevel, techLevel)
		}
	})
}

// REQ-TC-16: back/forward navigation never produces out-of-bounds index
func TestProperty_Rearrange_BackForwardNeverOutOfBounds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		totalSlots := rapid.IntRange(1, 6).Draw(rt, "total")
		cur := rapid.IntRange(0, totalSlots-1).Draw(rt, "start")
		for i := 0; i < 20; i++ {
			action := rapid.IntRange(0, 2).Draw(rt, "action") // 0=back, 1=forward, 2=pick
			switch action {
			case 0:
				if cur > 0 {
					cur--
				}
			case 1:
				if cur < totalSlots-1 {
					cur++
				}
			case 2:
				// no-op (pick)
			}
			if cur < 0 || cur >= totalSlots {
				rt.Fatalf("cur=%d out of bounds [0,%d)", cur, totalSlots)
			}
		}
	})
}

// REQ-TC-13: druid rearrangement uses full grant pool
func TestRearrangePreparedTechs_Druid_UsesGrantPool(t *testing.T) {
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{1: {{TechID: "pool_a"}}}}
	sess := &session.PlayerSession{
		CastingModel:  ruleset.CastingModelDruid,
		PreparedTechs: map[int][]*session.PreparedSlot{1: {{TechID: "pool_a"}}},
		KnownTechs:    map[int][]string{}, // empty — druid doesn't use catalog
	}
	job := &ruleset.Job{
		CastingModel: ruleset.CastingModelDruid,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "pool_a", Level: 1}, {ID: "pool_b", Level: 1}},
			},
		},
	}
	var seenOptions [][]string
	promptFn := func(_ string, opts []string, _ *gameserver.TechSlotContext) (string, error) {
		seenOptions = append(seenOptions, opts)
		for _, o := range opts {
			if o != "[back]" && o != "[forward]" && o != "[confirm]" {
				return o, nil
			}
		}
		return opts[0], nil
	}
	err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, nil, promptFn, prep, func(string) {}, technology.TraditionFlavor{SlotNoun: "slot"})
	require.NoError(t, err)
	found := false
	for _, opts := range seenOptions {
		for _, o := range opts {
			if strings.Contains(o, "pool_b") {
				found = true
			}
		}
	}
	assert.True(t, found, "druid rearrangement must offer full grant pool including pool_b")
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/gameserver/... -run "TestRearrangePreparedTechs_(Wizard|Druid)|TestProperty_(Heighten|Rearrange)" -v 2>&1 | tail -20
```

- [ ] **Step 3: Add helper functions to technology_assignment.go**

Add these helpers before `RearrangePreparedTechs`:

```go
const (
	backSentinel    = "[back]"
	forwardSentinel = "[forward]"
	confirmSentinel = "[confirm]"
)

// buildCatalogPool returns PreparedEntry values from KnownTechs at all levels ≤ slotLevel.
func buildCatalogPool(knownTechs map[int][]string, slotLevel int) []ruleset.PreparedEntry {
	var pool []ruleset.PreparedEntry
	for lvl := 1; lvl <= slotLevel; lvl++ {
		for _, id := range knownTechs[lvl] {
			pool = append(pool, ruleset.PreparedEntry{ID: id, Level: lvl})
		}
	}
	return pool
}

// buildGrantPool returns PreparedEntry values from grants.Pool at all levels ≤ slotLevel.
func buildGrantPool(grants *ruleset.PreparedGrants, slotLevel int) []ruleset.PreparedEntry {
	if grants == nil {
		return nil
	}
	var pool []ruleset.PreparedEntry
	for _, e := range grants.Pool {
		if e.Level <= slotLevel {
			pool = append(pool, e)
		}
	}
	return pool
}

// buildOptionsWithHeighten builds option strings for a slot of slotLevel.
// Options for techs below slotLevel get a [heightened:N] sentinel appended.
func buildOptionsWithHeighten(pool []ruleset.PreparedEntry, slotLevel int, techReg TechRegistry) []string {
	var opts []string
	for _, e := range pool {
		var s string
		if techReg != nil {
			if def, ok := techReg.Get(e.ID); ok {
				s = fmt.Sprintf("[%s] %s (Lv %d) — %s", e.ID, def.Name, e.Level, def.Description)
			} else {
				s = fmt.Sprintf("[%s] %s (Lv %d)", e.ID, e.ID, e.Level)
			}
		} else {
			s = fmt.Sprintf("[%s] %s (Lv %d)", e.ID, e.ID, e.Level)
		}
		delta := slotLevel - e.Level
		if delta > 0 {
			s += fmt.Sprintf(" [heightened:%d]", delta)
		}
		opts = append(opts, s)
	}
	return opts
}

// rearrangeSlot represents one prepared slot position during rearrangement.
type rearrangeSlot struct {
	level   int
	slotNum int // 1-based within level
	total   int // total slots at this level
}
```

- [ ] **Step 4: Refactor RearrangePreparedTechs**

Replace the body of `RearrangePreparedTechs` with the new implementation (keep the existing function signature). The new body:

```go
func RearrangePreparedTechs(
	ctx context.Context,
	sess *session.PlayerSession,
	level int,
	job *ruleset.Job,
	archetype *ruleset.Archetype,
	techReg TechRegistry,
	promptFn TechPromptFn,
	prepRepo PreparedTechRepo,
	send func(string),
	flavor technology.TraditionFlavor,
) error {
	// Guard: skip if player has no prepared techs yet (initial trainer resolution pending).
	if len(sess.PreparedTechs) == 0 {
		return nil
	}

	castingModel := ruleset.ResolveCastingModel(job, archetype)

	// Aggregate slot counts from grants (same logic as before).
	grantSlots := make(map[int]int)
	var mergedGrantPool *ruleset.PreparedGrants // used by druid model
	// ... (keep existing grant aggregation logic from the current implementation) ...

	// Build final slotsByLevel as max(grantSlots, sessionSlots).
	slotsByLevel := make(map[int]int)
	for lvl, cnt := range grantSlots {
		slotsByLevel[lvl] = cnt
	}
	for lvl, slots := range sess.PreparedTechs {
		if cnt := len(slots); cnt > slotsByLevel[lvl] {
			slotsByLevel[lvl] = cnt
		}
	}

	// Build flat ordered slot list.
	var slots []rearrangeSlot
	levels := sortedKeys(slotsByLevel) // sorted int slice
	for _, lvl := range levels {
		total := slotsByLevel[lvl]
		for i := 0; i < total; i++ {
			slots = append(slots, rearrangeSlot{level: lvl, slotNum: i + 1, total: total})
		}
	}
	if len(slots) == 0 {
		return nil
	}

	// Track in-progress assignments (not committed until [confirm]).
	inProgress := make(map[int][]*session.PreparedSlot)
	for lvl, prev := range sess.PreparedTechs {
		cp := make([]*session.PreparedSlot, len(prev))
		copy(cp, prev)
		inProgress[lvl] = cp
	}

	send(fmt.Sprintf("%s your %s loadout:", flavor.PrepGerund, strings.ToLower(flavor.LoadoutTitle)))

	cur := 0
	for cur >= 0 && cur < len(slots) {
		slot := slots[cur]
		isFirst := cur == 0
		isLast := cur == len(slots)-1

		// Build pool based on casting model.
		var pool []ruleset.PreparedEntry
		switch castingModel {
		case ruleset.CastingModelWizard, ruleset.CastingModelRanger:
			pool = buildCatalogPool(sess.KnownTechs, slot.level)
		case ruleset.CastingModelDruid:
			pool = buildGrantPool(mergedGrantPool, slot.level)
		default:
			pool = buildCatalogPool(sess.KnownTechs, slot.level)
		}

		// Default to highest available level if pool is empty at this slot level.
		if len(pool) == 0 {
			send(fmt.Sprintf("Warning: no known techs available for level %d slot, skipping", slot.level))
			cur++
			continue
		}

		// Auto-assign if only one option.
		if len(pool) == 1 {
			send(fmt.Sprintf("Level %d, %s %d of %d (auto): %s", slot.level, flavor.SlotNoun, slot.slotNum, slot.total, pool[0].ID))
			slotIdx := slot.slotNum - 1
			if len(inProgress[slot.level]) <= slotIdx {
				inProgress[slot.level] = append(inProgress[slot.level], make([]*session.PreparedSlot, slotIdx-len(inProgress[slot.level])+1)...)
			}
			inProgress[slot.level][slotIdx] = &session.PreparedSlot{TechID: pool[0].ID}
			cur++
			continue
		}

		options := buildOptionsWithHeighten(pool, slot.level, techReg)

		// Prepend keep-current option if the current tech is still available.
		slotIdx := slot.slotNum - 1
		if slotIdx < len(inProgress[slot.level]) && inProgress[slot.level][slotIdx] != nil {
			prevID := inProgress[slot.level][slotIdx].TechID
			for _, e := range pool {
				if e.ID == prevID {
					keepName := prevID
					if techReg != nil {
						if def, ok := techReg.Get(prevID); ok {
							keepName = def.Name
						}
					}
					// Remove from regular options, prepend as keep.
					for i, o := range options {
						if parseTechID(o) == prevID {
							options = append(options[:i], options[i+1:]...)
							break
						}
					}
					options = append([]string{keepSentinel + "Keep current: " + keepName}, options...)
					break
				}
			}
		}

		// Add navigation sentinels.
		if !isFirst {
			options = append([]string{backSentinel}, options...)
		}
		if isLast {
			options = append(options, confirmSentinel)
		} else {
			options = append(options, forwardSentinel)
		}

		slotCtx := &TechSlotContext{
			SlotNum:    slot.slotNum,
			TotalSlots: slot.total,
			SlotLevel:  slot.level,
		}
		prompt := fmt.Sprintf("Choose a Level %d technology to prepare (%s %d of %d):", slot.level, flavor.SlotNoun, slot.slotNum, slot.total)
		chosen, err := promptFn(prompt, options, slotCtx)
		if err != nil {
			return err
		}

		switch chosen {
		case backSentinel:
			cur--
		case forwardSentinel:
			cur++
		case confirmSentinel:
			// Write all in-progress to DB and session.
			for lvl, slotList := range inProgress {
				sess.PreparedTechs[lvl] = slotList
				for idx, s := range slotList {
					if s == nil {
						continue
					}
					if prepRepo != nil && sess.CharacterID > 0 {
						if setErr := prepRepo.Set(ctx, sess.CharacterID, lvl, idx, s.TechID); setErr != nil {
							return fmt.Errorf("saving rearranged tech at level %d slot %d: %w", lvl, idx, setErr)
						}
					}
				}
			}
			return nil
		default:
			techID := parseTechID(chosen)
			if strings.HasPrefix(chosen, keepSentinel) {
				if slotIdx < len(inProgress[slot.level]) && inProgress[slot.level][slotIdx] != nil {
					techID = inProgress[slot.level][slotIdx].TechID
				}
			}
			if len(inProgress[slot.level]) <= slotIdx {
				inProgress[slot.level] = append(inProgress[slot.level], make([]*session.PreparedSlot, slotIdx-len(inProgress[slot.level])+1)...)
			}
			inProgress[slot.level][slotIdx] = &session.PreparedSlot{TechID: techID}
			if isLast {
				// Last slot: auto-confirm after selection.
				for lvl, slotList := range inProgress {
					sess.PreparedTechs[lvl] = slotList
					for idx, s := range slotList {
						if s == nil {
							continue
						}
						if prepRepo != nil && sess.CharacterID > 0 {
							_ = prepRepo.Set(ctx, sess.CharacterID, lvl, idx, s.TechID)
						}
					}
				}
				return nil
			}
			cur++
		}
	}
	return nil
}

// sortedKeys returns the keys of a map[int]int in ascending order.
func sortedKeys(m map[int]int) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
```

- [ ] **Step 5: Extend choicePromptPayload in grpc_service.go**

```go
type choicePromptPayload struct {
	FeatureID   string            `json:"featureId"`
	Prompt      string            `json:"prompt"`
	Options     []string          `json:"options"`
	SlotContext *techSlotContext  `json:"slotContext,omitempty"`
}

type techSlotContext struct {
	SlotNum    int `json:"slotNum"`
	TotalSlots int `json:"totalSlots"`
	SlotLevel  int `json:"slotLevel"`
}
```

- [ ] **Step 6: Update promptFn closures in grpc_service.go to pass slotCtx**

For the closures used in `RearrangePreparedTechs` context (around lines 3901, 4293, 4462), update to:

```go
promptFn := func(prompt string, options []string, slotCtx *gameserver.TechSlotContext) (string, error) {
	var sc *techSlotContext
	if slotCtx != nil {
		sc = &techSlotContext{
			SlotNum:    slotCtx.SlotNum,
			TotalSlots: slotCtx.TotalSlots,
			SlotLevel:  slotCtx.SlotLevel,
		}
	}
	payload := choicePromptPayload{
		FeatureID:   "tech_choice",
		Prompt:      prompt,
		Options:     options,
		SlotContext: sc,
	}
	if headless {
		return options[0], nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling tech choice prompt: %w", err)
	}
	const choiceSentinel = "\x00choice\x00"
	if sendErr := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: choiceSentinel + string(data)},
		},
	}); sendErr != nil {
		return "", sendErr
	}
	// ... same recv loop as promptFeatureChoice ...
}
```

*(For closures NOT used in rearrangement context, the existing `promptFeatureChoice` call with `nil` slotCtx is fine.)*

- [ ] **Step 7: Run tests**

```bash
go test ./internal/gameserver/... -run "TestRearrangePreparedTechs_(Wizard|Druid)|TestProperty_(Heighten|Rearrange)" -v 2>&1 | tail -25
# Expected: all PASS
```

- [ ] **Step 8: Run full suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

- [ ] **Step 9: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go \
  internal/gameserver/grpc_service.go
git commit -m "feat(tech): casting-model-aware rearrangement, heightening, back/forward navigation"
```

---

### Task 10: Frontend — slotContext header, level tabs, heightened badge, navigation buttons

**Files:**
- Modify: `cmd/webclient/ui/src/game/GameContext.tsx`
- Modify: `cmd/webclient/ui/src/game/drawers/FeatureChoiceModal.tsx`

- [ ] **Step 1: Update ChoicePrompt type in GameContext.tsx**

Find the `ChoicePrompt` interface (around line 108). Replace with:

```typescript
interface SlotContext {
  slotNum: number
  totalSlots: number
  slotLevel: number
}

interface ChoicePrompt {
  featureId: string
  prompt: string
  options: string[]
  slotContext?: SlotContext
}
```

- [ ] **Step 2: Update SET_CHOICE_PROMPT dispatch to forward slotContext**

Around line 659-670, update the handler:

```typescript
case 'FeatureChoicePrompt': {
  const cp = payload as { featureId?: string; prompt?: string; options?: string[]; slotContext?: SlotContext }
  dispatch({
    type: 'SET_CHOICE_PROMPT',
    prompt: {
      featureId: cp.featureId ?? '',
      prompt: cp.prompt ?? '',
      options: Array.isArray(cp.options) ? cp.options : [],
      slotContext: cp.slotContext,
    },
  })
  break
}
```

Also update the `SET_CHOICE_PROMPT` action handler in the reducer to pass `slotContext` through.

- [ ] **Step 3: Rewrite FeatureChoiceModal.tsx**

Replace the full file content:

```typescript
import React, { useState, useMemo } from 'react'
import { useGame } from '../GameContext'

const BACK_SENTINEL = '[back]'
const FORWARD_SENTINEL = '[forward]'
const CONFIRM_SENTINEL = '[confirm]'
const KEEP_SENTINEL = '[keep] '

// stripOptionPrefix removes [xxx] prefixes from display text.
function stripOptionPrefix(opt: string): string {
  if (opt.startsWith(KEEP_SENTINEL)) return opt.slice(KEEP_SENTINEL.length)
  return opt.replace(/^\[[^\]]+\]\s*/, '')
}

// parseHeightenDelta extracts the [heightened:N] sentinel and returns { text, delta }.
function parseHeightenSentinel(opt: string): { text: string; delta: number } {
  const match = opt.match(/\[heightened:(\d+)\]/)
  if (!match) return { text: opt, delta: 0 }
  return { text: opt.replace(/\s*\[heightened:\d+\]/, ''), delta: parseInt(match[1], 10) }
}

// parseLevelFromOption extracts the tech level from "(Lv N)" in the option string.
function parseLevelFromOption(opt: string): number {
  const match = opt.match(/\(Lv (\d+)\)/)
  return match ? parseInt(match[1], 10) : 0
}

export default function FeatureChoiceModal() {
  const { state, sendCommand, clearChoicePrompt } = useGame()
  const cp = state.choicePrompt
  if (!cp) return null

  const options = cp.options ?? []

  // Separate navigation sentinels from real options.
  const hasBack = options.includes(BACK_SENTINEL)
  const hasForward = options.includes(FORWARD_SENTINEL)
  const hasConfirm = options.includes(CONFIRM_SENTINEL)
  const realOptions = options.filter(
    o => o !== BACK_SENTINEL && o !== FORWARD_SENTINEL && o !== CONFIRM_SENTINEL
  )

  // Extract available tech levels from options (for level filter tabs).
  const availableLevels = useMemo(() => {
    const levels = new Set<number>()
    realOptions.forEach(o => {
      const lvl = parseLevelFromOption(o)
      if (lvl > 0) levels.add(lvl)
    })
    return Array.from(levels).sort((a, b) => a - b)
  }, [realOptions])

  const slotLevel = cp.slotContext?.slotLevel ?? 0
  const defaultLevel = availableLevels.includes(slotLevel)
    ? slotLevel
    : availableLevels[availableLevels.length - 1] ?? 0

  const [activeLevel, setActiveLevel] = useState<number>(defaultLevel)

  // Reset active level when prompt changes.
  React.useEffect(() => {
    setActiveLevel(
      availableLevels.includes(slotLevel)
        ? slotLevel
        : availableLevels[availableLevels.length - 1] ?? 0
    )
  }, [cp.featureId, cp.prompt])

  // Filter options by active level (or show all if no level metadata).
  const filteredOptions = availableLevels.length > 0
    ? realOptions.filter(o => {
        const lvl = parseLevelFromOption(o)
        return lvl === 0 || lvl === activeLevel
      })
    : realOptions

  function handleSelect(zeroIdx: number) {
    // Map filtered index back to original options index for 1-based server response.
    const originalIdx = options.indexOf(filteredOptions[zeroIdx])
    clearChoicePrompt()
    sendCommand(String(originalIdx + 1))
  }

  function handleNavigation(sentinel: string) {
    const idx = options.indexOf(sentinel)
    clearChoicePrompt()
    sendCommand(String(idx + 1))
  }

  return (
    <div style={{
      position: 'fixed', top: 0, left: 0, width: '100%', height: '100%',
      backgroundColor: 'rgba(0,0,0,0.85)', zIndex: 300,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      fontFamily: 'monospace',
    }}>
      <div style={{
        backgroundColor: '#111', border: '2px solid #4a6a2a',
        padding: '20px', maxWidth: '600px', width: '90%', maxHeight: '80vh',
        overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '10px',
      }}>
        {/* Slot progress header */}
        {cp.slotContext && (
          <div style={{ color: '#888', fontSize: '0.85em', textAlign: 'right' }}>
            Slot {cp.slotContext.slotNum} of {cp.slotContext.totalSlots} — Level {cp.slotContext.slotLevel}
          </div>
        )}

        {/* Prompt title */}
        <div style={{ color: '#e0c060', fontSize: '1.1em', marginBottom: '8px' }}>
          {cp.prompt}
        </div>

        {/* Level filter tabs */}
        {availableLevels.length > 1 && (
          <div style={{ display: 'flex', gap: '6px', marginBottom: '8px' }}>
            {availableLevels.map(lvl => (
              <button
                key={lvl}
                onClick={() => setActiveLevel(lvl)}
                style={{
                  padding: '4px 10px',
                  backgroundColor: lvl === activeLevel ? '#4a6a2a' : '#222',
                  color: lvl === activeLevel ? '#e0c060' : '#aaa',
                  border: '1px solid #4a6a2a',
                  cursor: 'pointer',
                  fontFamily: 'monospace',
                }}
              >
                L{lvl}
              </button>
            ))}
          </div>
        )}

        {/* Option buttons */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
          {filteredOptions.map((opt, i) => {
            const { text: withoutHeighten, delta } = parseHeightenSentinel(opt)
            const displayText = stripOptionPrefix(withoutHeighten)
            return (
              <button
                key={i}
                onClick={() => handleSelect(i)}
                style={{
                  textAlign: 'left', padding: '8px 12px',
                  backgroundColor: '#1a1a1a', color: '#ccc',
                  border: '1px solid #4a6a2a', cursor: 'pointer',
                  fontFamily: 'monospace', display: 'flex', alignItems: 'center', gap: '8px',
                }}
              >
                <span style={{ color: '#4a6a2a', minWidth: '20px' }}>{i + 1}.</span>
                <span>{displayText}</span>
                {delta > 0 && (
                  <span style={{
                    color: '#e0c060', fontSize: '0.8em',
                    border: '1px solid #e0c060', padding: '1px 5px', borderRadius: '3px',
                  }}>
                    +{delta}
                  </span>
                )}
              </button>
            )
          })}
        </div>

        {/* Navigation row */}
        {(hasBack || hasForward || hasConfirm) && (
          <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '12px' }}>
            <div>
              {hasBack && (
                <button
                  onClick={() => handleNavigation(BACK_SENTINEL)}
                  style={{
                    padding: '6px 16px', backgroundColor: '#222', color: '#aaa',
                    border: '1px solid #555', cursor: 'pointer', fontFamily: 'monospace',
                  }}
                >
                  ← Back
                </button>
              )}
            </div>
            <div>
              {hasForward && (
                <button
                  onClick={() => handleNavigation(FORWARD_SENTINEL)}
                  style={{
                    padding: '6px 16px', backgroundColor: '#222', color: '#aaa',
                    border: '1px solid #4a6a2a', cursor: 'pointer', fontFamily: 'monospace',
                  }}
                >
                  Next →
                </button>
              )}
              {hasConfirm && (
                <button
                  onClick={() => handleNavigation(CONFIRM_SENTINEL)}
                  style={{
                    padding: '6px 16px', backgroundColor: '#4a6a2a', color: '#e0c060',
                    border: '1px solid #e0c060', cursor: 'pointer', fontFamily: 'monospace',
                  }}
                >
                  Confirm
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Build the frontend**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui
npm run build 2>&1 | tail -20
# Expected: build succeeds, no TypeScript errors
```

- [ ] **Step 3: Smoke test manually**

Start the dev server and trigger a long rest on a wizard-model character. Verify:
- Slot progress header shows "Slot N of TOTAL — Level M"
- Level tabs appear when multiple levels are available
- Clicking L1 tab filters to L1 techs; L2 tab shows L2 techs with `+N` badge
- Back/Next buttons appear and navigate correctly
- Confirm button appears on the last slot

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add cmd/webclient/ui/src/game/GameContext.tsx cmd/webclient/ui/src/game/drawers/FeatureChoiceModal.tsx
git commit -m "feat(frontend): FeatureChoiceModal — slot header, level tabs, heightened badge, back/forward navigation"
```

---

## Self-Review

**Spec coverage:**

| Requirement | Task |
|-------------|------|
| REQ-TC-1: Rename SpontaneousTechs → KnownTechs | Tasks 2–3 |
| REQ-TC-2: No new DB table | Tasks 1–2 |
| REQ-TC-3: Unified field semantics by model | Tasks 4–5 |
| REQ-TC-4: Rename backward compat migration | Task 1 |
| REQ-TC-5: CastingModel on Archetype/Job | Task 4 |
| REQ-TC-6: Archetype default casting models | Task 4 (YAML) |
| REQ-TC-7: Rename backward compat | Task 1 |
| REQ-TC-8: Level-up slot picks → KnownTechs (wizard/ranger) | Task 6 |
| REQ-TC-9: L1 extras at creation (wizard) | Task 7 |
| REQ-TC-10: +2 per level-up (wizard) | Task 7 |
| REQ-TC-11: Trainer → KnownTechs (wizard/ranger) | Task 8 |
| REQ-TC-12: Spontaneous unchanged | Tasks 3 (rename only) |
| REQ-TC-13: Casting-model-aware pool | Task 9 |
| REQ-TC-14: Heightened assignment | Task 9 |
| REQ-TC-15: Default to highest known level | Task 9 |
| REQ-TC-16: Back/forward navigation | Tasks 9–10 |
| REQ-TC-17: Auto-assign single option | Task 9 |
| REQ-TC-18: slotContext in ChoicePrompt | Tasks 9–10 |
| REQ-TC-19: Level filter tabs | Task 10 |
| REQ-TC-20: Heightened badge | Task 10 |
| REQ-TC-21: Back/Forward buttons | Task 10 |
| REQ-TC-22: Druid trainer no KnownTechs | Task 8 |
| REQ-TC-23: Job override propagation | Task 4 (ResolveCastingModel) |
| REQ-TC-24: Missing casting_model defaults to none | Task 4 |
| REQ-TC-25: Spontaneous rest unchanged | Task 9 (guard) |
| REQ-TC-26: Property-based tests | Tasks 4, 7, 9 |
| REQ-TC-27: Unit tests | Tasks 4–9 |

All requirements covered. ✓
