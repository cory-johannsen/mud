# World Map Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `world-map` (priority 340)
**Dependencies:** `persistent-calendar`

---

## Overview

Adds a world-level map view and instant fast travel between discovered zones. The existing `map` command enters a modal state that takes over the console region. While in map mode, the player toggles between the per-zone view and the world view, selects a zone, and initiates fast travel. Zone discovery is derived from existing automap data — no new tracking table is needed.

**Architecture note on modal state:** All map modal state (`mapMode`, `mapView`, `mapSelectedZone`) lives on the **frontend** layer (in `gameBridge`), not on `PlayerSession`. The frontend intercepts input while in map mode without any round-trip to the gameserver. The server only serves map data and travel requests via RPCs.

**Architecture note on fast travel:** Fast travel preconditions (combat, Wanted level) are enforced **server-side** via a new `TravelRequest` RPC. The frontend initiates travel by sending a `TravelRequest`; the server validates preconditions, moves the player, and sends back a room view event. This avoids the frontend needing to cache server-side session state.

---

## 1. Zone Discovery

A zone is considered **discovered** if the player has at least one room in `AutomapCache` for that zone. No new database table or field is required.

The existing `engine.map.reveal_zone(uid, zoneID)` Lua callback, currently assigned `nil` at the gameserver layer, is wired in `GameServiceServer` initialization: calling it bulk-inserts all rooms in the specified zone into `character_map_rooms` and populates `AutomapCache[zoneID]` for the player's session.

- REQ-WM-1: A zone MUST be considered discovered if `len(sess.AutomapCache[zoneID]) > 0`.
- REQ-WM-2: The `engine.map.reveal_zone(uid, zoneID)` Lua callback MUST be wired in `GameServiceServer` initialization to insert all rooms in the target zone into `character_map_rooms` and into `sess.AutomapCache[zoneID]`.
- REQ-WM-3: `reveal_zone` MUST use idempotent inserts (`ON CONFLICT DO NOTHING`) consistent with the existing automap DB pattern.

---

## 2. Zone World Coordinates

Each zone YAML gains optional `world_x` and `world_y` fields. The `Zone` struct gains `WorldX *int` and `WorldY *int` pointer fields (nil by default). A nil `WorldX` or `WorldY` means the zone has no world map position and is excluded from the world map. This pointer type is required to distinguish zones at grid position `(0, 0)` (e.g., `downtown`) from zones that have not been assigned a position.

### 2.1 Zone Grid Positions

Lower `world_y` values are further north. The grid is defined as follows:

| Zone ID | world_x | world_y | Notes |
|---------|---------|---------|-------|
| `battleground` | 4 | -6 | Battleground, WA (far NE) |
| `the_couve` | 0 | -4 | Vancouver, WA (north across Columbia) |
| `vantucky` | 2 | -4 | East Vancouver, WA |
| `sauvie_island` | -2 | -2 | Northwest, in the Columbia |
| `pdx_international` | 2 | -2 | NE Portland / airport |
| `hillsboro` | -4 | 0 | Far west |
| `beaverton` | -2 | 0 | West |
| `downtown` | 0 | 0 | Center |
| `ne_portland` | 2 | 0 | NE Portland |
| `rustbucket_ridge` | 4 | 0 | East Portland |
| `troutdale` | 6 | 0 | Far east along I-84 |
| `aloha` | -4 | 2 | Southwest |
| `ross_island` | 0 | 2 | South (in Willamette) |
| `se_industrial` | 2 | 2 | SE Portland |
| `felony_flats` | 4 | 2 | SE 82nd Ave |
| `lake_oswego` | 0 | 4 | South |

Note: `battleground` (4, -6) and `vantucky` (2, -4) differ by 2 on both axes — this is intentional. They are not adjacent on the world map (see REQ-WM-31).

- REQ-WM-4: Zone YAML MUST support optional `world_x` and `world_y` integer fields with struct tags `yaml:"world_x,omitempty"` and `yaml:"world_y,omitempty"`. The `Zone` struct MUST use `*int` pointer fields so that omitted values decode to `nil` rather than `0`.
- REQ-WM-5: The `Zone` struct MUST gain `WorldX *int` and `WorldY *int` pointer fields. A nil pointer MUST indicate the zone has no world map position.
- REQ-WM-6: All 16 zones MUST have `world_x` and `world_y` set per Section 2.1.
- REQ-WM-7: Zones with nil `WorldX` or `WorldY` MUST be excluded from the world map render and MUST NOT appear in `world_tiles`.

