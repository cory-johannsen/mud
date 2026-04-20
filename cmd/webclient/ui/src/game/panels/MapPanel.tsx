import { useEffect, useRef, useState, useCallback } from 'react'
import { useGame } from '../GameContext'
import type { MapTile, CoverObjectPosition } from '../../proto'
import { RoomTooltip } from '../RoomTooltip'
import { ZoneMapSvg, REFERENCE_W } from '../ZoneMapSvg'
import { WorldMapSvg } from '../WorldMapSvg'

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

// Returns 0 (not reachable), 1 (1-move range), or 2 (2-move range)
export function cellMoveCost(
  playerX: number, playerY: number,
  targetX: number, targetY: number,
  strideCells: number,
  movementRemaining: number,
): 0 | 1 | 2 {
  const dx = Math.abs(targetX - playerX)
  const dy = Math.abs(targetY - playerY)
  const dist = Math.max(dx, dy) // Chebyshev distance
  if (dist === 0) return 0 // can't move to own cell
  if (dist <= strideCells && movementRemaining >= 1) return 1
  if (dist <= 2 * strideCells && movementRemaining >= 2) return 2
  return 0
}

const COVER_TIER_COLORS: Record<string, string> = {
  lesser: '#3a2a10',
  standard: '#4a3218',
  greater: '#5c3e1e',
}

