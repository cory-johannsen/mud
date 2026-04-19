// GameContext manages WebSocket connection, message dispatch, and game state.
// REQ-CTX-1: Connect to ws://<host>/ws?token=<JWT> on mount.
// REQ-CTX-2: sendMessage serializes and sends frames, queuing if not OPEN.
// REQ-CTX-3: sendCommand sends CommandRequest frames.
// REQ-CTX-4: Incoming frames dispatch on type field to update state.
// REQ-CTX-5: On RoomView, update state.roomView AND append room_event feed entry.
// REQ-CTX-6: On RoundStartEvent, set state.combatRound. On RoundEndEvent, set to null.
// REQ-CTX-7: On Disconnected, navigate to /characters.
// REQ-CTX-8: Auto-reconnect with backoff: 1s, 2s, 4s, 8s, max 30s. Only code 1000 (client-initiated close) suppresses reconnect.
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
  WorldZoneTile,
  RoundStartEvent,
  CombatantPosition,
  ConditionEvent,
  TimeOfDayEvent,
  ShopView,
  WeatherEvent,
  LoadoutView,
  HotbarSlot,
  JobGrantsResponse,
  APUpdateEvent,
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

export interface CombatantHp {
  current: number
  max: number
}

export interface SlotContext {
  slotNum: number
  totalSlots: number
  slotLevel: number
}

export interface ChoicePrompt {
  featureId: string
  prompt: string
  options: string[]
  slotContext?: SlotContext
}

export interface CombatantAP {
  remaining: number
  total: number
  movementRemaining: number  // how many movement actions (Stride/Step) remain this round; max 2
}

export interface GameState {
  connected: boolean
  roomView: RoomView | null
  characterInfo: CharacterInfo | null
  characterSheet: CharacterSheetView | null
  inventoryView: InventoryView | null
  mapTiles: MapTile[]
  worldTiles: WorldZoneTile[]
  feedEntries: FeedEntry[]
  combatRound: RoundStartEvent | null
  combatGridWidth: number
  combatGridHeight: number
  combatPositions: Record<string, { x: number; y: number }>
  combatantHp: Record<string, CombatantHp>
  combatantAP: Record<string, CombatantAP>
  hotbarSlots: HotbarSlot[]
  timeOfDay: TimeOfDayEvent | null
  activeWeather: string | null
  activeWeatherDescription: string | null
  shopView: ShopView | null
  healerView: import('../proto').HealerView | null
  trainerView: import('../proto').TrainerView | null
  techTrainerView: import('../proto').TechTrainerView | null
  fixerView: import('../proto').FixerView | null
  restView: import('../proto').RestView | null
  npcView: { name: string; description: string; npcType: string; level: number; health: string } | null
  combatNpcView: { name: string; description: string; npcType: string; level: number; health: string } | null
  questGiverView: import('../proto').QuestGiverView | null
  questLogView: import('../proto').QuestLogView | null
  questCompleteQueue: import('../proto').QuestCompleteEvent[]
  loadoutView: LoadoutView | null
  choicePrompt: ChoicePrompt | null
  jobGrants: JobGrantsResponse | null
}

