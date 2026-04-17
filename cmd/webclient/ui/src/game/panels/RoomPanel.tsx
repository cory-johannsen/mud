import { useState } from 'react'
import { useGame } from '../GameContext'
import { RoomTooltip } from '../RoomTooltip'
import type { ExitInfo, MapTile } from '../../proto'

const TRADITION_TAG_LABELS: Record<string, string> = {
  technical:        'Technical',
  bio_synthetic:    'Biosynthetic',
  neural:           'Neural',
  fanatic_doctrine: 'Fanatic Doctrine',
}

function npcTypeTag(npcType: string, tradition?: string): string {
  switch (npcType) {
    case 'merchant':   return '[shop]'
    case 'healer':     return '[healer]'
    case 'banker':     return '[bank]'
    case 'job_trainer':  return '[trainer]'
    case 'tech_trainer': {
      const label = tradition ? TRADITION_TAG_LABELS[tradition] : undefined
      return label ? `[${label} tech trainer]` : '[tech trainer]'
    }
    case 'quest_giver':  return '[quest]'
    case 'guard':      return '[guard]'
    case 'fixer':      return '[fixer]'
    case 'hireling':     return '[hire]'
    case 'motel_keeper':          return '[motel]'
    case 'chip_doc':              return '[chip doc]'
    case 'crafter':               return '[crafter]'
    case 'brothel_keeper':        return '[brothel]'
    case 'black_market_merchant': return '[black market]'
    default:                      return ''
  }
}


