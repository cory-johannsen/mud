import type { MapTile } from '../proto'

const CELL_W = 56
const CELL_H = 36

const DANGER_FILLS: Record<string, string> = {
  safe: '#2a4a2a',
  sketchy: '#3a3a1a',
  dangerous: '#4a2a1a',
  deadly: '#4a1a1a',
}

const POI_SYMBOLS: Record<string, string> = {
  merchant: '$',
  healer: '+',
  trainer: 'T',
  quest_giver: '!',
  motel: 'Z',
  npc: '@',
  guard: 'G',
}

const DIR_OFFSETS: Record<string, [number, number]> = {
  n: [0, -1],
  s: [0, 1],
  e: [1, 0],
  w: [-1, 0],
  ne: [1, -1],
  nw: [-1, -1],
  se: [1, 1],
  sw: [-1, 1],
}

interface ZoneMapSvgProps {
  tiles: MapTile[]
  onHover?: (tile: MapTile, e: React.MouseEvent) => void
  onHoverEnd?: () => void
}

export function ZoneMapSvg({ tiles, onHover, onHoverEnd }: ZoneMapSvgProps): JSX.Element {
  if (tiles.length === 0) {
    return <p style={{ color: '#666', fontFamily: 'monospace', padding: '0.5rem' }}>No map data.</p>
  }

  // Build lookup map keyed by "x,y"
  const tileMap = new Map<string, MapTile>()
  for (const tile of tiles) {
    const tx = tile.x ?? 0
    const ty = tile.y ?? 0
    tileMap.set(`${tx},${ty}`, tile)
  }

  // Compute bounds
  const xs = tiles.map(t => t.x ?? 0)
  const ys = tiles.map(t => t.y ?? 0)
  const minX = Math.min(...xs)
  const maxX = Math.max(...xs)
  const minY = Math.min(...ys)
  const maxY = Math.max(...ys)

  const viewBox = `${minX * CELL_W - 8} ${minY * CELL_H - 8} ${(maxX - minX + 1) * CELL_W + 16} ${(maxY - minY + 1) * CELL_H + 16}`

  const OPPOSITE_DIR: Record<string, string> = {
    n: 's', s: 'n', e: 'w', w: 'e', ne: 'sw', sw: 'ne', nw: 'se', se: 'nw',
  }

  // Build connectors, deduplicating pairs
  const drawnPairs = new Set<string>()
  const connectors: JSX.Element[] = []

  for (const tile of tiles) {
    const tx = tile.x ?? 0
    const ty = tile.y ?? 0
    const cx1 = tx * CELL_W + CELL_W / 2
    const cy1 = ty * CELL_H + CELL_H / 2
    const isZoneExit = !!(tile.zoneExits?.length || tile.zone_exits?.length)

    for (const dir of tile.exits ?? []) {
      const offsets = DIR_OFFSETS[dir]
      if (!offsets) continue

      const [dx, dy] = offsets
      const oppDir = OPPOSITE_DIR[dir]

      // Search along direction for any tile that has the opposite exit
      let step = 1
      let found: MapTile | undefined
      let nx = tx + dx * step
      let ny = ty + dy * step
      while (tileMap.has(`${nx},${ny}`) || step <= maxX - minX + maxY - minY) {
        const candidate = tileMap.get(`${nx},${ny}`)
        if (candidate) {
          // Only connect if the neighbor has an exit pointing back
          if (oppDir && (candidate.exits ?? []).includes(oppDir)) {
            found = candidate
          }
          break
        }
        step++
        nx = tx + dx * step
        ny = ty + dy * step
        if (Math.abs(nx - tx) > maxX - minX + 1 || Math.abs(ny - ty) > maxY - minY + 1) break
      }

      if (!found) continue

      // Build a canonical pair key (sorted so each pair is unique)
      const fnx = found.x ?? 0
      const fny = found.y ?? 0
      const [ax, ay, bx, by] =
        tx < fnx || (tx === fnx && ty < fny)
          ? [tx, ty, fnx, fny]
          : [fnx, fny, tx, ty]
      const pairKey = `${ax},${ay}-${bx},${by}`
      if (drawnPairs.has(pairKey)) continue
      drawnPairs.add(pairKey)

      const cx2 = fnx * CELL_W + CELL_W / 2
      const cy2 = fny * CELL_H + CELL_H / 2

      const neighborIsZoneExit = !!(found.zoneExits?.length || found.zone_exits?.length)
      const zoneConnector = isZoneExit || neighborIsZoneExit

      connectors.push(
        <line
          key={pairKey}
          x1={cx1}
          y1={cy1}
          x2={cx2}
          y2={cy2}
          stroke={zoneConnector ? '#8888ff' : '#555'}
          strokeDasharray={zoneConnector ? '4 2' : undefined}
        />
      )
    }
  }

  return (
    <div style={{ overflow: 'auto', width: '100%', height: '100%' }}>
      <svg viewBox={viewBox} style={{ display: 'block', minWidth: '100%' }}>
        {/* connectors rendered first so tiles appear on top */}
        {connectors}

        {/* tile rects */}
        {tiles.map(tile => {
          const tx = tile.x ?? 0
          const ty = tile.y ?? 0
          const dangerKey = tile.dangerLevel ?? tile.danger_level ?? ''
          const fill = DANGER_FILLS[dangerKey] ?? '#1e1e2e'
          const isCurrent = tile.current ?? false
          const isBoss = tile.bossRoom ?? false
          const stroke = isCurrent ? '#f0c040' : isBoss ? '#cc4444' : '#333'
          const strokeWidth = isCurrent || isBoss ? 2 : 1
          return (
            <rect
              key={`rect-${tile.roomId ?? tx}-${ty}`}
              x={tx * CELL_W}
              y={ty * CELL_H}
              width={CELL_W}
              height={CELL_H}
              rx={4}
              fill={fill}
              stroke={stroke}
              strokeWidth={strokeWidth}
              onMouseEnter={onHover ? e => onHover(tile, e) : undefined}
              onMouseLeave={onHoverEnd}
            />
          )
        })}

        {/* room name texts */}
        {tiles.map(tile => {
          const tx = tile.x ?? 0
          const ty = tile.y ?? 0
          const name = tile.roomName ?? tile.name ?? ''

          return (
            <text
              key={`name-${tile.roomId ?? tx}-${ty}`}
              x={tx * CELL_W + CELL_W / 2}
              y={ty * CELL_H + CELL_H / 2 + 2}
              textAnchor="middle"
              dominantBaseline="middle"
              fontSize={9}
              fill="#ccc"
              pointerEvents="none"
            >
              {name}
            </text>
          )
        })}

        {/* POI symbol texts */}
        {tiles.flatMap(tile => {
          const tx = tile.x ?? 0
          const ty = tile.y ?? 0
          return (tile.pois ?? []).map((poi, idx) => (
            <text
              key={`poi-${tile.roomId ?? tx}-${ty}-${idx}`}
              x={tx * CELL_W + CELL_W - 4}
              y={ty * CELL_H + 10 + idx * 11}
              textAnchor="end"
              fontSize={10}
              fill="#f0c040"
              pointerEvents="none"
            >
              {POI_SYMBOLS[poi] ?? '?'}
            </text>
          ))
        })}
      </svg>
    </div>
  )
}
