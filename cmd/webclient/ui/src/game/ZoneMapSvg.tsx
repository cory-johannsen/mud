import type { MapTile } from '../proto'

const CELL_W = 52
const CELL_H = 32
const GAP = 10  // gap between adjacent tiles — connectors are visible here

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

const OPPOSITE_DIR: Record<string, string> = {
  n: 's', s: 'n', e: 'w', w: 'e', ne: 'sw', sw: 'ne', nw: 'se', se: 'nw',
}

const STEP = CELL_W + GAP  // horizontal step between tile origins
const STEP_H = CELL_H + GAP  // vertical step between tile origins

interface ZoneMapSvgProps {
  tiles: MapTile[]
  onHover?: (tile: MapTile, e: React.MouseEvent) => void
  onHoverEnd?: () => void
}

function clipId(tile: MapTile): string {
  return `clip-${tile.roomId ?? `${tile.x ?? 0}-${tile.y ?? 0}`}`
}

// Split a room name into up to 2 lines that fit within the tile width.
// At font-size 9, ~9 chars fit across CELL_W=52.
function wrapRoomName(name: string, maxChars = 9): [string, string | null] {
  if (name.length <= maxChars) return [name, null]
  const words = name.split(' ')
  let line1 = ''
  let rest = words
  for (let i = 0; i < words.length; i++) {
    const attempt = words.slice(0, i + 1).join(' ')
    if (attempt.length <= maxChars) {
      line1 = attempt
    } else {
      rest = words.slice(i)
      break
    }
    rest = words.slice(i + 1)
  }
  if (!line1) line1 = name.slice(0, maxChars)
  const line2 = rest.join(' ').slice(0, maxChars) || null
  return [line1, line2]
}

