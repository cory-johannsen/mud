# Room Equipment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Permanent fixtures and auto-spawning items in rooms, configurable via YAML and in-game editor commands, with Lua-scripted use effects.

**Architecture:** Add `RoomEquipmentConfig` to the `Room` struct (YAML-loaded, mirrors existing NPC `RoomSpawnConfig` pattern). A new in-memory `RoomEquipmentManager` owns live instances and a respawn goroutine. `handleLook` appends equipment to `RoomView`. A new `use` command invokes the Lua script for an equipment item. A new `roomequip` editor command provides full CRUD at runtime, writing config changes back to zone YAML.

**Tech Stack:** Go, protobuf/gRPC, `pgregory.net/rapid` for property tests, `testify` for assertions, `gopher-lua` scripting via existing `scripting.Manager`.

---

### Task 1: Add RoomEquipmentConfig to world model and YAML loader

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/loader.go`
- Modify: `internal/game/world/loader_test.go` (or create if missing)

**Step 1: Read model.go**

Read `internal/game/world/model.go` lines 1-120. Note the existing `RoomSpawnConfig` struct and `Room` struct — `Equipment` will follow the same pattern.

**Step 2: Add RoomEquipmentConfig struct and Equipment field to Room**

In `internal/game/world/model.go`, add after `RoomSpawnConfig`:

```go
// RoomEquipmentConfig defines a static or respawning item present in a room.
//
// Precondition: ItemID must reference a valid ItemDef ID.
// Postcondition: Immovable items with RespawnAfter==0 persist indefinitely.
type RoomEquipmentConfig struct {
	ItemID       string        // references inventory.ItemDef.ID
	MaxCount     int           // max live instances allowed in this room
	RespawnAfter time.Duration // 0 = permanent (never despawn); >0 = respawn after pickup
	Immovable    bool          // if true, cannot be picked up
	Script       string        // path to Lua script for use effect; empty = no effect
}
```

Add `"time"` to imports if not present.

In the `Room` struct, add after `Spawns []RoomSpawnConfig`:

```go
Equipment []RoomEquipmentConfig
```

**Step 3: Read loader.go**

Read `internal/game/world/loader.go`. Note the `yamlRoom` struct and how `Spawns` is parsed. Mirror that pattern for `Equipment`.

**Step 4: Add YAML structs and parsing to loader.go**

Add a `yamlRoomEquipment` struct alongside `yamlRoomSpawn`:

```go
type yamlRoomEquipment struct {
	ItemID       string `yaml:"item_id"`
	MaxCount     int    `yaml:"max_count"`
	RespawnAfter string `yaml:"respawn_after"` // e.g. "5m", "0s"
	Immovable    bool   `yaml:"immovable"`
	Script       string `yaml:"script"`
}
```

Add `Equipment []yamlRoomEquipment \`yaml:"equipment"\`` to the `yamlRoom` struct.

In the room conversion function, parse `Equipment`:

```go
for _, e := range yr.Equipment {
	dur, err := time.ParseDuration(e.RespawnAfter)
	if err != nil {
		dur = 0
	}
	room.Equipment = append(room.Equipment, RoomEquipmentConfig{
		ItemID:       e.ItemID,
		MaxCount:     e.MaxCount,
		RespawnAfter: dur,
		Immovable:    e.Immovable,
		Script:       e.Script,
	})
}
```

**Step 5: Add equipment block to one test zone YAML**

Add to `content/zones/downtown.yaml` (or any zone file), in one room under `spawns`:

```yaml
equipment:
  - item_id: first_aid_kit
    max_count: 1
    respawn_after: "5m"
    immovable: false
    script: ""
```

(Use an item_id that exists in `content/items/`.)

**Step 6: Write failing test**

In `internal/game/world/loader_test.go`, add:

```go
func TestLoader_ParsesRoomEquipment(t *testing.T) {
	// Load a zone that has equipment defined (downtown or any zone with equipment block)
	// Assert the Room.Equipment slice has the expected entry.
	zones, err := LoadZones("../../../content/zones")
	require.NoError(t, err)
	var found *RoomEquipmentConfig
	for _, z := range zones {
		for _, r := range z.Rooms {
			for i := range r.Equipment {
				found = &r.Equipment[i]
				break
			}
			if found != nil {
				break
			}
		}
		if found != nil {
			break
		}
	}
	require.NotNil(t, found, "expected at least one room with equipment defined")
	assert.NotEmpty(t, found.ItemID)
	assert.Greater(t, found.MaxCount, 0)
}
```

**Step 7: Run to confirm failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run TestLoader_ParsesRoomEquipment -v 2>&1 | head -15
```

Expected: compile error or FAIL (field not defined yet).

**Step 8: Implement (already done in steps 2-5), run test**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -v 2>&1 | tail -10
```

Expected: all PASS.

**Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/world/model.go internal/game/world/loader.go internal/game/world/loader_test.go content/zones/ && git commit -m "feat: add RoomEquipmentConfig to world model and YAML loader"
```

---

### Task 2: Create RoomEquipmentManager

**Files:**
- Create: `internal/game/inventory/room_equipment.go`
- Create: `internal/game/inventory/room_equipment_test.go`

**Step 1: Write the test file first**

Create `internal/game/inventory/room_equipment_test.go`:

```go
package inventory_test

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestRoomEquipmentManager_SpawnInitializesInstances(t *testing.T) {
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 2, RespawnAfter: 5 * time.Minute, Immovable: false},
	}
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room1", cfg)

	items := mgr.EquipmentInRoom("room1")
	assert.Len(t, items, 2)
	for _, it := range items {
		assert.Equal(t, "medkit", it.ItemDefID)
		assert.False(t, it.Immovable)
		assert.NotEmpty(t, it.InstanceID)
	}
}

func TestRoomEquipmentManager_ImmovableCannotBePickedUp(t *testing.T) {
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "water_fountain", MaxCount: 1, RespawnAfter: 0, Immovable: true},
	}
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room1", cfg)

	items := mgr.EquipmentInRoom("room1")
	require.Len(t, items, 1)

	ok := mgr.Pickup("room1", items[0].InstanceID)
	assert.False(t, ok, "immovable item should not be pickable")

	after := mgr.EquipmentInRoom("room1")
	assert.Len(t, after, 1, "item should still be present")
}

func TestRoomEquipmentManager_PickupRemovesMovableItem(t *testing.T) {
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 1, RespawnAfter: 5 * time.Minute, Immovable: false},
	}
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room1", cfg)

	items := mgr.EquipmentInRoom("room1")
	require.Len(t, items, 1)

	ok := mgr.Pickup("room1", items[0].InstanceID)
	assert.True(t, ok)
	assert.Empty(t, mgr.EquipmentInRoom("room1"))
}

func TestRoomEquipmentManager_EmptyRoomReturnsEmpty(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	assert.Empty(t, mgr.EquipmentInRoom("nonexistent"))
}

func TestProperty_RoomEquipmentManager_SpawnCountNeverExceedsMax(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxCount := rapid.IntRange(0, 10).Draw(rt, "maxCount")
		cfg := []world.RoomEquipmentConfig{
			{ItemID: "item", MaxCount: maxCount, RespawnAfter: 0, Immovable: false},
		}
		mgr := inventory.NewRoomEquipmentManager()
		mgr.InitRoom("r1", cfg)
		items := mgr.EquipmentInRoom("r1")
		assert.LessOrEqual(t, len(items), maxCount)
	})
}
```

**Step 2: Run to confirm compile failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -run "TestRoomEquipment" -v 2>&1 | head -10
```

Expected: compile error — package not defined.

**Step 3: Create room_equipment.go**

Create `internal/game/inventory/room_equipment.go`:

```go
package inventory

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cory-johannsen/mud/internal/game/world"
)

// EquipmentInstance is a live instance of room equipment.
type EquipmentInstance struct {
	InstanceID string
	ItemDefID  string
	RoomID     string
	Immovable  bool
	Script     string
	configIdx  int
}

// respawnEntry tracks a pending respawn.
type respawnEntry struct {
	roomID    string
	configIdx int
	at        time.Time
	itemDefID string
	immovable bool
	script    string
}

// RoomEquipmentManager manages live equipment instances in rooms.
//
// Precondition: NewRoomEquipmentManager must be called before use.
// Postcondition: All methods are safe for concurrent use.
type RoomEquipmentManager struct {
	mu       sync.RWMutex
	rooms    map[string][]*EquipmentInstance   // roomID -> live instances
	configs  map[string][]world.RoomEquipmentConfig
	respawns []respawnEntry
}

// NewRoomEquipmentManager creates an empty manager.
func NewRoomEquipmentManager() *RoomEquipmentManager {
	return &RoomEquipmentManager{
		rooms:   make(map[string][]*EquipmentInstance),
		configs: make(map[string][]world.RoomEquipmentConfig),
	}
}

// InitRoom spawns initial instances for a room based on its equipment configs.
//
// Precondition: roomID must be non-empty; configs may be nil.
// Postcondition: Room is seeded with min(MaxCount, MaxCount) instances per config.
func (m *RoomEquipmentManager) InitRoom(roomID string, configs []world.RoomEquipmentConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configs[roomID] = configs
	m.rooms[roomID] = nil

	for idx, cfg := range configs {
		for i := 0; i < cfg.MaxCount; i++ {
			m.rooms[roomID] = append(m.rooms[roomID], &EquipmentInstance{
				InstanceID: uuid.New().String(),
				ItemDefID:  cfg.ItemID,
				RoomID:     roomID,
				Immovable:  cfg.Immovable,
				Script:     cfg.Script,
				configIdx:  idx,
			})
		}
	}
}

// EquipmentInRoom returns a snapshot of live equipment instances in a room.
//
// Precondition: roomID may be any string.
// Postcondition: Returns a non-nil slice (may be empty).
func (m *RoomEquipmentManager) EquipmentInRoom(roomID string) []*EquipmentInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := m.rooms[roomID]
	if len(items) == 0 {
		return []*EquipmentInstance{}
	}
	cp := make([]*EquipmentInstance, len(items))
	copy(cp, items)
	return cp
}

// GetInstance returns the instance with the given ID in the given room, or nil.
func (m *RoomEquipmentManager) GetInstance(roomID, instanceID string) *EquipmentInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, it := range m.rooms[roomID] {
		if it.InstanceID == instanceID {
			return it
		}
	}
	return nil
}

// Pickup removes a movable instance from the room and schedules respawn.
// Returns false if instanceID is not found or item is immovable.
//
// Precondition: roomID and instanceID must be non-empty.
// Postcondition: If returned true, instance is removed and respawn scheduled when RespawnAfter > 0.
func (m *RoomEquipmentManager) Pickup(roomID, instanceID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := m.rooms[roomID]
	for i, it := range items {
		if it.InstanceID != instanceID {
			continue
		}
		if it.Immovable {
			return false
		}
		// Remove
		m.rooms[roomID] = append(items[:i], items[i+1:]...)
		// Schedule respawn
		cfg := m.configs[roomID]
		if it.configIdx < len(cfg) && cfg[it.configIdx].RespawnAfter > 0 {
			m.respawns = append(m.respawns, respawnEntry{
				roomID:    roomID,
				configIdx: it.configIdx,
				at:        time.Now().Add(cfg[it.configIdx].RespawnAfter),
				itemDefID: it.ItemDefID,
				immovable: it.Immovable,
				script:    it.Script,
			})
		}
		return true
	}
	return false
}

// ProcessRespawns spawns new instances for any pending respawn entries whose time has come.
// Call this periodically (e.g. every 10s) from a background goroutine.
//
// Precondition: none.
// Postcondition: Expired respawn entries are removed; new instances are added up to MaxCount.
func (m *RoomEquipmentManager) ProcessRespawns() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	remaining := m.respawns[:0]
	for _, r := range m.respawns {
		if r.at.After(now) {
			remaining = append(remaining, r)
			continue
		}
		// Count current live instances for this config slot
		live := 0
		for _, it := range m.rooms[r.roomID] {
			if it.configIdx == r.configIdx {
				live++
			}
		}
		cfg := m.configs[r.roomID]
		if r.configIdx < len(cfg) && live < cfg[r.configIdx].MaxCount {
			m.rooms[r.roomID] = append(m.rooms[r.roomID], &EquipmentInstance{
				InstanceID: uuid.New().String(),
				ItemDefID:  r.itemDefID,
				RoomID:     r.roomID,
				Immovable:  r.immovable,
				Script:     r.script,
				configIdx:  r.configIdx,
			})
		}
	}
	m.respawns = remaining
}

// AddConfig adds a new equipment config to a room at runtime (editor command support).
//
// Precondition: roomID must be non-empty; cfg.ItemID must be non-empty.
// Postcondition: Config is added; initial instances are spawned up to MaxCount.
func (m *RoomEquipmentManager) AddConfig(roomID string, cfg world.RoomEquipmentConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := len(m.configs[roomID])
	m.configs[roomID] = append(m.configs[roomID], cfg)
	for i := 0; i < cfg.MaxCount; i++ {
		m.rooms[roomID] = append(m.rooms[roomID], &EquipmentInstance{
			InstanceID: uuid.New().String(),
			ItemDefID:  cfg.ItemID,
			RoomID:     roomID,
			Immovable:  cfg.Immovable,
			Script:     cfg.Script,
			configIdx:  idx,
		})
	}
}

// RemoveConfig removes all instances of a config by item_id and removes the config.
//
// Precondition: roomID and itemID must be non-empty.
// Postcondition: All instances with matching ItemDefID are removed.
func (m *RoomEquipmentManager) RemoveConfig(roomID, itemID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfgs := m.configs[roomID]
	newCfgs := cfgs[:0]
	found := false
	for _, c := range cfgs {
		if c.ItemID == itemID {
			found = true
		} else {
			newCfgs = append(newCfgs, c)
		}
	}
	if !found {
		return false
	}
	m.configs[roomID] = newCfgs

	items := m.rooms[roomID]
	newItems := items[:0]
	for _, it := range items {
		if it.ItemDefID != itemID {
			newItems = append(newItems, it)
		}
	}
	m.rooms[roomID] = newItems
	return true
}

// ListConfigs returns the equipment configs for a room.
func (m *RoomEquipmentManager) ListConfigs(roomID string) []world.RoomEquipmentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfgs := m.configs[roomID]
	cp := make([]world.RoomEquipmentConfig, len(cfgs))
	copy(cp, cfgs)
	return cp
}
```

