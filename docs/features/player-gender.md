# Player Gender

Players select their gender at character creation, with options including custom input and random selection.

## Requirements

- [x] Player gender
  - [x] Allow the player to select their gender at creation time.  Allow for random selection.
    - Male
    - Female
    - Non-binary
    - Indeterminate
    - Other (player enters)
    - [x] Add `gender` field (string enum: male, female, non-binary, indeterminate, custom) to character data model and DB schema
    - [x] Add gender selection step to character creation flow: present numbered options; option 5 prompts for custom text input; option 0 randomizes
  - [x] Backfill missing gender at player load
    - [x] On player load, if gender field is null or empty, assign a random gender value and persist it
