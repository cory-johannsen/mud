# Architecture Docs & Agent Skills Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce 9 agent skill files in `.claude/skills/` and 9 matching architecture docs in `docs/architecture/` covering every implemented system in the MUD codebase.

**Architecture:** Skills-first — each skill is written as the dense, agent-optimized primary artifact; the architecture doc is the same content expanded with prose and Mermaid diagrams. Overview skill is written first and acts as the routing table for all system skills.

**Spec:** `docs/superpowers/specs/2026-03-18-architecture-docs-and-skills-design.md`

**Tech Stack:** Markdown, Mermaid diagrams. Go source files are read-only reference material.

---

## Skill File Template (apply to every system skill)

Every `.claude/skills/mud-*.md` file MUST contain these sections in order:

```markdown
---
name: mud-<system>
description: <one line — used to decide relevance>
type: reference
---

## Trigger
[When to load this skill]

## Responsibility Boundary
[What this system owns vs. delegates. Named packages.]

## Key Files
[5–10 absolute paths with one-line descriptions]

## Core Data Structures
[Primary types and proto messages with key fields]

## Primary Data Flow
[Numbered steps for the most important operation]

## Invariants & Contracts
[Preconditions/postconditions that must always hold]

## Extension Points
[Step-by-step: how to add a new X]

## Common Pitfalls
[Known mistakes in this area]
```

## Architecture Doc Template (apply to every doc)

Every `docs/architecture/*.md` file MUST contain:

```markdown
# [System] Architecture

**As of:** YYYY-MM-DD (commit: <output of `git rev-parse HEAD`>)
**Skill:** `.claude/skills/mud-<system>.md`
**Requirements:** `docs/requirements/<relevant>.md`

## Overview
[2–3 paragraphs: what the system does, why it exists, how it fits in the larger architecture]

## Package Structure
[Named packages with one-line descriptions]

## Core Data Structures
[Same as skill but with more prose]

## Primary Data Flow

```mermaid
sequenceDiagram
[...]
```

## Component Dependencies

```mermaid
graph TD
[...]
```

## Extension Points
[Same as skill, expanded with context]

## Known Constraints & Pitfalls
[Same as skill, expanded with context]

## Cross-References
- Requirements: [links to docs/requirements/ files]
- Skill: [link to .claude/skills/ file]
```

---

## Task 0: Create output directories

**Files:**
- Create: `.claude/skills/` (directory)
- Create: `docs/architecture/` (directory)

- [ ] **Step 1: Create directories**

```bash
mkdir -p /home/cjohannsen/src/mud/.claude/skills
mkdir -p /home/cjohannsen/src/mud/docs/architecture
```

- [ ] **Step 2: Verify**

```bash
ls /home/cjohannsen/src/mud/.claude/skills
ls /home/cjohannsen/src/mud/docs/architecture
```

Expected: both commands succeed (empty directories).

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/.gitkeep docs/architecture/.gitkeep 2>/dev/null || true
# directories are tracked implicitly when files are added in later tasks
```

No commit needed for empty directories — they will be committed with their first file.

---

## Task 1: Overview skill + architecture doc

**Read these files first (do not skip):**
- `cmd/frontend/main.go` — frontend entrypoint
- `cmd/gameserver/main.go` — gameserver entrypoint
- `internal/frontend/handlers/bridge_handlers.go` — command routing table (first 60 lines)
- `internal/gameserver/grpc_service.go` — dispatch type switch (first 80 lines)
- `api/proto/game/v1/game.proto` — all message types
- `Makefile` — build and deploy targets

**Files:**
- Create: `.claude/skills/mud-overview.md`
- Create: `docs/architecture/overview.md`

- [ ] **Step 1: Read the source files listed above**

- [ ] **Step 2: Write `.claude/skills/mud-overview.md`**

The skill MUST contain all 7 sections in the order below. Do NOT use the generic skill template — the overview skill has its own structure:

```
---
name: mud-overview
description: Orientation skill for the MUD codebase — load at session start
type: reference
---

