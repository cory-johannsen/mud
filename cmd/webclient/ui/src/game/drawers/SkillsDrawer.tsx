import { useEffect } from 'react'
import { useGame } from '../GameContext'

const PROF_COLORS: Record<string, string> = {
  legendary: '#c8f',
  master:    '#cc0',
  expert:    '#7bc',
  trained:   '#ccc',
}

function profColor(proficiency?: string): string | undefined {
  return proficiency ? PROF_COLORS[proficiency.toLowerCase()] : undefined
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
                const color = profColor(skill.proficiency)
                return (
                  <tr key={i} style={color ? { color } : undefined}>
                    <td>{skill.name ?? ''}</td>
                    <td>{skill.ability ?? ''}</td>
                    <td>{skill.proficiency ?? ''}</td>
                    <td>{skill.bonus ?? 0}</td>
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
