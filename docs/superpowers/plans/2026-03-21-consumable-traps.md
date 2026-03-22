# Consumable Traps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add player-deployable trap items that arm at the player's combat position and fire when combatants move within trigger range, using the existing TrapTemplate/TrapManager/fireTrap infrastructure.

**Architecture:** Extend the existing `internal/game/trap/` types with positional fields, add a `"trap"` item kind to inventory, wire a new `onCombatantMoved` callback from `CombatHandler` into a `checkConsumableTraps` service method, and add a `deploy` command that removes the item from the backpack and arms it via `TrapManager.AddConsumableTrap`.

**Tech Stack:** Go, `pgregory.net/rapid` (property-based testing), protobuf, YAML content files.

---

## File Map

| File | Role |
|---|---|
| `internal/game/trap/template.go` | Add `TriggerRangeFt`, `BlastRadiusFt` fields + `EffectiveTriggerRange` helper |
| `internal/game/trap/manager.go` | Add `DeployPosition`, `IsConsumable` to `TrapInstanceState`; add `TrapKindConsumable` const; add `AddConsumableTrap` method |
| `internal/game/inventory/item.go` | Add `KindTrap` const, `TrapTemplateRef` field to `ItemDef`, validation |
| `content/traps/*.yaml` | Add `trigger_range_ft` + `blast_radius_ft` to all 6 trap YAMLs |
| `content/items/deployable_traps.yaml` | New: 6 deployable trap item definitions |
| `internal/gameserver/combat_handler.go` | Add `onCombatantMoved` callback field, `SetOnCombatantMoved` setter, `CombatantPosition(roomID, uid string) int` method, `CombatantsInRoom(roomID string) []*combat.Combatant` method; fire callback after Stride/Step/Shove |
| `internal/gameserver/grpc_service_trap.go` | Add `WireConsumableTrapTrigger()`, `checkConsumableTraps()`, `fireConsumableTrapOnCombatant()` |
| `internal/gameserver/grpc_service_deploy_trap.go` | New: `handleDeployTrap` handler |
| `internal/gameserver/grpc_service_deploy_trap_test.go` | New: deploy command tests (internal package) |
| `api/proto/game/v1/game.proto` | Add `DeployTrapRequest` message + field 85 in `ClientMessage` |
| `internal/game/command/commands.go` | Add `HandlerDeployTrap` const + `deploy_trap` command entry |
| `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeDeployTrap` function + map entry |
| `internal/gameserver/grpc_service.go` | Wire `deploy_trap` dispatch case + call `WireConsumableTrapTrigger()` |
| `docs/features/consumable-traps.md` | Mark all requirements complete |
| `docs/features/index.yaml` | Set `status: complete` for `consumable-traps` entry |

---

## NewGameServiceServer parameter reference

The function takes exactly 46 parameters. When writing test calls, copy this nil pattern (used in `makeTrapSvc` in `grpc_service_trap_internal_test.go`):

```go
NewGameServiceServer(
    worldMgr, sessMgr,                          // 1-2
    nil,                                         // 3: cmdRegistry
    NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil), // 4
    NewChatHandler(sessMgr),                     // 5
    zaptest.NewLogger(t),                        // 6
    nil, nil, nil, nil, nil, nil,                // 7-12: charSaver, diceRoller, npcHandler, npcMgr, combatHandler, scriptMgr
    nil, nil, nil, nil, nil, nil,                // 13-18: respawnMgr, floorMgr, roomEquipMgr, automapRepo, invRegistry, accountAdmin
    nil, nil, nil, nil, nil, nil, nil, nil, "",  // 19-27: calendar, jobRegistry, condRegistry, techRegistry, hardwiredTechRepo, preparedTechRepo, spontaneousTechRepo, innateTechRepo, loadoutsDir
    nil, nil, nil,                               // 28-30: allSkills, characterSkillsRepo, characterProficienciesRepo
    nil, nil, nil,                               // 31-33: allFeats, featRegistry, characterFeatsRepo
    nil, nil, nil, nil, nil, nil, nil,           // 34-40: allClassFeatures, classFeatureRegistry, characterClassFeaturesRepo, featureChoicesRepo, charAbilityBoostsRepo, archetypes, regions
    nil, nil,                                    // 41-42: mentalStateMgr, actionH
    nil,                                         // 43: spontaneousUsePoolRepo
    nil,                                         // 44: wantedRepo
    trapMgr, tmplMap,                            // 45-46: trapMgr, trapTemplates
)
```

When a test needs a non-nil `combatHandler` (param 11), replace the `nil` at position 11 with the handler variable. When needing `invRegistry` (param 17), replace nil at position 17.

---

### Task 1: TrapTemplate + TrapManager data model

**Files:**
- Modify: `internal/game/trap/template.go`
- Modify: `internal/game/trap/manager.go`
- Test: `internal/game/trap/manager_test.go` (check `ls internal/game/trap/` first; append to existing test file or create)

- [ ] **Step 1: Write failing tests**

```go
// Append to the existing test file in internal/game/trap/
// (check ls internal/game/trap/ for the test file name, e.g., manager_test.go or trap_test.go)
package trap_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/trap"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"
)

func TestTrapTemplate_NewFields_ZeroValue(t *testing.T) {
    tmpl := &trap.TrapTemplate{}
    assert.Equal(t, 0, tmpl.TriggerRangeFt)
    assert.Equal(t, 0, tmpl.BlastRadiusFt)
}

func TestEffectiveTriggerRange_ZeroIsDefault(t *testing.T) {
    tmpl := &trap.TrapTemplate{TriggerRangeFt: 0}
    assert.Equal(t, 5, trap.EffectiveTriggerRange(tmpl))
}

func TestEffectiveTriggerRange_NonZeroPassthrough(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        n := rapid.IntRange(1, 100).Draw(t, "range")
        tmpl := &trap.TrapTemplate{TriggerRangeFt: n}
        assert.Equal(t, n, trap.EffectiveTriggerRange(tmpl))
    })
}

func TestTrapKindConsumable_Constant(t *testing.T) {
    assert.Equal(t, "consumable", trap.TrapKindConsumable)
}

func TestAddConsumableTrap_ArmedWithPosition(t *testing.T) {
    mgr := trap.NewTrapManager()
    tmpl := &trap.TrapTemplate{ID: "mine", Name: "Mine"}
    err := mgr.AddConsumableTrap("zone/room/consumable/1", tmpl, 15)
    require.NoError(t, err)
    inst, ok := mgr.GetTrap("zone/room/consumable/1")
    require.True(t, ok)
    assert.True(t, inst.Armed)
    assert.True(t, inst.IsConsumable)
    assert.Equal(t, 15, inst.DeployPosition)
}

func TestAddConsumableTrap_DuplicateReturnsError(t *testing.T) {
    mgr := trap.NewTrapManager()
    tmpl := &trap.TrapTemplate{ID: "mine"}
    require.NoError(t, mgr.AddConsumableTrap("id1", tmpl, 0))
    err := mgr.AddConsumableTrap("id1", tmpl, 0)
    assert.Error(t, err)
}

func TestProperty_AddConsumableTrap_AlwaysArmedIsConsumable(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        pos := rapid.Int().Draw(t, "pos")
        mgr := trap.NewTrapManager()
        tmpl := &trap.TrapTemplate{ID: "t"}
        id := "zone/room/consumable/" + rapid.StringN(1, 20, -1).Draw(t, "id")
        err := mgr.AddConsumableTrap(id, tmpl, pos)
        require.NoError(t, err)
        inst, ok := mgr.GetTrap(id)
        require.True(t, ok)
        assert.True(t, inst.Armed)
        assert.True(t, inst.IsConsumable)
        assert.Equal(t, pos, inst.DeployPosition)
    })
}

// REQ-CTR-5: A deployed trap must be position-anchored — DeployPosition must not change after creation.
func TestAddConsumableTrap_PositionAnchored(t *testing.T) {
    mgr := trap.NewTrapManager()
    tmpl := &trap.TrapTemplate{ID: "mine"}
    require.NoError(t, mgr.AddConsumableTrap("zone/room/consumable/anc-1", tmpl, 20))
    inst, ok := mgr.GetTrap("zone/room/consumable/anc-1")
    require.True(t, ok)
    assert.Equal(t, 20, inst.DeployPosition, "DeployPosition must be the value set at creation")
    // Callers must not mutate DeployPosition — the struct is returned by value, so mutating
    // the returned copy must not affect the stored state.
    inst.DeployPosition = 999
    inst2, ok2 := mgr.GetTrap("zone/room/consumable/anc-1")
    require.True(t, ok2)
    assert.Equal(t, 20, inst2.DeployPosition, "stored DeployPosition must be immutable after creation")
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/trap/... 2>&1 | tail -20
```
Expected: compile errors — `TriggerRangeFt`, `BlastRadiusFt`, `EffectiveTriggerRange`, `TrapKindConsumable`, `AddConsumableTrap`, `IsConsumable`, `DeployPosition` undefined.

