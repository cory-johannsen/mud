---
title: Structured Battle Maps with Adjustable Size
issue: https://github.com/cory-johannsen/mud/issues/264
date: 2026-04-25
status: spec
prefix: BMAP
depends_on: []
related:
  - "#250 AoE drawing in combat (template placement bounded by grid)"
  - "#251 Smarter NPC movement (candidate cells bounded by grid)"
  - "#254 Detection states (per-pair state already keyed on cells, no grid-size dependence)"
---

# Structured Battle Maps with Adjustable Size

## 1. Summary

The combat grid size is hard-coded today:

- `engine.go:541-542` sets `cbt.GridWidth = 20` and `cbt.GridHeight = 20` in `StartCombat`.
- Telnet renderer constants `combatGridWidth = 20` and `combatGridHeight = 20` (`combat_grid.go:11-12`).
- Web `MapPanel.tsx` reads `gridWidth` / `gridHeight` from `GameContext` state — i.e., it already adapts to whatever the server sends, but the server always sends 20×20.
- Initial placement is hard-coded for a 20×20 grid: players at `GridX=0, GridY=10`, NPCs at `GridX=19, GridY=10+i` (`combat_handler.go:1939-1940`, `:1971-1972`).
- The room model (`internal/game/world/model.go:161-215`) has no per-room combat map size field.

A separate feature document at `docs/features/2d-combat-expansion.md` references 10×10, but the backend ignores that.

The fix is small in scope:

1. Add `Room.CombatMap` (struct) declaring `Width int`, `Height int`, `PlayerSpawn Cell`, `NpcSpawn Cell`, `NpcStackAxis enum {x,y}` — all optional with sensible defaults.
2. `StartCombat` reads from the room when present; otherwise falls back to today's 20×20 with the same hard-coded placements.
3. The `gridWidth` / `gridHeight` fields already on the wire propagate to clients; web auto-resizes, telnet's renderer learns to use the runtime dimensions instead of constants.
4. An admin UI tab adds a battle-map editor for any room.
5. Validation enforces sane bounds (min 5×5, max 30×30) and that all spawn cells fall inside the grid.

## 2. Goals & Non-Goals

### 2.1 Goals

- BMAP-G1: Each room MAY declare a `CombatMap` block in its YAML; rooms without one continue to use the existing 20×20 default.
- BMAP-G2: `StartCombat` honors the room's declared map size and spawn positions.
- BMAP-G3: Web client renders the room's actual grid size (it already adapts).
- BMAP-G4: Telnet renderer uses the runtime grid dimensions instead of constants.
- BMAP-G5: Admin UI gains a "Battle Map" subtab on the existing room editor (or as a new top-level tab if no room editor exists yet) that authors can use to set width / height / spawn cells / stack axis.
- BMAP-G6: All existing combat tests pass; new tests cover non-default sizes and spawn cells.

### 2.2 Non-Goals

