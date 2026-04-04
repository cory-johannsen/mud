# Feat Level-Up Grant System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a feat level-up grant system that auto-awards archetype feats on level-up and retroactively backfills missing feats at session start.

**Architecture:** Add `LevelUpFeatGrants map[int]*FeatGrants` to both `Archetype` and `Job` structs (parallel to existing `LevelUpGrants` for technologies). On level-up, auto-assign fixed feats and auto-pick choices from pool (first N not already owned — idempotent). A `BackfillLevelUpFeats` function runs at session start before feats are loaded into the session, ensuring existing characters receive all missing grants. No new DB tables required — feats go into the existing `character_feats` table.

**Tech Stack:** Go, pgx/v5, `pgregory.net/rapid` for property tests, YAML content files.

---

## File Structure

| File | Change |
|------|--------|
| `internal/game/ruleset/job.go` | Add `LevelUpFeatGrants map[int]*FeatGrants` field to `Job` struct |
| `internal/game/ruleset/archetype.go` | Add `LevelUpFeatGrants map[int]*FeatGrants` field to `Archetype` struct |
| `internal/game/ruleset/feat.go` | Add `MergeFeatLevelUpGrants` and `MergeFeatGrants` helpers |
| `internal/storage/postgres/character_feats.go` | Add `Add(ctx, characterID, featID)` method |
| `internal/gameserver/grpc_service.go` | Widen `CharacterFeatsGetter` → `CharacterFeatsRepo`; update `handleGrant`; update `Session()` |
| `internal/gameserver/feat_levelup_grant.go` | New: `ApplyFeatGrant`, `BackfillLevelUpFeats` |
| `internal/gameserver/feat_levelup_grant_test.go` | New: tests for above |
| `content/archetypes/aggressor.yaml` | Add `level_up_feat_grants` (levels 2–20) |
| `content/archetypes/criminal.yaml` | Add `level_up_feat_grants` (levels 2–20) |

---

## Task 1: Add `LevelUpFeatGrants` to `Job` and `Archetype` structs

**Files:**
- Modify: `internal/game/ruleset/job.go`
- Modify: `internal/game/ruleset/archetype.go`
- Test: `internal/game/ruleset/archetype_test.go` (create if not exists)

- [ ] **Step 1: Write the failing test**

Create `internal/game/ruleset/archetype_levelup_feat_test.go`:

```go
package ruleset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestArchetype_LevelUpFeatGrants_ParsesFromYAML(t *testing.T) {
	raw := `
id: test_archetype
name: Test
key_ability: brutality
hit_points_per_level: 10
level_up_feat_grants:
  2:
    choices:
      pool: [feat_a, feat_b]
      count: 1
  4:
    fixed: [feat_c]
`
	var a Archetype
	require.NoError(t, yaml.Unmarshal([]byte(raw), &a))
	require.NotNil(t, a.LevelUpFeatGrants)
	assert.NotNil(t, a.LevelUpFeatGrants[2])
	assert.Equal(t, 1, a.LevelUpFeatGrants[2].Choices.Count)
	assert.Equal(t, []string{"feat_a", "feat_b"}, a.LevelUpFeatGrants[2].Choices.Pool)
	assert.NotNil(t, a.LevelUpFeatGrants[4])
	assert.Equal(t, []string{"feat_c"}, a.LevelUpFeatGrants[4].Fixed)
}

func TestJob_LevelUpFeatGrants_ParsesFromYAML(t *testing.T) {
	raw := `
id: test_job
name: Test Job
archetype: test_archetype
tier: 1
key_ability: brutality
hit_points_per_level: 10
level_up_feat_grants:
  2:
    fixed: [job_feat_a]
`
	var j Job
	require.NoError(t, yaml.Unmarshal([]byte(raw), &j))
	require.NotNil(t, j.LevelUpFeatGrants)
	assert.Equal(t, []string{"job_feat_a"}, j.LevelUpFeatGrants[2].Fixed)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/game/ruleset/... -run "TestArchetype_LevelUpFeatGrants|TestJob_LevelUpFeatGrants" -v
```

Expected: FAIL — `Archetype has no field LevelUpFeatGrants`

- [ ] **Step 3: Add `LevelUpFeatGrants` to `Archetype`**

