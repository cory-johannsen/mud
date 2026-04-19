import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

// sendCommand and clearChoicePrompt mocks are fresh per test via beforeEach.
let mockSendCommand: ReturnType<typeof vi.fn>
let mockClearChoicePrompt: ReturnType<typeof vi.fn>

const BASE_STATE = {
  state: {
    connected: false,
    characterSheet: null,
    hotbarSlots: [],
    choicePrompt: null,
  },
  sendMessage: vi.fn(),
}

function makePrompt(options: string[], slotContext?: { slotNum: number; totalSlots: number; slotLevel: number }) {
  return {
    featureId: 'tech_choice',
    prompt: 'Choose a technology:',
    options,
    slotContext,
  }
}

beforeEach(() => {
  mockSendCommand = vi.fn()
  mockClearChoicePrompt = vi.fn()
  vi.clearAllMocks()
})

import { FeatureChoiceModal } from './FeatureChoiceModal'

// REQ-FCM-3: Clicking a selectable option sends the 1-based original option index as CommandText.
describe('FeatureChoiceModal option selection', () => {
  it('sends the 1-based original index for the first option (no sentinels)', () => {
    const prompt = makePrompt(['[shock_wave] Shock Wave (Lv 1) — desc'])
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, choicePrompt: prompt },
      sendCommand: mockSendCommand,
      clearChoicePrompt: mockClearChoicePrompt,
    })
    render(<FeatureChoiceModal />)

    fireEvent.click(screen.getByText('Shock Wave (Lv 1) — desc'))
    expect(mockSendCommand).toHaveBeenCalledWith('1')
    expect(mockClearChoicePrompt).toHaveBeenCalledOnce()
  })

  it('sends originalIdx+1 when [back] sentinel shifts the option to index 1', () => {
    // Options: [back] at 0, L1 tech at 1, [confirm] at 2
    const prompt = makePrompt(
      ['[back]', '[shock_wave] Shock Wave (Lv 1) — desc', '[confirm]'],
      { slotNum: 2, totalSlots: 2, slotLevel: 2 },
    )
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, choicePrompt: prompt },
      sendCommand: mockSendCommand,
      clearChoicePrompt: mockClearChoicePrompt,
    })
    render(<FeatureChoiceModal />)

    // The displayed button says "1." (filteredIdx 0) but sends "2" (originalIdx 1)
    fireEvent.click(screen.getByText('Shock Wave (Lv 1) — desc'))
    expect(mockSendCommand).toHaveBeenCalledWith('2')
  })

  // REQ-FCM-10: Double-clicking MUST NOT send the command twice.
  it('ignores a second click after the first (double-submit prevention)', () => {
    const prompt = makePrompt(['[shock_wave] Shock Wave (Lv 1) — desc', '[forward]'])
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, choicePrompt: prompt },
      sendCommand: mockSendCommand,
      clearChoicePrompt: mockClearChoicePrompt,
    })
    render(<FeatureChoiceModal />)

    const btn = screen.getByText('Shock Wave (Lv 1) — desc')
    fireEvent.click(btn)
    fireEvent.click(btn) // second click — must be ignored
    expect(mockSendCommand).toHaveBeenCalledTimes(1)
  })

  // REQ-FCM-10: Double-clicking a navigation button MUST NOT send twice.
  it('ignores a second click on a navigation sentinel (forward)', () => {
    const prompt = makePrompt(['[shock_wave] Shock Wave (Lv 1) — desc', '[forward]'])
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { ...BASE_STATE.state, choicePrompt: prompt },
      sendCommand: mockSendCommand,
      clearChoicePrompt: mockClearChoicePrompt,
    })
    render(<FeatureChoiceModal />)

    const fwdBtn = screen.getByText('Next →')
    fireEvent.click(fwdBtn)
    fireEvent.click(fwdBtn) // second click — must be ignored
    expect(mockSendCommand).toHaveBeenCalledTimes(1)
  })
})
