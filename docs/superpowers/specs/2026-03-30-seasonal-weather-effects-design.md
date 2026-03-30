# Seasonal Weather Effects

**Date:** 2026-03-30
**Status:** spec

## Overview

Random severe weather events occur during appropriate seasons, applying conditions to all outdoor players simultaneously for the duration of the event. Active weather is displayed on-screen for both the telnet and web clients. Events persist across server restarts and are separated by a mandatory cooldown period.

## Architecture

```
GameCalendar (tick) ──► WeatherManager.OnTick(dt)
                              │
                    ┌─────────┴──────────┐
                    │                    │
               active event?        cooldown expired?
               end if past          roll for new event
               end_time             (season-weighted)
                    │                    │
               DB clear             DB write
               BroadcastAll         BroadcastAll
               WeatherEvent         WeatherEvent
               (active=false)       (active=true)

applyRoomEffectsOnEntry() ──► weatherMgr.ActiveEffects(room.Indoor)
combat round handler      ──► weatherMgr.ActiveEffects(room.Indoor)
```

**Key constraints:**
- At most one active weather event at a time
- Outdoor rooms (`indoor: false`) receive weather conditions; indoor rooms (`indoor: true`) are fully shielded
- Events survive server restart via DB persistence
- Mandatory 24–72 game-hour cooldown between events

## Requirements

### 1. Seasons

- REQ-SWE-1: Seasons MUST be computed from the calendar month with no stored state:
  - Spring: months 3–5
  - Summer: months 6–8
  - Fall: months 9–11
  - Winter: months 12, 1, 2

### 2. Weather Type Content (`content/weather.yaml`)

- REQ-SWE-2: A new `content/weather.yaml` file MUST define all weather event types. Each entry MUST contain:
  - `id` (string): unique identifier
  - `name` (string): display name (e.g., `"Blizzard"`)
  - `announce` (string): broadcast message on event start (e.g., `"A blizzard is sweeping across Portland!"`)
  - `end_announce` (string): broadcast message on event end (e.g., `"The blizzard has passed."`)
  - `seasons` (list of strings): which seasons this event can occur in
  - `weight` (int): relative probability weight within eligible seasons
  - `conditions` (list of strings): condition IDs applied to outdoor players

- REQ-SWE-3: The following weather types MUST be defined in `content/weather.yaml`:

| ID | Name | Seasons | Conditions |
|----|------|---------|------------|
| `rain` | Rain | spring, fall, winter | `reduced_visibility` |
| `heavy_rain` | Heavy Rain | spring, fall, winter | `reduced_visibility`, `terrain_flooded` |
| `fog` | Dense Fog | spring, fall, winter | `reduced_visibility` |
| `thunderstorm` | Thunderstorm | spring, summer, fall | `reduced_visibility`, `terrain_flooded` |
| `blizzard` | Blizzard | winter | `reduced_visibility`, `terrain_ice`, `terrain_mud` |
| `ice_storm` | Ice Storm | winter | `terrain_ice`, `reduced_visibility` |
| `sleet` | Sleet | winter, spring | `terrain_ice`, `reduced_visibility` |
| `hailstorm` | Hailstorm | spring, summer, fall | `reduced_visibility` |
| `windstorm` | Windstorm | spring, fall | `reduced_visibility` |
| `extreme_heat` | Extreme Heat | summer | `dazzled` |

- REQ-SWE-4: Weather conditions that do not already exist in `content/conditions/` MUST be created as new YAML condition files before use by the weather system.

### 3. Room Indoor Flag

- REQ-SWE-5: The `Room` struct MUST gain a new `Indoor bool` field (YAML key: `indoor`, default `false`).
- REQ-SWE-6: Zone YAML files MUST be updated to mark explicitly indoor rooms (e.g., interiors of buildings, shops, underground). All rooms default to outdoor if `indoor` is omitted.

### 4. Calendar Tick Counter

- REQ-SWE-7a: The `GameCalendar` MUST maintain a monotonically-increasing `Tick int64` field, incremented by 1 on every game-hour advance. The tick value MUST be persisted in the existing `calendar` DB table as a new `tick` column (migration required). On startup the tick is loaded from DB; if absent (upgrade from older schema), it is initialised to `(month-1)*31*24 + (day-1)*24 + hour`.

### 5. Database (`weather_events` table)

- REQ-SWE-7: A new migration MUST create the `weather_events` table:

```sql
CREATE TABLE weather_events (
    id               SERIAL PRIMARY KEY,
    weather_type     TEXT  NOT NULL,
    end_tick         BIGINT NOT NULL,
    cooldown_end_tick BIGINT NOT NULL DEFAULT 0,
    active           BOOL  NOT NULL DEFAULT TRUE
);
```

- REQ-SWE-8: At most one row with `active = TRUE` MAY exist at a time. When an event ends, `active` is set to `FALSE` and `cooldown_end_tick` is set to `current_tick + random(24, 72)`. When the cooldown expires and a new event starts, the old row is deleted before inserting a new one.

### 6. WeatherRepo (`internal/storage/postgres/weather_repo.go`)

