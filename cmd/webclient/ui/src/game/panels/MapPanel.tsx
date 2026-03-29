import { useEffect } from 'react'
import { useGame } from '../GameContext'
import { renderMapTiles } from '../mapRenderer'

export function MapPanel() {
  const { state, sendMessage } = useGame()

  function refreshMap() {
    sendMessage('MapRequest', { view: 'zone' })
  }

  // Auto-request map on first connect.
  useEffect(() => {
    if (state.connected) {
      sendMessage('MapRequest', { view: 'zone' })
    }
  // Only fire when connection state changes to true.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.connected])

  const ascii = renderMapTiles(state.mapTiles)

  return (
    <div>
      <div className="map-header">
        <h3>Map</h3>
        <button className="map-refresh-btn" onClick={refreshMap}>Refresh</button>
      </div>
      {state.mapTiles.length === 0 ? (
        <p className="map-empty">No map data.</p>
      ) : (
        <pre className="map-ascii">{ascii}</pre>
      )}
    </div>
  )
}