In `internal/game/ruleset/archetype.go`, add the field after `LevelUpGrants`:

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
}
```

- [ ] **Step 4: Add `LevelUpFeatGrants` to `Job`**

In `internal/game/ruleset/job.go`, add after `LevelUpGrants`:

```go
LevelUpFeatGrants  map[int]*FeatGrants       `yaml:"level_up_feat_grants,omitempty"`
```

The `FeatGrants` struct already exists in `job.go` (lines ~64-71).

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/game/ruleset/... -run "TestArchetype_LevelUpFeatGrants|TestJob_LevelUpFeatGrants" -v
```

Expected: PASS

- [ ] **Step 6: Run full ruleset tests**

```bash
go test ./internal/game/ruleset/... -timeout 60s
```

Expected: all pass

- [ ] **Step 7: Commit**

```bash
git add internal/game/ruleset/archetype.go internal/game/ruleset/job.go internal/game/ruleset/archetype_levelup_feat_test.go
git commit -m "feat(ruleset): add LevelUpFeatGrants field to Archetype and Job structs"
```

---

## Task 2: Add `MergeFeatLevelUpGrants` helper

**Files:**
- Modify: `internal/game/ruleset/feat.go`
- Test: `internal/game/ruleset/feat_merge_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `internal/game/ruleset/feat_merge_test.go`:

```go
package ruleset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestMergeFeatGrants_NilInputs(t *testing.T) {
	assert.Nil(t, MergeFeatGrants(nil, nil))
	assert.Equal(t, &FeatGrants{Fixed: []string{"a"}}, MergeFeatGrants(&FeatGrants{Fixed: []string{"a"}}, nil))
	assert.Equal(t, &FeatGrants{Fixed: []string{"b"}}, MergeFeatGrants(nil, &FeatGrants{Fixed: []string{"b"}}))
}

func TestMergeFeatGrants_MergesFixed(t *testing.T) {
	a := &FeatGrants{Fixed: []string{"feat_a"}}
	b := &FeatGrants{Fixed: []string{"feat_b"}}
	merged := MergeFeatGrants(a, b)
	assert.ElementsMatch(t, []string{"feat_a", "feat_b"}, merged.Fixed)
}

func TestMergeFeatGrants_MergesChoices(t *testing.T) {
	a := &FeatGrants{Choices: &FeatChoices{Pool: []string{"x", "y"}, Count: 1}}
	b := &FeatGrants{Choices: &FeatChoices{Pool: []string{"z"}, Count: 1}}
	merged := MergeFeatGrants(a, b)
	assert.Equal(t, 2, merged.Choices.Count)
	assert.ElementsMatch(t, []string{"x", "y", "z"}, merged.Choices.Pool)
}

func TestMergeFeatLevelUpGrants_NilInputs(t *testing.T) {
	assert.Nil(t, MergeFeatLevelUpGrants(nil, nil))
}

func TestMergeFeatLevelUpGrants_MergesByLevel(t *testing.T) {
	arch := map[int]*FeatGrants{
		2: {Choices: &FeatChoices{Pool: []string{"a"}, Count: 1}},
	}
	job := map[int]*FeatGrants{
		2: {Fixed: []string{"b"}},
		4: {Fixed: []string{"c"}},
	}
	merged := MergeFeatLevelUpGrants(arch, job)
	assert.ElementsMatch(t, []string{"b"}, merged[2].Fixed)
	assert.Equal(t, []string{"a"}, merged[2].Choices.Pool)
	assert.Equal(t, []string{"c"}, merged[4].Fixed)
}

