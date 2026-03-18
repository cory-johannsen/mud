# Interactive Level-Up Technology Selection Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a player levels up with technology pool choices, apply hardwired/auto-assign grants immediately with notifications, defer pool selections to `PendingTechGrants`, and resolve them interactively at next login or via `selecttech` command.

**Architecture:** `partitionTechGrants` splits a `TechnologyGrants` into immediate (no choice needed) and deferred (pool > open slots) parts. `ResolvePendingTechGrants` iterates pending levels in ascending order and calls `LevelUpTechnologies` with a live promptFn, persisting to `character_pending_tech_levels` after each resolution. `handleGrant` uses partitioning to notify and defer; login and `selecttech` resolve deferred grants.

**Tech Stack:** Go modules (`github.com/cory-johannsen/mud`), `pgregory.net/rapid v1.2.0`, `github.com/stretchr/testify`, protobuf/gRPC, PostgreSQL (pgx)

---

## Chunk 1 — Task 1 + Task 2

### Task 1: DB migration + CharacterProgressRepository + interfaces + session field

**Files:**
- Create: `migrations/026_pending_tech_levels.up.sql`
- Create: `migrations/026_pending_tech_levels.down.sql`
- Modify: `internal/storage/postgres/character_progress.go`
- Modify: `internal/gameserver/grpc_service.go` (ProgressRepository interface only)
- Modify: `internal/gameserver/technology_assignment.go` (PendingTechLevelsRepo interface only)
- Modify: `internal/game/session/manager.go` (add PendingTechGrants field + ruleset import)

---

- [ ] **T1-S1: Create migration up file**

  Create `migrations/026_pending_tech_levels.up.sql`:

  ```sql
  CREATE TABLE character_pending_tech_levels (
      character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
      level        INT    NOT NULL,
      PRIMARY KEY (character_id, level)
  );
  ```

- [ ] **T1-S2: Create migration down file**

  Create `migrations/026_pending_tech_levels.down.sql`:

  ```sql
  DROP TABLE IF EXISTS character_pending_tech_levels;
  ```

- [ ] **T1-S3: Add repo methods to CharacterProgressRepository**

  Add to `internal/storage/postgres/character_progress.go`:

  ```go
  // GetPendingTechLevels returns the list of character levels with unresolved
  // technology pool selections.
  //
  // Precondition: id > 0.
  // Postcondition: Returns all pending tech levels (may be empty slice).
  func (r *CharacterProgressRepository) GetPendingTechLevels(ctx context.Context, id int64) ([]int, error) {
      if id <= 0 {
          return nil, fmt.Errorf("characterID must be > 0, got %d", id)
      }
      rows, err := r.pool.Query(ctx,
          `SELECT level FROM character_pending_tech_levels WHERE character_id = $1 ORDER BY level`, id,
      )
      if err != nil {
          return nil, fmt.Errorf("GetPendingTechLevels: %w", err)
      }
      defer rows.Close()
      var levels []int
      for rows.Next() {
          var lvl int
          if err := rows.Scan(&lvl); err != nil {
              return nil, fmt.Errorf("GetPendingTechLevels scan: %w", err)
          }
          levels = append(levels, lvl)
      }
      return levels, rows.Err()
  }

  // SetPendingTechLevels replaces the stored pending tech levels for a character.
  // Pass an empty slice to clear all pending levels.
  //
  // Precondition: id > 0.
  // Postcondition: character_pending_tech_levels contains exactly the given levels.
  func (r *CharacterProgressRepository) SetPendingTechLevels(ctx context.Context, id int64, levels []int) error {
      if id <= 0 {
          return fmt.Errorf("characterID must be > 0, got %d", id)
      }
      tx, err := r.pool.Begin(ctx)
      if err != nil {
          return fmt.Errorf("SetPendingTechLevels begin: %w", err)
      }
      defer tx.Rollback(ctx)
      if _, err := tx.Exec(ctx,
          `DELETE FROM character_pending_tech_levels WHERE character_id = $1`, id,
      ); err != nil {
          return fmt.Errorf("SetPendingTechLevels delete: %w", err)
      }
      for _, lvl := range levels {
          if _, err := tx.Exec(ctx,
              `INSERT INTO character_pending_tech_levels (character_id, level) VALUES ($1, $2)`,
              id, lvl,
          ); err != nil {
              return fmt.Errorf("SetPendingTechLevels insert level %d: %w", lvl, err)
          }
      }
      return tx.Commit(ctx)
  }
  ```

- [ ] **T1-S4: Extend ProgressRepository interface**

  In `internal/gameserver/grpc_service.go`, append to the `ProgressRepository` interface (lines 109-117):

  ```go
  GetPendingTechLevels(ctx context.Context, id int64) ([]int, error)
  SetPendingTechLevels(ctx context.Context, id int64, levels []int) error
  ```

- [ ] **T1-S5: Add PendingTechLevelsRepo interface to technology_assignment.go**

  Add to `internal/gameserver/technology_assignment.go` after the `InnateTechRepo` interface:

  ```go
  // PendingTechLevelsRepo persists the list of character levels with unresolved
  // technology pool selections.
  type PendingTechLevelsRepo interface {
      GetPendingTechLevels(ctx context.Context, characterID int64) ([]int, error)
      SetPendingTechLevels(ctx context.Context, characterID int64, levels []int) error
  }
  ```

- [ ] **T1-S6: Add PendingTechGrants field to PlayerSession**

  In `internal/game/session/manager.go`:

  - Add `"github.com/cory-johannsen/mud/internal/game/ruleset"` to imports (verify no import cycle: `ruleset` must not import `session`).
  - Add field after `InnateTechs`:

  ```go
  // PendingTechGrants maps character level to the technology grants that require
  // interactive player selection (pool > open slots). Populated at level-up;
  // cleared by ResolvePendingTechGrants.
  PendingTechGrants map[int]*ruleset.TechnologyGrants
  ```

- [ ] **T1-S7: Build to verify no errors**

  ```bash
  cd /home/cjohannsen/src/mud && go build ./... 2>&1 | head -20
  ```

  Expected: no errors. `CharacterProgressRepository` now satisfies the extended `ProgressRepository` interface.

