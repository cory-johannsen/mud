## Trigger
When to load: working on NPC behavior, HTN planning, Lua scripting, respawn, NPC lifecycle, combat AI, zone ticks, loot drops, or any code under `internal/game/ai/`, `internal/scripting/`, `internal/game/npc/`, or `internal/gameserver/npc_handler.go`.

## Responsibility Boundary

### HTN Planner (`internal/game/ai/`)
Owns: domain model (Task, Method, Operator), domain validation and loading from YAML, the Planner that decomposes the root `behave` task into an ordered `[]PlannedAction` via method/operator lookup, WorldState construction (`build_state.go`), and the Registry that indexes Planners by domain ID.

Delegates to: `internal/scripting/` (ScriptCaller interface) for Lua precondition evaluation; `internal/game/combat` (via `build_state.go`) for the raw Combat snapshot. Does NOT execute actions — action execution belongs to CombatHandler.

### Lua Scripting (`internal/scripting/`)
Owns: sandboxed GopherLua VM lifecycle (`NewSandboxedState`), per-zone LState management (Manager.LoadZone / LoadGlobal), instruction-count enforcement via `countingContext`, and all `engine.*` module registrations (log, dice, entity, combat, world, event, map). Exposes `CallHook` and `CallHookWithContext` as the sole public dispatch surface.

Delegates to: all game callbacks (GetCombatant, ApplyCondition, ApplyDamage, Broadcast, QueryRoom, GetCombatantsInRoom, GetEntityRoom, RevealZoneMap) which are injected after construction. The scripting package has zero imports from game domain packages; all cross-cutting behavior is wired by the caller (gameserver).

### NPC Lifecycle (`internal/game/npc/` + `internal/gameserver/npc_handler.go`)
Owns: Template definition and YAML loading, Instance creation (NewInstanceWithResolver with weighted equipment roll), NPC Manager (concurrent registry of instances by ID and room), loot generation (GenerateLoot), and respawn scheduling/draining (RespawnManager.Schedule, Tick, PopulateRoom). NPCHandler in gameserver provides the examine command and thin wrappers over Manager.

Delegates to: `internal/gameserver/combat_handler.go` for NPC death processing (removeDeadNPCsLocked calls GenerateLoot, distributes currency, drops items on floor, then calls RespawnManager.Schedule). Zone tick callbacks in grpc_service.go drive RespawnManager.Tick.

## Key Files

- `/home/cjohannsen/src/mud/internal/game/ai/domain.go` — Task, Method, Operator types; Domain validation; LoadDomains from YAML dir
- `/home/cjohannsen/src/mud/internal/game/ai/planner.go` — Planner.Plan: root `behave` decomposition, ScriptCaller interface, PlannedAction
- `/home/cjohannsen/src/mud/internal/game/ai/world_state.go` — WorldState, NPCState, CombatantState, target resolution (nearest_enemy, weakest_enemy, self)
- `/home/cjohannsen/src/mud/internal/game/ai/build_state.go` — BuildCombatWorldState: constructs WorldState from a live Combat snapshot
- `/home/cjohannsen/src/mud/internal/game/ai/registry.go` — Registry: domain ID → Planner index; prevents duplicate registration
- `/home/cjohannsen/src/mud/internal/scripting/sandbox.go` — NewSandboxedState: safe stdlib only, dangerous globals stripped, instruction-count context
- `/home/cjohannsen/src/mud/internal/scripting/manager.go` — Manager: per-zone LState map, CallHook, CallHookWithContext, __global__ fallback
- `/home/cjohannsen/src/mud/internal/scripting/modules.go` — RegisterModules: engine.log, dice, entity, combat, world, event, map
- `/home/cjohannsen/src/mud/internal/game/npc/template.go` — Template YAML schema and validation; LoadTemplates
- `/home/cjohannsen/src/mud/internal/game/npc/instance.go` — Instance struct; NewInstanceWithResolver; TryTaunt; IsDead
- `/home/cjohannsen/src/mud/internal/game/npc/manager.go` — Manager: Spawn (with letter-suffix deduplication), Remove, Move, FindInRoom, InstancesInRoom
- `/home/cjohannsen/src/mud/internal/game/npc/loot.go` — LootTable, ItemDrop, GenerateLoot
- `/home/cjohannsen/src/mud/internal/game/npc/respawn.go` — RespawnManager: Schedule, Tick, PopulateRoom; RoomSpawn config
- `/home/cjohannsen/src/mud/internal/gameserver/npc_handler.go` — NPCHandler: InstancesInRoom, MoveNPC, Examine
- `/home/cjohannsen/src/mud/internal/gameserver/zone_tick.go` — ZoneTickManager: RegisterTick, Start; one goroutine fires all zone callbacks per interval