func TestProperty_MergeFeatLevelUpGrants_ContainsAllKeys(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		archKeys := rapid.SliceOfN(rapid.IntRange(2, 10), 0, 5).Draw(rt, "archKeys")
		jobKeys  := rapid.SliceOfN(rapid.IntRange(2, 10), 0, 5).Draw(rt, "jobKeys")

		arch := make(map[int]*FeatGrants)
		for _, k := range archKeys {
			arch[k] = &FeatGrants{Fixed: []string{"arch_feat"}}
		}
		job := make(map[int]*FeatGrants)
		for _, k := range jobKeys {
			job[k] = &FeatGrants{Fixed: []string{"job_feat"}}
		}

		merged := MergeFeatLevelUpGrants(arch, job)

		for _, k := range archKeys {
			if _, ok := merged[k]; !ok {
				rt.Fatalf("archetype key %d missing from merged result", k)
			}
		}
		for _, k := range jobKeys {
			if _, ok := merged[k]; !ok {
				rt.Fatalf("job key %d missing from merged result", k)
			}
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/game/ruleset/... -run "TestMergeFeat" -v
```

Expected: FAIL — `MergeFeatGrants undefined`

- [ ] **Step 3: Add merge helpers to `internal/game/ruleset/feat.go`**

Append to the end of `internal/game/ruleset/feat.go`:

```go
// MergeFeatGrants merges two FeatGrants into a single combined grant.
// Fixed lists are unioned; Choices pools are unioned with summed counts.
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned grant contains all fixed IDs and all pool entries from both inputs.
func MergeFeatGrants(a, b *FeatGrants) *FeatGrants {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := &FeatGrants{
		GeneralCount: a.GeneralCount + b.GeneralCount,
		Fixed:        append(append([]string(nil), a.Fixed...), b.Fixed...),
	}
	if a.Choices != nil || b.Choices != nil {
		merged.Choices = &FeatChoices{}
		if a.Choices != nil {
			merged.Choices.Count += a.Choices.Count
			merged.Choices.Pool = append(merged.Choices.Pool, a.Choices.Pool...)
		}
		if b.Choices != nil {
			merged.Choices.Count += b.Choices.Count
			merged.Choices.Pool = append(merged.Choices.Pool, b.Choices.Pool...)
		}
	}
	return merged
}

// MergeFeatLevelUpGrants merges two level-keyed feat grant maps.
// Keys present in both are merged via MergeFeatGrants.
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned map contains all keys from both inputs.
func MergeFeatLevelUpGrants(archetype, job map[int]*FeatGrants) map[int]*FeatGrants {
	if len(archetype) == 0 && len(job) == 0 {
		return nil
	}
	out := make(map[int]*FeatGrants)
	for lvl, g := range archetype {
		out[lvl] = g
	}
	for lvl, g := range job {
		if existing, ok := out[lvl]; ok {
			out[lvl] = MergeFeatGrants(existing, g)
		} else {
			out[lvl] = g
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/game/ruleset/... -run "TestMergeFeat" -v
```

Expected: PASS (all merge tests including property test with 100 cases)

- [ ] **Step 5: Run full ruleset tests**

```bash
go test ./internal/game/ruleset/... -timeout 60s
```

Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/game/ruleset/feat.go internal/game/ruleset/feat_merge_test.go
git commit -m "feat(ruleset): add MergeFeatGrants and MergeFeatLevelUpGrants helpers"
```

---

## Task 3: Add `Add` method to `CharacterFeatsRepository` and widen interface

**Files:**
- Modify: `internal/storage/postgres/character_feats.go`
- Modify: `internal/gameserver/grpc_service.go` (interface only)

- [ ] **Step 1: Write the failing test**

In `internal/storage/postgres/character_feats_test.go` (create if not exists, or add to existing):

```go
package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterFeatsRepository_Add(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charID := createTestCharacter(t, db)
	repo := NewCharacterFeatsRepository(db)

	// Start empty.
	feats, err := repo.GetAll(ctx, charID)
	require.NoError(t, err)
	assert.Empty(t, feats)

	// Add one feat.
	require.NoError(t, repo.Add(ctx, charID, "feat_a"))
	feats, err = repo.GetAll(ctx, charID)
	require.NoError(t, err)
	assert.Equal(t, []string{"feat_a"}, feats)

	// Add another; original must still be present.
	require.NoError(t, repo.Add(ctx, charID, "feat_b"))
	feats, err = repo.GetAll(ctx, charID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"feat_a", "feat_b"}, feats)

	// Adding a duplicate must be a no-op (not an error).
	require.NoError(t, repo.Add(ctx, charID, "feat_a"))
	feats, err = repo.GetAll(ctx, charID)
	require.NoError(t, err)
	assert.Len(t, feats, 2)
}
```

> **Note:** `testDB(t)` and `createTestCharacter(t, db)` are helpers that already exist in the postgres test package — use the same pattern as other postgres integration tests (check `internal/storage/postgres/main_test.go` for the helper names used by existing tests).

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/storage/postgres/... -run "TestCharacterFeatsRepository_Add" -v
```

Expected: FAIL — `Add method not found`

- [ ] **Step 3: Add `Add` method to `CharacterFeatsRepository`**

In `internal/storage/postgres/character_feats.go`, append after `SetAll`:

```go
// Add inserts a single feat for a character. If the feat already exists, it is
// a no-op (INSERT … ON CONFLICT DO NOTHING).
//
// Precondition: characterID > 0; featID non-empty.
// Postcondition: feat_id row exists for this character.
func (r *CharacterFeatsRepository) Add(ctx context.Context, characterID int64, featID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_feats (character_id, feat_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		characterID, featID,
	)
	if err != nil {
		return fmt.Errorf("Add feat %s: %w", featID, err)
	}
	return nil
}
```

- [ ] **Step 4: Widen `CharacterFeatsGetter` to `CharacterFeatsRepo` in `grpc_service.go`**

In `internal/gameserver/grpc_service.go`, replace the existing `CharacterFeatsGetter` interface definition and the `characterFeatsRepo` field declaration:

Find (around line 172):
```go
// CharacterFeatsGetter retrieves the feat IDs assigned to a character.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns a slice of feat IDs (may be empty).
type CharacterFeatsGetter interface {
	GetAll(ctx context.Context, characterID int64) ([]string, error)
}
```

Replace with:
```go
// CharacterFeatsGetter retrieves the feat IDs assigned to a character.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns a slice of feat IDs (may be empty).
type CharacterFeatsGetter interface {
	GetAll(ctx context.Context, characterID int64) ([]string, error)
}

// CharacterFeatsRepo extends CharacterFeatsGetter with mutation needed for
// level-up feat grants.
//
// Precondition: characterID must be > 0; featID must be non-empty.
// Postcondition: Add inserts a feat row; duplicate adds are no-ops.
type CharacterFeatsRepo interface {
	CharacterFeatsGetter
	Add(ctx context.Context, characterID int64, featID string) error
}
```

Then find the field on `GameServiceServer` (around line 233):
```go
characterFeatsRepo         CharacterFeatsGetter
```

Change to:
```go
characterFeatsRepo         CharacterFeatsRepo
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/storage/postgres/... -run "TestCharacterFeatsRepository_Add" -v
```

Expected: PASS

- [ ] **Step 6: Build the full project to confirm no type errors**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/storage/postgres/character_feats.go internal/gameserver/grpc_service.go
git commit -m "feat(feats): add CharacterFeatsRepo.Add for incremental feat grants"
```

---

## Task 4: Implement `ApplyFeatGrant` and `BackfillLevelUpFeats`

**Files:**
- Create: `internal/gameserver/feat_levelup_grant.go`
- Create: `internal/gameserver/feat_levelup_grant_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/gameserver/feat_levelup_grant_test.go`:

```go
package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// ---------------------------------------------------------------------------
// Test double for CharacterFeatsRepo
// ---------------------------------------------------------------------------

type fakeFeatsRepo struct{ feats map[string]bool }

func (r *fakeFeatsRepo) GetAll(_ context.Context, _ int64) ([]string, error) {
	out := make([]string, 0, len(r.feats))
	for id := range r.feats {
		out = append(out, id)
	}
	return out, nil
}
func (r *fakeFeatsRepo) Add(_ context.Context, _ int64, featID string) error {
	if r.feats == nil {
		r.feats = make(map[string]bool)
	}
	r.feats[featID] = true
	return nil
}

// ---------------------------------------------------------------------------
// Tests for ApplyFeatGrant
// ---------------------------------------------------------------------------

func TestApplyFeatGrant_GrantsFixedFeats(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	existing := map[string]bool{}
	grants := &ruleset.FeatGrants{Fixed: []string{"snap_shot", "raging_threat"}}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"snap_shot", "raging_threat"}, granted)
	assert.True(t, repo.feats["snap_shot"])
	assert.True(t, repo.feats["raging_threat"])
}

func TestApplyFeatGrant_SkipsDuplicateFixed(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{feats: map[string]bool{"snap_shot": true}}
	existing := map[string]bool{"snap_shot": true}
	grants := &ruleset.FeatGrants{Fixed: []string{"snap_shot", "raging_threat"}}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.Equal(t, []string{"raging_threat"}, granted)
}

func TestApplyFeatGrant_AutoPicksChoices(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	existing := map[string]bool{}
	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"reactive_block", "overpower", "snap_shot"},
			Count: 1,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.Len(t, granted, 1)
	assert.Equal(t, "reactive_block", granted[0]) // first from pool
}

func TestApplyFeatGrant_SkipsAlreadyOwnedChoices(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{feats: map[string]bool{"reactive_block": true}}
	existing := map[string]bool{"reactive_block": true}
	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"reactive_block", "overpower", "snap_shot"},
			Count: 1,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	// Already has 1 pool feat and count is 1 → no new grant needed.
	assert.Empty(t, granted)
}

func TestApplyFeatGrant_PicksRemainingChoices(t *testing.T) {
	t.Parallel()
	// Has 1 of 2 required choices already.
	repo := &fakeFeatsRepo{feats: map[string]bool{"reactive_block": true}}
	existing := map[string]bool{"reactive_block": true}
	grants := &ruleset.FeatGrants{
		Choices: &ruleset.FeatChoices{
			Pool:  []string{"reactive_block", "overpower", "snap_shot"},
			Count: 2,
		},
	}

	granted, err := ApplyFeatGrant(context.Background(), 1, existing, grants, nil, repo)
	require.NoError(t, err)
	assert.Len(t, granted, 1)
	assert.Equal(t, "overpower", granted[0])
}

// ---------------------------------------------------------------------------
// Tests for BackfillLevelUpFeats
// ---------------------------------------------------------------------------

func TestBackfillLevelUpFeats_NoOpAtLevelOne(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	sess := &session.PlayerSession{Level: 1}
	grants := map[int]*ruleset.FeatGrants{
		2: {Fixed: []string{"snap_shot"}},
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	assert.Empty(t, repo.feats)
}

func TestBackfillLevelUpFeats_GrantsMissingFeats(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{feats: map[string]bool{"creation_feat": true}}
	sess := &session.PlayerSession{Level: 4}
	grants := map[int]*ruleset.FeatGrants{
		2: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_a", "feat_b"}, Count: 1}},
		4: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_a", "feat_b", "feat_c"}, Count: 1}},
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	// Should have granted 2 feats (1 per level), plus creation_feat.
	assert.Len(t, repo.feats, 3)
}

func TestBackfillLevelUpFeats_IdempotentWhenAlreadyGranted(t *testing.T) {
	t.Parallel()
	// Player already has feats from levels 2 and 4.
	repo := &fakeFeatsRepo{feats: map[string]bool{"feat_a": true, "feat_b": true}}
	sess := &session.PlayerSession{Level: 4}
	grants := map[int]*ruleset.FeatGrants{
		2: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_a", "feat_c"}, Count: 1}},
		4: {Choices: &ruleset.FeatChoices{Pool: []string{"feat_b", "feat_d"}, Count: 1}},
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	// No new feats added — already has 1 from each pool.
	assert.Len(t, repo.feats, 2)
}

func TestBackfillLevelUpFeats_SkipsHigherLevels(t *testing.T) {
	t.Parallel()
	repo := &fakeFeatsRepo{}
	sess := &session.PlayerSession{Level: 3}
	grants := map[int]*ruleset.FeatGrants{
		2: {Fixed: []string{"feat_lvl2"}},
		4: {Fixed: []string{"feat_lvl4"}}, // above current level
	}
	err := BackfillLevelUpFeats(context.Background(), sess, 1, grants, nil, repo)
	require.NoError(t, err)
	assert.True(t, repo.feats["feat_lvl2"])
	assert.False(t, repo.feats["feat_lvl4"])
}

func TestProperty_BackfillLevelUpFeats_Idempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(2, 10).Draw(rt, "level")
		pool := []string{"feat_a", "feat_b", "feat_c", "feat_d", "feat_e"}

		grants := make(map[int]*ruleset.FeatGrants)
		for lvl := 2; lvl <= level; lvl += 2 {
			grants[lvl] = &ruleset.FeatGrants{
				Choices: &ruleset.FeatChoices{Pool: pool, Count: 1},
			}
		}

		repo := &fakeFeatsRepo{}
		sess := &session.PlayerSession{Level: level}
		ctx := context.Background()

		// First call.
		require.NoError(rt, BackfillLevelUpFeats(ctx, sess, 1, grants, nil, repo))
		countAfterFirst := len(repo.feats)

		// Second call must be idempotent.
		require.NoError(rt, BackfillLevelUpFeats(ctx, sess, 1, grants, nil, repo))
		assert.Equal(rt, countAfterFirst, len(repo.feats), "second call must not add more feats")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/... -run "TestApplyFeatGrant|TestBackfillLevelUpFeats|TestProperty_BackfillLevelUpFeats" -v 2>&1 | head -30
```

Expected: FAIL — `ApplyFeatGrant undefined`

- [ ] **Step 3: Create `internal/gameserver/feat_levelup_grant.go`**

```go
package gameserver

import (
	"context"
	"sort"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// ApplyFeatGrant grants all feats described by grants to characterID. Fixed feats
// are granted immediately; choice feats are auto-assigned by picking the first N
// pool entries not already owned (pool order determines priority).
//
// existing is a live map[featID]bool that is updated in place for dedup.
// featReg may be nil — feat existence validation is skipped when nil.
//
// Precondition: characterID > 0; grants non-nil; featsRepo non-nil.
// Postcondition: Returns the feat IDs that were newly added. existing is updated.
func ApplyFeatGrant(
	ctx context.Context,
	characterID int64,
	existing map[string]bool,
	grants *ruleset.FeatGrants,
	featReg *ruleset.FeatRegistry,
	featsRepo CharacterFeatsRepo,
) ([]string, error) {
	var granted []string

	// Fixed feats: grant any not already owned.
	for _, id := range grants.Fixed {
		if existing[id] {
			continue
		}
		if featReg != nil {
			if _, ok := featReg.Feat(id); !ok {
				continue // skip unknown feats
			}
		}
		if err := featsRepo.Add(ctx, characterID, id); err != nil {
			return granted, err
		}
		existing[id] = true
		granted = append(granted, id)
	}

	// Choice feats: count how many pool entries the character already owns.
	// If they already have count or more from the pool, nothing to do.
	if grants.Choices != nil && grants.Choices.Count > 0 {
		alreadyOwned := 0
		for _, id := range grants.Choices.Pool {
			if existing[id] {
				alreadyOwned++
			}
		}
		remaining := grants.Choices.Count - alreadyOwned
		for _, id := range grants.Choices.Pool {
			if remaining <= 0 {
				break
			}
			if existing[id] {
				continue
			}
			if featReg != nil {
				if _, ok := featReg.Feat(id); !ok {
					continue
				}
			}
			if err := featsRepo.Add(ctx, characterID, id); err != nil {
				return granted, err
			}
			existing[id] = true
			granted = append(granted, id)
			remaining--
		}
	}

	return granted, nil
}

// BackfillLevelUpFeats retroactively applies all feat level-up grants the player
// should have earned for levels 2..sess.Level but does not yet have. Auto-assigns
// by first-available pool order. Safe to call on every login (idempotent).
//
// Precondition: characterID > 0; featsRepo non-nil.
// Postcondition: character_feats table contains all expected level-up feats.
func BackfillLevelUpFeats(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	mergedFeatGrants map[int]*ruleset.FeatGrants,
	featReg *ruleset.FeatRegistry,
	featsRepo CharacterFeatsRepo,
) error {
	if characterID == 0 || sess.Level < 2 || len(mergedFeatGrants) == 0 {
		return nil
	}

	existingIDs, err := featsRepo.GetAll(ctx, characterID)
	if err != nil {
		return err
	}
	existing := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = true
	}

	// Process levels in ascending order for deterministic pool deduplication.
	levels := make([]int, 0, len(mergedFeatGrants))
	for lvl := range mergedFeatGrants {
		if lvl >= 2 && lvl <= sess.Level {
			levels = append(levels, lvl)
		}
	}
	sort.Ints(levels)

	for _, lvl := range levels {
		grants := mergedFeatGrants[lvl]
		if grants == nil {
			continue
		}
		if _, err := ApplyFeatGrant(ctx, characterID, existing, grants, featReg, featsRepo); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/gameserver/... -run "TestApplyFeatGrant|TestBackfillLevelUpFeats|TestProperty_BackfillLevelUpFeats" -v
```

Expected: all PASS (including property test with 100 rapid cases)

- [ ] **Step 5: Run full gameserver tests**

```bash
go test ./internal/gameserver/... -timeout 120s
```

Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/feat_levelup_grant.go internal/gameserver/feat_levelup_grant_test.go
git commit -m "feat(gameserver): add ApplyFeatGrant and BackfillLevelUpFeats"
```

---

## Task 5: Wire feat grants into `handleGrant` (live level-up)

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleGrant function ~line 10012)

- [ ] **Step 1: Locate the end of the tech grants block in `handleGrant`**

Open `internal/gameserver/grpc_service.go`. The tech grants block ends at the closing `}` after `SetPendingTechLevels` (around line 10220). The exact marker to find is:

```go
					}
				}
			}
		}
	}
