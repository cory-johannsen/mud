# NPC AI Requirements

## HTN Architecture

- AI-1: NPC behavior MUST be driven by Hierarchical Task Network (HTN) planning.
- AI-2: The engine MUST provide an HTN planner that evaluates world state and produces action sequences for NPCs.
- AI-3: HTN domains (task decompositions and methods) MUST be defined in YAML with Lua condition and operator functions.
- AI-4: The HTN planner MUST re-plan when world state changes invalidate the current plan.
- AI-5: HTN planning MUST execute within the zone tick budget; plans that exceed a configurable time limit MUST be deferred to the next tick.

## NPC Definition

- AI-6: NPCs MUST be defined in YAML data files specifying: identity, stats, inventory, dialogue, and HTN domain reference.
- AI-7: NPCs MUST be instances of templates; multiple NPCs MAY share the same template with per-instance overrides.
- AI-8: NPC stat blocks MUST conform to the active ruleset's entity schema.

## Behavior Systems

- AI-9: NPCs MUST support sensory awareness: detecting players and other entities within a configurable perception range.
- AI-10: NPCs MUST support multiple behavioral states (idle, patrol, combat, flee, interact) driven by HTN plan output.
- AI-11: NPCs in combat MUST participate in the same round-based action economy as players.
- AI-12: NPC combat behavior MUST be governed by HTN domains that select actions, targets, and positioning.

## Dialogue and Interaction

- AI-13: NPCs MUST support dialogue trees defined in YAML with Lua-scripted branching conditions.
- AI-14: NPC dialogue MUST support conditional branches based on quest state, character attributes, and world state.
- AI-15: NPCs MUST support interaction hooks (talk, trade, quest-give, quest-turn-in) triggerable by players.

## Performance

- AI-16: The AI system MUST support at least 1000 active NPCs across all zones without degrading the server tick rate below target.
- AI-17: NPC planning MUST be distributed across zone processing units; each zone MUST manage its own NPC population.
- AI-18: Idle NPCs in unoccupied zones SHOULD reduce their planning frequency to conserve resources.
