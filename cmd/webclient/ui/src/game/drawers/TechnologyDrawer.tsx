import { useEffect } from 'react'
import { useGame } from '../GameContext'
import type {
  HardwiredSlotView,
  InnateSlotView,
  PreparedSlotView,
  SpontaneousKnownEntry,
  SpontaneousUsePoolView,
} from '../../proto'

function SectionLabel({ label }: { label: string }) {
  return (
    <div style={styles.sectionLabel}>{label}</div>
  )
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

  const sortedPools = [...spontPools].sort(
    (a, b) => (a.techLevel ?? a.tech_level ?? 0) - (b.techLevel ?? b.tech_level ?? 0)
  )
  const knownByLevel = new Map<number, SpontaneousKnownEntry[]>()
  for (const entry of spontKnown) {
    const lvl = entry.techLevel ?? entry.tech_level ?? 0
    if (!knownByLevel.has(lvl)) knownByLevel.set(lvl, [])
    knownByLevel.get(lvl)!.push(entry)
  }

  const hasAny = hardwired.length > 0 || prepared.length > 0 || innate.length > 0 || spontKnown.length > 0

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

            {hardwired.length > 0 && (
              <section style={styles.section}>
                <SectionLabel label="Hardwired" />
                <ul style={styles.list}>
                  {hardwired.map((h, i) => (
                    <li key={h.techId ?? h.tech_id ?? i} style={styles.techItem}>
                      <div style={styles.techName}>{h.techName ?? h.tech_name ?? h.techId ?? h.tech_id}</div>
                      {h.description && <div style={styles.techDesc}>{h.description}</div>}
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {prepared.length > 0 && (
              <section style={styles.section}>
                <SectionLabel label="Prepared" />
                <ul style={styles.list}>
                  {prepared.map((p, i) => (
                    <li key={(p.techId ?? p.tech_id ?? '') + i} style={styles.techItem}>
                      <div style={styles.techRow}>
                        <span style={{ ...styles.techName, ...(p.expended ? styles.expended : {}) }}>
                          {p.techName ?? p.tech_name ?? p.techId ?? p.tech_id}
                        </span>
                        {p.expended && <span style={styles.expendedBadge}>expended</span>}
                      </div>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {innate.length > 0 && (
              <section style={styles.section}>
                <SectionLabel label="Innate" />
                <ul style={styles.list}>
                  {innate.map((inn, i) => {
                    const remaining = inn.usesRemaining ?? inn.uses_remaining ?? 0
                    const max = inn.maxUses ?? inn.max_uses ?? 0
                    return (
                      <li key={inn.techId ?? inn.tech_id ?? i} style={styles.techItem}>
                        <div style={styles.techRow}>
                          <span style={styles.techName}>{inn.techName ?? inn.tech_name ?? inn.techId ?? inn.tech_id}</span>
                          {max > 0 && <UsePips remaining={remaining} max={max} />}
                        </div>
                        {inn.description && <div style={styles.techDesc}>{inn.description}</div>}
                      </li>
                    )
                  })}
                </ul>
              </section>
            )}

            {spontKnown.length > 0 && (
              <section style={styles.section}>
                <SectionLabel label="Spontaneous" />
                {sortedPools.map((pool) => {
                  const lvl = pool.techLevel ?? pool.tech_level ?? 0
                  const remaining = pool.usesRemaining ?? pool.uses_remaining ?? 0
                  const max = pool.maxUses ?? pool.max_uses ?? 0
                  const known = knownByLevel.get(lvl) ?? []
                  return (
                    <div key={lvl} style={styles.spontLevel}>
                      <div style={styles.spontHeader}>
                        <span style={styles.spontLevelLabel}>Level {lvl}</span>
                        {max > 0 && <UsePips remaining={remaining} max={max} />}
                      </div>
                      <ul style={styles.list}>
                        {known.map((k, i) => (
                          <li key={k.techId ?? k.tech_id ?? i} style={styles.techItemInline}>
                            {k.techName ?? k.tech_name ?? k.techId ?? k.tech_id}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )
                })}
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
  techItem: { marginBottom: '0.5rem' },
  techItemInline: { fontSize: '0.8rem', color: '#ccc', padding: '0.1rem 0' },
  techRow: { display: 'flex', alignItems: 'center', gap: '0.5rem' },
  techName: { color: '#e0c060', fontSize: '0.85rem', fontWeight: 'bold' as const },
  techDesc: { color: '#888', fontSize: '0.78rem', marginTop: '0.15rem' },
  expended: { color: '#666', textDecoration: 'line-through' as const },
  expendedBadge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.35rem',
    borderRadius: '3px',
    background: '#2a1a1a',
    border: '1px solid #5a2a2a',
    color: '#966',
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
  spontLevel: { marginBottom: '0.6rem' },
  spontHeader: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.2rem' },
  spontLevelLabel: { color: '#aaa', fontSize: '0.8rem' },
}
