import { useEffect } from 'react'
import { useGame } from '../GameContext'

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
                return (
                  <tr key={i}>
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
