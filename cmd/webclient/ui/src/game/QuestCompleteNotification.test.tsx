import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

vi.mock('./GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from './GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

const dismissQuestComplete = vi.fn()

function makeState(queue: object[]) {
  return {
    state: { questCompleteQueue: queue },
    dismissQuestComplete,
  }
}

beforeEach(() => {
  dismissQuestComplete.mockClear()
  mockUseGame.mockReturnValue(makeState([]))
})

import { QuestCompleteNotification } from './QuestCompleteNotification'

// REQ-QCN-1: notification renders nothing when queue is empty.
describe('QuestCompleteNotification', () => {
  it('renders nothing when queue is empty', () => {
    const { container } = render(<QuestCompleteNotification />)
    expect(container.firstChild).toBeNull()
  })

  it('renders the quest title', () => {
    mockUseGame.mockReturnValue(makeState([{ title: 'Scavenger Sweep', xpReward: 150, creditsReward: 75 }]))
    render(<QuestCompleteNotification />)
    expect(screen.getByText('Scavenger Sweep')).toBeDefined()
  })

  it('shows XP reward', () => {
    mockUseGame.mockReturnValue(makeState([{ title: 'Test', xpReward: 200, creditsReward: 0 }]))
    render(<QuestCompleteNotification />)
    expect(screen.getByText(/200/)).toBeDefined()
    expect(screen.getByText(/XP/)).toBeDefined()
  })

  it('shows credits reward when non-zero', () => {
    mockUseGame.mockReturnValue(makeState([{ title: 'Test', xpReward: 100, creditsReward: 50 }]))
    render(<QuestCompleteNotification />)
    expect(screen.getByText(/50/)).toBeDefined()
    expect(screen.getByText(/Credits/)).toBeDefined()
  })

  it('shows item rewards when present', () => {
    mockUseGame.mockReturnValue(makeState([{ title: 'Test', xpReward: 0, creditsReward: 0, itemRewards: ['Combat Stims x2'] }]))
    render(<QuestCompleteNotification />)
    expect(screen.getByText('+Combat Stims x2')).toBeDefined()
  })

  it('calls dismissQuestComplete when dismiss button is clicked', () => {
    mockUseGame.mockReturnValue(makeState([{ title: 'Test', xpReward: 100, creditsReward: 0 }]))
    render(<QuestCompleteNotification />)
    const btn = screen.getByRole('button', { name: /dismiss/i })
    fireEvent.click(btn)
    expect(dismissQuestComplete).toHaveBeenCalledOnce()
  })

  it('shows only the first queued event when multiple are queued', () => {
    mockUseGame.mockReturnValue(makeState([
      { title: 'First Quest', xpReward: 100, creditsReward: 0 },
      { title: 'Second Quest', xpReward: 200, creditsReward: 0 },
    ]))
    render(<QuestCompleteNotification />)
    expect(screen.getByText('First Quest')).toBeDefined()
    expect(screen.queryByText('Second Quest')).toBeNull()
  })
})
