# Editor Commands — Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `editor-commands` (priority 290)
**Dependencies:** none

---

## Overview

Formalizes the Editor role in the command system: adds `CategoryEditor`, a `RequireEditor` helper, an NPC spawn command, four world-editing commands with atomic YAML persistence to a PersistentVolumeClaim, and an editor command listing.

---

## 1. Role Enforcement & Category

### 1.1 CategoryEditor Constant

`internal/game/command/commands.go` gains:

```go
CategoryEditor = "Editor"
```

Existing editor-scoped commands MUST be re-categorized:

- REQ-EC-1: `grant`, `summon_item`, and `roomequip` MUST be changed from `CategoryAdmin` to `CategoryEditor`.
- REQ-EC-2: `setrole` and `teleport` MUST remain in `CategoryAdmin`.

### 1.2 RequireEditor Helper

Two helper functions are added to `internal/gameserver/grpc_service.go`. They use the typed constants from `internal/storage/postgres` (already imported by `grpc_service.go`):

```go
// requireEditor returns an error ServerEvent if the session lacks editor or admin role.
func requireEditor(sess *session.PlayerSession) *gamev1.ServerEvent {
    if sess.Role != storage.RoleEditor && sess.Role != storage.RoleAdmin {
        return errorEvent("permission denied: editor role required")
    }
    return nil
}

// requireAdmin returns an error ServerEvent if the session lacks admin role.
func requireAdmin(sess *session.PlayerSession) *gamev1.ServerEvent {
    if sess.Role != storage.RoleAdmin {
        return errorEvent("permission denied: admin role required")
    }
    return nil
}
```

- REQ-EC-3: All existing inline role string comparisons in `grpc_service.go` (`handleSetRole`, `handleTeleport`, `handleGrant`, `handleSummonItem`) MUST be replaced with calls to `requireEditor` or `requireAdmin`.
- REQ-EC-4: `handleRoomEquip` currently has no role check. A `requireEditor` call MUST be added as the first validation step.
- REQ-EC-5: All new editor command handlers MUST call `requireEditor` as the first validation step.
- REQ-EC-6: All new admin command handlers MUST call `requireAdmin` as the first validation step.

---

## 2. NPC Spawn Command

### 2.1 NPC Template Accessor

`internal/game/npc/respawn.go` gains a public method on `RespawnManager`:

```go
// GetTemplate returns the NPC template with the given ID, or false if not found.
func (r *RespawnManager) GetTemplate(id string) (*Template, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    t, ok := r.templates[id]
    return t, ok
}
```

- REQ-EC-7: `RespawnManager.GetTemplate` MUST be added before `spawnnpc` can be implemented.

### 2.2 Command Definition

**`spawnnpc <template_id> [room_id]`** — spawns one NPC instance from a registered template into the target room.

- If `room_id` is omitted, the editor's current room is used.
- Looks up template via `respawnMgr.GetTemplate(templateID)`.
- Spawns a live instance via `npcMgr` using the same path as the normal respawn system.
- On success: `"Spawned <template name> in <room title>."`
- On template not found: `"Unknown NPC template: <template_id>."`
- On room not found: `"Unknown room: <room_id>."`

- REQ-EC-8: `spawnnpc` MUST NOT create a permanent YAML entry. The spawned instance is runtime-only and behaves identically to a normally-respawned NPC.
- REQ-EC-9: `spawnnpc` MUST be categorized as `CategoryEditor`.

### 2.3 Proto Message

```protobuf
message SpawnNPCRequest {
    string template_id = 1;
    string room_id     = 2; // empty = current room
}
```

Added to the `ClientMessage` oneof as field 86: `spawn_npc`.

---

## 3. World Editing Commands

### 3.1 Infrastructure: PersistentVolumeClaim

The `content/` directory MUST be mounted as a Kubernetes PersistentVolumeClaim so YAML edits survive pod restarts and redeployments.

- REQ-EC-10: `deployments/k8s/mud/values.yaml` MUST gain a `content.persistentVolume` section:

```yaml
content:
  persistentVolume:
    enabled: false        # set true in production
    storageClass: ""      # uses cluster default if empty
    size: 1Gi
    accessMode: ReadWriteOnce
```

