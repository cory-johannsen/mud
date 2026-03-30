# Character Switch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the plain Logout button in the GamePage toolbar with a dropdown offering "Switch Character" (navigate to `/characters`) and "Logout" (clear auth).

**Architecture:** A new `LogoutDropdown` React component manages its own open/closed state with an outside-click `useEffect` listener. It sources `useAuth` and `useNavigate` internally and replaces the single-line logout button in `GamePage`. No server-side changes required — the gRPC stream closes naturally when `GamePage` unmounts.

**Tech Stack:** React 18, React Router v6, TypeScript, Vitest, React Testing Library, jsdom

---

## File Map

| Action | Path | Purpose |
|--------|------|---------|
| Create | `cmd/webclient/ui/src/components/LogoutDropdown.tsx` | Dropdown component |
| Create | `cmd/webclient/ui/src/components/LogoutDropdown.test.tsx` | Unit tests |
| Create | `cmd/webclient/ui/src/test-setup.ts` | jest-dom matchers setup |
| Modify | `cmd/webclient/ui/vite.config.ts` | Add vitest test config |
| Modify | `cmd/webclient/ui/tsconfig.json` | Add vitest/globals types |
| Modify | `cmd/webclient/ui/package.json` | Add test deps + test script |
| Modify | `cmd/webclient/ui/src/pages/GamePage.tsx:41,71` | Swap button for component |

---

## Task 1: Add Vitest + React Testing Library

**Files:**
- Modify: `cmd/webclient/ui/package.json`
- Modify: `cmd/webclient/ui/vite.config.ts`
- Modify: `cmd/webclient/ui/tsconfig.json`
- Create: `cmd/webclient/ui/src/test-setup.ts`

- [ ] **Step 1: Install test dependencies**

```bash
cd cmd/webclient/ui
npm install --save-dev vitest @vitest/ui @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom
```

Expected: packages added to `node_modules/` and `package-lock.json` updated.

- [ ] **Step 2: Add test script to package.json**

Replace the `scripts` section in `cmd/webclient/ui/package.json`:

```json
"scripts": {
  "dev": "vite",
  "build": "tsc && vite build",
  "preview": "vite preview",
  "test": "vitest run",
  "test:watch": "vitest"
},
```

- [ ] **Step 3: Update vite.config.ts with test config**

Replace the full contents of `cmd/webclient/ui/vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
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

- [ ] **Step 4: Create test-setup.ts**

Create `cmd/webclient/ui/src/test-setup.ts`:

```ts
import '@testing-library/jest-dom'
```

- [ ] **Step 5: Update tsconfig.json to include vitest globals**

Replace the full contents of `cmd/webclient/ui/tsconfig.json`:

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
    "noFallthroughCasesInSwitch": true,
    "types": ["vitest/globals", "@testing-library/jest-dom"]
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 6: Verify test runner initializes**

```bash
cd cmd/webclient/ui
npm test
```

Expected output: `No test files found` (or 0 tests run, exit 0). Confirms Vitest is configured correctly before any test files exist.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add cmd/webclient/ui/package.json cmd/webclient/ui/package-lock.json cmd/webclient/ui/vite.config.ts cmd/webclient/ui/tsconfig.json cmd/webclient/ui/src/test-setup.ts
git commit -m "chore: add vitest and react testing library to web client"
```

---

## Task 2: LogoutDropdown Component + GamePage Integration

**Files:**
- Create: `cmd/webclient/ui/src/components/LogoutDropdown.tsx`
- Create: `cmd/webclient/ui/src/components/LogoutDropdown.test.tsx`
- Modify: `cmd/webclient/ui/src/pages/GamePage.tsx`

- [ ] **Step 1: Create the components directory and write failing tests**

Create `cmd/webclient/ui/src/components/LogoutDropdown.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LogoutDropdown } from './LogoutDropdown'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

const mockLogout = vi.fn()
vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({ logout: mockLogout }),
}))

describe('LogoutDropdown', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('dropdown is closed by default', () => {
    render(<LogoutDropdown />)
    expect(screen.queryByText('Switch Character')).toBeNull()
  })

  it('clicking the trigger opens the dropdown', () => {
    render(<LogoutDropdown />)
    fireEvent.click(screen.getByText('Logout ▾'))
    expect(screen.getByText('Switch Character')).toBeDefined()
  })

  it('clicking Switch Character navigates to /characters', () => {
    render(<LogoutDropdown />)
    fireEvent.click(screen.getByText('Logout ▾'))
    fireEvent.click(screen.getByText('Switch Character'))
    expect(mockNavigate).toHaveBeenCalledWith('/characters')
  })

  it('clicking Logout calls logout()', () => {
    render(<LogoutDropdown />)
    fireEvent.click(screen.getByText('Logout ▾'))
    fireEvent.click(screen.getByText('Logout'))
    expect(mockLogout).toHaveBeenCalled()
  })

  it('clicking outside closes the dropdown', () => {
    render(
      <div>
        <LogoutDropdown />
        <div data-testid="outside">outside</div>
      </div>
    )
    fireEvent.click(screen.getByText('Logout ▾'))
    expect(screen.getByText('Switch Character')).toBeDefined()
    fireEvent.mouseDown(screen.getByTestId('outside'))
    expect(screen.queryByText('Switch Character')).toBeNull()
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd cmd/webclient/ui
npm test
```

