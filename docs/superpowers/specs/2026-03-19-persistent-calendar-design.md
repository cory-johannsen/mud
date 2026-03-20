# Persistent Calendar Design

**Date:** 2026-03-19
**Feature:** Persistent in-game calendar (day + month) tracking and display

---

## Goal

Add a `GameCalendar` that wraps the existing `GameClock`, tracks the current in-game day and month, persists them to the database across server restarts, and displays them in the room header instead of the prompt.

---

## Scope

In scope: `GameCalendar` type; `world_calendar` DB table + repo; room header display of date + time; removal of time from the prompt; day/month rollover on midnight; first-boot default (day=1, month=1).

Out of scope: year tracking; exploration mode tracking.

Deferred: seasons and weather effects — the feature doc specifies that weather is driven by season and scales with room danger level. This is deferred pending the Room Danger Levels feature. No requirements in this spec cover season or weather.

---

## Data Model

### REQ-CAL1
`GameDateTime` MUST be defined in `internal/gameserver/calendar.go`:
```go
type GameDateTime struct {
    Hour  GameHour // 0–23
    Day   int      // 1–28/29/30/31
    Month int      // 1–12
}
```

### REQ-CAL2
`GameCalendar` MUST be defined in `internal/gameserver/calendar.go`. It MUST subscribe to a `*GameClock`, maintain `day int` and `month int` in memory, own a `CalendarRepo` for persistence, and protect its subscriber map with a `sync.Mutex` to allow concurrent subscribe/unsubscribe calls.

### REQ-CAL3
`GameCalendar` MUST expose the following methods, mirroring the pattern of `GameClock`:
```go
func (c *GameCalendar) Subscribe(ch chan<- GameDateTime)
func (c *GameCalendar) Unsubscribe(ch chan<- GameDateTime)
```
The subscriber map MUST be protected by the same mutex as REQ-CAL2. Callers create and own the channel; `Subscribe` registers it to receive ticks.

### REQ-CAL4
`FormatDate(month, day int) string` MUST be defined in `internal/gameserver/calendar.go`. It MUST return the month name and day with correct ordinal suffix. Ordinal rules: day ending in 1 (except 11) → "st"; day ending in 2 (except 12) → "nd"; day ending in 3 (except 13) → "rd"; all others → "th". Examples: "January 1st", "February 2nd", "March 3rd", "April 12th", "May 21st", "June 22nd", "July 23rd", "November 11th", "December 13th".

### REQ-CAL5
`CalendarRepo` MUST be an interface defined in `internal/gameserver/calendar.go` (not in the storage package, to avoid import cycles):
```go
type CalendarRepo interface {
    Load() (day, month int, err error)
    Save(day, month int) error
}
```

---

## Rollover Logic

### REQ-CAL6
`GameCalendar` MUST subscribe to `GameClock`. On each tick it MUST:
1. If the received `GameHour` is 0 (midnight): advance day/month first (see REQ-CAL7), then call `Save` (see REQ-CAL8).
2. Broadcast `GameDateTime{Hour: h, Day: day, Month: month}` to all subscribers **after** any rollover, so midnight subscribers receive the new day (not the old day).

### REQ-CAL7
When the received `GameHour` is 0 (midnight), `GameCalendar` MUST advance the day by 1 using `time.Date` with the fixed year **2001** (a non-leap year, so February has exactly 28 days). The call MUST be:
```go
next := time.Date(2001, time.Month(month), day+1, 0, 0, 0, 0, time.UTC)
day, month = next.Day(), int(next.Month())
```
This correctly handles all month boundaries including February (28 days) and months with 30 vs 31 days.

### REQ-CAL8
On every midnight rollover, `GameCalendar` MUST call `CalendarRepo.Save(day, month)`. Save failures MUST be logged at warn level but MUST NOT stop the clock or prevent subsequent ticks.

---

## Persistence

### REQ-CAL9
A `world_calendar` table MUST be created via a new migration pair:
- `migrations/030_world_calendar.up.sql`
- `migrations/030_world_calendar.down.sql`

Up migration:
```sql
CREATE TABLE IF NOT EXISTS world_calendar (
    id    INTEGER PRIMARY KEY DEFAULT 1,
    day   INTEGER NOT NULL,
    month INTEGER NOT NULL
);
```
Down migration:
```sql
DROP TABLE IF EXISTS world_calendar;
```

### REQ-CAL10
`PostgresCalendarRepo` MUST implement `CalendarRepo` in `internal/storage/postgres/calendar.go`. `Load` MUST return `(1, 1, nil)` when no row exists (first boot). `Save` MUST upsert the single row (id=1) using `INSERT ... ON CONFLICT (id) DO UPDATE`.

### REQ-CAL11
`GameServiceServer` MUST call `CalendarRepo.Load()` at startup before constructing `GameCalendar`. The loaded day and month MUST be passed to `NewGameCalendar(clock, day, month, repo)`. The initial state is NOT saved to the DB at construction time — it is only persisted at the first midnight rollover. A server restarted before midnight on day=1 will re-load `(1, 1)` from the empty table, which is the correct first-boot behavior.

---

## UI Changes

