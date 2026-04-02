import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type { SkillEntry } from '../../proto'

function signedInt(n: number): string {
  return n >= 0 ? `+${n}` : `${n}`
}

function abilMod(score: number): string {
  return signedInt(Math.floor((score - 10) / 2))
}

function SectionHeader({ label }: { label: string }) {
  return <div className="stats-section-header">{label}</div>
}

function StatRow({ label, value, color }: { label: string; value: string | number; color?: string }) {
  return (
    <div className="stats-row">
      <span className="stats-label">{label}</span>
      <span className="stats-value" style={color ? { color } : undefined}>{value}</span>
    </div>
  )
}

const ABILITIES = ['Brutality', 'Grit', 'Quickness', 'Reasoning', 'Savvy', 'Flair']

function AbilityBoostModal({
  onSelect,
  onCancel,
}: {
  onSelect: (ability: string) => void
  onCancel: () => void
}) {
  return (
    <div style={styles.modalOverlay}>
      <div style={styles.modal}>
        <div style={styles.modalTitle}>Apply Ability Boost</div>
        <div style={styles.modalSubtitle}>Choose an ability to increase by 2:</div>
        <div style={styles.abilityGrid}>
          {ABILITIES.map(a => (
            <button key={a} style={styles.abilityBtn} onClick={() => onSelect(a)} type="button">
              {a}
            </button>
          ))}
        </div>
        <button style={styles.cancelBtn} onClick={onCancel} type="button">Cancel</button>
      </div>
    </div>
  )
}

function SkillIncreaseModal({
  skills,
  onSelect,
  onCancel,
}: {
  skills: SkillEntry[]
  onSelect: (skillId: string) => void
  onCancel: () => void
}) {
  const upgradeable = skills.filter(s => {
    const rank = s.proficiency ?? ''
    return rank !== 'legendary'
  })

  return (
    <div style={styles.modalOverlay}>
      <div style={styles.modal}>
        <div style={styles.modalTitle}>Increase Skill Proficiency</div>
        <div style={styles.modalSubtitle}>Choose a skill to advance:</div>
        <div style={styles.skillList}>
          {upgradeable.length === 0 ? (
            <div style={{ color: '#666', fontSize: '0.8rem' }}>No upgradeable skills.</div>
          ) : (
            upgradeable.map(s => (
              <button
                key={s.skillId}
                style={styles.skillBtn}
                onClick={() => onSelect(s.skillId ?? '')}
                type="button"
              >
                <span style={styles.skillBtnName}>{s.name}</span>
                <span style={styles.skillBtnDetail}>{s.proficiency ?? 'untrained'} ({s.ability})</span>
              </button>
            ))
          )}
        </div>
        <button style={styles.cancelBtn} onClick={onCancel} type="button">Cancel</button>
      </div>
    </div>
  )
}

type Modal = 'boost' | 'skill' | null

