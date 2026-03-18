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
3. Update `SYSREQ-2` in `.claude/rules/AGENTS.md` to designate `docs/features/` as the canonical
   source of truth for product features (scope limited to FEATURES.md content; all other
   `docs/requirements/` files remain maintained per existing obligation).
4. Deprecate `FEATURES.md` with a redirect notice.

---

## Non-Goals

- Changing existing requirement text or checklist items.
- Implementing any of the features described.
- Deleting `FEATURES.md` (it is kept as a redirect stub).
- Migrating or deprecating any `docs/requirements/` file other than `FEATURES.md`.

---

## Scope of Migration — Consolidation Rules

Every top-level bullet in `FEATURES.md` is migrated under one of these four rules:

**Rule A — Own file:** A top-level feature section with sub-bullets or substantive content
receives its own `docs/features/<slug>.md` file and its own `index.yaml` entry.

**Rule B — Section merge:** Two or more top-level bullets that describe the same feature
domain are merged into a single file. Each merged slug gets its own `## <Section Name>` heading
in the shared file, containing only the checklist lines that belong to that slug's source bullet.
The `file` field in `index.yaml` for each merged slug points to the shared file.

Examples:
- "Non-combat NPCs" and "Non-combat NPCs — All zones" → both → `non-combat-npcs.md`, each under
  its own `## ` heading.
- "Disarm action" (FEATURES.md lines 145–146, a 2-line cross-reference bullet) → merged into
  `npc-equipment.md` under a `## Disarm Action` heading containing those 2 lines verbatim.

**Rule C — Completed-items consolidation:** Top-level checked items (`[x]`) that have at most
two sub-bullets and no planned follow-on work are consolidated into `completed-items.md`. This
covers: "Console notifications", "Player prompt should include mental state", "Room section
should include NPC mental state", "map and legend 2-column view", "Replace Level+10", "NPCs
rob the player". Each of these items is reproduced in full (with any sub-bullets) in
`completed-items.md`; they each also appear as individual entries in `index.yaml` pointing to
`file: docs/features/completed-items.md` so they remain independently queryable.

**Rule D — Housekeeping consolidation:** Top-level items that are project maintenance tasks
rather than game features are consolidated into `housekeeping.md`. This covers: "TODO list",
"`trainskill` does not persist", "`grant` Editor command", "refactor to use `wire`". Each
appears in full in `housekeeping.md` and as individual `index.yaml` entries pointing to
`file: docs/features/housekeeping.md`.

For rules C and D, the `file` field in `index.yaml` deviates from the `docs/features/<slug>.md`
convention and explicitly names the consolidated file path. The `file` field is authoritative
for locating the content; the slug convention is the default only when `file` would equal
`docs/features/<slug>.md`.

---

## Directory Structure

```
docs/features/
  index.yaml                  # priority-ordered manifest with metadata per feature
  actions.md                  # Rule A
  advanced-combat.md          # Rule A
  advanced-enemies.md         # Rule A
  advanced-health.md          # Rule A
  completed-items.md          # Rule C: console-notifications, player-prompt-mental-state,
                              #         room-mental-state, map-legend-2col, replace-level10,
                              #         npcs-rob-player
  crafting.md                 # Rule A
  documentation.md            # Rule A
  editor-commands.md          # Rule A
  equipment-mechanics.md      # Rule A
  factions.md                 # Rule A
  feat-import.md              # Rule A
  game-client-ebiten.md       # Rule A
  hero-points.md              # Rule A
  housekeeping.md             # Rule D: todo-list, trainskill-persistence,
                              #         grant-editor-command, wire-refactor
  job-development.md          # Rule A
  long-rest.md                # Rule A
  map-poi.md                  # Rule A
  multiplayer-combat.md       # Rule A
  non-combat-npcs.md          # Rule B: merges "Non-combat NPCs" + "Non-combat NPCs — All zones"
  non-human-npcs.md           # Rule A
  npc-behaviors.md            # Rule A
  npc-equipment.md            # Rule A — includes "Disarm action" top-level bullet (Rule B)
  npcs-named.md               # Rule A: Wayne Dawg, Dwayne Dawg, Jennifer Dawg
  perception.md               # Rule A
  persistent-calendar.md      # Rule A
  player-gender.md            # Rule A
  quests.md                   # Rule A
  resting.md                  # Rule A
  room-danger-levels.md       # Rule A
  room-display.md             # Rule A
  technology.md               # Rule A
  traps.md                    # Rule A
  use-command.md              # Rule A
  web-client.md               # Rule A
  world-map.md                # Rule A
  zones-new.md                # Rule A
```

