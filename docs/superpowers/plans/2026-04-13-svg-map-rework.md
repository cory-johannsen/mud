# Plan: Map Rework — SVG-Based Zone, World, and Combat Maps

**GitHub Issue:** cory-johannsen/mud#51
**Spec:** `docs/superpowers/specs/2026-04-13-svg-map-rework.md`
**Date:** 2026-04-13

---

## Phase A: Combat Map

### Step A1 — Proto: add grid dimensions to RoundStartEvent (REQ-PA-1a)

**File:** `api/proto/game/v1/game.proto`

Add to `RoundStartEvent`:
```proto
int32 grid_width  = 6;
int32 grid_height = 7;
```

Regenerate:
```bash
cd api/proto/game/v1 && mise exec -- protoc --go_out=. --go_opt=paths=source_relative game.proto
```

---

### Step A2 — Server: populate grid dimensions in RoundStartEvent (REQ-PA-1b)

**File:** `internal/gameserver/combat_handler.go`

Find the `RoundStartEvent` construction (the broadcast that already sets `InitialPositions`). Add:
```go
GridWidth:  int32(combat.GridWidth),
GridHeight: int32(combat.GridHeight),
```

**TDD:** In `combat_handler_test.go`, assert that a 20×20 combat produces a `RoundStartEvent` with `GridWidth = 20` and `GridHeight = 20`.

---

### Step A3 — TypeScript: add grid fields to proto interface (REQ-PA-1c)

**File:** `cmd/webclient/ui/src/proto/index.ts`

Add to `RoundStartEvent` interface:
```typescript
gridWidth?: number
grid_width?: number
gridHeight?: number
grid_height?: number
```

---

### Step A4 — GameContext: store grid dimensions and AP in state (REQ-PA-1d, REQ-PA-4)

**File:** `cmd/webclient/ui/src/game/GameContext.tsx`

Add to state type:
```typescript
combatGridWidth: number
combatGridHeight: number
combatApRemaining: number
combatApTotal: number
```

Defaults: `combatGridWidth: 20, combatGridHeight: 20, combatApRemaining: 0, combatApTotal: 3`.

In `ROUND_START` reducer:
```typescript
combatGridWidth: action.payload.gridWidth ?? action.payload.grid_width ?? 20,
combatGridHeight: action.payload.gridHeight ?? action.payload.grid_height ?? 20,
// find current player's entry in initial_positions
combatApRemaining: playerPos?.apRemaining ?? playerPos?.ap_remaining ?? 3,
combatApTotal: playerPos?.apTotal ?? playerPos?.ap_total ?? 3,
```

In `UPDATE_COMBAT_POSITION` (when updated combatant matches current player):
```typescript
combatApRemaining: action.payload.apRemaining ?? action.payload.ap_remaining ?? state.combatApRemaining,
combatApTotal: action.payload.apTotal ?? action.payload.ap_total ?? state.combatApTotal,
```

---

### Step A5 — MapPanel: dynamic grid + compass dpad + AP pips (REQ-PA-2, REQ-PA-3, REQ-PA-4, REQ-PA-5)

**File:** `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

#### A5a — Update `renderBattleGrid` signature and remove hardcoded size

Change signature to accept `gridWidth: number, gridHeight: number`. Remove `const GRID_SIZE = 20` and `const CELL_PX = 16`. Compute dynamically:
```typescript
const rawCell = Math.floor(320 / Math.max(gridWidth, gridHeight))
const CELL_PX = Math.max(12, Math.min(32, rawCell))
```
Use `gridWidth`/`gridHeight` in loops instead of `GRID_SIZE`.

#### A5b — `DPad` component

```typescript
const COMPASS_DIRS = [
  ['nw','n','ne'],
  ['w', '', 'e'],
  ['sw','s','se'],
] as const

