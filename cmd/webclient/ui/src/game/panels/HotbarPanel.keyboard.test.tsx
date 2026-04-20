import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent } from '@testing-library/react'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

import { HotbarPanel } from './HotbarPanel'

function makeSlot(ref: string) {
  return { kind: 'command', ref }
}

let mockSendCommand: ReturnType<typeof vi.fn>

function setupGame(slots: object[] = [], choicePrompt: object | null = null) {
  const filledSlots = Array.from({ length: 10 }, (_, i) => slots[i] ?? { kind: 'command', ref: '' })
  mockSendCommand = vi.fn()
  mockUseGame.mockReturnValue({
    state: {
      hotbarSlots: filledSlots,
      combatRound: null,
      characterInfo: null,
      choicePrompt,
    },
    sendCommand: mockSendCommand,
    sendMessage: vi.fn(),
  })
}

beforeEach(() => {
  vi.clearAllMocks()
})

// REQ-HKB-1: Digit keys 1-9 must activate the corresponding hotbar slot.
// REQ-HKB-2: Keypresses must NOT be forwarded to any focused input.
describe('HotbarPanel keyboard shortcuts', () => {
  it('pressing "1" on document body activates slot 0', () => {
    setupGame([makeSlot('look')])
    render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: '1' })
    expect(mockSendCommand).toHaveBeenCalledWith('look')
  })

  it('pressing "2" activates slot 1', () => {
    setupGame([makeSlot(''), makeSlot('north')])
    render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: '2' })
    expect(mockSendCommand).toHaveBeenCalledWith('north')
  })

  it('pressing "0" activates slot 9 (index 9, the 10th slot)', () => {
    const slots = Array.from({ length: 10 }, (_, i) =>
      i === 9 ? makeSlot('flee') : makeSlot(''),
    )
    setupGame(slots)
    render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: '0' })
    expect(mockSendCommand).toHaveBeenCalledWith('flee')
  })

  it('does not fire sendCommand when slot is empty', () => {
    setupGame([{ kind: 'command', ref: '' }])
    render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: '1' })
    expect(mockSendCommand).not.toHaveBeenCalled()
  })

  it('does not intercept keys in a non-prompt input (e.g. EditPopup input)', () => {
    setupGame([makeSlot('look')])
    render(
      <>
        <HotbarPanel />
        <input className="other-input" defaultValue="" data-testid="other" />
      </>,
    )
    const other = document.querySelector<HTMLInputElement>('.other-input')!
    other.focus()
    // Fire keyDown on document; the handler checks document.activeElement.
    // other-input lacks class input-field so the handler must return early.
    fireEvent.keyDown(document, { key: '1' })
    expect(mockSendCommand).not.toHaveBeenCalled()
  })

  it('intercepts digit keys when the prompt input (class input-field) is focused', () => {
    setupGame([makeSlot('look')])
    render(
      <>
        <HotbarPanel />
        <input className="input-field" defaultValue="" data-testid="prompt" />
      </>,
    )
    const prompt = document.querySelector<HTMLInputElement>('.input-field')!
    prompt.focus()
    fireEvent.keyDown(document, { key: '1' })
    expect(mockSendCommand).toHaveBeenCalledWith('look')
  })

  it('does not intercept non-digit keys', () => {
    setupGame([makeSlot('look')])
    render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: 'a' })
    fireEvent.keyDown(document, { key: 'Enter' })
    fireEvent.keyDown(document, { key: 'ArrowUp' })
    expect(mockSendCommand).not.toHaveBeenCalled()
  })

  // REQ-HKB-3: When choicePrompt is non-null (FeatureChoiceModal open), hotbar MUST NOT
  // intercept digit keys — they belong to the modal selection flow.
  it('does not intercept digit keys when choicePrompt is set (FeatureChoiceModal open)', () => {
    const prompt = { featureId: 'tech_choice', prompt: 'Choose:', options: ['[shock_wave] Shock Wave'] }
    setupGame([makeSlot('look')], prompt)
    render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: '1' })
    expect(mockSendCommand).not.toHaveBeenCalled()
  })

  it('resumes intercepting digit keys after choicePrompt becomes null', () => {
    const prompt = { featureId: 'tech_choice', prompt: 'Choose:', options: ['[shock_wave] Shock Wave'] }
    setupGame([makeSlot('look')], prompt)
    const { unmount } = render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: '1' })
    expect(mockSendCommand).not.toHaveBeenCalled()
    unmount()

    // Re-render with no choicePrompt
    setupGame([makeSlot('look')], null)
    render(<HotbarPanel />)
    fireEvent.keyDown(document, { key: '1' })
    expect(mockSendCommand).toHaveBeenCalledWith('look')
  })
})
