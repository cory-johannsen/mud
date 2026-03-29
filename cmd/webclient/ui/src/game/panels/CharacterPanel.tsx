import { useGame } from '../GameContext'

export function CharacterPanel() {
  const { state } = useGame()
  const { characterInfo, characterSheet, combatRound } = state

  if (!characterInfo && !characterSheet) {
    return <p style={{ color: '#555', fontStyle: 'italic' }}>Loading…</p>
  }

  const name = characterInfo?.name ?? characterSheet?.name ?? ''
  const className = characterInfo?.className ?? characterInfo?.class_name ?? characterInfo?.class ?? characterSheet?.className ?? characterSheet?.class_name ?? characterSheet?.job ?? ''
  const level = characterInfo?.level ?? characterSheet?.level ?? 0
  const currentHp = characterInfo?.currentHp ?? characterInfo?.current_hp ?? 0
  const maxHp = characterInfo?.maxHp ?? characterInfo?.max_hp ?? 1
  const heroPoints = characterSheet?.heroPoints ?? characterSheet?.hero_points ?? 0
  const hpPct = maxHp > 0 ? (currentHp / maxHp) * 100 : 0
  const hpClass = hpPct > 50 ? 'hp-green' : hpPct > 25 ? 'hp-yellow' : 'hp-red'

  const conditions = state.roomView?.activeConditions ?? state.roomView?.active_conditions ?? []

  return (
    <div>
      <h3 className="char-name">{name}</h3>
      <span className="char-class">Level {level} {className}</span>

      <div className="hp-bar-track">
        <div
          className={`hp-bar-fill ${hpClass}`}
          style={{ width: `${Math.min(100, Math.max(0, hpPct))}%` }}
        />
      </div>
      <span className="hp-text">{currentHp} / {maxHp} HP</span>

      {conditions.length > 0 && (
        <div className="conditions">
          {conditions.map((c, i) => (
            <span key={i} className="condition-badge">
              {typeof c === 'string' ? c : (c as { name?: string }).name ?? String(c)}
            </span>
          ))}
        </div>
      )}

      {characterSheet && (
        <span className="hero-points">✦ Hero: {heroPoints}</span>
      )}

      {combatRound && (
        <div className="actions-info">
          Actions: {combatRound.actionsPerTurn ?? combatRound.actions_per_turn ?? 0}
        </div>
      )}
    </div>
  )
}
