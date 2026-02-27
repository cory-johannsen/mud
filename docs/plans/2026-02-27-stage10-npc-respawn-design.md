# Stage 10 — NPC Respawn System Design

## Goal

Replace the single startup spawn (all NPCs in the start room) with per-room spawn configurations defined in zone YAML. NPCs that die in combat are removed from the manager, then automatically respawned after a configurable delay so long as the room's population cap is not already met.

## Architecture

Three coordinated changes: zone YAML gains spawn configs, the NPC template gains a respawn delay, and a new `RespawnManager` component schedules and executes respawns driven by the existing `ZoneTickManager`.

```
Zone YAML (spawns per room)
        │
        ▼
RespawnManager.PopulateRoom()   ← startup: initial population
        │
        ▼
npc.Manager.Spawn()

CombatHandler (NPC death)
  → npc.Manager.Remove(id)
  → RespawnManager.Schedule(templateID, roomID, delay)

ZoneTickManager (every tick)
  → RespawnManager.Tick(now, npcMgr)
        │  for each ready entry: count live instances
        │  if count < max → npc.Manager.Spawn()
```

## Data Model

### Zone YAML room entry

```yaml
- id: pioneer_square
  spawns:
    - template: ganger
      count: 2          # population cap: max live instances of this template in this room
      respawn_after: 3m # optional — overrides template's respawn_delay
  exits:
    - direction: north
      target: morrison_bridge
```

### NPC template YAML

```yaml
respawn_delay: 5m   # default delay; empty or absent means NPC does not respawn
```

## Components

### `internal/game/world/model.go`

Add to `Room`:
```go
type RoomSpawnConfig struct {
    Template     string `yaml:"template"`
    Count        int    `yaml:"count"`
    RespawnAfter string `yaml:"respawn_after"` // parsed as time.Duration; empty = use template default
}
// Room.Spawns []RoomSpawnConfig
```

### `internal/game/npc/template.go`

Add to `Template`:
```go
RespawnDelay string `yaml:"respawn_delay"` // e.g. "5m", "30s"; empty = no respawn
```

### `internal/game/npc/respawn.go` (new)

```go
type RoomSpawn struct {
    TemplateID   string
    Max          int
    RespawnDelay time.Duration // resolved: room override or template default
}

type RespawnManager struct { ... }

func NewRespawnManager(
    spawns    map[string][]RoomSpawn,  // roomID → configs
    templates map[string]*Template,    // templateID → Template
) *RespawnManager

// PopulateRoom spawns up to Max instances for each RoomSpawn entry in roomID.
// Called at startup for each room with spawn configs.
func (r *RespawnManager) PopulateRoom(roomID string, mgr *Manager)

// Schedule enqueues a future respawn for templateID in roomID.
// No-op if the template has no respawn delay and delay == 0.
func (r *RespawnManager) Schedule(templateID, roomID string, delay time.Duration)

// Tick drains all entries whose ReadyAt <= now, checks population cap,
// and calls mgr.Spawn for each slot available.
func (r *RespawnManager) Tick(now time.Time, mgr *Manager)
```

### `internal/gameserver/combat_handler.go`

- Add `respawnMgr *npc.RespawnManager` field
- After `npcMgr.Remove(inst.ID)` on NPC death: call `respawnMgr.Schedule(inst.TemplateID, inst.RoomID, resolvedDelay)`
- `resolvedDelay` comes from the room spawn config (or template default if no room override)

### `cmd/gameserver/main.go`

- Parse spawn configs from zone rooms after loading zones
- Build `map[string][]npc.RoomSpawn` from zone data + template lookup (to resolve delays)
- Create `RespawnManager`
- Replace per-template startup spawn loop with `respawnMgr.PopulateRoom()` per room
- Pass `respawnMgr` to `NewCombatHandler`
- Wire `respawnMgr.Tick(time.Now(), npcMgr)` into each zone's `ZoneTickManager` callback

## Error Handling

- Unknown template ID in spawn config: fatal at startup (same as today for templates)
- `RespawnDelay` parse error: fatal at startup
- `RespawnAfter` parse error in zone YAML: fatal at startup
- `npcMgr.Spawn` failure during Tick: log warning, drop the entry (non-fatal)

## Testing

- Unit tests for `RespawnManager` using `pgregory.net/rapid`:
  - Property: after scheduling N respawns for a room with cap M, `Tick` at T+delay spawns exactly `min(N, M - currentAlive)` instances
  - Property: `Tick` before the deadline spawns nothing
  - `PopulateRoom` respects cap
- Integration test: spawn → kill → advance clock past delay → tick → verify repopulated
- Zone loader test: rooms with spawn configs parse correctly, unknown templates are rejected
