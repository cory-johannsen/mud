# Character Ability Boosts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow players to select free ability boosts at creation (Archetype: 2 fixed + 2 free; Region: 2 fixed + 1 free; Job: 1 fixed already done), with missing choices prompted at login.

**Architecture:** YAML content files gain `ability_boosts` blocks; a new `AbilityBoostGrant` struct is added to the ruleset; `ApplyAbilityBoosts` computes final scores; a new `character_ability_boosts` DB table persists chosen boosts; `GameServiceServer.Session()` prompts for missing choices after the initial room view using the existing `promptFeatureChoice` pattern.

**Tech Stack:** Go, pgx v5, pgregory.net/rapid (property-based tests), PostgreSQL

---

### Task 1: Add `ability_boosts` to all archetype and region YAML files

**Files:**
- Modify: `content/archetypes/aggressor.yaml`
- Modify: `content/archetypes/criminal.yaml`
- Modify: `content/archetypes/drifter.yaml`
- Modify: `content/archetypes/influencer.yaml`
- Modify: `content/archetypes/nerd.yaml`
- Modify: `content/archetypes/normie.yaml`
- Modify: `content/regions/gresham_outskirts.yaml`
- Modify: `content/regions/midwest.yaml`
- Modify: `content/regions/mountain.yaml`
- Modify: `content/regions/northeast.yaml`
- Modify: `content/regions/north_portland.yaml`
- Modify: `content/regions/old_town.yaml`
- Modify: `content/regions/pacific_northwest.yaml`
- Modify: `content/regions/pearl_district.yaml`
- Modify: `content/regions/southeast_portland.yaml`
- Modify: `content/regions/southern_california.yaml`
- Modify: `content/regions/south.yaml`

**Step 1: Add ability_boosts to each archetype YAML**

For each archetype file, append the `ability_boosts` block as shown:

`content/archetypes/aggressor.yaml`:
```yaml
ability_boosts:
  fixed: [brutality, grit]
  free: 2
```

`content/archetypes/criminal.yaml`:
```yaml
ability_boosts:
  fixed: [quickness, savvy]
  free: 2
```

`content/archetypes/drifter.yaml`:
```yaml
ability_boosts:
  fixed: [grit, brutality]
  free: 2
```

`content/archetypes/influencer.yaml`:
```yaml
ability_boosts:
  fixed: [flair, savvy]
  free: 2
```

`content/archetypes/nerd.yaml`:
```yaml
ability_boosts:
  fixed: [reasoning, savvy]
  free: 2
```

`content/archetypes/normie.yaml`:
```yaml
ability_boosts:
  fixed: [savvy, flair]
  free: 2
```

**Step 2: Add ability_boosts to each region YAML**

For each region file, append the `ability_boosts` block:

`content/regions/gresham_outskirts.yaml`: `fixed: [savvy, grit]`, `free: 1`
`content/regions/midwest.yaml`: `fixed: [brutality, reasoning]`, `free: 1`
`content/regions/mountain.yaml`: `fixed: [brutality, grit]`, `free: 1`
`content/regions/northeast.yaml`: `fixed: [quickness, reasoning]`, `free: 1`
`content/regions/north_portland.yaml`: `fixed: [brutality, grit]`, `free: 1`
`content/regions/old_town.yaml`: `fixed: [flair, quickness]`, `free: 1`
`content/regions/pacific_northwest.yaml`: `fixed: [flair, quickness]`, `free: 1`
`content/regions/pearl_district.yaml`: `fixed: [reasoning, savvy]`, `free: 1`
`content/regions/southeast_portland.yaml`: `fixed: [grit, quickness]`, `free: 1`
`content/regions/southern_california.yaml`: `fixed: [quickness, reasoning]`, `free: 1`
`content/regions/south.yaml`: `fixed: [grit, brutality]`, `free: 1`

**Step 3: Commit**

```bash
git add content/archetypes/ content/regions/
git commit -m "feat: add ability_boosts to all archetype and region YAML files"
```

---

### Task 2: Add `AbilityBoostGrant` struct to ruleset; extend Archetype and Region

**Files:**
- Create: `internal/game/ruleset/ability_boost.go`
- Modify: `internal/game/ruleset/archetype.go`
- Modify: `internal/game/ruleset/region.go`
- Create: `internal/game/ruleset/ability_boost_test.go`

**Step 1: Write the failing test**

Create `internal/game/ruleset/ability_boost_test.go`:

```go
package ruleset_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadArchetypes_HasAbilityBoosts(t *testing.T) {
    archetypes, err := ruleset.LoadArchetypes("../../../content/archetypes")
    require.NoError(t, err)
    require.NotEmpty(t, archetypes)
    for _, a := range archetypes {
        assert.NotNil(t, a.AbilityBoosts, "archetype %q missing ability_boosts", a.ID)
        assert.Len(t, a.AbilityBoosts.Fixed, 2, "archetype %q must have exactly 2 fixed boosts", a.ID)
        assert.Equal(t, 2, a.AbilityBoosts.Free, "archetype %q must have free=2", a.ID)
    }
}

func TestLoadRegions_HasAbilityBoosts(t *testing.T) {
    regions, err := ruleset.LoadRegions("../../../content/regions")
    require.NoError(t, err)
    require.NotEmpty(t, regions)
    for _, r := range regions {
        assert.NotNil(t, r.AbilityBoosts, "region %q missing ability_boosts", r.ID)
        assert.Len(t, r.AbilityBoosts.Fixed, 2, "region %q must have exactly 2 fixed boosts", r.ID)
        assert.Equal(t, 1, r.AbilityBoosts.Free, "region %q must have free=1", r.ID)
    }
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/ruleset/... -run "TestLoadArchetypes_HasAbilityBoosts|TestLoadRegions_HasAbilityBoosts" -v
```

Expected: FAIL (AbilityBoosts field undefined)

**Step 3: Create `internal/game/ruleset/ability_boost.go`**

```go
package ruleset

// AbilityBoostGrant describes the ability boosts a content source (archetype or region) provides.
// Fixed boosts are always applied. Free boosts require player selection during character creation.
//
// Valid ability IDs: "brutality", "grit", "quickness", "reasoning", "savvy", "flair"
type AbilityBoostGrant struct {
    Fixed []string `yaml:"fixed"` // ability IDs always boosted by this source
    Free  int      `yaml:"free"`  // number of player-chosen free boost slots
}

// AllAbilities returns the canonical ordered list of all six ability IDs.
func AllAbilities() []string {
    return []string{"brutality", "grit", "quickness", "reasoning", "savvy", "flair"}
}
```

**Step 4: Add `AbilityBoosts` to `Archetype` in `internal/game/ruleset/archetype.go`**

Add the field to the struct:
```go
AbilityBoosts *AbilityBoostGrant `yaml:"ability_boosts"`
```

**Step 5: Add `AbilityBoosts` to `Region` in `internal/game/ruleset/region.go`**

Add the field to the `Region` struct:
```go
AbilityBoosts *AbilityBoostGrant `yaml:"ability_boosts"`
```

**Step 6: Run tests**

```bash
go test ./internal/game/ruleset/... -run "TestLoadArchetypes_HasAbilityBoosts|TestLoadRegions_HasAbilityBoosts" -v
```

Expected: PASS

**Step 7: Run full ruleset suite to check regressions**

```bash
go test ./internal/game/ruleset/... -v 2>&1 | tail -10
```

**Step 8: Commit**

```bash
git add internal/game/ruleset/ability_boost.go internal/game/ruleset/archetype.go \
        internal/game/ruleset/region.go internal/game/ruleset/ability_boost_test.go
git commit -m "feat: add AbilityBoostGrant struct and wire into Archetype and Region"
```

---

### Task 3: Implement `ApplyAbilityBoosts` and `AbilityBoostPool` in character builder

**Files:**
- Modify: `internal/game/character/builder.go`
- Create: `internal/game/character/ability_boost_test.go`

**Step 1: Write the failing tests**

Create `internal/game/character/ability_boost_test.go`:

```go
package character_test

import (
    "testing"

    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/character"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestApplyAbilityBoosts_EachBoostAddsTwo(t *testing.T) {
    base := character.AbilityScores{
        Brutality: 10, Grit: 10, Quickness: 10,
        Reasoning: 10, Savvy: 10, Flair: 10,
    }
    archetypeBoosts := &ruleset.AbilityBoostGrant{Fixed: []string{"brutality", "grit"}, Free: 2}
    archetypeChosen := []string{"quickness", "reasoning"}
    regionBoosts := &ruleset.AbilityBoostGrant{Fixed: []string{"savvy", "flair"}, Free: 1}
    regionChosen := []string{"brutality"}

    got := character.ApplyAbilityBoosts(base, archetypeBoosts, archetypeChosen, regionBoosts, regionChosen)

    // brutality: +2 (archetype fixed) +2 (region chosen) = 14
    assert.Equal(t, 14, got.Brutality)
    // grit: +2 (archetype fixed) = 12
    assert.Equal(t, 12, got.Grit)
    // quickness: +2 (archetype free) = 12
    assert.Equal(t, 12, got.Quickness)
    // reasoning: +2 (archetype free) = 12
    assert.Equal(t, 12, got.Reasoning)
    // savvy: +2 (region fixed) = 12
    assert.Equal(t, 12, got.Savvy)
    // flair: +2 (region fixed) = 12
    assert.Equal(t, 12, got.Flair)
}

func TestApplyAbilityBoosts_NilGrantsAreNoOp(t *testing.T) {
    base := character.AbilityScores{Brutality: 14, Grit: 12}
    got := character.ApplyAbilityBoosts(base, nil, nil, nil, nil)
    assert.Equal(t, base, got)
}

func TestProperty_ApplyAbilityBoosts_EachBoostExactlyTwo(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        // Generate a random subset of abilities to boost
        abilities := []string{"brutality", "grit", "quickness", "reasoning", "savvy", "flair"}
        n := rapid.IntRange(0, 4).Draw(rt, "n")
        chosen := abilities[:n]

        base := character.AbilityScores{
            Brutality: 10, Grit: 10, Quickness: 10,
            Reasoning: 10, Savvy: 10, Flair: 10,
        }
        grant := &ruleset.AbilityBoostGrant{Fixed: chosen, Free: 0}
        got := character.ApplyAbilityBoosts(base, grant, nil, nil, nil)

        for _, ab := range chosen {
            switch ab {
            case "brutality":
                assert.Equal(rt, 12, got.Brutality)
            case "grit":
                assert.Equal(rt, 12, got.Grit)
            case "quickness":
                assert.Equal(rt, 12, got.Quickness)
            case "reasoning":
                assert.Equal(rt, 12, got.Reasoning)
            case "savvy":
                assert.Equal(rt, 12, got.Savvy)
            case "flair":
                assert.Equal(rt, 12, got.Flair)
            }
        }
    })
}

func TestAbilityBoostPool_ExcludesFixed(t *testing.T) {
    pool := character.AbilityBoostPool([]string{"brutality", "grit"}, nil)
    assert.NotContains(t, pool, "brutality")
    assert.NotContains(t, pool, "grit")
    assert.Len(t, pool, 4)
}

func TestAbilityBoostPool_ExcludesAlreadyChosen(t *testing.T) {
    pool := character.AbilityBoostPool([]string{"brutality"}, []string{"quickness"})
    assert.NotContains(t, pool, "brutality")
    assert.NotContains(t, pool, "quickness")
    assert.Len(t, pool, 4)
}

func TestAbilityBoostPool_EmptyInputsReturnsAll(t *testing.T) {
    pool := character.AbilityBoostPool(nil, nil)
    assert.Len(t, pool, 6)
}
```

