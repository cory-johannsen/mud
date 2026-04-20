---
issue: 204
title: Zone map click-to-travel with auto-navigation along explored paths
slug: zone-map-click-to-travel
date: 2026-04-19
---

## Summary

Allow players to click any explored room on the zone map to automatically
navigate to it along the shortest path of explored rooms. The client computes
the path via BFS, then replays `MoveRequest` messages on a configurable timer.
Each individual move still passes through all server-side validation (combat
block, faction gates, etc.).

---

## Architecture Overview

- **Pathfinding**: client-side BFS over `MapTile[]` data already held in the
  React frontend. Graph edges come from the existing `SameZoneExitTarget`
  entries on each tile (direction + target room ID). Only explored tiles are
  eligible as graph nodes.
- **Execution**: `useAutoNav` hook fires one `MoveRequest` per step on a
  configurable timer. Each request goes through the existing gRPC stream and
  all server movement validation.
- **Interruption**: clicking any explored room cancels any active path and
  starts a new one; clicking the current room cancels.
- **Backend changes**: minimal — one new proto field (`explored` bool on
  `MapTile`) and one new server config key (`auto_nav_step_ms`).

---

## Requirements

### REQ-CNT-1: Proto — MapTile explored field

`MapTile` MUST gain a new field:

```proto
bool explored = 13; // true if the player has physically entered this room
```

The server map handler MUST set `explored = true` for rooms present in
`ExploredCache[zoneID][roomID]` and `explored = false` for rooms present only
in `AutomapCache`.

### REQ-CNT-2: Server config — step delay

The server config YAML MUST support a new key:

```yaml
auto_nav_step_ms: 1000   # default; minimum 100
```

The value MUST be included in the session-init `ServerEvent` so the client
receives it at login. The client MUST store it in `GameContext` alongside other
session configuration.

### REQ-CNT-3: Pathfinding utility

A pure function MUST be implemented at
`cmd/webclient/ui/src/game/autoNav.ts`:

```typescript
function findPath(
  tiles: MapTile[],
  fromId: string,
  toId: string,
): string[] | null
```

- REQ-CNT-3a: BFS MUST traverse only tiles where `explored === true`.
- REQ-CNT-3b: Graph edges MUST be derived exclusively from each tile's
  `sameZoneExitTargets` (field 12 of `MapTile`), which contains
  `{ direction, targetRoomId }` for every same-zone exit.
- REQ-CNT-3c: If `fromId === toId`, the function MUST return `[]` (empty path,
  no movement needed).
- REQ-CNT-3d: If no explored path exists, the function MUST return `null`.
- REQ-CNT-3e: The function MUST have no side effects and no React dependencies;
  it MUST be unit-testable in isolation.

### REQ-CNT-4: Auto-navigation hook

A React hook MUST be implemented at
`cmd/webclient/ui/src/game/useAutoNav.ts`:

```typescript
function useAutoNav(
  tiles: MapTile[],
  currentRoomId: string,
  stepMs: number,
  sendMove: (direction: string) => void,
  onNoPath: (roomName: string) => void,
): {
  start: (targetTile: MapTile) => void;
  cancel: () => void;
  active: boolean;
  destinationRoomId: string | null;
}
```

- REQ-CNT-4a: `start` MUST call `findPath`; if the result is `null`, it MUST
  call `onNoPath(targetTile.roomName)` and return without scheduling any timer.
- REQ-CNT-4b: `start` MUST call `cancel` on any existing active path before
  starting a new one (retarget behavior).
- REQ-CNT-4c: `start` with `targetTile.roomId === currentRoomId` MUST call
  `cancel` and return without dispatching any move or message.
- REQ-CNT-4d: Each timer tick MUST resolve the move direction from the current
  tile's `sameZoneExitTargets` entry whose `targetRoomId` matches `path[0]`,
  then call `sendMove(direction)`.
- REQ-CNT-4e: The hook MUST read `currentRoomId` from its argument on each
  tick, not from a stale closure, so server-confirmed room changes are
  reflected correctly.
- REQ-CNT-4f: When the path is exhausted (all steps sent), the hook MUST clear
  its active state.
- REQ-CNT-4g: `cancel` MUST clear the active timer and path immediately.
- REQ-CNT-4h: `destinationRoomId` MUST be the final room ID of the active path,
  or `null` when inactive. Used by `ZoneMapSvg` to render the destination
  indicator.

### REQ-CNT-5: ZoneMapSvg click handling

`cmd/webclient/ui/src/game/ZoneMapSvg.tsx` MUST accept one new optional prop:

```typescript
onTileClick?: (tile: MapTile) => void
```

- REQ-CNT-5a: Tiles where `tile.explored === true` AND `tile.current !== true`
  MUST render `cursor: pointer` and fire `onTileClick` on click.
- REQ-CNT-5b: Tiles where `tile.current === true` or `tile.explored !== true`
  MUST NOT fire `onTileClick`.
- REQ-CNT-5c: Tiles where `useAutoNav.destinationRoomId === tile.roomId` MUST
  render with a distinct border color (blue, `#4a9eff`) to indicate the active
  navigation target.

### REQ-CNT-6: MapPanel wiring

`cmd/webclient/ui/src/game/panels/MapPanel.tsx` MUST:

- REQ-CNT-6a: Instantiate `useAutoNav` with `sendMove` wired to the existing
  `MoveRequest` dispatch path and `stepMs` from `GameContext`.
- REQ-CNT-6b: Pass `onTileClick` down to `ZoneMapSvg` that calls
  `useAutoNav.start(tile)`.
- REQ-CNT-6c: Implement `onNoPath` to dispatch a console message of the form:
  `No explored path to <room name>.`
  using the existing console message dispatch mechanism.
- REQ-CNT-6d: The zone map view MUST only be shown (not the world map view)
  when click-to-travel is active; clicking the world-map toggle MUST call
  `cancel()` first.

### REQ-CNT-7: Interruption

- REQ-CNT-7a: Clicking any explored room while auto-navigation is active MUST
  cancel the current path and start a new one toward the clicked room (retarget).
- REQ-CNT-7b: Clicking the current room while auto-navigation is active MUST
  cancel the path with no new navigation started.
- REQ-CNT-7c: The server blocking a move (e.g. combat starts mid-path) MUST
  cause the next tick's direction lookup to fail (path[0] is not a
  `sameZoneExitTarget` of the now-current room). The hook MUST detect this
  mismatch and call `cancel()` automatically.

### REQ-CNT-8: Test coverage

- REQ-CNT-8a: Unit tests for `findPath` MUST cover: direct neighbor, multi-hop
  path, destination not explored (returns null), no path through unexplored
  room, `fromId === toId` (returns []).
- REQ-CNT-8b: Unit tests for `useAutoNav` MUST cover: successful navigation
  fires correct sequence of `sendMove` calls; no-path calls `onNoPath`;
  `cancel` stops the timer; retarget cancels existing path and starts new one.
- REQ-CNT-8c: Property-based test: for any two explored tiles that are
  graph-reachable through `sameZoneExitTargets`, `findPath` MUST return a
  non-null path of length ≤ number of explored tiles.

---

## Out of Scope

- Telnet client (feature is web client only; no terminal equivalent).
- Cross-zone auto-navigation (click-to-travel stops at zone boundaries).
- Fast-travel integration (existing instant-teleport fast-travel is unchanged).
- World map click-to-travel (world map clicks remain zone-selection only).
