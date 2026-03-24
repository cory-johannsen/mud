# Exploration Actions

PF2E exploration-mode actions. Requires an exploration mode tracking system.

## Requirements

- [ ] Exploration Actions
  - [ ] Avoid Notice: Use Stealth to roll for initiative and start the fight hidden.
    - [ ] Avoid Notice mode — player may declare `avoid notice` during exploration; on combat start, roll Stealth vs NPC Perception for initiative and apply hidden condition if successful; requires exploration mode tracking
  - [ ] Defend: Move with your shield up (start combat with Raise a Shield active).
    - [ ] Defend mode — player may declare `defend` during exploration; on combat start automatically apply shield_raised condition (requires shield equipped); requires exploration mode tracking
  - [ ] Detect Magic: Constantly scan for magical auras while moving.
    - [ ] Detect Magic mode — player may declare `detect magic` during exploration; rooms with magical auras or items emit a notification on entry; requires magic aura flag on rooms/items and exploration mode tracking
  - [ ] Search: Meticulously look for secret doors and traps (free secret Perception checks).
    - [ ] Search mode — player may declare `search` during exploration; on room entry make a secret Perception check to reveal hidden exits, traps, or items; requires hidden exit/trap/item flags on rooms and exploration mode tracking
  - [ ] Scout: Give your whole party a +1 circumstance bonus to Initiative.
    - [ ] Scout mode — player may declare `scout` during exploration; all party members gain +1 circumstance bonus to Initiative rolls when combat starts; requires multi-player party system and exploration mode tracking
  - [ ] Follow the Expert: A high-level ally helps you with a skill you aren't good at.
    - [ ] Follow the Expert mode — player may declare `follow <ally>` during exploration; ally's skill rank is used in place of the player's for the chosen skill while exploring; requires multi-player party system and exploration mode tracking
  - [ ] Investigate: Use Recall Knowledge (Arcana, Society, etc.) while traveling.
    - [ ] Investigate mode — player may declare `investigate` during exploration; on room entry make a secret Recall Knowledge check to surface lore about NPCs, items, or zone history; requires lore/knowledge flags on rooms and NPCs and exploration mode tracking
  - [ ] Refocus: Spend 10 minutes to regain a Focus Point.
    - [ ] Refocus command — implement `refocus` (unavailable in combat; costs in-game time; restores 1 Focus Point; requires Focus Point system and in-game time tracking)
