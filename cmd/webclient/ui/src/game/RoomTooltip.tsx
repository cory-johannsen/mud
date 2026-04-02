import ReactDOM from 'react-dom'
import type { MapTile } from '../proto'
import { POI_TYPES } from './mapRenderer'

const DANGER_COLOR: Record<string, string> = {
  safe:        '#4a8',
  sketchy:     '#cc0',
  dangerous:   '#f80',
  all_out_war: '#f44',
}

interface RoomTooltipProps {
  tile: MapTile
  pos: { x: number; y: number }
}

export function RoomTooltip({ tile, pos }: RoomTooltipProps) {
  const name = tile.roomName ?? tile.name ?? 'Unknown Room'
  const danger = tile.dangerLevel ?? tile.danger_level ?? ''
  const dangerColor = DANGER_COLOR[danger] ?? '#8ab'
  const pois = Array.isArray(tile.pois) ? tile.pois : []
  const exits = Array.isArray(tile.exits) ? tile.exits : []
  const isCurrent = tile.current === true

  // Resolve tooltip position: appear below the hovered element, clamp to viewport.
  const style: React.CSSProperties = {
    position:    'fixed',
    left:        Math.min(pos.x, window.innerWidth - 220),
    top:         pos.y + 6,
    zIndex:      2000,
    background:  '#1a1a1a',
    border:      '1px solid #444',
    borderRadius: '4px',
    padding:     '0.5rem 0.65rem',
    minWidth:    '180px',
    maxWidth:    '260px',
    pointerEvents: 'none',
    fontFamily:  'monospace',
    fontSize:    '0.78rem',
    lineHeight:  '1.5',
    color:       '#ccc',
    boxShadow:   '0 4px 12px rgba(0,0,0,0.6)',
  }

  return ReactDOM.createPortal(
    <div style={style}>
      {/* Room name */}
      <div style={{ color: '#fff', fontWeight: 'bold', marginBottom: '0.25rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
        <span>{name}</span>
        {isCurrent && (
          <span style={{ fontSize: '0.65rem', color: '#4a8', border: '1px solid #4a8', borderRadius: '3px', padding: '0 3px' }}>
            current room
          </span>
        )}
      </div>

      {/* Danger level */}
      {danger && (
        <div style={{ marginBottom: '0.2rem' }}>
          <span style={{ color: '#666' }}>Danger: </span>
          <span style={{ color: dangerColor }}>{danger}</span>
        </div>
      )}

      {/* POIs */}
      {pois.length > 0 && (
        <div style={{ marginBottom: '0.2rem' }}>
          <div style={{ color: '#666', marginBottom: '0.1rem' }}>Points of Interest:</div>
          {pois.map(id => {
            const pt = POI_TYPES.find(p => p.id === id)
            return (
              <div key={id} style={{ paddingLeft: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                <span style={{ color: pt?.color ?? '#ccc' }}>{pt?.symbol ?? '?'}</span>
                <span>{pt?.label ?? id}</span>
              </div>
            )
          })}
        </div>
      )}

      {/* Exits */}
      {exits.length > 0 && (
        <div>
          <span style={{ color: '#666' }}>Exits: </span>
          {exits.map((e, i) => (
            <span key={e}>
              {i > 0 && <span style={{ color: '#555' }}>, </span>}
              <span style={{ color: '#aac' }}>{e}</span>
            </span>
          ))}
        </div>
      )}
    </div>,
    document.body,
  )
}