type Action =
  | { type: 'SET_CONNECTED'; connected: boolean }
  | { type: 'SET_ROOM'; room: RoomView }
  | { type: 'SET_CHARACTER_INFO'; info: CharacterInfo }
  | { type: 'SET_CHARACTER_SHEET'; sheet: CharacterSheetView }
  | { type: 'SET_INVENTORY'; inv: InventoryView }
  | { type: 'SET_MAP_TILES'; tiles: MapTile[] }
  | { type: 'SET_WORLD_TILES'; tiles: WorldZoneTile[] }
  | { type: 'SET_COMBAT_ROUND'; round: RoundStartEvent | null }
  | { type: 'START_COMBAT_ROUND'; round: RoundStartEvent; positions: Record<string, { x: number; y: number }>; ap: Record<string, CombatantAP>; gridWidth: number; gridHeight: number }
  | { type: 'SET_COMBAT_GRID'; width: number; height: number }
  | { type: 'UPDATE_COMBAT_POSITION'; combatantName: string; x: number; y: number }
  | { type: 'CLEAR_COMBAT_POSITIONS' }
  | { type: 'UPDATE_COMBATANT_HP'; name: string; current: number; max: number }
  | { type: 'CLEAR_COMBATANT_HP' }
  | { type: 'UPDATE_COMBATANT_AP'; name: string; remaining: number; total: number; movementRemaining?: number }
  | { type: 'CLEAR_COMBATANT_AP' }
  | { type: 'SET_HOTBAR'; slots: HotbarSlot[] }
  | { type: 'SET_TIME_OF_DAY'; tod: TimeOfDayEvent }
  | { type: 'SET_ACTIVE_WEATHER'; weather: string | null; description: string | null }
  | { type: 'UPDATE_PLAYER_HP'; current: number; max: number }
  | { type: 'APPEND_FEED'; entry: FeedEntry }
  | { type: 'SET_SHOP_VIEW'; shop: ShopView | null }
  | { type: 'SET_HEALER_VIEW'; view: import('../proto').HealerView | null }
  | { type: 'SET_TRAINER_VIEW'; view: import('../proto').TrainerView | null }
  | { type: 'SET_TECH_TRAINER_VIEW'; view: import('../proto').TechTrainerView | null }
  | { type: 'SET_FIXER_VIEW'; view: import('../proto').FixerView | null }
  | { type: 'SET_REST_VIEW'; view: import('../proto').RestView | null }
  | { type: 'SET_NPC_VIEW'; view: { name: string; description: string; npcType: string; level: number; health: string } | null }
  | { type: 'SET_COMBAT_NPC_VIEW'; view: { name: string; description: string; npcType: string; level: number; health: string } | null }
  | { type: 'SET_QUEST_GIVER_VIEW'; view: import('../proto').QuestGiverView | null }
  | { type: 'SET_QUEST_LOG_VIEW'; view: import('../proto').QuestLogView | null }
  | { type: 'ENQUEUE_QUEST_COMPLETE'; event: import('../proto').QuestCompleteEvent }
  | { type: 'DEQUEUE_QUEST_COMPLETE' }
  | { type: 'SET_LOADOUT_VIEW'; view: LoadoutView | null }
  | { type: 'SET_CHOICE_PROMPT'; prompt: ChoicePrompt }
  | { type: 'CLEAR_CHOICE_PROMPT' }
  | { type: 'SET_JOB_GRANTS'; grants: JobGrantsResponse | null }

