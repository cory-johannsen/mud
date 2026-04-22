import { useState } from 'react'

// REQ-RDY-PICKER-1: ReadyActionPicker renders a two-step picker — first the
// action choice, then the trigger choice — and on confirm emits a
// `ready <action> when <trigger>` command via onSubmit.
// REQ-RDY-PICKER-2: When the selected action is `attack`, the trigger step
// MUST include an optional target text field whose value is appended to the
// action portion of the emitted command.
// REQ-RDY-PICKER-3: A Back button on the trigger step MUST return the picker
// to the action step without emitting a command.
// REQ-RDY-PICKER-4: A Cancel (×) button MUST dismiss the picker via onCancel.

export type ReadyAction = 'attack' | 'stride toward' | 'stride away' | 'reload'
export type ReadyTrigger =
  | 'enemy enters room'
  | 'enemy moves adjacent'
  | 'ally damaged'

interface ActionOption {
  id: ReadyAction
  label: string
}

interface TriggerOption {
  id: ReadyTrigger
  label: string
}

const ACTION_OPTIONS: ActionOption[] = [
  { id: 'attack', label: 'Attack' },
  { id: 'stride toward', label: 'Stride Toward' },
  { id: 'stride away', label: 'Stride Away' },
  { id: 'reload', label: 'Reload' },
]

const TRIGGER_OPTIONS: TriggerOption[] = [
  { id: 'enemy enters room', label: 'Enemy enters room' },
  { id: 'enemy moves adjacent', label: 'Enemy moves adjacent' },
  { id: 'ally damaged', label: 'Ally damaged' },
]

export interface ReadyActionPickerProps {
  onSubmit: (cmd: string) => void
  onCancel: () => void
}

export function ReadyActionPicker({
  onSubmit,
  onCancel,
}: ReadyActionPickerProps): JSX.Element {
  const [step, setStep] = useState<'action' | 'trigger'>('action')
  const [action, setAction] = useState<ReadyAction | null>(null)
  const [target, setTarget] = useState<string>('')

  function pickAction(a: ReadyAction): void {
    setAction(a)
    setStep('trigger')
  }

  function pickTrigger(t: ReadyTrigger): void {
    if (!action) return
    const trimmedTarget = target.trim()
    const actionPart =
      action === 'attack' && trimmedTarget.length > 0
        ? `attack ${trimmedTarget}`
        : action
    onSubmit(`ready ${actionPart} when ${t}`)
  }

  function back(): void {
    setStep('action')
    setTarget('')
  }

  return (
    <div
      role="dialog"
      aria-label="Ready action picker"
      style={styles.overlay}
    >
      <div style={styles.panel}>
        <div style={styles.header}>
          <h3 style={styles.title}>
            {step === 'action' ? 'Ready Action' : 'Choose Trigger'}
          </h3>
          <button
            type="button"
            aria-label="Cancel"
            onClick={onCancel}
            style={styles.closeBtn}
          >
            ×
          </button>
        </div>

        {step === 'action' && (
          <>
            <p style={styles.hint}>
              Pick an action to prepare. It will fire when the trigger
              you choose next occurs.
            </p>
            <div style={styles.buttonColumn}>
              {ACTION_OPTIONS.map((opt) => (
                <button
                  key={opt.id}
                  type="button"
                  onClick={() => pickAction(opt.id)}
                  style={styles.choiceBtn}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </>
        )}

        {step === 'trigger' && action && (
          <>
            <p style={styles.hint}>
              Ready <strong style={{ color: '#e0c060' }}>{action}</strong>{' '}
              — trigger when?
            </p>
            {action === 'attack' && (
              <label style={styles.label}>
                <span style={styles.labelText}>Target (optional)</span>
                <input
                  type="text"
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  placeholder="e.g. goblin"
                  spellCheck={false}
                  style={styles.input}
                />
              </label>
            )}
            <div style={styles.buttonColumn}>
              {TRIGGER_OPTIONS.map((opt) => (
                <button
                  key={opt.id}
                  type="button"
                  onClick={() => pickTrigger(opt.id)}
                  style={styles.choiceBtn}
                >
                  {opt.label}
                </button>
              ))}
            </div>
            <div style={styles.footerRow}>
              <button
                type="button"
                onClick={back}
                style={styles.backBtn}
              >
                ← Back
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    zIndex: 300,
    background: 'rgba(0,0,0,0.65)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  panel: {
    background: '#161622',
    border: '1px solid #6a5a2a',
    borderRadius: 6,
    padding: '1rem 1.25rem',
    minWidth: 320,
    maxWidth: 480,
    fontFamily: 'monospace',
    color: '#e0c060',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '0.5rem',
  },
  title: { margin: 0, fontSize: '0.95rem' },
  closeBtn: {
    background: 'transparent',
    border: '1px solid #555',
    color: '#aaa',
    borderRadius: 3,
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.9rem',
    width: 24,
    height: 24,
    lineHeight: '1',
  },
  hint: {
    margin: '0 0 0.75rem',
    fontSize: '0.8rem',
    color: '#bbb',
  },
  buttonColumn: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.35rem',
  },
  choiceBtn: {
    padding: '0.4rem 0.6rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: 3,
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.8rem',
    textAlign: 'left',
  },
  label: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.2rem',
    marginBottom: '0.6rem',
  },
  labelText: { color: '#888', fontSize: '0.72rem' },
  input: {
    background: '#111',
    border: '1px solid #555',
    borderRadius: 3,
    color: '#e0c060',
    fontFamily: 'monospace',
    fontSize: '0.8rem',
    padding: '0.3rem 0.5rem',
    outline: 'none',
    width: '100%',
    boxSizing: 'border-box',
  },
  footerRow: {
    display: 'flex',
    justifyContent: 'flex-start',
    marginTop: '0.6rem',
  },
  backBtn: {
    padding: '0.3rem 0.6rem',
    background: '#22222a',
    border: '1px solid #555',
    color: '#aaa',
    borderRadius: 3,
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.78rem',
  },
}
