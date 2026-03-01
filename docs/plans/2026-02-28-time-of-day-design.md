# Time of Day — Design Document

**Date:** 2026-02-28
**Status:** Approved

---

## Overview

A game clock advances one game-hour per real minute, giving a full 24-hour cycle every 24 real minutes. The clock drives atmospheric room descriptions, outdoor visibility effects, and live prompt updates showing the current period, game hour, and character health.

---

## Architecture

A `GameClock` singleton goroutine runs inside the gameserver. It tracks game time as an integer 0–23 (game hours), advancing by 1 every real minute. On each tick it broadcasts a `TimeOfDayEvent` proto to all active sessions. The frontend receives these events and updates the prompt in real time. `RoomView` also carries the current game hour so room descriptions include time-aware flavor text without a separate round-trip.

---

## Components

### Game Clock (`internal/gameserver/clock.go`)

- `GameClock` struct with `hour atomic.Int32` (0–23), tick goroutine advancing every real minute
- `Subscribe(ch chan<- GameHour) / Unsubscribe(ch)` for session fan-out
- `GameHour` type with methods: `Period() TimePeriod`, `String() string` (e.g. `"18:00"`)
- `TimePeriod` typed string constants:

| Period | Hours |
|--------|-------|
| Midnight | 0 |
| LateNight | 1–4 |
| Dawn | 5–6 |
| Morning | 7–11 |
| Afternoon | 12–16 |
| Dusk | 17–18 |
| Evening | 19–21 |
| Night | 22–23 |

- Configurable via `GameServerConfig`: `game_clock_start` (default `6` = dawn), `game_tick_duration` (default `1m`)

### Proto Additions (`api/proto/game/v1/game.proto`)

- New `TimeOfDayEvent` message: `int32 hour`, `string period` — added to `ServerEvent` oneof
- `RoomView` gets two new fields: `int32 hour`, `string period`

### Room Description Flavor (`internal/gameserver/`)

- `FlavorText(period TimePeriod, isOutdoor bool) string` — returns a short atmospheric sentence appended to outdoor room descriptions; returns empty string for indoor rooms
- Outdoor rooms detected via `room.Properties["outdoor"] == "true"`
- Dark periods (Midnight, LateNight, Night): exit list in `RoomView` trimmed to visible exits only for outdoor rooms (darkness hides exits)

### Prompt Renderer (`internal/frontend/handlers/game_bridge.go`)

- `BuildPrompt(name, period, hour string, currentHP, maxHP int32) string` — pure function, fully testable
- Format: `[Thorald | Dusk 18:00 | 45/60hp]> `
- Color scheme:

| Element | Color |
|---------|-------|
| Name | Bright cyan |
| HP ≥ 75% | Bright green |
| HP ≥ 40% | Yellow |
| HP < 40% | Red |
| Dawn | Yellow |
| Morning | Bright yellow |
| Afternoon | White |
| Dusk | (orange/bright red) |
| Evening | Magenta |
| Night / Midnight / LateNight | Dark blue (blue) |

---

## Data Flow

1. **Gameserver starts** → `GameClock` goroutine launched, begins at configured start hour
2. **Session joins** → `handleJoinWorld` subscribes session channel to `GameClock`; current hour embedded in first `RoomView`
3. **Every real minute** → `GameClock` advances hour, broadcasts `TimeOfDayEvent` to all subscribed session channels via non-blocking send (slow clients drop the tick)
4. **Session receives tick** → `grpc_service.go` fan-out loop sends `TimeOfDayEvent` as `ServerEvent` to the client stream
5. **Frontend receives `TimeOfDayEvent`** → `forwardServerEvents` updates `currentTime atomic.Value`, calls `WritePrompt(BuildPrompt(...))` with fresh time — no room re-render
6. **Player moves / looks** → `RoomView` carries current `hour` + `period`; `RenderRoomView` appends flavor text for outdoor rooms; prompt re-rendered via `BuildPrompt`
7. **Session ends** → `handleJoinWorld` unsubscribes session channel from `GameClock`

---

## Error Handling

- `WritePrompt` failures on time tick logged at debug level and ignored — a dropped prompt update is not fatal
- Slow client subscription channels: non-blocking send with `select/default` — tick dropped silently
- `FlavorText` always returns a string (empty for indoor) — no error path
- `BuildPrompt` is a pure function — no error path

---

## Testing

- **`GameClock`**: property-based test — for any N ticks, `hour` is always in [0,23] and wraps correctly; subscribe/unsubscribe race tested with `-race`
- **`BuildPrompt`**: table-driven tests covering all 8 periods, all 3 HP color thresholds, zero HP, max HP
- **`FlavorText`**: table-driven tests for all 8 periods × indoor/outdoor (16 cases)
- **`RenderRoomView`**: existing tests extended to verify flavor text appended for outdoor rooms and suppressed for indoor
- **`TimePeriod` mapping**: property test — for any hour in [0,23], `Period()` returns one of the 8 valid periods
