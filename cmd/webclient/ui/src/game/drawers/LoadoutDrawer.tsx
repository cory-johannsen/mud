import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type { LoadoutWeaponPreset } from '../../proto'

function WeaponSlot({ label, name, damage }: { label: string; name?: string; damage?: string }) {
  const isEmpty = !name
  return (
    <div style={styles.weaponSlot}>
      <div style={styles.slotLabel}>{label}</div>
      {isEmpty ? (
        <div style={{ ...styles.slotValue, ...styles.emptySlot }}>—</div>
      ) : (
        <div style={styles.slotValue}>
          <span style={styles.weaponName}>{name}</span>
          {damage && <span style={styles.damageBadge}>{damage}</span>}
        </div>
      )}
    </div>
  )
}

function PresetCard({
  preset,
  index,
  isActive,
  onSwitch,
  isSwitching,
}: {
  preset: LoadoutWeaponPreset
  index: number
  isActive: boolean
  onSwitch: (n: number) => void
  isSwitching: boolean
}) {
  return (
    <div style={{ ...styles.presetCard, ...(isActive ? styles.presetCardActive : {}) }}>
      <div style={styles.presetHeader}>
        <div style={styles.presetLabel}>
          <span style={styles.presetName}>Preset {index + 1}</span>
          {isActive && <span style={styles.activeBadge}>active</span>}
        </div>
        {!isActive && (
          <button
            style={styles.switchBtn}
            onClick={() => onSwitch(index + 1)}
            disabled={isSwitching}
            type="button"
          >
            Switch
          </button>
        )}
      </div>
      <WeaponSlot label="Main Hand" name={preset.mainHand} damage={preset.mainHandDamage} />
      <WeaponSlot label="Off Hand" name={preset.offHand} damage={preset.offHandDamage} />
    </div>
  )
}

export function LoadoutDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage, clearLoadout } = useGame()
  const [isSwitching, setIsSwitching] = useState(false)

  useEffect(() => {
    sendMessage('LoadoutRequest', { arg: '' })
  }, [sendMessage])

  // Re-fetch after a switch settles.
  useEffect(() => {
    if (!isSwitching) return
    const id = setTimeout(() => {
      sendMessage('LoadoutRequest', { arg: '' })
      setIsSwitching(false)
    }, 400)
    return () => clearTimeout(id)
  }, [isSwitching, sendMessage])

  function handleSwitch(presetNumber: number) {
    setIsSwitching(true)
    sendMessage('LoadoutRequest', { arg: String(presetNumber) })
  }

  const lv = state.loadoutView
  const activeIndex = lv?.activeIndex ?? 0
  const presets = lv?.presets ?? []

  return (
    <>
      <div className="drawer-header">
        <h3>Loadouts</h3>
        <button className="drawer-close" onClick={() => { clearLoadout(); onClose() }}>✕</button>
      </div>
      <div className="drawer-body">
        {!lv ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : presets.length === 0 ? (
          <p style={{ color: '#666' }}>No loadout presets configured.</p>
        ) : (
          <div style={styles.presetList}>
            {presets.map((preset, i) => (
              <PresetCard
                key={`preset-${i}`}
                preset={preset}
                index={i}
                isActive={i === activeIndex}
                onSwitch={handleSwitch}
                isSwitching={isSwitching}
              />
            ))}
          </div>
        )}
        {isSwitching && <p style={styles.switchingNote}>Switching…</p>}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  presetList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.75rem',
  },
  presetCard: {
    background: '#111',
    border: '1px solid #2a2a2a',
    borderRadius: '4px',
    padding: '0.6rem 0.75rem',
  },
  presetCardActive: {
    borderColor: '#4a6a2a',
    background: '#0d1a0d',
  },
  presetHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '0.5rem',
  },
  presetLabel: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
  },
  presetName: {
    color: '#ccc',
    fontSize: '0.85rem',
    fontWeight: 600,
  },
  activeBadge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#2a3a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    whiteSpace: 'nowrap' as const,
  },
  switchBtn: {
    padding: '0.15rem 0.55rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  weaponSlot: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.5rem',
    marginBottom: '0.25rem',
  },
  slotLabel: {
    color: '#7af',
    fontSize: '0.72rem',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.06em',
    minWidth: '5.5rem',
    flexShrink: 0,
  },
  slotValue: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.4rem',
    flexWrap: 'wrap' as const,
  },
  emptySlot: {
    color: '#444',
    fontSize: '0.85rem',
  },
  weaponName: {
    color: '#e0c060',
    fontSize: '0.85rem',
  },
  damageBadge: {
    color: '#999',
    fontSize: '0.75rem',
  },
  switchingNote: {
    color: '#666',
    fontSize: '0.8rem',
    marginTop: '0.5rem',
    fontStyle: 'italic' as const,
  },
}
