# Downtime Activity Queue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the single-activity downtime system to support a persisted, per-character queue of up to N activities that auto-start on completion.

**Architecture:** Job YAML files gain a `tier int` field (fatal load error if absent). `DowntimeQueueLimitRegistry` computes the per-character queue cap from tier+level at login. `CharacterDowntimeQueueRepository` persists queue rows with 1-based contiguous positions. `StartNext` (added to `engine.go`) pops, validates, and starts the next queued activity — called both on activity completion and on reconnect. Queue subcommands are dispatched through the existing `handleDowntime` handler, no new handler constant needed.

**Tech Stack:** Go, PostgreSQL, testify, rapid, YAML

**Dependencies:** `downtime` plan must be executed first (adds `DowntimeActivityID`, `DowntimeBusy`, `DowntimeCompletesAt`, `DowntimeMetadata`, `CharacterDowntimeRepository`, `AllUIDs()` to session.Manager, `checkDowntimeCompletion`, `resolveDowntimeActivity`)

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/game/ruleset/job.go` | Add `Tier int` field to `Job` struct |
| Modify | `content/jobs/*.yaml` (76 files) | Add `tier: N` to each job YAML |
| Add | `content/downtime_queue_limits.yaml` | Queue limit lookup table (tier × level range → max_queue) |
| Add | `internal/game/downtime/queue_limits.go` | `DowntimeQueueLimitRegistry` — Lookup(jobTier, level) int |
| Add | `internal/game/downtime/queue_limits_test.go` | Unit tests for registry lookup |
| Modify | `internal/game/session/manager.go` | Add `JobTier int`, `DowntimeQueueLimit int` to PlayerSession |
| Add migration | `internal/storage/postgres/migrations/<N>_add_character_downtime_queue.up.sql` | `character_downtime_queue` table |
| Add migration | `internal/storage/postgres/migrations/<N>_add_character_downtime_queue.down.sql` | rollback |
| Add | `internal/storage/postgres/character_downtime_queue.go` | `CharacterDowntimeQueueRepository` (Enqueue, ListQueue, RemoveAt, Clear, PopHead) |
| Add | `internal/storage/postgres/character_downtime_queue_test.go` | Integration tests for queue repo |
| Modify | `internal/game/downtime/engine.go` | Add `StartNext` function |
| Add | `internal/gameserver/start_next_test.go` | Integration tests for `startNext` |
| Modify | `internal/gameserver/grpc_service.go` | Add queue subcommands to `handleDowntime`; update `checkDowntimeCompletion`; update reconnect resume |
| Modify | `internal/gameserver/deps.go` | Add `DowntimeQueueRepo`, `DowntimeQueueLimitRegistry` to deps |
| Modify | `cmd/gameserver/wire.go` | Wire new repo + registry |

---

### Task 1: Job struct `Tier` field + YAML updates

**Files:**
- Modify: `internal/game/ruleset/job.go`
- Modify: `content/jobs/*.yaml` (76 files)

- [ ] **Step 1: Add `Tier` field to Job struct**

In `internal/game/ruleset/job.go`, add to the `Job` struct:

```go
Tier int `yaml:"tier"` // REQ-DTQ-13: must be present; 0 = missing = fatal error
```

Also add validation in the job loading function (wherever `Job` structs are unmarshalled/loaded — search for `JobRegistry` load loop):

```go
if job.Tier == 0 {
    return fmt.Errorf("job %q missing required 'tier' field (REQ-DTQ-13)", job.ID)
}
```

The error MUST propagate to a fatal startup error (not just a log warning).

- [ ] **Step 2: Write failing test**

```go
// In internal/game/ruleset/ (follow existing test pattern)
func TestJob_MissingTier_FatalError(t *testing.T) {
    // Attempt to load a Job YAML with no tier field
    // Assert error returned containing "missing required 'tier' field"
}

func TestJob_TierField_LoadsCorrectly(t *testing.T) {
    yamlContent := `id: test_job
name: Test Job
archetype: aggressor
tier: 2
key_ability: brutality
hit_points_per_level: 8
`
    // Unmarshal and assert job.Tier == 2
}
```

- [ ] **Step 2a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/game/ruleset/... -run TestJob_.*Tier -v
```

Expected: FAIL — `Tier` field not yet defined.

- [ ] **Step 3: Implement `Tier` field and fatal-load validation**

Add the `Tier int` field to the Job struct as shown in Step 1. Add the `Tier == 0` validation to the job registry load loop.

- [ ] **Step 4: Add `tier` to all 76 job YAML files**

Each job must have a `tier: N` line added. Assign tiers based on the spec's conceptual model (entry-level = 1, mid-tier = 2). Use the following convention as a starting point — verify against lore/balance docs if they exist:

- **Tier 1** (entry-level, unskilled): goon, thug, street_kid, drifter, and similar low-complexity jobs
- **Tier 2** (skilled, mid-career): mercenary, thief, medic, mechanic, and similar
- **Tier 3** (specialist, high-tier): assassin, hacker, fixer, and similar

Search `content/jobs/` for the full list. For each YAML file, add `tier: 1`, `tier: 2`, or `tier: 3` after the `archetype` line. Use tier 1 as the safe default if uncertain — the tier value affects queue capacity only and can be tuned later.

```bash
# Verify all 76 job files have been updated:
grep -rL "^tier:" content/jobs/
# Expected: no output (all files have tier)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/game/ruleset/... -run TestJob_.*Tier -v
mise exec -- go build ./...
```

Expected: all PASS; build succeeds.

- [ ] **Step 6: Commit**

```bash
git add internal/game/ruleset/job.go content/jobs/
git commit -m "feat(downtime-queue): add tier field to Job struct; update all 76 job YAMLs (REQ-DTQ-13)"
```

---

### Task 2: Queue limit registry

**Files:**
- Add: `content/downtime_queue_limits.yaml`
- Add: `internal/game/downtime/queue_limits.go`
- Add: `internal/game/downtime/queue_limits_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/downtime/queue_limits_test.go
package downtime_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/downtime"
    rapid "pgregory.net/rapid"
)

func TestDowntimeQueueLimitRegistry_Lookup_MatchFound(t *testing.T) {
    reg := downtime.NewDowntimeQueueLimitRegistryFromEntries([]downtime.QueueLimitEntry{
        {JobTier: 1, LevelMin: 1, LevelMax: 4, MaxQueue: 3},
        {JobTier: 1, LevelMin: 5, LevelMax: 8, MaxQueue: 5},
        {JobTier: 2, LevelMin: 1, LevelMax: 4, MaxQueue: 10},
    })
    assert.Equal(t, 3, reg.Lookup(1, 1))
    assert.Equal(t, 3, reg.Lookup(1, 4))
    assert.Equal(t, 5, reg.Lookup(1, 5))
    assert.Equal(t, 10, reg.Lookup(2, 1))
}

func TestDowntimeQueueLimitRegistry_Lookup_DefaultOnNoMatch(t *testing.T) {
    reg := downtime.NewDowntimeQueueLimitRegistryFromEntries(nil)
    assert.Equal(t, 3, reg.Lookup(99, 99)) // REQ-DTQ-14: default is 3
}

func TestDowntimeQueueLimitRegistry_Property_LookupAlwaysPositive(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        tier := rapid.IntRange(0, 10).Draw(t, "tier")
        level := rapid.IntRange(0, 20).Draw(t, "level")
        reg := downtime.NewDowntimeQueueLimitRegistryFromEntries(nil)
        result := reg.Lookup(tier, level)
        assert.GreaterOrEqual(t, result, 1)
    })
}

func TestDowntimeQueueLimitRegistry_LoadFromYAML(t *testing.T) {
    reg, err := downtime.LoadDowntimeQueueLimitRegistry("testdata/queue_limits_test.yaml")
    assert.NoError(t, err)
    assert.NotNil(t, reg)
}

func TestDowntimeQueueLimitRegistry_MissingFile_Error(t *testing.T) {
    _, err := downtime.LoadDowntimeQueueLimitRegistry("testdata/nonexistent.yaml")
    assert.Error(t, err)
}
```

- [ ] **Step 1a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/game/downtime/... -run TestDowntimeQueueLimit -v
```

Expected: FAIL — `DowntimeQueueLimitRegistry` not yet defined.

- [ ] **Step 2: Create `content/downtime_queue_limits.yaml`**

```yaml
limits:
  - job_tier: 1
    level_range: [1, 4]
    max_queue: 3
  - job_tier: 1
    level_range: [5, 8]
    max_queue: 5
  - job_tier: 1
    level_range: [9, 12]
    max_queue: 8
  - job_tier: 2
    level_range: [1, 4]
    max_queue: 10
  - job_tier: 2
    level_range: [5, 8]
    max_queue: 15
  - job_tier: 2
    level_range: [9, 12]
    max_queue: 20
  - job_tier: 3
    level_range: [1, 4]
    max_queue: 25
  - job_tier: 3
    level_range: [5, 8]
    max_queue: 40
  - job_tier: 3
    level_range: [9, 12]
    max_queue: 60
```

Also create `internal/game/downtime/testdata/queue_limits_test.yaml` as a copy of the above for tests.

- [ ] **Step 3: Implement `queue_limits.go`**

```go
// internal/game/downtime/queue_limits.go
package downtime

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// QueueLimitEntry maps a job tier + level range to a max queue size.
type QueueLimitEntry struct {
    JobTier    int    `yaml:"job_tier"`
    LevelMin   int    `yaml:"-"` // set from level_range[0]
    LevelMax   int    `yaml:"-"` // set from level_range[1]
    MaxQueue   int    `yaml:"max_queue"`
    LevelRange [2]int `yaml:"level_range"`
}

type queueLimitsYAML struct {
    Limits []struct {
        JobTier    int    `yaml:"job_tier"`
        LevelRange [2]int `yaml:"level_range"`
        MaxQueue   int    `yaml:"max_queue"`
    } `yaml:"limits"`
}

// DowntimeQueueLimitRegistry computes the downtime queue limit for a given job tier + level.
type DowntimeQueueLimitRegistry struct {
    entries []QueueLimitEntry
}

// NewDowntimeQueueLimitRegistryFromEntries builds a registry from a slice of entries.
// Used for testing.
func NewDowntimeQueueLimitRegistryFromEntries(entries []QueueLimitEntry) *DowntimeQueueLimitRegistry {
    return &DowntimeQueueLimitRegistry{entries: entries}
}

// LoadDowntimeQueueLimitRegistry loads the registry from a YAML file.
// Returns error if the file is missing or malformed (REQ-DTQ-15: caller must treat as fatal).
func LoadDowntimeQueueLimitRegistry(path string) (*DowntimeQueueLimitRegistry, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("loading queue limits YAML %q: %w", path, err)
    }
    var raw queueLimitsYAML
    if err := yaml.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("parsing queue limits YAML %q: %w", path, err)
    }
    entries := make([]QueueLimitEntry, 0, len(raw.Limits))
    for _, l := range raw.Limits {
        entries = append(entries, QueueLimitEntry{
            JobTier:  l.JobTier,
            LevelMin: l.LevelRange[0],
            LevelMax: l.LevelRange[1],
            MaxQueue: l.MaxQueue,
        })
    }
    return &DowntimeQueueLimitRegistry{entries: entries}, nil
}

// Lookup returns the max queue size for the given job tier and level.
// Returns 3 (default) if no matching entry is found (REQ-DTQ-14).
func (r *DowntimeQueueLimitRegistry) Lookup(jobTier int, level int) int {
    for _, e := range r.entries {
        if e.JobTier == jobTier && level >= e.LevelMin && level <= e.LevelMax {
            return e.MaxQueue
        }
    }
    return 3 // REQ-DTQ-14: default
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/game/downtime/... -run TestDowntimeQueueLimit -v
```

Expected: all PASS.

- [ ] **Step 5: Add `JobTier` and `DowntimeQueueLimit` to PlayerSession**

In `internal/game/session/manager.go`, add to `PlayerSession`:

```go
// Downtime queue — computed at login; not persisted.
JobTier            int // job.Tier from active job definition
DowntimeQueueLimit int // computed from JobTier + Level via DowntimeQueueLimitRegistry
```

At login in `grpc_service.go`, after loading the job:

```go
// Compute downtime queue limit from job tier + level (REQ-DTQ-1)
if activeJob, ok := s.jobRegistry.Job(sess.Class); ok {
    sess.JobTier = activeJob.Tier
} else {
    sess.JobTier = 1 // safe default
}
sess.DowntimeQueueLimit = s.downtimeQueueLimitRegistry.Lookup(sess.JobTier, sess.Level)
```

- [ ] **Step 6: Commit**

```bash
git add content/downtime_queue_limits.yaml internal/game/downtime/queue_limits.go internal/game/downtime/queue_limits_test.go internal/game/downtime/testdata/ internal/game/session/manager.go
git commit -m "feat(downtime-queue): add DowntimeQueueLimitRegistry and JobTier/DowntimeQueueLimit session fields"
```

---

### Task 3: DB migration and `CharacterDowntimeQueueRepository`

**Files:**
- Add: `internal/storage/postgres/migrations/<N>_add_character_downtime_queue.up.sql`
- Add: `internal/storage/postgres/migrations/<N>_add_character_downtime_queue.down.sql`
- Add: `internal/storage/postgres/character_downtime_queue.go`
- Add: `internal/storage/postgres/character_downtime_queue_test.go`

- [ ] **Step 1: Find migration number**

```bash
ls internal/storage/postgres/migrations/ | sort | tail -3
```

Use the next sequential number.

- [ ] **Step 2: Create migrations**

Up:
```sql
CREATE TABLE character_downtime_queue (
    id            bigserial PRIMARY KEY,
    character_id  bigint NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    position      int    NOT NULL,
    activity_id   text   NOT NULL,
    activity_args text,
    activity_metadata jsonb,
    UNIQUE (character_id, position)
);
```

Down:
```sql
DROP TABLE IF EXISTS character_downtime_queue;
```

- [ ] **Step 3: Run migration**

```bash
mise exec -- make migrate-up
```

- [ ] **Step 4: Write failing tests**

```go
// internal/storage/postgres/character_downtime_queue_test.go
func TestCharacterDowntimeQueue_EnqueueAndList(t *testing.T) {
    // Enqueue 3 entries; ListQueue; assert 3 rows in order (position 1,2,3)
}

func TestCharacterDowntimeQueue_RemoveAt_Reindexes(t *testing.T) {
    // Enqueue 3 entries; RemoveAt(2); ListQueue; assert 2 rows at positions 1,2
    // Assert original entry 3 is now at position 2 (REQ-DTQ-12)
}

func TestCharacterDowntimeQueue_PopHead_RemovesFirst(t *testing.T) {
    // Enqueue 3 entries; PopHead; assert returned entry = first; ListQueue has 2 rows
    // Assert positions reindexed to 1,2 (REQ-DTQ-12)
}

func TestCharacterDowntimeQueue_PopHead_EmptyQueue_NilResult(t *testing.T) {
    // Empty queue; PopHead; assert nil returned, no error
}

func TestCharacterDowntimeQueue_Clear(t *testing.T) {
    // Enqueue 3 entries; Clear; ListQueue; assert 0 rows
}
```

- [ ] **Step 4a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/storage/postgres/... -run TestCharacterDowntimeQueue -v
```

Expected: FAIL — repository not yet implemented.

- [ ] **Step 5: Implement `character_downtime_queue.go`**

```go
// internal/storage/postgres/character_downtime_queue.go
package postgres

import (
    "context"
    "database/sql"
    "encoding/json"
)

// QueueEntry is a single entry in the downtime activity queue.
type QueueEntry struct {
    ID               int64
    CharacterID      int64
    Position         int
    ActivityID       string
    ActivityArgs     string
    ActivityMetadata json.RawMessage
}

// CharacterDowntimeQueueRepository manages per-character downtime activity queues.
type CharacterDowntimeQueueRepository struct {
    db *sql.DB
}

func NewCharacterDowntimeQueueRepository(db *sql.DB) *CharacterDowntimeQueueRepository {
    return &CharacterDowntimeQueueRepository{db: db}
}

// Enqueue adds an entry at the end of the queue (next position = max(position)+1).
func (r *CharacterDowntimeQueueRepository) Enqueue(ctx context.Context, characterID int64, entry QueueEntry) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO character_downtime_queue (character_id, position, activity_id, activity_args)
        VALUES ($1,
            COALESCE((SELECT MAX(position) FROM character_downtime_queue WHERE character_id = $1), 0) + 1,
            $2, $3)`,
        characterID, entry.ActivityID, entry.ActivityArgs)
    return err
}

// ListQueue returns all entries for a character ordered by position.
func (r *CharacterDowntimeQueueRepository) ListQueue(ctx context.Context, characterID int64) ([]QueueEntry, error) {
    rows, err := r.db.QueryContext(ctx,
        `SELECT id, character_id, position, activity_id, activity_args FROM character_downtime_queue
         WHERE character_id = $1 ORDER BY position`, characterID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var entries []QueueEntry
    for rows.Next() {
        var e QueueEntry
        var args sql.NullString
        if err := rows.Scan(&e.ID, &e.CharacterID, &e.Position, &e.ActivityID, &args); err != nil {
            return nil, err
        }
        e.ActivityArgs = args.String
        entries = append(entries, e)
    }
    return entries, rows.Err()
}

// RemoveAt deletes the entry at the given position and reindexes positions above it.
// Executes within a single transaction (REQ-DTQ-12).
func (r *CharacterDowntimeQueueRepository) RemoveAt(ctx context.Context, characterID int64, position int) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    if _, err := tx.ExecContext(ctx,
        `DELETE FROM character_downtime_queue WHERE character_id = $1 AND position = $2`,
        characterID, position); err != nil {
        return err
    }
    if _, err := tx.ExecContext(ctx,
        `UPDATE character_downtime_queue SET position = position - 1
         WHERE character_id = $1 AND position > $2`,
        characterID, position); err != nil {
        return err
    }
    return tx.Commit()
}

// Clear removes all queue entries for a character (REQ-DTQ-16: does NOT touch character_downtime).
func (r *CharacterDowntimeQueueRepository) Clear(ctx context.Context, characterID int64) error {
    _, err := r.db.ExecContext(ctx,
        `DELETE FROM character_downtime_queue WHERE character_id = $1`, characterID)
    return err
}

// PopHead atomically removes position 1 and reindexes remaining entries.
// Returns nil if the queue is empty (REQ-DTQ-12).
func (r *CharacterDowntimeQueueRepository) PopHead(ctx context.Context, characterID int64) (*QueueEntry, error) {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    row := tx.QueryRowContext(ctx,
        `SELECT id, character_id, position, activity_id, activity_args
         FROM character_downtime_queue WHERE character_id = $1 AND position = 1`,
        characterID)
    var e QueueEntry
    var args sql.NullString
    if err := row.Scan(&e.ID, &e.CharacterID, &e.Position, &e.ActivityID, &args); err == sql.ErrNoRows {
        tx.Commit()
        return nil, nil
    } else if err != nil {
        return nil, err
    }
    e.ActivityArgs = args.String
    if _, err := tx.ExecContext(ctx,
        `DELETE FROM character_downtime_queue WHERE id = $1`, e.ID); err != nil {
        return nil, err
    }
    if _, err := tx.ExecContext(ctx,
        `UPDATE character_downtime_queue SET position = position - 1 WHERE character_id = $1`,
        characterID); err != nil {
        return nil, err
    }
    return &e, tx.Commit()
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/storage/postgres/... -run TestCharacterDowntimeQueue -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/storage/postgres/migrations/ internal/storage/postgres/character_downtime_queue.go internal/storage/postgres/character_downtime_queue_test.go
git commit -m "feat(downtime-queue): add character_downtime_queue table and repository (REQ-DTQ-12)"
```

---

### Task 4: `StartNext` engine extension

**Files:**
- Modify: `internal/game/downtime/engine.go`
- Add: `internal/gameserver/start_next_test.go`

`StartNext` is called by `checkDowntimeCompletion` after a successful resolve, and by the reconnect resume loop. It owns exactly one `PopHead` call per invocation; if the popped entry is invalid, it calls itself recursively (REQ-DTQ-10).

- [ ] **Step 1: Write failing tests**

```go
// internal/gameserver/start_next_test.go
package gameserver_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

// Tests use the same gameserver test harness as other handler tests.

func TestStartNext_EmptyQueue_NotifiesPlayer(t *testing.T) {
    // Mock PopHead returns nil (empty queue)
    // Assert StartNext sends "queue is empty" notification to player
    // Assert no activity started (sess.DowntimeBusy == false)
}

func TestStartNext_ValidActivity_StartsIt(t *testing.T) {
    // Mock PopHead returns earn_creds entry
    // Room has "safe" tag
    // Assert sess.DowntimeBusy == true; sess.DowntimeActivityID == "earn_creds"
    // Assert player notified "Starting: Earn Creds."
}

func TestStartNext_InvalidRoom_SkipsToNext(t *testing.T) {
    // First PopHead: earn_creds; room missing "safe" tag → skip
    // Second PopHead (recursive call): returns nil (queue empty)
    // Assert player notified about skip; not busy
}

func TestStartNext_CraftInsufficientMaterials_SkipsToNext(t *testing.T) {
    // First PopHead: craft; DeductMany fails → skip
    // Second PopHead (recursive call): returns earn_creds with valid room
    // Assert earn_creds started; busy == true
}
```

- [ ] **Step 1a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run TestStartNext -v
```

Expected: FAIL — `startNext` not yet defined.

- [ ] **Step 2: Add `StartNext` signature to `engine.go`**

`StartNext` is implemented in `grpc_service.go` (where session, DB, world, and crafting engine are accessible) and calls `PopHead` on the queue repo. The engine.go file documents the contract:

```go
// internal/game/downtime/engine.go (add these interfaces and doc comment)

// QueuePopper abstracts PopHead for testing.
type QueuePopper interface {
    PopHead(ctx context.Context, characterID int64) (*postgres.QueueEntry, error)
}

// StartNextFn is the signature for the auto-start function called after activity completion.
// Implementations live in grpc_service.go (has access to session, world, DB, crafting engine).
// The function pops from the queue, validates, and starts the next eligible activity.
// It calls itself recursively if the popped entry is invalid (REQ-DTQ-10).
// Termination is guaranteed because PopHead removes one item per invocation.
type StartNextFn func(uid string)
```

Note: Because `StartNext` needs session, world, crafting engine, and the queue repo — all stored on `GameServiceServer` — implement it as a method `(s *GameServiceServer) startNext(uid string)` in `grpc_service.go`. The engine package defines the interfaces for documentation and test purposes only.

- [ ] **Step 2a: Verify `downtimePreStartCraft` exists from `downtime` plan**

```bash
grep -n "downtimePreStartCraft" internal/gameserver/grpc_service.go
```

Expected: at least one definition line. If absent, the `downtime` plan has not been executed — execute it first before continuing.

- [ ] **Step 3: Implement `startNext` in `grpc_service.go`**

```go
// startNext pops and starts the next eligible queued activity for uid.
// Calls itself recursively if the head item is invalid (REQ-DTQ-10).
// Termination guaranteed: PopHead removes one item per call; queue is finite.
func (s *GameServiceServer) startNext(uid string) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return
    }

    entry, err := s.downtimeQueueRepo.PopHead(context.Background(), sess.CharacterID)
    if err != nil {
        s.logger.Error("startNext: PopHead error", "err", err)
        return
    }
    if entry == nil {
        s.sendConsole(uid, "Downtime complete. Your queue is empty.") // REQ-DTQ-10 termination
        return
    }

    act, ok := downtime.ActivityByID(entry.ActivityID)
    if !ok {
        s.sendConsole(uid, fmt.Sprintf("Skipped: unknown activity %q.", entry.ActivityID))
        s.startNext(uid) // recursive: next call pops next item
        return
    }

    // Validate room tags (REQ-DTQ-7)
    room, roomOK := s.world.GetRoom(sess.RoomID)
    if !roomOK {
        s.sendConsole(uid, fmt.Sprintf("Skipped: %s — cannot determine room.", act.Name))
        s.startNext(uid)
        return
    }
    roomTags := room.Properties["tags"]
    if errMsg := downtime.CanStart(act.Alias, roomTags, false); errMsg != "" {
        s.sendConsole(uid, fmt.Sprintf("Skipped: %s — %s", act.Name, errMsg))
        s.startNext(uid)
        return
    }

    // For Craft: deduct materials now (REQ-DTQ-8)
    metadata := entry.ActivityArgs
    if entry.ActivityID == "craft" {
        metadata, err = s.downtimePreStartCraft(uid, sess, entry.ActivityArgs)
        if err != nil {
            s.sendConsole(uid, fmt.Sprintf("Skipped: %s — %s", act.Name, err.Error()))
            s.startNext(uid) // REQ-DTQ-9
            return
        }
    }

    // Start the activity
    completesAt := time.Now().Add(time.Duration(act.DurationMinutes) * time.Minute)
    sess.DowntimeActivityID = act.ID
    sess.DowntimeCompletesAt = completesAt
    sess.DowntimeBusy = true
    sess.DowntimeMetadata = metadata

    if s.downtimeRepo != nil {
        state := postgres.DowntimeState{
            ActivityID:  act.ID,
            CompletesAt: completesAt,
            RoomID:      sess.RoomID,
            Metadata:    metadata,
        }
        _ = s.downtimeRepo.Save(context.Background(), sess.CharacterID, state)
    }

    s.sendConsole(uid, fmt.Sprintf("Starting: %s.", act.Name))
}
```

Update `checkDowntimeCompletion` to call `startNext` after resolving:

```go
func (s *GameServiceServer) checkDowntimeCompletion(uid string) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok || !sess.DowntimeBusy {
        return
    }
    if time.Now().Before(sess.DowntimeCompletesAt) {
        return
    }
    s.resolveDowntimeActivity(uid, sess)
    s.startNext(uid) // REQ-DTQ: auto-start next queued activity
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run TestStartNext -v
mise exec -- go test ./...
```

Also add to `git add` in Step 5:

```bash
git add internal/gameserver/start_next_test.go
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game/downtime/engine.go internal/gameserver/start_next_test.go internal/gameserver/grpc_service.go
git commit -m "feat(downtime-queue): add startNext engine; auto-start next queued activity on completion"
```

---

### Task 5: Queue subcommands in `handleDowntime`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

All queue subcommands use the existing `DowntimeRequest { subcommand, args }` proto. When `subcommand == "queue"`, dispatch on the first token of `args`.

- [ ] **Step 1: Write failing tests**

```go
func TestHandleDowntime_QueueAdd_Success(t *testing.T) {
    // Room has "safe" tag; queue empty; limit > 0
    // Send DowntimeRequest{Subcommand: "queue", Args: "earn"}
    // Assert queue has 1 entry
}

func TestHandleDowntime_QueueAdd_AtLimit_Fails(t *testing.T) {
    // sess.DowntimeQueueLimit = 1; queue already has 1 entry
    // Send DowntimeRequest{Subcommand: "queue", Args: "earn"}
    // Assert error: "Your downtime queue is full." (REQ-DTQ-1)
}

func TestHandleDowntime_QueueAdd_InvalidAlias_Fails(t *testing.T) {
    // Send DowntimeRequest{Subcommand: "queue", Args: "notanactivity"}
    // Assert error (REQ-DTQ-2)
}

func TestHandleDowntime_QueueAdd_CraftMissingRecipe_Fails(t *testing.T) {
    // Send DowntimeRequest{Subcommand: "queue", Args: "craft nonexistent_recipe"}
    // Assert error (REQ-DTQ-3)
}

func TestHandleDowntime_QueueList_ShowsEstimatedTimes(t *testing.T) {
    // Queue has 2 entries
    // Send DowntimeRequest{Subcommand: "queue", Args: "list"}
    // Assert response contains position numbers and time estimates (REQ-DTQ-6)
}

func TestHandleDowntime_QueueRemove_Reindexes(t *testing.T) {
    // Queue has 3 entries; remove position 2
    // Assert queue has 2 entries at positions 1,2
}

func TestHandleDowntime_QueueClear_OnlyClearsQueue(t *testing.T) {
    // Active activity + 2 queued; send clear
    // Assert queued entries gone; active activity still running (REQ-DTQ-16)
}
```

- [ ] **Step 1a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleDowntime_Queue -v
```

Expected: FAIL — queue subcommands not yet implemented.

- [ ] **Step 2: Implement queue dispatch in `handleDowntime`**

At the top of the `handleDowntime` switch, add before the existing subcommand cases:

```go
// Queue subcommand
if sub == "queue" {
    return s.handleDowntimeQueue(uid, sess, req.GetArgs())
}
```

Implement `handleDowntimeQueue`:

```go
func (s *GameServiceServer) handleDowntimeQueue(uid string, sess *session.PlayerSession, args string) error {
    parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
    queueSub := ""
    queueArgs := ""
    if len(parts) >= 1 {
        queueSub = strings.ToLower(parts[0])
    }
    if len(parts) >= 2 {
        queueArgs = parts[1]
    }

    switch queueSub {
    case "list":
        return s.handleDowntimeQueueList(uid, sess)
    case "clear":
        return s.handleDowntimeQueueClear(uid, sess) // REQ-DTQ-16: queue only
    case "remove":
        pos := 0
        fmt.Sscanf(queueArgs, "%d", &pos)
        if pos < 1 {
            return s.sendError(uid, "Usage: downtime queue remove <position>")
        }
        if err := s.downtimeQueueRepo.RemoveAt(context.Background(), sess.CharacterID, pos); err != nil {
            return s.sendError(uid, "Failed to remove queue entry.")
        }
        return s.sendConsole(uid, fmt.Sprintf("Queue position %d removed.", pos))
    default:
        // Treat queueSub as activity alias; queueArgs as activity args
        return s.handleDowntimeQueueAdd(uid, sess, queueSub, queueArgs)
    }
}
```

Implement `handleDowntimeQueueAdd`:

```go
func (s *GameServiceServer) handleDowntimeQueueAdd(uid string, sess *session.PlayerSession, alias, activityArgs string) error {
    // REQ-DTQ-1: check limit
    entries, err := s.downtimeQueueRepo.ListQueue(context.Background(), sess.CharacterID)
    if err != nil {
        return s.sendError(uid, "Failed to read queue.")
    }
    if len(entries) >= sess.DowntimeQueueLimit {
        return s.sendError(uid, fmt.Sprintf("Your downtime queue is full (%d/%d).", len(entries), sess.DowntimeQueueLimit))
    }

    // REQ-DTQ-2: validate alias
    act, ok := downtime.ActivityByAlias(alias)
    if !ok {
        return s.sendError(uid, fmt.Sprintf("Unknown downtime activity %q.", alias))
    }

    // REQ-DTQ-3: craft recipe must exist
    if act.ID == "craft" {
        if _, ok := s.recipeRegistry.Recipe(activityArgs); !ok {
            return s.sendError(uid, fmt.Sprintf("Recipe %q not found.", activityArgs))
        }
    }

    // REQ-DTQ-4: analyze/repair warn (not fail) if item not in inventory
    if act.ID == "analyze_tech" || act.ID == "field_repair" {
        if len(sess.Backpack.FindByItemDefID(activityArgs)) == 0 {
            s.sendConsole(uid, fmt.Sprintf("Warning: %q not currently in inventory; will be re-checked at start time.", activityArgs))
        }
    }

    // REQ-DTQ-5: do NOT validate room tags at queue time

    entry := postgres.QueueEntry{
        ActivityID:   act.ID,
        ActivityArgs: activityArgs,
    }
    if err := s.downtimeQueueRepo.Enqueue(context.Background(), sess.CharacterID, entry); err != nil {
        return s.sendError(uid, "Failed to add to queue.")
    }
    return s.sendConsole(uid, fmt.Sprintf("Queued: %s (position %d).", act.Name, len(entries)+1))
}
```

Helper used in queue list and reconnect — add to `grpc_service.go`:

```go
// complexityToMinutes converts a recipe complexity (1–4) to real-time duration in minutes.
// Complexity 1=2h, 2=4h, 3=6h, 4=8h (per downtime spec).
func complexityToMinutes(complexity int) int {
    switch complexity {
    case 1:
        return 120
    case 2:
        return 240
    case 3:
        return 360
    case 4:
        return 480
    default:
        return 240 // default to 4h
    }
}
```

Note: Verify the `RecipeRegistry` lookup method name matches the crafting plan before execution. The plan uses `s.recipeRegistry.Recipe(id) (*Recipe, bool)` — if the actual method is named `Get`, `Lookup`, or `Find`, update all call sites.

Implement `handleDowntimeQueueList` — display estimated start and end times:

```go
func (s *GameServiceServer) handleDowntimeQueueList(uid string, sess *session.PlayerSession) error {
    entries, err := s.downtimeQueueRepo.ListQueue(context.Background(), sess.CharacterID)
    if err != nil {
        return s.sendError(uid, "Failed to read queue.")
    }
    if len(entries) == 0 {
        return s.sendConsole(uid, "Your downtime queue is empty.")
    }

    // Baseline: active activity completion time, or now if none (REQ-DTQ-6)
    baseline := time.Now()
    if sess.DowntimeBusy && !sess.DowntimeCompletesAt.IsZero() {
        baseline = sess.DowntimeCompletesAt
    }

    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("--- Downtime Queue (%d/%d) ---\n", len(entries), sess.DowntimeQueueLimit))
    cursor := baseline
    for _, e := range entries {
        act, ok := downtime.ActivityByID(e.ActivityID)
        if !ok {
            sb.WriteString(fmt.Sprintf("  %d. %-20s start:unknown  end:unknown\n", e.Position, e.ActivityID))
            continue
        }
        durationMin := act.DurationMinutes
        if e.ActivityID == "craft" {
            if recipe, ok := s.recipeRegistry.Recipe(e.ActivityArgs); ok {
                durationMin = complexityToMinutes(recipe.Complexity)
            } else {
                sb.WriteString(fmt.Sprintf("  %d. %-20s start:%s  end:unknown\n", e.Position, act.Name, cursor.Format("15:04")))
                // can't compute cursor without duration; mark rest as unknown
                sb.WriteString("  (subsequent times unknown — recipe not found)\n")
                break
            }
        }
        start := cursor
        end := cursor.Add(time.Duration(durationMin) * time.Minute)
        sb.WriteString(fmt.Sprintf("  %d. %-20s start:%s  end:%s\n", e.Position, act.Name, start.Format("15:04"), end.Format("15:04")))
        cursor = end
    }
    return s.sendConsole(uid, sb.String())
}
```

Implement `handleDowntimeQueueClear`:

```go
func (s *GameServiceServer) handleDowntimeQueueClear(uid string, sess *session.PlayerSession) error {
    if err := s.downtimeQueueRepo.Clear(context.Background(), sess.CharacterID); err != nil {
        return s.sendError(uid, "Failed to clear queue.")
    }
    return s.sendConsole(uid, "Downtime queue cleared. Active activity (if any) is unaffected.") // REQ-DTQ-16
}
```

- [ ] **Step 3: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleDowntime_Queue -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(downtime-queue): implement downtime queue subcommands (add/list/remove/clear)"
```

---

### Task 6: Reconnect offline resolution (REQ-DTQ-11)

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (Session() reconnect block)

Extends the existing downtime reconnect resume (downtime plan Task 8) to also resolve offline-completed queued activities.

- [ ] **Step 1: Write failing tests**

```go
func TestDowntimeReconnect_ResolvesQueuedOfflineActivities(t *testing.T) {
    // Active activity completed offline; queue has 2 entries that also elapsed
    // Reconnect: assert 3 activities resolved with "(offline)" markers
    // Assert summary "2 activities completed, 0 skipped" (REQ-DTQ-11)
}

func TestDowntimeReconnect_SkipsInvalidRoomInQueue(t *testing.T) {
    // Active completes offline; queued earn_creds requires "safe" tag but room lacks it
    // Reconnect: assert earn_creds skipped; summary shows 0 completed, 1 skipped (REQ-DTQ-11)
}

func TestDowntimeReconnect_StartsRemainingQueuedActivity(t *testing.T) {
    // Active completes offline; queue has 2 entries; first elapsed, second in future
    // Reconnect: first resolved offline; second started as new active activity
    // Assert sess.DowntimeBusy == true; DowntimeActivityID == second activity
}
```

- [ ] **Step 1a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run TestDowntimeReconnect -v
```

Expected: FAIL — offline queue resolution not yet implemented.

- [ ] **Step 2: Implement offline queue resolution in reconnect block**

After the existing downtime reconnect resume (which resolves the active activity if elapsed), add:

```go
// REQ-DTQ-11: resolve queued activities that completed while offline.
// Read-first approach: ListQueue for hypothetical time computation (non-destructive),
// then process each entry in order — using PopHead to consume entries already decided.
// This avoids permanently dropping future-but-invalid entries that failed room validation.
queueEntries, _ := s.downtimeQueueRepo.ListQueue(context.Background(), characterID)

cursor := sess.DowntimeCompletesAt // baseline = when active activity completed
if cursor.IsZero() {
    cursor = time.Now()
}

room, roomOK := s.world.GetRoom(sess.RoomID)
roomTags := ""
if roomOK {
    roomTags = room.Properties["tags"]
}

completed := 0
skipped := 0
newActiveStarted := false

for _, entry := range queueEntries {
    act, ok := downtime.ActivityByID(entry.ActivityID)
    if !ok {
        // Pop and discard unknown activity
        _, _ = s.downtimeQueueRepo.PopHead(context.Background(), characterID)
        s.sendConsole(uid, fmt.Sprintf("Skipped (offline): unknown activity %q.", entry.ActivityID))
        skipped++
        continue
    }

    durationMin := act.DurationMinutes
    if entry.ActivityID == "craft" {
        if recipe, ok := s.recipeRegistry.Recipe(entry.ActivityArgs); ok {
            durationMin = complexityToMinutes(recipe.Complexity)
        }
    }
    hypotheticalEnd := cursor.Add(time.Duration(durationMin) * time.Minute)

    if time.Now().Before(hypotheticalEnd) {
        // Activity hasn't completed yet — attempt to start as new active
        if errMsg := downtime.CanStart(act.Alias, roomTags, false); errMsg != "" {
            // Room invalid for this future candidate — skip and try next (per spec 3.1 step 3c)
            _, _ = s.downtimeQueueRepo.PopHead(context.Background(), characterID)
            s.sendConsole(uid, fmt.Sprintf("Skipped (offline): %s — %s", act.Name, errMsg))
            skipped++
            cursor = hypotheticalEnd // advance baseline so next item's time is correct
            continue
        }
        // Room valid: start this as the new active activity; stop loop
        _, _ = s.downtimeQueueRepo.PopHead(context.Background(), characterID)
        completesAt := hypotheticalEnd
        sess.DowntimeActivityID = act.ID
        sess.DowntimeCompletesAt = completesAt
        sess.DowntimeBusy = true
        sess.DowntimeMetadata = entry.ActivityArgs
        state := postgres.DowntimeState{
            ActivityID:  act.ID,
            CompletesAt: completesAt,
            RoomID:      sess.RoomID,
            Metadata:    entry.ActivityArgs,
        }
        _ = s.downtimeRepo.Save(context.Background(), characterID, state)
        newActiveStarted = true
        break
    }

    // Activity elapsed offline — validate and resolve
    _, _ = s.downtimeQueueRepo.PopHead(context.Background(), characterID)
    if errMsg := downtime.CanStart(act.Alias, roomTags, false); errMsg != "" {
        s.sendConsole(uid, fmt.Sprintf("Skipped (offline): %s — %s", act.Name, errMsg))
        skipped++
        cursor = hypotheticalEnd
        continue
    }

    sess.DowntimeActivityID = act.ID
    sess.DowntimeBusy = true
    sess.DowntimeMetadata = entry.ActivityArgs
    sess.DowntimeCompletesAt = hypotheticalEnd
    s.resolveDowntimeActivity(uid, sess) // sends result message
    s.sendConsole(uid, "(offline)")      // REQ-DTQ-11: offline marker
    completed++
    cursor = hypotheticalEnd
}
_ = newActiveStarted // used implicitly by loop break

if completed > 0 || skipped > 0 {
    s.sendConsole(uid, fmt.Sprintf("While you were away: %d activities completed, %d skipped.", completed, skipped))
}
```

- [ ] **Step 3: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run TestDowntimeReconnect -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(downtime-queue): resolve offline-completed queued activities on reconnect (REQ-DTQ-11)"
```

---

### Task 7: Wire integration

**Files:**
- Modify: `internal/gameserver/deps.go`
- Modify: `cmd/gameserver/wire.go`

- [ ] **Step 1: Add deps**

In `internal/gameserver/deps.go`:

Add `CharacterDowntimeQueueRepository` interface:
```go
type CharacterDowntimeQueueRepository interface {
    Enqueue(ctx context.Context, characterID int64, entry postgres.QueueEntry) error
    ListQueue(ctx context.Context, characterID int64) ([]postgres.QueueEntry, error)
    RemoveAt(ctx context.Context, characterID int64, position int) error
    Clear(ctx context.Context, characterID int64) error
    PopHead(ctx context.Context, characterID int64) (*postgres.QueueEntry, error)
}
```

Add to `StorageDeps`:
```go
DowntimeQueueRepo CharacterDowntimeQueueRepository
```

Add `DowntimeQueueLimitRegistryProvider` to `ContentDeps`:
```go
DowntimeQueueLimitRegistry *downtime.DowntimeQueueLimitRegistry
```

- [ ] **Step 2: Wire providers**

In `cmd/gameserver/wire.go`:

1. Add `postgres.NewCharacterDowntimeQueueRepository(db)` as a provider.
2. Add `wire.Bind(new(gameserver.CharacterDowntimeQueueRepository), new(*postgres.CharacterDowntimeQueueRepository))`.
3. For the limit registry: load from file in the `Initialize()` function:

```go
queueLimitRegistry, err := downtime.LoadDowntimeQueueLimitRegistry("content/downtime_queue_limits.yaml")
if err != nil {
    log.Fatalf("fatal: downtime queue limits: %v", err) // REQ-DTQ-15
}
```

Pass to ContentDeps.

- [ ] **Step 3: Run wire and build**

```bash
mise exec -- wire ./cmd/gameserver/...
mise exec -- go build ./...
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/gameserver/wire.go cmd/gameserver/wire_gen.go internal/gameserver/deps.go
git commit -m "feat(downtime-queue): wire CharacterDowntimeQueueRepository and DowntimeQueueLimitRegistry"
```

---

## Verification Checklist

- [ ] `downtime queue earn` fails if queue is at `DowntimeQueueLimit` (REQ-DTQ-1)
- [ ] `downtime queue notanactivity` fails with unknown alias error (REQ-DTQ-2)
- [ ] `downtime queue craft nonexistent` fails if recipe not found (REQ-DTQ-3)
- [ ] `downtime queue analyze item_not_in_bag` warns (does not fail) (REQ-DTQ-4)
- [ ] Room tags are NOT checked at queue time (REQ-DTQ-5)
- [ ] `downtime queue list` shows estimated start and end times per activity (REQ-DTQ-6)
- [ ] `startNext` validates room tags before starting each queued activity (REQ-DTQ-7)
- [ ] Craft materials deducted at start time (via `downtimePreStartCraft`), not at queue time (REQ-DTQ-8)
- [ ] Insufficient Craft materials at start time: activity skipped, next attempted (REQ-DTQ-9)
- [ ] `startNext` recursion terminates: each call pops exactly one item; base case is nil from PopHead (REQ-DTQ-10)
- [ ] Reconnect resolves all elapsed queued activities in order with "(offline)" marker + summary (REQ-DTQ-11)
- [ ] `PopHead` and `RemoveAt` reindex positions within a single DB transaction (REQ-DTQ-12)
- [ ] All 76 job YAMLs have `tier:` field; missing tier causes fatal startup error (REQ-DTQ-13)
- [ ] `DowntimeQueueLimitRegistry.Lookup` returns 3 when no entry matches (REQ-DTQ-14)
- [ ] Missing `content/downtime_queue_limits.yaml` causes fatal startup error (REQ-DTQ-15)
- [ ] `downtime queue clear` removes only queued entries; active activity unaffected (REQ-DTQ-16)
- [ ] Full test suite passes with zero failures
