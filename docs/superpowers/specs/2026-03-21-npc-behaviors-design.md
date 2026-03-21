# NPC Behaviors Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `npc-behaviors` (priority 320)
**Dependencies:** `non-combat-npcs`, `persistent-calendar`

---

## Overview

Adds per-NPC custom behaviors across five areas: HTN-driven dialog, threat-assessment aggressiveness, time-of-day schedules, expanded combat AI operators, and map movement fencing. All behavior is expressed through the existing HTN planning system wherever possible. The template+instance separation is preserved throughout.

---

## 1. Dialog via HTN

All NPC speech is unified under a new `say` HTN operator. The existing dedicated taunt subsystem is removed and replaced by HTN domain entries.

### 1.1 The `say` Operator

The `say` operator selects a random string from its `strings` parameter list and broadcasts it to the room as an NPC speech event. Cooldown is enforced via the existing `AbilityCooldowns` map on `Instance`, keyed by operator ID.

Operator parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `strings` | `[]string` | Pool of lines; one is chosen at random |
| `cooldown` | `string` | Go duration string (e.g., `"30s"`); parsed via `time.ParseDuration` |

### 1.2 New HTN Preconditions

Four new preconditions are added to the HTN evaluator to gate dialog context:

| Precondition | Type | Description |
|-------------|------|-------------|
| `in_combat` | `bool` | True when NPC is an active participant in a combat round |
| `player_entered_room` | `bool` | True for one tick after a hostile player enters the NPC's room |
| `hp_pct_below` | `int` | True when NPC current HP% is below the specified value |
| `on_damage_taken` | `bool` | True for one tick in the round the NPC received damage |

### 1.3 Dialog Context Mapping

| Dialog Context | Preconditions | Replaces |
|----------------|--------------|---------|
| Combat taunt | `in_combat: true` | `Taunts` / `TauntChance` / `TauntCooldown` |
| Idle statement | `in_combat: false` | — (new) |
| Challenge statement | `player_entered_room: true` | — (new) |
| Reaction statement | `on_damage_taken: true` | — (new) |
| Desperation | `hp_pct_below: 25` | — (new) |

### 1.4 Template and Instance Migration

Existing NPC YAML templates that use `taunts`, `taunt_chance`, and `taunt_cooldown` fields are migrated to HTN domain entries. Templates with no `ai_domain` and existing taunts receive a minimal generated domain containing a single `say` task with `in_combat: true` precondition.

Existing NPC YAML templates with no `ai_domain` and existing taunts receive a minimal generated domain containing a single `say` task with `in_combat: true` precondition.

- REQ-NB-1: The `say` HTN operator MUST select a random string from its `strings` parameter and broadcast it to the NPC's room as a speech event.
- REQ-NB-2: The `say` operator MUST enforce its `cooldown` parameter via `Instance.AbilityCooldowns` keyed by operator ID. The `cooldown` value MUST be a Go duration string parsed via `time.ParseDuration`.
- REQ-NB-3: The HTN evaluator MUST support the `in_combat`, `player_entered_room`, `hp_pct_below`, and `on_damage_taken` preconditions.
- REQ-NB-4: The `player_entered_room` and `on_damage_taken` preconditions MUST be true for exactly one evaluation tick after the triggering event.
- REQ-NB-5: All existing NPC templates using `taunts`/`taunt_chance`/`taunt_cooldown` MUST be migrated to `say` operator HTN domain entries.
- REQ-NB-6: `Template.Taunts`, `Template.TauntChance`, `Template.TauntCooldown`, `Instance.Taunts`, `Instance.TauntChance`, `Instance.TauntCooldown`, `Instance.LastTauntTime`, and `Instance.TryTaunt()` MUST be removed. `Template.Validate()` MUST be updated to remove the corresponding checks.

---

## 2. Aggressiveness Model

Replaces binary hostile/neutral disposition with threat-assessment-driven engagement decisions.

### 2.1 Threat Assessment

Threat assessment fires when a hostile-disposition NPC shares a room with one or more players (on player room entry and on each idle tick while not in combat).

Threat score formula:

```
threat_score = (player_avg_level - npc_level)
             + (party_size - 1) * 2
             - floor((1.0 - player_avg_hp_pct) * 3)
```