**Step 4: Run all tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -v 2>&1 | tail -20
```

Expected: all PASS.

**Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/room_equipment.go internal/game/inventory/room_equipment_test.go && git commit -m "feat: add RoomEquipmentManager with respawn scheduler"
```

---

### Task 3: Add proto types and regenerate

**Files:**
- Modify: `api/proto/game/v1/game.proto`

**Step 1: Read game.proto**

Read `api/proto/game/v1/game.proto` lines 1-60 (ClientMessage oneof) and lines 124-145 (RoomView). Identify:
- Last `ClientMessage` oneof field: `archetype_selection = 34`
- Last `RoomView` field: `period = 10`

**Step 2: Add RoomEquipmentItem message**

After the `FloorItem` message (around line 347), add:

```proto
// RoomEquipmentItem describes a permanent or respawning item in a room.
message RoomEquipmentItem {
    string instance_id = 1;
    string name        = 2;
    int32  quantity    = 3;
    bool   immovable   = 4;
    bool   usable      = 5;
}
```

**Step 3: Add equipment field to RoomView**

In `RoomView`, add after `period = 10`:

```proto
repeated RoomEquipmentItem equipment = 11;
```

**Step 4: Add UseEquipmentRequest and RoomEquipRequest messages**

Add near other request messages:

```proto
// UseEquipmentRequest asks the server to use a room equipment item.
message UseEquipmentRequest {
    string instance_id = 1;
}

// RoomEquipRequest asks the server to manage room equipment (editor command).
message RoomEquipRequest {
    string sub_command = 1; // "add", "remove", "list", "modify"
    string item_id     = 2;
    int32  max_count   = 3;
    string respawn     = 4; // duration string e.g. "5m"
    bool   immovable   = 5;
    string script      = 6;
}
```

