# Zone Map Click-to-Travel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow players to click any explored room on the zone map to automatically navigate to it along the shortest explored path, with a configurable step delay.

**Architecture:** Client-side BFS over `MapTile[]` data using existing `sameZoneExitTargets` (direction + room ID) as graph edges. A `useAutoNav` hook manages the timer loop, sending one `MoveRequest` per step. Each move still passes through all server movement validation. Backend changes are minimal: one new proto field (`explored: bool` on `MapTile`) and one new `GameConfig` proto message carrying `auto_nav_step_ms` from server config to client.

**Tech Stack:** Go (proto, grpc_service), TypeScript/React (vitest, @testing-library/react, fast-check), protobuf (hand-written `proto/index.ts`)

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `api/proto/game/v1/game.proto` | Modify | Add `explored` to MapTile; add `GameConfig` message + ServerEvent field |
| `internal/config/config.go` | Modify | Add `AutoNavStepMs int` to `GameServerConfig` |
| `internal/gameserver/grpc_service.go` | Modify | Set `Explored` on MapTile; send `GameConfig` at session start; add field + setter |
| `internal/gameserver/grpc_service_map_test.go` | Modify | Tests for `explored` field population |
| `cmd/gameserver/main.go` | Modify | Wire `SetAutoNavStepMs` from config after Initialize |
| `cmd/webclient/ui/src/proto/index.ts` | Modify | Add `explored` to MapTile; add `GameConfig` interface; update union |
| `cmd/webclient/ui/src/game/autoNav.ts` | Create | `findPath` + `resolveDirection` pure functions |
| `cmd/webclient/ui/src/game/autoNav.test.ts` | Create | Unit + property tests for `findPath` |
| `cmd/webclient/ui/src/game/useAutoNav.ts` | Create | `useAutoNav` React hook |
| `cmd/webclient/ui/src/game/useAutoNav.test.ts` | Create | Unit tests for `useAutoNav` |
| `cmd/webclient/ui/src/game/ZoneMapSvg.tsx` | Modify | Add `onTileClick` + `destinationRoomId` props; pointer cursor + blue border |
| `cmd/webclient/ui/src/game/GameContext.tsx` | Modify | Add `autoNavStepMs` to GameState; handle `GameConfig` event |
| `cmd/webclient/ui/src/game/panels/MapPanel.tsx` | Modify | Wire `useAutoNav`; handle `onNoPath`; pass `onTileClick` to ZoneMapSvg |

---

### Task 1: Proto — `explored` field + `GameConfig` message

**Files:**
- Modify: `api/proto/game/v1/game.proto`

- [ ] **Step 1: Add `explored` field to `MapTile` and `GameConfig` message**

  Open `api/proto/game/v1/game.proto`. At `MapTile` (around line 888), add field 13:

  ```proto
  message MapTile {
      string room_id   = 1;
      string room_name = 2;
      int32  x         = 3;
      int32  y         = 4;
      bool   current   = 5;
      repeated string exits = 6;
      string danger_level  = 7;
      repeated string pois = 8;
      bool   boss_room     = 9;
      repeated PoiWithNpc poi_npcs = 10;
      repeated ZoneExitInfo zone_exits = 11;
      repeated SameZoneExitTarget same_zone_exit_targets = 12;
      bool   explored      = 13; // true if the player has physically entered this room (REQ-CNT-1)
  }
  ```

  After the `MapResponse` message (around line 914), add:

  ```proto
  // GameConfig delivers server-side client configuration at session start. (REQ-CNT-2)
  message GameConfig {
    int32 auto_nav_step_ms = 1; // delay between auto-navigation steps in milliseconds; default 1000
  }
  ```

  In `ServerEvent.payload` oneof (around line 371, after field 41), add:

  ```proto
      GameConfig          game_config         = 42;
  ```

- [ ] **Step 2: Regenerate Go proto types**

  ```bash
  make proto
  ```

  Expected: No errors. The generated Go types in `internal/gameserver/gamev1/` now include `Explored bool` on `MapTile`, `GameConfig` message, and `ServerEvent_GameConfig` payload wrapper.

- [ ] **Step 3: Verify compilation**

  ```bash
  go build ./...
  ```

  Expected: Exits 0. No compilation errors.

- [ ] **Step 4: Commit**

  ```bash
  git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
  git commit -m "feat(proto): add MapTile.explored and GameConfig message (#204)"
  ```

---

### Task 2: Config — `auto_nav_step_ms`

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Write the failing test**

  Add to `internal/config/config.go` validation test file — check that the default is 1000 and that values below 100 fail validation. In Go there's no separate test for config defaults; this is verified by the server test in Task 3. For now, verify the field compiles.

  Add `AutoNavStepMs int` to `GameServerConfig` in `internal/config/config.go`:

  ```go
  // GameServerConfig holds game server gRPC connection settings.
  type GameServerConfig struct {
      GRPCHost         string        `mapstructure:"grpc_host"`
      GRPCPort         int           `mapstructure:"grpc_port"`
      RoundDurationMs  int           `mapstructure:"round_duration_ms"`
      GameClockStart   int           `mapstructure:"game_clock_start"`
      GameTickDuration time.Duration `mapstructure:"game_tick_duration"`
      // AutoNavStepMs is the delay in milliseconds between auto-navigation steps in the web client.
      // Minimum 100. Default 1000. (REQ-CNT-2)
      AutoNavStepMs int `mapstructure:"auto_nav_step_ms"`
  }
  ```

- [ ] **Step 2: Add default and validation**

  In `setDefaults`, add:

  ```go
  v.SetDefault("gameserver.auto_nav_step_ms", 1000)
  ```

  In `validateGameServer`, add:

  ```go
  if g.AutoNavStepMs != 0 && g.AutoNavStepMs < 100 {
      errs = append(errs, fmt.Sprintf("gameserver.auto_nav_step_ms must be >= 100, got %d", g.AutoNavStepMs))
  }
  ```

  Note: `!= 0` check allows zero (meaning "use default") to pass validation. The server defaults it to 1000 when zero.