## Core Data Structures

```go
// Domain (internal/game/ai/domain.go)
type Domain struct {
    ID          string
    Tasks       []*Task     // abstract goals
    Methods     []*Method   // decomposition rules; Precondition is a Lua hook name
    Operators   []*Operator // primitive actions: action, target, cooldown_rounds, ap_cost
}

// WorldState (internal/game/ai/world_state.go)
type WorldState struct {
    NPC        *NPCState         // planning NPC
    Room       *RoomState        // current room
    Combatants []*CombatantState // all combatants including dead
}

// PlannedAction (internal/game/ai/planner.go)
type PlannedAction struct {
    Action string // "attack", "strike", "pass", "reload", "apply_mental_state", etc.
    Target         string // resolved name/UID
    OperatorID     string
    Track          string // mental state track (rage/despair/delirium)
    Severity       string // mild/moderate/severe
    CooldownRounds int
    APCost         int
}

// Template (internal/game/npc/template.go)
type Template struct {
    ID           string
    Name         string
    Level        int
    MaxHP        int
    AC           int
    AIDomain     string         // HTN domain ID; empty = legacy fallback
    RespawnDelay string         // Go duration string; empty = no respawn
    Loot         *LootTable
    Weapon       []EquipmentEntry // weighted random
    Armor        []EquipmentEntry // weighted random
    Abilities    Abilities
    Combat       CombatStrategy
}

// Instance (internal/game/npc/instance.go) — live runtime entity
// Key fields: ID, TemplateID, RoomID, CurrentHP, MaxHP, AC, AIDomain,
//             Loot, AbilityCooldowns (map[operatorID]roundsRemaining)
```

## Primary Data Flow

1. **Zone tick fires**: ZoneTickManager calls `tickZone(zoneID)` once per interval. For each non-combat NPC in the zone, `tickNPCIdle` runs a Planner.Plan cycle. For each combat-active NPC, CombatHandler's auto-queue step calls `applyPlanLocked`.
2. **HTN decomposition**: Planner.Plan starts with `taskQueue = ["behave"]`. Each abstract task is replaced by the first Method whose Lua precondition returns `true` via `ScriptCaller.CallHook`; operators are emitted as PlannedActions. Max depth = 32.
3. **Action queue**: `applyPlanLocked` converts PlannedActions to `combat.QueuedAction` entries and enqueues them on the Combatant until AP budget exhausted. `apply_mental_state` operators check `AbilityCooldowns` before execution.
4. **Combat auto-queue**: At start-of-NPC-turn (`autoQueueNPCActionsLocked`), if no actions are queued and AIDomain is set, Planner.Plan is called fresh from the current Combat snapshot; if no domain or planning fails, legacyAutoQueueLocked (simple attack fallback) runs instead.
5. **NPC death**: After a combat round, `removeDeadNPCsLocked` detects dead NPC combatants. It calls `GenerateLoot`, distributes currency to living participants, drops items via floorMgr, awards kill XP, then calls `npcMgr.Remove` followed by `respawnMgr.Schedule`.
6. **Respawn**: On each zone tick, `tickZone` calls `respawnMgr.Tick(now, npcMgr)`. Ready entries (readyAt <= now) that have room below population cap call `mgr.Spawn`, re-inserting the NPC into the room index.

## Invariants & Contracts

