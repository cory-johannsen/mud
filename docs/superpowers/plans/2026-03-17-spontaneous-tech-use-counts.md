# Spontaneous Technology Use Counts Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add daily use-count tracking for spontaneous technologies using a shared per-level pool model (PF2E spell-slot style).

**Architecture:** New `character_spontaneous_use_pools` DB table; `UsePool` struct in session package; `SpontaneousUsePoolRepo` interface + postgres implementation; updated `AssignTechnologies`, `LevelUpTechnologies`, `LoadTechnologies`, `ResolvePendingTechGrants`; extended `handleUse` and `handleRest`; new `SpontaneousUsePoolView` proto message.

**Tech Stack:** Go, PostgreSQL (pgx/v5), pgregory.net/rapid (property-based testing), protobuf

---

## Chunk 1: DB Migration + UsePool Struct + Repo Interface/Implementation

### Task 1.1: Write failing repo tests (TDD red)

- [ ] Create `/home/cjohannsen/src/mud/internal/storage/postgres/character_spontaneous_use_pool_test.go` in package `postgres_test`.
  - Import `pgregory.net/rapid`, `testing`, `context`, and the postgres package.
  - Add `TestSpontaneousUsePool_RoundTrip` (REQ-SUC6): create a fresh account+character using the existing helper pattern from sibling test files, call `Set`, then `GetAll`, verify the returned map has the expected `UsePool{Remaining, Max}`, then call `Decrement` and verify `GetAll` returns `Remaining-1`.
  - Add `TestSpontaneousUsePool_DecrementBelowZero_Property` (REQ-SUC7) using `rapid.Check`: draw `N` (1–10) as max uses and `calls` (0–15) as activation count; `Set` pool to `{N, N}`; call `Decrement` `calls` times; assert `GetAll` returns `max(0, N-calls)`.
  - Add `TestSpontaneousUsePool_RestoreAll`: `Set` two levels, `Decrement` both, call `RestoreAll`, assert both levels return `Remaining == Max`.
  - Add `TestSpontaneousUsePool_DeleteAll`: `Set` a level, call `DeleteAll`, assert `GetAll` returns empty map.
- [ ] Run `go test ./internal/storage/postgres/...` and confirm compilation failure (repo type does not exist yet).
  - Expected: compile error referencing `CharacterSpontaneousUsePoolRepository`.

### Task 1.2: Add migration SQL files

- [ ] Create `/home/cjohannsen/src/mud/migrations/028_spontaneous_use_pools.up.sql`:
  ```sql
  CREATE TABLE character_spontaneous_use_pools (
      character_id   BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
      tech_level     INT    NOT NULL,
      uses_remaining INT    NOT NULL DEFAULT 0,
      max_uses       INT    NOT NULL DEFAULT 0,
      PRIMARY KEY (character_id, tech_level)
  );
  ```
- [ ] Create `/home/cjohannsen/src/mud/migrations/028_spontaneous_use_pools.down.sql`:
  ```sql
  DROP TABLE character_spontaneous_use_pools;
  ```

### Task 1.3: Add `character_spontaneous_use_pools` to main_test.go migrations

- [ ] Modify `/home/cjohannsen/src/mud/internal/storage/postgres/main_test.go`.
  - Inside `applyAllMigrations`, after the `character_innate_technologies` CREATE TABLE block (line ~254) and before the closing backtick of the large SQL string, append:
    ```sql
    CREATE TABLE IF NOT EXISTS character_spontaneous_use_pools (
        character_id   BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
        tech_level     INT    NOT NULL,
        uses_remaining INT    NOT NULL DEFAULT 0,
        max_uses       INT    NOT NULL DEFAULT 0,
        PRIMARY KEY (character_id, tech_level)
    );
    ```

### Task 1.4: Add `UsePool` struct to session package

- [ ] Modify `/home/cjohannsen/src/mud/internal/game/session/technology.go`.
  - Append after the existing `InnateSlot` struct:
    ```go
    // UsePool tracks remaining and maximum daily uses for a spontaneous tech level.
    type UsePool struct {
        Remaining int
        Max       int
    }
    ```

