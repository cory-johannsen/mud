import { useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthContext'

interface LogEntry {
  type: string
  payload: unknown
  time: string
}

const ALL_TYPES = ['CombatEvent', 'MessageEvent', 'RoomEvent', 'ErrorEvent', 'Other'] as const
type EventTypeName = typeof ALL_TYPES[number]

const TYPE_COLORS: Record<EventTypeName, string> = {
  CombatEvent: '#c44',
  MessageEvent: '#4a9',
  RoomEvent: '#55b',
  ErrorEvent: '#a44',
  Other: '#888',
}

const MAX_ENTRIES = 500

export function LiveLogTab() {
  const { token } = useAuth()
  const [enabledTypes, setEnabledTypes] = useState<Set<EventTypeName>>(new Set(ALL_TYPES))
  const [entries, setEntries] = useState<LogEntry[]>([])
  const esRef = useRef<EventSource | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const tokenRef = useRef(token)
  tokenRef.current = token

  const openES = (types: Set<EventTypeName>) => {
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }
    const typesParam = types.size < ALL_TYPES.length ? `types=${[...types].join(',')}` : ''
    const tokenParam = tokenRef.current ? `token=${encodeURIComponent(tokenRef.current)}` : ''
    const qs = [typesParam, tokenParam].filter(Boolean).join('&')
    const url = `/api/admin/events${qs ? `?${qs}` : ''}`
    const es = new EventSource(url)

    es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as LogEntry
        setEntries(prev => {
          const next = [data, ...prev]
          return next.length > MAX_ENTRIES ? next.slice(0, MAX_ENTRIES) : next
        })
      } catch {
        // ignore malformed frames
      }
    }

    es.onerror = () => {
      es.close()
      esRef.current = null
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = setTimeout(() => openES(types), 2_000)
    }

    esRef.current = es
  }

  useEffect(() => {
    openES(enabledTypes)
    return () => {
      esRef.current?.close()
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
    }
  }, []) // intentionally empty — reconnect on filter change handled separately

  const toggleType = (t: EventTypeName) => {
    setEnabledTypes(prev => {
      const next = new Set(prev)
      if (next.has(t)) {
        next.delete(t)
      } else {
        next.add(t)
      }
      // Reconnect with new filter.
      openES(next)
      return next
    })
  }

  const resolvedTypeName = (type: string): EventTypeName => {
    if ((ALL_TYPES as readonly string[]).includes(type)) return type as EventTypeName
    return 'Other'
  }

  return (
    <div style={{ padding: '1rem' }}>
      <div style={{ display: 'flex', gap: '1rem', alignItems: 'center', marginBottom: '0.8rem', flexWrap: 'wrap' }}>
        <h2 style={{ color: '#ccc', margin: 0 }}>Live Log</h2>
        <div style={{ display: 'flex', gap: '0.6rem', flexWrap: 'wrap' }}>
          {ALL_TYPES.map(t => (
            <label key={t} style={{ color: TYPE_COLORS[t], cursor: 'pointer', fontSize: '0.9em', display: 'flex', gap: '0.3rem', alignItems: 'center' }}>
              <input
                type="checkbox"
                checked={enabledTypes.has(t)}
                onChange={() => toggleType(t)}
              />
              {t}
            </label>
          ))}
        </div>
        <button
          onClick={() => setEntries([])}
          style={{ background: '#444', color: '#ccc', border: 'none', padding: '0.3rem 0.8rem', borderRadius: '3px', cursor: 'pointer', fontSize: '0.85em' }}
        >
          Clear
        </button>
        <span style={{ color: '#555', fontSize: '0.8em' }}>{entries.length} entries</span>
      </div>

      <div style={{ fontFamily: 'monospace', fontSize: '0.8em', overflowY: 'auto', maxHeight: '60vh', background: '#111', borderRadius: '4px', padding: '0.5rem' }}>
        {entries.length === 0 && <p style={{ color: '#555' }}>Waiting for events…</p>}
        {entries.map((e, i) => {
          const resolved = resolvedTypeName(e.type)
          const color = TYPE_COLORS[resolved]
          return (
            <div key={i} style={{ borderBottom: '1px solid #1e1e1e', padding: '0.25rem 0', display: 'flex', gap: '0.6rem' }}>
              <span style={{ color: '#666', minWidth: '160px' }}>
                {new Date(e.time).toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })}
              </span>
              <span style={{ color, minWidth: '100px' }}>{e.type}</span>
              <span style={{ color: '#aaa', wordBreak: 'break-all' }}>
                {JSON.stringify(e.payload).slice(0, 300)}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
