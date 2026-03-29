import { useEffect } from 'react'
import { useGame } from '../GameContext'

function EquipSlot({ label, value, bonus, dmg }: { label: string; value?: string | null; bonus?: string | null; dmg?: string | null }) {
  return (
    <div className="equip-slot">
      <div className="equip-slot-label">{label}</div>
      {value ? (
        <div className="equip-slot-value">
          {value}{bonus ? ` (${bonus})` : ''}{dmg ? ` [${dmg}]` : ''}
        </div>
      ) : (
        <div className="equip-slot-value equip-empty">—</div>
      )}
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
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet
  const armor = (sheet?.armor ?? {}) as Record<string, string>
  const accessories = (sheet?.accessories ?? {}) as Record<string, string>

  return (
    <>
      <div className="drawer-header">
        <h3>Equipment</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!sheet ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : (
          <>
            <EquipSlot
              label="Main Hand"
              value={sheet.mainHand ?? sheet.main_hand}
              bonus={sheet.mainHandAttackBonus ?? sheet.main_hand_attack_bonus}
              dmg={sheet.mainHandDamage ?? sheet.main_hand_damage}
            />
            <EquipSlot label="Off Hand" value={sheet.offHand ?? sheet.off_hand} />
            {ARMOR_SLOTS.map(({ key, label }) => (
              <EquipSlot key={key} label={label} value={armor[key] || null} />
            ))}
            {ACCESSORY_SLOTS.map(({ key, label }) => (
              <EquipSlot key={key} label={label} value={accessories[key] || null} />
            ))}
          </>
        )}
      </div>
    </>
  )
}
