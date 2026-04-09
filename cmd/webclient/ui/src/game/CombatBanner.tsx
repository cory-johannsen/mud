import { useGame } from './GameContext'

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

  return (
    <div className="combat-banner">
      <strong>⚔ COMBAT — Round {roundNum}</strong>
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
