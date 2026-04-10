# Spec: The Barrel House and The Keg Hole — Dangerous Zone Upgrade

**GitHub Issue:** cory-johannsen/mud#34
**Date:** 2026-04-10

## Context
Both rooms are in `content/zones/rustbucket_ridge.yaml` under the `rotgut_alley` sub-area. They currently have `danger_level: sketchy` with a single level-1 NPC each (Scavenger in Barrel House, Ganger in Keg Hole). The adjacent room The Still already has a level-3 Lieutenant, establishing the level ceiling for this sub-area.

---

## REQ-1: The Barrel House — danger level and NPC roster

- REQ-1a: `the_barrel_house` MUST have `danger_level: dangerous`
- REQ-1b: The Barrel House MUST spawn 2–3 combat NPCs of level 2–3
- REQ-1c: NPCs MUST fit the narrative (distillery enforcers / Pete's hired muscle protecting his operation)
- REQ-1d: A new NPC template `barrel_house_enforcer` MUST be created in `content/npcs/combat/rustbucket_ridge.yaml` at level 2, HP ~18, AC 13, primary stat Brutality
- REQ-1e: Respawn timer MUST be 5 minutes

## REQ-2: The Keg Hole — danger level, NPCs, and Boss

- REQ-2a: `the_keg_hole` MUST have `danger_level: dangerous`
- REQ-2b: The Keg Hole MUST spawn 1–2 regular combat NPCs of level 2 in addition to the boss
- REQ-2c: A Boss NPC `big_grizz` MUST be created in `content/npcs/combat/rustbucket_ridge.yaml` at level 4
- REQ-2d: `big_grizz` MUST be flagged as a boss (`is_boss: true`)
- REQ-2e: `big_grizz` stats: HP ~50, AC 15, primary stats Brutality (18) and Grit (16); description consistent with the room lore (former bar bouncer, large, intimidating)
- REQ-2f: `big_grizz` respawn timer MUST be 15 minutes
- REQ-2g: Regular NPC respawn timer MUST be 5 minutes

## REQ-3: No changes to adjacent rooms

- REQ-3a: The Still, and all other rooms in `rotgut_alley`, MUST NOT be modified

## Files to Modify

- `content/zones/rustbucket_ridge.yaml` — update danger levels and spawn lists for `the_barrel_house` and `the_keg_hole`
- `content/npcs/combat/rustbucket_ridge.yaml` — add `barrel_house_enforcer` and `big_grizz` templates