- [ ] **Step 3: Add fields to TrapTemplate**

In `internal/game/trap/template.go`, add to the `TrapTemplate` struct after the `DangerScaling` field:
```go
TriggerRangeFt int `yaml:"trigger_range_ft"`
BlastRadiusFt  int `yaml:"blast_radius_ft"`
```

Add exported helper at the bottom of `template.go`:
```go
// EffectiveTriggerRange returns TriggerRangeFt, or 5 if zero (the default trigger range in feet).
func EffectiveTriggerRange(tmpl *TrapTemplate) int {
    if tmpl.TriggerRangeFt == 0 {
        return 5
    }
    return tmpl.TriggerRangeFt
}
```

- [ ] **Step 4: Add constant + fields + method to TrapManager**

In `internal/game/trap/manager.go`:

Add constant (near the top, after package/imports):
```go
const TrapKindConsumable = "consumable"
```

Add fields to `TrapInstanceState` struct:
```go
DeployPosition int  // combat position (feet) at deploy time; 0 for world traps
IsConsumable   bool // true for player-deployed traps; enforces one-shot semantics
```

Add `AddConsumableTrap` method. NOTE: the instances map is called `traps` (not `instances`) — read the struct definition to verify before writing:
```go
// AddConsumableTrap arms a player-deployed consumable trap at the given deploy position.
// Precondition: instanceID is unique within this TrapManager.
// Postcondition: GetTrap(instanceID) returns a state with Armed=true, IsConsumable=true, DeployPosition=deployPos.
func (m *TrapManager) AddConsumableTrap(instanceID string, tmpl *TrapTemplate, deployPos int) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if _, exists := m.traps[instanceID]; exists {
        return fmt.Errorf("trap instance %q already exists", instanceID)
    }
    m.traps[instanceID] = &TrapInstanceState{
        TemplateID:     tmpl.ID,
        Armed:          true,
        DeployPosition: deployPos,
        IsConsumable:   true,
    }
    return nil
}
```

Make sure `"fmt"` is imported (add to imports if not already present).

- [ ] **Step 5: Run tests — verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/trap/... 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/trap/
git commit -m "feat(consumable-traps): Task 1 — TrapTemplate positional fields + AddConsumableTrap"
```

---

### Task 2: Inventory — KindTrap and TrapTemplateRef

**Files:**
- Modify: `internal/game/inventory/item.go`
- Test: `internal/game/inventory/item_test.go` (create if absent; check with `ls internal/game/inventory/`)

- [ ] **Step 1: Write failing tests**

```go
// internal/game/inventory/item_test.go (create or append)
package inventory_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestKindTrap_Constant(t *testing.T) {
    assert.Equal(t, "trap", inventory.KindTrap)
}

func TestItemDef_TrapTemplateRef_Field(t *testing.T) {
    item := &inventory.ItemDef{
        ID:              "deployable_mine",
        Name:            "Deployable Mine",
        Kind:            inventory.KindTrap,
        TrapTemplateRef: "mine",
        Weight:          2.0,
        Stackable:       true,
        MaxStack:        5,
        Value:           300,
    }
    assert.Equal(t, "mine", item.TrapTemplateRef)
}

func TestRegistry_RegisterItem_TrapKind_RequiresTrapTemplateRef(t *testing.T) {
    reg := inventory.NewRegistry()
    err := reg.RegisterItem(&inventory.ItemDef{
        ID:   "bad_trap",
        Name: "Bad Trap",
        Kind: inventory.KindTrap,
        // TrapTemplateRef intentionally missing — must fail
    })
    assert.Error(t, err, "empty TrapTemplateRef must be rejected for kind=trap")
}

