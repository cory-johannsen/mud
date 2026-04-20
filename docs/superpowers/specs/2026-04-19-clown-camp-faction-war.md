---
issue: 164
title: Clown Camp faction war — Just Clownin', The Queer Clowning Experience, The Unwoke MAGA Clown Army
slug: clown-camp-faction-war
date: 2026-04-19
---

## Summary

Expand Clown Camp with two new factions and their home territories, plus
faction-aware NPC combat so all three factions automatically fight each other
on The Stage. This requires both content additions and a new engine capability
(NPC-vs-NPC faction hostility).

---

## Architecture Overview

### New Engine Capability Required

The current combat engine identifies enemies as `Kind != selfKind` (player vs
NPC), so NPCs from hostile factions treat each other as allies. A new
faction-aware enemy detection path MUST be added to the Lua combat API before
NPC-vs-NPC faction war can function.

### Content Additions

- 3 new faction definitions
- ~10 new NPC templates (sortie fighters + leaders + service NPCs per faction)
- ~8 new rooms (QCE territory + UMCA territory)
- Updated spawn tables in `clown_camp.yaml`
- 2 new HTN AI domains (faction-aware combat)

---

## Requirements

### REQ-CCF-1: New Faction Definitions

Three new faction YAML files MUST be created in `content/factions/`:

#### `just_clownin.yaml`
```yaml
id: just_clownin
name: "Just Clownin'"
zone_id: clown_camp
hostile_factions:
  - queer_clowning_experience
  - unwoke_maga_clown_army
tiers:
  - id: curious
    label: "Curious"
    min_rep: 0
    price_discount: 0.0
  - id: initiate
    label: "Initiate"
    min_rep: 100
    price_discount: 0.05
  - id: clown
    label: "Clown"
    min_rep: 500
    price_discount: 0.10
  - id: ringleader
    label: "Ringleader"
    min_rep: 2000
    price_discount: 0.20
rep_sources:
  - source: kill_queer_clowning_experience
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
  - source: kill_unwoke_maga_clown_army
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
```

#### `queer_clowning_experience.yaml`
Same structure; hostile to `just_clownin` and `unwoke_maga_clown_army`. Same
tier labels: Curious → Initiate → Clown → Ringleader.

#### `unwoke_maga_clown_army.yaml`
Same structure; hostile to `just_clownin` and `queer_clowning_experience`.
Same tier labels: Curious → Initiate → Clown → Ringleader.

### REQ-CCF-2: Faction-Aware Enemy Detection (Engine)

A new Lua function `get_faction_enemies()` MUST be added to the Lua combat API
in `internal/scripting/modules.go`:

- REQ-CCF-2a: `get_faction_enemies(uid)` MUST return a table of combatant UIDs
  currently in the same room as `uid` that belong to factions declared hostile
  by `uid`'s faction.
- REQ-CCF-2b: If `uid` has no faction, the function MUST return an empty table.
- REQ-CCF-2c: If a hostile-faction combatant is already in combat with `uid`,
  they MUST still be included (the function returns candidates, not initiators).
- REQ-CCF-2d: The function MUST be available to HTN domain Lua preconditions and
  operators, identical to the existing `get_enemies` / `get_allies` pattern.
- REQ-CCF-2e: The combatant faction ID MUST be stored on the combat combatant
  model (`internal/game/combat/combatant.go`) and populated from the NPC
  template's `faction_id` field at combat registration.

### REQ-CCF-3: Faction Combat Initiation (Engine)

When an NPC with a faction enters a room (or spawns), the server MUST check
whether any NPC from a hostile faction is already present and in combat or idle.

- REQ-CCF-3a: If a hostile-faction NPC is present, the arriving NPC MUST
  immediately enter combat with one hostile NPC, selecting the lowest-HP target.
- REQ-CCF-3b: This check MUST only fire in rooms with `danger_level:
  all_out_war`. Faction NPCs in `safe` or `sketchy` rooms MUST NOT auto-initiate
  against each other.
- REQ-CCF-3c: The initiation MUST produce a console message visible to any
  player in the room, e.g.:
  `A Just Clownin' Fighter lunges at a QCE Agitator!`

### REQ-CCF-4: HTN AI Domain — Faction Combat

A new HTN domain `clown_faction_combat` MUST be created in `content/ai/`:

```yaml
domain:
  id: clown_faction_combat
  tasks:
    - id: behave
    - id: fight_faction_enemy
    - id: fight_player
  methods:
    - task: behave
      id: faction_enemy_present
      precondition: has_faction_enemy     # calls get_faction_enemies(uid) — non-empty
      subtasks: [fight_faction_enemy]
    - task: behave
      id: player_enemy_present
      precondition: has_player_enemy      # existing get_enemies — non-empty
      subtasks: [fight_player]
    - task: fight_faction_enemy
      id: strike_faction_enemy
      precondition: always_true
      subtasks: [attack_faction_enemy]
    - task: fight_player
      id: strike_player
      precondition: always_true
      subtasks: [attack_enemy]
  operators:
    - id: attack_faction_enemy
      action: attack
      target: nearest_faction_enemy      # new target type: lowest HP in get_faction_enemies()
    - id: attack_enemy
      action: attack
      target: nearest_enemy
```

- REQ-CCF-4a: `has_faction_enemy` Lua precondition MUST call `get_faction_enemies(uid)` and return true if the table is non-empty.
- REQ-CCF-4b: `nearest_faction_enemy` target resolver MUST select the combatant
  from `get_faction_enemies()` with the lowest current HP.
- REQ-CCF-4c: All three factions' sortie NPC templates MUST use
  `ai_domain: clown_faction_combat`.

### REQ-CCF-5: Assign Faction to Existing Clown Camp NPCs

All existing Clown Camp NPC templates MUST be updated to assign `faction_id: just_clownin`:

| Template | Current faction_id | New faction_id |
|---|---|---|
| `clown` | (none) | `just_clownin` |
| `clown_mime` | (none) | `just_clownin` |
| `just_clownin` (ringleader) | (none) | `just_clownin` |
| `big_top` (boss) | (none) | `just_clownin` |

### REQ-CCF-6: New NPC Templates

#### Just Clownin' Sortie (2 templates)
- `jc_fighter`: level 4, standard, `faction_id: just_clownin`, `ai_domain: clown_faction_combat`. Armed with improvised melee.
- `jc_enforcer`: level 6, elite, `faction_id: just_clownin`, `ai_domain: clown_faction_combat`. Carries face-paint debuff ability.

#### The Queer Clowning Experience (4 templates)
- `qce_agitator`: level 4, standard, `faction_id: queer_clowning_experience`, `ai_domain: clown_faction_combat`. Fast attacker, low HP.
- `qce_drag_bruiser`: level 6, elite, `faction_id: queer_clowning_experience`, `ai_domain: clown_faction_combat`. High HP, AoE glitter-bomb attack.
- `qce_ringleader`: level 8, boss, `faction_id: queer_clowning_experience`. Boss for QCE territory; `boss_abilities` with crowd-control and self-heal.
- `qce_merchant`: non-combat, sells QCE-exclusive items to allied players.

#### The Unwoke MAGA Clown Army (4 templates)
- `umca_grunt`: level 4, standard, `faction_id: unwoke_maga_clown_army`, `ai_domain: clown_faction_combat`. Ranged attacks.
- `umca_flag_bearer`: level 6, elite, `faction_id: unwoke_maga_clown_army`, `ai_domain: clown_faction_combat`. Buffs nearby UMCA allies (+AC).
- `umca_commander`: level 8, boss, `faction_id: unwoke_maga_clown_army`. Boss for UMCA territory; `boss_abilities` with rally (summons a grunt) and suppressive fire.
- `umca_merchant`: non-combat, sells UMCA-exclusive items to allied players.

### REQ-CCF-7: New Rooms — QCE Territory

Five rooms MUST be added to `clown_camp.yaml` forming The Queer Clowning
Experience's home territory. All rooms MUST have `danger_level: dangerous`
except the final boss room (`all_out_war`). Exit from The Stage leads west into
QCE territory.

| Room ID | Title | Spawns |
|---|---|---|
| `cc_qce_entrance` | The Rainbow Gate | `qce_agitator` ×2 |
| `cc_qce_costume_vault` | Costume Vault | `qce_agitator` ×1, `qce_drag_bruiser` ×1 |
| `cc_qce_rehearsal_hall` | Rehearsal Hall | `qce_drag_bruiser` ×2 |
| `cc_qce_green_room` | The Green Room | `qce_merchant` ×1 |
| `cc_qce_throne` | The Sequined Throne | `qce_ringleader` ×1 (boss room) |

