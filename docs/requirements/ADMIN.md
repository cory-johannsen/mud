# Admin and Builder Requirements

## Permission Model

- ADMIN-1: The engine MUST implement a role-based permission system for administrative access.
- ADMIN-2: The engine MUST define at minimum three tiers: player, builder, and administrator.
- ADMIN-3: Permissions MUST be granular; each administrative command MUST require a specific permission.
- ADMIN-4: Role assignments MUST be persisted to PostgreSQL and modifiable at runtime by administrators.

## In-Game Admin Commands

- ADMIN-5: Administrators MUST have access to in-game commands for: player management (kick, ban, mute, teleport), server status, and configuration.
- ADMIN-6: Administrators MUST be able to inspect and modify entity state (players, NPCs, items) via in-game commands.
- ADMIN-7: Administrators MUST be able to broadcast server-wide messages.
- ADMIN-8: All administrative actions MUST be logged with the acting administrator, the action taken, and a timestamp.

## Builder Tools

- ADMIN-9: Builders MUST be able to create, edit, and delete rooms via in-game commands.
- ADMIN-10: Builders MUST be able to create, edit, and delete NPCs via in-game commands.
- ADMIN-11: Builders MUST be able to create, edit, and delete items via in-game commands.
- ADMIN-12: Builders MUST be able to define and modify zone properties via in-game commands.
- ADMIN-13: Builder changes MUST be persisted to the database immediately.
- ADMIN-14: Builders MUST be able to export their creations to YAML data files for version control.
- ADMIN-15: Builder commands MUST support an undo operation for the most recent change.

## Scripting Access

- ADMIN-16: Builders MUST be able to attach and detach Lua scripts to rooms, NPCs, and items via in-game commands.
- ADMIN-17: Builders MUST be able to test Lua scripts in a sandboxed environment before deploying them to live zones.
- ADMIN-18: The engine MUST provide a Lua REPL accessible to administrators for debugging and ad-hoc scripting.

## External Interfaces (Future Phases)

- ADMIN-19: The gRPC API MUST expose administrative and builder operations for consumption by external web UIs and GUI tools.
- ADMIN-20: Administrative operations exposed via gRPC MUST enforce the same permission model as in-game commands.
