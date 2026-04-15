import { useGame } from '../GameContext'

// FeatureChoiceModal renders an overlay modal presenting a feature choice prompt.
// It is displayed when state.choicePrompt is non-null.
//
// REQ-FCM-1: Display cp.prompt as the modal header.
// REQ-FCM-2: Each option rendered as a numbered, clickable button.
// REQ-FCM-3: Clicking an option sends the 1-based option number as a CommandText message.
// REQ-FCM-4: After selection, clearChoicePrompt() is called and onClose() is invoked.
// REQ-FCM-5: Internal ID prefixes of the form "[xxx] " MUST be stripped before display.

// stripOptionPrefix removes a leading "[xxx] " tag (e.g. "[keep] " or "[tech_id] ") from an option
// string so that players never see internal identifiers.
function stripOptionPrefix(opt: string): string {
  if (opt.startsWith('[')) {
    const end = opt.indexOf('] ')
    if (end > 0) return opt.slice(end + 2)
  }
  return opt
}

export function FeatureChoiceModal({ onClose }: { onClose: () => void }) {
  const { state, sendCommand, clearChoicePrompt } = useGame()
  const cp = state.choicePrompt
  if (!cp || cp.options.length === 0) return null

  function handleSelect(zeroBasedIndex: number) {
    // Send 1-based option number: server expects "1", "2", etc. as the choice.
    sendCommand(String(zeroBasedIndex + 1))
    clearChoicePrompt()
    onClose()
  }

  return (
    <div style={styles.overlay} onClick={() => { /* intentionally non-dismissible */ }}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <h3 style={styles.title}>{cp.prompt}</h3>
        </div>
        <div style={styles.body}>
          {cp.options.map((opt, i) => (
            <button
              key={i}
              style={styles.optionBtn}
              onClick={() => handleSelect(i)}
              type="button"
            >
              <span style={styles.optionNumber}>{i + 1}.</span>
              <span style={styles.optionText}>{stripOptionPrefix(opt)}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.8)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 300,
  },
  modal: {
    background: '#111',
    border: '1px solid #4a6a2a',
    borderRadius: '6px',
    width: 'min(540px, 95vw)',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  header: {
    padding: '0.75rem 1rem',
    borderBottom: '1px solid #2a3a1a',
    flexShrink: 0,
  },
  title: {
    margin: 0,
    color: '#e0c060',
    fontSize: '0.95rem',
    fontFamily: 'monospace',
    fontWeight: 600,
  },
  body: {
    padding: '0.75rem 1rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    overflowY: 'auto',
  },
  optionBtn: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.6rem',
    padding: '0.5rem 0.75rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    borderRadius: '4px',
    color: '#ccc',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    cursor: 'pointer',
    textAlign: 'left' as const,
    width: '100%',
  },
  optionNumber: {
    color: '#8d4',
    fontWeight: 700,
    minWidth: '1.5rem',
    flexShrink: 0,
  },
  optionText: {
    color: '#ddd',
  },
}
