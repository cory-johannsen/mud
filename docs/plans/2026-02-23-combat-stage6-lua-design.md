# Combat Stage 6 — Lua Scripting Engine Design

**Date:** 2026-02-23
**Status:** Approved

## Goal

Embed GopherLua into the game server to enable data-driven combat and world scripting. Wire the condition lifecycle hooks (`lua_on_apply`, `lua_on_remove`, `lua_on_tick`) already stored in condition YAMLs, the three combat resolution hooks (`on_attack_roll`, `on_damage_roll`, `on_condition_apply`), and the three room hooks (`on_enter`, `on_exit`, `on_look`). The full engine API surface is implemented; modules not exercised by current content are explicit no-op stubs.

## Architecture

`internal/scripting` is a standalone package with no knowledge of `combat`, `condition`, or `world`. It exposes a narrow Go API — `Manager.CallHook` — and registers engine API modules into each VM at construction time. The combat engine, condition system, and gameserver each hold a `*scripting.Manager` reference and call hooks at the defined points. Hot-reload is deferred to a future stage; scripts load at startup only.

## Package Structure

```
internal/scripting/
├── manager.go        — Manager: per-zone LState map, LoadZone, CallHook
├── sandbox.go        — NewSandboxedState: strips unsafe libs, sets instruction limit
├── modules.go        — RegisterModules: registers all engine.* Lua libs
├── manager_test.go
└── sandbox_test.go

content/scripts/
├── zones/
│   └── downtown/
│       ├── combat.lua    — on_attack_roll, on_damage_roll, on_condition_apply stubs
│       └── rooms.lua     — on_enter, on_exit, on_look stubs
└── conditions/
    └── dying.lua         — lua_on_apply, lua_on_remove, lua_on_tick implementations
```

## VM Lifecycle

`Manager` holds `map[zoneID]*lua.LState`. On startup, `LoadZone(zoneID, scriptDir string, instLimit int)` creates a sandboxed VM, registers all engine modules, then `dofile`s every `*.lua` in the zone's script directory.

`CallHook(zoneID, hook string, args *lua.LTable) (lua.LValue, error)` looks up the VM, calls the named global function if it exists (no-op if absent), catches all errors, and always returns. A missing hook is never an error.

The instruction limit defaults to `DefaultInstructionLimit = 100_000`. The zone YAML's `script_instruction_limit` field overrides it if set.

## Sandbox

The sandbox strips all unsafe Lua standard libs on VM creation: `os`, `io`, `debug`, `dofile`, `loadfile`. `require` is replaced with a whitelist (empty for now). The instruction limit is enforced via GopherLua's `lua.SetMx`.

## Engine API Modules

All six modules are registered as Lua global tables. Exercised functions have real implementations; unexercised ones are explicit no-op stubs with `// TODO(stage7): implement`.

| Module | Exercised in Stage 6 | Stub only |
|--------|---------------------|-----------|
| `engine.log` | `debug`, `info`, `warn`, `error` | — |
| `engine.dice` | `roll(expr)` → `{total, dice, modifier}` | — |
| `engine.entity` | `get_hp`, `get_conditions`, `get_ac`, `get_name` | `set_attr`, `move` |
| `engine.combat` | `apply_condition`, `apply_damage`, `query_combatant` | `initiate`, `resolve_action` |
| `engine.world` | `broadcast`, `query_room` | `move_entity` |
| `engine.event` | — | `register_listener`, `emit`, `schedule` (all stubs) |

`engine.dice.roll` delegates to the `dice.Roller` injected at `Manager` construction.
`engine.log` delegates to the zone's `zap.Logger`.

## Hook Points & Call Conventions

### Condition Hooks

Called from `internal/game/condition` when `ActiveSet.Apply`, `Remove`, and `Tick` run. `ActiveSet` gains optional `scriptMgr *scripting.Manager` and `zoneID string` fields, set by `Combat` after `StartCombat`.

