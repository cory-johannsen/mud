import React, { useState, useEffect, useRef } from 'react'
import { useGame } from './GameContext'
import { ReadyActionPicker } from './ReadyActionPicker'

// REQ-61-5: RoundTimerBar renders a countdown progress bar depleting over durationMs.
// REQ-61-6: Fill starts at 100% and decreases to 0% over the round duration.
// REQ-61-8: roundKey (the round number) is included in the effect dependency so the
//   timer resets even when durationMs is unchanged between rounds.
function RoundTimerBar({ durationMs, roundKey }: { durationMs: number; roundKey: number }) {
  const [elapsed, setElapsed] = useState(0)
  const startRef = useRef(Date.now())

  useEffect(() => {
    // Reset on new round (roundKey or durationMs changes).
    startRef.current = Date.now()
    setElapsed(0)

    const interval = setInterval(() => {
      const e = Date.now() - startRef.current
      setElapsed(Math.min(e, durationMs))
    }, 100)

    return () => clearInterval(interval)
  }, [durationMs, roundKey])

  const fraction = Math.max(0, Math.min(1, 1 - elapsed / durationMs))
  const pct = Math.round(fraction * 100)

  return (
    <div
      className="round-timer-track"
      role="progressbar"
      aria-label="Round timer"
      aria-valuenow={pct}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className="round-timer-fill"
        data-testid="round-timer-fill"
        style={{ width: `${pct}%` }}
      />
    </div>
  )
}

// ReactionBadge renders the per-combatant reactions-remaining indicator
// ("R: N" or "R: N/M" fraction when Max > 1). GH #244 Task 15.
//
// - reactionMax=0 suppresses the badge entirely (budget not yet initialised for this combatant).
// - reactionMax=1: shows "R: <remaining>" (e.g. "R: 1" or "R: 0").
// - reactionMax>1: shows "R: <remaining>/<max>" (e.g. "R: 1/2").
// - When remaining == 0, the badge is rendered with strike-through and muted colour.
export function ReactionBadge({ reactionMax, reactionSpent }: { reactionMax: number; reactionSpent: number }) {
  if (reactionMax <= 0) return null
  const remaining = Math.max(0, reactionMax - reactionSpent)
  const exhausted = remaining === 0
  const label = reactionMax > 1 ? `R: ${remaining}/${reactionMax}` : `R: ${remaining}`
  const title = `${reactionMax} reaction${reactionMax === 1 ? '' : 's'} per round`
  const style: React.CSSProperties = {
    marginLeft: '0.35rem',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
    color: exhausted ? '#666' : '#8cf',
    textDecoration: exhausted ? 'line-through' : 'none',
  }
  return (
    <span
      className={`combat-reaction-badge${exhausted ? ' combat-reaction-exhausted' : ''}`}
      style={style}
      title={title}
      data-testid="reaction-badge"
    >
      {label}
    </span>
  )
}

export function CombatBanner() {
  const { state, sendCommand } = useGame()
  const [readyPickerOpen, setReadyPickerOpen] = useState(false)
  const round = state.combatRound
  if (!round) return null

  const turnOrder: string[] = Array.isArray(round.turnOrder)
    ? round.turnOrder
    : Array.isArray(round.turn_order)
      ? round.turn_order
      : []
  const actionsPerTurn = round.actionsPerTurn ?? round.actions_per_turn ?? 3
  const roundNum = round.round ?? 0
  const durationMs = round.durationMs ?? 0

  return (
    <div className="combat-banner">
      <strong>⚔ COMBAT — Round {roundNum}</strong>
      {durationMs > 0 && <RoundTimerBar durationMs={durationMs} roundKey={roundNum} />}
      <div className="combat-turn-order">
        {turnOrder.map((name) => {
          const ap = state.combatantAP[name]
          const total = ap?.total ?? actionsPerTurn
          const remaining = ap?.remaining ?? total
          return (
            <span key={name} className="combat-combatant">
              <span className="combat-combatant-name">{name}</span>
              <span className="combat-combatant-pips">
                {Array.from({ length: total }, (_, i) => (
                  <span
                    key={i}
                    className={`combat-pip ${i < remaining ? 'combat-pip-full' : 'combat-pip-empty'}`}
                  />
                ))}
              </span>
              <ReactionBadge reactionMax={ap?.reactionMax ?? 0} reactionSpent={ap?.reactionSpent ?? 0} />
            </span>
          )
        })}
      </div>
      <button
        type="button"
        className="combat-ready-btn"
        onClick={() => setReadyPickerOpen(true)}
        style={{
          marginLeft: 'auto',
          padding: '0.25rem 0.6rem',
          background: '#1a1a2a',
          border: '1px solid #4a4a6a',
          color: '#8cf',
          borderRadius: 3,
          cursor: 'pointer',
          fontFamily: 'monospace',
          fontSize: '0.78rem',
        }}
      >
        Ready
      </button>
      {readyPickerOpen && (
        <ReadyActionPicker
          onSubmit={(cmd) => {
            sendCommand(cmd)
            setReadyPickerOpen(false)
          }}
          onCancel={() => setReadyPickerOpen(false)}
        />
      )}
    </div>
  )
}
