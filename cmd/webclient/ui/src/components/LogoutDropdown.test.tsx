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
