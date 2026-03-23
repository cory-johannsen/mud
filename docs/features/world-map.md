# World Map / Fast Travel

Adds a world-level map view and instant fast travel between discovered zones. The existing `map` command enters a modal state that takes over the console region. Players toggle between zone view and world view, select a destination zone, and initiate fast travel.

Design spec: `docs/superpowers/specs/2026-03-21-world-map-design.md`

## Requirements

- [ ] REQ-WM-1: A zone MUST be considered discovered if `len(sess.AutomapCache[zoneID]) > 0`.
- [ ] REQ-WM-2: `reveal_zone` MUST be wired in `GameServiceServer` initialization to insert all rooms into `character_map_rooms` and `sess.AutomapCache[zoneID]`.
- [ ] REQ-WM-3: `reveal_zone` MUST use idempotent inserts (`ON CONFLICT DO NOTHING`).
- [ ] REQ-WM-4: Zone YAML MUST support optional `world_x` and `world_y` integer fields with struct tags `yaml:"world_x,omitempty"` and `yaml:"world_y,omitempty"`. The `Zone` struct MUST use `*int` pointer fields so omitted values decode to `nil`.
- [ ] REQ-WM-5: The `Zone` struct MUST gain `WorldX *int` and `WorldY *int` pointer fields. Nil MUST indicate no world map position.
- [ ] REQ-WM-6: All 16 zones MUST have `world_x` and `world_y` set per Section 2.1 of the design spec.
- [ ] REQ-WM-7: Zones with nil `WorldX` or `WorldY` MUST be excluded from world map render and `world_tiles`.
- [ ] REQ-WM-8: `MapRequest` MUST gain a `view` string field at field number 1; empty or absent MUST default to `"zone"`. This MUST be backward-compatible with clients sending empty `MapRequest{}`.
- [ ] REQ-WM-9: A `WorldZoneTile` proto message MUST be added per Section 3.2 of the design spec.
- [ ] REQ-WM-10: `MapResponse` MUST gain a `world_tiles` repeated field at field number `2`.
- [ ] REQ-WM-10A: A `TravelRequest` proto message MUST be added with a `zone_id` string field at field number 1.
- [ ] REQ-WM-10B: `ClientMessage` MUST gain a `travel` oneof variant holding a `TravelRequest` at field number `112` (field 87 is occupied by `BrowseRequest`).
- [ ] REQ-WM-11: `handleMap` MUST accept `(uid string, req *gamev1.MapRequest)`. The dispatch site MUST pass `p.Map` as the second argument.
- [ ] REQ-WM-12: When `req.View == "world"`, `handleMap` MUST populate `world_tiles` for all zones with non-nil `WorldX`/`WorldY`.
- [ ] REQ-WM-13: `WorldZoneTile.discovered` MUST be `true` when `len(sess.AutomapCache[zone.ID]) > 0`.
- [ ] REQ-WM-14: `WorldZoneTile.current` MUST be `true` for the zone containing the player's current room.
- [ ] REQ-WM-15: `WorldZoneTile.zone_name` MUST be `zone.Name`; `WorldZoneTile.danger_level` MUST be `zone.DangerLevel`.
- [ ] REQ-WM-16: The zone view path MUST remain unchanged when `req.View == "zone"` or is empty.
- [ ] REQ-WM-17: `handleTravel` MUST return `"That zone does not exist."` if `req.ZoneId` is not in the world model. This check MUST be first.
- [ ] REQ-WM-18: `handleTravel` MUST return `"You don't know how to get there."` if the zone is undiscovered.
- [ ] REQ-WM-19: `handleTravel` MUST return `"You can't travel while in combat."` if `sess.Status == statusInCombat`.
- [ ] REQ-WM-20: `handleTravel` MUST return `"You can't travel while Wanted."` if any `WantedLevel` entry is non-zero.
- [ ] REQ-WM-21: `handleTravel` MUST return `"You're already there."` if the player is already in the target zone.
- [ ] REQ-WM-22: On success, `handleTravel` MUST relocate the player to `targetZone.StartRoom`.
- [ ] REQ-WM-23: On success, `handleTravel` MUST send `"You make your way to <zone.Name>."` and a full room view event.
- [ ] REQ-WM-24: All normal room-entry hooks MUST fire after travel relocation.
- [ ] REQ-WM-25: The `ClientMessage_Travel` case MUST be added to the dispatch switch, calling `handleTravel(uid, p.Travel)`.
- [ ] REQ-WM-26: `RenderWorldMap` MUST build a 2D grid using 0-based coordinate normalization, `col * 5` (cellStride = 4-char cell + 1-char connector slot), and `row * 2` (rowStride = cell row + connector row), consistent with `RenderMap`.
- [ ] REQ-WM-27: Zone legend numbers MUST be assigned top-to-bottom, left-to-right.
- [ ] REQ-WM-28: Discovered zones MUST render as `[ZZ]` colored with `DangerColor(tile.DangerLevel)`.
- [ ] REQ-WM-29: Undiscovered zones MUST render as `[??]` in light gray with legend entry `???`.
- [ ] REQ-WM-30: The current zone MUST render as `<ZZ>`.
- [ ] REQ-WM-31: Connectors MUST be rendered between zones differing by exactly 2 on exactly one axis and 0 on the other. Diagonal pairs MUST NOT have a connector.
- [ ] REQ-WM-32: Layout MUST be two-column at width >= 100 and single-column at width < 100.
- [ ] REQ-WM-33: `gameBridge` MUST gain `mapMode bool` (Go zero value `false`).
- [ ] REQ-WM-34: `gameBridge` MUST gain `mapView string` (Go zero value `""`; frontend MUST treat `""` as `"zone"` wherever `mapView` is read).
- [ ] REQ-WM-35: `gameBridge` MUST gain `mapSelectedZone string` (Go zero value `""`).
- [ ] REQ-WM-36: `gameBridge` MUST gain `lastMapResponse *gamev1.MapResponse` (Go zero value `nil`), updated on every `MapResponse` received in map mode.
- [ ] REQ-WM-37: `PlayerSession` MUST NOT gain map modal fields.
- [ ] REQ-WM-38: `map` and `map zone` MUST set `mapMode = true` and `mapView = "zone"`.
- [ ] REQ-WM-39: `map world` MUST set `mapMode = true` and `mapView = "world"`.
- [ ] REQ-WM-40: On entering map mode, the frontend MUST set `mapMode = true` first, send a `MapRequest{view: mapView}`, clear the console region, and render the received `MapResponse` using `RenderMap` (zone) or `RenderWorldMap` (world). Resize events MUST be no-ops while `lastMapResponse == nil`.
- [ ] REQ-WM-41: On entering map mode, the prompt MUST change to: `[MAP] z=zone  w=world  <num/name>=select  t=travel  q=exit`.
- [ ] REQ-WM-42: While `mapMode` is true, the `commandLoop` MUST route all input to the map mode handler before the normal command dispatcher.
- [ ] REQ-WM-43: Reserved key aliases MUST be matched before zone name prefix resolution.
- [ ] REQ-WM-44: `z` or `zone` MUST switch to zone view, refresh `lastMapResponse`, and redraw.
- [ ] REQ-WM-45: `w` or `world` MUST switch to world view, refresh `lastMapResponse`, and redraw.
- [ ] REQ-WM-46: In world view, non-reserved input MUST be treated as a zone selector; the first matching zone in lexicographic zone ID order MUST be selected.
- [ ] REQ-WM-47: After selection, the prompt MUST update to: `[MAP] Selected: <Zone Name> (<danger_level>)  t=travel  q=exit`.
- [ ] REQ-WM-48: `mapSelectedZone` MUST be preserved across view switches and cleared only on map mode exit.
- [ ] REQ-WM-49: `t` or `travel` in map mode MUST send a `TravelRequest` for `mapSelectedZone` (permitted from any view), or write `"Select a zone first."` if none is selected.
- [ ] REQ-WM-50: `q`, `quit`, or `esc` MUST exit map mode. A bare ESC byte (0x1B) MUST exit; incomplete escape sequences (ESC + more bytes) MUST be silently consumed without exiting.
- [ ] REQ-WM-51: All other input in map mode MUST write `"Unknown map command. Press q to exit."` to console and MUST NOT reach the normal command dispatcher.
- [ ] REQ-WM-52: While `mapMode` is true, any `MessageEvent` from the server MUST be displayed in the console without exiting map mode.
- [ ] REQ-WM-53: On receiving a `RoomView` event while in map mode, the frontend MUST exit map mode before displaying the room view.
- [ ] REQ-WM-54: On map mode exit, `mapMode` MUST be `false` and `mapSelectedZone` MUST be cleared.
- [ ] REQ-WM-55: On map mode exit, the console region MUST be redrawn via `conn.redrawConsole()` (export as `RedrawConsole()` if called from outside the telnet package).
- [ ] REQ-WM-56: On terminal resize while in map mode, the map MUST be redrawn using `lastMapResponse` at the new width without a new RPC.
- [ ] REQ-WM-57: The `travel <zone-name>` command MUST work outside map mode by sending a `TravelRequest` to the server.
- [ ] REQ-WM-58: Zone name resolution for `travel` MUST use case-insensitive prefix matching against world-map-visible zones only (non-nil `WorldX`/`WorldY`); ties broken by lexicographic zone ID order (same as REQ-WM-46).
- [ ] REQ-WM-59: If no zone matches, the frontend MUST write `"No such zone."` to the console without sending an RPC.
- [ ] REQ-WM-60: Server-side `handleTravel` preconditions MUST apply regardless of whether travel is initiated inside or outside map mode.
- [ ] REQ-WM-61: If `travel` is typed with no argument (empty zone name), the frontend MUST write `"Usage: travel <zone name>"` to the console without sending an RPC.
