import type { MapTile, SameZoneExitTarget } from '../proto'

const BASE_CELL_W = 52
const BASE_CELL_H = 32
const BASE_GAP = 10  // gap between adjacent tiles at reference width

// Reference container width at which base constants apply.
export const REFERENCE_W = 400

// computeZoneMapLayout returns cell and gap dimensions for the given container width.
// Gap grows linearly with container width (fast); cells grow by sqrt (slow).
// This makes map expansion primarily widen spacing rather than just enlarging cells.
//
// Precondition: containerW > 0.
// Postcondition: All returned values are >= the base constants.
export function computeZoneMapLayout(containerW: number): { cellW: number; cellH: number; gap: number } {
  const ratio = Math.max(1, containerW / REFERENCE_W)
  const gap = Math.round(BASE_GAP * ratio)
  const cellScale = Math.sqrt(ratio)
  const cellW = Math.round(BASE_CELL_W * cellScale)
  const cellH = Math.round(BASE_CELL_H * cellScale)
  return { cellW, cellH, gap }
}

const DANGER_FILLS: Record<string, string> = {
  safe: '#2a4a2a',
  sketchy: '#3a3a1a',
  dangerous: '#4a2a1a',
  deadly: '#4a1a1a',
}

// Server sends full direction names (e.g. "north"), not abbreviations.
// Single source of truth for POI display — symbol, color, and label.
// Colors match RoomTooltip POI_TYPES so tiles and hover are consistent.
const POI_DEFS: Array<{ id: string; symbol: string; color: string; label: string }> = [
  { id: 'merchant',    symbol: '$',  color: '#0bc', label: 'Merchant'     },
  { id: 'healer',      symbol: '+',  color: '#4a8', label: 'Healer'       },
  { id: 'trainer',     symbol: 'T',  color: '#48f', label: 'Trainer'      },
  { id: 'quest_giver', symbol: '!',  color: '#fa0', label: 'Quest'        },
  { id: 'motel',       symbol: '💤', color: '#d8f', label: 'Motel (rest)' },
  { id: 'brothel',     symbol: 'B',  color: '#f64', label: 'Brothel'      },
  { id: 'banker',      symbol: '¤',  color: '#fa0', label: 'Banker'       },
  { id: 'npc',         symbol: '@',  color: '#aaa', label: 'NPC'          },
  { id: 'guard',       symbol: 'G',  color: '#cc0', label: 'Guard'        },
  { id: 'cover',       symbol: 'C',  color: '#cc0', label: 'Cover'        },
  { id: 'equipment',   symbol: 'E',  color: '#c8f', label: 'Equipment'    },
  { id: 'map',         symbol: 'M',  color: '#0cc', label: 'Map'          },
]

const POI_BY_ID = new Map(POI_DEFS.map(p => [p.id, p]))

// buildZoneExitArrows returns directional arrow descriptors computed from cell and gap dimensions.
// Arrows are rendered outside the tile in the gap area so they remain visible.
function buildZoneExitArrows(cellW: number, cellH: number, gap: number): Record<string, { glyph: string; ex: number; ey: number; anchor: 'start' | 'middle' | 'end' | 'inherit' }> {
  return {
    north:     { glyph: '↑', ex: cellW / 2,       ey: -(gap / 2),       anchor: 'middle' },
    south:     { glyph: '↓', ex: cellW / 2,       ey: cellH + gap / 2,  anchor: 'middle' },
    east:      { glyph: '→', ex: cellW + gap / 2, ey: cellH / 2,        anchor: 'middle' },
    west:      { glyph: '←', ex: -(gap / 2),       ey: cellH / 2,        anchor: 'middle' },
    northeast: { glyph: '↗', ex: cellW + gap / 2, ey: -(gap / 2),       anchor: 'middle' },
    northwest: { glyph: '↖', ex: -(gap / 2),       ey: -(gap / 2),       anchor: 'middle' },
    southeast: { glyph: '↘', ex: cellW + gap / 2, ey: cellH + gap / 2,  anchor: 'middle' },
    southwest: { glyph: '↙', ex: -(gap / 2),       ey: cellH + gap / 2,  anchor: 'middle' },
  }
}

