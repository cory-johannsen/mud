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

## Phase B: Sprite-Based Zone & World Maps

### Context

The ASCII zone map is too information-dense to be legible. Phase B replaces both the zone map and world map with SVG-based graphical renderers. No external asset pipeline is required — all tiles and icons are drawn programmatically. `mapRenderer.ts` and the `renderLines` rendering path are replaced entirely.

The data model is already sufficient: each `MapTile` has `(x, y)` grid coordinates, exit directions, danger level, POI types, boss flag, and zone-exit info. Room connectivity is inferred by finding adjacent tiles at the expected offset for each exit direction.

### REQ-PB-1: SVG zone map renderer

- REQ-PB-1a: A new `ZoneMapSvg` React component MUST replace the `<pre className="map-ascii">` + `renderLines` rendering path in `MapPanel.tsx`
- REQ-PB-1b: `ZoneMapSvg` MUST accept `tiles: MapTile[]` and render an SVG element that fills its container
- REQ-PB-1c: Each tile MUST be rendered as a rounded rectangle (`<rect>`) at pixel position `(x * CELL_W, y * CELL_H)`. Default cell size: 56×36 px
- REQ-PB-1d: Tile fill color MUST be determined by `dangerLevel` using the existing color palette: `safe` → `#2a4a2a`, `sketchy` → `#3a3a1a`, `dangerous` → `#4a2a1a`, `deadly` → `#4a1a1a`, unknown/undiscovered → `#1e1e2e`
- REQ-PB-1e: The current room tile MUST be outlined with a 2px `#f0c040` stroke and rendered on top of all others
- REQ-PB-1f: Boss room tiles MUST use a 2px `#cc4444` stroke
- REQ-PB-1g: Each tile MUST display the room name as a `<text>` element centered within the tile, font size 9px, clipped to tile width. If the name exceeds the tile, it MUST be truncated with an ellipsis
- REQ-PB-1h: Exit connectors MUST be drawn as `<line>` elements between the centers of adjacent tiles. Connector color: `#555`. Zone-crossing exits MUST use a dashed `<line>` with color `#8888ff`
- REQ-PB-1i: Exit direction offsets for connector inference MUST be: `n`=(0,−1), `s`=(0,+1), `e`=(+1,0), `w`=(−1,0), `ne`=(+1,−1), `nw`=(−1,−1), `se`=(+1,+1), `sw`=(−1,+1). Diagonal connectors connect tile corners rather than centers
- REQ-PB-1j: POI icons MUST be rendered as small `<text>` elements (unicode symbols) in the top-right corner of the tile at 10px. Symbols: `merchant` → `$`, `healer` → `+`, `trainer` → `T`, `quest_giver` → `!`, `motel` → `Z`, `npc` → `@`, `guard` → `G`
- REQ-PB-1k: Hovering a tile MUST trigger the existing `RoomTooltip` portal with room details, preserving current hover behavior
- REQ-PB-1l: The SVG MUST be scrollable within its container when the map exceeds the panel bounds

### REQ-PB-2: SVG world map renderer

- REQ-PB-2a: A new `WorldMapSvg` React component MUST replace the `<table>`-based `WorldMapView` component
- REQ-PB-2b: Each `WorldZoneTile` MUST be rendered as a rectangle at `(worldX * ZONE_W, worldY * ZONE_H)`. Default zone cell size: 80×50 px
- REQ-PB-2c: Tile fill color MUST follow the same danger-level palette as REQ-PB-1d. Undiscovered tiles MUST render as dark `#111` with no label
- REQ-PB-2d: Discovered tiles MUST display the zone name as centered `<text>`, font size 10px, truncated if needed
- REQ-PB-2e: The current zone tile MUST have a 2px `#f0c040` stroke
- REQ-PB-2f: Clicking a discovered, non-current tile MUST call the existing `onTravel(zoneId)` handler. Cursor MUST be `pointer` on hoverable tiles
- REQ-PB-2g: A legend row below the SVG MUST show color swatches for each danger level and "Undiscovered", matching the existing legend

### REQ-PB-3: Resizable splitter — map panel layout

- REQ-PB-3a: The zone map view MUST be divided into a left SVG map pane and a right details pane (showing POI legend and room list) with a draggable vertical splitter
- REQ-PB-3b: The splitter position MUST be persisted in `localStorage` under key `mud-map-splitter` and restored on load; default split 70% map / 30% details
- REQ-PB-3c: The details pane MUST list discovered rooms grouped by danger level with their POI icons

### REQ-PB-4: Remove ASCII map infrastructure

- REQ-PB-4a: `cmd/webclient/ui/src/game/mapRenderer.ts` MUST be deleted once `ZoneMapSvg` and `WorldMapSvg` are complete
- REQ-PB-4b: All imports of `renderMapTiles` and `ColoredLine` MUST be removed from `MapPanel.tsx`

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

- `cmd/webclient/ui/src/game/panels/MapPanel.tsx` — replace ASCII rendering with `ZoneMapSvg` and `WorldMapSvg`
- `cmd/webclient/ui/src/game/ZoneMapSvg.tsx` (new component)
- `cmd/webclient/ui/src/game/WorldMapSvg.tsx` (new component)
- `cmd/webclient/ui/src/game/mapRenderer.ts` — delete after Phase B

### Phase C

- `internal/client/assets/` (new package)
- `cmd/webclient/` — asset proxy endpoint, `WebConfig`
- `cmd/webclient/ui/src/` — `AssetPackContext`, `ScenePanel`, `CombatAnimationQueue`
