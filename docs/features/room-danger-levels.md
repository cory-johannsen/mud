# Room Danger Levels

Rooms are classified as Safe, Sketchy, Dangerous, or All Out War. Danger level drives combat rules, trap probabilities, NPC behavior, and map display. See `docs/superpowers/specs/2026-03-20-room-danger-levels-design.md` for the full design spec.

## Requirements

- [ ] Room danger levels
  - [x] DangerLevel enum: Safe, Sketchy, Dangerous, AllOutWar
    - REQ-DL-1: Zones MUST declare a `danger_level` field; rooms MAY override it.
  - [ ] Combat enforcement per danger level:
    - Safe: No combat. First violation = warning. Second violation = WantedLevel++ + guard initiates combat.
      - REQ-DL-2: Safe room second violation MUST increment WantedLevel and trigger guard combat.
    - Sketchy: Players may initiate combat; NPCs do not initiate.
    - Dangerous: All parties may initiate combat.
    - All Out War: Combat NPCs attack on sight.
  - [ ] WantedLevel system (5 levels):
    - 0=None, 1=Flagged, 2=Burned, 3=Hunted, 4=D.O.S.
    - REQ-DL-3: WantedLevel MUST decay by 1 level per in-game day when no new violations occur.
    - REQ-DL-4: Active clearing (bribe, quest, surrender) MUST be handled by the `wanted-clearing` feature.
    - REQ-DL-5: WantedLevel 1 MUST cause merchants to add a 10% surcharge to all transactions.
    - REQ-DL-6: WantedLevel 2+ MUST cause guards to initiate combat to detain.
    - REQ-DL-7: WantedLevel 3-4 MUST cause guards to attack on sight.
  - [ ] Trap probabilities by danger level:
    - Safe: 0% room trap / 0% cover trap
    - Sketchy: 0% room trap / 15% cover trap
    - Dangerous: 35% room trap / 50% cover trap
    - All Out War: 60% room trap / 75% cover trap
    - REQ-DL-8: Zone YAML MAY override default trap probabilities.
  - [ ] Map display:
    - REQ-DL-9: Room map cells MUST be color-coded by danger level (Safe=green, Sketchy=yellow, Dangerous=orange, AllOutWar=red).
    - REQ-DL-10: Unexplored rooms MUST display as light gray. Explored state MUST be tracked per player on the character record.
  - [ ] Cover display: items usable as cover MUST include cover tier info in room equipment descriptions.
