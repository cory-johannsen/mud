import { useCallback, useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthContext'

interface Account {
  id: number
  username: string
  role: string
  banned: boolean
}

const VALID_ROLES = ['player', 'editor', 'moderator', 'admin']

export function AccountsTab() {
  const { token } = useAuth()
  const [query, setQuery] = useState('')
  const [accounts, setAccounts] = useState<Account[]>([])
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<Record<number, { role: string; banned: boolean }>>({})
  const [saveStatus, setSaveStatus] = useState<Record<number, string>>({})
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const search = useCallback(
    async (q: string) => {
      try {
        const resp = await fetch(`/api/admin/accounts?q=${encodeURIComponent(q)}`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
        const data: Account[] = await resp.json()
        setAccounts(data)
        setError(null)
      } catch (e) {
        setError(String(e))
      }
    },
    [token],
  )

  useEffect(() => {
    search('')
  }, [search])

  const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const q = e.target.value
    setQuery(q)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => search(q), 300)
  }

  const startEdit = (a: Account) => {
    setEditing(prev => ({ ...prev, [a.id]: { role: a.role, banned: a.banned } }))
  }

  const save = async (id: number) => {
    const e = editing[id]
    if (!e) return
    const resp = await fetch(`/api/admin/accounts/${id}`, {
      method: 'PUT',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ role: e.role, banned: e.banned }),
    })
    if (resp.ok) {
      setSaveStatus(prev => ({ ...prev, [id]: 'Saved' }))
      setEditing(prev => { const n = { ...prev }; delete n[id]; return n })
      search(query)
    } else {
      setSaveStatus(prev => ({ ...prev, [id]: `Error ${resp.status}` }))
    }
  }

  return (
    <div style={{ padding: '1rem' }}>
      <h2 style={{ color: '#ccc' }}>Accounts</h2>
      <input
        placeholder="Search username…"
        value={query}
        onChange={handleQueryChange}
        style={{ ...inputStyle, width: '260px', marginBottom: '1rem' }}
      />
      {error && <p style={{ color: '#f55' }}>{error}</p>}
      <table style={{ width: '100%', borderCollapse: 'collapse', color: '#ccc' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #555' }}>
            {['ID', 'Username', 'Role', 'Banned', 'Actions'].map(h => (
              <th key={h} style={{ textAlign: 'left', padding: '0.4rem 0.6rem' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {accounts.map(a => {
            const ed = editing[a.id]
            const rowStyle: React.CSSProperties = a.banned
              ? { borderBottom: '1px solid #333', background: '#2a1111', textDecoration: 'line-through' }
              : { borderBottom: '1px solid #333' }
            return (
              <tr key={a.id} style={rowStyle}>
                <td style={{ padding: '0.4rem 0.6rem' }}>{a.id}</td>
                <td style={{ padding: '0.4rem 0.6rem' }}>{a.username}</td>
                <td style={{ padding: '0.4rem 0.6rem' }}>
                  {ed ? (
                    <select
                      value={ed.role}
                      onChange={e => setEditing(prev => ({ ...prev, [a.id]: { ...prev[a.id], role: e.target.value } }))}
                      style={selectStyle}
                    >
                      {VALID_ROLES.map(r => <option key={r} value={r}>{r}</option>)}
                    </select>
                  ) : a.role}
                </td>
                <td style={{ padding: '0.4rem 0.6rem' }}>
                  {ed ? (
                    <input
                      type="checkbox"
                      checked={ed.banned}
                      onChange={e => setEditing(prev => ({ ...prev, [a.id]: { ...prev[a.id], banned: e.target.checked } }))}
                    />
                  ) : (a.banned ? 'Yes' : 'No')}
                </td>
                <td style={{ padding: '0.4rem 0.6rem' }}>
                  {ed ? (
                    <>
                      <button onClick={() => save(a.id)} style={btnStyle('#353')}>Save</button>
                      <button
                        onClick={() => setEditing(prev => { const n = { ...prev }; delete n[a.id]; return n })}
                        style={{ ...btnStyle('#444'), marginLeft: '0.4rem' }}
                      >Cancel</button>
                    </>
                  ) : (
                    <button onClick={() => startEdit(a)} style={btnStyle('#335')}>Edit</button>
                  )}
                  {saveStatus[a.id] && (
                    <span style={{ color: '#9c9', fontSize: '0.8em', marginLeft: '0.4rem' }}>{saveStatus[a.id]}</span>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
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
  padding: '0.3rem 0.5rem',
  fontSize: '0.9em',
}

const selectStyle: React.CSSProperties = {
  background: '#222',
  color: '#ccc',
  border: '1px solid #555',
  borderRadius: '3px',
  padding: '0.2rem 0.4rem',
}