- [ ] **T1-S8: Run storage tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/storage/... -count=1 2>&1 | tail -5
  ```

  Expected: PASS.

- [ ] **T1-S9: Commit**

  ```bash
  git add migrations/026_pending_tech_levels.up.sql migrations/026_pending_tech_levels.down.sql \
    internal/storage/postgres/character_progress.go \
    internal/gameserver/grpc_service.go \
    internal/gameserver/technology_assignment.go \
    internal/game/session/manager.go
  git commit -m "feat(storage,session): pending tech levels DB table, repo methods, PendingTechLevelsRepo interface, PendingTechGrants on session"
  ```

---

### Task 2: PartitionTechGrants + ResolvePendingTechGrants

**Files:**
- Modify: `internal/gameserver/technology_assignment.go`
- Modify: `internal/gameserver/technology_assignment_test.go`

---

- [ ] **T2-S0: Read existing test infrastructure**

  Read `internal/gameserver/technology_assignment_test.go` to confirm:
  - The exact fields of `fakePreparedRepo` (specifically how assigned slots are stored — e.g., `slots map[int][]*session.PreparedSlot`)
  - The definition of `noPrompt` (nil or a no-op function)
  - The signatures of `fakeHardwiredRepo`, `fakeSpontaneousRepo`, `fakeInnateRepo`

  This step produces no code output — it informs T2-S1 and T2-S3. Adjust the property test assertion in T2-S3 to match the actual `fakePreparedRepo` field shape.

- [ ] **T2-S1: Add fakePendingTechLevelsRepo test double**

  Add to `internal/gameserver/technology_assignment_test.go` (in `package gameserver_test`) before the new test functions:

  ```go
  type fakePendingTechLevelsRepo struct {
      levels       []int
      setWasCalled bool
  }

  func (r *fakePendingTechLevelsRepo) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
      return r.levels, nil
  }
  func (r *fakePendingTechLevelsRepo) SetPendingTechLevels(_ context.Context, _ int64, levels []int) error {
      r.levels = levels
      r.setWasCalled = true
      return nil
  }
  ```

- [ ] **T2-S2: Write failing tests for partition logic**

  Add to `internal/gameserver/technology_assignment_test.go` after the `RearrangePreparedTechs` tests:

  ```go
  // REQ-ILT2: All-immediate grants (pool <= open slots) → deferred is nil.
  func TestPartitionTechGrants_AllImmediate(t *testing.T) {
      grants := &ruleset.TechnologyGrants{
          Prepared: &ruleset.PreparedGrants{
              SlotsByLevel: map[int]int{1: 2},
              Fixed:        []ruleset.PreparedEntry{{ID: "fixed", Level: 1}},
              Pool:         []ruleset.PreparedEntry{{ID: "only_pool", Level: 1}},
          },
      }
      immediate, deferred := gameserver.PartitionTechGrants(grants)
      assert.NotNil(t, immediate)
      assert.Nil(t, deferred, "no player choice needed when pool <= open slots")
  }

  // REQ-ILT1 (partition): Pool > open slots → deferred is non-nil for that level.
  func TestPartitionTechGrants_DeferredWhenPoolExceedsSlots(t *testing.T) {
      grants := &ruleset.TechnologyGrants{
          Prepared: &ruleset.PreparedGrants{
              SlotsByLevel: map[int]int{1: 1},
              Pool: []ruleset.PreparedEntry{
                  {ID: "pool_a", Level: 1},
                  {ID: "pool_b", Level: 1},
              },
          },
      }
      immediate, deferred := gameserver.PartitionTechGrants(grants)
      assert.Nil(t, immediate, "no immediate grants when no fixed/auto-assign at this level")
      require.NotNil(t, deferred)
      require.NotNil(t, deferred.Prepared)
      assert.Equal(t, 1, deferred.Prepared.SlotsByLevel[1])
  }

  // REQ-ILT1: Hardwired entries always go to immediate.
  func TestPartitionTechGrants_HardwiredAlwaysImmediate(t *testing.T) {
      grants := &ruleset.TechnologyGrants{
          Hardwired: []string{"hw1", "hw2"},
          Prepared: &ruleset.PreparedGrants{
              SlotsByLevel: map[int]int{1: 1},
              Pool: []ruleset.PreparedEntry{
                  {ID: "p1", Level: 1},
                  {ID: "p2", Level: 1},
              },
          },
      }
      immediate, deferred := gameserver.PartitionTechGrants(grants)
      require.NotNil(t, immediate)
      assert.Equal(t, []string{"hw1", "hw2"}, immediate.Hardwired)
      require.NotNil(t, deferred)
  }
  ```

- [ ] **T2-S3: Write failing tests for ResolvePendingTechGrants**

  Add to `internal/gameserver/technology_assignment_test.go`:

  ```go
  // REQ-ILT5: ResolvePendingTechGrants prompts for each pending level in ascending order,
  // calls LevelUpTechnologies, and clears each entry.
  func TestResolvePendingTechGrants_ResolvesAndClears(t *testing.T) {
      ctx := context.Background()
      prep := &fakePreparedRepo{}
      hw := &fakeHardwiredRepo{}
      spont := &fakeSpontaneousRepo{}
      innate := &fakeInnateRepo{}
      progressRepo := &fakePendingTechLevelsRepo{}

      sess := &session.PlayerSession{
          Level: 3,
          PendingTechGrants: map[int]*ruleset.TechnologyGrants{
              2: {Prepared: &ruleset.PreparedGrants{
                  SlotsByLevel: map[int]int{1: 1},
                  Pool: []ruleset.PreparedEntry{{ID: "level2_tech", Level: 1}},
              }},
          },
      }
      job := &ruleset.Job{
          LevelUpGrants: map[int]*ruleset.TechnologyGrants{
              2: sess.PendingTechGrants[2],
          },
      }

      err := gameserver.ResolvePendingTechGrants(ctx, sess, 1, job, nil, noPrompt, hw, prep, spont, innate, progressRepo)
      require.NoError(t, err)
      assert.Empty(t, sess.PendingTechGrants, "pending grants must be cleared after resolution")
      assert.True(t, progressRepo.setWasCalled, "SetPendingTechLevels must be called after resolution")
  }

  // REQ-ILT7 (property): After ResolvePendingTechGrants, all chosen tech IDs are valid pool members.
  func TestPropertyResolvePendingTechGrants_ChosenFromPool(t *testing.T) {
      rapid.Check(t, func(rt *rapid.T) {
          nPool := rapid.IntRange(1, 4).Draw(rt, "nPool")
          pool := make([]ruleset.PreparedEntry, nPool)
          for i := range pool {
              pool[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("tech_%d", i), Level: 1}
          }
          // Open slots = 1 (pool > open → was deferred)
          grants := &ruleset.TechnologyGrants{
              Prepared: &ruleset.PreparedGrants{
                  SlotsByLevel: map[int]int{1: 1},
                  Pool:         pool,
              },
          }
          sess := &session.PlayerSession{
              Level: 5,
              PendingTechGrants: map[int]*ruleset.TechnologyGrants{3: grants},
          }
          prep := &fakePreparedRepo{}
          progressRepo := &fakePendingTechLevelsRepo{}

          err := gameserver.ResolvePendingTechGrants(context.Background(), sess, 1,
              &ruleset.Job{}, nil, noPrompt, &fakeHardwiredRepo{}, prep,
              &fakeSpontaneousRepo{}, &fakeInnateRepo{}, progressRepo)
          if err != nil {
              rt.Fatalf("ResolvePendingTechGrants: %v", err)
          }
          validIDs := make(map[string]bool)
          for _, e := range pool {
              validIDs[e.ID] = true
          }
          for _, slot := range prep.slots[1] {
              if !validIDs[slot.TechID] {
                  rt.Fatalf("chosen tech %q not in pool", slot.TechID)
              }
          }
      })
  }
  ```

- [ ] **T2-S4: Run tests to confirm compile failure**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestPartition|TestResolvePending|TestPropertyResolve" -v 2>&1 | tail -10
  ```

  Expected: compile error — `PartitionTechGrants` and `ResolvePendingTechGrants` undefined.

