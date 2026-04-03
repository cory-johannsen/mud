import { useEffect } from 'react'
import { useGame } from '../GameContext'

export function JobDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  useEffect(() => {
    sendMessage('CharacterSheetRequest', {})
  }, [sendMessage])

  const sheet = state.characterSheet

  const xp = sheet?.experience ?? 0
  const xpToNext = sheet?.xpToNext ?? sheet?.xp_to_next ?? 0
  const pendingBoosts = sheet?.pendingBoosts ?? sheet?.pending_boosts ?? 0
  const pendingSkillIncreases = sheet?.pendingSkillIncreases ?? sheet?.pending_skill_increases ?? 0

  const xpPercent = xpToNext > 0 ? Math.min(100, Math.floor((xp / xpToNext) * 100)) : 0

  return (
    <>
      <div className="drawer-header">
        <h3>Job</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!sheet ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : (
          <>
            <section style={{ marginBottom: '1.25rem' }}>
              <h4 style={{ color: '#aaa', marginBottom: '0.5rem', textTransform: 'uppercase', fontSize: '0.75rem', letterSpacing: '0.08em' }}>Job</h4>
              <div style={{ fontSize: '1.1rem', color: '#eee', fontWeight: 'bold', marginBottom: '0.25rem' }}>
                {sheet.job ?? '—'}
              </div>
              {sheet.archetype && (
                <div style={{ color: '#aaa', fontSize: '0.9rem', marginBottom: '0.25rem' }}>
                  Archetype: <span style={{ color: '#ccc' }}>{sheet.archetype}</span>
                </div>
              )}
              {sheet.team && (
                <div style={{ display: 'inline-block', background: 'rgba(120,180,255,0.15)', border: '1px solid rgba(120,180,255,0.4)', borderRadius: '10px', padding: '1px 10px', fontSize: '0.8rem', color: '#7bb8ff' }}>
                  {sheet.team}
                </div>
              )}
            </section>

            <section style={{ marginBottom: '1.25rem' }}>
              <h4 style={{ color: '#aaa', marginBottom: '0.5rem', textTransform: 'uppercase', fontSize: '0.75rem', letterSpacing: '0.08em' }}>Progression</h4>
              <div style={{ color: '#eee', marginBottom: '0.5rem' }}>
                Level <span style={{ fontSize: '1.2rem', fontWeight: 'bold', color: '#f0c040' }}>{sheet.level}</span>
              </div>
              <div style={{ marginBottom: '0.25rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', color: '#aaa', marginBottom: '3px' }}>
                  <span>XP</span>
                  <span>{xp} / {xpToNext > 0 ? xpToNext : '—'}</span>
                </div>
                {xpToNext > 0 && (
                  <div style={{ background: '#333', borderRadius: '4px', height: '8px', overflow: 'hidden' }}>
                    <div style={{ width: `${xpPercent}%`, background: 'linear-gradient(90deg, #f0c040, #f08020)', height: '100%', borderRadius: '4px', transition: 'width 0.3s ease' }} />
                  </div>
                )}
              </div>
            </section>

            {(pendingBoosts > 0 || pendingSkillIncreases > 0) && (
              <section>
                <h4 style={{ color: '#aaa', marginBottom: '0.5rem', textTransform: 'uppercase', fontSize: '0.75rem', letterSpacing: '0.08em' }}>Pending</h4>
                {pendingBoosts > 0 && (
                  <div style={{ background: 'rgba(240,192,64,0.12)', border: '1px solid rgba(240,192,64,0.4)', borderRadius: '6px', padding: '6px 10px', marginBottom: '6px', color: '#f0c040', fontSize: '0.9rem' }}>
                    {pendingBoosts} ability boost{pendingBoosts !== 1 ? 's' : ''} available
                  </div>
                )}
                {pendingSkillIncreases > 0 && (
                  <div style={{ background: 'rgba(120,220,120,0.12)', border: '1px solid rgba(120,220,120,0.4)', borderRadius: '6px', padding: '6px 10px', color: '#78dc78', fontSize: '0.9rem' }}>
                    {pendingSkillIncreases} skill increase{pendingSkillIncreases !== 1 ? 's' : ''} available
                  </div>
                )}
              </section>
            )}
          </>
        )}
      </div>
    </>
  )
}
