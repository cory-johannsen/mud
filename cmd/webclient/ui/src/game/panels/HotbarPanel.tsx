// HotbarPanel renders the 10 hotbar slots (keys 1–9, 0) and executes the bound
// command when a slot is clicked.
// REQ-HCA-7: web slot displays display_name (not raw ref).
// REQ-HCA-8: hover tooltip shows display_name + description for typed; raw ref for command.
import { useEffect, useRef, useState } from 'react'
import { useGame } from '../GameContext'
import type { HotbarSlot } from '../../proto'

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

const COMBAT_TARGET_CMDS = new Set([
  'attack', 'att', 'kill',
  'strike', 'st',
  'burst', 'bf',
])

export function slotActivationCommand(slot: HotbarSlot): string {
  if (!slot.ref) return ''
  switch (slot.kind) {
    case 'feat':
    case 'technology':
    case 'consumable':
      return `use ${slot.ref}`
    case 'throwable':
      return `throw ${slot.ref}`
    default: // 'command' or ''
      return slot.ref
  }
}

export function slotDisplayLabel(slot: HotbarSlot): string {
  return slot.displayName ?? slot.display_name ?? slot.ref ?? ''
}

export function slotTooltip(slot: HotbarSlot): string {
  if (!slot.ref) return 'empty'
  if (slot.kind === 'command' || !slot.kind) {
    return `${slot.ref} (right-click to edit)`
  }
  const name = slot.displayName ?? slot.display_name ?? slot.ref
  const desc = slot.description ? `\n${slot.description}` : ''
  return `${name}${desc}\n(right-click to edit)`
}

interface EditPopupProps {
  slotIndex: number
  slot: HotbarSlot
  onSave: (slot: number, text: string) => void
  onClear: (slot: number) => void
  onCancel: () => void
}

function EditPopup({ slotIndex, slot, onSave, onClear, onCancel }: EditPopupProps) {
  const isCommand = !slot.kind || slot.kind === 'command'
  const [text, setText] = useState(isCommand ? (slot.ref ?? '') : '')
  const inputRef = useRef<HTMLInputElement>(null)
  const slotKey = KEYS[slotIndex]

  useEffect(() => {
    if (isCommand) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [isCommand])

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && isCommand) {
      e.preventDefault()
      if (text.trim()) onSave(slotIndex + 1, text.trim())
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onCancel()
    }
  }

  const label = isCommand ? `Slot ${slotKey}` : `Slot ${slotKey}: ${slotDisplayLabel(slot)}`

  return (
    <div style={styles.popupOverlay} onClick={onCancel}>
      <div style={styles.popup} onClick={(e) => e.stopPropagation()}>
        <div style={styles.popupTitle}>{label}</div>
        {isCommand && (
          <input
            ref={inputRef}
            style={styles.popupInput}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="command…"
            spellCheck={false}
          />
        )}
        {!isCommand && (
          <div style={{ color: '#888', fontSize: '0.78rem', fontFamily: 'monospace' }}>
            {slot.description ?? 'Typed slot — use the Feats/Tech/Inventory drawer to reassign.'}
          </div>
        )}
        <div style={styles.popupButtons}>
          {isCommand && (
            <button
              style={styles.saveBtn}
              onClick={() => { if (text.trim()) onSave(slotIndex + 1, text.trim()) }}
              disabled={!text.trim()}
              type="button"
            >
              Save
            </button>
          )}
          <button style={styles.clearBtn} onClick={() => onClear(slotIndex + 1)} type="button">
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
    const slot = hotbarSlots[idx]
    if (!slot?.ref) return

    let cmd = slotActivationCommand(slot)
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

  function handleSave(slot: number, text: string) {
    sendMessage('HotbarRequest', { action: 'set', slot, text })
    setEditingSlot(null)
  }

  function handleClear(slot: number) {
    sendMessage('HotbarRequest', { action: 'clear', slot })
    setEditingSlot(null)
  }

  return (
    <>
      {editingSlot !== null && (
        <EditPopup
          slotIndex={editingSlot}
          slot={hotbarSlots[editingSlot] ?? { kind: 'command', ref: '' }}
          onSave={handleSave}
          onClear={handleClear}
          onCancel={() => setEditingSlot(null)}
        />
      )}
      <div className="hotbar">
        {KEYS.map((key, i) => {
          const slot = hotbarSlots[i] ?? { kind: 'command', ref: '' }
          const label = slotDisplayLabel(slot)
          const isEmpty = !slot.ref
          return (
            <button
              key={key}
              className={`hotbar-slot${isEmpty ? ' hotbar-slot-empty' : ''}`}
              onClick={() => activate(i)}
              onContextMenu={(e) => { e.preventDefault(); setEditingSlot(i) }}
              title={slotTooltip(slot)}
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
  popupTitle: { color: '#7af', fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.08em' },
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
  popupButtons: { display: 'flex', gap: '0.4rem' },
  saveBtn: { padding: '0.2rem 0.6rem', background: '#1a2a1a', border: '1px solid #4a6a2a', color: '#8d4', borderRadius: '3px', cursor: 'pointer', fontFamily: 'monospace', fontSize: '0.78rem' },
  clearBtn: { padding: '0.2rem 0.6rem', background: '#2a1a1a', border: '1px solid #5a2a2a', color: '#c66', borderRadius: '3px', cursor: 'pointer', fontFamily: 'monospace', fontSize: '0.78rem' },
  cancelBtn: { padding: '0.2rem 0.6rem', background: 'none', border: '1px solid #444', color: '#666', borderRadius: '3px', cursor: 'pointer', fontFamily: 'monospace', fontSize: '0.78rem', marginLeft: 'auto' },
}
