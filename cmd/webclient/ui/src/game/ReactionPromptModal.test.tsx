import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

vi.mock('./GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from './GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

import { ReactionPromptModal } from './ReactionPromptModal'

const BASE_STATE = {
  state: { reactionPrompt: null },
  sendMessage: vi.fn(),
  clearReactionPrompt: vi.fn(),
}

beforeEach(() => {
  mockUseGame.mockReturnValue(BASE_STATE)
})

// REQ-RXN-MODAL-1: ReactionPromptModal MUST render when state.reactionPrompt is set.
describe('ReactionPromptModal', () => {
  it('renders nothing when reactionPrompt is null', () => {
    const { container } = render(<ReactionPromptModal />)
    expect(container.firstChild).toBeNull()
  })

  it('renders options and a Skip button when a prompt is set', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: {
        reactionPrompt: {
          promptId: 'rxn-123',
          deadlineUnixMs: Date.now() + 3000,
          options: [
            { id: 'chrome_reflex', label: 'Chrome Reflex' },
            { id: 'reactive_block', label: 'Reactive Block' },
          ],
        },
      },
    })
    render(<ReactionPromptModal />)
    expect(screen.getByText('Chrome Reflex')).toBeDefined()
    expect(screen.getByText('Reactive Block')).toBeDefined()
    expect(screen.getByText('Skip')).toBeDefined()
    expect(screen.getByTestId('reaction-countdown')).toBeDefined()
  })

  it('sends ReactionResponse with chosen option id on click', () => {
    const sendMessage = vi.fn()
    const clearReactionPrompt = vi.fn()
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      sendMessage,
      clearReactionPrompt,
      state: {
        reactionPrompt: {
          promptId: 'rxn-xyz',
          deadlineUnixMs: Date.now() + 3000,
          options: [{ id: 'chrome_reflex', label: 'Chrome Reflex' }],
        },
      },
    })
    render(<ReactionPromptModal />)
    fireEvent.click(screen.getByText('Chrome Reflex'))
    expect(sendMessage).toHaveBeenCalledWith('ReactionResponse', {
      prompt_id: 'rxn-xyz',
      chosen: 'chrome_reflex',
    })
    expect(clearReactionPrompt).toHaveBeenCalled()
  })

  it('sends empty chosen string when Skip is clicked', () => {
    const sendMessage = vi.fn()
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      sendMessage,
      state: {
        reactionPrompt: {
          promptId: 'rxn-skip',
          deadlineUnixMs: Date.now() + 3000,
          options: [{ id: 'chrome_reflex', label: 'Chrome Reflex' }],
        },
      },
    })
    render(<ReactionPromptModal />)
    fireEvent.click(screen.getByText('Skip'))
    expect(sendMessage).toHaveBeenCalledWith('ReactionResponse', {
      prompt_id: 'rxn-skip',
      chosen: '',
    })
  })

  it('renders even when only a snake_case prompt_id is provided', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: {
        reactionPrompt: {
          prompt_id: 'rxn-snake',
          deadline_unix_ms: Date.now() + 3000,
          options: [{ id: 'x', label: 'X Reaction' }],
        },
      },
    })
    render(<ReactionPromptModal />)
    expect(screen.getByText('X Reaction')).toBeDefined()
  })
})