---

## 3. Proto Changes

### 3.1 MapRequest

`MapRequest` gains a `view` field at field number 1. Existing clients that send an empty `MapRequest{}` (no `view` field) will have `view` decoded as the empty string, which defaults to `"zone"` — preserving backward compatibility with the existing zone map behavior.

```protobuf
message MapRequest {
    string view = 1; // "zone" (default) | "world"
}
```

### 3.2 WorldZoneTile

New message for world map tiles:

```protobuf
message WorldZoneTile {
    string zone_id      = 1;
    string zone_name    = 2;
    int32  world_x      = 3;
    int32  world_y      = 4;
    bool   discovered   = 5;
    bool   current      = 6;
    string danger_level = 7;
}
```

### 3.3 MapResponse

`MapResponse` gains a `world_tiles` field at field number 2:

```protobuf
message MapResponse {
    repeated MapTile       tiles       = 1; // zone view (existing)
    repeated WorldZoneTile world_tiles = 2; // world view (new)
}
```

### 3.4 TravelRequest

New message for server-side fast travel. The `ClientMessage` oneof gains `travel` at field number 87 (the next sequential number after the existing `ready = 86` field):

```protobuf
message TravelRequest {
    string zone_id = 1; // target zone ID
}

// In ClientMessage oneof payload:
TravelRequest travel = 87;
```

`TravelRequest` triggers a server-side handler that validates preconditions and executes travel. On success, the server sends a room view event for the new room (same as any room transition). On failure, the server sends a message event with the appropriate error string.

- REQ-WM-8: `MapRequest` MUST gain a `view` string field at field number 1. An empty or absent value MUST default to `"zone"`. This addition is backward-compatible: existing clients sending empty `MapRequest{}` continue to receive zone map data.
- REQ-WM-9: A `WorldZoneTile` proto message MUST be added with fields: `zone_id`, `zone_name`, `world_x`, `world_y`, `discovered`, `current`, `danger_level`.
- REQ-WM-10: `MapResponse` MUST gain a `world_tiles` repeated field at field number `2`.
- REQ-WM-10A: A `TravelRequest` proto message MUST be added with a `zone_id` string field at field number 1.
- REQ-WM-10B: `ClientMessage` MUST gain a `travel` oneof variant holding a `TravelRequest` at field number `87`.

---

## 4. handleMap Changes

`handleMap` is updated to accept `*gamev1.MapRequest` in addition to `uid string`. The dispatch site in the `ClientMessage_Map` case MUST pass `p.Map` to `handleMap`. When `MapRequest.view == "world"`, `handleMap` builds `WorldZoneTile` entries for all zones with non-nil `WorldX`/`WorldY`.

- REQ-WM-11: `handleMap` MUST accept `(uid string, req *gamev1.MapRequest)`. The dispatch site at `case *gamev1.ClientMessage_Map` MUST pass `p.Map` as the second argument.
- REQ-WM-12: When `req.View == "world"`, `handleMap` MUST populate `MapResponse.world_tiles` for all zones with non-nil `WorldX`/`WorldY`.
- REQ-WM-13: `WorldZoneTile.discovered` MUST be set to `true` when `len(sess.AutomapCache[zone.ID]) > 0`, and `false` otherwise.
- REQ-WM-14: `WorldZoneTile.current` MUST be set to `true` for the zone whose ID matches the zone containing the player's current room.
- REQ-WM-15: `WorldZoneTile.zone_name` MUST be set to `zone.Name` and `WorldZoneTile.danger_level` MUST be set to `zone.DangerLevel`.
- REQ-WM-16: The zone view path in `handleMap` MUST remain unchanged when `req.View == "zone"` or is empty.

---

## 5. handleTravel

A new `handleTravel(uid string, req *gamev1.TravelRequest)` function enforces all fast travel preconditions server-side and executes relocation. Preconditions MUST be checked in the order listed below:

