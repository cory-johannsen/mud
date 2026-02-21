# Combat System Design

**Date:** 2026-02-21
**Status:** Approved

## Goal

Implement the full Gunchete combat system incrementally across 8 stages. Each stage delivers a shippable, playable feature that builds on the previous one. The full system covers: dice rolling, NPC entities, PF2E 3-action economy with real-time round timer, conditions, Lua scripting, firearms/explosives, and HTN NPC AI.

## Architecture

The combat system follows the existing two-tier pattern:
- **Frontend** handles command parsing and text rendering; sends `AttackRequest`, `QueueActionRequest`, etc. via gRPC stream
- **Gameserver** owns all combat state: initiative order, round timer, action queues, HP tracking, condition tracking
- **Lua VM** (Stage 6+) runs per-zone in isolated GopherLua states; called from Go for resolution hooks
- **YAML** is the canonical data source for NPCs, weapons, conditions, HTN domains

---

## Stage 1 — Dice & Roll Engine

**Scope:** Standalone `internal/game/dice` package. No combat yet.

**Deliverables:**
- Parse dice expressions: `d20`, `2d6+3`, `d100`, `4d6kh3` (keep highest)
- Roll using `crypto/rand` as entropy source
- Return structured `RollResult`: expression, individual die values, modifier, total
- All rolls logged with expression + result (COMBAT-29)
- Property-based tests: totals always within theoretical min/max, distribution uniformity

**New packages:** `internal/game/dice`

**Requirements covered:** COMBAT-27, COMBAT-28, COMBAT-29

---

## Stage 2 — NPC Definitions + Room Entities

**Scope:** YAML-defined NPCs loaded at startup; rooms track and display living entities.

**Deliverables:**
- `internal/game/npc` package: `NPC` model, `Template` (YAML), `Instance`
- NPC YAML schema: id, name, description, archetype, level, abilities, max_hp, ac, perception
- `content/npcs/` directory with starter NPCs (2–3 gang members, a scavenger)
- Room `EntityList` tracks players + NPC instances
- `look` output updated to show NPCs present in room
- `examine <target>` command: shows NPC name, description, visible health state
- Migration 005: `npc_instances` table (template_id, room_id, current_hp, conditions JSONB)
- Gameserver loads NPC templates at startup, spawns instances per zone config

**New packages:** `internal/game/npc`
**Modified:** `internal/game/world` (room entity list), `internal/gameserver` (NPC spawning), `internal/game/command`

**Requirements covered:** AI-6, AI-7, AI-8, WORLD-8, ARCH-15

---

## Stage 3 — MVP Combat Loop (PvE)

**Scope:** First playable combat. Single action per turn, no timer. Players fight NPCs.

**Deliverables:**
- `attack <target>` command enters combat
- Initiative roll: d20 + DEX modifier, all combatants rolled at combat start
- Turn order established by initiative result (highest first)
- On player turn: one attack action
  - Attack roll: d20 + STR/DEX modifier + proficiency bonus vs target AC
  - PF2E 4-tier success (COMBAT-14): crit success (double damage), success (full), failure (miss), crit failure (special)
  - Damage roll: weapon damage dice + ability modifier
  - HP reduced; broadcast result to room
- NPC turn: simple attack back (no AI yet — target is attacker, always attacks)
- Death/incapacitation at 0 HP: entity removed from combat, broadcast "X falls"
- `flee` command: opposed check to escape combat
- Combat ends when one side is eliminated or all players flee
- Proto additions: `AttackRequest`, `CombatEvent` (initiative, attack result, damage, death)
- Text renderer: formatted combat output with dice results

**New packages:** `internal/game/combat`
**Modified:** `internal/gameserver`, proto, frontend handlers

**Requirements covered:** COMBAT-3, COMBAT-10, COMBAT-11, COMBAT-13, COMBAT-14, COMBAT-17, COMBAT-18, COMBAT-20

---

## Stage 4 — Three-Action Economy + Round Timer

**Scope:** Upgrade combat to full PF2E 3-action economy with real-time 6-second round window.

**Deliverables:**
- Players queue up to 3 actions per round via commands during the window
- Action types: single (1 action), activity (2 or 3 actions), free action, reaction
- `attack` costs 1 action; `strike` (full attack routine) costs 2; reload costs defined per weapon
- Round timer: configurable duration (default 6s), server-side goroutine per combat
- Actions not submitted before timer expires are forfeited (COMBAT-7)
- All queued actions resolve in initiative order after timer fires
- `pass` command: explicitly forfeit remaining actions
- `ready <action>` command: hold an action as a reaction trigger (COMBAT-9)
- Frontend: displays "Round N begins. You have 3 actions. [6s]" countdown
- Proto additions: `QueueActionRequest`, `RoundStartEvent`, `RoundEndEvent`, `ActionResultEvent`

**Modified:** `internal/game/combat`, `internal/gameserver`, proto, frontend

**Requirements covered:** COMBAT-4, COMBAT-5, COMBAT-6, COMBAT-7, COMBAT-8, COMBAT-9

---

## Stage 5 — Conditions System

**Scope:** YAML-defined conditions with duration tracking, roll modifiers, and action restrictions.