Room connections: `cc_the_stage` ←→ `cc_qce_entrance` ←→ `cc_qce_costume_vault` ←→ `cc_qce_rehearsal_hall` ←→ `cc_qce_green_room` ←→ `cc_qce_throne`

### REQ-CCF-8: New Rooms — UMCA Territory

Five rooms MUST be added forming The Unwoke MAGA Clown Army's home territory.
All rooms MUST have `danger_level: dangerous` except the final boss room
(`all_out_war`). Exit from The Stage leads east into UMCA territory.

| Room ID | Title | Spawns |
|---|---|---|
| `cc_umca_gate` | The Patriot Gate | `umca_grunt` ×2 |
| `cc_umca_armory` | MAGA Armory | `umca_grunt` ×1, `umca_flag_bearer` ×1 |
| `cc_umca_rally_grounds` | Rally Grounds | `umca_flag_bearer` ×2 |
| `cc_umca_commissary` | The Commissary | `umca_merchant` ×1 |
| `cc_umca_war_tent` | The War Tent | `umca_commander` ×1 (boss room) |

Room connections: `cc_the_stage` ←→ `cc_umca_gate` ←→ `cc_umca_armory` ←→ `cc_umca_rally_grounds` ←→ `cc_umca_commissary` ←→ `cc_umca_war_tent`

### REQ-CCF-9: The Stage — Sortie Spawns

`cc_the_stage` MUST have spawn tables for all three factions' sortie NPCs:

```yaml
- id: cc_the_stage
  danger_level: all_out_war
  boss_room: true
  spawns:
    - template: big_top          # existing JC boss
      count: 1
      respawn_after: 60m
    - template: jc_fighter       # JC sortie
      count: 1
      respawn_after: 10m
    - template: qce_agitator     # QCE sortie
      count: 1
      respawn_after: 10m
    - template: umca_grunt       # UMCA sortie
      count: 1
      respawn_after: 10m
```

- REQ-CCF-9a: `cc_the_stage` MUST have exits: west to `cc_qce_entrance` (zone:
  clown_camp), east to `cc_umca_gate` (zone: clown_camp), west to existing
  `cc_backstage` (existing JC territory).
- REQ-CCF-9b: The room description MUST reflect the contested, chaotic nature of
  the space.

### REQ-CCF-10: Map Coordinates

QCE rooms MUST extend west from The Stage (`map_x` decreasing from `cc_the_stage`'s X).
UMCA rooms MUST extend east from The Stage (`map_x` increasing). Both territories
MUST be placed such that they do not overlap with existing rooms.

### REQ-CCF-11: Faction Rep from Clown Camp Combat

- REQ-CCF-11a: Killing a `just_clownin` faction NPC in Clown Camp MUST grant rep
  with both `queer_clowning_experience` and `unwoke_maga_clown_army` (cross-faction
  rep, same as existing `rep_sources` pattern).
- REQ-CCF-11b: Same logic applies symmetrically for the other two factions.

### REQ-CCF-12: Test Coverage

- REQ-CCF-12a: Unit test for `get_faction_enemies()`: NPC with `faction_id: just_clownin` in a room with an NPC of `faction_id: queer_clowning_experience` MUST return the QCE NPC.
- REQ-CCF-12b: Unit test for `get_faction_enemies()`: NPC with no faction_id MUST return empty table.
- REQ-CCF-12c: Unit test for faction combat initiation: arriving JC NPC in `all_out_war` room with QCE NPC present MUST auto-enter combat.
- REQ-CCF-12d: Unit test: faction combat initiation MUST NOT fire in `safe` or `sketchy` rooms.
- REQ-CCF-12e: Property-based test: after any faction NPC spawn event on The Stage, `ActiveHostilities` MUST include at least one cross-faction combat pair when both hostile factions have a combatant present.

---

## Delivery Order

1. Engine: `get_faction_enemies()` Lua function + combatant faction_id field (REQ-CCF-2)
2. Engine: faction combat initiation on room entry (REQ-CCF-3)
3. Content: faction YAML files (REQ-CCF-1)
4. Content: HTN domain `clown_faction_combat` (REQ-CCF-4)
5. Content: NPC templates (REQ-CCF-5, REQ-CCF-6)
6. Content: new rooms in clown_camp.yaml (REQ-CCF-7, REQ-CCF-8)
7. Content: The Stage sortie spawns + exits (REQ-CCF-9, REQ-CCF-10)
8. Content: rep sources (REQ-CCF-11)

Steps 1–2 are blocking for all other steps and MUST be completed first.