- BMAP-NG1: Per-cell terrain (covered by #248 / its sibling spec).
- BMAP-NG2: Per-cell cover objects authored statically in the room (combat already supports `CoverObjects`; this spec doesn't add an editor for them).
- BMAP-NG3: Visual / atmospheric assets (background images, lighting). Pure geometry only.
- BMAP-NG4: Dynamic resize during combat (a wall collapses, expanding the playable area). v1 sets the size at combat start.
- BMAP-NG5: Procedural / generated map sizes from room descriptions. v1 is hand-authored.
- BMAP-NG6: Per-encounter override (one room may host different encounters with different maps). v1 is per-room only.

## 3. Glossary

- **Combat map**: the per-room declared grid that combat uses when initiated in that room.
- **Spawn cell**: an `(x, y)` cell where the first combatant of a given side appears at combat start.
- **Stack axis**: the axis (`x` or `y`) along which subsequent same-side combatants are arrayed when more than one share a side.
- **Default placement**: the existing hard-coded `(0, 10)` players / `(19, 10+i)` NPCs scheme used when a room declares no combat map.

## 4. Requirements

### 4.1 Room Schema

- BMAP-1: `Room` MUST gain an optional `CombatMap *CombatMap` field. The struct MUST declare:
  - `width int` (default 20)
  - `height int` (default 20)
  - `player_spawn Cell` (default `{0, height/2}`)
  - `npc_spawn Cell` (default `{width-1, height/2}`)
  - `npc_stack_axis enum {x, y}` (default `y`)
- BMAP-2: The loader MUST validate:
  - `width` and `height` are within `[5, 30]` inclusive.
  - `player_spawn.x` ∈ `[0, width-1]`, `player_spawn.y` ∈ `[0, height-1]`; same for `npc_spawn`.
  - `player_spawn != npc_spawn`.
- BMAP-3: When a room has no `combat_map` block, `StartCombat` MUST behave exactly as today (20×20, default placements). This is the back-compat guard.

### 4.2 StartCombat Integration

- BMAP-4: `engine.go:StartCombat` MUST read the room's `CombatMap` (if any) and set `cbt.GridWidth`, `cbt.GridHeight`, and the initial combatant positions accordingly.
- BMAP-5: When stacking NPCs along the chosen axis, the stack MUST clamp at the grid boundary. NPCs that cannot fit (insufficient cells) MUST be placed at the nearest valid cell with one warning logged per omitted cell.
- BMAP-6: Players MUST stack along the opposite axis from `npc_stack_axis` by default. When player count exceeds the perpendicular dimension, fall back to filling rows / columns row-major.
- BMAP-7: Cover objects, hazards, and any other room-level cell-bound state MUST have their coordinates validated against the new grid size at combat start; any out-of-bounds entry MUST be silently dropped with a warning. (Current content authors only have positions for 20×20; smaller maps will lose some cover.)

### 4.3 Telnet Renderer

- BMAP-8: Telnet `combat_grid.go` MUST use `cbt.GridWidth` / `cbt.GridHeight` instead of the existing constants.
- BMAP-9: When the rendered grid would exceed the telnet window width, the renderer MUST scale to half-width characters or compress horizontally with a clear `[map clipped]` annotation. The implementer MUST confirm the chosen approach with the user before locking it in.
- BMAP-10: Constants `combatGridWidth` and `combatGridHeight` MAY be retained as defaults but MUST NOT override runtime values from `cbt`.

### 4.4 Web Client

- BMAP-11: The web client already adapts to `gridWidth` / `gridHeight` via `MapPanel.tsx`. No change required beyond ensuring the wire payload is up-to-date when `cbt.GridWidth/Height` differ from 20.
- BMAP-12: The cell-size auto-fit at `MapPanel.tsx:73-74` MUST continue to work for the new size range (5×5 to 30×30); a quick visual smoke test with each extreme is required at landing.
- BMAP-13: AoE template placement (#250) and movement candidate enumeration (#251) already consume `cbt.GridWidth/Height`; no changes required, but the implementer MUST confirm the bounds checks are not hard-coded to 20.

### 4.5 Admin UI

- BMAP-14: A new admin tab (or subtab on the existing room editor) MUST allow setting:
  - Width / height (numeric inputs with the `[5, 30]` validation).
  - Player and NPC spawn cells (selectable on a preview grid).
  - Stack axis (toggle).
- BMAP-15: Saved changes MUST persist to the underlying room YAML or, if rooms are loaded from the database, to the room record. Existing room-edit save paths apply.
- BMAP-16: A "preview" panel MUST render the current `CombatMap` as a small grid with spawn cells highlighted, so authors verify the geometry before saving.

### 4.6 Tests

- BMAP-17: Existing `StartCombat` tests MUST pass (default 20×20 path).
- BMAP-18: New tests MUST cover:
  - Loader validation: out-of-bounds spawn rejected; oversize / undersize grid rejected.
  - StartCombat reads custom 10×10 map and places players / NPCs at declared cells.
  - StartCombat falls back to defaults when no `combat_map` block.
  - NPC stacking clamps at boundary and logs warning.
  - Out-of-bounds cover object dropped with warning.
- BMAP-19: A property test under `internal/game/combat/testdata/rapid/TestStartCombat_GridSize_Property/` MUST verify that for any valid `(width, height, spawns)` tuple, all spawned combatants land at in-bounds cells with no overlap with cover.

## 5. Architecture

### 5.1 Where the new code lives

```
content/zones/<zone>.yaml
  rooms:
    <id>:
      combat_map:
        width: 12
        height: 8
        player_spawn: { x: 1, y: 4 }
        npc_spawn: { x: 10, y: 4 }
        npc_stack_axis: y

internal/game/world/
  model.go                       # Room.CombatMap *CombatMap; CombatMap struct + Cell

internal/game/combat/
  engine.go                      # StartCombat reads room.CombatMap; placement helper

internal/frontend/telnet/
  combat_grid.go                 # use cbt.GridWidth/Height

cmd/webclient/ui/src/admin/
  RoomCombatMapTab.tsx           # NEW (or subtab on existing room editor)

api/proto/game/v1/game.proto
  CombatStartView already carries grid_width/grid_height; ensure spawn cells are visible if the
  client wants them for animations (optional).
```

### 5.2 Combat start flow

```
combatH.StartCombat(roomID, players, npcs)
   │
   ├── room := world.LookupRoom(roomID)
   ├── cm := room.CombatMap (or default20x20)
   ├── cbt.GridWidth, cbt.GridHeight = cm.Width, cm.Height
   ├── place players at cm.PlayerSpawn (stack perpendicular to npc_stack_axis)
   ├── place NPCs at cm.NpcSpawn (stack along npc_stack_axis), clamp at boundary
   ├── validate cover/hazard cells against grid → drop out-of-bounds with warning
   ▼
combat ready; broadcast CombatStartView with grid size; clients render
```

### 5.3 Single sources of truth

- Grid dimensions: `Combat.GridWidth/Height` populated from `Room.CombatMap` only.
- Spawn cells: `Room.CombatMap.PlayerSpawn / NpcSpawn` only.
- Renderer dimension consumption: read from `Combat`, not from constants.

## 6. Open Questions

- BMAP-Q1: Telnet's clipping strategy when a 30-wide grid exceeds an 80-column terminal. Options: half-width chars, horizontal scroll, or a "snapshot view" with viewport. Recommendation: half-width chars (each cell becomes one character instead of two). Confirm at landing.
- BMAP-Q2: When a room has cover objects authored at coordinates that fall outside a smaller declared `CombatMap`, the spec says drop with warning. Should a content-validation pass (linter) refuse to build the world if such mismatches exist? Recommendation: yes — add a linter check, but warn-only on hot reload so authors can iterate.
- BMAP-Q3: A `npc_stack_axis: y` with NPCs stacked at `(19, 10), (19, 11), …` — does the stack continue past `(19, height-1)` or wrap to `(18, 10)`? Recommendation: wrap to the previous column at the same starting y, then continue.
- BMAP-Q4: For multi-encounter rooms (a hub room that can host different encounters), a per-encounter override would be preferable to the per-room scheme. Recommendation: defer; v1 is per-room. The encounter system can layer per-encounter when it lands.
- BMAP-Q5: Does the admin UI need a "preview" of cell scale at typical web pixel sizes so authors don't author unreadable 30×30 grids? Recommendation: yes — the preview panel (BMAP-16) shows scaled cells.

## 7. Acceptance

- [ ] All existing combat tests pass with no `combat_map` declared on test rooms.
- [ ] A 10×10 `combat_map` declaration produces a 10×10 grid in `Combat`, in `MapPanel.tsx`, and in telnet output.
- [ ] Player and NPC spawn cells respect the declared positions; stacking clamps at boundary with logged warnings.
- [ ] Out-of-bounds cover objects are dropped with a warning, not crashing combat.
- [ ] Admin tab can edit a room's `combat_map` and preview the grid.
- [ ] AoE template placement (#250) and movement enumeration (#251) honor the new bounds.
- [ ] Property test passes across the full size range.

## 8. Out-of-Scope Follow-Ons

- BMAP-F1: Per-encounter map overrides.
- BMAP-F2: Dynamic mid-combat resize (wall collapse, room expansion).
- BMAP-F3: Authoring tools for cover objects / hazards on the map preview.
- BMAP-F4: Background-art / lighting overlays.
- BMAP-F5: Procedural map size derivation from room description.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/264
- Hard-coded grid size: `internal/game/combat/engine.go:541-542`
- Telnet constants: `internal/frontend/telnet/combat_grid.go:11-12`
- Web grid renderer: `cmd/webclient/ui/src/game/panels/MapPanel.tsx:70-90`
- Initial placement: `internal/gameserver/combat_handler.go:1939-1940` (players), `:1971-1972` (NPCs)
- Room model: `internal/game/world/model.go:161-215`
- AoE bounds consumer: `docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md`
- Movement bounds consumer: `docs/superpowers/specs/2026-04-24-smarter-npc-movement-in-combat.md` MOVE-8
- Existing 2D combat feature doc (10×10 reference): `docs/features/2d-combat-expansion.md`
