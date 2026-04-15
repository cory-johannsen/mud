import { useState } from 'react'
import { useGame } from '../GameContext'
import type { PendingFeatChoice } from '../../proto/index'

// FeatChoiceModal presents a feat selection modal for pending feat choices.
//
// REQ-FCM-4: Opens when player clicks the notification badge.
// REQ-FCM-5: Shows each feat option with name, category badge, and description.
// REQ-FCM-6: Confirm button disabled until selected count === choice.count.
// REQ-FCM-7: On confirm, sends one ChooseFeatRequest per selected feat.
// REQ-FCM-11: Dark-theme monospace styling matching AbilityBoostModal.

interface Props {
  choices: PendingFeatChoice[]
  onClose: () => void
}

export function FeatChoiceModal({ choices, onClose }: Props) {
  const { sendMessage } = useGame()
  const [choiceIndex, setChoiceIndex] = useState(0)
  const [selected, setSelected] = useState<string[]>([])

  if (choices.length === 0) return null
  const choice = choices[choiceIndex]
  const required = choice.count ?? 1
  const options = choice.options ?? []
  const grantLevel = choice.grantLevel ?? choice.grant_level ?? 1

  function toggleFeat(featId: string) {
    setSelected(prev => {
      if (prev.includes(featId)) return prev.filter(id => id !== featId)
      if (prev.length >= required) return prev
      return [...prev, featId]
    })
  }

  function handleConfirm() {
    for (const featId of selected) {
      sendMessage('ChooseFeatRequest', { grantLevel, featId })
    }
    if (choiceIndex + 1 < choices.length) {
      setChoiceIndex(i => i + 1)
      setSelected([])
    } else {
      onClose()
    }
  }

  const categoryColor = (cat?: string) => {
    if (cat === 'job') return '#ffcc88'
    if (cat === 'skill') return '#88ccff'
    return '#aaa'
  }

  return (
    <div style={styles.overlay}>
      <div style={styles.modal} onClick={e => e.stopPropagation()}>
        <div style={styles.header}>
          <span style={styles.title}>
            Choose {required} Feat{required !== 1 ? 's' : ''} — Level {grantLevel}
          </span>
          {choices.length > 1 && (
            <span style={styles.pager}>{choiceIndex + 1} / {choices.length}</span>
          )}
        </div>
        <div style={styles.body}>
          {options.map(opt => {
            const featId = opt.featId ?? opt.feat_id ?? ''
            const isSelected = selected.includes(featId)
            return (
              <button
                key={featId}
                type="button"
                style={{ ...styles.card, ...(isSelected ? styles.cardSelected : {}) }}
                onClick={() => toggleFeat(featId)}
              >
                <div style={styles.cardHeader}>
                  <span style={styles.featName}>{opt.name ?? featId}</span>
                  {opt.category && (
                    <span style={{ ...styles.categoryBadge, color: categoryColor(opt.category), borderColor: categoryColor(opt.category) }}>
                      {opt.category}
                    </span>
                  )}
                </div>
                {opt.description && (
                  <div style={styles.description}>{opt.description}</div>
                )}
              </button>
            )
          })}
        </div>
        <div style={styles.footer}>
          <span style={styles.counter}>{selected.length} / {required} selected</span>
          <button
            type="button"
            style={{ ...styles.confirmBtn, ...(selected.length !== required ? styles.confirmBtnDisabled : {}) }}
            disabled={selected.length !== required}
            onClick={handleConfirm}
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.8)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 300,
  },
  modal: {
    background: '#111',
    border: '1px solid #4a6a2a',
    borderRadius: '6px',
    width: 'min(560px, 95vw)',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    fontFamily: 'monospace',
  },
  header: {
    padding: '0.75rem 1rem',
    borderBottom: '1px solid #2a3a1a',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    flexShrink: 0,
  },
  title: {
    color: '#e0c060',
    fontSize: '0.95rem',
    fontWeight: 600,
  },
  pager: {
    color: '#666',
    fontSize: '0.8rem',
  },
  body: {
    padding: '0.75rem 1rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    overflowY: 'auto',
  },
  card: {
    background: '#1a2a1a',
    border: '1px solid #333',
    borderRadius: '4px',
    padding: '0.6rem 0.75rem',
    textAlign: 'left',
    cursor: 'pointer',
    fontFamily: 'monospace',
    color: '#ccc',
    width: '100%',
  },
  cardSelected: {
    background: '#1a3a1a',
    border: '1px solid #8d4',
  },
  cardHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
    marginBottom: '0.25rem',
  },
  featName: {
    fontWeight: 700,
    color: '#eee',
    fontSize: '0.9rem',
  },
  categoryBadge: {
    fontSize: '0.7rem',
    border: '1px solid',
    borderRadius: '4px',
    padding: '0 4px',
  },
  description: {
    color: '#999',
    fontSize: '0.8rem',
    lineHeight: 1.4,
  },
  footer: {
    padding: '0.5rem 1rem',
    borderTop: '1px solid #2a3a1a',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    flexShrink: 0,
  },
  counter: {
    color: '#888',
    fontSize: '0.8rem',
  },
  confirmBtn: {
    background: '#3a5a1a',
    border: '1px solid #8d4',
    color: '#8d4',
    borderRadius: '4px',
    padding: '0.3rem 0.9rem',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    cursor: 'pointer',
  },
  confirmBtnDisabled: {
    background: '#222',
    border: '1px solid #444',
    color: '#555',
    cursor: 'not-allowed',
  },
}