## What This Repo Is
[two-binary Go system: frontend (telnet server) + gameserver (gRPC service).
PF2E-derived ruleset. Gunchete sci-fi setting (post-collapse Portland).
Bidirectional gRPC streaming between the two binaries.]

## Directory Map
[Table: directory → one-line description, covering internal/, content/, api/proto/,
cmd/, migrations/, docs/, .claude/skills/]

## Key Architectural Patterns
[functional core / imperative shell; YAML-driven content loaded at startup into
in-memory registries; bridge handler dispatch (bridgeHandlerMap); proto oneof for
all client/server messages; property-based testing (rapid/v10)]

## Build & Deploy
[make build, make test, make k8s-redeploy; registry at registry.johannsen.cloud:5000;
k8s namespace mud; DB password from .claude/rules/.env; NEVER sudo]

## Skill Routing Table
[Table: "if working on X → load skill Y" — one row per system skill]

## Hard Constraints
[never sudo; never DECSTBM scroll regions in telnet; DB password ONLY from
.claude/rules/.env; k8s namespace is mud not default; make k8s-redeploy is
the only valid deploy path; helm release named mud in namespace mud]

## Key File Index
[~15 files by absolute path, one per system, no _test.go, no .pb.go:
- cmd/frontend/main.go
- cmd/gameserver/main.go
- internal/frontend/handlers/bridge_handlers.go
- internal/frontend/handlers/game_bridge.go
- internal/frontend/telnet/screen.go
- internal/frontend/handlers/text_renderer.go
- internal/gameserver/grpc_service.go
- internal/gameserver/combat_handler.go
- internal/gameserver/technology_assignment.go
- internal/gameserver/tech_effect_resolver.go
- internal/gameserver/npc_handler.go
- internal/game/ai/planner.go
- internal/scripting/sandbox.go
- internal/storage/postgres/account.go
- internal/importer/importer.go
- api/proto/game/v1/game.proto
]
```

- [ ] **Step 3: Write `docs/architecture/overview.md`**

Expand the skill into a full architecture doc using the Architecture Doc Template. Include:
- Mermaid sequence diagram: telnet input → frontend → gRPC → gameserver → response
- Mermaid component diagram: cmd/frontend → internal/frontend → gRPC → cmd/gameserver → internal/gameserver → internal/game/*
- Cross-references to all docs/requirements/ files

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-overview.md docs/architecture/overview.md
git commit -m "docs(arch): add overview skill and architecture doc"
```

---

## Task 2: Commands skill + architecture doc

**Read these files first (do not skip):**
- `internal/game/command/commands.go` — full file (Handler constants + BuiltinCommands)
- `internal/frontend/handlers/bridge_handlers.go` — full file (bridgeHandlerMap + all bridge* funcs)
- `internal/frontend/handlers/game_bridge.go` — parse and dispatch logic
- `internal/frontend/telnet/acceptor.go` — telnet connection handling
- `internal/gameserver/grpc_service.go` — dispatch type switch (full file)
- `.claude/rules/AGENTS.md` — CMD-1 through CMD-7 rules

**Files:**
- Create: `.claude/skills/mud-commands.md`
- Create: `docs/architecture/commands.md`

- [ ] **Step 1: Read the source files listed above**

- [ ] **Step 2: Write `.claude/skills/mud-commands.md`**

Use the generic skill template. The Extension Points section MUST reproduce all 7 CMD rules
verbatim from AGENTS.md and then translate each into a concrete code step (e.g.
"CMD-1: add `HandlerFoo` constant to `internal/game/command/commands.go`").

Key files section MUST list the 5 files above by absolute path.