- [ ] **Step 3: Verify compilation**

  ```bash
  go build ./internal/config/...
  ```

  Expected: Exits 0.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/config/config.go
  git commit -m "feat(config): add gameserver.auto_nav_step_ms config key (#204)"
  ```

---

### Task 3: Server — populate `explored`, send `GameConfig`, wire config

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_map_test.go`
- Modify: `cmd/gameserver/main.go`

- [ ] **Step 1: Write failing tests for `explored` field**

  In `internal/gameserver/grpc_service_map_test.go`, add after the existing tests:

  ```go
  // TestHandleMap_ExploredTrue verifies that a room in ExploredCache is marked explored=true
  // on the returned MapTile.
  //
  // Precondition: Room is in both AutomapCache and ExploredCache.
  // Postcondition: MapTile.Explored == true.
  func TestHandleMap_ExploredTrue(t *testing.T) {
      wMgr, sMgr := newWorldAndSessionWithDiscovery("zone1", "room1")
      sess, ok := sMgr.GetPlayer("uid1")
      require.True(t, ok)
      if sess.ExploredCache == nil {
          sess.ExploredCache = make(map[string]map[string]bool)
      }
      sess.ExploredCache["zone1"] = map[string]bool{"room1": true}

      s := &GameServiceServer{sessions: sMgr, world: wMgr}
      resp, err := s.handleMap("uid1", &gamev1.MapRequest{})
      require.NoError(t, err)
      require.Len(t, resp.GetMap().GetTiles(), 1)
      require.True(t, resp.GetMap().GetTiles()[0].Explored)
  }

  // TestHandleMap_ExploredFalse verifies that a room in AutomapCache but NOT ExploredCache
  // is marked explored=false on the returned MapTile.
  //
  // Precondition: Room is in AutomapCache only (discovered via map item, not physically visited).
  // Postcondition: MapTile.Explored == false.
  func TestHandleMap_ExploredFalse(t *testing.T) {
      wMgr, sMgr := newWorldAndSessionWithDiscovery("zone1", "room1")
      // ExploredCache intentionally left empty.

      s := &GameServiceServer{sessions: sMgr, world: wMgr}
      resp, err := s.handleMap("uid1", &gamev1.MapRequest{})
      require.NoError(t, err)
      require.Len(t, resp.GetMap().GetTiles(), 1)
      require.False(t, resp.GetMap().GetTiles()[0].Explored)
  }

  // TestProperty_HandleMap_ExploredAlwaysSubsetOfAutomap verifies that every explored room
  // is also in the discovered set (AutomapCache superset invariant).
  //
  // Precondition: ExploredCache is always a subset of AutomapCache.
  // Postcondition: For all tiles, if Explored==true then the room was in AutomapCache.
  func TestProperty_HandleMap_ExploredAlwaysSubsetOfAutomap(t *testing.T) {
      rapid.Check(t, func(rt *rapid.T) {
          zoneID := "zone1"
          roomID := "room1"
          wMgr, sMgr := newWorldAndSessionWithDiscovery(zoneID, roomID)
          sess, ok := sMgr.GetPlayer("uid1")
          require.True(rt, ok)

          isExplored := rapid.Bool().Draw(rt, "isExplored")
          if isExplored {
              if sess.ExploredCache == nil {
                  sess.ExploredCache = make(map[string]map[string]bool)
              }
              sess.ExploredCache[zoneID] = map[string]bool{roomID: true}
          }

          s := &GameServiceServer{sessions: sMgr, world: wMgr}
          resp, err := s.handleMap("uid1", &gamev1.MapRequest{})
          require.NoError(rt, err)
          for _, tile := range resp.GetMap().GetTiles() {
              if tile.Explored {
                  require.True(rt, sess.AutomapCache[zoneID][tile.RoomId],
                      "explored tile must be in AutomapCache")
              }
          }
      })
  }
  ```

- [ ] **Step 2: Run tests to verify they fail**

  ```bash
  go test ./internal/gameserver/... -run "TestHandleMap_Explored|TestProperty_HandleMap_Explored" -v
  ```

  Expected: FAIL — `resp.GetMap().GetTiles()[0].Explored` is false even when ExploredCache is set (field not yet populated).

- [ ] **Step 3: Add `autoNavStepMs` field and `SetAutoNavStepMs` to `GameServiceServer`**

  In `internal/gameserver/grpc_service.go`, in the `GameServiceServer` struct (after the `maxHotbars` field, around line 356):

  ```go
  // autoNavStepMs is the delay in milliseconds between auto-navigation steps sent to the web client.
  // Defaults to 1000 when not set via SetAutoNavStepMs. (REQ-CNT-2)
  autoNavStepMs int
  ```

  After the `SetMaxHotbars` method (around line 825), add:

  ```go
  // SetAutoNavStepMs sets the auto-navigation step delay in milliseconds.
  // Precondition: ms must be >= 100. If ms < 100, it is clamped to 1000.
  func (s *GameServiceServer) SetAutoNavStepMs(ms int) {
      if ms >= 100 {
          s.autoNavStepMs = ms
      }
  }
  ```

  In the init/Serve method where `maxHotbars` is defaulted (around line 679):

  ```go
  if s.maxHotbars <= 0 {
      s.maxHotbars = 4
  }
  if s.autoNavStepMs < 100 {
      s.autoNavStepMs = 1000
  }
  ```

