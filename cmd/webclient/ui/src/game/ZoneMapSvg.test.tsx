import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { ZoneMapSvg, computeZoneMapLayout, REFERENCE_W } from './ZoneMapSvg'
import type { MapTile } from '../proto'

const CURRENT_TILE: MapTile = {
  roomId: 'room_current',
  roomName: 'Current Room',
  x: 0,
  y: 0,
  exits: ['e'],
  pois: [],
  current: true,
  bossRoom: false,
  // Connector to boss via sameZoneExitTargets (required for line rendering).
  sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'room_boss' }],
}

const BOSS_TILE: MapTile = {
  roomId: 'room_boss',
  roomName: 'Boss Chamber',
  x: 2,
  y: 0,
  exits: ['w'],
  pois: [],
  current: false,
  bossRoom: true,
  sameZoneExitTargets: [{ direction: 'west', targetRoomId: 'room_current' }],
}

const MERCHANT_TILE: MapTile = {
  roomId: 'room_merchant',
  roomName: 'Market Hall',
  x: 0,
  y: 2,
  exits: [],
  pois: ['merchant'],
  current: false,
  bossRoom: false,
}

const ALL_TILES: MapTile[] = [CURRENT_TILE, BOSS_TILE, MERCHANT_TILE]

describe('ZoneMapSvg', () => {
  it('renders one <rect> per tile (excluding defs/clipPath rects)', () => {
    const { container: c } = render(<ZoneMapSvg tiles={ALL_TILES} />)
    // Exclude rects inside <defs> (used for clipPaths) — only count visible tile rects
    const rects = c.querySelectorAll('svg rect:not(defs rect)')
    expect(rects.length).toBe(3)
  })

  it('renders current tile rect with gold stroke', () => {
    const { container: c } = render(<ZoneMapSvg tiles={ALL_TILES} />)
    const rects = Array.from(c.querySelectorAll('rect'))
    const currentRect = rects.find(r => r.getAttribute('stroke') === '#f0c040')
    expect(currentRect).toBeDefined()
  })

  it('renders boss tile rect with red stroke', () => {
    const { container: c } = render(<ZoneMapSvg tiles={ALL_TILES} />)
    const rects = Array.from(c.querySelectorAll('rect'))
    const bossRect = rects.find(r => r.getAttribute('stroke') === '#cc4444')
    expect(bossRect).toBeDefined()
  })

  it('renders a <text> containing $ for merchant POI', () => {
    const { container: c } = render(<ZoneMapSvg tiles={ALL_TILES} />)
    const texts = Array.from(c.querySelectorAll('text'))
    const merchantText = texts.find(t => t.textContent?.includes('$'))
    expect(merchantText).toBeDefined()
  })

  // REQ-MAP-CONN-1: Connectors MUST be drawn between rooms that share sameZoneExitTargets.
  it('renders at least one <line> connecting tiles that share sameZoneExitTargets', () => {
    // CURRENT_TILE and BOSS_TILE both declare sameZoneExitTargets pointing at each other.
    const { container: c } = render(<ZoneMapSvg tiles={[CURRENT_TILE, BOSS_TILE]} />)
    const lines = c.querySelectorAll('line')
    expect(lines.length).toBeGreaterThanOrEqual(1)
  })

  // REQ-MAP-POI-1: Multiple POI symbols MUST be rendered without whitespace between them,
  //   so all indicators fit within their cell's boundaries.
  it('renders multiple POI symbols packed into a single text element with no overflow', () => {
    const multiPoiTile: MapTile = {
      roomId: 'room_multi_poi',
      roomName: 'Hub',
      x: 0,
      y: 0,
      exits: [],
      pois: ['merchant', 'healer', 'trainer', 'quest_giver', 'banker'],
      current: false,
      bossRoom: false,
    }
    const { container: c } = render(<ZoneMapSvg tiles={[multiPoiTile]} />)
    // All POI symbols MUST live inside a single <text> element (via <tspan> children)
    // so they are packed together without spacing — preventing overflow.
    // Find the text element containing the merchant '$' symbol.
    const merchantText = Array.from(c.querySelectorAll('text')).find(t =>
      t.textContent?.includes('$'),
    )
    expect(merchantText).toBeDefined()
    // The same element must also contain all other POI symbols packed adjacently.
    expect(merchantText?.textContent).toContain('+')
    expect(merchantText?.textContent).toContain('T')
    expect(merchantText?.textContent).toContain('!')
    expect(merchantText?.textContent).toContain('¤')
  })
})

// REQ-MAP-SCALE-1: computeZoneMapLayout MUST produce larger gap growth than cell growth
//   as containerW increases, so expanding the pane primarily widens spacing.
describe('computeZoneMapLayout', () => {
  it('returns base values at REFERENCE_W', () => {
    const { cellW, gap } = computeZoneMapLayout(REFERENCE_W)
    // At reference width the values should be the base constants.
    expect(cellW).toBeGreaterThan(0)
    expect(gap).toBeGreaterThan(0)
  })

  it('gap grows faster than cell size as containerW doubles', () => {
    const base = computeZoneMapLayout(REFERENCE_W)
    const doubled = computeZoneMapLayout(REFERENCE_W * 2)

    const gapGrowth = doubled.gap / base.gap
    const cellGrowth = doubled.cellW / base.cellW

    expect(gapGrowth).toBeGreaterThan(cellGrowth)
  })

  it('gap grows faster than cell size as containerW quadruples', () => {
    const base = computeZoneMapLayout(REFERENCE_W)
    const quad = computeZoneMapLayout(REFERENCE_W * 4)

    const gapGrowth = quad.gap / base.gap
    const cellGrowth = quad.cellW / base.cellW

    expect(gapGrowth).toBeGreaterThan(cellGrowth)
  })

  it('values do not decrease for containers narrower than REFERENCE_W', () => {
    const narrow = computeZoneMapLayout(REFERENCE_W / 2)
    const base = computeZoneMapLayout(REFERENCE_W)

    // Must not shrink below base values when container is smaller.
    expect(narrow.cellW).toBe(base.cellW)
    expect(narrow.gap).toBe(base.gap)
  })
})
