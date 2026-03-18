# Feature Management System — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Author:** Claude Code (brainstorming session)

---

## Problem Statement

All product features are defined in a single monolithic `docs/requirements/FEATURES.md` file.
This makes priority management, status tracking, and dependency mapping difficult. There is no
structured way to query or reorder features without editing a large checklist file.

---

## Goals

1. Migrate all top-level features from `FEATURES.md` into individual markdown files.
2. Provide a YAML index with per-feature metadata: priority, status, category, dependencies.
3. Update `SYSREQ-2` to designate `docs/features/` as the canonical source of truth.
4. Deprecate `FEATURES.md` with a redirect notice.

---

## Non-Goals

- Changing existing requirement text or checklist items.
- Implementing any of the features described.
- Deleting `FEATURES.md` (it is kept as a redirect stub).

---

## Directory Structure

```
docs/features/
  index.yaml              # priority-ordered manifest with metadata per feature
  actions.md
  advanced-combat.md
  advanced-enemies.md
  advanced-health.md
  console-notifications.md
  crafting.md
  documentation.md
  editor-commands.md
  equipment-mechanics.md
  factions.md
  feat-import.md
  game-client-ebiten.md
  hero-points.md
  job-development.md
  long-rest.md
  map-poi.md
  non-combat-npcs.md
  non-human-npcs.md
  npc-behaviors.md
  npc-equipment.md
  perception.md
  persistent-calendar.md
  player-gender.md
  quests.md
  resting.md
  room-danger-levels.md
  room-display.md
  technology.md
  traps.md
  use-command.md
  web-client.md
  world-map.md
  zones-new.md
```

---

## Index Schema

`docs/features/index.yaml` contains a top-level `features` list. Each entry:

```yaml
features:
  - slug: advanced-combat
    name: Advanced Combat Mechanics
    status: in_progress       # done | in_progress | planned
    priority: 1               # lower number = higher priority; 1 is highest
    category: combat          # combat | character | technology | world | ui | meta
    dependencies: []          # list of slugs this feature depends on
    file: docs/features/advanced-combat.md
```

### Status Values

- `done` — all checklist items complete
- `in_progress` — partially implemented (some checklist items checked)
- `planned` — not yet started

### Category Values

- `combat` — combat mechanics, actions, conditions
- `character` — character creation, progression, jobs, feats
- `technology` — tech system, traditions, effects
- `world` — NPCs, zones, rooms, factions, quests
- `ui` — display, map, client
- `meta` — documentation, editor tools, architecture

---

## Feature File Format

Each `docs/features/<slug>.md` contains:

1. A `# Feature Name` heading
2. A brief one-paragraph description (added during migration; not changing existing requirement text)
3. The full checklist content migrated verbatim from `FEATURES.md`

Example:

```markdown
# Advanced Combat Mechanics

Extends the base combat system with reactions, distance tracking, cover, area of effect,
fleeing/pursuit, mental state, and terrain types.

## Requirements

- [ ] Reactions
    - [x] Reactive Strike ...
  ...
```

---

## SYSREQ-2 Update

`docs/requirements/ARCHITECTURE.md` SYSREQ-2 is updated to:

> SYSREQ-2: Agents MUST treat `docs/features/index.yaml` and the files in `docs/features/` as
> the canonical source of truth for product feature definitions and priority. The file
> `docs/requirements/FEATURES.md` is a deprecated redirect stub and MUST NOT be edited.

---

## FEATURES.md Update

`docs/requirements/FEATURES.md` is replaced with a redirect stub:

```markdown
# Features

> **Deprecated.** Feature definitions have been migrated to `docs/features/`.
> See `docs/features/index.yaml` for the priority-ordered feature manifest.
> Individual feature specs are in `docs/features/<slug>.md`.
```

---

## Migration Mapping

All top-level bullets in `FEATURES.md` map to feature slugs as follows:

| Slug | Name | Status | Category |
|---|---|---|---|
| `actions` | Actions | in_progress | combat |
| `console-notifications` | Console Notifications | done | ui |
| `player-gender` | Player Gender | done | character |
| `room-display` | Room Display | done | ui |
| `perception` | Perception / Awareness | done | character |
| `npc-equipment` | NPC Equipment | done | world |
| `advanced-combat` | Advanced Combat Mechanics | in_progress | combat |
| `technology` | Technology | in_progress | technology |
| `long-rest` | Long Rest | done | character |
| `hero-points` | Hero Points | done | character |
| `non-combat-npcs` | Non-Combat NPCs | planned | world |
| `persistent-calendar` | Persistent Calendar | planned | world |
| `use-command` | Use Command Expansion | planned | ui |
| `feat-import` | Feat Import | planned | character |
| `job-development` | Job Development | planned | character |
| `equipment-mechanics` | Equipment Mechanics Expansion | planned | world |
| `editor-commands` | Editor Commands | planned | meta |
| `resting` | Resting | planned | world |
| `map-poi` | Map Points of Interest | planned | ui |
| `room-danger-levels` | Room Danger Levels | planned | world |
| `npc-behaviors` | Per-NPC Custom Behaviors | planned | world |
| `world-map` | World Map / Fast Travel | planned | ui |
| `non-human-npcs` | Non-Human NPCs | planned | world |
| `advanced-health` | Advanced Health Effects | planned | character |
| `advanced-enemies` | Advanced Enemies | planned | world |
| `quests` | Quests | planned | world |
| `factions` | Factions & Allegiances | planned | world |
| `crafting` | Crafting | planned | world |
| `traps` | Traps | planned | world |
| `zones-new` | New Zones | planned | world |
| `web-client` | Web Game Client | planned | ui |
| `game-client-ebiten` | Ebiten Game Client | planned | ui |
| `documentation` | Documentation | planned | meta |

---

## Success Criteria

- `docs/features/index.yaml` exists with all 33 features listed in priority order.
- All 33 `docs/features/<slug>.md` files exist with content migrated from `FEATURES.md`.
- `docs/requirements/FEATURES.md` contains only the redirect stub.
- `docs/requirements/ARCHITECTURE.md` SYSREQ-2 references `docs/features/` as canonical source.
- No requirement text is altered during migration.