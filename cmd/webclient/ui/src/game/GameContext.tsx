// GameContext manages WebSocket connection, message dispatch, and game state.
// REQ-CTX-1: Connect to ws://<host>/ws?token=<JWT> on mount.
// REQ-CTX-2: sendMessage serializes and sends frames, queuing if not OPEN.
// REQ-CTX-3: sendCommand sends CommandRequest frames.
// REQ-CTX-4: Incoming frames dispatch on type field to update state.
// REQ-CTX-5: On RoomView, update state.roomView AND append room_event feed entry.
// REQ-CTX-6: On RoundStartEvent, set state.combatRound. On RoundEndEvent, set to null.
// REQ-CTX-7: On Disconnected, navigate to /characters.
// REQ-CTX-8: Auto-reconnect with backoff: 1s, 2s, 4s, 8s, max 30s.
// REQ-CTX-9: Provider wraps child panels.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useReducer,
  useRef,
  type ReactNode,
} from 'react'
import { useNavigate } from 'react-router-dom'
import type {
  RoomView,
  CharacterInfo,
  CharacterSheetView,
  InventoryView,
  MapTile,
  RoundStartEvent,
} from '../proto'

const TOKEN_KEY = 'mud_token'
const FEED_CAP = 500

export type FeedEntryType =
  | 'message'
  | 'combat'
  | 'round_start'
  | 'round_end'
  | 'room_event'
  | 'error'
  | 'character_info'
  | 'system'

export interface FeedEntry {
  id: string
  timestamp: Date
  type: FeedEntryType
  text: string
}

export interface GameState {
  connected: boolean
  roomView: RoomView | null
  characterInfo: CharacterInfo | null
  characterSheet: CharacterSheetView | null
  inventoryView: InventoryView | null
  mapTiles: MapTile[]
  feedEntries: FeedEntry[]
  combatRound: RoundStartEvent | null
}

type Action =
  | { type: 'SET_CONNECTED'; connected: boolean }
  | { type: 'SET_ROOM'; room: RoomView }
  | { type: 'SET_CHARACTER_INFO'; info: CharacterInfo }
  | { type: 'SET_CHARACTER_SHEET'; sheet: CharacterSheetView }
  | { type: 'SET_INVENTORY'; inv: InventoryView }
  | { type: 'SET_MAP_TILES'; tiles: MapTile[] }
  | { type: 'SET_COMBAT_ROUND'; round: RoundStartEvent | null }
  | { type: 'APPEND_FEED'; entry: FeedEntry }

function reducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case 'SET_CONNECTED':
      return { ...state, connected: action.connected }
    case 'SET_ROOM':
      return { ...state, roomView: action.room }
    case 'SET_CHARACTER_INFO':
      return { ...state, characterInfo: action.info }
    case 'SET_CHARACTER_SHEET':
      return { ...state, characterSheet: action.sheet }
    case 'SET_INVENTORY':
      return { ...state, inventoryView: action.inv }
    case 'SET_MAP_TILES':
      return { ...state, mapTiles: action.tiles }
    case 'SET_COMBAT_ROUND':
      return { ...state, combatRound: action.round }
    case 'APPEND_FEED': {
      const updated = [...state.feedEntries, action.entry]
      return {
        ...state,
        feedEntries: updated.length > FEED_CAP
          ? updated.slice(updated.length - FEED_CAP)
          : updated,
      }
    }
    default:
      return state
  }
}

const initialState: GameState = {
  connected: false,
  roomView: null,
  characterInfo: null,
  characterSheet: null,
  inventoryView: null,
  mapTiles: [],
  feedEntries: [],
  combatRound: null,
}

interface GameContextValue {
  state: GameState
  sendMessage: (type: string, payload: object) => void
  sendCommand: (raw: string) => void
}

const GameContext = createContext<GameContextValue | null>(null)

function makeFeedEntry(type: FeedEntryType, text: string): FeedEntry {
  return { id: crypto.randomUUID(), timestamp: new Date(), type, text }
}