### REQ-CAL12
`RenderRoomView` in `internal/frontend/handlers/text_renderer.go` MUST gain a `dt GameDateTime` parameter appended after `maxLines`. The complete new signature MUST be:
```go
func RenderRoomView(rv *gamev1.RoomView, width int, maxLines int, dt GameDateTime) string
```
The room name line MUST display the date and time appended after the room name, formatted as:
```
Room Name — January 1st Morning 07:00
```
The separator MUST be ` — ` (en-dash with spaces). The period name comes from `dt.Hour.Period()` and the hour from `dt.Hour.String()`.

### REQ-CAL13
`BuildPrompt` in `internal/frontend/handlers/game_bridge.go` MUST have the time segment removed. The `period` and `hour` string parameters MUST be removed. The complete new signature MUST be:
```go
func BuildPrompt(name string, currentHP, maxHP int32, conditions []string) string
```
The prompt MUST display only HP and active conditions.

### REQ-CAL14
`game_bridge.go` MUST subscribe to `GameCalendar` (not `GameClock`) for its display channel. It MUST pass the received `GameDateTime` to `RenderRoomView` on each tick and on room entry.

### REQ-CAL15
Existing `GameClock` subscribers that only need hour (e.g. room effect ticks) MUST remain subscribed to `GameClock` directly and MUST NOT be changed.

---

## Wiring

### REQ-CAL16
`GameServiceServer` MUST gain a `*GameCalendar` field. It MUST be constructed after `GameClock` using the DB-loaded day and month, then started alongside `GameClock` at server startup.

### REQ-CAL17
No new config keys are required. `GameCalendar` derives its tick cadence entirely from `GameClock` (which uses `GameTickDuration` from config).

---

## Testing

### REQ-CAL18
Unit tests in `internal/gameserver/calendar_test.go` MUST verify:
- `FormatDate` produces correct ordinal suffixes for days 1, 2, 3, 4, 11, 12, 13, 21, 22, 23, 31
- Day rollover advances day by 1 on hour=0 ticks
- Month rollover occurs correctly: Jan 31 → Feb 1; Feb 28 → Mar 1 (using fixed year 2001)
- `GameDateTime` is broadcast on every tick (not just midnight)
- No subscribers is a no-op (broadcast with empty map does not panic)
- A `Save`-failing `CalendarRepo` (stub that always returns error) does NOT prevent subsequent ticks from being broadcast

### REQ-CAL19
Unit tests in `internal/storage/postgres/calendar_test.go` MUST verify:
- `Load` returns `(1, 1, nil)` when the table is empty
- `Save` then `Load` round-trips day and month correctly
- A second `Save` overwrites the first (upsert behavior)

### REQ-CAL20
All existing tests in `internal/frontend/handlers/text_renderer_test.go` and `internal/frontend/handlers/game_bridge_test.go` that call `RenderRoomView` or `BuildPrompt` MUST be updated to match the new signatures. Any test that asserts `RenderRoomView` output does NOT contain time/period fields (e.g. `TestRenderRoomView_TimeFields_NoExtraOutput`) MUST be inverted or replaced to assert that the room header DOES contain the formatted date and time — this is a behavioral change, not just a signature update.

### REQ-CAL21
`go test ./internal/gameserver/... ./internal/storage/postgres/... ./internal/frontend/...` MUST pass after all changes.

---

## Architecture

### File Map

| File | Change |
|---|---|
| `internal/gameserver/calendar.go` | New — `GameDateTime`, `GameCalendar`, `FormatDate`, `CalendarRepo` interface |
| `internal/gameserver/calendar_test.go` | New — unit tests for rollover, formatting, broadcast, Save-failure resilience |
| `internal/storage/postgres/calendar.go` | New — `PostgresCalendarRepo` implementing `CalendarRepo` |
| `internal/storage/postgres/calendar_test.go` | New — repo round-trip tests |
| `migrations/030_world_calendar.up.sql` | New — create `world_calendar` table |
| `migrations/030_world_calendar.down.sql` | New — drop `world_calendar` table |
| `internal/gameserver/grpc_service.go` | Modified — wire `GameCalendar`; call `Load()` at startup |
| `internal/frontend/handlers/text_renderer.go` | Modified — `RenderRoomView` gains `dt GameDateTime` parameter |
| `internal/frontend/handlers/text_renderer_test.go` | Modified — update `RenderRoomView` call sites to pass `dt` |
| `internal/frontend/handlers/game_bridge.go` | Modified — subscribe to `GameCalendar`; remove time from prompt; pass `dt` to `RenderRoomView` |
| `internal/frontend/handlers/game_bridge_test.go` | Modified — update `BuildPrompt` and `RenderRoomView` call sites |

### Data Flow

```
GameClock tick (every GameTickDuration)
  └─ GameCalendar.onTick(hour)
       ├─ if hour == 0: advance day/month using time.Date(2001,...); Save() to DB (log on failure)
       └─ broadcast GameDateTime{Hour, Day, Month} to subscribers
            └─ game_bridge.go display channel
                 ├─ RenderRoomView(rv, width, maxLines, dt) → room header: "Room — January 1st Morning 07:00"
                 └─ BuildPrompt(name, hp, conditions) → prompt: HP + conditions only
```
