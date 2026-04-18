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
    inventoryView: null,
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

const EMPTY_SHEET = { name: 'Test Player', level: 1 }

const WEAPON_INVENTORY = {
  items: [
    { name: 'Rusty Blade', kind: 'weapon', itemDefId: 'rusty_blade', quantity: 1, weight: 1.5 },
    { name: 'Iron Knife', kind: 'weapon', itemDefId: 'iron_knife', quantity: 1, weight: 0.8 },
  ],
}

const ARMOR_INVENTORY = {
  items: [
    { name: 'Leather Cap', kind: 'armor', itemDefId: 'leather_cap', armorSlot: 'head', quantity: 1, weight: 0.5 },
    { name: 'Leather Jacket', kind: 'armor', itemDefId: 'leather_jacket', armorSlot: 'torso', quantity: 1, weight: 2.0 },
  ],
}

// REQ-UI-EQUIP-2: Clicking an empty weapon slot MUST show a picker with weapons from inventory.
describe('EquipmentDrawer — empty slot click opens picker', () => {
  it('clicking empty Main Hand slot shows weapon picker', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: {
        ...BASE_STATE.state,
        characterSheet: EMPTY_SHEET,
        inventoryView: WEAPON_INVENTORY,
        loadoutView: { activeIndex: 0, presets: [{ mainHand: null, offHand: null }] },
      },
    })
    render(<EquipmentDrawer onClose={vi.fn()} />)

    // Empty slots show '—' as a clickable button
    const emptySlots = screen.getAllByRole('button', { name: '—' })
    expect(emptySlots.length).toBeGreaterThan(0)
    fireEvent.click(emptySlots[0])

    // Picker should appear with weapon items
    expect(screen.getByTestId('slot-picker')).toBeDefined()
    expect(screen.getByText('Rusty Blade')).toBeDefined()
    expect(screen.getByText('Iron Knife')).toBeDefined()
  })

  it('selecting weapon with single preset sends EquipRequest immediately', () => {
    const ctx = {
      ...BASE_STATE,
      state: {
        ...BASE_STATE.state,
        characterSheet: EMPTY_SHEET,
        inventoryView: WEAPON_INVENTORY,
        loadoutView: { activeIndex: 0, presets: [{ mainHand: null, offHand: null }] },
      },
      sendMessage: vi.fn(),
    }
    mockUseGame.mockReturnValue(ctx)
    render(<EquipmentDrawer onClose={vi.fn()} />)

    const emptySlots = screen.getAllByRole('button', { name: '—' })
    fireEvent.click(emptySlots[0])

    // Click the first weapon item in the picker
    fireEvent.click(screen.getByText('Rusty Blade'))

    expect(ctx.sendMessage).toHaveBeenCalledWith('EquipRequest', {
      weaponId: 'rusty_blade',
      slot: 'main',
      preset: 1,
    })
  })

  it('selecting weapon with multiple presets shows preset picker before sending', () => {
    const ctx = {
      ...BASE_STATE,
      state: {
        ...BASE_STATE.state,
        characterSheet: EMPTY_SHEET,
        inventoryView: WEAPON_INVENTORY,
        loadoutView: {
          activeIndex: 0,
          presets: [
            { mainHand: null, offHand: null },
            { mainHand: null, offHand: null },
          ],
        },
      },
      sendMessage: vi.fn(),
    }
    mockUseGame.mockReturnValue(ctx)
    render(<EquipmentDrawer onClose={vi.fn()} />)

    const emptySlots = screen.getAllByRole('button', { name: '—' })
    fireEvent.click(emptySlots[0])
    fireEvent.click(screen.getByText('Rusty Blade'))

    // Should now show preset picker — findAll because the preset cards also say "Preset N"
    const picker = screen.getByTestId('slot-picker')
    expect(picker).toBeDefined()
    const presetBtns = Array.from(picker.querySelectorAll('button')).filter(b => b.textContent?.startsWith('Preset'))
    expect(presetBtns.length).toBe(2)
    expect(ctx.sendMessage).not.toHaveBeenCalledWith('EquipRequest', expect.anything())

    // Click the Preset 2 button inside the picker
    fireEvent.click(presetBtns[1])
    expect(ctx.sendMessage).toHaveBeenCalledWith('EquipRequest', {
      weaponId: 'rusty_blade',
      slot: 'main',
      preset: 2,
    })
  })

  it('clicking empty armor slot shows armor picker and selecting sends WearRequest', () => {
    const ctx = {
      ...BASE_STATE,
      state: {
        ...BASE_STATE.state,
        characterSheet: EMPTY_SHEET,
        inventoryView: ARMOR_INVENTORY,
        loadoutView: { activeIndex: 0, presets: [] },
      },
      sendMessage: vi.fn(),
    }
    mockUseGame.mockReturnValue(ctx)
    render(<EquipmentDrawer onClose={vi.fn()} />)

    // Click the Head slot empty button (main + off = 2 weapon slots, Head is index 2)
    const emptySlots = screen.getAllByRole('button', { name: '—' })
    fireEvent.click(emptySlots[2]) // first armor empty slot (Head)

    expect(screen.getByTestId('slot-picker')).toBeDefined()
    expect(screen.getByText('Leather Cap')).toBeDefined()

    fireEvent.click(screen.getByText('Leather Cap'))
    expect(ctx.sendMessage).toHaveBeenCalledWith('WearRequest', {
      item_id: 'leather_cap',
      slot: 'head',
    })
  })
})