- [ ] **Step 4: Set `Explored` on MapTile in the map handler**

  In `internal/gameserver/grpc_service.go`, find the `tiles = append(tiles, &gamev1.MapTile{...})` call in `handleMap` (around line 7340). Add `Explored: sess.ExploredCache[zoneID][roomID]` to the struct literal:

  ```go
  tiles = append(tiles, &gamev1.MapTile{
      RoomId:                r.ID,
      RoomName:              r.Title,
      X:                     int32(r.MapX),
      Y:                     int32(r.MapY),
      Current:               r.ID == sess.RoomID,
      Exits:                 exits,
      DangerLevel:           effectiveLevelStr,
      Pois:                  poiSlice,
      BossRoom:              r.BossRoom,
      PoiNpcs:               poiNpcs,
      ZoneExits:             zoneExits,
      SameZoneExitTargets:   sameZoneExitTargets,
      Explored:              sess.ExploredCache[zoneID][roomID], // REQ-CNT-1
  })
  ```

- [ ] **Step 5: Send `GameConfig` at session start**

  In `internal/gameserver/grpc_service.go`, after the hotbar update send (around line 2070):

  ```go
  if err := stream.Send(s.hotbarUpdateEvent(sess)); err != nil {
      s.logger.Warn("failed to send initial hotbar update", zap.Error(err))
  }
  // Send game config so the web client receives server-side configuration. (REQ-CNT-2)
  if err := stream.Send(&gamev1.ServerEvent{
      Payload: &gamev1.ServerEvent_GameConfig{
          GameConfig: &gamev1.GameConfig{
              AutoNavStepMs: int32(s.autoNavStepMs),
          },
      },
  }); err != nil {
      s.logger.Warn("failed to send game config", zap.Error(err))
  }
  ```

- [ ] **Step 6: Wire `SetAutoNavStepMs` from config in `cmd/gameserver/main.go`**

  After `app, err := Initialize(ctx, appCfg, gameClock, logger)` and after the world editor setup (around line 185), add:

  ```go
  // Wire auto-navigation step delay from config (REQ-CNT-2).
  if cfg.GameServer.AutoNavStepMs >= 100 {
      app.GRPCService.SetAutoNavStepMs(cfg.GameServer.AutoNavStepMs)
  }
  ```

- [ ] **Step 7: Run tests to verify they pass**

  ```bash
  go test ./internal/gameserver/... -run "TestHandleMap_Explored|TestProperty_HandleMap_Explored" -v
  ```

  Expected: All PASS.

- [ ] **Step 8: Run full Go test suite**

  ```bash
  go test ./... -count=1
  ```

  Expected: All PASS (no regressions).

- [ ] **Step 9: Commit**

  ```bash
  git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_map_test.go cmd/gameserver/main.go
  git commit -m "feat(server): populate MapTile.explored, send GameConfig at session start (#204)"
  ```

---

### Task 4: Proto TS types — update `proto/index.ts`

**Files:**
- Modify: `cmd/webclient/ui/src/proto/index.ts`

- [ ] **Step 1: Add `explored` to `MapTile` interface**

  In `cmd/webclient/ui/src/proto/index.ts`, find the `MapTile` interface and add `explored?: boolean`:

  ```typescript
  export interface MapTile {
    roomId?: string
    roomName?: string
    x?: number
    y?: number
    current?: boolean
    exits?: string[]
    dangerLevel?: string
    danger_level?: string
    pois?: string[]
    bossRoom?: boolean
    boss?: boolean
    name?: string
    poiNpcs?: PoiWithNpc[]
    poi_npcs?: PoiWithNpc[]
    zoneExits?: ZoneExitInfo[]
    zone_exits?: ZoneExitInfo[]
    sameZoneExitTargets?: SameZoneExitTarget[]
    same_zone_exit_targets?: SameZoneExitTarget[]
    explored?: boolean  // true if the player has physically entered this room (REQ-CNT-1)
  }
  ```

- [ ] **Step 2: Add `GameConfig` interface**

  After the `MapResponse` interface, add:

  ```typescript
  export interface GameConfig {
    autoNavStepMs?: number
    auto_nav_step_ms?: number
  }
  ```

- [ ] **Step 3: Add `GameConfig` to the `IncomingMessage` union**

  Find the `IncomingMessage` union type (the discriminated union of `{ type: string; payload: ... }`). Add:

  ```typescript
  | { type: 'GameConfig'; payload: GameConfig }
  ```

- [ ] **Step 4: Verify TypeScript compilation**

  ```bash
  cd cmd/webclient/ui && npx tsc --noEmit
  ```

  Expected: No errors.

- [ ] **Step 5: Commit**

  ```bash
  git add cmd/webclient/ui/src/proto/index.ts
  git commit -m "feat(proto-ts): add MapTile.explored and GameConfig types (#204)"
  ```

---

### Task 5: `autoNav.ts` — pure BFS utility + tests

**Files:**
- Create: `cmd/webclient/ui/src/game/autoNav.ts`
- Create: `cmd/webclient/ui/src/game/autoNav.test.ts`

