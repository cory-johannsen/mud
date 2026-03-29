import { useCallback, useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthContext'

interface ZoneSummary {
  id: string
  name: string
  danger_level: string
  room_count: number
}

interface RoomSummary {
  id: string
  title: string
  description: string
  danger_level: string
}

export function ZoneEditorTab() {
  const { token } = useAuth()
  const [zones, setZones] = useState<ZoneSummary[]>([])
  const [selectedZone, setSelectedZone] = useState<string | null>(null)
  const [rooms, setRooms] = useState<RoomSummary[]>([])
  const [expandedRoom, setExpandedRoom] = useState<string | null>(null)
  const [roomEdits, setRoomEdits] = useState<Record<string, RoomSummary>>({})
  const [saveStatus, setSaveStatus] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)

  const authHeader = { Authorization: `Bearer ${token}` }

  const fetchZones = useCallback(async () => {
    try {
      const resp = await fetch('/api/admin/zones', { headers: authHeader })
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const data: ZoneSummary[] = await resp.json()
      setZones(data)
    } catch (e) {
      setError(String(e))
    }
  }, [token])

  useEffect(() => { fetchZones() }, [fetchZones])

  const fetchRooms = useCallback(async (zoneID: string) => {
    try {
      const resp = await fetch(`/api/admin/zones/${encodeURIComponent(zoneID)}/rooms`, { headers: authHeader })
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const data: RoomSummary[] = await resp.json()
      setRooms(data)
    } catch (e) {
      setError(String(e))
    }
  }, [token])

  const selectZone = (id: string) => {
    setSelectedZone(id)
    setExpandedRoom(null)
    fetchRooms(id)
  }

  const startEditRoom = (r: RoomSummary) => {
    setRoomEdits(prev => ({ ...prev, [r.id]: { ...r } }))
    setExpandedRoom(r.id)
  }

  const saveRoom = async (roomID: string) => {
    const ed = roomEdits[roomID]
    if (!ed) return
    const resp = await fetch(`/api/admin/rooms/${encodeURIComponent(roomID)}`, {
      method: 'PUT',
      headers: { ...authHeader, 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: ed.title, description: ed.description, danger_level: ed.danger_level }),
    })
    if (resp.ok) {
      setSaveStatus(prev => ({ ...prev, [roomID]: 'Saved' }))
      if (selectedZone) fetchRooms(selectedZone)
    } else {
      setSaveStatus(prev => ({ ...prev, [roomID]: `Error ${resp.status}` }))
    }
  }

  return (
    <div style={{ padding: '1rem', display: 'flex', gap: '1.5rem' }}>
      {/* Left: zone list */}
      <div style={{ width: '200px', flexShrink: 0 }}>
        <h3 style={{ color: '#ccc', marginTop: 0 }}>Zones</h3>
        {error && <p style={{ color: '#f55', fontSize: '0.85em' }}>{error}</p>}
        <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
          {zones.map(z => (
            <li key={z.id}>
              <button
                onClick={() => selectZone(z.id)}
                style={{
                  ...zoneBtnStyle,
                  background: selectedZone === z.id ? '#2a3a4a' : '#1e1e1e',
                  color: '#ccc',
                }}
              >
                {z.name}
                <br />
                <span style={{ fontSize: '0.75em', color: '#888' }}>
                  {z.danger_level} · {z.room_count} rooms
                </span>
              </button>
            </li>
          ))}
        </ul>
      </div>

      {/* Right: rooms */}
      <div style={{ flex: 1 }}>
        {selectedZone ? (
          <>
            <h3 style={{ color: '#ccc', marginTop: 0 }}>
              Rooms in {zones.find(z => z.id === selectedZone)?.name ?? selectedZone}
            </h3>
            {rooms.length === 0 && <p style={{ color: '#999' }}>No rooms found.</p>}
            {rooms.map(r => {
              const ed = roomEdits[r.id]
              const isOpen = expandedRoom === r.id
              return (
                <div key={r.id} style={{ marginBottom: '0.5rem', border: '1px solid #444', borderRadius: '4px' }}>
                  <div
                    onClick={() => isOpen ? setExpandedRoom(null) : startEditRoom(r)}
                    style={{ padding: '0.5rem 0.8rem', cursor: 'pointer', color: '#ccc', display: 'flex', justifyContent: 'space-between' }}
                  >
                    <span>{r.title}</span>
                    <span style={{ fontSize: '0.8em', color: '#888' }}>{r.danger_level} · {r.id}</span>
                  </div>
                  {isOpen && ed && (
                    <div style={{ padding: '0.8rem', background: '#181818', borderTop: '1px solid #333' }}>
                      <label style={labelStyle}>Title</label>
                      <input
                        value={ed.title}
                        onChange={e => setRoomEdits(prev => ({ ...prev, [r.id]: { ...prev[r.id], title: e.target.value } }))}
                        style={{ ...fieldStyle, width: '100%' }}
                      />
                      <label style={labelStyle}>Description</label>
                      <textarea
                        value={ed.description}
                        onChange={e => setRoomEdits(prev => ({ ...prev, [r.id]: { ...prev[r.id], description: e.target.value } }))}
                        rows={4}
                        style={{ ...fieldStyle, width: '100%', resize: 'vertical' }}
                      />
                      <label style={labelStyle}>Danger Level</label>
                      <input
                        value={ed.danger_level}
                        onChange={e => setRoomEdits(prev => ({ ...prev, [r.id]: { ...prev[r.id], danger_level: e.target.value } }))}
                        style={{ ...fieldStyle, width: '120px' }}
                      />
                      <div style={{ marginTop: '0.6rem' }}>
                        <button onClick={() => saveRoom(r.id)} style={saveBtnStyle}>Save</button>
                        {saveStatus[r.id] && (
                          <span style={{ marginLeft: '0.6rem', color: '#9c9', fontSize: '0.85em' }}>{saveStatus[r.id]}</span>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
          </>
        ) : (
          <p style={{ color: '#888' }}>Select a zone to view rooms.</p>
        )}
      </div>
    </div>
  )
}

const zoneBtnStyle: React.CSSProperties = {
  width: '100%',
  textAlign: 'left',
  border: 'none',
  borderRadius: '4px',
  padding: '0.5rem 0.7rem',
  cursor: 'pointer',
  marginBottom: '0.3rem',
}

const labelStyle: React.CSSProperties = {
  display: 'block',
  color: '#999',
  fontSize: '0.8em',
  marginBottom: '0.2rem',
  marginTop: '0.6rem',
}

const fieldStyle: React.CSSProperties = {
  background: '#222',
  color: '#ccc',
  border: '1px solid #555',
  borderRadius: '3px',
  padding: '0.3rem 0.5rem',
  fontSize: '0.9em',
}

const saveBtnStyle: React.CSSProperties = {
  background: '#353',
  color: '#eee',
  border: 'none',
  padding: '0.3rem 0.8rem',
  borderRadius: '3px',
  cursor: 'pointer',
}
