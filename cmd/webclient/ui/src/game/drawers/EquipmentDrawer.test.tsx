import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

const BASE_STATE = {
  state: {
    connected: false,
    characterSheet: null,
    loadoutView: null,
  },
  sendMessage: vi.fn(),
  sendCommand: vi.fn(),
  clearLoadout: vi.fn(),
}

const SHEET_WITH_MAIN_HAND = {
  name: 'Test Player',
  level: 5,
  mainHand: 'Tactical Machete',
  mainHandAttackBonus: '+7',
  mainHandDamage: '1d8+3 slashing',
  mainHandAbilityBonus: 3,
  mainHandProfBonus: 4,
  mainHandProfRank: 'expert',
}

beforeEach(() => {
  mockUseGame.mockReturnValue(BASE_STATE)
})

import { EquipmentDrawer } from './EquipmentDrawer'

// REQ-WEC-71: hovering an equipped weapon in the Weapons section MUST display
// a tooltip showing weapon name, damage, total to-hit, and bonus breakdown.
describe('EquipmentDrawer — weapon hover tooltip', () => {
  it('shows no tooltip when no weapon is equipped', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, characterSheet: { name: 'Test', level: 1 } },
    })
    render(<EquipmentDrawer onClose={vi.fn()} />)
    // No tooltip should be present
    expect(screen.queryByRole('tooltip')).toBeNull()
    expect(document.querySelector('[data-testid="weapon-tooltip"]')).toBeNull()
  })

  it('shows weapon tooltip on hover over equipped main-hand weapon', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, characterSheet: SHEET_WITH_MAIN_HAND },
    })
    const { container } = render(<EquipmentDrawer onClose={vi.fn()} />)

    // Find the main-hand weapon slot value element
    const weaponText = screen.getByText(/Tactical Machete/)
    fireEvent.mouseEnter(weaponText.closest('[data-weapon-slot]') ?? weaponText)

    // Tooltip must be visible with weapon name
    const tooltip = container.querySelector('[data-testid="weapon-tooltip"]')
    expect(tooltip).not.toBeNull()
    expect(tooltip?.textContent).toContain('Tactical Machete')
  })

  it('tooltip contains damage string', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, characterSheet: SHEET_WITH_MAIN_HAND },
    })
    const { container } = render(<EquipmentDrawer onClose={vi.fn()} />)

    const weaponText = screen.getByText(/Tactical Machete/)
    fireEvent.mouseEnter(weaponText.closest('[data-weapon-slot]') ?? weaponText)

    const tooltip = container.querySelector('[data-testid="weapon-tooltip"]')
    expect(tooltip?.textContent).toContain('1d8+3 slashing')
  })

  it('tooltip contains total to-hit bonus', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, characterSheet: SHEET_WITH_MAIN_HAND },
    })
    const { container } = render(<EquipmentDrawer onClose={vi.fn()} />)

    const weaponText = screen.getByText(/Tactical Machete/)
    fireEvent.mouseEnter(weaponText.closest('[data-weapon-slot]') ?? weaponText)

    const tooltip = container.querySelector('[data-testid="weapon-tooltip"]')
    expect(tooltip?.textContent).toContain('+7')
  })

  it('tooltip contains ability bonus and proficiency bonus breakdown', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, characterSheet: SHEET_WITH_MAIN_HAND },
    })
    const { container } = render(<EquipmentDrawer onClose={vi.fn()} />)

    const weaponText = screen.getByText(/Tactical Machete/)
    fireEvent.mouseEnter(weaponText.closest('[data-weapon-slot]') ?? weaponText)

    const tooltip = container.querySelector('[data-testid="weapon-tooltip"]')
    // Should show ability bonus (+3) and proficiency bonus (+4)
    expect(tooltip?.textContent).toContain('+3')
    expect(tooltip?.textContent).toContain('+4')
  })

  it('tooltip disappears on mouse leave', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, characterSheet: SHEET_WITH_MAIN_HAND },
    })
    const { container } = render(<EquipmentDrawer onClose={vi.fn()} />)

    const weaponText = screen.getByText(/Tactical Machete/)
    const target = weaponText.closest('[data-weapon-slot]') ?? weaponText
    fireEvent.mouseEnter(target)
    expect(container.querySelector('[data-testid="weapon-tooltip"]')).not.toBeNull()

    fireEvent.mouseLeave(target)
    expect(container.querySelector('[data-testid="weapon-tooltip"]')).toBeNull()
  })
})
