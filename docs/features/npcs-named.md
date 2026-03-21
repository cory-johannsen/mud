# Named NPCs

Specific named NPCs to be added to the game world.

Design spec: `docs/superpowers/specs/2026-03-21-npcs-named-design.md`

## Requirements

- [ ] REQ-NN-0: Implementation MUST NOT proceed until the `non-combat-npcs` feature has added `NpcRole string` with yaml tag `npc_role` to `npc.Template`.
- [ ] REQ-NN-1: All three NPC templates MUST set `disposition: friendly`.
- [ ] REQ-NN-2: All three NPC templates MUST set `npc_role: merchant`.
- [ ] REQ-NN-3: All three NPC templates MUST set `respawn_delay: 0s`.
- [ ] REQ-NN-4: All three NPC templates MUST NOT define weapon or armor fields.
- [ ] REQ-NN-5: All three NPC template files MUST include a YAML comment marking each as a future quest giver pending the `quests` feature.
- [ ] REQ-NN-6: All three NPC templates MUST set `type: human` and `loot` to credits only (no item drops).
- [ ] REQ-NN-7: `wayne_dawgs_trailer` MUST be updated to `danger_level: safe`.
- [ ] REQ-NN-8: `wayne_dawgs_trailer` description MUST be updated to the text specified in the spec Section 2.1.
- [ ] REQ-NN-9: `wayne_dawgs_trailer` MUST gain spawn entries for both `wayne_dawg` and `jennifer_dawg`, each with `count: 1` and `respawn_delay: 0s`.
- [ ] REQ-NN-10: `wayne_dawgs_trailer` MUST gain a west exit to `dwayne_dawgs_trailer`.
- [ ] REQ-NN-11: A new room `dwayne_dawgs_trailer` MUST be added to `rustbucket_ridge.yaml` at `map_x: -4`, `map_y: 4` with `danger_level: safe`.
- [ ] REQ-NN-12: `dwayne_dawgs_trailer` MUST have an east exit to `wayne_dawgs_trailer` and a spawn entry for `dwayne_dawg` with `count: 1` and `respawn_delay: 0s`.
- [ ] REQ-NN-13: `map_x: -4`, `map_y: 4` MUST NOT overlap with any existing room in `rustbucket_ridge.yaml`.
- [ ] REQ-NN-14: NPC template files MUST be placed directly in `content/npcs/`, not a subdirectory.
- [ ] REQ-NN-15: Each template MUST have a unique `id` matching the filenames: `wayne_dawg`, `jennifer_dawg`, `dwayne_dawg`.
- [ ] REQ-NN-16: Each template MUST include lore-appropriate `taunts` reflecting the character's personality.
