# Combat Stage 5 — Conditions System Design

**Date:** 2026-02-23
**Status:** Approved

## Goal

Add a YAML-defined conditions system with duration tracking, roll modifiers, action restrictions, and the full PF2E dying/wounded chain to the combat engine.

## Architecture

`internal/game/condition` is a pure data package — no knowledge of `combat`. It owns YAML loading, a condition registry, and `ActiveSet` per combatant. The `Combat` struct holds `Conditions map[string]*condition.ActiveSet`. The combat engine calls into `ActiveSet` for modifiers and lifecycle. No circular imports.

Lua hook fields (`lua_on_apply`, `lua_on_remove`, `lua_on_tick`) are parsed and stored but ignored until Stage 6 wires the VM.

## Package Structure

```
internal/game/condition/
├── definition.go       — ConditionDef, Registry, YAML loader
├── active.go           — ActiveCondition, ActiveSet
├── modifiers.go        — AttackBonus(), ACBonus(), SpeedPenalty(), RestrictedActions()
├── definition_test.go
├── active_test.go
└── modifiers_test.go

content/conditions/
├── dying.yaml
├── wounded.yaml
├── unconscious.yaml
├── stunned.yaml
├── frightened.yaml
├── prone.yaml
└── flat_footed.yaml
```

## Data Model

```go
type ConditionDef struct {
    ID              string
    Name            string
    Description     string
    DurationType    string   // "rounds" | "until_save" | "permanent"
    MaxStacks       int      // 0 = no stacking; dying has MaxStacks=4
    AttackPenalty   int
    ACPenalty       int
    SpeedPenalty    int
    RestrictActions []string // action types blocked (e.g. ["attack","strike"] for stunned)
    LuaOnApply      string   // stored, ignored until Stage 6
    LuaOnRemove     string
    LuaOnTick       string
}

type ActiveCondition struct {
    Def               *ConditionDef
    Stacks            int
    DurationRemaining int   // rounds left; -1 = permanent
}

type ActiveSet struct { /* map[conditionID]*ActiveCondition */ }
func (s *ActiveSet) Apply(def *ConditionDef, stacks, duration int) error
func (s *ActiveSet) Remove(id string)
func (s *ActiveSet) Tick() []string           // decrements durations; returns expired IDs
func (s *ActiveSet) Has(id string) bool
func (s *ActiveSet) Stacks(id string) int
func (s *ActiveSet) All() []*ActiveCondition  // snapshot for status display
```

## Starter Conditions

| ID | MaxStacks | DurationType | Effect |
|----|-----------|--------------|--------|
| `dying` | 4 | until_save | no actions; DC 15 flat check recovery each round; dying 4 = death |
| `wounded` | 3 | permanent | raises future dying entry stacks by current stacks |
| `unconscious` | 0 | permanent | no actions; cleared after combat |
| `stunned` | 3 | rounds | lose stacks AP from action queue each round |
| `frightened` | 4 | rounds | –stacks to attack rolls and AC |
| `prone` | 0 | permanent | –2 attack; +2 AC vs ranged, –2 AC vs melee; stand costs 1 AP |
| `flat_footed` | 0 | rounds | –2 AC |

## Combat Engine Integration

### `Combat` struct additions
```go
Conditions map[string]*condition.ActiveSet  // keyed by combatant UID
```

### New engine methods
```go
func (c *Combat) ApplyCondition(uid, conditionID string, stacks, duration int) error
func (c *Combat) RemoveCondition(uid, conditionID string)
func (c *Combat) GetConditions(uid string) []*condition.ActiveCondition
```

### Round lifecycle

**`StartRound()`:**
1. For each living combatant: `Conditions[uid].Tick()` → collect expired IDs → broadcast removals
2. For combatants with `stunned`: reduce ActionQueue AP by stacks (minimum 0)
3. For combatants with `dying`: roll flat check (d20, no modifiers) vs DC 15
   - ≥ 25 (crit success): remove dying, set HP = 1
   - ≥ 15 (success): remove dying, apply `wounded +1`, set HP = 1
   - < 15 (failure): advance dying stacks by 1
   - dying stacks == 4: mark combatant dead, broadcast death

**`ResolveRound()` (in `round.go`):**
- Before each attack: subtract `Conditions[attacker].AttackBonus()` from `atkTotal`; add `Conditions[target].ACBonus()` to effective AC
- After crit failure: `ApplyCondition(attacker, "prone", 1, -1)`
- After crit success: `ApplyCondition(target, "flat_footed", 1, 1)`
- After target HP hits 0:
  - `dying_stacks := 1 + Conditions[target].Stacks("wounded")`
  - `ApplyCondition(target, "dying", dying_stacks, -1)`
  - Do NOT remove combatant from combat (dying entities remain and make recovery checks)

### New command: `status`
Queries `GetConditions` for the requesting player UID; renders active conditions with stacks and duration remaining.

## Proto Additions

```protobuf
// ClientMessage oneof additions
StatusRequest status = 15;

// ServerEvent oneof additions
ConditionEvent condition_event = 14;

// New messages
message StatusRequest {}

message ConditionEvent {
  string target_uid    = 1;
  string target_name   = 2;
  string condition_id  = 3;
  string condition_name = 4;
  int32  stacks        = 5;
  bool   applied       = 6;  // true=applied, false=removed
}

message ConditionInfo {
  string id                 = 1;
  string name               = 2;
  int32  stacks             = 3;
  int32  duration_remaining = 4;  // -1 = permanent
}

// Extend RoomView
repeated ConditionInfo active_conditions = 6;
```

## Frontend Additions

- `status` / `st` command → `StatusRequest`
- `ServerEvent_ConditionEvent` → `RenderConditionEvent`
  - Applied: `"[CONDITION] Yusuf is now Prone."`
  - Removed: `"[CONDITION] Prone fades from Yusuf."`
- `RenderStatus`: bulleted list of active conditions with stacks and duration
- `RenderCombatEvent` updated: dying recovery narrative added
- `showGameHelp`: `status` listed under system commands

## Testing Strategy

**`internal/game/condition` (pure, no Docker):**
- Unit: `Apply` stacks, `Remove`, `Tick` decrement/expiry, accessors
- Property (`rapid`): `Tick` N times → `DurationRemaining` never < -1; apply+remove → `Has` false; `AttackBonus` always ≤ 0; `ACBonus` always ≤ 0

**`internal/game/combat`:**
- Unit: crit failure → prone; crit success → flat-footed; 0 HP → dying stacks = 1 + wounded; dying 4 → dead
- Dying recovery: all three check outcomes via controlled `src.Intn`
- `StartRound` ticks conditions, applies stunned AP reduction
- Property (`rapid`): dying stacks never > 4; wounded stacks never > 3; HP never negative

**`internal/gameserver`:**
- `combat_handler_test.go`: crit failure → `ConditionEvent{applied:true, condition_id:"prone"}`; status request returns player's conditions

**`internal/frontend/handlers`:**
- `text_renderer_test.go`: `RenderConditionEvent` applied/removed; `RenderStatus` multiple conditions

## Requirements Covered

- COMBAT-15: Conditions applied/removed with duration tracking
- COMBAT-16: Condition modifiers factored into attack/AC calculations