- REQ-EC-11: `deployments/k8s/mud/templates/deployment.yaml` MUST conditionally mount the PVC at `/app/content` when `content.persistentVolume.enabled` is `true`.
- REQ-EC-12: When `content.persistentVolume.enabled` is `true`, an init container MUST be added to the pod spec. The init container MUST copy the contents of `/app/content` (from the game image) to the PVC mount path only if the PVC is empty (i.e., the init container checks whether the target directory has any `.yaml` files; if none, it copies). This seeds the PVC from the image on first deploy without overwriting subsequent edits.
- REQ-EC-13: The gameserver MUST verify at startup that the `content/` path is writable using `os.WriteFile` to a temp probe file. If the write fails, world-editing commands MUST be disabled.
- REQ-EC-14: When world-editing commands are disabled (REQ-EC-13), the server MUST log: `"WARNING: content/ is not writable — world-editing commands disabled."` at startup. All non-editing server functionality MUST proceed normally.

### 3.2 Atomic YAML Writes and Hot-Reload

- REQ-EC-15: All YAML writes MUST be atomic: write to a temp file created via `os.CreateTemp` in the same directory as the target, write the full YAML, call `Sync()`, close, then `os.Rename` to replace the original. On any error the temp file MUST be removed.
- REQ-EC-16: After a successful YAML write, the `WorldEditor` MUST parse the written YAML back into a `*Zone` and call `worldMgr.ReloadZone(zone)` (new method; see §5.3). The parsed zone is the authoritative in-memory representation after the reload.

### 3.3 addroom

**`addroom <zone_id> <room_id> <title>`** — adds a new room to an existing zone.

- Appends a new room with:
  - `id: <room_id>`
  - `title: <title>`
  - `description: ""`
  - `map_x`: `max(all existing rooms' MapX) + 1`; `0` if the zone has no rooms.
  - `map_y`: the `MapY` of the room with the maximum `MapX`; `0` if no rooms exist.
  - No exits.
- On success: `"Room <room_id> added to zone <zone_id>."`
- On zone not found: `"Unknown zone: <zone_id>."`
- On duplicate room ID within the zone: `"Room ID <room_id> already exists in zone <zone_id>."`

- REQ-EC-17: `addroom` MUST write atomically (REQ-EC-15) and hot-reload (REQ-EC-16).
- REQ-EC-18: `addroom` MUST be categorized as `CategoryEditor`.

### 3.4 Proto Message

```protobuf
message AddRoomRequest {
    string zone_id = 1;
    string room_id = 2;
    string title   = 3;
}
```

Added to the `ClientMessage` oneof as field 87: `add_room`.

### 3.5 addlink

**`addlink <from_room_id> <direction> <to_room_id>`** — adds a bidirectional exit between two rooms.

Valid directions: all values in `world.StandardDirections` (`north`, `south`, `east`, `west`, `northeast`, `northwest`, `southeast`, `southwest`, `up`, `down`).

- Adds `direction → to_room_id` exit on `from_room_id`.
- Adds the reverse direction (via `world.Direction.Opposite()`) exit on `to_room_id` pointing back to `from_room_id`.
- On success: `"Linked <from_room_id> <direction> ↔ <to_room_id>."`
- On invalid direction: `"Invalid direction: <direction>. Valid: north, south, east, west, northeast, northwest, southeast, southwest, up, down."`
- On either room not found: `"Unknown room: <room_id>."`
- On direction already occupied in `from_room_id`: `"Direction <direction> is already occupied in <from_room_id>."`
- On reverse direction already occupied in `to_room_id`: `"Reverse direction <reverse> is already occupied in <to_room_id>."`

- REQ-EC-19: `addlink` MUST write the affected zone YAML(s) atomically and hot-reload each affected zone.
- REQ-EC-20: If both rooms are in the same zone, only one YAML write and one hot-reload occur.
- REQ-EC-21: `addlink` MUST be categorized as `CategoryEditor`.

### 3.6 Proto Message

```protobuf
message AddLinkRequest {
    string from_room_id = 1;
    string direction    = 2;
    string to_room_id   = 3;
}
```

Added to the `ClientMessage` oneof as field 88: `add_link`.

### 3.7 removelink

**`removelink <room_id> <direction>`** — removes one directional exit from a room.

- Removes only the specified exit. The reverse link is NOT automatically removed.
- On success: `"Removed <direction> exit from <room_id>."`
- On room not found: `"Unknown room: <room_id>."`
- On direction not present: `"No <direction> exit exists in <room_id>."`

- REQ-EC-22: `removelink` MUST write the affected zone YAML atomically and hot-reload.
- REQ-EC-23: `removelink` MUST be categorized as `CategoryEditor`.

### 3.8 Proto Message

```protobuf
message RemoveLinkRequest {
    string room_id    = 1;
    string direction  = 2;
}
```

Added to the `ClientMessage` oneof as field 89: `remove_link`.

### 3.9 setroom

**`setroom <field> <value>`** — sets a field on the room the editor is currently standing in.

