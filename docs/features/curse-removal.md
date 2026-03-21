# Curse Removal

Adds a `chip_doc` non-combat NPC type that can remove cursed items from players for a credit cost and Rigging skill check. Depends on `equipment-mechanics` (which defines the cursed modifier) and `non-combat-npcs` (which provides the NPC type framework).

## Requirements

- [ ] `chip_doc` NPC type added to the non-combat NPC system
  - [ ] NPC type config: `ChipDocConfig { removal_cost int; check_dc int }`
  - [ ] Personality: cower (same as job_trainer)
  - [ ] Player-facing name: "Chip Doc" or lore-appropriate variant per zone
- [ ] `uncurse <item>` command
  - [ ] Must be in the same room as a `chip_doc` NPC
  - [ ] Fails if item is not equipped and cursed
  - [ ] Deducts `removal_cost` credits
  - [ ] Rigging skill check vs `check_dc`
  - [ ] Critical Success / Success: curse removed; item unequipped and returned to inventory as a `defective` modifier item (curse becomes defective)
  - [ ] Failure: credits lost; item remains cursed
  - [ ] Critical Failure: credits lost; item remains cursed; `fatigued` condition applied to player
- [ ] Each zone MUST have a lore-appropriate `chip_doc` NPC instance in a Safe room
