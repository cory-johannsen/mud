import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import type { MapTile } from '../proto'
import { useAutoNav } from './useAutoNav'

function makeChain(ids: string[]): MapTile[] {
  return ids.map((id, i) => ({
    roomId: id,
    roomName: `Room ${id}`,
    explored: true,
    sameZoneExitTargets: [
      ...(i > 0 ? [{ direction: 'west', targetRoomId: ids[i - 1] }] : []),
      ...(i < ids.length - 1 ? [{ direction: 'east', targetRoomId: ids[i + 1] }] : []),
    ],
  }))
}

describe('useAutoNav', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('fires sendMove for each step then clears active state', async () => {
    const tiles = makeChain(['a', 'b', 'c'])
    const sendMove = vi.fn()
    const onNoPath = vi.fn()

    // Use rerender to simulate server ACKing each move by updating currentRoomId.
    const { result, rerender } = renderHook(
      ({ roomId }: { roomId: string }) =>
        useAutoNav(tiles, roomId, 100, sendMove, onNoPath),
      { initialProps: { roomId: 'a' } },
    )

    expect(result.current.active).toBe(false)

    act(() => { result.current.start(tiles[2]) })  // navigate a→c, path=['b','c']

    expect(result.current.active).toBe(true)
    expect(result.current.destinationRoomId).toBe('c')

    // First step: currentRoomId='a', resolves 'east' to 'b'
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(sendMove).toHaveBeenCalledWith('east')
    expect(sendMove).toHaveBeenCalledTimes(1)

    // Server ACKs: re-render with updated currentRoomId='b'
    rerender({ roomId: 'b' })

    // Second step: currentRoomId='b', resolves 'east' to 'c'
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(sendMove).toHaveBeenCalledWith('east')
    expect(sendMove).toHaveBeenCalledTimes(2)
    expect(result.current.active).toBe(false)
    expect(result.current.destinationRoomId).toBeNull()
  })

  it('calls onNoPath when no explored path exists', () => {
    const tiles: MapTile[] = [
      { roomId: 'a', explored: true, sameZoneExitTargets: [] },
      { roomId: 'b', explored: true, roomName: 'Room B', sameZoneExitTargets: [] },
    ]
    const sendMove = vi.fn()
    const onNoPath = vi.fn()

    const { result } = renderHook(() =>
      useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
    )

    act(() => { result.current.start(tiles[1]) })

    expect(onNoPath).toHaveBeenCalledWith('Room B')
    expect(result.current.active).toBe(false)
    expect(sendMove).not.toHaveBeenCalled()
  })

  it('cancel stops the timer and clears path', async () => {
    const tiles = makeChain(['a', 'b', 'c', 'd'])
    const sendMove = vi.fn()
    const onNoPath = vi.fn()

    const { result, rerender } = renderHook(
      ({ roomId }: { roomId: string }) =>
        useAutoNav(tiles, roomId, 100, sendMove, onNoPath),
      { initialProps: { roomId: 'a' } },
    )

    act(() => { result.current.start(tiles[3]) })
    expect(result.current.active).toBe(true)

    await act(async () => { vi.advanceTimersByTime(100) })
    expect(sendMove).toHaveBeenCalledTimes(1)
    rerender({ roomId: 'b' })

    act(() => { result.current.cancel() })
    expect(result.current.active).toBe(false)

    await act(async () => { vi.advanceTimersByTime(300) })
    expect(sendMove).toHaveBeenCalledTimes(1)
  })

  it('start cancels existing path and retargets', async () => {
    const tiles = makeChain(['a', 'b', 'c', 'd'])
    const sendMove = vi.fn()
    const onNoPath = vi.fn()

    const { result } = renderHook(() =>
      useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
    )

    act(() => { result.current.start(tiles[3]) })
    expect(result.current.destinationRoomId).toBe('d')

    act(() => { result.current.start(tiles[1]) })
    expect(result.current.destinationRoomId).toBe('b')

    await act(async () => { vi.advanceTimersByTime(100) })
    expect(sendMove).toHaveBeenCalledWith('east')
    expect(sendMove).toHaveBeenCalledTimes(1)
    expect(result.current.active).toBe(false)
  })

  it('clicking current room cancels without starting navigation', () => {
    const tiles = makeChain(['a', 'b', 'c'])
    const sendMove = vi.fn()
    const onNoPath = vi.fn()

    const { result } = renderHook(() =>
      useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
    )

    act(() => { result.current.start(tiles[0]) })

    expect(result.current.active).toBe(false)
    expect(sendMove).not.toHaveBeenCalled()
    expect(onNoPath).not.toHaveBeenCalled()
  })

  it('cancels automatically when direction lookup fails (server-blocked movement)', async () => {
    const tiles = makeChain(['a', 'b', 'c'])
    const sendMove = vi.fn()
    const onNoPath = vi.fn()

    // currentRoomId stays 'a' throughout — server never ACKs the move.
    const { result } = renderHook(() =>
      useAutoNav(tiles, 'a', 100, sendMove, onNoPath)
    )

    act(() => { result.current.start(tiles[2]) })  // a→c, path=['b','c']

    // Step 1: currentRoomId='a', resolves 'east' to 'b'. sendMove.
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(sendMove).toHaveBeenCalledTimes(1)

    // Step 2: currentRoomId still 'a' (server blocked). path[0]='c'.
    // Tile 'a' has no exit to 'c', so resolveDirection returns null → cancel.
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(result.current.active).toBe(false)
    expect(sendMove).toHaveBeenCalledTimes(1)
  })
})
