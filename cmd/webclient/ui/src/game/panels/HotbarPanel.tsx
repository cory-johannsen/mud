// HotbarPanel renders the 10 hotbar slots (keys 1–9, 0) and executes the bound
// command when a slot is clicked.
// REQ-HCA-7: web slot displays display_name (not raw ref).
// REQ-HCA-8: hover tooltip shows display_name + description for typed; raw ref for command.
import { useEffect, useRef, useState } from 'react'
import ReactDOM from 'react-dom'
import { useGame } from '../GameContext'
import type { HotbarSlot } from '../../proto'
import { DamageTypeIcon, parseDamageType } from '../DamageTypeIcon'

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

const COMBAT_TARGET_CMDS = new Set([
  'attack', 'att', 'kill',
  'strike', 'st',
  'burst', 'bf',
])

// Human-readable labels for built-in action commands
export const ACTION_NAMES: Record<string, string> = {
  stride: 'Stride',
  close: 'Close',
  move: 'Move',
  approach: 'Approach',
  step: 'Step',
  attack: 'Attack',
  att: 'Attack',
  kill: 'Attack',
  strike: 'Strike',
  st: 'Strike',
  burst: 'Burst',
  bf: 'Burst',
  auto: 'Auto',
  af: 'Auto',
  reload: 'Reload',
  rl: 'Reload',
  flee: 'Flee',
  run: 'Flee',
  pass: 'Pass',
  look: 'Look',
  l: 'Look',
  north: 'North',
  south: 'South',
  east: 'East',
  west: 'West',
  up: 'Up',
  down: 'Down',
  n: 'North',
  s: 'South',
  e: 'East',
  w: 'West',
  'explore lay_low': 'Lay Low',
  'explore hold_ground': 'Hold Ground',
  'explore active_sensors': 'Active Sensors',
  'explore case_it': 'Case It',
  'explore run_point': 'Run Point',
  'explore shadow': 'Shadow',
  'explore poke_around': 'Poke Around',
  'explore off': 'Explore Off',
  explore: 'Explore',
  exp: 'Explore',
}

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
  // Use || instead of ?? to fall through empty proto string defaults
  const typed = slot.displayName || slot.display_name
  if (typed) return typed
  const ref = slot.ref || ''
  if (!ref) return ''
  // For command slots, check full command then verb against known action names
  if (!slot.kind || slot.kind === 'command') {
    const lower = ref.toLowerCase()
    if (ACTION_NAMES[lower]) return ACTION_NAMES[lower]
    const verb = lower.split(/\s+/)[0]
    if (ACTION_NAMES[verb]) return ACTION_NAMES[verb]
  }
  return ref
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

interface HotbarTooltipProps {
  slot: HotbarSlot
  pos: { x: number; y: number }
}

function ApPips({ cost }: { cost: number }) {
  if (cost <= 0) return null
  const pips = Math.min(cost, 5)
  return (
    <span style={{ display: 'inline-flex', gap: '2px', alignItems: 'center' }}>
      {Array.from({ length: pips }, (_, i) => (
        <span key={i} style={{ display: 'inline-block', width: '7px', height: '7px', background: '#e0c060', borderRadius: '1px', transform: 'rotate(45deg)' }} />
      ))}
    </span>
  )
}