Supported fields and validation:

| Field | Type | Valid Values / Validation |
|-------|------|--------------------------|
| `title` | string | non-empty |
| `description` | string | non-empty |
| `danger_level` | string | one of: `safe`, `sketchy`, `dangerous`, `all_out_war` (from `internal/game/danger/level.go`; note: the comment in `world/model.go` line 262 is stale and MUST be updated) |

- On success: `"Room <room_id> <field> updated."`
- On unsupported field: `"Unknown field: <field>. Valid fields: title, description, danger_level."`
- On invalid `danger_level` value: `"Invalid danger level: <value>. Valid: safe, sketchy, dangerous, all_out_war."`
- On empty value for `title` or `description`: `"<field> cannot be empty."`

- REQ-EC-24: `setroom` MUST write the affected zone YAML atomically and hot-reload.
- REQ-EC-25: `setroom` MUST be categorized as `CategoryEditor`.
- REQ-EC-26: After a successful `setroom title` or `setroom description`, all players currently in the affected room MUST receive the updated room display.

### 3.10 Proto Message

```protobuf
message SetRoomRequest {
    string field = 1;
    string value = 2;
}
```

Added to the `ClientMessage` oneof as field 90: `set_room`.

---

## 4. Editor Help & Listing

### 4.1 ecmds Command

**`ecmds`** — lists all `CategoryEditor` commands with one-line descriptions.

- Requires Editor or Admin role (`requireEditor`).
- Output format mirrors the existing `help` command category sections.
- On permission denied: `"permission denied: editor role required"`.

- REQ-EC-27: `ecmds` MUST be categorized as `CategoryEditor`.
- REQ-EC-28: `ecmds` output MUST list all commands with `Category == CategoryEditor`, sorted alphabetically by command name.

### 4.2 Proto Message

```protobuf
message EditorCmdsRequest {}
```

Added to the `ClientMessage` oneof as field 91: `editor_cmds`.

### 4.3 help Command Integration

- REQ-EC-29: The `help` command output MUST include the `CategoryEditor` section when the requesting player's role is `editor` or `admin`. Players with `player` role MUST NOT see the Editor section.

---

## 5. Architecture

### 5.1 Command Registration

New commands registered in `BuiltinCommands()`:

- REQ-EC-32: The `HandlePlayerMessage` dispatch switch in `grpc_service.go` MUST include a case for each new proto oneof field (`spawn_npc`, `add_room`, `add_link`, `remove_link`, `set_room`, `editor_cmds`) routing to the corresponding handler function.

```go
{Name: "spawnnpc",   Category: CategoryEditor, Handler: HandlerSpawnNPC,   Help: "Spawn an NPC from template into a room"},
{Name: "addroom",    Category: CategoryEditor, Handler: HandlerAddRoom,    Help: "Add a new room to a zone"},
{Name: "addlink",    Category: CategoryEditor, Handler: HandlerAddLink,    Help: "Add a bidirectional exit between two rooms"},
{Name: "removelink", Category: CategoryEditor, Handler: HandlerRemoveLink, Help: "Remove a directional exit from a room"},
{Name: "setroom",    Category: CategoryEditor, Handler: HandlerSetRoom,    Help: "Set a field on the current room"},
{Name: "ecmds",      Category: CategoryEditor, Handler: HandlerEditorCmds, Help: "List all editor commands"},
```

### 5.2 WorldEditor Service

`internal/game/world/editor.go` — encapsulates atomic YAML write + hot-reload logic:

```go
type WorldEditor struct {
    contentDir string
    manager    *Manager
}

func NewWorldEditor(contentDir string, manager *Manager) (*WorldEditor, error)
// Returns nil, error if contentDir is not writable.

func (e *WorldEditor) AddRoom(zoneID, roomID, title string) error
func (e *WorldEditor) AddLink(fromRoomID, direction, toRoomID string) error
func (e *WorldEditor) RemoveLink(roomID, direction string) error
func (e *WorldEditor) SetRoomField(roomID, field, value string) error

// writeZoneAtomic serializes the Zone to yamlZoneFile, writes to a temp file
// in the same directory, syncs, closes, then os.Rename to the target path.
// Removes the temp file on any error.
func (e *WorldEditor) writeZoneAtomic(zone *Zone) error
```

`writeZoneAtomic` accepts `*world.Zone`, converts it to `yamlZoneFile` (the existing unexported serialization type), marshals to YAML, and writes atomically. The `Zone → yamlZoneFile` conversion function is added to `loader.go` as `zoneToYAML(z *Zone) yamlZoneFile`.

