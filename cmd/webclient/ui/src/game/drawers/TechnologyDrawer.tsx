import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type {
  HardwiredSlotView,
  HotbarSlot,
  InnateSlotView,
  PreparedSlotView,
  SpontaneousKnownEntry,
  SpontaneousUsePoolView,
} from '../../proto'
import { HotbarSlotPicker } from '../HotbarSlotPicker'
import { DamageTypeIcon, KNOWN_DAMAGE_TYPES } from '../DamageTypeIcon'

function SectionLabel({ label }: { label: string }) {
  return <div style={styles.sectionLabel}>{label}</div>
}

function LevelBadge({ level }: { level: number }) {
  if (!level || level <= 0) return null
  return <span style={styles.levelBadge}>Lv {level}</span>
}

function UsePips({ remaining, max }: { remaining: number; max: number }) {
  return (
    <span style={styles.pips} title={`${remaining} / ${max}`}>
      {Array.from({ length: max }, (_, i) => (
        <span key={i} style={{ ...styles.pip, ...(i < remaining ? styles.pipFull : styles.pipEmpty) }} />
      ))}
      <span style={styles.pipLabel}>{remaining}/{max}</span>
    </span>
  )
}

function InfiniteUses() {
  return <span style={styles.infiniteUses} title="Unlimited uses">∞</span>
}

function detectDamageType(line: string): string {
  const lower = line.toLowerCase()
  return KNOWN_DAMAGE_TYPES.find(t => lower.includes(t)) ?? ''
}

function EffectsSummary({ text }: { text: string }) {
  const lines = text.split('\n')
  return (
    <div style={styles.effectsSummary}>
      {lines.map((line, i) => {
        const dt = detectDamageType(line)
        return (
          <div key={i} style={{ ...(line.startsWith('  ') ? styles.effectsIndent : undefined), display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
            {dt && <DamageTypeIcon damageType={dt} size="0.8em" />}
            <span>{line}</span>
          </div>
        )
      })}
    </div>
  )
}

// Active tech item — Prepared
function PreparedItem({
  slot,
  total,
  remaining,
  hotbarSlots,
  sendMessage,
}: {
  slot: PreparedSlotView
  total: number
  remaining: number
  hotbarSlots: HotbarSlot[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const level = slot.techLevel ?? slot.tech_level ?? 0
  const exhausted = remaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, kind: 'technology', ref: techId })
    setPicking(false)
  }

  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: exhausted ? '#666' : '#e0c060', textDecoration: exhausted ? 'line-through' : 'none' }}>
          {name}
        </strong>
        <LevelBadge level={level} />
        <span style={styles.badgeActive}>active</span>
        {total >= 1 && <UsePips remaining={remaining} max={total} />}
      </div>
      {slot.description && <p style={styles.techDesc}>{slot.description}</p>}
      {slot.effectsSummary && <EffectsSummary text={slot.effectsSummary} />}
      {!exhausted && !picking && (
        <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
          + Add to Hotbar
        </button>
      )}
      {picking && (
        <HotbarSlotPicker hotbarSlots={hotbarSlots} onPick={handlePick} onCancel={() => setPicking(false)} />
      )}
    </li>
  )
}

// Active tech item — Innate
function InnateItem({
  slot,
  hotbarSlots,
  sendMessage,
}: {
  slot: InnateSlotView
  hotbarSlots: HotbarSlot[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const level = slot.techLevel ?? slot.tech_level ?? 0
  const remaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
  const max = slot.maxUses ?? slot.max_uses ?? 0
  const exhausted = max > 0 && remaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, kind: 'technology', ref: techId })
    setPicking(false)
  }

  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: exhausted ? '#666' : '#e0c060' }}>{name}</strong>
        <LevelBadge level={level} />
        <span style={styles.badgeInnate}>innate</span>
        {slot.isReaction && <span style={styles.badgeReaction}>reaction</span>}
        {max === 0 ? <InfiniteUses /> : <UsePips remaining={remaining} max={max} />}
      </div>
      {slot.description && <p style={styles.techDesc}>{slot.description}</p>}
      {slot.effectsSummary && <EffectsSummary text={slot.effectsSummary} />}
      {!exhausted && !slot.isReaction && !picking && (
        <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
          + Add to Hotbar
        </button>
      )}
      {picking && (
        <HotbarSlotPicker hotbarSlots={hotbarSlots} onPick={handlePick} onCancel={() => setPicking(false)} />
      )}
    </li>
  )
}

