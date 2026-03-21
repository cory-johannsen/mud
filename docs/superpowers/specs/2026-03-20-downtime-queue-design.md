# Downtime Activity Queue — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `downtime-queue` (priority 238)
**Dependencies:** `downtime`

---

## Overview

Extends the single-activity downtime system to allow players to queue up to 100 activities upfront. When the active activity completes, the engine automatically starts the next eligible queued activity without player intervention. The queue is DB-persisted and survives disconnects.

---

## 1. Data Model

### 1.1 Queue Table

```sql
character_downtime_queue (
    id                bigserial PRIMARY KEY,
    character_id      bigint NOT NULL REFERENCES characters(id),
    position          int NOT NULL,
    activity_id       text NOT NULL,
    activity_args     text,          -- recipe ID, target name, item name, NPC name, etc.
    activity_metadata jsonb,         -- NULL in the queue; populated by StartNext before writing character_downtime
    UNIQUE (character_id, position)
)
```

`activity_metadata` is always NULL in queue rows. When `StartNext` promotes a queue entry to the active activity, it generates the metadata (e.g., resolving item names to IDs, NPC lookups) and writes it to `character_downtime.activity_metadata`, not back to the queue row.

Position is 1-based and always contiguous (1..N with no gaps). On remove or pop, all positions above the removed row are decremented by 1 within the same DB transaction, maintaining the 1..N invariant.

### 1.2 Queue Limit

Queue limit is determined by a static lookup table keyed by job tier and character level. Job tier is defined by a `tier int` field on the job YAML definition (e.g., `tier: 1` for entry-level jobs, `tier: 2` for mid-tier, etc.). `PlayerSession.JobTier int` is set at login from the character's active job definition.

Limit table stored in `content/downtime_queue_limits.yaml`:

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
  # ... tiers and levels scale up to max_queue: 100
