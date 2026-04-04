import { describe, it, expect } from 'vitest'
import { slotActivationCommand, slotDisplayLabel, slotTooltip } from './HotbarPanel'
import type { HotbarSlot } from '../../proto'

// ── slotActivationCommand ────────────────────────────────────────────────────

describe('slotActivationCommand', () => {
  it('returns "" for empty ref', () => {
    expect(slotActivationCommand({ kind: 'feat', ref: '' })).toBe('')
  })

  it('returns "use ref" for feat kind', () => {
    expect(slotActivationCommand({ kind: 'feat', ref: 'power_strike' })).toBe('use power_strike')
  })

  it('returns "use ref" for technology kind', () => {
    expect(slotActivationCommand({ kind: 'technology', ref: 'neural_hack' })).toBe('use neural_hack')
  })

  it('returns "use ref" for consumable kind', () => {
    expect(slotActivationCommand({ kind: 'consumable', ref: 'stimpak' })).toBe('use stimpak')
  })

  it('returns "throw ref" for throwable kind', () => {
    expect(slotActivationCommand({ kind: 'throwable', ref: 'frag_grenade' })).toBe('throw frag_grenade')
  })

  it('returns ref directly for command kind', () => {
    expect(slotActivationCommand({ kind: 'command', ref: 'look north' })).toBe('look north')
  })

  it('returns ref directly for empty kind (default)', () => {
    expect(slotActivationCommand({ kind: '', ref: 'status' })).toBe('status')
  })

  it('returns ref directly for unrecognized kind', () => {
    expect(slotActivationCommand({ kind: 'future_kind', ref: 'some_ref' })).toBe('some_ref')
  })
})

// ── slotDisplayLabel ─────────────────────────────────────────────────────────

describe('slotDisplayLabel', () => {
  it('returns "" when ref is empty or undefined', () => {
    expect(slotDisplayLabel({ kind: 'command', ref: '' })).toBe('')
    expect(slotDisplayLabel({ kind: 'command', ref: undefined as unknown as string })).toBe('')
  })

  it('prefers displayName over display_name and ref', () => {
    const slot: HotbarSlot = { kind: 'feat', ref: 'power_strike', displayName: 'Power Strike', display_name: 'PS', description: 'desc' }
    expect(slotDisplayLabel(slot)).toBe('Power Strike')
  })

  it('falls back to display_name when displayName absent', () => {
    const slot: HotbarSlot = { kind: 'feat', ref: 'power_strike', display_name: 'Power Strike' }
    expect(slotDisplayLabel(slot)).toBe('Power Strike')
  })

  it('falls back to ref when both display names absent', () => {
    const slot: HotbarSlot = { kind: 'command', ref: 'look' }
    expect(slotDisplayLabel(slot)).toBe('look')
  })
})

// ── slotTooltip ──────────────────────────────────────────────────────────────

describe('slotTooltip', () => {
  it('returns "empty" for empty ref', () => {
    expect(slotTooltip({ kind: 'command', ref: '' })).toBe('empty')
  })

  it('returns "ref (right-click to edit)" for command kind', () => {
    expect(slotTooltip({ kind: 'command', ref: 'look north' })).toBe('look north (right-click to edit)')
  })

  it('returns "ref (right-click to edit)" for empty kind', () => {
    expect(slotTooltip({ kind: '', ref: 'status' })).toBe('status (right-click to edit)')
  })

  it('returns displayName + right-click hint for typed slot without description', () => {
    const slot: HotbarSlot = { kind: 'feat', ref: 'power_strike', displayName: 'Power Strike' }
    expect(slotTooltip(slot)).toBe('Power Strike\n(right-click to edit)')
  })

  it('returns displayName + description + right-click hint for typed slot with description', () => {
    const slot: HotbarSlot = { kind: 'feat', ref: 'power_strike', displayName: 'Power Strike', description: 'A mighty blow.' }
    expect(slotTooltip(slot)).toBe('Power Strike\nA mighty blow.\n(right-click to edit)')
  })

  it('falls back to ref when displayName absent', () => {
    const slot: HotbarSlot = { kind: 'technology', ref: 'neural_hack' }
    expect(slotTooltip(slot)).toBe('neural_hack\n(right-click to edit)')
  })
})