- REQ-WM-17: `handleTravel` MUST return a message event with `"That zone does not exist."` if `req.ZoneId` does not correspond to any zone in the world model. This check MUST occur before all other preconditions.
- REQ-WM-18: `handleTravel` MUST return a message event with `"You don't know how to get there."` if `len(sess.AutomapCache[req.ZoneId]) == 0`.
- REQ-WM-19: `handleTravel` MUST return a message event with `"You can't travel while in combat."` if `sess.Status == statusInCombat`.
- REQ-WM-20: `handleTravel` MUST return a message event with `"You can't travel while Wanted."` if any entry in `sess.WantedLevel` is non-zero.
- REQ-WM-21: `handleTravel` MUST return a message event with `"You're already there."` if the player's current room is in the target zone.
- REQ-WM-22: On successful travel, `handleTravel` MUST relocate the player to `targetZone.StartRoom`. (`Zone.Validate()` enforces that `StartRoom` is non-empty for all loaded zones; no fallback is required.)
- REQ-WM-23: On successful travel, `handleTravel` MUST send a console message event `"You make your way to <zone.Name>."` followed by a full room view event for the destination room.
- REQ-WM-24: All normal room-entry hooks MUST fire after travel relocation (NPC awareness, traps, guard aggro, etc.) — using the same room-entry path as normal movement.
- REQ-WM-25: The `ClientMessage_Travel` case MUST be added to the message dispatch switch, calling `handleTravel(uid, p.Travel)`.

---

## 6. World Map Rendering

`RenderWorldMap(resp *gamev1.MapResponse, width int) string` — new function in `text_renderer.go` alongside `RenderMap`.

The grid-building algorithm mirrors `RenderMap`: normalize tile coordinates to 0-based grid indices by subtracting the minimum `world_x`/`world_y` across all tiles, then compute rendered cell positions using `cellStride = 5` (4-char cell body + 1-char connector slot) and `rowStride = 2` (cell row + connector row).

- REQ-WM-26: `RenderWorldMap` MUST build a 2D grid by normalizing tile coordinates to 0-based indices and placing cells at column positions `col * 5` (cellStride = 4-char cell + 1-char connector slot) and row positions `row * 2` (rowStride = cell row + connector row), consistent with `RenderMap`.
- REQ-WM-27: Zone legend numbers MUST be assigned top-to-bottom, left-to-right, consistent with `RenderMap` room number assignment.
- REQ-WM-28: Discovered zones MUST render as `[ZZ]` (4 chars) colored with `DangerColor(tile.DangerLevel)`.
- REQ-WM-29: Undiscovered zones MUST render as `[??]` in light gray (`\033[37m`) with legend entry `???`.
- REQ-WM-30: The current zone MUST render as `<ZZ>` (angle brackets) instead of square brackets.
- REQ-WM-31: East/south connectors MUST be rendered between zones whose coordinates differ by exactly 2 on exactly one axis and 0 on the other. Pairs differing by 2 on both axes (diagonal) MUST NOT have a connector.
- REQ-WM-32: Layout MUST be two-column (grid left, legend right) at width ≥ 100 and single-column stacked at width < 100, consistent with `RenderMap`.

---

## 7. Modal Map Mode

### 7.1 Frontend State

All map modal state lives in the frontend `gameBridge` struct, not on `PlayerSession`.

- REQ-WM-33: `gameBridge` MUST gain a `mapMode bool` field. Its Go zero value is `false` (not in map mode).
- REQ-WM-34: `gameBridge` MUST gain a `mapView string` field. Its Go zero value is `""` (empty string); the frontend MUST treat `""` as equivalent to `"zone"` wherever `mapView` is read, consistent with how `MapRequest.view` defaults per REQ-WM-8.
- REQ-WM-35: `gameBridge` MUST gain a `mapSelectedZone string` field. Its Go zero value `""` means no zone is selected.
- REQ-WM-36: `gameBridge` MUST gain a `lastMapResponse *gamev1.MapResponse` field. Its Go zero value is `nil`. It MUST be updated with every `MapResponse` received while in map mode, and used to redraw the map on terminal resize without issuing a new RPC.
- REQ-WM-37: `PlayerSession` MUST NOT gain map modal fields.

### 7.2 Entering Map Mode

- REQ-WM-38: The `map` command and `map zone` MUST set `mapMode = true` and `mapView = "zone"`.
- REQ-WM-39: `map world` MUST set `mapMode = true` and `mapView = "world"`.
- REQ-WM-40: On entering map mode, the frontend MUST set `mapMode = true` first, then send a `MapRequest{view: mapView}` to the server, clear the console region, and render the received `MapResponse` in the console region using `RenderMap` (zone view) or `RenderWorldMap` (world view). Terminal resize events MUST be treated as no-ops while `mapMode` is true but `lastMapResponse` is still `nil` (i.e., the initial RPC response has not yet arrived).
- REQ-WM-41: On entering map mode, the prompt MUST change to: `[MAP] z=zone  w=world  <num/name>=select  t=travel  q=exit`.

