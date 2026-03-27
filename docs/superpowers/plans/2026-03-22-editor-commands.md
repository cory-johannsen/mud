# Editor Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Editor role enforcement and in-game commands to spawn NPCs/items/money, add/link/set rooms, list editor commands, with atomic YAML persistence and hot-reload.

**Architecture:** New `internal/gameserver/grpc_service_editor.go` file handles editor command dispatch; role check helpers added to `internal/gameserver/grpc_service.go`; `internal/game/world/editor.go` encapsulates atomic YAML write + hot-reload; `Manager.ReloadZone` added to `internal/game/world/manager.go`; help display updated in `internal/frontend/handlers/game_bridge.go`.

**Tech Stack:** Go, gRPC, YAML marshaling (`gopkg.in/yaml.v3`), existing session/room/NPC packages, `pgregory.net/rapid` for property-based tests

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/game/command/commands.go` | Modify | Add `CategoryEditor` constant; add new handler constants; recategorize `grant`, `summon_item`, `roomequip`; add new commands to `BuiltinCommands()` |
| `internal/game/npc/respawn.go` | Modify | Add `GetTemplate` accessor |
| `internal/game/npc/respawn_test.go` | Modify | Add tests for `GetTemplate` |
| `internal/game/world/manager.go` | Modify | Add `ReloadZone` method |
| `internal/game/world/manager_test.go` | Modify | Add tests for `ReloadZone` |
| `internal/game/world/loader.go` | Modify | Add `zoneToYAML` conversion function |
| `internal/game/world/loader_test.go` | Modify | Add tests for `zoneToYAML` round-trip |
| `internal/game/world/editor.go` | Create | `WorldEditor` struct with atomic write + hot-reload methods |
| `internal/game/world/editor_test.go` | Create | Tests for `WorldEditor` methods |
| `internal/gameserver/grpc_service.go` | Modify | Add `worldEditor *world.WorldEditor` field; add `requireEditor`/`requireAdmin` helpers; replace inline role checks; add `worldEditor` init in `NewGameServiceServer`; add 6 new dispatch cases in `HandlePlayerMessage` switch |
| `internal/gameserver/grpc_service_editor.go` | Create | All 6 new handler functions: `handleSpawnNPC`, `handleAddRoom`, `handleAddLink`, `handleRemoveLink`, `handleSetRoom`, `handleEditorCmds` |
| `internal/gameserver/grpc_service_editor_test.go` | Create | Tests for all 6 editor handlers |
| `internal/frontend/handlers/game_bridge.go` | Modify | Add `CategoryEditor` section to help display for editor/admin roles |
| `api/proto/game/v1/game.proto` | Modify | Add 6 new message types and oneof fields 106–111 |
| `internal/gameserver/gamev1/game.pb.go` | Regenerate | Regenerate from proto |
| `internal/gameserver/gamev1/game_grpc.pb.go` | Regenerate | Regenerate from proto |
| `deployments/k8s/mud/values.yaml` | Modify | Add `content.persistentVolume` section |
| `deployments/k8s/mud/templates/gameserver/deployment.yaml` | Modify | Conditional PVC mount and init container |
| `cmd/gameserver/main.go` | Modify | Add `--content-dir` flag; pass to `NewWorldEditor`; wire into `GameServiceServer` |

---

## Task 1: Add CategoryEditor and recategorize commands

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `internal/game/command/commands_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/game/command/commands_test.go`, add:

```go
func TestCategoryEditorExists(t *testing.T) {
    assert.Equal(t, "Editor", CategoryEditor)
}

func TestEditorCommandsRecategorized(t *testing.T) {
    cmds := BuiltinCommands()
    byName := make(map[string]Command)
    for _, c := range cmds {
        byName[c.Name] = c
    }
    // REQ-EC-1: grant, summon_item, roomequip must be CategoryEditor
    assert.Equal(t, CategoryEditor, byName["grant"].Category)
    assert.Equal(t, CategoryEditor, byName["summon_item"].Category)
    assert.Equal(t, CategoryEditor, byName["roomequip"].Category)
    // REQ-EC-2: setrole and teleport must remain CategoryAdmin
    assert.Equal(t, CategoryAdmin, byName["setrole"].Category)
    assert.Equal(t, CategoryAdmin, byName["teleport"].Category)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run TestCategoryEditorExists -v
```

Expected: FAIL with `CategoryEditor undefined`

- [ ] **Step 3: Add `CategoryEditor` constant and recategorize commands**

In `internal/game/command/commands.go`, add after `CategoryHidden`:

```go
// CategoryEditor marks commands available to editor and admin roles.
CategoryEditor = "Editor"
```

Change the three commands:
- `{Name: "roomequip", ..., Category: CategoryEditor, ...}`
- `{Name: "grant", ..., Category: CategoryEditor, ...}`
- `{Name: "summon_item", ..., Category: CategoryEditor, ...}`

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/command/commands.go internal/game/command/commands_test.go
git commit -m "feat(editor-commands): add CategoryEditor; recategorize grant/summon_item/roomequip (REQ-EC-1,2)"
```

---

## Task 2: Add requireEditor/requireAdmin helpers and replace inline role checks

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/setrole_handler_test.go` (or nearby tests to verify role check behavior)

The `GameServiceServer` struct field is `world *world.Manager`. The role constants are in `internal/storage/postgres` (already imported).

Existing inline checks to replace (lines ~4159, ~4191, ~4226, ~7585):
- `handleSetRole` line ~4159: `sess.Role != "admin"` → `requireAdmin(sess)`
- `handleTeleport` line ~4226: `sess.Role != "admin"` → `requireAdmin(sess)`
- `handleSummonItem` line ~4191: `sess.Role != "editor" && sess.Role != "admin"` → `requireEditor(sess)`
- `handleGrant` line ~7585: `sess.Role != "editor" && sess.Role != "admin"` → `requireEditor(sess)`
- `handleRoomEquip` (no current check): add `requireEditor(sess)` as first validation step

- [ ] **Step 1: Write failing tests for role-check helpers**

In `internal/gameserver/grpc_service_editor_test.go` (create new file):

```go
package gameserver_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"

    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/storage/postgres"
    . "github.com/cory-johannsen/mud/internal/gameserver"
)

func TestRequireEditor_AllowsEditorRole(t *testing.T) {
    sess := &session.PlayerSession{Role: postgres.RoleEditor}
    assert.Nil(t, RequireEditor(sess))
}

func TestRequireEditor_AllowsAdminRole(t *testing.T) {
    sess := &session.PlayerSession{Role: postgres.RoleAdmin}
    assert.Nil(t, RequireEditor(sess))
}

func TestRequireEditor_DeniesPlayerRole(t *testing.T) {
    sess := &session.PlayerSession{Role: postgres.RolePlayer}
    evt := RequireEditor(sess)
    assert.NotNil(t, evt)
}

func TestRequireAdmin_AllowsAdminRole(t *testing.T) {
    sess := &session.PlayerSession{Role: postgres.RoleAdmin}
    assert.Nil(t, RequireAdmin(sess))
}

func TestRequireAdmin_DeniesEditorRole(t *testing.T) {
    sess := &session.PlayerSession{Role: postgres.RoleEditor}
    evt := RequireAdmin(sess)
    assert.NotNil(t, evt)
}

