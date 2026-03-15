# NPC Ability Triggers for Rage, Despair, Delirium Tracks — Design Spec

**Date:** 2026-03-14

---

## Goal

NPCs can inflict Rage, Despair, and Delirium mental state conditions on players during combat via dedicated HTN operators. Each ability costs AP, has a round-based cooldown, and delivers a narrative message drawn from the NPC's existing Taunts list.

---

## Feature 1: Data Model

### Operator struct extension (`internal/game/ai/domain.go`)

The existing `Operator` struct has three fields: `ID`, `Action`, `Target`. Add four new fields:

```go
// Track is the mental state track this operator targets.
// One of "rage", "despair", "delirium". Empty string means not a mental state ability.
Track string `yaml:"track,omitempty"`

// Severity is the minimum severity to apply: "mild", "moderate", or "severe".
Severity string `yaml:"severity,omitempty"`

// CooldownRounds is the number of rounds that must pass before this operator
// can be used again. Zero means no cooldown.
CooldownRounds int `yaml:"cooldown_rounds,omitempty"`

// APCost is the AP consumed when this operator executes. Zero means free.
APCost int `yaml:"ap_cost,omitempty"`
```

The new action string `"apply_mental_state"` is added alongside the existing values `"attack"`, `"strike"`, `"pass"`, `"flee"`.

### PlannedAction struct extension (`internal/game/ai/planner.go`)

The existing `PlannedAction` has two string fields: `Action` and `Target`. Add operator metadata so the execution path can access it without reverse lookup:

```go
// OperatorID is the ID of the operator that produced this action.
// Empty for legacy/fallback actions.
OperatorID string

// Track is the mental state track for "apply_mental_state" actions.
Track string

// Severity is the severity level for "apply_mental_state" actions.
Severity string

// CooldownRounds is the cooldown to set after execution.
CooldownRounds int

// APCost overrides the default AP budget for this action when non-zero.
APCost int
```

The planner copies these fields from the `Operator` when building a `PlannedAction`.

### NPC Instance extension (`internal/game/npc/instance.go`)

```go
// AbilityCooldowns maps operator ID → rounds remaining until the ability is usable again.
// Nil at spawn; initialized lazily on first write.
AbilityCooldowns map[string]int
```

### Combat struct extension (`internal/game/combat/engine.go`)

Add to the `Combat` struct:

```go
// DamageDealt maps combatant UID → total damage dealt this combat.
// Initialized in the Combat constructor. Reset to empty map at combat start.
DamageDealt map[string]int
```

Add a method on `Combat`:

```go
// RecordDamage increments the total damage recorded for the given attacker.
func (c *Combat) RecordDamage(attackerUID string, amount int) {
    c.DamageDealt[attackerUID] += amount
}
```

`RecordDamage` is called from the damage resolution callback in `resolveAndAdvanceLocked` alongside existing damage application, wherever the attacker UID and damage amount are both known.

`DamageDealt` is per-combat-instance; it is naturally reset when a new `Combat` is created for a room.

### YAML operator example

```yaml
operators:
  - id: ganger_taunt
    action: apply_mental_state
    track: rage
    severity: mild
    target: highest_damage_enemy
    cooldown_rounds: 3
    ap_cost: 1
```

---

## Feature 2: Combat Execution

**Location:** `internal/gameserver/combat_handler.go`.

### 2a: Cooldown decrement

At the start of `autoQueueNPCsLocked`, before calling `PlannerFor` or `legacyAutoQueueLocked`, decrement all cooldowns for each living NPC:

```
for each living NPC combatant cbt:
    inst := npcMgr.GetInstance(cbt.ID)
    for k := range inst.AbilityCooldowns:
        inst.AbilityCooldowns[k]--
        if inst.AbilityCooldowns[k] < 0:
            inst.AbilityCooldowns[k] = 0
```

### 2b: Planner extension

In `planner.go`, when building a `PlannedAction` from an `Operator`:

```go
pa := PlannedAction{
    Action:         op.Action,
    Target:         op.Target,
    OperatorID:     op.ID,
    Track:          op.Track,
    Severity:       op.Severity,
    CooldownRounds: op.CooldownRounds,
    APCost:         op.APCost,
}
```

### 2c: Cooldown gate in applyPlanLocked

In `applyPlanLocked`, before queuing an `apply_mental_state` action, check the cooldown in Go (no Lua involvement):

```
if pa.Action == "apply_mental_state":
    inst := npcMgr.GetInstance(actor.ID)
    if inst.AbilityCooldowns[pa.OperatorID] > 0:
        continue  // skip; still on cooldown
```

### 2d: apply_mental_state execution

Add a new case to the `applyPlanLocked` action switch alongside `attack`, `strike`, `pass`:

```
case "apply_mental_state":
    1. Resolve target player session by pa.Target selector:
       - "nearest_enemy"        → any living player combatant (first found)
       - "lowest_hp_enemy"      → living player with lowest CurrentHP
       - "highest_damage_enemy" → living player with highest DamageDealt entry

    2. If no valid target: skip (no AP deducted, no cooldown set). Continue to next action.

    3. Parse track and severity:
       track:    "rage"     → mentalstate.TrackRage
                 "despair"  → mentalstate.TrackDespair
                 "delirium" → mentalstate.TrackDelirium
       severity: "mild"     → mentalstate.SeverityMild
                 "moderate" → mentalstate.SeverityMod
                 "severe"   → mentalstate.SeveritySevere

    4. Call: changes := h.mentalStateMgr.ApplyTrigger(targetUID, track, severity)
       Apply returned state changes: h.applyMentalStateChanges(targetUID, changes)
       (This updates cbt.Conditions and generates any escalation narrative.)

    5. Send taunt message to target player's stream:
       - If inst.Taunts is non-empty: pick a random entry from inst.Taunts.
       - Otherwise: fmt.Sprintf("The %s unsettles you.", inst.Name())
       - If the target player session is no longer active (disconnected),
         skip the push silently.

    6. Set cooldown (lazy-initialize map if nil):
       if inst.AbilityCooldowns == nil:
           inst.AbilityCooldowns = make(map[string]int)
       inst.AbilityCooldowns[pa.OperatorID] = pa.CooldownRounds

    7. Deduct AP: use pa.APCost if non-zero, otherwise default to 1.
       Break if AP budget exhausted (same as all other actions).
```