Where `player_avg_hp_pct` is a `float64` in `[0.0, 1.0]` computed as `sum(currentHP) / sum(maxHP)` across all players in the encounter. `floor()` truncates toward negative infinity (Go `math.Floor`).

A positive score means the players are stronger than the NPC; a negative score means the NPC is stronger. When `threat_score > courage_threshold`, the NPC does not engage. When `threat_score ≤ courage_threshold`, the NPC engages.

### 2.2 New Template Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `courage_threshold` | `int` | `999` | Threat score above which the NPC will not engage. Default 999 preserves current always-engage behavior for all existing templates. |
| `flee_hp_pct` | `int` | `0` | HP percentage below which the NPC attempts to flee combat. 0 = never flee. |

### 2.3 Coward Behavior

- REQ-NB-7A: When threat assessment determines the NPC should not engage and the NPC is not in combat, the NPC MUST remain passive and NOT initiate combat.
- REQ-NB-7B: When threat assessment determines the NPC should not engage and the NPC is already in combat and `flee_hp_pct > 0` and the threshold is met, the NPC MUST execute the `flee` operator (Section 4.1).
- REQ-NB-7C: When the NPC is already in combat and no flee threshold applies, engagement MUST NOT be re-evaluated mid-combat; the NPC continues fighting.

### 2.4 Flee HP Threshold

`flee_hp_pct` is evaluated at the end of each round in which the NPC took damage. If `currentHP * 100 / maxHP < flee_hp_pct`, the NPC queues the `flee` operator for its next turn.

### 2.5 Grudge System

`Instance` gains a `GrudgePlayerID string` field, cleared on respawn. When an NPC takes damage, `GrudgePlayerID` is set to the attacking player's ID. If multiple players deal damage in the same round, the last damage event in round-processing order wins (last-hit-wins). A new HTN precondition `has_grudge_target: true` gates grudge-specific domain tasks that target `GrudgePlayerID` preferentially.

- REQ-NB-7: Threat assessment MUST fire on each idle tick when a hostile-disposition NPC shares a room with one or more players and is not in combat.
- REQ-NB-8: Threat assessment MUST fire when a player enters a room containing a hostile-disposition NPC not in combat.
- REQ-NB-9: The threat score formula MUST incorporate player average level, NPC level, party size, and player average HP% as a `float64` in `[0.0, 1.0]`.
- REQ-NB-10: `Template.CourageThreshold` MUST default to `999` to preserve current always-engage behavior for all existing templates.
- REQ-NB-11: `Template.FleeHPPct` MUST default to `0` (never flee).
- REQ-NB-12: `Instance` MUST gain a `GrudgePlayerID string` field with zero value `""`. It MUST be cleared to `""` on NPC respawn.
- REQ-NB-13: When an NPC takes damage, it MUST set `GrudgePlayerID` to the attacking player's ID; if multiple attackers deal damage in the same round, the last damage event in processing order wins.
- REQ-NB-14: `flee_hp_pct` MUST be evaluated at the end of each round in which the NPC took damage.
- REQ-NB-15: The HTN evaluator MUST support the `has_grudge_target` precondition.

---

## 3. Daily Patterns (Time-of-Day)

### 3.1 Game-Hour Exposure

The NPC manager gains a read-only accessor to the current game hour (0–23) from the `persistent-calendar` service. No new clock is introduced. The accessor is injected at manager construction.

### 3.2 Template Schedule Field

An optional `schedule` list on the template. Each entry:

| Field | Type | Description |
|-------|------|-------------|
| `hours` | `string` | Hour range (`"6-18"`) or comma-separated hours (`"8,12,20"`) |
| `preferred_room` | `string` | Room ID the NPC moves toward during this window |
| `behavior_mode` | `string` | One of: `idle`, `patrol`, `aggressive` |

When the end hour is less than the start hour in a range, the range wraps midnight (e.g., `"19-5"` covers hours 19–23 and 0–5).

Example:

```yaml
schedule:
  - hours: "6-18"
    preferred_room: market_stall_3
    behavior_mode: patrol
  - hours: "19-5"
    preferred_room: barracks_bunk
    behavior_mode: idle
```

### 3.3 Behavior Modes

