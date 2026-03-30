import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import { renderMapTiles } from '../mapRenderer'
import type { ColoredLine } from '../mapRenderer'
import type { WorldZoneTile } from '../../proto'

function renderLines(lines: ColoredLine[]): JSX.Element {
  return (
    <>
      {lines.map((line, i) => (
        <span key={i}>
          {line.map((seg, j) =>
            seg.color
              ? <span key={j} style={{ color: seg.color }}>{seg.text}</span>
              : <span key={j}>{seg.text}</span>
          )}
          {i < lines.length - 1 ? '\n' : ''}
        </span>
      ))}
    </>
  )
}

function renderBattleMap(positions: Record<string, number>): JSX.Element {
  const entries = Object.entries(positions).sort((a, b) => a[1] - b[1])
  if (entries.length === 0) {
    return <span className="map-empty">No combatants positioned.</span>
  }
  const maxPos = entries[entries.length - 1][1]
  const scale = maxPos > 0 ? maxPos : 1
  const barWidth = 50
  const ruler = Array(barWidth + 1).fill('─')
  const labels: Array<{ col: number; name: string }> = entries.map(([name, pos]) => ({
    col: Math.round((pos / scale) * barWidth),
    name,
  }))
  for (const { col } of labels) {
    ruler[Math.min(col, barWidth)] = '┼'
  }
  const rulerStr = ruler.join('')
  return (
    <div style={{ fontFamily: 'monospace', fontSize: '0.85em' }}>
      <div style={{ marginBottom: '0.25rem', color: '#aaa' }}>Battlefield (1ft = 1 unit)</div>
      <div>{rulerStr}</div>
      {labels.map(({ col, name }, i) => (
        <div key={i} style={{ paddingLeft: `${col}ch`, whiteSpace: 'nowrap' }}>
          <span style={{ color: '#f0c040' }}>{`▲ ${name} (${entries[i][1]}ft)`}</span>
        </div>
      ))}
    </div>
  )
}

const DANGER_COLORS: Record<string, string> = {
  safe: '#4a8',
  low: '#8b4',
  medium: '#b84',
  dangerous: '#c54',
  deadly: '#c33',
}

function WorldMapView({ tiles, onTravel }: { tiles: WorldZoneTile[]; onTravel: (zoneId: string) => void }) {
  if (tiles.length === 0) {
    return <p className="map-empty">No world map data.</p>
  }

  const xs = tiles.map(t => t.worldX ?? 0)
  const ys = tiles.map(t => t.worldY ?? 0)
  const minX = Math.min(...xs)
  const maxX = Math.max(...xs)
  const minY = Math.min(...ys)
  const maxY = Math.max(...ys)

  const byCoord = new Map<string, WorldZoneTile>()
  for (const t of tiles) {
    byCoord.set(`${t.worldX ?? 0},${t.worldY ?? 0}`, t)
  }

  const rows: JSX.Element[] = []
  for (let y = minY; y <= maxY; y++) {
    const cells: JSX.Element[] = []
    for (let x = minX; x <= maxX; x++) {
      const tile = byCoord.get(`${x},${y}`)
      if (!tile) {
        cells.push(
          <td key={x} style={styles.worldCell}>
            <span style={{ color: '#333' }}>{'   '}</span>
          </td>
        )
        continue
      }

      const isCurrent = tile.current ?? false
      const isDiscovered = tile.discovered ?? false
      const danger = tile.dangerLevel ?? ''
      const color = isCurrent ? '#ff0' : isDiscovered ? (DANGER_COLORS[danger] ?? '#aaa') : '#444'
      const abbrev = isDiscovered
        ? (tile.zoneName ?? tile.zoneId ?? '?').slice(0, 8).padEnd(8)
        : '  ???   '
      const canTravel = isDiscovered && !isCurrent
      const zoneId = tile.zoneId ?? ''

      cells.push(
        <td key={x} style={styles.worldCell}>
          <span
            style={{
              ...styles.worldTile,
              color,
              cursor: canTravel ? 'pointer' : 'default',
              background: isCurrent ? '#1a2a1a' : canTravel ? '#111' : 'transparent',
              border: isCurrent ? '1px solid #4a8' : canTravel ? '1px solid #333' : '1px solid transparent',
            }}
            onClick={canTravel ? () => onTravel(zoneId) : undefined}
            title={isDiscovered ? `${tile.zoneName ?? zoneId}${canTravel ? ' — click to travel' : ' (current)'}` : 'Undiscovered'}
          >
            {abbrev}
          </span>
        </td>
      )
    }
    rows.push(<tr key={y}>{cells}</tr>)
  }

  return (
    <div style={{ overflow: 'auto', padding: '0.5rem' }}>
      <table style={styles.worldTable}>
        <tbody>{rows}</tbody>
      </table>
      <div style={styles.worldLegend}>
        <span style={{ color: '#ff0' }}>■ Current</span>
        {Object.entries(DANGER_COLORS).map(([d, c]) => (
          <span key={d} style={{ color: c }}>■ {d}</span>
        ))}
        <span style={{ color: '#444' }}>■ Undiscovered</span>
      </div>
    </div>
  )
}

