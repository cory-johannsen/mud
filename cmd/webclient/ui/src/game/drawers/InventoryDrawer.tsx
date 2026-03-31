import { useEffect, useState } from 'react'
import { useGame } from '../GameContext'
import type { InventoryItem } from '../../proto'

function WeaponRow({
  item,
  sendMessage,
}: {
  item: InventoryItem
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)
  const weaponId = item.itemDefId ?? item.item_def_id ?? ''

  return (
    <tr style={{ position: 'relative' }}>
      <td>{item.name}</td>
      <td>{item.kind}</td>
      <td>{item.quantity ?? 1}</td>
      <td>{(item.weight ?? 0).toFixed(1)}</td>
      <td style={{ position: 'relative' }}>
        {!picking && (
          <button
            style={styles.actionBtn}
            onClick={() => setPicking(true)}
            type="button"
          >
            Equip
          </button>
        )}
        {picking && (
          <div style={styles.slotPickerOverlay}>
            <button
              style={styles.slotPickerBtn}
              onClick={() => {
                sendMessage('Equip', { weapon_id: weaponId, slot: 'main' })
                setPicking(false)
              }}
              type="button"
            >
              Main Hand
            </button>
            <button
              style={styles.slotPickerBtn}
              onClick={() => {
                sendMessage('Equip', { weapon_id: weaponId, slot: 'off' })
                setPicking(false)
              }}
              type="button"
            >
              Off Hand
            </button>
            <button
              style={styles.cancelBtn}
              onClick={() => setPicking(false)}
              type="button"
            >
              ✕
            </button>
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
  const disabled = !armorSlot

  return (
    <tr>
      <td>{item.name}</td>
      <td>{item.kind}</td>
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
  const { state, sendMessage } = useGame()

  useEffect(() => {
    if (!state.inventoryView) {
      sendMessage('InventoryRequest', {})
    }
  }, [state.inventoryView, sendMessage])

  const inv = state.inventoryView

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
                      <WeaponRow key={i} item={item} sendMessage={sendMessage} />
                    )
                  }
                  if (item.kind === 'armor') {
                    return (
                      <ArmorRow key={i} item={item} sendMessage={sendMessage} />
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
  slotPickerOverlay: {
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
  slotPickerBtn: {
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
