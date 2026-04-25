import { useEffect, useMemo, useState } from 'react'
import { useGame } from './GameContext'

// REQ-RXN-MODAL-1: ReactionPromptModal renders when state.reactionPrompt is set
// and dispatches a ReactionResponse WS message on button click.
//
// Precondition: GameContext must expose state.reactionPrompt, sendMessage, and
// clearReactionPrompt.
// Postcondition: The modal auto-closes when the deadline passes or when any
// option/skip button is clicked. Clicking an option sends a ReactionResponse
// carrying the option id; clicking Skip sends an empty chosen string.
export function ReactionPromptModal(): JSX.Element | null {
  const { state, sendMessage, clearReactionPrompt } = useGame()
  const prompt = state.reactionPrompt
  const promptId = prompt?.promptId ?? prompt?.prompt_id ?? ''
  const deadline = prompt?.deadlineUnixMs ?? prompt?.deadline_unix_ms ?? 0
  const options = prompt?.options ?? []

  // totalMs captures the full window the moment the prompt arrives so the
  // progress bar stays visually stable even if React re-renders.
  const totalMs = useMemo(() => {
    if (!deadline) return 0
    const remaining = deadline - Date.now()
    return remaining > 0 ? remaining : 0
  }, [deadline])

  const [nowMs, setNowMs] = useState<number>(() => Date.now())

  useEffect(() => {
    if (!prompt) return
    // Re-baseline the clock whenever a fresh prompt arrives.
    setNowMs(Date.now())
    const handle = window.setInterval(() => setNowMs(Date.now()), 100)
    return () => window.clearInterval(handle)
  }, [prompt])

  useEffect(() => {
    if (!prompt || !deadline) return
    if (nowMs >= deadline) {
      clearReactionPrompt()
    }
  }, [nowMs, deadline, prompt, clearReactionPrompt])

  if (!prompt || !promptId) return null

  const remainingMs = Math.max(0, deadline - nowMs)
  const pctRemaining = totalMs > 0 ? Math.max(0, Math.min(100, (remainingMs / totalMs) * 100)) : 0

  function respond(chosen: string): void {
    sendMessage('ReactionResponse', { prompt_id: promptId, chosen })
    clearReactionPrompt()
  }

  return (
    <div
      role="dialog"
      aria-label="Reaction prompt"
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 300,
        background: 'rgba(0,0,0,0.65)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <div
        style={{
          background: '#161622',
          border: '1px solid #6a5a2a',
          borderRadius: 6,
          padding: '1rem 1.25rem',
          minWidth: 320,
          maxWidth: 480,
          fontFamily: 'monospace',
          color: '#e0c060',
        }}
      >
        <h3 style={{ margin: '0 0 0.5rem', fontSize: '0.95rem' }}>Reaction!</h3>
        <p style={{ margin: '0 0 0.75rem', fontSize: '0.8rem', color: '#bbb' }}>
          A trigger fires — spend your reaction?
        </p>

        {/* Countdown progress bar */}
        <div
          data-testid="reaction-countdown"
          aria-label="time remaining"
          style={{
            height: 4,
            background: '#22222a',
            borderRadius: 2,
            overflow: 'hidden',
            marginBottom: '0.75rem',
          }}
        >
          <div
            style={{
              height: '100%',
              width: `${pctRemaining}%`,
              background: pctRemaining > 33 ? '#e0c060' : '#d45',
              transition: 'width 0.1s linear',
            }}
          />
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
          {options.map((opt) => {
            const id = opt.id ?? ''
            const label = opt.label ?? id
            return (
              <button
                key={id}
                type="button"
                onClick={() => respond(id)}
                style={{
                  padding: '0.4rem 0.6rem',
                  background: '#1a2a1a',
                  border: '1px solid #4a6a2a',
                  color: '#8d4',
                  borderRadius: 3,
                  cursor: 'pointer',
                  fontFamily: 'monospace',
                  fontSize: '0.8rem',
                  textAlign: 'left',
                }}
              >
                {label}
              </button>
            )
          })}
          <button
            type="button"
            onClick={() => respond('')}
            style={{
              padding: '0.4rem 0.6rem',
              background: '#22222a',
              border: '1px solid #555',
              color: '#aaa',
              borderRadius: 3,
              cursor: 'pointer',
              fontFamily: 'monospace',
              fontSize: '0.8rem',
              marginTop: '0.25rem',
            }}
          >
            Skip
          </button>
        </div>
      </div>
    </div>
  )
}
