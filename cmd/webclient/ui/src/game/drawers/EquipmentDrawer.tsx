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

export function EquipmentDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.characterSheet) {
      sendMessage('CharacterSheetRequest', {})
    }
  }, [state.characterSheet, sendMessage])

  const sheet = state.characterSheet

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
            {sheet.armor && Object.entries(sheet.armor as Record<string, string>).map(([slot, item]) => (
              <EquipSlot key={slot} label={slot} value={item} />
            ))}
            {sheet.accessories && Object.entries(sheet.accessories as Record<string, string>).map(([slot, item]) => (
              <EquipSlot key={slot} label={slot} value={item} />
            ))}
          </>
        )}
      </div>
    </>
  )
}
