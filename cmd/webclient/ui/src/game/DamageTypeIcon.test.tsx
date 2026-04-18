import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { DamageTypeIcon, damageTypeInfo, parseDamageType } from './DamageTypeIcon'

const ALL_TYPES = [
  'acid', 'bleed', 'bludgeoning', 'cold', 'electricity', 'fire', 'force',
  'mental', 'neural', 'piercing', 'poison', 'slashing', 'sonic', 'spirit',
  'untyped', 'vitality', 'void',
]

describe('damageTypeInfo', () => {
  it('returns non-empty symbol and color for every known type', () => {
    for (const t of ALL_TYPES) {
      const info = damageTypeInfo(t)
      expect(info.symbol.length).toBeGreaterThan(0)
      expect(info.color.startsWith('#')).toBe(true)
    }
  })

  it('returns fallback for unknown type', () => {
    const info = damageTypeInfo('unknown')
    expect(info.symbol.length).toBeGreaterThan(0)
    expect(info.color.startsWith('#')).toBe(true)
  })
})

describe('parseDamageType', () => {
  it('extracts type from "2d6 fire" format', () => {
    expect(parseDamageType('2d6 fire')).toBe('fire')
  })

  it('extracts type from "1d8+3 slashing" format', () => {
    expect(parseDamageType('1d8+3 slashing')).toBe('slashing')
  })

  it('returns empty string for empty input', () => {
    expect(parseDamageType('')).toBe('')
  })

  it('handles single word (type only)', () => {
    expect(parseDamageType('poison')).toBe('poison')
  })
})

describe('DamageTypeIcon', () => {
  it('renders with title for known type', () => {
    render(<DamageTypeIcon damageType="fire" />)
    expect(screen.getByTitle(/fire/i)).toBeTruthy()
  })

  it('renders nothing for empty string', () => {
    const { container } = render(<DamageTypeIcon damageType="" />)
    expect(container.firstChild).toBeNull()
  })
})
