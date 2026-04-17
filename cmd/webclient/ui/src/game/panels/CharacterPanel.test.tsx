// REQ-70-3: CharacterPanel MUST display the Crypto balance beneath the XP display.
// REQ-70-4: The Crypto display MUST use the currency string from characterSheet.currency.

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import fc from 'fast-check'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

// Minimal CharacterPanel — import after mock is set.
import { CharacterPanel } from './CharacterPanel'

function makeState(characterSheet: Record<string, unknown> | null) {
  return {
    state: {
      connected: true,
      characterSheet,
      characterInfo: null,
      roomView: null,
      combatRound: null,
      combatantHp: {},
      combatantAP: {},
      combatPositions: {},
      feed: [],
      hotbar: [],
      mapTiles: [],
      worldTiles: [],
      combatGridWidth: 20,
      combatGridHeight: 20,
    },
    dispatch: vi.fn(),
    sendMessage: vi.fn(),
    sendCommand: vi.fn(),
    clearShop: vi.fn(),
    clearHealer: vi.fn(),
    clearTrainer: vi.fn(),
    clearFixer: vi.fn(),
    clearRestView: vi.fn(),
    clearNpcView: vi.fn(),
    clearQuestGiverView: vi.fn(),
    dismissQuestComplete: vi.fn(),
    clearLoadout: vi.fn(),
    clearChoicePrompt: vi.fn(),
  }
}

describe('CharacterPanel — Crypto balance display', () => {
  it('renders the Crypto balance beneath XP when currency is set (REQ-70-3)', () => {
    mockUseGame.mockReturnValue(
      makeState({
        name: 'Alice',
        level: 1,
        currency: '340 Crypto',
        experience: 1240,
        xpToNext: 2000,
        currentHp: 30,
        maxHp: 30,
      })
    )

    render(<CharacterPanel />)

    expect(screen.getByText(/340 Crypto/)).toBeDefined()
  })

  it('renders the currency string including Crypto (REQ-70-4)', () => {
    mockUseGame.mockReturnValue(
      makeState({
        name: 'Alice',
        level: 1,
        currency: '99 Crypto',
        experience: 0,
        xpToNext: 500,
        currentHp: 20,
        maxHp: 20,
      })
    )

    render(<CharacterPanel />)

    expect(screen.getByText('99 Crypto')).toBeDefined()
  })

  it('does not render a Crypto line when currency is absent (REQ-70-3)', () => {
    mockUseGame.mockReturnValue(
      makeState({
        name: 'Alice',
        level: 1,
        experience: 0,
        xpToNext: 500,
        currentHp: 20,
        maxHp: 20,
        // no currency field
      })
    )

    render(<CharacterPanel />)

    expect(screen.queryByText(/Crypto/)).toBeNull()
  })

  it('property: any non-empty currency string is displayed (REQ-70-4)', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 999999 }),
        (amount) => {
          const currency = `${amount} Crypto`
          mockUseGame.mockReturnValue(
            makeState({
              name: 'Test',
              level: 1,
              currency,
              experience: 0,
              xpToNext: 1000,
              currentHp: 10,
              maxHp: 10,
            })
          )

          const { unmount, container } = render(<CharacterPanel />)
          const found = container.textContent?.includes(amount.toString()) ?? false
          unmount()
          return found
        }
      )
    )
  })
})