### Task 1.5: Add `SpontaneousUsePools` field to `PlayerSession`

- [ ] Modify `/home/cjohannsen/src/mud/internal/game/session/manager.go`.
  - In `PlayerSession`, after the `InnateTechs` field (line ~127), add:
    ```go
    // SpontaneousUsePools tracks daily use pools per tech level.
    // Key: tech level (1-based). Value: UsePool with remaining and max uses.
    SpontaneousUsePools map[int]UsePool
    ```

### Task 1.6: Add `SpontaneousUsePoolRepo` interface to technology_assignment.go

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/technology_assignment.go`.
  - After the `InnateTechRepo` interface block (~line 48), add:
    ```go
    // SpontaneousUsePoolRepo manages the daily use pool for spontaneous technologies.
    //
    // Precondition: characterID > 0; techLevel >= 1; uses >= 0.
    type SpontaneousUsePoolRepo interface {
        // GetAll returns all use pools for the character.
        // Postcondition: returned map contains one UsePool per initialized tech level.
        GetAll(ctx context.Context, characterID int64) (map[int]session.UsePool, error)

        // Set initializes or overwrites a pool entry.
        // Postcondition: row (characterID, techLevel) has uses_remaining=usesRemaining, max_uses=maxUses.
        Set(ctx context.Context, characterID int64, techLevel, usesRemaining, maxUses int) error

        // Decrement atomically decrements uses_remaining by 1 if > 0.
        // Precondition: caller has verified uses_remaining > 0 in session before calling.
        // Postcondition: uses_remaining = max(0, uses_remaining - 1).
        Decrement(ctx context.Context, characterID int64, techLevel int) error

        // RestoreAll sets uses_remaining = max_uses for all rows of this character.
        // Postcondition: all pools are at maximum.
        RestoreAll(ctx context.Context, characterID int64) error

        // DeleteAll removes all pool entries for the character.
        DeleteAll(ctx context.Context, characterID int64) error
    }
    ```

### Task 1.7: Implement `CharacterSpontaneousUsePoolRepository`

- [ ] Create `/home/cjohannsen/src/mud/internal/storage/postgres/character_spontaneous_use_pool.go` in package `postgres`.
  - Struct:
    ```go
    type CharacterSpontaneousUsePoolRepository struct {
        db *pgxpool.Pool
    }

    func NewCharacterSpontaneousUsePoolRepository(db *pgxpool.Pool) *CharacterSpontaneousUsePoolRepository {
        return &CharacterSpontaneousUsePoolRepository{db: db}
    }
    ```
  - Implement all five methods (`GetAll`, `Set`, `Decrement`, `RestoreAll`, `DeleteAll`) exactly as specified in the design spec (Feature 3). Each method wraps errors with the repository name prefix, e.g. `"CharacterSpontaneousUsePoolRepository.GetAll: %w"`.
  - `GetAll` queries `character_spontaneous_use_pools WHERE character_id = $1`, scans `tech_level`, `uses_remaining`, `max_uses`, returns `map[int]session.UsePool`.
  - `Set` uses `INSERT ... ON CONFLICT (character_id, tech_level) DO UPDATE SET uses_remaining = EXCLUDED.uses_remaining, max_uses = EXCLUDED.max_uses`.
  - `Decrement` uses `UPDATE ... SET uses_remaining = GREATEST(0, uses_remaining - 1) WHERE character_id = $1 AND tech_level = $2`.
  - `RestoreAll` uses `UPDATE ... SET uses_remaining = max_uses WHERE character_id = $1`.
  - `DeleteAll` uses `DELETE FROM character_spontaneous_use_pools WHERE character_id = $1`.

### Task 1.8: Verify tests pass (TDD green)

- [ ] Run `go test ./internal/storage/postgres/... -v -run TestSpontaneousUsePool` and confirm all four tests pass.
- [ ] Run `go test ./internal/...` and confirm no regressions.

### Task 1.9: Commit Chunk 1

- [ ] `git add migrations/028_spontaneous_use_pools.up.sql migrations/028_spontaneous_use_pools.down.sql internal/game/session/technology.go internal/game/session/manager.go internal/gameserver/technology_assignment.go internal/storage/postgres/character_spontaneous_use_pool.go internal/storage/postgres/character_spontaneous_use_pool_test.go internal/storage/postgres/main_test.go`
- [ ] `git commit -m "feat(spontaneous-tech): DB migration 028, UsePool struct, SpontaneousUsePoolRepo interface + postgres implementation"`

---

## Chunk 2: AssignTechnologies + LevelUpTechnologies + LoadTechnologies + ResolvePendingTechGrants

### Task 2.1: Write failing unit tests for updated assignment functions (TDD red)

- [ ] Create `/home/cjohannsen/src/mud/internal/gameserver/technology_assignment_use_pool_test.go`.
  - `TestAssignTechnologies_InitializesUsePools`: construct a fake `SpontaneousUsePoolRepo` (stub that records `Set` calls), call `AssignTechnologies` with a job that has `Spontaneous.UsesByLevel = {1: 3}`, assert that `Set` was called with `(characterID, 1, 3, 3)` and `sess.SpontaneousUsePools[1] == UsePool{Remaining:3, Max:3}`.
  - `TestLevelUpTechnologies_AddsToUsePools`: prime `sess.SpontaneousUsePools[1] = UsePool{Remaining:2, Max:3}`, call `LevelUpTechnologies` with `levelGrants.Spontaneous.UsesByLevel = {1: 2}`, assert `sess.SpontaneousUsePools[1] == UsePool{Remaining:4, Max:5}` and `Set` called with `(charID, 1, 4, 5)`.
  - `TestLoadTechnologies_LoadsUsePools`: stub `GetAll` to return `{2: UsePool{1,3}}`, call `LoadTechnologies`, assert `sess.SpontaneousUsePools[2] == UsePool{1,3}`.
- [ ] Run `go build ./internal/gameserver/...` and confirm failure (signatures not yet updated).

### Task 2.2: Update `AssignTechnologies` signature and body

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/technology_assignment.go`.
  - Add `usePoolRepo SpontaneousUsePoolRepo` as the last parameter of `AssignTechnologies`.
  - After the block that populates `sess.SpontaneousTechs`, add the pool initialization block:
    ```go
    if grants != nil && grants.Spontaneous != nil {
        if sess.SpontaneousUsePools == nil {
            sess.SpontaneousUsePools = make(map[int]session.UsePool)
        }
        for level, uses := range grants.Spontaneous.UsesByLevel {
            sess.SpontaneousUsePools[level] = session.UsePool{Remaining: uses, Max: uses}
            if err := usePoolRepo.Set(ctx, characterID, level, uses, uses); err != nil {
                return fmt.Errorf("AssignTechnologies: set spontaneous use pool level %d: %w", level, err)
            }
        }
    }
    ```

