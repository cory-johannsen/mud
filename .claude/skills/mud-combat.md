---
name: mud-combat
description: PF2E combat engine — actions, rounds, initiative, conditions, degree-of-success
type: reference
---

## Trigger

Invoke this skill when:
- Implementing or modifying a combat action (Attack, Strike, Reload, FireBurst, FireAutomatic, Throw, UseAbility, Stride, Pass)
- Adding a new condition type
- Debugging combat round resolution, AP tracking, or initiative order
- Understanding how CombatEvent protos are produced and broadcast
- Tracing the full request path from a player's Attack command to the resulting events

## Responsibility Boundary

- `internal/game/combat/` — pure combat logic (model, action resolution, round loop); no I/O, no gRPC, no database access
  - `combat.go` — Combatant struct, Outcome enum, OutcomeFor, proficiency/ability math helpers
  - `engine.go` — Combat struct, Engine (room-keyed map), StartCombat/EndCombat/AddCombatant, StartRound, condition lifecycle
  - `action.go` — ActionType enum, QueuedAction, ActionQueue, AP deduction/enqueue logic
  - `resolver.go` — ResolveAttack, ResolveFirearmAttack, ResolveSave, ResolveExplosive
  - `initiative.go` — RollInitiative, InitiativeBonusForMargin
- `internal/gameserver/combat_handler.go` — imperative shell: receives gRPC requests, drives the combat engine, emits CombatEvent protos, manages round timers, broadcasts events to all room occupants
- `internal/game/condition/` — condition types (ConditionDef YAML), active condition tracking (ActiveSet), and modifier helpers (AttackBonus, ACBonus, APReduction, etc.)

## Key Files

| File | Purpose |
|------|---------|
| `internal/game/combat/engine.go` | Combat and Engine structs; StartCombat, StartRound, dying recovery |
| `internal/game/combat/combat.go` | Combatant struct; Outcome enum; OutcomeFor; proficiency math |
| `internal/game/combat/action.go` | ActionType; ActionQueue; AP tracking |
| `internal/game/combat/resolver.go` | Attack/save/explosive roll resolution |
| `internal/game/combat/initiative.go` | RollInitiative; InitiativeBonusForMargin |
| `internal/gameserver/combat_handler.go` | CombatHandler; Attack/Strike/Pass/Flee entry points; round timer; broadcast |
| `internal/game/condition/definition.go` | ConditionDef YAML schema; Registry; LoadDirectory |
| `internal/game/condition/active.go` | ActiveCondition; ActiveSet; Apply/Remove/Tick; Lua hooks |
| `internal/game/condition/modifiers.go` | AttackBonus, ACBonus, APReduction, StunnedAPReduction, DamageBonus, etc. |

## Core Data Structures

### Combat (`engine.go`)
```go
type Combat struct {
    RoomID       string
    Combatants   []*Combatant           // initiative-ordered; highest first
    turnIndex    int
    Over         bool
    Participants []string               // player UIDs ever active; used for XP/loot
    Round        int                    // starts at 0; incremented by StartRound
    ActionQueues map[string]*ActionQueue // combatant UID → queue for current round
    Conditions   map[string]*condition.ActiveSet // combatant UID → active conditions
    DamageDealt  map[string]int         // combatant UID → total damage dealt
    condRegistry *condition.Registry
    scriptMgr    *scripting.Manager     // nil = no Lua hooks
    zoneID       string
    invRegistry  *inventory.Registry
    sessionGetter func(uid string) (*session.PlayerSession, bool)
}
```

### Combatant (`combat.go`)
Key fields: `ID`, `Kind` (KindPlayer/KindNPC), `Name`, `MaxHP`, `CurrentHP`, `AC`, `Level`, `StrMod`, `DexMod`, `Initiative`, `InitiativeBonus`, `Dead`, `Loadout`, `WeaponProficiencyRank`, `WeaponDamageType`, `Resistances`, `Weaknesses`, `GritMod`/`QuicknessMod`/`SavvyMod`, `ToughnessRank`/`HustleRank`/`CoolRank`, `ACMod`, `AttackMod`, `Hidden`, `Position`, `CoverTier`.

### ActionType constants (`action.go`)

Player-queueable actions (each has an AP cost via `Cost()`):
- `ActionAttack` — melee attack (1 AP)
- `ActionStrike` — ranged/firearm attack (1 AP)
- `ActionReload` — reload firearm (1 AP)
- `ActionFireBurst` — burst-fire (2 AP)
- `ActionFireAutomatic` — automatic fire (3 AP)
- `ActionThrow` — thrown weapon/grenade (1 AP)
- `ActionUseAbility` — ability activation; cost from `QueuedAction.AbilityCost`
- `ActionStride` — move action (1 AP)
- `ActionPass` — forfeit remaining AP; marks queue as submitted