**Step 2: Run to verify failures**

```bash
go test ./internal/game/character/... -run "TestApplyAbilityBoosts|TestAbilityBoostPool|TestProperty_Apply" -v
```

Expected: FAIL (functions undefined)

**Step 3: Implement in `internal/game/character/builder.go`**

Add to the end of `builder.go`:

```go
// ApplyAbilityBoosts returns the ability scores after stacking all boost sources.
// Each boost adds +2 to the named ability. Nil grants and nil chosen slices are no-ops.
//
// Precondition: archetypeChosen and regionChosen must satisfy within-source uniqueness
//   (caller is responsible for enforcing this during selection).
// Postcondition: Each boosted ability is increased by exactly +2 per application.
func ApplyAbilityBoosts(
    base AbilityScores,
    archetypeBoosts *ruleset.AbilityBoostGrant, archetypeChosen []string,
    regionBoosts *ruleset.AbilityBoostGrant, regionChosen []string,
) AbilityScores {
    result := base
    applyBoost := func(ability string) {
        switch ability {
        case "brutality":
            result.Brutality += 2
        case "grit":
            result.Grit += 2
        case "quickness":
            result.Quickness += 2
        case "reasoning":
            result.Reasoning += 2
        case "savvy":
            result.Savvy += 2
        case "flair":
            result.Flair += 2
        }
    }
    if archetypeBoosts != nil {
        for _, ab := range archetypeBoosts.Fixed {
            applyBoost(ab)
        }
    }
    for _, ab := range archetypeChosen {
        applyBoost(ab)
    }
    if regionBoosts != nil {
        for _, ab := range regionBoosts.Fixed {
            applyBoost(ab)
        }
    }
    for _, ab := range regionChosen {
        applyBoost(ab)
    }
    return result
}

// AbilityBoostPool returns the valid free-boost ability pool for a source.
// Excludes abilities listed in fixed and abilities already in alreadyChosen.
// The returned slice is in canonical order (brutality, grit, quickness, reasoning, savvy, flair).
//
// Precondition: fixed and alreadyChosen may be nil.
// Postcondition: No ability appears more than once in the returned slice.
func AbilityBoostPool(fixed []string, alreadyChosen []string) []string {
    excluded := make(map[string]bool)
    for _, ab := range fixed {
        excluded[ab] = true
    }
    for _, ab := range alreadyChosen {
        excluded[ab] = true
    }
    var pool []string
    for _, ab := range ruleset.AllAbilities() {
        if !excluded[ab] {
            pool = append(pool, ab)
        }
    }
    return pool
}
```

Note: you will need to add `"github.com/cory-johannsen/mud/internal/game/ruleset"` to the import block in `builder.go`.

**Step 4: Run tests**

```bash
go test ./internal/game/character/... -run "TestApplyAbilityBoosts|TestAbilityBoostPool|TestProperty_Apply" -v
```

Expected: all PASS

**Step 5: Run full character suite**

```bash
go test ./internal/game/character/... -v 2>&1 | tail -5
```

**Step 6: Commit**

```bash
git add internal/game/character/builder.go internal/game/character/ability_boost_test.go
git commit -m "feat: implement ApplyAbilityBoosts and AbilityBoostPool in character builder"
```

---

### Task 4: DB migration — `character_ability_boosts` table

**Files:**
- Create: `migrations/016_character_ability_boosts.up.sql`
- Create: `migrations/016_character_ability_boosts.down.sql`

**Step 1: Create the migration files**

