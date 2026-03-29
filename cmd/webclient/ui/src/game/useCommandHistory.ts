import { useRef, useCallback } from 'react'

const HISTORY_CAP = 100

interface CommandHistoryResult {
  push: (cmd: string) => void
  navigateUp: () => string
  navigateDown: () => string
  reset: () => void
}

export function useCommandHistory(): CommandHistoryResult {
  const historyRef = useRef<string[]>([])
  const cursorRef = useRef(-1)

  const push = useCallback((cmd: string) => {
    historyRef.current = [cmd, ...historyRef.current].slice(0, HISTORY_CAP)
    cursorRef.current = -1
  }, [])

  const navigateUp = useCallback((): string => {
    const h = historyRef.current
    if (h.length === 0) return ''
    cursorRef.current = Math.min(cursorRef.current + 1, h.length - 1)
    return h[cursorRef.current] ?? ''
  }, [])

  const navigateDown = useCallback((): string => {
    cursorRef.current = Math.max(cursorRef.current - 1, -1)
    if (cursorRef.current === -1) return ''
    return historyRef.current[cursorRef.current] ?? ''
  }, [])

  const reset = useCallback(() => {
    cursorRef.current = -1
  }, [])

  return { push, navigateUp, navigateDown, reset }
}
