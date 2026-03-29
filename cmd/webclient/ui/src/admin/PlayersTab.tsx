import { useCallback, useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthContext'

interface Player {
  char_id: number
  account_id: number
  name: string
  level: number
  room_id: string
  zone: string
  current_hp: number
}

export function PlayersTab() {
  const { token } = useAuth()
  const [players, setPlayers] = useState<Player[]>([])
  const [error, setError] = useState<string | null>(null)
  const [teleportRoomIDs, setTeleportRoomIDs] = useState<Record<number, string>>({})
  const [messageTexts, setMessageTexts] = useState<Record<number, string>>({})
  const [actionStatus, setActionStatus] = useState<Record<number, string>>({})
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchPlayers = useCallback(async () => {
    try {
      const resp = await fetch('/api/admin/players', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const data: Player[] = await resp.json()
      setPlayers(data)
      setError(null)
    } catch (e) {
      setError(String(e))
    }
  }, [token])

  useEffect(() => {
    fetchPlayers()
    intervalRef.current = setInterval(fetchPlayers, 10_000)
    return () => {
      if (intervalRef.current !== null) clearInterval(intervalRef.current)
    }
  }, [fetchPlayers])

  const kick = async (charID: number) => {
    if (!window.confirm(`Kick player ${charID}?`)) return
    const resp = await fetch(`/api/admin/players/${charID}/kick`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
    })
    setActionStatus(prev => ({ ...prev, [charID]: resp.ok ? 'Kicked' : `Error ${resp.status}` }))
    fetchPlayers()
  }

  const teleport = async (charID: number) => {
    const roomID = teleportRoomIDs[charID] ?? ''
    if (!roomID.trim()) return
    const resp = await fetch(`/api/admin/players/${charID}/teleport`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ room_id: roomID }),
    })
    setActionStatus(prev => ({ ...prev, [charID]: resp.ok ? 'Teleported' : `Error ${resp.status}` }))
  }

  const sendMessage = async (charID: number) => {
    const text = messageTexts[charID] ?? ''
    if (!text.trim()) return
    const resp = await fetch(`/api/admin/players/${charID}/message`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ text }),
    })
    setActionStatus(prev => ({ ...prev, [charID]: resp.ok ? 'Sent' : `Error ${resp.status}` }))
  }

  return (
    <div style={{ padding: '1rem' }}>
      <h2 style={{ color: '#ccc' }}>Online Players</h2>
      {error && <p style={{ color: '#f55' }}>{error}</p>}
      {players.length === 0 ? (
        <p style={{ color: '#999' }}>No players online.</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', color: '#ccc' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #555' }}>
              {['Name', 'Level', 'Zone', 'Room ID', 'HP', 'Account ID', 'Actions'].map(h => (
                <th key={h} style={{ textAlign: 'left', padding: '0.4rem 0.6rem' }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {players.map(p => (
              <tr key={p.char_id} style={{ borderBottom: '1px solid #333' }}>
                <td style={{ padding: '0.4rem 0.6rem' }}>{p.name}</td>
                <td style={{ padding: '0.4rem 0.6rem' }}>{p.level}</td>
                <td style={{ padding: '0.4rem 0.6rem' }}>{p.zone}</td>
                <td style={{ padding: '0.4rem 0.6rem', fontFamily: 'monospace', fontSize: '0.8em' }}>{p.room_id}</td>
                <td style={{ padding: '0.4rem 0.6rem' }}>{p.current_hp}</td>
                <td style={{ padding: '0.4rem 0.6rem' }}>{p.account_id}</td>
                <td style={{ padding: '0.4rem 0.6rem' }}>
                  <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap', alignItems: 'center' }}>
                    <button onClick={() => kick(p.char_id)} style={btnStyle('#a33')}>Kick</button>
                    <input
                      placeholder="room_id"
                      value={teleportRoomIDs[p.char_id] ?? ''}
                      onChange={e => setTeleportRoomIDs(prev => ({ ...prev, [p.char_id]: e.target.value }))}
                      style={inputStyle}
                    />
                    <button onClick={() => teleport(p.char_id)} style={btnStyle('#335')}>Teleport</button>
                    <input
                      placeholder="message"
                      value={messageTexts[p.char_id] ?? ''}
                      onChange={e => setMessageTexts(prev => ({ ...prev, [p.char_id]: e.target.value }))}
                      style={inputStyle}
                    />
                    <button onClick={() => sendMessage(p.char_id)} style={btnStyle('#353')}>Msg</button>
                    {actionStatus[p.char_id] && (
                      <span style={{ color: '#9c9', fontSize: '0.8em' }}>{actionStatus[p.char_id]}</span>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

const btnStyle = (bg: string): React.CSSProperties => ({
  background: bg,
  color: '#eee',
  border: 'none',
  padding: '0.2rem 0.5rem',
  borderRadius: '3px',
  cursor: 'pointer',
  fontSize: '0.85em',
})

const inputStyle: React.CSSProperties = {
  background: '#222',
  color: '#ccc',
  border: '1px solid #555',
  borderRadius: '3px',
  padding: '0.2rem 0.4rem',
  fontSize: '0.85em',
  width: '100px',
}