`migrations/016_character_ability_boosts.up.sql`:
```sql
CREATE TABLE character_ability_boosts (
    character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    source        TEXT    NOT NULL,  -- "archetype" or "region"
    ability       TEXT    NOT NULL,
    PRIMARY KEY (character_id, source, ability)
);
```

`migrations/016_character_ability_boosts.down.sql`:
```sql
DROP TABLE IF EXISTS character_ability_boosts;
```

**Step 2: Verify migration runs cleanly**

```bash
go run ./cmd/migrate/main.go up
```

Expected: migration 016 applied without error.

**Step 3: Commit**

```bash
git add migrations/016_character_ability_boosts.up.sql migrations/016_character_ability_boosts.down.sql
git commit -m "feat: add character_ability_boosts migration"
```

---

### Task 5: Postgres repository for ability boosts

**Files:**
- Create: `internal/storage/postgres/character_ability_boosts.go`
- Create: `internal/storage/postgres/character_ability_boosts_test.go`

**Step 1: Write the failing tests**

Create `internal/storage/postgres/character_ability_boosts_test.go`:

```go
package postgres_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCharacterAbilityBoostsRepo_AddAndGetAll(t *testing.T) {
    db := testDB(t) // helper that spins up test postgres with migrations applied
    repo := postgres.NewCharacterAbilityBoostsRepo(db)
    charID := testCharacterID(t, db) // creates a test character and returns its ID

    ctx := context.Background()
    require.NoError(t, repo.Add(ctx, charID, "archetype", "brutality"))
    require.NoError(t, repo.Add(ctx, charID, "archetype", "grit"))
    require.NoError(t, repo.Add(ctx, charID, "region", "quickness"))

    choices, err := repo.GetAll(ctx, charID)
    require.NoError(t, err)
    assert.ElementsMatch(t, []string{"brutality", "grit"}, choices["archetype"])
    assert.ElementsMatch(t, []string{"quickness"}, choices["region"])
}

func TestCharacterAbilityBoostsRepo_AddIdempotent(t *testing.T) {
    db := testDB(t)
    repo := postgres.NewCharacterAbilityBoostsRepo(db)
    charID := testCharacterID(t, db)

    ctx := context.Background()
    require.NoError(t, repo.Add(ctx, charID, "archetype", "brutality"))
    // Adding again must not return an error (ON CONFLICT DO NOTHING).
    require.NoError(t, repo.Add(ctx, charID, "archetype", "brutality"))

    choices, err := repo.GetAll(ctx, charID)
    require.NoError(t, err)
    assert.Len(t, choices["archetype"], 1)
}

func TestCharacterAbilityBoostsRepo_GetAll_Empty(t *testing.T) {
    db := testDB(t)
    repo := postgres.NewCharacterAbilityBoostsRepo(db)
    charID := testCharacterID(t, db)

    choices, err := repo.GetAll(context.Background(), charID)
    require.NoError(t, err)
    assert.Empty(t, choices)
}
```

Note: `testDB` and `testCharacterID` are existing helpers in the postgres test package. Look at `internal/storage/postgres/character_feature_choices_test.go` for the pattern.

**Step 2: Run to verify failures**

```bash
go test ./internal/storage/postgres/... -run "TestCharacterAbilityBoostsRepo" -v
```

Expected: FAIL (NewCharacterAbilityBoostsRepo undefined)

**Step 3: Implement `internal/storage/postgres/character_ability_boosts.go`**

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

// CharacterAbilityBoostsRepo persists and retrieves per-character free ability boost selections.
type CharacterAbilityBoostsRepo struct {
    db *pgxpool.Pool
}

// NewCharacterAbilityBoostsRepo constructs a CharacterAbilityBoostsRepo.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repo.
func NewCharacterAbilityBoostsRepo(db *pgxpool.Pool) *CharacterAbilityBoostsRepo {
    return &CharacterAbilityBoostsRepo{db: db}
}

// GetAll returns all stored free boost choices for characterID as source → []ability.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *CharacterAbilityBoostsRepo) GetAll(ctx context.Context, characterID int64) (map[string][]string, error) {
    rows, err := r.db.Query(ctx,
        `SELECT source, ability FROM character_ability_boosts WHERE character_id = $1`,
        characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("CharacterAbilityBoostsRepo.GetAll: %w", err)
    }
    defer rows.Close()

    out := make(map[string][]string)
    for rows.Next() {
        var source, ability string
        if err := rows.Scan(&source, &ability); err != nil {
            return nil, fmt.Errorf("CharacterAbilityBoostsRepo.GetAll scan: %w", err)
        }
        out[source] = append(out[source], ability)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("CharacterAbilityBoostsRepo.GetAll rows: %w", err)
    }
    return out, nil
}

