import { describe, it, expect } from 'vitest'
import fc from 'fast-check'
import type { MapTile } from '../proto'
import { findPath, resolveDirection } from './autoNav'

// Helper: build a simple linear chain of explored rooms: A → B → C...
function makeChain(ids: string[]): MapTile[] {
  return ids.map((id, i) => ({
    roomId: id,
    roomName: id,
    explored: true,
    sameZoneExitTargets: [
      ...(i > 0 ? [{ direction: 'west', targetRoomId: ids[i - 1] }] : []),
      ...(i < ids.length - 1 ? [{ direction: 'east', targetRoomId: ids[i + 1] }] : []),
    ],
  }))
}

describe('findPath', () => {
  it('returns [] when fromId === toId', () => {
    const tiles = makeChain(['a', 'b', 'c'])
    expect(findPath(tiles, 'a', 'a')).toEqual([])
  })

  it('returns direct neighbor in one hop', () => {
    const tiles = makeChain(['a', 'b', 'c'])
    expect(findPath(tiles, 'a', 'b')).toEqual(['b'])
  })

  it('returns multi-hop path through chain', () => {
    const tiles = makeChain(['a', 'b', 'c', 'd'])
    expect(findPath(tiles, 'a', 'd')).toEqual(['b', 'c', 'd'])
  })

  it('returns null when destination is not explored', () => {
    const tiles: MapTile[] = [
      { roomId: 'a', explored: true, sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'b' }] },
      { roomId: 'b', explored: false, sameZoneExitTargets: [] },
    ]
    expect(findPath(tiles, 'a', 'b')).toBeNull()
  })

  it('returns null when path requires traversing unexplored room', () => {
    const tiles: MapTile[] = [
      { roomId: 'a', explored: true, sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'b' }] },
      { roomId: 'b', explored: false, sameZoneExitTargets: [{ direction: 'east', targetRoomId: 'c' }] },
      { roomId: 'c', explored: true, sameZoneExitTargets: [{ direction: 'west', targetRoomId: 'b' }] },
    ]
    // b is unexplored, so a→c has no explored path
    expect(findPath(tiles, 'a', 'c')).toBeNull()
  })

  it('returns null when fromId not in explored tiles', () => {
    const tiles = makeChain(['a', 'b'])
    expect(findPath(tiles, 'z', 'b')).toBeNull()
  })
})

describe('resolveDirection', () => {
  it('returns direction when target is in sameZoneExitTargets', () => {
    const tile: MapTile = {
      roomId: 'a',
      sameZoneExitTargets: [{ direction: 'north', targetRoomId: 'b' }],
    }
    expect(resolveDirection(tile, 'b')).toBe('north')
  })

  it('returns null when target is not in sameZoneExitTargets', () => {
    const tile: MapTile = {
      roomId: 'a',
      sameZoneExitTargets: [{ direction: 'north', targetRoomId: 'b' }],
    }
    expect(resolveDirection(tile, 'z')).toBeNull()
  })
})

describe('findPath property tests', () => {
  it('path length never exceeds number of explored tiles', () => {
    fc.assert(fc.property(
      fc.integer({ min: 2, max: 6 }).chain(n =>
        fc.tuple(
          fc.constant(n),
          fc.uniqueArray(fc.string({ minLength: 1, maxLength: 4 }), { minLength: n, maxLength: n }),
        )
      ),
      ([n, ids]) => {
        const tiles = makeChain(ids)
        const path = findPath(tiles, ids[0], ids[n - 1])
        if (path === null) return true  // null means no path found, which is valid
        return path.length <= tiles.filter(t => t.explored).length
      }
    ))
  })

  it('findPath result is non-null for any pair of reachable explored tiles', () => {
    fc.assert(fc.property(
      fc.integer({ min: 2, max: 8 }).chain(n =>
        fc.tuple(
          fc.constant(n),
          fc.uniqueArray(fc.string({ minLength: 1, maxLength: 4 }), { minLength: n, maxLength: n }),
          fc.integer({ min: 0, max: n - 1 }),
          fc.integer({ min: 0, max: n - 1 }),
        )
      ),
      ([_n, ids, fromIdx, toIdx]) => {
        const tiles = makeChain(ids)
        const path = findPath(tiles, ids[fromIdx], ids[toIdx])
        if (fromIdx === toIdx) return path !== null && path.length === 0
        // In a fully connected linear chain all explored, a path must exist
        return path !== null
      }
    ))
  })
})
