# Admin UI Completion — Issue #45

**Date:** 2026-04-14  
**Status:** Approved

## Background

The admin web UI at `/admin` already has five tabs. Three are fully working:
- **Players** — list, kick, message, teleport via real gRPC
- **Accounts** — search and edit role/banned via PostgreSQL
- **Live Log** — real-time SSE stream of game events

Two tabs are non-functional:
- **Zone Editor** — backend wired to `NoOpWorldEditor` (always returns empty; never saves)
- **NPC Spawner** — backend `HandleSpawnNPC` returns HTTP 501; NPC template list is stubbed

No frontend changes are needed. All UI is already built. This spec covers the backend wiring only.

---

## Requirements

### Zone Editor

- REQ-AUI-1: The admin Zone Editor MUST list all zones loaded in the running gameserver, not from a static file.
- REQ-AUI-2: Zones MUST include: `id`, `name`, `danger_level`, `room_count`.
- REQ-AUI-3: Rooms in a zone MUST include: `id`, `title`, `description`, `danger_level`.
- REQ-AUI-4: Room updates (title, description, danger_level) MUST be persisted via the world manager's YAML-write path.
- REQ-AUI-5: An invalid `danger_level` value in an update request MUST be rejected with a descriptive error.

### NPC Spawner

- REQ-AUI-6: The admin NPC Spawner MUST list all NPC templates registered in the running gameserver.
- REQ-AUI-7: Templates MUST include: `id`, `name`, `level`, `type`.
- REQ-AUI-8: Spawning MUST accept `template_id`, `room_id`, and `count` (1–20).
- REQ-AUI-9: Each spawn call MUST invoke `npcMgr.Spawn` for the given template and room, `count` times.
- REQ-AUI-10: The response MUST return the number of instances successfully spawned.
- REQ-AUI-11: An unknown `template_id` MUST be rejected with a descriptive error.
- REQ-AUI-12: `count` outside the range [1, 20] MUST be rejected.

### gRPC Transport

- REQ-AUI-13: All new admin operations MUST be implemented as unary gRPC RPCs on the `GameService`.
- REQ-AUI-14: The web client MUST reach the gameserver via the existing `GRPCWorldEditor` adapter pattern (same as `grpcSessionManager`).
- REQ-AUI-15: `NewNoOpWorldEditor()` in `server.go` MUST be replaced with `NewGRPCWorldEditor(s.gameClient)`.

### Testing

- REQ-AUI-16: All new gRPC handlers MUST have unit tests following TDD with property-based testing where applicable.
- REQ-AUI-17: The `GRPCWorldEditor` adapter MUST be covered by interface-compliance tests.

---

## Architecture

### Proto additions (`api/proto/game/v1/game.proto`)

Five new RPC pairs added to the `GameService`:

```proto
rpc AdminListZones(AdminListZonesRequest)               returns (AdminListZonesResponse);
rpc AdminListRooms(AdminListRoomsRequest)               returns (AdminListRoomsResponse);
rpc AdminUpdateRoom(AdminUpdateRoomRequest)             returns (AdminUpdateRoomResponse);
rpc AdminListNPCTemplates(AdminListNPCTemplatesRequest) returns (AdminListNPCTemplatesResponse);
rpc AdminSpawnNPC(AdminSpawnNPCRequest)                 returns (AdminSpawnNPCResponse);

message AdminListZonesRequest {}
message AdminZoneSummary { string id = 1; string name = 2; string danger_level = 3; int32 room_count = 4; }
message AdminListZonesResponse { repeated AdminZoneSummary zones = 1; }

message AdminListRoomsRequest  { string zone_id = 1; }
message AdminRoomSummary { string id = 1; string title = 2; string description = 3; string danger_level = 4; }
message AdminListRoomsResponse { repeated AdminRoomSummary rooms = 1; }

message AdminUpdateRoomRequest  { string room_id = 1; string title = 2; string description = 3; string danger_level = 4; }
message AdminUpdateRoomResponse {}

message AdminListNPCTemplatesRequest {}
message AdminNPCTemplateSummary { string id = 1; string name = 2; int32 level = 3; string type = 4; }
message AdminListNPCTemplatesResponse { repeated AdminNPCTemplateSummary templates = 1; }

message AdminSpawnNPCRequest  { string template_id = 1; string room_id = 2; int32 count = 3; }
message AdminSpawnNPCResponse { int32 spawned_count = 1; }
```

### Gameserver handlers (`internal/gameserver/grpc_service_admin.go`)

Five new methods on `GameServiceServer`:

- `AdminListZones` — iterate `s.world.AllZones()`, map to `AdminZoneSummary`
- `AdminListRooms` — call `s.world.GetZone(zone_id)`, iterate its `Rooms` map
- `AdminUpdateRoom` — validate danger_level, call `s.worldEditor.SetRoomField()` for each non-empty patch field
- `AdminListNPCTemplates` — call `s.npcMgr.AllTemplates()`, map to `AdminNPCTemplateSummary`
- `AdminSpawnNPC` — validate count [1,20], look up template, call `s.npcMgr.Spawn()` count times

### Web client adapter (`cmd/webclient/handlers/grpc_world_editor.go`)

New file implementing the `WorldEditor` interface backed by gRPC:

```go
type grpcWorldEditor struct { client gamev1.GameServiceClient }

func NewGRPCWorldEditor(c gamev1.GameServiceClient) WorldEditor { ... }

func (g *grpcWorldEditor) AllZones() []ZoneSummary { ... }
func (g *grpcWorldEditor) RoomsInZone(zoneID string) ([]RoomSummary, error) { ... }
func (g *grpcWorldEditor) UpdateRoom(roomID string, patch RoomPatch) error { ... }
func (g *grpcWorldEditor) AllNPCTemplates() []NPCTemplate { ... }
```

`HandleSpawnNPC` in `admin.go` will also be wired to call the new gRPC RPC directly via the `grpcSessionManager`'s client (or a new `SpawnNPC(templateID, roomID string, count int) (int, error)` method on a `WorldEditor` extension).

### Wiring (`cmd/webclient/server.go`)

```go
// Before:
handlers.NewNoOpWorldEditor()

// After:
handlers.NewGRPCWorldEditor(s.gameClient)
```

---

## Files Changed

| File | Change |
|------|--------|
| `api/proto/game/v1/game.proto` | 5 new RPCs + 10 new message types |
| `internal/gameserver/grpc_service_admin.go` | 5 new RPC handler methods |
| `internal/gameserver/grpc_service_admin_test.go` | Unit tests for new handlers |
| `cmd/webclient/handlers/grpc_world_editor.go` | New — GRPCWorldEditor adapter |
| `cmd/webclient/handlers/admin.go` | Wire HandleSpawnNPC to call gRPC |
| `cmd/webclient/server.go` | Replace NoOp with GRPCWorldEditor |

No frontend (TypeScript/React) changes required.