Total: **36 `.md` files** + `index.yaml`.

---

## Index Schema

`docs/features/index.yaml` contains a top-level `features` list ordered by ascending `priority`.
The YAML list order and the `priority` integer MUST agree — the entry with the lowest `priority`
value MUST appear first in the file. When entries are reordered, both the `priority` values and
the list position must be updated together.

### Required fields (all entries must have these)

| Field | Type | Description |
|---|---|---|
| `slug` | string | Unique identifier; kebab-case; matches filename for Rule A entries |
| `name` | string | Human-readable feature name |
| `status` | enum | `done`, `in_progress`, or `planned` — see Status Values below |
| `priority` | integer | Unique positive integer; lower = higher priority |
| `category` | enum | `combat`, `character`, `technology`, `world`, `ui`, or `meta` |
| `file` | string | Absolute-from-repo-root path to the feature markdown file |

### Optional fields

| Field | Type | Description |
|---|---|---|
| `dependencies` | list of slugs | Slugs that must be complete before this feature can start; omit or use `[]` for no dependencies |

### Status Values

- `done` — every checklist item in the feature's section(s) is checked (`[x]`)
- `in_progress` — at least one checklist item is checked and at least one is unchecked
- `planned` — no checklist items are checked (or the feature has no checklist)

For consolidated files (Rule C and Rule D), status is evaluated per slug based only on the
checklist items that belong to that slug's section within the consolidated file, not the whole file.

### Slug Naming Convention

Slugs are derived from the feature's heading or top-level bullet text using these rules:
- Lowercase all characters
- Replace spaces, dashes, em-dashes, and any punctuation with a single hyphen
- Remove leading/trailing hyphens
- Collapse consecutive hyphens to one

Example: "Non-combat NPCs — All zones" → `non-combat-npcs-all-zones`

### Priority Assignment During Migration

During migration, each slug receives a unique priority value assigned in multiples of 10.
Priority is assigned by the order the slug's source top-level bullet first appears in
`FEATURES.md` — the first bullet's slug(s) get priority 10, the next bullet's slug(s) get
priority 20, and so on. When a single source bullet produces multiple slugs (Rule B, C, D),
all slugs from that bullet share the same priority block: increment by 1 for each sibling
(e.g., if `non-combat-npcs` gets 100, `non-combat-npcs-all-zones` gets 101).

Gaps (multiples of 10) allow insertion between features without renumbering the whole file.
After migration, the user may edit `index.yaml` to reorder priorities; `priority` values must
remain unique after any edit.

### Example entry (Rule A)

```yaml
- slug: advanced-combat
  name: Advanced Combat Mechanics
  status: in_progress
  priority: 10
  category: combat
  file: docs/features/advanced-combat.md
  dependencies:
    - actions
```

### Example entry (Rule C — consolidated)

```yaml
- slug: console-notifications
  name: Console Notifications
  status: done
  priority: 40
  category: ui
  file: docs/features/completed-items.md
```

---

## Feature File Format

Each `docs/features/<slug>.md` follows this structure:

```markdown
# Feature Name

<one-paragraph description. If the section in FEATURES.md has introductory prose, use that
verbatim. If no prose exists, write a one-sentence summary derived from the checklist content.
This is the only text added during migration.>

## Requirements

<verbatim checklist content migrated from FEATURES.md; indentation and check state preserved>
```