- REQ-NB-21A: When `behavior_mode` is `idle`, the NPC MUST remain in its `preferred_room` without movement and MUST fire idle-context `say` operator entries.
- REQ-NB-21B: When `behavior_mode` is `patrol`, the NPC MUST wander within `wander_radius` hops of its `preferred_room` using the `move_random` operator.
- REQ-NB-21C: When `behavior_mode` is `aggressive`, the NPC's effective `courage_threshold` MUST be `0` for the duration of the matching schedule window.

### 3.4 Schedule Evaluation

Schedule evaluation runs on each idle tick while the NPC is not in combat.

- REQ-NB-16: The NPC manager MUST receive the current game hour via an injected accessor to the calendar service.
- REQ-NB-17: `Template.Schedule` MUST be an optional field; templates without it MUST behave as before.
- REQ-NB-18: Schedule `hours` MUST support range format (`"6-18"`) and comma-separated format (`"8,12,20"`). When the end hour is less than the start hour, the range MUST wrap midnight.
- REQ-NB-19: On each idle tick, the manager MUST read the current game hour and find the first schedule entry whose `hours` range includes that hour.
- REQ-NB-20: When a matching schedule entry is found and the NPC is not in its `preferred_room`, the NPC MUST move one BFS step toward `preferred_room` per idle tick.
- REQ-NB-21: When a matching schedule entry is found, the NPC MUST apply the entry's `behavior_mode` for that tick.
- REQ-NB-22: When no schedule entry matches the current hour, the NPC MUST apply its default template behavior.
- REQ-NB-23: `behavior_mode: aggressive` MUST set the NPC's effective `courage_threshold` to `0` for the duration of the matching schedule window.
- REQ-NB-24: Schedule evaluation MUST NOT fire while the NPC is in active combat.

---

## 4. Combat AI Expansion

Three new HTN operators added to the operator registry. All are opt-in via domain YAML.

### 4.1 `flee` Operator

- REQ-NB-25: The `flee` operator MUST require the NPC to be in active combat as a precondition.
- REQ-NB-26: On success, the `flee` operator MUST remove the NPC from the active combat round before relocating it to an adjacent room via a randomly selected visible exit.
- REQ-NB-27: After a successful `flee`, combat MUST continue among remaining participants without the fled NPC.
- REQ-NB-28: When no exits are available, the `flee` operator MUST fail and allow HTN fallback to the next task.

### 4.2 `target_weakest` Operator

- REQ-NB-29: The `target_weakest` operator MUST set the NPC's current combat target to the living player with the lowest current HP% in the room.
- REQ-NB-30: The `target_weakest` operator MUST require two or more living players in combat in the NPC's room as a precondition.
- REQ-NB-31: When fewer than two living players are present, `target_weakest` MUST fail silently and retain the existing target.

### 4.3 `call_for_help` Operator

- REQ-NB-32A: For the purposes of `call_for_help`, "adjacent room" MUST mean a room reachable via exactly one exit (BFS distance 1) from the caller's room.
- REQ-NB-32: The `call_for_help` operator MUST require the NPC to be in active combat as a precondition.
- REQ-NB-33: The `call_for_help` operator MUST require at least one adjacent room (BFS distance 1) to contain an idle NPC with a matching `type` field and hostile disposition as a precondition.
- REQ-NB-34: On success, `call_for_help` MUST cause all qualifying idle NPCs in adjacent rooms (matching `type`, hostile disposition, not in combat) to join combat in the caller's room on the following tick.
- REQ-NB-35: The `call_for_help` operator MUST fire at most once per combat instance per NPC, enforced via `AbilityCooldowns` with a duration that persists until respawn.

---

## 5. Map Movement & Fencing

### 5.1 New Template Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `home_room` | `string` | spawn room | Room ID the NPC returns to when idle. Defaults to spawn room if not set. |
| `wander_radius` | `int` | `0` | Maximum BFS hop distance from `home_room` during patrol. 0 = stay in home room. |

### 5.2 Patrol Constraint

The existing `move_random` HTN operator is updated to filter candidate exits to only those within `wander_radius` BFS hops of `home_room`. The BFS distance map (room ID → hop count from `home_room`) is computed once at zone load and cached on the NPC instance.

