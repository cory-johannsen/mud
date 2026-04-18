# Admin UI Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the Zone Editor and NPC Spawner admin tabs by adding five admin gRPC RPCs to the gameserver, a `GRPCWorldEditor` adapter in the web client, and replacing the `NoOpWorldEditor` stub with the real implementation.

**Architecture:** Five new unary gRPC RPCs are added to `GameService` (zone list, room list, room update, NPC template list, NPC spawn). A new `grpcWorldEditor` struct in the web layer implements the existing `WorldEditor` interface by calling these RPCs. `HandleSpawnNPC` in `admin.go` is wired through an extended `WorldEditor.SpawnNPC` method. `server.go` replaces `NewNoOpWorldEditor()` with `NewGRPCWorldEditor(s.gameClient)`.

**Tech Stack:** Go, protobuf/gRPC (`google.golang.org/grpc`, `google.golang.org/protobuf`), `pgregory.net/rapid` for property-based tests, `github.com/stretchr/testify` for assertions.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `api/proto/game/v1/game.proto` | Modify | Add 5 RPCs + 10 message types |
| `internal/gameserver/gamev1/game.pb.go` | Generated | Auto-updated by `make proto` |
| `internal/gameserver/gamev1/game_grpc.pb.go` | Generated | Auto-updated by `make proto` |
| `internal/gameserver/grpc_service_admin.go` | Modify | 5 new handler methods |
| `internal/gameserver/grpc_service_admin_world_test.go` | Create | Unit tests for world/NPC handlers |
| `cmd/webclient/handlers/admin.go` | Modify | Extend WorldEditor interface + wire SpawnNPC |
| `cmd/webclient/handlers/admin_noop.go` | Modify | Add SpawnNPC no-op |
| `cmd/webclient/handlers/grpc_world_editor.go` | Create | GRPCWorldEditor adapter |
| `cmd/webclient/server.go` | Modify | Replace NoOp with GRPCWorldEditor |

---

## Task 1: Add proto messages and RPCs

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Generated: `internal/gameserver/gamev1/game.pb.go`, `internal/gameserver/gamev1/game_grpc.pb.go`

- [ ] **Step 1.1: Add 5 RPCs to the GameService definition**

Open `api/proto/game/v1/game.proto`. The service block currently ends at:
```proto
  rpc AdminTeleportPlayer(AdminTeleportRequest) returns (AdminTeleportResponse);
}
```
Change it to:
```proto
  rpc AdminTeleportPlayer(AdminTeleportRequest) returns (AdminTeleportResponse);

  // World admin RPCs.
  rpc AdminListZones(AdminListZonesRequest)               returns (AdminListZonesResponse);
  rpc AdminListRooms(AdminListRoomsRequest)               returns (AdminListRoomsResponse);
  rpc AdminUpdateRoom(AdminUpdateRoomRequest)             returns (AdminUpdateRoomResponse);

  // NPC admin RPCs.
  rpc AdminListNPCTemplates(AdminListNPCTemplatesRequest) returns (AdminListNPCTemplatesResponse);
  rpc AdminSpawnNPC(AdminSpawnNPCRequest)                 returns (AdminSpawnNPCResponse);
}
```

- [ ] **Step 1.2: Append new message types to the end of game.proto**

Add after the last line (after `message AdminTeleportResponse {}`):
```proto
// Admin zone/room/NPC messages.

message AdminListZonesRequest {}

message AdminZoneSummary {
  string id           = 1;
  string name         = 2;
  string danger_level = 3;
  int32  room_count   = 4;
}

message AdminListZonesResponse {
  repeated AdminZoneSummary zones = 1;
}

message AdminListRoomsRequest {
  string zone_id = 1;
}

message AdminRoomSummary {
  string id           = 1;
  string title        = 2;
  string description  = 3;
  string danger_level = 4;
}

message AdminListRoomsResponse {
  repeated AdminRoomSummary rooms = 1;
}

message AdminUpdateRoomRequest {
  string room_id      = 1;
  string title        = 2;
  string description  = 3;
  string danger_level = 4;
}

message AdminUpdateRoomResponse {}

message AdminListNPCTemplatesRequest {}

message AdminNPCTemplateSummary {
  string id    = 1;
  string name  = 2;
  int32  level = 3;
  string type  = 4;
}

message AdminListNPCTemplatesResponse {
  repeated AdminNPCTemplateSummary templates = 1;
}

message AdminSpawnNPCRequest {
  string template_id = 1;
  string room_id     = 2;
  int32  count       = 3;
}

message AdminSpawnNPCResponse {
  int32 spawned_count = 1;
}
```

- [ ] **Step 1.3: Regenerate Go proto code**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: no errors; `internal/gameserver/gamev1/game.pb.go` and `game_grpc.pb.go` updated.

- [ ] **Step 1.4: Verify compilation**

```bash
go build ./...
```

