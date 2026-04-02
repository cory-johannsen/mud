import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type { LoadoutWeaponPreset } from '../../proto'

function EquipSlot({
  label,
  value,
  bonus,
  dmg,
  onUnequip,
}: {
  label: string
  value?: string | null
  bonus?: string | null
  dmg?: string | null
  onUnequip?: () => void
}) {
  return (
    <div className="equip-slot">
      <div className="equip-slot-label">{label}</div>
      {value ? (
        <div className="equip-slot-value" style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
          <span>{value}{bonus ? ` (${bonus})` : ''}{dmg ? ` [${dmg}]` : ''}</span>
          {onUnequip && (
            <button style={styles.unequipBtn} onClick={onUnequip} type="button">Unequip</button>
          )}
        </div>
      ) : (
        <div className="equip-slot-value equip-empty">—</div>
      )}
    </div>
  )
}

function WeaponSlot({ label, name, damage }: { label: string; name?: string; damage?: string }) {
  const isEmpty = !name
  return (
    <div style={styles.weaponSlot}>
      <div style={styles.weaponSlotLabel}>{label}</div>
      {isEmpty ? (
        <div style={{ ...styles.weaponSlotValue, color: '#444' }}>—</div>
      ) : (
        <div style={styles.weaponSlotValue}>
          <span style={{ color: '#e0c060', fontSize: '0.85rem' }}>{name}</span>
          {damage && <span style={{ color: '#999', fontSize: '0.75rem' }}> {damage}</span>}
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
        <span style={styles.presetName}>Preset {index + 1}</span>
        {isActive
          ? <span style={styles.activeBadge}>active</span>
          : (
            <button
              style={styles.switchBtn}
              onClick={() => onSwitch(index + 1)}
              disabled={isSwitching}
              type="button"
            >
              Switch
            </button>
          )
        }
      </div>
      <WeaponSlot label="Main" name={preset.mainHand} damage={preset.mainHandDamage} />
      <WeaponSlot label="Off" name={preset.offHand} damage={preset.offHandDamage} />
    </div>
  )
}

const ARMOR_SLOTS: Array<{ key: string; label: string }> = [
  { key: 'head',      label: 'Head'      },
  { key: 'torso',     label: 'Torso'     },
  { key: 'left_arm',  label: 'Left Arm'  },
  { key: 'right_arm', label: 'Right Arm' },
  { key: 'hands',     label: 'Hands'     },
  { key: 'left_leg',  label: 'Left Leg'  },
  { key: 'right_leg', label: 'Right Leg' },
  { key: 'feet',      label: 'Feet'      },
]

const ACCESSORY_SLOTS: Array<{ key: string; label: string }> = [
  { key: 'neck',         label: 'Neck'              },
  { key: 'left_ring_1',  label: 'Left Hand Ring 1'  },
  { key: 'left_ring_2',  label: 'Left Hand Ring 2'  },
  { key: 'left_ring_3',  label: 'Left Hand Ring 3'  },
  { key: 'left_ring_4',  label: 'Left Hand Ring 4'  },
  { key: 'left_ring_5',  label: 'Left Hand Ring 5'  },
  { key: 'right_ring_1', label: 'Right Hand Ring 1' },
  { key: 'right_ring_2', label: 'Right Hand Ring 2' },
  { key: 'right_ring_3', label: 'Right Hand Ring 3' },
  { key: 'right_ring_4', label: 'Right Hand Ring 4' },
  { key: 'right_ring_5', label: 'Right Hand Ring 5' },
]

export function EquipmentDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage, sendCommand, clearLoadout } = useGame()
  const [isSwitching, setIsSwitching] = useState(false)

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
    sendMessage('LoadoutRequest', { arg: '' })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Re-fetch after a switch settles.
  useEffect(() => {
    if (!isSwitching) return
    const id = setTimeout(() => {
      sendMessage('LoadoutRequest', { arg: '' })
      sendMessage('CharacterSheetRequest', {})
      setIsSwitching(false)
    }, 400)
    return () => clearTimeout(id)
  }, [isSwitching, sendMessage])

  function handleSwitch(presetNumber: number) {
    setIsSwitching(true)
    sendMessage('LoadoutRequest', { arg: String(presetNumber) })
  }

  // REQ-WEC-3/4/5: send unequip command then refresh character sheet and inventory.
  function handleUnequip(slot: string) {
    sendCommand(`unequip ${slot}`)
    sendMessage('CharacterSheetRequest', {})
    sendMessage('InventoryRequest', {})
  }

  function handleClose() {
    clearLoadout()
    onClose()
  }

  const sheet = state.characterSheet
  const armor = (sheet?.armor ?? {}) as Record<string, string>
  const accessories = (sheet?.accessories ?? {}) as Record<string, string>
  const mainHand = sheet?.mainHand ?? sheet?.main_hand ?? null
  const mainHandBonus = sheet?.mainHandAttackBonus ?? sheet?.main_hand_attack_bonus ?? null
  const mainHandDamage = sheet?.mainHandDamage ?? sheet?.main_hand_damage ?? null
  const offHand = sheet?.offHand ?? sheet?.off_hand ?? null
  const offHandBonus = sheet?.offHandAttackBonus ?? null
  const offHandDamage = sheet?.offHandDamage ?? null
  const lv = state.loadoutView
  const activeIndex = lv?.activeIndex ?? 0
  const presets = lv?.presets ?? []

  return (
    <>
      <div className="drawer-header">
        <h3>Equipment</h3>
        <button className="drawer-close" onClick={handleClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!sheet ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : (
          <>
            {/* Loadout presets side-by-side */}
            <div style={styles.sectionLabel}>Loadouts</div>
            {presets.length === 0 ? (
              <p style={{ color: '#666', fontSize: '0.8rem', marginBottom: '0.75rem' }}>No loadout presets.</p>
            ) : (
              <div style={styles.presetRow}>
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
            {isSwitching && <p style={{ color: '#666', fontSize: '0.8rem', fontStyle: 'italic', marginBottom: '0.5rem' }}>Switching…</p>}

            {/* Weapons (active preset — with unequip) */}
            <div style={{ ...styles.sectionLabel, marginTop: '0.75rem' }}>Weapons</div>
            <EquipSlot
              label="Main Hand"
              value={mainHand}
              bonus={mainHandBonus}
              dmg={mainHandDamage}
              onUnequip={mainHand ? () => handleUnequip('main') : undefined}
            />
            <EquipSlot
              label="Off Hand"
              value={offHand}
              bonus={offHandBonus}
              dmg={offHandDamage}
              onUnequip={offHand ? () => handleUnequip('off') : undefined}
            />

            {/* Armor */}
            <div style={{ ...styles.sectionLabel, marginTop: '0.75rem' }}>Armor</div>
            {ARMOR_SLOTS.map(({ key, label }) => {
              const armorCategories = (sheet?.armorCategories ?? sheet?.armor_categories ?? {}) as Record<string, string>
              const cat = armorCategories[key]
              const displayName = armor[key] ? (cat ? `${armor[key]} [${cat}]` : armor[key]) : null
              return (
                <EquipSlot
                  key={key}
                  label={label}
                  value={displayName}
                  onUnequip={armor[key] ? () => handleUnequip(key) : undefined}
                />
              )
            })}

            {/* Accessories */}
            <div style={{ ...styles.sectionLabel, marginTop: '0.75rem' }}>Accessories</div>
            {ACCESSORY_SLOTS.map(({ key, label }) => (
              <EquipSlot
                key={key}
                label={label}
                value={accessories[key] || null}
                onUnequip={accessories[key] ? () => handleUnequip(key) : undefined}
              />
            ))}
          </>
        )}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  sectionLabel: {
    color: '#7af',
    fontSize: '0.72rem',
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    marginBottom: '0.4rem',
    borderBottom: '1px solid #2a2a2a',
    paddingBottom: '0.2rem',
  },
  presetRow: {
    display: 'flex',
    gap: '0.5rem',
    marginBottom: '0.5rem',
  },
  presetCard: {
    flex: 1,
    background: '#111',
    border: '1px solid #2a2a2a',
    borderRadius: '4px',
    padding: '0.5rem 0.6rem',
    minWidth: 0,
  },
  presetCardActive: {
    borderColor: '#4a6a2a',
    background: '#0d1a0d',
  },
  presetHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '0.4rem',
  },
  presetName: {
    color: '#ccc',
    fontSize: '0.8rem',
    fontWeight: 600,
  },
  activeBadge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.35rem',
    borderRadius: '3px',
    background: '#2a3a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    whiteSpace: 'nowrap' as const,
  },
  switchBtn: {
    padding: '0.1rem 0.45rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.7rem',
  },
  weaponSlot: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.3rem',
    marginBottom: '0.2rem',
  },
  weaponSlotLabel: {
    color: '#7af',
    fontSize: '0.68rem',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.05em',
    minWidth: '2.2rem',
    flexShrink: 0,
  },
  weaponSlotValue: {
    fontSize: '0.82rem',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
  },
  unequipBtn: {
    padding: '0.1rem 0.4rem',
    background: '#2a1a1a',
    border: '1px solid #6a2a2a',
    color: '#d84',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.65rem',
    flexShrink: 0,
  },
}
