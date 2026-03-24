# Downtime Actions

PF2E downtime actions. Requires a downtime time-tracking system.

## Requirements

- [ ] Downtime Actions
  - [ ] Earn Income: Use a skill (Crafting, Lore, Performance) to make money.
    - [ ] Earn Income command — implement `earnincome <skill>` (skill check vs city DC; result determines credits earned per day; requires downtime time-tracking system and city-tier DC table)
  - [ ] Craft: Spend days creating items, equipment, or consumables.
    - [ ] Craft command — implement `craft <item>` (Crafting check vs item DC; costs materials and downtime days; produces item on success; requires item recipe data, material inventory, and downtime time-tracking)
  - [ ] Retrain: Spend a week or more to change a Feat, Skill, or Job feature.
    - [ ] Retrain command — implement `retrain <feat|skill> <old> <new>` (costs downtime days scaled to what is retrained; requires downtime time-tracking and trainer NPC in safe room)
  - [ ] Treat Disease: Spend time caring for an ill patient (Medicine).
    - [ ] Treat Disease command — implement `treatdisease <target>` (Medicine check vs disease DC once per day during downtime; success reduces disease severity; requires disease/condition system and downtime time-tracking)
  - [ ] Subsist: Find food and shelter in the wild or a city for free.
    - [ ] Subsist command — implement `subsist` (Survival or Society check vs zone DC; success covers food/shelter for the week at no cost; failure imposes a condition penalty; requires downtime time-tracking and zone DC table)
  - [ ] Create Forgery: Spend a day or more making a fake document.
    - [ ] Create Forgery command — implement `forgery <document>` (Society check vs DC; costs downtime days; produces a forged document item used in quests or social encounters; requires document item type and downtime time-tracking)
  - [ ] Long-Term Rest: Spend 24 hours to recover double your level in HP.
    - [ ] Long-Term Rest command — implement `longrest` (unavailable in combat or dangerous rooms; costs 24 in-game hours; heals 2×level HP and removes minor conditions; requires in-game time tracking and Resting/safe-room enforcement)
