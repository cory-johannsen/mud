# Map Room Tooltip Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a popup tooltip when hovering over a room cell in the web UI zone map, displaying room name, danger level, POIs, and exits.

**Architecture:** Extend the `Segment` type in `mapRenderer.ts` with an optional `tile` field so room cells carry their tile reference. `MapPanel` checks for this field and wraps those segments in interactive `<span>` elements. A new `RoomTooltip` component renders the popup via `ReactDOM.createPortal`, following the same pattern as `NpcModal.tsx`'s `ItemTooltip`.

**Tech Stack:** React 18, TypeScript, ReactDOM.createPortal, Vitest + @testing-library/react, jsdom

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `cmd/webclient/ui/src/game/mapRenderer.ts` | Add `tile?: MapTile` to `Segment`; export `POI_TYPES`; attach tile to room cell segments |
| Create | `cmd/webclient/ui/src/game/mapRenderer.test.ts` | Verify room segments carry tile reference |
| Create | `cmd/webclient/ui/src/game/RoomTooltip.tsx` | Tooltip component (portal to body, pointer-events: none) |
| Create | `cmd/webclient/ui/src/game/RoomTooltip.test.tsx` | Verify tooltip renders name/danger/POIs/exits |
| Modify | `cmd/webclient/ui/src/game/panels/MapPanel.tsx` | Hover state; interactive room spans; render RoomTooltip |

---

### Task 1: Extend Segment type and tag room cells in mapRenderer.ts

**Files:**
- Modify: `cmd/webclient/ui/src/game/mapRenderer.ts`
- Create: `cmd/webclient/ui/src/game/mapRenderer.test.ts`

- [ ] **Step 1: Write the failing test**

Create `cmd/webclient/ui/src/game/mapRenderer.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { renderMapTiles } from './mapRenderer'
import type { MapTile } from '../proto'

const WEST: MapTile = { roomId: 'r1', roomName: 'West Room', x: 0, y: 0, exits: ['east'], dangerLevel: 'safe', pois: [] }
const EAST: MapTile = { roomId: 'r2', roomName: 'East Room', x: 2, y: 0, exits: ['west'], dangerLevel: 'sketchy', pois: ['merchant'] }

describe('renderMapTiles', () => {
  it('attaches tile reference to room cell segments', () => {
    const { gridLines } = renderMapTiles([WEST, EAST])
    const roomRow = gridLines[0]

    // Find all segments that have a tile attached
    const tiledSegs = roomRow.filter(s => s.tile !== undefined)

    expect(tiledSegs).toHaveLength(2)
    expect(tiledSegs[0].tile?.roomId).toBe('r1')
    expect(tiledSegs[1].tile?.roomId).toBe('r2')
  })

  it('does not attach tile to connector or padding segments', () => {
    const { gridLines } = renderMapTiles([WEST, EAST])
    const roomRow = gridLines[0]
    const connectors = roomRow.filter(s => s.tile === undefined && s.text.trim() !== '')
    // '-' connector between the two rooms should have no tile
    expect(connectors.some(s => s.text === '-')).toBe(true)
  })

  it('returns empty result for empty tiles', () => {
    const result = renderMapTiles([])
    expect(result.gridLines).toHaveLength(0)
    expect(result.legendLines).toHaveLength(0)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd cmd/webclient/ui && npm test -- mapRenderer.test.ts
```

Expected: FAIL — `tile` property does not exist on `Segment`.

- [ ] **Step 3: Add `tile` to Segment and tag room cells in mapRenderer.ts**

In `cmd/webclient/ui/src/game/mapRenderer.ts`:

**a) Export `POI_TYPES` (add `export` keyword):**

```typescript
// POI type table — matches poi.go
export const POI_TYPES: Array<{ id: string; symbol: string; color: string; label: string }> = [
```

**b) Add `tile` to `Segment`:**

```typescript
export interface Segment {
  text: string
  color?: string  // CSS color; undefined = inherit from .map-ascii
  tile?: MapTile  // set on room cell segments only; used by MapPanel for hover tooltips
}
```

**c) Update the `seg` helper to accept an optional `tile`:**

```typescript
function seg(text: string, color?: string, tile?: MapTile): Segment {
  const s: Segment = color ? { text, color } : { text }
  if (tile !== undefined) s.tile = tile
  return s
}
```

**d) In the room cell rendering block (lines ~133–141), pass `t` as the third argument:**