- [ ] **T2-S5: Implement PartitionTechGrants**

  Add to `internal/gameserver/technology_assignment.go` after `RearrangePreparedTechs` (exported so `_test` package can call it):

  ```go
  // PartitionTechGrants splits grants into immediate (no player choice needed) and
  // deferred (pool > open slots, player must choose) parts.
  //
  // Precondition: grants is non-nil and valid.
  // Postcondition: immediate + deferred together cover all grants in the input.
  // Either return value may be nil if its category is empty.
  func PartitionTechGrants(grants *ruleset.TechnologyGrants) (immediate, deferred *ruleset.TechnologyGrants) {
      var imm, def ruleset.TechnologyGrants

      // Hardwired: always immediate.
      if len(grants.Hardwired) > 0 {
          imm.Hardwired = append(imm.Hardwired, grants.Hardwired...)
      }

      // Prepared: partition per tech level.
      if grants.Prepared != nil {
          for lvl, slots := range grants.Prepared.SlotsByLevel {
              nFixed := 0
              for _, e := range grants.Prepared.Fixed {
                  if e.Level == lvl {
                      nFixed++
                  }
              }
              nPool := 0
              for _, e := range grants.Prepared.Pool {
                  if e.Level == lvl {
                      nPool++
                  }
              }
              open := slots - nFixed
              if nPool <= open {
                  if imm.Prepared == nil {
                      imm.Prepared = &ruleset.PreparedGrants{SlotsByLevel: make(map[int]int)}
                  }
                  imm.Prepared.SlotsByLevel[lvl] = slots
                  for _, e := range grants.Prepared.Fixed {
                      if e.Level == lvl {
                          imm.Prepared.Fixed = append(imm.Prepared.Fixed, e)
                      }
                  }
                  for _, e := range grants.Prepared.Pool {
                      if e.Level == lvl {
                          imm.Prepared.Pool = append(imm.Prepared.Pool, e)
                      }
                  }
              } else {
                  if def.Prepared == nil {
                      def.Prepared = &ruleset.PreparedGrants{SlotsByLevel: make(map[int]int)}
                  }
                  def.Prepared.SlotsByLevel[lvl] = slots
                  for _, e := range grants.Prepared.Fixed {
                      if e.Level == lvl {
                          def.Prepared.Fixed = append(def.Prepared.Fixed, e)
                      }
                  }
                  for _, e := range grants.Prepared.Pool {
                      if e.Level == lvl {
                          def.Prepared.Pool = append(def.Prepared.Pool, e)
                      }
                  }
              }
          }
      }

      // Spontaneous: partition per tech level.
      if grants.Spontaneous != nil {
          for lvl, known := range grants.Spontaneous.KnownByLevel {
              nFixed := 0
              for _, e := range grants.Spontaneous.Fixed {
                  if e.Level == lvl {
                      nFixed++
                  }
              }
              nPool := 0
              for _, e := range grants.Spontaneous.Pool {
                  if e.Level == lvl {
                      nPool++
                  }
              }
              open := known - nFixed
              if nPool <= open {
                  if imm.Spontaneous == nil {
                      imm.Spontaneous = &ruleset.SpontaneousGrants{KnownByLevel: make(map[int]int)}
                  }
                  imm.Spontaneous.KnownByLevel[lvl] = known
                  for _, e := range grants.Spontaneous.Fixed {
                      if e.Level == lvl {
                          imm.Spontaneous.Fixed = append(imm.Spontaneous.Fixed, e)
                      }
                  }
                  for _, e := range grants.Spontaneous.Pool {
                      if e.Level == lvl {
                          imm.Spontaneous.Pool = append(imm.Spontaneous.Pool, e)
                      }
                  }
              } else {
                  if def.Spontaneous == nil {
                      def.Spontaneous = &ruleset.SpontaneousGrants{KnownByLevel: make(map[int]int)}
                  }
                  def.Spontaneous.KnownByLevel[lvl] = known
                  for _, e := range grants.Spontaneous.Fixed {
                      if e.Level == lvl {
                          def.Spontaneous.Fixed = append(def.Spontaneous.Fixed, e)
                      }
                  }
                  for _, e := range grants.Spontaneous.Pool {
                      if e.Level == lvl {
                          def.Spontaneous.Pool = append(def.Spontaneous.Pool, e)
                      }
                  }
              }
          }
      }

      if len(imm.Hardwired) > 0 || imm.Prepared != nil || imm.Spontaneous != nil {
          immCopy := imm
          immediate = &immCopy
      }
      if def.Prepared != nil || def.Spontaneous != nil {
          defCopy := def
          deferred = &defCopy
      }
      return
  }
  ```