Primary Data Flow MUST be numbered steps:
1. Player types command in telnet terminal
2. `internal/frontend/telnet/conn.go` reads input line
3. `internal/frontend/handlers/game_bridge.go` parses command string → looks up in `BuiltinCommands()`
4. Dispatches to matching `bridge<Name>` func in `internal/frontend/handlers/bridge_handlers.go`
5. Bridge func encodes proto `ClientMessage` oneof variant
6. Sends over bidirectional gRPC `Session` stream to gameserver
7. `internal/gameserver/grpc_service.go` `dispatch` type switch routes to `handle<Name>`
8. Handler executes game logic, builds `ServerEvent` oneof response
9. Response streams back to frontend → rendered to telnet terminal

Invariants section MUST include:
- Every constant in `HandlerXxx` MUST have a `Command{...}` entry in `BuiltinCommands()` (CMD-2)
- Every `BuiltinCommands()` entry MUST have a `bridge<Name>` in `bridgeHandlerMap` (CMD-5); `TestAllCommandHandlersAreWired` enforces this
- Every bridge func MUST have a corresponding `handle<Name>` in grpc_service.go (CMD-6)

- [ ] **Step 3: Write `docs/architecture/commands.md`**

Expand using Architecture Doc Template. Include:
- Mermaid sequence: full path from telnet input to ServerEvent response
- Mermaid component: telnet conn → game_bridge → bridge_handlers → grpc → grpc_service → game logic
- Cross-reference: `docs/requirements/NETWORKING.md`, `docs/requirements/ARCHITECTURE.md`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-commands.md docs/architecture/commands.md
git commit -m "docs(arch): add commands skill and architecture doc"
```

---

## Task 3: Character skill + architecture doc

**Read these files first (do not skip):**
- `internal/game/character/` — all non-test .go files
- `internal/frontend/handlers/character_flow.go` — character creation flow
- `internal/gameserver/grpc_service.go` — handleChar, handleArchetypeSelection, handleLevelUp
- `internal/gameserver/grpc_service_tech_helpers.go` — tech grant helpers
- `content/archetypes/aggressor.yaml` — example archetype YAML structure
- `content/jobs/goon.yaml` — example job YAML structure

**Files:**
- Create: `.claude/skills/mud-character.md`
- Create: `docs/architecture/character.md`

- [ ] **Step 1: Read the source files listed above**

- [ ] **Step 2: Write `.claude/skills/mud-character.md`**

Use the generic skill template.

Primary Data Flow MUST cover:
1. Player selects archetype → `handleArchetypeSelection` → sets fixed+free ability boosts, HP/level
2. Player selects job → ability boosts applied, hardwired tech grants assigned
3. Player completes creation → `handleChar` assembles `CharacterInfo` proto
4. On level-up → `handleLevelUp` → class feature grants, prepared/spontaneous slot increases
5. Innate region grants assigned at character creation via `AssignTechnologies(char, region)`

Core Data Structures MUST document key fields of:
- `Character` struct (internal/game/character/)
- `CharacterInfo` proto (from game.proto)
- Archetype YAML structure (id, key_ability, hit_points_per_level, ability_boosts, technology_grants, level_up_grants)
- Job YAML structure (id, archetype, key fields)

Extension Points: "How to add a new archetype" and "How to add a new job"

- [ ] **Step 3: Write `docs/architecture/character.md`**

Include:
- Mermaid sequence: archetype selection → job → level-up flow
- Cross-reference: `docs/requirements/CHARACTERS.md`, `docs/requirements/SETTING.md`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-character.md docs/architecture/character.md
git commit -m "docs(arch): add character skill and architecture doc"
```

---

## Task 4: Technology skill + architecture doc

**Read these files first (do not skip):**
- `internal/game/technology/registry.go` — registry interface + in-memory impl
- `internal/game/technology/model.go` — Technology, UsageType, Tradition types (if exists; else check registry.go)
- `internal/gameserver/technology_assignment.go` — AssignTechnologies, innate slot assignment
- `internal/gameserver/tech_effect_resolver.go` — full file
- `internal/gameserver/grpc_service_tech_helpers.go` — full file
- `content/technologies/neural/mind_spike.yaml` — example tech YAML
- `docs/superpowers/specs/` — find and read the Tech Effect Resolution design spec

