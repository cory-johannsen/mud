// HotbarPanel renders the 10 hotbar slots (keys 1–9, 0) and executes the bound
// command when a slot is clicked. Right-clicking or clicking an occupied slot
// opens an inline edit popup to change or clear the slot's command.
import { useEffect, useRef, useState } from 'react'
import { useGame } from '../GameContext'

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

// Commands that require a target argument. When fired from the hotbar during
// combat and no argument is present, the first NPC in turn order is appended.
const COMBAT_TARGET_CMDS = new Set([
  'attack', 'att', 'kill',
  'strike', 'st',
  'burst', 'bf',
])

interface EditPopupProps {
  slotIndex: number        // 0-based
  currentText: string
  onSave: (slot: number, text: string) => void
  onClear: (slot: number) => void
  onCancel: () => void
}

function EditPopup({ slotIndex, currentText, onSave, onClear, onCancel }: EditPopupProps) {
  const [text, setText] = useState(currentText)
  const inputRef = useRef<HTMLInputElement>(null)
  const slotKey = KEYS[slotIndex]

  useEffect(() => {
    inputRef.current?.focus()
    inputRef.current?.select()
  }, [])

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      if (text.trim()) onSave(slotIndex + 1, text.trim())
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onCancel()
    }
  }

  return (
    <div style={styles.popupOverlay} onClick={onCancel}>
      <div style={styles.popup} onClick={(e) => e.stopPropagation()}>
        <div style={styles.popupTitle}>Slot {slotKey}</div>
        <input
          ref={inputRef}
          style={styles.popupInput}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="command…"
          spellCheck={false}
        />
        <div style={styles.popupButtons}>
          <button
            style={styles.saveBtn}
            onClick={() => { if (text.trim()) onSave(slotIndex + 1, text.trim()) }}
            disabled={!text.trim()}
            type="button"
          >
            Save
          </button>
          <button
            style={styles.clearBtn}
            onClick={() => onClear(slotIndex + 1)}
            type="button"
          >
            Clear
          </button>
          <button style={styles.cancelBtn} onClick={onCancel} type="button">
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}

export function HotbarPanel() {
  const { state, sendCommand, sendMessage } = useGame()
  const { hotbarSlots, combatRound, characterInfo } = state
  const [editingSlot, setEditingSlot] = useState<number | null>(null)

  function activate(idx: number) {
    let cmd = hotbarSlots[idx]
    if (!cmd) return

    // Auto-fill combat target when the command has no argument and we're in combat.
    if (combatRound) {
      const words = cmd.trim().split(/\s+/)
      const verb = words[0].toLowerCase()
      if (words.length === 1 && COMBAT_TARGET_CMDS.has(verb)) {
        const playerName = characterInfo?.name ?? ''
        const turnOrder = combatRound.turnOrder ?? combatRound.turn_order ?? []
        const target = turnOrder.find((n) => n !== playerName)
        if (target) {
          cmd = `${cmd} ${target}`
        }
      }
    }

    sendCommand(cmd)
  }

  function handleSlotClick(idx: number) {
    const label = hotbarSlots[idx] ?? ''
    if (label) {
      // Occupied: open edit popup
      setEditingSlot(idx)
    } else {
      // Empty: open edit popup to set a new command
      setEditingSlot(idx)
    }
  }

  function handleSave(slot: number, text: string) {
    sendMessage('HotbarRequest', { action: 'set', slot, text })
    setEditingSlot(null)
  }

  function handleClear(slot: number) {
    sendMessage('HotbarRequest', { action: 'clear', slot, text: '' })
    setEditingSlot(null)
  }

  return (
    <>
      {editingSlot !== null && (
        <EditPopup
          slotIndex={editingSlot}
          currentText={hotbarSlots[editingSlot] ?? ''}
          onSave={handleSave}
          onClear={handleClear}
          onCancel={() => setEditingSlot(null)}
        />
      )}
      <div className="hotbar">
        {KEYS.map((key, i) => {
          const label = hotbarSlots[i] ?? ''
          return (
            <button
              key={key}
              className={`hotbar-slot${label ? '' : ' hotbar-slot-empty'}`}
              onClick={() => activate(i)}
              onContextMenu={(e) => { e.preventDefault(); handleSlotClick(i) }}
              title={label ? `${label} (right-click to edit)` : `Slot ${key} — empty (right-click to set)`}
              type="button"
            >
              <span className="hotbar-key">{key}</span>
              <span className="hotbar-label">{label || '—'}</span>
            </button>
          )
        })}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  popupOverlay: {
    position: 'fixed',
    inset: 0,
    zIndex: 200,
    background: 'rgba(0,0,0,0.5)',
    display: 'flex',
    alignItems: 'flex-end',
    justifyContent: 'center',
    paddingBottom: '60px',
  },
  popup: {
    background: '#1a1a1a',
    border: '1px solid #444',
    borderRadius: '6px',
    padding: '0.75rem',
    minWidth: '260px',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    fontFamily: 'monospace',
  },
  popupTitle: {
    color: '#7af',
    fontSize: '0.75rem',
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
  },
  popupInput: {
    background: '#111',
    border: '1px solid #555',
    borderRadius: '3px',
    color: '#e0c060',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    padding: '0.3rem 0.5rem',
    outline: 'none',
    width: '100%',
    boxSizing: 'border-box' as const,
  },
  popupButtons: {
    display: 'flex',
    gap: '0.4rem',
  },
  saveBtn: {
    padding: '0.2rem 0.6rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.78rem',
  },
  clearBtn: {
    padding: '0.2rem 0.6rem',
    background: '#2a1a1a',
    border: '1px solid #5a2a2a',
    color: '#c66',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.78rem',
  },
  cancelBtn: {
    padding: '0.2rem 0.6rem',
    background: 'none',
    border: '1px solid #444',
    color: '#666',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.78rem',
    marginLeft: 'auto',
  },
}