func TestRegistry_RegisterItem_TrapKind_ValidRef(t *testing.T) {
    reg := inventory.NewRegistry()
    err := reg.RegisterItem(&inventory.ItemDef{
        ID:              "good_trap",
        Name:            "Good Trap",
        Kind:            inventory.KindTrap,
        TrapTemplateRef: "mine",
        Weight:          1.0,
        Stackable:       true,
        MaxStack:        5,
        Value:           100,
    })
    require.NoError(t, err)
    item, ok := reg.Item("good_trap")
    require.True(t, ok)
    assert.Equal(t, "mine", item.TrapTemplateRef)
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... 2>&1 | tail -20
```
Expected: compile errors — `KindTrap` undefined, `TrapTemplateRef` field not found.

- [ ] **Step 3: Implement**

In `internal/game/inventory/item.go`:

Add constant alongside the existing `KindWeapon`, `KindExplosive`, etc.:
```go
const KindTrap = "trap"
```

Add field to `ItemDef` struct (after `ExplosiveRef`):
```go
TrapTemplateRef string `yaml:"trap_template_ref"`
```

Find where kind validation happens in `RegisterItem` or `Validate`. Add:
```go
case KindTrap:
    if d.TrapTemplateRef == "" {
        return fmt.Errorf("item %q: TrapTemplateRef is required when Kind is %q", d.ID, KindTrap)
    }
```

Also add `KindTrap` to the set of valid kinds so the existing kind validation accepts it. Look for a switch, map, or list of valid kinds and add `KindTrap` to it.

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/inventory/
git commit -m "feat(consumable-traps): Task 2 — KindTrap + TrapTemplateRef on ItemDef"
```

---

### Task 3: Content YAML files

**Files:**
- Modify: `content/traps/mine.yaml`, `content/traps/pit.yaml`, `content/traps/bear_trap.yaml`, `content/traps/trip_wire.yaml`, `content/traps/honkeypot_charmer.yaml`, `content/traps/pressure_plate_mine.yaml`
- Create: `content/items/deployable_traps.yaml`

- [ ] **Step 1: Add fields to all 6 trap YAMLs**

Read each file, then add these two lines:

| File | `trigger_range_ft` | `blast_radius_ft` |
|---|---|---|
| `mine.yaml` | 5 | 10 |
| `pit.yaml` | 5 | 0 |
| `bear_trap.yaml` | 5 | 0 |
| `trip_wire.yaml` | 5 | 5 |
| `honkeypot_charmer.yaml` | 5 | 0 |
| `pressure_plate_mine.yaml` | 5 | 10 |

- [ ] **Step 2: Create `content/items/deployable_traps.yaml`**

```yaml
- id: deployable_mine
  name: Deployable Mine
  kind: trap
  trap_template_ref: mine
  weight: 2.0
  stackable: true
  max_stack: 5
  value: 300

- id: deployable_pit_trap
  name: Pit Trap Kit
  kind: trap
  trap_template_ref: pit
  weight: 3.0
  stackable: true
  max_stack: 3
  value: 150

- id: deployable_bear_trap
  name: Bear Trap
  kind: trap
  trap_template_ref: bear_trap
  weight: 4.0
  stackable: true
  max_stack: 3
  value: 120

- id: deployable_trip_wire
  name: Trip Wire
  kind: trap
  trap_template_ref: trip_wire
  weight: 0.5
  stackable: true
  max_stack: 10
  value: 80

- id: deployable_honkeypot
  name: Honkeypot Device
  kind: trap
  trap_template_ref: honkeypot_charmer
  weight: 1.0
  stackable: true
  max_stack: 5
  value: 250

- id: deployable_pressure_plate_mine
  name: Pressure Plate Mine
  kind: trap
  trap_template_ref: pressure_plate_mine
  weight: 2.5
  stackable: true
  max_stack: 3
  value: 350
```

- [ ] **Step 3: Verify tests still pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/trap/... ./internal/game/inventory/... 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/traps/ content/items/deployable_traps.yaml
git commit -m "feat(consumable-traps): Task 3 — trap YAML positional fields + deployable item definitions"
```

---

### Task 4: CombatHandler — onCombatantMoved callback + position methods

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: create or append to `internal/gameserver/combat_handler_test.go` (package `gameserver`)

**Context:** Read `internal/gameserver/combat_handler.go` fully before implementing. Key patterns to follow:
- `onCoverHit` is an existing callback field; `SetOnCoverHit` is its setter — mirror this pattern exactly
- `resolveAndAdvanceLocked` (or equivalent) is where actions resolve — find the `ActionStride`, `ActionStep`, `ActionShove` cases and call the callback after the position update
- The engine field is accessed as `h.engine` — verify in the struct definition
- `h.engine.GetCombat(roomID)` is the engine lookup — verify exact method name

- [ ] **Step 1: Write failing tests**

```go
// internal/gameserver/combat_handler_test.go
// package gameserver (internal)
package gameserver

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/dice"
    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap/zaptest"
)

func TestSetOnCombatantMoved_CallbackFiresOnStride(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    _, sessMgr := testWorldAndSession(t)
    npcMgr := npc.NewManager()
    h := NewCombatHandler(
        combat.NewEngine(), npcMgr, sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )

    var gotRoomID, gotUID string
    h.SetOnCombatantMoved(func(roomID, uid string) {
        gotRoomID = roomID
        gotUID = uid
    })

    _, err := npcMgr.Spawn(&npc.Template{
        ID: "guard-t4", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
    }, "room_a")
    require.NoError(t, err)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "mover", Username: "mover", CharName: "mover", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    _, err = h.Attack("mover", "Guard")
    require.NoError(t, err)
    h.cancelTimer("room_a")

    err = h.Stride("mover", "away")
    require.NoError(t, err)

    assert.Equal(t, "room_a", gotRoomID)
    assert.Equal(t, "mover", gotUID)
}

func TestCombatantPosition_PlayerStartsAtZero(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    _, sessMgr := testWorldAndSession(t)
    npcMgr := npc.NewManager()
    h := NewCombatHandler(
        combat.NewEngine(), npcMgr, sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )

    _, err := npcMgr.Spawn(&npc.Template{
        ID: "guard-pos", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
    }, "room_a")
    require.NoError(t, err)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "posplayer", Username: "posplayer", CharName: "posplayer", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    _, err = h.Attack("posplayer", "Guard")
    require.NoError(t, err)
    h.cancelTimer("room_a")

    pos := h.CombatantPosition("room_a", "posplayer")
    assert.Equal(t, 0, pos, "player starts at position 0")
}

func TestCombatantPosition_NotInCombat_ReturnsZero(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    _, sessMgr := testWorldAndSession(t)
    h := NewCombatHandler(
        combat.NewEngine(), npc.NewManager(), sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )
    assert.Equal(t, 0, h.CombatantPosition("no_room", "nobody"))
}

func TestCombatantsInRoom_ReturnsAllCombatants(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    _, sessMgr := testWorldAndSession(t)
    npcMgr := npc.NewManager()
    h := NewCombatHandler(
        combat.NewEngine(), npcMgr, sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )

    _, err := npcMgr.Spawn(&npc.Template{
        ID: "guard-cir", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
    }, "room_a")
    require.NoError(t, err)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "cirplayer", Username: "cirplayer", CharName: "cirplayer", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    _, err = h.Attack("cirplayer", "Guard")
    require.NoError(t, err)
    h.cancelTimer("room_a")

    combatants := h.CombatantsInRoom("room_a")
    assert.GreaterOrEqual(t, len(combatants), 2, "must include player and NPC")
}

