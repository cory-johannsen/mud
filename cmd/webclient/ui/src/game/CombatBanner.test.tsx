// REQ-61-5: CombatBanner MUST render a round countdown progress bar when combatRound.durationMs > 0.
// REQ-61-6: The countdown bar's fill width MUST start at 100% and decrease toward 0% over durationMs.
// REQ-61-7: CombatBanner MUST NOT render a countdown bar when combatRound is null.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import fc from 'fast-check'
import { CombatBanner, ReactionBadge } from './CombatBanner'
import type { GameState } from './GameContext'
import { initialState } from './GameContext'

// Mock useGame so we can control state without a full provider.
vi.mock('./GameContext', async (importOriginal) => {
  const real = await importOriginal<typeof import('./GameContext')>()
  return {
    ...real,
    useGame: vi.fn(),
  }
})

import { useGame } from './GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

function makeState(overrides: Partial<GameState>): GameState {
  return { ...initialState, ...overrides }
}

beforeEach(() => {
  vi.useFakeTimers()
})
afterEach(() => {
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('CombatBanner — round countdown timer bar', () => {
  it('renders a timer progress bar when combatRound has durationMs > 0 (REQ-61-5)', () => {
    mockUseGame.mockReturnValue({
      state: makeState({
        combatRound: {
          round: 1,
          durationMs: 6000,
          turnOrder: ['Alice'],
          actionsPerTurn: 3,
        },
        combatantAP: {},
      }),
      dispatch: vi.fn(),
    })

    render(<CombatBanner />)

    const bar = screen.getByRole('progressbar', { name: /round timer/i })
    expect(bar).toBeDefined()
  })

  it('timer bar starts at 100% fill on round start (REQ-61-6)', () => {
    mockUseGame.mockReturnValue({
      state: makeState({
        combatRound: {
          round: 1,
          durationMs: 6000,
          turnOrder: ['Alice'],
          actionsPerTurn: 3,
        },
        combatantAP: {},
      }),
      dispatch: vi.fn(),
    })

    render(<CombatBanner />)

    const fill = screen.getByTestId('round-timer-fill')
    const style = fill.getAttribute('style') ?? ''
    // Should start at 100% width.
    expect(style).toContain('width: 100%')
  })

  it('timer bar depletes over time (REQ-61-6)', () => {
    mockUseGame.mockReturnValue({
      state: makeState({
        combatRound: {
          round: 1,
          durationMs: 6000,
          turnOrder: ['Alice'],
          actionsPerTurn: 3,
        },
        combatantAP: {},
      }),
      dispatch: vi.fn(),
    })

    render(<CombatBanner />)

    // Advance 3000ms — half the round should be gone.
    act(() => {
      vi.advanceTimersByTime(3000)
    })

    const fill = screen.getByTestId('round-timer-fill')
    const style = fill.getAttribute('style') ?? ''
    // At 3000ms of a 6000ms round, fill should be near 50%.
    // We check it's not 100% anymore.
    expect(style).not.toContain('width: 100%')
  })

  it('timer bar reaches 0% at round end (REQ-61-6)', () => {
    mockUseGame.mockReturnValue({
      state: makeState({
        combatRound: {
          round: 1,
          durationMs: 6000,
          turnOrder: ['Alice'],
          actionsPerTurn: 3,
        },
        combatantAP: {},
      }),
      dispatch: vi.fn(),
    })

    render(<CombatBanner />)

    act(() => {
      vi.advanceTimersByTime(6000)
    })

    const fill = screen.getByTestId('round-timer-fill')
    const style = fill.getAttribute('style') ?? ''
    expect(style).toContain('width: 0%')
  })

  it('does not render a timer bar when combatRound is null (REQ-61-7)', () => {
    mockUseGame.mockReturnValue({
      state: makeState({ combatRound: null }),
      dispatch: vi.fn(),
    })

    render(<CombatBanner />)

    expect(screen.queryByRole('progressbar', { name: /round timer/i })).toBeNull()
  })

  it('does not render a timer bar when durationMs is 0', () => {
    mockUseGame.mockReturnValue({
      state: makeState({
        combatRound: {
          round: 1,
          durationMs: 0,
          turnOrder: ['Alice'],
          actionsPerTurn: 3,
        },
        combatantAP: {},
      }),
      dispatch: vi.fn(),
    })

    render(<CombatBanner />)

    expect(screen.queryByRole('progressbar', { name: /round timer/i })).toBeNull()
  })

  // REQ-61-8: Timer bar MUST reset to 100% at the start of each new round,
  // even when durationMs is unchanged between rounds.
  it('timer bar resets to 100% on a new round when durationMs stays the same', () => {
    mockUseGame.mockReturnValue({
      state: makeState({
        combatRound: {
          round: 1,
          durationMs: 6000,
          turnOrder: ['Alice'],
          actionsPerTurn: 3,
        },
        combatantAP: {},
      }),
      dispatch: vi.fn(),
    })

    const { rerender } = render(<CombatBanner />)

    // Deplete half the round.
    act(() => {
      vi.advanceTimersByTime(3000)
    })

    const fill = screen.getByTestId('round-timer-fill')
    expect(fill.getAttribute('style')).not.toContain('width: 100%')

    // Simulate a new round with the same durationMs.
    mockUseGame.mockReturnValue({
      state: makeState({
        combatRound: {
          round: 2,
          durationMs: 6000,
          turnOrder: ['Alice'],
          actionsPerTurn: 3,
        },
        combatantAP: {},
      }),
      dispatch: vi.fn(),
    })
    rerender(<CombatBanner />)

    // Timer must reset to full at the start of the new round.
    expect(fill.getAttribute('style')).toContain('width: 100%')
  })

  it('property: fill width is always a percentage in [0%, 100%]', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 30000 }),
        fc.integer({ min: 0, max: 30000 }),
        (durationMs, elapsedMs) => {
          mockUseGame.mockReturnValue({
            state: makeState({
              combatRound: {
                round: 1,
                durationMs,
                turnOrder: ['Alice'],
                actionsPerTurn: 3,
              },
              combatantAP: {},
            }),
            dispatch: vi.fn(),
          })

          const { unmount } = render(<CombatBanner />)

          act(() => {
            vi.advanceTimersByTime(elapsedMs)
          })

          const fill = document.querySelector('[data-testid="round-timer-fill"]')
          if (fill) {
            const style = (fill as HTMLElement).style.width
            const pct = parseFloat(style)
            const ok = pct >= 0 && pct <= 100
            unmount()
            return ok
          }
          unmount()
          return true // durationMs=0 means no bar, which is acceptable
        }
      )
    )
  })
})