// Active tech item — Spontaneous known entry
function SpontaneousItem({
  entry,
  poolRemaining,
  hotbarSlots,
  sendMessage,
}: {
  entry: SpontaneousKnownEntry
  poolRemaining: number
  hotbarSlots: HotbarSlot[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)
  const techId = entry.techId ?? entry.tech_id ?? ''
  const name = entry.techName ?? entry.tech_name ?? techId
  const level = entry.techLevel ?? entry.tech_level ?? 0
  const exhausted = poolRemaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, kind: 'technology', ref: techId })
    setPicking(false)
  }

  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: exhausted ? '#666' : '#e0c060' }}>{name}</strong>
        <LevelBadge level={level} />
        <span style={styles.badgeActive}>active</span>
      </div>
      {entry.description && <p style={styles.techDesc}>{entry.description}</p>}
      {entry.effectsSummary && <EffectsSummary text={entry.effectsSummary} />}
      {!exhausted && !picking && (
        <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
          + Add to Hotbar
        </button>
      )}
      {picking && (
        <HotbarSlotPicker hotbarSlots={hotbarSlots} onPick={handlePick} onCancel={() => setPicking(false)} />
      )}
    </li>
  )
}

// Known-but-unassigned tech item — shown greyed out for prepared casters
function UnassignedKnownItem({ entry }: { entry: SpontaneousKnownEntry }) {
  const techId = entry.techId ?? entry.tech_id ?? ''
  const name = entry.techName ?? entry.tech_name ?? techId
  const level = entry.techLevel ?? entry.tech_level ?? 0
  return (
    <li style={{ ...styles.techItem, opacity: 0.45 }}>
      <div style={styles.techHeader}>
        <strong style={{ color: '#888' }}>{name}</strong>
        <LevelBadge level={level} />
        <span style={styles.badgeUnassigned}>unassigned</span>
      </div>
      {entry.description && <p style={styles.techDesc}>{entry.description}</p>}
    </li>
  )
}

// Passive tech item — Innate with passive: true
function PassiveInnateItem({ slot }: { slot: InnateSlotView }) {
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const level = slot.techLevel ?? slot.tech_level ?? 0
  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: '#aaa' }}>{name}</strong>
        <LevelBadge level={level} />
        <span style={styles.badgePassive}>passive</span>
      </div>
      {slot.description && <p style={styles.techDesc}>{slot.description}</p>}
      {slot.effectsSummary && <EffectsSummary text={slot.effectsSummary} />}
    </li>
  )
}

// Passive tech item — Hardwired
function HardwiredItem({ slot }: { slot: HardwiredSlotView }) {
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const level = slot.techLevel ?? slot.tech_level ?? 0
  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: '#aaa' }}>{name}</strong>
        <LevelBadge level={level} />
        <span style={styles.badgePassive}>passive</span>
      </div>
      {slot.description && <p style={styles.techDesc}>{slot.description}</p>}
      {slot.effectsSummary && <EffectsSummary text={slot.effectsSummary} />}
    </li>
  )
}

