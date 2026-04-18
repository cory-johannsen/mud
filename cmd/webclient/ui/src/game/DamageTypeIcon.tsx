export interface DamageTypeIconInfo {
  symbol: string
  color: string
}

const DAMAGE_TYPE_MAP: Record<string, DamageTypeIconInfo> = {
  acid:        { symbol: '◆', color: '#5dbb63' },
  bleed:       { symbol: '●', color: '#c0392b' },
  bludgeoning: { symbol: '⬟', color: '#888' },
  cold:        { symbol: '❅', color: '#7ecfed' },
  electricity: { symbol: '⚡', color: '#f0e060' },
  fire:        { symbol: '♨', color: '#f87040' },
  force:       { symbol: '✦', color: '#c080e0' },
  mental:      { symbol: '◈', color: '#9b7bd0' },
  neural:      { symbol: '⊕', color: '#50c8c8' },
  piercing:    { symbol: '▶', color: '#e08840' },
  poison:      { symbol: '⬡', color: '#78c050' },
  slashing:    { symbol: '∕', color: '#e05050' },
  sonic:       { symbol: '≋', color: '#a080d0' },
  spirit:      { symbol: '✧', color: '#d0d8f0' },
  untyped:     { symbol: '○', color: '#888' },
  vitality:    { symbol: '✚', color: '#60d080' },
  void:        { symbol: '◉', color: '#7050a0' },
}

/** All recognized damage type strings, derived from DAMAGE_TYPE_MAP. */
export const KNOWN_DAMAGE_TYPES: readonly string[] = Object.keys(DAMAGE_TYPE_MAP)

const FALLBACK: DamageTypeIconInfo = { symbol: '?', color: '#666' }

export function damageTypeInfo(damageType: string): DamageTypeIconInfo {
  return DAMAGE_TYPE_MAP[damageType.toLowerCase()] ?? FALLBACK
}

export function parseDamageType(summary: string): string {
  const trimmed = summary.trim()
  if (!trimmed) return ''
  const parts = trimmed.split(/\s+/)
  return parts[parts.length - 1].toLowerCase()
}

export function DamageTypeIcon({ damageType, size = '0.85em' }: { damageType: string; size?: string }): JSX.Element | null {
  if (!damageType) return null
  const info = damageTypeInfo(damageType)
  return (
    <span
      title={damageType}
      style={{ color: info.color, fontSize: size, lineHeight: 1, userSelect: 'none' }}
      aria-label={`${damageType} damage`}
    >
      {info.symbol}
    </span>
  )
}