func TestCombatantsInRoom_NoActiveCombat_ReturnsNil(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    _, sessMgr := testWorldAndSession(t)
    h := NewCombatHandler(
        combat.NewEngine(), npc.NewManager(), sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )
    combatants := h.CombatantsInRoom("empty_room")
    assert.Nil(t, combatants)
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestSetOnCombatantMoved|TestCombatantPosition|TestCombatantsInRoom" 2>&1 | tail -20
```
Expected: compile errors.

- [ ] **Step 3: Add callback field + setter to CombatHandler**

In `internal/gameserver/combat_handler.go`, add to the struct (near `onCoverHit`):
```go
onCombatantMoved func(roomID, movedCombatantID string)
```

Add setter method (near `SetOnCoverHit`):
```go
// SetOnCombatantMoved registers a callback that fires after any Stride, Step, or Shove resolves.
func (h *CombatHandler) SetOnCombatantMoved(fn func(roomID, movedCombatantID string)) {
    h.onCombatantMoved = fn
}
```

- [ ] **Step 4: Add CombatantPosition and CombatantsInRoom methods**

Read `combat_handler.go` to confirm: the engine field name (likely `h.engine`), the `GetCombat` method name on the engine, and how the `Combat` struct's combatants are accessed.

Add:
```go
// CombatantPosition returns the current combat position (feet) of the given combatant in the given room.
// Returns 0 if no combat is active for roomID or the combatant is not found.
func (h *CombatHandler) CombatantPosition(roomID, uid string) int {
    h.mu.Lock()
    defer h.mu.Unlock()
    cbt, ok := h.engine.GetCombat(roomID)
    if !ok {
        return 0
    }
    // Find the combatant by UID. Read combat.Combat to find the correct method or field.
    // If Combat has GetCombatant(id string) *Combatant, use it.
    // Otherwise iterate the Combatants slice: for _, c := range cbt.Combatants { if c.ID == uid { return c.Position } }
    c := cbt.GetCombatant(uid) // verify method name in internal/game/combat/
    if c == nil {
        return 0
    }
    return c.Position
}

// CombatantsInRoom returns all combatants in the active combat for the given room.
// Returns nil if no combat is active.
func (h *CombatHandler) CombatantsInRoom(roomID string) []*combat.Combatant {
    h.mu.Lock()
    defer h.mu.Unlock()
    cbt, ok := h.engine.GetCombat(roomID)
    if !ok {
        return nil
    }
    // Return a snapshot copy to avoid races.
    // Read Combat struct to find the Combatants field name (likely Combatants []*Combatant).
    result := make([]*combat.Combatant, len(cbt.Combatants))
    copy(result, cbt.Combatants)
    return result
}
```

**IMPORTANT:** Before writing these, read `internal/game/combat/combat.go` and `internal/game/combat/engine.go` (or wherever the `Combat` struct is defined) to find:
1. The exact method or field to look up a `Combatant` by ID
2. The exact field name of the combatants slice on `Combat`

If `GetCombatant` doesn't exist, replace `cbt.GetCombatant(uid)` with:
```go
for _, c := range cbt.Combatants {
    if c.ID == uid {
        return c.Position
    }
}
return 0
```

- [ ] **Step 5: Fire callback after Stride/Step/Shove in resolveAndAdvanceLocked**

Find `resolveAndAdvanceLocked` (or the equivalent action-resolution function). Locate the `ActionStride`, `ActionStep`, and `ActionShove` cases. After each position update completes, add:
```go
if h.onCombatantMoved != nil {
    h.onCombatantMoved(roomID, action.ActorID)
}
```

Note: `roomID` is whatever variable holds the room's ID in that scope. `action.ActorID` is the mover's UID — verify the field name on the action struct.

- [ ] **Step 6: Run tests — verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | grep -E "FAIL|ok"
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go
git commit -m "feat(consumable-traps): Task 4 — onCombatantMoved callback + CombatantPosition + CombatantsInRoom"
```

---

### Task 5: grpc_service_trap.go — combat trigger logic

**Files:**
- Modify: `internal/gameserver/grpc_service_trap.go`
- Test: `internal/gameserver/grpc_service_trap_internal_test.go` (append)

**Context:** Read `internal/gameserver/grpc_service_trap.go` fully before implementing. Key details:
- `fireTrap(uid string, sess *session.PlayerSession, tmpl *trap.TrapTemplate, instanceID, dangerLevel string, disarmFailure bool)` — existing function, do NOT modify
- `pushMessage(sess *session.PlayerSession, msg string)` — existing helper
- `s.sessions.GetPlayer(uid)` — look up player session by UID (verify method name)
- Danger level: get zone via `s.world.GetZone(room.ZoneID)`, then `zone.DangerLevel`
- `result.DamageDice` is the damage dice field on `TriggerResult` (from `trap.ResolveTrigger`)
- `s.combatH.CombatantsInRoom(roomID)` returns all `[]*combat.Combatant` in a room
- `s.combatH.CombatantPosition(roomID, uid)` returns a combatant's position

- [ ] **Step 1: Write failing tests**

```go
// Append to internal/gameserver/grpc_service_trap_internal_test.go
// package gameserver

func TestCheckConsumableTraps_SkipsNonConsumable(t *testing.T) {
    svc, trapMgr := makeTrapSvc(t)

    mineTmpl := &trap.TrapTemplate{
        ID: "mine_ent", Name: "Mine",
        Trigger: trap.TriggerEntry, TriggerRangeFt: 5, ResetMode: trap.ResetOneShot,
        Payload: &trap.TrapPayload{Type: "mine"},
    }
    svc.trapTemplates["mine_ent"] = mineTmpl

    // Add as a WORLD trap (not consumable) — checkConsumableTraps must skip it
    instanceID := trap.TrapInstanceID("test", "room_a", "room", "world-mine")
    trapMgr.AddTrap(instanceID, "mine_ent", true)

    svc.checkConsumableTraps("room_a", "any-combatant")
    inst, ok := trapMgr.GetTrap(instanceID)
    require.True(t, ok)
    assert.True(t, inst.Armed, "world trap must not be fired by checkConsumableTraps")
}

func TestCheckConsumableTraps_NoActiveCombat_NoPanic(t *testing.T) {
    svc, trapMgr := makeTrapSvc(t)
    mineTmpl := &trap.TrapTemplate{
        ID: "mine_nc", Name: "Mine", TriggerRangeFt: 5, ResetMode: trap.ResetOneShot,
        Payload: &trap.TrapPayload{Type: "mine"},
    }
    svc.trapTemplates["mine_nc"] = mineTmpl
    instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "nc-1")
    require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 0))

    // No active combat — must return without panic
    assert.NotPanics(t, func() {
        svc.checkConsumableTraps("room_a", "combatant-x")
    })
    // Trap remains armed (no combat to resolve position)
    inst, ok := trapMgr.GetTrap(instanceID)
    require.True(t, ok)
    assert.True(t, inst.Armed)
}

// REQ-CTR-12: Deployed consumable trap must be disarmable via the existing disarm command.
func TestConsumableTrap_DisarmableViaExistingPath(t *testing.T) {
    svc, trapMgr := makeTrapSvc(t)
    mineTmpl := &trap.TrapTemplate{
        ID: "mine_dis", Name: "Mine", TriggerRangeFt: 5, DisableDC: 15,
        ResetMode: trap.ResetOneShot,
        Payload:   &trap.TrapPayload{Type: "mine"},
    }
    svc.trapTemplates["mine_dis"] = mineTmpl

    // Arm a consumable trap
    instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "dis-1")
    require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 5))

    // Mark it as detected (prerequisite for disarm)
    trapMgr.MarkDetected("disarmer", instanceID)
    assert.True(t, trapMgr.IsDetected("disarmer", instanceID))

    // Disarm via TrapManager.Disarm (the path used by handleDisarmTrap)
    trapMgr.Disarm(instanceID)
    inst, ok := trapMgr.GetTrap(instanceID)
    require.True(t, ok)
    assert.False(t, inst.Armed, "consumable trap must be disarmable via Disarm()")
}

// REQ-CTR-11: Out-of-combat consumable trap fires on next room entry.
func TestConsumableTrap_OutOfCombat_FiresOnRoomEntry(t *testing.T) {
    svc, trapMgr := makeTrapSvc(t)

    // Add a consumable mine template with TriggerEntry semantics for out-of-combat
    mineTmpl := &trap.TrapTemplate{
        ID: "mine_ooc", Name: "Mine",
        Trigger: trap.TriggerEntry, TriggerRangeFt: 5,
        ResetMode: trap.ResetOneShot,
        Payload:   &trap.TrapPayload{Type: "mine", DamageDice: "1d4", DamageType: "fire"},
    }
    svc.trapTemplates["mine_ooc"] = mineTmpl

    // Arm as consumable with DeployPosition=0 (out-of-combat)
    instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "ooc-1")
    require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 0))

    // checkEntryTraps should pick this up: it iterates all armed traps including consumables.
    // Add a player session to receive the trap fire.
    sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: "ooc_victim", Username: "ooc_victim", CharName: "ooc_victim", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()

    room, ok := svc.world.GetRoomByID("room_a") // verify method name
    if !ok {
        // Try alternate lookup
        t.Skip("room_a not found via GetRoomByID — verify lookup method")
    }

    // Fire checkEntryTraps — the consumable trap should fire and then disarm (one-shot)
    svc.checkEntryTraps("ooc_victim", sess, room)

    inst, ok := trapMgr.GetTrap(instanceID)
    require.True(t, ok)
    assert.False(t, inst.Armed, "one-shot consumable trap must disarm after firing on room entry")
}

// REQ-CTR-7: Multiple overlapping consumable traps must all fire independently.
// This test wires real combat so CombatantsInRoom returns a real mover, making both traps
// fire and disarm. The damage path is validated in fireConsumableTrapOnCombatant unit tests.
func TestCheckConsumableTraps_MultipleOverlapping_AllFire(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    worldMgr, sessMgr := testWorldAndSession(t)
    npcMgr := npc.NewManager()
    combatHandler := NewCombatHandler(
        combat.NewEngine(), npcMgr, sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )
    trapMgr := trap.NewTrapManager()
    mineTmpl := &trap.TrapTemplate{
        ID: "mine_multi", Name: "Mine",
        TriggerRangeFt: 5, BlastRadiusFt: 0, ResetMode: trap.ResetOneShot,
        Payload: &trap.TrapPayload{Type: "mine"},
    }
    tmplMap := map[string]*trap.TrapTemplate{"mine_multi": mineTmpl}
    svc := NewGameServiceServer(
        worldMgr, sessMgr, nil,
        NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
        NewChatHandler(sessMgr),
        zaptest.NewLogger(t),
        nil, roller, nil, npcMgr, combatHandler, nil,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil,
        trapMgr, tmplMap,
    )

    // Start combat so CombatantsInRoom returns real combatants.
    _, err := npcMgr.Spawn(&npc.Template{
        ID: "guard-multi", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
    }, "room_a")
    require.NoError(t, err)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "multi_mover", Username: "multi_mover", CharName: "multi_mover", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    sess.Status = statusInCombat
    _, err = combatHandler.Attack("multi_mover", "Guard")
    require.NoError(t, err)
    combatHandler.cancelTimer("room_a")

    // Deploy both traps at position 0; mover is at position 0 (default) — within range 5.
    id1 := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "multi-1")
    id2 := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "multi-2")
    require.NoError(t, trapMgr.AddConsumableTrap(id1, mineTmpl, 0))
    require.NoError(t, trapMgr.AddConsumableTrap(id2, mineTmpl, 0))

    svc.checkConsumableTraps("room_a", "multi_mover")

    inst1, ok1 := trapMgr.GetTrap(id1)
    inst2, ok2 := trapMgr.GetTrap(id2)
    require.True(t, ok1)
    require.True(t, ok2)
    assert.False(t, inst1.Armed, "first overlapping trap must be disarmed (fired)")
    assert.False(t, inst2.Armed, "second overlapping trap must be disarmed independently (fired)")
}

// REQ-CTR-9: Blast-radius trap fires on all combatants within radius (including deploying player).
// This test verifies the trap disarms after triggering; damage delivery per target is tested
// in fireConsumableTrapOnCombatant unit tests.
func TestCheckConsumableTraps_BlastRadius_DisarmsAfterFiring(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    worldMgr, sessMgr := testWorldAndSession(t)
    npcMgr := npc.NewManager()
    combatHandler := NewCombatHandler(
        combat.NewEngine(), npcMgr, sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )
    trapMgr := trap.NewTrapManager()
    mineTmpl := &trap.TrapTemplate{
        ID: "mine_blast", Name: "Mine",
        TriggerRangeFt: 5, BlastRadiusFt: 10, ResetMode: trap.ResetOneShot,
        Payload: &trap.TrapPayload{Type: "mine"},
    }
    tmplMap := map[string]*trap.TrapTemplate{"mine_blast": mineTmpl}
    svc := NewGameServiceServer(
        worldMgr, sessMgr, nil,
        NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
        NewChatHandler(sessMgr),
        zaptest.NewLogger(t),
        nil, roller, nil, npcMgr, combatHandler, nil,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil,
        trapMgr, tmplMap,
    )

    // Start combat.
    _, err := npcMgr.Spawn(&npc.Template{
        ID: "guard-blast", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
    }, "room_a")
    require.NoError(t, err)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "blast_mover", Username: "blast_mover", CharName: "blast_mover", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    sess.Status = statusInCombat
    _, err = combatHandler.Attack("blast_mover", "Guard")
    require.NoError(t, err)
    combatHandler.cancelTimer("room_a")

    instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "blast-1")
    require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 0))

    svc.checkConsumableTraps("room_a", "blast_mover")

    inst, ok := trapMgr.GetTrap(instanceID)
    require.True(t, ok)
    assert.False(t, inst.Armed, "blast-radius trap must be disarmed after firing (REQ-CTR-10)")
}

func TestProperty_CheckConsumableTraps_NoPanicWithArbitraryInputs(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        svc, _ := makeTrapSvc(t)
        roomID := rapid.SampledFrom([]string{"room_a", "room_b", "nonexistent"}).Draw(t, "room")
        uid := rapid.StringN(1, 20, -1).Draw(t, "uid")
        assert.NotPanics(t, func() {
            svc.checkConsumableTraps(roomID, uid)
        })
    })
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestCheckConsumableTraps|TestConsumableTrap" 2>&1 | tail -20
```
Expected: compile errors for missing methods.

- [ ] **Step 3: Implement `fireConsumableTrapOnCombatant`**

Add to `internal/gameserver/grpc_service_trap.go`:

```go
// fireConsumableTrapOnCombatant applies a consumable trap's payload to a single combat.Combatant.
// Handles both player and NPC targets. Does NOT call trapMgr.Disarm — caller is responsible.
// Precondition: target is non-nil; tmpl and s.trapTemplates are non-nil.
func (s *GameServiceServer) fireConsumableTrapOnCombatant(
    target *combat.Combatant,
    tmpl *trap.TrapTemplate,
    instanceID, dangerLevel string,
) {
    result := trap.ResolveTrigger(tmpl, dangerLevel, s.trapTemplates)
    dmg := 0
    if result.DamageDice != "" {
        dmg = s.dice.RollExpr(result.DamageDice)
        if dmg < 0 {
            dmg = 0
        }
    }
    target.ApplyDamage(dmg)

    // Player target: apply condition + send personal message.
    if sess, ok := s.sessions.GetPlayer(target.ID); ok {
        if result.ConditionID != "" && s.condRegistry != nil {
            if condDef, condOk := s.condRegistry.Get(result.ConditionID); condOk {
                _ = sess.Conditions.Apply(target.ID, condDef, 1, 0)
            }
        }
        pushMessage(sess, fmt.Sprintf("A %s triggers on you! (%d damage)", tmpl.Name, dmg))
        return
    }

    // NPC target: broadcast to all players in the same room.
    // Derive roomID from instanceID format: "zoneID/roomID/kind/trapID"
    parts := strings.SplitN(instanceID, "/", 4)
    if len(parts) < 2 {
        return
    }
    trapRoomID := parts[1]
    for _, p := range s.sessions.AllPlayers() {
        if p.RoomID == trapRoomID {
            pushMessage(p, fmt.Sprintf("A %s catches %s! (%d damage)", tmpl.Name, target.Name, dmg))
        }
    }
}
```

Note: verify `s.sessions.GetPlayer(uid)` is the correct method name. If it's `s.sessions.Get(uid)` or `s.sessions.Session(uid)`, use the actual name. Read `grpc_service_trap.go` for existing usage.

- [ ] **Step 4: Implement `checkConsumableTraps`**

```go
// checkConsumableTraps checks all armed consumable traps in the room against the moving combatant.
// Called via the onCombatantMoved callback after any Stride, Step, or Shove resolves.
// Precondition: roomID is non-empty.
func (s *GameServiceServer) checkConsumableTraps(roomID, movedCombatantID string) {
    if s.trapMgr == nil || s.trapTemplates == nil {
        return
    }
    room, ok := s.world.GetRoomByID(roomID)
    if !ok {
        return
    }
    zone, ok := s.world.GetZone(room.ZoneID)
    if !ok {
        return
    }
    dangerLevel := zone.DangerLevel

    // Find the moving combatant. If not in active combat, return early.
    combatants := s.combatH.CombatantsInRoom(roomID)
    var mover *combat.Combatant
    for _, c := range combatants {
        if c.ID == movedCombatantID {
            mover = c
            break
        }
    }
    if mover == nil {
        return
    }
    movedPos := mover.Position

    instanceIDs := s.trapMgr.TrapsForRoom(zone.ID, roomID)
    for _, instanceID := range instanceIDs {
        inst, ok := s.trapMgr.GetTrap(instanceID)
        if !ok || !inst.Armed || !inst.IsConsumable {
            continue
        }
        tmpl, ok := s.trapTemplates[inst.TemplateID]
        if !ok {
            continue
        }

        dist := movedPos - inst.DeployPosition
        if dist < 0 {
            dist = -dist
        }
        if dist > trap.EffectiveTriggerRange(tmpl) {
            continue
        }

        // Trap fires. Multiple overlapping traps all fire independently.
        if tmpl.BlastRadiusFt == 0 {
            // Single target: the moving combatant.
            s.fireConsumableTrapOnCombatant(mover, tmpl, instanceID, dangerLevel)
        } else {
            // Blast radius: all combatants within radius of DeployPosition (REQ-CTR-9).
            for _, c := range combatants {
                d := c.Position - inst.DeployPosition
                if d < 0 {
                    d = -d
                }
                if d <= tmpl.BlastRadiusFt {
                    s.fireConsumableTrapOnCombatant(c, tmpl, instanceID, dangerLevel)
                }
            }
        }
        s.trapMgr.Disarm(instanceID) // always one-shot (REQ-CTR-10)
    }
}
```

Note: verify `s.world.GetRoomByID(roomID)` — if the room manager method has a different name (e.g., `s.world.Room(roomID)` or `s.world.GetRoom(roomID)`), use the correct one. Read `grpc_service_trap.go` for existing room lookup calls.

- [ ] **Step 5: Implement `WireConsumableTrapTrigger`**

```go
// WireConsumableTrapTrigger connects the combat movement callback to consumable trap checking.
// Call during GameServiceServer initialization after combatH and trapMgr are set.
func (s *GameServiceServer) WireConsumableTrapTrigger() {
    if s.combatH == nil {
        return
    }
    s.combatH.SetOnCombatantMoved(func(roomID, movedCombatantID string) {
        s.checkConsumableTraps(roomID, movedCombatantID)
    })
}
```

- [ ] **Step 6: Run tests — verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | grep -E "FAIL|ok"
```
Fix any API mismatches. All tests must pass.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service_trap.go internal/gameserver/grpc_service_trap_internal_test.go
git commit -m "feat(consumable-traps): Task 5 — checkConsumableTraps + fireConsumableTrapOnCombatant"
```

---

### Task 6: Proto + command + bridge handler

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/game/command/commands.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Add proto message and field**

Read `api/proto/game/v1/game.proto`. Find `DisarmTrapRequest` and the `ClientMessage.payload` oneof. After `disarm_trap = 84`, add:

In the message definitions section:
```proto
message DeployTrapRequest {
    string item_name = 1;
}
```

In `ClientMessage.payload` oneof:
```proto
DeployTrapRequest deploy_trap = 85;
```

- [ ] **Step 2: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto 2>&1 | tail -10
```
If no `make proto` target, check the Makefile: `grep -n "proto" Makefile`. Run the correct target. Verify the generated Go file in `internal/gameserver/gamev1/` updated with `DeployTrapRequest`.

- [ ] **Step 3: Add command constant and entry**

In `internal/game/command/commands.go`, add constant near `HandlerDisarmTrap`:
```go
HandlerDeployTrap = "deploy_trap"
```

Add command entry near the `disarm_trap` entry:
```go
{
    Name:     "deploy_trap",
    Aliases:  []string{"deploy"},
    Help:     "deploy <item> — arm a trap item at your current position (1 AP in combat)",
    Category: CategoryCombat,
    Handler:  HandlerDeployTrap,
},
```

- [ ] **Step 4: Add bridge handler**

In `internal/frontend/handlers/bridge_handlers.go`, add to `bridgeHandlerMap`:
```go
command.HandlerDeployTrap: bridgeDeployTrap,
```

Add function (near `bridgeDisarmTrap`):
```go
func bridgeDeployTrap(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorPrompt(bctx, "Usage: deploy <item>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload: &gamev1.ClientMessage_DeployTrap{
            DeployTrap: &gamev1.DeployTrapRequest{ItemName: bctx.parsed.RawArgs},
        },
    }}, nil
}
```

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok"
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add api/proto/ internal/game/command/ internal/frontend/handlers/
git commit -m "feat(consumable-traps): Task 6 — proto DeployTrapRequest + command + bridge handler"
```

