# Combat System Requirements

## Core Model

- COMBAT-1: The combat system MUST be defined by the active ruleset, not hardcoded in the engine.
- COMBAT-2: The engine MUST provide a combat framework that rulesets extend via interfaces and Lua scripts.
- COMBAT-3: The engine's combat framework MUST support round-based combat with a configurable round duration timer.

## PF2E Action Economy (Default Ruleset)

- COMBAT-4: The default ruleset MUST implement a three-action economy per round, adapted from Pathfinder Second Edition.
- COMBAT-5: Each combat round MUST have a configurable real-time duration (default: 6 seconds).
- COMBAT-6: Players MUST be able to queue up to three actions during a round.
- COMBAT-7: Actions not submitted before the round timer expires MUST be forfeited.
- COMBAT-8: The default ruleset MUST support action types: single actions, two-action activities, three-action activities, free actions, and reactions.
- COMBAT-9: Reactions MUST be triggerable outside of a player's turn, within the same round.

## Initiative and Turns

- COMBAT-10: Combat MUST begin with an initiative roll to determine action resolution order.
- COMBAT-11: All participants MUST submit their actions during the round window; actions MUST resolve in initiative order.
- COMBAT-12: NPCs MUST submit actions through the AI system within the same round window as players.

## Combat Resolution

- COMBAT-13: The ruleset MUST define attack rolls, damage calculations, saving throws, and armor class via Lua scripts and YAML data.
- COMBAT-14: The combat system MUST support the PF2E four-tier success scale: critical failure, failure, success, critical success.
- COMBAT-15: The combat system MUST support conditions (stunned, frightened, prone, etc.) that modify actions and rolls.
- COMBAT-16: Conditions MUST be defined in YAML with duration tracking and Lua hooks for application and removal.

## Engagement Rules

- COMBAT-17: Combat MUST be entered explicitly via hostile action or scripted trigger.
- COMBAT-18: Players MUST be able to attempt to flee combat, subject to ruleset-defined mechanics.
- COMBAT-19: Combat MUST support multiple combatant groups (more than two sides).
- COMBAT-20: PvP combat MUST be supported and governed by the same combat rules as PvE.

## Firearms and Explosives (Default Ruleset — Gunchete)

- COMBAT-21: The default ruleset MUST support firearms as a ranged weapon category with mechanics for: damage dice, range increments, reload actions, magazine capacity, and weapon traits.
- COMBAT-22: Reload MUST consume actions from the three-action economy (e.g., a 1-action reload, a 2-action reload) as defined per weapon in YAML.
- COMBAT-23: Firearms MUST support firing modes (single, burst, automatic) where applicable, each with defined action costs and effects.
- COMBAT-24: The default ruleset MUST support explosives with area-of-effect damage, saving throws, and fuse/trigger mechanics.
- COMBAT-25: Explosive area effects MUST affect all entities in the targeted room or a subset of entities as defined by the weapon's area type.
- COMBAT-26: All firearm and explosive definitions MUST be data-driven in YAML, extending the PF2E weapon trait and property system.

## Dice and Randomness

- COMBAT-27: The engine MUST provide a dice-rolling subsystem supporting standard polyhedral dice (d4, d6, d8, d10, d12, d20, d100).
- COMBAT-28: Dice rolls MUST use a cryptographically secure random number generator.
- COMBAT-29: All dice rolls MUST be logged with the roll expression, result, and applicable modifiers for auditability.
