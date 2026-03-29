import { useEffect, useRef } from 'react'
import { useGame } from '../GameContext'

function formatTimestamp(d: Date): string {
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `[${hh}:${mm}]`
}

export function FeedPanel() {
  const { state } = useGame()
  const scrollRef = useRef<HTMLDivElement>(null)
  const userScrolledRef = useRef(false)

  useEffect(() => {
    const el = scrollRef.current
    if (!el || userScrolledRef.current) return
    el.scrollTop = el.scrollHeight
  }, [state.feedEntries.length])

  function handleScroll() {
    const el = scrollRef.current
    if (!el) return
    const atBottom = el.scrollTop >= el.scrollHeight - el.clientHeight - 50
    userScrolledRef.current = !atBottom
  }

  return (
    <div
      ref={scrollRef}
      className="feed-scroll"
      onScroll={handleScroll}
      style={{ height: '100%', overflowY: 'auto' }}
    >
      {state.feedEntries.map((entry) => (
        <div key={entry.id} className={`feed-entry feed-${entry.type}`}>
          <span className="ts">{formatTimestamp(entry.timestamp)}</span>
          {entry.text}
        </div>
      ))}
    </div>
  )
}