```

Immediately after the tech grants block (still inside the `if leveled {` block), add the feat grants block.

- [ ] **Step 2: Add feat grants block after tech grants in `handleGrant`**

Find this text (end of the tech grants block):

```go
					if len(target.PendingTechGrants) > 0 && s.progressRepo != nil {
						levels := make([]int, 0, len(target.PendingTechGrants))
						for lvl := range target.PendingTechGrants {
							levels = append(levels, lvl)
						}
						sort.Ints(levels)
						if err := s.progressRepo.SetPendingTechLevels(ctx, target.CharacterID, levels); err != nil {
							s.logger.Warn("handleGrant: SetPendingTechLevels failed", zap.Error(err))
						}
						selectNotif := messageEvent("You have pending technology selections! Type 'selecttech' to choose your technologies.")
						if data, mErr := proto.Marshal(selectNotif); mErr == nil {
							_ = target.Entity.Push(data)
						}
					}
				}
			}
		}
```

After the closing `}` of the tech grants block (i.e., after `}` that closes `if s.hardwiredTechRepo != nil ...`), add:

```go
			// Apply feat level-up grants for each level gained.
			// Fixed feats are granted immediately; choice feats are auto-picked
			// from the pool (first available not already owned).
			if s.characterFeatsRepo != nil && s.jobRegistry != nil && s.featRegistry != nil && target.CharacterID > 0 {
				if job, ok := s.jobRegistry.Job(target.Class); ok {
					var archetypeFeatGrants map[int]*ruleset.FeatGrants
					if job.Archetype != "" {
						if arch, archOK := s.archetypes[job.Archetype]; archOK {
							archetypeFeatGrants = arch.LevelUpFeatGrants
						}
					}
					mergedFeatGrants := ruleset.MergeFeatLevelUpGrants(archetypeFeatGrants, job.LevelUpFeatGrants)
					if len(mergedFeatGrants) > 0 {
						existingFeatIDs, featGetErr := s.characterFeatsRepo.GetAll(ctx, target.CharacterID)
						if featGetErr != nil {
							s.logger.Warn("handleGrant: GetAll feats failed", zap.Error(featGetErr))
						} else {
							existing := make(map[string]bool, len(existingFeatIDs))
							for _, id := range existingFeatIDs {
								existing[id] = true
							}
							for lvl := oldLevel + 1; lvl <= result.NewLevel; lvl++ {
								fg, hasFG := mergedFeatGrants[lvl]
								if !hasFG || fg == nil {
									continue
								}
								grantedIDs, applyErr := ApplyFeatGrant(ctx, target.CharacterID, existing, fg, s.featRegistry, s.characterFeatsRepo)
								if applyErr != nil {
									s.logger.Warn("handleGrant: ApplyFeatGrant failed",
										zap.Int64("character_id", target.CharacterID),
										zap.Int("level", lvl),
										zap.Error(applyErr),
									)
								}
								for _, id := range grantedIDs {
									f, _ := s.featRegistry.Feat(id)
									name := id
									if f != nil && f.Name != "" {
										name = f.Name
									}
									notifMsg := messageEvent(fmt.Sprintf("You gained the feat: %s!", name))
									if data, mErr := proto.Marshal(notifMsg); mErr == nil {
										_ = target.Entity.Push(data)
									}
								}
							}
						}
					}
				}
			}