// Add stores a free boost choice for characterID. Duplicate adds are silently ignored.
//
// Precondition: characterID > 0; source must be "archetype" or "region"; ability must be non-empty.
// Postcondition: Exactly one row exists for (character_id, source, ability).
func (r *CharacterAbilityBoostsRepo) Add(ctx context.Context, characterID int64, source, ability string) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO character_ability_boosts (character_id, source, ability)
         VALUES ($1, $2, $3)
         ON CONFLICT DO NOTHING`,
        characterID, source, ability,
    )
    if err != nil {
        return fmt.Errorf("CharacterAbilityBoostsRepo.Add: %w", err)
    }
    return nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/storage/postgres/... -run "TestCharacterAbilityBoostsRepo" -v
```

Expected: all PASS

**Step 5: Commit**

```bash
git add internal/storage/postgres/character_ability_boosts.go \
        internal/storage/postgres/character_ability_boosts_test.go
git commit -m "feat: add CharacterAbilityBoostsRepo postgres implementation"
```

---

### Task 6: Add `SaveAbilities` to `CharacterSaver` and `CharacterRepository`

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (CharacterSaver interface)
- Modify: `internal/storage/postgres/character.go`
- Modify: `internal/storage/postgres/character_test.go`

**Step 1: Write failing test**

Add to `internal/storage/postgres/character_test.go`:

```go
func TestCharacterRepository_SaveAbilities(t *testing.T) {
    db := testDB(t)
    repo := postgres.NewCharacterRepository(db)
    charID := testCharacterID(t, db)

    ctx := context.Background()
    newScores := character.AbilityScores{
        Brutality: 14, Grit: 12, Quickness: 16,
        Reasoning: 10, Savvy: 12, Flair: 10,
    }
    require.NoError(t, repo.SaveAbilities(ctx, charID, newScores))

    loaded, err := repo.GetByID(ctx, charID)
    require.NoError(t, err)
    assert.Equal(t, newScores, loaded.Abilities)
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/storage/postgres/... -run "TestCharacterRepository_SaveAbilities" -v
```

Expected: FAIL (SaveAbilities undefined)

**Step 3: Add `SaveAbilities` to `CharacterSaver` interface in `grpc_service.go`**

In the `CharacterSaver` interface, add:
```go
// SaveAbilities persists updated ability scores for the character with the given ID.
// Postcondition: Character record reflects the new scores.
SaveAbilities(ctx context.Context, id int64, abilities character.AbilityScores) error
```

**Step 4: Implement `SaveAbilities` in `internal/storage/postgres/character.go`**

```go
// SaveAbilities persists updated ability scores for the character with the given id.
//
// Precondition: id > 0.
// Postcondition: The characters row reflects the new ability scores.
func (r *CharacterRepository) SaveAbilities(ctx context.Context, id int64, abilities character.AbilityScores) error {
    _, err := r.db.Exec(ctx,
        `UPDATE characters
         SET brutality=$2, grit=$3, quickness=$4, reasoning=$5, savvy=$6, flair=$7, updated_at=NOW()
         WHERE id=$1`,
        id,
        abilities.Brutality, abilities.Grit, abilities.Quickness,
        abilities.Reasoning, abilities.Savvy, abilities.Flair,
    )
    if err != nil {
        return fmt.Errorf("CharacterRepository.SaveAbilities: %w", err)
    }
    return nil
}
```

**Step 5: Run tests**

```bash
go test ./internal/storage/postgres/... -run "TestCharacterRepository_SaveAbilities" -v
```

Expected: PASS

**Step 6: Fix any CharacterSaver mock in tests that doesn't implement SaveAbilities**

Search for mock implementations:
```bash
grep -rn "SaveAbilities\|CharacterSaver" /home/cjohannsen/src/mud --include="*.go" | grep -v "^Binary" | head -20
```

Any struct implementing `CharacterSaver` that's missing `SaveAbilities` must have a stub added:
```go
func (m *mockCharSaver) SaveAbilities(_ context.Context, _ int64, _ character.AbilityScores) error {
    return nil
}
```

**Step 7: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all PASS

**Step 8: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/storage/postgres/character.go \
        internal/storage/postgres/character_test.go
git commit -m "feat: add SaveAbilities to CharacterSaver and CharacterRepository"
```

---

### Task 7: Add ArchetypeRegistry and RegionRegistry; wire into GameServiceServer

**Files:**
- Create: `internal/game/ruleset/archetype_registry.go`
- Create: `internal/game/ruleset/archetype_registry_test.go`
- Create: `internal/game/ruleset/region_registry.go`
- Create: `internal/game/ruleset/region_registry_test.go`
- Modify: `internal/gameserver/grpc_service.go` (new fields + constructor arg)
- Modify: `cmd/gameserver/main.go`

**Step 1: Write failing tests for registries**

Create `internal/game/ruleset/archetype_registry_test.go`:
```go
package ruleset_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestArchetypeRegistry_Lookup(t *testing.T) {
    archetypes := []*ruleset.Archetype{
        {ID: "aggressor", Name: "Aggressor"},
        {ID: "nerd", Name: "Nerd"},
    }
    reg := ruleset.NewArchetypeRegistry(archetypes)

    a, ok := reg.Archetype("aggressor")
    assert.True(t, ok)
    assert.Equal(t, "Aggressor", a.Name)

    _, ok = reg.Archetype("missing")
    assert.False(t, ok)
}
```

Create `internal/game/ruleset/region_registry_test.go`:
```go
package ruleset_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestRegionRegistry_Lookup(t *testing.T) {
    regions := []*ruleset.Region{
        {ID: "north_portland", Name: "North Portland"},
    }
    reg := ruleset.NewRegionRegistry(regions)

    r, ok := reg.Region("north_portland")
    assert.True(t, ok)
    assert.Equal(t, "North Portland", r.Name)

    _, ok = reg.Region("missing")
    assert.False(t, ok)
}
```

**Step 2: Run to verify failures**

```bash
go test ./internal/game/ruleset/... -run "TestArchetypeRegistry_Lookup|TestRegionRegistry_Lookup" -v
```

Expected: FAIL

**Step 3: Implement registries**

Create `internal/game/ruleset/archetype_registry.go`:
```go
package ruleset