### 7.3 Map Mode Input Handling

- REQ-WM-42: While `mapMode` is true, the `commandLoop` MUST route all input to the map mode handler before the normal command dispatcher.
- REQ-WM-43: The map mode handler MUST match reserved key aliases (`z`, `zone`, `w`, `world`, `t`, `travel`, `q`, `quit`, `esc`) before performing zone name prefix resolution. This prevents zone names beginning with reserved characters from shadowing the key bindings.
- REQ-WM-44: `z` or `zone` MUST set `mapView = "zone"`, send a `MapRequest{view:"zone"}` to refresh `lastMapResponse`, and redraw the console region with the zone map.
- REQ-WM-45: `w` or `world` MUST set `mapView = "world"`, send a `MapRequest{view:"world"}` to refresh `lastMapResponse`, and redraw the console region with the world map.
- REQ-WM-46: In world view, input that does not match a reserved key MUST be treated as a zone selector: a legend number or case-insensitive prefix of a zone name MUST set `mapSelectedZone` to the matching zone ID. If multiple zones share the same prefix, the first match in lexicographic zone ID order MUST be selected.
- REQ-WM-47: After a zone is selected via REQ-WM-46, the prompt MUST update to: `[MAP] Selected: <Zone Name> (<danger_level>)  t=travel  q=exit`.
- REQ-WM-48: `mapSelectedZone` MUST be preserved when the player switches between zone view and world view. It MUST be cleared only on map mode exit (REQ-WM-52).
- REQ-WM-49: `t` or `travel` in map mode MUST send a `TravelRequest{zone_id: mapSelectedZone}` to the server if `mapSelectedZone` is non-empty. Travel is permitted regardless of current `mapView`. If `mapSelectedZone` is empty, the frontend MUST write `"Select a zone first."` to the console without sending an RPC.
- REQ-WM-50: `q`, `quit`, or `esc` MUST exit map mode (see Section 7.4). A bare ESC byte (0x1B) MUST trigger map mode exit; incomplete escape sequences (ESC followed by additional bytes, e.g., arrow keys) MUST be silently consumed and MUST NOT exit map mode or be forwarded to the command dispatcher.
- REQ-WM-51: Any input in map mode that does not match a reserved key and is not a valid zone selector (in world view) or is input while in zone view other than reserved keys MUST cause the frontend to write `"Unknown map command. Press q to exit."` to the console (via `conn.WriteConsole`) and MUST NOT be forwarded to the normal command dispatcher.

### 7.4 Exiting Map Mode

- REQ-WM-52: While `mapMode` is true, any `MessageEvent` received from the server MUST be displayed in the console region via `conn.WriteConsole` without exiting map mode, regardless of the message origin. The player remains in map mode.
- REQ-WM-53: On receiving a `RoomView` event from the server while in map mode (indicating successful travel), the frontend MUST exit map mode before displaying the room view.
- REQ-WM-54: On map mode exit (via `q`/`quit`/`esc` or REQ-WM-53 travel success), `mapMode` MUST be set to `false` and `mapSelectedZone` MUST be cleared to `""`.
- REQ-WM-55: On map mode exit, the console region MUST be redrawn by calling `conn.redrawConsole()`. This method exists on `telnet.Conn`; if called from outside the telnet package it MUST be exported as `RedrawConsole()`.
- REQ-WM-56: On terminal resize while `mapMode` is true, the frontend MUST redraw the map in the console region using `lastMapResponse` at the new terminal width without issuing a new RPC.

---

## 8. Travel Command Outside Map Mode

- REQ-WM-57: The `travel <zone-name>` command MUST work outside map mode by resolving a zone name to a zone ID and sending a `TravelRequest{zone_id: resolvedZoneID}` to the server.
- REQ-WM-58: Zone name resolution for the `travel` command MUST use case-insensitive prefix matching against zone names for all zones with non-nil `WorldX`/`WorldY` (world-map-visible zones only). Zones without world coordinates are not valid `travel` targets.
- REQ-WM-59: If multiple zones share the same prefix, the first match in lexicographic zone ID order MUST be used. This is the same tiebreak rule as REQ-WM-46 (map mode zone selection). If no zone matches, the frontend MUST write `"No such zone."` to the console without sending an RPC.
- REQ-WM-60: The same server-side `handleTravel` preconditions (REQ-WM-17 through REQ-WM-21) MUST apply regardless of whether travel is initiated from inside or outside map mode.
- REQ-WM-61: If `travel` is typed with no argument (empty zone name), the frontend MUST write `"Usage: travel <zone name>"` to the console without sending an RPC.