- REQ-NB-36: `Template.HomeRoom` MUST default to the NPC's spawn room when not set.
- REQ-NB-37: `Template.WanderRadius` MUST default to `0`; templates without it MUST NOT move during patrol.
- REQ-NB-38: At zone load, the BFS distance map from `home_room` MUST be computed and cached on the NPC instance. If `home_room` is not found within the same zone, the zone loader MUST return an error.
- REQ-NB-39: The `move_random` operator MUST exclude exits that would place the NPC beyond `wander_radius` hops from `home_room`.
- REQ-NB-40: When `move_random`'s filtered exit pool is empty, the operator MUST fail and allow HTN fallback.

### 5.3 Home-Room Return

- REQ-NB-41: `Instance` MUST gain a `ReturningHome bool` field. After combat ends with no players remaining in the NPC's room, if the NPC is not in `home_room`, the NPC MUST set `Instance.ReturningHome = true`.
- REQ-NB-42: On each idle tick while `Instance.ReturningHome` is true, the NPC MUST move one BFS step toward `home_room`.
- REQ-NB-43: When the NPC arrives at `home_room`, `Instance.ReturningHome` MUST be cleared.
- REQ-NB-44: Home-room return movement MUST NOT fire while the NPC is in active combat.

### 5.4 Schedule Interaction

- REQ-NB-45: When a schedule entry is active, its `preferred_room` MUST replace `home_room` as the fencing anchor for `move_random` radius calculation and home-room return. When the schedule window ends, the template `home_room` MUST resume as the fencing anchor.

---

## 6. Requirements Summary