- [ ] **T2-S6: Implement ResolvePendingTechGrants**

  Add to `internal/gameserver/technology_assignment.go` after `PartitionTechGrants`. Also add `"sort"` to imports:

  ```go
  // ResolvePendingTechGrants interactively resolves all pending tech grants for a session.
  // For each entry in sess.PendingTechGrants (ascending level order), calls LevelUpTechnologies
  // with a live promptFn. Removes each entry after successful resolution.
  //
  // Precondition: sess, promptFn, progressRepo, and all repos are non-nil.
  // Postcondition: sess.PendingTechGrants is empty on full success; partially cleared on error.
  // SetPendingTechLevels is called after each resolved entry to keep the DB in sync.
  func ResolvePendingTechGrants(
      ctx context.Context,
      sess *session.PlayerSession,
      characterID int64,
      job *ruleset.Job,
      techReg *technology.Registry,
      promptFn TechPromptFn,
      hwRepo HardwiredTechRepo,
      prepRepo PreparedTechRepo,
      spontRepo SpontaneousTechRepo,
      innateRepo InnateTechRepo,
      progressRepo PendingTechLevelsRepo,
  ) error {
      if len(sess.PendingTechGrants) == 0 {
          return nil
      }
      levels := make([]int, 0, len(sess.PendingTechGrants))
      for lvl := range sess.PendingTechGrants {
          levels = append(levels, lvl)
      }
      sort.Ints(levels)

      for _, lvl := range levels {
          grants := sess.PendingTechGrants[lvl]
          if err := LevelUpTechnologies(ctx, sess, characterID, grants, techReg, promptFn,
              hwRepo, prepRepo, spontRepo, innateRepo,
          ); err != nil {
              return fmt.Errorf("ResolvePendingTechGrants level %d: %w", lvl, err)
          }
          delete(sess.PendingTechGrants, lvl)
          remaining := make([]int, 0, len(sess.PendingTechGrants))
          for k := range sess.PendingTechGrants {
              remaining = append(remaining, k)
          }
          sort.Ints(remaining)
          if err := progressRepo.SetPendingTechLevels(ctx, characterID, remaining); err != nil {
              return fmt.Errorf("ResolvePendingTechGrants SetPendingTechLevels: %w", err)
          }
      }
      return nil
  }
  ```

- [ ] **T2-S7: Run new tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestPartition|TestResolvePending|TestPropertyResolve" -v 2>&1 | tail -20
  ```

  Expected: all PASS.

- [ ] **T2-S8: Run full gameserver test suite**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 2>&1 | tail -5
  ```

  Expected: PASS.

- [ ] **T2-S9: Commit**

  ```bash
  git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
  git commit -m "feat(gameserver): PartitionTechGrants, ResolvePendingTechGrants (REQ-ILT1,2,5,7)"
  ```

---

## Chunk 2 — Task 3

### Task 3: handleGrant changes + login-time resolution

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_levelup_tech_test.go` (package gameserver)

**Context:** `handleGrant` is in `package gameserver`. Tests in `grpc_service_grant_test.go` use `package gameserver`. The `fakePreparedRepoRest` is defined in `grpc_service_rest_test.go` and is visible within the same package in tests. A new internal fake `fakePendingTechLevelsRepoInternal` must be defined in the new test file since `fakePendingTechLevelsRepo` lives in `package gameserver_test`.

---

- [ ] **T3-S1: Read handleGrant signature and testMinimalService helper**

  Before writing any code, read:
  - The `handleGrant` signature in `internal/gameserver/grpc_service.go` (around line 6572)
  - The `testMinimalService` definition (search `grpc_service_*_test.go` files)
  - The `addTargetForGrant` helper (if it exists) or the equivalent pattern used in existing grant tests
  - The existing `fakeSessionStream` definition

  This step produces no code output — it informs T3-S2.

- [ ] **T3-S2: Write failing tests (REQ-ILT1, REQ-ILT2, REQ-ILT3, REQ-ILT4)**

  After completing T3-S1, create `internal/gameserver/grpc_service_levelup_tech_test.go` as `package gameserver` with complete, production-quality test bodies.

  The file MUST include:

  - `fakePendingTechLevelsRepoInternal`: a struct implementing the full `ProgressRepository` interface (read from T3-S1). All existing `ProgressRepository` methods must be implemented as no-ops returning zero values. Add `pendingLevels []int` and `setWasCalled bool` fields for the two new methods.

  - Four complete test functions, each with full arrange/act/assert — no comment-only bodies:

    **TestHandleGrant_LevelUp_AutoAssignOnly_NoPending (REQ-ILT2):**
    - Create a `session.Manager`, call `testMinimalService` (signature from T3-S1)
    - Register a `ruleset.Job` with `LevelUpGrants[2]` containing `PreparedGrants{SlotsByLevel: {1: 1}, Pool: [{ID: "auto_tech", Level: 1}]}`
    - Add a target player via `sessMgr.AddPlayer` (params from T3-S1 helper pattern)
    - Set target level to 1, experience to 0
    - Wire `fakePreparedRepoRest` and `fakePendingTechLevelsRepoInternal` into service via setter methods
    - Call `svc.handleGrant` with enough XP to reach level 2 (use the exact call signature from T3-S1)
    - Assert `target.PendingTechGrants` is empty
    - Assert the prepared tech repo has a slot set for the auto-assigned tech

    **TestHandleGrant_LevelUp_PoolChoiceDeferred (REQ-ILT1):**
    - Same setup, but `LevelUpGrants[2]` contains `Pool: [{ID: "choice_a", Level: 1}, {ID: "choice_b", Level: 1}]` with `SlotsByLevel: {1: 1}`
    - Assert `target.PendingTechGrants[2]` is non-nil
    - Assert `pendingRepo.setWasCalled` is true

    **TestHandleGrant_LevelUp_AutoAssign_PushesNotification (REQ-ILT3):**
    - Same auto-assign setup as REQ-ILT2
    - Capture target entity push events (use the entity mock/channel from T3-S1)
    - Assert at least one pushed message contains "auto-assigned"

    **TestHandleGrant_LevelUp_Deferred_PushesSelectTechNotification (REQ-ILT4):**
    - Same deferred setup as REQ-ILT1
    - Assert at least one pushed message contains "selecttech"

- [ ] **T3-S3: Run tests to confirm compile failure**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleGrant_LevelUp" -v 2>&1 | tail -10
  ```

  Expected: compile errors.

