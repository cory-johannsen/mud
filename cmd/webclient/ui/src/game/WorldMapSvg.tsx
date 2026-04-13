import type { WorldZoneTile } from '../proto'

const ZONE_W = 80
const ZONE_H = 50

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

interface WorldMapSvgProps {
  tiles: WorldZoneTile[]
  onTravel: (zoneId: string) => void
}

export function WorldMapSvg({ tiles, onTravel }: WorldMapSvgProps): JSX.Element {
  if (tiles.length === 0) {
    return <p style={{ color: '#666', fontFamily: 'monospace', padding: '0.5rem' }}>No world map data.</p>
  }

  const xs = tiles.map(t => t.worldX ?? 0)
  const ys = tiles.map(t => t.worldY ?? 0)
  const minX = Math.min(...xs)
  const maxX = Math.max(...xs)
  const minY = Math.min(...ys)
  const maxY = Math.max(...ys)

  const vbX = minX * ZONE_W
  const vbY = minY * ZONE_H
  const vbW = (maxX - minX + 1) * ZONE_W
  const vbH = (maxY - minY + 1) * ZONE_H
  const viewBox = `${vbX} ${vbY} ${vbW} ${vbH}`

  return (
    <div style={{ overflow: 'auto', padding: '0.5rem' }}>
      <svg
        viewBox={viewBox}
        width={vbW}
        height={vbH}
        style={{ display: 'block', fontFamily: 'monospace' }}
      >
        <defs>
          {tiles.map(tile => {
            const id = tile.zoneId ?? ''
            const cx = (tile.worldX ?? 0) * ZONE_W
            const cy = (tile.worldY ?? 0) * ZONE_H
            return (
              <clipPath key={`clip-${id}`} id={`clip-${id}`}>
                <rect x={cx + 2} y={cy + 2} width={ZONE_W - 4} height={ZONE_H - 4} />
              </clipPath>
            )
          })}
        </defs>

        {tiles.map(tile => {
          const id = tile.zoneId ?? ''
          const wx = tile.worldX ?? 0
          const wy = tile.worldY ?? 0
          const cx = wx * ZONE_W
          const cy = wy * ZONE_H
          const discovered = tile.discovered ?? false
          const isCurrent = tile.current ?? false
          const danger = tile.dangerLevel ?? ''
          const fill = discovered ? (DANGER_FILLS[danger] ?? DANGER_FILLS['safe']) : UNDISCOVERED_FILL
          const stroke = isCurrent ? CURRENT_STROKE : DEFAULT_STROKE
          const strokeWidth = isCurrent ? CURRENT_STROKE_WIDTH : DEFAULT_STROKE_WIDTH
          const canTravel = discovered && !isCurrent
          const name = tile.zoneName ?? id

          return (
            <g
              key={id}
              onClick={canTravel ? () => onTravel(id) : undefined}
              style={{ cursor: canTravel ? 'pointer' : 'default' }}
            >
              <rect
                x={cx}
                y={cy}
                width={ZONE_W}
                height={ZONE_H}
                fill={fill}
                stroke={stroke}
                strokeWidth={strokeWidth}
              />
              {discovered && (
                <text
                  x={cx + ZONE_W / 2}
                  y={cy + ZONE_H / 2}
                  textAnchor="middle"
                  dominantBaseline="middle"
                  fontSize={10}
                  fill="#ccc"
                  clipPath={`url(#clip-${id})`}
                >
                  {name}
                </text>
              )}
            </g>
          )
        })}
      </svg>

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
      </div>
    </div>
  )
}