export function ZoneMapSvg({ tiles, onHover, onHoverEnd }: ZoneMapSvgProps): JSX.Element {
  if (tiles.length === 0) {
    return <p style={{ color: '#666', fontFamily: 'monospace', padding: '0.5rem' }}>No map data.</p>
  }

  // Build lookup map keyed by original "x,y"
  const tileMap = new Map<string, MapTile>()
  for (const tile of tiles) {
    tileMap.set(`${tile.x ?? 0},${tile.y ?? 0}`, tile)
  }

  // Normalize coordinates: compress sparse grid to consecutive indices so
  // there's no wasted blank space between non-adjacent rooms.
  const rawXs = tiles.map(t => t.x ?? 0)
  const rawYs = tiles.map(t => t.y ?? 0)
  const sortedUniqueXs = [...new Set(rawXs)].sort((a, b) => a - b)
  const sortedUniqueYs = [...new Set(rawYs)].sort((a, b) => a - b)
  const normX = new Map(sortedUniqueXs.map((x, i) => [x, i]))
  const normY = new Map(sortedUniqueYs.map((y, i) => [y, i]))

  // Convert original tile coordinate to SVG pixel position (top-left of tile)
  const px = (tx: number) => (normX.get(tx) ?? 0) * STEP
  const py = (ty: number) => (normY.get(ty) ?? 0) * STEP_H

  // Center of a tile in SVG space
  const cx = (tx: number) => px(tx) + CELL_W / 2
  const cy = (ty: number) => py(ty) + CELL_H / 2

  const totalW = sortedUniqueXs.length * STEP - GAP
  const totalH = sortedUniqueYs.length * STEP_H - GAP
  const viewBox = `-8 -8 ${totalW + 16} ${totalH + 16}`

  // Compute original bounds (for connector search loop limit)
  const minX = Math.min(...rawXs)
  const maxX = Math.max(...rawXs)
  const minY = Math.min(...rawYs)
  const maxY = Math.max(...rawYs)

  // Build connectors, deduplicating pairs
  const drawnPairs = new Set<string>()
  const connectors: JSX.Element[] = []

  for (const tile of tiles) {
    const tx = tile.x ?? 0
    const ty = tile.y ?? 0
    const isZoneExit = !!(tile.zoneExits?.length || tile.zone_exits?.length)

    for (const dir of tile.exits ?? []) {
      const offsets = DIR_OFFSETS[dir]
      if (!offsets) continue

      const [dx, dy] = offsets
      const oppDir = OPPOSITE_DIR[dir]

      // Search along direction in original coordinate space
      let step = 1
      let found: MapTile | undefined
      while (step <= maxX - minX + maxY - minY + 2) {
        const nx = tx + dx * step
        const ny = ty + dy * step
        if (Math.abs(nx - tx) > maxX - minX + 1 || Math.abs(ny - ty) > maxY - minY + 1) break
        const candidate = tileMap.get(`${nx},${ny}`)
        if (candidate) {
          if (oppDir && (candidate.exits ?? []).includes(oppDir)) {
            found = candidate
          }
          break
        }
        step++
      }

      if (!found) continue

      const fnx = found.x ?? 0
      const fny = found.y ?? 0
      const [ax, ay, bx, by] =
        tx < fnx || (tx === fnx && ty < fny)
          ? [tx, ty, fnx, fny]
          : [fnx, fny, tx, ty]
      const pairKey = `${ax},${ay}-${bx},${by}`
      if (drawnPairs.has(pairKey)) continue
      drawnPairs.add(pairKey)

      const neighborIsZoneExit = !!(found.zoneExits?.length || found.zone_exits?.length)
      const zoneConnector = isZoneExit || neighborIsZoneExit

      connectors.push(
        <line
          key={pairKey}
          x1={cx(tx)} y1={cy(ty)} x2={cx(fnx)} y2={cy(fny)}
          stroke={zoneConnector ? '#8888ff' : '#888'}
          strokeWidth={2}
          strokeDasharray={zoneConnector ? '4 2' : undefined}
        />
      )
    }
  }

  // Render tiles in two passes: non-current first, current tile last (on top)
  const nonCurrentTiles = tiles.filter(t => !(t.current ?? false))
  const currentTiles = tiles.filter(t => t.current ?? false)
  const orderedTiles = [...nonCurrentTiles, ...currentTiles]

  function renderTile(tile: MapTile): JSX.Element {
    const tx = tile.x ?? 0
    const ty = tile.y ?? 0
    const rx = px(tx)
    const ry = py(ty)
    const dangerKey = tile.dangerLevel ?? tile.danger_level ?? ''
    const fill = DANGER_FILLS[dangerKey] ?? '#1e1e2e'
    const isCurrent = tile.current ?? false
    const isBoss = tile.bossRoom ?? tile.boss ?? false
    const stroke = isCurrent ? '#f0c040' : isBoss ? '#cc4444' : '#333'
    const strokeWidth = isCurrent || isBoss ? 2 : 1
    const name = tile.roomName ?? ''
    const id = clipId(tile)
    const [line1, line2] = wrapRoomName(name)
    // Vertically center text block; shift up slightly when there are POIs at bottom
    const hasPois = (tile.pois ?? []).length > 0
    const textMidY = hasPois ? ry + CELL_H / 2 - 4 : ry + CELL_H / 2
    const lineH = 10  // px between baselines

    return (
      <g key={`tile-${tile.roomId ?? tx}-${ty}`}>
        <rect
          x={rx} y={ry}
          width={CELL_W} height={CELL_H}
          rx={4}
          fill={fill} stroke={stroke} strokeWidth={strokeWidth}
          onMouseEnter={onHover ? e => onHover(tile, e) : undefined}
          onMouseLeave={onHoverEnd}
        />
        {line2 ? (
          <text
            fontSize={9} fill="#ccc" pointerEvents="none"
            clipPath={`url(#${id})`}
          >
            <tspan x={rx + CELL_W / 2} y={textMidY - lineH / 2} textAnchor="middle" dominantBaseline="middle">
              {line1}
            </tspan>
            <tspan x={rx + CELL_W / 2} dy={lineH} textAnchor="middle" dominantBaseline="middle">
              {line2}
            </tspan>
          </text>
        ) : (
          <text
            x={rx + CELL_W / 2}
            y={textMidY}
            textAnchor="middle"
            dominantBaseline="middle"
            fontSize={9}
            fill="#ccc"
            pointerEvents="none"
            clipPath={`url(#${id})`}
          >
            {line1}
          </text>
        )}
        {(tile.pois ?? []).map((poi, idx) => (
          <text
            key={`poi-${id}-${idx}`}
            x={rx + 4 + idx * 11}
            y={ry + CELL_H - 4}
            textAnchor="start"
            fontSize={9}
            fill="#f0c040"
            pointerEvents="none"
          >
            {POI_SYMBOLS[poi] ?? '?'}
          </text>
        ))}
      </g>
    )
  }

  return (
    <div style={{ overflow: 'auto', width: '100%', height: '100%' }}>
      <svg viewBox={viewBox} style={{ display: 'block', minWidth: '100%' }}>
        <defs>
          {tiles.map(tile => {
            const tx = tile.x ?? 0
            const ty = tile.y ?? 0
            return (
              <clipPath key={`clip-${clipId(tile)}`} id={clipId(tile)}>
                <rect x={px(tx)} y={py(ty)} width={CELL_W} height={CELL_H} />
              </clipPath>
            )
          })}
        </defs>
        {/* connectors rendered first; visible in the GAP between tiles */}
        {connectors}
        {/* tiles: non-current first, then current tile on top */}
        {orderedTiles.map(renderTile)}
      </svg>
    </div>
  )
}