// Property: requireEditor denies all roles except editor and admin.
func TestRequireEditorProperty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        role := rapid.StringOf(rapid.Rune()).Draw(t, "role")
        sess := &session.PlayerSession{Role: role}
        evt := RequireEditor(sess)
        if role == postgres.RoleEditor || role == postgres.RoleAdmin {
            assert.Nil(t, evt)
        } else {
            assert.NotNil(t, evt)
        }
    })
}
```

Note: exported wrappers `RequireEditor`/`RequireAdmin` must be added to `internal/gameserver/export_test.go` (the existing export_test.go pattern for this package).

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestRequireEditor -v 2>&1 | head -30
```

Expected: FAIL with `RequireEditor undefined`

- [ ] **Step 3: Add `requireEditor`/`requireAdmin` to grpc_service.go and export in export_test.go**

In `internal/gameserver/grpc_service.go`, add after the `errQuit` declaration:

```go
// requireEditor returns an error ServerEvent if the session lacks editor or admin role.
// Precondition: sess must be non-nil.
// Postcondition: Returns nil if role is editor or admin; returns error ServerEvent otherwise.
func requireEditor(sess *session.PlayerSession) *gamev1.ServerEvent {
    if sess.Role != postgres.RoleEditor && sess.Role != postgres.RoleAdmin {
        return errorEvent("permission denied: editor role required")
    }
    return nil
}

// requireAdmin returns an error ServerEvent if the session lacks admin role.
// Precondition: sess must be non-nil.
// Postcondition: Returns nil if role is admin; returns error ServerEvent otherwise.
func requireAdmin(sess *session.PlayerSession) *gamev1.ServerEvent {
    if sess.Role != postgres.RoleAdmin {
        return errorEvent("permission denied: admin role required")
    }
    return nil
}
```

In `internal/gameserver/export_test.go`, add:

```go
var RequireEditor = requireEditor
var RequireAdmin = requireAdmin
```

- [ ] **Step 4: Replace inline role checks in grpc_service.go**

In `handleSetRole` (~line 4159), replace:
```go
if sess.Role != "admin" {
    return errorEvent("permission denied: admin role required"), nil
}
```
with:
```go
if evt := requireAdmin(sess); evt != nil {
    return evt, nil
}
```

In `handleTeleport` (~line 4226), same replacement using `requireAdmin`.

In `handleSummonItem` (~line 4191), replace:
```go
if sess.Role != "editor" && sess.Role != "admin" {
    return errorEvent(...), nil
}
```
with:
```go
if evt := requireEditor(sess); evt != nil {
    return evt, nil
}
```

In `handleGrant` (~line 7585), same replacement using `requireEditor`.

In `handleRoomEquip` (no existing role check), add as the first validation step after fetching `sess`:
```go
if evt := requireEditor(sess); evt != nil {
    return evt, nil
}
```

- [ ] **Step 5: Run all tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -v 2>&1 | tail -20
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/export_test.go internal/gameserver/grpc_service_editor_test.go
git commit -m "feat(editor-commands): add requireEditor/requireAdmin helpers; replace inline role checks (REQ-EC-3,4,5,6)"
```

---

## Task 3: Add RespawnManager.GetTemplate accessor

**Files:**
- Modify: `internal/game/npc/respawn.go`
- Modify: `internal/game/npc/respawn_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/game/npc/respawn_test.go`, add:

```go
func TestRespawnManagerGetTemplate_Found(t *testing.T) {
    tmpl := &Template{ID: "guard"}
    rm := NewRespawnManager(nil, map[string]*Template{"guard": tmpl})
    got, ok := rm.GetTemplate("guard")
    assert.True(t, ok)
    assert.Equal(t, tmpl, got)
}

func TestRespawnManagerGetTemplate_NotFound(t *testing.T) {
    rm := NewRespawnManager(nil, nil)
    got, ok := rm.GetTemplate("nobody")
    assert.False(t, ok)
    assert.Nil(t, got)
}