Expected: compilation fails with "AdminListZones not implemented" (or similar) on the GameServiceServer — that's correct and expected because the server interface now requires 5 new methods. We'll implement them in the next tasks.

Actually the generated gRPC server will embed `UnimplementedGameServiceServer` which provides default "not implemented" responses. So `go build ./...` should succeed. Verify:
```bash
go build ./... 2>&1
```
Expected: no output (success).

- [ ] **Step 1.5: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(proto): add AdminListZones, AdminListRooms, AdminUpdateRoom, AdminListNPCTemplates, AdminSpawnNPC RPCs"
```

---

## Task 2: Implement AdminListZones and AdminListRooms handlers

**Files:**
- Modify: `internal/gameserver/grpc_service_admin.go`
- Create: `internal/gameserver/grpc_service_admin_world_test.go`

- [ ] **Step 2.1: Write failing tests for AdminListZones and AdminListRooms**

Create `/home/cjohannsen/src/mud/internal/gameserver/grpc_service_admin_world_test.go`:

```go
package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestAdminListZones_ReturnsAllLoadedZones verifies that AdminListZones returns
// a summary for every zone registered in the world manager.
// REQ-AUI-1, REQ-AUI-2.
func TestAdminListZones_ReturnsAllLoadedZones(t *testing.T) {
	svc, _ := newAdminSvc(t)

	resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// testWorldAndSession creates a world with at least one zone ("test_sr" zone from the helper).
	// We just verify the response is non-nil and each zone has an ID and name.
	for _, z := range resp.Zones {
		assert.NotEmpty(t, z.Id, "zone ID must not be empty")
		assert.NotEmpty(t, z.Name, "zone Name must not be empty")
		assert.GreaterOrEqual(t, z.RoomCount, int32(0), "RoomCount must be >= 0")
	}
}

// TestAdminListZones_RoomCountMatchesActualRooms verifies that RoomCount in each
// zone summary equals the number of rooms in that zone. REQ-AUI-2.
func TestAdminListZones_RoomCountMatchesActualRooms(t *testing.T) {
	svc, _ := newAdminSvc(t)

	resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)

	for _, z := range resp.Zones {
		zone, ok := svc.world.GetZone(z.Id)
		require.True(t, ok, "zone %q must exist in world manager", z.Id)
		assert.Equal(t, int32(len(zone.Rooms)), z.RoomCount,
			"RoomCount for zone %q must match actual room count", z.Id)
	}
}

// TestAdminListRooms_ReturnsRoomsForZone verifies that AdminListRooms returns
// all rooms in the given zone. REQ-AUI-3.
func TestAdminListRooms_ReturnsRoomsForZone(t *testing.T) {
	svc, _ := newAdminSvc(t)

	// First, get a valid zone ID.
	zonesResp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, zonesResp.Zones, "need at least one zone for this test")

	zoneID := zonesResp.Zones[0].Id
	zone, _ := svc.world.GetZone(zoneID)

	resp, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: zoneID})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, len(zone.Rooms), len(resp.Rooms),
		"room count must match zone's actual room count")

	for _, r := range resp.Rooms {
		assert.NotEmpty(t, r.Id, "room ID must not be empty")
		assert.NotEmpty(t, r.Title, "room Title must not be empty")
	}
}

// TestAdminListRooms_UnknownZone_ReturnsNotFound verifies that an unknown zone ID
// returns a gRPC NotFound error. REQ-AUI-3.
func TestAdminListRooms_UnknownZone_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)

	_, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: "nonexistent-zone-xyz"})
	require.Error(t, err)
	assertGRPCCode(t, err, "codes.NotFound")
}

// TestProperty_AdminListZones_NeverPanics is a property test verifying AdminListZones
// always returns without panic and with a valid response. REQ-AUI-1.
func TestProperty_AdminListZones_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _ := newAdminSvc(t)
		resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
		if err != nil {
			rt.Fatal("AdminListZones must not return error on valid world:", err)
		}
		if resp == nil {
			rt.Fatal("AdminListZones must not return nil response")
		}
	})
}
```

Add this helper at the bottom of the file (used in multiple tests):
```go
// assertGRPCCode asserts that err is a gRPC status error with code matching codeStr.
// codeStr is like "codes.NotFound", "codes.InvalidArgument", etc.
func assertGRPCCode(t *testing.T, err error, codeStr string) {
	t.Helper()
	import_status := "google.golang.org/grpc/status"
	_ = import_status
	// Use the status package.
	st, ok := status.FromError(err)
	require.True(t, ok, "error must be a gRPC status error, got: %v", err)
	assert.Contains(t, st.Code().String(), codeStr[len("codes."):],
		"expected gRPC code %s, got %s: %v", codeStr, st.Code(), err)
}
```

Wait — the above approach for assertGRPCCode is overly complex. Use this simpler version instead:

```go
// assertGRPCCode asserts that err carries the given gRPC status code.
func assertGRPCCode(t *testing.T, err error, wantCode codes.Code) {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error, got: %T %v", err, err)
	assert.Equal(t, wantCode, st.Code(), "unexpected gRPC code: %v", err)
}
```

So the test file needs these imports added:
```go
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)
```

And `TestAdminListRooms_UnknownZone_ReturnsNotFound` becomes:
```go
func TestAdminListRooms_UnknownZone_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: "nonexistent-zone-xyz"})
	require.Error(t, err)
	assertGRPCCode(t, err, codes.NotFound)
}
```

Here is the **complete** final test file to create:

```go
package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// assertGRPCCode asserts that err carries the given gRPC status code.
func assertGRPCCode(t *testing.T, err error, wantCode codes.Code) {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error, got: %T %v", err, err)
	assert.Equal(t, wantCode, st.Code(), "unexpected gRPC code: %v", err)
}

