import { useEffect, useRef, useState } from 'react'
import { useGame } from '../GameContext'
import type { SkillEntry } from '../../proto'

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
                  ({pos.x},{pos.y})
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

const ABILITIES = ['Brutality', 'Grit', 'Quickness', 'Reasoning', 'Savvy', 'Flair']

function AbilityBoostModal({ onSelect, onCancel }: { onSelect: (a: string) => void; onCancel: () => void }) {
  return (
    <div style={modalStyles.overlay}>
      <div style={modalStyles.box}>
        <div style={modalStyles.title}>Apply Ability Boost</div>
        <div style={modalStyles.subtitle}>Choose an ability to increase by 2:</div>
        <div style={modalStyles.abilityGrid}>
          {ABILITIES.map(a => (
            <button key={a} style={modalStyles.abilityBtn} onClick={() => onSelect(a)} type="button">{a}</button>
          ))}
        </div>
        <button style={modalStyles.cancelBtn} onClick={onCancel} type="button">Cancel</button>
      </div>
    </div>
  )
}

function SkillIncreaseModal({ skills, onSelect, onCancel }: { skills: SkillEntry[]; onSelect: (id: string) => void; onCancel: () => void }) {
  const upgradeable = skills.filter(s => s.proficiency !== 'legendary')
  return (
    <div style={modalStyles.overlay}>
      <div style={modalStyles.box}>
        <div style={modalStyles.title}>Increase Skill Proficiency</div>
        <div style={modalStyles.subtitle}>Choose a skill to advance:</div>
        <div style={modalStyles.skillList}>
          {upgradeable.length === 0
            ? <div style={{ color: '#666', fontSize: '0.8rem' }}>No upgradeable skills.</div>
            : upgradeable.map(s => (
              <button key={s.skillId} style={modalStyles.skillBtn} onClick={() => onSelect(s.skillId ?? '')} type="button">
                <span style={{ fontWeight: 600 }}>{s.name}</span>
                <span style={{ color: '#666', fontSize: '0.7rem' }}>{s.proficiency ?? 'untrained'} ({s.ability})</span>
              </button>
            ))}
        </div>
        <button style={modalStyles.cancelBtn} onClick={onCancel} type="button">Cancel</button>
      </div>
    </div>
  )
}

type Modal = 'boost' | 'skill' | null

