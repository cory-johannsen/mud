# Downtime Activity Queue

Allow players to queue up to 100 downtime activities upfront. After one activity completes, the next starts automatically without player intervention. The queue is DB-persisted and survives disconnects. See `docs/superpowers/specs/2026-03-20-downtime-queue-design.md` for full design spec.

## Requirements

### Data Model

- [ ] `character_downtime_queue` table: `id bigserial PK`, `character_id bigint`, `position int`, `activity_id text`, `activity_args text`, `activity_metadata jsonb` (NULL in queue; populated at start)
- [ ] REQ-DTQ-12: Position is 1-based and always contiguous (1..N). `PopHead` and `RemoveAt` MUST reindex in a single DB transaction.
- [ ] `content/downtime_queue_limits.yaml` — queue limit lookup table keyed by job tier + level range, up to max 100
- [ ] REQ-DTQ-15: Failure to load `content/downtime_queue_limits.yaml` MUST be a fatal startup error
- [ ] REQ-DTQ-13: Job YAML files in `content/jobs/` MUST include `tier int` field; absence MUST be a fatal load error
- [ ] `PlayerSession.DowntimeQueueLimit int` — computed at login from `DowntimeQueueLimitRegistry.Lookup(jobTier, level)`; REQ-DTQ-14: default 3 if no entry matches
- [ ] `PlayerSession.JobTier int` — read from active job YAML `tier` field at login
- [ ] Note: `character_downtime` table gains `room_id text NOT NULL` (per downtime spec update) for reconnect tag validation

### Commands

- [ ] `downtime queue <alias> [args]` — add activity to end of queue (same aliases as `downtime <alias>`)
- [ ] `downtime queue list` — show position, activity name, estimated start/end times (durations looked up from activity/recipe registries at display time; "unknown" if recipe deleted)
- [ ] `downtime queue remove <position>` — remove activity at position N; higher positions shift down
- [ ] `downtime queue clear` — cancel queued (not yet started) activities only; does NOT cancel active activity; REQ-DTQ-16

### Validation at Queue Time

- [ ] REQ-DTQ-1: Fail if queue is at `DowntimeQueueLimit`
- [ ] REQ-DTQ-2: Fail if activity alias or required args are invalid
- [ ] REQ-DTQ-3: `downtime queue craft <recipe>` MUST fail if recipe does not exist
- [ ] REQ-DTQ-4: `downtime queue analyze <item>` / `downtime queue repair <item>` MUST warn (not fail) if item not in inventory; re-validated at start time
- [ ] REQ-DTQ-5: Room tag requirements MUST NOT be validated at queue time; validated at activity start time only
- [ ] REQ-DTQ-8: Materials for queued Craft activities MUST be deducted at activity start time, not queue time

### Auto-start Engine

- [ ] `DowntimeEngine.StartNext(sess, gameClock)` — called after `CheckCompletion` resolves an activity
- [ ] Each `StartNext` call owns exactly one `PopHead`; if popped item fails validation, recurse (next call pops next item); REQ-DTQ-10
- [ ] REQ-DTQ-7: Validate room tags (`safe` + activity-specific) before starting each queued activity
- [ ] REQ-DTQ-9: Skip queued Craft if materials insufficient at start time; notify player; try next
- [ ] If all queued items skipped: notify "Downtime queue exhausted — no eligible activities remain."

### Reconnect Behavior

- [ ] REQ-DTQ-11: On reconnect, resolve all elapsed queued activities in order (full skill check + side effects) before returning control
- [ ] Each resolved offline activity notified with "(offline)" marker
- [ ] Room tag validation during reconnect uses `room_id` stored in `character_downtime` at activity start
- [ ] Summary notification on queue exhaustion: "While you were away: N activities completed, M skipped."

### Repository

- [ ] `CharacterDowntimeQueueRepository`: `Enqueue`, `ListQueue`, `RemoveAt`, `Clear`, `PopHead`
- [ ] `DowntimeQueueLimitRegistry`: loaded from `content/downtime_queue_limits.yaml` at startup; `Lookup(jobTier, level) int`

### Architecture

- [ ] `internal/game/downtime/engine.go` — add `StartNext(sess, gameClock)`
- [ ] `internal/storage/postgres/` — `CharacterDowntimeQueueRepository`
- [ ] `internal/game/downtime/queue_limits.go` — `DowntimeQueueLimitRegistry`
- [ ] `content/downtime_queue_limits.yaml` — limit table
- [ ] Queue subcommands dispatched within existing `handleDowntime` in `grpc_service.go`; no new handler constant needed
