import { describe, it, expect } from 'vitest'
import { renderMapTiles } from './mapRenderer'
import type { MapTile } from '../proto'

const WEST: MapTile = { roomId: 'r1', roomName: 'West Room', x: 0, y: 0, exits: ['east'], dangerLevel: 'safe', pois: [] }
const EAST: MapTile = { roomId: 'r2', roomName: 'East Room', x: 2, y: 0, exits: ['west'], dangerLevel: 'sketchy', pois: ['merchant'] }

describe('renderMapTiles', () => {
  it('attaches tile reference to room cell segments', () => {
    const { gridLines } = renderMapTiles([WEST, EAST])
    const roomRow = gridLines[0]

    // Find all segments that have a tile attached
    const tiledSegs = roomRow.filter(s => s.tile !== undefined)

    expect(tiledSegs).toHaveLength(2)
    expect(tiledSegs[0].tile?.roomId).toBe('r1')
    expect(tiledSegs[1].tile?.roomId).toBe('r2')
  })

  it('does not attach tile to connector or padding segments', () => {
    const { gridLines } = renderMapTiles([WEST, EAST])
    const roomRow = gridLines[0]
    const connectors = roomRow.filter(s => s.tile === undefined && s.text.trim() !== '')
    // '-' connector between the two rooms should have no tile
    expect(connectors.some(s => s.text === '-')).toBe(true)
  })

  it('returns empty result for empty tiles', () => {
    const result = renderMapTiles([])
    expect(result.gridLines).toHaveLength(0)
    expect(result.legendLines).toHaveLength(0)
  })
})
