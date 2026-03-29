import { useEffect } from 'react'
import { useGame } from '../GameContext'

export function FeatsDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet
  const feats = sheet?.feats ?? []

  return (
    <>
      <div className="drawer-header">
        <h3>Feats</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!sheet ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
            {(Array.isArray(feats) ? feats : []).map((f, i) => {
              const feat = f as { name?: string; description?: string }
              return (
                <li key={i} style={{ marginBottom: '0.6rem' }}>
                  <strong style={{ color: '#e0c060' }}>{feat.name ?? ''}</strong>
                  {feat.description && (
                    <p style={{ margin: '0.2rem 0 0', color: '#888', fontSize: '0.8rem' }}>
                      {feat.description}
                    </p>
                  )}
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </>
  )
}
