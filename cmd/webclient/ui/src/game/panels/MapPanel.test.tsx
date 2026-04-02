import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

// Mock GameContext so MapPanel can render without a real WebSocket.
vi.mock('../GameContext', () => ({
  useGame: () => ({
    state: {
      connected: false,
      mapTiles: [
        {
          roomId: 'grinders_row',
          roomName: "Grinder's Row",
          x: 0,
          y: 0,
          current: true,
          exits: ['east'],
          dangerLevel: 'safe',
          pois: ['merchant'],
        },
        {
          roomId: 'last_stand_lodge',
          roomName: 'Last Stand Lodge',
          x: 2,
          y: 0,
          current: false,
          exits: ['west'],
          dangerLevel: 'safe',
          pois: [],
        },
      ],
      worldTiles: [],
      combatRound: null,
      combatPositions: {},
    },
    sendMessage: vi.fn(),
    sendCommand: vi.fn(),
  }),
}))

import { MapPanel } from './MapPanel'

describe('MapPanel room hover tooltip', () => {
  it('shows no tooltip initially', () => {
    render(<MapPanel />)
    // The tooltip uniquely shows "current room" badge for the current room
    expect(screen.queryByText('current room')).toBeNull()
  })

  it('shows tooltip with room name on mouse enter', () => {
    render(<MapPanel />)
    // Room cells are rendered as spans with data-room attribute
    const roomSpan = document.querySelector('[data-room="grinders_row"]')
    expect(roomSpan).not.toBeNull()
    fireEvent.mouseEnter(roomSpan!, { clientX: 100, clientY: 200 })
    // Tooltip uniquely shows "current room" badge for the current room
    expect(screen.getByText('current room')).toBeDefined()
  })

  it('hides tooltip on mouse leave', () => {
    render(<MapPanel />)
    const roomSpan = document.querySelector('[data-room="grinders_row"]')!
    fireEvent.mouseEnter(roomSpan, { clientX: 100, clientY: 200 })
    expect(screen.getByText('current room')).toBeDefined()
    fireEvent.mouseLeave(roomSpan)
    expect(screen.queryByText('current room')).toBeNull()
  })
})
