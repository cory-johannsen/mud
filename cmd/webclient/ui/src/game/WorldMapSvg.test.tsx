import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import { WorldMapSvg } from './WorldMapSvg'
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