// parseLevelRange parses "min-max" or "N" into numeric bounds. Returns null if unparseable.
function parseLevelRange(range: string): { min: number; max: number } | null {
  const rangeMatch = range.match(/^(\d+)-(\d+)$/)
  if (rangeMatch) return { min: parseInt(rangeMatch[1], 10), max: parseInt(rangeMatch[2], 10) }
  const singleMatch = range.match(/^(\d+)$/)
  if (singleMatch) { const n = parseInt(singleMatch[1], 10); return { min: n, max: n } }
  return null
}

// difficultyBorderColor returns a CSS color string for a zone's difficulty relative to the player's
// level, or null when no comparison can be made (missing range or level).
//
// Postcondition: returns one of: '#4a8' (green), '#e6c84e' (yellow), '#e08030' (orange),
//   '#c03030' (red), '#444' (dark grey), or null.
export function difficultyBorderColor(levelRange: string | undefined, playerLevel: number): string | null {
  if (!levelRange || !playerLevel) return null
  const parsed = parseLevelRange(levelRange)
  if (!parsed) return null
  const { min, max } = parsed
  if (max < playerLevel) return '#444'            // zone is below player level
  if (playerLevel >= min && playerLevel <= max) return '#4a8'  // player is within range
  const gap = min - playerLevel
  if (gap <= 2) return '#e6c84e'                 // 1–2 levels above player
  if (gap <= 4) return '#e08030'                 // 3–4 levels above player
  return '#c03030'                               // 5+ levels above player
}

interface ZoneMapSvgProps {
  tiles: MapTile[]
  onHover?: (tile: MapTile, e: React.MouseEvent) => void
  onHoverEnd?: () => void
  // containerWidth drives adaptive gap/cell scaling. Defaults to REFERENCE_W.
  containerWidth?: number
  // playerLevel and zoneLevelRange enable difficulty-relative border color coding.
  playerLevel?: number
  zoneLevelRange?: string
  // onTileClick fires when the user clicks an explored, non-current tile.
  onTileClick?: (tile: MapTile) => void
  // destinationRoomId highlights the target tile with a blue border.
  destinationRoomId?: string | null
}

function clipId(tile: MapTile): string {
  return `clip-${tile.roomId ?? `${tile.x ?? 0}-${tile.y ?? 0}`}`
}

// Split a room name into up to 2 lines that fit within the tile width.
// At font-size 9, ~10 chars fit across CELL_W=52.
function wrapRoomName(name: string, maxChars = 10): [string, string | null] {
  if (name.length <= maxChars) return [name, null]

  // Try to break at the last word boundary that fits in maxChars
  const words = name.split(' ')
  let line1 = ''
  for (let i = 0; i < words.length; i++) {
    const attempt = words.slice(0, i + 1).join(' ')
    if (attempt.length <= maxChars) line1 = attempt
    else break
  }

  if (line1) {
    // Break after line1; remainder starts after the trailing space
    const line2 = name.slice(line1.length + 1, line1.length + 1 + maxChars).trim() || null
    return [line1, line2]
  }

  // First word itself exceeds maxChars — hard-split by character
  return [name.slice(0, maxChars), name.slice(maxChars, maxChars * 2).trim() || null]
}