**Files:**
- Create: `.claude/skills/mud-technology.md`
- Create: `docs/architecture/technology.md`

- [ ] **Step 1: Read the source files listed above**

- [ ] **Step 2: Write `.claude/skills/mud-technology.md`**

Responsibility Boundary MUST name three sub-areas:
- `internal/game/technology/` — model and registry (pure data, no side effects)
- `internal/gameserver/technology_assignment.go` — assigns techs to characters at creation/level-up
- `internal/gameserver/tech_effect_resolver.go` — resolves UseRequest into game effects

NOTE: "traditions" (Technical, Neural, Bio-Synthetic, Fanatic Doctrine) are content-level YAML
field values only — NOT implemented as Go types. Do not reference them as code constructs.

Primary Data Flow MUST cover UseRequest path:
1. Player sends `UseRequest{tech_id}` proto
2. `grpc_service.go` routes to `handleUse`
3. Looks up tech in registry by ID
4. Dispatches by usage type: innate → check `InnateSlot` uses_remaining → decrement;
   spontaneous → check slot availability; prepared → check slot; hardwired → always available
5. `tech_effect_resolver.go` resolves effect: damage, condition, heal, or utility
6. Applies effect to target(s)
7. Returns `UseResponse` proto

Core Data Structures MUST document:
- `Technology` struct fields (id, name, tradition, usage_type, range, effects)
- `InnateSlot` struct (tech_id, uses_remaining, max_uses)
- `UseRequest` / `UseResponse` proto fields

Extension Points: "How to add a new technology (YAML + wiring)"

- [ ] **Step 3: Write `docs/architecture/technology.md`**

Include:
- Mermaid sequence: UseRequest → resolver → effect → UseResponse
- Cross-reference: `docs/requirements/FEATURES.md`, `docs/superpowers/specs/` (innate tech + tech effect resolution specs)

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-technology.md docs/architecture/technology.md
git commit -m "docs(arch): add technology skill and architecture doc"
```

---

## Task 5: Combat skill + architecture doc

**Read these files first (do not skip):**
- `internal/game/combat/engine.go` — Combat, Combatant structs and engine entry points
- `internal/game/combat/combat.go` — round loop, AP tracking
- `internal/game/combat/action.go` — action resolution, degree-of-success logic
- `internal/game/combat/resolver.go` — save/attack resolvers
- `internal/game/combat/initiative.go` — initiative rolling and ordering
- `internal/gameserver/combat_handler.go` — gRPC-layer combat handler
- `internal/game/condition/` — condition model

**Files:**
- Create: `.claude/skills/mud-combat.md`
- Create: `docs/architecture/combat.md`

- [ ] **Step 1: Read the source files listed above**

- [ ] **Step 2: Write `.claude/skills/mud-combat.md`**

Responsibility Boundary:
- `internal/game/combat/` — pure combat logic (model, action resolution, round loop); no I/O
- `internal/gameserver/combat_handler.go` — imperative shell: receives gRPC, drives combat engine, sends events
- `internal/game/condition/` — condition types and application rules

Primary Data Flow MUST cover:
1. Player sends `Attack` proto → `handleAttack` in combat_handler.go
2. If no combat active: initialize `Combat{combatants}`, roll initiative
3. Round loop: each combatant gets 3 AP per round
4. Player action (Strike/Feint/Grapple/etc.) → `ResolveAction` in action.go
5. `DegreeOfSuccess`: crit success / success / failure / crit failure based on d20 + modifier vs DC
6. Apply results: damage, conditions (`prone`, `grabbed`, `frightened`, etc.)
7. Emit `CombatEvent` proto with narrative text + HP updates
8. Round ends when all AP exhausted → `RoundEndEvent`; repeat until combat over

Core Data Structures: `Combat`, `Combatant`, `Round`, `ActionResult`, condition types.

Invariants: PF2E 4-tier outcomes always apply; 3 AP per combatant per round; conditions are
applied via `condition.Apply(target, conditionID)` never set directly.

Extension Points: "How to add a new combat action" — includes adding handler constant (CMD-1),
BuiltinCommands entry (CMD-2), Handle func (CMD-3), proto message (CMD-4), bridge func (CMD-5),
grpc handler (CMD-6), and completion gate CMD-7 (all steps done, all tests pass). Also covers
adding a new condition YAML.

- [ ] **Step 3: Write `docs/architecture/combat.md`**

Include:
- Mermaid sequence: Attack → initiative → round loop → action resolution → CombatEvent
- Cross-reference: `docs/requirements/COMBAT.md`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-combat.md docs/architecture/combat.md
git commit -m "docs(arch): add combat skill and architecture doc"
```

