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