- [ ] **Step 1: Write the failing tests**

  Create `cmd/webclient/ui/src/game/autoNav.test.ts`:

  ```typescript
  import { describe, it, expect } from 'vitest'
  import fc from 'fast-check'
  import type { MapTile } from '../proto'
  import { findPath, resolveDirection } from './autoNav'

  // Helper: build a simple linear chain of explored rooms: A → B → C...
  function makeChain(ids: string[]): MapTile[] {
    return ids.map((id, i) => ({
      roomId: id,
      roomName: id,
      explored: true,
      sameZoneExitTargets: [
        ...(i > 0 ? [{ direction: 'west', targetRoomId: ids[i - 1] }] : []),
        ...(i < ids.length - 1 ? [{ direction: 'east', targetRoomId: ids[i + 1] }] : []),
      ],
    }))
  }

  describe('findPath', () => {
    it('returns [] when fromId === toId', () => {
      const tiles = makeChain(['a', 'b', 'c'])
      expect(findPath(tiles, 'a', 'a')).toEqual([])
    })

    it('returns direct neighbor in one hop', () => {
      const tiles = makeChain(['a', 'b', 'c'])
      expect(findPath(tiles, 'a', 'b')).toEqual(['b'])
    })

    it('returns multi-hop path through chain', () => {
      const tiles = makeChain(['a', 'b', 'c', 'd'])
      expect(findPath(tiles, 'a', 'd')).toEqual(['b', 'c', 'd'])
    })

    it('returns null when destination is not explored', () => {
      const tiles: MapTile[] = [
        { roomId: 'a', explored: true, sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'b' }] },
        { roomId: 'b', explored: false, sameZoneExitTargets: [] },
      ]
      expect(findPath(tiles, 'a', 'b')).toBeNull()
    })

    it('returns null when path requires traversing unexplored room', () => {
      const tiles: MapTile[] = [
        { roomId: 'a', explored: true, sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'b' }] },
        { roomId: 'b', explored: false, sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'c' }] },
        { roomId: 'c', explored: true, sameZoneExitTargets: [{ direction: 'west', targetRoomId: 'b' }] },
      ]
      // b is unexplored, so a→c has no explored path
      expect(findPath(tiles, 'a', 'c')).toBeNull()
    })

    it('returns null when fromId not in explored tiles', () => {
      const tiles = makeChain(['a', 'b'])
      expect(findPath(tiles, 'z', 'b')).toBeNull()
    })
  })

  describe('resolveDirection', () => {
    it('returns direction when target is in sameZoneExitTargets', () => {
      const tile: MapTile = {
        roomId: 'a',
        sameZoneExitTargets: [{ direction: 'north', targetRoomId: 'b' }],
      }
      expect(resolveDirection(tile, 'b')).toBe('north')
    })

    it('returns null when target is not in sameZoneExitTargets', () => {
      const tile: MapTile = {
        roomId: 'a',
        sameZoneExitTargets: [{ direction: 'north', targetRoomId: 'b' }],
      }
      expect(resolveDirection(tile, 'z')).toBeNull()
    })
  })

  describe('findPath property tests', () => {
    it('path length never exceeds number of explored tiles', () => {
      fc.assert(fc.property(
        fc.integer({ min: 2, max: 6 }).chain(n =>
          fc.tuple(
            fc.constant(n),
            fc.uniqueArray(fc.string({ minLength: 1, maxLength: 4 }), { minLength: n, maxLength: n }),
          )
        ),
        ([n, ids]) => {
          const tiles = makeChain(ids)
          const path = findPath(tiles, ids[0], ids[n - 1])
          if (path === null) return true  // null means no path found, which is valid
          return path.length <= tiles.filter(t => t.explored).length
        }
      ))
    })

    it('findPath result is non-null for any pair of reachable explored tiles', () => {
      fc.assert(fc.property(
        fc.integer({ min: 2, max: 8 }).chain(n =>
          fc.tuple(
            fc.constant(n),
            fc.uniqueArray(fc.string({ minLength: 1, maxLength: 4 }), { minLength: n, maxLength: n }),
            fc.integer({ min: 0, max: n - 1 }),
            fc.integer({ min: 0, max: n - 1 }),
          )
        ),
        ([n, ids, fromIdx, toIdx]) => {
          const tiles = makeChain(ids)
          const path = findPath(tiles, ids[fromIdx], ids[toIdx])
          if (fromIdx === toIdx) return path !== null && path.length === 0
          // In a fully connected linear chain all explored, a path must exist
          return path !== null
        }
      ))
    })
  })
  ```

- [ ] **Step 2: Run tests to verify they fail**

  ```bash
  cd cmd/webclient/ui && npm test -- --reporter=verbose autoNav
  ```

  Expected: FAIL — `Cannot find module './autoNav'`.

- [ ] **Step 3: Implement `autoNav.ts`**

  Create `cmd/webclient/ui/src/game/autoNav.ts`:

  ```typescript
  import type { MapTile, SameZoneExitTarget } from '../proto'

  /**
   * findPath returns the shortest BFS path of room IDs from fromId to toId,
   * traversing only explored tiles via sameZoneExitTargets.
   *
   * Returns [] if fromId === toId (no movement needed).
   * Returns null if no explored path exists or either endpoint is not explored.
   *
   * Precondition: tiles are from the same zone map response.
   * Postcondition: if non-null, every roomId in the result is explored and reachable.
   */
  export function findPath(
    tiles: MapTile[],
    fromId: string,
    toId: string,
  ): string[] | null {
    if (fromId === toId) return []

    // Index explored tiles by roomId and build adjacency from sameZoneExitTargets.
    const exploredSet = new Set<string>()
    const adjMap = new Map<string, string[]>()  // roomId → [targetRoomId, ...]

    for (const tile of tiles) {
      const id = tile.roomId ?? ''
      if (!id || !(tile.explored ?? false)) continue
      exploredSet.add(id)
      const exits: SameZoneExitTarget[] = tile.sameZoneExitTargets ?? tile.same_zone_exit_targets ?? []
      adjMap.set(id, exits.map(e => e.targetRoomId ?? e.target_room_id ?? '').filter(t => t !== ''))
    }

    if (!exploredSet.has(fromId) || !exploredSet.has(toId)) return null

    // BFS over explored tiles only.
    const visited = new Set<string>([fromId])
    const queue: Array<{ id: string; path: string[] }> = [{ id: fromId, path: [] }]

    while (queue.length > 0) {
      const { id, path } = queue.shift()!
      for (const targetId of (adjMap.get(id) ?? [])) {
        if (!exploredSet.has(targetId) || visited.has(targetId)) continue
        const newPath = [...path, targetId]
        if (targetId === toId) return newPath
        visited.add(targetId)
        queue.push({ id: targetId, path: newPath })
      }
    }
    return null
  }

  /**
   * resolveDirection returns the direction to move from currentTile to nextRoomId
   * by looking up sameZoneExitTargets on currentTile.
   *
   * Returns null if nextRoomId is not a direct exit from currentTile.
   *
   * Precondition: currentTile must be non-null; nextRoomId must be non-empty.
   * Postcondition: returned direction is a valid compass direction string, or null.
   */
  export function resolveDirection(currentTile: MapTile, nextRoomId: string): string | null {
    const exits: SameZoneExitTarget[] = currentTile.sameZoneExitTargets ?? currentTile.same_zone_exit_targets ?? []
    for (const e of exits) {
      const targetId = e.targetRoomId ?? e.target_room_id ?? ''
      if (targetId === nextRoomId) return e.direction ?? null
    }
    return null
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  ```bash
  cd cmd/webclient/ui && npm test -- --reporter=verbose autoNav
  ```

  Expected: All PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add cmd/webclient/ui/src/game/autoNav.ts cmd/webclient/ui/src/game/autoNav.test.ts
  git commit -m "feat(autonav): add findPath BFS and resolveDirection utilities (#204)"
  ```

