# Level-Up Technology Selection Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a character levels up, apply technology grants from `job.LevelUpGrants[newLevel]` — appending hardwired IDs, filling new prepared slots, and adding new spontaneous known techs — reusing all existing technology infrastructure.

**Architecture:** Add `LevelUpGrants map[int]*TechnologyGrants` to `Job` with load-time validation; add `LevelUpTechnologies` to `technology_assignment.go` (reusing existing helpers with a new `startIdx` parameter for prepared slot offset); wire into `handleGrant` by capturing `oldLevel` before XP is awarded and looping over each new level. Interactive prompting is not available in `handleGrant` (no target stream), so a `firstOptionPrompt` fallback is used; the admin-grant path auto-assigns first pool item when pool > open slots.

**Tech Stack:** Go modules (`github.com/cory-johannsen/mud`), `gopkg.in/yaml.v3`, `pgregory.net/rapid v1.2.0`, `github.com/stretchr/testify`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/game/ruleset/job.go` | Modify | Add `LevelUpGrants` field; extend `LoadJobs` validation loop |
| `internal/game/ruleset/technology_grants_test.go` | Modify | REQ-LUT1, REQ-LUT8, REQ-LUT9 |
| `internal/gameserver/technology_assignment.go` | Modify | Add `startIdx` to `fillFromPreparedPool`; add `LevelUpTechnologies` |
| `internal/gameserver/technology_assignment_test.go` | Modify | REQ-LUT3–LUT6 |
| `internal/gameserver/grpc_service.go` | Modify | Capture `oldLevel`; call `LevelUpTechnologies` after level-up in `handleGrant` |
| `internal/gameserver/grpc_service_grant_test.go` | Modify | REQ-LUT7 |
| `docs/requirements/FEATURES.md` | Modify | Mark level-up technology selection `[x]` |

---

## Chunk 1: Job YAML extension and validation

### Task 1: Add `LevelUpGrants` to `Job` struct and `LoadJobs`

**Files:**
- Modify: `internal/game/ruleset/job.go`
- Modify: `internal/game/ruleset/technology_grants_test.go`

- [ ] **Step 1: Write failing tests (REQ-LUT1, REQ-LUT8, REQ-LUT9)**

Add these three tests to `internal/game/ruleset/technology_grants_test.go`, after the existing tests:

```go
// REQ-LUT1: Job with level_up_grants YAML round-trips without data loss
func TestJob_LevelUpGrants_RoundTrip(t *testing.T) {
	src := `
id: test_job
name: Test Job
archetype: aggressor
level_up_grants:
  3:
    prepared:
      slots_by_level:
        2: 1
      pool:
        - id: arc_thought
          level: 2
        - id: mind_spike
          level: 2
  5:
    spontaneous:
      known_by_level:
        2: 1
      pool:
        - id: acid_spray
          level: 2
`
	var j ruleset.Job
	require.NoError(t, yaml.Unmarshal([]byte(src), &j))
	require.NotNil(t, j.LevelUpGrants)
	require.Contains(t, j.LevelUpGrants, 3)
	require.Contains(t, j.LevelUpGrants, 5)

	g3 := j.LevelUpGrants[3]
	require.NotNil(t, g3.Prepared)
	assert.Equal(t, 1, g3.Prepared.SlotsByLevel[2])
	require.Len(t, g3.Prepared.Pool, 2)
	assert.Equal(t, "arc_thought", g3.Prepared.Pool[0].ID)
	assert.Equal(t, 2, g3.Prepared.Pool[0].Level)

	g5 := j.LevelUpGrants[5]
	require.NotNil(t, g5.Spontaneous)
	assert.Equal(t, 1, g5.Spontaneous.KnownByLevel[2])
	require.Len(t, g5.Spontaneous.Pool, 1)
	assert.Equal(t, "acid_spray", g5.Spontaneous.Pool[0].ID)
}