// REQ-244-TASK15-1: ReactionBadge renders "R: <remaining>" when Max == 1 and no reactions spent.
// REQ-244-TASK15-2: ReactionBadge renders with strike-through when the budget is exhausted
//   (remaining == 0), signalling the player has no reactions left this round.
describe('ReactionBadge — reaction budget display (GH #244 Task 15)', () => {
  beforeEach(() => {
    vi.useRealTimers() // this suite does not rely on fake timers
  })

  it('renders "R: 1" when reactionMax=1 and no reactions spent (REQ-244-TASK15-1)', () => {
    render(<ReactionBadge reactionMax={1} reactionSpent={0} />)
    const badge = screen.getByTestId('reaction-badge')
    expect(badge.textContent).toBe('R: 1')
    // Not exhausted: no strike-through.
    expect(badge.getAttribute('style') ?? '').not.toContain('line-through')
    expect(badge.className).not.toContain('combat-reaction-exhausted')
    // Tooltip reflects max reactions per round.
    expect(badge.getAttribute('title')).toBe('1 reaction per round')
  })

  it('renders with strike-through / muted styling when exhausted (REQ-244-TASK15-2)', () => {
    render(<ReactionBadge reactionMax={1} reactionSpent={1} />)
    const badge = screen.getByTestId('reaction-badge')
    expect(badge.textContent).toBe('R: 0')
    const style = badge.getAttribute('style') ?? ''
    expect(style).toContain('line-through')
    expect(badge.className).toContain('combat-reaction-exhausted')
  })

  it('renders "R: 1/2" fraction when reactionMax > 1 (BonusReactions path)', () => {
    render(<ReactionBadge reactionMax={2} reactionSpent={1} />)
    const badge = screen.getByTestId('reaction-badge')
    expect(badge.textContent).toBe('R: 1/2')
    expect(badge.getAttribute('title')).toBe('2 reactions per round')
  })

  it('does not render when reactionMax is 0 (budget not initialised)', () => {
    const { container } = render(<ReactionBadge reactionMax={0} reactionSpent={0} />)
    expect(container.querySelector('[data-testid="reaction-badge"]')).toBeNull()
  })
})
