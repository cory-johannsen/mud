# Persistence Requirements

## PostgreSQL

- PERS-1: PostgreSQL MUST be the sole persistent data store for runtime state.
- PERS-2: The database schema MUST be managed via versioned migrations applied at startup.
- PERS-3: Migrations MUST be forward-only and idempotent; rollback MUST be handled by compensating migrations.
- PERS-4: The engine MUST use a connection pool with configurable minimum and maximum connections.
- PERS-5: All database operations MUST use parameterized queries; string concatenation for query building MUST NOT be used.

## Data Model

- PERS-6: The database MUST store: accounts, characters, inventory, world state, zone state, NPC state, quest state, guild data, and economy data.
- PERS-7: Character data MUST be saved on disconnect, on zone transition, and at a configurable periodic interval.
- PERS-8: Zone state MUST be persisted at a configurable interval and on server shutdown.
- PERS-9: The persistence layer MUST support transactional writes for operations that span multiple tables.

## YAML Data Files

- PERS-10: YAML data files MUST serve as the canonical source of truth for content definitions (ancestries, classes, items, NPCs, zones, rooms, abilities).
- PERS-11: The engine MUST load YAML data files at startup and reconcile them with the database.
- PERS-12: When a YAML definition changes, the engine MUST apply the update to the database during the next startup or reload.
- PERS-13: YAML data files MUST support schema validation; the engine MUST reject files that fail validation.
- PERS-14: YAML data files MUST be organized by content type in a directory hierarchy defined by the setting and ruleset modules.

## Caching

- PERS-15: Frequently accessed read-heavy data (room definitions, item templates, NPC templates) MUST be cached in memory.
- PERS-16: The cache MUST be invalidated when underlying data changes (database update, YAML reload, or Lua script reload).
- PERS-17: Cache size MUST be configurable and bounded.

## Backup and Recovery

- PERS-18: The deployment MUST support automated database backups via standard PostgreSQL tooling (pg_dump, WAL archiving).
- PERS-19: The engine MUST log all state-altering operations at a level sufficient for post-incident reconstruction.
