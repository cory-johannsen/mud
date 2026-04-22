import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type { InventoryItem, LoadoutWeaponPreset } from '../../proto'
import { DamageTypeIcon, parseDamageType } from '../DamageTypeIcon'

interface WeaponTooltipData {
  name: string
  damage: string
  attackBonus: string
  abilityBonus: number
  profBonus: number
  profRank: string
  // GH #242: the weapon's required proficiency category (e.g. "simple_melee").
  profCategory: string
}

// humanProficiencyCategory converts a snake_case proficiency category like
// "martial_melee" to a Title Case display string like "Martial Melee".
function humanProficiencyCategory(cat: string): string {
  if (!cat) return ''
  return cat
    .split('_')
    .map(p => p.length === 0 ? '' : p[0].toUpperCase() + p.slice(1))
    .join(' ')
}

function WeaponTooltip({ data }: { data: WeaponTooltipData }): JSX.Element {
  const fmt = (n: number) => n >= 0 ? `+${n}` : `${n}`
  return (
    <div
      data-testid="weapon-tooltip"
      style={{
        position: 'absolute',
        zIndex: 100,
        background: '#1a1a2e',
        border: '1px solid #4a5a7a',
        borderRadius: 4,
        padding: '0.4rem 0.6rem',
        color: '#ccc',
        fontFamily: 'monospace',
        fontSize: '0.75rem',
        whiteSpace: 'nowrap',
        pointerEvents: 'none',
        minWidth: '12rem',
      }}
    >
      <div style={{ color: '#e0c060', fontWeight: 'bold', marginBottom: '0.25rem' }}>{data.name}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
        <span style={{ color: '#7af' }}>Damage:</span>
        <DamageTypeIcon damageType={parseDamageType(data.damage)} size="0.85em" />
        <span>{data.damage}</span>
      </div>
      <div style={{ marginTop: '0.2rem' }}>
        <span style={{ color: '#7af' }}>To-hit:</span> {data.attackBonus}
      </div>
      <div style={{ paddingLeft: '0.75rem', color: '#aaa', fontSize: '0.7rem' }}>
        <div>Ability:      {fmt(data.abilityBonus)}</div>
        <div>Proficiency ({data.profRank || 'untrained'}): {fmt(data.profBonus)}</div>
        {data.profCategory && (
          <div data-testid="weapon-req-proficiency">
            Required: {humanProficiencyCategory(data.profCategory)}
          </div>
        )}
      </div>
    </div>
  )
}