---

## Task 6: UI skill + architecture doc

**Read these files first (do not skip):**
- `internal/frontend/telnet/screen.go` — full file
- `internal/frontend/handlers/text_renderer.go` — full file
- `internal/frontend/telnet/conn.go` — connection and window-size handling
- `internal/frontend/telnet/ansi.go` — ANSI escape sequences used

**Files:**
- Create: `.claude/skills/mud-ui.md`
- Create: `docs/architecture/ui.md`

- [ ] **Step 1: Read the source files listed above**

- [ ] **Step 2: Write `.claude/skills/mud-ui.md`**

Primary Data Flow MUST cover:
1. Client sends telnet NAWS (window size) negotiation
2. `conn.go` captures terminal width/height
3. On room entry or resize: call `InitScreen(width, height)` → clears and redraws
4. `WriteRoom(rv *RoomView, width int)` → renders room region rows 1–8
5. Divider row 9: horizontal rule
6. `WriteConsole(text string)` → appends to console region rows 10+
7. `WritePromptSplit(prompt string)` → writes to row H (last row)

MUST document the split-screen layout explicitly:
- Rows 1–8: room region (description, exits, NPCs)
- Row 9: divider
- Rows 10–(H-1): scrolling console
- Row H: input prompt

Invariants:
- NEVER use DECSTBM (scroll regions) — TinTin++ DECOM coordinate offset quirk
- `RenderRoomView(rv, width)` wraps description, 3 exits per row, `direction*` for locked exits
- `RenderCharacterSheet(csv, width)` uses 2-column layout at width ≥ 73
- `game_bridge.go` stores `*gamev1.RoomView` (not rendered string) for resize re-render

- [ ] **Step 3: Write `docs/architecture/ui.md`**

Include:
- ASCII art or Mermaid diagram of the split-screen layout (rows labeled)
- Mermaid sequence: window resize → InitScreen → WriteRoom → WriteConsole → WritePrompt
- Cross-reference: `docs/requirements/NETWORKING.md`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-ui.md docs/architecture/ui.md
git commit -m "docs(arch): add UI skill and architecture doc"
```

---

## Task 7: Persistence skill + architecture doc

**Read these files first (do not skip):**
- `internal/storage/postgres/account.go` — example repo impl pattern
- `internal/storage/postgres/` — list all files to understand scope
- `internal/game/` — find repo interfaces (grep for `Repository` or `Repo` in interface declarations)
- `migrations/` — list all migration files; note the count (`ls migrations/ | wc -l`)
- `configs/` — database config

**Files:**
- Create: `.claude/skills/mud-persistence.md`
- Create: `docs/architecture/persistence.md`

- [ ] **Step 1: Read the source files listed above. Run `ls migrations/ | wc -l` to get migration count.**

- [ ] **Step 2: Write `.claude/skills/mud-persistence.md`**

Primary Data Flow:
1. Game logic calls repo interface method (e.g. `CharacterRepo.Save(ctx, char)`)
2. Interface defined in `internal/game/` domain package
3. Postgres implementation in `internal/storage/postgres/` satisfies the interface
4. Uses `pgx` or `database/sql` with parameterized queries
5. Schema managed by numbered migration files in `migrations/`
6. Migration runner: `cmd/migrate/` — run via `make migrate`

Core Data Structures: list all repo interfaces with their key methods.

Invariants:
- Repo interfaces defined in game domain packages — never in storage/
- No raw SQL in game logic — only in postgres impl
- Migration filenames are sequential integers; never edit a committed migration — always add a new one
- Current migration count: (derive at write time from `ls migrations/ | wc -l`)

Extension Points: "How to add a new table / repo" — (1) add migration file, (2) add interface method in game package, (3) implement in postgres package, (4) wire in service constructor.

- [ ] **Step 3: Write `docs/architecture/persistence.md`**

Include:
- Mermaid component: game domain repo interface ← postgres impl → DB
- Cross-reference: `docs/requirements/PERSISTENCE.md`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-persistence.md docs/architecture/persistence.md
git commit -m "docs(arch): add persistence skill and architecture doc"
```