---

### Task 7: handleDeployTrap handler + tests

**Files:**
- Create: `internal/gameserver/grpc_service_deploy_trap.go`
- Create: `internal/gameserver/grpc_service_deploy_trap_test.go`

**Context:**
- Read `internal/gameserver/grpc_service_trap_internal_test.go` for the exact `makeTrapSvc` nil pattern
- The `NewGameServiceServer` 46-param nil pattern is at the top of this plan — copy it exactly
- For backpack operations: `sess.Backpack.Add(itemDefID, qty, registry)`, `sess.Backpack.Remove(instanceID, qty)`, `sess.Backpack.Items()`
- For UUID: use `fmt.Sprintf("%d", time.Now().UnixNano())` — avoids external deps
- `s.world.GetRoomByID(roomID)` — verify method name from existing code
- `s.world.GetZone(zoneID)` — existing method

- [ ] **Step 1: Write failing tests**

```go
// internal/gameserver/grpc_service_deploy_trap_test.go
package gameserver

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/dice"
    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/trap"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap/zaptest"
    "pgregory.net/rapid"
)

// makeTrapDeploySvc builds on makeTrapSvc, adding an invRegistry with a deployable mine.
func makeTrapDeploySvc(t *testing.T) (*GameServiceServer, *trap.TrapManager, *inventory.Registry) {
    t.Helper()
    svc, trapMgr := makeTrapSvc(t)

    reg := inventory.NewRegistry()
    mineTmpl := &trap.TrapTemplate{
        ID: "mine", Name: "Mine", TriggerRangeFt: 5, BlastRadiusFt: 10,
        ResetMode: trap.ResetOneShot,
        Payload:   &trap.TrapPayload{Type: "mine", DamageDice: "4d6", DamageType: "fire"},
    }
    svc.trapTemplates["mine"] = mineTmpl

    require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
        ID: "deployable_mine", Name: "Deployable Mine",
        Kind: inventory.KindTrap, TrapTemplateRef: "mine",
        Weight: 2.0, Stackable: true, MaxStack: 5, Value: 300,
    }))
    svc.invRegistry = reg
    return svc, trapMgr, reg
}

func TestHandleDeployTrap_OutOfCombat_Success(t *testing.T) {
    svc, trapMgr, reg := makeTrapDeploySvc(t)

    sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: "deployer", Username: "deployer", CharName: "deployer", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    _, err = sess.Backpack.Add("deployable_mine", 1, reg)
    require.NoError(t, err)

    ev, err := svc.handleDeployTrap("deployer", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
    require.NoError(t, err)
    assert.Contains(t, ev.GetMessage().GetContent(), "arm")

    // Backpack must be empty after deploy
    items := sess.Backpack.Items()
    assert.Empty(t, items)

    // TrapManager must have a new armed consumable trap in room_a
    traps := trapMgr.TrapsForRoom("test", "room_a")
    found := false
    for _, id := range traps {
        inst, ok := trapMgr.GetTrap(id)
        if ok && inst.IsConsumable && inst.Armed {
            assert.Equal(t, 0, inst.DeployPosition, "out-of-combat deploy must use position 0")
            found = true
            break
        }
    }
    assert.True(t, found, "a consumable trap must be armed after deploy")
}

func TestHandleDeployTrap_InCombat_CostsOneAP(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    worldMgr, sessMgr := testWorldAndSession(t)
    npcMgr := npc.NewManager()
    combatHandler := NewCombatHandler(
        combat.NewEngine(), npcMgr, sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )
    trapMgr := trap.NewTrapManager()
    mineTmpl := &trap.TrapTemplate{ID: "mine", Name: "Mine", TriggerRangeFt: 5, ResetMode: trap.ResetOneShot}
    tmplMap := map[string]*trap.TrapTemplate{"mine": mineTmpl}
    reg := inventory.NewRegistry()
    require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
        ID: "deployable_mine", Name: "Deployable Mine",
        Kind: inventory.KindTrap, TrapTemplateRef: "mine",
        Weight: 2.0, Stackable: true, MaxStack: 5, Value: 300,
    }))

    svc := NewGameServiceServer(
        worldMgr, sessMgr,
        nil,
        NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
        NewChatHandler(sessMgr),
        zaptest.NewLogger(t),
        nil, roller, nil, npcMgr, combatHandler, nil,  // 7-12
        nil, nil, nil, nil, nil, nil,                   // 13-18
        nil, nil, nil, nil, nil, nil, nil, nil, "",     // 19-27
        nil, nil, nil,                                  // 28-30
        nil, nil, nil,                                  // 31-33
        nil, nil, nil, nil, nil, nil, nil,              // 34-40
        nil, nil,                                       // 41-42
        nil,                                            // 43
        nil,                                            // 44
        trapMgr, tmplMap,                               // 45-46
    )
    svc.invRegistry = reg

    _, err := npcMgr.Spawn(&npc.Template{
        ID: "guard-dep", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
    }, "room_a")
    require.NoError(t, err)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "dep_player", Username: "dep_player", CharName: "dep_player", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    sess.Status = statusInCombat
    _, err = combatHandler.Attack("dep_player", "Guard")
    require.NoError(t, err)
    combatHandler.cancelTimer("room_a")

    _, err = sess.Backpack.Add("deployable_mine", 1, reg)
    require.NoError(t, err)

    apBefore := combatHandler.RemainingAP("dep_player")
    require.GreaterOrEqual(t, apBefore, 1)

    ev, deployErr := svc.handleDeployTrap("dep_player", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
    require.NoError(t, deployErr)
    assert.Contains(t, ev.GetMessage().GetContent(), "arm")
    assert.Equal(t, apBefore-1, combatHandler.RemainingAP("dep_player"), "deploy must cost exactly 1 AP in combat")
}

func TestHandleDeployTrap_NotEnoughAP(t *testing.T) {
    logger := zaptest.NewLogger(t)
    roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
    worldMgr, sessMgr := testWorldAndSession(t)
    npcMgr := npc.NewManager()
    combatHandler := NewCombatHandler(
        combat.NewEngine(), npcMgr, sessMgr, roller,
        func(_ string, _ []*gamev1.CombatEvent) {},
        testRoundDuration, makeTestConditionRegistry(),
        nil, nil, nil, nil, nil, nil, nil,
    )
    trapMgr := trap.NewTrapManager()
    tmplMap := map[string]*trap.TrapTemplate{
        "mine": {ID: "mine", Name: "Mine", TriggerRangeFt: 5, ResetMode: trap.ResetOneShot},
    }
    reg := inventory.NewRegistry()
    require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
        ID: "deployable_mine", Name: "Deployable Mine",
        Kind: inventory.KindTrap, TrapTemplateRef: "mine",
        Weight: 2.0, Stackable: true, MaxStack: 5, Value: 300,
    }))

    svc := NewGameServiceServer(
        worldMgr, sessMgr,
        nil,
        NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
        NewChatHandler(sessMgr),
        zaptest.NewLogger(t),
        nil, roller, nil, npcMgr, combatHandler, nil,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil,
        nil,
        nil,
        trapMgr, tmplMap,
    )
    svc.invRegistry = reg

    _, err := npcMgr.Spawn(&npc.Template{
        ID: "guard-noap", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
    }, "room_a")
    require.NoError(t, err)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "noap", Username: "noap", CharName: "noap", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    sess.Status = statusInCombat
    _, err = combatHandler.Attack("noap", "Guard")
    require.NoError(t, err)
    combatHandler.cancelTimer("room_a")

    // Exhaust all AP — SpendAP exists on CombatHandler (confirmed: grpc_service_loadout_test.go:88).
    rem := combatHandler.RemainingAP("noap")
    if rem > 0 {
        require.NoError(t, combatHandler.SpendAP("noap", rem))
    }

    _, err = sess.Backpack.Add("deployable_mine", 1, reg)
    require.NoError(t, err)

    ev, deployErr := svc.handleDeployTrap("noap", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
    require.NoError(t, deployErr)
    assert.Contains(t, ev.GetMessage().GetContent(), "Not enough AP")
    // Item must NOT be consumed
    items := sess.Backpack.Items()
    total := 0
    for _, it := range items {
        total += it.Quantity
    }
    assert.Equal(t, 1, total, "item must remain in backpack when AP denied")
}

func TestHandleDeployTrap_ItemNotFound(t *testing.T) {
    svc, _, _ := makeTrapDeploySvc(t)
    sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: "nf", Username: "nf", CharName: "nf", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()

    ev, err := svc.handleDeployTrap("nf", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
    require.NoError(t, err)
    assert.Contains(t, ev.GetMessage().GetContent(), "don't have")
}

func TestHandleDeployTrap_WrongKind(t *testing.T) {
    svc, _, reg := makeTrapDeploySvc(t)
    // Use KindConsumable — exists in internal/game/inventory/item.go line 16; no extra required fields.
    require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
        ID: "bandage", Name: "Bandage", Kind: inventory.KindConsumable,
    }))
    sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: "wk", Username: "wk", CharName: "wk", Role: "player",
        RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Conditions = condition.NewActiveSet()
    _, err = sess.Backpack.Add("bandage", 1, reg)
    require.NoError(t, err)

    ev, err := svc.handleDeployTrap("wk", &gamev1.DeployTrapRequest{ItemName: "Bandage"})
    require.NoError(t, err)
    assert.Contains(t, ev.GetMessage().GetContent(), "can't deploy")
}

func TestProperty_DeployTrap_BackpackDecrementsByOne(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        svc, _, reg := makeTrapDeploySvc(t)
        count := rapid.IntRange(1, 5).Draw(t, "count")
        uid := "prop_dep_" + rapid.StringN(3, 8, -1).Draw(t, "uid")
        sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
            UID: uid, Username: uid, CharName: uid, Role: "player",
            RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
        })
        require.NoError(t, err)
        sess.Conditions = condition.NewActiveSet()
        _, err = sess.Backpack.Add("deployable_mine", count, reg)
        require.NoError(t, err)

        _, err = svc.handleDeployTrap(uid, &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
        require.NoError(t, err)

        total := 0
        for _, it := range sess.Backpack.Items() {
            total += it.Quantity
        }
        assert.Equal(t, count-1, total, "exactly 1 item must be consumed per deploy")
    })
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleDeployTrap|TestProperty_DeployTrap" 2>&1 | tail -20
```
Expected: compile errors.