### Lua preconditions

Each ability operator has a Lua precondition that checks only game-observable state (HP, enemy presence). Cooldown checking is done in Go (step 2c above); Lua does not need to know about cooldowns.

Example precondition for `ganger_taunt`:

```lua
function ganger_taunt_ready(uid)
    return engine.combat.enemy_count(uid) > 0
end
```

The precondition name follows the convention `<operator_id>_ready` and is registered in the zone's Lua script file alongside existing preconditions.

---

## Feature 3: NPC Ability Assignments

One ability operator per NPC. All mild abilities use `ap_cost: 1`; moderate use `ap_cost: 2`.

Each NPC's HTN domain YAML file receives a new operator entry under `operators:` and a corresponding `<operator_id>_ready` Lua precondition in the zone Lua script. The method that selects the ability is added under the existing `fight` task with lower priority than direct attack methods.

| NPC | Operator ID | Track | Severity | Target | Cooldown |
|-----|-------------|-------|----------|--------|----------|
| ganger | ganger_taunt | rage | mild | highest_damage_enemy | 3 |
| highway_bandit | bandit_intimidate | despair | mild | nearest_enemy | 3 |
| tarmac_raider | raider_unsettle | delirium | mild | nearest_enemy | 3 |
| mill_plain_thug | thug_taunt | rage | mild | highest_damage_enemy | 3 |
| motel_raider | motel_raider_intimidate | despair | mild | nearest_enemy | 3 |
| river_pirate | pirate_unsettle | delirium | mild | nearest_enemy | 4 |
| strip_mall_scav | scav_demoralize | despair | mild | lowest_hp_enemy | 3 |
| industrial_scav | iscav_demoralize | despair | mild | lowest_hp_enemy | 3 |
| outlet_scavenger | outlet_scav_confuse | delirium | mild | nearest_enemy | 4 |
| scavenger | scavenger_confuse | delirium | mild | nearest_enemy | 4 |
| alberta_drifter | drifter_taunt | rage | mild | nearest_enemy | 4 |
| terminal_squatter | squatter_demoralize | despair | mild | lowest_hp_enemy | 4 |
| cargo_cultist | cultist_unsettle | delirium | moderate | nearest_enemy | 5 |
| lieutenant | lt_intimidate | despair | moderate | lowest_hp_enemy | 4 |
| brew_warlord | warlord_enrage | rage | moderate | highest_damage_enemy | 4 |
| gravel_pit_boss | pitboss_demoralize | despair | moderate | lowest_hp_enemy | 4 |
| commissar | commissar_taunt | rage | moderate | highest_damage_enemy | 4 |
| bridge_troll | troll_unsettle | delirium | moderate | lowest_hp_enemy | 5 |

Note: `bridge_troll` uses `lowest_hp_enemy` with delirium by design — the troll focuses its disorienting attacks on the most vulnerable target.

NPCs without an `AIDomain` field use `legacyAutoQueueLocked` and never reach `applyPlanLocked`; they do not gain ability operators and require no changes.

---

## Testing

- **REQ-T1** (example): Operator `apply_mental_state` with cooldown=0 → `ApplyTrigger` called on target; `AbilityCooldowns[id]` set to `CooldownRounds`; `applyMentalStateChanges` called with returned StateChanges.
- **REQ-T2** (example): Operator with `AbilityCooldowns[id] > 0` → skipped in `applyPlanLocked`; no trigger applied; cooldown unchanged.
- **REQ-T3** (example): After N rounds equal to `CooldownRounds` (i.e., N decrements), `AbilityCooldowns[id]` reaches 0; operator executes on next eligible round.
- **REQ-T4** (example): Target selector `highest_damage_enemy` returns the player UID with the highest `DamageDealt` entry in the `Combat` struct.
- **REQ-T5** (example): Target selector `lowest_hp_enemy` returns the living player with the lowest `CurrentHP`.
- **REQ-T6** (example): NPC with non-empty `Taunts` → a taunt string is pushed to the targeted player's stream on ability use.
- **REQ-T7** (example): NPC with empty `Taunts` → fallback message `"The <name> unsettles you."` pushed to targeted player.
- **REQ-T8** (example): `apply_mental_state` with track=rage, severity=moderate → `applyMentalStateChanges` is called; target's rage condition is at ≥ moderate in `cbt.Conditions`.
- **REQ-T9** (property): For any track ∈ {rage, despair, delirium} and severity ∈ {mild, moderate, severe}, and any initial track state, executing `apply_mental_state` via the full combat execution path leaves the severity in `cbt.Conditions` consistent with `mentalstate.Manager` (no divergence between the two state stores).
- **REQ-T10** (example): Dead NPC combatant — `autoQueueNPCsLocked` skips dead NPCs; no ability fires.
- **REQ-T11** (example): No valid target (all players dead or session inactive) → ability silently skipped; no AP deducted; no cooldown set; no push attempted.
- **REQ-T12** (example): `AbilityCooldowns` nil at first use → map initialized before write; no panic.
- **REQ-T13** (example): `RecordDamage` accumulates across multiple hits; `highest_damage_enemy` correctly identifies player with total highest damage after 3 rounds.