function HotbarTooltip({ slot, pos }: HotbarTooltipProps) {
  const name = slot.displayName ?? slot.display_name ?? slot.ref
  const desc = slot.description
  const maxUses = slot.maxUses ?? slot.max_uses ?? 0
  const usesRemaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
  const rechargeCondition = slot.rechargeCondition ?? slot.recharge_condition ?? ''
  const apCost = slot.apCost ?? slot.ap_cost ?? 0
  const damageSummary = slot.damageSummary ?? slot.damage_summary ?? ''
  const isCommand = !slot.kind || slot.kind === 'command'

  const tooltipStyle: React.CSSProperties = {
    position: 'fixed',
    left: Math.min(pos.x, window.innerWidth - 240),
    top: Math.max(4, pos.y - 10),
    zIndex: 2000,
    background: '#1a1a1a',
    border: '1px solid #444',
    borderRadius: '4px',
    padding: '8px',
    minWidth: '160px',
    maxWidth: '240px',
    pointerEvents: 'none',
    fontFamily: 'monospace',
    fontSize: '0.78rem',
    lineHeight: '1.5',
    color: '#ccc',
    boxShadow: '0 4px 12px rgba(0,0,0,0.6)',
    transform: 'translateY(-100%)',
  }

  return ReactDOM.createPortal(
    <div style={tooltipStyle}>
      <div style={{ color: '#fff', fontWeight: 'bold', marginBottom: '0.15rem' }}>{name}</div>
      {!isCommand && desc && (
        <div style={{ color: '#ccc', marginBottom: '0.15rem' }}>{desc}</div>
      )}
      {apCost > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', marginBottom: '0.1rem' }}>
          <span style={{ color: '#888', fontSize: '0.72rem' }}>AP:</span>
          <ApPips cost={apCost} />
          <span style={{ color: '#e0c060', fontSize: '0.72rem' }}>{apCost}</span>
        </div>
      )}
      {damageSummary && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.3rem', fontSize: '0.72rem', marginBottom: '0.1rem' }}>
          <DamageTypeIcon damageType={parseDamageType(damageSummary)} size="0.85em" />
          <span style={{ color: '#f87' }}>{damageSummary}</span>
        </div>
      )}
      {maxUses < 0 && (
        <div style={{ color: '#7bc', marginBottom: '0.1rem' }}>∞ unlimited uses</div>
      )}
      {maxUses > 0 && (
        <div style={{ color: '#e0c060', marginBottom: '0.1rem' }}>
          {usesRemaining} / {maxUses} uses remaining
        </div>
      )}
      {rechargeCondition && (
        <div style={{ color: '#888', marginBottom: '0.1rem' }}>{rechargeCondition}</div>
      )}
      <div style={{ color: '#666', fontSize: '0.7rem' }}>right-click to edit</div>
    </div>,
    document.body,
  )
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
  const { hotbarSlots, combatRound, characterInfo, choicePrompt, activeHotbarIndex, hotbarCount, maxHotbars } = state
  const [editingSlot, setEditingSlot] = useState<number | null>(null)
  const [tooltip, setTooltip] = useState<{ slot: HotbarSlot; pos: { x: number; y: number } } | null>(null)

  // GH #235 / GH #237: dedupe rapid `create` requests so cycling past the
  // current hotbar's end doesn't spam the server (which would eventually
  // reply "Hotbar limit reached"). The earlier #235 fix gated on a ref that
  // only cleared when HotbarUpdate bumped hotbarCount above 1, which wedged
  // the Create button whenever the expected bump didn't arrive (GH #237).
  // This is a time-based gate: after a create is sent the ref stays true
  // for a short window (300ms) and then auto-clears, guaranteeing the
  // button recovers regardless of server response.
  const pendingCreateRef = useRef(false)
  function requestCreateOnce() {
    if (pendingCreateRef.current) return
    if (hotbarCount >= maxHotbars) return
    pendingCreateRef.current = true
    sendMessage('HotbarRequest', { action: 'create' })
    setTimeout(() => {
      pendingCreateRef.current = false
    }, 300)
  }

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

  // REQ-HKB-1: Digit keys 1-9 and 0 MUST activate the corresponding hotbar slot regardless
  // of which UI element has focus, except when a non-prompt input (e.g. EditPopup) is active,
  // or when the FeatureChoiceModal is open (choicePrompt non-null).
  // REQ-HKB-2: Intercepted keypresses MUST NOT be forwarded to the prompt or the server.
  // REQ-HKB-3: Hotbar MUST NOT intercept digit keys when choicePrompt is set; those keypresses
  // belong to the modal selection flow, not the hotbar.
  const activateRef = useRef(activate)
  useEffect(() => { activateRef.current = activate })
  useEffect(() => {
    if (editingSlot !== null) return // EditPopup is open — let it receive key events
    if (choicePrompt !== null) return // FeatureChoiceModal is open — do not intercept
    function handleKeyDown(e: globalThis.KeyboardEvent) {
      const idx = KEYS.indexOf(e.key)
      if (idx === -1) return
      const active = document.activeElement
      const tag = active?.tagName ?? ''
      const isOtherInput = (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT')
        && !active?.classList.contains('input-field')
      if (isOtherInput) return // Don't intercept keys in edit popups / drawer inputs
      e.preventDefault()
      activateRef.current(idx)
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [editingSlot, choicePrompt])

  // REQ-HMB-1: Ctrl+Up MUST cycle to the previous hotbar (wrapping) when hotbarCount > 1.
  // REQ-HMB-2: Ctrl+Down MUST cycle to the next hotbar (wrapping) when hotbarCount > 1.
  // GH #229: when only one hotbar exists, Ctrl+Up/Down or the switch buttons
  // auto-create a new hotbar (up to maxHotbars) instead of silently no-op'ing,
  // so the control never appears broken.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!e.ctrlKey) return
      if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
      if (maxHotbars <= 1) return
      e.preventDefault()
      e.stopPropagation()
      if (hotbarCount <= 1) {
        requestCreateOnce()
        return
      }
      if (e.key === 'ArrowUp') {
        const target = activeHotbarIndex === 1 ? hotbarCount : activeHotbarIndex - 1
        sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
      } else {
        const target = activeHotbarIndex === hotbarCount ? 1 : activeHotbarIndex + 1
        sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
      }
    }
    window.addEventListener('keydown', handleKeyDown, { capture: true })
    return () => window.removeEventListener('keydown', handleKeyDown, { capture: true })
  }, [activeHotbarIndex, hotbarCount, maxHotbars, sendMessage])

  const switchUp = () => {
    if (hotbarCount <= 1) {
      requestCreateOnce()
      return
    }
    const target = activeHotbarIndex === 1 ? hotbarCount : activeHotbarIndex - 1
    sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
  }
  const switchDown = () => {
    if (hotbarCount <= 1) {
      requestCreateOnce()
      return
    }
    const target = activeHotbarIndex === hotbarCount ? 1 : activeHotbarIndex + 1
    sendMessage('HotbarRequest', { action: 'switch', hotbar_index: target })
  }
  const createHotbar = () => {
    requestCreateOnce()
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
      {tooltip && <HotbarTooltip slot={tooltip.slot} pos={tooltip.pos} />}
      <div className="hotbar">
        <button
          className="hotbar-switch-btn"
          onClick={switchUp}
          disabled={maxHotbars <= 1}
          title="Previous hotbar (Ctrl+Up)"
          type="button"
        >▲</button>
        <button
          className="hotbar-switch-btn"
          onClick={switchDown}
          disabled={maxHotbars <= 1}
          title="Next hotbar (Ctrl+Down)"
          type="button"
        >▼</button>
        <span className="hotbar-indicator">{activeHotbarIndex}/{hotbarCount}</span>
        <div className="hotbar-slots">
        {KEYS.map((key, i) => {
          const slot = hotbarSlots[i] ?? { kind: 'command', ref: '' }
          const label = slotDisplayLabel(slot)
          const isEmpty = !slot.ref
          const maxUses = slot.maxUses ?? slot.max_uses ?? 0
          const usesRemaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
          const isInfinite = maxUses < 0
          const isExpended = maxUses > 0 && usesRemaining === 0
          const apCost = slot.apCost ?? slot.ap_cost ?? 0
          const isTech = slot.kind === 'technology'
          const isFeat = slot.kind === 'feat'
          let cls = 'hotbar-slot'
          if (isEmpty) cls += ' hotbar-slot-empty'
          if (isExpended) cls += ' hotbar-slot-expended'
          return (
            <button
              key={key}
              className={cls}
              onClick={() => activate(i)}
              onContextMenu={(e) => { e.preventDefault(); setEditingSlot(i) }}
              onMouseEnter={(e) => {
                if (!slot.ref) return
                const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
                setTooltip({ slot, pos: { x: rect.left, y: rect.top } })
              }}
              onMouseLeave={() => setTooltip(null)}
              type="button"
            >
              <span className="hotbar-key">{key}</span>
              <span className="hotbar-label">{label || '—'}</span>
              {(isTech || isFeat) && apCost > 0 && (
                <span className="hotbar-ap-badge">
                  {Array.from({ length: Math.min(apCost, 5) }, (_, j) => (
                    <span key={j} className="hotbar-ap-pip" />
                  ))}
                </span>
              )}
              {isInfinite && (
                <span className="hotbar-use-badge hotbar-use-infinite">∞</span>
              )}
              {!isInfinite && maxUses > 0 && (
                <span className="hotbar-use-badge">{usesRemaining}/{maxUses}</span>
              )}
            </button>
          )
        })}
        </div>
        {hotbarCount < maxHotbars && (
          <button
            className="hotbar-new-btn"
            onClick={createHotbar}
            title="Create new hotbar"
            type="button"
          >+ New Hotbar</button>
        )}
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