- REQ-SWE-9: A new `WeatherRepo` interface MUST be defined with:
  - `GetActive() (*WeatherEvent, error)` — returns the active event or nil
  - `GetCooldown() (endTick int64, found bool, err error)` — returns cooldown end tick if cooling down
  - `StartEvent(weatherType string, endTick int64) error`
  - `EndEvent(cooldownEndTick int64) error`
  - `ClearExpired() error` — deletes inactive rows whose cooldown has passed

### 7. WeatherManager (`internal/gameserver/weather_manager.go`)

- REQ-SWE-10: `WeatherManager` MUST implement `calendar.Subscriber` and be registered with the `GameCalendar` at startup.

- REQ-SWE-11: On startup, `WeatherManager` MUST load the active event and cooldown state from the DB. If the active event's end time is already past the current game time, it MUST be ended immediately (with broadcast) before the first tick.

- REQ-SWE-12: On each calendar tick, `WeatherManager.OnTick(dt GameDateTime)` MUST:
  1. If an active event exists and `dt >= end_time`: call `endEvent()` — delete the DB active row, write cooldown (random 24–72 game hours from `dt`), broadcast end message, clear in-memory state.
  2. Else if no active event and no active cooldown and `dt >= cooldown_end_time`: roll for new event — compute season from `dt.Month`, filter `content/weather.yaml` by season, weight-sample one entry, roll random duration (2–168 game hours), call `startEvent()` — write to DB, set in-memory state, broadcast start message.
  3. Else if no active event and cooldown not yet expired: do nothing.

- REQ-SWE-13: The roll probability per tick MUST be configurable via a `WeatherChancePerTick float64` field in the game config (YAML: `weather.chance_per_tick`, default `0.05`). The roll is `rand.Float64() < WeatherChancePerTick`.

- REQ-SWE-14: `WeatherManager.ActiveEffects(indoor bool) []RoomEffect` MUST return the active weather type's condition list as `RoomEffect` entries (one per condition ID, with a sensible base DC of 12), or an empty slice if no event is active or `indoor == true`.

- REQ-SWE-15: All game-time comparisons for event end and cooldown MUST use the `GameCalendar.Tick` counter (REQ-SWE-7a) to correctly handle month/year rollovers.

### 8. Integration with Existing Systems

- REQ-SWE-16: `applyRoomEffectsOnEntry()` in `grpc_service.go` MUST append `s.weatherMgr.ActiveEffects(room.Indoor)` to the room's effect list before resolving effects.

- REQ-SWE-17: The combat round effect application in `combat_handler.go` MUST append `s.weatherMgr.ActiveEffects(room.Indoor)` to the room's per-round effect list.

- REQ-SWE-18: `WeatherManager` MUST be constructed via Wire and injected into `GameServiceServer`.

- REQ-SWE-19: `configs/dev.yaml` and the production Helm values MUST gain a `weather.chance_per_tick` config field.

### 9. Proto (`WeatherEvent`)

- REQ-SWE-20: A new `WeatherEvent` message MUST be added to `ServerEvent`:
  ```proto
  message WeatherEvent {
    string weather_name = 1;
    bool   active       = 2;
  }
  ```

- REQ-SWE-21: The gameserver MUST send a `WeatherEvent` to all connected players when an event starts (`active: true`) or ends (`active: false`).

- REQ-SWE-22: The gameserver MUST send the current `WeatherEvent` state to each player on session join (after `RoomView` is delivered). If no event is active, no message is sent.

### 10. Telnet Client Weather Indicator

- REQ-SWE-23: `game_bridge.go` MUST store the active weather name (empty string when none) updated by `WeatherEvent` messages.

- REQ-SWE-24: `RenderRoomView()` MUST render a centered weather banner as the first line of the room region when weather is active (e.g., `          *** BLIZZARD ***          ` in cyan). When no weather is active, the banner line is omitted and the room region renders at its normal height.

### 11. Web Client Weather Indicator

- REQ-SWE-25: The `GameContext` MUST track `activeWeather: string | null`, updated when `WeatherEvent` messages are received.

- REQ-SWE-26: The game toolbar (top bar of the `/game` route) MUST render a centered weather badge when `activeWeather` is non-null: pill-shaped element with amber text and dark background, showing the weather name (e.g., `⛈ Thunderstorm`). The badge MUST be removed when `active: false` is received.

### 12. Testing

- REQ-SWE-27: `WeatherManager` MUST have property-based tests covering:
  - Event roll never fires during cooldown
  - Event always ends at or after its scheduled end time
  - `ActiveEffects` returns empty for indoor rooms regardless of active event
  - Season computation is correct for all 12 months

- REQ-SWE-28: `WeatherRepo` MUST have round-trip property tests in `internal/storage/postgres/testdata/`.

## Dependencies

- `persistent-calendar` — `GameCalendar` subscription and `GameDateTime` type
- `advanced-health` — condition registry for weather condition application
- `exploration` — zone effect application hooks (`applyRoomEffectsOnEntry`)
- `web-client` — web UI weather indicator

## Out of Scope

- Per-zone weather (all zones affected equally)
- Weather affecting NPC behavior
- Player crafting or shelter mechanics reducing weather severity
- Admin commands to trigger/end weather manually
