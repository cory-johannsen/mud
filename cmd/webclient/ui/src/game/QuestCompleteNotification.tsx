// QuestCompleteNotification — REQ-QCN-1: Visual notification when a quest completes.
//
// Precondition: state.questCompleteQueue is maintained by GameContext.
// Postcondition: Shows the first pending completion event; dismiss removes it from the queue.
import { useGame } from './GameContext'

export function QuestCompleteNotification(): JSX.Element | null {
  const { state, dismissQuestComplete } = useGame()
  const event = state.questCompleteQueue[0]
  if (!event) return null

  const title = event.title ?? ''
  const xp = event.xpReward ?? event.xp_reward ?? 0
  const credits = event.creditsReward ?? event.credits_reward ?? 0
  const items = event.itemRewards ?? event.item_rewards ?? []

  return (
    <div style={{
      position: 'fixed',
      bottom: '1.5rem',
      right: '1.5rem',
      zIndex: 300,
      background: '#0d1a0d',
      border: '2px solid #4a8a2a',
      borderRadius: 6,
      padding: '1rem 1.25rem',
      minWidth: 300,
      maxWidth: 400,
      fontFamily: 'monospace',
      boxShadow: '0 4px 24px rgba(0,0,0,0.7)',
    }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
        <div>
          <div style={{ fontSize: '0.65rem', color: '#8d4', textTransform: 'uppercase', letterSpacing: '0.1em', marginBottom: '0.2rem' }}>
            Quest Complete
          </div>
          <div style={{ fontSize: '1rem', fontWeight: 'bold', color: '#e0c060' }}>{title}</div>
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.2rem', margin: '0.5rem 0' }}>
        {xp > 0 && (
          <div style={{ fontSize: '0.82rem', color: '#aef' }}>
            +{xp} <span style={{ color: '#7bc' }}>XP</span>
          </div>
        )}
        {credits > 0 && (
          <div style={{ fontSize: '0.82rem', color: '#aef' }}>
            +{credits} <span style={{ color: '#7bc' }}>Credits</span>
          </div>
        )}
        {items.map((item, i) => (
          <div key={i} style={{ fontSize: '0.82rem', color: '#aef' }}>
            +{item}
          </div>
        ))}
      </div>

      <button
        type="button"
        aria-label="Dismiss"
        onClick={dismissQuestComplete}
        style={{
          marginTop: '0.5rem',
          width: '100%',
          padding: '0.3rem',
          background: '#1a2a1a',
          border: '1px solid #4a6a2a',
          color: '#8d4',
          borderRadius: 3,
          cursor: 'pointer',
          fontFamily: 'monospace',
          fontSize: '0.78rem',
        }}
      >
        Dismiss
      </button>
    </div>
  )
}
