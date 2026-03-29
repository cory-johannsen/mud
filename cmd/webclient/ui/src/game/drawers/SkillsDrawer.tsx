import { useEffect } from 'react'
import { useGame } from '../GameContext'

const PROF_COLORS: Record<string, string> = {
  legendary: '#c8f',
  master:    '#cc0',
  expert:    '#7bc',
  trained:   '#ccc',
  untrained: '#555',
}

const PROF_BONUS: Record<string, number> = {
  legendary: 8,
  master:    6,
  expert:    4,
  trained:   2,
  untrained: 0,
}

function abilMod(score: number): number {
  return Math.floor((score - 10) / 2)
}

function signedInt(n: number): string {
  return n >= 0 ? `+${n}` : `${n}`
}

export function SkillsDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet
  const skills = sheet?.skills ?? []

  const abilScores: Record<string, number> = {
    brutality: sheet?.brutality ?? 10,
    quickness: sheet?.quickness ?? 10,
    grit:      sheet?.grit      ?? 10,
    reasoning: sheet?.reasoning ?? 10,
    savvy:     sheet?.savvy     ?? 10,
    flair:     sheet?.flair     ?? 10,
  }

  return (
    <>
      <div className="drawer-header">
        <h3>Skills</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!sheet ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : (
          <table className="drawer-table">
            <thead>
              <tr>
                <th>Skill</th>
                <th>Ability</th>
                <th>Prof</th>
                <th>Bonus</th>
              </tr>
            </thead>
            <tbody>
              {(Array.isArray(skills) ? skills : []).map((s, i) => {
                const skill = s as { name?: string; ability?: string; proficiency?: string; bonus?: number }
                const prof = (skill.proficiency ?? '').toLowerCase()
                const color = PROF_COLORS[prof] ?? '#ccc'
                const profBonus = PROF_BONUS[prof] ?? 0
                const ability = (skill.ability ?? '').toLowerCase()
                const amod = abilMod(abilScores[ability] ?? 10)
                const bonus = amod + profBonus
                return (
                  <tr key={i}>
                    <td style={{ color }}>{skill.name ?? ''}</td>
                    <td style={{ color: '#888' }}>{skill.ability ?? ''}</td>
                    <td style={{ color }}>{skill.proficiency ?? ''}</td>
                    <td style={{ color }}>{signedInt(bonus)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </>
  )
}