// TestAdminListZones_ReturnsAllLoadedZones verifies that AdminListZones returns
// a summary for every zone registered in the world manager. REQ-AUI-1, REQ-AUI-2.
func TestAdminListZones_ReturnsAllLoadedZones(t *testing.T) {
	svc, _ := newAdminSvc(t)
	resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	for _, z := range resp.Zones {
		assert.NotEmpty(t, z.Id)
		assert.NotEmpty(t, z.Name)
		assert.GreaterOrEqual(t, z.RoomCount, int32(0))
	}
}

// TestAdminListZones_RoomCountMatchesActualRooms verifies RoomCount accuracy. REQ-AUI-2.
func TestAdminListZones_RoomCountMatchesActualRooms(t *testing.T) {
	svc, _ := newAdminSvc(t)
	resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	for _, z := range resp.Zones {
		zone, ok := svc.world.GetZone(z.Id)
		require.True(t, ok)
		assert.Equal(t, int32(len(zone.Rooms)), z.RoomCount)
	}
}

// TestAdminListRooms_ReturnsRoomsForZone verifies rooms are listed for a valid zone. REQ-AUI-3.
func TestAdminListRooms_ReturnsRoomsForZone(t *testing.T) {
	svc, _ := newAdminSvc(t)
	zonesResp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, zonesResp.Zones)
	zoneID := zonesResp.Zones[0].Id
	zone, _ := svc.world.GetZone(zoneID)
	resp, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: zoneID})
	require.NoError(t, err)
	assert.Equal(t, len(zone.Rooms), len(resp.Rooms))
	for _, r := range resp.Rooms {
		assert.NotEmpty(t, r.Id)
		assert.NotEmpty(t, r.Title)
	}
}

// TestAdminListRooms_UnknownZone_ReturnsNotFound verifies NotFound for bad zone. REQ-AUI-3.
func TestAdminListRooms_UnknownZone_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: "nonexistent-zone-xyz"})
	require.Error(t, err)
	assertGRPCCode(t, err, codes.NotFound)
}

// TestProperty_AdminListZones_NeverErrors is a property test. REQ-AUI-1.
func TestProperty_AdminListZones_NeverErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _ := newAdminSvc(t)
		resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if resp == nil {
			rt.Fatal("nil response")
		}
	})
}
```

- [ ] **Step 2.2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/ -run "TestAdminListZones|TestAdminListRooms|TestProperty_AdminListZones" -v -count=1 2>&1 | head -30
```

Expected: compile error — `svc.AdminListZones` and `svc.AdminListRooms` methods do not exist yet.

- [ ] **Step 2.3: Implement AdminListZones and AdminListRooms in grpc_service_admin.go**

Add these methods at the end of `internal/gameserver/grpc_service_admin.go`:

```go
// AdminListZones returns a summary of all zones loaded in the world manager.
//
// Precondition: s.world must be non-nil.
// Postcondition: Returns all zones; never returns an error for an empty world.
// REQ-AUI-1, REQ-AUI-2.
func (s *GameServiceServer) AdminListZones(ctx context.Context, req *gamev1.AdminListZonesRequest) (*gamev1.AdminListZonesResponse, error) {
	zones := s.world.AllZones()
	out := make([]*gamev1.AdminZoneSummary, 0, len(zones))
	for _, z := range zones {
		out = append(out, &gamev1.AdminZoneSummary{
			Id:          z.ID,
			Name:        z.Name,
			DangerLevel: z.DangerLevel,
			RoomCount:   int32(len(z.Rooms)),
		})
	}
	return &gamev1.AdminListZonesResponse{Zones: out}, nil
}

// AdminListRooms returns a summary of all rooms in the given zone.
//
// Precondition: req.ZoneId must be non-empty.
// Postcondition: Returns codes.NotFound if zone does not exist; otherwise returns all rooms.
// REQ-AUI-3.
func (s *GameServiceServer) AdminListRooms(ctx context.Context, req *gamev1.AdminListRoomsRequest) (*gamev1.AdminListRoomsResponse, error) {
	zone, ok := s.world.GetZone(req.ZoneId)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "zone %q not found", req.ZoneId)
	}
	out := make([]*gamev1.AdminRoomSummary, 0, len(zone.Rooms))
	for _, r := range zone.Rooms {
		out = append(out, &gamev1.AdminRoomSummary{
			Id:          r.ID,
			Title:       r.Title,
			Description: r.Description,
			DangerLevel: r.DangerLevel,
		})
	}
	return &gamev1.AdminListRoomsResponse{Rooms: out}, nil
}
```