---

## Task 8: Content pipeline skill + architecture doc

**Read these files first (do not skip):**
- `internal/importer/importer.go` — full file
- `internal/importer/source.go` — YAML source loading
- `content/archetypes/aggressor.yaml` — archetype format
- `content/jobs/goon.yaml` — job format
- `content/technologies/neural/mind_spike.yaml` — technology format
- `content/regions/` — pick one region YAML
- `content/conditions/prone.yaml` — condition format
- `cmd/import-content/main.go` — entrypoint for content import binary

**Files:**
- Create: `.claude/skills/mud-content-pipeline.md`
- Create: `docs/architecture/content-pipeline.md`

- [ ] **Step 1: Read the source files listed above. Also run `ls content/` to confirm which top-level content directories exist before writing the skill.**

- [ ] **Step 2: Write `.claude/skills/mud-content-pipeline.md`**

Primary Data Flow:
1. At startup, `cmd/import-content/` or gameserver startup calls `importer.Load(contentDir)`
2. `importer.go` walks `content/` directory tree
3. Each YAML file is parsed into its Go struct type (determined by directory path)
4. Struct is validated (required fields, referential integrity where applicable)
5. Registered into the appropriate in-memory registry (technology registry, archetype registry, etc.)
6. Registries are then read-only at runtime — no content changes after startup
7. DB is NOT used for content — content is stateless YAML loaded fresh each startup

MUST document each content directory and its corresponding Go type + registry:
- `archetypes/` → `Archetype` struct → archetype registry
- `jobs/` → `Job` struct → job registry
- `technologies/<tradition>/` → `Technology` struct → technology registry
- `regions/` → `Region` struct → region registry
- `conditions/` → `Condition` struct → condition registry
- `items/`, `weapons/`, `armor/` → item/weapon/armor types → item registry
- `npcs/` → NPC template → NPC registry
- `ai/` → HTN domain YAML → HTN domain registry

Extension Points: "How to add a new content type" and "How to add a new content item of an existing type"

Common Pitfalls: content changes require server restart (no hot reload); YAML field names are snake_case matching Go struct json/yaml tags.

- [ ] **Step 3: Write `docs/architecture/content-pipeline.md`**