The function name comes from `ConditionDef.LuaOnApply` / `LuaOnRemove` / `LuaOnTick`. Empty string = no-op.

```lua
-- lua_on_apply value in condition YAML names this function
function dying_on_apply(target_uid, condition_id, stacks, duration) end

-- lua_on_remove value in condition YAML names this function
function dying_on_remove(target_uid, condition_id) end

-- lua_on_tick value in condition YAML names this function
function dying_on_tick(target_uid, condition_id, stacks, duration_remaining) end
```

### Combat Hooks

Called from `internal/game/combat/round.go` via `Combat.scriptMgr`. Hooks may return a modified value; if they return nothing the original Go value is used.

```lua
-- may return modified roll_total; nil = use original
function on_attack_roll(attacker_uid, target_uid, roll_total, ac) end

-- may return modified damage; nil = use original
function on_damage_roll(attacker_uid, target_uid, damage) end

-- may return false to cancel condition application; nil/true = allow
function on_condition_apply(target_uid, condition_id, stacks) end
```

### Room Hooks

Called from `internal/gameserver` move/look handlers. Fire-and-forget; no return value consumed.

```lua
function on_enter(uid, room_id, from_room_id) end
function on_exit(uid, room_id, to_room_id) end
function on_look(uid, room_id) end
```

## Integration Points

### Struct Changes

- `Combat` gains `scriptMgr *scripting.Manager` and `zoneID string` — set by `StartCombat`, threaded into `ResolveRound` and `StartRoundWithSrc`
- `ActiveSet` gains `scriptMgr *scripting.Manager` and `zoneID string` — set by `Combat.ApplyCondition`
- `GameServiceServer` gains `scriptMgr *scripting.Manager` — passed to `NewCombatHandler`; called directly in move/look handlers for room hooks
- `world.Zone` YAML gains `ScriptDir string` and `ScriptInstructionLimit int` fields

### Startup Sequence (`cmd/gameserver/main.go`)

1. Load zones (existing)
2. Load condition registry (existing)
3. Construct `scripting.Manager` with dice roller + logger
4. For each zone: `mgr.LoadZone(zone.ID, zone.ScriptDir, zone.ScriptInstructionLimit)`
5. Pass `mgr` to `NewCombatHandler` and `GameServiceServer`

### Zone YAML Addition

```yaml
script_dir: content/scripts/zones/downtown
script_instruction_limit: 50000  # optional; defaults to DefaultInstructionLimit (100_000)
```

## Error Handling

| Situation | Behavior |
|-----------|----------|
| Lua syntax/load error at startup | `logger.Fatal` — bad scripts block server startup |
| Lua runtime error during hook call | `logger.Warn` with zone ID, hook name, Lua traceback; original Go value used |
| Missing hook function | Silent no-op — scripts need not define every hook |
| VM not found for zone | `logger.Info` — zone may legitimately have no scripts |

## Testing Strategy

**`internal/scripting` (pure, no Docker):**
- Unit: `LoadZone` loads scripts; `CallHook` invokes correct function; missing hook = no-op; runtime error = warn + no panic
- Property (`rapid`): any hook returning nil never panics; instruction limit exceeded always returns error; sandbox escape attempts (calling `os.exit` etc.) always return error

**`internal/game/combat`:**
- Existing tests pass nil `scriptMgr` (no behavior change)
- New tests inject a test VM with known hook scripts: `on_attack_roll` returning a modified value; `on_condition_apply` returning false to cancel

**`internal/gameserver`:**
- Existing tests unaffected (nil manager = hooks skipped)
- New tests: move handler fires `on_exit`/`on_enter`; look handler fires `on_look`

## Requirements Covered

- SCRIPT-3: Instruction count limit per hook call
- SCRIPT-4: Per-zone configurable instruction limit with global default fallback
- SCRIPT-5: Deferred (hot-reload is Stage 7+)
- SCRIPT-19: Lua runtime errors never crash the server
- SCRIPT-20: Lua errors logged with file and line number
