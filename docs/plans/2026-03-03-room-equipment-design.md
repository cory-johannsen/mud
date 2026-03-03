# Room Equipment Design

**Date:** 2026-03-03
**Feature:** Room Equipment — permanent fixtures and auto-spawning items in rooms

---

## Goal

Some items and equipment can permanently exist in a room (e.g. a water fountain), or generate automatically in a room up to a maximum count with a configurable respawn timer. Immovable items are interactive: using them invokes a Lua script that produces an effect on the player or environment.

---

## Architecture

- `RoomEquipmentConfig` added to the `Room` struct alongside existing `Spawns []RoomSpawnConfig`
- `RoomEquipmentManager` — new in-memory manager owning live `EquipmentInstance` entries and a respawn scheduler (goroutine ticker)
- Each config has: `item_id`, `max_count`, `respawn_after` (duration), `immovable` (bool), `script` (Lua path, optional)
- YAML zone files extended with an `equipment:` block per room
- Editor commands (`roomequip add/remove/list/modify`) mutate the manager at runtime; changes written back to zone YAML for persistence across restarts
- `handleLook` extended to include room equipment instances in `RoomView` (separate from `floor_items`)
- New `handleUse` gRPC dispatch for using an equipment item — invokes Lua script with player context
- `RoomView` proto gets a new `repeated RoomEquipmentItem equipment` field (with `immovable` and `usable` flags)

**Selected approach:** New `RoomEquipmentManager` (mirrors existing NPC spawn pattern — clean SRP, separate runtime state from config).

---

## Data Schema

### Go structs

```go
// RoomEquipmentConfig — static config loaded from YAML
type RoomEquipmentConfig struct {
    ItemID       string        // references ItemDef.ID
    MaxCount     int           // max live instances in room
    RespawnAfter time.Duration // 0 = permanent (immovable stays forever)
    Immovable    bool          // cannot be picked up
    Script       string        // Lua script path for use effect (optional)
}

// EquipmentInstance — runtime live instance
type EquipmentInstance struct {
    InstanceID string
    ConfigIdx  int    // index into Room.Equipment slice
    ItemDefID  string
    RoomID     string
    Immovable  bool
    Script     string
}
```

### Proto additions

```proto
message RoomEquipmentItem {
    string instance_id = 1;
    string name        = 2;
    int32  quantity    = 3;
    bool   immovable   = 4;
    bool   usable      = 5;  // has a script attached
}
// RoomView gets: repeated RoomEquipmentItem equipment = 11;
```

### YAML per room

```yaml
equipment:
  - item_id: water_fountain
    max_count: 1
    respawn_after: "0s"   # 0 = permanent, never removed
    immovable: true
    script: content/scripts/items/water_fountain.lua
  - item_id: medkit
    max_count: 2
    respawn_after: "5m"
    immovable: false
```

### State (in-memory only)

Runtime state is not persisted to the database. On server restart the manager re-initializes from zone YAML configs, spawning full counts immediately.

---

## Testing Plan

- **RoomEquipmentManager**: property tests for spawn/respawn/pickup across arbitrary room/item combinations; table tests for immovable item behavior (cannot be removed via pickup)
- **RoomEquipmentConfig YAML loading**: test all fields parse correctly; zero `respawn_after` treated as permanent
- **Lua script invocation**: test that `use` on an item with a script fires the script with correct player context; test no-op when no script attached
- **Editor commands**: TDD for add/remove/list/modify with property tests for invariants (count never exceeds max, immovable flag preserved)
- **RoomView proto**: verify `equipment` field populated correctly in `handleLook`; separate from `floor_items`
- **`TestAllCommandHandlersAreWired`** must pass after editor commands added
