# Character System Requirements

## Accounts

- CHAR-1: Players MUST authenticate with a username and password.
- CHAR-2: Passwords MUST be stored using bcrypt or argon2 hashing; plaintext passwords MUST NOT be stored.
- CHAR-3: Each account MUST support multiple characters.
- CHAR-4: A player MUST select one character to play per active session.
- CHAR-5: Account-level data (credentials, preferences, character list) MUST be persisted to PostgreSQL.

## Character Builder

- CHAR-6: The engine MUST provide a character creation flow as an in-game interactive process.
- CHAR-7: The character builder MUST be driven by the active ruleset; the engine MUST NOT hardcode creation steps.
- CHAR-8: The character builder MUST expose its state and transitions via the gRPC API for external UI consumption.
- CHAR-9: The character builder MUST validate all choices against the ruleset before finalizing a character.
- CHAR-10: The character builder MUST support step-by-step creation with the ability to go back and revise choices.

## PF2E Character Model (Default Ruleset — Gunchete)

- CHAR-11: All player characters MUST be human; non-human PF2E ancestries MUST NOT be available.
- CHAR-12: The PF2E ancestry system MUST be repurposed as a home region selection, representing the character's origin within post-collapse Portland.
- CHAR-13: Home region MUST be the first character creation step.
- CHAR-14: Each home region MUST define: ability score modifiers, a narrative description, and a list of selectable regional traits.
- CHAR-15: Home regions MUST be defined in YAML data files following the structure: name, description, modifiers (across PF2E ability scores), and traits array.
- CHAR-16: The default ruleset MUST implement classes as the second character creation step.
- CHAR-17: Each class MUST define: key ability, hit points per level, proficiencies, class features, and feat progression.
- CHAR-18: Classes MUST be defined in YAML data files with Lua scripts for class feature behavior.
- CHAR-19: The default ruleset MUST implement the PF2E ability score system (six abilities: Strength, Dexterity, Constitution, Intelligence, Wisdom, Charisma).
- CHAR-20: The default ruleset MUST implement the PF2E proficiency system (untrained, trained, expert, master, legendary).
- CHAR-21: The default ruleset MUST implement skills as defined by PF2E, with skill increases tied to level progression.

## Character Persistence

- CHAR-22: All character data MUST be persisted to PostgreSQL.
- CHAR-23: Character state MUST be saved on disconnect and at configurable intervals during play.
- CHAR-24: Character data MUST include: attributes, inventory, location, conditions, quest state, and all ruleset-defined properties.

## Character Progression

- CHAR-25: The ruleset MUST define a leveling and experience system.
- CHAR-26: Level-up MUST present choices (feats, skill increases, ability boosts) driven by the ruleset via the character builder interface.
- CHAR-27: The engine MUST support character progression events that Lua scripts and rulesets can observe and react to.