export function GameProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, initialState)
  const navigate = useNavigate()
  const wsRef = useRef<WebSocket | null>(null)
  const queueRef = useRef<string[]>([])
  const backoffRef = useRef(1000)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const unmountedRef = useRef(false)

  const sendMessage = useCallback((type: string, payload: object) => {
    const frame = JSON.stringify({ type, payload })
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(frame)
    } else {
      queueRef.current.push(frame)
    }
  }, [])

  const sendCommand = useCallback((raw: string) => {
    sendMessage('CommandRequest', { command: raw })
  }, [sendMessage])

  const connect = useCallback(() => {
    const token = localStorage.getItem(TOKEN_KEY)
    if (!token) {
      void navigate('/login')
      return
    }
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${window.location.host}/ws?token=${encodeURIComponent(token)}`
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      dispatch({ type: 'SET_CONNECTED', connected: true })
      backoffRef.current = 1000
      while (queueRef.current.length > 0) {
        ws.send(queueRef.current.shift()!)
      }
    }

    ws.onclose = (ev) => {
      dispatch({ type: 'SET_CONNECTED', connected: false })
      if (unmountedRef.current) return
      if (ev.code === 1000 || ev.code === 1001) return
      const delay = backoffRef.current
      backoffRef.current = Math.min(backoffRef.current * 2, 30000)
      reconnectTimerRef.current = setTimeout(() => {
        if (!unmountedRef.current) connect()
      }, delay)
    }

    ws.onerror = () => {
      // onclose will fire after onerror; reconnect handled there.
    }

    ws.onmessage = (ev) => {
      let parsed: { type: string; payload: unknown }
      try {
        parsed = JSON.parse(ev.data as string) as { type: string; payload: unknown }
      } catch {
        return
      }
      const { type: msgType, payload } = parsed

      switch (msgType) {
        case 'RoomView': {
          const room = payload as RoomView
          dispatch({ type: 'SET_ROOM', room })
          dispatch({
            type: 'APPEND_FEED',
            entry: makeFeedEntry('room_event', `— ${room.title ?? 'Room'} —`),
          })
          break
        }
        case 'CharacterInfo':
          dispatch({ type: 'SET_CHARACTER_INFO', info: payload as CharacterInfo })
          break
        case 'CharacterSheetView':
          dispatch({ type: 'SET_CHARACTER_SHEET', sheet: payload as CharacterSheetView })
          break
        case 'InventoryView':
          dispatch({ type: 'SET_INVENTORY', inv: payload as InventoryView })
          break
        case 'MapResponse': {
          const map = payload as { tiles?: MapTile[] }
          dispatch({ type: 'SET_MAP_TILES', tiles: map.tiles ?? [] })
          break
        }
        case 'MessageEvent': {
          const msg = payload as { sender?: string; content?: string }
          dispatch({
            type: 'APPEND_FEED',
            entry: makeFeedEntry('message', `${msg.sender ?? ''}: ${msg.content ?? ''}`),
          })
          break
        }
        case 'RoomEvent': {
          const re = payload as { player?: string; action?: string; message?: string }
          const text = re.message ?? `${re.player ?? ''} ${re.action ?? ''}`
          dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('room_event', text) })
          break
        }
        case 'CombatEvent': {
          const ce = payload as { narrative?: string; attacker?: string; target?: string; damage?: number }
          const text = ce.narrative
            ? ce.narrative
            : `${ce.attacker ?? '?'} → ${ce.target ?? '?'}: ${ce.damage ?? 0} dmg`
          dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('combat', text) })
          break
        }
        case 'RoundStartEvent': {
          const rs = payload as RoundStartEvent
          dispatch({ type: 'SET_COMBAT_ROUND', round: rs })
          const order = Array.isArray(rs.turnOrder) ? rs.turnOrder.join(', ') : ''
          dispatch({
            type: 'APPEND_FEED',
            entry: makeFeedEntry('round_start', `⚔ Round ${rs.round ?? '?'} — turn order: ${order}`),
          })
          break
        }
        case 'RoundEndEvent': {
          dispatch({ type: 'SET_COMBAT_ROUND', round: null })
          const re = payload as { round?: number }
          dispatch({
            type: 'APPEND_FEED',
            entry: makeFeedEntry('round_end', `Round ${re.round ?? '?'} ended`),
          })
          break
        }
        case 'ErrorEvent': {
          const err = payload as { message?: string }
          dispatch({
            type: 'APPEND_FEED',
            entry: makeFeedEntry('error', `⚠ ${err.message ?? 'Unknown error'}`),
          })
          break
        }
        case 'Disconnected':
          void navigate('/characters')
          break
        default: {
          dispatch({
            type: 'APPEND_FEED',
            entry: makeFeedEntry('system', `[${msgType}] ${JSON.stringify(payload)}`),
          })
        }
      }
    }
  }, [navigate])

  useEffect(() => {
    unmountedRef.current = false
    connect()
    return () => {
      unmountedRef.current = true
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      wsRef.current?.close(1000, 'unmount')
    }
  }, [connect])

  return (
    <GameContext.Provider value={{ state, sendMessage, sendCommand }}>
      {children}
    </GameContext.Provider>
  )
}

export function useGame(): GameContextValue {
  const ctx = useContext(GameContext)
  if (!ctx) throw new Error('useGame must be used within a GameProvider')
  return ctx
}
