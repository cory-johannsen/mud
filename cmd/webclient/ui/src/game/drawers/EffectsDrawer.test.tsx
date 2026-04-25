import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

const sendMessage = vi.fn()

function makeState(effectsSummary: string | undefined | null) {
  const characterSheet = effectsSummary === null ? null : { name: 'Tester', level: 1, effectsSummary: effectsSummary ?? '' }
  return {
    state: { connected: true, characterSheet },
    sendMessage,
    sendCommand: vi.fn(),
  }
}

beforeEach(() => {
  sendMessage.mockClear()
  mockUseGame.mockReturnValue(makeState(null))
})

import { EffectsDrawer } from './EffectsDrawer'

describe('EffectsDrawer', () => {
  it('requests the character sheet on mount when none cached', () => {
    mockUseGame.mockReturnValue(makeState(null))
    render(<EffectsDrawer onClose={() => {}} />)
    expect(sendMessage).toHaveBeenCalledWith('CharacterSheetRequest', {})
  })

  it('renders effects summary from the cached character sheet', () => {
    const summary = 'Effects:\n  Heroism                   (from Kira    )  attack +1 status                       (active)\n'
    mockUseGame.mockReturnValue(makeState(summary))
    render(<EffectsDrawer onClose={() => {}} />)
    // The block is rendered verbatim inside a <pre>, so partial text should match.
    expect(screen.getByText(/Heroism/)).toBeDefined()
    expect(screen.getByText(/attack \+1 status/)).toBeDefined()
  })

  it('shows an empty-state message when effectsSummary has no active effects', () => {
    mockUseGame.mockReturnValue(makeState('Effects:\n  No active effects.\n'))
    render(<EffectsDrawer onClose={() => {}} />)
    expect(screen.getByText(/No active effects/)).toBeDefined()
  })

  it('shows an empty-state message when effectsSummary is blank', () => {
    mockUseGame.mockReturnValue(makeState(''))
    render(<EffectsDrawer onClose={() => {}} />)
    expect(screen.getByText(/No active effects/)).toBeDefined()
  })

  it('renders the Active Effects header', () => {
    mockUseGame.mockReturnValue(makeState(''))
    render(<EffectsDrawer onClose={() => {}} />)
    // Match exactly the h3 header, not the empty-state message which also
    // contains "effects".
    const header = screen.getByRole('heading', { name: /active effects/i })
    expect(header).toBeDefined()
    expect(header.tagName.toLowerCase()).toBe('h3')
  })

  it('calls onClose when the close button is clicked', () => {
    const onClose = vi.fn()
    mockUseGame.mockReturnValue(makeState(''))
    render(<EffectsDrawer onClose={onClose} />)
    const closeBtn = screen.getByRole('button', { name: /close/i })
    fireEvent.click(closeBtn)
    expect(onClose).toHaveBeenCalled()
  })
})
