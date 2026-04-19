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
    hotbarSlots: [],
  },
  sendMessage: vi.fn(),
  sendCommand: vi.fn(),
}

function makeSheetWith(preparedSlots: object[]) {
  return {
    name: 'Test Player',
    level: 5,
    preparedSlots,
    hardwiredSlots: [],
    innateSlots: [],
    spontaneousKnown: [],
    spontaneousUsePools: [],
    focusPoints: 0,
    maxFocusPoints: 0,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
})

import { TechnologyDrawer } from './TechnologyDrawer'

// REQ-UI-191: Usage indicators MUST be shown for all prepared techs regardless of use count,
// including single-use techs.
describe('TechnologyDrawer usage indicators', () => {
  it('single-use non-expended tech shows a usage pip indicator', () => {
    const sheet = makeSheetWith([
      { techId: 'shock_wave', techName: 'Shock Wave', techLevel: 1, expended: false },
    ])
    mockUseGame.mockReturnValue({ ...BASE_STATE, state: { ...BASE_STATE.state, characterSheet: sheet } })
    render(<TechnologyDrawer onClose={() => {}} />)

    // UsePips renders "remaining/max" label — should show "1/1"
    expect(screen.getByTitle('1 / 1')).toBeTruthy()
  })

  it('single-use expended tech shows a usage pip indicator showing 0/1', () => {
    const sheet = makeSheetWith([
      { techId: 'shock_wave', techName: 'Shock Wave', techLevel: 1, expended: true },
    ])
    mockUseGame.mockReturnValue({ ...BASE_STATE, state: { ...BASE_STATE.state, characterSheet: sheet } })
    render(<TechnologyDrawer onClose={() => {}} />)

    // UsePips renders "remaining/max" label — should show "0/1"
    expect(screen.getByTitle('0 / 1')).toBeTruthy()
  })

  it('multi-use tech shows usage pips for all uses', () => {
    const sheet = makeSheetWith([
      { techId: 'emp_burst', techName: 'EMP Burst', techLevel: 2, expended: false },
      { techId: 'emp_burst', techName: 'EMP Burst', techLevel: 2, expended: false },
      { techId: 'emp_burst', techName: 'EMP Burst', techLevel: 2, expended: false },
    ])
    mockUseGame.mockReturnValue({ ...BASE_STATE, state: { ...BASE_STATE.state, characterSheet: sheet } })
    render(<TechnologyDrawer onClose={() => {}} />)

    // UsePips renders "remaining/max" label — 3 slots, none expended → "3/3"
    expect(screen.getByTitle('3 / 3')).toBeTruthy()
  })

  it('single-use tech does NOT show expended badge (usage pip is sufficient)', () => {
    const sheet = makeSheetWith([
      { techId: 'shock_wave', techName: 'Shock Wave', techLevel: 1, expended: true },
    ])
    mockUseGame.mockReturnValue({ ...BASE_STATE, state: { ...BASE_STATE.state, characterSheet: sheet } })
    render(<TechnologyDrawer onClose={() => {}} />)

    // The old "expended" badge should no longer appear; UsePips is used instead
    expect(screen.queryByText('expended')).toBeNull()
  })
})
