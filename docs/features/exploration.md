# Exploration

Exploration mode tracking for between-combat activity — player declares an exploration activity (Avoid Notice, Defend, Detect Magic, Search, Scout, etc.) which applies ongoing effects on room entry and at combat start.

## Requirements

- [ ] Exploration mode tracking
  - [ ] `explore` command — declare or change current exploration activity; display current activity in prompt/room
  - [ ] Avoid Notice — roll Stealth for initiative; apply hidden condition on combat start if successful
  - [ ] Defend — auto-apply shield_raised condition at combat start (requires shield equipped)
  - [ ] Detect Magic — on room entry, emit notification if magical auras or activatable items are present
  - [ ] Search — on room entry, secret Perception check to reveal hidden exits, traps, or items
  - [ ] Scout — all party members gain +1 circumstance bonus to Initiative on combat start (requires multi-player party)
  - [ ] Follow the Expert — use ally's skill rank in place of player's for chosen skill (requires multi-player party)
  - [ ] Investigate — on room entry, secret Recall Knowledge check to surface lore about NPCs, items, or zone history