// Property: GetTemplate returns found iff id was in the template map.
func TestRespawnManagerGetTemplateProperty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        id := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz"))).Filter(func(s string) bool { return len(s) > 0 }).Draw(t, "id")
        present := rapid.Bool().Draw(t, "present")
        templates := make(map[string]*Template)
        if present {
            templates[id] = &Template{ID: id}
        }
        rm := NewRespawnManager(nil, templates)
        _, ok := rm.GetTemplate(id)
        assert.Equal(t, present, ok)
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run TestRespawnManagerGetTemplate -v
```

Expected: FAIL with `rm.GetTemplate undefined`

- [ ] **Step 3: Implement GetTemplate**

In `internal/game/npc/respawn.go`, add after `NewRespawnManager`:

```go
// GetTemplate returns the NPC template with the given id, or (nil, false) if not found.
//
// Precondition: id must be non-empty for a meaningful result.
// Postcondition: Returned *Template is non-nil when ok is true.
func (r *RespawnManager) GetTemplate(id string) (*Template, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    t, ok := r.templates[id]
    return t, ok
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -10
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/respawn.go internal/game/npc/respawn_test.go
git commit -m "feat(editor-commands): add RespawnManager.GetTemplate accessor (REQ-EC-7)"
```

---

## Task 4: Add Manager.ReloadZone and zoneToYAML

**Files:**
- Modify: `internal/game/world/manager.go`
- Modify: `internal/game/world/manager_test.go`
- Modify: `internal/game/world/loader.go`
- Modify: `internal/game/world/loader_test.go`

### 4a: zoneToYAML round-trip

- [ ] **Step 1: Write failing test for zoneToYAML**

In `internal/game/world/loader_test.go`, add:

```go
func TestZoneToYAMLRoundTrip(t *testing.T) {
    // Load a real zone fixture, convert to YAML, reload, verify equality.
    original, err := LoadZoneFromFile("testdata/basic_zone.yaml")
    require.NoError(t, err)

    yf := ZoneToYAML(original)
    data, err := yaml.Marshal(yf)
    require.NoError(t, err)

    reloaded, err := LoadZoneFromBytes(data)
    require.NoError(t, err)

    assert.Equal(t, original.ID, reloaded.ID)
    assert.Equal(t, original.Name, reloaded.Name)
    assert.Equal(t, len(original.Rooms), len(reloaded.Rooms))
    for id, room := range original.Rooms {
        r2, ok := reloaded.Rooms[id]
        require.True(t, ok, "room %q missing after round-trip", id)
        assert.Equal(t, room.Title, r2.Title)
        assert.Equal(t, room.MapX, r2.MapX)
        assert.Equal(t, room.MapY, r2.MapY)
        assert.Equal(t, len(room.Exits), len(r2.Exits))
    }
}
```

Note: `ZoneToYAML` is an exported wrapper for testing. Add to `internal/game/world/export_test.go` (create if needed):

```go
package world

var ZoneToYAML = zoneToYAML
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run TestZoneToYAMLRoundTrip -v
```

Expected: FAIL with `zoneToYAML undefined`

- [ ] **Step 3: Implement zoneToYAML in loader.go**

In `internal/game/world/loader.go`, add after `convertYAMLZone`:

```go
// zoneToYAML converts a Zone domain object back to its YAML serialization form.
//
// Precondition: zone must be non-nil.
// Postcondition: Returns a yamlZoneFile that, when marshaled and passed to LoadZoneFromBytes,
// produces a Zone equal to the input (modulo floating-point trap fields).
func zoneToYAML(zone *Zone) yamlZoneFile {
    yrooms := make([]yamlRoom, 0, len(zone.Rooms))
    for _, room := range zone.Rooms {
        mapX := room.MapX
        mapY := room.MapY
        yr := yamlRoom{
            ID:              room.ID,
            Title:           room.Title,
            Description:     room.Description,
            Properties:      room.Properties,
            SkillChecks:     room.SkillChecks,
            Effects:         room.Effects,
            MapX:            &mapX,
            MapY:            &mapY,
            DangerLevel:     room.DangerLevel,
            RoomTrapChance:  room.RoomTrapChance,
            CoverTrapChance: room.CoverTrapChance,
        }
        for _, exit := range room.Exits {
            yr.Exits = append(yr.Exits, yamlExit{
                Direction: string(exit.Direction),
                Target:    exit.TargetRoom,
                Locked:    exit.Locked,
                Hidden:    exit.Hidden,
            })
        }
        for _, sp := range room.Spawns {
            yr.Spawns = append(yr.Spawns, yamlRoomSpawn{
                Template:     sp.Template,
                Count:        sp.Count,
                RespawnAfter: sp.RespawnAfter,
            })
        }
        for _, eq := range room.Equipment {
            respawnStr := ""
            if eq.RespawnAfter > 0 {
                respawnStr = eq.RespawnAfter.String()
            }
            yr.Equipment = append(yr.Equipment, yamlRoomEquipment{
                ItemID:       eq.ItemID,
                Description:  eq.Description,
                MaxCount:     eq.MaxCount,
                RespawnAfter: respawnStr,
                Immovable:    eq.Immovable,
                Script:       eq.Script,
                SkillChecks:  eq.SkillChecks,
                TrapTemplate: eq.TrapTemplate,
            })
        }
        for _, tr := range room.Traps {
            yr.Traps = append(yr.Traps, yamlRoomTrap{
                Template: tr.TemplateID,
                Position: tr.Position,
            })
        }
        yrooms = append(yrooms, yr)
    }

    yz := yamlZone{
        ID:                     zone.ID,
        Name:                   zone.Name,
        Description:            zone.Description,
        StartRoom:              zone.StartRoom,
        ScriptDir:              zone.ScriptDir,
        ScriptInstructionLimit: zone.ScriptInstructionLimit,
        Rooms:                  yrooms,
        DangerLevel:            zone.DangerLevel,
        RoomTrapChance:         zone.RoomTrapChance,
        CoverTrapChance:        zone.CoverTrapChance,
    }
    if zone.TrapProbabilities != nil {
        tp := &yamlTrapProbabilities{
            RoomTrapChance:  zone.TrapProbabilities.RoomTrapChance,
            CoverTrapChance: zone.TrapProbabilities.CoverTrapChance,
        }
        for _, e := range zone.TrapProbabilities.TrapPool {
            tp.TrapPool = append(tp.TrapPool, yamlTrapEntry{Template: e.Template, Weight: e.Weight})
        }
        yz.TrapProbabilities = tp
    }
    return yamlZoneFile{Zone: yz}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run TestZoneToYAMLRoundTrip -v
```

Expected: PASS

### 4b: ReloadZone

- [ ] **Step 5: Write failing test for ReloadZone**

In `internal/game/world/manager_test.go`, add:

```go
func TestReloadZone_ReplacesRooms(t *testing.T) {
    z1 := &Zone{
        ID:        "zone1",
        StartRoom: "r1",
        Rooms: map[string]*Room{
            "r1": {ID: "r1", ZoneID: "zone1", Title: "Old Title", MapX: 0, MapY: 0},
        },
    }
    mgr, err := NewManager([]*Zone{z1})
    require.NoError(t, err)

    // Reload with updated zone.
    z1Updated := &Zone{
        ID:        "zone1",
        StartRoom: "r1",
        Rooms: map[string]*Room{
            "r1": {ID: "r1", ZoneID: "zone1", Title: "New Title", MapX: 0, MapY: 0},
        },
    }
    err = mgr.ReloadZone(z1Updated)
    require.NoError(t, err)

    room, ok := mgr.GetRoom("r1")
    require.True(t, ok)
    assert.Equal(t, "New Title", room.Title)
}

func TestReloadZone_RemovesDeletedRooms(t *testing.T) {
    z := &Zone{
        ID:        "zone1",
        StartRoom: "r1",
        Rooms: map[string]*Room{
            "r1": {ID: "r1", ZoneID: "zone1", MapX: 0, MapY: 0},
            "r2": {ID: "r2", ZoneID: "zone1", MapX: 1, MapY: 0},
        },
    }
    mgr, err := NewManager([]*Zone{z})
    require.NoError(t, err)

    zUpdated := &Zone{
        ID:        "zone1",
        StartRoom: "r1",
        Rooms: map[string]*Room{
            "r1": {ID: "r1", ZoneID: "zone1", MapX: 0, MapY: 0},
        },
    }
    err = mgr.ReloadZone(zUpdated)
    require.NoError(t, err)

    _, ok := mgr.GetRoom("r2")
    assert.False(t, ok)
}

// Property: after ReloadZone, every room in the new zone is in the manager.
func TestReloadZoneProperty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        roomCount := rapid.IntRange(1, 5).Draw(t, "roomCount")
        rooms := make(map[string]*Room, roomCount)
        for i := 0; i < roomCount; i++ {
            id := fmt.Sprintf("r%d", i)
            rooms[id] = &Room{ID: id, ZoneID: "zone1", MapX: i, MapY: 0}
        }
        zone := &Zone{ID: "zone1", StartRoom: "r0", Rooms: rooms}
        mgr, err := NewManager([]*Zone{zone})
        if err != nil {
            t.Skip()
        }
        err = mgr.ReloadZone(zone)
        assert.NoError(t, err)
        for id := range rooms {
            _, ok := mgr.GetRoom(id)
            assert.True(t, ok)
        }
    })
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run TestReloadZone -v
```

Expected: FAIL with `mgr.ReloadZone undefined`

- [ ] **Step 7: Implement ReloadZone in manager.go**

In `internal/game/world/manager.go`, add after `AllZones`:

```go
// ReloadZone replaces the zone and its rooms in the manager under a write lock.
//
// Precondition: zone must be non-nil; zone.ID must match an already-loaded zone.
// Postcondition: All rooms for the replaced zone are removed and replaced with
// the new zone's rooms. Callers that previously obtained *Room pointers from
// GetRoom() MUST re-fetch after this call returns, as old pointers become stale.
// Returns nil on success or an error if exit validation fails.
func (m *Manager) ReloadZone(zone *Zone) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Remove all rooms belonging to the old zone.
    if old, ok := m.zones[zone.ID]; ok {
        for id := range old.Rooms {
            delete(m.rooms, id)
        }
    }
    delete(m.zones, zone.ID)

    // Insert new zone and rooms.
    m.zones[zone.ID] = zone
    for id, room := range zone.Rooms {
        m.rooms[id] = room
    }

    // Validate exits for the reloaded zone only (callers re-fetch after this).
    for _, room := range zone.Rooms {
        for _, exit := range room.Exits {
            if _, ok := m.rooms[exit.TargetRoom]; !ok {
                return fmt.Errorf("zone %q: room %q: exit %q targets unknown room %q",
                    zone.ID, room.ID, exit.Direction, exit.TargetRoom)
            }
        }
    }
    return nil
}
```

- [ ] **Step 8: Run all world tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -v 2>&1 | tail -20
```

Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/game/world/loader.go internal/game/world/loader_test.go internal/game/world/manager.go internal/game/world/manager_test.go internal/game/world/export_test.go
git commit -m "feat(editor-commands): add zoneToYAML conversion and Manager.ReloadZone (REQ-EC-16,31)"
```

---

## Task 5: Create WorldEditor

**Files:**
- Create: `internal/game/world/editor.go`
- Create: `internal/game/world/editor_test.go`

- [ ] **Step 1: Write failing tests for WorldEditor**

Create `internal/game/world/editor_test.go`:

```go
package world_test

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"

    "github.com/cory-johannsen/mud/internal/game/world"
)

