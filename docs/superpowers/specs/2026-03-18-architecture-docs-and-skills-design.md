# Architecture Docs & Agent Skills — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Author:** Claude Code (brainstorming session)

---

## Problem Statement

The `docs/requirements/` files were authored at project inception and have not been updated as
systems were implemented. There are no as-built architecture documents. There is no agent skill
that orients an LLM working in this codebase — agents must re-discover structure from scratch
each session.

---

## Goals

1. Produce as-built architecture documentation for every implemented system.
2. Produce a two-tier agent skill set that agents load during coding sessions to avoid redundant
   exploration and avoid known pitfalls.
3. Keep `docs/requirements/` unchanged (REQ-N specs remain the source of intent).

---

## Non-Goals

- Updating or rewriting `docs/requirements/` files.
- Adding new requirements or changing existing ones.
- Documenting unimplemented systems or PF2E concepts with no corresponding Go code.

---

## Approach

**Skills-first, docs as expansions.** Each skill file is written first as the dense, agent-optimized
primary artifact. The corresponding architecture doc is the same content expanded with prose,
Mermaid sequence diagrams, and human context. This avoids writing the same content twice and
keeps skills and docs structurally consistent.

---

## Output Artifacts

### Skill Files (`.claude/skills/`)

The directory `.claude/skills/` must be created if it does not already exist.

| File | Trigger Condition |
|---|---|
| `mud-overview.md` | Session start in this repo — always load |
| `mud-combat.md` | Working on combat actions, conditions, initiative, damage, rounds |
| `mud-character.md` | Working on character creation, ability scores, progression, level-up |
| `mud-technology.md` | Working on techs, innate slots, spontaneous selection, tech effect resolution |
| `mud-ai.md` | Working on NPC behavior, HTN planning, Lua scripting, respawn, NPC lifecycle |
| `mud-commands.md` | Adding/modifying player commands, bridge handlers, proto messages |
| `mud-ui.md` | Working on telnet rendering, screen layout, resize handling |
| `mud-content-pipeline.md` | Working on YAML content, importer, registry, content formats |
| `mud-persistence.md` | Working on DB schema, migrations, repos, queries |

### Architecture Docs (`docs/architecture/`)

The directory `docs/architecture/` must be created if it does not already exist.

One `.md` per system, mirroring the skill list above. Human-readable with Mermaid diagrams.

---

## Overview Skill Content Spec (`mud-overview.md`)

Sections (in order):

1. **What this repo is** — two-binary Go system: `frontend` (telnet server) + `gameserver` (gRPC
   service). PF2E-derived ruleset. Gunchete sci-fi setting (post-collapse Portland).
2. **Directory map** — `internal/`, `content/`, `api/proto/`, `cmd/`, `migrations/`, `docs/`,
   `.claude/skills/` with one-line descriptions.
3. **Key architectural patterns** — functional core / imperative shell; YAML-driven content;
   bridge handler dispatch; proto `oneof` for all client/server messages; property-based testing.
4. **Build & deploy** — `make build`, `make test`, `make k8s-redeploy`; registry at
   `registry.johannsen.cloud:5000`; k8s namespace `mud`; DB password from `.claude/rules/.env`.
5. **Skill routing table** — "if working on X, load skill Y" — covers all 8 system skills.
6. **Hard constraints** — never `sudo`; never DECSTBM; DB password from `.env` only;
   k8s namespace is `mud` not `default`; `make k8s-redeploy` is the only deploy path.
7. **Key file index** — one entry-point file per system (no `_test.go`, no generated `.pb.go`
   files), covering: frontend entrypoint, gameserver entrypoint, bridge handlers, gRPC service,
   combat resolver, character handler, technology assignment, tech effect resolver, HTN planner,
   Lua sandbox, content importer, postgres repo root, screen renderer, and the proto definition.
   ~15 files total by absolute path with one-line descriptions.

Target size: 500–800 tokens.

---

## System Skill Content Spec (all 8 skills)

Each skill follows this template:

```
## Trigger
When to load this skill.

## Responsibility Boundary
What this system owns. What it delegates. Named packages involved.

## Key Files
5–10 files by absolute path with one-line descriptions.

## Core Data Structures
Primary types and proto messages. Field-level detail for the most important ones.

## Primary Data Flow
Numbered steps or Mermaid sequence for the system's most important operation.

## Invariants & Contracts
Preconditions and postconditions that must always hold.

## Extension Points
How to add a new X (combat action / tech / command / condition / NPC / migration / etc.)
Cross-reference AGENTS.md rules (CMD-1 through CMD-7, SWENG-5, etc.) where applicable.

## Common Pitfalls
Mistakes agents have made in this area, from project history.
```