```

`DowntimeQueueLimitRegistry` loads this file at startup. `PlayerSession.DowntimeQueueLimit int` is computed at login via `DowntimeQueueLimitRegistry.Lookup(jobTier int, level int) int` and refreshed on level-up or job change. If no matching entry is found for the character's tier/level, a default of 3 applies.

### 1.3 PlayerSession Fields

`PlayerSession` gains:

```go
DowntimeQueueLimit int  // computed at login from job tier + level; not persisted
JobTier            int  // read from active job YAML definition's `tier` field at login
```

Job YAML definitions gain a required `tier int` field. All existing job definitions MUST be updated to include `tier`. Jobs without a `tier` field MUST cause a fatal load error at startup.

### 1.4 Material Deduction

Materials are NOT reserved at queue time. They are deducted at activity start time (when the queued activity becomes the active activity), using the same `DeductMany` transaction as a direct `downtime craft` confirm. If materials are insufficient at start time, the activity is skipped with a notification and the next queue item is attempted.

- REQ-DTQ-8: Materials for queued Craft activities MUST be deducted at activity start time, not at queue time.
- REQ-DTQ-9: If a queued Craft activity cannot deduct materials at start time, it MUST be skipped with a player notification and the next queued activity MUST be attempted.

---

## 2. Commands

```
downtime queue <alias> [args]     — add activity to end of queue
downtime queue list               — show queued activities with position and estimated times
downtime queue remove <position>  — remove activity at position N; higher positions shift down
downtime queue clear              — cancel all queued (not yet started) activities; no material refund
```

`downtime queue clear` affects only the queue rows in `character_downtime_queue`. It does NOT cancel the currently active activity (if any). To cancel the active activity, use `downtime cancel`.

Queue aliases match the existing `downtime <alias>` aliases exactly (earn, craft, retrain, sickness, subsist, forge, recalibrate, patchup, flushit, intel, analyze, repair, decode, cover, pressure).

### 2.1 Validation at Queue Time

- REQ-DTQ-1: `downtime queue <alias> [args]` MUST fail if the queue is at `DowntimeQueueLimit`.
- REQ-DTQ-2: `downtime queue <alias> [args]` MUST fail if the activity alias or args are invalid.
- REQ-DTQ-3: `downtime queue craft <recipe>` MUST fail if the recipe does not exist in the recipe registry.
- REQ-DTQ-4: `downtime queue analyze <item>` and `downtime queue repair <item>` MUST warn (not fail) if the item is not currently in inventory; the item is re-validated at activity start time.
- REQ-DTQ-5: Room tag requirements are NOT validated at queue time; they are validated at activity start time.

### 2.2 Estimated Times in `downtime queue list`

Estimated start and end times for each queued activity are computed by summing durations of all preceding activities. The baseline is:
- If an activity is currently active: `DowntimeCompletesAt` of the active activity.
- If no activity is active: `gameClock.Now()` at the time of the command.

Durations are looked up from the activity registry at display time (not stored in queue rows). For Craft activities, the recipe ID from `activity_args` is used to look up the recipe's complexity and thus its duration. If the recipe no longer exists (deleted after queuing), display duration as "unknown" and estimated times for subsequent activities as "unknown" as well. Display format: position, activity name, estimated start time (in-game clock), estimated end time (in-game clock).

- REQ-DTQ-6: `downtime queue list` MUST display estimated start and end times for each queued activity based on cumulative preceding durations looked up from the activity and recipe registries at display time.

---

## 3. Auto-start Engine

On `CheckCompletion()` successfully resolving an active activity:

1. Resolve the completed activity (skill check, side effects, player notification per downtime spec).
2. Clear `character_downtime` row; reset `PlayerSession` downtime fields (`DowntimeActivityID = ""`, `DowntimeBusy = false`, `DowntimeCompletesAt = zero`).
3. Call `StartNext(sess, gameClock)`.

`StartNext(sess, gameClock)`:

Each call to `StartNext` owns exactly one `PopHead` call at the start. If the popped entry fails validation, `StartNext` calls itself recursively — the recursive call will issue its own `PopHead` on the next item. This ensures one-item-per-call semantics with no double-pops or skipped items.

Pseudocode:
```
func StartNext(sess, gameClock):
    entry = PopHead(sess.CharacterID)        // removes position 1, reindexes
    if entry == nil:
        notify player "Downtime complete. Your queue is empty."
        return

    if room tags invalid for entry.ActivityID:
        notify player "Skipped: <name> — room requirements not met."
        StartNext(sess, gameClock)           // recursive: next call pops next item
        return

    if entry.ActivityID == "craft":
        if DeductMany fails:
            notify player "Skipped: <name> — insufficient materials."
            StartNext(sess, gameClock)       // recursive: next call pops next item
            return

    metadata = generateMetadata(entry)
    write character_downtime row (activity_id, completes_at, room_id, metadata)
    set PlayerSession downtime fields; DowntimeBusy = true
    notify player "Starting: <name>."