```

- [ ] **Step 3: Build to confirm no compile errors**

```bash
go build ./internal/gameserver/...
```

Expected: no errors

- [ ] **Step 4: Run full gameserver tests**

```bash
go test ./internal/gameserver/... -timeout 120s
```

Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): apply feat level-up grants in handleGrant on level-up"
```

---

## Task 6: Wire feat backfill into `Session()` startup

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (Session function, around line 1741)

- [ ] **Step 1: Find the feat loading block in `Session()`**

Search for this block (around line 1741):

```go
	// REQ-RXN15: register reactions from feats; also populate ActiveFeatUses for limited active feats.
	if s.characterFeatsRepo != nil && s.featRegistry != nil && characterID > 0 {
```

The backfill must run BEFORE this block so that newly-backfilled feats are picked up by the normal feat-loading code.

- [ ] **Step 2: Add `BackfillLevelUpFeats` call before the feat loading block**

Insert immediately before the `// REQ-RXN15` comment:

```go
	// Retroactively backfill any feat level-up grants never applied.
	// Runs before feat loading so newly-granted feats are included in session state.
	// BackfillLevelUpFeats is idempotent: only missing grants are applied.
	if s.characterFeatsRepo != nil && s.jobRegistry != nil && characterID > 0 && sess.Level >= 2 {
		if job, ok := s.jobRegistry.Job(sess.Class); ok {
			var archFeatGrants map[int]*ruleset.FeatGrants
			if job.Archetype != "" {
				if arch, archOK := s.archetypes[job.Archetype]; archOK {
					archFeatGrants = arch.LevelUpFeatGrants
				}
			}
			mergedFeatGrants := ruleset.MergeFeatLevelUpGrants(archFeatGrants, job.LevelUpFeatGrants)
			if err := BackfillLevelUpFeats(stream.Context(), sess, characterID,
				mergedFeatGrants, s.featRegistry, s.characterFeatsRepo,
			); err != nil {
				s.logger.Warn("BackfillLevelUpFeats failed",
					zap.Int64("character_id", characterID),
					zap.Error(err),
				)
			}
		}
	}

```

