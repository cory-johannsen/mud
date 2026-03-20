# Housekeeping

Project maintenance and tech-debt items tracked as top-level features.

## TODO List

- [x] TODO list:
    - [x] full itemized list of all stubs / unimplemented code.
      - [x] add implementation items to the appropriate feature category
        - [x] if no feature category exists, add a new feature category
    - [x] missing tests
      - [x] add implementation items to the appropriate feature category
        - [x] if no feature category exists, add a new feature category

## Trainskill Persistence

- [x] `trainskill` does not persist the player selection (the result is gone when the player logs in again and the character sheet shows Pending Skill Increases: 1)

## Grant Editor Command

- [x] `grant` Editor command
  - [x] Accepts a parameter for the type of grant
    - [x] xp: grants a player XP
    - [x] money: grants a player currency
  - [x] Accepts a character name (target must be online)
  - [x] Accepts the amount as an argument (must be > 0)