- [ ] **T3-S4: Add private helper functions**

  Add to `internal/gameserver/grpc_service.go` (or a new `internal/gameserver/grpc_service_tech_helpers.go` file if grpc_service.go is already very large):

  ```go
  // snapshotPreparedTechIDs returns a set of all tech IDs currently in PreparedTechs.
  func snapshotPreparedTechIDs(pt map[int][]*session.PreparedSlot) map[string]bool {
      out := make(map[string]bool)
      for _, slots := range pt {
          for _, s := range slots {
              if s != nil {
                  out[s.TechID] = true
              }
          }
      }
      return out
  }

  // snapshotSpontaneousTechIDs returns a set of all tech IDs in SpontaneousTechs.
  func snapshotSpontaneousTechIDs(st map[int][]string) map[string]bool {
      out := make(map[string]bool)
      for _, ids := range st {
          for _, id := range ids {
              out[id] = true
          }
      }
      return out
  }

  // newTechIDs returns tech IDs in after that are not in before slice.
  func newTechIDs(before []string, after []string) []string {
      s := make(map[string]bool, len(before))
      for _, id := range before {
          s[id] = true
      }
      var result []string
      for _, id := range after {
          if !s[id] {
              result = append(result, id)
          }
      }
      return result
  }

  // newTechIDsFromPrepared returns tech IDs in after that are not in the before snapshot.
  func newTechIDsFromPrepared(before map[string]bool, after map[int][]*session.PreparedSlot) []string {
      var result []string
      for _, slots := range after {
          for _, slot := range slots {
              if slot != nil && !before[slot.TechID] {
                  result = append(result, slot.TechID)
              }
          }
      }
      return result
  }

  // newTechIDsFromSpontaneous returns tech IDs in after that are not in the before snapshot.
  func newTechIDsFromSpontaneous(before map[string]bool, after map[int][]string) []string {
      var result []string
      for _, ids := range after {
          for _, id := range ids {
              if !before[id] {
                  result = append(result, id)
              }
          }
      }
      return result
  }
  ```

  **Note:** Check the actual field types for `PreparedTechs` and `SpontaneousTechs` on `PlayerSession` before writing — use the exact types defined in `session/manager.go`. Adjust these helpers to match.

- [ ] **T3-S5: Modify handleGrant**

  Replace the existing `LevelUpTechnologies` call block (around lines 6572-6579) with:

  ```go
  if s.hardwiredTechRepo != nil && s.jobRegistry != nil && target.CharacterID > 0 {
      if job, ok := s.jobRegistry.Job(target.Class); ok {
          if target.PendingTechGrants == nil {
              target.PendingTechGrants = make(map[int]*ruleset.TechnologyGrants)
          }
          for lvl := oldLevel + 1; lvl <= result.NewLevel; lvl++ {
              techGrants, hasGrants := job.LevelUpGrants[lvl]
              if !hasGrants {
                  continue
              }
              immediate, deferred := PartitionTechGrants(techGrants)
              if immediate != nil {
                  hwBefore := append([]string{}, target.HardwiredTechs...)
                  prepBefore := snapshotPreparedTechIDs(target.PreparedTechs)
                  spontBefore := snapshotSpontaneousTechIDs(target.SpontaneousTechs)

                  if err := LevelUpTechnologies(ctx, target, target.CharacterID,
                      immediate, s.techRegistry, nil,
                      s.hardwiredTechRepo, s.preparedTechRepo,
                      s.spontaneousTechRepo, s.innateTechRepo,
                  ); err != nil {
                      s.logger.Warn("handleGrant: LevelUpTechnologies failed",
                          zap.Int64("character_id", target.CharacterID),
                          zap.Int("level", lvl),
                          zap.Error(err))
                  }

                  for _, id := range newTechIDs(hwBefore, target.HardwiredTechs) {
                      _ = target.Entity.Push(marshalMsgEvent(fmt.Sprintf("You gained %s (auto-assigned).", id)))
                  }
                  for _, id := range newTechIDsFromPrepared(prepBefore, target.PreparedTechs) {
                      _ = target.Entity.Push(marshalMsgEvent(fmt.Sprintf("You gained %s (auto-assigned).", id)))
                  }
                  for _, id := range newTechIDsFromSpontaneous(spontBefore, target.SpontaneousTechs) {
                      _ = target.Entity.Push(marshalMsgEvent(fmt.Sprintf("You gained %s (auto-assigned).", id)))
                  }
              }
              if deferred != nil {
                  target.PendingTechGrants[lvl] = deferred
              }
          }
          if len(target.PendingTechGrants) > 0 && s.progressRepo != nil {
              levels := make([]int, 0, len(target.PendingTechGrants))
              for lvl := range target.PendingTechGrants {
                  levels = append(levels, lvl)
              }
              sort.Ints(levels)
              if err := s.progressRepo.SetPendingTechLevels(ctx, target.CharacterID, levels); err != nil {
                  s.logger.Warn("handleGrant: SetPendingTechLevels failed", zap.Error(err))
              }
              _ = target.Entity.Push(marshalMsgEvent("You have pending technology selections! Type 'selecttech' to choose your technologies."))
          }
      }
  }
  ```

  **Pre-step:** Before implementing, run:
  ```bash
  grep -n "func marshalMsgEvent\|marshalMsgEvent" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -5
  ```
  If `marshalMsgEvent` does not exist, grep for the existing pattern used to push a text message to a target entity in `handleGrant` and use the same pattern. Add `"sort"` to imports if not already present.

