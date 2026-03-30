# Login Splash Image Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace ASCII art on the login page with `gunchete.png` as a full-viewport background image, with the login/register form centered over a semi-transparent dark card.

**Architecture:** The PNG is converted to WebP (quality 80, max 1920px) and placed in `cmd/webclient/ui/public/` so Vite serves it at `/gunchete.webp`. `LoginPage.tsx` is updated to use the image as a CSS `background-image` with an absolute-positioned overlay div, and the ASCII art constants and splash div are removed. No other files change.

**Tech Stack:** cwebp (libwebp), React 18, TypeScript, Vite

---

## File Map

| Action | Path |
|--------|------|
| Create | `cmd/webclient/ui/public/gunchete.webp` |
| Modify | `cmd/webclient/ui/src/pages/LoginPage.tsx` |

---

### Task 1: Optimize and place the splash image

**Files:**
- Create: `cmd/webclient/ui/public/gunchete.webp`

- [ ] **Step 1: Install cwebp if not present**

```bash
which cwebp || sudo apt-get install -y webp
```

Expected: path printed (e.g. `/usr/bin/cwebp`) or package installs successfully.

- [ ] **Step 2: Create the public directory**

```bash
mkdir -p cmd/webclient/ui/public
```

- [ ] **Step 3: Convert and resize the image**

```bash
cwebp -q 80 -resize 1920 0 gunchete.png -o cmd/webclient/ui/public/gunchete.webp
```

The `-resize 1920 0` flag sets max width to 1920px; height scales proportionally. Expected output ends with `File:      cmd/webclient/ui/public/gunchete.webp` and a size well under 1MB.

- [ ] **Step 4: Verify the output file**

```bash
ls -lh cmd/webclient/ui/public/gunchete.webp
```

Expected: file exists, size is under 1MB.

- [ ] **Step 5: Commit**

```bash
git add cmd/webclient/ui/public/gunchete.webp
git commit -m "feat: add optimized WebP splash image for login page"
```

---

### Task 2: Update LoginPage.tsx

**Files:**
- Modify: `cmd/webclient/ui/src/pages/LoginPage.tsx`

- [ ] **Step 1: Replace the file contents**

Replace `cmd/webclient/ui/src/pages/LoginPage.tsx` with the following (logic is unchanged; only the splash block and styles are modified):

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
      <div style={styles.overlay} />
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
    width: '100vw',
    height: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundImage: "url('/gunchete.webp')",
    backgroundSize: 'cover',
    backgroundPosition: 'center',
    position: 'relative',
    fontFamily: 'monospace',
  },
  overlay: {
    position: 'absolute',
    inset: 0,
    background: 'rgba(0,0,0,0.55)',
    zIndex: 0,
  },
  card: {
    background: 'rgba(13,13,13,0.85)',
    border: '1px solid #333',
    borderRadius: '8px',
    padding: '2rem',
    width: '100%',
    maxWidth: '360px',
    color: '#ccc',
    position: 'relative',
    zIndex: 1,
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

- [ ] **Step 2: Type-check the UI**

```bash
cd cmd/webclient/ui && npx tsc --noEmit
```

Expected: no errors, exits 0.

- [ ] **Step 3: Build the UI**

```bash
cd cmd/webclient/ui && npm run build
```

Expected: `dist/` rebuilt with no errors. The build output should reference `gunchete.webp` in the assets (Vite copies `public/` verbatim to `dist/`).

- [ ] **Step 4: Verify the asset is in dist**

```bash
ls cmd/webclient/ui/dist/gunchete.webp
```

Expected: file present.

- [ ] **Step 5: Commit**

```bash
git add cmd/webclient/ui/src/pages/LoginPage.tsx
git commit -m "feat: replace login ASCII art with full-viewport splash image"
```