function DPad({ onDir, disabledDirs, disabled }: {
  onDir: (dir: string) => void
  disabledDirs: Set<string>
  disabled: boolean
}): JSX.Element {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 28px)', gap: '2px' }}>
      {COMPASS_DIRS.flat().map((dir, i) => (
        dir === '' ? (
          <button key={i} disabled={disabled}
            style={{ width: 28, height: 28, background: '#1a3a6b', border: '1px solid #3a5a9b', borderRadius: 3, color: '#7bb8ff', fontSize: '0.75rem', cursor: disabled ? 'not-allowed' : 'pointer' }}
            onClick={() => onDir('toward')} title="Stride toward nearest enemy">⊕</button>
        ) : (
          <button key={i} disabled={disabled || disabledDirs.has(dir)}
            style={{ width: 28, height: 28, background: disabled || disabledDirs.has(dir) ? '#111' : '#1a2a1a', border: '1px solid #333', borderRadius: 3, color: disabled || disabledDirs.has(dir) ? '#444' : '#8d4', fontSize: '0.7rem', cursor: disabled || disabledDirs.has(dir) ? 'not-allowed' : 'pointer' }}
            onClick={() => onDir(dir)} title={dir.toUpperCase()}>{dir.toUpperCase()}</button>
        )
      ))}
    </div>
  )
}
```

#### A5c — `ApPips` component

```typescript
function ApPips({ remaining, total }: { remaining: number; total: number }): JSX.Element {
  return (
    <div style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
      {Array.from({ length: total }).map((_, i) => (
        <div key={i} style={{ width: 10, height: 10, borderRadius: '50%', background: i < remaining ? '#f0c040' : 'transparent', border: '2px solid #f0c040' }} />
      ))}
    </div>
  )
}
```

#### A5d — Compute disabled directions from player position and grid bounds

```typescript
const gridWidth = state.combatGridWidth
const gridHeight = state.combatGridHeight
const apRemaining = state.combatApRemaining
const apTotal = state.combatApTotal
const apDisabled = apRemaining === 0
const playerPos = state.combatPositions[state.characterInfo?.name ?? '']

const disabledDirs = new Set<string>()
if (playerPos) {
  if (playerPos.y === 0)              { disabledDirs.add('n'); disabledDirs.add('nw'); disabledDirs.add('ne') }
  if (playerPos.y === gridHeight - 1) { disabledDirs.add('s'); disabledDirs.add('sw'); disabledDirs.add('se') }
  if (playerPos.x === 0)              { disabledDirs.add('w'); disabledDirs.add('nw'); disabledDirs.add('sw') }
  if (playerPos.x === gridWidth - 1)  { disabledDirs.add('e'); disabledDirs.add('ne'); disabledDirs.add('se') }
}
```

#### A5e — Step mode toggle + replace inCombat JSX

Add `const [stepMode, setStepMode] = useState(false)` to `MapPanel`.

Replace the `inCombat` return block:
```tsx
return (
  <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
    <div className="map-header">
      <h3>Battle Map</h3>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
        <ApPips remaining={apRemaining} total={apTotal} />
        <label style={{ fontSize: '0.75rem', color: '#aaa', display: 'flex', alignItems: 'center', gap: '4px' }}>
          <input type="checkbox" checked={stepMode} onChange={e => setStepMode(e.target.checked)} /> Step
        </label>
        <button className="map-refresh-btn" style={{ background: '#2a1a1a', borderColor: '#7a2a2a', color: '#f66' }} onClick={() => sendCommand('flee')}>Flee!</button>
      </div>
    </div>
    <div style={{ display: 'flex', gap: '0.75rem', padding: '0.5rem', overflow: 'auto', flex: 1 }}>
      <div style={{ overflow: 'auto', flexShrink: 0 }}>
        {renderBattleGrid(state.combatPositions, state.characterInfo?.name ?? '', gridWidth, gridHeight)}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', justifyContent: 'center' }}>
        <DPad onDir={dir => sendCommand(`${stepMode ? 'step' : 'stride'} ${dir}`)} disabledDirs={disabledDirs} disabled={apDisabled} />
      </div>
    </div>
  </div>
)
```

---

### Step A6 — Run Phase A tests and build

```bash
mise exec -- go test ./internal/gameserver/... -count=1
cd cmd/webclient/ui && npm run build
```

---

## Phase B: SVG Zone & World Maps

### Step B1 — TDD: write failing render tests for ZoneMapSvg

**File:** `cmd/webclient/ui/src/game/ZoneMapSvg.test.tsx` (new)

Using `@testing-library/react`:
- Render with 3 tiles: one `current`, one `bossRoom`, one with `pois: ['merchant']`
- Assert `<rect>` count equals tile count
- Assert current tile `rect` has `stroke="#f0c040"`
- Assert boss tile `rect` has `stroke="#cc4444"`
- Assert a `<text>` element contains `$` (merchant POI)
- Assert at least one `<line>` exists (connector between adjacent tiles)

Run — MUST fail before implementation.

---

### Step B2 — Implement `ZoneMapSvg` (REQ-PB-1)

**File:** `cmd/webclient/ui/src/game/ZoneMapSvg.tsx` (new)

```typescript
const CELL_W = 56
const CELL_H = 36
const DANGER_FILLS: Record<string, string> = {
  safe: '#2a4a2a', sketchy: '#3a3a1a', dangerous: '#4a2a1a', deadly: '#4a1a1a',
}
const POI_SYMBOLS: Record<string, string> = {
  merchant: '$', healer: '+', trainer: 'T', quest_giver: '!', motel: 'Z', npc: '@', guard: 'G',
}
const DIR_OFFSETS: Record<string, [number, number]> = {
  n:[0,-1], s:[0,1], e:[1,0], w:[-1,0], ne:[1,-1], nw:[-1,-1], se:[1,1], sw:[-1,1],
}
```

Implementation:
1. Build `Map<string, MapTile>` keyed by `"${x},${y}"`
2. Compute SVG `viewBox`: `minX * CELL_W - pad, minY * CELL_H - pad, (maxX-minX+1)*CELL_W + 2*pad, (maxY-minY+1)*CELL_H + 2*pad`
3. Render connectors (`<line>`) before tiles. For each tile, for each exit direction: look up adjacent tile; if found and not already rendered, draw line. Zone exits → `strokeDasharray="4 2"` blue. Normal → `#555`. Track rendered pairs by sorted key `"x1,y1-x2,y2"`
4. Render `<rect>` per tile: `rx="4"`, fill from `DANGER_FILLS` (default `#1e1e2e`), stroke `#f0c040` (current) or `#cc4444` (boss) or `#333`
5. Render `<text>` room name: `x=cx, y=cy+2`, `textAnchor="middle"`, `dominantBaseline="middle"`, `fontSize="9"`, clip via SVG `<clipPath>`
6. Render POI symbols: `<text>` at `(x*CELL_W + CELL_W - 4, y*CELL_H + 10)`, `textAnchor="end"`, `fontSize="10"`, gold color
7. `onMouseEnter` → call `onHover(tile, e)`, `onMouseLeave` → `onHoverEnd()`