Make sure the imports at the top of `grpc_service_admin.go` include `"google.golang.org/grpc/codes"` and `"google.golang.org/grpc/status"` — they should already be there from existing handlers.

- [ ] **Step 2.4: Run tests to verify they pass**

```bash
go test ./internal/gameserver/ -run "TestAdminListZones|TestAdminListRooms|TestProperty_AdminListZones" -v -count=1 2>&1
```

Expected: all tests PASS.

- [ ] **Step 2.5: Commit**

```bash
git add internal/gameserver/grpc_service_admin.go internal/gameserver/grpc_service_admin_world_test.go
git commit -m "feat(admin): implement AdminListZones and AdminListRooms gRPC handlers (REQ-AUI-1,2,3)"
```

---

## Task 3: Implement AdminUpdateRoom handler

**Files:**
- Modify: `internal/gameserver/grpc_service_admin.go`
- Modify: `internal/gameserver/grpc_service_admin_world_test.go`

- [ ] **Step 3.1: Write failing tests for AdminUpdateRoom**

Append to `internal/gameserver/grpc_service_admin_world_test.go`:

```go
// TestAdminUpdateRoom_NilWorldEditor_ReturnsInternal verifies that AdminUpdateRoom
// returns codes.Internal when no world editor is wired. REQ-AUI-4.
func TestAdminUpdateRoom_NilWorldEditor_ReturnsInternal(t *testing.T) {
	svc, _ := newAdminSvc(t)
	// newAdminSvc does not wire a worldEditor; s.worldEditor is nil.
	_, err := svc.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
		RoomId:      "room_safe",
		Title:       "New Title",
		Description: "New description.",
		DangerLevel: "safe",
	})
	require.Error(t, err)
	assertGRPCCode(t, err, codes.Internal)
}

// TestAdminUpdateRoom_EmptyRoomID_ReturnsInvalidArgument verifies that an empty
// room_id is rejected. REQ-AUI-4.
func TestAdminUpdateRoom_EmptyRoomID_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
		RoomId: "",
		Title:  "Title",
	})
	require.Error(t, err)
	assertGRPCCode(t, err, codes.InvalidArgument)
}
```

- [ ] **Step 3.2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/ -run "TestAdminUpdateRoom" -v -count=1 2>&1 | head -20
```

Expected: compile error — `svc.AdminUpdateRoom` method does not exist yet.

- [ ] **Step 3.3: Implement AdminUpdateRoom in grpc_service_admin.go**

Append to `internal/gameserver/grpc_service_admin.go`:

```go
// AdminUpdateRoom applies a patch to a room's title, description, and/or danger_level.
//
// Precondition: req.RoomId must be non-empty.
// Postcondition: Returns codes.InvalidArgument for empty room_id; codes.Internal if
// worldEditor is not wired or SetRoomField fails; codes.OK on success. REQ-AUI-4, REQ-AUI-5.
func (s *GameServiceServer) AdminUpdateRoom(ctx context.Context, req *gamev1.AdminUpdateRoomRequest) (*gamev1.AdminUpdateRoomResponse, error) {
	if req.RoomId == "" {
		return nil, status.Error(codes.InvalidArgument, "room_id must not be empty")
	}
	if s.worldEditor == nil {
		return nil, status.Error(codes.Internal, "world editor not available")
	}
	type fieldUpdate struct {
		field string
		value string
	}
	updates := []fieldUpdate{
		{"title", req.Title},
		{"description", req.Description},
		{"danger_level", req.DangerLevel},
	}
	for _, u := range updates {
		if u.value == "" {
			continue
		}
		if err := s.worldEditor.SetRoomField(req.RoomId, u.field, u.value); err != nil {
			return nil, status.Errorf(codes.Internal, "setting %s on room %s: %v", u.field, req.RoomId, err)
		}
	}
	return &gamev1.AdminUpdateRoomResponse{}, nil
}
```

- [ ] **Step 3.4: Run tests to verify they pass**

```bash
go test ./internal/gameserver/ -run "TestAdminUpdateRoom" -v -count=1 2>&1
```

Expected: all tests PASS.

- [ ] **Step 3.5: Commit**

```bash
git add internal/gameserver/grpc_service_admin.go internal/gameserver/grpc_service_admin_world_test.go
git commit -m "feat(admin): implement AdminUpdateRoom gRPC handler (REQ-AUI-4,5)"
```

---

## Task 4: Implement AdminListNPCTemplates and AdminSpawnNPC handlers

**Files:**
- Modify: `internal/gameserver/grpc_service_admin.go`
- Modify: `internal/gameserver/grpc_service_admin_world_test.go`

- [ ] **Step 4.1: Write failing tests**

Append to `internal/gameserver/grpc_service_admin_world_test.go`:

```go
// TestAdminListNPCTemplates_ReturnsAllTemplates verifies that all registered templates
// are returned with non-empty ID, Name, and non-negative Level. REQ-AUI-6, REQ-AUI-7.
func TestAdminListNPCTemplates_ReturnsAllTemplates(t *testing.T) {
	svc, _ := newAdminSvc(t)

	// Register a template so the list is non-empty.
	err := svc.npcMgr.RegisterTemplate(&npc.Template{
		ID:      "test-grunt",
		Name:    "Test Grunt",
		Level:   2,
		NPCType: "combat",
		MaxHP:   20,
		AC:      12,
	})
	require.NoError(t, err)

	resp, err := svc.AdminListNPCTemplates(context.Background(), &gamev1.AdminListNPCTemplatesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Templates, "expected at least one template")

	for _, tmpl := range resp.Templates {
		assert.NotEmpty(t, tmpl.Id)
		assert.NotEmpty(t, tmpl.Name)
		assert.GreaterOrEqual(t, tmpl.Level, int32(0))
	}
}

