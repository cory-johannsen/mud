import { useState } from 'react'
import type { WorldZoneTile } from '../proto'
import { difficultyBorderColor } from './ZoneMapSvg'

const ZONE_W = 72
const ZONE_H = 44
const GAP = 6  // gap between adjacent zone tiles

const STEP = ZONE_W + GAP
const STEP_H = ZONE_H + GAP

const DANGER_FILLS: Record<string, string> = {
  safe: '#2a4a2a',
  sketchy: '#3a3a1a',
  dangerous: '#4a2a1a',
  deadly: '#4a1a1a',
}

const UNDISCOVERED_FILL = '#111'
const CURRENT_STROKE = '#f0c040'
const CURRENT_STROKE_WIDTH = 2
const DEFAULT_STROKE = '#333'
const DEFAULT_STROKE_WIDTH = 1

interface TooltipState {
  tile: WorldZoneTile
  x: number
  y: number
}

// wrapZoneName splits a zone name into up to 2 lines that fit within ZONE_W.
// At font-size 10 with monospace, ~10 chars fit across 72px.
export function wrapZoneName(name: string, maxChars = 10): [string, string | null] {
  if (name.length <= maxChars) return [name, null]
  const words = name.split(' ')
  let line1 = ''
  for (let i = 0; i < words.length; i++) {
    const attempt = words.slice(0, i + 1).join(' ')
    if (attempt.length <= maxChars) line1 = attempt
    else break
  }
  if (line1) {
    const line2 = name.slice(line1.length + 1, line1.length + 1 + maxChars).trim() || null
    return [line1, line2]
  }
  return [name.slice(0, maxChars), name.slice(maxChars, maxChars * 2).trim() || null]
}

// Tooltip dimensions used for edge-clamping. The tooltip maxWidth is 240px;
// 160px is a conservative height estimate (adjusts for near-bottom edges).
const TOOLTIP_W = 256
const TOOLTIP_H = 160
const TOOLTIP_OFFSET = 12

// computeTooltipPos returns { x, y } clamped so the tooltip stays within containerRect.
// REQ-WM-TT-1: Near right edge, tooltip flips to the left of the cursor.
// REQ-WM-TT-2: Near bottom edge, tooltip flips above the cursor.
export function computeTooltipPos(
  clientX: number,
  clientY: number,
  containerRect: DOMRect,
): { x: number; y: number } {
  const relX = clientX - containerRect.left
  const relY = clientY - containerRect.top
  const x = relX + TOOLTIP_OFFSET + TOOLTIP_W > containerRect.width
    ? relX - TOOLTIP_W - TOOLTIP_OFFSET
    : relX + TOOLTIP_OFFSET
  const y = relY + TOOLTIP_OFFSET + TOOLTIP_H > containerRect.height
    ? relY - TOOLTIP_H - TOOLTIP_OFFSET
    : relY + TOOLTIP_OFFSET
  return { x, y }
}

// segmentIntersectsRect returns true when the line segment (x1,y1)-(x2,y2)
// has any point inside or on the boundary of the axis-aligned rectangle
// whose left/top/right/bottom edges are at (left, top, right, bottom).
// Uses the Liang–Barsky algorithm for robust segment-vs-AABB clipping.
//
// GH #230: used to decide whether a straight world-map connector would
// pass through another zone node's rectangle (and therefore needs to be
// drawn as an arc instead).
export function segmentIntersectsRect(
  x1: number, y1: number,
  x2: number, y2: number,
  left: number, top: number, right: number, bottom: number,
): boolean {
  const dx = x2 - x1
  const dy = y2 - y1
  let tMin = 0
  let tMax = 1
  const checks: [number, number][] = [
    [-dx, x1 - left],
    [dx, right - x1],
    [-dy, y1 - top],
    [dy, bottom - y1],
  ]
  for (const [p, q] of checks) {
    if (p === 0) {
      if (q < 0) return false
      continue
    }
    const t = q / p
    if (p < 0) {
      if (t > tMax) return false
      if (t > tMin) tMin = t
    } else {
      if (t < tMin) return false
      if (t < tMax) tMax = t
    }
  }
  return true
}

