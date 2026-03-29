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
  const actionsPerTurn = round.actionsPerTurn ?? round.actions_per_turn ?? 0
  const roundNum = round.round ?? 0

  return (
    <div className="combat-banner">
      <strong>⚔ COMBAT — Round {roundNum}</strong>
      <span>Turn order: {turnOrder.join(' → ')}</span>
      <span>{actionsPerTurn} actions</span>
    </div>
  )
}
