# Gear Actions

PF2E gear-related actions: Repair and Affix Material. Activate Item and Swap are already complete (see `actions.md`). Blocked by `equipment-mechanics`.

## Requirements

- [ ] Gear Actions (blocked by `equipment-mechanics`)
  - [ ] Repair: Spend 10 minutes (with a Repair Kit) to fix a damaged item.
    - [ ] Repair command — implement `repair <item>` (Crafting check vs item Hardness DC; costs 10 in-game minutes and a Repair Kit consumable; restores item to functioning; **requires `equipment-mechanics`: item durability/broken state and Repair Kit item type**)
  - [ ] Affix a Precious Material: Add specialized materials to gear.
    - [ ] Affix Material command — implement `affix <material> <item>` (Crafting check vs material DC; permanently upgrades item with material properties; **requires `equipment-mechanics`: precious material item type and item upgrade slots**)
