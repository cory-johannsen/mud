import { useLayoutEffect, useRef } from 'react'
import { useGame } from '../GameContext'

function formatTimestamp(d: Date): string {
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `[${hh}:${mm}]`
}

export function FeedPanel() {
  const { state } = useGame()
  const bottomRef = useRef<HTMLDivElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  // true when the user has intentionally scrolled away from the bottom
  const userScrolledRef = useRef(false)
  // true while a programmatic scroll is in flight — prevents the scroll event
  // from falsely marking the user as having scrolled away
  const programmaticScrollRef = useRef(false)

  useLayoutEffect(() => {
    if (userScrolledRef.current) return
    programmaticScrollRef.current = true
    bottomRef.current?.scrollIntoView({ behavior: 'instant' })
    // The scroll event fires synchronously inside scrollIntoView on some
    // browsers; reset the flag in a microtask so handleScroll can see it.
    Promise.resolve().then(() => {
      programmaticScrollRef.current = false
    })
  }, [state.feedEntries.length])

  function handleScroll() {
    if (programmaticScrollRef.current) return
    const el = scrollRef.current
    if (!el) return
    const atBottom = el.scrollTop >= el.scrollHeight - el.clientHeight - 50
    userScrolledRef.current = !atBottom // false when user returns to bottom — re-enables autoscroll
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
      <div ref={bottomRef} />
    </div>
  )
}