export function TechnologyDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet

  if (!sheet) {
    return (
      <>
        <div className="drawer-header">
          <h3>Technology</h3>
          <button className="drawer-close" onClick={onClose}>✕</button>
        </div>
        <div className="drawer-body">
          <p style={{ color: '#666' }}>Loading…</p>
        </div>
      </>
    )
  }

  const hardwired: HardwiredSlotView[] = sheet.hardwiredSlots ?? sheet.hardwired_slots ?? []
  const prepared: PreparedSlotView[] = sheet.preparedSlots ?? sheet.prepared_slots ?? []
  const innate: InnateSlotView[] = sheet.innateSlots ?? sheet.innate_slots ?? []
  const spontKnown: SpontaneousKnownEntry[] = sheet.spontaneousKnown ?? sheet.spontaneous_known ?? []
  const spontPools: SpontaneousUsePoolView[] = sheet.spontaneousUsePools ?? sheet.spontaneous_use_pools ?? []
  const focusPts = sheet.focusPoints ?? sheet.focus_points ?? 0
  const maxFocus = sheet.maxFocusPoints ?? sheet.max_focus_points ?? 0

  // Build a map from level → remaining uses for spontaneous pools
  const poolRemainingByLevel = new Map<number, number>()
  for (const pool of spontPools) {
    const lvl = pool.techLevel ?? pool.tech_level ?? 0
    poolRemainingByLevel.set(lvl, pool.usesRemaining ?? pool.uses_remaining ?? 0)
  }

  const sortedPools = [...spontPools].sort(
    (a, b) => (a.techLevel ?? a.tech_level ?? 0) - (b.techLevel ?? b.tech_level ?? 0)
  )
  const knownByLevel = new Map<number, SpontaneousKnownEntry[]>()
  for (const entry of spontKnown) {
    const lvl = entry.techLevel ?? entry.tech_level ?? 0
    if (!knownByLevel.has(lvl)) knownByLevel.set(lvl, [])
    knownByLevel.get(lvl)!.push(entry)
  }

  // Group prepared slots by (techId, techLevel) so that the same tech at different
  // levels appears as separate entries, each with its own remaining-use counter.
  const preparedGroupMap = new Map<string, { slot: PreparedSlotView; total: number; remaining: number }>()
  for (const p of prepared) {
    const id = p.techId ?? p.tech_id ?? ''
    const lvl = p.techLevel ?? p.tech_level ?? 0
    const groupKey = `${id}:${lvl}`
    const existing = preparedGroupMap.get(groupKey)
    if (existing) {
      existing.total++
      if (!p.expended) existing.remaining++
    } else {
      preparedGroupMap.set(groupKey, { slot: p, total: 1, remaining: p.expended ? 0 : 1 })
    }
  }
  const preparedGroups = [...preparedGroupMap.values()]

  // Unassigned known techs: for prepared casters (have spontKnown but no spontPools),
  // show techs that are known but not assigned to any prepared slot.
  const assignedIds = new Set(prepared.map(p => p.techId ?? p.tech_id ?? ''))
  const unassignedKnown = spontPools.length === 0
    ? spontKnown.filter(e => !assignedIds.has(e.techId ?? e.tech_id ?? ''))
    : []

  const reactionInnate = innate.filter((s) => s.isReaction)
  const passiveInnate = innate.filter((s) => !s.isReaction && s.passive)
  const normalInnate = innate.filter((s) => !s.isReaction && !s.passive)

  const hasReactions = reactionInnate.length > 0
  const hasActive = preparedGroups.length > 0 || normalInnate.length > 0 || spontKnown.length > 0 || unassignedKnown.length > 0
  const hasPassive = hardwired.length > 0 || passiveInnate.length > 0
  const hasAny = hasReactions || hasActive || hasPassive

  return (
    <>
      <div className="drawer-header">
        <h3>Technology</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!hasAny ? (
          <p style={{ color: '#666' }}>No technology.</p>
        ) : (
          <>
            {maxFocus > 0 && (
              <div style={styles.focusRow}>
                <span style={styles.focusLabel}>Focus Points</span>
                <UsePips remaining={focusPts} max={maxFocus} />
              </div>
            )}

            {hasReactions && (
              <section style={styles.section}>
                <SectionLabel label="Reactions" />
                <ul style={styles.list}>
                  {reactionInnate.map((inn, i) => (
                    <InnateItem
                      key={(inn.techId ?? inn.tech_id ?? '') + i}
                      slot={inn}
                      hotbarSlots={state.hotbarSlots}
                      sendMessage={sendMessage}
                    />
                  ))}
                </ul>
              </section>
            )}

            {hasActive && (
              <section style={styles.section}>
                <SectionLabel label="Active" />
                <ul style={styles.list}>
                  {preparedGroups.map((g) => (
                    <PreparedItem
                      key={`${g.slot.techId ?? g.slot.tech_id ?? g.slot.techName}:${g.slot.techLevel ?? g.slot.tech_level ?? 0}`}
                      slot={g.slot}
                      total={g.total}
                      remaining={g.remaining}
                      hotbarSlots={state.hotbarSlots}
                      sendMessage={sendMessage}
                    />
                  ))}
                  {normalInnate.map((inn, i) => (
                    <InnateItem
                      key={(inn.techId ?? inn.tech_id ?? '') + i}
                      slot={inn}
                      hotbarSlots={state.hotbarSlots}
                      sendMessage={sendMessage}
                    />
                  ))}
                  {unassignedKnown.map((entry) => (
                    <UnassignedKnownItem key={entry.techId ?? entry.tech_id ?? ''} entry={entry} />
                  ))}
                  {sortedPools.map((pool) => {
                    const lvl = pool.techLevel ?? pool.tech_level ?? 0
                    const remaining = pool.usesRemaining ?? pool.uses_remaining ?? 0
                    const max = pool.maxUses ?? pool.max_uses ?? 0
                    const known = knownByLevel.get(lvl) ?? []
                    return (
                      <li key={`spont-${lvl}`} style={styles.spontLevelItem}>
                        <div style={styles.spontHeader}>
                          <span style={styles.spontLevelLabel}>Level {lvl} Spontaneous</span>
                          {max > 0 && <UsePips remaining={remaining} max={max} />}
                        </div>
                        <ul style={styles.list}>
                          {known.map((k, i) => (
                            <SpontaneousItem
                              key={(k.techId ?? k.tech_id ?? '') + i}
                              entry={k}
                              poolRemaining={remaining}
                              hotbarSlots={state.hotbarSlots}
                              sendMessage={sendMessage}
                            />
                          ))}
                        </ul>
                      </li>
                    )
                  })}
                </ul>
              </section>
            )}

            {hasPassive && (
              <section style={styles.section}>
                <SectionLabel label="Passive" />
                <ul style={styles.list}>
                  {passiveInnate.map((s, i) => (
                    <PassiveInnateItem key={s.techId ?? s.tech_id ?? i} slot={s} />
                  ))}
                  {hardwired.map((h, i) => (
                    <HardwiredItem key={h.techId ?? h.tech_id ?? i} slot={h} />
                  ))}
                </ul>
              </section>
            )}
          </>
        )}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  section: { marginBottom: '1rem' },
  sectionLabel: {
    color: '#7af',
    fontSize: '0.72rem',
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    marginBottom: '0.4rem',
    borderBottom: '1px solid #2a2a2a',
    paddingBottom: '0.2rem',
  },
  list: { listStyle: 'none', padding: 0, margin: 0 },
  techItem: { marginBottom: '0.75rem' },
  techHeader: { display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.15rem' },
  techDesc: { margin: '0.15rem 0 0.2rem', color: '#888', fontSize: '0.8rem' },
  effectsSummary: { margin: '0.1rem 0 0.3rem', color: '#6a9', fontSize: '0.75rem', fontFamily: 'monospace', lineHeight: 1.4 },
  effectsIndent: { paddingLeft: '1em' },
  expended: { color: '#666', textDecoration: 'line-through' as const },
  expendedBadge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.35rem',
    borderRadius: '3px',
    background: '#2a1a1a',
    border: '1px solid #5a2a2a',
    color: '#966',
  },
  badgeActive: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#2a3a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    whiteSpace: 'nowrap' as const,
  },
  badgeInnate: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a2a3a',
    border: '1px solid #2a5a8a',
    color: '#6af',
    whiteSpace: 'nowrap' as const,
  },
  infiniteUses: {
    fontSize: '1rem',
    color: '#7bc',
    lineHeight: 1,
  },
  badgeReaction: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a2a3a',
    border: '1px solid #2a5a8a',
    color: '#68d',
    whiteSpace: 'nowrap' as const,
  },
  badgePassive: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a1a2a',
    border: '1px solid #3a3a5a',
    color: '#778',
    whiteSpace: 'nowrap' as const,
  },
  badgeUnassigned: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a1a1a',
    border: '1px solid #3a3a3a',
    color: '#666',
    whiteSpace: 'nowrap' as const,
  },
  levelBadge: {
    fontSize: '0.62rem',
    padding: '0.1rem 0.3rem',
    borderRadius: '3px',
    background: '#1a1a1a',
    border: '1px solid #3a3a3a',
    color: '#999',
    whiteSpace: 'nowrap' as const,
  },
  hotbarBtn: {
    marginTop: '0.2rem',
    padding: '0.2rem 0.5rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  focusRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '0.75rem',
    padding: '0.4rem 0.5rem',
    background: '#111',
    border: '1px solid #2a2a2a',
    borderRadius: '4px',
  },
  focusLabel: { color: '#7bc', fontSize: '0.82rem' },
  pips: { display: 'flex', alignItems: 'center', gap: '3px' },
  pip: { width: '10px', height: '10px', borderRadius: '50%', display: 'inline-block' },
  pipFull: { background: '#7bc', border: '1px solid #5a9ab5' },
  pipEmpty: { background: '#1a1a2a', border: '1px solid #3a3a5a' },
  pipLabel: { color: '#666', fontSize: '0.7rem', marginLeft: '4px' },
  spontLevelItem: { marginBottom: '0.75rem' },
  spontHeader: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.3rem' },
  spontLevelLabel: { color: '#aaa', fontSize: '0.8rem' },
}
