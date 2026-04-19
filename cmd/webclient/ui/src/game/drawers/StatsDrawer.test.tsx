import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

const BASE_STATE = {
  state: {
    connected: false,
    characterSheet: null,
  },
  sendMessage: vi.fn(),
  sendCommand: vi.fn(),
}

const FULL_SHEET = {
  name: 'Test Player',
  level: 5,
  brutality: 16,
  grit: 12,
  quickness: 14,
  reasoning: 10,
  savvy: 13,
  flair: 8,
  totalAc: 14,
  acBonus: 2,
  checkPenalty: 0,
  speedPenalty: 0,
  toughnessSave: 3,
  hustleSave: 2,
  coolSave: 1,
  awareness: 4,
  playerResistances: [],
  playerWeaknesses: [],
  proficiencies: [],
}

beforeEach(() => {
  mockUseGame.mockReturnValue({ ...BASE_STATE, state: { ...BASE_STATE.state, characterSheet: FULL_SHEET } })
})

import { StatsDrawer } from './StatsDrawer'

// REQ-UI-188: The AC row tooltip MUST appear on the entire row (label + value), not just the value span.
describe('StatsDrawer AC row tooltip', () => {
  it('AC row outer div has the tooltip text', () => {
    render(<StatsDrawer onClose={() => {}} />)
    const acRow = screen.getByText('AC').closest('.stats-row')
    expect(acRow).not.toBeNull()
    const title = acRow?.getAttribute('title')
    expect(title).toBeTruthy()
    expect(title).toContain('14')
  })
})

// REQ-UI-189: Each ability score cell MUST have a non-empty title attribute describing the ability.
describe('StatsDrawer ability score tooltips', () => {
  const abilities = ['Brutality', 'Grit', 'Quickness', 'Reasoning', 'Savvy', 'Flair']

  it.each(abilities)('%s cell has a non-empty title attribute', (ability) => {
    render(<StatsDrawer onClose={() => {}} />)
    const cell = screen.getByText(ability).closest('.stats-ability-cell')
    expect(cell).not.toBeNull()
    const title = cell?.getAttribute('title')
    expect(title).toBeTruthy()
    expect(title!.length).toBeGreaterThan(0)
  })

  it.each(abilities)('%s title contains the ability name', (ability) => {
    render(<StatsDrawer onClose={() => {}} />)
    const cell = screen.getByText(ability).closest('.stats-ability-cell')
    const title = cell?.getAttribute('title') ?? ''
    expect(title).toContain(ability)
  })
})