function renderBattleGrid(
  combatPositions: Record<string, { x: number; y: number }>,
  playerName: string,
  gridWidth: number,
  gridHeight: number,
  onHover: (name: string, pos: { x: number; y: number }, e: React.MouseEvent) => void,
  onHoverEnd: () => void,
  hoveredCell: { x: number; y: number } | null,
  onCellHover: (x: number, y: number) => void,
  onCellHoverEnd: () => void,
  onCellClick: (x: number, y: number) => void,
  onEnemyClick: (name: string) => void,
  playerX: number,
  playerY: number,
  strideCells: number,
  movementRemaining: number,
  coverObjects: CoverObjectPosition[],
): JSX.Element {
  const rawCell = Math.floor(320 / Math.max(gridWidth, gridHeight))
  const CELL_PX = Math.max(12, Math.min(32, rawCell))

  const occupants: Record<string, string> = {}
  for (const [name, pos] of Object.entries(combatPositions)) {
    occupants[`${pos.x},${pos.y}`] = name
  }

  const coverMap: Record<string, CoverObjectPosition> = {}
  for (const co of coverObjects) {
    const cx = co.x ?? 0
    const cy = co.y ?? 0
    coverMap[`${cx},${cy}`] = co
  }

  const cells: JSX.Element[] = []
  for (let y = 0; y < gridHeight; y++) {
    for (let x = 0; x < gridWidth; x++) {
      const name = occupants[`${x},${y}`] ?? ''
      const isPlayer = name === playerName
      const isEnemy = name !== '' && !isPlayer
      const cover = coverMap[`${x},${y}`]
      const isCover = cover !== undefined && name === ''

      const moveCost = (name === '' && !isCover && playerX >= 0)
        ? cellMoveCost(playerX, playerY, x, y, strideCells, movementRemaining)
        : 0

      let bg: string
      if (isPlayer) {
        bg = '#1a3a6b'
      } else if (isEnemy) {
        bg = '#6b1a1a'
      } else if (isCover) {
        const tier = cover.coverTier ?? cover.cover_tier ?? 'lesser'
        bg = COVER_TIER_COLORS[tier] ?? COVER_TIER_COLORS.lesser
      } else if (moveCost === 1) {
        bg = '#0d3b0d'
      } else if (moveCost === 2) {
        bg = '#2e2600'
      } else {
        bg = '#1a1a2e'
      }

      const isHovered = hoveredCell?.x === x && hoveredCell?.y === y
      const border = isHovered && moveCost > 0
        ? `2px solid ${moveCost === 1 ? '#4a8a4a' : '#8a7a00'}`
        : '1px solid #333'

      const cursor = moveCost > 0 || isEnemy ? 'pointer' : 'default'

      let token: string
      if (name) {
        token = name[0].toUpperCase()
      } else if (isCover) {
        const tier = cover.coverTier ?? cover.cover_tier ?? 'lesser'
        token = tier === 'greater' ? '▓' : tier === 'standard' ? '▒' : '░'
      } else {
        token = ''
      }

      const coverTitle = isCover
        ? `${cover.name ?? cover.itemId ?? cover.item_id ?? 'Cover'} (${cover.coverTier ?? cover.cover_tier ?? 'lesser'})`
        : undefined

      cells.push(
        <div
          key={`${x},${y}`}
          title={coverTitle}
          style={{
            width: CELL_PX,
            height: CELL_PX,
            background: bg,
            border,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '0.75rem',
            color: isPlayer ? '#7bb8ff' : isEnemy ? '#ff7b7b' : isCover ? '#c8a060' : '#555',
            fontWeight: 'bold',
            cursor,
            flexShrink: 0,
          }}
          onMouseEnter={e => {
            onCellHover(x, y)
            if (name !== '') {
              onHover(name, { x, y }, e)
            }
          }}
          onMouseLeave={() => {
            onCellHoverEnd()
            if (name !== '') {
              onHoverEnd()
            }
          }}
          onClick={() => {
            if (isEnemy) onEnemyClick(name)
            else if (moveCost > 0) onCellClick(x, y)
          }}
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

export function MapPanel() {
  const { state, sendMessage, sendCommand } = useGame()
  const [showWorld, setShowWorld] = useState(false)
  const [hoveredTile, setHoveredTile] = useState<MapTile | null>(null)
  const [tooltipPos, setTooltipPos] = useState({ x: 0, y: 0 })
  const [stepMode, setStepMode] = useState(false)
  const [combatHoverName, setCombatHoverName] = useState<string | null>(null)
  const [combatHoverPos, setCombatHoverPos] = useState({ x: 0, y: 0 })
  const [hoveredCell, setHoveredCell] = useState<{ x: number; y: number } | null>(null)
  const mapContainerRef = useRef<HTMLDivElement>(null)
  const [mapContainerW, setMapContainerW] = useState(REFERENCE_W)
  const prevRoomIdRef = useRef<string | null>(null)

  useEffect(() => {
    const el = mapContainerRef.current
    if (!el) return
    const ro = new ResizeObserver(entries => {
      const w = entries[0]?.contentRect.width ?? REFERENCE_W
      setMapContainerW(w)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  // REQ-MAP-TRAVEL-1: Auto-switch back to zone map when travel completes.
  // When the room ID changes while the world map is showing, the player has
  // fast-travelled to a new zone — switch back to zone view automatically.
  // showWorld is included in deps so the closure always sees the current value;
  // omitting it causes a stale closure where showWorld reads as false even after
  // the user has switched to world view.
  const currentRoomId = state.roomView?.roomId ?? null
  useEffect(() => {
    if (showWorld && currentRoomId !== null && currentRoomId !== prevRoomIdRef.current) {
      setShowWorld(false)
      sendMessage('MapRequest', { view: 'zone' })
    }
    prevRoomIdRef.current = currentRoomId
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentRoomId, showWorld])

  const handleRoomEnter = useCallback((tile: MapTile, e: React.MouseEvent) => {
    const rect = (e.currentTarget as Element).getBoundingClientRect()
    setTooltipPos({ x: rect.left, y: rect.bottom })
    setHoveredTile(tile)
  }, [])

  const handleRoomLeave = useCallback(() => {
    setHoveredTile(null)
  }, [])

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

    const speedPenalty = (state.characterSheet?.speedPenalty ?? (state.characterSheet as any)?.speed_penalty) ?? 0
    const effectiveSpeedFt = Math.max(5, 25 - speedPenalty)
    const strideCells = Math.floor(effectiveSpeedFt / 5)
    const movementRemaining = state.combatantAP[playerName]?.movementRemaining ?? 2

    function handleCellClick(x: number, y: number) {
      sendMessage('MoveToRequest', { target_x: x, target_y: y })
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
            {renderBattleGrid(
              state.combatPositions,
              playerName,
              gridWidth,
              gridHeight,
              (name, _pos, e) => {
                const rect = (e.currentTarget as Element).getBoundingClientRect()
                setCombatHoverPos({ x: rect.left, y: rect.bottom })
                setCombatHoverName(name)
              },
              () => setCombatHoverName(null),
              hoveredCell,
              (x, y) => setHoveredCell({ x, y }),
              () => setHoveredCell(null),
              handleCellClick,
              (name) => sendMessage('ExamineRequest', { target: name }),
              playerPos?.x ?? -1,
              playerPos?.y ?? -1,
              strideCells,
              movementRemaining,
              state.combatRound?.coverObjects ?? state.combatRound?.cover_objects ?? [],
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

  const exploreMode = state.characterSheet?.exploreMode ?? state.characterSheet?.explore_mode ?? ''
  const EXPLORE_MODE_LABELS: Record<string, string> = {
    lay_low: 'Lay Low',
    hold_ground: 'Hold Ground',
    active_sensors: 'Active Sensors',
    case_it: 'Case It',
    run_point: 'Run Point',
    shadow: 'Shadow',
    poke_around: 'Poke Around',
  }
  const exploreModeLabel = exploreMode ? (EXPLORE_MODE_LABELS[exploreMode] ?? exploreMode) : ''

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div className="map-header">
        <h3>{showWorld ? 'World Map' : 'Zone Map'}</h3>
        {!showWorld && exploreModeLabel && (
          <span style={{ color: '#8d4', fontSize: '0.72rem', fontFamily: 'monospace', fontWeight: 600, flex: 1, textAlign: 'center' }}>
            ◆ {exploreModeLabel}
          </span>
        )}
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
        <WorldMapSvg
          tiles={state.worldTiles}
          onTravel={handleTravel}
          playerLevel={state.characterSheet?.level ?? 0}
        />
      ) : state.mapTiles.length === 0 ? (
        <p className="map-empty">No map data.</p>
      ) : (
        <div ref={mapContainerRef} style={{ flex: 1, minHeight: 0, position: 'relative' }}>
          <ZoneMapSvg
            tiles={state.mapTiles}
            containerWidth={mapContainerW}
            onHover={handleRoomEnter}
            onHoverEnd={handleRoomLeave}
            playerLevel={state.characterSheet?.level ?? 0}
            zoneLevelRange={
              (state.worldTiles.find(t => !!t.current)?.levelRange)
              ?? (state.worldTiles.find(t => !!t.current)?.level_range)
            }
          />
          {hoveredTile && (
            <RoomTooltip tile={hoveredTile} pos={tooltipPos} />
          )}
        </div>
      )}
    </div>
  )
}
