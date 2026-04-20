import type { MapTile, SameZoneExitTarget } from '../proto'

/**
 * findPath returns the shortest BFS path of room IDs from fromId to toId,
 * traversing only explored tiles via sameZoneExitTargets.
 *
 * Returns [] if fromId === toId (no movement needed).
 * Returns null if no explored path exists or either endpoint is not explored.
 *
 * Precondition: tiles are from the same zone map response.
 * Postcondition: if non-null, every roomId in the result is explored and reachable.
 */
export function findPath(
  tiles: MapTile[],
  fromId: string,
  toId: string,
): string[] | null {
  if (fromId === toId) return []

  // Index explored tiles by roomId and build adjacency from sameZoneExitTargets.
  const exploredSet = new Set<string>()
  const adjMap = new Map<string, string[]>()  // roomId → [targetRoomId, ...]

  for (const tile of tiles) {
    const id = tile.roomId ?? ''
    if (!id || !(tile.explored ?? false)) continue
    exploredSet.add(id)
    const exits: SameZoneExitTarget[] = tile.sameZoneExitTargets ?? tile.same_zone_exit_targets ?? []
    adjMap.set(id, exits.map(e => e.targetRoomId ?? e.target_room_id ?? '').filter(t => t !== ''))
  }

  if (!exploredSet.has(fromId) || !exploredSet.has(toId)) return null

  // BFS over explored tiles only.
  const visited = new Set<string>([fromId])
  const queue: Array<{ id: string; path: string[] }> = [{ id: fromId, path: [] }]

  while (queue.length > 0) {
    const { id, path } = queue.shift()!
    for (const targetId of (adjMap.get(id) ?? [])) {
      if (!exploredSet.has(targetId) || visited.has(targetId)) continue
      const newPath = [...path, targetId]
      if (targetId === toId) return newPath
      visited.add(targetId)
      queue.push({ id: targetId, path: newPath })
    }
  }
  return null
}

/**
 * resolveDirection returns the direction to move from currentTile to nextRoomId
 * by looking up sameZoneExitTargets on currentTile.
 *
 * Returns null if nextRoomId is not a direct exit from currentTile.
 *
 * Precondition: currentTile must be non-null; nextRoomId must be non-empty.
 * Postcondition: returned direction is a valid compass direction string, or null.
 */
export function resolveDirection(currentTile: MapTile, nextRoomId: string): string | null {
  const exits: SameZoneExitTarget[] = currentTile.sameZoneExitTargets ?? currentTile.same_zone_exit_targets ?? []
  for (const e of exits) {
    const targetId = e.targetRoomId ?? e.target_room_id ?? ''
    if (targetId === nextRoomId) return e.direction ?? null
  }
  return null
}
