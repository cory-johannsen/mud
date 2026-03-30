import { useEffect } from 'react'
import { useGame } from '../GameContext'
import { renderMapTiles } from '../mapRenderer'
import type { ColoredLine } from '../mapRenderer'

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
  // Build a simple text ruler with combatant labels at proportional positions.
  const ruler = Array(barWidth + 1).fill('─')
  const labels: Array<{ col: number; name: string }> = entries.map(([name, pos]) => ({
    col: Math.round((pos / scale) * barWidth),
    name,
  }))
  // Place combatant markers on ruler
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

export function MapPanel() {
  const { state, sendMessage } = useGame()

  function refreshMap() {
    sendMessage('MapRequest', { view: 'zone' })
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
        <h3>Map</h3>
        <button className="map-refresh-btn" onClick={refreshMap}>Refresh</button>
      </div>
      {state.mapTiles.length === 0 ? (
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
