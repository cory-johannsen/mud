import { useEffect, useRef } from 'react'
import { useGame } from '../GameContext'

function HpBar({ current, max }: { current: number; max: number }) {
  const pct = max > 0 ? (current / max) * 100 : 0
  const cls = pct > 50 ? 'hp-green' : pct > 25 ? 'hp-yellow' : 'hp-red'
  return (
    <>
      <div className="hp-bar-track" style={{ marginBottom: 2 }}>
        <div className={`hp-bar-fill ${cls}`} style={{ width: `${Math.min(100, Math.max(0, pct))}%` }} />
      </div>
      <span className="hp-text">{current} / {max} HP</span>
    </>
  )
}

function CombatInitiativeList() {
  const { state } = useGame()
  const { combatRound, combatPositions, combatantHp, characterInfo, characterSheet } = state

  if (!combatRound) return null

  const turnOrder = combatRound.turnOrder ?? combatRound.turn_order ?? []
  const round = combatRound.round ?? 0
  const ap = combatRound.actionsPerTurn ?? combatRound.actions_per_turn ?? 0
  const playerName = characterInfo?.name ?? characterSheet?.name ?? ''
  const playerCurrentHp = characterInfo?.currentHp ?? characterInfo?.current_hp ?? 0
  const playerMaxHp = characterInfo?.maxHp ?? characterInfo?.max_hp ?? characterSheet?.maxHp ?? characterSheet?.max_hp ?? 0

  return (
    <div style={{ fontFamily: 'monospace' }}>
      <div style={{ color: '#e0c060', fontWeight: 'bold', marginBottom: 6, fontSize: '0.85rem' }}>
        ⚔ Round {round} — {ap} AP/turn
      </div>
      {turnOrder.map((name, idx) => {
        const isPlayer = name === playerName
        const hp = isPlayer
          ? { current: playerCurrentHp, max: playerMaxHp }
          : (combatantHp[name] ?? null)
        const pos = combatPositions[name] ?? null

        return (
          <div
            key={name}
            style={{
              borderLeft: isPlayer ? '3px solid #7af' : '3px solid #555',
              paddingLeft: 8,
              marginBottom: 8,
              opacity: 1,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 2 }}>
              <span style={{ color: '#888', fontSize: '0.72rem', minWidth: 18 }}>
                {idx + 1}.
              </span>
              <span style={{
                color: isPlayer ? '#7af' : '#ccc',
                fontWeight: isPlayer ? 'bold' : 'normal',
                fontSize: '0.82rem',
              }}>
                {name}{isPlayer ? ' (you)' : ''}
              </span>
              {pos !== null && (
                <span style={{ color: '#666', fontSize: '0.72rem', marginLeft: 'auto' }}>
                  {pos}ft
                </span>
              )}
            </div>
            {hp !== null && hp.max > 0 && (
              <HpBar current={hp.current} max={hp.max} />
            )}
            {hp === null && !isPlayer && (
              <span style={{ color: '#555', fontSize: '0.72rem' }}>HP unknown</span>
            )}
          </div>
        )
      })}
    </div>
  )
}

export function CharacterPanel() {
  const { state, sendMessage } = useGame()
  const { characterInfo, characterSheet, combatRound } = state
  const retryRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!characterSheet) {
      sendMessage('CharacterSheetRequest', {})
      retryRef.current = setInterval(() => {
        sendMessage('CharacterSheetRequest', {})
      }, 3000)
    } else {
      if (retryRef.current !== null) {
        clearInterval(retryRef.current)
        retryRef.current = null
      }
    }
    return () => {
      if (retryRef.current !== null) {
        clearInterval(retryRef.current)
        retryRef.current = null
      }
    }
  }, [characterSheet, sendMessage])

  if (combatRound) {
    return <CombatInitiativeList />
  }

  if (!characterInfo && !characterSheet) {
    return <p style={{ color: '#555', fontStyle: 'italic' }}>Loading…</p>
  }

  const name = characterInfo?.name ?? characterSheet?.name ?? ''
  const className = characterInfo?.className ?? characterInfo?.class_name ?? characterInfo?.class ?? characterSheet?.className ?? characterSheet?.class_name ?? characterSheet?.job ?? ''
  const level = characterInfo?.level ?? characterSheet?.level ?? 0
  const currentHp = characterInfo?.currentHp ?? characterInfo?.current_hp ?? 0
  const maxHp = characterInfo?.maxHp ?? characterInfo?.max_hp ?? characterSheet?.maxHp ?? characterSheet?.max_hp ?? 0
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
    </div>
  )
}