```typescript
      const t = byCoord.get(coordKey(x, y))
      if (!t) {
        row.push(seg('    '))
      } else {
        const num = numByCoord.get(coordKey(x, y))!
        if (t.current) {
          row.push(seg(`<${String(num).padStart(2)}>`, CURRENT_ROOM_COLOR, t))
        } else if (t.boss === true || t.bossRoom === true) {
          row.push(seg('<BB>', dangerColor(t), t))
        } else {
          row.push(seg(`[${String(num).padStart(2)}]`, dangerColor(t), t))
        }
      }
```

No other changes — connector, POI, and south-connector rows remain unchanged.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd cmd/webclient/ui && npm test -- mapRenderer.test.ts
```

Expected: 3 tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
cd cmd/webclient/ui && npm test
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/webclient/ui/src/game/mapRenderer.ts cmd/webclient/ui/src/game/mapRenderer.test.ts
git commit -m "feat(map): tag room cell segments with tile reference for hover tooltips"
```

---

### Task 2: Create RoomTooltip component

**Files:**
- Create: `cmd/webclient/ui/src/game/RoomTooltip.tsx`
- Create: `cmd/webclient/ui/src/game/RoomTooltip.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `cmd/webclient/ui/src/game/RoomTooltip.test.tsx`:

```typescript
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RoomTooltip } from './RoomTooltip'
import type { MapTile } from '../proto'

const tile: MapTile = {
  roomId: 'grinders_row',
  roomName: "Grinder's Row",
  x: 0,
  y: 0,
  current: true,
  exits: ['north', 'east', 'south'],
  dangerLevel: 'safe',
  pois: ['merchant', 'healer', 'npc'],
}

