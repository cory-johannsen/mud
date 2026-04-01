import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type {
  HardwiredSlotView,
  InnateSlotView,
  PreparedSlotView,
  SpontaneousKnownEntry,
  SpontaneousUsePoolView,
} from '../../proto'

const SLOT_KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

function SectionLabel({ label }: { label: string }) {
  return <div style={styles.sectionLabel}>{label}</div>
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

function SlotPicker({
  hotbarSlots,
  onPick,
  onCancel,
}: {
  hotbarSlots: string[]
  onPick: (slot: number) => void
  onCancel: () => void
}) {
  return (
    <div style={styles.slotPicker}>
      <span style={styles.slotPickerLabel}>Pick a hotbar slot:</span>
      <div style={styles.slotPickerGrid}>
        {SLOT_KEYS.map((key, i) => {
          const current = hotbarSlots[i] ?? ''
          return (
            <button
              key={key}
              style={{ ...styles.slotBtn, ...(current ? styles.slotBtnOccupied : {}) }}
              onClick={() => onPick(i + 1)}
              title={current ? `Replace: ${current}` : `Slot ${key} (empty)`}
              type="button"
            >
              <span style={styles.slotBtnKey}>{key}</span>
              {current && <span style={styles.slotBtnCurrent}>{current}</span>}
            </button>
          )
        })}
      </div>
      <button style={styles.cancelBtn} onClick={onCancel} type="button">Cancel</button>
    </div>
  )
}

// Active tech item — Prepared
function PreparedItem({
  slot,
  hotbarSlots,
  sendMessage,
}: {
  slot: PreparedSlotView
  hotbarSlots: string[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${techId}` })
    setPicking(false)
  }

  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: slot.expended ? '#666' : '#e0c060', textDecoration: slot.expended ? 'line-through' : 'none' }}>
          {name}
        </strong>
        <span style={styles.badgeActive}>active</span>
        {slot.expended && <span style={styles.expendedBadge}>expended</span>}
      </div>
      {slot.description && <p style={styles.techDesc}>{slot.description}</p>}
      {!slot.expended && !picking && (
        <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
          + Add to Hotbar
        </button>
      )}
      {picking && (
        <SlotPicker hotbarSlots={hotbarSlots} onPick={handlePick} onCancel={() => setPicking(false)} />
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
  hotbarSlots: string[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  const remaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
  const max = slot.maxUses ?? slot.max_uses ?? 0
  const exhausted = max > 0 && remaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${techId}` })
    setPicking(false)
  }

  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: exhausted ? '#666' : '#e0c060' }}>{name}</strong>
        <span style={styles.badgeActive}>active</span>
        {max > 0 && <UsePips remaining={remaining} max={max} />}
      </div>
      {slot.description && <p style={styles.techDesc}>{slot.description}</p>}
      {!exhausted && !slot.isReaction && !picking && (
        <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
          + Add to Hotbar
        </button>
      )}
      {picking && (
        <SlotPicker hotbarSlots={hotbarSlots} onPick={handlePick} onCancel={() => setPicking(false)} />
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
  hotbarSlots: string[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)
  const techId = entry.techId ?? entry.tech_id ?? ''
  const name = entry.techName ?? entry.tech_name ?? techId
  const exhausted = poolRemaining === 0

  function handlePick(s: number) {
    sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${techId}` })
    setPicking(false)
  }

  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: exhausted ? '#666' : '#e0c060' }}>{name}</strong>
        <span style={styles.badgeActive}>active</span>
      </div>
      {entry.description && <p style={styles.techDesc}>{entry.description}</p>}
      {!exhausted && !picking && (
        <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
          + Add to Hotbar
        </button>
      )}
      {picking && (
        <SlotPicker hotbarSlots={hotbarSlots} onPick={handlePick} onCancel={() => setPicking(false)} />
      )}
    </li>
  )
}

// Passive tech item — Hardwired
function HardwiredItem({ slot }: { slot: HardwiredSlotView }) {
  const techId = slot.techId ?? slot.tech_id ?? ''
  const name = slot.techName ?? slot.tech_name ?? techId
  return (
    <li style={styles.techItem}>
      <div style={styles.techHeader}>
        <strong style={{ color: '#aaa' }}>{name}</strong>
        <span style={styles.badgePassive}>passive</span>
      </div>
      {slot.description && <p style={styles.techDesc}>{slot.description}</p>}
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

  const reactionInnate = innate.filter((s) => s.isReaction)
  const normalInnate = innate.filter((s) => !s.isReaction)

  const hasReactions = reactionInnate.length > 0
  const hasActive = prepared.length > 0 || normalInnate.length > 0 || spontKnown.length > 0
  const hasPassive = hardwired.length > 0
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
                  {prepared.map((p, i) => (
                    <PreparedItem
                      key={(p.techId ?? p.tech_id ?? '') + i}
                      slot={p}
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
  techItem: { marginBottom: '0.75rem', position: 'relative' as const },
  techHeader: { display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.15rem' },
  techDesc: { margin: '0.15rem 0 0.3rem', color: '#888', fontSize: '0.8rem' },
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
  badgePassive: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a1a2a',
    border: '1px solid #3a3a5a',
    color: '#778',
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
  slotPicker: {
    position: 'absolute' as const,
    top: 0,
    left: 0,
    right: 0,
    zIndex: 10,
    background: '#111',
    border: '1px solid #333',
    borderRadius: '4px',
    padding: '0.5rem',
  },
  slotPickerLabel: { color: '#888', fontSize: '0.75rem', display: 'block', marginBottom: '0.4rem' },
  slotPickerGrid: { display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '3px', marginBottom: '0.4rem' },
  slotBtn: {
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    padding: '0.2rem',
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    minHeight: '32px',
    gap: '1px',
  },
  slotBtnOccupied: { borderColor: '#555', background: '#222' },
  slotBtnKey: { color: '#666', fontSize: '0.6rem', lineHeight: 1 },
  slotBtnCurrent: {
    color: '#999',
    fontSize: '0.6rem',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
    maxWidth: '100%',
    lineHeight: 1.2,
  },
  cancelBtn: {
    padding: '0.15rem 0.5rem',
    background: 'none',
    border: '1px solid #444',
    color: '#666',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.72rem',
  },
}
