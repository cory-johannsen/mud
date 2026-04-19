import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

const sendMessage = vi.fn()

const ACTIVE_QUESTS = [
  {
    questId: 'rrq_scavenger_sweep',
    title: 'Scavenger Sweep',
    description: 'Go put five scavengers down.',
    xpReward: 150,
    creditsReward: 75,
    status: 'active',
    objectives: [
      { id: 'kill_scavengers', description: 'Kill 5 scavengers', current: 2, required: 5 },
    ],
  },
  {
    questId: 'rrq_rail_gang_bounty',
    title: 'Rail Gang Bounty',
    description: 'Take out five raiders.',
    xpReward: 200,
    creditsReward: 100,
    status: 'active',
    objectives: [
      { id: 'kill_raiders', description: 'Kill 5 rail gang raiders', current: 5, required: 5 },
    ],
  },
]

function makeState(quests: typeof ACTIVE_QUESTS | null) {
  return {
    state: { questLogView: quests === null ? null : { quests }, questCompleteQueue: [] },
    sendMessage,
  }
}

beforeEach(() => {
  sendMessage.mockClear()
  mockUseGame.mockReturnValue(makeState(null))
})

import { QuestsDrawer } from './QuestsDrawer'

describe('QuestsDrawer', () => {
  it('requests quest log on mount', () => {
    mockUseGame.mockReturnValue(makeState(null))
    render(<QuestsDrawer onClose={() => {}} />)
    expect(sendMessage).toHaveBeenCalledWith('QuestLogRequest', {})
  })

  it('shows loading state when questLogView is null', () => {
    mockUseGame.mockReturnValue(makeState(null))
    render(<QuestsDrawer onClose={() => {}} />)
    expect(screen.getByText(/loading/i)).toBeDefined()
  })

  it('shows empty message when no active quests', () => {
    mockUseGame.mockReturnValue(makeState([]))
    render(<QuestsDrawer onClose={() => {}} />)
    expect(screen.getByText(/no active quests/i)).toBeDefined()
  })

  it('renders quest titles', () => {
    mockUseGame.mockReturnValue(makeState(ACTIVE_QUESTS))
    render(<QuestsDrawer onClose={() => {}} />)
    expect(screen.getByText('Scavenger Sweep')).toBeDefined()
    expect(screen.getByText('Rail Gang Bounty')).toBeDefined()
  })

  it('renders quest descriptions', () => {
    mockUseGame.mockReturnValue(makeState(ACTIVE_QUESTS))
    render(<QuestsDrawer onClose={() => {}} />)
    expect(screen.getByText('Go put five scavengers down.')).toBeDefined()
  })

  it('renders objective progress for each active quest', () => {
    mockUseGame.mockReturnValue(makeState(ACTIVE_QUESTS))
    render(<QuestsDrawer onClose={() => {}} />)
    expect(screen.getByText(/2\s*\/\s*5/)).toBeDefined()
    expect(screen.getByText(/5\s*\/\s*5/)).toBeDefined()
  })

  it('renders XP and credit rewards', () => {
    mockUseGame.mockReturnValue(makeState(ACTIVE_QUESTS))
    render(<QuestsDrawer onClose={() => {}} />)
    expect(screen.getByText(/150/)).toBeDefined()
    expect(screen.getByText(/75/)).toBeDefined()
  })

  it('calls onClose when close button is clicked', () => {
    const onClose = vi.fn()
    mockUseGame.mockReturnValue(makeState(ACTIVE_QUESTS))
    render(<QuestsDrawer onClose={onClose} />)
    const closeBtn = screen.getByRole('button', { name: /close/i })
    fireEvent.click(closeBtn)
    expect(onClose).toHaveBeenCalled()
  })
})
