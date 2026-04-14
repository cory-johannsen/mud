import { useState, useEffect, useRef } from 'react'
import { useGame } from './GameContext'

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

export function CombatBanner() {
  const { state } = useGame()
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
            </span>
          )
        })}
      </div>
    </div>
  )
}