Include:
- Mermaid sequence: startup → importer.Load → registry.Register → runtime serve
- Cross-reference: `docs/requirements/ARCHITECTURE.md`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-content-pipeline.md docs/architecture/content-pipeline.md
git commit -m "docs(arch): add content pipeline skill and architecture doc"
```

---

## Task 9: AI skill + architecture doc

**Read these files first (do not skip):**
- `internal/game/ai/planner.go` — HTN planner
- `internal/game/ai/domain.go` — task domain definitions
- `internal/game/ai/world_state.go` — world state model for HTN
- `internal/game/ai/build_state.go` — world state builder
- `internal/game/ai/registry.go` — HTN domain registry
- `internal/scripting/sandbox.go` — Lua sandbox
- `internal/scripting/modules.go` — Lua modules exposed to scripts
- `internal/scripting/manager.go` — script lifecycle management
- `internal/game/npc/instance.go` — NPC instance model
- `internal/game/npc/respawn.go` — respawn timer and logic
- `internal/game/npc/loot.go` — loot table and drop logic
- `internal/gameserver/npc_handler.go` — NPC lifecycle orchestration (gRPC layer)
- `content/ai/ganger_combat.yaml` — example HTN domain YAML

**Files:**
- Create: `.claude/skills/mud-ai.md`
- Create: `docs/architecture/ai.md`

- [ ] **Step 1: Read the source files listed above**

- [ ] **Step 2: Write `.claude/skills/mud-ai.md`**

Responsibility Boundary MUST have three named sub-sections:

**HTN Planner (`internal/game/ai/`)**
- HTN task decomposition: domain → primitive/compound tasks → ordered task network
- World state: typed key-value store representing NPC's perception of game state
- `planner.go`: `Plan(domain, worldState) []PrimitiveTask`
- `domain.go`: task definitions loaded from YAML
- `registry.go`: domain registry keyed by domain ID

**Lua Scripting (`internal/scripting/`)**
- `sandbox.go`: GopherLua VM with restricted stdlib (no os/io)
- `modules.go`: Go functions exposed to Lua (room queries, NPC actions, item access)
- `manager.go`: loads and caches compiled Lua scripts; executes hooks
- NPC behavior hooks: `on_spawn`, `on_death`, `on_combat_start`, `on_tick`

**NPC Lifecycle (`internal/game/npc/` + `internal/gameserver/npc_handler.go`)**
- Spawn: instantiate NPC from template, place in room, register in world
- Combat: HTN plan → action queue → `combat_handler.go` executes queued actions
- Death: loot table roll → drop items → remove from room → start respawn timer
- Respawn: timer fires → re-instantiate from template → re-place in room

Primary Data Flow (NPC combat turn):
1. Combat round tick → NPC's turn
2. `npc_handler.go` calls HTN planner with current world state
3. Planner decomposes domain tasks → returns ordered primitive action list
4. First action queued: Strike / Move / UseAbility / etc.
5. `combat_handler.go` executes queued NPC action (same path as player actions)
6. World state updated; repeat next round

Extension Points: "How to add a new NPC behavior script" and "How to add a new Lua module"

- [ ] **Step 3: Write `docs/architecture/ai.md`**

Include:
- Mermaid sequence: combat round tick → HTN plan → action queue → execute
- Mermaid component: npc_handler → HTN planner + Lua sandbox + NPC lifecycle
- Cross-reference: `docs/requirements/AI.md`, `docs/requirements/WORLD.md`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/mud-ai.md docs/architecture/ai.md
git commit -m "docs(arch): add AI skill and architecture doc"
```

---

## Verification Checklist

After all tasks complete, verify success criteria from the spec:

- [ ] `ls .claude/skills/mud-*.md` shows 9 files
- [ ] `ls docs/architecture/*.md` shows 9 files
- [ ] Every skill file has all 8 required sections (Trigger through Common Pitfalls)
- [ ] `mud-overview.md` contains a Skill Routing Table covering all 8 system skills
- [ ] `mud-commands.md` Extension Points lists CMD-1 through CMD-7 explicitly
- [ ] `mud-ai.md` names all three packages (HTN planner, Lua scripting, NPC lifecycle)
- [ ] `mud-persistence.md` states migration count derived from `ls migrations/ | wc -l`
- [ ] No files under `docs/requirements/` were modified (`git diff docs/requirements/` is empty)
- [ ] Every `docs/architecture/*.md` contains at least one `sequenceDiagram` Mermaid block: `grep -l "sequenceDiagram" docs/architecture/*.md | wc -l` should equal 9
