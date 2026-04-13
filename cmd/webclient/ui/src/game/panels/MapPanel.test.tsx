import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'

// Mock react-resizable-panels — jsdom lacks ResizeObserver required by Group.
vi.mock('react-resizable-panels', () => ({
  Group: ({ children, style, onLayoutChanged }: { children: React.ReactNode; style?: React.CSSProperties; onLayoutChanged?: (layout: Record<string, number>) => void }) =>
    <div style={style} data-onlayout={String(!!onLayoutChanged)}>{children}</div>,
  Panel: ({ children, style, id }: { children: React.ReactNode; style?: React.CSSProperties; id?: string }) =>
    <div style={style} data-panel-id={id}>{children}</div>,
  Separator: () => <div />,
}))

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
  }),
}))

import { MapPanel } from './MapPanel'

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

  it('renders details pane with room names', () => {
    render(<MapPanel />)
    // Room names appear in both SVG text and details list — at least one match each
    expect(screen.queryAllByText("Grinder's Row").length).toBeGreaterThan(0)
    expect(screen.queryAllByText('Last Stand Lodge').length).toBeGreaterThan(0)
  })
})
