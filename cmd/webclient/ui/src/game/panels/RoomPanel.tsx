import { useState } from 'react'
import { useGame } from '../GameContext'
import { RoomTooltip } from '../RoomTooltip'
import type { ExitInfo, MapTile } from '../../proto'

function npcTypeTag(npcType: string): string {
  switch (npcType) {
    case 'merchant':   return '[shop]'
    case 'healer':     return '[healer]'
    case 'banker':     return '[bank]'
    case 'job_trainer': return '[trainer]'
    case 'quest_giver': return '[quest]'
    case 'guard':      return '[guard]'
    case 'fixer':      return '[fixer]'
    case 'hireling':     return '[hire]'
    case 'motel_keeper': return '[motel]'
    default:             return ''
  }
}

export function RoomPanel() {
  const { state, sendMessage, sendCommand } = useGame()
  const room = state.roomView
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

  return (
    <div>
      <h2 className="room-title">{title}</h2>
      <p className="room-description">{description}</p>

      {exits.length > 0 && (() => {
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
                            onClick={() => sendMessage('MoveRequest', { direction: upExit.direction })}
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
                            onClick={() => sendMessage('MoveRequest', { direction: downExit.direction })}
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
                      onClick={() => sendMessage('MoveRequest', { direction: ex.direction })}
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
              const tag = npcTypeTag(npcType)
              const fightingTarget = npc.fightingTarget ?? npc.fighting_target ?? ''
              const health = npc.healthDescription ?? npc.health_description ?? ''
              if (tag !== '') {
                // Non-combat NPC: Name [type], clickable
                const onClick = npcType === 'merchant'
                  ? () => sendMessage('BrowseRequest', { npc_name: npc.name })
                  : npcType === 'motel_keeper'
                  ? () => sendMessage('RestRequest', {})
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