Target size per skill: 800–1500 tokens.

---

## System-Specific Notes

### `mud-combat`
Data flow: `Attack` command → bridge handler → `HandleAttack` gRPC → initiative check →
round loop → `ResolveAction` → degree-of-success → condition application → `CombatEvent` response

### `mud-character`
Data flow: Archetype selection → ability boosts (fixed + free) → job assignment → tech grants
(hardwired/prepared/spontaneous) → innate region grants → `CharacterInfo` response

### `mud-technology`
Data flow: `UseRequest` → tech registry lookup → usage type dispatch
(innate/spontaneous/prepared/hardwired) → `ResolveTech` → effect application
(damage / condition / heal) → `UseResponse`

Trigger covers: `internal/game/technology/`, `internal/gameserver/technology_assignment.go`,
`internal/gameserver/tech_effect_resolver.go`. Does NOT cover PF2E "traditions" as an
implemented system — reference content YAML tradition field only.

### `mud-ai`
Data flow: HTN decomposition (`internal/game/ai/`) → task queue → `ExecuteAction` per tick →
combat auto-queue → NPC death → loot drop → respawn timer → re-register in room

Three contributing packages must each have their own Responsibility Boundary sub-entry:
- `internal/game/ai/` — HTN planner (planner, domain, world_state, build_state)
- `internal/scripting/` — Lua sandbox (sandbox, modules, manager)
- `internal/game/npc/` + `internal/gameserver/npc_handler.go` — NPC lifecycle, loot, respawn

### `mud-commands`
Data flow: Telnet input → `internal/frontend/telnet/` parse → `internal/frontend/handlers/game_bridge.go`
→ `bridgeHandlerMap` in `internal/frontend/handlers/bridge_handlers.go` → proto encode →
gRPC `Session` stream → `internal/gameserver/grpc_service.go` dispatch type switch →
handler → `ServerEvent` response

Extension Points section MUST cross-reference AGENTS.md CMD-1 through CMD-7 (all 7 steps
required to add a command).

### `mud-ui`
Data flow: Window resize event → `InitScreen` → `WriteRoom(rv, width)` → `WriteConsole` →
`WritePromptSplit`; room region rows 1–8, divider row 9, console rows 10+, prompt row H

### `mud-content-pipeline`
Data flow: YAML file → `importer` parse → struct validation → `Register` into in-memory registry
→ served via registry interface at runtime; DB not used for content (content is read-only at startup)

### `mud-persistence`
Data flow: Repo interface defined in `internal/game/` → postgres impl in `internal/storage/postgres/`
→ migration version tracked in `schema_migrations`

Migration count must be derived at skill-write time by running `ls migrations/ | wc -l` — do not
hard-code a count in the spec or skill.

---

## Architecture Doc Content Spec (`docs/architecture/`)

Each architecture doc contains:

1. The full skill content (prose-expanded)
2. Mermaid sequence diagram for the primary data flow
3. Mermaid component diagram showing package dependencies
4. Cross-references to relevant `docs/requirements/` files
5. "Written as of" date; commit SHA obtained by running `git rev-parse HEAD` at time of writing

---

## Execution Order

0. Create `.claude/skills/` and `docs/architecture/` directories if absent
1. Write `mud-overview` skill + `docs/architecture/overview.md`
2. Write `mud-commands` skill + doc (foundation — other systems build on it; cross-ref CMD rules)
3. Write `mud-character` skill + doc
4. Write `mud-technology` skill + doc
5. Write `mud-combat` skill + doc
6. Write `mud-ui` skill + doc
7. Write `mud-persistence` skill + doc (derive migration count from `ls migrations/ | wc -l`)
8. Write `mud-content-pipeline` skill + doc
9. Write `mud-ai` skill + doc (three-package boundary: HTN planner, Lua scripting, NPC lifecycle)

---

## Success Criteria

- All 9 skill files exist in `.claude/skills/` and are loadable via the `Skill` tool.
- All 9 architecture docs exist in `docs/architecture/`.
- An agent loading `mud-overview` can navigate to the right system skill without any codebase
  exploration.
- An agent loading a system skill can add a new instance of the extension point (e.g. a new
  command, tech, condition) by following the "Extension Points" section alone.
- The `mud-commands` skill's Extension Points section explicitly lists all CMD-1 through CMD-7
  steps from `AGENTS.md`.
- The `mud-ai` skill names all three contributing packages with distinct responsibility boundaries.
- The `mud-persistence` skill states the correct migration count derived at write time.
- No `docs/requirements/` files are modified.
