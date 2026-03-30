import { useGame } from './GameContext'
import type { ShopView } from '../proto'

interface ShopModalProps {
  shop: ShopView
  onClose: () => void
}

function ShopModal({ shop, onClose }: ShopModalProps) {
  const { sendMessage } = useGame()
  const npcName = shop.npcName ?? shop.npc_name ?? 'Merchant'
  const items = shop.items ?? []

  function handleBuy(itemId: string, quantity: number) {
    sendMessage('BuyRequest', { npc_name: npcName, item_id: itemId, quantity })
    // Don't close — let player keep shopping
  }

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <h3 style={styles.title}>{npcName}'s Wares</h3>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {items.length === 0 ? (
            <p style={{ color: '#666' }}>No items in stock.</p>
          ) : (
            <table style={styles.table}>
              <thead>
                <tr>
                  <th style={{ ...styles.th, textAlign: 'left' }}>Item</th>
                  <th style={styles.th}>Buy</th>
                  <th style={styles.th}>Sell</th>
                  <th style={styles.th}>Stock</th>
                  <th style={styles.th}></th>
                </tr>
              </thead>
              <tbody>
                {items.map((item, i) => {
                  const id = item.itemId ?? item.item_id ?? ''
                  const name = item.name ?? id
                  const buy = item.buyPrice ?? item.buy_price ?? 0
                  const sell = item.sellPrice ?? item.sell_price ?? 0
                  const stock = item.stock ?? 0
                  return (
                    <tr key={i} style={styles.row}>
                      <td style={styles.tdName}>{name}</td>
                      <td style={styles.tdNum}>{buy}¢</td>
                      <td style={styles.tdNum}>{sell}¢</td>
                      <td style={{ ...styles.tdNum, color: stock === 0 ? '#666' : '#ccc' }}>
                        {stock === 0 ? 'out' : stock}
                      </td>
                      <td style={styles.tdAction}>
                        {stock > 0 && (
                          <button
                            style={styles.buyBtn}
                            onClick={() => handleBuy(id, 1)}
                            type="button"
                          >
                            Buy
                          </button>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
          <p style={styles.hint}>Type <code>sell &lt;item&gt;</code> to sell items to this merchant.</p>
        </div>
      </div>
    </div>
  )
}

export function NpcModal() {
  const { state, clearShop } = useGame()
  if (!state.shopView) return null
  return <ShopModal shop={state.shopView} onClose={clearShop} />
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.75)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 200,
  },
  modal: {
    background: '#111',
    border: '1px solid #333',
    borderRadius: '6px',
    width: 'min(600px, 95vw)',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.6rem 1rem',
    borderBottom: '1px solid #2a2a2a',
    flexShrink: 0,
  },
  title: {
    margin: 0,
    color: '#e0c060',
    fontSize: '1rem',
    fontFamily: 'monospace',
  },
  closeBtn: {
    background: 'none',
    border: '1px solid #444',
    color: '#888',
    cursor: 'pointer',
    fontFamily: 'monospace',
    borderRadius: '3px',
    padding: '0.1rem 0.4rem',
  },
  body: {
    padding: '0.75rem 1rem',
    overflowY: 'auto',
  },
  table: {
    width: '100%',
    borderCollapse: 'collapse',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
  },
  th: {
    color: '#7af',
    fontSize: '0.72rem',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.06em',
    padding: '0.3rem 0.5rem',
    borderBottom: '1px solid #2a2a2a',
    textAlign: 'right' as const,
  },
  row: {
    borderBottom: '1px solid #1a1a1a',
  },
  tdName: {
    color: '#ddd',
    padding: '0.35rem 0.5rem',
    textAlign: 'left' as const,
  },
  tdNum: {
    color: '#aaa',
    padding: '0.35rem 0.5rem',
    textAlign: 'right' as const,
    whiteSpace: 'nowrap' as const,
  },
  tdAction: {
    padding: '0.25rem 0.5rem',
    textAlign: 'center' as const,
  },
  buyBtn: {
    padding: '0.15rem 0.5rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  hint: {
    marginTop: '0.75rem',
    color: '#555',
    fontSize: '0.75rem',
    fontFamily: 'monospace',
  },
}
