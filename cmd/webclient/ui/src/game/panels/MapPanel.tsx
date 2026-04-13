import { useEffect, useState, useCallback } from 'react'
import { Panel, Group as PanelGroup, Separator as PanelResizeHandle } from 'react-resizable-panels'
import { useGame } from '../GameContext'
import { renderMapTiles } from '../mapRenderer'
import type { ColoredLine } from '../mapRenderer'
import type { WorldZoneTile } from '../../proto'
import type { MapTile } from '../../proto'
import { RoomTooltip } from '../RoomTooltip'

interface HoverHandlers {
  onMouseEnter: (tile: MapTile, e: React.MouseEvent) => void
  onMouseLeave: () => void
}

function renderLines(lines: ColoredLine[], hover?: HoverHandlers): JSX.Element {
  return (
    <>
      {lines.map((line, i) => (
        <span key={i}>
          {line.map((seg, j) => {
            if (seg.tile && hover) {
              const tile = seg.tile
              return (
                <span
                  key={j}
                  style={{ color: seg.color, cursor: 'default' }}
                  data-room={tile.roomId ?? ''}
                  onMouseEnter={e => hover.onMouseEnter(tile, e)}
                  onMouseLeave={hover.onMouseLeave}
                >
                  {seg.text}
                </span>
              )
            }
            return seg.color
              ? <span key={j} style={{ color: seg.color }}>{seg.text}</span>
              : <span key={j}>{seg.text}</span>
          })}
          {i < lines.length - 1 ? '\n' : ''}
        </span>
      ))}
    </>
  )
}

const COMPASS_DIRS = [
  ['nw', 'n', 'ne'],
  ['w',  '',  'e'],
  ['sw', 's', 'se'],
] as const

function DPad({ onDir, disabledDirs, disabled }: {
  onDir: (dir: string) => void
  disabledDirs: Set<string>
  disabled: boolean
}): JSX.Element {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 28px)', gap: '2px' }}>
      {COMPASS_DIRS.flat().map((dir, i) => (
        dir === '' ? (
          <button key={i} disabled={disabled}
            style={{ width: 28, height: 28, background: '#1a3a6b', border: '1px solid #3a5a9b', borderRadius: 3, color: '#7bb8ff', fontSize: '0.75rem', cursor: disabled ? 'not-allowed' : 'pointer' }}
            onClick={() => onDir('toward')} title="Stride toward nearest enemy">⊕</button>
        ) : (
          <button key={i} disabled={disabled || disabledDirs.has(dir)}
            style={{ width: 28, height: 28, background: disabled || disabledDirs.has(dir) ? '#111' : '#1a2a1a', border: '1px solid #333', borderRadius: 3, color: disabled || disabledDirs.has(dir) ? '#444' : '#8d4', fontSize: '0.7rem', cursor: disabled || disabledDirs.has(dir) ? 'not-allowed' : 'pointer' }}
            onClick={() => onDir(dir)} title={dir.toUpperCase()}>{dir.toUpperCase()}</button>
        )
      ))}
    </div>
  )
}

function ApPips({ remaining, total }: { remaining: number; total: number }): JSX.Element {
  return (
    <div style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
      {Array.from({ length: total }).map((_, i) => (
        <div key={i} style={{ width: 10, height: 10, borderRadius: '50%', background: i < remaining ? '#f0c040' : 'transparent', border: '2px solid #f0c040' }} />
      ))}
    </div>
  )
}

