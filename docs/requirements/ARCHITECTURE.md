# Architecture Requirements

## Engine–Game Separation

- ARCH-1: The core engine MUST be decoupled from any specific ruleset or setting.
- ARCH-2: Rulesets MUST be loadable as pluggable modules that register with the engine at startup.
- ARCH-3: Settings (world content, lore, items, NPCs) MUST be loadable as pluggable modules independent of the ruleset.
- ARCH-4: The engine MUST define stable interfaces that rulesets and settings implement; the engine MUST NOT depend on concrete game logic.
- ARCH-5: A single engine instance MUST be capable of running exactly one ruleset and one setting at a time.
- ARCH-6: Swapping a ruleset or setting MUST NOT require modifications to the engine source code.

## Pitaya Integration

- ARCH-7: The engine MUST use Pitaya frontend servers to handle inbound client connections (Telnet, WebSocket).
- ARCH-8: The engine MUST use Pitaya backend servers for game logic processing (world simulation, combat, AI).
- ARCH-9: The engine MUST use Pitaya's clustering mode with etcd for service discovery in production deployments.
- ARCH-10: The engine MUST use Pitaya's RPC layer (gRPC or NATS) for inter-server communication in clustered deployments.
- ARCH-11: The engine MUST support Pitaya's standalone mode for single-server development and testing.
- ARCH-12: The engine MUST use Pitaya's session management for tracking connected players.
- ARCH-13: The engine MUST use Pitaya's group system for broadcasting messages to rooms, zones, and chat channels.
- ARCH-14: The engine MUST use Pitaya's pipeline system for middleware (authentication, rate limiting, command routing).

## Data-Driven Design

- ARCH-15: Game entities (items, NPCs, abilities, rooms) MUST be defined in YAML data files.
- ARCH-16: YAML data files MUST be loaded at server startup and used to seed the database when content is new or updated.
- ARCH-17: Runtime state MUST be persisted to PostgreSQL; YAML files MUST serve as the canonical source for content definitions.
- ARCH-18: The engine MUST support hot-reloading of Lua scripts without server restart.

## Scripting Integration

- ARCH-19: The engine MUST expose a Lua scripting API that rulesets and settings use to define game behavior.
- ARCH-20: Lua scripts MUST be sandboxed; scripts MUST NOT access the filesystem, network, or OS facilities directly.
- ARCH-21: The engine MUST provide Lua bindings for core systems: combat, movement, inventory, NPC behavior, and events.
- ARCH-22: YAML data files and Lua scripts MUST work together: YAML defines structure and static data, Lua defines behavior and logic.

## Concurrency Model

- ARCH-23: The engine MUST process each zone as an independent unit of concurrency.
- ARCH-24: Cross-zone interactions MUST be mediated through message passing, not shared mutable state.
- ARCH-25: The game loop MUST run on a fixed tick rate configurable per deployment.
