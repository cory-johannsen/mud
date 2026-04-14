import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

vi.mock('./GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from './GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

const BASE_STATE = {
  state: {
    questGiverView: null,
  },
  sendMessage: vi.fn(),
  sendCommand: vi.fn(),
  clearQuestGiverView: vi.fn(),
}

const QUEST_GIVER_VIEW = {
  npcName: 'Dispatch Board Operator',
  npcInstanceId: 'inst_001',
  quests: [
    {
      questId: 'rrq_scavenger_sweep',
      title: 'Scavenger Sweep',
      description: 'Go put five scavengers down.',
      xpReward: 150,
      creditsReward: 75,
      status: 'available',
      objectives: [
        { id: 'kill_scavengers', description: 'Kill 5 scavengers', current: 0, required: 5 },
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
        { id: 'kill_raiders', description: 'Kill 5 rail gang raiders', current: 2, required: 5 },
      ],
    },
    {
      questId: 'rrq_barrel_house_cleanup',
      title: 'Barrel House Cleanup',
      description: 'Put two enforcers down.',
      xpReward: 250,
      creditsReward: 125,
      status: 'completed',
      objectives: [
        { id: 'kill_enforcers', description: 'Kill 2 Barrel House Enforcers', current: 2, required: 2 },
      ],
    },
    {
      questId: 'rrq_take_down_big_grizz',
      title: 'Take Down Big Grizz',
      description: 'Show us you can.',
      xpReward: 500,
      creditsReward: 300,
      status: 'locked',
      objectives: [
        { id: 'kill_big_grizz', description: 'Kill Big Grizz', current: 0, required: 1 },
      ],
    },
  ],
}

beforeEach(() => {
  mockUseGame.mockReturnValue(BASE_STATE)
})

import { QuestGiverModal } from './QuestGiverModal'

// REQ-QGM-1: Quest giver modal MUST render when questGiverView is set.
describe('QuestGiverModal', () => {
  it('renders nothing when questGiverView is null', () => {
    const { container } = render(<QuestGiverModal />)
    expect(container.firstChild).toBeNull()
  })

  it('renders NPC name in modal title', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { questGiverView: QUEST_GIVER_VIEW },
    })
    render(<QuestGiverModal />)
    expect(screen.getByText('Dispatch Board Operator')).toBeDefined()
  })

  it('lists all quest titles', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { questGiverView: QUEST_GIVER_VIEW },
    })
    render(<QuestGiverModal />)
    expect(screen.getByText('Scavenger Sweep')).toBeDefined()
    expect(screen.getByText('Rail Gang Bounty')).toBeDefined()
    expect(screen.getByText('Barrel House Cleanup')).toBeDefined()
    expect(screen.getByText('Take Down Big Grizz')).toBeDefined()
  })

  it('shows Accept button only for available quests', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { questGiverView: QUEST_GIVER_VIEW },
    })
    render(<QuestGiverModal />)
    // Only "available" quest should have Accept button
    const acceptBtns = screen.getAllByText('Accept')
    expect(acceptBtns).toHaveLength(1)
  })

  it('shows objective progress for active quests', () => {
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      state: { questGiverView: QUEST_GIVER_VIEW },
    })
    render(<QuestGiverModal />)
    // Active quest has 2/5 progress
    expect(screen.getByText(/2\s*\/\s*5/)).toBeDefined()
  })

  it('dispatches TalkRequest with accept args when Accept is clicked', () => {
    const sendMessage = vi.fn()
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      sendMessage,
      state: { questGiverView: QUEST_GIVER_VIEW },
    })
    render(<QuestGiverModal />)
    const acceptBtn = screen.getByText('Accept')
    fireEvent.click(acceptBtn)
    expect(sendMessage).toHaveBeenCalledWith('TalkRequest', {
      npc_name: 'Dispatch Board Operator',
      args: 'accept rrq_scavenger_sweep',
    })
  })

  it('calls clearQuestGiverView when close button is clicked', () => {
    const clearQuestGiverView = vi.fn()
    mockUseGame.mockReturnValue({
      ...BASE_STATE,
      clearQuestGiverView,
      state: { questGiverView: QUEST_GIVER_VIEW },
    })
    render(<QuestGiverModal />)
    const closeBtn = screen.getByText('✕')
    fireEvent.click(closeBtn)
    expect(clearQuestGiverView).toHaveBeenCalled()
  })
})