- [ ] **Step 3: Implement handleDeployTrap**

Create `internal/gameserver/grpc_service_deploy_trap.go`:

```go
package gameserver

import (
    "fmt"
    "strings"
    "time"

    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/cory-johannsen/mud/internal/game/trap"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "go.uber.org/zap"
)

// handleDeployTrap processes a deploy_trap command.
// Precondition: uid refers to an existing player session; req.ItemName is non-empty.
// Postcondition: on success, 1 item is removed from backpack and a consumable trap is armed.
func (s *GameServiceServer) handleDeployTrap(uid string, req *gamev1.DeployTrapRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("You are not in the game."), nil
    }

    // AP check first (REQ-CTR-1, per spec step order): in-combat deploys cost 1 AP.
    // Per spec: AP is spent before item/template validation. If the item is not found or
    // the template is missing after AP is spent, the AP is consumed without deploying a trap.
    // This is intentional per spec — the deploy attempt costs an action regardless.
    inCombat := sess.Status == statusInCombat
    if inCombat {
        if s.combatH.RemainingAP(uid) < 1 {
            return messageEvent("Not enough AP to deploy a trap."), nil
        }
        // SpendAP exists on CombatHandler (see grpc_service_loadout_test.go:88).
        if err := s.combatH.SpendAP(uid, 1); err != nil {
            return messageEvent("Not enough AP to deploy a trap."), nil
        }
    }

    // Find item by name (case-insensitive) — REQ-CTR-2.
    var foundInstanceID string
    var foundDef *inventory.ItemDef
    for _, inst := range sess.Backpack.Items() {
        def, defOk := s.invRegistry.Item(inst.ItemDefID)
        if !defOk {
            continue
        }
        if strings.EqualFold(def.Name, req.ItemName) {
            foundInstanceID = inst.InstanceID
            foundDef = def
            break
        }
    }
    if foundInstanceID == "" {
        return messageEvent(fmt.Sprintf("You don't have a %s.", req.ItemName)), nil
    }
    if foundDef.Kind != inventory.KindTrap {
        return messageEvent("You can't deploy that."), nil
    }

    tmpl, ok := s.trapTemplates[foundDef.TrapTemplateRef]
    if !ok {
        s.logger.Error("missing trap template for deployable item",
            zap.String("item_id", foundDef.ID),
            zap.String("trap_template_ref", foundDef.TrapTemplateRef),
        )
        return messageEvent("That trap is broken — contact an admin."), nil
    }

    // Remove 1 from backpack.
    if err := sess.Backpack.Remove(foundInstanceID, 1); err != nil {
        return messageEvent(fmt.Sprintf("Failed to remove %s from inventory.", req.ItemName)), nil
    }

    // Determine deploy position.
    deployPos := 0
    if inCombat {
        deployPos = s.combatH.CombatantPosition(sess.RoomID, uid)
    }

    // Determine zone for instance ID.
    room, ok := s.world.GetRoomByID(sess.RoomID)
    if !ok {
        return messageEvent("You are not in a valid room."), nil
    }
    zone, ok := s.world.GetZone(room.ZoneID)
    if !ok {
        return messageEvent("You are not in a valid zone."), nil
    }

    instanceID := trap.TrapInstanceID(
        zone.ID, sess.RoomID, trap.TrapKindConsumable,
        fmt.Sprintf("%d", time.Now().UnixNano()),
    )
    if err := s.trapMgr.AddConsumableTrap(instanceID, tmpl, deployPos); err != nil {
        s.logger.Error("failed to add consumable trap", zap.Error(err))
        return messageEvent("Failed to arm trap."), nil
    }

    if inCombat {
        return messageEvent(fmt.Sprintf("You arm a %s at your position.", foundDef.Name)), nil
    }
    return messageEvent(fmt.Sprintf("You arm a %s here.", foundDef.Name)), nil
}
```