// zoneDirection returns a compass direction label from zone A to zone B based on
// their world grid positions. Returns null for diagonal or same-position connections.
export function zoneDirection(
  ax: number, ay: number,
  bx: number, by: number,
): string | null {
  const dx = bx - ax
  const dy = by - ay
  if (dx === 0 && dy === 0) return null
  if (dx === 0) return dy < 0 ? 'N' : 'S'
  if (dy === 0) return dx > 0 ? 'E' : 'W'
  // Diagonal: return compound direction
  const ns = dy < 0 ? 'N' : 'S'
  const ew = dx > 0 ? 'E' : 'W'
  return `${ns}${ew}`
}

interface WorldMapSvgProps {
  tiles: WorldZoneTile[]
  onTravel: (zoneId: string) => void
  playerLevel?: number
}

export function WorldMapSvg({ tiles, onTravel, playerLevel }: WorldMapSvgProps): JSX.Element {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  if (tiles.length === 0) {
    return <p style={{ color: '#666', fontFamily: 'monospace', padding: '0.5rem' }}>No world map data.</p>
  }

  // Normalize coordinates: compress sparse grid to consecutive indices
  const rawXs = tiles.map(t => t.worldX ?? 0)
  const rawYs = tiles.map(t => t.worldY ?? 0)
  const sortedUniqueXs = [...new Set(rawXs)].sort((a, b) => a - b)
  const sortedUniqueYs = [...new Set(rawYs)].sort((a, b) => a - b)
  const normX = new Map(sortedUniqueXs.map((x, i) => [x, i]))
  const normY = new Map(sortedUniqueYs.map((y, i) => [y, i]))

  const px = (wx: number) => (normX.get(wx) ?? 0) * STEP
  const py = (wy: number) => (normY.get(wy) ?? 0) * STEP_H

  const totalW = sortedUniqueXs.length * STEP - GAP
  const totalH = sortedUniqueYs.length * STEP_H - GAP
  const viewBox = `-4 -4 ${totalW + 8} ${totalH + 8}`

  // Build a lookup map from zoneId to tile for connection rendering.
  const tileByZoneId = new Map(tiles.map(t => [t.zoneId ?? '', t]))

  // Connector color palette — cycles through distinct colors so overlapping connections
  // remain visually distinguishable, matching zone map behavior.
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

  // edgePoint returns the border point of a zone rect facing toward (towardX, towardY).
  // Using edge-based endpoints means connections exit from the cell face rather than
  // the center, so cells rendered on top of connectors hide only the interior portion.
  function edgePoint(rx: number, ry: number, toward: { x: number; y: number }): { x: number; y: number } {
    const cx = rx + ZONE_W / 2
    const cy = ry + ZONE_H / 2
    const dx = toward.x - cx
    const dy = toward.y - cy
    if (Math.abs(dx) * ZONE_H >= Math.abs(dy) * ZONE_W) {
      return { x: dx > 0 ? rx + ZONE_W : rx, y: cy }
    } else {
      return { x: cx, y: dy > 0 ? ry + ZONE_H : ry }
    }
  }

  // Build zone connection lines. Each connection is drawn once (dedup by sorted pair key).
  const drawnConnections = new Set<string>()
  const connectionLines: JSX.Element[] = []
  let colorIdx = 0
  for (const tile of tiles) {
    const connections = tile.connectedZoneIds ?? tile.connected_zone_ids ?? []
    const ax = tile.worldX ?? 0
    const ay = tile.worldY ?? 0
    for (const targetId of connections) {
      const target = tileByZoneId.get(targetId)
      if (!target) continue
      const bx = target.worldX ?? 0
      const by = target.worldY ?? 0
      const [ka, kb] = ax < bx || (ax === bx && ay < by)
        ? [`${ax},${ay}`, `${bx},${by}`]
        : [`${bx},${by}`, `${ax},${ay}`]
      const pairKey = `${ka}-${kb}`
      if (drawnConnections.has(pairKey)) continue
      drawnConnections.add(pairKey)

      const color = CONNECTOR_COLORS[colorIdx % CONNECTOR_COLORS.length]
      colorIdx++

      const bCX = px(bx) + ZONE_W / 2
      const bCY = py(by) + ZONE_H / 2
      const aCX = px(ax) + ZONE_W / 2
      const aCY = py(ay) + ZONE_H / 2
      const dep = edgePoint(px(ax), py(ay), { x: bCX, y: bCY })
      const arr = edgePoint(px(bx), py(by), { x: aCX, y: aCY })

      // GH #230: prefer straight connectors. A straight line is only
      // replaced by an arc when the segment would pass through another zone
      // node (i.e. some tile other than A or B has its rectangle intersected
      // by the segment). Adjacency in normalized grid units is no longer the
      // criterion — many distant-looking pairs still have a clear straight
      // path and should draw as a straight line.
      const segmentHitsZone = (): boolean => {
        for (const other of tiles) {
          const ox = other.worldX ?? 0
          const oy = other.worldY ?? 0
          if ((ox === ax && oy === ay) || (ox === bx && oy === by)) continue
          const rx = px(ox)
          const ry = py(oy)
          // Inflate the rectangle by a small margin so a segment that grazes
          // the edge still counts as a conflict.
          const margin = 2
          const left = rx - margin
          const right = rx + ZONE_W + margin
          const top = ry - margin
          const bottom = ry + ZONE_H + margin
          if (segmentIntersectsRect(dep.x, dep.y, arr.x, arr.y, left, top, right, bottom)) {
            return true
          }
        }
        return false
      }

      let pathD: string
      if (!segmentHitsZone()) {
        pathD = `M ${dep.x} ${dep.y} L ${arr.x} ${arr.y}`
      } else {
        const midX = (dep.x + arr.x) / 2
        const midY = (dep.y + arr.y) / 2
        const lineX = arr.x - dep.x
        const lineY = arr.y - dep.y
        const len = Math.sqrt(lineX * lineX + lineY * lineY) || 1
        const perpX = lineY / len
        const perpY = -lineX / len
        const arcOffset = Math.max(STEP, STEP_H) * 0.75
        pathD = `M ${dep.x} ${dep.y} Q ${midX + perpX * arcOffset} ${midY + perpY * arcOffset} ${arr.x} ${arr.y}`
      }

      connectionLines.push(
        <path key={pairKey} d={pathD} stroke={color} strokeWidth={1.5} fill="none" />
      )
    }
  }

  return (
    <div style={{ overflow: 'auto', padding: '0.5rem', position: 'relative' }}>
      <svg
        viewBox={viewBox}
        width={totalW + 8}
        height={totalH + 8}
        style={{ display: 'block', fontFamily: 'monospace' }}
        onMouseLeave={() => setTooltip(null)}
      >
        <defs>
          {tiles.map(tile => {
            const id = tile.zoneId ?? `${tile.worldX ?? 0}-${tile.worldY ?? 0}`
            const rx = px(tile.worldX ?? 0)
            const ry = py(tile.worldY ?? 0)
            return (
              <clipPath key={`clip-${id}`} id={`clip-${id}`}>
                <rect x={rx + 2} y={ry + 2} width={ZONE_W - 4} height={ZONE_H - 4} />
              </clipPath>
            )
          })}
        </defs>

        {connectionLines}

        {tiles.map(tile => {
          const id = tile.zoneId ?? `${tile.worldX ?? 0}-${tile.worldY ?? 0}`
          const rx = px(tile.worldX ?? 0)
          const ry = py(tile.worldY ?? 0)
          const discovered = tile.discovered ?? false
          const isCurrent = tile.current ?? false
          const isEnemy = tile.enemy ?? false
          const danger = tile.dangerLevel ?? tile.danger_level ?? ''
          const fill = discovered ? (DANGER_FILLS[danger] ?? DANGER_FILLS['safe']) : UNDISCOVERED_FILL
          const levelRange = tile.levelRange ?? tile.level_range ?? ''
          const diffColor = (!isEnemy && !isCurrent && discovered)
            ? (difficultyBorderColor(levelRange, playerLevel ?? 0) ?? DEFAULT_STROKE)
            : DEFAULT_STROKE
          const stroke = isEnemy ? '#c02020' : isCurrent ? CURRENT_STROKE : diffColor
          const strokeWidth = isEnemy ? 2 : isCurrent ? CURRENT_STROKE_WIDTH : DEFAULT_STROKE_WIDTH
          const canTravel = discovered && !isCurrent && !isEnemy
          const name = tile.zoneName ?? id

          return (
            <g
              key={id}
              onClick={canTravel ? () => onTravel(id) : undefined}
              style={{ cursor: canTravel ? 'pointer' : 'default' }}
              onMouseEnter={(e) => {
                const svgEl = (e.currentTarget as SVGGElement).closest('svg')
                const containerRect = svgEl?.parentElement?.getBoundingClientRect()
                if (containerRect) {
                  setTooltip({
                    tile,
                    ...computeTooltipPos(e.clientX, e.clientY, containerRect),
                  })
                }
              }}
              onMouseMove={(e) => {
                const svgEl = (e.currentTarget as SVGGElement).closest('svg')
                const containerRect = svgEl?.parentElement?.getBoundingClientRect()
                if (containerRect) {
                  setTooltip(prev => prev ? {
                    ...prev,
                    ...computeTooltipPos(e.clientX, e.clientY, containerRect),
                  } : null)
                }
              }}
              onMouseLeave={() => setTooltip(null)}
            >
              <rect
                x={rx}
                y={ry}
                width={ZONE_W}
                height={ZONE_H}
                fill={fill}
                stroke={stroke}
                strokeWidth={strokeWidth}
              />
              {discovered && (() => {
                const [line1, line2] = wrapZoneName(name)
                // Vertical layout with ZONE_H=44:
                // 1-line, no range:  name at center (22)
                // 2-line, no range:  line1 at 16, line2 at 28
                // 1-line + range:    name at 14, range at 29
                // 2-line + range:    line1 at 10, line2 at 22, range at 34
                const cx = rx + ZONE_W / 2
                const cy = ry + ZONE_H / 2
                const nameColor = isEnemy ? '#c07070' : '#ccc'
                let line1Y: number, line2Y: number | null, rangeY: number | null
                if (line2 && levelRange) {
                  line1Y = ry + 10; line2Y = ry + 22; rangeY = ry + 34
                } else if (line2) {
                  line1Y = cy - 6; line2Y = cy + 7; rangeY = null
                } else if (levelRange) {
                  line1Y = ry + 14; line2Y = null; rangeY = ry + 29
                } else {
                  line1Y = cy; line2Y = null; rangeY = null
                }
                return (
                  <>
                    <text
                      x={cx}
                      y={line1Y}
                      textAnchor="middle"
                      dominantBaseline="middle"
                      fontSize={10}
                      fill={nameColor}
                      clipPath={`url(#clip-${id})`}
                    >
                      {line1}
                    </text>
                    {line2 && (
                      <text
                        x={cx}
                        y={line2Y!}
                        textAnchor="middle"
                        dominantBaseline="middle"
                        fontSize={10}
                        fill={nameColor}
                        clipPath={`url(#clip-${id})`}
                      >
                        {line2}
                      </text>
                    )}
                    {levelRange && (
                      <text
                        x={cx}
                        y={rangeY!}
                        textAnchor="middle"
                        dominantBaseline="middle"
                        fontSize={8}
                        fill="#888"
                        clipPath={`url(#clip-${id})`}
                      >
                        {levelRange}
                      </text>
                    )}
                  </>
                )
              })()}
              {isEnemy && (
                <>
                  <line
                    x1={rx + 4} y1={ry + 4}
                    x2={rx + ZONE_W - 4} y2={ry + ZONE_H - 4}
                    stroke="#c02020" strokeWidth={1.5} opacity={0.6}
                  />
                  <line
                    x1={rx + ZONE_W - 4} y1={ry + 4}
                    x2={rx + 4} y2={ry + ZONE_H - 4}
                    stroke="#c02020" strokeWidth={1.5} opacity={0.6}
                  />
                </>
              )}
            </g>
          )
        })}
      </svg>

      {tooltip && (
        <ZoneTooltip tooltip={tooltip} tiles={tiles} />
      )}

      <div style={{
        display: 'flex',
        flexWrap: 'wrap',
        gap: '0.75rem',
        marginTop: '0.5rem',
        fontSize: '0.7rem',
        fontFamily: 'monospace',
        alignItems: 'center',
      }}>
        {Object.entries(DANGER_FILLS).map(([level, color]) => (
          <span key={level} style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <span style={{
              display: 'inline-block',
              width: 12,
              height: 12,
              background: color,
              border: '1px solid #555',
              borderRadius: 2,
              flexShrink: 0,
            }} />
            <span style={{ color: '#aaa', textTransform: 'capitalize' }}>{level}</span>
          </span>
        ))}
        <span style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          <span style={{
            display: 'inline-block',
            width: 12,
            height: 12,
            background: UNDISCOVERED_FILL,
            border: '1px solid #555',
            borderRadius: 2,
            flexShrink: 0,
          }} />
          <span style={{ color: '#aaa' }}>Undiscovered</span>
        </span>
        <span style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          <span style={{
            display: 'inline-block',
            width: 12,
            height: 12,
            background: '#1a1a2e',
            border: '2px solid #c02020',
            borderRadius: 2,
            flexShrink: 0,
          }} />
          <span style={{ color: '#c07070' }}>Enemy Territory</span>
        </span>
      </div>
    </div>
  )
}