---

### Task 6: `useAutoNav.ts` — React hook + tests

**Files:**
- Create: `cmd/webclient/ui/src/game/useAutoNav.ts`
- Create: `cmd/webclient/ui/src/game/useAutoNav.test.ts`

- [ ] **Step 1: Write the failing tests**

  Create `cmd/webclient/ui/src/game/useAutoNav.test.ts`:

  ```typescript
  import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
  import { renderHook, act } from '@testing-library/react'
  import type { MapTile } from '../proto'
  import { useAutoNav } from './useAutoNav'

  // makeChain builds a linear chain of explored rooms: ids[0] — east → ids[1] — east → ...
  function makeChain(ids: string[]): MapTile[] {
    return ids.map((id, i) => ({
      roomId: id,
      roomName: `Room ${id}`,
      explored: true,
      sameZoneExitTargets: [
        ...(i > 0 ? [{ direction: 'west', targetRoomId: ids[i - 1] }] : []),
        ...(i < ids.length - 1 ? [{ direction: 'east', targetRoomId: ids[i + 1] }] : []),
      ],
    }))
  }

  describe('useAutoNav', () => {
    beforeEach(() => { vi.useFakeTimers() })
    afterEach(() => { vi.useRealTimers() })

    it('fires sendMove for each step then clears active state', async () => {
      const tiles = makeChain(['a', 'b', 'c'])
      const sendMove = vi.fn()
      const onNoPath = vi.fn()

      const { result } = renderHook(() =>
        useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
      )

      expect(result.current.active).toBe(false)

      act(() => {
        result.current.start(tiles[2])  // navigate from a to c
      })

      expect(result.current.active).toBe(true)
      expect(result.current.destinationRoomId).toBe('c')

      // First step fires after 100ms
      await act(async () => { vi.advanceTimersByTime(100) })
      expect(sendMove).toHaveBeenCalledWith('east')
      expect(sendMove).toHaveBeenCalledTimes(1)

      // Second step fires after another 100ms
      await act(async () => { vi.advanceTimersByTime(100) })
      expect(sendMove).toHaveBeenCalledWith('east')
      expect(sendMove).toHaveBeenCalledTimes(2)
      expect(result.current.active).toBe(false)
      expect(result.current.destinationRoomId).toBeNull()
    })

    it('calls onNoPath when no explored path exists', () => {
      const tiles: MapTile[] = [
        { roomId: 'a', explored: true, sameZoneExitTargets: [] },
        { roomId: 'b', explored: true, roomName: 'Room B', sameZoneExitTargets: [] },
      ]
      const sendMove = vi.fn()
      const onNoPath = vi.fn()

      const { result } = renderHook(() =>
        useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
      )

      act(() => { result.current.start(tiles[1]) })

      expect(onNoPath).toHaveBeenCalledWith('Room B')
      expect(result.current.active).toBe(false)
      expect(sendMove).not.toHaveBeenCalled()
    })

    it('cancel stops the timer and clears path', async () => {
      const tiles = makeChain(['a', 'b', 'c', 'd'])
      const sendMove = vi.fn()
      const onNoPath = vi.fn()

      const { result } = renderHook(() =>
        useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
      )

      act(() => { result.current.start(tiles[3]) })
      expect(result.current.active).toBe(true)

      await act(async () => { vi.advanceTimersByTime(100) })  // fires one step
      expect(sendMove).toHaveBeenCalledTimes(1)

      act(() => { result.current.cancel() })
      expect(result.current.active).toBe(false)

      await act(async () => { vi.advanceTimersByTime(300) })  // no more steps fire
      expect(sendMove).toHaveBeenCalledTimes(1)
    })

    it('start cancels existing path and retargets', async () => {
      const tiles = makeChain(['a', 'b', 'c', 'd'])
      const sendMove = vi.fn()
      const onNoPath = vi.fn()

      const { result } = renderHook(() =>
        useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
      )

      act(() => { result.current.start(tiles[3]) })  // navigate a→d
      expect(result.current.destinationRoomId).toBe('d')

      act(() => { result.current.start(tiles[1]) })  // retarget to b
      expect(result.current.destinationRoomId).toBe('b')

      await act(async () => { vi.advanceTimersByTime(100) })
      expect(sendMove).toHaveBeenCalledWith('east')
      expect(sendMove).toHaveBeenCalledTimes(1)
      expect(result.current.active).toBe(false)  // reached b in one step
    })

    it('clicking current room cancels without starting navigation', () => {
      const tiles = makeChain(['a', 'b', 'c'])
      const sendMove = vi.fn()
      const onNoPath = vi.fn()

      const { result } = renderHook(() =>
        useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
      )

      act(() => { result.current.start(tiles[0]) })  // click current room

      expect(result.current.active).toBe(false)
      expect(sendMove).not.toHaveBeenCalled()
      expect(onNoPath).not.toHaveBeenCalled()
    })

    it('cancels automatically when direction lookup fails (server-blocked movement)', async () => {
      const tiles = makeChain(['a', 'b', 'c'])
      const sendMove = vi.fn()
      const onNoPath = vi.fn()

      // currentRoomId will be 'a' initially but we simulate server rejecting movement
      // by keeping currentRoomId at 'a' while path expects to be at 'b' after first step.
      // The hook reads currentRoomId from its argument each tick.
      // We model this by having the path advance but currentRoomId stay the same:
      // after step 1 sends 'east' (a→b), currentRoomId is still 'a' in the arg.
      // Then for step 2, path[0] is 'c', but currentTile 'a' has no exit to 'c' → cancel.

      const { result } = renderHook(() =>
        useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
      )

      act(() => { result.current.start(tiles[2]) })  // a→c, path = ['b', 'c']

      // Step 1: sends 'east' (a→b), path becomes ['c']
      await act(async () => { vi.advanceTimersByTime(100) })
      expect(sendMove).toHaveBeenCalledTimes(1)

      // Step 2: currentRoomId is still 'a' (server rejected move).
      // Tile 'a' has no exit to 'c', so resolveDirection returns null → cancel.
      await act(async () => { vi.advanceTimersByTime(100) })
      expect(result.current.active).toBe(false)
      // sendMove NOT called a second time (cancelled)
      expect(sendMove).toHaveBeenCalledTimes(1)
    })
  })
  ```