export function RoomPanel() {
  const { state, sendMessage, sendCommand } = useGame()
  const room = state.roomView
  const inCombat = state.combatRound !== null
  const [tooltip, setTooltip] = useState<{ tile: MapTile; pos: { x: number; y: number } } | null>(null)

  function tileForExit(ex: ExitInfo): MapTile | null {
    if (ex.targetRoomId) {
      const found = state.mapTiles.find(t => t.roomId === ex.targetRoomId)
      if (found) return found
    }
    // Room not yet on map — build a minimal tile from the exit's title
    const title = ex.targetTitle
    if (title) return { roomName: title }
    return null
  }

  function handleExitEnter(ex: ExitInfo, e: React.MouseEvent) {
    sendCommand(`look ${ex.direction}`)
    const tile = tileForExit(ex)
    if (tile) setTooltip({ tile, pos: { x: e.clientX, y: e.clientY + 8 } })
  }

  function handleExitLeave() {
    setTooltip(null)
  }

  if (!room) {
    return <p className="room-loading" style={{ color: '#555', fontStyle: 'italic' }}>Connecting…</p>
  }

  // protojson may produce camelCase or snake_case — handle both
  const exits = room.exits ?? []
  const npcs = room.npcs ?? []
  const floorItems = room.floorItems ?? room.floor_items ?? []
  const equipment = room.equipment ?? []
  const players = room.players ?? []
  const title = room.title ?? ''
  const description = room.description ?? ''

  // Combat movement: 8-direction compass grid on the battle grid.
  // Enabled only when the step is within grid bounds, the target cell is unoccupied,
  // and the player has at least 1 AP remaining.
  const COMBAT_GRID_SIZE = 20
  const DIR_DELTAS: Record<string, [number, number]> = {
    nw: [-1, -1], n: [0, -1], ne: [1, -1],
    w:  [-1,  0],             e:  [1,  0],
    sw: [-1,  1], s: [0,  1], se: [1,  1],
  }
  const DIR_LABELS: Record<string, string> = {
    nw: 'NW', n: 'N', ne: 'NE',
    w:  'W',           e: 'E',
    sw: 'SW', s: 'S', se: 'SE',
  }
  const playerName = state.characterInfo?.name ?? ''
  const playerPos = state.combatPositions[playerName]
  const playerAP = state.combatantAP[playerName]
  const apRemaining = playerAP?.remaining ?? 0

  function canStepDir(dir: string): boolean {
    if (apRemaining < 1) return false
    if (!playerPos) return true // position unknown — allow attempt
    const [dx, dy] = DIR_DELTAS[dir]!
    const nx = playerPos.x + dx
    const ny = playerPos.y + dy
    if (nx < 0 || nx >= COMBAT_GRID_SIZE || ny < 0 || ny >= COMBAT_GRID_SIZE) return false
    // Disable if the target cell is occupied by another combatant
    return !Object.entries(state.combatPositions).some(
      ([name, p]) => name !== playerName && p.x === nx && p.y === ny
    )
  }

  // 3x3 compass layout: [row][col] = dir key or null for center
  const combatCompassGrid: (string | null)[][] = [
    ['nw', 'n',  'ne'],
    ['w',   null, 'e'],
    ['sw', 's',  'se'],
  ]

  return (
    <div>
      <h2 className="room-title">{title}</h2>
      <p className="room-description">{description}</p>

      {inCombat ? (
        <>
          <div className="room-section-label">Step (1 AP)</div>
          <div className="room-exits">
            {combatCompassGrid.map((row, ri) =>
              row.map((dir, ci) => {
                if (dir === null) {
                  // Center cell: show player position if known
                  const posLabel = playerPos ? `${playerPos.x},${playerPos.y}` : '·'
                  return (
                    <div key={`${ri}-${ci}`} className="exit-cell-center" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <span style={{ color: '#555', fontSize: '0.65rem', fontFamily: 'monospace' }}>{posLabel}</span>
                    </div>
                  )
                }
                const enabled = canStepDir(dir)
                return (
                  <button
                    key={`${ri}-${ci}`}
                    className="exit-btn"
                    onClick={() => sendMessage('StepRequest', { direction: dir })}
                    disabled={!enabled}
                    title={enabled ? `Step ${DIR_LABELS[dir]} (1 AP)` : apRemaining < 1 ? 'No AP remaining' : 'Cannot step — out of bounds or cell occupied'}
                  >
                    {DIR_LABELS[dir]}
                  </button>
                )
              })
            )}
          </div>
        </>
      ) : exits.length > 0 && (() => {
        const visibleExits = exits.filter((ex) => !ex.hidden)
        // Build a lookup map keyed by normalized (lowercase) direction
        const exitMap = new Map(visibleExits.map((ex) => [ex.direction.toLowerCase(), ex]))

        // 3x3 compass grid: [row][col] = normalized direction key
        const compassGrid: (string | null)[][] = [
          ['nw', 'n', 'ne'],
          ['w',  null, 'e'],
          ['sw', 's', 'se'],
        ]

        // Aliases: short forms map to canonical keys used in compassGrid
        const aliases: Record<string, string> = {
          northwest: 'nw',
          north:     'n',
          northeast: 'ne',
          west:      'w',
          east:      'e',
          southwest: 'sw',
          south:     's',
          southeast: 'se',
        }

        // Resolve exit for a compass key: check key directly then aliases
        function resolveExit(key: string) {
          if (exitMap.has(key)) return exitMap.get(key)!
          for (const [alias, canonical] of Object.entries(aliases)) {
            if (canonical === key && exitMap.has(alias)) return exitMap.get(alias)!
          }
          return null
        }

        // Resolve up/down exits for the center cell
        const upExit = exitMap.get('up') ?? exitMap.get('u') ?? null
        const downExit = exitMap.get('down') ?? exitMap.get('d') ?? null

        return (
          <>
            <div className="room-section-label">Exits</div>
            <div className="room-exits">
              {compassGrid.map((row, ri) =>
                row.map((key, ci) => {
                  if (key === null) {
                    // Center cell — split into Up (top) and Down (bottom)
                    return (
                      <div key={`${ri}-${ci}`} className="exit-cell-center">
                        {upExit ? (
                          <button
                            className="exit-btn exit-btn-vert"
                            onClick={() => { handleExitLeave(); sendMessage('MoveRequest', { direction: upExit.direction }) }}
                            onMouseEnter={(e) => handleExitEnter(upExit, e)}
                            onMouseLeave={handleExitLeave}
                          >
                            {upExit.locked ? '↑*' : '↑'}
                          </button>
                        ) : (
                          <div className="exit-half-empty" />
                        )}
                        {downExit ? (
                          <button
                            className="exit-btn exit-btn-vert"
                            onClick={() => { handleExitLeave(); sendMessage('MoveRequest', { direction: downExit.direction }) }}
                            onMouseEnter={(e) => handleExitEnter(downExit, e)}
                            onMouseLeave={handleExitLeave}
                          >
                            {downExit.locked ? '↓*' : '↓'}
                          </button>
                        ) : (
                          <div className="exit-half-empty" />
                        )}
                      </div>
                    )
                  }
                  const ex = resolveExit(key)
                  if (!ex) {
                    return <div key={`${ri}-${ci}`} className="exit-cell-empty" />
                  }
                  return (
                    <button
                      key={`${ri}-${ci}`}
                      className="exit-btn"
                      onClick={() => {
                        handleExitLeave()
                        sendMessage('MoveRequest', { direction: ex.direction })
                      }}
                      onMouseEnter={(e) => handleExitEnter(ex, e)}
                      onMouseLeave={handleExitLeave}
                    >
                      {ex.locked ? `${ex.direction}*` : ex.direction}
                    </button>
                  )
                })
              )}
            </div>
            {tooltip && <RoomTooltip tile={tooltip.tile} pos={tooltip.pos} />}
          </>
        )
      })()}

      {npcs.length > 0 && (
        <>
          <div className="room-section-label">NPCs</div>
          <ul className="room-npcs">
            {npcs.map((npc) => {
              const npcType = npc.npcType ?? npc.npc_type ?? ''
              const tag = npcTypeTag(npcType, npc.tradition)
              const fightingTarget = npc.fightingTarget ?? npc.fighting_target ?? ''
              const health = npc.healthDescription ?? npc.health_description ?? ''
              if (tag !== '') {
                // Non-combat NPC: Name [type], clickable
                const onClick = npcType === 'merchant'
                  ? () => sendMessage('BrowseRequest', { npc_name: npc.name })
                  : npcType === 'quest_giver'
                  ? () => sendMessage('TalkRequest', { npc_name: npc.name })
                  : () => sendMessage('ExamineRequest', { target: npc.name })
                return (
                  <li key={npc.id ?? npc.name}>
                    <button className="item-link" onClick={onClick}>
                      {npc.name}
                    </button>
                    {' '}
                    <span style={{ color: '#7bc' }}>{tag}</span>
                  </li>
                )
              }
              // Combat NPC: Name (health) [fighting Target] — clickable to attack
              return (
                <li key={npc.id ?? npc.name}>
                  <button className="item-link" onClick={() => sendCommand(`attack ${npc.name}`)}>
                    {npc.name}
                  </button>{' '}
                  <span style={{ color: '#666', fontSize: '0.75rem' }}>({health})</span>
                  {fightingTarget && <span style={{ color: '#f66' }}> fighting {fightingTarget}</span>}
                </li>
              )
            })}
          </ul>
        </>
      )}

      {equipment.length > 0 && (
        <>
          <div className="room-section-label">Equipment</div>
          <ul className="room-items">
            {equipment.map((eq, i) => {
              const coverTier = eq.coverTier ?? ''
              if (coverTier) {
                return (
                  <li key={i}>
                    <button
                      className="item-link"
                      onClick={() => sendMessage('TakeCoverRequest', {})}
                    >
                      {eq.name}
                    </button>
                    {' '}
                    <span style={{ color: '#7bc', fontSize: '0.75rem' }}>[{coverTier} cover]</span>
                  </li>
                )
              }
              return (
                <li key={i}>
                  {eq.usable ? (
                    <button
                      className="item-link"
                      onClick={() => sendMessage('UseEquipmentRequest', { instance_id: eq.instanceId })}
                    >
                      {eq.name} [interact]
                    </button>
                  ) : (
                    <span>{eq.name}</span>
                  )}
                </li>
              )
            })}
          </ul>
        </>
      )}

      {floorItems.length > 0 && (
        <>
          <div className="room-section-label">Items</div>
          <ul className="room-items">
            {floorItems.map((item, i) => (
              <li key={i}>
                <button
                  className="item-link"
                  onClick={() => sendMessage('GetItemRequest', { target: item.name })}
                >
                  {item.name}
                </button>
              </li>
            ))}
          </ul>
        </>
      )}

      {players.length > 0 && (
        <>
          <div className="room-section-label">Players</div>
          <ul className="room-players">
            {players.map((p, i) => <li key={i}>{typeof p === 'string' ? p : String(p)}</li>)}
          </ul>
        </>
      )}
    </div>
  )
}