- [ ] **T3-S6: Add login-time pending tech loading**

  In `Session()`, add BEFORE the existing tech loading block (around line 1003):

  ```go
  // Load pending tech levels from DB and reconstruct PendingTechGrants.
  if sess.CharacterID > 0 && s.progressRepo != nil && s.jobRegistry != nil {
      if pendingLevels, err := s.progressRepo.GetPendingTechLevels(stream.Context(), sess.CharacterID); err == nil {
          if len(pendingLevels) > 0 {
              if job, ok := s.jobRegistry.Job(sess.Class); ok {
                  if sess.PendingTechGrants == nil {
                      sess.PendingTechGrants = make(map[int]*ruleset.TechnologyGrants)
                  }
                  for _, lvl := range pendingLevels {
                      if grants, ok := job.LevelUpGrants[lvl]; ok && grants != nil {
                          _, deferred := PartitionTechGrants(grants)
                          if deferred != nil {
                              sess.PendingTechGrants[lvl] = deferred
                          }
                      }
                  }
              }
          }
      } else {
          s.logger.Warn("Session: GetPendingTechLevels failed", zap.Error(err))
      }
  }
  ```

- [ ] **T3-S7: Add login-time resolution**

  In `Session()`, after the existing ability-boost prompts and BEFORE `commandLoop` (around line 1131), add:

  ```go
  if len(sess.PendingTechGrants) > 0 && s.jobRegistry != nil {
      if job, ok := s.jobRegistry.Job(sess.Class); ok {
          promptFn := func(options []string) (string, error) {
              choices := &ruleset.FeatureChoices{
                  Prompt:  "Choose a technology:",
                  Options: options,
                  Key:     "tech_choice",
              }
              return s.promptFeatureChoice(stream, "tech_choice", choices)
          }
          if err := ResolvePendingTechGrants(stream.Context(), sess, sess.CharacterID,
              job, s.techRegistry, promptFn,
              s.hardwiredTechRepo, s.preparedTechRepo,
              s.spontaneousTechRepo, s.innateTechRepo,
              s.progressRepo,
          ); err != nil {
              s.logger.Warn("Session: ResolvePendingTechGrants failed", zap.Error(err))
          }
      }
  }
  ```

  **Pre-step:** Before writing this block, run:
  ```bash
  grep -n "characterID\|CharacterID\|sess\.CharacterID" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | grep -A 2 -B 2 "1003\|1010" | head -10
  ```
  Use the exact variable name for the character ID that is in scope at the login block location. Typically this is `sess.CharacterID`.

- [ ] **T3-S8: Build**

  ```bash
  cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **T3-S9: Run grant/level-up tech tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleGrant_LevelUp" -v 2>&1 | tail -20
  ```

  Expected: REQ-ILT1, REQ-ILT2, REQ-ILT3, REQ-ILT4 pass.

- [ ] **T3-S10: Run full gameserver suite**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 2>&1 | tail -5
  ```

  Expected: PASS.

- [ ] **T3-S11: Commit**

  ```bash
  git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_levelup_tech_test.go
  git commit -m "feat(gameserver): handleGrant partition + auto-assign notifications + deferred pending grants + login resolution (REQ-ILT1–4, REQ-ILT8)"
  ```

---

## Chunk 3 — Task 4

### Task 4: selecttech command + character sheet

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/selecttech.go`
- Create: `internal/game/command/selecttech_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_selecttech_test.go`
- Modify: `docs/requirements/FEATURES.md`

---

- [ ] **T4-S1: Read command pattern**

  Read `internal/game/command/rest.go` to understand the exact function signature for a pre-dispatch command handler. Read the last few entries in `BuiltinCommands()` and `bridgeHandlerMap` to confirm insertion points. This step produces no code — it informs subsequent steps.

- [ ] **T4-S2: Write failing tests for HandleSelectTech (CMD-3)**

  Create `internal/game/command/selecttech_test.go`:

  ```go
  package command_test

  import (
      "testing"

      "github.com/stretchr/testify/assert"
      "github.com/stretchr/testify/require"

      "github.com/cory-johannsen/mud/internal/game/command"
      "github.com/cory-johannsen/mud/internal/game/session"
  )

  // HandleSelectTech returns a CommandResult with Handler == HandlerSelectTech.
  func TestHandleSelectTech_ReturnsCorrectHandler(t *testing.T) {
      cmd := command.Command{Handler: command.HandlerSelectTech}
      sess := &session.PlayerSession{}
      result := command.HandleSelectTech(cmd, []string{}, sess)
      require.NotNil(t, result)
      assert.Equal(t, command.HandlerSelectTech, result.Handler)
  }
  ```

  **Note:** Adjust the function signature to exactly match the pattern found in T4-S1.

- [ ] **T4-S3: CMD-1/CMD-2 — Add HandlerSelectTech to commands.go**

  In `internal/game/command/commands.go`:

  - Add after `HandlerRest`:
    ```go
    HandlerSelectTech = "selecttech"
    ```

  - Add to `BuiltinCommands()` after the rest entry:
    ```go
    {Name: "selecttech", Help: "Select pending technology upgrades from levelling up.", Category: CategoryCharacter, Handler: HandlerSelectTech},
    ```

    **Note:** The field names `Name`, `Help`, `Category`, `Handler` match the existing `Command` struct. Verify these field names against the struct definition in `commands.go` before implementing — if they differ, use the actual field names.

- [ ] **T4-S4: CMD-3 — Create selecttech.go**

  Create `internal/game/command/selecttech.go` with the exact signature matching the pattern from T4-S1:

  ```go
  package command

  import "github.com/cory-johannsen/mud/internal/game/session"

  // HandleSelectTech handles the selecttech command.
  //
  // Precondition: none.
  // Postcondition: returns a CommandResult with Handler == HandlerSelectTech.
  func HandleSelectTech(cmd Command, args []string, sess *session.PlayerSession) *CommandResult {
      return &CommandResult{Handler: HandlerSelectTech}
  }
  ```

