// CombatNpcModal renders examine info for a combat NPC and provides combat action buttons.
// Opened when the player clicks an enemy tile on the battle map during active combat.
import { useGame } from './GameContext'

export function CombatNpcModal() {
  const { state, sendCommand, clearCombatNpcView } = useGame()
  const view = state.combatNpcView
  if (!view) return null

  const hp = state.combatantHp[view.name]
  const ap = state.combatantAP[view.name]

  function handleAttack() {
    sendCommand(`attack ${view!.name}`)
    clearCombatNpcView()
  }

  function handleStrike() {
    sendCommand(`strike ${view!.name}`)
    clearCombatNpcView()
  }

  return (
    <div style={styles.overlay} onClick={clearCombatNpcView}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{view.name}</h3>
            <span style={styles.combatBadge}>HOSTILE</span>
          </div>
          <button style={styles.closeBtn} onClick={clearCombatNpcView} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          <div style={styles.infoGrid}>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Level</span>
              <span style={styles.infoValue}>{view.level}</span>
            </div>
            {view.health && (
              <div style={styles.infoRow}>
                <span style={styles.infoLabel}>Condition</span>
                <span style={styles.infoValue}>{view.health}</span>
              </div>
            )}
            {hp && (
              <div style={styles.infoRow}>
                <span style={styles.infoLabel}>HP</span>
                <span style={styles.infoValue}>{hp.current} / {hp.max}</span>
              </div>
            )}
            {ap && (
              <div style={styles.infoRow}>
                <span style={styles.infoLabel}>AP</span>
                <span style={styles.infoValue}>{ap.remaining} / {ap.total}</span>
              </div>
            )}
          </div>
        </div>
        <div style={styles.footer}>
          <button style={{ ...styles.actionBtn, background: '#7a1a1a', borderColor: '#c03030' }} onClick={handleAttack} type="button">
            Attack
          </button>
          <button style={{ ...styles.actionBtn, background: '#3a1a6a', borderColor: '#6030a0' }} onClick={handleStrike} type="button">
            Strike
          </button>
          <button style={{ ...styles.actionBtn, background: '#333' }} onClick={clearCombatNpcView} type="button">
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.75)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 200,
  },
  modal: {
    background: '#111',
    border: '1px solid #5a1a1a',
    borderRadius: '6px',
    width: 'min(480px, 95vw)',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.6rem 1rem',
    borderBottom: '1px solid #2a1a1a',
    flexShrink: 0,
  },
  headerLeft: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.6rem',
  },
  title: {
    margin: 0,
    color: '#ff8888',
    fontSize: '1rem',
    fontFamily: 'monospace',
  },
  combatBadge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#2a1010',
    border: '1px solid #5a2020',
    color: '#c06060',
    fontFamily: 'monospace',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.05em',
  },
  closeBtn: {
    background: 'transparent',
    border: 'none',
    color: '#888',
    fontSize: '1rem',
    cursor: 'pointer',
    padding: '0.2rem',
    lineHeight: 1,
  },
  body: {
    padding: '0.75rem 1rem',
    overflowY: 'auto',
    flex: 1,
  },
  desc: {
    margin: '0 0 0.75rem',
    color: '#aaa',
    fontSize: '0.85rem',
    lineHeight: 1.5,
    fontFamily: 'monospace',
  },
  infoGrid: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.3rem',
  },
  infoRow: {
    display: 'flex',
    justifyContent: 'space-between',
    fontSize: '0.82rem',
    fontFamily: 'monospace',
    padding: '0.2rem 0',
    borderBottom: '1px solid #1e1e1e',
  },
  infoLabel: {
    color: '#778',
  },
  infoValue: {
    color: '#ddd',
    textAlign: 'right' as const,
  },
  footer: {
    display: 'flex',
    gap: '0.5rem',
    padding: '0.75rem 1rem',
    borderTop: '1px solid #2a1a1a',
    flexShrink: 0,
    flexWrap: 'wrap' as const,
  },
  actionBtn: {
    padding: '0.4rem 0.9rem',
    border: '1px solid #555',
    borderRadius: '4px',
    color: '#ddd',
    fontSize: '0.82rem',
    fontFamily: 'monospace',
    cursor: 'pointer',
    background: '#222',
  },
}