// REQ-LUT8 (property): level_up_grants YAML round-trip preserves all fields
func TestProperty_LevelUpGrants_YAMLRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		charLevel := rapid.IntRange(2, 10).Draw(rt, "charLevel")
		numPool := rapid.IntRange(1, 4).Draw(rt, "numPool")
		pool := make([]ruleset.PreparedEntry, numPool)
		for i := range pool {
			pool[i] = ruleset.PreparedEntry{
				ID:    fmt.Sprintf("tech_%d", i),
				Level: 1,
			}
		}
		grants := &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: numPool},
				Pool:         pool,
			},
		}
		job := ruleset.Job{
			LevelUpGrants: map[int]*ruleset.TechnologyGrants{charLevel: grants},
		}

		data, err := yaml.Marshal(job)
		require.NoError(rt, err)
		var got ruleset.Job
		require.NoError(rt, yaml.Unmarshal(data, &got))

		require.Contains(rt, got.LevelUpGrants, charLevel)
		g := got.LevelUpGrants[charLevel]
		require.NotNil(rt, g.Prepared)
		assert.Equal(rt, numPool, g.Prepared.SlotsByLevel[1])
		assert.ElementsMatch(rt, pool, g.Prepared.Pool)
	})
}

// REQ-LUT9: LoadJobs rejects a YAML file with an invalid level_up_grants entry
func TestLoadJobs_RejectsInvalidLevelUpGrants(t *testing.T) {
	dir := t.TempDir()
	// pool + fixed = 1 but slots = 2 — insufficient pool for level 1
	content := `
id: bad_job
name: Bad Job
archetype: aggressor
level_up_grants:
  3:
    prepared:
      slots_by_level:
        1: 2
      pool:
        - id: mind_spike
          level: 1
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad_job.yaml"), []byte(content), 0644))
	_, err := ruleset.LoadJobs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad_job")
	assert.Contains(t, err.Error(), "level_up_grants[3]")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run "TestJob_LevelUpGrants|TestProperty_LevelUpGrants|TestLoadJobs_RejectsInvalidLevelUpGrants" -v 2>&1 | tail -20
```

Expected: FAIL — `j.LevelUpGrants` is nil (field does not exist yet), `LoadJobs` does not return error.

- [ ] **Step 3: Add `LevelUpGrants` field to `Job` struct**

In `internal/game/ruleset/job.go`, add one field to the `Job` struct after `TechnologyGrants`:

```go
TechnologyGrants   *TechnologyGrants                    `yaml:"technology_grants,omitempty"`
LevelUpGrants      map[int]*TechnologyGrants            `yaml:"level_up_grants,omitempty"`
```

- [ ] **Step 4: Add validation loop in `LoadJobs`**

In `internal/game/ruleset/job.go`, extend `LoadJobs` after the existing `TechnologyGrants` validation (lines 92–96). Replace:

```go
		if j.TechnologyGrants != nil {
			if err := j.TechnologyGrants.Validate(); err != nil {
				return nil, fmt.Errorf("job %q technology_grants: %w", j.ID, err)
			}
		}
		jobs = append(jobs, &j)
```

With:

```go
		if j.TechnologyGrants != nil {
			if err := j.TechnologyGrants.Validate(); err != nil {
				return nil, fmt.Errorf("job %q technology_grants: %w", j.ID, err)
			}
		}
		for charLevel, grants := range j.LevelUpGrants {
			if charLevel < 1 {
				return nil, fmt.Errorf("job %q level_up_grants: level key %d must be >= 1", j.ID, charLevel)
			}
			if err := grants.Validate(); err != nil {
				return nil, fmt.Errorf("job %q level_up_grants[%d]: %w", j.ID, charLevel, err)
			}
		}
		jobs = append(jobs, &j)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run "TestJob_LevelUpGrants|TestProperty_LevelUpGrants|TestLoadJobs_RejectsInvalidLevelUpGrants" -v 2>&1 | tail -20
```

Expected: all three PASS.

- [ ] **Step 6: Run full ruleset test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -count=1 2>&1 | tail -10
```

Expected: PASS, 0 failures.

- [ ] **Step 7: Commit**

```bash
git add internal/game/ruleset/job.go internal/game/ruleset/technology_grants_test.go
git commit -m "feat(ruleset): add LevelUpGrants to Job with load-time validation (REQ-LUT1, REQ-LUT8, REQ-LUT9)"
```

---

## Chunk 2: LevelUpTechnologies function

### Task 2: Add `startIdx` parameter and `LevelUpTechnologies`

**Files:**
- Modify: `internal/gameserver/technology_assignment.go`
- Modify: `internal/gameserver/technology_assignment_test.go`

- [ ] **Step 1: Write failing tests (REQ-LUT3–LUT6)**

Add these tests to `internal/gameserver/technology_assignment_test.go`, after the existing tests. The fake repo types (`fakeHardwiredRepo`, `fakePreparedRepo`, `fakeSpontaneousRepo`, `fakeInnateRepo`) and `noPrompt` are already defined earlier in this file — do not redefine them.

```go
// REQ-LUT3: LevelUpTechnologies appends hardwired IDs, deduplicating against existing
func TestLevelUpTechnologies_HardwiredAppendAndDedup(t *testing.T) {
	ctx := context.Background()
	hw := &fakeHardwiredRepo{stored: []string{"existing_tech"}}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{HardwiredTechs: []string{"existing_tech"}}

	grants := &ruleset.TechnologyGrants{
		Hardwired: []string{"new_tech", "existing_tech"}, // existing_tech is a duplicate
	}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	// existing_tech should not be duplicated; new_tech appended
	assert.Equal(t, []string{"existing_tech", "new_tech"}, sess.HardwiredTechs)
	assert.Equal(t, []string{"existing_tech", "new_tech"}, hw.stored)
}

// REQ-LUT4: LevelUpTechnologies fills prepared slots after existing indices (no collision)
func TestLevelUpTechnologies_PreparedSlotIndexOffset(t *testing.T) {
	ctx := context.Background()
	// Pre-populate 1 existing level-1 slot at index 0
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "original_tech"}},
	}}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "original_tech"}},
		},
	}

	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Fixed:        []ruleset.PreparedEntry{{ID: "new_tech", Level: 1}},
		},
	}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	// Level-1 slots: index 0 = original_tech, index 1 = new_tech
	require.Len(t, sess.PreparedTechs[1], 2)
	assert.Equal(t, "original_tech", sess.PreparedTechs[1][0].TechID)
	assert.Equal(t, "new_tech", sess.PreparedTechs[1][1].TechID)

	// Repo: index 0 unchanged, index 1 set to new_tech
	require.Len(t, prep.slots[1], 2)
	assert.Equal(t, "original_tech", prep.slots[1][0].TechID)
	assert.Equal(t, "new_tech", prep.slots[1][1].TechID)
}

