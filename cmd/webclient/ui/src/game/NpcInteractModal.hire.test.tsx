// Tests for HireModal: shown when state.npcView.npcType === 'hireling'
// REQ-HIRE-1: NpcInteractModal MUST render HireModal when npcView.npcType is 'hireling'.
// REQ-HIRE-2: HireModal MUST display the NPC name, description, and level.
// REQ-HIRE-3: Clicking the Hire button MUST send HireRequest with the NPC name.
// REQ-HIRE-4: Clicking Hire MUST close the modal (clearNpcView).
// REQ-HIRE-5: Clicking Cancel MUST close the modal without sending HireRequest.
// REQ-HIRE-6: Clicking the overlay MUST close the modal without sending HireRequest.
// REQ-HIRE-7: NpcInteractModal MUST render GenericNpcModal for non-hireling npcType values.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

vi.mock('./GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from './GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

import { NpcInteractModal } from './NpcInteractModal'

const mockClearNpcView = vi.fn()
const mockSendMessage = vi.fn()
const mockSendCommand = vi.fn()

function makeNpcView(overrides: Partial<{
  name: string
  description: string
  npcType: string
  level: number
  health: string
}> = {}) {
  return {
    name: 'Conscript',
    description: 'A wiry ex-soldier with steady hands and hollow eyes.',
    npcType: 'hireling',
    level: 3,
    health: 'Healthy',
    ...overrides,
  }
}

function setup(npcView: ReturnType<typeof makeNpcView> | null = null) {
  mockUseGame.mockReturnValue({
    state: {
      healerView: null,
      trainerView: null,
      techTrainerView: null,
      fixerView: null,
      restView: null,
      npcView,
    },
    sendMessage: mockSendMessage,
    sendCommand: mockSendCommand,
    clearHealer: vi.fn(),
    clearTrainer: vi.fn(),
    clearTechTrainer: vi.fn(),
    clearFixer: vi.fn(),
    clearRestView: vi.fn(),
    clearNpcView: mockClearNpcView,
  })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('NpcInteractModal — hireling (HireModal)', () => {
  it('renders HireModal when npcType is hireling', () => {
    setup(makeNpcView())
    render(<NpcInteractModal />)
    expect(screen.getByText('Conscript')).toBeTruthy()
    expect(screen.getByRole('button', { name: /hire/i })).toBeTruthy()
  })

  it('displays NPC name in modal header', () => {
    setup(makeNpcView({ name: 'Hired Gun' }))
    render(<NpcInteractModal />)
    expect(screen.getByText('Hired Gun')).toBeTruthy()
  })

  it('displays NPC description', () => {
    setup(makeNpcView())
    render(<NpcInteractModal />)
    expect(screen.getByText('A wiry ex-soldier with steady hands and hollow eyes.')).toBeTruthy()
  })

  it('displays NPC level', () => {
    setup(makeNpcView({ level: 5 }))
    render(<NpcInteractModal />)
    expect(screen.getByText('5')).toBeTruthy()
  })

  it('clicking Hire sends HireRequest with npc_name', () => {
    setup(makeNpcView({ name: 'Conscript' }))
    render(<NpcInteractModal />)
    fireEvent.click(screen.getByRole('button', { name: /hire/i }))
    expect(mockSendMessage).toHaveBeenCalledWith('HireRequest', { npc_name: 'Conscript' })
  })

  it('clicking Hire closes the modal', () => {
    setup(makeNpcView())
    render(<NpcInteractModal />)
    fireEvent.click(screen.getByRole('button', { name: /hire/i }))
    expect(mockClearNpcView).toHaveBeenCalled()
  })

  it('clicking Cancel closes the modal without sending HireRequest', () => {
    setup(makeNpcView())
    render(<NpcInteractModal />)
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(mockClearNpcView).toHaveBeenCalled()
    expect(mockSendMessage).not.toHaveBeenCalled()
  })

  it('clicking the overlay closes the modal without sending HireRequest', () => {
    setup(makeNpcView())
    const { container } = render(<NpcInteractModal />)
    // The overlay is the outermost div — click it directly
    const overlay = container.firstChild as HTMLElement
    fireEvent.click(overlay)
    expect(mockClearNpcView).toHaveBeenCalled()
    expect(mockSendMessage).not.toHaveBeenCalled()
  })

  it('renders GenericNpcModal (no Hire button) for non-hireling npcType', () => {
    setup(makeNpcView({ npcType: 'guard' }))
    render(<NpcInteractModal />)
    expect(screen.queryByRole('button', { name: /hire/i })).toBeNull()
  })

  it('renders nothing when npcView is null', () => {
    setup(null)
    const { container } = render(<NpcInteractModal />)
    expect(container.firstChild).toBeNull()
  })
})
