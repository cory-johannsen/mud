import { useGame } from '../GameContext'

function npcTypeTag(npcType: string): string {
  switch (npcType) {
    case 'merchant':   return '[shop]'
    case 'healer':     return '[healer]'
    case 'banker':     return '[bank]'
    case 'job_trainer': return '[trainer]'
    case 'quest_giver': return '[quest]'
    case 'guard':      return '[guard]'
    case 'fixer':      return '[fixer]'
    case 'hireling':   return '[hire]'
    default:           return ''
  }
}

export function RoomPanel() {
  const { state, sendMessage } = useGame()
  const room = state.roomView

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

      {exits.length > 0 && (
        <>
          <div className="room-section-label">Exits</div>
          <div className="room-exits">
            {exits
              .filter((ex) => !ex.hidden)
              .map((ex) => (
                <button
                  key={ex.direction}
                  className="exit-btn"
                  onClick={() => sendMessage('MoveRequest', { direction: ex.direction })}
                >
                  {ex.locked ? `${ex.direction}*` : ex.direction}
                </button>
              ))}
          </div>
        </>
      )}

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
              // Combat NPC: Name (health) [fighting Target]
              return (
                <li key={npc.id ?? npc.name}>
                  {npc.name}{' '}
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
            {equipment.map((eq, i) => (
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
            ))}
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
