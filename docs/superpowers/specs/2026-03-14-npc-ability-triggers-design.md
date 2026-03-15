# NPC Ability Triggers for Rage, Despair, Delirium Tracks — Design Spec

**Date:** 2026-03-14

---

## Goal

NPCs can inflict Rage, Despair, and Delirium mental state conditions on players during combat via dedicated HTN operators. Each ability costs AP, has a round-based cooldown, and delivers a narrative message drawn from the NPC's existing Taunts list.

---

## Feature 1: Data Model

### Operator struct extension (`internal/game/ai/domain.go`)

Add fields to the existing `Operator` struct:

```go
// Track is the mental state track this operator targets.
// One of "rage", "despair", "delirium". Empty string means not a mental state ability.
Track string `yaml:"track,omitempty"`

// Severity is the minimum severity to apply: "mild", "moderate", or "severe".
Severity string `yaml:"severity,omitempty"`

// CooldownRounds is the number of rounds that must pass before this operator
// can be used again. Zero means no cooldown.
CooldownRounds int `yaml:"cooldown_rounds,omitempty"`
```

The existing `Operator.Action` field uses the new value `"apply_mental_state"` to identify mental state abilities. The existing `Operator.APCost` field (already present) controls AP cost.

### NPC Instance extension (`internal/game/npc/instance.go`)

```go
// AbilityCooldowns maps operator ID → rounds remaining until the ability is usable again.
// Nil map means no cooldowns active (treated as all zero).
AbilityCooldowns map[string]int
```

Cooldowns are decremented once per round during the existing per-round NPC processing in `combat_handler.go`.

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

**Location:** `internal/gameserver/combat_handler.go`, in the NPC action execution path where `attack`, `strike`, `pass`, and `reload` are handled.

**Precondition:** The HTN planner has selected an operator with `action == "apply_mental_state"`. The planner's Lua precondition has already verified the target exists and the cooldown is zero.

**Algorithm:**

```
1. Resolve target player session by target selector:
   - "nearest_enemy"         → any living player combatant in the room (first found)
   - "lowest_hp_enemy"       → living player with lowest CurrentHP
   - "highest_damage_enemy"  → living player who dealt the most total damage this combat

2. If no valid target found: skip (no AP deducted, no cooldown set).

3. Parse track and severity from operator fields:
   - track: "rage" → mentalstate.TrackRage
              "despair" → mentalstate.TrackDespair
              "delirium" → mentalstate.TrackDelirium
   - severity: "mild" → mentalstate.SeverityMild
               "moderate" → mentalstate.SeverityMod
               "severe" → mentalstate.SeveritySevere

4. Call mentalStateMgr.ApplyTrigger(targetUID, track, severity).

5. Compose message:
   - If inst.Taunts is non-empty: pick a random entry from inst.Taunts.
   - Otherwise: fmt.Sprintf("The %s unsettles you.", inst.Name())
   - Push message to target player's stream.

6. Set inst.AbilityCooldowns[operator.ID] = operator.CooldownRounds.

7. Deduct operator.APCost from NPC's AP budget (same as all other operators).
```

**Cooldown decrement:** Each round, after NPC actions resolve, decrement all values in `inst.AbilityCooldowns` by 1, with a floor of 0.

### Lua precondition pattern

Each ability operator has a corresponding Lua precondition function registered in the zone's Lua VM. The precondition receives the NPC UID and returns true only when:
- At least one living player enemy exists in the room.
- `inst.AbilityCooldowns[operatorID] == 0`.

The HTN planner reads cooldown state from the NPC instance before passing it to Lua evaluation. A helper `npcCooldownReady(uid, operatorID string) bool` is exposed to the Lua VM via the existing Lua host binding mechanism.

---

## Feature 3: NPC Ability Assignments

One ability per NPC. All abilities use `ap_cost: 1` for mild, `ap_cost: 2` for moderate.

| NPC | Operator ID | Track | Severity | Target | Cooldown Rounds |
|-----|-------------|-------|----------|--------|-----------------|
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

Each NPC's HTN domain YAML file receives a new operator entry and a corresponding Lua precondition function. The method that selects the ability is added to the existing `fight` task with lower priority than direct attacks (ability fires when attack preconditions fail or as an alternative action).

---

## Feature 4: Highest-Damage Target Tracking

To support the `highest_damage_enemy` target selector, track cumulative damage dealt per player per combat instance.

**Location:** `internal/game/combat/combat.go` (or the combat engine state struct).

```go
// DamageDealt maps combatant UID → total damage dealt this combat.
DamageDealt map[string]int
```

Updated whenever damage is applied to any combatant. The combat handler reads this map when resolving `highest_damage_enemy`.

---

## Testing

- **REQ-T1** (example): Operator with `action: apply_mental_state`, cooldown=0 → `ApplyTrigger` called on target; `AbilityCooldowns[id]` set to `CooldownRounds`.
- **REQ-T2** (example): Operator with `AbilityCooldowns[id] > 0` → planner skips operator; no trigger applied; cooldown unchanged.
- **REQ-T3** (example): After N rounds equal to `CooldownRounds`, `AbilityCooldowns[id]` reaches 0; operator becomes eligible again.
- **REQ-T4** (example): Target selector `highest_damage_enemy` returns the player UID with the highest entry in `DamageDealt`.
- **REQ-T5** (example): Target selector `lowest_hp_enemy` returns the living player with the lowest `CurrentHP`.
- **REQ-T6** (example): NPC with non-empty `Taunts` → a taunt string is sent to the targeted player's stream on ability use.
- **REQ-T7** (example): NPC with empty `Taunts` → fallback message `"The <name> unsettles you."` sent to targeted player.
- **REQ-T8** (example): `apply_mental_state` with track=rage, severity=moderate → target session has rage track at ≥ moderate after execution.
- **REQ-T9** (property): For any track ∈ {rage, despair, delirium} and severity ∈ {mild, moderate, severe}, `ApplyTrigger` never sets the track severity below its current level.
- **REQ-T10** (example): Dead NPC does not execute ability operators (existing dead-NPC skip logic covers this).
- **REQ-T11** (example): No valid target found (all players dead or absent) → ability silently skipped; no AP deducted; no cooldown set.
