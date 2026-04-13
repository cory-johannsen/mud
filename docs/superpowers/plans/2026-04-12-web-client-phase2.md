# Plan: Web Game Client — Phase 2

**GitHub Issue:** cory-johannsen/mud#13
**Spec:** `docs/superpowers/specs/2026-04-12-web-client-phase2.md`
**Date:** 2026-04-12

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

**TDD:** In `combat_handler_test.go` (or the nearest test for RoundStart), assert that a 20×20 combat produces a `RoundStartEvent` with `GridWidth = 20` and `GridHeight = 20`.

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

### Step A4 — GameContext: store grid dimensions in state (REQ-PA-1d)

**File:** `cmd/webclient/ui/src/game/GameContext.tsx`

1. Add to state type:
```typescript
combatGridWidth: number
combatGridHeight: number
```
2. Initial state: `combatGridWidth: 20, combatGridHeight: 20`
3. In `ROUND_START` reducer case, extract and store:
```typescript
combatGridWidth: action.payload.gridWidth ?? action.payload.grid_width ?? 20,
combatGridHeight: action.payload.gridHeight ?? action.payload.grid_height ?? 20,
```

---

### Step A5 — MapPanel: dynamic grid + compass dpad + AP pips (REQ-PA-2, REQ-PA-3, REQ-PA-4, REQ-PA-5)

**File:** `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

This is the main UI change. Replace the entire `inCombat` branch (lines 238-286) and update `renderBattleGrid`.

#### A5a — Update `renderBattleGrid` signature

Change from:
```typescript
function renderBattleGrid(
  combatPositions: Record<string, { x: number; y: number }>,
  playerName: string
): JSX.Element
```

To:
```typescript
function renderBattleGrid(
  combatPositions: Record<string, { x: number; y: number }>,
  playerName: string,
  gridWidth: number,
  gridHeight: number
): JSX.Element
```

Remove `const GRID_SIZE = 20` and `const CELL_PX = 16`. Compute cell size dynamically:
```typescript
const rawCell = Math.floor(Math.min(320, 320) / Math.max(gridWidth, gridHeight))
const CELL_PX = Math.max(12, Math.min(32, rawCell))
```

Use `gridWidth` and `gridHeight` in the loop instead of `GRID_SIZE`.

#### A5b — Compass directional pad component

Add a `DPad` component inside `MapPanel.tsx`:

```typescript
const COMPASS_DIRS = [
  ['nw','n','ne'],
  ['w', '', 'e'],
  ['sw','s','se'],
] as const

function DPad({
  onDir,
  disabledDirs,
  disabled,
}: {
  onDir: (dir: string) => void
  disabledDirs: Set<string>
  disabled: boolean
}): JSX.Element {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 28px)', gap: '2px' }}>
      {COMPASS_DIRS.flat().map((dir, i) => (
        dir === '' ? (
          <button
            key={i}
            disabled={disabled}
            style={{ width: 28, height: 28, background: '#1a3a6b', border: '1px solid #3a5a9b', borderRadius: 3, color: '#7bb8ff', fontSize: '0.75rem', cursor: disabled ? 'not-allowed' : 'pointer' }}
            onClick={() => onDir('toward')}
            title="Stride toward nearest enemy"
          >⊕</button>
        ) : (
          <button
            key={i}
            disabled={disabled || disabledDirs.has(dir)}
            style={{
              width: 28, height: 28,
              background: disabled || disabledDirs.has(dir) ? '#111' : '#1a2a1a',
              border: '1px solid #333',
              borderRadius: 3, color: disabled || disabledDirs.has(dir) ? '#444' : '#8d4',
              fontSize: '0.7rem', cursor: disabled || disabledDirs.has(dir) ? 'not-allowed' : 'pointer',
            }}
            onClick={() => onDir(dir)}
            title={dir.toUpperCase()}
          >{dir.toUpperCase()}</button>
        )
      ))}
    </div>
  )
}
```

#### A5c — AP pip indicator component

```typescript
function ApPips({ remaining, total }: { remaining: number; total: number }): JSX.Element {
  return (
    <div style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
      {Array.from({ length: total }).map((_, i) => (
        <div key={i} style={{
          width: 10, height: 10, borderRadius: '50%',
          background: i < remaining ? '#f0c040' : 'transparent',
          border: '2px solid #f0c040',
        }} />
      ))}
    </div>
  )
}
```

#### A5d — Compute disabled directions

In the `inCombat` branch, compute bounds-disabled directions:
```typescript
const playerPos = state.combatPositions[state.characterInfo?.name ?? '']
const apEntry = /* find from initialPositions or derive from state */
const apRemaining = /* from CombatantPosition */
const apDisabled = apRemaining === 0

const disabledDirs = new Set<string>()
if (playerPos) {
  if (playerPos.y === 0) { disabledDirs.add('n'); disabledDirs.add('nw'); disabledDirs.add('ne') }
  if (playerPos.y === gridHeight - 1) { disabledDirs.add('s'); disabledDirs.add('sw'); disabledDirs.add('se') }
  if (playerPos.x === 0) { disabledDirs.add('w'); disabledDirs.add('nw'); disabledDirs.add('sw') }
  if (playerPos.x === gridWidth - 1) { disabledDirs.add('e'); disabledDirs.add('ne'); disabledDirs.add('se') }
}
```

**Note:** AP remaining requires storing it in state when `UPDATE_COMBAT_POSITION` is dispatched (it's already present in `CombatantPosition.ap_remaining`). Add `combatApRemaining: number` to `GameContext` state, updated on `ROUND_START` and `UPDATE_COMBAT_POSITION` for the current player.

#### A5e — Step mode toggle

Add `const [stepMode, setStepMode] = useState(false)` to `MapPanel`. The DPad `onDir` callback sends `stepMode ? \`step \${dir}\` : \`stride \${dir}\``.

