import { type FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import { useAuth } from '../auth/AuthContext'

type Tab = 'login' | 'register'

const AK47 = `                         ▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▓▓▓▓▓▓▓▓φ▄▄▄▄▄▄▄▄,,,,,_        ╒█▌
             ___    __,╫╫╫╫╫╫╫╫╫╫╫╫╣▓▓▄╠▄╠╠╠▓╬╣▓▓▓▓█▓██▓███▓▓█▓▓▓▓▓█▓▄______▌▄▌_
  ▄▄▓▓▓▓███████▓█▓▓█╬╬▓▌▒▒▒╠╢█▀▀▀╬╬╬╬Ñ╠▓╬╬╬╬╬╬╫▓▓▓▓▓███████▓█▓▓▓▓▓▓███▓▓▓▓▓███▀▀
  ╫█████▓███▓╬▓▓▓▓██▓▓▀▀▓╬╬╬╬▓M▓╙╙╟▓▓▓▓███▌└└└└╙▀╙"
  ▐█▓█▓▓▓▓▓▓▓██▓▀▀╙     ▓▓╬╣▓▓_)__▐╙╜█▓███▓
   █████▓▀▀╙           ╓█▓╬█▀        ╫▓████▌
   ╙"                 ┌█▓▓█▀          ██████▓_
                      ╙▀███            ▓██████▌_
                                        ╙███████▓
                                          ▀████▓
                                            ╙▀"`

const GUNCHETE_TITLE = `  ██████╗ ██╗   ██╗███╗   ██╗ ██████╗██╗  ██╗███████╗████████╗███████╗
 ██╔════╝ ██║   ██║████╗  ██║██╔════╝██║  ██║██╔════╝╚══██╔══╝██╔════╝
 ██║  ███╗██║   ██║██╔██╗ ██║██║     ███████║█████╗     ██║   █████╗
 ██║   ██║██║   ██║██║╚██╗██║██║     ██╔══██║██╔══╝     ██║   ██╔══╝
 ╚██████╔╝╚██████╔╝██║ ╚████║╚██████╗██║  ██║███████╗   ██║   ███████╗
  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚══════╝`

const MACHETE = `╖_
▌██▓▄╥_
▌╠▓▓▒███▓▄▄_
╟'███╣███████▓▄▄,
▓╙███╣███████████▓▓▄╖_
▀▄███▌▀███████████████▓▌▄,_
 'W╬▓▓▓▓▓▀██████████████████▓▄▄,_
   \`▀╬╠▀╟█████████████████████████▓▄▄,
       ╙¥▄▄╙▀▀▀███▓▓▓▓▓▓████████████████▓▄▄,
           \`╙╙M¥≡╗▄╠▀▀▀▓█▓▓▓▄▓▀▀██████████████▓▄▄,_
                     └"╙ª%≡╫╬▌▀▀▀▓▓▓▓▓▓▓████████▓▓██▐N▄▄╓_
                              \`└╙ª¥W╣▄╠▀▀▀▓▓▓▓▓█▓╫█▌▓╫██▀▓▓▓#▄▄_
                                        └"╙M╝╣▄▓▓██▐Ñ█▓█▓▓███▌Ñ▓▓▓▓▄▄,
                                                 '█▌▌"  ╙▀▓█▓█▓████▀▓▓▓⌐
                                                 ╒█Å        └╙▀▀▀▀█▓▓██▌
                                                 ╙▀               ▐███▀`

function validateUsername(u: string): string | null {
  if (!/^[a-zA-Z0-9_]{3,20}$/.test(u)) {
    return 'Username must be 3–20 alphanumeric characters or underscores.'
  }
  return null
}

function validatePassword(p: string): string | null {
  if (p.length < 8) {
    return 'Password must be at least 8 characters.'
  }
  return null
}

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [tab, setTab] = useState<Tab>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)

    const userErr = validateUsername(username)
    if (userErr) { setError(userErr); return }

    const passErr = validatePassword(password)
    if (passErr) { setError(passErr); return }

    setSubmitting(true)
    try {
      const resp =
        tab === 'login'
          ? await api.auth.login(username, password)
          : await api.auth.register(username, password)
      login(resp.token)
      navigate('/characters', { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Unexpected error. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div style={styles.container}>
      <div style={styles.splash}>
        <pre style={styles.ak47}>{AK47}</pre>
        <pre style={styles.title}>{GUNCHETE_TITLE}</pre>
        <pre style={styles.machete}>{MACHETE}</pre>
        <p style={styles.subtitle}>Post-Collapse Portland, OR — A Dystopian Sci-Fi MUD</p>
      </div>

      <div style={styles.card}>
        <div style={styles.tabs}>
          <button
            style={{ ...styles.tab, ...(tab === 'login' ? styles.tabActive : {}) }}
            onClick={() => { setTab('login'); setError(null) }}
            type="button"
          >
            Login
          </button>
          <button
            style={{ ...styles.tab, ...(tab === 'register' ? styles.tabActive : {}) }}
            onClick={() => { setTab('register'); setError(null) }}
            type="button"
          >
            Register
          </button>
        </div>

        <form onSubmit={handleSubmit} style={styles.form}>
          <label style={styles.label}>
            Username
            <input
              style={styles.input}
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
              required
            />
          </label>

          <label style={styles.label}>
            Password
            <input
              style={styles.input}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={tab === 'login' ? 'current-password' : 'new-password'}
              required
            />
          </label>

          {error && <p style={styles.error}>{error}</p>}

          <button style={styles.submit} type="submit" disabled={submitting}>
            {submitting ? 'Please wait…' : tab === 'login' ? 'Login' : 'Create Account'}
          </button>
        </form>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    background: '#0d0d0d',
    fontFamily: 'monospace',
    padding: '1rem',
    gap: '1.5rem',
    overflowX: 'hidden',
  },
  splash: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    width: '100%',
    maxWidth: '800px',
  },
  ak47: {
    margin: 0,
    color: '#00cc44',
    fontSize: '0.65rem',
    lineHeight: 1.2,
    whiteSpace: 'pre',
    alignSelf: 'flex-start',
  },
  title: {
    margin: '0.5rem 0',
    color: '#00ccff',
    fontWeight: 'bold',
    fontSize: '0.65rem',
    lineHeight: 1.2,
    whiteSpace: 'pre',
  },
  machete: {
    margin: 0,
    color: '#ccaa00',
    fontSize: '0.65rem',
    lineHeight: 1.2,
    whiteSpace: 'pre',
    alignSelf: 'flex-end',
  },
  subtitle: {
    margin: '0.5rem 0 0',
    color: '#ccaa00',
    fontSize: '0.85rem',
    letterSpacing: '0.05em',
    textAlign: 'center',
  },
  card: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '8px',
    padding: '2rem',
    width: '100%',
    maxWidth: '360px',
    color: '#ccc',
  },
  tabs: {
    display: 'flex',
    marginBottom: '1.5rem',
    borderBottom: '1px solid #333',
  },
  tab: {
    flex: 1,
    padding: '0.5rem',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    color: '#888',
    fontSize: '0.9rem',
  },
  tabActive: {
    color: '#e0c060',
    borderBottom: '2px solid #e0c060',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
  },
  label: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
    fontSize: '0.85rem',
    color: '#aaa',
  },
  input: {
    padding: '0.5rem',
    background: '#111',
    border: '1px solid #444',
    borderRadius: '4px',
    color: '#eee',
    fontSize: '1rem',
    fontFamily: 'monospace',
  },
  error: {
    color: '#f55',
    fontSize: '0.85rem',
    margin: 0,
  },
  submit: {
    padding: '0.6rem',
    background: '#e0c060',
    color: '#111',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontWeight: 'bold',
    fontSize: '1rem',
    fontFamily: 'monospace',
    marginTop: '0.5rem',
  },
}