- REQ-EC-30: `WorldEditor` MUST be initialized at server startup. If `contentDir` is not writable, `NewWorldEditor` MUST return `nil, error`. The `GameServiceServer` MUST store a `*WorldEditor` that is `nil` when world-editing is unavailable. All handlers that require `WorldEditor` MUST return `"world-editing is not available on this server"` when it is nil.

### 5.3 ReloadZone Method

`internal/game/world/manager.go` gains:

```go
// ReloadZone replaces the zone and its rooms in the manager under a write lock.
// All callers holding *Room pointers obtained before this call MUST re-fetch
// via GetRoom() after the call returns, as old pointers become stale.
func (m *Manager) ReloadZone(zone *Zone) error
```

Implementation:
1. Acquires `m.mu.Lock()`.
2. Removes all existing `m.rooms` entries where `room.ZoneID == zone.ID`.
3. Removes `m.zones[zone.ID]`.
4. Inserts new zone and all its rooms.
5. Calls `m.ValidateExits()` for the affected zone's exits only.
6. Releases the lock.

- REQ-EC-31: `ReloadZone` MUST acquire the manager write lock for the duration of the replacement. Callers that previously obtained `*Room` pointers from `GetRoom()` MUST NOT assume those pointers remain valid after `ReloadZone` completes; they MUST re-fetch.

### 5.4 Helm Chart Changes

The chart is at `deployments/k8s/mud/`. The `values.yaml` gains the `content.persistentVolume` section (REQ-EC-10). The gameserver deployment template is at `deployments/k8s/mud/templates/gameserver/deployment.yaml` and gains the conditional PVC volume mount and init container (REQ-EC-11/12).

---

## 6. Requirements Summary

- REQ-EC-1: `grant`, `summon_item`, `roomequip` recategorized to `CategoryEditor`.
- REQ-EC-2: `setrole`, `teleport` remain in `CategoryAdmin`.
- REQ-EC-3: Existing inline role checks replaced with `requireEditor`/`requireAdmin`.
- REQ-EC-4: `handleRoomEquip` gains a `requireEditor` call.
- REQ-EC-5: New editor handlers call `requireEditor` first.
- REQ-EC-6: New admin handlers call `requireAdmin` first.
- REQ-EC-7: `RespawnManager.GetTemplate` accessor added before implementing `spawnnpc`.
- REQ-EC-8: `spawnnpc` creates runtime-only NPC instances; no YAML write.
- REQ-EC-9: `spawnnpc` categorized as `CategoryEditor`.
- REQ-EC-10: `deployments/k8s/mud/values.yaml` gains `content.persistentVolume` section.
- REQ-EC-11: Deployment template conditionally mounts PVC at `/app/content`.
- REQ-EC-12: Init container seeds PVC from image on first deploy (no-op if YAML files exist).
- REQ-EC-13: Gameserver verifies `content/` writability at startup.
- REQ-EC-14: Startup warning logged when world-editing disabled; other functionality unaffected.
- REQ-EC-15: All YAML writes atomic via `os.CreateTemp` + sync + `os.Rename`; temp cleaned on error.
- REQ-EC-16: Successful YAML write triggers `worldMgr.ReloadZone(zone)`.
- REQ-EC-17: `addroom` writes atomically and hot-reloads.
- REQ-EC-18: `addroom` categorized as `CategoryEditor`.
- REQ-EC-19: `addlink` writes affected zone YAML(s) atomically and hot-reloads each.
- REQ-EC-20: Same-zone `addlink` performs one YAML write and one hot-reload.
- REQ-EC-21: `addlink` categorized as `CategoryEditor`.
- REQ-EC-22: `removelink` writes atomically and hot-reloads.
- REQ-EC-23: `removelink` categorized as `CategoryEditor`.
- REQ-EC-24: `setroom` writes atomically and hot-reloads.
- REQ-EC-25: `setroom` categorized as `CategoryEditor`.
- REQ-EC-26: `setroom title`/`description` triggers updated room display for all players in room.
- REQ-EC-27: `ecmds` categorized as `CategoryEditor`.
- REQ-EC-28: `ecmds` lists all `CategoryEditor` commands sorted alphabetically.
- REQ-EC-29: `help` includes `CategoryEditor` section for editor/admin roles only.
- REQ-EC-30: `WorldEditor` is nil when `content/` is not writable; handlers return disabled message.
- REQ-EC-31: `ReloadZone` holds write lock for full replacement; callers must re-fetch `*Room` pointers afterward.
- REQ-EC-32: `HandlePlayerMessage` dispatch switch MUST include cases for all six new proto oneof fields.
