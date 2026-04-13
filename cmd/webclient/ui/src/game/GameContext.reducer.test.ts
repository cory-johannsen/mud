import { describe, it, expect } from 'vitest'
import fc from 'fast-check'
import { reducer, initialState } from './GameContext'

// REQ-PA-1d: GameContext MUST store combatGridWidth and combatGridHeight in state,
// defaulting to 20 if absent from the event.

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