---

## 9. Requirements Summary

- REQ-WM-1: A zone MUST be considered discovered if `len(sess.AutomapCache[zoneID]) > 0`.
- REQ-WM-2: `reveal_zone` MUST be wired in `GameServiceServer` initialization to insert all rooms into `character_map_rooms` and `sess.AutomapCache[zoneID]`.
- REQ-WM-3: `reveal_zone` MUST use idempotent inserts (`ON CONFLICT DO NOTHING`).
- REQ-WM-4: Zone YAML MUST support optional `world_x` and `world_y` integer fields with struct tags `yaml:"world_x,omitempty"` and `yaml:"world_y,omitempty"`. The `Zone` struct MUST use `*int` pointer fields so omitted values decode to `nil`.
- REQ-WM-5: The `Zone` struct MUST gain `WorldX *int` and `WorldY *int` pointer fields. Nil MUST indicate no world map position.
- REQ-WM-6: All 16 zones MUST have `world_x` and `world_y` set per Section 2.1.
- REQ-WM-7: Zones with nil `WorldX` or `WorldY` MUST be excluded from world map render and `world_tiles`.
- REQ-WM-8: `MapRequest` MUST gain a `view` string field at field number 1; empty or absent MUST default to `"zone"`. This MUST be backward-compatible with clients sending empty `MapRequest{}`.
- REQ-WM-9: A `WorldZoneTile` proto message MUST be added per Section 3.2.
- REQ-WM-10: `MapResponse` MUST gain a `world_tiles` repeated field at field number `2`.
- REQ-WM-10A: A `TravelRequest` proto message MUST be added with a `zone_id` string field at field number 1.
- REQ-WM-10B: `ClientMessage` MUST gain a `travel` oneof variant holding a `TravelRequest` at field number `87`.
- REQ-WM-11: `handleMap` MUST accept `(uid string, req *gamev1.MapRequest)`. The dispatch site MUST pass `p.Map` as the second argument.
- REQ-WM-12: When `req.View == "world"`, `handleMap` MUST populate `world_tiles` for all zones with non-nil `WorldX`/`WorldY`.
- REQ-WM-13: `WorldZoneTile.discovered` MUST be `true` when `len(sess.AutomapCache[zone.ID]) > 0`.
- REQ-WM-14: `WorldZoneTile.current` MUST be `true` for the zone containing the player's current room.
- REQ-WM-15: `WorldZoneTile.zone_name` MUST be `zone.Name`; `WorldZoneTile.danger_level` MUST be `zone.DangerLevel`.
- REQ-WM-16: The zone view path MUST remain unchanged when `req.View == "zone"` or is empty.
- REQ-WM-17: `handleTravel` MUST return `"That zone does not exist."` if `req.ZoneId` is not in the world model. This check MUST be first.
- REQ-WM-18: `handleTravel` MUST return `"You don't know how to get there."` if the zone is undiscovered.
- REQ-WM-19: `handleTravel` MUST return `"You can't travel while in combat."` if `sess.Status == statusInCombat`.
- REQ-WM-20: `handleTravel` MUST return `"You can't travel while Wanted."` if any `WantedLevel` entry is non-zero.
- REQ-WM-21: `handleTravel` MUST return `"You're already there."` if the player is already in the target zone.
- REQ-WM-22: On success, `handleTravel` MUST relocate the player to `targetZone.StartRoom`.
- REQ-WM-23: On success, `handleTravel` MUST send `"You make your way to <zone.Name>."` and a full room view event.
- REQ-WM-24: All normal room-entry hooks MUST fire after travel relocation.
- REQ-WM-25: The `ClientMessage_Travel` case MUST be added to the dispatch switch, calling `handleTravel(uid, p.Travel)`.
- REQ-WM-26: `RenderWorldMap` MUST build a 2D grid using 0-based coordinate normalization, `col * 5` (cellStride = 4-char cell + 1-char connector slot), and `row * 2` (rowStride = cell row + connector row), consistent with `RenderMap`.
- REQ-WM-27: Zone legend numbers MUST be assigned top-to-bottom, left-to-right.
- REQ-WM-28: Discovered zones MUST render as `[ZZ]` colored with `DangerColor(tile.DangerLevel)`.
- REQ-WM-29: Undiscovered zones MUST render as `[??]` in light gray with legend entry `???`.
- REQ-WM-30: The current zone MUST render as `<ZZ>`.
- REQ-WM-31: Connectors MUST be rendered between zones differing by exactly 2 on exactly one axis and 0 on the other. Diagonal pairs MUST NOT have a connector.
- REQ-WM-32: Layout MUST be two-column at width ≥ 100 and single-column at width < 100.
- REQ-WM-33: `gameBridge` MUST gain `mapMode bool` (Go zero value `false`).
- REQ-WM-34: `gameBridge` MUST gain `mapView string` (Go zero value `""`; frontend MUST treat `""` as `"zone"` wherever `mapView` is read).
- REQ-WM-35: `gameBridge` MUST gain `mapSelectedZone string` (Go zero value `""`).
- REQ-WM-36: `gameBridge` MUST gain `lastMapResponse *gamev1.MapResponse` (Go zero value `nil`), updated on every `MapResponse` received in map mode.
- REQ-WM-37: `PlayerSession` MUST NOT gain map modal fields.
- REQ-WM-38: `map` and `map zone` MUST set `mapMode = true` and `mapView = "zone"`.
- REQ-WM-39: `map world` MUST set `mapMode = true` and `mapView = "world"`.
- REQ-WM-40: On entering map mode, the frontend MUST set `mapMode = true` first, send a `MapRequest{view: mapView}`, clear the console region, and render the received `MapResponse` using `RenderMap` (zone) or `RenderWorldMap` (world). Resize events MUST be no-ops while `lastMapResponse == nil`.
- REQ-WM-41: On entering map mode, the prompt MUST change to: `[MAP] z=zone  w=world  <num/name>=select  t=travel  q=exit`.
- REQ-WM-42: While `mapMode` is true, the `commandLoop` MUST route all input to the map mode handler before the normal command dispatcher.
- REQ-WM-43: Reserved key aliases MUST be matched before zone name prefix resolution.
- REQ-WM-44: `z` or `zone` MUST switch to zone view, refresh `lastMapResponse`, and redraw.
- REQ-WM-45: `w` or `world` MUST switch to world view, refresh `lastMapResponse`, and redraw.
- REQ-WM-46: In world view, non-reserved input MUST be treated as a zone selector; the first matching zone in lexicographic zone ID order MUST be selected.
- REQ-WM-47: After selection, the prompt MUST update to: `[MAP] Selected: <Zone Name> (<danger_level>)  t=travel  q=exit`.
- REQ-WM-48: `mapSelectedZone` MUST be preserved across view switches and cleared only on map mode exit.
- REQ-WM-49: `t` or `travel` in map mode MUST send a `TravelRequest` for `mapSelectedZone` (permitted from any view), or write `"Select a zone first."` if none is selected.
- REQ-WM-50: `q`, `quit`, or `esc` MUST exit map mode. A bare ESC byte (0x1B) MUST exit; incomplete escape sequences (ESC + more bytes) MUST be silently consumed without exiting.
- REQ-WM-51: All other input in map mode MUST write `"Unknown map command. Press q to exit."` to console and MUST NOT reach the normal command dispatcher.
- REQ-WM-52: While `mapMode` is true, any `MessageEvent` from the server MUST be displayed in the console without exiting map mode.
- REQ-WM-53: On receiving a `RoomView` event while in map mode, the frontend MUST exit map mode before displaying the room view.
- REQ-WM-54: On map mode exit, `mapMode` MUST be `false` and `mapSelectedZone` MUST be cleared.
- REQ-WM-55: On map mode exit, the console region MUST be redrawn via `conn.redrawConsole()` (export as `RedrawConsole()` if called from outside the telnet package).
- REQ-WM-56: On terminal resize while in map mode, the map MUST be redrawn using `lastMapResponse` at the new width without a new RPC.
- REQ-WM-57: The `travel <zone-name>` command MUST work outside map mode by sending a `TravelRequest` to the server.
- REQ-WM-58: Zone name resolution for `travel` MUST use case-insensitive prefix matching against world-map-visible zones only (non-nil `WorldX`/`WorldY`); ties broken by lexicographic zone ID order (same as REQ-WM-46).
- REQ-WM-59: If no zone matches, the frontend MUST write `"No such zone."` to the console without sending an RPC.
- REQ-WM-60: Server-side `handleTravel` preconditions MUST apply regardless of whether travel is initiated inside or outside map mode.
- REQ-WM-61: If `travel` is typed with no argument (empty zone name), the frontend MUST write `"Usage: travel <zone name>"` to the console without sending an RPC.
