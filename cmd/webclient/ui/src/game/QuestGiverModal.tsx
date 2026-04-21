import { useGame } from './GameContext'
import type { QuestEntryView } from '../proto'
import { difficultyBorderColor } from './ZoneMapSvg'

const STATUS_STYLES: Record<string, React.CSSProperties> = {
  available:  { color: '#8d4', background: '#1a2a1a', border: '1px solid #4a6a2a' },
  active:     { color: '#7bf', background: '#1a1a3a', border: '1px solid #2a4a9a' },
  completed:  { color: '#888', background: '#1a1a1a', border: '1px solid #333' },
  locked:     { color: '#666', background: '#111', border: '1px solid #222' },
}

function QuestEntry({
  quest,
  onAccept,
  playerLevel,
}: {
  quest: QuestEntryView
  onAccept: (questId: string) => void
  playerLevel: number
}): JSX.Element {
  const id = quest.questId ?? quest.quest_id ?? ''
  const status = quest.status ?? 'available'
  const statusStyle = STATUS_STYLES[status] ?? STATUS_STYLES.available
  const objectives = quest.objectives ?? []
  const xp = quest.xpReward ?? quest.xp_reward ?? 0
  const credits = quest.creditsReward ?? quest.credits_reward ?? 0
  const levelRange = quest.levelRange ?? quest.level_range ?? ''
  // GH #239: color the quest card border by enemy-level difficulty relative
  // to the player's level, reusing the zone-map difficulty palette.
  const difficultyColor = difficultyBorderColor(levelRange || undefined, playerLevel)

  return (
    <div style={{
      marginBottom: '0.75rem',
      borderRadius: 4,
      padding: '0.5rem 0.65rem',
      ...statusStyle,
      ...(difficultyColor ? { borderColor: difficultyColor, borderWidth: 2, borderStyle: 'solid' } : {}),
    }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.2rem' }}>
        <span style={{ fontWeight: 'bold', fontSize: '0.9rem', color: '#e0c060' }}>{quest.title ?? id}</span>
        <span style={{ display: 'flex', gap: '0.35rem', alignItems: 'center' }}>
          {levelRange && (
            <span
              data-testid="quest-level-range"
              style={{
                fontSize: '0.65rem',
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                padding: '0.1rem 0.3rem',
                borderRadius: 3,
                background: 'rgba(0,0,0,0.3)',
                color: difficultyColor ?? '#aaa',
                border: difficultyColor ? `1px solid ${difficultyColor}` : '1px solid #333',
              }}
            >Lv {levelRange}</span>
          )}
          <span style={{
            fontSize: '0.65rem',
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
            padding: '0.1rem 0.3rem',
            borderRadius: 3,
            background: 'rgba(0,0,0,0.3)',
            color: statusStyle.color,
          }}>{status}</span>
        </span>
      </div>
      {quest.description && (
        <p style={{ margin: '0 0 0.3rem', fontSize: '0.78rem', color: '#bbb', lineHeight: 1.4 }}>{quest.description}</p>
      )}
      {objectives.map(obj => {
        const current = obj.current ?? 0
        const required = obj.required ?? 0
        return (
          <div key={obj.id} style={{ fontSize: '0.75rem', color: '#999', marginBottom: '0.1rem' }}>
            {obj.description}
            {status === 'active' && ` — ${current} / ${required}`}
          </div>
        )
      })}
      <div style={{ marginTop: '0.35rem', fontSize: '0.72rem', color: '#aaa', display: 'flex', gap: '1rem' }}>
        {xp > 0 && <span>XP: {xp}</span>}
        {credits > 0 && <span>Credits: {credits}</span>}
      </div>
      {status === 'available' && (
        <button
          type="button"
          style={{
            marginTop: '0.4rem',
            padding: '0.2rem 0.7rem',
            background: '#1a2a1a',
            border: '1px solid #4a6a2a',
            color: '#8d4',
            borderRadius: 3,
            cursor: 'pointer',
            fontFamily: 'monospace',
            fontSize: '0.75rem',
          }}
          onClick={() => onAccept(id)}
        >
          Accept
        </button>
      )}
    </div>
  )
}

// REQ-QGM-1: Quest giver modal displays available quests and allows acceptance.
//
// Precondition: state.questGiverView is set with NPC name and quest list.
// Postcondition: Modal shows all quests with status indicators; Accept dispatches TalkRequest.
export function QuestGiverModal(): JSX.Element | null {
  const { state, sendMessage, clearQuestGiverView } = useGame()
  const view = state.questGiverView
  if (!view) return null

  const npcName = view.npcName ?? view.npc_name ?? 'Quest Giver'
  const quests = view.quests ?? []
  const playerLevel = state.characterInfo?.level ?? 0

  function handleAccept(questId: string) {
    sendMessage('TalkRequest', { npc_name: npcName, args: `accept ${questId}` })
    clearQuestGiverView()
  }

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 200,
        background: 'rgba(0,0,0,0.7)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}
      onClick={clearQuestGiverView}
    >
      <div
        style={{
          background: '#161622',
          border: '1px solid #3a3a5a',
          borderRadius: 6,
          padding: '1rem 1.25rem',
          minWidth: 380,
          maxWidth: 520,
          maxHeight: '80vh',
          overflowY: 'auto',
          fontFamily: 'monospace',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.75rem' }}>
          <div>
            <h3 style={{ margin: 0, color: '#e0c060', fontSize: '1rem' }}>{npcName}</h3>
            <span style={{ fontSize: '0.65rem', color: '#7af', textTransform: 'uppercase', letterSpacing: '0.08em' }}>Quest Giver</span>
          </div>
          <button
            type="button"
            style={{ background: 'none', border: 'none', color: '#888', fontSize: '1rem', cursor: 'pointer' }}
            onClick={clearQuestGiverView}
          >
            ✕
          </button>
        </div>
        {quests.length === 0 ? (
          <p style={{ color: '#666', fontSize: '0.85rem' }}>No quests available.</p>
        ) : (
          quests.map(q => (
            <QuestEntry
              key={q.questId ?? q.quest_id}
              quest={q}
              onAccept={handleAccept}
              playerLevel={playerLevel}
            />
          ))
        )}
      </div>
    </div>
  )
}
