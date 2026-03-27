# Web Client Phase 3: React SPA Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Vite/React/TypeScript SPA with auth flow, character list, and multi-step character creation wizard.

**Architecture:** React 18 SPA with react-router-dom, JWT auth context, typed API client, and @bufbuild/protobuf for proto types.

**Tech Stack:** React 18, TypeScript, Vite, react-router-dom v6, @bufbuild/protobuf, @connectrpc/connect, jwt-decode, buf CLI

**Assumes:** Phase 1 (Go backend HTTP server on port 8080) and Phase 2 (Auth + Character APIs) are complete and passing tests.

---

## Requirements Covered

- REQ-WC-21: React app in `cmd/webclient/ui/`, Vite + React 18 + TypeScript, `make ui-build`
- REQ-WC-22: TypeScript proto types generated from `api/proto/game/v1/game.proto` via `@bufbuild/protobuf`, `make proto-ts`
- REQ-WC-23: Three top-level routes (`/login`, `/characters`, `/game`, `/admin`); protected routes redirect to `/login`
- REQ-WC-33: Multi-step character creation wizard (region → job → archetype → name/gender)
- REQ-WC-34: Live stats preview sidebar updates with each wizard step selection
- REQ-WC-42: `package.json` with required deps; `vite.config.ts`; `tsconfig.json`
- REQ-WC-43: Vite dev server (port 5173) proxies `/api/*` and `/ws` to Go server (port 8080)
- REQ-WC-44: `make ui-install` and `make ui-build` targets; both prerequisites of `make build`
- REQ-WC-45: `.gitignore` excludes `cmd/webclient/ui/node_modules/` and `cmd/webclient/ui/dist/`

---

## Task 1: Vite Project Scaffold

**Files to create:**
- `cmd/webclient/ui/package.json`
- `cmd/webclient/ui/vite.config.ts`
- `cmd/webclient/ui/tsconfig.json`
- `cmd/webclient/ui/index.html`
- `cmd/webclient/ui/src/main.tsx`

### Steps

- [ ] 1.1 Create `cmd/webclient/ui/package.json`:

```json
{
  "name": "mud-web-client",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.26.2",
    "@bufbuild/protobuf": "^2.2.2",
    "@connectrpc/connect": "^2.0.0",
    "jwt-decode": "^4.0.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.3",
    "typescript": "^5.6.3",
    "vite": "^5.4.10"
  }
}
```

- [ ] 1.2 Create `cmd/webclient/ui/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
```

- [ ] 1.3 Create `cmd/webclient/ui/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] 1.4 Create `cmd/webclient/ui/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] 1.5 Create `cmd/webclient/ui/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>MUD Web Client</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] 1.6 Create `cmd/webclient/ui/src/main.tsx`:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'

const rootElement = document.getElementById('root')
if (!rootElement) {
  throw new Error('Root element #root not found in document')
}

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] 1.7 Create `cmd/webclient/ui/src/App.tsx` (router skeleton — full routes wired in subsequent tasks):

```tsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { ProtectedRoute } from './auth/ProtectedRoute'
import { LoginPage } from './pages/LoginPage'
import { CharactersPage } from './pages/CharactersPage'