### Task 2.3: Update `LevelUpTechnologies` signature and body

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/technology_assignment.go`.
  - Add `usePoolRepo SpontaneousUsePoolRepo` as a parameter of `LevelUpTechnologies`.
  - After the block that processes spontaneous tech known-slot grants, add the additive pool delta block:
    ```go
    if levelGrants.Spontaneous != nil {
        if sess.SpontaneousUsePools == nil {
            sess.SpontaneousUsePools = make(map[int]session.UsePool)
        }
        for level, uses := range levelGrants.Spontaneous.UsesByLevel {
            existing := sess.SpontaneousUsePools[level]
            newMax := existing.Max + uses
            newRemaining := existing.Remaining + uses
            sess.SpontaneousUsePools[level] = session.UsePool{Remaining: newRemaining, Max: newMax}
            if err := usePoolRepo.Set(ctx, characterID, level, newRemaining, newMax); err != nil {
                return fmt.Errorf("LevelUpTechnologies: set spontaneous use pool level %d: %w", level, err)
            }
        }
    }
    ```

### Task 2.4: Update `LoadTechnologies` signature and body

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/technology_assignment.go`.
  - Add `usePoolRepo SpontaneousUsePoolRepo` as a parameter of `LoadTechnologies`.
  - After loading spontaneous tech assignments into `sess.SpontaneousTechs`, add:
    ```go
    pools, err := usePoolRepo.GetAll(ctx, characterID)
    if err != nil {
        return fmt.Errorf("LoadTechnologies: load spontaneous use pools: %w", err)
    }
    sess.SpontaneousUsePools = pools
    ```