**Verify before writing:**
- `s.sessions.GetPlayer(uid)` — read `grpc_service_trap.go` for existing session lookup
- `sess.Backpack.Items()` and `inst.InstanceID`, `inst.ItemDefID` — read `inventory/backpack.go`
- `sess.Backpack.Remove(instanceID, qty)` — verify signature
- `s.world.GetRoomByID(roomID)` — if different name, use the correct one
- `messageEvent(content)` — check if this helper exists; if not, create it:
  ```go
  func messageEvent(content string) *gamev1.ServerEvent {
      return &gamev1.ServerEvent{
          Payload: &gamev1.ServerEvent_Message{
              Message: &gamev1.MessageEvent{Content: content},
          },
      }
  }
  ```

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | grep -E "FAIL|ok"
```
Fix any API mismatches. All tests must pass before continuing.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service_deploy_trap.go internal/gameserver/grpc_service_deploy_trap_test.go
git commit -m "feat(consumable-traps): Task 7 — handleDeployTrap + tests"
```

---

### Task 8: Wire dispatch + feature docs

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `docs/features/consumable-traps.md`
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Wire deploy_trap dispatch**

Read `internal/gameserver/grpc_service.go` and find the message dispatch switch where `*gamev1.ClientMessage_DisarmTrap` is handled. Add adjacent case:

```go
case *gamev1.ClientMessage_DeployTrap:
    ev, err = s.handleDeployTrap(uid, payload.DeployTrap)
```

- [ ] **Step 2: Call WireConsumableTrapTrigger during init**

In the same file, find where `WireCoverCrossfireTrap()` is called. Immediately after it, add:
```go
s.WireConsumableTrapTrigger()
```

- [ ] **Step 3: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok"
```
All tests must pass.

- [ ] **Step 4: Update docs/features/consumable-traps.md**

Mark all `[ ]` as `[x]`. Add at end:

```markdown
## Implementation

Completed 2026-03-21. `TrapTemplate` gained `trigger_range_ft` and `blast_radius_ft` fields. `TrapInstanceState` gained `DeployPosition` and `IsConsumable` fields. `TrapManager.AddConsumableTrap` arms player-deployed instances. `CombatHandler.SetOnCombatantMoved`/`CombatantPosition`/`CombatantsInRoom` support positional trigger checks. `checkConsumableTraps` and `fireConsumableTrapOnCombatant` in the service layer handle combat trigger + blast radius. `handleDeployTrap` dispatched via proto field 85.
```

- [ ] **Step 5: Update docs/features/index.yaml**

Find the `consumable-traps` entry and change `status: planned` to `status: complete`.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go docs/features/consumable-traps.md docs/features/index.yaml
git commit -m "feat(consumable-traps): Task 8 — wire dispatch + feature docs complete"
```
