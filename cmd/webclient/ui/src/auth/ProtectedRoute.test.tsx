import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
// ---------------------------------------------------------------------------
// Shared mutable state that the AuthContext mock reads on each call.
// ---------------------------------------------------------------------------
interface AuthUser {
  accountId: number
  role: string
  exp: number
}

let _mockUser: AuthUser | null = null

vi.mock('./AuthContext', () => ({
  useAuth: () => ({ user: _mockUser, token: null, login: vi.fn(), logout: vi.fn() }),
}))

// Navigate renders a sentinel element so we can assert the redirect target.
vi.mock('react-router-dom', () => ({
  Navigate: ({ to }: { to: string }) => (
    <div data-testid="navigate" data-to={to} />
  ),
}))

import { ProtectedRoute } from './ProtectedRoute'

function renderRoute(user: AuthUser | null, requiredRole?: string | string[], child = 'Protected Content') {
  _mockUser = user
  return render(
    <ProtectedRoute requiredRole={requiredRole as never}>
      <div>{child}</div>
    </ProtectedRoute>,
  )
}

describe('ProtectedRoute', () => {
  beforeEach(() => {
    _mockUser = null
  })

  it('redirects to /login when user is null (not logged in)', () => {
    renderRoute(null, ['admin', 'moderator'])
    expect(screen.getByTestId('navigate').getAttribute('data-to')).toBe('/login')
    expect(screen.queryByText('Protected Content')).toBeNull()
  })

  it('redirects to /game when user has player role but route requires admin or moderator', () => {
    renderRoute({ accountId: 2, role: 'player', exp: 9999999999 }, ['admin', 'moderator'])
    expect(screen.getByTestId('navigate').getAttribute('data-to')).toBe('/game')
    expect(screen.queryByText('Protected Content')).toBeNull()
  })

  it('renders children when user has admin role', () => {
    renderRoute({ accountId: 3, role: 'admin', exp: 9999999999 }, ['admin', 'moderator'])
    expect(screen.getByText('Protected Content')).toBeDefined()
    expect(screen.queryByTestId('navigate')).toBeNull()
  })

  it('renders children when user has moderator role', () => {
    renderRoute({ accountId: 4, role: 'moderator', exp: 9999999999 }, ['admin', 'moderator'])
    expect(screen.getByText('Protected Content')).toBeDefined()
    expect(screen.queryByTestId('navigate')).toBeNull()
  })
})
