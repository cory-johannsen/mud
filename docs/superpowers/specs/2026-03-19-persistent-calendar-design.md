# Persistent Calendar Design

**Date:** 2026-03-19
**Feature:** Persistent in-game calendar (day + month) tracking and display

---

## Goal

Add a `GameCalendar` that wraps the existing `GameClock`, tracks the current in-game day and month, persists them to the database across server restarts, and displays them in the room header instead of the prompt.

---

## Scope

In scope: `GameCalendar` type; `world_calendar` DB table + repo; room header display of date + time; removal of time from the prompt; day/month rollover on midnight; first-boot default (day=1, month=1).

Out of scope: year tracking; seasons/weather (depends on Room Danger Levels); exploration mode tracking.

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
`GameCalendar` MUST be defined in `internal/gameserver/calendar.go`. It MUST embed or reference a `*GameClock` (subscribe-only), maintain `day int` and `month int` in memory, and own a `CalendarRepo` for persistence.

### REQ-CAL3
`GameCalendar` MUST expose the same subscribe/unsubscribe pattern as `GameClock`, but with channels typed `chan<- GameDateTime`.

### REQ-CAL4
`FormatDate(month, day int) string` MUST be defined in `internal/gameserver/calendar.go`. It MUST return the month name and day with correct ordinal suffix (e.g. "January 1st", "March 22nd", "November 3rd", "April 12th").

---

## Rollover Logic

### REQ-CAL5
`GameCalendar` MUST subscribe to `GameClock`. On each tick, it MUST broadcast a `GameDateTime{Hour: h, Day: day, Month: month}` to all subscribers.

### REQ-CAL6
When the received `GameHour` is 0 (midnight), `GameCalendar` MUST advance the day by 1. It MUST use `time.Date` with a fixed year (e.g. 2000) to derive the correct next day and month, correctly handling month boundaries and varying month lengths.

### REQ-CAL7
On every midnight rollover, `GameCalendar` MUST call `CalendarRepo.Save(day, month)`. Save failures MUST be logged but MUST NOT stop the clock.

---

## Persistence

### REQ-CAL8
A `world_calendar` table MUST be created via migration:
```sql
CREATE TABLE IF NOT EXISTS world_calendar (
    id    INTEGER PRIMARY KEY DEFAULT 1,
    day   INTEGER NOT NULL,
    month INTEGER NOT NULL
);
```

### REQ-CAL9
`CalendarRepo` MUST be an interface in `internal/storage/postgres/calendar.go`:
```go
type CalendarRepo interface {
    Load() (day, month int, err error)
    Save(day, month int) error
}
```
`Load` MUST return `(1, 1, nil)` when no row exists (first boot). `Save` MUST upsert the single row (id=1).

### REQ-CAL10
`GameServiceServer` MUST call `CalendarRepo.Load()` at startup before constructing `GameCalendar`. The loaded day and month MUST be passed to `NewGameCalendar(clock, day, month, repo)`.

---

## UI Changes

### REQ-CAL11
`RenderRoomView` in `internal/frontend/text_renderer.go` MUST gain a `dt GameDateTime` parameter. The room name line MUST display the date and time appended after the room name, formatted as:
```
Room Name — January 1st Morning 07:00
```
The separator MUST be ` — ` (em-dash with spaces).

### REQ-CAL12
`BuildPrompt` in `internal/frontend/handlers/game_bridge.go` MUST have the time segment removed. The prompt MUST display only HP and active conditions.

### REQ-CAL13
`game_bridge.go` MUST subscribe to `GameCalendar` (not `GameClock`) for its display channel. It MUST pass the received `GameDateTime` to `RenderRoomView` on each tick and on room entry.

### REQ-CAL14
Existing `GameClock` subscribers that only need hour (e.g. room effect ticks) MUST remain subscribed to `GameClock` directly and MUST NOT be changed.

---

## Wiring

### REQ-CAL15
`GameServiceServer` MUST gain a `*GameCalendar` field. It MUST be constructed after `GameClock` using the DB-loaded day and month, then started alongside `GameClock` at server startup.

### REQ-CAL16
No new config keys are required. `GameCalendar` derives its tick cadence entirely from `GameClock` (which uses `GameTickDuration` from config).

---

## Testing

### REQ-CAL17
Unit tests in `internal/gameserver/calendar_test.go` MUST verify:
- `FormatDate` produces correct ordinal suffixes for 1st/2nd/3rd/4th/11th/12th/13th/21st/22nd/23rd
- Day rollover advances day by 1 on hour=0 ticks
- Month rollover occurs correctly (e.g. Jan 31 → Feb 1, Feb 28 → Mar 1 in a non-leap year)
- `GameDateTime` is broadcast on every tick (not just midnight)
- Empty `Triggers` (no subscribers) is a no-op

### REQ-CAL18
Unit tests in `internal/storage/postgres/calendar_test.go` MUST verify:
- `Load` returns `(1, 1, nil)` when the table is empty
- `Save` then `Load` round-trips day and month correctly
- A second `Save` overwrites the first (upsert)

### REQ-CAL19
`go test ./internal/gameserver/... ./internal/storage/postgres/... ./internal/frontend/...` MUST pass after all changes.

---

## Architecture

### File Map

| File | Change |
|---|---|
| `internal/gameserver/calendar.go` | New — `GameDateTime`, `GameCalendar`, `FormatDate`, `CalendarRepo` interface |
| `internal/gameserver/calendar_test.go` | New — unit tests for rollover, formatting, broadcast |
| `internal/storage/postgres/calendar.go` | New — `PostgresCalendarRepo` implementing `CalendarRepo` |
| `internal/storage/postgres/calendar_test.go` | New — repo round-trip tests |
| `internal/storage/postgres/migrations/` | New migration — `world_calendar` table |
| `internal/gameserver/grpc_service.go` | Wire `GameCalendar` into `GameServiceServer`; call `Load()` at startup |
| `internal/frontend/text_renderer.go` | `RenderRoomView` gains `dt GameDateTime` parameter |
| `internal/frontend/handlers/game_bridge.go` | Subscribe to `GameCalendar`; remove time from prompt; pass `dt` to `RenderRoomView` |

### Data Flow

```
GameClock tick (every GameTickDuration)
  └─ GameCalendar.onTick(hour)
       ├─ if hour == 0: advance day/month; Save() to DB
       └─ broadcast GameDateTime{Hour, Day, Month} to subscribers
            └─ game_bridge.go display channel
                 ├─ RenderRoomView(rv, width, dt) → room header: "Room — January 1st Morning 07:00"
                 └─ BuildPrompt(name, hp, conditions) → prompt: no time segment
```
