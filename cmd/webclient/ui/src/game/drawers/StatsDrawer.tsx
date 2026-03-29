import { useEffect } from 'react'
import { useGame } from '../GameContext'

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

export function StatsDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet

  return (
    <>
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
            <StatRow label="Pending Boosts" value={sheet.pendingBoosts ?? sheet.pending_boosts ?? 0} color={(sheet.pendingBoosts ?? sheet.pending_boosts ?? 0) > 0 ? '#cc0' : undefined} />
            {(sheet.pendingSkillIncreases ?? sheet.pending_skill_increases ?? 0) > 0 && (
              <StatRow label="Pending Skill Increases" value={sheet.pendingSkillIncreases ?? sheet.pending_skill_increases ?? 0} color="#cc0" />
            )}
          </>
        )}
      </div>
    </>
  )
}
