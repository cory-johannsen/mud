import { describe, it, expect } from 'vitest'
import fc from 'fast-check'
import { reducer, initialState } from './GameContext'
import type { QuestGiverView } from '../proto'

// REQ-PA-1d: GameContext MUST store combatGridWidth and combatGridHeight in state,
// defaulting to 20 if absent from the event.
// REQ-84: SET_QUEST_GIVER_VIEW MUST set questGiverView in state; setting to null MUST clear it.

describe('reducer: SET_COMBAT_GRID', () => {
  it('stores provided grid dimensions', () => {
    const state = reducer(initialState, { type: 'SET_COMBAT_GRID', width: 15, height: 10 })
    expect(state.combatGridWidth).toBe(15)
    expect(state.combatGridHeight).toBe(10)
  })

  it('defaults to 20×20 in initialState', () => {
    expect(initialState.combatGridWidth).toBe(20)
    expect(initialState.combatGridHeight).toBe(20)
  })

  it('does not mutate other state fields', () => {
    const state = reducer(initialState, { type: 'SET_COMBAT_GRID', width: 5, height: 5 })
    const { combatGridWidth, combatGridHeight, ...rest } = state
    const { combatGridWidth: _w, combatGridHeight: _h, ...initialRest } = initialState
    expect(rest).toEqual(initialRest)
  })

  it('property: SET_COMBAT_GRID reflects any valid positive dimensions', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 100 }),
        fc.integer({ min: 1, max: 100 }),
        (w, h) => {
          const state = reducer(initialState, { type: 'SET_COMBAT_GRID', width: w, height: h })
          return state.combatGridWidth === w && state.combatGridHeight === h
        }
      )
    )
  })
})

// REQ-84: Regression guard — QuestGiverView WebSocket message MUST update questGiverView state
// so the QuestGiverModal renders. The bug was: the case was missing from the ws.onmessage switch
// in an old deployment, causing the default case to log JSON to the feed instead of opening the
// modal. These tests prevent regressions in the reducer half of that dispatch chain.
describe('reducer: SET_QUEST_GIVER_VIEW', () => {
  const SAMPLE_VIEW: QuestGiverView = {
    npcName: 'Militia Quartermaster',
    npcInstanceId: 'vantucky_quest_giver-vantucky_the_compound-731',
    quests: [
      {
        questId: 'vtq_militia_patrol',
        title: 'Neutralize the Militia Patrol',
        description: 'Take out five Militia troopers patrolling the perimeter.',
        xpReward: 200,
        creditsReward: 100,
        status: 'available',
        objectives: [{ id: 'kill_patrol', description: 'Kill 5 Militia troopers', current: 0, required: 5 }],
      },
    ],
  }

  it('initialState has questGiverView null', () => {
    expect(initialState.questGiverView).toBeNull()
  })

  it('SET_QUEST_GIVER_VIEW with a view sets questGiverView', () => {
    const state = reducer(initialState, { type: 'SET_QUEST_GIVER_VIEW', view: SAMPLE_VIEW })
    expect(state.questGiverView).toEqual(SAMPLE_VIEW)
  })

  it('SET_QUEST_GIVER_VIEW with null clears questGiverView', () => {
    const withView = reducer(initialState, { type: 'SET_QUEST_GIVER_VIEW', view: SAMPLE_VIEW })
    const cleared = reducer(withView, { type: 'SET_QUEST_GIVER_VIEW', view: null })
    expect(cleared.questGiverView).toBeNull()
  })

  it('SET_QUEST_GIVER_VIEW does not mutate other state fields', () => {
    const state = reducer(initialState, { type: 'SET_QUEST_GIVER_VIEW', view: SAMPLE_VIEW })
    const { questGiverView, ...rest } = state
    const { questGiverView: _q, ...initialRest } = initialState
    expect(rest).toEqual(initialRest)
  })

  it('property: SET_QUEST_GIVER_VIEW always stores the provided view', () => {
    fc.assert(
      fc.property(
        fc.record({
          npcName: fc.string({ minLength: 1, maxLength: 50 }),
          npcInstanceId: fc.string({ minLength: 1, maxLength: 80 }),
          quests: fc.array(fc.record({
            questId: fc.string({ minLength: 1, maxLength: 30 }),
            title: fc.string({ minLength: 1, maxLength: 80 }),
            status: fc.constantFrom('available', 'active', 'completed', 'locked'),
          }), { maxLength: 5 }),
        }),
        (view) => {
          const state = reducer(initialState, { type: 'SET_QUEST_GIVER_VIEW', view: view as QuestGiverView })
          return state.questGiverView === view
        }
      )
    )
  })
})

// REQ-BUG195-1: QUEST_ADDED MUST increment questFlashCount each time it fires.
// REQ-BUG195-2: initialState MUST have questFlashCount === 0.
describe('reducer: QUEST_ADDED', () => {
  it('initialState has questFlashCount 0', () => {
    expect(initialState.questFlashCount).toBe(0)
  })

  it('QUEST_ADDED increments questFlashCount by 1', () => {
    const state = reducer(initialState, { type: 'QUEST_ADDED' })
    expect(state.questFlashCount).toBe(1)
  })

  it('QUEST_ADDED increments questFlashCount each time', () => {
    let state = initialState
    state = reducer(state, { type: 'QUEST_ADDED' })
    state = reducer(state, { type: 'QUEST_ADDED' })
    state = reducer(state, { type: 'QUEST_ADDED' })
    expect(state.questFlashCount).toBe(3)
  })

  it('QUEST_ADDED does not mutate other state fields', () => {
    const state = reducer(initialState, { type: 'QUEST_ADDED' })
    const { questFlashCount, ...rest } = state
    const { questFlashCount: _qfc, ...initialRest } = initialState
    expect(rest).toEqual(initialRest)
  })

  it('property: QUEST_ADDED always produces questFlashCount === initial + n after n dispatches', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 20 }),
        (n) => {
          let state = initialState
          for (let i = 0; i < n; i++) {
            state = reducer(state, { type: 'QUEST_ADDED' })
          }
          return state.questFlashCount === n
        }
      )
    )
  })
})