// ArchetypeRegistry provides O(1) lookup of Archetype by ID.
type ArchetypeRegistry struct {
    byID map[string]*Archetype
}

// NewArchetypeRegistry builds a registry from a slice of archetypes.
//
// Precondition: archetypes must not be nil.
// Postcondition: All archetypes are indexed by ID.
func NewArchetypeRegistry(archetypes []*Archetype) *ArchetypeRegistry {
    m := make(map[string]*Archetype, len(archetypes))
    for _, a := range archetypes {
        m[a.ID] = a
    }
    return &ArchetypeRegistry{byID: m}
}

// Archetype returns the archetype with the given ID, and whether it was found.
func (r *ArchetypeRegistry) Archetype(id string) (*Archetype, bool) {
    a, ok := r.byID[id]
    return a, ok
}
```

Create `internal/game/ruleset/region_registry.go`:
```go
package ruleset

// RegionRegistry provides O(1) lookup of Region by ID.
type RegionRegistry struct {
    byID map[string]*Region
}

// NewRegionRegistry builds a registry from a slice of regions.
//
// Precondition: regions must not be nil.
// Postcondition: All regions are indexed by ID.
func NewRegionRegistry(regions []*Region) *RegionRegistry {
    m := make(map[string]*Region, len(regions))
    for _, r := range regions {
        m[r.ID] = r
    }
    return &RegionRegistry{byID: m}
}

// Region returns the region with the given ID, and whether it was found.
func (r *RegionRegistry) Region(id string) (*Region, bool) {
    reg, ok := r.byID[id]
    return reg, ok
}
```

**Step 4: Add fields to `GameServiceServer` in `grpc_service.go`**

In the `GameServiceServer` struct, add:
```go
archetypeRegistry    *ruleset.ArchetypeRegistry
regionRegistry       *ruleset.RegionRegistry
abilityBoostsRepo   CharacterAbilityBoostsRepository
```

Add the interface near the other repo interfaces:
```go
// CharacterAbilityBoostsRepository persists and retrieves per-character free ability boost choices.
//
// Precondition: characterID must be > 0.
// Postcondition: GetAll returns a non-nil map; Add is idempotent.
type CharacterAbilityBoostsRepository interface {
    GetAll(ctx context.Context, characterID int64) (map[string][]string, error)
    Add(ctx context.Context, characterID int64, source, ability string) error
}
```

Add parameters to `NewGameServiceServer`:
```go
archetypeRegistry *ruleset.ArchetypeRegistry,
regionRegistry *ruleset.RegionRegistry,
abilityBoostsRepo CharacterAbilityBoostsRepository,
```

Wire them in the constructor body:
```go
archetypeRegistry:  archetypeRegistry,
regionRegistry:     regionRegistry,
abilityBoostsRepo:  abilityBoostsRepo,
```

**Step 5: Wire in `cmd/gameserver/main.go`**

After loading archetypes and regions (they're already loaded in the gameserver main — check how they're passed around and add the registries):

```go
archetypeReg := ruleset.NewArchetypeRegistry(archetypes)
regionReg := ruleset.NewRegionRegistry(regions)
abilityBoostsRepo := postgres.NewCharacterAbilityBoostsRepo(pool.DB())
```

Pass them to `NewGameServiceServer`.

**Step 6: Fix any tests broken by the constructor signature change**

Search for calls to `NewGameServiceServer` in tests:
```bash
grep -rn "NewGameServiceServer" /home/cjohannsen/src/mud --include="*.go" | head -20
```

Add `nil, nil, nil` for the three new params in any test that calls the old constructor.

**Step 7: Run all tests**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all PASS

**Step 8: Commit**

```bash
git add internal/game/ruleset/archetype_registry.go internal/game/ruleset/archetype_registry_test.go \
        internal/game/ruleset/region_registry.go internal/game/ruleset/region_registry_test.go \
        internal/gameserver/grpc_service.go cmd/gameserver/main.go
git commit -m "feat: add ArchetypeRegistry and RegionRegistry; wire into GameServiceServer"
```

---

### Task 8: Prompt for missing ability boost choices at login and apply

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (Session method)
- Create: `internal/gameserver/grpc_service_ability_boost_test.go`

**Step 1: Write the failing integration test**

Create `internal/gameserver/grpc_service_ability_boost_test.go`:

```go
package gameserver_test