For consolidated files (`completed-items.md`, `housekeeping.md`, `non-combat-npcs.md`,
`npc-equipment.md`), use a `## <Section Name>` heading per merged source section (one heading
per slug that maps to the file), each followed by the verbatim checklist for that section.
Every slug that maps to a consolidated file MUST have its own named `##` section.

**Single-line features:** Some top-level bullets in `FEATURES.md` are a single line with no
sub-bullets (e.g., `documentation`, `quests`, `crafting`, `traps`, `factions`,
`persistent-calendar`). For these, the feature file contains:
- The `# Feature Name` heading
- A one-sentence description (the implementer derives this from the single bullet line itself —
  paraphrase it as a description, do not copy it verbatim as the description paragraph)
- A `## Requirements` section containing the single bullet line verbatim as the checklist item

---

## SYSREQ-2 Update

`.claude/rules/AGENTS.md` SYSREQ-2 is updated from:

> SYSREQ-2: Agents MUST update and maintain the markdown files in `/docs/requirements/` as
> product requirements evolve.

To:

> SYSREQ-2: Agents MUST treat `docs/features/index.yaml` and the files in `docs/features/` as
> the canonical source of truth for product feature definitions and priority. When adding a new
> feature, agents MUST create `docs/features/<slug>.md` and add a corresponding entry to
> `docs/features/index.yaml`. The file `docs/requirements/FEATURES.md` is a deprecated redirect
> stub and MUST NOT be edited. All other files in `docs/requirements/` remain maintained per
> the original obligation: agents MUST update them as requirements evolve.

---

## FEATURES.md Replacement

`docs/requirements/FEATURES.md` is replaced in its entirety with:

```markdown
# Features

> **Deprecated.** Feature definitions have been migrated to `docs/features/`.
> See `docs/features/index.yaml` for the priority-ordered feature manifest.
> Individual feature specs are in `docs/features/<slug>.md`.
> Do not edit this file.
```

---

## Migration Mapping

The table below lists every `index.yaml` entry (one row per slug). 46 slugs map to 36 files.

