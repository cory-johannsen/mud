# Downtime

Downtime time-tracking system and actions — players spend in-game days on activities between adventures (earning income, crafting, retraining, treating disease, etc.). Requires the Persistent Calendar for time tracking.

## Requirements

- [ ] Downtime time-tracking system
  - [ ] `downtime` command — enter downtime mode; declare activity and number of days; advance the in-game calendar by that many days
  - [ ] Earn Income — skill check (Crafting, Lore, Performance) vs city DC; result determines credits earned per day
  - [ ] Craft — Crafting check vs item DC; costs materials and downtime days; produces item on success (requires item recipe data and material inventory)
  - [ ] Retrain — change a Feat, Skill, or Job feature; costs downtime days scaled to what is retrained (requires trainer NPC in safe room)
  - [ ] Treat Disease — Medicine check vs disease DC once per day; success reduces disease severity (requires disease/condition system)
  - [ ] Subsist — Survival or Society check vs zone DC; success covers food/shelter at no cost; failure imposes a condition penalty
  - [ ] Create Forgery — Society check vs DC; costs downtime days; produces a forged document item (requires document item type)
  - [ ] Long-Term Rest — 24 in-game hours; heals 2×level HP and removes minor conditions (requires safe-room enforcement)
  - [ ] Refocus — 10 in-game minutes; restores 1 Focus Point (requires Focus Point system)
