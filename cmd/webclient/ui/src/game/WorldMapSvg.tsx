import type { WorldZoneTile } from '../proto'

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

interface WorldMapSvgProps {
  tiles: WorldZoneTile[]
  onTravel: (zoneId: string) => void
}

export function WorldMapSvg({ tiles, onTravel }: WorldMapSvgProps): JSX.Element {
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

  return (
    <div style={{ overflow: 'auto', padding: '0.5rem' }}>
      <svg
        viewBox={viewBox}
        width={totalW + 8}
        height={totalH + 8}
        style={{ display: 'block', fontFamily: 'monospace' }}
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

        {tiles.map(tile => {
          const id = tile.zoneId ?? `${tile.worldX ?? 0}-${tile.worldY ?? 0}`
          const rx = px(tile.worldX ?? 0)
          const ry = py(tile.worldY ?? 0)
          const discovered = tile.discovered ?? false
          const isCurrent = tile.current ?? false
          const isEnemy = tile.enemy ?? false
          const danger = tile.dangerLevel ?? tile.danger_level ?? ''
          const fill = discovered ? (DANGER_FILLS[danger] ?? DANGER_FILLS['safe']) : UNDISCOVERED_FILL
          const stroke = isEnemy ? '#c02020' : isCurrent ? CURRENT_STROKE : DEFAULT_STROKE
          const strokeWidth = isEnemy ? 2 : isCurrent ? CURRENT_STROKE_WIDTH : DEFAULT_STROKE_WIDTH
          const canTravel = discovered && !isCurrent && !isEnemy
          const name = tile.zoneName ?? id

          return (
            <g
              key={id}
              onClick={canTravel ? () => onTravel(id) : undefined}
              style={{ cursor: canTravel ? 'pointer' : 'default' }}
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
                const levelRange = tile.levelRange ?? tile.level_range ?? ''
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