- [ ] **Step 3: Build to confirm no compile errors**

```bash
go build ./internal/gameserver/...
```

Expected: no errors

- [ ] **Step 4: Run full gameserver tests**

```bash
go test ./internal/gameserver/... -timeout 120s
```

Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): backfill missing feat level-up grants at session start"
```

---

## Task 7: Add content — `level_up_feat_grants` for `aggressor` and `criminal` archetypes

**Files:**
- Modify: `content/archetypes/aggressor.yaml`
- Modify: `content/archetypes/criminal.yaml`

**Aggressor feat pool** (all aggressor feats):
`reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt`

**Criminal feat pool** (all criminal feats):
`quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade`

Both archetypes follow PF2E fighter/rogue progression: 1 class feat choice at levels 2, 4, 6, 8, 10, 12, 14, 16, 18, 20.

- [ ] **Step 1: Add `level_up_feat_grants` to `content/archetypes/aggressor.yaml`**

Append to end of `content/archetypes/aggressor.yaml`:

```yaml
level_up_feat_grants:
  2:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  4:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  6:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  8:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  10:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  12:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  14:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  16:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  18:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
  20:
    choices:
      pool: [reactive_block, overpower, snap_shot, adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt]
      count: 1
```

- [ ] **Step 2: Add `level_up_feat_grants` to `content/archetypes/criminal.yaml`**

Append to end of `content/archetypes/criminal.yaml`:

```yaml
level_up_feat_grants:
  2:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  4:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  6:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  8:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  10:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  12:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  14:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  16:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  18:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
  20:
    choices:
      pool: [quick_dodge, trap_eye, twin_strike, youre_next, overextend, tumble_behind, plant_evidence, dueling_guard, flying_blade]
      count: 1
