---
name: mud-overview
description: Orientation skill for the MUD codebase — load at session start
type: reference
---

## What This Repo Is

A two-binary Go system: `cmd/frontend` (telnet server) and `cmd/gameserver` (gRPC service). The frontend accepts raw TCP/telnet connections from players, handles authentication and character selection via PostgreSQL, then enters a bidirectional gRPC streaming session with the gameserver for all game commands and events. The gameserver runs all game logic, drives world state, NPC AI, combat, and technology systems.

The ruleset is PF2E-derived (six ability scores: Brutality, Quickness, Grit, Reasoning, Savvy, Flair; PF2E action economy; proficiency ranks; feats; class features). The setting is Gunchete — a sci-fi post-collapse Portland where "technology" replaces magic spells. All content (zones, NPCs, weapons, armor, feats, class features, skills, conditions, AI domains) is loaded at startup from YAML files under `content/` into in-memory registries; there is no runtime content hot-reload.

## Directory Map

| Directory | Description |
|---|---|
| `cmd/frontend/` | Telnet server binary entrypoint: loads ruleset content, wires auth handler, starts telnet acceptor |
| `cmd/gameserver/` | gRPC game service binary entrypoint: loads world/content, wires all managers, serves gRPC |
| `cmd/devserver/` | All-in-one development server: wires frontend + database without gRPC (local dev only) |
| `cmd/migrate/` | Database migration binary |
| `cmd/import-content/` | PF2E content importer binary |
| `cmd/setrole/` | Admin role assignment utility binary |
| `internal/frontend/` | Telnet connection handling, auth flow, split-screen UI, command bridge dispatch |
| `internal/frontend/handlers/` | Auth handler, game bridge loop, bridge handler map (one function per command) |
| `internal/frontend/telnet/` | Raw telnet acceptor, connection, screen renderer, color/ANSI utilities |
| `internal/gameserver/` | gRPC service implementation, command dispatch, all sub-handlers (combat, world, chat, NPC, regen, etc.) |
| `internal/game/` | Pure functional game domain: ai, character, combat, command, condition, dice, inventory, mentalstate, npc, ruleset, scripting, session, skillcheck, technology, world, xp |
| `internal/game/ai/` | HTN (Hierarchical Task Network) planner, domain registry |
| `internal/game/combat/` | PF2E combat engine: initiative, action points, attack resolution, MAP |
| `internal/game/character/` | Character struct, ability scores, derived stats |
| `internal/game/command/` | Command registry, builtin command list, handler constants, shortcut registration |
| `internal/game/npc/` | NPC template loading, instance manager, respawn manager |
| `internal/game/ruleset/` | YAML loaders for regions, teams, jobs, archetypes, skills, feats, class features |
| `internal/game/technology/` | Technology definition registry and effect types |
| `internal/game/world/` | Zone and room structs, world manager, exit validation |
| `internal/scripting/` | Lua sandbox manager (gopher-lua), per-zone and global VMs, scripting callbacks |
| `internal/storage/postgres/` | PostgreSQL repositories: accounts, characters, skills, feats, technology slots, automap, progress, inventory |
| `internal/importer/` | PF2E data importer pipeline |
| `internal/config/` | YAML config loader |
| `internal/observability/` | Zap logger factory |
| `internal/server/` | Lifecycle manager (start/stop service coordination) |
| `api/proto/game/v1/` | Proto definition: GameService.Session (bidirectional streaming), ClientMessage oneof, ServerEvent oneof |
| `content/` | YAML content files: zones, npcs, weapons, armor, items, explosives, conditions, feats, skills, class_features, archetypes, regions, teams, jobs, loadouts, technologies, scripts (Lua), ai (HTN domains) |
| `migrations/` | SQL migration files |
| `deployments/` | Docker Compose, Dockerfile, Kubernetes/Helm chart |
| `docs/` | Requirements, architecture, and planning documents |
| `.claude/skills/` | Agent orientation and system skill files |

## Key Architectural Patterns

- **Functional Core / Imperative Shell**: game domain packages (`internal/game/*`) are pure Go structs and functions with no side effects; state mutation and I/O are isolated to `internal/gameserver/` and `internal/frontend/`.
- **YAML-driven content registries**: all game content is loaded at startup from `content/` directories into typed in-memory registries (e.g., `technology.Registry`, `condition.Registry`, `inventory.Registry`, `ruleset.FeatRegistry`). No database stores content definitions.
- **Bridge handler dispatch**: the frontend's `bridgeHandlerMap` (in `internal/frontend/handlers/bridge_handlers.go`) maps each `command.Handler*` constant to a `bridgeHandlerFunc`. Adding a command requires an entry in both `commands.go` and `bridgeHandlerMap`.
- **Proto oneof for all messages**: `ClientMessage.payload` and `ServerEvent.payload` are proto3 oneof fields covering all ~80 client commands and ~25 server events. Wire format is bidirectional gRPC streaming (`rpc Session(stream ClientMessage) returns (stream ServerEvent)`).
- **Property-based testing (rapid/v10)**: all new code uses `pgregory.net/rapid` for property-based tests (SWENG-5a). Postgres tests use testcontainers and run separately via `make test-postgres`.
- **Lua scripting sandbox**: `internal/scripting.Manager` maintains per-zone and global gopher-lua VMs for condition scripts, weapon scripts, and AI precondition scripts. Instruction limits prevent runaway scripts.
- **HTN AI planner**: NPCs run Hierarchical Task Network planning with YAML domain files and Lua preconditions. The `ZoneTickManager` (in `internal/gameserver/zone_tick.go`) fires AI ticks at a configurable interval (default 10s).
- **Game clock**: a goroutine advances the in-game hour and broadcasts `TimeOfDayEvent` to all connected sessions.