export function MapPanel() {
  const { state, sendMessage } = useGame()
  const [showWorld, setShowWorld] = useState(false)

  function refreshZone() {
    sendMessage('MapRequest', { view: 'zone' })
  }

  function switchToWorld() {
    setShowWorld(true)
    sendMessage('MapRequest', { view: 'world' })
  }

  function switchToZone() {
    setShowWorld(false)
    sendMessage('MapRequest', { view: 'zone' })
  }

  function handleTravel(zoneId: string) {
    sendMessage('TravelRequest', { zone_id: zoneId })
  }

  useEffect(() => {
    if (state.connected) {
      sendMessage('MapRequest', { view: 'zone' })
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.connected])

  const inCombat = state.combatRound !== null

  if (inCombat) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <div className="map-header">
          <h3>Battle Map</h3>
        </div>
        <div style={{ overflow: 'auto', padding: '0.5rem' }}>
          {renderBattleMap(state.combatPositions)}
        </div>
      </div>
    )
  }

  const { gridLines, legendLines } = renderMapTiles(state.mapTiles)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div className="map-header">
        <h3>{showWorld ? 'World Map' : 'Zone Map'}</h3>
        <div style={{ display: 'flex', gap: '0.3rem' }}>
          <button
            className={`map-refresh-btn${!showWorld ? ' active' : ''}`}
            onClick={switchToZone}
            style={!showWorld ? { background: '#1a2a1a', borderColor: '#4a6a2a', color: '#8d4' } : {}}
          >
            Zone
          </button>
          <button
            className={`map-refresh-btn${showWorld ? ' active' : ''}`}
            onClick={switchToWorld}
            style={showWorld ? { background: '#1a2a1a', borderColor: '#4a6a2a', color: '#8d4' } : {}}
          >
            World
          </button>
          {!showWorld && (
            <button className="map-refresh-btn" onClick={refreshZone}>Refresh</button>
          )}
        </div>
      </div>
      {showWorld ? (
        <WorldMapView tiles={state.worldTiles} onTravel={handleTravel} />
      ) : state.mapTiles.length === 0 ? (
        <p className="map-empty">No map data.</p>
      ) : (
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start', overflow: 'auto' }}>
          <pre className="map-ascii" style={{ margin: 0, flexShrink: 0 }}>
            {renderLines(gridLines)}
          </pre>
          <pre className="map-ascii" style={{ margin: 0, flexShrink: 1, minWidth: 0 }}>
            {renderLines(legendLines)}
          </pre>
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  worldTable: {
    borderCollapse: 'collapse',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  worldCell: {
    padding: '2px',
  },
  worldTile: {
    display: 'inline-block',
    padding: '0.15rem 0.3rem',
    borderRadius: '3px',
    whiteSpace: 'nowrap',
    fontSize: '0.7rem',
    letterSpacing: '0.02em',
  },
  worldLegend: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: '0.5rem',
    marginTop: '0.5rem',
    fontSize: '0.7rem',
    fontFamily: 'monospace',
    color: '#666',
  },
}
