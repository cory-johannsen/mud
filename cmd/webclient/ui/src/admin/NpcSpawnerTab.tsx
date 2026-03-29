import { useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthContext'

interface NPCTemplate {
  id: string
  name: string
  level: number
  type: string
}

export function NpcSpawnerTab() {
  const { token } = useAuth()
  const [templates, setTemplates] = useState<NPCTemplate[]>([])
  const [selectedNPC, setSelectedNPC] = useState('')
  const [count, setCount] = useState(1)
  const [roomID, setRoomID] = useState('')
  const [loading, setLoading] = useState(false)
  const [status, setStatus] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const authHeader = { Authorization: `Bearer ${token}` }

  useEffect(() => {
    fetch('/api/admin/npcs', { headers: authHeader })
      .then(r => r.json())
      .then((data: NPCTemplate[]) => {
        setTemplates(data)
        if (data.length > 0) setSelectedNPC(data[0].id)
      })
      .catch(e => setError(String(e)))
  }, [token])

  const spawn = async () => {
    if (!selectedNPC || count < 1 || !roomID.trim()) {
      setStatus('Please fill all fields.')
      return
    }
    setLoading(true)
    setStatus(null)
    try {
      const resp = await fetch(`/api/admin/rooms/${encodeURIComponent(roomID)}/spawn-npc`, {
        method: 'POST',
        headers: { ...authHeader, 'Content-Type': 'application/json' },
        body: JSON.stringify({ npc_id: selectedNPC, count, room_id: roomID }),
      })
      const body = await resp.json()
      if (resp.ok) {
        setStatus(`Spawned ${count}x ${selectedNPC} in ${roomID}`)
      } else {
        setStatus(`Error: ${body.error ?? resp.status}`)
      }
    } catch (e) {
      setStatus(String(e))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ padding: '1rem' }}>
      <h2 style={{ color: '#ccc' }}>NPC Spawner</h2>
      {error && <p style={{ color: '#f55' }}>{error}</p>}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem', maxWidth: '360px' }}>
        <label style={labelStyle}>
          NPC Template
          <select
            value={selectedNPC}
            onChange={e => setSelectedNPC(e.target.value)}
            style={selectStyle}
          >
            {templates.length === 0 ? (
              <option value="">No templates available</option>
            ) : (
              templates.map(t => (
                <option key={t.id} value={t.id}>
                  {t.name} (Lvl {t.level}{t.type ? ` · ${t.type}` : ''})
                </option>
              ))
            )}
          </select>
        </label>
        <label style={labelStyle}>
          Count
          <input
            type="number"
            min={1}
            value={count}
            onChange={e => setCount(Math.max(1, parseInt(e.target.value) || 1))}
            style={inputStyle}
          />
        </label>
        <label style={labelStyle}>
          Room ID
          <input
            placeholder="e.g. dungeon:entry"
            value={roomID}
            onChange={e => setRoomID(e.target.value)}
            style={inputStyle}
          />
        </label>
        <button
          onClick={spawn}
          disabled={loading}
          style={{
            background: loading ? '#555' : '#353',
            color: '#eee',
            border: 'none',
            padding: '0.5rem 1.2rem',
            borderRadius: '4px',
            cursor: loading ? 'not-allowed' : 'pointer',
            fontSize: '1em',
          }}
        >
          {loading ? 'Spawning…' : 'Spawn'}
        </button>
        {status && <p style={{ color: '#9c9', fontSize: '0.9em' }}>{status}</p>}
      </div>
    </div>
  )
}

const labelStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: '0.3rem',
  color: '#ccc',
  fontSize: '0.9em',
}

const inputStyle: React.CSSProperties = {
  background: '#222',
  color: '#ccc',
  border: '1px solid #555',
  borderRadius: '3px',
  padding: '0.35rem 0.5rem',
  fontSize: '0.9em',
}

const selectStyle: React.CSSProperties = {
  background: '#222',
  color: '#ccc',
  border: '1px solid #555',
  borderRadius: '3px',
  padding: '0.35rem 0.5rem',
  fontSize: '0.9em',
}