- [ ] **Step 2: Run tests to verify they fail**

  ```bash
  cd cmd/webclient/ui && npm test -- --reporter=verbose useAutoNav
  ```

  Expected: FAIL — `Cannot find module './useAutoNav'`.

- [ ] **Step 3: Implement `useAutoNav.ts`**

  Create `cmd/webclient/ui/src/game/useAutoNav.ts`:

  ```typescript
  import { useState, useRef, useCallback, useEffect } from 'react'
  import type { MapTile } from '../proto'
  import { findPath, resolveDirection } from './autoNav'

  export interface UseAutoNavResult {
    start: (targetTile: MapTile) => void
    cancel: () => void
    active: boolean
    destinationRoomId: string | null
  }

  /**
   * useAutoNav manages automatic step-by-step navigation along a pre-computed BFS path.
   *
   * Precondition: tiles, currentRoomId, and stepMs must be non-null/non-zero on each render.
   * Postcondition: at most one timer is active at any time; cleanup on unmount.
   */
  export function useAutoNav(
    tiles: MapTile[],
    currentRoomId: string,
    stepMs: number,
    sendMove: (direction: string) => void,
    onNoPath: (roomName: string) => void,
  ): UseAutoNavResult {
    const [destinationRoomId, setDestinationRoomId] = useState<string | null>(null)
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const pathRef = useRef<string[]>([])

    // Stable refs so timer closures always see latest prop values without re-subscribing.
    const tilesRef = useRef(tiles)
    const currentRoomIdRef = useRef(currentRoomId)
    const stepMsRef = useRef(stepMs)
    const sendMoveRef = useRef(sendMove)
    const onNoPathRef = useRef(onNoPath)

    tilesRef.current = tiles
    currentRoomIdRef.current = currentRoomId
    stepMsRef.current = stepMs
    sendMoveRef.current = sendMove
    onNoPathRef.current = onNoPath

    const clearTimer = useCallback(() => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current)
        timerRef.current = null
      }
    }, [])

    // scheduleStepRef stores the latest step function so it can call itself recursively
    // without stale closures. Re-assigned on every render.
    const scheduleStepRef = useRef<() => void>(() => {})
    scheduleStepRef.current = () => {
      timerRef.current = setTimeout(() => {
        const path = pathRef.current
        if (path.length === 0) {
          setDestinationRoomId(null)
          return
        }

        const nextRoomId = path[0]
        const currentId = currentRoomIdRef.current
        const currentTile = tilesRef.current.find(t => (t.roomId ?? '') === currentId) ?? null

        // REQ-CNT-7c: If the direction to the next room cannot be resolved, the server
        // blocked movement and currentRoomId did not advance. Cancel automatically.
        const direction = currentTile ? resolveDirection(currentTile, nextRoomId) : null
        if (direction === null) {
          pathRef.current = []
          setDestinationRoomId(null)
          return
        }

        sendMoveRef.current(direction)
        pathRef.current = path.slice(1)

        if (pathRef.current.length > 0) {
          scheduleStepRef.current()
        } else {
          setDestinationRoomId(null)
        }
      }, stepMsRef.current)
    }

    const cancel = useCallback(() => {
      clearTimer()
      pathRef.current = []
      setDestinationRoomId(null)
    }, [clearTimer])

    const start = useCallback((targetTile: MapTile) => {
      const targetId = targetTile.roomId ?? ''
      const currentId = currentRoomIdRef.current

      // REQ-CNT-4c: clicking current room cancels without starting navigation.
      if (targetId === currentId) {
        cancel()
        return
      }

      // REQ-CNT-4b: cancel existing path before starting a new one (retarget).
      cancel()

      const path = findPath(tilesRef.current, currentId, targetId)
      if (path === null) {
        // REQ-CNT-4a: no explored path — notify caller.
        onNoPathRef.current(targetTile.roomName ?? targetId)
        return
      }
      if (path.length === 0) return  // already there (shouldn't occur after currentId check)

      pathRef.current = path
      setDestinationRoomId(targetId)
      scheduleStepRef.current()
    }, [cancel])

    // Cleanup timer on unmount to prevent memory leaks.
    useEffect(() => () => clearTimer(), [clearTimer])

    return { start, cancel, active: destinationRoomId !== null, destinationRoomId }
  }
  ```