- **AI-INV-1**: Planner.Plan precondition: `state != nil && state.NPC != nil`; postcondition: returns `[]PlannedAction{}` (never nil) and never propagates Lua errors (treated as precondition-false).
- **AI-INV-2**: Domain.Validate guarantees unique IDs within Tasks, Methods, and Operators; all Method subtasks must resolve to a valid Task ID or Operator ID before a Domain is accepted.
- **AI-INV-3**: Lua sandbox invariant: only `base`, `table`, `string`, `math` stdlib are loaded; `dofile`, `loadfile`, `load`, `loadstring`, `collectgarbage`, `require`, `module`, `newproxy`, `setfenv`, `getfenv`, and `_printregs` are nil'd out before any user script runs.
- **AI-INV-4**: Instruction limit: each `CallHook` execution is bounded by the countingContext; when the opcode count reaches zero, the VM context cancels and the call returns `(LNil, nil)` — never a panic.
- **AI-INV-5**: Per-zone LState serialization: each zone VM holds a `sync.Mutex`; all CallHook calls for a given zone serialize through it. Cross-zone calls never share an LState.
- **AI-INV-6**: NPC state transitions: `IsDead()` ≡ `CurrentHP <= 0`. An Instance is removed from npcMgr.instances atomically before respawn is scheduled; there is no window where a dead Instance is in the room index.
- **AI-INV-7**: RespawnManager.Schedule is a no-op when delay == 0 (template has no `respawn_delay`). Population cap (RoomSpawn.Max) is checked before every Spawn in Tick and PopulateRoom.
- **AI-INV-8**: AbilityCooldowns is nil at spawn and lazily initialized on first write in applyPlanLocked; reading a nil map returns zero (safe in Go).

## Extension Points

### Adding a new NPC behavior / HTN operator
1. Add a new YAML file (or extend an existing one) under the AI domains content directory with the new `operator` entry. Required fields: `id`, `action`. Valid actions: `attack`, `strike`, `pass`, `apply_mental_state`.
2. Add a `task` entry if the new behavior is an abstract goal, and a `method` entry referencing it with a Lua `precondition` function name (or leave precondition empty for unconditional).
3. If the operator uses a Lua precondition, implement the named Lua function in the zone's `.lua` script directory. The function receives the NPC's UID as a string and must return `true` or `false`.
4. If the operator introduces a new `action` string not in the existing switch in `applyPlanLocked`, add a new `case` in `/home/cjohannsen/src/mud/internal/gameserver/combat_handler.go`.
5. Run `make test` and ensure all tests pass (SWENG-6).

### Adding a new Lua engine module
1. Add a `new<Name>Module` method to `Manager` in `/home/cjohannsen/src/mud/internal/scripting/modules.go`, following the existing pattern (create LTable, add LFunctions, call injected Go callbacks).
2. Wire any required Go callbacks as injected fields on `Manager` (nil-safe, injected after construction by the gameserver wiring layer).
3. Register the new module in `RegisterModules` with `L.SetField(engine, "<name>", m.new<Name>Module(L))`.
4. Add tests in `modules_test.go` with property-based coverage (SWENG-5a).
5. Update `/home/cjohannsen/src/mud/docs/requirements/SCRIPTING.md` to add the new module requirement.

## Common Pitfalls

- **Forgetting that Lua errors are silent**: CallHook logs at Warn level and returns `(LNil, nil)` on Lua runtime errors. A precondition that panics or errors is treated as `false`. Use `engine.log.warn` liberally in Lua scripts during development.
- **Calling Plan outside a lock**: `applyPlanLocked` is documented as requiring `combatMu` to be held. Never call `Planner.Plan` from a goroutine without appropriate serialization.
- **Adding AIDomain to a template without registering the domain**: If `inst.AIDomain` is set but no matching domain is in the Registry, the planner silently falls back to `legacyAutoQueueLocked`. Validate domain YAML with `Domain.Validate()` at startup.
- **Zero-delay respawn looping**: If `respawn_delay` is empty in both the RoomSpawn config and the Template, `Schedule` is a no-op. The NPC simply does not respawn — this is correct behavior, not a bug.
- **Concurrent PopulateRoom + Tick**: RespawnManager's docstring explicitly states these must not be called concurrently with each other. PopulateRoom is startup-only; Tick is zone-tick-only. Never call PopulateRoom from a zone tick callback.
- **Module callbacks are nil at test time**: Manager's injected callbacks (GetCombatant, ApplyCondition, etc.) are nil by default. Engine module functions guard against nil callbacks and return no-ops or LNil. Tests that exercise Lua scripts must inject stubs explicitly.
- **`"flee"` action is unimplemented in dispatch**: `domain.go`'s `Operator.Action` comment lists `"flee"` as a valid action, but `applyPlanLocked` in `combat_handler.go` has no case for it — it silently falls through to the `default: pass` branch. Do not add a `"flee"` action to NPC domain files without first implementing the dispatch case.