### Task 2.5: Update `ResolvePendingTechGrants` to pass `usePoolRepo` through

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/technology_assignment.go`.
  - Add `usePoolRepo SpontaneousUsePoolRepo` as a parameter of `ResolvePendingTechGrants`.
  - Pass it to the `LevelUpTechnologies` call inside.

### Task 2.6: Update all call sites in `grpc_service.go`

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go`.
  - Search for all calls to `AssignTechnologies`, `LevelUpTechnologies`, `LoadTechnologies`, and `ResolvePendingTechGrants` and append `s.spontaneousUsePoolRepo` as the final argument.
  - Note: `spontaneousUsePoolRepo` field does not yet exist on `GameServiceServer` — it will be wired in Chunk 5. For now, use a nil placeholder **only if the code compiles; otherwise proceed to Chunk 5 wiring first and come back**. To keep tasks sequential, add the field to `GameServiceServer` struct now (as `spontaneousUsePoolRepo SpontaneousUsePoolRepo`) and update `NewGameServiceServer` in Task 2.6 as well so all tests compile. Full wiring (constructor argument + postgres instantiation) is done in Chunk 5.

### Task 2.7: Verify tests pass (TDD green)

- [ ] Run `go build ./internal/...` — must compile with no errors.
- [ ] Run `go test ./internal/gameserver/... -v -run TestAssignTechnologies_InitializesUsePools` and confirm pass.
- [ ] Run `go test ./internal/gameserver/... -v -run TestLevelUpTechnologies_AddsToUsePools` and confirm pass.
- [ ] Run `go test ./internal/gameserver/... -v -run TestLoadTechnologies_LoadsUsePools` and confirm pass.
- [ ] Run `go test ./internal/...` — no regressions.

### Task 2.8: Commit Chunk 2

- [ ] Stage all modified files.
- [ ] `git commit -m "feat(spontaneous-tech): update AssignTechnologies, LevelUpTechnologies, LoadTechnologies, ResolvePendingTechGrants with usePoolRepo parameter"`

---

## Chunk 3: handleUse Extension (Spontaneous Path)

### Task 3.1: Write failing integration tests (TDD red)

- [ ] Create `/home/cjohannsen/src/mud/internal/gameserver/grpc_service_spontaneous_use_test.go` in package `gameserver`.
  - Use the same helper pattern as `grpc_service_grapple_test.go` (`testWorldAndSession`, `NewGameServiceServer` with nil arguments, `zaptest.NewLogger`).
  - Helper `newSpontaneousSvc(t)` constructs a minimal `GameServiceServer` with a stub `SpontaneousUsePoolRepo` that holds an in-memory map, suitable for unit-level handleUse tests without a real DB.
  - `TestHandleUse_SpontaneousActivation_REQ_SUC1`: inject a session with `SpontaneousTechs = {1: ["mind_spike"]}` and `SpontaneousUsePools = {1: UsePool{Remaining:2, Max:3}}`. Call `handleUse` with `abilityID = "mind_spike"`. Assert response message is `"You activate mind_spike. (1 uses remaining at level 1.)"`. Assert stub repo `Decrement` was called with `(charID, 1)`. Assert `sess.SpontaneousUsePools[1].Remaining == 1`.
  - `TestHandleUse_SpontaneousNoUsesRemaining_REQ_SUC2`: inject session with `SpontaneousUsePools = {1: UsePool{Remaining:0, Max:3}}`. Call `handleUse`. Assert response is `"No level 1 uses remaining."`.
  - `TestHandleUse_SpontaneousUnknownTech_REQ_SUC3`: inject session with empty `SpontaneousTechs`. Call `handleUse` with `abilityID = "unknown_tech"`. Assert response is `"You don't know unknown_tech."`.
  - `TestHandleUse_ListMode_IncludesSpontaneous_REQ_SUC4`: inject session with `SpontaneousTechs = {1: ["mind_spike"]}`, `SpontaneousUsePools = {1: UsePool{Remaining:2, Max:3}}`. Call `handleUse` with `abilityID = ""`. Assert the response text contains `"mind_spike (2 uses remaining at level 1)"`.
  - Property test `TestHandleUse_SpontaneousProperty_REQ_SUC7`: using `rapid.Check`, draw `N` (1–5) as max uses, set up session and stub repo. Call `handleUse` `N` times; assert each returns a success message. Call once more; assert response is `"No level 1 uses remaining."`.