**Deliverables:**
- `content/conditions/` YAML files: stunned, frightened, prone, flat-footed, wounded, dying, unconscious, etc.
- Condition schema: id, name, description, duration_type (rounds/until_save/permanent), modifiers (attack/ac/speed), restricted_actions, lua_on_apply, lua_on_remove, lua_on_tick
- `internal/game/condition` package: condition registry, active condition tracking per entity
- Conditions applied/removed by combat results (e.g., crit failure → prone)
- Condition modifiers factored into attack rolls, AC calculations, action availability
- Dying/wounded condition chain: 0 HP → dying 1, recovery checks each round, death at dying 4
- `status` command: shows player's active conditions
- Conditions broadcast to room ("X is now prone")

**New packages:** `internal/game/condition`
**Modified:** `internal/game/combat`, `internal/gameserver`

**Requirements covered:** COMBAT-15, COMBAT-16

---

## Stage 6 — Lua Scripting Engine

**Scope:** Embed GopherLua, expose engine API, port combat resolution to Lua hooks.

**Deliverables:**
- `internal/scripting` package: GopherLua VM manager, per-zone isolated states, sandboxing
- Sandbox: disable `os`, `io`, `debug`, `loadfile`; instruction count limit (SCRIPT-3, SCRIPT-4)
- Engine API modules exposed to Lua:
  - `engine.combat`: initiate, resolve_action, apply_damage, apply_condition, query_combatant
  - `engine.dice`: roll(expr) → {total, dice, modifier}
  - `engine.entity`: get/set attributes, query conditions
  - `engine.event`: register_listener, emit, schedule
  - `engine.log`: debug/info/warn/error
  - `engine.world`: query_room, move_entity, broadcast
- Combat resolution hooks called from Go → Lua: `on_attack_roll`, `on_damage_roll`, `on_condition_apply`
- Room hooks: `on_enter`, `on_exit`, `on_look`
- Hot-reload: file watcher reloads scripts without restart (SCRIPT-5, ARCH-18)
- Lua error handling: caught, logged with file+line, never crashes server (SCRIPT-19, SCRIPT-20)
- `content/scripts/` directory with starter scripts

**New packages:** `internal/scripting`
**Modified:** `internal/game/combat`, `internal/game/world`, `internal/gameserver`

**Requirements covered:** COMBAT-1, COMBAT-2, SCRIPT-1 through SCRIPT-21, ARCH-19 through ARCH-22

---

## Stage 7 — Firearms & Explosives

**Scope:** YAML weapon definitions with firearms mechanics and area-of-effect explosives.

**Deliverables:**
- `content/weapons/` YAML files: melee and ranged weapons
- Firearm schema: id, name, damage_dice, damage_type, range_increment, reload_actions, magazine_capacity, firing_modes, traits
- Firing modes: single (1 action), burst (2 actions, cone or multiple targets), automatic (3 actions, suppressive)
- Reload: consumes 1–2 actions from economy; magazine tracked per instance
- Explosive schema: id, name, damage_dice, damage_type, area_type (room/burst), save_type, save_dc, fuse (immediate/delayed), traits
- Area effects hit all entities in target room (or subset by area_type)
- Saving throws against explosives: Reflex save, PF2E 4-tier result determines damage fraction
- `internal/game/inventory` package: item instances, equip slots, magazine state
- `equip <weapon>`, `unload`, `reload` commands
- All weapon mechanics defined in Lua scripts referencing YAML data

**New packages:** `internal/game/inventory`
**Modified:** `internal/game/combat`, `internal/gameserver`, `internal/game/command`

**Requirements covered:** COMBAT-21, COMBAT-22, COMBAT-23, COMBAT-24, COMBAT-25, COMBAT-26

---

## Stage 8 — HTN NPC AI

**Scope:** Hierarchical Task Network planner; NPCs participate in combat rounds as full agents.

**Deliverables:**
- `internal/game/ai` package: HTN planner, world state model, task/method/operator types
- HTN domain YAML schema: tasks, methods (ordered decompositions with Lua conditions), primitive operators (Lua functions)
- `content/ai/` YAML domains: `ganger_combat.yaml`, `scavenger_patrol.yaml`
- Behavioral states: idle, patrol, combat, flee, interact — driven by HTN plan output
- Sensory awareness: NPCs detect entities within configurable perception range (WORLD-8)
- Combat participation: NPCs queue actions within the same round window as players (COMBAT-12)
  - Plan evaluated at round start → produces action queue for the round
  - Actions submitted to combat engine same as player actions
- Re-planning triggered on world state change (COMBAT-11 compat, AI-4)
- Plan time budget: configurable instruction limit; exceeded plans deferred to next tick (AI-5)
- Per-zone NPC management: each zone goroutine owns its NPC population (AI-17)

**New packages:** `internal/game/ai`
**Modified:** `internal/gameserver` (zone tick loop), `internal/game/npc`

**Requirements covered:** AI-1 through AI-18, COMBAT-12

---

## Cross-Cutting Concerns

- **TDD + property-based tests** required at every stage (SWENG-5, SWENG-5a)
- **All dice results logged** with expression, individual values, modifiers (COMBAT-29)
- **gRPC proto** extended incrementally per stage with backward-compatible additions
- **Migrations** numbered sequentially (005, 006, ...) per stage
- **YAML validation** at load time; invalid files rejected with clear errors (PERS-13)
- **Lua errors** never crash the server; always caught and logged (SCRIPT-19)

## Non-Goals (deferred)

- WebSocket transport (separate phase)
- Quest system
- Inventory persistence beyond session (Stage 7 tracks in-memory; persistence in a later phase)
- PvP consent/flagging system (mechanics work, no social governance layer yet)
- Weather/time effects on combat (after world time system is built)