**Step 5: Add to ClientMessage oneof**

In the `ClientMessage` oneof block, add after field 34:

```proto
UseEquipmentRequest use_equipment = 35;
RoomEquipRequest    room_equip    = 36;
```

**Step 6: Regenerate**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: exits 0.

**Step 7: Build (check for breakage)**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1 | head -20
```

Expected: exits 0 (new fields are additive; no existing code broke).

**Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add api/proto/game/v1/game.proto internal/gameserver/gamev1/ && git commit -m "feat: add RoomEquipmentItem proto and UseEquipment/RoomEquip request messages"
```

---

### Task 4: Wire RoomEquipmentManager; update handleLook

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_test.go` (or look test file)

**Step 1: Read grpc_service.go**

Read `internal/gameserver/grpc_service.go` lines 1-80 (struct fields, constructor) and the `handleLook` function. Note where `floorMgr` is declared and used — `roomEquipMgr` follows the same pattern.

**Step 2: Add roomEquipMgr field**

In the `GameServiceServer` struct, add after `floorMgr`:

```go
roomEquipMgr *inventory.RoomEquipmentManager
```

**Step 3: Initialize in constructor / wire-up**

Read `cmd/gameserver/main.go` to find where `GameServiceServer` is constructed. Add:

```go
roomEquipMgr := inventory.NewRoomEquipmentManager()
// Seed from world zones
for _, zone := range zones {
    for _, room := range zone.Rooms {
        if len(room.Equipment) > 0 {
            roomEquipMgr.InitRoom(room.ID, room.Equipment)
        }
    }
}
```

Pass `roomEquipMgr` to the `GameServiceServer` constructor (or use a setter — match existing pattern).

Also start the respawn goroutine in main.go:

```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        roomEquipMgr.ProcessRespawns()
    }
}()
```

**Step 4: Update handleLook**

In `handleLook`, after the existing `floorItems` loop, add:

```go
if s.roomEquipMgr != nil {
    for _, eq := range s.roomEquipMgr.EquipmentInRoom(sess.RoomID) {
        name := eq.ItemDefID
        if s.invRegistry != nil {
            if def, ok := s.invRegistry.Item(eq.ItemDefID); ok {
                name = def.Name
            }
        }
        view.Equipment = append(view.Equipment, &gamev1.RoomEquipmentItem{
            InstanceId: eq.InstanceID,
            Name:       name,
            Quantity:   1,
            Immovable:  eq.Immovable,
            Usable:     eq.Script != "",
        })
    }
}
```

**Step 5: Update text_renderer.go to render equipment**

Read `internal/frontend/handlers/text_renderer.go` find `RenderRoomView` (or equivalent). Add equipment rendering after floor items:

```go
for _, eq := range rv.Equipment {
    flags := ""
    if eq.Immovable {
        flags += " [fixed]"
    }
    if eq.Usable {
        flags += " [usable]"
    }
    sb.WriteString(fmt.Sprintf("  %s%s%s%s\r\n", telnet.Cyan, eq.Name, telnet.Reset, flags))
}
```

**Step 6: Build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```

Expected: exits 0.

**Step 7: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test $(mise exec -- go list ./... | grep -v postgres) 2>&1 | grep -E "FAIL|ok"
```

Expected: all ok.

**Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/frontend/handlers/text_renderer.go cmd/gameserver/main.go && git commit -m "feat: wire RoomEquipmentManager; render equipment in handleLook"
```

---

### Task 5: Add use command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/use_equipment.go`
- Create: `internal/game/command/use_equipment_test.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_use_equipment_test.go`

**Step 1: CMD-1 — Add constant**