- [ ] **T4-S5: Run selecttech command tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -run "TestHandleSelectTech" -v 2>&1 | tail -10
  ```

  Expected: PASS.

- [ ] **T4-S6: CMD-4 — Check proto highest field numbers**

  ```bash
  grep -n "= [0-9]*;" /home/cjohannsen/src/mud/api/proto/game/v1/game.proto | tail -10
  grep -n "pending_skill_increases\|pending_boosts" /home/cjohannsen/src/mud/api/proto/game/v1/game.proto
  ```

  Confirm `RestRequest rest = 81` is the last `ClientMessage` oneof entry. Identify the highest field number in `CharacterSheetView` for the `pending_tech_selections` field.

- [ ] **T4-S7: CMD-4 — Update proto file**

  In `api/proto/game/v1/game.proto`:

  - Add near the other request messages:
    ```protobuf
    message SelectTechRequest {}
    ```

  - Add to `ClientMessage` oneof:
    ```protobuf
    SelectTechRequest select_tech = 82;
    ```

  - Add to `CharacterSheetView` (using next available field number after reading from T4-S6):
    ```protobuf
    int32 pending_tech_selections = <N>;
    ```

- [ ] **T4-S8: Regenerate proto**

  ```bash
  cd /home/cjohannsen/src/mud && make proto 2>&1 | tail -5
  ```

  Expected: no errors; `game.pb.go` updated.

- [ ] **T4-S9: CMD-5 — Add bridge function**

  In `internal/frontend/handlers/bridge_handlers.go`:

  Add after `bridgeRest`:
  ```go
  func bridgeSelectTech(bctx *bridgeContext) (bridgeResult, error) {
      return bridgeResult{msg: &gamev1.ClientMessage{
          RequestId: bctx.reqID,
          Payload:   &gamev1.ClientMessage_SelectTech{SelectTech: &gamev1.SelectTechRequest{}},
      }}, nil
  }
  ```

  Add to `bridgeHandlerMap` after the rest entry:
  ```go
  command.HandlerSelectTech: bridgeSelectTech,
  ```

- [ ] **T4-S10: Build frontend**

  ```bash
  cd /home/cjohannsen/src/mud && go build ./internal/frontend/... 2>&1
  ```

  Expected: no errors.

- [ ] **T4-S11: Write failing tests for handleSelectTech (REQ-ILT6, REQ-ILT9)**

  Create `internal/gameserver/grpc_service_selecttech_test.go` as `package gameserver`:

  ```go
  package gameserver

  import (
      "context"
      "testing"

      "github.com/stretchr/testify/assert"
      "github.com/stretchr/testify/require"

      "github.com/cory-johannsen/mud/internal/game/session"
      gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
  )

  // REQ-ILT6: handleSelectTech with no pending grants sends "no pending technology selections."
  func TestHandleSelectTech_NoPending_SendsNoPending(t *testing.T) {
      sessMgr := session.NewManager()
      svc := testMinimalService(t, sessMgr)

      uid := "player-selecttech-empty"
      _, err := sessMgr.AddPlayer(session.AddPlayerOptions{
          UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
      })
      require.NoError(t, err)

      stream := &fakeSessionStream{}
      err = svc.handleSelectTech(uid, "req1", stream)
      require.NoError(t, err)

      // Verify last sent message contains "no pending".
      require.NotEmpty(t, stream.sent)
      last := stream.sent[len(stream.sent)-1]
      msg := last.GetMessage()
      require.NotNil(t, msg)
      assert.Contains(t, msg.Content, "no pending")
  }

  // REQ-ILT9: buildCharacterSheetView sets PendingTechSelections > 0 when PendingTechGrants non-empty.
  // Stub — replaced with full implementation in T4-S15 after proto regeneration in T4-S8.
  func TestBuildCharacterSheetView_PendingTechSelections(t *testing.T) {
      t.Skip("implemented in T4-S15 after proto regeneration")
  }
  ```

  After T4-S8 completes, proceed to T4-S15 which contains the full test implementation.

- [ ] **T4-S12: CMD-6 — Implement handleSelectTech**

  Add to `internal/gameserver/grpc_service.go`:

  ```go
  func (s *GameServiceServer) handleSelectTech(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
      sess, ok := s.sessions.GetPlayer(uid)
      if !ok {
          return fmt.Errorf("handleSelectTech: player %q not found", uid)
      }

      sendMsg := func(text string) error {
          return stream.Send(&gamev1.ServerEvent{
              RequestId: requestID,
              Payload:   &gamev1.ServerEvent_Message{Message: &gamev1.MessageEvent{Content: text}},
          })
      }

      if len(sess.PendingTechGrants) == 0 {
          return sendMsg("You have no pending technology selections.")
      }

      if s.jobRegistry == nil {
          return sendMsg("You have no pending technology selections.")
      }
      job, ok := s.jobRegistry.Job(sess.Class)
      if !ok {
          return sendMsg("You have no pending technology selections.")
      }

      promptFn := func(options []string) (string, error) {
          choices := &ruleset.FeatureChoices{
              Prompt:  "Choose a technology:",
              Options: options,
              Key:     "tech_choice",
          }
          return s.promptFeatureChoice(stream, "tech_choice", choices)
      }

      if err := ResolvePendingTechGrants(stream.Context(), sess, sess.CharacterID,
          job, s.techRegistry, promptFn,
          s.hardwiredTechRepo, s.preparedTechRepo,
          s.spontaneousTechRepo, s.innateTechRepo,
          s.progressRepo,
      ); err != nil {
          s.logger.Warn("handleSelectTech failed", zap.String("uid", uid), zap.Error(err))
          return sendMsg("Something went wrong selecting your technologies.")
      }

      return sendMsg("Your technology selections are complete.")
  }
  ```

  **Note:** Verify `s.progressRepo` satisfies `PendingTechLevelsRepo`. Since `ProgressRepository` now includes those two methods, and `s.progressRepo` is typed as `ProgressRepository`, this is satisfied. If the field is named differently, adjust.

- [ ] **T4-S13: Wire handleSelectTech pre-dispatch**

  In `commandLoop` (around lines 1147-1159, after the handleRest check), add:

  ```go
  if _, ok := msg.Payload.(*gamev1.ClientMessage_SelectTech); ok {
      if err := s.handleSelectTech(uid, msg.RequestId, stream); err != nil {
          s.logger.Warn("handleSelectTech error", zap.String("uid", uid), zap.Error(err))
      }
      continue
  }
  ```

- [ ] **T4-S14: Add PendingTechSelections to character sheet**

  In `grpc_service.go`, at the character sheet view construction (around line 3558, after `view.PendingSkillIncreases`), add:

  ```go
  view.PendingTechSelections = int32(len(sess.PendingTechGrants))
  ```

  **Note:** Verify the exact field name matches what was generated in T4-S8. Adjust if different.

- [ ] **T4-S15: Complete REQ-ILT9 test**

  Replace the `t.Skip(...)` body in `TestBuildCharacterSheetView_PendingTechSelections` in `grpc_service_selecttech_test.go` with:

  ```go
  func TestBuildCharacterSheetView_PendingTechSelections(t *testing.T) {
      sessMgr := session.NewManager()
      svc := testMinimalService(t, sessMgr)

      uid := "player-pending-tech-sheet"
      _, err := sessMgr.AddPlayer(session.AddPlayerOptions{
          UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
      })
      require.NoError(t, err)

      target, ok := sessMgr.GetPlayer(uid)
      require.True(t, ok)
      target.PendingTechGrants = map[int]*ruleset.TechnologyGrants{
          2: {Prepared: &ruleset.PreparedGrants{
              SlotsByLevel: map[int]int{1: 1},
              Pool:         []ruleset.PreparedEntry{{ID: "pending_choice", Level: 1}},
          }},
      }

      stream := &fakeSessionStream{}
      // handleChar sends a CharacterSheetView — capture the sent event.
      err = svc.handleChar(uid, "req-sheet", stream)
      require.NoError(t, err)
      require.NotEmpty(t, stream.sent)

      var sheetView *gamev1.CharacterSheetView
      for _, evt := range stream.sent {
          if cs := evt.GetCharacterSheet(); cs != nil {
              sheetView = cs
              break
          }
      }
      require.NotNil(t, sheetView, "CharacterSheetView must be sent by handleChar")
      assert.Equal(t, int32(1), sheetView.PendingTechSelections)
  }
  ```

  **Note:** If `handleChar` does not exist or has a different name, use the equivalent function from `grpc_service.go` that constructs and sends a `CharacterSheetView`. Confirm the function name by searching for `CharacterSheetView` assignments in `grpc_service.go` before implementing.

- [ ] **T4-S16: Build all**

  ```bash
  cd /home/cjohannsen/src/mud && go build ./... 2>&1 | head -20
  ```

  Expected: no errors.

- [ ] **T4-S17: Run all tests including TestAllCommandHandlersAreWired**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | grep -E "FAIL|^ok|TestAllCommandHandlersAreWired"
  ```

  Expected: all pass; `TestAllCommandHandlersAreWired` PASS.

