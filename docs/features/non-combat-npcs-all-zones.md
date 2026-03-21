# Non-Combat NPCs — All Zones

Places one lore-appropriate instance of each required non-combat NPC type in every zone. Each zone receives a safe room (or uses an existing one) as the NPC hub. All NPC templates are defined in `content/npcs/non_combat/` YAML files. This is a content/data feature — all NPC behavior is implemented by `non-combat-npcs`.

Design spec: `docs/superpowers/specs/2026-03-21-non-combat-npcs-all-zones-design.md`

## Requirements

- [ ] REQ-NCNAZ-0: The `banker` npc_role MUST be implemented in `non-combat-npcs` before this feature can be implemented.
- [ ] REQ-NCNAZ-1: Every zone MUST have at least one room with `danger_level: safe` before non-combat NPC spawns are placed.
- [ ] REQ-NCNAZ-2: New safe rooms MUST be connected bidirectionally to the anchor room listed in the spec using the specified exit direction pair.
- [ ] REQ-NCNAZ-3: New safe rooms MUST have a `description` field matching the lore descriptions in the spec.
- [ ] REQ-NCNAZ-4: All four core NPC types (merchant, healer, job_trainer, banker) MUST be present in every zone's safe room.
- [ ] REQ-NCNAZ-5: Optional NPC types MUST be present only in zones listed in the spec Section 2.2.
- [ ] REQ-NCNAZ-6: `quest_giver` and `crafter` NPC templates MUST NOT be placed in any zone until `quests` and `crafting` features are implemented respectively.
- [ ] REQ-NCNAZ-7: Template IDs MUST follow the `<zone_id>_<npc_role>` pattern.
- [ ] REQ-NCNAZ-8: All non-combat NPC templates MUST set `respawn_after: 0s`.
- [ ] REQ-NCNAZ-9: All non-combat NPC templates MUST set `disposition: neutral`.
- [ ] REQ-NCNAZ-10: Each template MUST have a unique lore-appropriate `name` and `description` matching the zone's theme.
- [ ] REQ-NCNAZ-11: Non-combat NPC template files MUST be located at `content/npcs/non_combat/<zone_id>.yaml`.
- [ ] REQ-NCNAZ-12: New safe room `map_x`/`map_y` coordinates MUST NOT overlap with any existing room in the zone. Coordinates MUST be adjacent to the anchor room in the direction specified in the spec.
- [ ] REQ-NCNAZ-13: The anchor room MUST gain an exit in the specified direction to the new safe room. The new safe room MUST have the reverse direction exit back to the anchor room.