In `internal/game/command/commands.go`, add:

```go
HandlerUseEquipment = "use_equipment"
```

**Step 2: CMD-2 — Add command entry**

In `BuiltinCommands()`, add:

```go
{Handler: HandlerUseEquipment, Name: "use", Description: "Use an item in the room"},
```

**Step 3: CMD-3 — Create use_equipment_test.go and use_equipment.go (TDD)**

Create `internal/game/command/use_equipment_test.go`:

```go
package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleUseEquipment_ReturnsNonEmpty(t *testing.T) {
	result := command.HandleUseEquipment("instance-123")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "instance-123")
}

func TestHandleUseEquipment_EmptyID(t *testing.T) {
	result := command.HandleUseEquipment("")
	assert.NotEmpty(t, result)
}

func TestProperty_HandleUseEquipment_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.String().Draw(rt, "id")
		_ = command.HandleUseEquipment(id)
	})
}
```

Run to confirm failure:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run "TestHandleUseEquipment|TestProperty_HandleUseEquipment" -v 2>&1 | head -10
```

Create `internal/game/command/use_equipment.go`:

```go
package command

import "fmt"

// HandleUseEquipment returns a plain-text acknowledgment for the use command.
//
// Precondition: instanceID may be any string.
// Postcondition: Returns a non-empty human-readable string.
func HandleUseEquipment(instanceID string) string {
	if instanceID == "" {
		return "Use what? Specify an item instance."
	}
	return fmt.Sprintf("You use %s.", instanceID)
}
```

Run tests:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run "TestHandleUseEquipment|TestProperty_HandleUseEquipment" -v 2>&1
```

Expected: all PASS.

**Step 4: CMD-5 — Add bridgeUseEquipment**

In `internal/frontend/handlers/bridge_handlers.go`, add:

```go
func bridgeUseEquipment(msg *gamev1.ClientMessage, instanceID string) {
	msg.Payload = &gamev1.ClientMessage_UseEquipment{
		UseEquipment: &gamev1.UseEquipmentRequest{InstanceId: instanceID},
	}
}
```

Register in `bridgeHandlerMap`:

```go
command.HandlerUseEquipment: func(msg *gamev1.ClientMessage, args string) {
	bridgeUseEquipment(msg, strings.TrimSpace(args))
},
```

Run `TestAllCommandHandlersAreWired`:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1
```

Expected: PASS.

**Step 5: CMD-6 — Add handleUseEquipment**

Read `internal/scripting/manager.go` to understand `CallHook(zoneID, hook string, args ...lua.LValue)`.

In `internal/gameserver/grpc_service.go`, add:

```go
func (s *GameServiceServer) handleUseEquipment(uid, instanceID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.roomEquipMgr == nil {
		return textEvent("No equipment manager available."), nil
	}
	inst := s.roomEquipMgr.GetInstance(sess.RoomID, instanceID)
	if inst == nil {
		return textEvent("That item is not here."), nil
	}
	if inst.Script == "" {
		return textEvent("You examine the item but nothing happens."), nil
	}
	// Invoke Lua script: hook name is the item_def_id, zoneID from player's zone.
	zoneID := s.worldH.RoomZone(sess.RoomID)
	result, err := s.scriptMgr.CallHook(zoneID, inst.ItemDefID, lua.LString(uid))
	if err != nil {
		return textEvent(fmt.Sprintf("The item malfunctions: %v", err)), nil
	}
	msg := "You use the item."
	if result != lua.LNil {
		msg = result.String()
	}
	return textEvent(msg), nil
}
```

Wire in dispatch:
```go
case *gamev1.ClientMessage_UseEquipment:
	return s.handleUseEquipment(uid, p.UseEquipment.InstanceId)
```

**Step 6: CMD-6 tests**

Create `internal/gameserver/grpc_service_use_equipment_test.go` with tests for:
- Unknown session returns error event
- Known session, no equipment manager → "No equipment manager" message
- Known session, invalid instance → "not here" message
- Known session, item with no script → "nothing happens" message

Run:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleUseEquipment" -v 2>&1
```

