import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { ZoneMapSvg } from './ZoneMapSvg'
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

  it('renders at least one <line> connecting adjacent tiles with matching exits', () => {
    // CURRENT_TILE at (0,0) has exit 'e'; BOSS_TILE at (2,0) has exit 'w' — they face each other
    const { container: c } = render(<ZoneMapSvg tiles={[CURRENT_TILE, BOSS_TILE]} />)
    const lines = c.querySelectorAll('line')
    expect(lines.length).toBeGreaterThanOrEqual(1)
  })
})
