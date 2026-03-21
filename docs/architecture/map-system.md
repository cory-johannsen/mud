# Map System Architecture

## Overview

The map system tracks which rooms each player has explored and renders an ASCII automap grid
color-coded by danger level. Discovery is per-character and persisted to PostgreSQL.

---

## Database Schema

### `character_map_rooms`

```sql
CREATE TABLE IF NOT EXISTS character_map_rooms (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    zone_id      VARCHAR(64) NOT NULL,
    room_id      VARCHAR(64) NOT NULL,
    PRIMARY KEY (character_id, zone_id, room_id)
);
```

Writes use INSERT ... ON CONFLICT DO NOTHING (idempotent). There is no DELETE path;
rooms once explored remain explored permanently.

---

## AutomapRepository

**File:** `internal/storage/postgres/automap.go`

| Method | Signature | Description |
|--------|-----------|-------------|
| Insert | `Insert(ctx, characterID int64, zoneID, roomID string) error` | Idempotent insert of a discovered room. |
| LoadAll | `LoadAll(ctx, characterID int64) (map[string]map[string]bool, error)` | Returns all explored rooms keyed by zoneID → roomID → true. |

---

## PlayerSession.AutomapCache

**Type:** `map[string]map[string]bool` (zoneID → roomID → explored)

Populated at session load via `AutomapRepository.LoadAll`. Updated in memory on each move
before the DB write. The cache is the authoritative in-memory source for `handleMap`.

---

## MapTile Proto Fields

**Message:** `MapTile` in `api/proto/game/v1/game.proto`

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| room_id | 1 | string | Room identifier |
| room_name | 2 | string | Display name |
| x | 3 | int32 | Grid X coordinate |
| y | 4 | int32 | Grid Y coordinate |
| current | 5 | bool | True if this is the player's current room |
| exits | 6 | repeated string | Available exit directions |
| danger_level | 7 | string | Effective danger level string (safe/sketchy/dangerous/all_out_war) |

---

## Discovery Flow

```
handleMove
  └─ move player to new room
  └─ AutomapRepository.Insert(characterID, zoneID, roomID)   ← idempotent DB write
  └─ sess.AutomapCache[zoneID][roomID] = true                ← in-memory update
  └─ XP award for new discovery (if first time)
  └─ RollRoomTrap(effectiveLevel, room.RoomTrapChance, dice)  ← trap roll
  └─ InitiateGuardCombat if WantedLevel >= 2                  ← guard enforcement

handleMap
  └─ iterate sess.AutomapCache[zoneID]                        ← explored rooms only
  └─ for each explored room: build MapTile with DangerLevel
  └─ return MapResponse{Tiles: tiles}
```

---

## RenderMap Rendering Pipeline

**Function:** `RenderMap(resp *gamev1.MapResponse, width int) string`
**File:** `internal/frontend/handlers/text_renderer.go`

1. Build a coordinate grid from `resp.Tiles` (x, y → tile index).
2. Find bounding box (min/max x, y).
3. For each cell in the bounding box:
   - If the cell has a tile (explored): render `[N]` wrapped in ANSI color from `DangerColor(tile.DangerLevel)`.
   - If no tile: render empty space.
4. Mark the current room with a special indicator (e.g., `[*]`).
5. Append a legend showing the exit directions.

---

## Danger Level Color Coding

| Danger Level | Color | ANSI Escape |
|--------------|-------|-------------|
| `safe` | Green | `\033[32m` |
| `sketchy` | Yellow | `\033[33m` |
| `dangerous` | Orange | `\033[38;5;208m` |
| `all_out_war` | Red | `\033[31m` |
| unexplored / unknown | Light Gray | `\033[37m` |

Unexplored rooms (not in `AutomapCache`) are not rendered in the current implementation.
If future requirements call for rendering unexplored rooms (e.g., from zone metadata),
they MUST use the light gray color regardless of their actual danger level (REQ-DL-10).

---

## Explored vs. Unexplored

- A room is **explored** if it appears in `sess.AutomapCache[zoneID][roomID]`.
- Only explored rooms are included in `handleMap`'s tile list.
- Unexplored rooms are not rendered; they simply do not appear on the map.
- This invariant is enforced at the `handleMap` layer, not the renderer.