// TestAdminSpawnNPC_UnknownTemplate_ReturnsNotFound verifies codes.NotFound
// for an unknown template ID. REQ-AUI-11.
func TestAdminSpawnNPC_UnknownTemplate_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
		TemplateId: "does-not-exist",
		RoomId:     "room_safe",
		Count:      1,
	})
	require.Error(t, err)
	assertGRPCCode(t, err, codes.NotFound)
}

// TestAdminSpawnNPC_InvalidCount_ReturnsInvalidArgument verifies count validation.
// REQ-AUI-12.
func TestAdminSpawnNPC_InvalidCount_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := newAdminSvc(t)
	for _, count := range []int32{0, -1, 21, 100} {
		_, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
			TemplateId: "any",
			RoomId:     "room_safe",
			Count:      count,
		})
		require.Error(t, err, "count=%d should be rejected", count)
		assertGRPCCode(t, err, codes.InvalidArgument)
	}
}

// TestAdminSpawnNPC_ValidRequest_SpawnsInstances verifies that a valid spawn request
// creates the expected number of NPC instances. REQ-AUI-8, REQ-AUI-9, REQ-AUI-10.
func TestAdminSpawnNPC_ValidRequest_SpawnsInstances(t *testing.T) {
	svc, _ := newAdminSvc(t)

	err := svc.npcMgr.RegisterTemplate(&npc.Template{
		ID:      "spawn-test-npc",
		Name:    "Spawn Test NPC",
		Level:   1,
		NPCType: "combat",
		MaxHP:   10,
		AC:      10,
	})
	require.NoError(t, err)

	const count = 3
	resp, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
		TemplateId: "spawn-test-npc",
		RoomId:     "room_safe",
		Count:      count,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(count), resp.SpawnedCount)
}

