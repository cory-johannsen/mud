import { createPortal } from 'react-dom'
import type { HotbarSlot } from '../proto'

const SLOT_KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.75)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 300,
  },
  modal: {
    background: '#111',
    border: '1px solid #333',
    borderRadius: '6px',
    padding: '1rem',
    maxWidth: '95vw',
    width: 'max-content',
    fontFamily: 'monospace',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '0.75rem',
  },
  label: { color: '#ccc', fontSize: '0.85rem', fontFamily: 'monospace' },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '6px' },
  slotBtn: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    padding: '0.3rem 0.4rem',
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    minHeight: '40px',
    minWidth: '60px',
    gap: '2px',
  },
  slotBtnOccupied: { borderColor: '#555', background: '#222' },
  slotBtnKey: { color: '#666', fontSize: '0.6rem', lineHeight: 1 },
  slotBtnCurrent: {
    color: '#999',
    fontSize: '0.6rem',
    lineHeight: 1.2,
    textAlign: 'center',
    wordBreak: 'break-word',
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

export function HotbarSlotPicker({
  hotbarSlots,
  onPick,
  onCancel,
}: {
  hotbarSlots: HotbarSlot[]
  onPick: (slot: number) => void
  onCancel: () => void
}) {
  return createPortal(
    <div style={styles.overlay} onClick={onCancel}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <span style={styles.label}>Add to Hotbar</span>
          <button style={styles.cancelBtn} onClick={onCancel} type="button">✕</button>
        </div>
        <div style={styles.grid}>
          {SLOT_KEYS.map((key, i) => {
            const slot = hotbarSlots[i]
            const current = slot?.ref ?? ''
            const label = slot?.displayName ?? slot?.display_name ?? current
            return (
              <button
                key={key}
                style={{ ...styles.slotBtn, ...(current ? styles.slotBtnOccupied : {}) }}
                onClick={() => onPick(i + 1)}
                title={current ? `Replace: ${label}` : `Slot ${key} (empty)`}
                type="button"
              >
                <span style={styles.slotBtnKey}>{key}</span>
                {current && <span style={styles.slotBtnCurrent}>{label}</span>}
              </button>
            )
          })}
        </div>
      </div>
    </div>,
    document.body
  )
}