function renderBattleGrid(
  combatPositions: Record<string, { x: number; y: number }>,
  playerName: string,
  gridWidth: number,
  gridHeight: number,
  onHover: (name: string, pos: { x: number; y: number }, e: React.MouseEvent) => void,
  onHoverEnd: () => void,
): JSX.Element {
  const rawCell = Math.floor(320 / Math.max(gridWidth, gridHeight))
  const CELL_PX = Math.max(12, Math.min(32, rawCell))

  const occupants: Record<string, string> = {}
  for (const [name, pos] of Object.entries(combatPositions)) {
    occupants[`${pos.x},${pos.y}`] = name
  }

  const cells: JSX.Element[] = []
  for (let y = 0; y < gridHeight; y++) {
    for (let x = 0; x < gridWidth; x++) {
      const name = occupants[`${x},${y}`] ?? ''
      const isPlayer = name === playerName
      const isEnemy = name !== '' && !isPlayer
      const bg = isPlayer ? '#1a3a6b' : isEnemy ? '#6b1a1a' : '#1a1a2e'
      const token = name ? name[0].toUpperCase() : ''
      cells.push(
        <div
          key={`${x},${y}`}
          style={{
            width: CELL_PX,
            height: CELL_PX,
            background: bg,
            border: '1px solid #333',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '0.75rem',
            color: isPlayer ? '#7bb8ff' : isEnemy ? '#ff7b7b' : '#555',
            fontWeight: 'bold',
            cursor: 'default',
            flexShrink: 0,
          }}
          onMouseEnter={name !== '' ? e => onHover(name, { x, y }, e) : undefined}
          onMouseLeave={name !== '' ? onHoverEnd : undefined}
        >
          {token}
        </div>
      )
    }
  }

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: `repeat(${gridWidth}, ${CELL_PX}px)`,
        gap: 0,
        border: '1px solid #555',
      }}
    >
      {cells}
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
  const { state, sendMessage, sendCommand } = useGame()
  const [showWorld, setShowWorld] = useState(false)
  const [hoveredTile, setHoveredTile] = useState<MapTile | null>(null)
  const [tooltipPos, setTooltipPos] = useState({ x: 0, y: 0 })
  const [stepMode, setStepMode] = useState(false)
  const [combatHoverName, setCombatHoverName] = useState<string | null>(null)
  const [combatHoverPos, setCombatHoverPos] = useState({ x: 0, y: 0 })

  const handleRoomEnter = useCallback((tile: MapTile, e: React.MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    setTooltipPos({ x: rect.left, y: rect.bottom })
    setHoveredTile(tile)
  }, [])

  const handleRoomLeave = useCallback(() => {
    setHoveredTile(null)
  }, [])

  const hoverHandlers: HoverHandlers = {
    onMouseEnter: handleRoomEnter,
    onMouseLeave: handleRoomLeave,
  }

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
    if (state.connected && state.combatRound === null) {
      sendMessage('MapRequest', { view: 'zone' })
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.connected, state.combatRound])

  const inCombat = state.combatRound !== null

  if (inCombat) {
    const gridWidth = state.combatGridWidth
    const gridHeight = state.combatGridHeight
    const playerName = state.characterInfo?.name ?? ''
    const playerPos = state.combatPositions[playerName]
    const playerAP = state.combatantAP[playerName]
    const apRemaining = playerAP?.remaining ?? 0
    const apTotal = playerAP?.total ?? 3
    const apDisabled = apRemaining === 0

    const disabledDirs = new Set<string>()
    if (playerPos) {
      if (playerPos.y === 0)               { disabledDirs.add('n'); disabledDirs.add('nw'); disabledDirs.add('ne') }
      if (playerPos.y === gridHeight - 1)  { disabledDirs.add('s'); disabledDirs.add('sw'); disabledDirs.add('se') }
      if (playerPos.x === 0)               { disabledDirs.add('w'); disabledDirs.add('nw'); disabledDirs.add('sw') }
      if (playerPos.x === gridWidth - 1)   { disabledDirs.add('e'); disabledDirs.add('ne'); disabledDirs.add('se') }
    }

    return (
      <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <div className="map-header">
          <h3>Battle Map</h3>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <ApPips remaining={apRemaining} total={apTotal} />
            <label style={{ fontSize: '0.75rem', color: '#aaa', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <input type="checkbox" checked={stepMode} onChange={e => setStepMode(e.target.checked)} /> Step
            </label>
            <button className="map-refresh-btn" style={{ background: '#2a1a1a', borderColor: '#7a2a2a', color: '#f66' }} onClick={() => sendCommand('flee')}>Flee!</button>
          </div>
        </div>
        <div style={{ display: 'flex', gap: '0.75rem', padding: '0.5rem', overflow: 'auto', flex: 1 }}>
          <div style={{ overflow: 'auto', flexShrink: 0, position: 'relative' }}>
            {renderBattleGrid(state.combatPositions, playerName, gridWidth, gridHeight,
              (name, _pos, e) => {
                const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
                setCombatHoverPos({ x: rect.left, y: rect.bottom })
                setCombatHoverName(name)
              },
              () => setCombatHoverName(null)
            )}
            {combatHoverName && (
              <RoomTooltip
                tile={null}
                pos={combatHoverPos}
                overrideText={`${combatHoverName} — AP: ${state.combatantAP[combatHoverName]?.remaining ?? '?'}/${state.combatantAP[combatHoverName]?.total ?? '?'}`}
              />
            )}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', justifyContent: 'center' }}>
            <DPad onDir={dir => sendCommand(`${stepMode ? 'step' : 'stride'} ${dir}`)} disabledDirs={disabledDirs} disabled={apDisabled} />
          </div>
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
        <PanelGroup orientation="horizontal" style={{ flex: 1, minHeight: 0 }}>
          <Panel defaultSize={60} minSize={20} style={{ overflow: 'auto', position: 'relative' }}>
            <pre className="map-ascii" style={{ margin: 0 }}>
              {renderLines(gridLines, hoverHandlers)}
            </pre>
            {hoveredTile && (
              <RoomTooltip tile={hoveredTile} pos={tooltipPos} />
            )}
          </Panel>
          <PanelResizeHandle className="resize-handle resize-handle-h" />
          <Panel defaultSize={40} minSize={20} style={{ overflow: 'auto' }}>
            <pre className="map-ascii" style={{ margin: 0 }}>
              {renderLines(legendLines)}
            </pre>
          </Panel>
        </PanelGroup>
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
