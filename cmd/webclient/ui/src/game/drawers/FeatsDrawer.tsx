import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type { FeatEntry, HotbarSlot } from '../../proto'
import { HotbarSlotPicker } from '../HotbarSlotPicker'

function FeatItem({
  feat,
  hotbarSlots,
  sendMessage,
}: {
  feat: FeatEntry
  hotbarSlots: HotbarSlot[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)

  function handlePick(slot: number) {
    const ref = feat.featId ?? ''
    sendMessage('HotbarRequest', { action: 'set', slot, kind: 'feat', ref })
    setPicking(false)
  }

  return (
    <li style={styles.featItem}>
      <div style={styles.featHeader}>
        <strong style={{ color: feat.isReaction ? '#f0a050' : feat.active ? '#e0c060' : '#aaa' }}>
          {feat.name ?? ''}{feat.armorCategory ? ` (${feat.armorCategory})` : ''}
        </strong>
        <span style={feat.isReaction ? styles.badgeReaction : feat.active ? styles.badgeActive : styles.badgePassive}>
          {feat.isReaction ? 'reaction' : feat.active ? 'active' : 'passive'}
        </span>
        {feat.active && !feat.isReaction && !!((feat.actionCost ?? feat.action_cost) ?? 0) && (
          <span style={styles.apCostBadge}>
            {feat.actionCost ?? feat.action_cost} AP
          </span>
        )}
      </div>
      {feat.description && (
        <p style={styles.featDesc}>{feat.description}</p>
      )}
      {feat.active && !feat.isReaction && feat.activateText && !picking && (
        <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
          + Add to Hotbar
        </button>
      )}
      {picking && (
        <HotbarSlotPicker
          hotbarSlots={hotbarSlots}
          onPick={handlePick}
          onCancel={() => setPicking(false)}
        />
      )}
    </li>
  )
}

export function FeatsDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet
  const rawFeats = Array.isArray(sheet?.feats) ? (sheet.feats as FeatEntry[]) : []
  const reactions = rawFeats.filter((f) => f.isReaction)
  const active = rawFeats.filter((f) => f.active && !f.isReaction)
  const passive = rawFeats.filter((f) => !f.active && !f.isReaction)

  return (
    <>
      <div className="drawer-header">
        <h3>Feats</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!sheet ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : rawFeats.length === 0 ? (
          <p style={{ color: '#666' }}>No feats.</p>
        ) : (
          <>
            {reactions.length > 0 && (
              <section style={styles.section}>
                <div style={styles.sectionLabel}>Reactions</div>
                <ul style={styles.list}>
                  {reactions.map((f, i) => (
                    <FeatItem
                      key={f.featId ?? i}
                      feat={f}
                      hotbarSlots={state.hotbarSlots}
                      sendMessage={sendMessage}
                    />
                  ))}
                </ul>
              </section>
            )}
            {active.length > 0 && (
              <section style={styles.section}>
                <div style={styles.sectionLabel}>Active</div>
                <ul style={styles.list}>
                  {active.map((f, i) => (
                    <FeatItem
                      key={f.featId ?? (reactions.length + i)}
                      feat={f}
                      hotbarSlots={state.hotbarSlots}
                      sendMessage={sendMessage}
                    />
                  ))}
                </ul>
              </section>
            )}
            {passive.length > 0 && (
              <section style={styles.section}>
                <div style={styles.sectionLabel}>Passive</div>
                <ul style={styles.list}>
                  {passive.map((f, i) => (
                    <FeatItem
                      key={f.featId ?? (reactions.length + active.length + i)}
                      feat={f}
                      hotbarSlots={state.hotbarSlots}
                      sendMessage={sendMessage}
                    />
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
  featItem: { marginBottom: '0.75rem' },
  featHeader: { display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.15rem' },
  featDesc: { margin: '0.15rem 0 0.3rem', color: '#888', fontSize: '0.8rem' },
  badgeReaction: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#3a2a0a',
    border: '1px solid #8a5a1a',
    color: '#f0a050',
    whiteSpace: 'nowrap' as const,
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
  apCostBadge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a1a3a',
    border: '1px solid #3a3a7a',
    color: '#aaf',
    whiteSpace: 'nowrap' as const,
  },
}