const DANGER_LABELS: Record<string, string> = {
  safe: 'Safe',
  sketchy: 'Sketchy',
  dangerous: 'Dangerous',
  deadly: 'Deadly',
}

const DANGER_COLORS: Record<string, string> = {
  safe: '#6abf69',
  sketchy: '#e6c84e',
  dangerous: '#e08030',
  deadly: '#c03030',
}

function ZoneTooltip({ tooltip, tiles }: { tooltip: TooltipState; tiles: WorldZoneTile[] }): JSX.Element {
  const { tile, x, y } = tooltip
  const danger = tile.dangerLevel ?? tile.danger_level ?? ''
  const levelRange = tile.levelRange ?? tile.level_range ?? ''
  const description = tile.description ?? ''
  const isEnemy = tile.enemy ?? false
  const discovered = tile.discovered ?? false

  // Build list of discovered connected zones with inferred direction.
  const tileByZoneId = new Map(tiles.map(t => [t.zoneId ?? '', t]))
  const connectedIds = tile.connectedZoneIds ?? tile.connected_zone_ids ?? []
  const connections = connectedIds
    .map(id => tileByZoneId.get(id))
    .filter((t): t is WorldZoneTile => !!t && (t.discovered ?? false))
    .map(t => ({
      name: t.zoneName ?? t.zoneId ?? '',
      dir: zoneDirection(
        tile.worldX ?? 0, tile.worldY ?? 0,
        t.worldX ?? 0, t.worldY ?? 0,
      ),
    }))

  return (
    <div
      role="tooltip"
      style={{
        position: 'absolute',
        left: x,
        top: y,
        zIndex: 100,
        background: '#1a1a1a',
        border: `1px solid ${isEnemy ? '#c02020' : '#444'}`,
        borderRadius: 4,
        padding: '0.5rem 0.75rem',
        fontFamily: 'monospace',
        fontSize: '0.72rem',
        color: '#ccc',
        maxWidth: 240,
        pointerEvents: 'none',
        boxShadow: '0 2px 8px rgba(0,0,0,0.6)',
      }}
    >
      {discovered ? (
        <div style={{ fontWeight: 'bold', fontSize: '0.8rem', color: isEnemy ? '#c07070' : '#eee', marginBottom: '0.25rem' }}>
          {tile.zoneName ?? tile.zoneId}
        </div>
      ) : (
        <div style={{ fontWeight: 'bold', fontSize: '0.8rem', color: '#555', marginBottom: '0.25rem' }}>???</div>
      )}
      {!discovered && (
        <div style={{ color: '#666', fontStyle: 'italic' }}>Undiscovered</div>
      )}
      {discovered && (
        <>
          {danger && (
            <div style={{ marginBottom: '0.15rem' }}>
              <span style={{ color: '#888' }}>Danger: </span>
              <span style={{ color: DANGER_COLORS[danger] ?? '#ccc' }}>
                {DANGER_LABELS[danger] ?? danger}
              </span>
            </div>
          )}
          {levelRange && (
            <div style={{ marginBottom: '0.15rem' }}>
              <span style={{ color: '#888' }}>Levels: </span>
              <span style={{ color: '#bbb' }}>{levelRange}</span>
            </div>
          )}
          {isEnemy && (
            <div style={{ color: '#c07070', marginBottom: '0.15rem' }}>Enemy Territory</div>
          )}
          {description && (
            <div style={{ color: '#999', marginTop: '0.35rem', lineHeight: 1.4, borderTop: '1px solid #333', paddingTop: '0.35rem' }}>
              {description}
            </div>
          )}
          {connections.length > 0 && (
            <div style={{ marginTop: '0.35rem', borderTop: '1px solid #333', paddingTop: '0.35rem' }}>
              <div style={{ color: '#888', fontSize: '0.68rem', marginBottom: '0.2rem' }}>Connections</div>
              {connections.map((c, i) => (
                <div key={i} style={{ color: '#bbb', fontSize: '0.72rem' }}>
                  {'→ '}{c.name}{c.dir ? ` (${c.dir})` : ''}
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