Informational-only constants (no AP cost, not queued by players — appear in event logs only):
- `ActionCoverHit` — emitted when an attack hits cover instead of the target
- `ActionCoverDestroy` — emitted when cover is destroyed by an attack

### ActionQueue (`action.go`)
```go
type ActionQueue struct {
    UID       string
    MaxPoints int
    remaining int
    actions   []QueuedAction
}
```
`Enqueue` validates AP; `ActionPass` drains remaining to 0. `IsSubmitted()` = remaining==0 or Pass queued.

### AttackResult (`resolver.go`)
```go
type AttackResult struct {
    AttackerID, TargetID string
    AttackRoll, AttackTotal int
    Outcome    Outcome       // CritSuccess/Success/Failure/CritFailure
    BaseDamage int
    DamageRoll []int
    DamageType, WeaponName string
}
```
`EffectiveDamage()` returns `BaseDamage*2` on CritSuccess, `BaseDamage` on Success, `0` otherwise.

### Outcome / 4-tier (`combat.go`)
```go
const (
    CritSuccess Outcome = iota  // roll >= AC+10
    Success                      // roll >= AC
    Failure                      // roll >= AC-10
    CritFailure                  // roll < AC-10
)
```
`OutcomeFor(roll, ac int) Outcome` is the canonical comparison; used by both attack and save resolution.

### ConditionDef (`condition/definition.go`)
YAML-defined fields: `id`, `name`, `description`, `duration_type` ("rounds"|"until_save"|"permanent"), `max_stacks`, `attack_penalty`, `ac_penalty`, `ap_reduction`, `skip_turn`, `forced_action`, `restrict_actions`, `lua_on_apply`, `lua_on_remove`, `lua_on_tick`.

### ActiveCondition / ActiveSet (`condition/active.go`)
```go
type ActiveCondition struct {
    Def               *ConditionDef
    Stacks            int
    DurationRemaining int // -1 = permanent/until_save
}
```
`ActiveSet.Apply` stacks or updates duration; `Tick` decrements round-based durations and returns expired IDs; `Remove` deletes and fires Lua hook.

## Primary Data Flow

1. **Player sends Attack proto** → `handleAttack` in `grpc_service.go` dispatches to `CombatHandler.Attack(uid, target)`.
2. **No active combat**: `startCombatLocked` builds `[]*Combatant` from the player session and the NPC instance, calls `RollInitiative`, then `engine.StartCombat(roomID, combatants, condRegistry, scriptMgr, zoneID)`. Initial `CombatEvent`s (initiative narrative) are returned.
3. **ActionQueue setup**: `engine.StartCombat` sorts combatants by initiative descending; `Combat.StartRound(3)` is called to reset `ActionQueues` with 3 AP each (reduced by stunned/AP-reduction conditions).
4. **Action queued**: `cbt.QueueAction(uid, QueuedAction{Type: ActionAttack, Target: npcName})` decrements remaining AP by 1.
5. **Round resolution** (triggered when `AllActionsSubmitted()` or round timer fires): for each combatant in initiative order, execute queued actions:
   - `ResolveAttack(attacker, target, src)` → `AttackResult` using `OutcomeFor(d20 + StrMod + profBonus + AttackMod, target.AC + target.ACMod)`.
   - `EffectiveDamage()` applies outcome multiplier (×2 on crit, ×0 on miss).
   - Damage is reduced by target `Resistances[damageType]` and increased by target `Weaknesses[damageType]`.
   - `Combat.RecordDamage(attackerUID, amount)` tracks cumulative damage dealt.
6. **Degree of success** drives outcome text and condition application: crit success may apply `prone` or `grabbed`; critical failure may apply conditions to the attacker.
7. **Condition apply**: `Combat.ApplyCondition(uid, condID, stacks, duration)` looks up the def from `condRegistry`, then calls `ActiveSet.Apply`. Conditions are NEVER set directly on the struct.
8. **Dying recovery** (at `StartRound`): combatants with `dying` condition make a DC 15 flat check (d20); natural 20 = crit success (remove dying, restore to 1 HP); 15–19 = success (remove dying, apply wounded, restore to 1 HP); below 15 = advance dying stacks; dying ≥ 4 = permanent death.
9. **Emit CombatEvent proto**: `broadcastFn(roomID, events)` sends narrative text, HP updates, and condition changes to all room occupants.
10. **Round ends**: when all ActionQueues are submitted → `RoundEndEvent` proto emitted; `StartRound(3)` resets queues; repeat until `!cbt.HasLivingNPCs()` or `!cbt.HasLivingPlayers()`, then `engine.EndCombat(roomID)`.

## Invariants & Contracts

