# Zones-New Phase 1: Mechanics Design Spec

**Date:** 2026-03-21
**Feature:** zones-new (Phase 1 — Mechanics)
**Project:** Gunchete MUD

## Overview

Phase 1 adds four sets of mechanics to the zone system:

1. **Zone-level effect inheritance** — zones can define effects that propagate to all their rooms at load time; the hardcoded `RoomEffect.Track` enum is eliminated and replaced with free condition IDs resolved via the condition registry.
2. **Seduction HTN operator** — a new player action using Flair vs Savvy to charm or antagonize NPCs; requires NPC gender field.
3. **New condition definitions** — conditions added to `content/conditions/` to support zone effects and the seduction/charmed mechanic.
4. **Typed terrain conditions** — terrain types are conditions with mechanical penalties (AP cost, skill modifiers); multiple terrain conditions on a room stack additively.

Phase 2 (content generation) is a separate spec covering the four specific zones.

---

## Section 1: Zone Effect Inheritance & Track Unification

### Motivation

The existing `RoomEffect.Track` field is a hardcoded enum (`rage`, `despair`, `delirium`, `fear`). Adding new zone effect types requires modifying Go source. Replacing the enum with a free condition ID validated at startup gives full extensibility.

Zone-level effects are needed so designers can set an atmospheric condition (e.g., `nausea`) on an entire zone in one YAML field rather than duplicating it in every room definition.

### Requirements

- **REQ-ZN-1**: `world.Zone` MUST gain a `ZoneEffects []RoomEffect` field (YAML: `zone_effects`). At world load time, the loader MUST append each `ZoneEffect` entry to every room's `Effects` slice in that zone. The world is loaded once at startup; hot-reload is not supported and no deduplication is required.

- **REQ-ZN-2**: `RoomEffect.Track` MUST be treated as a condition ID. The hardcoded track enum MUST be removed. The effect application code MUST resolve the condition via `conditionRegistry.Get(effect.Track)` and apply it via `condition.ActiveSet.Apply`. If `conditionRegistry.Get` returns nil at runtime, the effect application code MUST log a warning and skip that effect.

- **REQ-ZN-3**: At startup, `world.Validate()` MUST verify every `RoomEffect.Track` value (on both room-level and zone-level effects) exists in the condition registry. Any unknown track MUST cause a fatal startup error.

- **REQ-ZN-4**: The four existing track values (`"rage"`, `"despair"`, `"delirium"`, `"fear"`) MUST have corresponding condition definitions in `content/conditions/` if not already present. All existing zone and room YAML files using these track values remain valid without modification.

- **REQ-ZN-5**: New condition definitions MUST be added for: `"horror"`, `"nausea"`, `"reduced_visibility"`, `"temptation"`, `"revulsion"`, `"sonic_assault"`, `"charmed"`. These IDs MUST NOT use the `terrain_` prefix.

---

## Section 2: Seduction HTN Operator & NPC Gender

### Motivation

Seduction is a social HTN action that uses the player's Flair skill. It requires knowing NPC gender — not for flavor restriction but as a gameplay precondition (a genderless/robotic NPC cannot be seduced). On success the NPC is charmed; on failure the NPC turns hostile.

### Requirements

- **REQ-ZN-6**: `npc.Template` MUST gain `Gender string` (YAML: `gender`); empty string indicates no gender. `npc.Instance` MUST propagate `Gender` from its template at spawn. `npc.Instance.Gender` is a runtime-only field and MUST NOT have a YAML tag. Per-instance YAML override of gender is NOT supported; the template is the sole source.

- **REQ-ZN-7**: `seduce <npc>` MUST be an HTN operator. Preconditions: `instance.Gender != ""`, NPC does not already have the `charmed` condition, and `sess.Skills["flair"] > 0`.

- **REQ-ZN-8**: Seduction resolution MUST use an opposed skill check: player Flair vs NPC Savvy. On player success, the NPC gains the `charmed` condition. On player failure, the NPC's disposition flips to hostile and `npc.Instance` MUST gain a `SeductionRejected map[string]bool` field (keyed by player UID) set to `true`; this field is runtime-only with no YAML tag. At respawn, `SeductionRejected` MUST be set to nil. The HTN operator MUST fail as a precondition if `SeductionRejected[playerUID]` is true.

- **REQ-ZN-9**: The `charmed` condition MUST have `duration_type: until_save`. At the end of each round, the round-tick handler MUST check every NPC with the `charmed` condition and trigger a Savvy saving throw using the same resolution mechanism as all other NPC saving throws, against DC 15. On success the `charmed` condition is removed.

- **REQ-ZN-10**: The `charmed` condition definition is included in REQ-ZN-5. Charmed NPCs treat the player as allied for the duration.

---

## Section 3: Typed Terrain Conditions

### Motivation

The existing `Properties["terrain"] == "difficult"` check is a single binary flag with no penalty gradation and no zone-level inheritance. Different terrain types (mud, rubble, ice, flooding) warrant different mechanical penalties. Making terrain types condition definitions with penalty fields allows zone-level inheritance via `ZoneEffects`, room-level stacking via `Effects`, and extensibility without code changes.

### Requirements

- **REQ-ZN-11**: `ConditionDef` MUST gain optional fields `MoveAPCost int` (YAML: `move_ap_cost`) and `SkillPenalties map[string]int` (YAML: `skill_penalties`). Both default to zero/nil and are ignored on conditions that do not set them. Keys in `SkillPenalties` MUST be canonical skill IDs (lowercase, underscore-separated, e.g. `"flair"`, `"savvy"`). At startup, `world.Validate()` MUST verify every key in every condition's `SkillPenalties` exists in the skill registry; any unknown key MUST cause a fatal startup error.

- **REQ-ZN-12**: Terrain condition definitions MUST be added to `content/conditions/` with IDs: `terrain_rubble`, `terrain_mud`, `terrain_flooded`, `terrain_ice`, `terrain_dense_vegetation`. All terrain condition IDs MUST use the `terrain_` prefix. Each MUST set `move_ap_cost` and/or `skill_penalties` values appropriate to the terrain type.

- **REQ-ZN-13**: The existing `Properties["terrain"] == "difficult"` check MUST be removed from the movement handler. The movement handler MUST instead collect all conditions in the room's effective condition set (after zone propagation) whose ID has the `terrain_` prefix and whose `MoveAPCost > 0`. The total AP cost deducted MUST equal the sum of all matching conditions' `MoveAPCost` values. One message per matching terrain condition MUST be sent using that condition's label, ordered by condition ID alphabetically. The `zone_awareness` passive feat MUST suppress all terrain messages (conditions with the `terrain_` prefix) but MUST NOT suppress the AP cost deduction.

- **REQ-ZN-14**: The terrain condition IDs added in this phase are defined in REQ-ZN-12. Terrain condition IDs MUST NOT appear in REQ-ZN-5, and non-terrain condition IDs MUST NOT use the `terrain_` prefix.
