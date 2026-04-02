import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type { InventoryItem } from '../../proto'

function ConsumableRow({
  item,
  sendCommand,
  sendMessage,
}: {
  item: InventoryItem
  sendCommand: (raw: string) => void
  sendMessage: (type: string, payload: object) => void
}) {
  const itemDefId = item.itemDefId ?? item.item_def_id ?? ''
  const qty = item.quantity ?? 1

  function handleConsume() {
    sendCommand(`use ${itemDefId}`)
    sendMessage('InventoryRequest', {})
  }

  return (
    <tr>
      <td>{item.name}</td>
      <td>{item.kind}</td>
      <td>{qty}</td>
      <td>{(item.weight ?? 0).toFixed(1)}</td>
      <td>
        <button
          style={{ ...styles.actionBtn, background: '#a74', ...(qty <= 0 ? styles.actionBtnDisabled : {}) }}
          disabled={qty <= 0}
          onClick={handleConsume}
          type="button"
        >
          Consume
        </button>
      </td>
    </tr>
  )
}

type PickStage = 'preset' | 'hand'

function WeaponRow({
  item,
  numPresets,
  sendMessage,
}: {
  item: InventoryItem
  numPresets: number
  sendMessage: (type: string, payload: object) => void
}) {
  const [stage, setStage] = useState<PickStage | null>(null)
  const [chosenPreset, setChosenPreset] = useState<number>(0)
  const weaponId = item.itemDefId ?? item.item_def_id ?? ''

  function handlePreset(preset: number) {
    setChosenPreset(preset)
    setStage('hand')
  }

  function handleHand(slot: 'main' | 'off') {
    sendMessage('EquipRequest', { weaponId, slot, preset: chosenPreset })
    setStage(null)
    setChosenPreset(0)
  }

  return (
    <tr style={{ position: 'relative' }}>
      <td>{item.name}</td>
      <td>{item.kind}</td>
      <td>{item.quantity ?? 1}</td>
      <td>{(item.weight ?? 0).toFixed(1)}</td>
      <td style={{ position: 'relative' }}>
        {stage === null && (
          <button
            style={styles.actionBtn}
            onClick={() => setStage('preset')}
            type="button"
          >
            Equip
          </button>
        )}
        {stage === 'preset' && (
          <div style={styles.pickerOverlay}>
            {Array.from({ length: numPresets }, (_, i) => (
              <button
                key={i}
                style={styles.pickerBtn}
                onClick={() => handlePreset(i + 1)}
                type="button"
              >
                Preset {i + 1}
              </button>
            ))}
            <button style={styles.cancelBtn} onClick={() => setStage(null)} type="button">✕</button>
          </div>
        )}
        {stage === 'hand' && (
          <div style={styles.pickerOverlay}>
            <button style={styles.pickerBtn} onClick={() => handleHand('main')} type="button">Main Hand</button>
            <button style={styles.pickerBtn} onClick={() => handleHand('off')} type="button">Off Hand</button>
            <button style={styles.cancelBtn} onClick={() => { setStage('preset'); setChosenPreset(0) }} type="button">←</button>
            <button style={styles.cancelBtn} onClick={() => { setStage(null); setChosenPreset(0) }} type="button">✕</button>
          </div>
        )}
      </td>
    </tr>
  )
}

function ArmorRow({
  item,
  sendMessage,
}: {
  item: InventoryItem
  sendMessage: (type: string, payload: object) => void
}) {
  const armorSlot = item.armorSlot ?? item.armor_slot ?? ''
  const armorCategory = item.armorCategory ?? item.armor_category ?? ''
  const disabled = !armorSlot

  return (
    <tr>
      <td>{item.name}</td>
      <td>{armorCategory ? `armor (${armorCategory})` : item.kind}</td>
      <td>{item.quantity ?? 1}</td>
      <td>{(item.weight ?? 0).toFixed(1)}</td>
      <td>
        <button
          style={{ ...styles.actionBtn, ...(disabled ? styles.actionBtnDisabled : {}) }}
          disabled={disabled}
          onClick={() => {
            if (!disabled) {
              sendMessage('Wear', {
                item_id: item.itemDefId ?? item.item_def_id,
                slot: armorSlot,
              })
            }
          }}
          type="button"
        >
          Wear
        </button>
      </td>
    </tr>
  )
}

function PlainRow({ item }: { item: InventoryItem }) {
  return (
    <tr>
      <td>{item.name}</td>
      <td>{item.kind}</td>
      <td>{item.quantity ?? 1}</td>
      <td>{(item.weight ?? 0).toFixed(1)}</td>
      <td />
    </tr>
  )
}

export function InventoryDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage, sendCommand } = useGame()

  useEffect(() => {
    if (!state.inventoryView) {
      sendMessage('InventoryRequest', {})
    }
  }, [state.inventoryView, sendMessage])

  const inv = state.inventoryView
  const numPresets = state.loadoutView?.presets?.length ?? 2

  return (
    <>
      <div className="drawer-header">
        <h3>Inventory</h3>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        {!inv ? (
          <p style={{ color: '#666' }}>Loading…</p>
        ) : (
          <>
            <table className="drawer-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Kind</th>
                  <th>Qty</th>
                  <th>Weight</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {(inv.items ?? []).map((item, i) => {
                  if (item.kind === 'weapon') {
                    return (
                      <WeaponRow key={i} item={item} numPresets={numPresets} sendMessage={sendMessage} />
                    )
                  }
                  if (item.kind === 'armor') {
                    return (
                      <ArmorRow key={i} item={item} sendMessage={sendMessage} />
                    )
                  }
                  if (item.kind === 'consumable') {
                    return (
                      <ConsumableRow key={i} item={item} sendCommand={sendCommand} sendMessage={sendMessage} />
                    )
                  }
                  return <PlainRow key={i} item={item} />
                })}
              </tbody>
            </table>
            <div className="drawer-summary">
              <div>{inv.usedSlots ?? inv.used_slots ?? 0}/{inv.maxSlots ?? inv.max_slots ?? 0} slots · {((inv.totalWeight ?? inv.total_weight ?? 0) as number).toFixed(1)} kg</div>
              <div style={{ marginTop: '0.25rem' }}>{inv.currency ?? 0} credits</div>
            </div>
          </>
        )}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  actionBtn: {
    background: '#8d4',
    color: '#000',
    border: 'none',
    cursor: 'pointer',
    padding: '2px 6px',
    fontSize: '0.85em',
  },
  actionBtnDisabled: {
    opacity: 0.4,
    cursor: 'not-allowed',
  },
  pickerOverlay: {
    position: 'absolute',
    zIndex: 10,
    background: '#1a2a1a',
    border: '1px solid #8d4',
    padding: '4px',
    display: 'flex',
    gap: '4px',
    top: 0,
    left: 0,
  },
  pickerBtn: {
    background: '#9ab',
    color: '#000',
    border: 'none',
    cursor: 'pointer',
    padding: '2px 6px',
    fontSize: '0.85em',
    margin: '0 2px',
  },
  cancelBtn: {
    background: 'none',
    border: '1px solid #444',
    color: '#666',
    cursor: 'pointer',
    padding: '2px 6px',
    fontSize: '0.85em',
  },
}