- REQ-NB-1: The `say` HTN operator MUST select a random string from its `strings` parameter and broadcast it to the NPC's room as a speech event.
- REQ-NB-2: The `say` operator MUST enforce its `cooldown` parameter via `Instance.AbilityCooldowns` keyed by operator ID. The `cooldown` value MUST be a Go duration string parsed via `time.ParseDuration`.
- REQ-NB-3: The HTN evaluator MUST support the `in_combat`, `player_entered_room`, `hp_pct_below`, and `on_damage_taken` preconditions.
- REQ-NB-4: The `player_entered_room` and `on_damage_taken` preconditions MUST be true for exactly one evaluation tick after the triggering event.
- REQ-NB-5: All existing NPC templates using `taunts`/`taunt_chance`/`taunt_cooldown` MUST be migrated to `say` operator HTN domain entries.
- REQ-NB-6: `Template.Taunts`, `Template.TauntChance`, `Template.TauntCooldown`, `Instance.Taunts`, `Instance.TauntChance`, `Instance.TauntCooldown`, `Instance.LastTauntTime`, and `Instance.TryTaunt()` MUST be removed. `Template.Validate()` MUST be updated to remove the corresponding checks.
- REQ-NB-7: Threat assessment MUST fire on each idle tick when a hostile-disposition NPC shares a room with one or more players and is not in combat.
- REQ-NB-7A: When threat assessment determines the NPC should not engage and the NPC is not in combat, the NPC MUST remain passive and MUST NOT initiate combat.
- REQ-NB-7B: When threat assessment determines the NPC should not engage and the NPC is already in combat and `flee_hp_pct > 0` and the threshold is met, the NPC MUST execute the `flee` operator.
- REQ-NB-7C: When the NPC is already in combat and no flee threshold applies, engagement MUST NOT be re-evaluated mid-combat.
- REQ-NB-8: Threat assessment MUST fire when a player enters a room containing a hostile-disposition NPC not in combat.
- REQ-NB-9: The threat score formula MUST incorporate player average level, NPC level, party size, and player average HP% as a `float64` in `[0.0, 1.0]`.
- REQ-NB-10: `Template.CourageThreshold` MUST default to `999` to preserve current always-engage behavior.
- REQ-NB-11: `Template.FleeHPPct` MUST default to `0` (never flee).
- REQ-NB-12: `Instance` MUST gain a `GrudgePlayerID string` field with zero value `""`. It MUST be cleared to `""` on NPC respawn.
- REQ-NB-13: When an NPC takes damage, it MUST set `GrudgePlayerID` to the attacking player's ID; last damage event in processing order wins.
- REQ-NB-14: `flee_hp_pct` MUST be evaluated at the end of each round in which the NPC took damage.
- REQ-NB-15: The HTN evaluator MUST support the `has_grudge_target` precondition.
- REQ-NB-16: The NPC manager MUST receive the current game hour via an injected accessor to the calendar service.
- REQ-NB-17: `Template.Schedule` MUST be an optional field; templates without it MUST behave as before.
- REQ-NB-18: Schedule `hours` MUST support range format (`"6-18"`) and comma-separated format (`"8,12,20"`). When end hour < start hour, the range MUST wrap midnight.
- REQ-NB-19: On each idle tick, the manager MUST read the current game hour and find the first matching schedule entry.
- REQ-NB-20: When a matching entry is found and the NPC is not in `preferred_room`, the NPC MUST move one BFS step toward `preferred_room` per idle tick.
- REQ-NB-21: When a matching entry is found, the NPC MUST apply the entry's `behavior_mode` for that tick.
- REQ-NB-21A: When `behavior_mode` is `idle`, the NPC MUST remain in its `preferred_room` without movement and MUST fire idle-context `say` operator entries.
- REQ-NB-21B: When `behavior_mode` is `patrol`, the NPC MUST wander within `wander_radius` hops of its `preferred_room` using the `move_random` operator.
- REQ-NB-21C: When `behavior_mode` is `aggressive`, the NPC's effective `courage_threshold` MUST be `0` for the duration of the matching schedule window.
- REQ-NB-22: When no entry matches, the NPC MUST apply its default template behavior.
- REQ-NB-24: Schedule evaluation MUST NOT fire while the NPC is in active combat.
- REQ-NB-25: The `flee` operator MUST require the NPC to be in active combat as a precondition.
- REQ-NB-26: On success, `flee` MUST remove the NPC from the active combat round before relocating it via a randomly selected visible exit.
- REQ-NB-27: After a successful `flee`, combat MUST continue among remaining participants without the fled NPC.
- REQ-NB-28: When no exits are available, `flee` MUST fail and allow HTN fallback.
- REQ-NB-29: `target_weakest` MUST set the NPC's combat target to the living player with the lowest current HP% in the room.
- REQ-NB-30: `target_weakest` MUST require two or more living players in the room as a precondition.
- REQ-NB-31: When fewer than two living players are present, `target_weakest` MUST fail silently and retain the existing target.
- REQ-NB-32A: For the purposes of `call_for_help`, "adjacent room" MUST mean a room reachable via exactly one exit (BFS distance 1) from the caller's room.
- REQ-NB-32: `call_for_help` MUST require the NPC to be in active combat as a precondition.
- REQ-NB-33: `call_for_help` MUST require at least one adjacent room (BFS distance 1) containing a qualifying idle NPC as a precondition.
- REQ-NB-34: On success, `call_for_help` MUST cause all qualifying idle NPCs in adjacent rooms (matching `type`, hostile disposition, not in combat) to join combat in the caller's room on the following tick.
- REQ-NB-35: `call_for_help` MUST fire at most once per combat instance per NPC.
- REQ-NB-36: `Template.HomeRoom` MUST default to the NPC's spawn room when not set.
- REQ-NB-37: `Template.WanderRadius` MUST default to `0`; templates without it MUST NOT move during patrol.
- REQ-NB-38: At zone load, the BFS distance map from `home_room` MUST be computed and cached on the NPC instance. If `home_room` is not in the same zone, the zone loader MUST return an error.
- REQ-NB-39: The `move_random` operator MUST exclude exits beyond `wander_radius` hops from `home_room`.
- REQ-NB-40: When `move_random`'s filtered pool is empty, the operator MUST fail and allow HTN fallback.
- REQ-NB-41: After combat ends with no players in the room, if the NPC is not in `home_room`, it MUST set `Instance.ReturningHome = true`.
- REQ-NB-42: On each idle tick while `ReturningHome` is true, the NPC MUST move one BFS step toward `home_room`.
- REQ-NB-43: When the NPC arrives at `home_room`, `Instance.ReturningHome` MUST be cleared.
- REQ-NB-44: Home-room return movement MUST NOT fire while the NPC is in active combat.
- REQ-NB-45: When a schedule entry is active, its `preferred_room` MUST replace `home_room` as the fencing anchor. When the window ends, the template `home_room` MUST resume.
