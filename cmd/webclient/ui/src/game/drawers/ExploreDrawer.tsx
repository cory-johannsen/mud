import { useState } from 'react'
import { useGame } from '../GameContext'
import type { HotbarSlot } from '../../proto'
import { HotbarSlotPicker } from '../HotbarSlotPicker'

interface ExploreMode {
  id: string
  label: string
  command: string
  description: string
  badge: string
}

// All exploration modes supported by the "explore" command.
const EXPLORE_MODES: ExploreMode[] = [
  {
    id: 'lay_low',
    label: 'Lay Low',
    command: 'explore lay_low',
    description: 'Stealth mode — triggers a secret Ghosting check when entering a room. Reduces your visibility to threats.',
    badge: 'Stealth',
  },
  {
    id: 'hold_ground',
    label: 'Hold Ground',
    command: 'explore hold_ground',
    description: 'Shield mode — automatically raises your shield at the start of combat. Provides immediate defensive positioning.',
    badge: 'Defense',
  },
  {
    id: 'active_sensors',
    label: 'Active Sensors',
    command: 'explore active_sensors',
    description: 'Scan mode — triggers a secret Tech Lore check on room entry. Reveals technological hazards and anomalies.',
    badge: 'Scan',
  },
  {
    id: 'case_it',
    label: 'Case It',
    command: 'explore case_it',
    description: 'Search mode — actively looks for traps, hidden exits, and concealed threats on room entry.',
    badge: 'Search',
  },
  {
    id: 'run_point',
    label: 'Run Point',
    command: 'explore run_point',
    description: 'Scout mode — grants +1 Initiative bonus to co-located allies. You take the lead so your team reacts faster.',
    badge: 'Scout',
  },
  {
    id: 'shadow',
    label: 'Shadow',
    command: 'explore shadow',
    description: 'Follow mode — borrows an ally\'s skill rank for checks. Requires a target ally name: "explore shadow <ally>".',
    badge: 'Follow',
  },
  {
    id: 'poke_around',
    label: 'Poke Around',
    command: 'explore poke_around',
    description: 'Lore mode — triggers a secret Recall Knowledge check on room entry. Uncovers hidden history and details.',
    badge: 'Lore',
  },
]

function ExploreModeItem({
  mode,
  hotbarSlots,
  sendMessage,
}: {
  mode: ExploreMode
  hotbarSlots: HotbarSlot[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)

  function handlePick(slot: number) {
    sendMessage('HotbarRequest', {
      action: 'set',
      slot,
      kind: 'command',
      ref: mode.command,
    })
    setPicking(false)
  }

  return (
    <li style={styles.modeItem}>
      <div style={styles.modeHeader}>
        <strong style={{ color: '#e0c060' }}>{mode.label}</strong>
        <span style={styles.badge}>{mode.badge}</span>
      </div>
      <p style={styles.modeDesc}>{mode.description}</p>
      <div style={styles.actionRow}>
        <button
          style={styles.useBtn}
          onClick={() => sendMessage('CommandText', { text: mode.command })}
          type="button"
        >
          ▶ Use
        </button>
        {!picking && (
          <button style={styles.hotbarBtn} onClick={() => setPicking(true)} type="button">
            + Add to Hotbar
          </button>
        )}
      </div>
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

export function ExploreDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  return (
    <>
      <div className="drawer-header">
        <h3>Exploration</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        <section style={styles.section}>
          <div style={styles.sectionLabel}>Current Mode</div>
          <div style={styles.currentModeRow}>
            <button
              style={styles.useBtn}
              onClick={() => sendMessage('CommandText', { text: 'explore' })}
              type="button"
            >
              Check Mode
            </button>
            <button
              style={styles.offBtn}
              onClick={() => sendMessage('CommandText', { text: 'explore off' })}
              type="button"
            >
              Clear Mode
            </button>
          </div>
        </section>
        <section style={styles.section}>
          <div style={styles.sectionLabel}>Modes</div>
          <ul style={styles.list}>
            {EXPLORE_MODES.map((mode) => (
              <ExploreModeItem
                key={mode.id}
                mode={mode}
                hotbarSlots={state.hotbarSlots}
                sendMessage={sendMessage}
              />
            ))}
          </ul>
        </section>
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
  modeItem: { marginBottom: '0.85rem' },
  modeHeader: { display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.15rem' },
  modeDesc: { margin: '0.15rem 0 0.3rem', color: '#888', fontSize: '0.8rem' },
  badge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a2a3a',
    border: '1px solid #2a5a7a',
    color: '#7af',
    whiteSpace: 'nowrap' as const,
  },
  actionRow: { display: 'flex', gap: '0.4rem', flexWrap: 'wrap' as const },
  currentModeRow: { display: 'flex', gap: '0.4rem', marginBottom: '0.5rem' },
  useBtn: {
    padding: '0.2rem 0.5rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  offBtn: {
    padding: '0.2rem 0.5rem',
    background: '#2a1a1a',
    border: '1px solid #6a2a2a',
    color: '#c66',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  hotbarBtn: {
    padding: '0.2rem 0.5rem',
    background: '#1a1a2a',
    border: '1px solid #3a3a6a',
    color: '#88c',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
}
