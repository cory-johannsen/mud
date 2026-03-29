import { useGame } from '../GameContext'

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
            {npcs.map((npc) => (
              <li key={npc.id ?? npc.name}>
                {npc.name}{npc.fightingTarget ?? npc.fighting_target ? ' ⚔' : ''}{' '}
                <span style={{ color: '#666', fontSize: '0.75rem' }}>
                  ({npc.healthDescription ?? npc.health_description ?? ''})
                </span>
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
