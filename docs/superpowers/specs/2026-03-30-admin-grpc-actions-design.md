# Admin gRPC Actions

**Date:** 2026-03-30
**Status:** spec

## Overview

The web admin panel (`cmd/webclient/`) has no-op stub implementations for player session management — listing online players, kicking, messaging, and teleporting. This feature wires those stubs to the gameserver via four new unary gRPC RPCs on the existing `GameService`, replacing `noOpSessionManager` with a real `grpcSessionManager`.

## Architecture

```
Web Server (cmd/webclient/)
  AdminHandler
    └── grpcSessionManager (implements SessionManager)
          └── gamev1.GameServiceClient
                │  unary gRPC (internal network)
                ▼
  Gameserver (cmd/gameserver/)
    GameServiceServer
      └── grpc_service_admin.go
            ├── AdminListSessions → s.sessions.AllPlayers()
            ├── AdminKickPlayer   → target.Entity.Push(Disconnected)
            ├── AdminMessagePlayer→ target.Entity.Push(MessageEvent)
            └── AdminTeleportPlayer → handleTeleport logic
```

**Trust model:** network isolation only — no gRPC-level auth. The admin RPCs are reachable only from within the k8s cluster.

## Requirements

### 1. Proto (`api/proto/game/v1/game.proto`)

- REQ-AGA-1: Four unary RPCs MUST be added to `GameService`:
  - `AdminListSessions(AdminListSessionsRequest) returns (AdminListSessionsResponse)`
  - `AdminKickPlayer(AdminKickRequest) returns (AdminKickResponse)`
  - `AdminMessagePlayer(AdminMessageRequest) returns (AdminMessageResponse)`
  - `AdminTeleportPlayer(AdminTeleportRequest) returns (AdminTeleportResponse)`

- REQ-AGA-2: The following messages MUST be added:

```proto
message AdminSessionInfo {
  int64  char_id     = 1;
  string player_name = 2;
  int32  level       = 3;
  string room_id     = 4;
  string zone        = 5;
  int32  current_hp  = 6;
  int64  account_id  = 7;
}
message AdminListSessionsRequest  {}
message AdminListSessionsResponse { repeated AdminSessionInfo sessions = 1; }

message AdminKickRequest     { int64 char_id = 1; }
message AdminKickResponse    {}

message AdminMessageRequest  { int64 char_id = 1; string text = 2; }
message AdminMessageResponse {}

message AdminTeleportRequest  { int64 char_id = 1; string room_id = 2; }
message AdminTeleportResponse {}
```

### 2. Gameserver (`internal/gameserver/grpc_service_admin.go`)

- REQ-AGA-3: A new file `internal/gameserver/grpc_service_admin.go` MUST implement the four RPCs as methods on `GameServiceServer`.

- REQ-AGA-4: `AdminListSessions` MUST enumerate all active player sessions via `s.sessions`, map each to `AdminSessionInfo`, and return them. Sessions with no character loaded MUST be omitted.

- REQ-AGA-5: `AdminKickPlayer` MUST look up the target session by `char_id`, push a `Disconnected` `ServerEvent` directly to the target's stream (same `Entity.Push()` pattern used in `handleTeleport`), and return an empty response. If the char ID is not found, MUST return a gRPC `NotFound` error.

- REQ-AGA-6: `AdminMessagePlayer` MUST look up the target session by `char_id`, push a `MessageEvent` `ServerEvent` with the provided text directly to the target's stream, and return an empty response. If the char ID is not found, MUST return a gRPC `NotFound` error.

- REQ-AGA-7: `AdminTeleportPlayer` MUST reuse the existing `handleTeleport` logic (move player via `s.sessions.MovePlayer()`, persist via `s.charSaver.SaveState()`, broadcast DEPART/ARRIVE events, push RoomView to target). It MUST accept `char_id int64` and `room_id string` as inputs. If the char ID or room ID is not found, MUST return the appropriate gRPC `NotFound` error.

- REQ-AGA-8: None of the four RPCs MUST perform role checks — trust is enforced at the network boundary.

### 3. Web Server (`cmd/webclient/`)

- REQ-AGA-9: A new file `cmd/webclient/handlers/grpc_session_manager.go` MUST implement the `SessionManager` interface with a `grpcSessionManager` struct holding a `gamev1.GameServiceClient`.

- REQ-AGA-10: `grpcSessionManager.AllSessions()` MUST call `AdminListSessions` and map each `AdminSessionInfo` to a `grpcManagedSession`. On RPC error, MUST return an empty slice (not panic).

- REQ-AGA-11: `grpcSessionManager.GetSession(charID int64)` MUST call `AllSessions()` and return the matching session. If not found, MUST return `nil, false`.

- REQ-AGA-12: `grpcManagedSession` MUST implement the `ManagedSession` interface:
  - `CharID()`, `AccountID()`, `PlayerName()`, `Level()`, `RoomID()`, `Zone()`, `CurrentHP()` MUST return values from the cached `AdminSessionInfo`.
  - `SendAdminMessage(text string) error` MUST call `AdminMessagePlayer` RPC.
  - `Kick() error` MUST call `AdminKickPlayer` RPC.

- REQ-AGA-13: `HandleTeleportPlayer` in `admin.go` MUST be updated to call `AdminTeleportPlayer` RPC directly (using the gRPC client), replacing the current stub that returns `"teleport_enqueued"`. On success it MUST return HTTP 200 with `{"status": "ok"}`.

- REQ-AGA-14: `server.go` MUST replace `NewNoOpSessionManager()` with `NewGRPCSessionManager(grpcClient)`, passing the existing shared gRPC client.

- REQ-AGA-15: `NewGRPCSessionManager` MUST be exported and accept a `gamev1.GameServiceClient` parameter.

### 4. Testing

- REQ-AGA-16: The gameserver MUST have unit tests for each of the four new RPC methods in `internal/gameserver/grpc_service_admin_test.go`, using property-based testing where applicable.

- REQ-AGA-17: `grpcSessionManager` MUST have unit tests using a mock `GameServiceClient` (generated or hand-written), verifying correct RPC dispatch and error handling.

## Out of Scope

- WorldEditor wiring (`noOpWorldEditor` remains; spawn-npc and zone/room editing are a separate future feature)
- gRPC-level authentication for admin RPCs
- Admin audit logging