```

- [ ] **Step 3: Verify YAML parses correctly**

```bash
go test ./internal/game/ruleset/... -run "TestLoadArchetypes" -v 2>/dev/null || go test ./internal/game/ruleset/... -v -timeout 30s 2>&1 | tail -20
```

Expected: all pass (no YAML parse errors)

- [ ] **Step 4: Run full test suite**

```bash
go test ./... -timeout 120s 2>&1 | grep -E "FAIL|ok" | tail -20
```

Expected: all ok, no FAIL

- [ ] **Step 5: Commit and deploy**

```bash
git add content/archetypes/aggressor.yaml content/archetypes/criminal.yaml
git commit -m "feat(content): add level_up_feat_grants to aggressor and criminal archetypes"
git push
make k8s-redeploy
```

---

## Self-Review

**1. Spec coverage:**
- ✅ Feat grants on level-up: Task 5 (handleGrant)
- ✅ Retroactive backfill at session start: Task 6
- ✅ Content for aggressor: Task 7
- ✅ Content for criminal: Task 7
- ✅ Data model (LevelUpFeatGrants on Archetype + Job): Task 1
- ✅ Merge helper: Task 2
- ✅ CharacterFeatsRepo.Add (non-destructive): Task 3
- ✅ Core grant logic: Task 4

**2. Placeholder scan:** None found.

**3. Type consistency:**
- `CharacterFeatsRepo` used consistently across Tasks 3, 4, 5, 6
- `ruleset.MergeFeatLevelUpGrants` defined in Task 2, used in Tasks 5 and 6
- `ApplyFeatGrant` signature: `(ctx, characterID int64, existing map[string]bool, grants *ruleset.FeatGrants, featReg *ruleset.FeatRegistry, featsRepo CharacterFeatsRepo) ([]string, error)` — consistent across Tasks 4, 5, 6
- `BackfillLevelUpFeats` signature: `(ctx, sess *session.PlayerSession, characterID int64, mergedFeatGrants map[int]*ruleset.FeatGrants, featReg *ruleset.FeatRegistry, featsRepo CharacterFeatsRepo) error` — consistent
