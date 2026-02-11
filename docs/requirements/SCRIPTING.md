# Scripting Requirements

## Runtime

- SCRIPT-1: The engine MUST embed GopherLua as the scripting runtime.
- SCRIPT-2: Each zone MUST execute Lua scripts in an isolated Lua state (VM instance) to prevent cross-zone interference.
- SCRIPT-3: Lua scripts MUST be sandboxed; access to `os`, `io`, `debug`, and `loadfile` MUST be disabled.
- SCRIPT-4: Lua script execution MUST be time-bounded; scripts exceeding a configurable instruction count MUST be terminated.
- SCRIPT-5: The engine MUST support hot-reloading of Lua scripts at runtime without restarting the server.

## Engine API Surface

- SCRIPT-6: The engine MUST expose the following modules to Lua scripts: `engine.world`, `engine.combat`, `engine.entity`, `engine.inventory`, `engine.event`, `engine.log`, `engine.dice`, `engine.time`.
- SCRIPT-7: `engine.world` MUST provide functions for: querying rooms, moving entities, modifying room properties, and broadcasting messages.
- SCRIPT-8: `engine.combat` MUST provide functions for: initiating combat, resolving actions, applying damage, applying conditions, and querying combatant state.
- SCRIPT-9: `engine.entity` MUST provide functions for: querying and modifying entity attributes, inventory, and status.
- SCRIPT-10: `engine.event` MUST provide functions for: registering event listeners, emitting events, and scheduling delayed events.
- SCRIPT-11: `engine.dice` MUST provide functions for: rolling dice expressions (e.g., "2d6+3") and returning structured results.
- SCRIPT-12: `engine.time` MUST provide functions for: querying the in-game clock, time of day, season, and weather.
- SCRIPT-13: `engine.log` MUST provide functions for structured logging at debug, info, warn, and error levels.

## Script Types

- SCRIPT-14: The engine MUST support room scripts triggered on: player enter, player exit, look, and custom commands.
- SCRIPT-15: The engine MUST support NPC scripts for: HTN operators, dialogue conditions, interaction handlers, and combat AI.
- SCRIPT-16: The engine MUST support item scripts for: use, equip, unequip, and custom interaction effects.
- SCRIPT-17: The engine MUST support ruleset scripts for: character creation steps, level-up logic, skill checks, and combat resolution.
- SCRIPT-18: The engine MUST support global event scripts that respond to server-wide events (time changes, weather changes, world events).

## Error Handling

- SCRIPT-19: Lua script errors MUST be caught and logged without crashing the server or the zone.
- SCRIPT-20: Script errors MUST include the script file path, line number, and a descriptive error message.
- SCRIPT-21: Repeated script errors from the same source MUST trigger rate-limited alerts, not log floods.