export function CharacterPanel() {
  const { state, sendMessage } = useGame()
  const { characterInfo, characterSheet, combatRound } = state
  const retryRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const [modal, setModal] = useState<Modal>(null)

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

  function handleBoostSelect(ability: string) {
    sendMessage('LevelUpRequest', { ability: ability.toLowerCase() })
    sendMessage('CharacterSheetRequest', {})
    setModal(null)
  }

  function handleSkillSelect(skillId: string) {
    sendMessage('TrainSkillRequest', { skillId })
    sendMessage('CharacterSheetRequest', {})
    setModal(null)
  }

  if (combatRound) {
    return <CombatInitiativeList />
  }

  if (!characterInfo && !characterSheet) {
    return <p style={{ color: '#555', fontStyle: 'italic' }}>Loading…</p>
  }

  const name = characterInfo?.name ?? characterSheet?.name ?? ''
  const className = characterSheet?.job ?? characterInfo?.className ?? characterInfo?.class_name ?? characterInfo?.class ?? ''
  const level = characterInfo?.level ?? characterSheet?.level ?? 0
  const currentHp = characterInfo?.currentHp ?? characterInfo?.current_hp ?? 0
  const maxHp = characterInfo?.maxHp ?? characterInfo?.max_hp ?? characterSheet?.maxHp ?? characterSheet?.max_hp ?? 0
  const heroPoints = characterSheet?.heroPoints ?? characterSheet?.hero_points ?? 0
  const hpPct = maxHp > 0 ? (currentHp / maxHp) * 100 : 0
  const hpClass = hpPct > 50 ? 'hp-green' : hpPct > 25 ? 'hp-yellow' : 'hp-red'
  const conditions = state.roomView?.activeConditions ?? state.roomView?.active_conditions ?? []
  const xp = characterSheet?.experience ?? 0
  const xpToNext = characterSheet?.xpToNext ?? characterSheet?.xp_to_next ?? 0
  const xpPct = xpToNext > 0 ? Math.min(100, (xp / xpToNext) * 100) : 100
  const pendingBoosts = characterSheet?.pendingBoosts ?? characterSheet?.pending_boosts ?? 0
  const pendingSkillInc = characterSheet?.pendingSkillIncreases ?? 0

  return (
    <>
      {modal === 'boost' && (
        <AbilityBoostModal onSelect={handleBoostSelect} onCancel={() => setModal(null)} />
      )}
      {modal === 'skill' && (
        <SkillIncreaseModal skills={characterSheet?.skills ?? []} onSelect={handleSkillSelect} onCancel={() => setModal(null)} />
      )}
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

        {(characterSheet?.totalAc ?? 0) > 0 && (() => {
          const totalAc = characterSheet!.totalAc ?? 0
          const armorBonus = characterSheet!.acBonus ?? 0
          const effectiveDex = totalAc - 10 - armorBonus
          const parts: string[] = ['Base: 10']
          if (effectiveDex !== 0) parts.push(`Defense: ${effectiveDex >= 0 ? '+' : ''}${effectiveDex}`)
          if (armorBonus !== 0) parts.push(`Armor: +${armorBonus}`)
          const acTooltip = parts.join('  |  ') + `  =  ${totalAc}`
          return (
            <span className="hp-text" style={{ marginLeft: '0.5rem' }} title={acTooltip}>
              AC {totalAc}
            </span>
          )
        })()}

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
          <>
            <span className="hero-points">✦ Hero: {heroPoints}</span>
            <div style={styles.progressBlock}>
              <div className="hp-bar-track">
                <div className="hp-bar-fill" style={{ width: `${xpPct}%`, background: '#47a' }} />
              </div>
              <span className="hp-text">
                {xp}{xpToNext > 0 ? ` / ${xpToNext} XP` : ' XP (max)'}
              </span>
              {/* REQ-70-3: Crypto balance displayed directly beneath XP. */}
              {characterSheet.currency && (
                <span className="hp-text">{characterSheet.currency}</span>
              )}
              {pendingBoosts > 0 && (
                <button style={styles.pendingBtn} onClick={() => setModal('boost')} type="button">
                  ★ {pendingBoosts} Pending Boost{pendingBoosts !== 1 ? 's' : ''} — Apply
                </button>
              )}
              {pendingSkillInc > 0 && (
                <button style={styles.pendingBtn} onClick={() => setModal('skill')} type="button">
                  ★ {pendingSkillInc} Pending Skill Increase{pendingSkillInc !== 1 ? 's' : ''} — Apply
                </button>
              )}
            </div>
          </>
        )}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  progressBlock: {
    marginTop: '0.4rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
  },
  pendingBtn: {
    background: '#3a3a00',
    border: '1px solid #cc0',
    color: '#cc0',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.72rem',
    padding: '0.15rem 0.4rem',
    textAlign: 'left',
  },
}

const modalStyles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.7)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 200,
  },
  box: {
    background: '#1a1a1a',
    border: '1px solid #444',
    borderRadius: '6px',
    padding: '1.2rem',
    minWidth: '240px',
    maxWidth: '320px',
  },
  title: {
    color: '#7af',
    fontSize: '0.9rem',
    fontWeight: 600,
    marginBottom: '0.3rem',
  },
  subtitle: {
    color: '#888',
    fontSize: '0.75rem',
    marginBottom: '0.75rem',
  },
  abilityGrid: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '0.4rem',
    marginBottom: '0.75rem',
  },
  abilityBtn: {
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.8rem',
    padding: '0.3rem 0.5rem',
  },
  skillList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.3rem',
    marginBottom: '0.75rem',
    maxHeight: '240px',
    overflowY: 'auto',
  },
  skillBtn: {
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.78rem',
    padding: '0.25rem 0.5rem',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: '0.5rem',
  },
  cancelBtn: {
    background: 'none',
    border: '1px solid #444',
    color: '#666',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
    padding: '0.2rem 0.6rem',
    width: '100%',
  },
}
