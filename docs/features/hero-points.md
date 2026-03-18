# Hero Points

Tracks hero points per player, awarding them on milestone events, and allowing rerolls or stabilization.

## Requirements

- [x] Hero points
  - [x] Add `HeroPoints int` field to PlayerSession; award 1 point at session start and on milestone events (level up, boss kill, GM grant)
  - [x] `heropoint reroll` — re-roll the most recent skill or attack check and take the higher result; costs 1 hero point; unavailable if no recent check
  - [x] `heropoint stabilize` — when in a dying state, stabilize at 0 HP; costs 1 hero point
  - [x] Display current hero point count on the character sheet