// GamePage and AdminPage are stubs completed in Phase 4 and Phase 5 respectively.
function GamePageStub() {
  return <div style={{ color: '#ccc', padding: '2rem' }}>Game view — Phase 4</div>
}
function AdminPageStub() {
  return <div style={{ color: '#ccc', padding: '2rem' }}>Admin dashboard — Phase 5</div>
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/characters"
            element={
              <ProtectedRoute>
                <CharactersPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/game"
            element={
              <ProtectedRoute>
                <GamePageStub />
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin"
            element={
              <ProtectedRoute requiredRole="admin">
                <AdminPageStub />
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
```

- [ ] 1.8 Verify scaffold type-checks cleanly:
  ```
  cd cmd/webclient/ui
  npm install
  npm run build
  ```
  Expected: `✓ built in N.Nms` with no TypeScript errors. The build will fail on missing imports until Tasks 2–3 are complete; run `tsc --noEmit` to type-check without bundling after each task.

---

## Task 2: Auth Context and Protected Route

**Files to create:**
- `cmd/webclient/ui/src/auth/AuthContext.tsx`
- `cmd/webclient/ui/src/auth/ProtectedRoute.tsx`

### Steps

- [ ] 2.1 Create `cmd/webclient/ui/src/auth/AuthContext.tsx`:

```tsx
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { jwtDecode } from 'jwt-decode'

const TOKEN_KEY = 'mud_token'

interface JwtClaims {
  account_id: number
  role: string
  exp: number
}

interface AuthUser {
  accountId: number
  role: string
  exp: number
}

interface AuthContextValue {
  user: AuthUser | null
  token: string | null
  login: (token: string) => void
  logout: () => void
}

function decodeToken(token: string): AuthUser | null {
  try {
    const claims = jwtDecode<JwtClaims>(token)
    if (claims.exp * 1000 < Date.now()) {
      return null
    }
    return { accountId: claims.account_id, role: claims.role, exp: claims.exp }
  } catch {
    return null
  }
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY))
  const [user, setUser] = useState<AuthUser | null>(() => {
    const stored = localStorage.getItem(TOKEN_KEY)
    return stored ? decodeToken(stored) : null
  })

  // Evict expired token on mount and when token changes.
  useEffect(() => {
    if (token) {
      const decoded = decodeToken(token)
      if (!decoded) {
        localStorage.removeItem(TOKEN_KEY)
        setToken(null)
        setUser(null)
      } else {
        setUser(decoded)
      }
    }
  }, [token])

  const login = useCallback((newToken: string) => {
    localStorage.setItem(TOKEN_KEY, newToken)
    setToken(newToken)
    setUser(decodeToken(newToken))
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({ user, token, login, logout }),
    [user, token, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return ctx
}
```

- [ ] 2.2 Create `cmd/webclient/ui/src/auth/ProtectedRoute.tsx`:

```tsx
import { type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'

interface ProtectedRouteProps {
  children: ReactNode
  /** If set, the user's role must equal this value; otherwise HTTP 403 redirect. */
  requiredRole?: string
}

export function ProtectedRoute({ children, requiredRole }: ProtectedRouteProps) {
  const { user } = useAuth()

  if (!user) {
    return <Navigate to="/login" replace />
  }

  if (requiredRole && user.role !== requiredRole && user.role !== 'admin') {
    // Non-admins without the required role are redirected to /characters.
    return <Navigate to="/characters" replace />
  }

  return <>{children}</>
}
```

- [ ] 2.3 Type-check:
  ```
  cd cmd/webclient/ui && npx tsc --noEmit
  ```
  Expected: no errors for `src/auth/`.

---

## Task 3: Login / Register Page

**Files to create:**
- `cmd/webclient/ui/src/pages/LoginPage.tsx`
- `cmd/webclient/ui/src/api/client.ts` (auth endpoints only; extended in Task 5)

### Steps

- [ ] 3.1 Create `cmd/webclient/ui/src/api/client.ts`:

```typescript
const BASE = ''  // Requests are relative; Vite proxy handles /api/* in dev.

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

function getToken(): string | null {
  return localStorage.getItem('mud_token')
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  auth = true,
): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (auth) {
    const token = getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
  }

  const resp = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (!resp.ok) {
    let message = resp.statusText
    try {
      const json = (await resp.json()) as { error?: string }
      if (json.error) {
        message = json.error
      }
    } catch {
      // ignore parse failure; use statusText
    }
    throw new ApiError(resp.status, message)
  }

  return resp.json() as Promise<T>
}

export interface AuthResponse {
  token: string
  account_id: number
  role: string
}

export interface CharacterOption {
  id: string
  name: string
  description: string
}

export interface CharacterOptions {
  regions: CharacterOption[]
  jobs: CharacterOption[]
  archetypes: CharacterOption[]
  starting_stats: Record<string, Record<string, number>>
}

export interface Character {
  id: number
  name: string
  job: string
  level: number
  current_hp: number
  max_hp: number
  region: string
  archetype: string
}

export interface CreateCharacterPayload {
  name: string
  job: string
  archetype: string
  region: string
  gender: string
}

export const api = {
  auth: {
    login(username: string, password: string): Promise<AuthResponse> {
      return request<AuthResponse>('POST', '/api/auth/login', { username, password }, false)
    },
    register(username: string, password: string): Promise<AuthResponse> {
      return request<AuthResponse>('POST', '/api/auth/register', { username, password }, false)
    },
  },
  characters: {
    list(): Promise<Character[]> {
      return request<Character[]>('GET', '/api/characters')
    },
    create(payload: CreateCharacterPayload): Promise<{ character: Character }> {
      return request<{ character: Character }>('POST', '/api/characters', payload)
    },
    options(): Promise<CharacterOptions> {
      return request<CharacterOptions>('GET', '/api/characters/options')
    },
    checkName(name: string): Promise<{ available: boolean }> {
      return request<{ available: boolean }>(
        'GET',
        `/api/characters/check-name?name=${encodeURIComponent(name)}`,
      )
    },
  },
}
```

- [ ] 3.2 Create `cmd/webclient/ui/src/pages/LoginPage.tsx`:

```tsx
import { type FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import { useAuth } from '../auth/AuthContext'

type Tab = 'login' | 'register'

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
      <div style={styles.card}>
        <h1 style={styles.title}>MUD</h1>

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
    alignItems: 'center',
    justifyContent: 'center',
    background: '#0d0d0d',
    fontFamily: 'monospace',
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
  title: {
    margin: '0 0 1.5rem',
    textAlign: 'center',
    color: '#e0c060',
    fontSize: '2rem',
    letterSpacing: '0.2em',
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
```

- [ ] 3.3 Type-check:
  ```
  cd cmd/webclient/ui && npx tsc --noEmit
  ```
  Expected: no errors in `src/api/` or `src/pages/LoginPage.tsx`.

---

## Task 4: Proto TypeScript Type Generation

**Files to create:**
- `cmd/webclient/ui/buf.gen.yaml`
- `cmd/webclient/buf.yaml` (buf workspace root)

### Steps

- [ ] 4.1 Verify `buf` CLI is available: `buf --version`. If missing, install via `npm install -g @bufbuild/buf` or download binary from `https://buf.build/docs/installation`.

- [ ] 4.2 Create `cmd/webclient/buf.yaml`:

```yaml
version: v2
modules:
  - path: ../../api/proto
    name: buf.build/local/mud
```

- [ ] 4.3 Create `cmd/webclient/ui/buf.gen.yaml`:

```yaml
version: v2
inputs:
  - directory: ../../../api/proto
plugins:
  - remote: buf.build/bufbuild/es:v2.2.2
    out: src/proto
    opt:
      - target=ts
```

  Note: `buf.build/bufbuild/es` generates `@bufbuild/protobuf`-compatible TypeScript. Requires `npm install` to have run first (it uses the local `@bufbuild/protobuf` package).

- [ ] 4.4 Create the output directory:
  ```
  mkdir -p cmd/webclient/ui/src/proto
  ```

- [ ] 4.5 Run generation from `cmd/webclient/ui/`:
  ```
  cd cmd/webclient/ui && npx buf generate --config buf.gen.yaml
  ```
  Expected: creates `src/proto/game/v1/game_pb.ts` (and any dependency files).

- [ ] 4.6 Verify the generated file exists and type-checks:
  ```
  ls cmd/webclient/ui/src/proto/game/v1/
  cd cmd/webclient/ui && npx tsc --noEmit
  ```
  Expected: `game_pb.ts` present; no TypeScript errors.

- [ ] 4.7 Create `cmd/webclient/ui/src/proto/index.ts` re-exporting the generated types for clean imports:

```typescript
// Re-export all generated proto types for convenient imports.
// Generated by: make proto-ts (buf generate)
export * from './game/v1/game_pb'
```

---

## Task 5: Characters List Page

**Files to create/update:**
- `cmd/webclient/ui/src/pages/CharactersPage.tsx`

### Steps

- [ ] 5.1 Create `cmd/webclient/ui/src/pages/CharactersPage.tsx`:

```tsx
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

  function handlePlay(char: Character) {
    // Store character_id for WebSocket JWT; Phase 4 will wire the game session.
    localStorage.setItem('mud_character_id', String(char.id))
    navigate('/game')
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
          <CharacterCard key={char.id} char={char} onPlay={handlePlay} />
        ))}
      </div>
    </div>
  )
}

function CharacterCard({ char, onPlay }: { char: Character; onPlay: (c: Character) => void }) {
  const hpPct = char.max_hp > 0 ? (char.current_hp / char.max_hp) * 100 : 0
  const hpColor = hpPct > 50 ? '#4caf50' : hpPct > 25 ? '#ff9800' : '#f44336'

  return (
    <div style={styles.card}>
      <div style={styles.cardName}>{char.name}</div>
      <div style={styles.cardSub}>
        Level {char.level} {char.job} ({char.archetype})
      </div>
      <div style={styles.cardSub}>{char.region}</div>
      <div style={styles.hpBar}>
        <div style={{ ...styles.hpFill, width: `${hpPct}%`, background: hpColor }} />
      </div>
      <div style={styles.hpText}>
        {char.current_hp} / {char.max_hp} HP
      </div>
      <button style={styles.playButton} onClick={() => onPlay(char)}>
        Play
      </button>
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
}
```

- [ ] 5.2 Type-check:
  ```
  cd cmd/webclient/ui && npx tsc --noEmit
  ```
  Expected: no errors in `src/pages/CharactersPage.tsx`.

---

## Task 6: Character Creation Wizard

**Files to create:**
- `cmd/webclient/ui/src/pages/CharacterWizard.tsx`

### Steps

- [ ] 6.1 Create `cmd/webclient/ui/src/pages/CharacterWizard.tsx` — multi-step form with live stats preview.

The wizard tracks state `{ region, job, archetype, name, gender }` across 4 steps. A right-side sidebar previews computed starting stats from the `GET /api/characters/options` payload. Name availability is checked via a 300 ms debounce.

```tsx
import {
  type ChangeEvent,
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react'
import { api, type CharacterOptions, type CharacterOption, ApiError } from '../api/client'

interface WizardState {
  region: string
  job: string
  archetype: string
  name: string
  gender: string
}

const EMPTY_STATE: WizardState = { region: '', job: '', archetype: '', name: '', gender: '' }
const STEPS = ['Region', 'Job', 'Archetype', 'Name & Gender'] as const
type StepIndex = 0 | 1 | 2 | 3

interface Props {
  onComplete: () => void
  onCancel: () => void
}

export function CharacterWizard({ onComplete, onCancel }: Props) {
  const [step, setStep] = useState<StepIndex>(0)
  const [state, setState] = useState<WizardState>(EMPTY_STATE)
  const [options, setOptions] = useState<CharacterOptions | null>(null)
  const [optionsError, setOptionsError] = useState<string | null>(null)
  const [nameAvailable, setNameAvailable] = useState<boolean | null>(null)
  const [nameChecking, setNameChecking] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Load character options on mount.
  useEffect(() => {
    api.characters.options()
      .then((opts) => setOptions(opts))
      .catch(() => setOptionsError('Failed to load character options.'))
  }, [])

  // Debounced name availability check.
  useEffect(() => {
    if (step !== 3 || state.name.length < 3) {
      setNameAvailable(null)
      return
    }
    if (debounceRef.current) clearTimeout(debounceRef.current)
    setNameChecking(true)
    debounceRef.current = setTimeout(async () => {
      try {
        const res = await api.characters.checkName(state.name)
        setNameAvailable(res.available)
      } catch {
        setNameAvailable(null)
      } finally {
        setNameChecking(false)
      }
    }, 300)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [state.name, step])

  const update = useCallback((patch: Partial<WizardState>) => {
    setState((prev) => ({ ...prev, ...patch }))
  }, [])

  function canAdvance(): boolean {
    if (step === 0) return state.region !== ''
    if (step === 1) return state.job !== ''
    if (step === 2) return state.archetype !== ''
    return (
      state.name.length >= 3 &&
      state.name.length <= 20 &&
      state.gender !== '' &&
      nameAvailable === true
    )
  }

  function handleNext() {
    if (step < 3) setStep((s) => (s + 1) as StepIndex)
  }

  function handleBack() {
    if (step > 0) setStep((s) => (s - 1) as StepIndex)
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!canAdvance()) return
    setSubmitError(null)
    setSubmitting(true)
    try {
      await api.characters.create({
        name: state.name,
        job: state.job,
        archetype: state.archetype,
        region: state.region,
        gender: state.gender,
      })
      onComplete()
    } catch (err) {
      if (err instanceof ApiError) {
        setSubmitError(err.message)
      } else {
        setSubmitError('Unexpected error. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (optionsError) {
    return (
      <div style={styles.container}>
        <p style={styles.error}>{optionsError}</p>
        <button style={styles.secondaryBtn} onClick={onCancel}>Back</button>
      </div>
    )
  }

  if (!options) {
    return <div style={styles.container}><p style={styles.status}>Loading options…</p></div>
  }

  const archetypesForJob = options.archetypes.filter((a) => {
    // Archetypes are filtered by job prefix convention: e.g. "ganger-brawler" for job "ganger".
    // If no archetypes match by prefix, show all (graceful fallback).
    const filtered = options.archetypes.filter((x) => x.id.startsWith(state.job))
    return filtered.length > 0 ? x.id.startsWith(state.job) : true
  })

  const previewStats = computePreviewStats(options, state)

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>Create Character</h1>
        <button style={styles.secondaryBtn} onClick={onCancel} type="button">Cancel</button>
      </header>

      {/* Step progress indicator */}
      <div style={styles.stepBar}>
        {STEPS.map((label, idx) => (
          <div key={label} style={styles.stepItem}>
            <div style={{
              ...styles.stepDot,
              ...(idx < step ? styles.stepDone : {}),
              ...(idx === step ? styles.stepActive : {}),
            }}>
              {idx < step ? '✓' : idx + 1}
            </div>
            <span style={{ ...styles.stepLabel, ...(idx === step ? styles.stepLabelActive : {}) }}>
              {label}
            </span>
          </div>
        ))}
      </div>

      <div style={styles.body}>
        {/* Main step content */}
        <form onSubmit={handleSubmit} style={styles.stepContent}>
          {step === 0 && (
            <OptionCards
              label="Select a Region"
              options={options.regions}
              selected={state.region}
              onSelect={(id) => update({ region: id })}
            />
          )}
          {step === 1 && (
            <OptionCards
              label="Select a Job"
              options={options.jobs}
              selected={state.job}
              onSelect={(id) => update({ job: id, archetype: '' })}
            />
          )}
          {step === 2 && (
            <OptionCards
              label="Select an Archetype"
              options={archetypesForJob}
              selected={state.archetype}
              onSelect={(id) => update({ archetype: id })}
            />
          )}
          {step === 3 && (
            <NameGenderStep
              name={state.name}
              gender={state.gender}
              nameAvailable={nameAvailable}
              nameChecking={nameChecking}
              onNameChange={(n) => update({ name: n })}
              onGenderChange={(g) => update({ gender: g })}
            />
          )}

          {submitError && <p style={styles.error}>{submitError}</p>}

          <div style={styles.navButtons}>
            {step > 0 && (
              <button style={styles.secondaryBtn} type="button" onClick={handleBack}>
                ← Back
              </button>
            )}
            {step < 3 && (
              <button
                style={{ ...styles.primaryBtn, ...(canAdvance() ? {} : styles.btnDisabled) }}
                type="button"
                onClick={handleNext}
                disabled={!canAdvance()}
              >
                Next →
              </button>
            )}
            {step === 3 && (
              <button
                style={{ ...styles.primaryBtn, ...(canAdvance() ? {} : styles.btnDisabled) }}
                type="submit"
                disabled={!canAdvance() || submitting}
              >
                {submitting ? 'Creating…' : 'Create Character'}
              </button>
            )}
          </div>
        </form>

        {/* Live stats preview sidebar */}
        <aside style={styles.sidebar}>
          <h3 style={styles.sidebarTitle}>Starting Stats Preview</h3>
          {previewStats.length === 0 ? (
            <p style={styles.sidebarEmpty}>Select options to see stats.</p>
          ) : (
            <dl style={styles.statList}>
              {previewStats.map(([key, val]) => (
                <div key={key} style={styles.statRow}>
                  <dt style={styles.statKey}>{key}</dt>
                  <dd style={styles.statVal}>{val}</dd>
                </div>
              ))}
            </dl>
          )}
          {state.name && <p style={styles.previewName}>{state.name}</p>}
          {state.region && <p style={styles.previewTag}>{state.region}</p>}
          {state.job && <p style={styles.previewTag}>{state.job}{state.archetype ? ` / ${state.archetype}` : ''}</p>}
        </aside>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Sub-components

interface OptionCardsProps {
  label: string
  options: CharacterOption[]
  selected: string
  onSelect: (id: string) => void
}

function OptionCards({ label, options, selected, onSelect }: OptionCardsProps) {
  return (
    <div>
      <h2 style={styles.stepHeading}>{label}</h2>
      {options.length === 0 && <p style={styles.status}>No options available.</p>}
      <div style={styles.optionGrid}>
        {options.map((opt) => (
          <button
            key={opt.id}
            type="button"
            style={{
              ...styles.optionCard,
              ...(selected === opt.id ? styles.optionCardSelected : {}),
            }}
            onClick={() => onSelect(opt.id)}
          >
            <div style={styles.optionName}>{opt.name}</div>
            {opt.description && (
              <div style={styles.optionDesc}>{opt.description}</div>
            )}
          </button>
        ))}
      </div>
    </div>
  )
}

interface NameGenderStepProps {
  name: string
  gender: string
  nameAvailable: boolean | null
  nameChecking: boolean
  onNameChange: (n: string) => void
  onGenderChange: (g: string) => void
}

function NameGenderStep({
  name,
  gender,
  nameAvailable,
  nameChecking,
  onNameChange,
  onGenderChange,
}: NameGenderStepProps) {
  function handleNameInput(e: ChangeEvent<HTMLInputElement>) {
    onNameChange(e.target.value)
  }

  let nameStatus: React.ReactNode = null
  if (name.length >= 3) {
    if (nameChecking) {
      nameStatus = <span style={styles.nameChecking}>Checking…</span>
    } else if (nameAvailable === true) {
      nameStatus = <span style={styles.nameAvailable}>✓ Available</span>
    } else if (nameAvailable === false) {
      nameStatus = <span style={styles.nameTaken}>✗ Name taken</span>
    }
  }

  return (
    <div>
      <h2 style={styles.stepHeading}>Name &amp; Gender</h2>

      <label style={styles.formLabel}>
        Character Name
        <div style={styles.nameInputRow}>
          <input
            style={styles.input}
            type="text"
            value={name}
            onChange={handleNameInput}
            minLength={3}
            maxLength={20}
            autoFocus
            placeholder="3–20 characters"
          />
          <span style={styles.nameStatusBadge}>{nameStatus}</span>
        </div>
      </label>

      <label style={styles.formLabel}>
        Gender
        <select
          style={styles.input}
          value={gender}
          onChange={(e) => onGenderChange(e.target.value)}
        >
          <option value="">Select…</option>
          <option value="male">Male</option>
          <option value="female">Female</option>
          <option value="nonbinary">Non-binary</option>
          <option value="other">Other</option>
        </select>
      </label>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Helpers

function computePreviewStats(
  options: CharacterOptions,
  state: WizardState,
): [string, number][] {
  const merged: Record<string, number> = {}

  // Merge stats from region, job, and archetype keys in starting_stats map.
  for (const key of [state.region, state.job, state.archetype]) {
    if (!key) continue
    const stats = options.starting_stats[key]
    if (!stats) continue
    for (const [stat, val] of Object.entries(stats)) {
      merged[stat] = (merged[stat] ?? 0) + val
    }
  }

  return Object.entries(merged).sort(([a], [b]) => a.localeCompare(b))
}

// ---------------------------------------------------------------------------
// Styles

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
    marginBottom: '1.5rem',
  },
  title: { margin: 0, color: '#e0c060', fontSize: '1.5rem' },
  stepBar: {
    display: 'flex',
    gap: '1rem',
    marginBottom: '2rem',
    alignItems: 'center',
  },
  stepItem: { display: 'flex', alignItems: 'center', gap: '0.4rem' },
  stepDot: {
    width: '28px',
    height: '28px',
    borderRadius: '50%',
    background: '#333',
    color: '#888',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: '0.75rem',
    fontWeight: 'bold',
    flexShrink: 0,
  },
  stepDone: { background: '#4caf50', color: '#fff' },
  stepActive: { background: '#e0c060', color: '#111' },
  stepLabel: { fontSize: '0.8rem', color: '#666' },
  stepLabelActive: { color: '#e0c060' },
  body: { display: 'flex', gap: '2rem', alignItems: 'flex-start' },
  stepContent: { flex: 1, minWidth: 0 },
  stepHeading: { color: '#e0c060', margin: '0 0 1rem', fontSize: '1.1rem' },
  optionGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
    gap: '0.75rem',
    marginBottom: '1.5rem',
  },
  optionCard: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '6px',
    padding: '0.75rem',
    cursor: 'pointer',
    textAlign: 'left' as const,
    fontFamily: 'monospace',
    color: '#ccc',
    transition: 'border-color 0.15s',
  },
  optionCardSelected: { border: '2px solid #e0c060', color: '#fff' },
  optionName: { fontWeight: 'bold', marginBottom: '0.25rem', color: '#eee' },
  optionDesc: { fontSize: '0.75rem', color: '#888', lineHeight: 1.4 },
  formLabel: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
    fontSize: '0.85rem',
    color: '#aaa',
    marginBottom: '1rem',
  },
  nameInputRow: { display: 'flex', alignItems: 'center', gap: '0.5rem' },
  input: {
    padding: '0.5rem',
    background: '#111',
    border: '1px solid #444',
    borderRadius: '4px',
    color: '#eee',
    fontSize: '1rem',
    fontFamily: 'monospace',
    flex: 1,
  },
  nameStatusBadge: { fontSize: '0.8rem', whiteSpace: 'nowrap' as const },
  nameChecking: { color: '#888' },
  nameAvailable: { color: '#4caf50' },
  nameTaken: { color: '#f55' },
  navButtons: { display: 'flex', gap: '0.75rem', marginTop: '1rem' },
  primaryBtn: {
    padding: '0.5rem 1.25rem',
    background: '#e0c060',
    color: '#111',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontWeight: 'bold',
  },
  btnDisabled: { opacity: 0.4, cursor: 'not-allowed' },
  secondaryBtn: {
    padding: '0.5rem 1rem',
    background: 'none',
    color: '#888',
    border: '1px solid #444',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
  },
  sidebar: {
    width: '220px',
    flexShrink: 0,
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '8px',
    padding: '1rem',
  },
  sidebarTitle: { margin: '0 0 0.75rem', color: '#e0c060', fontSize: '0.9rem' },
  sidebarEmpty: { color: '#555', fontSize: '0.8rem' },
  statList: { margin: 0, padding: 0 },
  statRow: {
    display: 'flex',
    justifyContent: 'space-between',
    padding: '0.2rem 0',
    borderBottom: '1px solid #222',
  },
  statKey: { color: '#aaa', fontSize: '0.8rem' },
  statVal: { color: '#eee', fontSize: '0.8rem', fontWeight: 'bold' },
  previewName: { marginTop: '0.75rem', color: '#e0c060', fontWeight: 'bold', fontSize: '0.9rem' },
  previewTag: { margin: '0.2rem 0 0', color: '#888', fontSize: '0.75rem' },
  status: { color: '#888' },
  error: { color: '#f55', fontSize: '0.85rem' },
}
```

- [ ] 6.2 Fix archetype filtering — the inline filter inside `OptionCards` reference is incorrect. Update the `archetypesForJob` computation in `CharacterWizard` to:

```typescript
const archetypesForJob: CharacterOption[] = (() => {
  if (!state.job) return options.archetypes
  const filtered = options.archetypes.filter((a) => a.id.startsWith(state.job))
  return filtered.length > 0 ? filtered : options.archetypes
})()
```

And remove the broken filter inside `OptionCards` — pass the pre-filtered `archetypesForJob` array directly (it is already passed correctly in the JSX; the only change is computing it outside the JSX as shown above, replacing the incorrect inline version in step 6.1).

- [ ] 6.3 Type-check:
  ```
  cd cmd/webclient/ui && npx tsc --noEmit
  ```
  Expected: no errors in `src/pages/CharacterWizard.tsx`.

---

## Task 7: Makefile Targets and .gitignore

**Files to update:**
- `Makefile`
- `.gitignore`

### Steps

- [ ] 7.1 Add the following lines to `.gitignore` (append at end of file):

```
# Web client build artifacts
cmd/webclient/ui/node_modules/
cmd/webclient/ui/dist/
```

- [ ] 7.2 Add the following targets to `Makefile`. The `.PHONY` declaration at the top of the file MUST be extended to include `ui-install ui-build proto-ts build-webclient`. Locate the existing `build:` target line and:

  (a) Add `ui-install` and `ui-build` and `proto-ts` and `build-webclient` to the `.PHONY` list at the top.

  (b) Add `build-webclient` to the `build:` prerequisite list.

  (c) Append the following new targets after the existing `proto:` target block:

```makefile
# Web client UI targets
UI_DIR := cmd/webclient/ui

ui-install:
	cd $(UI_DIR) && npm install

ui-build: ui-install
	cd $(UI_DIR) && npm run build

proto-ts: ui-install
	cd $(UI_DIR) && npx buf generate --config buf.gen.yaml

build-webclient: proto ui-build
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/webclient ./cmd/webclient
```

- [ ] 7.3 Update the `build:` target line to include `build-webclient`:

  Before:
  ```makefile
  build: proto build-frontend build-gameserver build-devserver build-migrate build-import-content build-setrole build-seed-claude-accounts
  ```
  After:
  ```makefile
  build: proto build-frontend build-gameserver build-devserver build-migrate build-import-content build-setrole build-seed-claude-accounts build-webclient
  ```

  Note: `build-webclient` already depends on `ui-build`, which depends on `ui-install`, so all UI prerequisites are transitively satisfied.

- [ ] 7.4 Verify Makefile syntax:
  ```
  make --dry-run ui-build
  make --dry-run proto-ts
  ```
  Expected: prints `cd cmd/webclient/ui && npm install` and `npm run build` (or `npx buf generate`) without errors.

- [ ] 7.5 Run a full type-check build to confirm all TypeScript files compile:
  ```
  cd cmd/webclient/ui && npm run build
  ```
  Expected: `✓ built in N.Nms` — Vite successfully bundles `dist/`. Any TypeScript compilation errors MUST be resolved before marking this task complete.

---

## Verification Checklist

After all tasks are complete:

- [ ] `make ui-install` succeeds (node_modules populated)
- [ ] `make proto-ts` succeeds (`src/proto/game/v1/game_pb.ts` present)
- [ ] `make ui-build` succeeds (`dist/index.html` present)
- [ ] `cd cmd/webclient/ui && npm run dev` starts Vite dev server on port 5173 with proxy config active
- [ ] Browser navigates to `http://localhost:5173/login` and shows the Login/Register tab form
- [ ] Login with valid credentials (Phase 2 backend running) redirects to `/characters`
- [ ] Characters page shows "No characters yet" for a fresh account with a "Create one!" link
- [ ] "Create New" opens the wizard at Step 1 (Region selection)
- [ ] Advancing through all 4 wizard steps populates the stats preview sidebar
- [ ] Name availability check fires after 300 ms and shows ✓/✗ indicator
- [ ] Submitting the wizard POSTs to `/api/characters` and returns to the characters list
- [ ] `.gitignore` prevents `node_modules/` and `dist/` from being staged: `git status` shows neither

---

## Notes for Implementer

- REQ-WC-30 (shared command parse function) and the game session WebSocket wiring are Phase 4 scope — `GamePageStub` in `App.tsx` is intentionally left as a placeholder for that phase.
- The admin page stub in `App.tsx` is Phase 5 scope.
- The `buf.gen.yaml` remote plugin (`buf.build/bufbuild/es`) requires internet access at `make proto-ts` time. In air-gapped environments, install `@bufbuild/protoc-gen-es` locally and use `local: protoc-gen-es` in `buf.gen.yaml` instead.
- All inline styles use `React.CSSProperties` — no CSS modules or Tailwind are introduced in this phase to keep dependencies minimal.