- PF2E 4-tier outcomes ALWAYS apply: every attack and save produces exactly one of CritSuccess, Success, Failure, or CritFailure via `OutcomeFor`. There is no binary hit/miss.
- Each living combatant receives exactly 3 AP per round, reduced by `condition.StunnedAPReduction(s) + condition.APReduction(s)`, floored at 0.
- Conditions are applied via `Combat.ApplyCondition` or `ActiveSet.Apply` — NEVER by direct struct field assignment on Combatant.
- `ActionQueue.remaining` is NEVER negative (invariant enforced by `Enqueue` and `DeductAP`).
- `IsDead()` for NPCs: `CurrentHP <= 0`. For players: `Dead == true` (only set when dying stack reaches 4).
- `Combat.Conditions` is initialized in `StartCombat` and `AddCombatant` for every combatant; no combatant may be present without a corresponding `ActiveSet`.
- `Engine.StartCombat` returns an error if combat is already active in the room; callers must check.
- `StartRound` resets `ACMod` and `AttackMod` to 0 at the start of each round (per-round modifiers do not carry over).

## Extension Points

### How to add a new combat action

Adding a new combat action requires ALL of the following steps (CMD-1 through CMD-7):

- CMD-1: Add a `Handler<Name>` constant to `internal/game/command/commands.go`.
- CMD-2: Append a `Command{...}` entry referencing the new constant to `BuiltinCommands()` in `internal/game/command/commands.go`.
- CMD-3: Implement a `Handle<Name>` function in `internal/game/command/<name>.go` with full TDD coverage (property-based tests required).
- CMD-4: Add a proto request message to `api/proto/game/v1/game.proto` and add it to the `ClientMessage` oneof. Run `make proto` to regenerate.
- CMD-5: Add a `bridge<Name>` function to `internal/frontend/handlers/bridge_handlers.go` and register it in `bridgeHandlerMap`. Verify `TestAllCommandHandlersAreWired` passes.
- CMD-6: Implement a `handle<Name>` function in `internal/gameserver/grpc_service.go` and wire it into the `dispatch` type switch. This function MUST call the appropriate `CombatHandler` method.
- CMD-7: For the combat engine itself — add the `ActionType` constant and `Cost()` case in `action.go`, add resolution logic in `resolver.go` or a new file, and wire into the round-resolution loop in `combat_handler.go`. All tests MUST pass before the command is considered done.

### How to add a new condition YAML

1. Create a new file in the conditions data directory (e.g., `data/conditions/<name>.yaml`).
2. Define the YAML fields: `id` (unique slug), `name`, `description`, `duration_type` ("rounds"|"until_save"|"permanent"), `max_stacks` (0 = unstackable), and any applicable modifier fields (`attack_penalty`, `ac_penalty`, `ap_reduction`, `skip_turn`, `restrict_actions`, `forced_action`, `lua_on_apply`, `lua_on_remove`, `lua_on_tick`).
3. `condition.LoadDirectory` will automatically pick up the new file on next startup; no code changes required for basic conditions.
4. If the condition requires special mechanical behavior beyond the existing modifier fields, add a new helper function to `internal/game/condition/modifiers.go` and call it from the combat round resolution loop in `combat_handler.go`.
5. Write property-based tests for the new condition's mechanics (SWENG-5a).

## Common Pitfalls

- **Direct condition mutation**: never set conditions by writing to `Combatant` fields directly. Always use `Combat.ApplyCondition` or `ActiveSet.Apply` so that Lua hooks and stack logic are respected.
- **Missing AP cost**: `ActionUseAbility` ignores `ActionType.Cost()` — the cost comes from `QueuedAction.AbilityCost`. Forgetting to set `AbilityCost` results in a free action.
- **Round timer race**: `CombatHandler` uses `combatMu` to serialize all access to combat state. Code that touches `Combat` or `ActionQueue` outside the handler must acquire this lock.
- **IsDead divergence**: NPC death and player death have different semantics. NPCs die at `CurrentHP <= 0`; players enter the `dying` condition chain and only die when `Dead == true`. Calling `IsDead()` handles both cases correctly; checking `CurrentHP == 0` alone is wrong for players.
- **Initiative bonus scope**: `InitiativeBonus` is only set for players who beat all NPCs. It is NOT applied automatically — `combat_handler.go` is responsible for propagating it to `AttackMod`/`ACMod` each round.
- **Condition duration -1**: a `DurationRemaining` of -1 means permanent or until_save. `Tick` skips these. Passing 0 would cause immediate expiry.
- **ResolveFirearmAttack vs ResolveAttack**: firearm attacks use `DexMod` and weapon `DamageDice`; melee attacks use `StrMod` and a 1d6 baseline. Use the wrong resolver and modifiers will be incorrect.
- **sortByInitiativeDesc tie-breaking**: `sortByInitiativeDesc` uses a stable insertion sort; equal initiative preserves insertion order. This behavior is untested — if any re-sort elsewhere uses `>=` instead of `>` as the comparison, ties will silently break in a different order.
