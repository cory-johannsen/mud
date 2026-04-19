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
  it('renders at least one connector path connecting tiles that share sameZoneExitTargets', () => {
    // CURRENT_TILE and BOSS_TILE both declare sameZoneExitTargets pointing at each other.
    const { container: c } = render(<ZoneMapSvg tiles={[CURRENT_TILE, BOSS_TILE]} />)
    const paths = c.querySelectorAll('path')
    expect(paths.length).toBeGreaterThanOrEqual(1)
  })

  // REQ-MAP-CONN-2: Adjacent connectors MUST use straight-line paths (M L command).
  it('renders straight-line path for directly adjacent room pair', () => {
    const tileA: MapTile = { roomId: 'a', roomName: 'A', x: 0, y: 0, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'b' }] }
    const tileB: MapTile = { roomId: 'b', roomName: 'B', x: 1, y: 0, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [{ direction: 'west', targetRoomId: 'a' }] }
    const { container: c } = render(<ZoneMapSvg tiles={[tileA, tileB]} />)
    const paths = Array.from(c.querySelectorAll('path'))
    expect(paths.length).toBeGreaterThanOrEqual(1)
    // Adjacent rooms: path d attribute should use M...L (straight line, no Q)
    const connectorPath = paths[0]
    expect(connectorPath.getAttribute('d')).toMatch(/^M .* L /)
  })

  // REQ-MAP-CONN-3: Non-adjacent connectors MUST use curved paths (M Q command) to arc around intermediate cells.
  it('renders curved path (Q) for non-adjacent room pair', () => {
    // Create three tiles: A at x=0, Middle at x=1 (intermediate, no direct connection to C),
    // C at x=2 — with a connection from A directly to C skipping the middle tile.
    const tileA: MapTile = { roomId: 'far_a', roomName: 'A', x: 0, y: 0, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'far_c' }] }
    const tileMiddle: MapTile = { roomId: 'far_mid', roomName: 'Mid', x: 1, y: 0, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [] }
    const tileC: MapTile = { roomId: 'far_c', roomName: 'C', x: 2, y: 0, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [{ direction: 'west', targetRoomId: 'far_a' }] }
    // With three tiles at normalized x=0,1,2, tiles A and C have normalized distance=2 → non-adjacent
    const { container: c } = render(<ZoneMapSvg tiles={[tileA, tileMiddle, tileC]} />)
    const paths = Array.from(c.querySelectorAll('path'))
    expect(paths.length).toBeGreaterThanOrEqual(1)
    const connectorPath = paths[0]
    expect(connectorPath.getAttribute('d')).toMatch(/^M .* Q /)
  })

  // REQ-MAP-CONN-4: Each connector MUST have a distinct stroke color from the palette.
  it('renders connectors with distinct colors when multiple connections exist', () => {
    const tileA: MapTile = { roomId: 'a', roomName: 'A', x: 0, y: 0, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'b' }, { direction: 'south', targetRoomId: 'c' }] }
    const tileB: MapTile = { roomId: 'b', roomName: 'B', x: 1, y: 0, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [{ direction: 'west', targetRoomId: 'a' }] }
    const tileC: MapTile = { roomId: 'c', roomName: 'C', x: 0, y: 1, pois: [], current: false, bossRoom: false,
      sameZoneExitTargets: [{ direction: 'north', targetRoomId: 'a' }] }
    const { container: c } = render(<ZoneMapSvg tiles={[tileA, tileB, tileC]} />)
    const paths = Array.from(c.querySelectorAll('path'))
    expect(paths.length).toBe(2)
    const colors = paths.map(p => p.getAttribute('stroke'))
    // Each connector must have a distinct color
    expect(colors[0]).not.toBe(colors[1])
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