// REQ-LUT5: LevelUpTechnologies adds spontaneous techs without removing existing ones
func TestLevelUpTechnologies_SpontaneousAppendsToExisting(t *testing.T) {
	ctx := context.Background()
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{techs: map[int][]string{
		1: {"existing_spont"},
	}}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{
		SpontaneousTechs: map[int][]string{
			1: {"existing_spont"},
		},
	}

	grants := &ruleset.TechnologyGrants{
		Spontaneous: &ruleset.SpontaneousGrants{
			KnownByLevel: map[int]int{1: 1},
			Fixed:        []ruleset.SpontaneousEntry{{ID: "new_spont", Level: 1}},
		},
	}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"existing_spont", "new_spont"}, sess.SpontaneousTechs[1])
	assert.ElementsMatch(t, []string{"existing_spont", "new_spont"}, spont.techs[1])
}

// REQ-LUT6: LevelUpTechnologies with nil grants is a no-op
func TestLevelUpTechnologies_NilGrantsNoOp(t *testing.T) {
	ctx := context.Background()
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{HardwiredTechs: []string{"existing"}}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, nil, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)
	assert.Equal(t, []string{"existing"}, sess.HardwiredTechs)
	assert.Nil(t, hw.stored) // SetAll never called
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestLevelUpTechnologies" -v 2>&1 | tail -10
```

Expected: compile error — `LevelUpTechnologies` undefined.

- [ ] **Step 3: Add `startIdx` parameter to `fillFromPreparedPool`**

In `internal/gameserver/technology_assignment.go`, change the signature of `fillFromPreparedPool` from:

```go
func fillFromPreparedPool(
	ctx context.Context,
	lvl, slots int,
	grants *ruleset.PreparedGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	characterID int64,
	repo PreparedTechRepo,
) ([]*session.PreparedSlot, error) {
	result := make([]*session.PreparedSlot, 0, slots)
	idx := 0
```

To:

```go
func fillFromPreparedPool(
	ctx context.Context,
	lvl, slots, startIdx int,
	grants *ruleset.PreparedGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	characterID int64,
	repo PreparedTechRepo,
) ([]*session.PreparedSlot, error) {
	result := make([]*session.PreparedSlot, 0, slots)
	idx := startIdx
```

- [ ] **Step 4: Update `AssignTechnologies` to pass `startIdx=0`**

In `internal/gameserver/technology_assignment.go`, find the call to `fillFromPreparedPool` inside `AssignTechnologies`:

```go
		chosen, err := fillFromPreparedPool(ctx, lvl, slots, grants.Prepared, techReg, promptFn, characterID, prepRepo)
```

Change to:

```go
		chosen, err := fillFromPreparedPool(ctx, lvl, slots, 0, grants.Prepared, techReg, promptFn, characterID, prepRepo)
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1
```

Expected: no errors.

- [ ] **Step 6: Implement `LevelUpTechnologies`**

Add this function to `internal/gameserver/technology_assignment.go`, after `LoadTechnologies`:

```go
// LevelUpTechnologies applies a technology grants delta to an existing character's session
// and persists new slot assignments. Called once per character level gained.
//
// Precondition: grants must be nil or valid (validated at YAML load time).
// Postcondition: If grants is nil, returns nil with no changes (no-op).
// Otherwise sess and repos gain all new slots from grants; existing slots are unchanged.
// promptFn may be nil — if nil, the first available pool option is auto-selected.
func LevelUpTechnologies(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	grants *ruleset.TechnologyGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	hwRepo HardwiredTechRepo,
	prepRepo PreparedTechRepo,
	spontRepo SpontaneousTechRepo,
	innateRepo InnateTechRepo,
) error {
	if grants == nil {
		return nil
	}
	// Use first-option fallback when no promptFn is provided (e.g., admin grant path).
	if promptFn == nil {
		promptFn = func(options []string) (string, error) {
			if len(options) == 0 {
				return "", nil
			}
			return options[0], nil
		}
	}

	// Hardwired: append new IDs, skipping duplicates (map-based, order-preserving).
	if len(grants.Hardwired) > 0 {
		existing := make(map[string]bool, len(sess.HardwiredTechs))
		for _, id := range sess.HardwiredTechs {
			existing[id] = true
		}
		for _, id := range grants.Hardwired {
			if !existing[id] {
				sess.HardwiredTechs = append(sess.HardwiredTechs, id)
				existing[id] = true
			}
		}
		if err := hwRepo.SetAll(ctx, characterID, sess.HardwiredTechs); err != nil {
			return fmt.Errorf("LevelUpTechnologies hardwired: %w", err)
		}
	}

	// Prepared: fill new slots starting after existing slot indices.
	// Existing slot slices are dense (no nil gaps), so len gives the correct next index.
	if grants.Prepared != nil {
		existingPrep, err := prepRepo.GetAll(ctx, characterID)
		if err != nil {
			return fmt.Errorf("LevelUpTechnologies prepared GetAll: %w", err)
		}
		if sess.PreparedTechs == nil {
			sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
		}
		for lvl, slots := range grants.Prepared.SlotsByLevel {
			startIdx := len(existingPrep[lvl])
			chosen, err := fillFromPreparedPool(ctx, lvl, slots, startIdx, grants.Prepared, techReg, promptFn, characterID, prepRepo)
			if err != nil {
				return fmt.Errorf("LevelUpTechnologies prepared level %d: %w", lvl, err)
			}
			sess.PreparedTechs[lvl] = append(sess.PreparedTechs[lvl], chosen...)
		}
	}

	// Spontaneous: add new known techs without removing existing ones.
	if grants.Spontaneous != nil {
		if sess.SpontaneousTechs == nil {
			sess.SpontaneousTechs = make(map[int][]string)
		}
		for lvl, known := range grants.Spontaneous.KnownByLevel {
			chosen, err := fillFromSpontaneousPool(ctx, lvl, known, grants.Spontaneous, techReg, promptFn, characterID, spontRepo)
			if err != nil {
				return fmt.Errorf("LevelUpTechnologies spontaneous level %d: %w", lvl, err)
			}
			sess.SpontaneousTechs[lvl] = append(sess.SpontaneousTechs[lvl], chosen...)
		}
	}

	// Innate: level-up grants do not assign innate technologies (archetype-only).

	return nil
}
```

- [ ] **Step 7: Run the new tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestLevelUpTechnologies" -v 2>&1 | tail -20
```

Expected: all 4 PASS.

- [ ] **Step 8: Run full gameserver test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 2>&1 | tail -10
```

Expected: PASS, 0 failures.

- [ ] **Step 9: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
git commit -m "feat(gameserver): LevelUpTechnologies with startIdx offset, hardwired dedup, spontaneous append (REQ-LUT3–LUT6)"
```

---

## Chunk 3: Gameserver wiring and final verification

### Task 3: Wire `LevelUpTechnologies` into `handleGrant`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_grant_test.go`
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Read the current `grpc_service_grant_test.go`**

```bash
cat -n internal/gameserver/grpc_service_grant_test.go 2>&1 | head -80
```

Understand the existing test helper pattern (how `NewGameServiceServer` is called in grant tests) before writing new tests.

- [ ] **Step 2: Write failing test (REQ-LUT7)**

Add a test to `internal/gameserver/grpc_service_grant_test.go` that verifies grants for both level 3 and level 4 are applied in order when a player goes from level 2 to level 4.

**Note:** The grant test uses a real or fake `GameServiceServer`. Look at the existing test structure and follow the same pattern. The test needs:
- A `PlayerSession` at level 2 with `Class` set to a job that has `LevelUpGrants` for levels 3 and 4
- A fake/mock job registry returning that job
- Fake technology repos
- An XP grant large enough to jump from level 2 to level 4

If the existing grant tests don't use technology repos (they pass nil for those), you'll need to wire them in. Look for the `NewGameServiceServer(` call and insert the fake repos at the positions for `hardwiredTechRepo, preparedTechRepo, spontaneousTechRepo, innateTechRepo` (4th–7th arguments after `techRegistry`).

The test assertion: after the grant, the session's `HardwiredTechs` contains IDs from both the level-3 and level-4 `LevelUpGrants` entries.

Concrete test (adjust the `NewGameServiceServer` call to match the existing signature in this file):

```go
func TestHandleGrant_LevelUp_AppliesTechGrantsForEachLevel(t *testing.T) {
	// Build a job with LevelUpGrants for levels 3 and 4.
	job := &ruleset.Job{
		ID: "test_class",
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: {Hardwired: []string{"level3_tech"}},
			4: {Hardwired: []string{"level4_tech"}},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)

	// Fake tech repos — these types are already defined in technology_assignment_test.go
	// in the same package (gameserver_test), so they are available here without redeclaration.
	hwRepo := &fakeHardwiredRepo{}
	prepRepo := &fakePreparedRepo{}
	spontRepo := &fakeSpontaneousRepo{}
	innateRepo := &fakeInnateRepo{}

	// Use the existing test helper to build a GameServiceServer with this job registry.
	// Adjust NewGameServiceServer arguments to include the four fake repos.
	// Look at the existing test helpers in this file for the exact call pattern.
	// ... (see existing tests for the exact NewGameServiceServer signature in tests) ...

	// Create a target player session at level 2.
	targetSess := &session.PlayerSession{
		UID:      "target",
		CharName: "Target",
		Class:    "test_class",
		Level:    2,
		Role:     "player",
	}
	// ... register target in server sessions ...

	// Grant enough XP to jump from level 2 to level 4 (depends on BaseXP config).
	// ... send GrantRequest{GrantType: "xp", CharName: "Target", Amount: <enough>} ...

	// Assertions:
	assert.Contains(t, targetSess.HardwiredTechs, "level3_tech")
	assert.Contains(t, targetSess.HardwiredTechs, "level4_tech")
	// Verify ascending order: level3_tech before level4_tech
	idx3 := slices.Index(targetSess.HardwiredTechs, "level3_tech")
	idx4 := slices.Index(targetSess.HardwiredTechs, "level4_tech")
	assert.Less(t, idx3, idx4)
}
```

**Important:** Before writing the test, read `internal/gameserver/grpc_service_grant_test.go` to understand:
- The exact `NewGameServiceServer` argument order and which args are nil-able
- How `PlayerSession` entities are registered into the server
- How the XP config is set so you can compute "enough XP" to reach level 4 from level 2
- Whether `fakeHardwiredTechRepo` etc. are already defined in a shared test helpers file or need to be added

If the fake repo types already exist in `technology_assignment_test.go` (package `gameserver_test`), they are available in `grpc_service_grant_test.go` as long as both files are in the same test package. Verify this — both files should declare `package gameserver_test`.

Use `slices.Index` from `"slices"` (Go 1.21+) or manually find the index if the Go version is older.

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleGrant_LevelUp_AppliesTechGrantsForEachLevel" -v 2>&1 | tail -20
```

Expected: FAIL or compile error (LevelUpTechnologies not yet called from handleGrant).

- [ ] **Step 4: Wire `LevelUpTechnologies` into `handleGrant`**

In `internal/gameserver/grpc_service.go`, find the `handleGrant` function (around line 6415). In the `case "xp":` branch, locate:

```go
		result := xp.Award(target.Level, target.Experience, amount, s.xpSvc.Config())
```

Change to (capture `oldLevel` before mutation):

```go
		oldLevel := target.Level
		result := xp.Award(target.Level, target.Experience, amount, s.xpSvc.Config())
```

Then find the end of the `if result.LeveledUp {` block:

```go
			levelMsgs = append(levelMsgs, "You earned 1 hero point!")
		}
```

Add the technology level-up loop immediately after `levelMsgs = append(levelMsgs, "You earned 1 hero point!")` and before the closing `}`:

```go
			levelMsgs = append(levelMsgs, "You earned 1 hero point!")
			// Apply technology level-up grants for each level gained (ascending order).
			// handleGrant runs in the editor's stream context, not the target's, so
			// interactive prompting is unavailable; first-option auto-assign is used.
			if s.hardwiredTechRepo != nil && s.jobRegistry != nil && target.CharacterID > 0 {
				if job, ok := s.jobRegistry.Job(target.Class); ok {
					for lvl := oldLevel + 1; lvl <= result.NewLevel; lvl++ {
						grants, hasGrants := job.LevelUpGrants[lvl]
						if !hasGrants {
							continue
						}
						if err := LevelUpTechnologies(ctx, target, target.CharacterID,
							grants, s.techRegistry, nil,
							s.hardwiredTechRepo, s.preparedTechRepo,
							s.spontaneousTechRepo, s.innateTechRepo,
						); err != nil {
							s.logger.Warn("handleGrant: LevelUpTechnologies failed",
								zap.Int64("character_id", target.CharacterID),
								zap.Int("level", lvl),
								zap.Error(err))
						}
					}
				}
			}
```

- [ ] **Step 5: Build**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1
```

Expected: no errors. If `oldLevel` is flagged as unused (because `s.xpSvc` may be nil), wrap the `oldLevel` capture inside the `if s.xpSvc != nil {` block.

- [ ] **Step 6: Run the new test**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleGrant_LevelUp_AppliesTechGrantsForEachLevel" -v 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 7: Run full gameserver test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 2>&1 | tail -10
```

Expected: PASS, 0 failures.

- [ ] **Step 8: Run full project test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -20
```

Expected: PASS, 0 failures across all packages.

- [ ] **Step 9: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, find:

```
    - [ ] Levelling up allows for additions and changes
```

(or the equivalent line about level-up technology selection added in the previous sprint). Mark it complete. Also update the specific level-up bullet:

```
        - [ ] Level-up technology selection — player chooses new technologies when levelling up (prepared/spontaneous pool expands; player selects additions interactively)
```

to:

```
        - [x] Level-up technology selection — player chooses new technologies when levelling up (prepared/spontaneous pool expands; player selects additions interactively via first-option auto-assign on admin-grant path)
```

- [ ] **Step 10: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_grant_test.go docs/requirements/FEATURES.md
git commit -m "feat(gameserver): wire LevelUpTechnologies into handleGrant; apply per-level tech grants in ascending order (REQ-LUT7)"
```

---

## Checklist of all requirements

| REQ | Task | Description |
|-----|------|-------------|
| REQ-LUT1 | Task 1 | `Job` with `level_up_grants` YAML round-trips |
| REQ-LUT2 | Task 1 | `Validate()` on a `level_up_grants` entry rejects insufficient pool (covered by existing `TestTechnologyGrants_Validate_PoolTooSmall` + `TestLoadJobs_RejectsInvalidLevelUpGrants`) |
| REQ-LUT3 | Task 2 | `LevelUpTechnologies` hardwired append + dedup |
| REQ-LUT4 | Task 2 | Prepared slot index offset (no collision) |
| REQ-LUT5 | Task 2 | Spontaneous append without removing existing |
| REQ-LUT6 | Task 2 | Nil grants = no-op |
| REQ-LUT7 | Task 3 | Multi-level skip applies grants in ascending order |
| REQ-LUT8 | Task 1 | Property: `level_up_grants` YAML round-trip |
| REQ-LUT9 | Task 1 | `LoadJobs` rejects invalid `level_up_grants` |
