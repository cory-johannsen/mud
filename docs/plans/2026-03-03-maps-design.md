# Maps Design

**Date:** 2026-03-03
**Feature:** Player automap — persisted per-character exploration map with `map` command

---

## Goal

Players have an automap that grows as they explore. It can be consulted with the `map` command. Zone entrance safe areas contain a zone map item that, when used, bulk-reveals all rooms and exits for that zone (but not equipment or POIs). The automap persists across sessions via PostgreSQL.

---

## Architecture

Four components:

**1. Room coordinates** — Add required `MapX int` / `MapY int` to `Room` (yaml: `map_x`, `map_y`). Zone loader fails fast if any room is missing coordinates. The `map` command renders directly from stored coordinates — no graph traversal.

**2. Automap persistence** — New DB table `character_map_rooms(character_id, zone_id, room_id, PRIMARY KEY(...))`. `AutomapRepository` handles insert-on-discover and bulk-load. An in-memory `AutomapCache` (per session) is populated at login and written through on each new room discovery.

**3. Zone map item** — A `[fixed]` room equipment item using the existing system. Lua script `content/scripts/items/zone_map.lua` calls a new scripting hook `reveal_zone_map(player, zoneID)` which bulk-inserts all room IDs for the zone into the character's automap (rooms and exits only — no equipment or POIs).

**4. `map` command** — Full CMD pipeline (CMD-1 through CMD-7). Renders hybrid ASCII grid + numbered legend. Current room marked `[@]`, discovered rooms `[#]`, exits as `---` / `|`. Legend lists room number → name.

**Selected approach:** Mandatory explicit coordinates — every room must declare `map_x`/`map_y` in YAML; zone load fails fast if any room is missing them. No inference.

---

## Data Schema

### DB migration

```sql
CREATE TABLE character_map_rooms (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    zone_id      TEXT   NOT NULL,
    room_id      TEXT   NOT NULL,
    PRIMARY KEY (character_id, zone_id, room_id)
);
```

### Room struct additions

```go
MapX int `yaml:"map_x"`
MapY int `yaml:"map_y"`
```

### Proto additions

```proto
message MapRequest {}

message MapTile {
    string room_id   = 1;
    string room_name = 2;
    int32  x         = 3;
    int32  y         = 4;
    bool   current   = 5;
    repeated string exits = 6;  // "north","south","east","west"
}

message MapResponse {
    repeated MapTile tiles = 1;
}
```

`MapRequest` added to `ClientMessage` oneof. `MapResponse` added to `ServerMessage` oneof.

### State

`AutomapCache` is in-memory per session, written through to PostgreSQL on each new discovery. Populated from DB at login.

---

## Testing Plan

- **Room coordinates** — table tests: zone load fails if any room is missing `map_x`/`map_y`; all rooms in valid zone have non-overlapping coordinates
- **AutomapRepository** — table tests: insert new room, duplicate insert is idempotent, load returns all discovered rooms for character
- **AutomapCache** — property tests: discover N rooms in any order, cache contains exactly the discovered set; bulk-reveal adds all zone rooms
- **`map` command** — table tests: empty map renders gracefully, single room, linear corridor, branching rooms; current room always `[@]`
- **Text renderer** — table tests: ASCII grid dimensions, legend numbering, exit connectors between adjacent rooms
- **Zone map Lua script** — test that `use` on zone_map item calls `reveal_zone_map` with correct zoneID and player context
- **`TestAllCommandHandlersAreWired`** must pass after `map` command added