```

- REQ-DTQ-7: `StartNext` MUST validate room tags for each queued activity before starting it.
- REQ-DTQ-10: Each `StartNext` invocation MUST call `PopHead` exactly once at the start. If the popped item is invalid, `StartNext` calls itself recursively (the recursive call pops the next item). Termination is guaranteed because `PopHead` always removes one item and the queue is finite.

### 3.1 Reconnect Behavior

On player reconnect:
1. Resolve any elapsed active activity per downtime spec REQ-DT-6 (full skill check, side effects, notification).
2. Compute the hypothetical completion time of each queued activity in order, using the just-resolved activity's `DowntimeCompletesAt` as the baseline.
3. For each queued activity whose hypothetical completion time is in the past (i.e., it would have completed while the player was offline):
   a. Validate room tags (using the room the player was in when they disconnected — stored as the character's last known room).
   b. If tags valid: deduct materials (for Craft), resolve skill check, apply side effects, notify player with "(offline)" tag, e.g., "Downtime completed (offline): Earn Creds — Success."
   c. If tags invalid or materials insufficient: skip with notification, advance to next.

Room tag validation during reconnect uses the `room_id` stored in `character_downtime` at activity start time (per downtime spec Section 1). The room is looked up from the world registry to check its current tags.
   d. Advance the hypothetical clock baseline to this activity's hypothetical completion time.
4. If a queued activity's hypothetical completion time is in the future: start it as the new active activity (set `DowntimeBusy = true`, write `character_downtime` row, restore player to busy state).
5. If all queued activities have been resolved offline and the queue is now empty: return full control. Send a summary notification: "While you were away: N activities completed, M skipped." (N and M derived from the reconnect loop counts).

- REQ-DTQ-11: On reconnect, all queued activities whose estimated completion time has elapsed MUST be fully resolved (skill check + side effects) in order before returning control to the player. Each resolved activity MUST be notified to the player with an "(offline)" marker.

---

## 4. Architecture

### 4.1 Repository

New `CharacterDowntimeQueueRepository` in `internal/storage/postgres/`:

```go
Enqueue(ctx context.Context, characterID int64, entry QueueEntry) error
ListQueue(ctx context.Context, characterID int64) ([]QueueEntry, error)  // ordered by position
RemoveAt(ctx context.Context, characterID int64, position int) error      // deletes + reindexes in one tx
Clear(ctx context.Context, characterID int64) error
PopHead(ctx context.Context, characterID int64) (*QueueEntry, error)     // atomically removes pos 1, reindexes
```

`QueueEntry`:
```go
type QueueEntry struct {
    ID               int64
    CharacterID      int64
    Position         int
    ActivityID       string
    ActivityArgs     string
    ActivityMetadata json.RawMessage
}
```

- REQ-DTQ-12: `PopHead` and `RemoveAt` MUST execute position reindexing within a single DB transaction.

### 4.2 Engine Extension

`internal/game/downtime/engine.go` gains:

```go
StartNext(sess *PlayerSession, gameClock GameClock) error
```

Called by `CheckCompletion` after resolving the active activity. Also called on reconnect after resolving an elapsed active activity.

### 4.3 Queue Limit Registry

`internal/game/downtime/queue_limits.go`:

```go
type DowntimeQueueLimitRegistry struct { ... }
func (r *DowntimeQueueLimitRegistry) Lookup(jobTier int, level int) int
```

Loaded at startup from `content/downtime_queue_limits.yaml`. Used at login and on level-up/job-change to set `PlayerSession.DowntimeQueueLimit`.

### 4.4 Command Dispatch

`downtime queue` subcommands are dispatched within the existing `handleDowntime` case in `grpc_service.go`. No new handler constant is needed. The existing `DowntimeRequest { subcommand string, args string }` proto message handles all queue commands: `subcommand = "queue"`, `args = "list"` / `"clear"` / `"remove 3"` / `"craft stim_pack"` etc.

---

## 5. Requirements Summary

- REQ-DTQ-1: `downtime queue <alias>` MUST fail if the queue is at `DowntimeQueueLimit`.
- REQ-DTQ-2: `downtime queue <alias>` MUST fail if the activity alias or required args are invalid.
- REQ-DTQ-3: `downtime queue craft <recipe>` MUST fail if the recipe does not exist.
- REQ-DTQ-4: `downtime queue analyze <item>` and `downtime queue repair <item>` MUST warn (not fail) if the item is not in inventory at queue time; re-validated at start time.
- REQ-DTQ-5: Room tag requirements MUST NOT be validated at queue time; they MUST be validated at activity start time.
- REQ-DTQ-6: `downtime queue list` MUST display estimated start and end times for each queued activity.
- REQ-DTQ-7: `StartNext` MUST validate room tags for each queued activity before starting it.
- REQ-DTQ-8: Materials for queued Craft activities MUST be deducted at activity start time, not at queue time.
- REQ-DTQ-9: If a queued Craft activity cannot deduct materials at start time, it MUST be skipped with a player notification and the next queued activity MUST be attempted.
- REQ-DTQ-10: `StartNext` recursion MUST terminate when the queue is empty or an eligible activity is found. Termination is guaranteed by `PopHead` removing one item per call.
- REQ-DTQ-11: On reconnect, all queued activities whose estimated completion time has elapsed MUST be fully resolved (skill check + side effects) in order, notified with an "(offline)" marker, before returning control to the player.
- REQ-DTQ-12: `PopHead` and `RemoveAt` MUST execute position reindexing within a single DB transaction.
- REQ-DTQ-13: Job YAML definitions MUST include a `tier int` field. Absence of `tier` on any job MUST be a fatal load error at startup. Job YAML files are located in `content/jobs/`.
- REQ-DTQ-14: `DowntimeQueueLimitRegistry.Lookup` MUST return 3 as the default if no entry matches the character's job tier and level.
- REQ-DTQ-15: Failure to load `content/downtime_queue_limits.yaml` MUST be a fatal startup error.
- REQ-DTQ-16: `downtime queue clear` MUST affect only queued (not yet started) activities. It MUST NOT cancel the currently active activity.