- [ ] **T4-S18: Update FEATURES.md**

  In `docs/requirements/FEATURES.md`, mark the level-up technology selection feature complete:

  ```markdown
  - [x] Level-up technology selection — player selects new prepared/spontaneous techs interactively at next login or via `selecttech`; auto-assigned grants notify in-console; persisted in `character_pending_tech_levels`
  ```

- [ ] **T4-S19: Commit**

  ```bash
  git add \
    internal/game/command/commands.go \
    internal/game/command/selecttech.go \
    internal/game/command/selecttech_test.go \
    api/proto/game/v1/game.proto \
    internal/gameserver/gamev1/game.pb.go \
    internal/gameserver/gamev1/game_grpc.pb.go \
    internal/frontend/handlers/bridge_handlers.go \
    internal/gameserver/grpc_service.go \
    internal/gameserver/grpc_service_selecttech_test.go \
    docs/requirements/FEATURES.md
  git commit -m "feat(gameserver): selecttech command; character sheet pending tech display (REQ-ILT6, REQ-ILT9, CMD-1–7)"
  ```

---

## Requirements Checklist

| REQ | Task | Description |
|-----|------|-------------|
| REQ-ILT1 | Task 2 + Task 3 | Pool > open → `PendingTechGrants` populated; `LevelUpTechnologies` not called for deferred levels at grant time |
| REQ-ILT2 | Task 2 + Task 3 | Auto-assign only → `PendingTechGrants` empty; tech applied immediately |
| REQ-ILT3 | Task 3 | Auto-assigned tech pushes notification containing tech name to target entity stream |
| REQ-ILT4 | Task 3 | Deferred grants push "Type 'selecttech'" notification to target entity stream |
| REQ-ILT5 | Task 2 | `ResolvePendingTechGrants` prompts for each pending level, calls `LevelUpTechnologies`, clears entries after resolution |
| REQ-ILT6 | Task 4 | `handleSelectTech` with no pending grants sends "no pending technology selections." |
| REQ-ILT7 | Task 2 | Property: for any combination of grants, all chosen tech IDs after `ResolvePendingTechGrants` are valid pool members |
| REQ-ILT8 | Task 3 | `Session()` login path loads and resolves pending grants before `commandLoop` |
| REQ-ILT9 | Task 4 | Character sheet shows `PendingTechSelections: N` when N > 0 |
| CMD-1 | Task 4 | `HandlerSelectTech` constant added to `commands.go` |
| CMD-2 | Task 4 | `Command{...}` entry appended to `BuiltinCommands()` |
| CMD-3 | Task 4 | `HandleSelectTech` implemented in `selecttech.go` with TDD coverage |
| CMD-4 | Task 4 | `SelectTechRequest` proto message added; `make proto` regenerated |
| CMD-5 | Task 4 | `bridgeSelectTech` added to `bridge_handlers.go`; `TestAllCommandHandlersAreWired` passes |
| CMD-6 | Task 4 | `handleSelectTech` implemented in `grpc_service.go`; wired into `commandLoop` pre-dispatch |
| CMD-7 | Task 4 | All tests pass; `selecttech` fully wired end-to-end |
