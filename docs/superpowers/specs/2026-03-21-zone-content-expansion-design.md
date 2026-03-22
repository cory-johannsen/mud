# Zone Content Expansion — Design Spec

## Overview

Expands all 16 existing zones with additional rooms, per-room danger level variation, diversified combat NPC types, and improved NPC spawn density. This is purely content/YAML work — no code changes are required. All 16 zone YAML files in `content/zones/` are updated to meet the standards defined in this spec. Each zone also receives a safe cluster serving as the anchor for non-combat NPCs (from non-combat-npcs-all-zones) and the chip_doc NPC (from curse-removal).

## Requirements

### Room Count

- REQ-ZCE-1: Every zone MUST have at least 30 rooms after expansion.
- REQ-ZCE-2: New rooms MUST be lore-appropriate to the zone's theme and setting.

### Safe Cluster

- REQ-ZCE-3: Every zone MUST contain exactly one safe cluster, regardless of the zone's default danger level.
- REQ-ZCE-4: A safe cluster MUST consist of one anchor Safe room plus 2–4 adjacent Safe rooms, for a total of 3–5 Safe rooms per zone.
- REQ-ZCE-5: All rooms in the safe cluster MUST have danger level `safe`.
- REQ-ZCE-6: Non-combat NPCs (per non-combat-npcs-all-zones) and the chip_doc NPC (per curse-removal) MUST be distributed across the safe cluster rooms rather than concentrated in a single room.

### Danger Level Distribution

- REQ-ZCE-7: Rooms in the outer ring of a zone (those farthest from the zone core by map adjacency) MUST have a danger level one step below the zone's default danger level.
- REQ-ZCE-8: Rooms in the zone core MUST have the zone's default danger level.
- REQ-ZCE-9: The danger level step-down from REQ-ZCE-7 MUST follow the sequence: `safe` → `sketchy` → `dangerous` → `all_out_war`. A zone whose default is `safe` has no step-down; all non-cluster rooms are `safe`.

### Combat NPC Diversity

- REQ-ZCE-10: Each zone MUST contain at least `floor(room_count / 10) + 2` distinct combat NPC types, where `room_count` is the zone's final room count after expansion.
- REQ-ZCE-11: Combat NPC types MUST be lore-appropriate to the zone's theme (e.g. gangers and scavengers in rustbucket_ridge; corporate security and drones in downtown).

### Combat NPC Spawn Density

- REQ-ZCE-12: Safe rooms MUST have 0 combat NPC spawns.
- REQ-ZCE-13: Sketchy rooms MUST have exactly 1 combat NPC spawn.
- REQ-ZCE-14: Dangerous rooms MUST have exactly 2 combat NPC spawns.
- REQ-ZCE-15: All Out War rooms MUST have exactly 3 combat NPC spawns.

## Design

### Scope

This feature is entirely content work. Each of the 16 zone YAML files in `content/zones/` is edited to:
1. Add rooms until the 30-room minimum is met
2. Designate a 3–5 room safe cluster
3. Assign per-room danger levels per REQ-ZCE-7 through REQ-ZCE-9
4. Add or diversify combat NPC spawn entries to meet REQ-ZCE-10 through REQ-ZCE-15

No changes to Go source code, proto definitions, or database schema are required.

### Per-Zone Work

Each zone is treated as an independent content task. The 16 zones are:
`aloha`, `beaverton`, `battleground`, `downtown`, `felony_flats`, `hillsboro`, `lake_oswego`, `ne_portland`, `pdx_international`, `ross_island`, `rustbucket_ridge`, `sauvie_island`, `se_industrial`, `the_couve`, `troutdale`, `vantucky`.

`rustbucket_ridge` (34 rooms) already meets REQ-ZCE-1 and requires only danger level distribution, safe cluster designation, NPC diversity, and spawn density updates.

`downtown` (13 rooms) requires the most new rooms (minimum 17 additions).

### Dependencies

- `non-combat-npcs-all-zones` — defines which non-combat NPC types are placed in each zone's safe cluster; this spec defers NPC type selection and placement to that feature
- `curse-removal` — defines the chip_doc NPC type; one chip_doc MUST be placed in each zone's safe cluster per REQ-ZCE-6

## Out of Scope

- New zone creation (Clown Camp, SteamPDX, etc.) is covered by the zones-new feature.
- Non-combat NPC type definitions and dialogue are covered by non-combat-npcs and non-combat-npcs-all-zones.
- chip_doc NPC YAML config values (removal_cost, check_dc) are content work belonging to the curse-removal implementation.
- Code changes of any kind are not included.
