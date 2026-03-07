# Map Bugs Design

## Bug 1: `interact zone_map` requires ItemDefID instead of descriptive name

### Problem

`interact Zone Map` fails because `GetInstance` in `internal/game/inventory/room_equipment.go` only matches by UUID then ItemDefID. The player-visible name is the `description` field in YAML, which is not currently stored or matched.

### Fix

**`internal/game/world/model.go`** — add `Description string` to `RoomEquipmentConfig`:
```go
type RoomEquipmentConfig struct {
    ItemID      string `yaml:"item_id"`
    Script      string `yaml:"script"`
    Description string `yaml:"description"`
}
```

**`internal/game/inventory/room_equipment.go`** — add `Description string` to `EquipmentInstance` and add 3rd match tier in `GetInstance`:
```go
type EquipmentInstance struct {
    UUID        string
    ItemDefID   string
    Description string
    Script      string
}

func (re *RoomEquipment) GetInstance(query string) (*EquipmentInstance, bool) {
    for _, it := range re.items {
        if it.UUID == query || it.ItemDefID == query || strings.EqualFold(it.Description, query) {
            return it, true
        }
    }
    return nil, false
}
```

**All 15 zone YAML files** — add `description: Zone Map` to each zone_map equipment entry. Example:
```yaml
equipment:
  - item_id: zone_map
    script: zone_map_use
    description: Zone Map
```

The existing ItemDefID match (`zone_map`) continues to work as a fallback.

---

## Bug 2: Using zone map does not populate player's automap

### Problem

`use zone_map` → Lua `engine.map.reveal_zone(uid, zoneID)` → `scriptMgr.RevealZoneMap(uid, zoneID)` — the callback is wired in `main.go` and calls `automapRepo.BulkInsert` + updates `sess.AutomapCache`. Whether the map command sees the update in the same session is unverified.

### Investigation Plan

Add diagnostic logging at each boundary before proposing any fix:

1. Log entry to `RevealZoneMap` callback (uid + zoneID)
2. Log session lookup result (found/not-found)
3. Log `automapRepo.BulkInsert` row count and error
4. Log `sess.AutomapCache` size before/after
5. Log cache hit/miss when `map` command reads

Run once with logging, identify the failing boundary.

### Fix Strategy

Based on the most likely failure modes:

- **Session not found**: The uid passed from Lua may not match the session key. Verify uid format.
- **BulkInsert succeeds but cache not updated**: Ensure `sess.AutomapCache` is updated after DB write in the same goroutine.
- **`map` command rebuilds from DB not cache**: Ensure `map` handler reads `sess.AutomapCache` directly, not a fresh DB query.
- **DB write is correct but cache stale**: After `BulkInsert`, assign updated room IDs into `sess.AutomapCache` so the current session reflects the change without requiring re-login.

The fix ensures that after `RevealZoneMap`:
1. DB is written (all zone rooms inserted for this player)
2. `sess.AutomapCache` is updated in-place with the new room IDs
3. The `map` command reads from `sess.AutomapCache` and sees the new rooms immediately
