import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

import { RoomPanel } from './RoomPanel'

function makeState(npcs: object[]) {
  return {
    state: {
      connected: true,
      roomView: {
        roomId: 'test_room',
        title: 'Test Room',
        description: 'A test room.',
        exits: [],
        npcs,
        players: [],
        items: [],
        zone: 'Test Zone',
        dangerLevel: 'safe',
      },
      mapTiles: [],
      worldTiles: [],
      combatRound: null,
      combatPositions: {},
      combatantAP: {},
      combatGridWidth: 10,
      combatGridHeight: 10,
    },
    sendMessage: vi.fn(),
    sendCommand: vi.fn(),
  }
}

beforeEach(() => {
  vi.clearAllMocks()
})

// REQ-MERCHANT-CLICK-1: Clicking a merchant NPC MUST send BrowseRequest.
describe('RoomPanel NPC click routing — merchant', () => {
  it('sends BrowseRequest when clicking a merchant NPC', () => {
    const ctx = makeState([{ id: 'npc1', name: 'Old Rusty', npc_type: 'merchant' }])
    mockUseGame.mockReturnValue(ctx)
    render(<RoomPanel />)
    fireEvent.click(screen.getByText('Old Rusty'))
    expect(ctx.sendMessage).toHaveBeenCalledWith('BrowseRequest', { npc_name: 'Old Rusty' })
    expect(ctx.sendMessage).not.toHaveBeenCalledWith('ExamineRequest', expect.anything())
  })
})

// REQ-MERCHANT-CLICK-2: Clicking a black_market_merchant NPC MUST send BrowseRequest, not ExamineRequest.
describe('RoomPanel NPC click routing — black_market_merchant', () => {
  it('sends BrowseRequest when clicking a black_market_merchant NPC', () => {
    const ctx = makeState([{ id: 'npc2', name: 'Garage Sale Dealer', npc_type: 'black_market_merchant' }])
    mockUseGame.mockReturnValue(ctx)
    render(<RoomPanel />)
    fireEvent.click(screen.getByText('Garage Sale Dealer'))
    expect(ctx.sendMessage).toHaveBeenCalledWith('BrowseRequest', { npc_name: 'Garage Sale Dealer' })
    expect(ctx.sendMessage).not.toHaveBeenCalledWith('ExamineRequest', expect.anything())
  })
})

// REQ-MERCHANT-CLICK-3: Clicking a quest_giver NPC MUST send TalkRequest.
describe('RoomPanel NPC click routing — quest_giver', () => {
  it('sends TalkRequest when clicking a quest_giver NPC', () => {
    const ctx = makeState([{ id: 'npc3', name: 'Captain Marlowe', npc_type: 'quest_giver' }])
    mockUseGame.mockReturnValue(ctx)
    render(<RoomPanel />)
    fireEvent.click(screen.getByText('Captain Marlowe'))
    expect(ctx.sendMessage).toHaveBeenCalledWith('TalkRequest', { npc_name: 'Captain Marlowe' })
  })
})

// REQ-MERCHANT-CLICK-4: Clicking other non-combat NPCs (e.g. healer) MUST send ExamineRequest.
describe('RoomPanel NPC click routing — other non-combat NPC', () => {
  it('sends ExamineRequest when clicking a healer NPC', () => {
    const ctx = makeState([{ id: 'npc4', name: 'Doc Malone', npc_type: 'healer' }])
    mockUseGame.mockReturnValue(ctx)
    render(<RoomPanel />)
    fireEvent.click(screen.getByText('Doc Malone'))
    expect(ctx.sendMessage).toHaveBeenCalledWith('ExamineRequest', { target: 'Doc Malone' })
  })
})
