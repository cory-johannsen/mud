import React, { useState, useMemo, useRef } from 'react'
import { useGame } from '../GameContext'

// FeatureChoiceModal renders an overlay modal presenting a feature choice prompt.
// It is displayed when state.choicePrompt is non-null.
//
// REQ-FCM-1: Display cp.prompt as the modal title.
// REQ-FCM-2: Each selectable option rendered as a numbered, clickable button.
// REQ-FCM-3: Clicking a selectable option sends the 1-based original option index as CommandText.
// REQ-FCM-4: After selection, clearChoicePrompt() is called.
// REQ-FCM-5: Internal ID prefixes of the form "[xxx] " MUST be stripped before display.
// REQ-FCM-6: Navigation sentinels ([back], [forward], [confirm]) rendered as navigation buttons, not option rows.
// REQ-FCM-7: slotContext (when present) renders a slot progress header above the prompt title.
// REQ-FCM-8: When options contain "(Lv N)" metadata, level filter tabs are shown; active level defaults to slotContext.slotLevel.
// REQ-FCM-9: [heightened:N] sentinel embedded in option text is stripped from display and shown as a "+N" badge.

const BACK_SENTINEL = '[back]'
const FORWARD_SENTINEL = '[forward]'
const CONFIRM_SENTINEL = '[confirm]'

// parseHeightenSentinel extracts [heightened:N] from option text.
function parseHeightenSentinel(opt: string): { text: string; delta: number } {
  const match = opt.match(/\[heightened:(\d+)\]/)
  if (!match) return { text: opt, delta: 0 }
  return { text: opt.replace(/\s*\[heightened:\d+\]/, ''), delta: parseInt(match[1], 10) }
}

// parseLevelFromOption extracts tech level from "(Lv N)" in option string.
function parseLevelFromOption(opt: string): number {
  const match = opt.match(/\(Lv (\d+)\)/)
  return match ? parseInt(match[1], 10) : 0
}

// stripTechIdPrefix removes leading [techId] prefix from display text.
function stripTechIdPrefix(opt: string): string {
  return opt.replace(/^\[[^\]]+\]\s*/, '')
}