#### A5f — Replace inCombat JSX

Replace the current `inCombat` return block with:

```tsx
return (
  <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
    <div className="map-header">
      <h3>Battle Map</h3>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
        <ApPips remaining={apRemaining} total={apTotal} />
        <label style={{ fontSize: '0.75rem', color: '#aaa', display: 'flex', alignItems: 'center', gap: '4px' }}>
          <input type="checkbox" checked={stepMode} onChange={e => setStepMode(e.target.checked)} />
          Step
        </label>
        <button
          className="map-refresh-btn"
          style={{ background: '#2a1a1a', borderColor: '#7a2a2a', color: '#f66' }}
          onClick={() => sendCommand('flee')}
        >
          Flee!
        </button>
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

### Step A6 — GameContext: store AP remaining (REQ-PA-4a, REQ-PA-4c)

**File:** `cmd/webclient/ui/src/game/GameContext.tsx`

Add to state:
```typescript
combatApRemaining: number
combatApTotal: number
```

Defaults: `0`, `3`.

In `ROUND_START`: find the current player's `CombatantPosition` in `initial_positions` and set `combatApRemaining` and `combatApTotal`.

In `UPDATE_COMBAT_POSITION` (if the updated combatant matches current player): update `combatApRemaining` and `combatApTotal`.

---

### Step A7 — Run Phase A tests and build

```bash
mise exec -- go test ./internal/gameserver/... -count=1
cd cmd/webclient/ui && npm run build
```

---

## Phase B: Zone & World Map Polish

### Step B1 — Section separators in legend (REQ-PB-1)

**File:** `cmd/webclient/ui/src/game/mapRenderer.ts` (or wherever `legendLines` is built)

Identify where legend lines are generated. Add section header rows (`LEGEND`, `POINTS OF INTEREST`, `ROOMS`) and separator lines between sections in the output.

If `renderMapTiles` returns `{ gridLines, legendLines }`, split `legendLines` into sections and inject separator `ColoredLine` entries with `color: '#555'` and text `─────────────`.

---

### Step B2 — Resizable splitter for map/legend (REQ-PB-2)

**File:** `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

Replace the static flex layout for zone map (the `display: 'flex', gap: '1rem'` div at lines 318-330) with a resizable split. Use the same pattern as the existing horizontal splitters in the main layout.

```typescript
const [legendWidthPct, setLegendWidthPct] = useState(() => {
  const saved = localStorage.getItem('mud-map-splitter')
  return saved ? Number(saved) : 40
})
```

Render as two panels with a draggable 6px divider between them. On drag end, persist to `localStorage`.

---

### Step B3 — Run Phase B build

```bash
cd cmd/webclient/ui && npm run build
```

---

## Phase C: PixiJS Tiled Room Scene

Phase C implements `docs/superpowers/specs/2026-03-26-web-client-phase2-design.md` requirements REQ-WC2-1 through REQ-WC2-24. The steps below are a high-level ordering only — a detailed subagent-driven plan should be created when Phase C begins.

### Step C1 — `internal/client/assets` sub-package (REQ-WC2-1..5)

New package: `internal/client/assets/`
- `assets.go`: `AssetVersion`, `FetchLatestVersion`, `ParseVersion`, `ErrNoRelease`, `ErrNetwork`
- `assets_test.go`: `httptest.Server`-backed tests for all exported functions

### Step C2 — Go asset proxy endpoint (REQ-WC2-6..7)

**File:** `cmd/webclient/`
- `WebConfig.GitHubReleasesURL` field
- `GET /api/assets/version` handler (auth-exempt)
- `configs/dev.yaml` updated

### Step C3 — AssetPackContext (REQ-WC2-8..12)

**Files:** `cmd/webclient/ui/src/`
- `AssetPackContext.tsx`: download, SHA-256 verify, IndexedDB cache, version check
- `AssetErrorScreen.tsx`: retry screen for no-network/no-cache
- `TilesConfig` and `PixiTextureMap` TypeScript types

### Step C4 — ScenePanel + layers (REQ-WC2-13..22)

**Files:** `cmd/webclient/ui/src/game/`
- `ScenePanel.tsx`: PixiJS `Application` via `useRef`, four layers
- Room panel layout update (60/40 split: scene + text)

### Step C5 — CombatAnimationQueue (REQ-WC2-23..24)

**File:** `cmd/webclient/ui/src/game/CombatAnimationQueue.ts`
- `enqueue(spriteId, type)`, sequential queue per sprite
- `attack`, `hit-flash`, `death` animation types

### Step C6 — Full integration test

```bash
mise exec -- go test ./internal/client/... -count=1
cd cmd/webclient/ui && npm run build
```

---

## Dependency Order

```
A1 (proto) ──▶ A2 (server) ──▶ A7 (test)
A1 ──────────▶ A3 (TS types)
A3 ──────────▶ A4 (GameContext grid)
A4 ──────────▶ A5 (MapPanel grid + dpad)
A4 ──────────▶ A6 (GameContext AP)
A6 ──────────▶ A5d (AP-gated nav)
A5 + A6 ─────▶ A7

B1 + B2 ─────▶ B3

A7 ──▶ B (can run after Phase A)
B3 ──▶ C (can run after Phase B)
```

Steps A1 and A3 are independent of A2 and can run in parallel.
Phase B is independent of Phase C.
