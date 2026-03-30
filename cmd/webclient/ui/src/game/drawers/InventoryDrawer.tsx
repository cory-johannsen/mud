import { useEffect } from 'react'
import { useGame } from '../GameContext'

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
                </tr>
              </thead>
              <tbody>
                {(inv.items ?? []).map((item, i) => (
                  <tr key={i}>
                    <td>{item.name}</td>
                    <td>{item.kind}</td>
                    <td>{item.quantity ?? 1}</td>
                    <td>{(item.weight ?? 0).toFixed(1)}</td>
                  </tr>
                ))}
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