// TestSession_AbilityBoostsPromptedWhenMissing verifies that a character
// missing archetype and region ability boost selections is prompted for them
// after the initial room view, and that the choices are persisted and applied.
//
// Follow the pattern from grpc_service_login_test.go (TestSession_FavoredTargetPromptedWhenMissing).
// The test should:
//  1. Set up a GameServiceServer with mock repos including abilityBoostsRepo (returns empty map).
//  2. Send a JoinSession request for a character with class="boot_gun" (archetype=aggressor, region=north_portland).
//  3. Receive the initial RoomView.
//  4. Receive the archetype free boost 1 prompt message.
//  5. Send "1" (selects first available ability).
//  6. Receive the archetype free boost 2 prompt message.
//  7. Send "1".
//  8. Receive the region free boost prompt message.
//  9. Send "1".
// 10. Verify abilityBoostsRepo.Add was called 3 times.
// 11. Verify sess.Abilities reflects the applied boosts.
//
// Use the existing test infrastructure from grpc_service_login_test.go.
// The mock abilityBoostsRepo must implement CharacterAbilityBoostsRepository.
```

(This test is complex — follow the existing test pattern closely. The test file documents the expectations; the subagent implementing this task must study `grpc_service_login_test.go` before writing the test.)

**Step 2: Implement the prompt loop in `Session()`**

In `grpc_service.go`, inside the `Session()` method, **after the existing feat/class-feature choice prompts** (after the `sess.FavoredTarget` derivation, before the goroutine spawning), add:

```go
// Resolve missing ability boost choices.
if characterID > 0 && s.abilityBoostsRepo != nil && s.archetypeRegistry != nil && s.regionRegistry != nil {
    storedBoosts, boostErr := s.abilityBoostsRepo.GetAll(stream.Context(), characterID)
    if boostErr != nil {
        s.logger.Warn("loading ability boosts", zap.Int64("character_id", characterID), zap.Error(boostErr))
        storedBoosts = map[string][]string{}
    }

    // Determine character's archetype from job.
    archetypeID := ""
    if s.jobRegistry != nil {
        if job, ok := s.jobRegistry.Job(sess.Class); ok {
            archetypeID = job.Archetype
        }
    }

    // Prompt for archetype free boosts (2 slots).
    if archetypeID != "" {
        if archetype, ok := s.archetypeRegistry.Archetype(archetypeID); ok && archetype.AbilityBoosts != nil {
            chosenForArchetype := storedBoosts["archetype"]
            needed := archetype.AbilityBoosts.Free - len(chosenForArchetype)
            for i := 0; i < needed; i++ {
                pool := character.AbilityBoostPool(archetype.AbilityBoosts.Fixed, chosenForArchetype)
                if len(pool) == 0 {
                    break
                }
                choices := &ruleset.FeatureChoices{
                    Prompt:  fmt.Sprintf("Choose archetype free ability boost %d of %d:", i+1+len(storedBoosts["archetype"]), archetype.AbilityBoosts.Free),
                    Options: pool,
                    Key:     fmt.Sprintf("archetype_boost_%d", i),
                }
                chosen, promptErr := s.promptFeatureChoice(stream, "archetype_boost", choices)
                if promptErr != nil || chosen == "" {
                    break
                }
                if addErr := s.abilityBoostsRepo.Add(stream.Context(), characterID, "archetype", chosen); addErr != nil {
                    s.logger.Warn("persisting archetype boost", zap.Error(addErr))
                }
                chosenForArchetype = append(chosenForArchetype, chosen)
            }
            storedBoosts["archetype"] = chosenForArchetype
        }
    }

    // Prompt for region free boost (1 slot).
    regionID := dbChar.Region // dbChar is the character loaded from DB at the top of Session()
    if region, ok := s.regionRegistry.Region(regionID); ok && region.AbilityBoosts != nil {
        chosenForRegion := storedBoosts["region"]
        needed := region.AbilityBoosts.Free - len(chosenForRegion)
        for i := 0; i < needed; i++ {
            pool := character.AbilityBoostPool(region.AbilityBoosts.Fixed, chosenForRegion)
            if len(pool) == 0 {
                break
            }
            choices := &ruleset.FeatureChoices{
                Prompt:  fmt.Sprintf("Choose region free ability boost %d of %d:", i+1+len(storedBoosts["region"]), region.AbilityBoosts.Free),
                Options: pool,
                Key:     fmt.Sprintf("region_boost_%d", i),
            }
            chosen, promptErr := s.promptFeatureChoice(stream, "region_boost", choices)
            if promptErr != nil || chosen == "" {
                break
            }
            if addErr := s.abilityBoostsRepo.Add(stream.Context(), characterID, "region", chosen); addErr != nil {
                s.logger.Warn("persisting region boost", zap.Error(addErr))
            }
            chosenForRegion = append(chosenForRegion, chosen)
        }
        storedBoosts["region"] = chosenForRegion
    }

    // Apply all boosts and recompute character abilities.
    archetypeBoosts := (*ruleset.AbilityBoostGrant)(nil)
    if archetypeID != "" {
        if archetype, ok := s.archetypeRegistry.Archetype(archetypeID); ok {
            archetypeBoosts = archetype.AbilityBoosts
        }
    }
    regionBoosts := (*ruleset.AbilityBoostGrant)(nil)
    regionID2 := dbChar.Region
    if region, ok := s.regionRegistry.Region(regionID2); ok {
        regionBoosts = region.AbilityBoosts
    }

    // Recompute from the base scores stored in the DB character record.
    // dbChar.Abilities already has: base 10 + region modifiers + job key ability.
    // We add the archetype and region boosts on top.
    // NOTE: To avoid double-applying boosts on repeat logins, we must compute from
    // a pre-boost baseline. Store the raw (pre-boost) scores in a separate approach.
    //
    // SIMPLER: Track whether boosts have been applied by checking if ability scores
    // already include the expected boosts. Instead, recompute the base scores from
    // scratch using the region modifiers and job key ability, then apply all boosts.
    baseScores := recomputeBaseScores(dbChar, s.regionRegistry, s.jobRegistry)
    newAbilities := character.ApplyAbilityBoosts(
        baseScores,
        archetypeBoosts, storedBoosts["archetype"],
        regionBoosts, storedBoosts["region"],
    )

    // Update in-memory session and persist.
    sess.Abilities = newAbilities
    if s.charSaver != nil {
        if saveErr := s.charSaver.SaveAbilities(stream.Context(), characterID, newAbilities); saveErr != nil {
            s.logger.Warn("saving ability boosts", zap.Error(saveErr))
        }
    }
}
```

Add `recomputeBaseScores` helper near the bottom of `grpc_service.go`:

```go
// recomputeBaseScores returns the ability scores for a character before any
// archetype or region boost choices are applied: base 10 + region modifiers + job key ability.
//
// Precondition: dbChar must not be nil.
// Postcondition: Returns scores reflecting only fixed sources (region modifiers and job key ability).
func recomputeBaseScores(dbChar *character.Character, regionReg *ruleset.RegionRegistry, jobReg *ruleset.JobRegistry) character.AbilityScores {
    base := character.AbilityScores{
        Brutality: 10, Grit: 10, Quickness: 10,
        Reasoning: 10, Savvy: 10, Flair: 10,
    }
    // Apply region modifiers.
    if regionReg != nil {
        if region, ok := regionReg.Region(dbChar.Region); ok {
            for ab, delta := range region.Modifiers {
                switch ab {
                case "brutality": base.Brutality += delta
                case "grit": base.Grit += delta
                case "quickness": base.Quickness += delta
                case "reasoning": base.Reasoning += delta
                case "savvy": base.Savvy += delta
                case "flair": base.Flair += delta
                }
            }
        }
    }
    // Apply job key ability boost (+2).
    if jobReg != nil {
        if job, ok := jobReg.Job(dbChar.Class); ok {
            switch job.KeyAbility {
            case "brutality": base.Brutality += 2
            case "grit": base.Grit += 2
            case "quickness": base.Quickness += 2
            case "reasoning": base.Reasoning += 2
            case "savvy": base.Savvy += 2
            case "flair": base.Flair += 2
            }
        }
    }
    return base
}
```

Also add `Abilities` to `PlayerSession` if not present (check `internal/game/session/manager.go`):
```go
// Abilities holds the character's computed ability scores (base + all boosts).
Abilities character.AbilityScores
```

Note: `dbChar` must be accessible in scope. It's already loaded at the top of `Session()` via `s.charSaver.GetByID`. Confirm it's stored in a local variable accessible in the boost prompt section.

**Step 3: Run all tests**

```bash
go test ./internal/gameserver/... -v 2>&1 | tail -30
```

Fix any compilation errors. Run again until all PASS.

**Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_ability_boost_test.go \
        internal/game/session/manager.go
git commit -m "feat: prompt for ability boost choices at login and apply to character scores"
```

---

### Task 9: Deploy, verify, mark done

**Step 1: Run full test suite**

```bash
go test ./... 2>&1 | tail -30
```

Expected: all PASS

**Step 2: Deploy**

```bash
make k8s-redeploy
kubectl rollout restart deployment/frontend deployment/gameserver -n mud
kubectl rollout status deployment/frontend deployment/gameserver -n mud --timeout=60s
```

**Step 3: Run DB migration on the cluster**

```bash
kubectl exec -n mud deployment/gameserver -- /bin/migrate up 2>&1 || \
  go run ./cmd/migrate/main.go up
```

(Check how migrations are run in this project — look at `cmd/migrate/main.go` and the k8s setup.)

**Step 4: Mark done in FEATURES.md**

Change:
```
- [ ] Character ability boosts
  - [ ] At creation player's get to select attributes boosts (as in P2FE). ...
  - [ ] Player's that have not selected boosts must be prompted at login
```
to:
```
- [x] Character ability boosts
  - [x] At creation player's get to select attributes boosts (as in P2FE). ...
  - [x] Player's that have not selected boosts must be prompted at login
```

**Step 5: Commit and push**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark character ability boosts as complete"
git push
```
