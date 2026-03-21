# NPC Awareness Terminology Alignment

Rename the `perception` field on NPC templates and instances to `awareness`, aligning NPC terminology with the player-facing Awareness stat. Eliminates the ambiguity where players use Awareness and NPCs use Perception for the same concept.

## Requirements

- REQ-AWR-1: The `perception` field on `npc.Template` MUST be renamed to `awareness`.
- REQ-AWR-2: The `perception` field on `npc.Instance` MUST be renamed to `awareness`.
- REQ-AWR-3: All NPC YAML files MUST use `awareness:` in place of `perception:`.
- REQ-AWR-4: All code references to `Template.Perception` and `Instance.Perception` MUST be updated to `Template.Awareness` and `Instance.Awareness`.
- REQ-AWR-5: All documentation, specs, and plans that reference NPC `perception` as a stat field MUST be updated to use `awareness`.
- REQ-AWR-6: The term "Perception" MUST NOT appear in any player-facing output; all player-visible text MUST use "Awareness".