// TestProperty_AdminSpawnNPC_CountAlwaysValid is a property test verifying valid
// counts [1,20] always succeed with a registered template. REQ-AUI-12.
func TestProperty_AdminSpawnNPC_CountAlwaysValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _ := newAdminSvc(t)
		err := svc.npcMgr.RegisterTemplate(&npc.Template{
			ID: "prop-npc", Name: "Prop NPC", Level: 1, NPCType: "combat", MaxHP: 10, AC: 10,
		})
		require.NoError(t, err)
		count := rapid.Int32Range(1, 20).Draw(rt, "count")
		resp, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
			TemplateId: "prop-npc",
			RoomId:     "room_safe",
			Count:      count,
		})
		if err != nil {
			rt.Fatalf("valid count %d failed: %v", count, err)
		}
		if resp.SpawnedCount != count {
			rt.Fatalf("expected SpawnedCount=%d, got %d", count, resp.SpawnedCount)
		}
	})
}
```

Add `"github.com/cory-johannsen/mud/internal/game/npc"` to the imports in `grpc_service_admin_world_test.go`.

- [ ] **Step 4.2: Check if npc.Manager has RegisterTemplate method**

```bash
grep -n "RegisterTemplate" /home/cjohannsen/src/mud/internal/game/npc/manager.go
```

If `RegisterTemplate` does not exist, find the correct method name:
```bash
grep -n "func.*Manager.*Template\|func.*Register\|func.*Load" /home/cjohannsen/src/mud/internal/game/npc/manager.go | head -20
```

Adjust the test calls accordingly. The method to register templates may be `LoadTemplate`, `AddTemplate`, or similar. Use whatever exists.

- [ ] **Step 4.3: Run tests to verify they fail**

```bash
go test ./internal/gameserver/ -run "TestAdminListNPCTemplates|TestAdminSpawnNPC|TestProperty_AdminSpawnNPC" -v -count=1 2>&1 | head -20
```

Expected: compile error — methods do not exist yet.

- [ ] **Step 4.4: Implement AdminListNPCTemplates and AdminSpawnNPC**

Append to `internal/gameserver/grpc_service_admin.go`:

```go
// AdminListNPCTemplates returns a summary of all NPC templates registered in the NPC manager.
//
// Postcondition: Never returns error; returns empty list if no templates are registered.
// REQ-AUI-6, REQ-AUI-7.
func (s *GameServiceServer) AdminListNPCTemplates(ctx context.Context, req *gamev1.AdminListNPCTemplatesRequest) (*gamev1.AdminListNPCTemplatesResponse, error) {
	templates := s.npcMgr.AllTemplates()
	out := make([]*gamev1.AdminNPCTemplateSummary, 0, len(templates))
	for _, tmpl := range templates {
		out = append(out, &gamev1.AdminNPCTemplateSummary{
			Id:    tmpl.ID,
			Name:  tmpl.Name,
			Level: int32(tmpl.Level),
			Type:  tmpl.NPCType,
		})
	}
	return &gamev1.AdminListNPCTemplatesResponse{Templates: out}, nil
}

// AdminSpawnNPC spawns count instances of the named template in the given room.
//
// Precondition: template_id must match a registered template; count must be in [1,20].
// Postcondition: Returns codes.NotFound for unknown template; codes.InvalidArgument for
// bad count; codes.Internal if any spawn fails; codes.OK with spawned_count otherwise.
// REQ-AUI-8, REQ-AUI-9, REQ-AUI-10, REQ-AUI-11, REQ-AUI-12.
func (s *GameServiceServer) AdminSpawnNPC(ctx context.Context, req *gamev1.AdminSpawnNPCRequest) (*gamev1.AdminSpawnNPCResponse, error) {
	if req.Count < 1 || req.Count > 20 {
		return nil, status.Errorf(codes.InvalidArgument, "count must be in [1,20], got %d", req.Count)
	}
	tmpl := s.npcMgr.TemplateByID(req.TemplateId)
	if tmpl == nil {
		return nil, status.Errorf(codes.NotFound, "NPC template %q not found", req.TemplateId)
	}
	var spawned int32
	for i := 0; i < int(req.Count); i++ {
		if _, err := s.npcMgr.Spawn(tmpl, req.RoomId); err != nil {
			return nil, status.Errorf(codes.Internal, "spawn %d/%d failed: %v", i+1, req.Count, err)
		}
		spawned++
	}
	return &gamev1.AdminSpawnNPCResponse{SpawnedCount: spawned}, nil
}
```

- [ ] **Step 4.5: Run tests to verify they pass**

```bash
go test ./internal/gameserver/ -run "TestAdminListNPCTemplates|TestAdminSpawnNPC|TestProperty_AdminSpawnNPC" -v -count=1 2>&1
```

Expected: all tests PASS.

- [ ] **Step 4.6: Run full gameserver test suite**

```bash
go test ./internal/gameserver/ -count=1 2>&1 | tail -5
```

Expected: `ok  github.com/cory-johannsen/mud/internal/gameserver`.

- [ ] **Step 4.7: Commit**

```bash
git add internal/gameserver/grpc_service_admin.go internal/gameserver/grpc_service_admin_world_test.go
git commit -m "feat(admin): implement AdminListNPCTemplates and AdminSpawnNPC gRPC handlers (REQ-AUI-6..12)"
```

---

## Task 5: Create GRPCWorldEditor adapter and extend WorldEditor interface

**Files:**
- Modify: `cmd/webclient/handlers/admin.go` (extend WorldEditor interface)
- Modify: `cmd/webclient/handlers/admin_noop.go` (add SpawnNPC no-op)
- Create: `cmd/webclient/handlers/grpc_world_editor.go`

- [ ] **Step 5.1: Extend the WorldEditor interface in admin.go**

In `cmd/webclient/handlers/admin.go`, find the `WorldEditor` interface:
```go
type WorldEditor interface {
	AllZones() []ZoneSummary
	RoomsInZone(zoneID string) ([]RoomSummary, error)
	UpdateRoom(roomID string, patch RoomPatch) error
	AllNPCTemplates() []NPCTemplate
}
```

Add the `SpawnNPC` method:
```go
type WorldEditor interface {
	AllZones() []ZoneSummary
	RoomsInZone(zoneID string) ([]RoomSummary, error)
	UpdateRoom(roomID string, patch RoomPatch) error
	AllNPCTemplates() []NPCTemplate
	SpawnNPC(templateID, roomID string, count int) (int, error)
}
```

- [ ] **Step 5.2: Add SpawnNPC no-op to admin_noop.go**

In `cmd/webclient/handlers/admin_noop.go`, append:
```go
func (n *noOpWorldEditor) SpawnNPC(_ string, _ string, _ int) (int, error) { return 0, nil }
```

- [ ] **Step 5.3: Wire HandleSpawnNPC in admin.go**

Find `HandleSpawnNPC` in `cmd/webclient/handlers/admin.go`. It currently ends with:
```go
	// Spawn via game server is not yet wired through an admin gRPC stream (REQ-WC-38).
	writeError(w, http.StatusNotImplemented, "not implemented")
	return