export function reducer(state: GameState, action: Action): GameState {
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
    case 'SET_WORLD_TILES':
      return { ...state, worldTiles: action.tiles }
    case 'SET_COMBAT_ROUND':
      return { ...state, combatRound: action.round }
    case 'START_COMBAT_ROUND':
      return {
        ...state,
        combatRound: action.round,
        combatPositions: action.positions,
        combatantAP: { ...state.combatantAP, ...action.ap },
        combatGridWidth: action.gridWidth,
        combatGridHeight: action.gridHeight,
      }
    case 'SET_COMBAT_GRID':
      return { ...state, combatGridWidth: action.width, combatGridHeight: action.height }
    case 'UPDATE_COMBAT_POSITION':
      return { ...state, combatPositions: { ...state.combatPositions, [action.combatantName]: { x: action.x, y: action.y } } }
    case 'CLEAR_COMBAT_POSITIONS':
      return { ...state, combatPositions: {} }
    case 'UPDATE_COMBATANT_HP': {
      const newHp = { ...state.combatantHp, [action.name]: { current: action.current, max: action.max } }
      const isPlayer = state.characterInfo?.name === action.name
      return {
        ...state,
        combatantHp: newHp,
        characterInfo: isPlayer && state.characterInfo
          ? { ...state.characterInfo, currentHp: action.current, current_hp: action.current, maxHp: action.max, max_hp: action.max }
          : state.characterInfo,
      }
    }
    case 'CLEAR_COMBATANT_HP':
      return { ...state, combatantHp: {} }
    case 'UPDATE_COMBATANT_AP':
      return { ...state, combatantAP: { ...state.combatantAP, [action.name]: { remaining: action.remaining, total: action.total, movementRemaining: action.movementRemaining ?? 2 } } }
    case 'CLEAR_COMBATANT_AP':
      return { ...state, combatantAP: {} }
    case 'SET_HOTBAR':
      return { ...state, hotbarSlots: action.slots }
    case 'UPDATE_PLAYER_HP':
      return {
        ...state,
        characterInfo: state.characterInfo
          ? { ...state.characterInfo, currentHp: action.current, current_hp: action.current, maxHp: action.max, max_hp: action.max }
          : state.characterInfo,
      }
    case 'SET_TIME_OF_DAY':
      return { ...state, timeOfDay: action.tod }
    case 'SET_ACTIVE_WEATHER':
      return { ...state, activeWeather: action.weather, activeWeatherDescription: action.description }
    case 'SET_SHOP_VIEW':
      return { ...state, shopView: action.shop }
    case 'SET_HEALER_VIEW':
      return { ...state, healerView: action.view }
    case 'SET_TRAINER_VIEW':
      return { ...state, trainerView: action.view }
    case 'SET_TECH_TRAINER_VIEW':
      return { ...state, techTrainerView: action.view }
    case 'SET_FIXER_VIEW':
      return { ...state, fixerView: action.view }
    case 'SET_REST_VIEW':
      return { ...state, restView: action.view }
    case 'SET_NPC_VIEW':
      return { ...state, npcView: action.view }
    case 'SET_COMBAT_NPC_VIEW':
      return { ...state, combatNpcView: action.view }
    case 'SET_QUEST_GIVER_VIEW':
      return { ...state, questGiverView: action.view }
    case 'SET_QUEST_LOG_VIEW':
      return { ...state, questLogView: action.view }
    case 'ENQUEUE_QUEST_COMPLETE':
      return { ...state, questCompleteQueue: [...state.questCompleteQueue, action.event] }
    case 'DEQUEUE_QUEST_COMPLETE':
      return { ...state, questCompleteQueue: state.questCompleteQueue.slice(1) }
    case 'SET_LOADOUT_VIEW':
      return { ...state, loadoutView: action.view }
    case 'SET_CHOICE_PROMPT':
      return { ...state, choicePrompt: action.prompt }
    case 'CLEAR_CHOICE_PROMPT':
      return { ...state, choicePrompt: null }
    case 'SET_JOB_GRANTS':
      return { ...state, jobGrants: action.grants }
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

export const initialState: GameState = {
  connected: false,
  roomView: null,
  characterInfo: null,
  characterSheet: null,
  inventoryView: null,
  mapTiles: [],
  worldTiles: [],
  feedEntries: [],
  combatRound: null,
  combatGridWidth: 20,
  combatGridHeight: 20,
  combatPositions: {},
  combatantHp: {},
  combatantAP: {},
  hotbarSlots: Array(10).fill({ kind: 'command', ref: '' }) as HotbarSlot[],
  timeOfDay: null,
  activeWeather: null,
  activeWeatherDescription: null,
  shopView: null,
  healerView: null,
  trainerView: null,
  techTrainerView: null,
  fixerView: null,
  restView: null,
  npcView: null,
  combatNpcView: null,
  questGiverView: null,
  questLogView: null,
  questCompleteQueue: [],
  loadoutView: null,
  choicePrompt: null,
  jobGrants: null,
}

interface GameContextValue {
  state: GameState
  sendMessage: (type: string, payload: object) => void
  sendCommand: (raw: string) => void
  clearShop: () => void
  clearHealer: () => void
  clearTrainer: () => void
  clearTechTrainer: () => void
  clearFixer: () => void
  clearRestView: () => void
  clearNpcView: () => void
  clearCombatNpcView: () => void
  clearQuestGiverView: () => void
  dismissQuestComplete: () => void
  clearLoadout: () => void
  clearChoicePrompt: () => void
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
  const lastRoomIdRef = useRef<string | null>(null)
  const hasConnectedRef = useRef(false)

  const sendMessage = useCallback((type: string, payload: object) => {
    const frame = JSON.stringify({ type, payload })
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(frame)
    } else {
      queueRef.current.push(frame)
    }
  }, [])

  const sendCommand = useCallback((raw: string) => {
    sendMessage('CommandText', { text: raw })
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
      const isReconnect = hasConnectedRef.current
      hasConnectedRef.current = true
      dispatch({ type: 'SET_CONNECTED', connected: true })
      backoffRef.current = 1000
      if (isReconnect) {
        // Clear stale combat state so the server can restore it via a fresh RoundStartEvent
        // pushed during reconnect handling. This avoids stale combat UI when combat ended
        // while the client was disconnected (BUG-143).
        dispatch({ type: 'SET_COMBAT_ROUND', round: null })
        dispatch({ type: 'CLEAR_COMBAT_POSITIONS' })
        dispatch({ type: 'CLEAR_COMBATANT_HP' })
        dispatch({ type: 'CLEAR_COMBATANT_AP' })
        dispatch({ type: 'SET_COMBAT_GRID', width: 20, height: 20 })
        // Reconnect notification suppressed — server restores state seamlessly (BUG-143).
      }
      while (queueRef.current.length > 0) {
        ws.send(queueRef.current.shift()!)
      }
      // Request character info and map so panels populate immediately on connect.
      ws.send(JSON.stringify({ type: 'StatusRequest', payload: {} }))
      ws.send(JSON.stringify({ type: 'MapRequest', payload: {} }))
    }

    ws.onclose = (ev) => {
      dispatch({ type: 'SET_CONNECTED', connected: false })
      if (unmountedRef.current) return
      // Code 1000 = normal closure initiated by this client (e.g. unmount). Do not reconnect.
      // Code 1001 = server going away (redeploy, restart). Reconnect with backoff.
      if (ev.code === 1000) return
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
          const incomingId = room.roomId ?? room.title ?? null
          if (incomingId !== lastRoomIdRef.current) {
            lastRoomIdRef.current = incomingId
            dispatch({
              type: 'APPEND_FEED',
              entry: makeFeedEntry('room_event', `— ${room.title ?? 'Room'} —`),
            })
            sendMessage('MapRequest', { view: 'zone' })
          }
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
          const map = payload as { tiles?: MapTile[]; worldTiles?: WorldZoneTile[]; world_tiles?: WorldZoneTile[] }
          if (map.tiles !== undefined) {
            dispatch({ type: 'SET_MAP_TILES', tiles: map.tiles })
          }
          const wt = map.worldTiles ?? map.world_tiles
          // EmitUnpopulated: true on the server marshaler causes nil repeated fields
          // to appear as [] in zone map responses. Guard against clearing world tiles
          // with an empty array from a non-world response.
          if (wt !== undefined && wt.length > 0) {
            dispatch({ type: 'SET_WORLD_TILES', tiles: wt })
          }
          break
        }
        case 'MessageEvent': {
          const msg = payload as { sender?: string; content?: string }
          dispatch({
            type: 'APPEND_FEED',
            entry: makeFeedEntry('message', msg.sender ? `${msg.sender}: ${msg.content ?? ''}` : (msg.content ?? '')),
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
          const ce = payload as { type?: string; narrative?: string; attacker?: string; target?: string; damage?: number; attackerPosition?: number; attacker_position?: number; attackerX?: number; attackerY?: number; flanking?: boolean; targetHp?: number; targetMaxHp?: number; target_hp?: number; target_max_hp?: number }
          if (ce.type === 'COMBAT_EVENT_TYPE_POSITION') {
            if (ce.attacker) {
              dispatch({
                type: 'UPDATE_COMBAT_POSITION',
                combatantName: ce.attacker ?? '',
                x: ce.attackerX ?? 0,
                y: ce.attackerY ?? 0,
              })
            }
            break
          }
          if (ce.type === 'COMBAT_EVENT_TYPE_END' || ce.type === 'COMBAT_EVENT_TYPE_FLEE') {
            dispatch({ type: 'SET_COMBAT_ROUND', round: null })
            dispatch({ type: 'CLEAR_COMBAT_POSITIONS' })
            dispatch({ type: 'CLEAR_COMBATANT_HP' })
            dispatch({ type: 'CLEAR_COMBATANT_AP' })
            dispatch({ type: 'SET_COMBAT_GRID', width: 20, height: 20 })
          }
          // Track target HP from attack events.
          const tHp = ce.targetHp ?? ce.target_hp
          const tMaxHp = ce.targetMaxHp ?? ce.target_max_hp
          if (ce.target && tHp !== undefined && tMaxHp !== undefined && tMaxHp > 0) {
            // UPDATE_COMBATANT_HP also updates characterInfo HP when the target is the player.
            dispatch({ type: 'UPDATE_COMBATANT_HP', name: ce.target, current: tHp, max: tMaxHp })
          }
          const text = ce.narrative
            ? ce.narrative
            : `${ce.attacker ?? '?'} → ${ce.target ?? '?'}: ${ce.damage ?? 0} dmg`
          dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('combat', text) })
          break
        }
        case 'RoundStartEvent': {
          const rs = payload as RoundStartEvent
          const actionsPerTurn = rs.actionsPerTurn ?? rs.actions_per_turn ?? 3
          // Build positions and AP maps atomically to avoid intermediate renders
          // where combatRound is set but combatPositions is empty (REQ-BUG183-1).
          const positions: Record<string, { x: number; y: number }> = {}
          const apMap: Record<string, CombatantAP> = {}
          const hpUpdates: Array<{ name: string; current: number; max: number }> = []
          if (rs.initialPositions) {
            for (const pos of rs.initialPositions as CombatantPosition[]) {
              positions[pos.name] = { x: pos.x ?? 0, y: pos.y ?? 0 }
              const apTotal = pos.apTotal ?? pos.ap_total ?? actionsPerTurn
              const apRemaining = pos.apRemaining ?? pos.ap_remaining ?? apTotal
              apMap[pos.name] = { remaining: apRemaining, total: apTotal, movementRemaining: 2 }
              const hpMax = pos.hpMax ?? pos.hp_max ?? 0
              const hpCurrent = pos.hpCurrent ?? pos.hp_current ?? 0
              if (hpMax > 0) {
                hpUpdates.push({ name: pos.name, current: hpCurrent, max: hpMax })
              }
            }
          }
          // Single dispatch: round + positions + AP + grid all in one state update (REQ-BUG183-1)
          dispatch({
            type: 'START_COMBAT_ROUND',
            round: rs,
            positions,
            ap: apMap,
            gridWidth: rs.gridWidth ?? rs.grid_width ?? 20,
            gridHeight: rs.gridHeight ?? rs.grid_height ?? 20,
          })
          // HP updates don't affect movement range display; can dispatch separately.
          for (const hp of hpUpdates) {
            dispatch({ type: 'UPDATE_COMBATANT_HP', name: hp.name, current: hp.current, max: hp.max })
          }
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
        case 'APUpdateEvent': {
          const au = payload as APUpdateEvent & { movement_ap_remaining?: number; movementApRemaining?: number }
          if (au.name) {
            const remaining = au.apRemaining ?? au.ap_remaining ?? 0
            const total = au.apTotal ?? au.ap_total ?? 0
            const movementRemaining = au.movementApRemaining ?? au.movement_ap_remaining ?? 2
            dispatch({ type: 'UPDATE_COMBATANT_AP', name: au.name, remaining, total, movementRemaining })
          }
          break
        }
        case 'TimeOfDay': {
          dispatch({ type: 'SET_TIME_OF_DAY', tod: payload as TimeOfDayEvent })
          break
        }
        case 'WeatherEvent': {
          const ev = payload as WeatherEvent
          dispatch({
            type: 'SET_ACTIVE_WEATHER',
            weather: ev.active ? (ev.weatherName ?? ev.weather_name ?? null) : null,
            description: ev.active ? (ev.description ?? null) : null,
          })
          break
        }
        case 'ConditionEvent': {
          const ce = payload as ConditionEvent
          const name = ce.conditionName ?? ce.condition_name ?? ''
          const target = ce.targetName ?? ''
          if (name) {
            const text = ce.applied
              ? `[CONDITION] ${target} is now ${name} (stacks: ${ce.stacks ?? 1})`
              : `[CONDITION] ${name} fades from ${target}`
            dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('system', text) })
          }
          break
        }
        case 'HotbarUpdate': {
          const hu = payload as { slots?: HotbarSlot[] }
          const slots = Array.isArray(hu.slots)
            ? hu.slots
            : (Array(10).fill({ kind: 'command', ref: '' }) as HotbarSlot[])
          dispatch({ type: 'SET_HOTBAR', slots })
          break
        }
        case 'UseResponse': {
          const ur = payload as { message?: string; choices?: Array<{ name?: string; featId?: string }> }
          if (ur.message) {
            dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('system', ur.message) })
          } else if (Array.isArray(ur.choices) && ur.choices.length > 0) {
            const names = ur.choices.map((c) => c.name ?? c.featId ?? '?').join(', ')
            dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('system', `Available: ${names}`) })
          }
          break
        }
        case 'NpcView': {
          const nv = payload as { name?: string; description?: string; healthDescription?: string; health_description?: string; level?: number; npcType?: string; npc_type?: string }
          const npcType = nv.npcType ?? nv.npc_type ?? ''
          const combatTypes = new Set(['combat', ''])
          const npcViewPayload = {
            name: nv.name ?? 'Unknown',
            description: nv.description ?? '',
            npcType,
            level: nv.level ?? 0,
            health: nv.healthDescription ?? nv.health_description ?? '',
          }
          if (!combatTypes.has(npcType)) {
            // Non-combat NPC: show as modal
            dispatch({ type: 'SET_NPC_VIEW', view: npcViewPayload })
          } else if (state.combatRound !== null) {
            // Combat NPC examined during active combat: show combat examine modal
            dispatch({ type: 'SET_COMBAT_NPC_VIEW', view: npcViewPayload })
          } else {
            // Combat NPC outside combat: append to feed
            const health = nv.healthDescription ?? nv.health_description ?? ''
            const lines = [
              `${nv.name ?? 'Unknown'} (level ${nv.level ?? '?'}) — ${health}`,
              nv.description ?? '',
            ].filter(Boolean).join('\n')
            dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('system', lines) })
          }
          break
        }
        case 'ShopView': {
          dispatch({ type: 'SET_SHOP_VIEW', shop: payload as import('../proto').ShopView })
          break
        }
        case 'HealerView': {
          dispatch({ type: 'SET_HEALER_VIEW', view: payload as import('../proto').HealerView })
          break
        }
        case 'TrainerView': {
          dispatch({ type: 'SET_TRAINER_VIEW', view: payload as import('../proto').TrainerView })
          break
        }
        case 'TechTrainerView': {
          dispatch({ type: 'SET_TECH_TRAINER_VIEW', view: payload as import('../proto').TechTrainerView })
          break
        }
        case 'FixerView': {
          dispatch({ type: 'SET_FIXER_VIEW', view: payload as import('../proto').FixerView })
          break
        }
        case 'RestView': {
          dispatch({ type: 'SET_REST_VIEW', view: payload as import('../proto').RestView })
          break
        }
        case 'QuestGiverView': {
          dispatch({ type: 'SET_QUEST_GIVER_VIEW', view: payload as import('../proto').QuestGiverView })
          break
        }
        case 'QuestLogView': {
          dispatch({ type: 'SET_QUEST_LOG_VIEW', view: payload as import('../proto').QuestLogView })
          break
        }
        case 'QuestCompleteEvent': {
          const qce = payload as import('../proto').QuestCompleteEvent
          dispatch({ type: 'ENQUEUE_QUEST_COMPLETE', event: qce })
          const xp = qce.xpReward ?? qce.xp_reward ?? 0
          const credits = qce.creditsReward ?? qce.credits_reward ?? 0
          const items = qce.itemRewards ?? qce.item_rewards ?? []
          const rewards = [
            xp > 0 ? `+${xp} XP` : '',
            credits > 0 ? `+${credits} Credits` : '',
            ...items.map((it: string) => `+${it}`),
          ].filter(Boolean).join('  ')
          const text = rewards
            ? `✓ Quest Complete: ${qce.title ?? ''} — ${rewards}`
            : `✓ Quest Complete: ${qce.title ?? ''}`
          dispatch({ type: 'APPEND_FEED', entry: makeFeedEntry('system', text) })
          break
        }
        case 'LoadoutView': {
          dispatch({ type: 'SET_LOADOUT_VIEW', view: payload as LoadoutView })
          break
        }
        case 'JobGrantsResponse': {
          dispatch({ type: 'SET_JOB_GRANTS', grants: payload as JobGrantsResponse })
          break
        }
        case 'FeatureChoicePrompt': {
          const cp = payload as { featureId?: string; prompt?: string; options?: string[]; slotContext?: SlotContext }
          dispatch({
            type: 'SET_CHOICE_PROMPT',
            prompt: {
              featureId: cp.featureId ?? '',
              prompt: cp.prompt ?? '',
              options: Array.isArray(cp.options) ? cp.options : [],
              slotContext: cp.slotContext,
            },
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
        case 'HpUpdate': {
          const hp = payload as { currentHp?: number; current_hp?: number; maxHp?: number; max_hp?: number }
          const current = hp.currentHp ?? hp.current_hp
          const max = hp.maxHp ?? hp.max_hp
          if (current !== undefined && max !== undefined) {
            dispatch({ type: 'UPDATE_PLAYER_HP', current, max })
          }
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

  const clearShop = useCallback(() => {
    dispatch({ type: 'SET_SHOP_VIEW', shop: null })
  }, [])

  const clearHealer = useCallback(() => {
    dispatch({ type: 'SET_HEALER_VIEW', view: null })
  }, [])

  const clearTrainer = useCallback(() => {
    dispatch({ type: 'SET_TRAINER_VIEW', view: null })
  }, [])

  const clearTechTrainer = useCallback(() => {
    dispatch({ type: 'SET_TECH_TRAINER_VIEW', view: null })
  }, [])

  const clearFixer = useCallback(() => {
    dispatch({ type: 'SET_FIXER_VIEW', view: null })
  }, [])

  const clearRestView = useCallback(() => {
    dispatch({ type: 'SET_REST_VIEW', view: null })
  }, [])

  const clearNpcView = useCallback(() => {
    dispatch({ type: 'SET_NPC_VIEW', view: null })
  }, [])

  const clearCombatNpcView = useCallback(() => {
    dispatch({ type: 'SET_COMBAT_NPC_VIEW', view: null })
  }, [])

  const clearQuestGiverView = useCallback(() => {
    dispatch({ type: 'SET_QUEST_GIVER_VIEW', view: null })
  }, [])

  const dismissQuestComplete = useCallback(() => {
    dispatch({ type: 'DEQUEUE_QUEST_COMPLETE' })
  }, [])
  const clearLoadout = useCallback(() => {
    dispatch({ type: 'SET_LOADOUT_VIEW', view: null })
  }, [])

  const clearChoicePrompt = useCallback(() => {
    dispatch({ type: 'CLEAR_CHOICE_PROMPT' })
  }, [])

  return (
    <GameContext.Provider value={{ state, sendMessage, sendCommand, clearShop, clearHealer, clearTrainer, clearTechTrainer, clearFixer, clearRestView, clearNpcView, clearCombatNpcView, clearQuestGiverView, dismissQuestComplete, clearLoadout, clearChoicePrompt }}>
      {children}
    </GameContext.Provider>
  )
}

export function useGame(): GameContextValue {
  const ctx = useContext(GameContext)
  if (!ctx) throw new Error('useGame must be used within a GameProvider')
  return ctx
}