Expected: all PASS.

**Step 7: Build and run full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
cd /home/cjohannsen/src/mud && mise exec -- go test $(mise exec -- go list ./... | grep -v postgres) 2>&1 | grep -E "FAIL|ok"
```

**Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/command/commands.go internal/game/command/use_equipment.go internal/game/command/use_equipment_test.go internal/frontend/handlers/bridge_handlers.go internal/gameserver/grpc_service.go internal/gameserver/grpc_service_use_equipment_test.go && git commit -m "feat: add use command for room equipment items"
```

---

### Task 6: Add roomequip editor command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/roomequip.go`
- Create: `internal/game/command/roomequip_test.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_roomequip_test.go`

**Step 1: CMD-1 — Add constant**

```go
HandlerRoomEquip = "room_equip"
```

**Step 2: CMD-2 — Add command entry**

```go
{Handler: HandlerRoomEquip, Name: "roomequip", Description: "Manage room equipment (editor)"},
```

**Step 3: CMD-3 — TDD for HandleRoomEquip**

Create `internal/game/command/roomequip_test.go`:

```go
package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleRoomEquip_ReturnsNonEmpty(t *testing.T) {
	result := command.HandleRoomEquip("add item1 1 5m false")
	assert.NotEmpty(t, result)
}

func TestHandleRoomEquip_UnknownSubcommand(t *testing.T) {
	result := command.HandleRoomEquip("bogus")
	assert.Contains(t, result, "Usage")
}

func TestHandleRoomEquip_EmptyArgs(t *testing.T) {
	result := command.HandleRoomEquip("")
	assert.Contains(t, result, "Usage")
}

func TestProperty_HandleRoomEquip_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.String().Draw(rt, "args")
		_ = command.HandleRoomEquip(args)
	})
}
```

Create `internal/game/command/roomequip.go`:

```go
package command

// HandleRoomEquip returns a plain-text acknowledgment for the roomequip command.
// The actual CRUD logic executes server-side; this function returns the client-side acknowledgment.
//
// Precondition: args may be any string.
// Postcondition: Returns a non-empty human-readable string.
func HandleRoomEquip(args string) string {
	if args == "" {
		return roomEquipUsage()
	}
	switch firstWord(args) {
	case "add", "remove", "list", "modify":
		return "roomequip " + args
	default:
		return roomEquipUsage()
	}
}

func roomEquipUsage() string {
	return "Usage: roomequip <add|remove|list|modify> [item_id] [max_count] [respawn] [immovable] [script]"
}

func firstWord(s string) string {
	for i, c := range s {
		if c == ' ' {
			return s[:i]
		}
	}
	return s
}
```

Run tests:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run "TestHandleRoomEquip|TestProperty_HandleRoomEquip" -v 2>&1
```

Expected: all PASS.

**Step 4: CMD-5 — bridgeRoomEquip**

In `bridge_handlers.go`, parse sub-command and args, populate `RoomEquipRequest`:

```go
func bridgeRoomEquip(msg *gamev1.ClientMessage, args string) {
	parts := strings.Fields(args)
	req := &gamev1.RoomEquipRequest{}
	if len(parts) > 0 {
		req.SubCommand = parts[0]
	}
	if len(parts) > 1 {
		req.ItemId = parts[1]
	}
	if len(parts) > 2 {
		if n, err := strconv.Atoi(parts[2]); err == nil {
			req.MaxCount = int32(n)
		}
	}
	if len(parts) > 3 {
		req.Respawn = parts[3]
	}
	if len(parts) > 4 {
		req.Immovable = parts[4] == "true"
	}
	if len(parts) > 5 {
		req.Script = parts[5]
	}
	msg.Payload = &gamev1.ClientMessage_RoomEquip{RoomEquip: req}
}
```

Register in `bridgeHandlerMap`:
```go
command.HandlerRoomEquip: func(msg *gamev1.ClientMessage, args string) {
	bridgeRoomEquip(msg, args)
},
```

Run `TestAllCommandHandlersAreWired`:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1
```

