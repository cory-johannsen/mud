import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RoomTooltip } from './RoomTooltip'
import type { MapTile } from '../proto'

const tile: MapTile = {
  roomId: 'grinders_row',
  roomName: "Grinder's Row",
  x: 0,
  y: 0,
  current: true,
  exits: ['north', 'east', 'south'],
  dangerLevel: 'safe',
  pois: ['merchant', 'healer', 'npc'],
}

describe('RoomTooltip', () => {
  it('renders the room name', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText("Grinder's Row")).toBeDefined()
  })

  it('renders the danger level', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('safe')).toBeDefined()
  })

  it('renders all POI labels', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('Merchant')).toBeDefined()
    expect(screen.getByText('Healer')).toBeDefined()
    expect(screen.getByText('NPC')).toBeDefined()
  })

  it('renders all exits', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('north')).toBeDefined()
    expect(screen.getByText('east')).toBeDefined()
    expect(screen.getByText('south')).toBeDefined()
  })

  it('renders "(current room)" indicator when tile.current is true', () => {
    render(<RoomTooltip tile={tile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('current room')).toBeDefined()
  })

  it('does not render "(current room)" when tile.current is false', () => {
    const nonCurrentTile: MapTile = { ...tile, current: false }
    render(<RoomTooltip tile={nonCurrentTile} pos={{ x: 100, y: 200 }} />)
    expect(screen.queryByText('current room')).toBeNull()
  })

  it('renders nothing for POIs section when pois is empty', () => {
    const emptyPoisTile: MapTile = { ...tile, pois: [] }
    render(<RoomTooltip tile={emptyPoisTile} pos={{ x: 100, y: 200 }} />)
    expect(screen.queryByText('Points of Interest:')).toBeNull()
  })

  it('renders nothing for exits section when exits is empty', () => {
    const emptyExitsTile: MapTile = { ...tile, exits: [] }
    render(<RoomTooltip tile={emptyExitsTile} pos={{ x: 100, y: 200 }} />)
    expect(screen.queryByText('Exits:', { exact: false })).toBeNull()
  })

  it('falls back to raw id for unknown POI', () => {
    const unknownPOITile: MapTile = { ...tile, pois: ['unknown_poi_type'] }
    render(<RoomTooltip tile={unknownPOITile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('unknown_poi_type')).toBeDefined()
  })

  it('renders danger level from snake_case danger_level field', () => {
    const snakeTile: MapTile = { ...tile, dangerLevel: undefined, danger_level: 'dangerous' }
    render(<RoomTooltip tile={snakeTile} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText('dangerous')).toBeDefined()
  })
})