## Build & Deploy

| Target | Action |
|---|---|
| `make build` | Compile all binaries to `bin/` |
| `make test-fast` | Unit + integration tests, no Docker (~5s) |
| `make test-postgres` | Docker-dependent postgres + property tests |
| `make test` | Both test suites |
| `make proto` | Regenerate protobuf Go code from `api/proto/game/v1/game.proto` |
| `make docker-push` | Build and push container images to `registry.johannsen.cloud:5000` |
| `make k8s-redeploy` | `docker-push` + `helm upgrade` — **only valid deploy path** |

- Registry: `registry.johannsen.cloud:5000`
- Kubernetes namespace: `mud` (not `default`)
- Helm release: `mud` in namespace `mud`
- DB password: read from `.claude/rules/.env` only — never hardcode
- NEVER use `sudo`

## Skill Routing Table

| Working on... | Load skill |
|---|---|
| Player commands, bridge handlers, command registry, CMD-* rules | `mud-commands` |
| Character creation, ability scores, feats, class features, archetypes, level-up | `mud-character` |
| Technology system (hardwired/prepared/spontaneous/innate), tech effects | `mud-technology` |
| Combat engine, PF2E action economy, conditions, skill checks, combat commands | `mud-combat` |
| Telnet UI, split-screen rendering, ANSI/color, screen layout | `mud-ui` |
| PostgreSQL repositories, migrations, persistence contracts | `mud-persistence` |
| YAML content loading, importer pipeline, content registries | `mud-content-pipeline` |
| HTN AI planner, Lua scripting, NPC lifecycle, zone ticks | `mud-ai` |

## Hard Constraints

- NEVER run `sudo` — not allowed under any circumstances.
- NEVER use DECSTBM scroll regions in telnet output — causes coordinate offset bugs in TinTin++.
- DB password MUST come ONLY from `.claude/rules/.env` — never from memory, never hardcoded.
- Kubernetes namespace is `mud`, not `default`. Helm release is named `mud` in namespace `mud`.
- `make k8s-redeploy` is the only valid deploy path (builds images, pushes, upgrades helm).
- If helm gets stuck in `pending-upgrade`: `helm rollback mud <last-good-revision> -n mud`.
- All new commands must complete CMD-1 through CMD-7 before being considered done — a partially wired command is a defect.

## Key File Index

| File | Purpose |
|---|---|
| `/home/cjohannsen/src/mud/cmd/frontend/main.go` | Frontend binary: loads content, wires auth handler, starts telnet acceptor |
| `/home/cjohannsen/src/mud/cmd/gameserver/main.go` | Gameserver binary: loads world/content, wires all managers, starts gRPC server |
| `/home/cjohannsen/src/mud/internal/frontend/handlers/bridge_handlers.go` | `bridgeHandlerMap` — single source of truth for frontend command dispatch |
| `/home/cjohannsen/src/mud/internal/frontend/handlers/game_bridge.go` | `commandLoop` — reads player input, dispatches via bridge map, handles server events |
| `/home/cjohannsen/src/mud/internal/frontend/telnet/screen.go` | `WriteRoom`, `WriteConsole`, `WritePromptSplit`, `InitScreen` — split-screen terminal control |
| `/home/cjohannsen/src/mud/internal/frontend/handlers/text_renderer.go` | `RenderRoomView(rv *gamev1.RoomView, width int, maxLines int) string`, `RenderCharacterSheet(csv, width)` — width-aware text rendering |
| `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go` | `GameServiceServer` — gRPC Session handler, dispatch type switch over `ClientMessage.payload` |
| `/home/cjohannsen/src/mud/internal/gameserver/combat_handler.go` | PF2E combat resolution: initiative, attack, MAP, death, flee, conditions |
| `/home/cjohannsen/src/mud/internal/gameserver/technology_assignment.go` | Technology slot assignment: hardwired/prepared/spontaneous/innate grant logic |
| `/home/cjohannsen/src/mud/internal/gameserver/tech_effect_resolver.go` | Technology effect resolution: translates TechEffect definitions into game outcomes |
| `/home/cjohannsen/src/mud/internal/gameserver/npc_handler.go` | NPC query helpers used by grpc_service for room NPC listings |
| `/home/cjohannsen/src/mud/internal/game/ai/planner.go` | HTN planner: decomposes compound tasks to primitive tasks via Lua preconditions |
| `/home/cjohannsen/src/mud/internal/scripting/sandbox.go` | Lua VM manager: per-zone and global sandboxes, instruction limits, scripting callbacks |
| `/home/cjohannsen/src/mud/internal/storage/postgres/account.go` | Account repository: create, lookup, role mutation |
| `/home/cjohannsen/src/mud/internal/importer/importer.go` | PF2E content importer pipeline |
| `/home/cjohannsen/src/mud/api/proto/game/v1/game.proto` | All proto message types: `ClientMessage` oneof (~80 requests), `ServerEvent` oneof (~25 events) |
