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

function StatRow({ label, value, color, title }: { label: string; value: string | number; color?: string; title?: string }) {
  return (
    <div className="stats-row" title={title}>
      <span className="stats-label">{label}</span>
      <span className="stats-value" style={color ? { color } : undefined}>{value}</span>
    </div>
  )
}

// ABILITY_TOOLTIPS provides hover descriptions for each ability score.
const ABILITY_TOOLTIPS: Record<string, string> = {
  Brutality: 'Brutality — Raw physical power. Used for melee attacks, forced entry, and resisting knockback. Powers Toughness saves vs physical hazards.',
  Grit:      'Grit — Endurance and resilience. Determines max HP and Toughness saves vs poison, disease, exhaustion, and death effects.',
  Quickness: 'Quickness — Speed and reflexes. Used for ranged attacks, Initiative, Hustle saves vs traps, and determines Speed penalty from heavy armor.',
  Reasoning: 'Reasoning — Intelligence and technical aptitude. Powers tech usage, knowledge checks, and Toughness saves vs mental effects and confusion.',
  Savvy:     'Savvy — Street smarts and situational awareness. Used for Awareness checks, detecting threats, and Cool saves vs deception and surprise.',
  Flair:     'Flair — Charm and social presence. Powers Negotiate, intimidation, and Cool saves vs fear and influence effects.',
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
                <div key={label} className="stats-ability-cell" title={ABILITY_TOOLTIPS[label]}>
                  <span className="stats-label">{label}</span>
                  <span className="stats-value">{abilMod(val)}</span>
                </div>
              ))}
            </div>

            <SectionHeader label="— Defense —" />
            {(() => {
              const totalAc = sheet.totalAc ?? 0
              const armorBonus = sheet.acBonus ?? 0
              const effectiveDex = totalAc - 10 - armorBonus
              const parts: string[] = [`Base: 10`]
              if (effectiveDex !== 0) parts.push(`Dex: ${signedInt(effectiveDex)}`)
              if (armorBonus !== 0) parts.push(`Armor: +${armorBonus}`)
              const acTooltip = parts.join('  |  ') + `  =  ${totalAc}`
              return (
                <StatRow
                  label="AC"
                  value={`${totalAc}${armorBonus ? `  (armor +${armorBonus})` : ''}${sheet.checkPenalty ? `  check ${sheet.checkPenalty}` : ''}${sheet.speedPenalty ? `  speed ${sheet.speedPenalty}` : ''}`}
                  title={acTooltip}
                />
              )
            })()}
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

            {(sheet.proficiencies ?? []).length > 0 && (() => {
              const profs = sheet.proficiencies!
              const armor   = profs.filter(p => p.kind === 'armor')
              const weapons = profs.filter(p => p.kind === 'weapon')
              const other   = profs.filter(p => p.kind !== 'armor' && p.kind !== 'weapon')
              return (
                <>
                  <SectionHeader label="— Proficiencies —" />
                  {armor.length > 0 && (
                    <div style={styles.profGroup}>
                      <span style={styles.profGroupLabel}>Armor</span>
                      {armor.map(p => (
                        <div key={p.category} className="stats-row">
                          <span className="stats-label">{p.name ?? p.category}</span>
                          <span className="stats-value" style={styles.profRank(p.rank ?? '')}>
                            {p.rank ?? '—'}{p.bonus !== undefined && p.bonus !== 0 ? ` (${p.bonus >= 0 ? '+' : ''}${p.bonus})` : ''}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                  {weapons.length > 0 && (
                    <div style={styles.profGroup}>
                      <span style={styles.profGroupLabel}>Weapons</span>
                      {weapons.map(p => (
                        <div key={p.category} className="stats-row">
                          <span className="stats-label">{p.name ?? p.category}</span>
                          <span className="stats-value" style={styles.profRank(p.rank ?? '')}>
                            {p.rank ?? '—'}{p.bonus !== undefined && p.bonus !== 0 ? ` (${p.bonus >= 0 ? '+' : ''}${p.bonus})` : ''}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                  {other.length > 0 && (
                    <div style={styles.profGroup}>
                      {other.map(p => (
                        <div key={p.category} className="stats-row">
                          <span className="stats-label">{p.name ?? p.category}</span>
                          <span className="stats-value" style={styles.profRank(p.rank ?? '')}>
                            {p.rank ?? '—'}{p.bonus !== undefined && p.bonus !== 0 ? ` (${p.bonus >= 0 ? '+' : ''}${p.bonus})` : ''}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              )
            })()}

          </>
        )}
      </div>
    </>
  )
}

const RANK_COLORS: Record<string, string> = {
  untrained: '#555',
  trained:   '#aaa',
  expert:    '#7af',
  master:    '#fa7',
  legendary: '#fa4',
}

const styles = {
  profGroup: {
    marginBottom: '0.4rem',
  } as React.CSSProperties,
  profGroupLabel: {
    display: 'block',
    color: '#666',
    fontSize: '0.68rem',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.06em',
    marginBottom: '0.15rem',
    marginTop: '0.3rem',
  } as React.CSSProperties,
  profRank: (rank: string): React.CSSProperties => ({
    color: RANK_COLORS[rank] ?? '#aaa',
    fontSize: '0.8rem',
  }),
}