| Slug | Name | Status | Category | File | Rule |
|---|---|---|---|---|---|
| `actions` | Actions | in_progress | combat | `actions.md` | A |
| `advanced-combat` | Advanced Combat Mechanics | in_progress | combat | `advanced-combat.md` | A |
| `advanced-enemies` | Advanced Enemies | planned | world | `advanced-enemies.md` | A |
| `advanced-health` | Advanced Health Effects | planned | character | `advanced-health.md` | A |
| `console-notifications` | Console Notifications | done | ui | `completed-items.md` | C |
| `crafting` | Crafting | planned | world | `crafting.md` | A |
| `disarm-action` | Disarm Action | done | combat | `npc-equipment.md` | B |
| `documentation` | Documentation | planned | meta | `documentation.md` | A |
| `editor-commands` | Editor Commands | planned | meta | `editor-commands.md` | A |
| `equipment-mechanics` | Equipment Mechanics Expansion | planned | world | `equipment-mechanics.md` | A |
| `factions` | Factions & Allegiances | planned | world | `factions.md` | A |
| `feat-import` | Feat Import | planned | character | `feat-import.md` | A |
| `game-client-ebiten` | Ebiten Game Client | planned | ui | `game-client-ebiten.md` | A |
| `grant-editor-command` | Grant Editor Command | done | meta | `housekeeping.md` | D |
| `hero-points` | Hero Points | done | character | `hero-points.md` | A |
| `job-development` | Job Development | planned | character | `job-development.md` | A |
| `long-rest` | Long Rest | done | character | `long-rest.md` | A |
| `map-legend-2col` | Map/Legend 2-Column View | done | ui | `completed-items.md` | C |
| `map-poi` | Map Points of Interest | planned | ui | `map-poi.md` | A |
| `multiplayer-combat` | Multi-Player Combat | in_progress | combat | `multiplayer-combat.md` | A |
| `non-combat-npcs` | Non-Combat NPCs | planned | world | `non-combat-npcs.md` | B |
| `non-combat-npcs-all-zones` | Non-Combat NPCs — All Zones | planned | world | `non-combat-npcs.md` | B |
| `non-human-npcs` | Non-Human NPCs | planned | world | `non-human-npcs.md` | A |
| `npc-behaviors` | Per-NPC Custom Behaviors | planned | world | `npc-behaviors.md` | A |
| `npc-equipment` | NPC Equipment | done | world | `npc-equipment.md` | A |
| `npcs-named` | Named NPCs (Wayne/Dwayne/Jennifer Dawg) | planned | world | `npcs-named.md` | A |
| `npcs-rob-player` | NPCs Rob Player | done | world | `completed-items.md` | C |
| `perception` | Perception / Awareness | done | character | `perception.md` | A |
| `persistent-calendar` | Persistent Calendar | planned | world | `persistent-calendar.md` | A |
| `player-gender` | Player Gender | done | character | `player-gender.md` | A |
| `player-prompt-mental-state` | Player Prompt Mental State | done | ui | `completed-items.md` | C |
| `quests` | Quests | planned | world | `quests.md` | A |
| `replace-level10` | Replace Level+10 DCs | done | combat | `completed-items.md` | C |
| `resting` | Resting | planned | world | `resting.md` | A |
| `room-danger-levels` | Room Danger Levels | planned | world | `room-danger-levels.md` | A |
| `room-display` | Room Display | done | ui | `room-display.md` | A |
| `room-mental-state` | Room Section Mental State | done | ui | `completed-items.md` | C |
| `technology` | Technology | in_progress | technology | `technology.md` | A |
| `todo-list` | TODO List | done | meta | `housekeeping.md` | D |
| `trainskill-persistence` | Trainskill Persistence | done | meta | `housekeeping.md` | D |
| `traps` | Traps | planned | world | `traps.md` | A |
| `use-command` | Use Command Expansion | planned | ui | `use-command.md` | A |
| `web-client` | Web Game Client | planned | ui | `web-client.md` | A |
| `wire-refactor` | Wire Dependency Injection Refactor | planned | meta | `housekeeping.md` | D |
| `world-map` | World Map / Fast Travel | planned | ui | `world-map.md` | A |
| `zones-new` | New Zones | planned | world | `zones-new.md` | A |

Total: **46 slugs** → **35 `.md` files**.

---

## Adding New Features (Post-Migration)

To add a new feature after migration:

1. Create `docs/features/<slug>.md` following the feature file format.
2. Add a new entry to `docs/features/index.yaml` with a unique `priority` value, inserting it
   at the desired position in the list and updating the `priority` integers of adjacent entries
   if needed to maintain sort order.
3. Do NOT edit `docs/requirements/FEATURES.md`.

---

## Success Criteria

1. `docs/features/index.yaml` exists and contains exactly 46 entries, each with all required
   fields populated, `priority` values unique, and list order matching ascending `priority`.
2. All 36 `docs/features/<slug>.md` files listed in the Directory Structure section exist.
3. For every non-consolidated feature file (Rule A files), the checklist lines in
   `## Requirements` are byte-identical (whitespace-normalized) to the corresponding lines in
   the original `FEATURES.md`. Compliance is verified by diffing checklist lines only (lines
   beginning with `- [` or spaces followed by `- [`).
4. For all consolidated files (`completed-items.md`, `housekeeping.md`, `non-combat-npcs.md`,
   `npc-equipment.md`), all source checklist items from `FEATURES.md` are present and
   byte-identical to their originals; each slug's content appears under its own `## ` heading;
   no checklist item check-state (`[x]` / `[ ]`) is altered.
5. Each entry's `status` value is consistent with the check state of items in its section:
   `done` entries have all `[x]`; `in_progress` entries have mixed; `planned` entries have all
   `[ ]` or no checklist.
6. `docs/requirements/FEATURES.md` is ≤ 6 lines and contains only the redirect stub text.
7. `.claude/rules/AGENTS.md` SYSREQ-2 matches the updated text in this spec verbatim.