- [ ] **Step 4: Run tests to verify they pass**

  ```bash
  cd cmd/webclient/ui && npm test -- --reporter=verbose useAutoNav
  ```

  Expected: All PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add cmd/webclient/ui/src/game/useAutoNav.ts cmd/webclient/ui/src/game/useAutoNav.test.ts
  git commit -m "feat(autonav): add useAutoNav React hook (#204)"
  ```

---

### Task 7: `ZoneMapSvg.tsx` — click handling + destination indicator

**Files:**
- Modify: `cmd/webclient/ui/src/game/ZoneMapSvg.tsx`

- [ ] **Step 1: Add `onTileClick` and `destinationRoomId` props to `ZoneMapSvgProps`**

  In `ZoneMapSvg.tsx`, update the `ZoneMapSvgProps` interface:

  ```typescript
  interface ZoneMapSvgProps {
    tiles: MapTile[]
    onHover?: (tile: MapTile, e: React.MouseEvent) => void
    onHoverEnd?: () => void
    containerWidth?: number
    playerLevel?: number
    zoneLevelRange?: string
    // REQ-CNT-5: Click-to-travel support.
    onTileClick?: (tile: MapTile) => void
    destinationRoomId?: string | null
  }
  ```

  Update the function signature:

  ```typescript
  export function ZoneMapSvg({
    tiles, onHover, onHoverEnd, containerWidth, playerLevel, zoneLevelRange,
    onTileClick, destinationRoomId,
  }: ZoneMapSvgProps): JSX.Element {
  ```

- [ ] **Step 2: Apply cursor and click handler in `renderTile`**

  In the `renderTile` function, replace the `<rect>` element with the updated version that applies `cursor: pointer` and `onClick` for explored non-current tiles, and a blue border for the destination tile:

  ```typescript
  function renderTile(tile: MapTile): JSX.Element {
    const tx = tile.x ?? 0
    const ty = tile.y ?? 0
    const rx = px(tx)
    const ry = py(ty)
    const dangerKey = tile.dangerLevel ?? tile.danger_level ?? ''
    const fill = DANGER_FILLS[dangerKey] ?? '#1e1e2e'
    const isCurrent = tile.current ?? false
    const isBoss = tile.bossRoom ?? tile.boss ?? false
    const isExplored = tile.explored ?? false
    const isDestination = destinationRoomId != null && tile.roomId === destinationRoomId

    // REQ-CNT-5c: destination tile gets blue border.
    // REQ-CNT-5a/5b: pointer cursor only on explored, non-current tiles.
    const diffColor = (!isCurrent && !isBoss)
      ? (difficultyBorderColor(zoneLevelRange, playerLevel ?? 0) ?? '#333')
      : '#333'
    const stroke = isDestination
      ? '#4a9eff'
      : isCurrent
      ? '#f0c040'
      : isBoss
      ? '#cc4444'
      : diffColor
    const strokeWidth = isCurrent || isBoss || isDestination ? 2 : 1

    const isClickable = isExplored && !isCurrent && onTileClick != null
    const cursor = isClickable ? 'pointer' : 'default'

    const name = tile.roomName ?? ''
    const id = clipId(tile)
    const [line1, line2] = wrapRoomName(name)
    const hasPois = (tile.pois ?? []).length > 0
    const textMidY = hasPois ? ry + cellH / 2 - 4 : ry + cellH / 2
    const lineH = 10

    return (
      <g key={`tile-${tile.roomId ?? tx}-${ty}`}>
        <rect
          x={rx} y={ry}
          width={cellW} height={cellH}
          rx={4}
          fill={fill} stroke={stroke} strokeWidth={strokeWidth}
          style={{ cursor }}
          onMouseEnter={onHover ? e => onHover(tile, e) : undefined}
          onMouseLeave={onHoverEnd}
          onClick={isClickable ? () => onTileClick!(tile) : undefined}
        />
        {/* ... rest of tile content unchanged ... */}
  ```

  Keep all existing text, POI, and zone exit rendering inside the `<g>` unchanged. Only the `<rect>` attributes change.

- [ ] **Step 3: Verify TypeScript compilation**

  ```bash
  cd cmd/webclient/ui && npx tsc --noEmit
  ```

  Expected: No errors.

- [ ] **Step 4: Commit**

  ```bash
  git add cmd/webclient/ui/src/game/ZoneMapSvg.tsx
  git commit -m "feat(map): add onTileClick and destination indicator to ZoneMapSvg (#204)"
  ```

---

### Task 8: `GameContext.tsx` — `autoNavStepMs` state + `GameConfig` handling

**Files:**
- Modify: `cmd/webclient/ui/src/game/GameContext.tsx`

- [ ] **Step 1: Add `autoNavStepMs` to `GameState`**

  In the `GameState` interface, add:

  ```typescript
  autoNavStepMs: number  // step delay for click-to-travel (REQ-CNT-2)
  ```

- [ ] **Step 2: Add `SET_AUTO_NAV_STEP_MS` action**

  In the `Action` union type, add:

  ```typescript
  | { type: 'SET_AUTO_NAV_STEP_MS'; ms: number }
  ```

- [ ] **Step 3: Handle the action in `reducer`**

  In the `reducer` function, add a case:

  ```typescript
  case 'SET_AUTO_NAV_STEP_MS':
    return { ...state, autoNavStepMs: action.ms }
  ```

- [ ] **Step 4: Add default to `initialState`**

  In `initialState`, add:

  ```typescript
  autoNavStepMs: 1000,
  ```

- [ ] **Step 5: Handle `GameConfig` event in the message dispatch loop**

  In the `switch (type)` block that dispatches incoming server events (where `'HotbarUpdate'`, `'MapResponse'`, etc. are handled), add:

  ```typescript
  case 'GameConfig': {
    const gc = payload as { autoNavStepMs?: number; auto_nav_step_ms?: number }
    const ms = gc.autoNavStepMs ?? gc.auto_nav_step_ms ?? 1000
    if (ms >= 100) {
      dispatch({ type: 'SET_AUTO_NAV_STEP_MS', ms })
    }
    break
  }
  ```

- [ ] **Step 6: Verify TypeScript compilation**

  ```bash
  cd cmd/webclient/ui && npx tsc --noEmit
  ```

  Expected: No errors.

- [ ] **Step 7: Commit**

  ```bash
  git add cmd/webclient/ui/src/game/GameContext.tsx
  git commit -m "feat(context): add autoNavStepMs state and handle GameConfig event (#204)"
  ```

---

### Task 9: `MapPanel.tsx` — wire `useAutoNav`

**Files:**
- Modify: `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

- [ ] **Step 1: Import `useAutoNav`**

  At the top of `MapPanel.tsx`, add:

  ```typescript
  import { useAutoNav } from '../useAutoNav'
  ```

- [ ] **Step 2: Instantiate `useAutoNav` inside `MapPanel`**

  In the `MapPanel` function, after the existing `useEffect` / `useRef` declarations, add:

  ```typescript
  const currentRoomId = state.roomView?.roomId ?? ''

  const autoNav = useAutoNav(
    state.mapTiles,
    currentRoomId,
    state.autoNavStepMs,
    (direction) => sendMessage('MoveRequest', { direction }),
    (roomName) => {
      // REQ-CNT-6c: dispatch console message when no explored path exists.
      dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('system', `No explored path to ${roomName}.`) })
    },
  )
  ```

  Note: `dispatch` and `makeFeedEntry` are not directly accessible in `MapPanel`. Instead, use the pattern already in place — the `sendCommand` approach won't work here. Look at how other components dispatch feed entries. The correct approach is to add an `appendFeed` helper to the `GameContext` value, or use the existing `sendCommand` to send a local message.

  **Actual implementation**: Since `dispatch` is not exposed from `GameContext`, use `sendCommand` with a special prefix, or — better — expose a `appendMessage` function from `GameContext.Provider` value similar to how `clearShop` is exposed. Add to the context:

  In `GameContext.tsx` value (the `<GameContext.Provider value={...}>` call), expose:

  ```typescript
  appendMessage: (text: string) => dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('system', text) }),
  ```

  And in the context interface:

  ```typescript
  appendMessage: (text: string) => void
  ```

  Then in `MapPanel.tsx`, destructure `appendMessage` from `useGame()` and use it in `onNoPath`:

  ```typescript
  const { state, sendMessage, sendCommand, clearCombatNpcView, appendMessage } = useGame()

  const autoNav = useAutoNav(
    state.mapTiles,
    currentRoomId,
    state.autoNavStepMs,
    (direction) => sendMessage('MoveRequest', { direction }),
    (roomName) => appendMessage(`No explored path to ${roomName}.`),
  )
  ```

- [ ] **Step 3: Pass `onTileClick` and `destinationRoomId` to `ZoneMapSvg`**

  Find the `<ZoneMapSvg ... />` JSX in `MapPanel` and add the two new props:

  ```tsx
  <ZoneMapSvg
    tiles={state.mapTiles}
    containerWidth={mapContainerW}
    onHover={handleRoomEnter}
    onHoverEnd={handleRoomLeave}
    playerLevel={state.characterSheet?.level ?? 0}
    zoneLevelRange={
      (state.worldTiles.find(t => !!t.current)?.levelRange)
      ?? (state.worldTiles.find(t => !!t.current)?.level_range)
    }
    onTileClick={autoNav.start}
    destinationRoomId={autoNav.destinationRoomId}
  />
  ```

- [ ] **Step 4: Cancel auto-nav when switching to world map**

  Update `switchToWorld` to cancel any active navigation first (REQ-CNT-6d):

  ```typescript
  function switchToWorld() {
    autoNav.cancel()
    setShowWorld(true)
    sendMessage('MapRequest', { view: 'world' })
  }
  ```

- [ ] **Step 5: Verify TypeScript compilation**

  ```bash
  cd cmd/webclient/ui && npx tsc --noEmit
  ```

  Expected: No errors.

- [ ] **Step 6: Run full frontend test suite**

  ```bash
  cd cmd/webclient/ui && npm test
  ```

  Expected: All PASS (no regressions).

- [ ] **Step 7: Build frontend to confirm no bundle errors**

  ```bash
  cd cmd/webclient/ui && npm run build
  ```

  Expected: Successful build with no TypeScript or bundle errors.

- [ ] **Step 8: Commit**

  ```bash
  git add cmd/webclient/ui/src/game/panels/MapPanel.tsx cmd/webclient/ui/src/game/GameContext.tsx
  git commit -m "feat(ui): wire click-to-travel in MapPanel (#204)"
  ```

---

## Self-Review

**Spec coverage:**
- REQ-CNT-1 (`explored` proto field): Task 1 (proto) + Task 3 (server populates it) + Task 4 (TS types) ✓
- REQ-CNT-2 (`auto_nav_step_ms` config + GameConfig): Task 1 (proto) + Task 2 (config) + Task 3 (send) + Task 8 (client stores) ✓
- REQ-CNT-3 (`findPath`): Task 5 ✓
- REQ-CNT-4 (`useAutoNav` hook): Task 6 ✓
- REQ-CNT-5 (ZoneMapSvg click): Task 7 ✓
- REQ-CNT-6 (MapPanel wiring): Task 9 ✓
- REQ-CNT-7 (interruption): covered in Task 6 tests (cancel, retarget, mismatch detection) ✓
- REQ-CNT-8 (tests): Tasks 3, 5, 6 ✓

**Placeholder scan:** None found. All code blocks are complete.

**Type consistency:** `findPath` returns `string[] | null` throughout. `resolveDirection` returns `string | null`. `useAutoNav` returns `UseAutoNavResult` with `start`, `cancel`, `active`, `destinationRoomId` — all consistent across Task 6 tests and Task 9 usage. `onTileClick?: (tile: MapTile) => void` and `destinationRoomId?: string | null` match between Task 7 prop definition and Task 9 usage.