```

Replace the stub with:
```go
	roomID := r.PathValue("room_id")
	if roomID == "" {
		roomID = body.RoomID
	}
	if strings.TrimSpace(roomID) == "" {
		writeError(w, http.StatusBadRequest, "room_id must not be empty")
		return
	}
	if body.Count > 20 {
		writeError(w, http.StatusBadRequest, "count must not exceed 20")
		return
	}
	spawned, err := ah.world.SpawnNPC(body.NPCID, roomID, body.Count)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spawned_count": spawned})
```

- [ ] **Step 5.4: Create grpc_world_editor.go**

Create `cmd/webclient/handlers/grpc_world_editor.go`:

```go
package handlers

import (
	"context"
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// grpcWorldEditor implements WorldEditor via the gameserver admin gRPC RPCs.
//
// Precondition: client must be non-nil.
// Postcondition: Each method makes one unary gRPC call and maps the result to the
// handlers layer types; gRPC errors are propagated as-is.
type grpcWorldEditor struct {
	client gamev1.GameServiceClient
}

// NewGRPCWorldEditor returns a WorldEditor backed by the gameserver admin gRPC RPCs.
func NewGRPCWorldEditor(client gamev1.GameServiceClient) WorldEditor {
	return &grpcWorldEditor{client: client}
}

// AllZones calls AdminListZones and maps the result to []ZoneSummary.
func (g *grpcWorldEditor) AllZones() []ZoneSummary {
	resp, err := g.client.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	if err != nil {
		return []ZoneSummary{}
	}
	out := make([]ZoneSummary, 0, len(resp.Zones))
	for _, z := range resp.Zones {
		out = append(out, ZoneSummary{
			ID:          z.Id,
			Name:        z.Name,
			DangerLevel: z.DangerLevel,
			RoomCount:   int(z.RoomCount),
		})
	}
	return out
}

// RoomsInZone calls AdminListRooms for the given zone and maps the result to []RoomSummary.
// Returns an error if the zone is not found or the RPC fails.
func (g *grpcWorldEditor) RoomsInZone(zoneID string) ([]RoomSummary, error) {
	resp, err := g.client.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: zoneID})
	if err != nil {
		return nil, err
	}
	out := make([]RoomSummary, 0, len(resp.Rooms))
	for _, r := range resp.Rooms {
		out = append(out, RoomSummary{
			ID:          r.Id,
			Title:       r.Title,
			Description: r.Description,
			DangerLevel: r.DangerLevel,
		})
	}
	return out, nil
}

// UpdateRoom calls AdminUpdateRoom with the non-empty fields from patch.
func (g *grpcWorldEditor) UpdateRoom(roomID string, patch RoomPatch) error {
	_, err := g.client.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
		RoomId:      roomID,
		Title:       patch.Title,
		Description: patch.Description,
		DangerLevel: patch.DangerLevel,
	})
	return err
}

// AllNPCTemplates calls AdminListNPCTemplates and maps the result to []NPCTemplate.
func (g *grpcWorldEditor) AllNPCTemplates() []NPCTemplate {
	resp, err := g.client.AdminListNPCTemplates(context.Background(), &gamev1.AdminListNPCTemplatesRequest{})
	if err != nil {
		return []NPCTemplate{}
	}
	out := make([]NPCTemplate, 0, len(resp.Templates))
	for _, t := range resp.Templates {
		out = append(out, NPCTemplate{
			ID:    t.Id,
			Name:  t.Name,
			Level: int(t.Level),
			Type:  t.Type,
		})
	}
	return out
}

