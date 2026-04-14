import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen, act } from '@testing-library/react'

// MapPanel no longer uses react-resizable-panels for the zone view,
// but mock it to be safe if any import remains.
vi.mock('react-resizable-panels', () => ({
  Group: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Panel: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Separator: () => <div />,
}))

// Mock GameContext so MapPanel can render without a real WebSocket.
// useGame is mocked as a vi.fn() so individual tests can override its return value.
vi.mock('../GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../GameContext'
const mockUseGame = useGame as ReturnType<typeof vi.fn>

const ZONE_STATE = {
  state: {
    connected: false,
    roomView: { roomId: 'grinders_row', title: "Grinder's Row" },
    mapTiles: [
      {
        roomId: 'grinders_row',
        roomName: "Grinder's Row",
        x: 0,
        y: 0,
        current: true,
        exits: [],
        dangerLevel: 'safe',
        pois: ['merchant'],
      },
      {
        roomId: 'last_stand_lodge',
        roomName: 'Last Stand Lodge',
        x: 2,
        y: 0,
        current: false,
        exits: [],
        dangerLevel: 'safe',
        pois: [],
      },
    ],
    worldTiles: [],
    combatRound: null,
    combatPositions: {},
    combatantAP: {},
    combatGridWidth: 20,
    combatGridHeight: 20,
  },
  sendMessage: vi.fn(),
  sendCommand: vi.fn(),
}

beforeEach(() => {
  mockUseGame.mockReturnValue(ZONE_STATE)
})

import { MapPanel } from './MapPanel'

// REQ-MAP-TRAVEL-1: After fast travel, MapPanel MUST automatically switch back
// to zone map view even if the world map was active during travel.
describe('MapPanel — fast travel switches back to zone view', () => {
  it('switches from world map to zone map when the room changes after travel', () => {
    const { rerender, container } = render(<MapPanel />)

    // Simulate user switching to world map view.
    const worldBtn = screen.getByText('World')
    fireEvent.click(worldBtn)

    // World map is now active — there should be no zone-map SVG (worldTiles is empty,
    // so WorldMapSvg renders a "No map data" placeholder).
    const zoneHeader = container.querySelector('h3')
    expect(zoneHeader?.textContent).toBe('World Map')

    // Simulate travel completing: the room ID changes.
    mockUseGame.mockReturnValue({
      ...ZONE_STATE,
      state: {
        ...ZONE_STATE.state,
        roomView: { roomId: 'vantucky_plaza', title: 'Vantucky Plaza' },
      },
    })
    act(() => {
      rerender(<MapPanel />)
    })

    // MapPanel must have switched back to zone view.
    const header = container.querySelector('h3')
    expect(header?.textContent).toBe('Zone Map')
  })
})

describe('MapPanel zone map — SVG rendering', () => {
  it('renders SVG with rects for each tile', () => {
    const { container } = render(<MapPanel />)
    const rects = container.querySelectorAll('svg rect:not(defs rect)')
    expect(rects.length).toBe(2)
  })

  it('renders current room tile with gold stroke', () => {
    const { container } = render(<MapPanel />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    const currentRect = rects.find(r => r.getAttribute('stroke') === '#f0c040')
    expect(currentRect).toBeDefined()
  })

  it('shows tooltip on SVG tile hover', () => {
    const { container } = render(<MapPanel />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    const currentRect = rects.find(r => r.getAttribute('stroke') === '#f0c040')
    expect(currentRect).not.toBeNull()
    fireEvent.mouseEnter(currentRect!)
    // Tooltip portal renders room name — at least one match exists
    expect(screen.queryAllByText("Grinder's Row").length).toBeGreaterThan(0)
  })

  it('renders room names in SVG text elements', () => {
    const { container } = render(<MapPanel />)
    // Names may be word-wrapped across tspans; check the text container's full textContent
    const allText = container.querySelector('svg')?.textContent ?? ''
    expect(allText).toContain("Grinder")
    expect(allText).toContain("Last")
  })
})
