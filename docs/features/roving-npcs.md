# Multi-Zone Roving NPCs

## Overview

Boss-tier combat NPCs traverse multi-zone routes autonomously. A `RovingConfig` block on the NPC template defines the ordered route of room IDs, travel interval, and exploration probability. The `RovingManager` background engine drives movement, fires room entry/exit notifications, and enforces the rule that only `tier: boss` combat NPCs may have a roving config.

## Requirements

- ROVING-1: The `Template` struct MUST include a `RovingConfig` field (`roving`, omitempty) of type `*RovingConfig`.
- ROVING-2: `RovingConfig` MUST contain `route []string`, `travel_interval string`, and `explore_probability float64`.
- ROVING-3: `Template.Validate()` MUST reject roving combat NPCs that do not have `tier: boss`.
- ROVING-4: `travel_interval` MUST be a valid Go duration string; empty defaults to `5m`.
- ROVING-5: `explore_probability` MUST be in the range `[0, 1]`.
- ROVING-6: The `RovingManager` MUST move each roving NPC along its route at the configured interval.
- ROVING-7: The `RovingManager` MUST broadcast room entry and exit notifications when an NPC moves.
- ROVING-8: The `RovingManager` MUST be wired into the gameserver startup sequence.

## NPC Content

- `content/npcs/dayton_james_weber.yaml` — The Cornhole Quad, level-10 boss roving across Southeast Industrial and Cornhole Shrimp Plaza zones.
- `content/ai/cornhole_quad_combat.yaml` — HTN domain for Dayton's combat behaviors.
- `content/items/cornhole_sack.yaml` — Exploding cornhole sack item used as loot and in the AI domain.
