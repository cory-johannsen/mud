# Spec: Web Game Client — Phase 2

**GitHub Issue:** cory-johannsen/mud#13
**Date:** 2026-04-12
**Supersedes:** `docs/superpowers/specs/2026-03-26-web-client-phase2-design.md`

---

## Overview

Phase 2 extends the web client in three sequential phases, with the map as the highest priority. All three phases share a single GitHub issue and are tracked together.

| Phase | Scope | Priority |
|-------|-------|----------|
| Phase A | Combat map — configurable grid, compass navigation | Highest |
| Phase B | Zone & world map polish — section separators, resizable splitter | High |
| Phase C | PixiJS tiled room scene — sprites, asset pack, combat animations | Medium |

---

## Phase A: Combat Map

### Context

The 2D combat grid (`renderBattleGrid`) is already implemented in `MapPanel.tsx` with a hardcoded 20×20 size. The backend already supports compass directions (n/s/e/w/ne/nw/se/sw) in `handleStride` and `handleStep`. `RoundStartEvent` already carries `initial_positions` with x/y per combatant. The grid dimensions (`Combat.GridWidth`, `Combat.GridHeight`) are not yet sent to the client.

### REQ-PA-1: Grid dimensions from server

- REQ-PA-1a: `RoundStartEvent` MUST include `int32 grid_width = 6` and `int32 grid_height = 7`
- REQ-PA-1b: `combat_handler.go` MUST populate these fields from `combat.GridWidth` and `combat.GridHeight` when broadcasting `RoundStartEvent`
- REQ-PA-1c: The TypeScript `RoundStartEvent` interface MUST add `gridWidth?: number`, `grid_width?: number`, `gridHeight?: number`, `grid_height?: number`
- REQ-PA-1d: `GameContext` MUST store `combatGridWidth: number` and `combatGridHeight: number` in state, defaulting to 20 if absent from the event

### REQ-PA-2: Configurable grid rendering

- REQ-PA-2a: `renderBattleGrid` MUST read grid dimensions from `state.combatGridWidth` and `state.combatGridHeight` instead of the hardcoded constant `GRID_SIZE = 20`
- REQ-PA-2b: Cell pixel size MUST be computed dynamically so the grid fills the available panel area: `cellPx = Math.floor(Math.min(panelWidth, panelHeight) / Math.max(gridWidth, gridHeight))`
- REQ-PA-2c: Minimum cell size MUST be 12px; maximum MUST be 32px

### REQ-PA-3: Compass direction navigation controls

- REQ-PA-3a: The combat map MUST render a 3×3 directional pad (NW, N, NE / W, center, E / SW, S, SE) replacing the current hardcoded action buttons (Close, Stride Away, Step, Step Away, Flee!)
- REQ-PA-3b: Each compass button sends `stride <dir>` (e.g. `stride n`, `stride ne`) via `sendCommand`
- REQ-PA-3c: The center button sends `stride` (toward nearest enemy)
- REQ-PA-3d: A separate **Flee** button MUST remain outside the directional pad, styled distinctly in red
- REQ-PA-3e: A **Step mode** toggle MUST switch all directional buttons to send `step <dir>` instead of `stride <dir>`

### REQ-PA-4: AP-gated and bounds-gated nav controls

- REQ-PA-4a: Navigation buttons MUST be disabled when the player's `ap_remaining = 0`
- REQ-PA-4b: Navigation buttons MUST be disabled in directions that would move the player outside the grid bounds (e.g. N button disabled when player `y = 0`)
- REQ-PA-4c: The player's current AP MUST be visible in the combat map header as pip indicators: filled circle per remaining AP, empty circle per spent AP
- REQ-PA-4d: AP state MUST be read from the `CombatantPosition` entry matching the current player's name in `state.combatPositions`

### REQ-PA-5: Combatant hover details

- REQ-PA-5a: Hovering a non-empty cell MUST display a tooltip showing combatant name, current AP remaining, and (if available) HP
- REQ-PA-5b: The tooltip MUST use the same portal-based `RoomTooltip` pattern used by the zone map

---

## Phase B: Zone & World Map Polish

### REQ-PB-1: Section separators in zone map legend

- REQ-PB-1a: The legend rendered alongside the ASCII zone map MUST visually separate the Legend, POIs, and Rooms sections with a horizontal rule between each
- REQ-PB-1b: Each section MUST have a bold uppercase header label (e.g. `LEGEND`, `POINTS OF INTEREST`, `ROOMS`)

### REQ-PB-2: Resizable splitter between map grid and legend

- REQ-PB-2a: A draggable vertical splitter bar MUST divide the ASCII map grid and the legend panel, consistent with the existing horizontal splitters between Room/Map and Map/Character panels
- REQ-PB-2b: The splitter MUST be draggable; dragging adjusts the width ratio between map and legend
- REQ-PB-2c: The splitter position MUST be persisted in `localStorage` under key `mud-map-splitter` and restored on load
- REQ-PB-2d: Default split: 60% map, 40% legend

---

## Phase C: PixiJS Tiled Room Scene

Phase C carries forward the approved design from `docs/superpowers/specs/2026-03-26-web-client-phase2-design.md` without modification. All requirements REQ-WC2-1 through REQ-WC2-24 from that document remain in force for Phase C. The key requirements are summarised below for reference:

- REQ-PC-1 (`= REQ-WC2-1..5`): `internal/client/assets` sub-package — GitHub Releases version check, `FetchLatestVersion`, `ParseVersion`
- REQ-PC-2 (`= REQ-WC2-6..7`): Go asset proxy endpoint `GET /api/assets/version`; `WebConfig.GitHubReleasesURL`
- REQ-PC-3 (`= REQ-WC2-8..12`): `AssetPackContext` — download, SHA-256 verify, IndexedDB cache, `PixiTextureMap`, `TilesConfig`
- REQ-PC-4 (`= REQ-WC2-13..15`): Room panel split into Scene sub-panel (~60%) + Text sub-panel (~40%)
- REQ-PC-5 (`= REQ-WC2-16..22`): `ScenePanel` React component — BackgroundLayer, NpcLayer, PlayerLayer, ExitLayer, AnimationLayer
- REQ-PC-6 (`= REQ-WC2-23..24`): `CombatAnimationQueue` — `attack`, `hit-flash`, `death` animation types

---

## Files to Modify

### Phase A

- `api/proto/game/v1/game.proto` — add `grid_width = 6`, `grid_height = 7` to `RoundStartEvent`
- `api/proto/game/v1/game.pb.go` — regenerate
- `internal/gameserver/combat_handler.go` — populate new fields in `RoundStartEvent` broadcast
- `cmd/webclient/ui/src/proto/index.ts` — add grid dimension fields to `RoundStartEvent`
- `cmd/webclient/ui/src/game/GameContext.tsx` — add `combatGridWidth`, `combatGridHeight` to state; populate from `RoundStartEvent`
- `cmd/webclient/ui/src/game/panels/MapPanel.tsx` — dynamic grid size, compass dpad, AP pips, hover tooltip

### Phase B

- `cmd/webclient/ui/src/game/panels/MapPanel.tsx` — section separators, resizable splitter

### Phase C

- `internal/client/assets/` (new package)
- `cmd/webclient/` — asset proxy endpoint, `WebConfig`
- `cmd/webclient/ui/src/` — `AssetPackContext`, `ScenePanel`, `CombatAnimationQueue`