// FeatureChoiceModal is the exported named component consumed by GamePage.
// The onClose prop is accepted for API compatibility but is no longer required —
// the modal dismisses itself via clearChoicePrompt().
export function FeatureChoiceModal({ onClose: _onClose }: { onClose?: () => void }) {
  const { state, sendCommand, clearChoicePrompt } = useGame()
  // REQ-FCM-10: A sent ref prevents double-submission when the user clicks faster than
  // React's async state update can unmount the modal after clearChoicePrompt().
  const sentRef = useRef(false)
  const cp = state.choicePrompt
  // Reset sentRef whenever the prompt changes so each new prompt accepts one submission.
  React.useEffect(() => { sentRef.current = false }, [cp?.featureId, cp?.prompt])
  if (!cp) return null

  const options = cp.options ?? []

  // Separate navigation sentinels from selectable options.
  const hasBack = options.includes(BACK_SENTINEL)
  const hasForward = options.includes(FORWARD_SENTINEL)
  const hasConfirm = options.includes(CONFIRM_SENTINEL)
  const realOptions = options.filter(
    o => o !== BACK_SENTINEL && o !== FORWARD_SENTINEL && o !== CONFIRM_SENTINEL
  )

  // Extract available tech levels for filter tabs.
  const availableLevels = useMemo(() => {
    const levels = new Set<number>()
    realOptions.forEach(o => {
      const lvl = parseLevelFromOption(o)
      if (lvl > 0) levels.add(lvl)
    })
    return Array.from(levels).sort((a, b) => a - b)
  }, [realOptions.join('|')])

  const slotLevel = cp.slotContext?.slotLevel ?? 0
  const defaultLevel = availableLevels.includes(slotLevel)
    ? slotLevel
    : (availableLevels[availableLevels.length - 1] ?? 0)

  const [activeLevel, setActiveLevel] = useState<number>(defaultLevel)

  // Reset active level when the prompt changes.
  React.useEffect(() => {
    setActiveLevel(
      availableLevels.includes(slotLevel)
        ? slotLevel
        : (availableLevels[availableLevels.length - 1] ?? 0)
    )
  }, [cp.featureId, cp.prompt])

  // Filter by active level tab; show all options when no level metadata is present.
  const filteredOptions = availableLevels.length > 0
    ? realOptions.filter(o => {
        const lvl = parseLevelFromOption(o)
        return lvl === 0 || lvl === activeLevel
      })
    : realOptions

  function handleSelect(filteredIdx: number) {
    if (sentRef.current) return
    sentRef.current = true
    const opt = filteredOptions[filteredIdx]
    const originalIdx = options.indexOf(opt)
    clearChoicePrompt()
    sendCommand(String(originalIdx + 1))
  }

  function handleNavigation(sentinel: string) {
    if (sentRef.current) return
    sentRef.current = true
    const idx = options.indexOf(sentinel)
    clearChoicePrompt()
    sendCommand(String(idx + 1))
  }

  return (
    <div style={{
      position: 'fixed', top: 0, left: 0, width: '100%', height: '100%',
      backgroundColor: 'rgba(0,0,0,0.85)', zIndex: 300,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      fontFamily: 'monospace',
    }}>
      {/* Outer modal shell: flex column, capped at 80vh, overflow hidden so inner sections control scroll */}
      <div style={{
        backgroundColor: '#111', border: '2px solid #4a6a2a',
        maxWidth: '600px', width: '90%', maxHeight: '80vh',
        display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>
        {/* Fixed header: slot progress + prompt title + level tabs — never scrolls */}
        <div style={{ padding: '16px 20px 0', flexShrink: 0 }}>
          {cp.slotContext && (
            <div style={{ color: '#888', fontSize: '0.85em', textAlign: 'right', marginBottom: '6px' }}>
              Slot {cp.slotContext.slotNum} of {cp.slotContext.totalSlots} — Level {cp.slotContext.slotLevel}
            </div>
          )}

          <div style={{ color: '#e0c060', fontSize: '1.1em', marginBottom: '10px' }}>
            {cp.prompt}
          </div>

          {availableLevels.length > 0 && (
            <div style={{ display: 'flex', gap: '6px', marginBottom: '10px' }}>
              {availableLevels.map(lvl => (
                <button
                  key={lvl}
                  onClick={() => setActiveLevel(lvl)}
                  style={{
                    padding: '4px 10px',
                    backgroundColor: lvl === activeLevel ? '#4a6a2a' : '#222',
                    color: lvl === activeLevel ? '#e0c060' : '#aaa',
                    border: '1px solid #4a6a2a',
                    cursor: 'pointer',
                    fontFamily: 'monospace',
                  }}
                >
                  L{lvl}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Scrollable option list — grows to fill available space */}
        <div style={{ overflowY: 'auto', padding: '0 20px', flex: '1 1 auto', display: 'flex', flexDirection: 'column', gap: '6px' }}>
          {filteredOptions.map((opt, i) => {
            const { text: withoutHeighten, delta } = parseHeightenSentinel(opt)
            const displayText = stripTechIdPrefix(withoutHeighten)
            return (
              <button
                key={i}
                type="button"
                onClick={() => handleSelect(i)}
                style={{
                  textAlign: 'left', padding: '8px 12px',
                  backgroundColor: '#1a1a1a', color: '#ccc',
                  border: '1px solid #4a6a2a', cursor: 'pointer',
                  fontFamily: 'monospace', display: 'flex', alignItems: 'center', gap: '8px',
                  flexShrink: 0,
                }}
              >
                <span style={{ color: '#4a6a2a', minWidth: '20px' }}>{i + 1}.</span>
                <span>{displayText}</span>
                {delta > 0 && (
                  <span style={{
                    color: '#e0c060', fontSize: '0.8em',
                    border: '1px solid #e0c060', padding: '1px 5px', borderRadius: '3px',
                  }}>
                    +{delta}
                  </span>
                )}
              </button>
            )
          })}
        </div>

        {/* Navigation row — always visible at bottom, never scrolls away */}
        {(hasBack || hasForward || hasConfirm) && (
          <div style={{
            display: 'flex', justifyContent: 'space-between', padding: '12px 20px',
            borderTop: '1px solid #2a3a1a', flexShrink: 0,
          }}>
            <div>
              {hasBack && (
                <button
                  type="button"
                  onClick={() => handleNavigation(BACK_SENTINEL)}
                  style={{
                    padding: '6px 16px', backgroundColor: '#222', color: '#ccc',
                    border: '1px solid #888', cursor: 'pointer', fontFamily: 'monospace',
                  }}
                >
                  ← Back
                </button>
              )}
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              {hasForward && !hasConfirm && (
                <button
                  type="button"
                  onClick={() => handleNavigation(FORWARD_SENTINEL)}
                  style={{
                    padding: '6px 16px', backgroundColor: '#222', color: '#aaa',
                    border: '1px solid #4a6a2a', cursor: 'pointer', fontFamily: 'monospace',
                  }}
                >
                  Next →
                </button>
              )}
              {hasConfirm && (
                <button
                  type="button"
                  onClick={() => handleNavigation(CONFIRM_SENTINEL)}
                  style={{
                    padding: '6px 16px', backgroundColor: '#4a6a2a', color: '#e0c060',
                    border: '1px solid #e0c060', cursor: 'pointer', fontFamily: 'monospace',
                  }}
                >
                  Confirm
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