Expected: PASS.

**Step 5: CMD-6 — handleRoomEquip**

In `grpc_service.go`, add:

```go
func (s *GameServiceServer) handleRoomEquip(uid string, req *gamev1.RoomEquipRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.roomEquipMgr == nil {
		return textEvent("Room equipment manager not available."), nil
	}
	roomID := sess.RoomID
	switch req.SubCommand {
	case "list":
		cfgs := s.roomEquipMgr.ListConfigs(roomID)
		if len(cfgs) == 0 {
			return textEvent("No equipment configured for this room."), nil
		}
		var sb strings.Builder
		for _, c := range cfgs {
			sb.WriteString(fmt.Sprintf("  %s (max:%d respawn:%s immovable:%v)\r\n",
				c.ItemID, c.MaxCount, c.RespawnAfter, c.Immovable))
		}
		return textEvent(sb.String()), nil
	case "add":
		if req.ItemId == "" {
			return textEvent("Usage: roomequip add <item_id> [max_count] [respawn] [immovable] [script]"), nil
		}
		dur, _ := time.ParseDuration(req.Respawn)
		cfg := world.RoomEquipmentConfig{
			ItemID:       req.ItemId,
			MaxCount:     max(int(req.MaxCount), 1),
			RespawnAfter: dur,
			Immovable:    req.Immovable,
			Script:       req.Script,
		}
		s.roomEquipMgr.AddConfig(roomID, cfg)
		return textEvent(fmt.Sprintf("Added %s to room equipment.", req.ItemId)), nil
	case "remove":
		if req.ItemId == "" {
			return textEvent("Usage: roomequip remove <item_id>"), nil
		}
		if !s.roomEquipMgr.RemoveConfig(roomID, req.ItemId) {
			return textEvent(fmt.Sprintf("Item %q not found in room equipment.", req.ItemId)), nil
		}
		return textEvent(fmt.Sprintf("Removed %s from room equipment.", req.ItemId)), nil
	case "modify":
		// Remove then re-add
		s.roomEquipMgr.RemoveConfig(roomID, req.ItemId)
		dur, _ := time.ParseDuration(req.Respawn)
		cfg := world.RoomEquipmentConfig{
			ItemID:       req.ItemId,
			MaxCount:     max(int(req.MaxCount), 1),
			RespawnAfter: dur,
			Immovable:    req.Immovable,
			Script:       req.Script,
		}
		s.roomEquipMgr.AddConfig(roomID, cfg)
		return textEvent(fmt.Sprintf("Modified %s in room equipment.", req.ItemId)), nil
	default:
		return textEvent("Usage: roomequip <add|remove|list|modify>"), nil
	}
}
```

Wire in dispatch:
```go
case *gamev1.ClientMessage_RoomEquip:
	return s.handleRoomEquip(uid, p.RoomEquip)
```

**Step 6: Tests for handleRoomEquip**

Create `internal/gameserver/grpc_service_roomequip_test.go` with tests:
- Unknown session → error
- list on empty room → "No equipment configured"
- add → "Added"
- remove → "Removed" / item not found
- modify → "Modified"

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleRoomEquip" -v 2>&1
```

Expected: all PASS.

**Step 7: Build + full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
cd /home/cjohannsen/src/mud && mise exec -- go test $(mise exec -- go list ./... | grep -v postgres) 2>&1 | grep -E "FAIL|ok"
```

**Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/command/commands.go internal/game/command/roomequip.go internal/game/command/roomequip_test.go internal/frontend/handlers/bridge_handlers.go internal/gameserver/grpc_service.go internal/gameserver/grpc_service_roomequip_test.go && git commit -m "feat: add roomequip editor command for room equipment CRUD"
```

---

### Task 7: Final verification

**Step 1: Full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test $(mise exec -- go list ./... | grep -v postgres) 2>&1
```

Expected: all `ok`.

**Step 2: Build all binaries**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./...
```

Expected: exits 0.

**Step 3: Verify TestAllCommandHandlersAreWired**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1
```

Expected: PASS.
