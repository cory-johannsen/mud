// QuestsDrawer — REQ-QL-UI-1: Displays the player's active quests with objectives and rewards.
//
// Precondition: sendMessage is available via useGame; QuestLogRequest fetches quest data.
// Postcondition: Renders a list of active quests with title, description, objective
//   progress bars, and reward summary. Shows loading/empty states as appropriate.
import { useEffect } from 'react'
import { useGame } from '../GameContext'
import type { QuestEntryView, QuestObjectiveView } from '../../proto'

function ObjectiveRow({ obj }: { obj: QuestObjectiveView }) {
  const current = obj.current ?? 0
  const required = obj.required ?? 1
  const pct = required > 0 ? Math.min(100, (current / required) * 100) : 100
  const done = current >= required

  return (
    <div style={{ marginBottom: '0.4rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', color: done ? '#8d4' : '#aaa', marginBottom: '0.15rem' }}>
        <span>{obj.description ?? ''}</span>
        <span style={{ whiteSpace: 'nowrap', marginLeft: '0.5rem' }}>{current} / {required}</span>
      </div>
      <div style={{ height: 4, background: '#1a1a1a', borderRadius: 2, overflow: 'hidden' }}>
        <div style={{
          height: '100%',
          width: `${pct}%`,
          background: done ? '#4a8a2a' : '#2a5a8a',
          borderRadius: 2,
          transition: 'width 0.2s',
        }} />
      </div>
    </div>
  )
}

function QuestCard({ quest }: { quest: QuestEntryView }) {
  const title = quest.title ?? quest.questId ?? quest.quest_id ?? '(unknown)'
  const xp = quest.xpReward ?? quest.xp_reward ?? 0
  const credits = quest.creditsReward ?? quest.credits_reward ?? 0
  const objectives = quest.objectives ?? []
  const allDone = objectives.length > 0 && objectives.every(o => (o.current ?? 0) >= (o.required ?? 1))

  return (
    <div style={{
      marginBottom: '0.75rem',
      background: '#0f1420',
      border: `1px solid ${allDone ? '#4a6a2a' : '#1e2a3a'}`,
      borderRadius: 4,
      padding: '0.6rem 0.75rem',
    }}>
      <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginBottom: '0.25rem' }}>
        <span style={{ fontWeight: 'bold', fontSize: '0.88rem', color: '#e0c060' }}>{title}</span>
        {allDone && (
          <span style={{ fontSize: '0.65rem', color: '#8d4', background: '#1a2a1a', border: '1px solid #4a6a2a', borderRadius: 3, padding: '0.05rem 0.3rem', marginLeft: '0.5rem' }}>
            READY TO TURN IN
          </span>
        )}
      </div>
      {quest.description && (
        <p style={{ margin: '0 0 0.4rem', fontSize: '0.77rem', color: '#999', lineHeight: 1.4 }}>{quest.description}</p>
      )}
      {objectives.map(obj => (
        <ObjectiveRow key={obj.id ?? obj.description} obj={obj} />
      ))}
      {(xp > 0 || credits > 0) && (
        <div style={{ marginTop: '0.35rem', fontSize: '0.7rem', color: '#7bc', display: 'flex', gap: '1rem' }}>
          {xp > 0 && <span>XP: {xp}</span>}
          {credits > 0 && <span>Credits: {credits}</span>}
        </div>
      )}
    </div>
  )
}

export function QuestsDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  // Refresh quest log on mount and whenever a quest completes (so the completed
  // quest is removed from the drawer without requiring the player to close/reopen).
  useEffect(() => {
    sendMessage('QuestLogRequest', {})
  }, [sendMessage, state.questCompleteQueue.length])

  const questLogView = state.questLogView
  const quests = questLogView?.quests ?? null

  return (
    <div style={{ fontFamily: 'monospace', color: '#ccc' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
        <h3 style={{ margin: 0, color: '#e0c060', fontSize: '0.95rem' }}>Active Quests</h3>
        <button
          type="button"
          aria-label="Close"
          style={{ background: 'none', border: '1px solid #444', color: '#888', cursor: 'pointer', borderRadius: 3, padding: '0.1rem 0.4rem', fontFamily: 'monospace' }}
          onClick={onClose}
        >
          ✕
        </button>
      </div>

      {quests === null && (
        <p style={{ color: '#555', fontSize: '0.82rem' }}>Loading…</p>
      )}
      {quests !== null && quests.length === 0 && (
        <p style={{ color: '#555', fontSize: '0.82rem' }}>No active quests.</p>
      )}
      {quests !== null && quests.length > 0 && (
        <div>
          {quests.map(q => (
            <QuestCard key={q.questId ?? q.quest_id} quest={q} />
          ))}
        </div>
      )}
    </div>
  )
}