func TestNewWorldEditor_NonWritableDir(t *testing.T) {
    // Use a path that cannot exist as writable.
    _, err := world.NewWorldEditor("/nonexistent/path/xyz", nil)
    assert.Error(t, err)
}

func TestNewWorldEditor_WritableDir(t *testing.T) {
    dir := t.TempDir()
    // Need a real zone on disk. Write a minimal zone file.
    zoneYAML := `zone:
  id: "test"
  name: "Test Zone"
  start_room: "r1"
  rooms:
    - id: "r1"
      title: "Room One"
      description: "A room."
      map_x: 0
      map_y: 0
      exits: []
`
    require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(zoneYAML), 0644))
    zones, err := world.LoadZonesFromDir(dir)
    require.NoError(t, err)
    mgr, err := world.NewManager(zones)
    require.NoError(t, err)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)
    assert.NotNil(t, editor)
}

func TestWorldEditor_AddRoom_Success(t *testing.T) {
    dir, mgr := setupEditorFixture(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.AddRoom("test", "r2", "Room Two")
    require.NoError(t, err)

    room, ok := mgr.GetRoom("r2")
    require.True(t, ok)
    assert.Equal(t, "Room Two", room.Title)
}

func TestWorldEditor_AddRoom_DuplicateID(t *testing.T) {
    dir, mgr := setupEditorFixture(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.AddRoom("test", "r1", "Duplicate")
    assert.ErrorContains(t, err, "already exists")
}

func TestWorldEditor_AddRoom_UnknownZone(t *testing.T) {
    dir, mgr := setupEditorFixture(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.AddRoom("nonexistent", "r2", "Room Two")
    assert.ErrorContains(t, err, "unknown zone")
}

func TestWorldEditor_AddLink_Success(t *testing.T) {
    dir, mgr := setupEditorFixtureTwoRooms(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.AddLink("r1", "north", "r2")
    require.NoError(t, err)

    r1, ok := mgr.GetRoom("r1")
    require.True(t, ok)
    found := false
    for _, exit := range r1.Exits {
        if exit.Direction == world.North && exit.TargetRoom == "r2" {
            found = true
        }
    }
    assert.True(t, found, "r1 should have north exit to r2")
}

func TestWorldEditor_AddLink_InvalidDirection(t *testing.T) {
    dir, mgr := setupEditorFixtureTwoRooms(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.AddLink("r1", "diagonal", "r2")
    assert.ErrorContains(t, err, "invalid direction")
}

func TestWorldEditor_RemoveLink_Success(t *testing.T) {
    dir, mgr := setupEditorFixtureTwoRooms(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    require.NoError(t, editor.AddLink("r1", "north", "r2"))
    require.NoError(t, editor.RemoveLink("r1", "north"))

    r1, ok := mgr.GetRoom("r1")
    require.True(t, ok)
    for _, exit := range r1.Exits {
        assert.NotEqual(t, world.North, exit.Direction)
    }
}

func TestWorldEditor_RemoveLink_NotPresent(t *testing.T) {
    dir, mgr := setupEditorFixture(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.RemoveLink("r1", "north")
    assert.ErrorContains(t, err, "no north exit")
}

func TestWorldEditor_SetRoomField_Title(t *testing.T) {
    dir, mgr := setupEditorFixture(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.SetRoomField("r1", "title", "New Title")
    require.NoError(t, err)

    room, ok := mgr.GetRoom("r1")
    require.True(t, ok)
    assert.Equal(t, "New Title", room.Title)
}

func TestWorldEditor_SetRoomField_InvalidDangerLevel(t *testing.T) {
    dir, mgr := setupEditorFixture(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.SetRoomField("r1", "danger_level", "lethal")
    assert.ErrorContains(t, err, "invalid danger level")
}

func TestWorldEditor_SetRoomField_UnknownField(t *testing.T) {
    dir, mgr := setupEditorFixture(t)
    editor, err := world.NewWorldEditor(dir, mgr)
    require.NoError(t, err)

    err = editor.SetRoomField("r1", "color", "blue")
    assert.ErrorContains(t, err, "unknown field")
}

// Property: AddRoom with a new unique ID always succeeds.
func TestWorldEditorAddRoomProperty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        dir, mgr := setupEditorFixture(t)
        editor, err := world.NewWorldEditor(dir, mgr)
        if err != nil {
            t.Skip()
        }
        newID := rapid.StringMatching(`[a-z][a-z0-9_]{1,15}`).Draw(t, "id")
        if newID == "r1" {
            t.Skip()
        }
        title := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz "))).Filter(func(s string) bool { return len(s) > 0 }).Draw(t, "title")
        err = editor.AddRoom("test", newID, title)
        assert.NoError(t, err)
    })
}

// setupEditorFixture creates a temp dir with a single-zone/single-room fixture.
func setupEditorFixture(t *testing.T) (string, *world.Manager) {
    t.Helper()
    dir := t.TempDir()
    zoneYAML := `zone:
  id: "test"
  name: "Test Zone"
  start_room: "r1"
  rooms:
    - id: "r1"
      title: "Room One"
      description: "A room."
      map_x: 0
      map_y: 0
      exits: []
`
    require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(zoneYAML), 0644))
    zones, err := world.LoadZonesFromDir(dir)
    require.NoError(t, err)
    mgr, err := world.NewManager(zones)
    require.NoError(t, err)
    return dir, mgr
}

// setupEditorFixtureTwoRooms creates a temp dir with two rooms in the same zone.
func setupEditorFixtureTwoRooms(t *testing.T) (string, *world.Manager) {
    t.Helper()
    dir := t.TempDir()
    zoneYAML := `zone:
  id: "test"
  name: "Test Zone"
  start_room: "r1"
  rooms:
    - id: "r1"
      title: "Room One"
      description: "A room."
      map_x: 0
      map_y: 0
      exits: []
    - id: "r2"
      title: "Room Two"
      description: "Another room."
      map_x: 1
      map_y: 0
      exits: []
`
    require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(zoneYAML), 0644))
    zones, err := world.LoadZonesFromDir(dir)
    require.NoError(t, err)
    mgr, err := world.NewManager(zones)
    require.NoError(t, err)
    return dir, mgr
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run TestNewWorldEditor -v
```

Expected: FAIL with `world.NewWorldEditor undefined`

- [ ] **Step 3: Implement WorldEditor**

Create `internal/game/world/editor.go`:

```go
// Package world: editor.go provides atomic YAML write and hot-reload for world editing.
package world

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"

    "github.com/cory-johannsen/mud/internal/game/danger"
)

// WorldEditor encapsulates atomic YAML write and hot-reload operations for world editing.
//
// Invariant: contentDir is writable at construction time.
// Concurrency: All methods are safe for concurrent use via the Manager's internal locking.
type WorldEditor struct {
    contentDir string
    manager    *Manager
}

// NewWorldEditor creates a WorldEditor that writes zone files to contentDir and reloads via manager.
//
// Precondition: contentDir must be a writable directory path; manager must be non-nil.
// Postcondition: Returns (nil, error) if contentDir is not writable. Returns non-nil *WorldEditor on success.
func NewWorldEditor(contentDir string, manager *Manager) (*WorldEditor, error) {
    probe := filepath.Join(contentDir, ".write_probe")
    if err := os.WriteFile(probe, []byte("probe"), 0600); err != nil {
        return nil, fmt.Errorf("content dir %q is not writable: %w", contentDir, err)
    }
    _ = os.Remove(probe)
    return &WorldEditor{contentDir: contentDir, manager: manager}, nil
}

// AddRoom appends a new room to an existing zone, writes atomically, and hot-reloads.
//
// Precondition: zoneID must match a loaded zone; roomID must not already exist in the zone.
// Postcondition: On success, the zone is persisted and the manager reflects the new room.
func (e *WorldEditor) AddRoom(zoneID, roomID, title string) error {
    zone, ok := e.manager.GetZone(zoneID)
    if !ok {
        return fmt.Errorf("unknown zone: %s", zoneID)
    }
    if _, exists := zone.Rooms[roomID]; exists {
        return fmt.Errorf("room ID %s already exists in zone %s", roomID, zoneID)
    }

    // Compute map position: max(MapX)+1 for x; MapY of the room with max MapX.
    maxX := -1
    mapY := 0
    for _, r := range zone.Rooms {
        if r.MapX > maxX {
            maxX = r.MapX
            mapY = r.MapY
        }
    }
    newX := maxX + 1
    if maxX < 0 {
        newX = 0
        mapY = 0
    }

    newRoom := &Room{
        ID:          roomID,
        ZoneID:      zoneID,
        Title:       title,
        Description: "",
        MapX:        newX,
        MapY:        mapY,
        Properties:  make(map[string]string),
    }
    zone.Rooms[roomID] = newRoom

    if err := e.writeZoneAtomic(zone); err != nil {
        delete(zone.Rooms, roomID)
        return err
    }
    return e.manager.ReloadZone(zone)
}

// AddLink adds a bidirectional exit between two rooms, writes affected zone(s) atomically, and hot-reloads each.
//
// Precondition: Both rooms must exist; direction must be a standard direction; neither direction slot may be occupied.
// Postcondition: On success, both rooms persist the new exits and the manager is updated.
func (e *WorldEditor) AddLink(fromRoomID, directionStr, toRoomID string) error {
    dir := Direction(directionStr)
    if !dir.IsStandard() {
        return fmt.Errorf("invalid direction: %s. Valid: north, south, east, west, northeast, northwest, southeast, southwest, up, down", directionStr)
    }
    fromRoom, ok := e.manager.GetRoom(fromRoomID)
    if !ok {
        return fmt.Errorf("unknown room: %s", fromRoomID)
    }
    toRoom, ok := e.manager.GetRoom(toRoomID)
    if !ok {
        return fmt.Errorf("unknown room: %s", toRoomID)
    }
    for _, exit := range fromRoom.Exits {
        if exit.Direction == dir {
            return fmt.Errorf("direction %s is already occupied in %s", directionStr, fromRoomID)
        }
    }
    rev := dir.Opposite()
    for _, exit := range toRoom.Exits {
        if exit.Direction == rev {
            return fmt.Errorf("reverse direction %s is already occupied in %s", string(rev), toRoomID)
        }
    }

    fromRoom.Exits = append(fromRoom.Exits, Exit{Direction: dir, TargetRoom: toRoomID})
    toRoom.Exits = append(toRoom.Exits, Exit{Direction: rev, TargetRoom: fromRoomID})

    fromZone, _ := e.manager.GetZone(fromRoom.ZoneID)
    toZone, _ := e.manager.GetZone(toRoom.ZoneID)

    if fromZone.ID == toZone.ID {
        // REQ-EC-20: single write and reload for same-zone link.
        if err := e.writeZoneAtomic(fromZone); err != nil {
            // Undo in-memory mutation.
            fromRoom.Exits = fromRoom.Exits[:len(fromRoom.Exits)-1]
            toRoom.Exits = toRoom.Exits[:len(toRoom.Exits)-1]
            return err
        }
        return e.manager.ReloadZone(fromZone)
    }

    if err := e.writeZoneAtomic(fromZone); err != nil {
        fromRoom.Exits = fromRoom.Exits[:len(fromRoom.Exits)-1]
        toRoom.Exits = toRoom.Exits[:len(toRoom.Exits)-1]
        return err
    }
    if err := e.manager.ReloadZone(fromZone); err != nil {
        return err
    }
    if err := e.writeZoneAtomic(toZone); err != nil {
        toRoom.Exits = toRoom.Exits[:len(toRoom.Exits)-1]
        return err
    }
    return e.manager.ReloadZone(toZone)
}

// RemoveLink removes a single directional exit from a room, writes atomically, and hot-reloads.
//
// Precondition: roomID must exist; direction must be present in the room's exits.
// Postcondition: On success, the exit is removed, persisted, and manager is updated.
func (e *WorldEditor) RemoveLink(roomID, directionStr string) error {
    room, ok := e.manager.GetRoom(roomID)
    if !ok {
        return fmt.Errorf("unknown room: %s", roomID)
    }
    dir := Direction(directionStr)
    found := -1
    for i, exit := range room.Exits {
        if exit.Direction == dir {
            found = i
            break
        }
    }
    if found < 0 {
        return fmt.Errorf("no %s exit exists in %s", directionStr, roomID)
    }

    removed := room.Exits[found]
    room.Exits = append(room.Exits[:found], room.Exits[found+1:]...)

    zone, _ := e.manager.GetZone(room.ZoneID)
    if err := e.writeZoneAtomic(zone); err != nil {
        // Undo.
        room.Exits = append(room.Exits[:found], append([]Exit{removed}, room.Exits[found:]...)...)
        return err
    }
    return e.manager.ReloadZone(zone)
}

// SetRoomField sets a supported field on the room the editor has targeted, writes atomically, and hot-reloads.
//
// Supported fields: title, description, danger_level.
// Precondition: roomID must exist; field and value must be valid per field constraints.
// Postcondition: On success, the room field is updated, persisted, and manager is updated.
func (e *WorldEditor) SetRoomField(roomID, field, value string) error {
    room, ok := e.manager.GetRoom(roomID)
    if !ok {
        return fmt.Errorf("unknown room: %s", roomID)
    }

    switch field {
    case "title":
        if value == "" {
            return fmt.Errorf("title cannot be empty")
        }
        room.Title = value
    case "description":
        if value == "" {
            return fmt.Errorf("description cannot be empty")
        }
        room.Description = value
    case "danger_level":
        switch danger.DangerLevel(value) {
        case danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar:
            // valid
        default:
            return fmt.Errorf("invalid danger level: %s. Valid: safe, sketchy, dangerous, all_out_war", value)
        }
        room.DangerLevel = value
    default:
        return fmt.Errorf("unknown field: %s. Valid fields: title, description, danger_level", field)
    }

    zone, _ := e.manager.GetZone(room.ZoneID)
    if err := e.writeZoneAtomic(zone); err != nil {
        return err
    }
    return e.manager.ReloadZone(zone)
}

// writeZoneAtomic serializes zone to its YAML file atomically.
//
// Precondition: zone must be non-nil; zone.ID must correspond to a file in e.contentDir.
// Postcondition: The target YAML file is replaced atomically. Temp file is removed on any error.
func (e *WorldEditor) writeZoneAtomic(zone *Zone) error {
    targetPath := filepath.Join(e.contentDir, zone.ID+".yaml")

    yf := zoneToYAML(zone)
    data, err := yaml.Marshal(yf)
    if err != nil {
        return fmt.Errorf("marshaling zone %s: %w", zone.ID, err)
    }

    tmp, err := os.CreateTemp(e.contentDir, ".zone_write_*")
    if err != nil {
        return fmt.Errorf("creating temp file for zone %s: %w", zone.ID, err)
    }
    tmpName := tmp.Name()
    defer func() {
        if err != nil {
            _ = os.Remove(tmpName)
        }
    }()

    if _, err = tmp.Write(data); err != nil {
        _ = tmp.Close()
        return fmt.Errorf("writing zone %s: %w", zone.ID, err)
    }
    if err = tmp.Sync(); err != nil {
        _ = tmp.Close()
        return fmt.Errorf("syncing zone %s: %w", zone.ID, err)
    }
    if err = tmp.Close(); err != nil {
        return fmt.Errorf("closing temp for zone %s: %w", zone.ID, err)
    }
    if err = os.Rename(tmpName, targetPath); err != nil {
        return fmt.Errorf("renaming zone %s: %w", zone.ID, err)
    }
    return nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -v 2>&1 | tail -20
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/world/editor.go internal/game/world/editor_test.go
git commit -m "feat(editor-commands): add WorldEditor with atomic YAML write and hot-reload (REQ-EC-13,15,16,17,19,20,22,24,30)"
```

---

## Task 6: Add proto messages and regenerate

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go`, `internal/gameserver/gamev1/game_grpc.pb.go`

The last used oneof field is 105 (`release_request`). New fields start at 106.

- [ ] **Step 1: Add new messages and oneof fields to game.proto**

In `api/proto/game/v1/game.proto`, add after `ReleaseRequest release_request = 105;` in the `ClientMessage` oneof:

```protobuf
    SpawnNPCRequest    spawn_npc    = 106;
    AddRoomRequest     add_room     = 107;
    AddLinkRequest     add_link     = 108;
    RemoveLinkRequest  remove_link  = 109;
    SetRoomRequest     set_room     = 110;
    EditorCmdsRequest  editor_cmds  = 111;
```

Add the message definitions (place with other request messages):

```protobuf
// SpawnNPCRequest asks the server to spawn one NPC instance from a template.
message SpawnNPCRequest {
    string template_id = 1;
    string room_id     = 2; // empty = editor's current room
}

// AddRoomRequest asks the server to add a new room to a zone.
message AddRoomRequest {
    string zone_id = 1;
    string room_id = 2;
    string title   = 3;
}

// AddLinkRequest asks the server to add a bidirectional exit between two rooms.
message AddLinkRequest {
    string from_room_id = 1;
    string direction    = 2;
    string to_room_id   = 3;
}

// RemoveLinkRequest asks the server to remove a directional exit from a room.
message RemoveLinkRequest {
    string room_id   = 1;
    string direction = 2;
}

// SetRoomRequest asks the server to set a field on the editor's current room.
message SetRoomRequest {
    string field = 1;
    string value = 2;
}

// EditorCmdsRequest asks the server to list all editor commands.
message EditorCmdsRequest {}
```

- [ ] **Step 2: Regenerate protobuf code**

```bash
cd /home/cjohannsen/src/mud && mise exec -- make proto 2>&1 || mise exec -- buf generate 2>&1 || mise exec -- protoc --go_out=. --go-grpc_out=. api/proto/game/v1/game.proto 2>&1
```

Check the Makefile for the exact proto generation command:
```bash
grep -n "proto\|protoc\|buf" /home/cjohannsen/src/mud/Makefile | head -10
```

Run whichever command the Makefile defines. Verify `internal/gameserver/gamev1/game.pb.go` now contains `ClientMessage_SpawnNpc`.

- [ ] **Step 3: Verify build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(editor-commands): add proto messages for 6 editor commands (fields 106-111)"
```

---

## Task 7: Wire WorldEditor into GameServiceServer and add startup check

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`

- [ ] **Step 1: Add worldEditor field to GameServiceServer**

In `internal/gameserver/grpc_service.go`, in the `GameServiceServer` struct, add after the `world` field:

```go
worldEditor *world.WorldEditor
```

The `worldEditor` is NOT passed through `NewGameServiceServer` — it is set after construction via `SetWorldEditor` (see Step 2). This avoids requiring writability at wire-time and allows the startup check to determine availability.

- [ ] **Step 2: Add `--content-dir` flag and WorldEditor init in cmd/gameserver/main.go**

In `cmd/gameserver/main.go`, add flag:

```go
contentDir := flag.String("content-dir", "content", "path to content directory for world editing")
```

After `Initialize(...)` call, attempt `NewWorldEditor`:

```go
worldEditor, weErr := world.NewWorldEditor(*contentDir, app.GRPCService.World())
if weErr != nil {
    logger.Warn("WARNING: content/ is not writable — world-editing commands disabled.", zap.Error(weErr))
} else {
    app.GRPCService.SetWorldEditor(worldEditor)
}
```

Add exported `World()` accessor and `SetWorldEditor()` setter to `GameServiceServer` in `grpc_service.go`:

```go
// World returns the world Manager. Used by startup initialization.
func (s *GameServiceServer) World() *world.Manager {
    return s.world
}

// SetWorldEditor sets the WorldEditor after startup writability check.
// Passing nil disables world-editing commands.
func (s *GameServiceServer) SetWorldEditor(we *world.WorldEditor) {
    s.worldEditor = we
}
```

- [ ] **Step 3: Verify build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/deps.go cmd/gameserver/main.go
git commit -m "feat(editor-commands): wire WorldEditor into GameServiceServer; add startup writability check (REQ-EC-13,14,30)"
```

---

## Task 8: Implement 6 editor command handlers

**Files:**
- Create: `internal/gameserver/grpc_service_editor.go`
- Modify: `internal/gameserver/grpc_service_editor_test.go`

This also covers adding handler constants and new commands to `BuiltinCommands()`.

### 8a: Add handler constants and register new commands

- [ ] **Step 1: Add handler constants to commands.go**

In `internal/game/command/commands.go`, add to the `Handler*` constants:

```go
HandlerSpawnNPC    = "spawn_npc"
HandlerAddRoom     = "add_room"
HandlerAddLink     = "add_link"
HandlerRemoveLink  = "remove_link"
HandlerSetRoom     = "set_room"
HandlerEditorCmds  = "ecmds"
```

Add to `BuiltinCommands()`:

```go
// Editor commands (REQ-EC-9,18,21,23,25,27)
{Name: "spawnnpc",   Category: CategoryEditor, Handler: HandlerSpawnNPC,   Help: "Spawn an NPC from template into a room"},
{Name: "addroom",    Category: CategoryEditor, Handler: HandlerAddRoom,    Help: "Add a new room to a zone"},
{Name: "addlink",    Category: CategoryEditor, Handler: HandlerAddLink,    Help: "Add a bidirectional exit between two rooms"},
{Name: "removelink", Category: CategoryEditor, Handler: HandlerRemoveLink, Help: "Remove a directional exit from a room"},
{Name: "setroom",    Category: CategoryEditor, Handler: HandlerSetRoom,    Help: "Set a field on the current room"},
{Name: "ecmds",      Category: CategoryEditor, Handler: HandlerEditorCmds, Help: "List all editor commands"},
```

### 8b: Implement handlers

- [ ] **Step 2: Write failing tests for handlers**

In `internal/gameserver/grpc_service_editor_test.go`, add test cases for:
- `handleSpawnNPC`: unknown template, unknown room, success
- `handleAddRoom`: no worldEditor (disabled), unknown zone, success
- `handleAddLink`: invalid direction, success
- `handleRemoveLink`: no such direction, success
- `handleSetRoom`: unknown field, invalid danger_level, success
- `handleEditorCmds`: returns sorted list with all CategoryEditor commands

Use the existing test helper pattern from `grpc_service_testhelper_test.go`. Look at an existing simple test like `grpc_service_grant_test.go` to understand the helper setup.

```bash
head -60 /home/cjohannsen/src/mud/internal/gameserver/grpc_service_grant_test.go
```

Write tests following the same pattern before implementing handlers.

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestHandleSpawnNPC -v 2>&1 | head -20
```

- [ ] **Step 4: Implement handlers**

Create `internal/gameserver/grpc_service_editor.go`:

```go
package gameserver

import (
    "fmt"
    "sort"
    "strings"

    "github.com/cory-johannsen/mud/internal/game/command"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleSpawnNPC spawns a runtime-only NPC instance from a template. (REQ-EC-8,9)
//
// Precondition: uid session must exist; template_id must be non-empty.
// Postcondition: NPC instance created in target room. No YAML written.
func (s *GameServiceServer) handleSpawnNPC(uid string, req *gamev1.SpawnNPCRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.Get(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }
    if evt := requireEditor(sess); evt != nil {
        return evt, nil
    }

    tmpl, ok := s.respawnMgr.GetTemplate(req.GetTemplateId())
    if !ok {
        return errorEvent(fmt.Sprintf("Unknown NPC template: %s.", req.GetTemplateId())), nil
    }

    roomID := req.GetRoomId()
    if roomID == "" {
        roomID = sess.RoomID
    }
    if _, roomOk := s.world.GetRoom(roomID); !roomOk {
        return errorEvent(fmt.Sprintf("Unknown room: %s.", roomID)), nil
    }

    if _, err := s.npcMgr.Spawn(tmpl, roomID); err != nil {
        return errorEvent(fmt.Sprintf("Failed to spawn NPC: %v", err)), nil
    }

    return messageEvent(fmt.Sprintf("Spawned %s in %s.", tmpl.Name, roomID)), nil
}

// handleAddRoom adds a new room to a zone. (REQ-EC-17,18)
//
// Precondition: worldEditor must be non-nil; zone_id and room_id must be non-empty.
// Postcondition: New room added to zone YAML and hot-reloaded.
func (s *GameServiceServer) handleAddRoom(uid string, req *gamev1.AddRoomRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.Get(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }
    if evt := requireEditor(sess); evt != nil {
        return evt, nil
    }
    if s.worldEditor == nil {
        return errorEvent("world-editing is not available on this server"), nil
    }

    if err := s.worldEditor.AddRoom(req.GetZoneId(), req.GetRoomId(), req.GetTitle()); err != nil {
        return errorEvent(err.Error()), nil
    }
    return messageEvent(fmt.Sprintf("Room %s added to zone %s.", req.GetRoomId(), req.GetZoneId())), nil
}

// handleAddLink adds a bidirectional exit between two rooms. (REQ-EC-19,20,21)
//
// Precondition: worldEditor must be non-nil; from_room_id, direction, to_room_id must be non-empty.
// Postcondition: Exit added in affected zone YAML(s) and hot-reloaded.
func (s *GameServiceServer) handleAddLink(uid string, req *gamev1.AddLinkRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.Get(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }
    if evt := requireEditor(sess); evt != nil {
        return evt, nil
    }
    if s.worldEditor == nil {
        return errorEvent("world-editing is not available on this server"), nil
    }

    if err := s.worldEditor.AddLink(req.GetFromRoomId(), req.GetDirection(), req.GetToRoomId()); err != nil {
        return errorEvent(err.Error()), nil
    }
    return messageEvent(fmt.Sprintf("Linked %s %s ↔ %s.", req.GetFromRoomId(), req.GetDirection(), req.GetToRoomId())), nil
}

// handleRemoveLink removes a directional exit from a room. (REQ-EC-22,23)
//
// Precondition: worldEditor must be non-nil; room_id and direction must be non-empty.
// Postcondition: Exit removed in zone YAML and hot-reloaded.
func (s *GameServiceServer) handleRemoveLink(uid string, req *gamev1.RemoveLinkRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.Get(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }
    if evt := requireEditor(sess); evt != nil {
        return evt, nil
    }
    if s.worldEditor == nil {
        return errorEvent("world-editing is not available on this server"), nil
    }

    if err := s.worldEditor.RemoveLink(req.GetRoomId(), req.GetDirection()); err != nil {
        return errorEvent(err.Error()), nil
    }
    return messageEvent(fmt.Sprintf("Removed %s exit from %s.", req.GetDirection(), req.GetRoomId())), nil
}

// handleSetRoom sets a field on the editor's current room. (REQ-EC-24,25,26)
//
// Precondition: worldEditor must be non-nil; field must be one of title/description/danger_level.
// Postcondition: Room field updated in zone YAML, hot-reloaded, and updated display pushed to
// all players in the affected room when field is title or description.
func (s *GameServiceServer) handleSetRoom(uid string, req *gamev1.SetRoomRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.Get(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }
    if evt := requireEditor(sess); evt != nil {
        return evt, nil
    }
    if s.worldEditor == nil {
        return errorEvent("world-editing is not available on this server"), nil
    }

    roomID := sess.RoomID
    if err := s.worldEditor.SetRoomField(roomID, req.GetField(), req.GetValue()); err != nil {
        return errorEvent(err.Error()), nil
    }

    // REQ-EC-26: push updated room display to all players in the affected room.
    if req.GetField() == "title" || req.GetField() == "description" {
        s.pushRoomViewToAllInRoom(roomID)
    }

    return messageEvent(fmt.Sprintf("Room %s %s updated.", roomID, req.GetField())), nil
}

// handleEditorCmds lists all CategoryEditor commands sorted alphabetically. (REQ-EC-27,28)
//
// Precondition: caller must have editor or admin role.
// Postcondition: Returns sorted list of all CategoryEditor commands with descriptions.
func (s *GameServiceServer) handleEditorCmds(uid string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.Get(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }
    if evt := requireEditor(sess); evt != nil {
        return evt, nil
    }

    allCmds := s.commands.Commands()
    var editorCmds []*command.Command
    for _, cmd := range allCmds {
        if cmd.Category == command.CategoryEditor {
            editorCmds = append(editorCmds, cmd)
        }
    }
    sort.Slice(editorCmds, func(i, j int) bool {
        return editorCmds[i].Name < editorCmds[j].Name
    })

    var lines []string
    lines = append(lines, "Editor commands:")
    for _, cmd := range editorCmds {
        lines = append(lines, fmt.Sprintf("  %-14s %s", cmd.Name, cmd.Help))
    }
    return messageEvent(strings.Join(lines, "\r\n")), nil
}
```

Note: The `messageEvent` helper is defined in `grpc_service.go` at line ~3438. The `errorEvent` helper is at line ~3426. The `pushRoomViewToAllInRoom` method is defined in `grpc_service.go` and used at line ~319.

- [ ] **Step 5: Add 6 dispatch cases to HandlePlayerMessage**

In `internal/gameserver/grpc_service.go`, in the `HandlePlayerMessage` switch, add after the `ReleaseRequest` case:

```go
case *gamev1.ClientMessage_SpawnNpc:
    return s.handleSpawnNPC(uid, p.SpawnNpc)
case *gamev1.ClientMessage_AddRoom:
    return s.handleAddRoom(uid, p.AddRoom)
case *gamev1.ClientMessage_AddLink:
    return s.handleAddLink(uid, p.AddLink)
case *gamev1.ClientMessage_RemoveLink:
    return s.handleRemoveLink(uid, p.RemoveLink)
case *gamev1.ClientMessage_SetRoom:
    return s.handleSetRoom(uid, p.SetRoom)
case *gamev1.ClientMessage_EditorCmds:
    return s.handleEditorCmds(uid)
```

- [ ] **Step 6: Run all tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -v 2>&1 | tail -30
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service_editor.go internal/gameserver/grpc_service_editor_test.go internal/gameserver/grpc_service.go internal/game/command/commands.go
git commit -m "feat(editor-commands): implement 6 editor handlers and dispatch cases (REQ-EC-7-9,17-28,32)"
```

---

## Task 9: Update help display for CategoryEditor

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`

The help display renders twice (split-screen and normal modes) in `game_bridge.go` around lines 795–855. Both blocks check `role == postgres.RoleAdmin` for the Admin section. Add an Editor section after the Admin section in both blocks.

- [ ] **Step 1: Write failing test**

In `internal/frontend/handlers/game_bridge_test.go` (or create), add a test that calls the help render with editor role and verifies the `CategoryEditor` section appears, and with player role verifies it does not appear. Look at existing tests in the frontend package before writing.

```bash
ls /home/cjohannsen/src/mud/internal/frontend/handlers/
grep -n "TestHelp\|func Test" /home/cjohannsen/src/mud/internal/frontend/handlers/game_bridge_test.go 2>/dev/null | head -10
```

After inspecting the test file, write an appropriate test using the existing pattern.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/... -run TestHelp -v 2>&1 | head -20
```

- [ ] **Step 3: Update game_bridge.go**

In `internal/frontend/handlers/game_bridge.go`, in both help render blocks (split-screen ~line 809 and normal ~line 843), after the `if role == postgres.RoleAdmin {` Admin section block, add:

```go
if role == postgres.RoleEditor || role == postgres.RoleAdmin {
    if cmds := byCategory[command.CategoryEditor]; len(cmds) > 0 {
        sb.WriteString("\r\n")
        sb.WriteString(telnet.Colorf(telnet.BrightYellow, "  Editor:"))
        for _, cmd := range cmds {
            aliases := ""
            if len(cmd.Aliases) > 0 {
                aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
            }
            sb.WriteString("\r\n")
            sb.WriteString(telnet.Colorf(telnet.Green, "    %-12s", cmd.Name) + aliases + " — " + cmd.Help)
        }
    }
}
```

For the non-split-screen block (using `conn.WriteLine`), use the same pattern with `conn.WriteLine` instead of `sb.WriteString`.

- [ ] **Step 4: Run all frontend tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/... -v 2>&1 | tail -20
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/frontend/handlers/game_bridge.go
git commit -m "feat(editor-commands): add CategoryEditor section to help display for editor/admin roles (REQ-EC-29)"
```

---

## Task 10: Helm chart — PVC and init container

**Files:**
- Modify: `deployments/k8s/mud/values.yaml`
- Modify: `deployments/k8s/mud/templates/gameserver/deployment.yaml`

- [ ] **Step 1: Add content.persistentVolume to values.yaml**

In `deployments/k8s/mud/values.yaml`, add after the `frontend:` section:

```yaml
content:
  persistentVolume:
    enabled: false        # set true in production
    storageClass: ""      # uses cluster default if empty
    size: 1Gi
    accessMode: ReadWriteOnce
```

- [ ] **Step 2: Update deployment.yaml with conditional PVC mount and init container**

In `deployments/k8s/mud/templates/gameserver/deployment.yaml`, add after the `resources:` block in the `containers:` array:

```yaml
          {{- if .Values.content.persistentVolume.enabled }}
          volumeMounts:
            - name: content
              mountPath: /app/content
          {{- end }}
      {{- if .Values.content.persistentVolume.enabled }}
      initContainers:
        - name: content-seed
          image: {{ .Values.image.registry }}/{{ .Values.image.gameserverRepository }}:{{ .Values.image.tag }}
          command:
            - /bin/sh
            - -c
            - |
              if [ -z "$(find /mnt/content -name '*.yaml' 2>/dev/null | head -1)" ]; then
                cp -r /app/content/. /mnt/content/
              fi
          volumeMounts:
            - name: content
              mountPath: /mnt/content
      volumes:
        - name: content
          persistentVolumeClaim:
            claimName: mud-content
      {{- end }}
```

Also add a PVC template at `deployments/k8s/mud/templates/gameserver/content-pvc.yaml`:

```yaml
{{- if .Values.content.persistentVolume.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mud-content
  namespace: mud
spec:
  accessModes:
    - {{ .Values.content.persistentVolume.accessMode }}
  resources:
    requests:
      storage: {{ .Values.content.persistentVolume.size }}
  {{- if .Values.content.persistentVolume.storageClass }}
  storageClassName: {{ .Values.content.persistentVolume.storageClass }}
  {{- end }}
{{- end }}
```

- [ ] **Step 3: Validate helm template renders cleanly**

```bash
cd /home/cjohannsen/src/mud && helm template mud deployments/k8s/mud --set db.password=test 2>&1 | head -40
```

Expected: no template errors

- [ ] **Step 4: Commit**

```bash
git add deployments/k8s/mud/values.yaml deployments/k8s/mud/templates/gameserver/deployment.yaml deployments/k8s/mud/templates/gameserver/content-pvc.yaml
git commit -m "feat(editor-commands): add content PVC, init container, and helm values (REQ-EC-10,11,12)"
```

---

## Task 11: Full test suite and final validation

- [ ] **Step 1: Run complete test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -40
```

Expected: 100% PASS, no failures

- [ ] **Step 2: Run build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```

Expected: no errors

- [ ] **Step 3: Verify all REQ-EC requirements are covered**

Cross-check:
- REQ-EC-1,2: Task 1 (`CategoryEditor`, command recategorization)
- REQ-EC-3,4,5,6: Task 2 (`requireEditor`/`requireAdmin`, inline replacements)
- REQ-EC-7: Task 3 (`RespawnManager.GetTemplate`)
- REQ-EC-8,9: Task 8 (`handleSpawnNPC`, no YAML write, `CategoryEditor`)
- REQ-EC-10,11,12: Task 10 (helm PVC + init container)
- REQ-EC-13,14: Task 7 (startup writability check + warning log)
- REQ-EC-15,16: Task 5 (`writeZoneAtomic` + `ReloadZone`)
- REQ-EC-17,18: Task 5/8 (`addroom` atomic + CategoryEditor)
- REQ-EC-19,20,21: Task 5/8 (`addlink`, same-zone single write, CategoryEditor)
- REQ-EC-22,23: Task 5/8 (`removelink`, CategoryEditor)
- REQ-EC-24,25,26: Task 5/8 (`setroom`, CategoryEditor, player push)
- REQ-EC-27,28: Task 8 (`ecmds`, CategoryEditor, sorted)
- REQ-EC-29: Task 9 (help shows Editor section for editor/admin)
- REQ-EC-30: Task 7 (nil `worldEditor` returns disabled message)
- REQ-EC-31: Task 4 (`ReloadZone` write-lock, stale pointer warning in docstring)
- REQ-EC-32: Task 8 (6 dispatch cases in switch)

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "feat(editor-commands): complete implementation — all REQ-EC-1 through REQ-EC-32 covered"
```
