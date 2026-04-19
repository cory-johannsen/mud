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

  // Build zone connection lines. Each connection is drawn once (dedup by sorted pair key).
  const drawnConnections = new Set<string>()
  const connectionLines: JSX.Element[] = []
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
      const x1 = px(ax) + ZONE_W / 2
      const y1 = py(ay) + ZONE_H / 2
      const x2 = px(bx) + ZONE_W / 2
      const y2 = py(by) + ZONE_H / 2
      connectionLines.push(
        <line key={pairKey} x1={x1} y1={y1} x2={x2} y2={y2}
          stroke="#5577aa" strokeWidth={1.5} opacity={0.6} />
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
                const nameY = levelRange ? ry + ZONE_H / 2 - 5 : ry + ZONE_H / 2
                return (
                  <>
                    <text
                      x={rx + ZONE_W / 2}
                      y={nameY}
                      textAnchor="middle"
                      dominantBaseline="middle"
                      fontSize={10}
                      fill={isEnemy ? '#c07070' : '#ccc'}
                      clipPath={`url(#clip-${id})`}
                    >
                      {name}
                    </text>
                    {levelRange && (
                      <text
                        x={rx + ZONE_W / 2}
                        y={nameY + 14}
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
        <ZoneTooltip tooltip={tooltip} />
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

function ZoneTooltip({ tooltip }: { tooltip: TooltipState }): JSX.Element {
  const { tile, x, y } = tooltip
  const danger = tile.dangerLevel ?? tile.danger_level ?? ''
  const levelRange = tile.levelRange ?? tile.level_range ?? ''
  const description = tile.description ?? ''
  const isEnemy = tile.enemy ?? false
  const discovered = tile.discovered ?? false

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
        </>
      )}
    </div>
  )
}
