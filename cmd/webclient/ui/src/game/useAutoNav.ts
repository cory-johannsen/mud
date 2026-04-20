import { useState, useRef, useCallback, useEffect } from 'react'
import type { MapTile } from '../proto'
import { findPath, resolveDirection } from './autoNav'

export interface UseAutoNavResult {
  start: (targetTile: MapTile) => void
  cancel: () => void
  active: boolean
  destinationRoomId: string | null
}

/**
 * useAutoNav manages automatic step-by-step navigation along a pre-computed BFS path.
 *
 * The hook resolves the move direction from the actual currentRoomId at each step.
 * If the server rejects a move (currentRoomId does not advance to the next room in
 * the path), the direction lookup for the subsequent step will fail and navigation
 * cancels automatically.
 *
 * Precondition: tiles, currentRoomId, and stepMs must be non-null/non-zero on each render.
 * Postcondition: at most one timer is active at any time; cleanup on unmount.
 */
export function useAutoNav(
  tiles: MapTile[],
  currentRoomId: string,
  stepMs: number,
  sendMove: (direction: string) => void,
  onNoPath: (roomName: string) => void,
): UseAutoNavResult {
  const [destinationRoomId, setDestinationRoomId] = useState<string | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // pathRef contains the remaining room IDs to visit (not yet moved to).
  const pathRef = useRef<string[]>([])

  // Stable refs so timer closures always see latest prop values without re-subscribing.
  const tilesRef = useRef(tiles)
  const currentRoomIdRef = useRef(currentRoomId)
  const stepMsRef = useRef(stepMs)
  const sendMoveRef = useRef(sendMove)
  const onNoPathRef = useRef(onNoPath)

  tilesRef.current = tiles
  currentRoomIdRef.current = currentRoomId
  stepMsRef.current = stepMs
  sendMoveRef.current = sendMove
  onNoPathRef.current = onNoPath

  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  // scheduleStepRef stores the latest step function so it can call itself recursively
  // without stale closures. Re-assigned on every render.
  const scheduleStepRef = useRef<() => void>(() => {})
  scheduleStepRef.current = () => {
    timerRef.current = setTimeout(() => {
      const path = pathRef.current
      if (path.length === 0) {
        setDestinationRoomId(null)
        return
      }

      const nextRoomId = path[0]
      const currentId = currentRoomIdRef.current
      const currentTile = tilesRef.current.find(t => (t.roomId ?? '') === currentId) ?? null

      // If direction cannot be resolved, the server blocked movement. Cancel automatically.
      const direction = currentTile ? resolveDirection(currentTile, nextRoomId) : null
      if (direction === null) {
        pathRef.current = []
        setDestinationRoomId(null)
        return
      }

      sendMoveRef.current(direction)
      pathRef.current = path.slice(1)

      if (pathRef.current.length > 0) {
        scheduleStepRef.current()
      } else {
        setDestinationRoomId(null)
      }
    }, stepMsRef.current)
  }

  const cancel = useCallback(() => {
    clearTimer()
    pathRef.current = []
    setDestinationRoomId(null)
  }, [clearTimer])

  const start = useCallback((targetTile: MapTile) => {
    const targetId = targetTile.roomId ?? ''
    const currentId = currentRoomIdRef.current

    // Clicking the current room cancels without starting navigation.
    if (targetId === currentId) {
      cancel()
      return
    }

    // Cancel existing path before starting a new one (retarget).
    cancel()

    const path = findPath(tilesRef.current, currentId, targetId)
    if (path === null) {
      // No explored path — notify caller.
      onNoPathRef.current(targetTile.roomName ?? targetId)
      return
    }
    if (path.length === 0) return  // already there (shouldn't occur after currentId check)

    pathRef.current = path
    setDestinationRoomId(targetId)
    scheduleStepRef.current()
  }, [cancel])

  // Cleanup timer on unmount to prevent memory leaks.
  useEffect(() => () => clearTimer(), [clearTimer])

  return { start, cancel, active: destinationRoomId !== null, destinationRoomId }
}