Props: `{ tiles: MapTile[], onHover: (tile: MapTile, e: React.MouseEvent) => void, onHoverEnd: () => void }`

---

### Step B3 — Implement `WorldMapSvg` (REQ-PB-2)

**File:** `cmd/webclient/ui/src/game/WorldMapSvg.tsx` (new)

```typescript
const ZONE_W = 80
const ZONE_H = 50
```

Props: `{ tiles: WorldZoneTile[], onTravel: (zoneId: string) => void }`

Implementation:
1. Render `<rect>` per tile at `(worldX * ZONE_W, worldY * ZONE_H)`. Discovered: fill from `DANGER_FILLS`. Undiscovered: `#111`
2. Discovered `<text>` zone name centered in tile, truncated
3. Current tile: `#f0c040` stroke. Non-current discovered: `onClick → onTravel`, `cursor: pointer`
4. Legend row as HTML below the `<svg>`

---

### Step B4 — Wire into `MapPanel.tsx` (REQ-PB-1a, REQ-PB-2a, REQ-PB-3)

**File:** `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

1. Import `ZoneMapSvg`, `WorldMapSvg`; remove `renderMapTiles`, `ColoredLine`, `renderLines` imports
2. Replace `<pre className="map-ascii">` block with:
   ```tsx
   <ZoneMapSvg tiles={state.mapTiles} onHover={handleRoomEnter} onHoverEnd={handleRoomLeave} />
   ```
3. Replace `<WorldMapView>` with:
   ```tsx
   <WorldMapSvg tiles={state.worldTiles} onTravel={handleTravel} />
   ```
4. Add resizable splitter state:
   ```typescript
   const [mapPct, setMapPct] = useState(() => Number(localStorage.getItem('mud-map-splitter') ?? 70))
   ```
   Render zone view as two panes separated by a 6px draggable divider. Left pane = `ZoneMapSvg` at `mapPct%` width. Right pane = details list. On drag end: `localStorage.setItem('mud-map-splitter', String(newPct))`

---

### Step B5 — Delete ASCII renderer (REQ-PB-4)

1. Delete `cmd/webclient/ui/src/game/mapRenderer.ts`
2. Verify no remaining imports of `renderMapTiles` or `ColoredLine` in the codebase

---

### Step B6 — Run Phase B tests and build

```bash
cd cmd/webclient/ui && npm test -- --testPathPattern="ZoneMapSvg|WorldMapSvg" --passWithNoTests
cd cmd/webclient/ui && npm run build
```

---

## Dependency Order

```
A1 (proto) ──▶ A2 (server)
A1 ──────────▶ A3 (TS types) ──▶ A4 (GameContext) ──▶ A5 (MapPanel)
A4 ──────────▶ A6 (run tests)
A2 + A5 ─────▶ A6

B1 ──▶ B2 (ZoneMapSvg) ──┐
B3 (WorldMapSvg) ─────────┼──▶ B4 (wire MapPanel) ──▶ B5 (delete) ──▶ B6
                           ┘
```

Steps A1 and A3 are independent and can run in parallel.
Steps B2 and B3 are independent and can run in parallel.
Phase B is independent of Phase A — both can ship separately.