- [ ] Run `go test ./internal/gameserver/... -run TestHandleUse_Spontaneous` — expect compile or test failure (spontaneous path not yet implemented).

### Task 3.2: Implement spontaneous path in `handleUse`

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go`.
  - Locate the end of `handleUse`, just before the final `return messageEvent(fmt.Sprintf("You don't have an active ability named %q.", abilityID)), nil` statement (~line 4731).
  - **No-arg list mode:** In the section that lists choices when `abilityID == ""`, after the block that appends prepared tech entries to the choices slice, append spontaneous tech entries. Collect levels from `sess.SpontaneousUsePools` where `pool.Remaining > 0`, sort ascending, and for each tech at that level append `fmt.Sprintf("%s (%d uses remaining at level %d)", techID, pool.Remaining, level)` to choices.
  - **Activation path:** Before the final fallthrough `return messageEvent("You don't have an active ability..."`)`, add a spontaneous tech lookup block:
    ```go
    // Spontaneous tech lookup — only if no feat/class-feature/prepared-tech matched.
    if len(sess.SpontaneousTechs) > 0 {
        levels := make([]int, 0, len(sess.SpontaneousTechs))
        for l := range sess.SpontaneousTechs {
            levels = append(levels, l)
        }
        sort.Ints(levels)
        foundLevel := -1
        for _, l := range levels {
            for _, tid := range sess.SpontaneousTechs[l] {
                if tid == abilityID {
                    foundLevel = l
                    break
                }
            }
            if foundLevel >= 0 {
                break
            }
        }
        if foundLevel < 0 {
            return messageEvent(fmt.Sprintf("You don't know %s.", abilityID)), nil
        }
        pool := sess.SpontaneousUsePools[foundLevel]
        if pool.Remaining <= 0 {
            return messageEvent(fmt.Sprintf("No level %d uses remaining.", foundLevel)), nil
        }
        if err := s.spontaneousUsePoolRepo.Decrement(ctx, sess.CharacterID, foundLevel); err != nil {
            s.logger.Warn("handleUse: Decrement spontaneous pool failed",
                zap.String("uid", uid),
                zap.String("techID", abilityID),
                zap.Error(err))
        }
        pool.Remaining--
        sess.SpontaneousUsePools[foundLevel] = pool
        return messageEvent(fmt.Sprintf("You activate %s. (%d uses remaining at level %d.)", abilityID, pool.Remaining, foundLevel)), nil
    }
    ```
  - The existing final `return messageEvent("You don't have an active ability named...")` remains as the ultimate fallback for when `SpontaneousTechs` is nil or empty.

### Task 3.3: Verify tests pass (TDD green)

- [ ] Run `go test ./internal/gameserver/... -v -run TestHandleUse_Spontaneous` — all five tests must pass.
- [ ] Run `go test ./internal/...` — no regressions.

### Task 3.4: Commit Chunk 3

- [ ] Stage modified and new files.
- [ ] `git commit -m "feat(spontaneous-tech): extend handleUse with spontaneous path (REQ-SUC1..4, REQ-SUC7)"`

---

## Chunk 4: handleRest Extension + Proto + Character Sheet

### Task 4.1: Write failing test for handleRest restoration (TDD red)

- [ ] In `/home/cjohannsen/src/mud/internal/gameserver/grpc_service_spontaneous_use_test.go`, add:
  - `TestHandleRest_RestoresSpontaneousPools_REQ_SUC5`: build a minimal svc with a stub `SpontaneousUsePoolRepo` that starts with `SpontaneousUsePools = {1: UsePool{Remaining:0, Max:3}}`. Call `handleRest`. Assert stub `RestoreAll` was called. Assert `sess.SpontaneousUsePools[1].Remaining == 3`.
- [ ] Run `go test ./internal/gameserver/... -run TestHandleRest_Restores` — expect failure.

### Task 4.2: Extend `handleRest` in `grpc_service.go`

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go`.
  - Locate `handleRest` (~line 2454), after the `RearrangePreparedTechs` call (before the final `sendMsg`), insert:
    ```go
    if err := s.spontaneousUsePoolRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
        return fmt.Errorf("handleRest: restore spontaneous use pools: %w", err)
    }
    pools, err := s.spontaneousUsePoolRepo.GetAll(ctx, sess.CharacterID)
    if err != nil {
        return fmt.Errorf("handleRest: reload spontaneous use pools: %w", err)
    }
    sess.SpontaneousUsePools = pools
    ```
  - Note: `ctx` must be declared in scope. In the current `handleRest`, `context.Background()` is passed inline to `RearrangePreparedTechs`. Extract it to a local `ctx := context.Background()` before that call, or reuse the existing inline ctx by declaring it.

### Task 4.3: Add `SpontaneousUsePoolView` proto message and `CharacterSheetView` field

- [ ] Modify `/home/cjohannsen/src/mud/api/proto/game/v1/game.proto`.
  - After the `ResistanceEntry` message (~line 711), add:
    ```protobuf
    // SpontaneousUsePoolView delivers the daily use pool for one spontaneous tech level.
    message SpontaneousUsePoolView {
        int32 tech_level     = 1;
        int32 uses_remaining = 2;
        int32 max_uses       = 3;
    }
    ```
  - In `CharacterSheetView`, after field 44 (`repeated PreparedSlotView prepared_slots = 44;`), add:
    ```protobuf
    repeated SpontaneousUsePoolView spontaneous_use_pools = 45;
    ```

### Task 4.4: Regenerate proto

- [ ] Run `make proto` from `/home/cjohannsen/src/mud`.
  - Expected output: proto files regenerated with no errors; `internal/gameserver/gamev1/game.pb.go` updated.

### Task 4.5: Populate `SpontaneousUsePools` in `handleChar`

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go`.
  - Find the `handleChar` function that populates `CharacterSheetView`.
  - After the block that populates `PreparedSlots`, add:
    ```go
    for level, pool := range sess.SpontaneousUsePools {
        view.SpontaneousUsePools = append(view.SpontaneousUsePools, &gamev1.SpontaneousUsePoolView{
            TechLevel:     int32(level),
            UsesRemaining: int32(pool.Remaining),
            MaxUses:       int32(pool.Max),
        })
    }
    ```

