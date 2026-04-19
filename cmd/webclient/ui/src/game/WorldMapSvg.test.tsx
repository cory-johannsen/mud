import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import { WorldMapSvg } from './WorldMapSvg'
import type { WorldZoneTile } from '../proto'

const CURRENT_ZONE: WorldZoneTile = {
  zoneId: 'zone_a', zoneName: 'Alpha Zone',
  worldX: 0, worldY: 0,
  discovered: true, current: true, dangerLevel: 'safe',
}

const DISCOVERED_ZONE: WorldZoneTile = {
  zoneId: 'zone_b', zoneName: 'Beta Zone',
  worldX: 1, worldY: 0,
  discovered: true, current: false, dangerLevel: 'dangerous',
}

const UNDISCOVERED_ZONE: WorldZoneTile = {
  zoneId: 'zone_c',
  worldX: 2, worldY: 0,
  discovered: false, current: false,
}

describe('WorldMapSvg', () => {
  it('renders one rect per tile', () => {
    const { container } = render(<WorldMapSvg tiles={[CURRENT_ZONE, DISCOVERED_ZONE, UNDISCOVERED_ZONE]} onTravel={() => {}} />)
    // Rects outside defs (excludes clipPath rects)
    const rects = container.querySelectorAll('svg rect:not(defs rect)')
    expect(rects.length).toBe(3)
  })

  it('renders current zone with gold stroke', () => {
    const { container } = render(<WorldMapSvg tiles={[CURRENT_ZONE]} onTravel={() => {}} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    const currentRect = rects.find(r => r.getAttribute('stroke') === '#f0c040')
    expect(currentRect).toBeDefined()
  })

  it('renders undiscovered zone with #111 fill and no label', () => {
    const { container } = render(<WorldMapSvg tiles={[UNDISCOVERED_ZONE]} onTravel={() => {}} />)
    const rect = container.querySelector('svg rect:not(defs rect)')
    expect(rect?.getAttribute('fill')).toBe('#111')
    // No text labels for undiscovered tiles
    const texts = container.querySelectorAll('text')
    expect(texts.length).toBe(0)
  })

  it('calls onTravel with zoneId when discovered non-current tile is clicked', () => {
    const onTravel = vi.fn()
    const { container } = render(<WorldMapSvg tiles={[CURRENT_ZONE, DISCOVERED_ZONE]} onTravel={onTravel} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    // Click the non-current discovered tile (zone_b)
    const travelRect = rects.find(r => r.getAttribute('fill') === '#4a2a1a') // dangerous fill
    expect(travelRect).toBeDefined()
    fireEvent.click(travelRect!)
    expect(onTravel).toHaveBeenCalledWith('zone_b')
  })

  it('renders a legend with danger level swatches', () => {
    const { container } = render(<WorldMapSvg tiles={[CURRENT_ZONE]} onTravel={() => {}} />)
    // Legend is a div below the SVG
    const legendText = container.textContent
    expect(legendText).toContain('safe')
    expect(legendText).toContain('dangerous')
    expect(legendText).toContain('Undiscovered')
  })

  it('renders level range text when levelRange is set', () => {
    const tiles: WorldZoneTile[] = [{
      zoneId: 'test_zone',
      zoneName: 'Test Zone',
      worldX: 0,
      worldY: 0,
      discovered: true,
      current: false,
      dangerLevel: 'safe',
      levelRange: '1-3',
    }]
    const { container } = render(<WorldMapSvg tiles={tiles} onTravel={vi.fn()} />)
    expect(container.textContent).toContain('1-3')
  })

  it('does not render level range text when levelRange is absent', () => {
    const tiles: WorldZoneTile[] = [{
      zoneId: 'test_zone',
      zoneName: 'Test Zone',
      worldX: 0,
      worldY: 0,
      discovered: true,
      current: false,
      dangerLevel: 'safe',
    }]
    const { container } = render(<WorldMapSvg tiles={tiles} onTravel={vi.fn()} />)
    const texts = container.querySelectorAll('text')
    const contents = Array.from(texts).map(t => t.textContent ?? '')
    expect(contents.every(t => !t.match(/\d+-\d+/))).toBe(true)
  })

  it('renders zone name on discovered tile', () => {
    const tiles: WorldZoneTile[] = [{
      zoneId: 'test_zone',
      zoneName: 'My Zone',
      worldX: 0,
      worldY: 0,
      discovered: true,
      current: false,
      dangerLevel: 'safe',
    }]
    const { container } = render(<WorldMapSvg tiles={tiles} onTravel={vi.fn()} />)
    expect(container.textContent).toContain('My Zone')
  })

  it('does not render zone name on undiscovered tile', () => {
    const tiles: WorldZoneTile[] = [{
      zoneId: 'test_zone',
      zoneName: 'Hidden Zone',
      worldX: 0,
      worldY: 0,
      discovered: false,
      current: false,
      dangerLevel: 'safe',
    }]
    const { container } = render(<WorldMapSvg tiles={tiles} onTravel={vi.fn()} />)
    expect(container.textContent).not.toContain('Hidden Zone')
  })
})
