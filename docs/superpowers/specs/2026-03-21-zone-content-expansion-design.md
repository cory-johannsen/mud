# Zone Content Expansion — Design Spec

## Overview

Expands all 16 existing zones with additional rooms, per-room danger level variation, diversified combat NPC types, and improved NPC spawn density. This is purely content/YAML work — no code changes are required. All 16 zone YAML files in `content/zones/` are updated to meet the standards defined in this spec. Each zone also receives a safe cluster serving as the anchor for non-combat NPCs (from non-combat-npcs-all-zones) and the chip_doc NPC (from curse-removal).

## Definitions

- **Zone core rooms:** Rooms designated `core: true` in the zone YAML by the content author. These are the thematically central or most dangerous rooms in the zone (e.g. a gang leader's lair, a restricted facility wing).
- **Zone edge rooms:** All non-safe-cluster rooms that are not designated core.
- **Safe cluster:** The set of 3–5 contiguous Safe rooms serving as the zone's neutral ground.

## Requirements

### Room Count

- REQ-ZCE-1: Every zone MUST have at least 30 rooms after expansion.
- REQ-ZCE-2: New rooms MUST be lore-appropriate to the zone's theme and setting. Lore-appropriateness is determined by the content author with reference to the zone's established NPC types, location name, and danger level.

### Safe Cluster

- REQ-ZCE-3: Every zone MUST contain exactly one safe cluster, regardless of the zone's default danger level.
- REQ-ZCE-4: A safe cluster MUST consist of one anchor Safe room plus 2–4 adjacent Safe rooms, for a total of 3–5 Safe rooms per zone.
- REQ-ZCE-5: All rooms in the safe cluster MUST have danger level `safe`.
- REQ-ZCE-6: Non-combat NPCs (per non-combat-npcs-all-zones) and the chip_doc NPC (per curse-removal) MUST be distributed across the safe cluster rooms such that no single safe cluster room contains more than two non-combat NPCs total, counting chip_doc as one non-combat NPC for the purpose of this cap.

### Danger Level Distribution

- REQ-ZCE-7: Zone edge rooms MUST have a danger level one step toward `safe` from the zone's default danger level, following the sequence (descending toward safe): `all_out_war` → `dangerous` → `sketchy` → `safe`.
- REQ-ZCE-8: Zone core rooms MUST have the zone's default danger level.
- REQ-ZCE-9: A zone whose default danger level is `safe` MUST treat all non-cluster rooms as `safe` (no step-down applies).

### Combat NPC Diversity

- REQ-ZCE-10: Each zone MUST contain at least `floor(room_count / 10) + 2` distinct combat NPC types, where `room_count` is the total number of rooms in the zone after expansion (including safe cluster rooms).
- REQ-ZCE-11: Combat NPC types MUST be lore-appropriate to the zone's theme (e.g. gangers and scavengers in rustbucket_ridge; corporate security and drones in downtown). Lore-appropriateness is determined by the content author with reference to the zone's established theme.

### Combat NPC Spawn Density

- REQ-ZCE-12: Safe rooms MUST have 0 combat NPC spawns.
- REQ-ZCE-13: Sketchy rooms MUST have at least 1 and at most 2 combat NPC spawns.
- REQ-ZCE-14: Dangerous rooms MUST have at least 2 and at most 3 combat NPC spawns.
- REQ-ZCE-15: All Out War rooms MUST have at least 3 and at most 4 combat NPC spawns.
- REQ-ZCE-16: If a zone contains no All Out War rooms, REQ-ZCE-15 does not apply to that zone.

## Design

### Scope

This feature is entirely content work. Each of the 16 zone YAML files in `content/zones/` is edited to:
1. Add rooms until the 30-room minimum is met
2. Designate core rooms with `core: true` and a 3–5 room safe cluster
3. Assign per-room danger levels per REQ-ZCE-7 through REQ-ZCE-9
4. Add or diversify combat NPC spawn entries to meet REQ-ZCE-10 through REQ-ZCE-15

No changes to Go source code, proto definitions, or database schema are required.

### Per-Zone Work

Each zone is treated as an independent content task. The 16 zones are:
`aloha`, `beaverton`, `battleground`, `downtown`, `felony_flats`, `hillsboro`, `lake_oswego`, `ne_portland`, `pdx_international`, `ross_island`, `rustbucket_ridge`, `sauvie_island`, `se_industrial`, `the_couve`, `troutdale`, `vantucky`.

`rustbucket_ridge` (34 rooms) already meets REQ-ZCE-1 and requires only core room designation, safe cluster creation, danger level distribution, NPC diversity, and spawn density updates.

`downtown` (13 rooms) requires the most new rooms (minimum 17 additions).

All 16 zones are held to the same hard minimums. Per-zone exceptions require explicit approval and MUST be documented in a comment in the zone YAML file.

### Dependencies

- `non-combat-npcs-all-zones` — defines which non-combat NPC types are placed in each zone's safe cluster; this spec defers NPC type selection and placement to that feature
- `curse-removal` — defines the chip_doc NPC type; one chip_doc MUST be placed in each zone's safe cluster per REQ-ZCE-6

## Out of Scope

- New zone creation (Clown Camp, SteamPDX, etc.) is covered by the zones-new feature.
- Non-combat NPC type definitions and dialogue are covered by non-combat-npcs and non-combat-npcs-all-zones.
- chip_doc NPC YAML config values (removal_cost, check_dc) are content work belonging to the curse-removal implementation.
- Boss rooms with elevated spawn counts beyond REQ-ZCE-15 are not included in this expansion pass.
- Code changes of any kind are not included.