### Task 4.6: Verify tests pass (TDD green)

- [ ] Run `go test ./internal/gameserver/... -v -run TestHandleRest_Restores` — must pass.
- [ ] Run `go build ./...` — must compile with no errors.
- [ ] Run `go test ./internal/...` — no regressions.

### Task 4.7: Commit Chunk 4

- [ ] Stage all modified files (`grpc_service.go`, `game.proto`, regenerated `game.pb.go`).
- [ ] `git commit -m "feat(spontaneous-tech): handleRest restores pools; SpontaneousUsePoolView proto; character sheet population (REQ-SUC5)"`

---

## Chunk 5: Wire GameServiceServer + Full Integration Verification

### Task 5.1: Add `spontaneousUsePoolRepo` field to `GameServiceServer` struct

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go`.
  - Locate the `GameServiceServer` struct definition. Add:
    ```go
    spontaneousUsePoolRepo SpontaneousUsePoolRepo
    ```
  - Place it adjacent to `spontaneousTechRepo` for readability.

### Task 5.2: Add `spontaneousUsePoolRepo` parameter to `NewGameServiceServer`

- [ ] Modify `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go`.
  - Add `spontaneousUsePoolRepo SpontaneousUsePoolRepo` as the last parameter of `NewGameServiceServer` (after `actionH *ActionHandler`).
  - In the body, assign: `spontaneousUsePoolRepo: spontaneousUsePoolRepo,`.

### Task 5.3: Update all `NewGameServiceServer` call sites to pass the new argument

There are approximately 80 call sites across many test files. Use a mechanical approach:

- [ ] Run `grep -rn "NewGameServiceServer(" /home/cjohannsen/src/mud --include="*.go" | grep -v "grpc_service.go"` to enumerate all call sites.
- [ ] For the **production wiring** call site (in `cmd/` — likely `cmd/gameserver/main.go` or `cmd/mud/main.go`): add `postgres.NewCharacterSpontaneousUsePoolRepository(db)` as the last argument.
- [ ] For **every test call site** (all `*_test.go` files in `internal/gameserver/`): the new parameter is `spontaneousUsePoolRepo SpontaneousUsePoolRepo`. For tests that do NOT exercise spontaneous use (i.e., all tests except those in `grpc_service_spontaneous_use_test.go`), pass `nil`. For tests in `grpc_service_spontaneous_use_test.go`, pass the stub repo already constructed in the test helper.
- [ ] Use sed for the bulk test-file update: `grep -rln "NewGameServiceServer(" /home/cjohannsen/src/mud/internal/gameserver --include="*_test.go" | xargs sed -i 's/NewGameServiceServer(\(.*\))$/NewGameServiceServer(\1, nil)/'` — BUT verify the sed transformation on 2-3 files first before running across all files, since `NewGameServiceServer` calls may span multiple lines. If multi-line, update each file individually.
- [ ] After updating all call sites, run `go build ./...` to confirm zero compile errors before proceeding.

### Task 5.4: Verify all call sites compile

- [ ] Run `go build ./...` — must succeed with no errors.

### Task 5.5: Run full test suite

- [ ] Run `go test ./internal/... -timeout 5m` and confirm 100% pass (SWENG-6).
- [ ] Run `go test ./internal/storage/postgres/... -v -run TestSpontaneousUsePool` — all pool repo tests pass.
- [ ] Run `go test ./internal/gameserver/... -v -run TestHandleUse_Spontaneous` — all handleUse tests pass.
- [ ] Run `go test ./internal/gameserver/... -v -run TestHandleRest_Restores` — passes.

### Task 5.6: Commit Chunk 5

- [ ] Stage all files.
- [ ] `git commit -m "feat(spontaneous-tech): wire GameServiceServer; full spontaneous use-count feature complete"`

---

## Requirements Traceability

| Requirement | Task(s) |
|---|---|
| REQ-SUC1 (activation → message + decrement) | 3.1, 3.2 |
| REQ-SUC2 (0 uses → error message) | 3.1, 3.2 |
| REQ-SUC3 (unknown tech → error message) | 3.1, 3.2 |
| REQ-SUC4 (list mode shows spontaneous with counts) | 3.1, 3.2 |
| REQ-SUC5 (rest restores pools) | 4.1, 4.2 |
| REQ-SUC6 (DB round-trip Set/GetAll/Decrement) | 1.1, 1.7 |
| REQ-SUC7 (property: N uses → exactly min(calls,N) consumed) | 1.1, 3.1 |

## Files Modified/Created Summary

| File | Action | Chunk |
|---|---|---|
| `migrations/028_spontaneous_use_pools.up.sql` | Create | 1 |
| `migrations/028_spontaneous_use_pools.down.sql` | Create | 1 |
| `internal/game/session/technology.go` | Modify (add `UsePool`) | 1 |
| `internal/game/session/manager.go` | Modify (add `SpontaneousUsePools` field) | 1 |
| `internal/gameserver/technology_assignment.go` | Modify (interface + updated function signatures) | 1, 2 |
| `internal/storage/postgres/character_spontaneous_use_pool.go` | Create | 1 |
| `internal/storage/postgres/character_spontaneous_use_pool_test.go` | Create | 1 |
| `internal/storage/postgres/main_test.go` | Modify (add table DDL) | 1 |
| `internal/gameserver/grpc_service.go` | Modify (struct field, constructor, handleUse, handleRest, handleChar, call sites) | 2, 3, 4, 5 |
| `internal/gameserver/grpc_service_spontaneous_use_test.go` | Create | 3, 4 |
| `api/proto/game/v1/game.proto` | Modify (new message + field) | 4 |
| `internal/gameserver/gamev1/game.pb.go` | Regenerated via `make proto` | 4 |