export function ZoneMapSvg({ tiles, onHover, onHoverEnd, containerWidth, playerLevel, zoneLevelRange, onTileClick, destinationRoomId }: ZoneMapSvgProps): JSX.Element {
  if (tiles.length === 0) {
    return <p style={{ color: '#666', fontFamily: 'monospace', padding: '0.5rem' }}>No map data.</p>
  }

  // Compute adaptive cell and gap sizes based on the container width.
  const { cellW, cellH, gap } = computeZoneMapLayout(containerWidth ?? REFERENCE_W)
  const STEP = cellW + gap
  const STEP_H = cellH + gap
  const ZONE_EXIT_ARROW = buildZoneExitArrows(cellW, cellH, gap)

  // Build lookup map keyed by original "x,y"
  const tileMap = new Map<string, MapTile>()
  for (const tile of tiles) {
    tileMap.set(`${tile.x ?? 0},${tile.y ?? 0}`, tile)
  }

  // Normalize coordinates: compress sparse grid to consecutive indices
  const rawXs = tiles.map(t => t.x ?? 0)
  const rawYs = tiles.map(t => t.y ?? 0)
  const sortedUniqueXs = [...new Set(rawXs)].sort((a, b) => a - b)
  const sortedUniqueYs = [...new Set(rawYs)].sort((a, b) => a - b)
  const normX = new Map(sortedUniqueXs.map((x, i) => [x, i]))
  const normY = new Map(sortedUniqueYs.map((y, i) => [y, i]))

  const px = (tx: number) => (normX.get(tx) ?? 0) * STEP
  const py = (ty: number) => (normY.get(ty) ?? 0) * STEP_H
  const centerX = (tx: number) => px(tx) + cellW / 2
  const centerY = (ty: number) => py(ty) + cellH / 2

  const totalW = sortedUniqueXs.length * STEP - gap
  const totalH = sortedUniqueYs.length * STEP_H - gap
  const viewBox = `-8 -8 ${totalW + 16} ${totalH + 16}`

  // Build a room-ID → tile lookup for connector drawing.
  const tileByRoomId = new Map<string, MapTile>()
  for (const tile of tiles) {
    if (tile.roomId) tileByRoomId.set(tile.roomId, tile)
  }

  // Connector color palette — each connection gets a distinct color from this palette,
  // cycling as needed, so overlapping lines remain visually distinguishable.
  const CONNECTOR_COLORS = [
    '#6688bb', // blue-grey
    '#88bb66', // green-grey
    '#bb8866', // amber
    '#66bbbb', // teal
    '#bb6688', // pink
    '#bbbb66', // yellow-green
    '#8866bb', // purple
    '#66bb88', // mint
  ]

  // edgePoint returns the point on the border of a cell rect that faces toward (towardX, towardY).
  // Lines drawn from one cell's edge to another's edge avoid interior-cell rendering: since cells
  // are drawn on top of connectors, only the gap-area portions of each path are visible.
  function edgePoint(rx: number, ry: number, cW: number, cH: number, towardX: number, towardY: number): { x: number; y: number } {
    const cx = rx + cW / 2
    const cy = ry + cH / 2
    const dx = towardX - cx
    const dy = towardY - cy
    // Compare normalized distances to determine which edge (east/west vs north/south) is closer.
    if (Math.abs(dx) * cH >= Math.abs(dy) * cW) {
      return { x: dx > 0 ? rx + cW : rx, y: cy }
    } else {
      return { x: cx, y: dy > 0 ? ry + cH : ry }
    }
  }

  // Build connectors using same-zone exit targets (direction → target room ID).
  // This correctly handles exits that are not spatially adjacent in the grid.
  const drawnPairs = new Set<string>()
  const connectors: JSX.Element[] = []
  let colorIdx = 0

  for (const tile of tiles) {
    const tx = tile.x ?? 0
    const ty = tile.y ?? 0
    const exitTargets: SameZoneExitTarget[] = tile.sameZoneExitTargets ?? tile.same_zone_exit_targets ?? []
    for (const et of exitTargets) {
      const targetId = et.targetRoomId ?? et.target_room_id ?? ''
      const found = tileByRoomId.get(targetId)
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

      const color = CONNECTOR_COLORS[colorIdx % CONNECTOR_COLORS.length]
      colorIdx++

      const aCX = centerX(ax)
      const aCY = centerY(ay)
      const bCX = centerX(bx)
      const bCY = centerY(by)

      // Edge-based endpoints: line exits the source cell face toward the destination
      // and enters the destination cell face toward the source.
      const dep = edgePoint(px(ax), py(ay), cellW, cellH, bCX, bCY)
      const arr = edgePoint(px(bx), py(by), cellW, cellH, aCX, aCY)

      // Adjacent connections (normalized distance ≤ 1 in both axes) use a straight line.
      // Non-adjacent connections use a quadratic Bézier arc — the control point is offset
      // perpendicular to the direct path by one STEP, routing the curve through the gap
      // area and visually clearing intermediate cells.
      const aNX = normX.get(ax) ?? 0
      const aNY = normY.get(ay) ?? 0
      const bNX = normX.get(bx) ?? 0
      const bNY = normY.get(by) ?? 0
      const isAdjacent = Math.abs(aNX - bNX) <= 1 && Math.abs(aNY - bNY) <= 1

      let pathD: string
      if (isAdjacent) {
        pathD = `M ${dep.x} ${dep.y} L ${arr.x} ${arr.y}`
      } else {
        const midX = (dep.x + arr.x) / 2
        const midY = (dep.y + arr.y) / 2
        const lineX = arr.x - dep.x
        const lineY = arr.y - dep.y
        const len = Math.sqrt(lineX * lineX + lineY * lineY) || 1
        // Perpendicular unit vector (rotate 90° clockwise)
        const perpX = lineY / len
        const perpY = -lineX / len
        const arcOffset = Math.max(STEP, STEP_H) * 0.75
        pathD = `M ${dep.x} ${dep.y} Q ${midX + perpX * arcOffset} ${midY + perpY * arcOffset} ${arr.x} ${arr.y}`
      }

      connectors.push(
        <path key={pairKey} d={pathD} stroke={color} strokeWidth={2} fill="none" />
      )
    }
  }

  // Render tiles in two passes: non-current first, current tile last (on top)
  const nonCurrentTiles = tiles.filter(t => !(t.current ?? false))
  const currentTiles = tiles.filter(t => t.current ?? false)
  const orderedTiles = [...nonCurrentTiles, ...currentTiles]

  // Collect which POI types are actually present in this zone for the legend
  const presentPois = new Set(tiles.flatMap(t => t.pois ?? []))

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
    const diffColor = (!isCurrent && !isBoss)
      ? (difficultyBorderColor(zoneLevelRange, playerLevel ?? 0) ?? '#333')
      : '#333'
    const stroke = isDestination ? '#4a9eff' : isCurrent ? '#f0c040' : isBoss ? '#cc4444' : diffColor
    const strokeWidth = isDestination || isCurrent || isBoss ? 2 : 1
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
        {line2 ? (
          <text fontSize={9} fill="#ccc" pointerEvents="none" clipPath={`url(#${id})`}>
            <tspan x={rx + cellW / 2} y={textMidY - lineH / 2} textAnchor="middle" dominantBaseline="middle">
              {line1}
            </tspan>
            <tspan x={rx + cellW / 2} dy={lineH} textAnchor="middle" dominantBaseline="middle">
              {line2}
            </tspan>
          </text>
        ) : (
          <text
            x={rx + cellW / 2} y={textMidY}
            textAnchor="middle" dominantBaseline="middle"
            fontSize={9} fill="#ccc" pointerEvents="none"
            clipPath={`url(#${id})`}
          >
            {line1}
          </text>
        )}
        {(tile.pois ?? []).length > 0 && (
          <text
            x={rx + 2}
            y={ry + cellH - 4}
            textAnchor="start"
            fontSize={9}
            pointerEvents="none"
            clipPath={`url(#${id})`}
          >
            {(tile.pois ?? []).map((poi, idx) => {
              const def = POI_BY_ID.get(poi)
              return (
                <tspan key={`poi-${id}-${idx}`} fill={def?.color ?? '#aaa'}>
                  {def?.symbol ?? '?'}
                </tspan>
              )
            })}
          </text>
        )}
        {(tile.zoneExits ?? tile.zone_exits ?? []).map((ze, idx) => {
          const dir = ze.direction ?? ''
          const arrow = ZONE_EXIT_ARROW[dir]
          if (!arrow) return null
          return (
            <text
              key={`ze-${id}-${idx}`}
              x={rx + arrow.ex}
              y={ry + arrow.ey}
              textAnchor={arrow.anchor}
              dominantBaseline="middle"
              fontSize={10}
              fill="#8888ff"
              pointerEvents="none"
            >
              {arrow.glyph}
            </text>
          )
        })}
      </g>
    )
  }

  const legendEntries = POI_DEFS.filter(e => presentPois.has(e.id))

  return (
    <div style={{ overflow: 'auto', width: '100%', height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ overflow: 'auto', flex: 1 }}>
        <svg viewBox={viewBox} style={{ display: 'block', minWidth: '100%' }}>
          <defs>
            {tiles.map(tile => {
              const tx = tile.x ?? 0
              const ty = tile.y ?? 0
              return (
                <clipPath key={`clip-${clipId(tile)}`} id={clipId(tile)}>
                  <rect x={px(tx)} y={py(ty)} width={cellW} height={cellH} />
                </clipPath>
              )
            })}
          </defs>
          {connectors}
          {orderedTiles.map(renderTile)}
        </svg>
      </div>
      {legendEntries.length > 0 && (
        <div style={{
          display: 'flex', flexWrap: 'wrap', gap: '0.4rem 0.75rem',
          padding: '0.35rem 0.5rem',
          borderTop: '1px solid #333',
          fontSize: '0.68rem', fontFamily: 'monospace', color: '#aaa',
          flexShrink: 0,
        }}>
          {legendEntries.map(e => (
            <span key={e.id}>
              <span style={{ color: e.color }}>{e.symbol}</span>
              {' '}{e.label}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}