Expected: 5 tests FAIL with `Cannot find module './LogoutDropdown'` or similar. Confirms tests are wired up correctly.

- [ ] **Step 3: Implement LogoutDropdown**

Create `cmd/webclient/ui/src/components/LogoutDropdown.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

const dropdownPanelStyle: React.CSSProperties = {
  position: 'absolute',
  top: '100%',
  right: 0,
  marginTop: '2px',
  background: '#0d0d0d',
  border: '1px solid #333',
  borderRadius: '3px',
  minWidth: '140px',
  zIndex: 1000,
}

const dropdownItemStyle: React.CSSProperties = {
  display: 'block',
  width: '100%',
  padding: '0.4rem 0.75rem',
  background: 'none',
  border: 'none',
  color: '#aaa',
  fontFamily: 'monospace',
  fontSize: '0.8rem',
  cursor: 'pointer',
  textAlign: 'left',
}

export function LogoutDropdown() {
  const [open, setOpen] = useState(false)
  const { logout } = useAuth()
  const navigate = useNavigate()
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  function handleSwitchCharacter() {
    setOpen(false)
    navigate('/characters')
  }

  function handleLogout() {
    setOpen(false)
    logout()
  }

  return (
    <div ref={ref} style={{ position: 'relative', marginLeft: 'auto' }}>
      <button
        className="toolbar-btn toolbar-btn-logout"
        style={{ marginLeft: 0 }}
        onClick={() => setOpen((o) => !o)}
      >
        Logout ▾
      </button>
      {open && (
        <div style={dropdownPanelStyle}>
          <button
            style={dropdownItemStyle}
            onMouseEnter={(e) => { e.currentTarget.style.background = '#1a1a1a' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'none' }}
            onClick={handleSwitchCharacter}
          >
            Switch Character
          </button>
          <button
            style={dropdownItemStyle}
            onMouseEnter={(e) => { e.currentTarget.style.background = '#1a1a1a' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'none' }}
            onClick={handleLogout}
          >
            Logout
          </button>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd cmd/webclient/ui
npm test
```

Expected: `5 tests passed`. If any fail, fix the component before proceeding.

- [ ] **Step 5: Update GamePage.tsx**

In `cmd/webclient/ui/src/pages/GamePage.tsx`:

**a.** Add the import at line 18 (after the existing imports, before the style import):

Replace:
```tsx
import '../styles/game.css'
```

With:
```tsx
import { LogoutDropdown } from '../components/LogoutDropdown'
import '../styles/game.css'
```

**b.** Remove `logout` from the `useAuth()` destructure at line 41:

Replace:
```tsx
  const { logout } = useAuth()
```

With:
```tsx
  const { } = useAuth()
```

Wait — if `logout` is the only thing used from `useAuth()` in `GameLayout`, remove the entire line:

Replace:
```tsx
  const { state } = useGame()
  const { logout } = useAuth()
```

With:
```tsx
  const { state } = useGame()
```

**c.** Replace the Logout button at line 71:

Replace:
```tsx
          <button className="toolbar-btn toolbar-btn-logout" onClick={logout}>Logout</button>
```

With:
```tsx
          <LogoutDropdown />
```

- [ ] **Step 6: Verify TypeScript compiles**

```bash
cd cmd/webclient/ui
npx tsc --noEmit
```

Expected: no errors. If you see `noUnusedLocals` errors about a removed import, verify `useAuth` import was fully removed from `GameLayout`.

- [ ] **Step 7: Run full test suite**

```bash
cd cmd/webclient/ui
npm test
```

Expected: `5 passed`. All green.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud
git add cmd/webclient/ui/src/components/LogoutDropdown.tsx cmd/webclient/ui/src/components/LogoutDropdown.test.tsx cmd/webclient/ui/src/pages/GamePage.tsx
git commit -m "feat: replace logout button with switch-character dropdown"
```

---

## Self-Review Checklist

**Spec coverage:**
- REQ-CS-1 ✅ `LogoutDropdown.tsx` created
- REQ-CS-2 ✅ trigger labeled `Logout ▾`
- REQ-CS-3 ✅ toggle on click via `useState`
- REQ-CS-4 ✅ two items: Switch Character, Logout
- REQ-CS-5 ✅ Switch Character calls `navigate('/characters')`
- REQ-CS-6 ✅ Logout calls `logout()`
- REQ-CS-7 ✅ outside click closes via mousedown listener
- REQ-CS-8 ✅ `useEffect` with `document.addEventListener('mousedown', ...)` + cleanup
- REQ-CS-9 ✅ `GamePage.tsx` replaces button with `<LogoutDropdown />`
- REQ-CS-10 ✅ no props; deps sourced inside component
- REQ-CS-11 ✅ `toolbar-btn toolbar-btn-logout` classes on trigger
- REQ-CS-12 ✅ `position: absolute`, `zIndex: 1000`
- REQ-CS-13 ✅ background `#0d0d0d`, border `1px solid #333`
- REQ-CS-14 ✅ hover handler sets `#1a1a1a`
- REQ-CS-15 ✅ `CharactersPage` not touched
- REQ-CS-16 ✅ 5 unit tests covering all specified behaviors