describe('RoomTooltip', () => {
  it('renders the room name', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText("Grinder's Row")).toBeDefined()
  })

  it('renders the danger level', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('safe')).toBeDefined()
  })

  it('renders all POI labels', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('Merchant')).toBeDefined()
    expect(screen.getByText('Healer')).toBeDefined()
    expect(screen.getByText('NPC')).toBeDefined()
  })

  it('renders all exits', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('north')).toBeDefined()
    expect(screen.getByText('east')).toBeDefined()
    expect(screen.getByText('south')).toBeDefined()
  })

  it('renders "(current room)" indicator when tile.current is true', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('current room')).toBeDefined()
  })

  it('does not render "(current room)" when tile.current is false', () => {
    const nonCurrentTile: MapTile = { ...tile, current: false }
    render(<RoomTooltip tile={nonCurrentTile} pos={{ x: 100, y: 200 }} />)
    expect(screen.queryByText('current room')).toBeNull()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd cmd/webclient/ui && npm test -- RoomTooltip.test.tsx
```

Expected: FAIL — `RoomTooltip` module not found.

- [ ] **Step 3: Create RoomTooltip.tsx**

Create `cmd/webclient/ui/src/game/RoomTooltip.tsx`:

```typescript
import ReactDOM from 'react-dom'
import type { MapTile } from '../proto'
import { POI_TYPES } from './mapRenderer'

const DANGER_COLOR: Record<string, string> = {
  safe:        '#4a8',
  sketchy:     '#cc0',
  dangerous:   '#f80',
  all_out_war: '#f44',
}

interface RoomTooltipProps {
  tile: MapTile
  pos: { x: number; y: number }
}

export function RoomTooltip({ tile, pos }: RoomTooltipProps) {
  const name = tile.roomName ?? tile.name ?? 'Unknown Room'
  const danger = tile.dangerLevel ?? tile.danger_level ?? ''
  const dangerColor = DANGER_COLOR[danger] ?? '#8ab'
  const pois = Array.isArray(tile.pois) ? tile.pois : []
  const exits = Array.isArray(tile.exits) ? tile.exits : []
  const isCurrent = tile.current === true

  // Resolve tooltip position: appear below the hovered element, clamp to viewport.
  const style: React.CSSProperties = {
    position:    'fixed',
    left:        Math.min(pos.x, window.innerWidth - 220),
    top:         pos.y + 6,
    zIndex:      2000,
    background:  '#1a1a1a',
    border:      '1px solid #444',
    borderRadius: '4px',
    padding:     '0.5rem 0.65rem',
    minWidth:    '180px',
    maxWidth:    '260px',
    pointerEvents: 'none',
    fontFamily:  'monospace',
    fontSize:    '0.78rem',
    lineHeight:  '1.5',
    color:       '#ccc',
    boxShadow:   '0 4px 12px rgba(0,0,0,0.6)',
  }

  return ReactDOM.createPortal(
    <div style={style}>
      {/* Room name */}
      <div style={{ color: '#fff', fontWeight: 'bold', marginBottom: '0.25rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
        <span>{name}</span>
        {isCurrent && (
          <span style={{ fontSize: '0.65rem', color: '#4a8', border: '1px solid #4a8', borderRadius: '3px', padding: '0 3px' }}>
            current room
          </span>
        )}
      </div>

      {/* Danger level */}
      {danger && (
        <div style={{ marginBottom: '0.2rem' }}>
          <span style={{ color: '#666' }}>Danger: </span>
          <span style={{ color: dangerColor }}>{danger}</span>
        </div>
      )}

      {/* POIs */}
      {pois.length > 0 && (
        <div style={{ marginBottom: '0.2rem' }}>
          <div style={{ color: '#666', marginBottom: '0.1rem' }}>Points of Interest:</div>
          {pois.map(id => {
            const pt = POI_TYPES.find(p => p.id === id)
            return (
              <div key={id} style={{ paddingLeft: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                <span style={{ color: pt?.color ?? '#ccc' }}>{pt?.symbol ?? '?'}</span>
                <span>{pt?.label ?? id}</span>
              </div>
            )
          })}
        </div>
      )}

      {/* Exits */}
      {exits.length > 0 && (
        <div>
          <span style={{ color: '#666' }}>Exits: </span>
          {exits.map((e, i) => (
            <span key={e}>
              {i > 0 && <span style={{ color: '#555' }}>, </span>}
              <span style={{ color: '#aac' }}>{e}</span>
            </span>
          ))}
        </div>
      )}
    </div>,
    document.body,
  )
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd cmd/webclient/ui && npm test -- RoomTooltip.test.tsx
```

Expected: 6 tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
cd cmd/webclient/ui && npm test
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/webclient/ui/src/game/RoomTooltip.tsx cmd/webclient/ui/src/game/RoomTooltip.test.tsx
git commit -m "feat(map): add RoomTooltip component for zone map hover popups"
```

---

### Task 3: Wire hover state and tooltip into MapPanel

**Files:**
- Modify: `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

- [ ] **Step 1: Write the failing test**

There is no existing `MapPanel.test.tsx`. Create `cmd/webclient/ui/src/game/panels/MapPanel.test.tsx`:

```typescript
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

// Mock GameContext so MapPanel can render without a real WebSocket.
vi.mock('../GameContext', () => ({
  useGame: () => ({
    state: {
      connected: false,
      mapTiles: [
        {
          roomId: 'grinders_row',
          roomName: "Grinder's Row",
          x: 0,
          y: 0,
          current: true,
          exits: ['east'],
          dangerLevel: 'safe',
          pois: ['merchant'],
        },
        {
          roomId: 'last_stand_lodge',
          roomName: 'Last Stand Lodge',
          x: 2,
          y: 0,
          current: false,
          exits: ['west'],
          dangerLevel: 'safe',
          pois: [],
        },
      ],
      worldTiles: [],
      combatRound: null,
      combatPositions: {},
    },
    sendMessage: vi.fn(),
    sendCommand: vi.fn(),
  }),
}))

import { MapPanel } from './MapPanel'

describe('MapPanel room hover tooltip', () => {
  it('shows no tooltip initially', () => {
    render(<MapPanel />)
    expect(screen.queryByText("Grinder's Row")).toBeNull()
  })

  it('shows tooltip with room name on mouse enter', () => {
    render(<MapPanel />)
    // Room cells are rendered as spans with data-room attribute
    const roomSpan = document.querySelector('[data-room="grinders_row"]')
    expect(roomSpan).not.toBeNull()
    fireEvent.mouseEnter(roomSpan!, { clientX: 100, clientY: 200 })
    expect(screen.getByText("Grinder's Row")).toBeDefined()
  })

  it('hides tooltip on mouse leave', () => {
    render(<MapPanel />)
    const roomSpan = document.querySelector('[data-room="grinders_row"]')!
    fireEvent.mouseEnter(roomSpan, { clientX: 100, clientY: 200 })
    expect(screen.getByText("Grinder's Row")).toBeDefined()
    fireEvent.mouseLeave(roomSpan)
    expect(screen.queryByText("Grinder's Row")).toBeNull()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd cmd/webclient/ui && npm test -- MapPanel.test.tsx
```

Expected: FAIL — `data-room` attribute not present; no tooltip rendered.

- [ ] **Step 3: Update MapPanel.tsx to add hover state and tooltip**

**a) Add imports at top of `MapPanel.tsx`:**

```typescript
import { useEffect, useState, useCallback } from 'react'
import { useGame } from '../GameContext'
import { renderMapTiles } from '../mapRenderer'
import type { ColoredLine } from '../mapRenderer'
import type { MapTile } from '../../proto'
import type { WorldZoneTile } from '../../proto'
import { RoomTooltip } from '../RoomTooltip'
```

**b) Replace the `renderLines` function and add hover-aware variant:**

Remove the existing `renderLines` function (lines 7–22) and replace with:

```typescript
interface HoverHandlers {
  onMouseEnter: (tile: MapTile, e: React.MouseEvent) => void
  onMouseLeave: () => void
}

function renderLines(lines: ColoredLine[], hover?: HoverHandlers): JSX.Element {
  return (
    <>
      {lines.map((line, i) => (
        <span key={i}>
          {line.map((seg, j) => {
            if (seg.tile && hover) {
              const tile = seg.tile
              return (
                <span
                  key={j}
                  style={{ color: seg.color, cursor: 'default' }}
                  data-room={tile.roomId ?? ''}
                  onMouseEnter={e => hover.onMouseEnter(tile, e)}
                  onMouseLeave={hover.onMouseLeave}
                >
                  {seg.text}
                </span>
              )
            }
            return seg.color
              ? <span key={j} style={{ color: seg.color }}>{seg.text}</span>
              : <span key={j}>{seg.text}</span>
          })}
          {i < lines.length - 1 ? '\n' : ''}
        </span>
      ))}
    </>
  )
}
```

**c) In `MapPanel()`, add hover state after the existing `useState` calls:**

```typescript
  const [hoveredTile, setHoveredTile] = useState<MapTile | null>(null)
  const [tooltipPos, setTooltipPos] = useState({ x: 0, y: 0 })

  const handleRoomEnter = useCallback((tile: MapTile, e: React.MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    setTooltipPos({ x: rect.left, y: rect.bottom })
    setHoveredTile(tile)
  }, [])

  const handleRoomLeave = useCallback(() => {
    setHoveredTile(null)
  }, [])

  const hoverHandlers: HoverHandlers = {
    onMouseEnter: handleRoomEnter,
    onMouseLeave: handleRoomLeave,
  }
```

**d) Pass `hoverHandlers` to the grid `renderLines` call and add tooltip. Replace the zone map `<div>` block (lines ~222–230):**

```typescript
      ) : (
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start', overflow: 'auto' }}>
          <pre className="map-ascii" style={{ margin: 0, flexShrink: 0 }}>
            {renderLines(gridLines, hoverHandlers)}
          </pre>
          <pre className="map-ascii" style={{ margin: 0, flexShrink: 1, minWidth: 0 }}>
            {renderLines(legendLines)}
          </pre>
          {hoveredTile && (
            <RoomTooltip tile={hoveredTile} pos={tooltipPos} />
          )}
        </div>
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd cmd/webclient/ui && npm test -- MapPanel.test.tsx
```

Expected: 3 tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
cd cmd/webclient/ui && npm test
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/webclient/ui/src/game/panels/MapPanel.tsx cmd/webclient/ui/src/game/panels/MapPanel.test.tsx
git commit -m "feat(map): show room info tooltip on hover in zone map"
```

---

### Task 4: Build, deploy, and verify

**Files:** None (build + deploy only)

- [ ] **Step 1: Build the UI locally to confirm no TypeScript errors**

```bash
cd cmd/webclient/ui && npm run build
```

Expected: `✓ built in` — no type errors.

- [ ] **Step 2: Deploy**

```bash
make k8s-redeploy
```

Expected: All three images built and pushed; helm upgrade succeeds.

- [ ] **Step 3: Verify in browser**

Open the web client, navigate to the Zone Map tab. Hover over any room cell `[XX]` — a popup should appear showing:
- Room name (white, bold)
- Danger level (colored)
- Points of Interest (if any — symbol + label)
- Exits (comma-separated, light blue)
- "current room" badge on the player's current room

Move the mouse away — popup disappears.