export function StatsDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()
  const [modal, setModal] = useState<Modal>(null)

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet

  function handleBoostSelect(ability: string) {
    sendMessage('LevelUpRequest', { ability })
    sendMessage('CharacterSheetRequest', {})
    setModal(null)
  }

  function handleSkillSelect(skillId: string) {
    sendMessage('TrainSkillRequest', { skillId })
    sendMessage('CharacterSheetRequest', {})
    setModal(null)
  }

  const pendingBoosts = sheet?.pendingBoosts ?? sheet?.pending_boosts ?? 0
  const pendingSkillInc = sheet?.pendingSkillIncreases ?? 0

  return (
    <>
      {modal === 'boost' && (
        <AbilityBoostModal
          onSelect={handleBoostSelect}
          onCancel={() => setModal(null)}
        />
      )}
      {modal === 'skill' && (
        <SkillIncreaseModal
          skills={sheet?.skills ?? []}
          onSelect={handleSkillSelect}
          onCancel={() => setModal(null)}
        />
      )}
      <div className="drawer-header">
        <h3>Stats</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!sheet ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : (
          <>
            <SectionHeader label="— Abilities —" />
            <div className="stats-ability-grid">
              {[
                { label: 'Brutality', val: sheet.brutality ?? 0 },
                { label: 'Grit',      val: sheet.grit      ?? 0 },
                { label: 'Quickness', val: sheet.quickness ?? 0 },
                { label: 'Reasoning', val: sheet.reasoning ?? 0 },
                { label: 'Savvy',     val: sheet.savvy     ?? 0 },
                { label: 'Flair',     val: sheet.flair     ?? 0 },
              ].map(({ label, val }) => (
                <div key={label} className="stats-ability-cell">
                  <span className="stats-label">{label}</span>
                  <span className="stats-value">{abilMod(val)}</span>
                </div>
              ))}
            </div>

            <SectionHeader label="— Defense —" />
            <StatRow
              label="AC"
              value={`${sheet.totalAc ?? 0}${sheet.acBonus ? `  (armor +${sheet.acBonus})` : ''}${sheet.checkPenalty ? `  check ${sheet.checkPenalty}` : ''}${sheet.speedPenalty ? `  speed ${sheet.speedPenalty}` : ''}`}
            />
            {((sheet.playerResistances ?? sheet.player_resistances) ?? []).length > 0 && (
              <StatRow
                label="Resist"
                value={(sheet.playerResistances ?? sheet.player_resistances ?? [])
                  .map(r => `${r.damageType ?? r.damage_type ?? ''} ${r.value ?? 0}`)
                  .join('  ')}
                color="#4a8"
              />
            )}
            {((sheet.playerWeaknesses ?? sheet.player_weaknesses) ?? []).length > 0 && (
              <StatRow
                label="Weak"
                value={(sheet.playerWeaknesses ?? sheet.player_weaknesses ?? [])
                  .map(r => `${r.damageType ?? r.damage_type ?? ''} ${r.value ?? 0}`)
                  .join('  ')}
                color="#f66"
              />
            )}

            <SectionHeader label="— Saves —" />
            <div className="stats-save-row">
              <span>Toughness: <strong>{signedInt(sheet.toughnessSave ?? 0)}</strong></span>
              <span>Hustle: <strong>{signedInt(sheet.hustleSave ?? 0)}</strong></span>
              <span>Cool: <strong>{signedInt(sheet.coolSave ?? 0)}</strong></span>
            </div>
            <div className="stats-save-row">
              <span>Awareness: <strong>{signedInt(sheet.awareness ?? 0)}</strong></span>
              <span>Initiative: <strong>{abilMod(sheet.quickness ?? 10)}</strong></span>
            </div>

            <SectionHeader label="— Progress —" />
            {(sheet.xpToNext ?? sheet.xp_to_next ?? 0) === 0 ? (
              <StatRow label="XP" value={`${sheet.experience ?? 0} (max)`} />
            ) : (
              <StatRow label="XP" value={`${sheet.experience ?? 0} / ${sheet.xpToNext ?? sheet.xp_to_next ?? 0}`} />
            )}

            <div className="stats-row">
              <span className="stats-label">Pending Boosts</span>
              {pendingBoosts > 0 ? (
                <button
                  style={styles.pendingBtn}
                  onClick={() => setModal('boost')}
                  type="button"
                >
                  {pendingBoosts} — Apply
                </button>
              ) : (
                <span className="stats-value">{pendingBoosts}</span>
              )}
            </div>

            {pendingSkillInc > 0 && (
              <div className="stats-row">
                <span className="stats-label">Pending Skill Increases</span>
                <button
                  style={styles.pendingBtn}
                  onClick={() => setModal('skill')}
                  type="button"
                >
                  {pendingSkillInc} — Apply
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  pendingBtn: {
    background: '#3a3a00',
    border: '1px solid #cc0',
    color: '#cc0',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
    padding: '0.1rem 0.4rem',
  },
  modalOverlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.7)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 200,
  },
  modal: {
    background: '#1a1a1a',
    border: '1px solid #444',
    borderRadius: '6px',
    padding: '1.2rem',
    minWidth: '240px',
    maxWidth: '320px',
  },
  modalTitle: {
    color: '#7af',
    fontSize: '0.9rem',
    fontWeight: 600,
    marginBottom: '0.3rem',
  },
  modalSubtitle: {
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
  skillBtnName: {
    fontWeight: 600,
  },
  skillBtnDetail: {
    color: '#666',
    fontSize: '0.7rem',
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
