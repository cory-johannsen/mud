import { describe, it, expect, vi } from 'vitest'
import fc from 'fast-check'
import { render, fireEvent } from '@testing-library/react'
import { WorldMapSvg, computeTooltipPos } from './WorldMapSvg'
import { difficultyBorderColor } from './ZoneMapSvg'
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
    expect(legendText).toContain('Enemy Territory')
  })

  it('renders enemy zone with red stroke and X lines', () => {
    const enemyTile: WorldZoneTile = {
      zoneId: 'enemy_zone', zoneName: 'Enemy Zone',
      worldX: 0, worldY: 0,
      discovered: true, current: false, dangerLevel: 'dangerous',
      enemy: true,
    }
    const { container } = render(<WorldMapSvg tiles={[enemyTile]} onTravel={vi.fn()} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    const enemyRect = rects.find(r => r.getAttribute('stroke') === '#c02020')
    expect(enemyRect).toBeDefined()
    // Two X lines should be present
    const lines = container.querySelectorAll('line')
    expect(lines.length).toBe(2)
  })

  it('does not call onTravel when enemy zone tile is clicked', () => {
    const enemyTile: WorldZoneTile = {
      zoneId: 'enemy_zone', zoneName: 'Enemy Zone',
      worldX: 0, worldY: 0,
      discovered: true, current: false, dangerLevel: 'dangerous',
      enemy: true,
    }
    const onTravel = vi.fn()
    const { container } = render(<WorldMapSvg tiles={[enemyTile]} onTravel={onTravel} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    fireEvent.click(rects[0]!)
    expect(onTravel).not.toHaveBeenCalled()
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
    // Exactly one text element (the zone name) — no level range text rendered
    expect(texts).toHaveLength(1)
    expect(texts[0].textContent).toBe('Test Zone')
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

  describe('hover tooltip', () => {
    it('shows tooltip with zone name on mouseenter', () => {
      const { container, getByRole } = render(
        <WorldMapSvg tiles={[DISCOVERED_ZONE]} onTravel={vi.fn()} />
      )
      const g = container.querySelector('svg g')!
      fireEvent.mouseEnter(g)
      const tooltip = getByRole('tooltip')
      expect(tooltip.textContent).toContain('Beta Zone')
    })

    it('shows danger level in tooltip for discovered zone', () => {
      const { container, getByRole } = render(
        <WorldMapSvg tiles={[DISCOVERED_ZONE]} onTravel={vi.fn()} />
      )
      fireEvent.mouseEnter(container.querySelector('svg g')!)
      const tooltip = getByRole('tooltip')
      expect(tooltip.textContent).toContain('Dangerous')
    })

    it('shows level range in tooltip when set', () => {
      const tile: WorldZoneTile = {
        ...DISCOVERED_ZONE, levelRange: '5-10',
      }
      const { container, getByRole } = render(
        <WorldMapSvg tiles={[tile]} onTravel={vi.fn()} />
      )
      fireEvent.mouseEnter(container.querySelector('svg g')!)
      const tooltip = getByRole('tooltip')
      expect(tooltip.textContent).toContain('5-10')
    })

    it('shows description in tooltip when present', () => {
      const tile: WorldZoneTile = {
        ...DISCOVERED_ZONE, description: 'A gritty urban zone.',
      }
      const { container, getByRole } = render(
        <WorldMapSvg tiles={[tile]} onTravel={vi.fn()} />
      )
      fireEvent.mouseEnter(container.querySelector('svg g')!)
      const tooltip = getByRole('tooltip')
      expect(tooltip.textContent).toContain('A gritty urban zone.')
    })

    it('shows Undiscovered in tooltip for undiscovered zone', () => {
      const { container, getByRole } = render(
        <WorldMapSvg tiles={[UNDISCOVERED_ZONE]} onTravel={vi.fn()} />
      )
      fireEvent.mouseEnter(container.querySelector('svg g')!)
      const tooltip = getByRole('tooltip')
      expect(tooltip.textContent).toContain('Undiscovered')
    })

    it('does not reveal zone name in tooltip for undiscovered zone', () => {
      const undiscoveredWithName: WorldZoneTile = {
        zoneId: 'hidden_zone', zoneName: 'Secret Place',
        worldX: 0, worldY: 0,
        discovered: false, current: false,
      }
      const { container, getByRole } = render(
        <WorldMapSvg tiles={[undiscoveredWithName]} onTravel={vi.fn()} />
      )
      fireEvent.mouseEnter(container.querySelector('svg g')!)
      const tooltip = getByRole('tooltip')
      expect(tooltip.textContent).not.toContain('Secret Place')
      expect(tooltip.textContent).toContain('???')
    })

    it('shows Enemy Territory in tooltip for enemy zone', () => {
      const enemyTile: WorldZoneTile = {
        zoneId: 'enemy_zone', zoneName: 'Enemy Zone',
        worldX: 0, worldY: 0,
        discovered: true, current: false, dangerLevel: 'dangerous',
        enemy: true,
      }
      const { container, getByRole } = render(
        <WorldMapSvg tiles={[enemyTile]} onTravel={vi.fn()} />
      )
      fireEvent.mouseEnter(container.querySelector('svg g')!)
      const tooltip = getByRole('tooltip')
      expect(tooltip.textContent).toContain('Enemy Territory')
    })

    it('hides tooltip on mouseleave from tile', () => {
      const { container, queryByRole } = render(
        <WorldMapSvg tiles={[DISCOVERED_ZONE]} onTravel={vi.fn()} />
      )
      const g = container.querySelector('svg g')!
      fireEvent.mouseEnter(g)
      expect(queryByRole('tooltip')).not.toBeNull()
      fireEvent.mouseLeave(g)
      expect(queryByRole('tooltip')).toBeNull()
    })
  })
})

describe('WorldMapSvg zone connections', () => {
  it('renders a line for each connected zone pair', () => {
    const tileA: WorldZoneTile = {
      zoneId: 'zone_a', zoneName: 'Alpha Zone',
      worldX: 0, worldY: 0,
      discovered: true, current: true, dangerLevel: 'safe',
      connectedZoneIds: ['zone_b'],
    }
    const tileB: WorldZoneTile = {
      zoneId: 'zone_b', zoneName: 'Beta Zone',
      worldX: 1, worldY: 0,
      discovered: true, current: false, dangerLevel: 'safe',
      connectedZoneIds: ['zone_a'],
    }
    const { container } = render(<WorldMapSvg tiles={[tileA, tileB]} onTravel={vi.fn()} />)
    // Connections are now <path> elements (supports both straight lines and Bézier arcs)
    const paths = container.querySelectorAll('svg path')
    // Exactly one connection path drawn (deduped from bidirectional data)
    expect(paths.length).toBe(1)
  })

  it('deduplicates connections from both ends', () => {
    const tileA: WorldZoneTile = {
      zoneId: 'zone_a', zoneName: 'A',
      worldX: 0, worldY: 0,
      discovered: true, current: false, dangerLevel: 'safe',
      connectedZoneIds: ['zone_b', 'zone_c'],
    }
    const tileB: WorldZoneTile = {
      zoneId: 'zone_b', zoneName: 'B',
      worldX: 1, worldY: 0,
      discovered: true, current: false, dangerLevel: 'safe',
      connectedZoneIds: ['zone_a'],
    }
    const tileC: WorldZoneTile = {
      zoneId: 'zone_c', zoneName: 'C',
      worldX: 0, worldY: 1,
      discovered: true, current: false, dangerLevel: 'safe',
      connectedZoneIds: ['zone_a'],
    }
    const { container } = render(<WorldMapSvg tiles={[tileA, tileB, tileC]} onTravel={vi.fn()} />)
    const paths = container.querySelectorAll('svg path')
    // A-B and A-C: exactly 2 paths
    expect(paths.length).toBe(2)
  })

  it('renders no connection paths when no connections exist', () => {
    const { container } = render(
      <WorldMapSvg tiles={[CURRENT_ZONE, DISCOVERED_ZONE, UNDISCOVERED_ZONE]} onTravel={vi.fn()} />
    )
    const paths = container.querySelectorAll('svg path')
    expect(paths.length).toBe(0)
  })
})

describe('difficultyBorderColor', () => {
  it('returns green when player level is within zone range', () => {
    expect(difficultyBorderColor('3-5', 4)).toBe('#4a8')
    expect(difficultyBorderColor('3-5', 3)).toBe('#4a8')
    expect(difficultyBorderColor('3-5', 5)).toBe('#4a8')
  })

  it('returns dark grey when zone is below player level', () => {
    expect(difficultyBorderColor('1-3', 5)).toBe('#444')
    expect(difficultyBorderColor('3', 4)).toBe('#444')
  })

  it('returns yellow for zone 1-2 levels above player', () => {
    expect(difficultyBorderColor('5-7', 4)).toBe('#e6c84e') // min=5, player=4 → gap=1
    expect(difficultyBorderColor('5-7', 3)).toBe('#e6c84e') // gap=2
  })

  it('returns orange for zone 3-4 levels above player', () => {
    expect(difficultyBorderColor('6-8', 3)).toBe('#e08030') // min=6, player=3 → gap=3
    expect(difficultyBorderColor('7-9', 3)).toBe('#e08030') // gap=4
  })

  it('returns red for zone 5+ levels above player', () => {
    expect(difficultyBorderColor('8-10', 3)).toBe('#c03030') // min=8, player=3 → gap=5
    expect(difficultyBorderColor('10-12', 1)).toBe('#c03030')
  })

  it('returns null when levelRange is undefined', () => {
    expect(difficultyBorderColor(undefined, 5)).toBeNull()
  })

  it('returns null when playerLevel is 0', () => {
    expect(difficultyBorderColor('3-5', 0)).toBeNull()
  })
})

describe('WorldMapSvg difficulty border colors', () => {
  it('applies green border to a zone at player level', () => {
    const tile: WorldZoneTile = {
      zoneId: 'z1', zoneName: 'Zone 1',
      worldX: 0, worldY: 0,
      discovered: true, current: false, dangerLevel: 'safe',
      levelRange: '3-5',
    }
    const { container } = render(<WorldMapSvg tiles={[tile]} onTravel={vi.fn()} playerLevel={4} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    const greenRect = rects.find(r => r.getAttribute('stroke') === '#4a8')
    expect(greenRect).toBeDefined()
  })

  it('applies red border to a zone 5+ levels above player', () => {
    const tile: WorldZoneTile = {
      zoneId: 'z1', zoneName: 'Zone 1',
      worldX: 0, worldY: 0,
      discovered: true, current: false, dangerLevel: 'safe',
      levelRange: '10-12',
    }
    const { container } = render(<WorldMapSvg tiles={[tile]} onTravel={vi.fn()} playerLevel={1} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    const redRect = rects.find(r => r.getAttribute('stroke') === '#c03030')
    expect(redRect).toBeDefined()
  })

  it('does not apply difficulty color to undiscovered zones', () => {
    const tile: WorldZoneTile = {
      zoneId: 'z1',
      worldX: 0, worldY: 0,
      discovered: false, current: false,
      levelRange: '3-5',
    }
    const { container } = render(<WorldMapSvg tiles={[tile]} onTravel={vi.fn()} playerLevel={4} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    // Undiscovered — should not have green difficulty border
    const greenRect = rects.find(r => r.getAttribute('stroke') === '#4a8')
    expect(greenRect).toBeUndefined()
  })

  it('does not apply difficulty color to current zone', () => {
    const tile: WorldZoneTile = {
      zoneId: 'z1', zoneName: 'Zone 1',
      worldX: 0, worldY: 0,
      discovered: true, current: true, dangerLevel: 'safe',
      levelRange: '3-5',
    }
    const { container } = render(<WorldMapSvg tiles={[tile]} onTravel={vi.fn()} playerLevel={4} />)
    const rects = Array.from(container.querySelectorAll('svg rect:not(defs rect)'))
    // Current zone — gold stroke takes priority
    const goldRect = rects.find(r => r.getAttribute('stroke') === '#f0c040')
    expect(goldRect).toBeDefined()
    const greenRect = rects.find(r => r.getAttribute('stroke') === '#4a8')
    expect(greenRect).toBeUndefined()
  })
})

// REQ-WM-TT-1: Near right edge the tooltip flips left so it stays in bounds.
// REQ-WM-TT-2: Near bottom edge the tooltip flips above so it stays in bounds.
function makeContainer(width: number, height: number): DOMRect {
  return { left: 0, top: 0, right: width, bottom: height, width, height, x: 0, y: 0, toJSON: () => ({}) } as DOMRect
}

describe('computeTooltipPos', () => {
  it('places tooltip to the right and below when space is available', () => {
    const pos = computeTooltipPos(100, 100, makeContainer(800, 600))
    expect(pos.x).toBeGreaterThan(100)
    expect(pos.y).toBeGreaterThan(100)
  })

  it('flips left when cursor is near the right edge', () => {
    const pos = computeTooltipPos(780, 100, makeContainer(800, 600))
    // Right-side flip: tooltip x should be to the LEFT of the cursor
    expect(pos.x).toBeLessThan(780)
  })

  it('flips above when cursor is near the bottom edge', () => {
    const pos = computeTooltipPos(100, 570, makeContainer(800, 600))
    // Bottom flip: tooltip y should be ABOVE the cursor
    expect(pos.y).toBeLessThan(570)
  })

  it('flips both axes when cursor is near the bottom-right corner', () => {
    const pos = computeTooltipPos(780, 570, makeContainer(800, 600))
    expect(pos.x).toBeLessThan(780)
    expect(pos.y).toBeLessThan(570)
  })

  it('property: tooltip is always fully within container bounds', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 800 }),
        fc.integer({ min: 0, max: 600 }),
        (cx, cy) => {
          const container = makeContainer(800, 600)
          const { x, y } = computeTooltipPos(cx, cy, container)
          // 256 = TOOLTIP_W, 160 = TOOLTIP_H
          return x >= 0 && x + 256 <= 800 + 256 && y >= 0
        }
      )
    )
  })
})
