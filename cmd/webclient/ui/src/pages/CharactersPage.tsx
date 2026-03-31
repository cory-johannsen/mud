import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Character, ApiError } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { CharacterWizard } from './CharacterWizard'

export function CharactersPage() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [characters, setCharacters] = useState<Character[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showWizard, setShowWizard] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const list = await api.characters.list()
      setCharacters(list)
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        logout()
        navigate('/login', { replace: true })
      } else {
        setError('Failed to load characters.')
      }
    } finally {
      setLoading(false)
    }
  }, [logout, navigate])

  useEffect(() => { void load() }, [load])

  async function handlePlay(char: Character) {
    try {
      const resp = await api.characters.play(char.id)
      localStorage.setItem('mud_token', resp.token)
      navigate('/game')
    } catch {
      setError('Failed to start game session.')
    }
  }

  async function handleDelete(char: Character) {
    if (!window.confirm(`Delete "${char.name}"? This cannot be undone.`)) return
    try {
      await api.characters.delete(char.id)
      await load()
    } catch {
      setError(`Failed to delete ${char.name}.`)
    }
  }

  if (showWizard) {
    return (
      <CharacterWizard
        onComplete={() => { setShowWizard(false); void load() }}
        onCancel={() => setShowWizard(false)}
      />
    )
  }

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>Your Characters</h1>
        <div style={styles.headerActions}>
          <button style={styles.newButton} onClick={() => setShowWizard(true)}>
            + Create New
          </button>
          <button style={styles.logoutButton} onClick={logout}>
            Logout
          </button>
        </div>
      </header>

      {loading && <p style={styles.status}>Loading…</p>}
      {error && <p style={styles.error}>{error}</p>}

      {!loading && !error && characters.length === 0 && (
        <p style={styles.status}>
          No characters yet.{' '}
          <button style={styles.inlineLink} onClick={() => setShowWizard(true)}>
            Create one!
          </button>
        </p>
      )}

      <div style={styles.grid}>
        {characters.map((char) => (
          <CharacterCard key={char.id} char={char} onPlay={handlePlay} onDelete={handleDelete} />
        ))}
      </div>
    </div>
  )
}

function CharacterCard({ char, onPlay, onDelete }: { char: Character; onPlay: (c: Character) => void; onDelete: (c: Character) => void }) {
  const hpPct = char.max_hp > 0 ? (char.current_hp / char.max_hp) * 100 : 0
  const hpColor = hpPct > 50 ? '#4caf50' : hpPct > 25 ? '#ff9800' : '#f44336'

  return (
    <div style={styles.card}>
      <div style={styles.cardName}>{char.name}</div>
      <div style={styles.cardSub}>
        Level {char.level} {char.job} ({char.archetype})
      </div>
      <div style={styles.cardSub}>{char.region}</div>
      {char.location && (
        <div style={styles.cardSub}>
          Location: {char.location.replace(/_/g, ' ')}
        </div>
      )}
      <div style={styles.hpBar}>
        <div style={{ ...styles.hpFill, width: `${hpPct}%`, background: hpColor }} />
      </div>
      <div style={styles.hpText}>
        {char.current_hp} / {char.max_hp} HP
      </div>
      <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem' }}>
        <button style={{ ...styles.playButton, margin: 0, flex: 1 }} onClick={() => onPlay(char)}>
          Play
        </button>
        <button style={styles.deleteButton} onClick={() => onDelete(char)}>
          Delete
        </button>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    background: '#0d0d0d',
    color: '#ccc',
    fontFamily: 'monospace',
    padding: '2rem',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '2rem',
  },
  title: { margin: 0, color: '#e0c060', fontSize: '1.5rem' },
  headerActions: { display: 'flex', gap: '0.75rem' },
  newButton: {
    padding: '0.5rem 1rem',
    background: '#e0c060',
    color: '#111',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontWeight: 'bold',
  },
  logoutButton: {
    padding: '0.5rem 1rem',
    background: 'none',
    color: '#888',
    border: '1px solid #444',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
  },
  status: { color: '#888', textAlign: 'center' as const },
  error: { color: '#f55', textAlign: 'center' as const },
  inlineLink: {
    background: 'none',
    border: 'none',
    color: '#e0c060',
    cursor: 'pointer',
    fontFamily: 'monospace',
    textDecoration: 'underline',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
    gap: '1rem',
  },
  card: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '8px',
    padding: '1.25rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.4rem',
  },
  cardName: { fontSize: '1.1rem', color: '#eee', fontWeight: 'bold' },
  cardSub: { fontSize: '0.8rem', color: '#888' },
  hpBar: { background: '#333', borderRadius: '4px', height: '6px', overflow: 'hidden', marginTop: '0.5rem' },
  hpFill: { height: '100%', borderRadius: '4px', transition: 'width 0.3s' },
  hpText: { fontSize: '0.75rem', color: '#aaa' },
  playButton: {
    marginTop: '0.75rem',
    padding: '0.4rem',
    background: '#2a4a2a',
    color: '#7f7',
    border: '1px solid #4a8a4a',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontWeight: 'bold',
  },
  deleteButton: {
    padding: '0.4rem 0.6rem',
    background: 'none',
    color: '#f55',
    border: '1px solid #7a2a2a',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.8rem',
  },
}