// SpawnNPC calls AdminSpawnNPC with the given template ID, room ID, and count.
// Returns the number of instances spawned, or an error.
func (g *grpcWorldEditor) SpawnNPC(templateID, roomID string, count int) (int, error) {
	resp, err := g.client.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
		TemplateId: templateID,
		RoomId:     roomID,
		Count:      int32(count),
	})
	if err != nil {
		return 0, fmt.Errorf("AdminSpawnNPC: %w", err)
	}
	return int(resp.SpawnedCount), nil
}
```

- [ ] **Step 5.5: Verify compilation**

```bash
go build ./cmd/webclient/... 2>&1
```

Expected: no output (success). If there are errors about missing interface methods, add the missing ones.

- [ ] **Step 5.6: Commit**

```bash
git add cmd/webclient/handlers/admin.go cmd/webclient/handlers/admin_noop.go cmd/webclient/handlers/grpc_world_editor.go
git commit -m "feat(admin): add GRPCWorldEditor adapter and extend WorldEditor with SpawnNPC (REQ-AUI-13,14,17)"
```

---

## Task 6: Wire server.go and run full test suite

**Files:**
- Modify: `cmd/webclient/server.go`

- [ ] **Step 6.1: Replace NoOpWorldEditor with GRPCWorldEditor in server.go**

In `cmd/webclient/server.go`, find:
```go
adminHandler := handlers.NewAdminHandler(
	handlers.NewGRPCSessionManager(s.gameClient),
	s.accountRepo,
	handlers.NewNoOpWorldEditor(),
	s.bus,
)
```

Change to:
```go
adminHandler := handlers.NewAdminHandler(
	handlers.NewGRPCSessionManager(s.gameClient),
	s.accountRepo,
	handlers.NewGRPCWorldEditor(s.gameClient),
	s.bus,
)
```

- [ ] **Step 6.2: Verify full build**

```bash
go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 6.3: Run full test suite**

```bash
go test ./... 2>&1 | grep -E "^(ok|FAIL|---)" | tail -30
```

Expected: all packages show `ok`. No `FAIL` lines.

- [ ] **Step 6.4: Build web UI**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build 2>&1 | tail -5
```

Expected: `✓ built in N.NNs`.

- [ ] **Step 6.5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add cmd/webclient/server.go
git commit -m "feat(admin): wire GRPCWorldEditor into server; replace NoOpWorldEditor (REQ-AUI-15)"
```

- [ ] **Step 6.6: Push and deploy**

```bash
git push origin main && make k8s-redeploy 2>&1 | tail -10
```

Expected: helm release upgraded successfully.

- [ ] **Step 6.7: Close issue #45**

```bash
gh issue close 45 --repo cory-johannsen/mud --comment "Admin UI complete. Zone Editor and NPC Spawner tabs are now fully wired via five new admin gRPC RPCs (AdminListZones, AdminListRooms, AdminUpdateRoom, AdminListNPCTemplates, AdminSpawnNPC). The NoOpWorldEditor stub has been replaced by a GRPCWorldEditor adapter. All five admin tabs are functional."
```

---

## Self-Review

**Spec coverage:**
- REQ-AUI-1 (list zones from running server) → Task 2 ✓
- REQ-AUI-2 (zone fields: id, name, danger_level, room_count) → Task 2 ✓
- REQ-AUI-3 (room fields: id, title, description, danger_level) → Task 2 ✓
- REQ-AUI-4 (room updates persisted via world manager YAML path) → Task 3 ✓
- REQ-AUI-5 (invalid danger_level rejected) → Task 3 (SetRoomField validates this) ✓
- REQ-AUI-6 (list NPC templates from running server) → Task 4 ✓
- REQ-AUI-7 (template fields: id, name, level, type) → Task 4 ✓
- REQ-AUI-8 (spawn accepts template_id, room_id, count) → Task 4 ✓
- REQ-AUI-9 (each spawn calls npcMgr.Spawn count times) → Task 4 ✓
- REQ-AUI-10 (response returns spawned_count) → Task 4 ✓
- REQ-AUI-11 (unknown template_id rejected with NotFound) → Task 4 ✓
- REQ-AUI-12 (count outside [1,20] rejected) → Task 4 ✓
- REQ-AUI-13 (all ops as unary gRPC RPCs) → Task 1 ✓
- REQ-AUI-14 (web client uses GRPCWorldEditor adapter) → Task 5 ✓
- REQ-AUI-15 (NoOp replaced with GRPCWorldEditor) → Task 6 ✓
- REQ-AUI-16 (TDD + property-based tests for handlers) → Tasks 2, 3, 4 ✓
- REQ-AUI-17 (GRPCWorldEditor interface-compliance) → Task 5 (compile-time via interface assignment) ✓

**Placeholder scan:** No TBD, TODO, or incomplete steps found.

**Type consistency:**
- `AdminZoneSummary.Id` (proto) → `ZoneSummary.ID` (handler) — mapped correctly in grpc_world_editor.go ✓
- `AdminNPCTemplateSummary.Type` (proto) ← `tmpl.NPCType` (go) ✓
- `AdminSpawnNPCResponse.SpawnedCount` (proto) → `resp.SpawnedCount` (go) ✓
- `WorldEditor.SpawnNPC` signature matches between interface (admin.go), no-op (admin_noop.go), and adapter (grpc_world_editor.go) ✓