function EquipSlot({
  label,
  value,
  bonus,
  dmg,
  onUnequip,
  onEquip,
  weaponTooltip,
}: {
  label: string
  value?: string | null
  bonus?: string | null
  dmg?: string | null
  onUnequip?: () => void
  onEquip?: () => void
  weaponTooltip?: WeaponTooltipData | null
}) {
  const [hovered, setHovered] = useState(false)
  return (
    <div className="equip-slot">
      <div className="equip-slot-label">{label}</div>
      {value ? (
        <div
          className="equip-slot-value"
          style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', position: 'relative' }}
          data-weapon-slot={weaponTooltip ? 'true' : undefined}
          onMouseEnter={weaponTooltip ? () => setHovered(true) : undefined}
          onMouseLeave={weaponTooltip ? () => setHovered(false) : undefined}
        >
          <span>{value}{bonus ? ` (${bonus})` : ''}{dmg ? ` [${dmg}]` : ''}</span>
          {onUnequip && (
            <button style={styles.unequipBtn} onClick={onUnequip} type="button">Unequip</button>
          )}
          {weaponTooltip && hovered && (
            <div style={{ position: 'absolute', top: '100%', left: 0, marginTop: '0.25rem' }}>
              <WeaponTooltip data={weaponTooltip} />
            </div>
          )}
        </div>
      ) : (
        <div className="equip-slot-value equip-empty">
          {onEquip
            ? <button style={styles.equipEmptyBtn} onClick={onEquip} type="button">—</button>
            : '—'
          }
        </div>
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

// SlotPicker renders an inline list of items eligible for a slot.
// For weapon slots (slotKind === 'weapon'): shows weapons; selecting one triggers preset/hand picker.
// For armor slots (slotKind === 'armor'): shows matching armor items; selecting one calls onWear.
function SlotPicker({
  slotKey,
  slotKind,
  items,
  numPresets,
  onEquip,
  onWear,
  onCancel,
}: {
  slotKey: string       // 'main' | 'off' | armor slot key
  slotKind: 'weapon' | 'armor'
  items: InventoryItem[]
  numPresets: number
  onEquip: (weaponId: string, slot: 'main' | 'off', preset: number) => void
  onWear: (itemId: string, slot: string) => void
  onCancel: () => void
}) {
  const [stage, setStage] = useState<'items' | 'preset'>('items')
  const [chosenItem, setChosenItem] = useState<string>('')

  if (items.length === 0) {
    return (
      <div style={styles.pickerOverlay} data-testid="slot-picker">
        <span style={{ color: '#666', fontSize: '0.75rem' }}>No eligible items</span>
        <button style={styles.cancelBtn} onClick={onCancel} type="button">✕</button>
      </div>
    )
  }

  if (stage === 'items') {
    return (
      <div style={styles.pickerOverlay} data-testid="slot-picker">
        {items.map(item => {
          const id = item.itemDefId ?? item.item_def_id ?? item.name
          return (
            <button
              key={id}
              style={styles.pickerBtn}
              type="button"
              onClick={() => {
                if (slotKind === 'armor') {
                  onWear(id, slotKey)
                } else if (numPresets <= 1) {
                  onEquip(id, slotKey as 'main' | 'off', 1)
                } else {
                  setChosenItem(id)
                  setStage('preset')
                }
              }}
            >
              {item.name}
            </button>
          )
        })}
        <button style={styles.cancelBtn} onClick={onCancel} type="button">✕</button>
      </div>
    )
  }

  // stage === 'preset'
  return (
    <div style={styles.pickerOverlay} data-testid="slot-picker">
      {Array.from({ length: numPresets }, (_, i) => (
        <button
          key={i}
          style={styles.pickerBtn}
          type="button"
          onClick={() => onEquip(chosenItem, slotKey as 'main' | 'off', i + 1)}
        >
          Preset {i + 1}
        </button>
      ))}
      <button style={styles.cancelBtn} onClick={() => setStage('items')} type="button">←</button>
      <button style={styles.cancelBtn} onClick={onCancel} type="button">✕</button>
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
  const [pickingSlot, setPickingSlot] = useState<string | null>(null)

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
    sendMessage('LoadoutRequest', { arg: '' })
    sendMessage('InventoryRequest', {})
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

  // REQ-UI-EQUIP-2: Clicking an empty slot equips an item from inventory.
  function handleEquipFromSlot(weaponId: string, slot: 'main' | 'off', preset: number) {
    sendMessage('EquipRequest', { weaponId, slot, preset })
    setPickingSlot(null)
  }

  function handleWearFromSlot(itemId: string, slot: string) {
    sendMessage('WearRequest', { item_id: itemId, slot })
    setPickingSlot(null)
  }

  const invItems = state.inventoryView?.items ?? []
  const weaponItems = invItems.filter(it => it.kind === 'weapon')

  function armorItemsForSlot(slotKey: string): InventoryItem[] {
    return invItems.filter(it => {
      const s = it.armorSlot ?? it.armor_slot ?? ''
      return s === slotKey
    })
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

  // REQ-WEC-71: build weapon tooltip data from character sheet breakdown fields.
  const mainHandTooltip: WeaponTooltipData | null = (mainHand && mainHandBonus && mainHandDamage) ? {
    name: mainHand,
    damage: mainHandDamage,
    attackBonus: mainHandBonus,
    abilityBonus: sheet?.mainHandAbilityBonus ?? sheet?.main_hand_ability_bonus ?? 0,
    profBonus: sheet?.mainHandProfBonus ?? sheet?.main_hand_prof_bonus ?? 0,
    profRank: sheet?.mainHandProfRank ?? sheet?.main_hand_prof_rank ?? '',
    profCategory: sheet?.mainHandProfCategory ?? sheet?.main_hand_prof_category ?? '',
  } : null
  const offHandTooltip: WeaponTooltipData | null = (offHand && offHandBonus && offHandDamage) ? {
    name: offHand,
    damage: offHandDamage,
    attackBonus: offHandBonus,
    abilityBonus: sheet?.offHandAbilityBonus ?? sheet?.off_hand_ability_bonus ?? 0,
    profBonus: sheet?.offHandProfBonus ?? sheet?.off_hand_prof_bonus ?? 0,
    profRank: sheet?.offHandProfRank ?? sheet?.off_hand_prof_rank ?? '',
    profCategory: sheet?.offHandProfCategory ?? sheet?.off_hand_prof_category ?? '',
  } : null
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
              onEquip={!mainHand ? () => setPickingSlot('main') : undefined}
              weaponTooltip={mainHandTooltip}
            />
            {pickingSlot === 'main' && (
              <SlotPicker
                slotKey="main"
                slotKind="weapon"
                items={weaponItems}
                numPresets={presets.length || 1}
                onEquip={handleEquipFromSlot}
                onWear={handleWearFromSlot}
                onCancel={() => setPickingSlot(null)}
              />
            )}
            <EquipSlot
              label="Off Hand"
              value={offHand}
              bonus={offHandBonus}
              dmg={offHandDamage}
              onUnequip={offHand ? () => handleUnequip('off') : undefined}
              onEquip={!offHand ? () => setPickingSlot('off') : undefined}
              weaponTooltip={offHandTooltip}
            />
            {pickingSlot === 'off' && (
              <SlotPicker
                slotKey="off"
                slotKind="weapon"
                items={weaponItems}
                numPresets={presets.length || 1}
                onEquip={handleEquipFromSlot}
                onWear={handleWearFromSlot}
                onCancel={() => setPickingSlot(null)}
              />
            )}

            {/* Armor */}
            <div style={{ ...styles.sectionLabel, marginTop: '0.75rem' }}>Armor</div>
            {ARMOR_SLOTS.map(({ key, label }) => {
              const armorCategories = (sheet?.armorCategories ?? sheet?.armor_categories ?? {}) as Record<string, string>
              const cat = armorCategories[key]
              const displayName = armor[key] ? (cat ? `${armor[key]} [${cat}]` : armor[key]) : null
              return (
                <div key={key}>
                  <EquipSlot
                    label={label}
                    value={displayName}
                    onUnequip={armor[key] ? () => handleUnequip(key) : undefined}
                    onEquip={!armor[key] ? () => setPickingSlot(key) : undefined}
                  />
                  {pickingSlot === key && (
                    <SlotPicker
                      slotKey={key}
                      slotKind="armor"
                      items={armorItemsForSlot(key)}
                      numPresets={presets.length || 1}
                      onEquip={handleEquipFromSlot}
                      onWear={handleWearFromSlot}
                      onCancel={() => setPickingSlot(null)}
                    />
                  )}
                </div>
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
  equipEmptyBtn: {
    background: 'transparent',
    border: 'none',
    color: '#555',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    padding: '0 0.2rem',
    textDecoration: 'underline dotted',
  },
  pickerOverlay: {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: '0.25rem',
    padding: '0.3rem 0.4rem',
    background: '#0d0d1a',
    border: '1px solid #3a3a5a',
    borderRadius: '4px',
    marginBottom: '0.4rem',
  },
  pickerBtn: {
    padding: '0.15rem 0.45rem',
    background: '#1a1a3a',
    border: '1px solid #4a4a7a',
    color: '#aac',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.72rem',
  },
  cancelBtn: {
    padding: '0.15rem 0.4rem',
    background: '#2a1a1a',
    border: '1px solid #5a2a2a',
    color: '#c66',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.72rem',
  },
}
