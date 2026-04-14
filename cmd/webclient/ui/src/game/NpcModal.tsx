import { useEffect, useRef, useState } from 'react'
import ReactDOM from 'react-dom'
import { useGame } from './GameContext'
import type { ShopItem, ShopView } from '../proto'

// ---------- Stat tooltip ----------

function ItemTooltip({ item, pos }: { item: ShopItem; pos: { x: number; y: number } }) {
  const kind = item.kind ?? ''
  const desc = item.description ?? ''
  const lines: Array<{ label: string; value: string }> = []

  if (kind === 'weapon') {
    const dmg = item.weaponDamage ?? item.weapon_damage ?? ''
    const dmgType = item.weaponDamageType ?? item.weapon_damage_type ?? ''
    const range = item.weaponRange ?? item.weapon_range ?? 0
    const traits = item.weaponTraits ?? item.weapon_traits ?? []
    if (dmg) lines.push({ label: 'Damage', value: dmgType ? `${dmg} ${dmgType}` : dmg })
    lines.push({ label: 'Range', value: range > 0 ? `${range} ft` : 'Melee' })
    if (traits.length > 0) lines.push({ label: 'Traits', value: traits.join(', ') })
  } else if (kind === 'armor') {
    const ac = item.armorAcBonus ?? item.armor_ac_bonus ?? 0
    const slot = item.armorSlot ?? item.armor_slot ?? ''
    const chk = item.armorCheckPenalty ?? item.armor_check_penalty ?? 0
    const spd = item.armorSpeedPenalty ?? item.armor_speed_penalty ?? 0
    const prof = item.armorProfCategory ?? item.armor_prof_category ?? ''
    if (ac !== 0) lines.push({ label: 'AC Bonus', value: `+${ac}` })
    if (slot) lines.push({ label: 'Slot', value: slot.replace(/_/g, ' ') })
    if (chk !== 0) lines.push({ label: 'Check Penalty', value: `${chk}` })
    if (spd !== 0) lines.push({ label: 'Speed Penalty', value: `${spd} ft` })
    if (prof) lines.push({ label: 'Proficiency', value: prof.replace(/_/g, ' ') })
  } else if (kind === 'consumable') {
    const effects = item.effectsSummary ?? item.effects_summary ?? ''
    if (effects) lines.push({ label: 'Effects', value: effects })
  }

  return ReactDOM.createPortal(
    <div style={{ ...styles.tooltip, left: pos.x, top: pos.y }}>
      <div style={styles.tooltipName}>{item.name ?? item.itemId ?? item.item_id}</div>
      {kind && <div style={styles.tooltipKind}>{kind}</div>}
      {desc && <p style={styles.tooltipDesc}>{desc}</p>}
      {lines.map((l) => (
        <div key={l.label} style={styles.tooltipRow}>
          <span style={styles.tooltipLabel}>{l.label}</span>
          <span style={styles.tooltipValue}>{l.value}</span>
        </div>
      ))}
      {lines.length === 0 && !desc && <div style={{ color: '#666', fontSize: '0.75rem' }}>No stats available.</div>}
    </div>,
    document.body,
  )
}

// ---------- Equipped badge helper ----------

function equippedLabel(item: ShopItem, sheet: ReturnType<typeof useGame>['state']['characterSheet']): string | null {
  if (!sheet) return null
  const name = item.name ?? ''
  const kind = item.kind ?? ''

  if (kind === 'weapon') {
    const mh = sheet.mainHand ?? sheet.main_hand ?? ''
    const oh = sheet.offHand ?? sheet.off_hand ?? ''
    if (mh && name && mh.toLowerCase().includes(name.toLowerCase())) return 'main hand'
    if (oh && name && oh.toLowerCase().includes(name.toLowerCase())) return 'off hand'
  }
  if (kind === 'armor') {
    const armor = sheet.armor ?? {}
    for (const [slot, equipped] of Object.entries(armor)) {
      if (equipped && name && equipped.toLowerCase().includes(name.toLowerCase())) {
        return slot.replace(/_/g, ' ')
      }
    }
  }
  return null
}

// ---------- Shop row ----------

function ShopRow({
  item,
  sheet,
  onBuy,
}: {
  item: ShopItem
  sheet: ReturnType<typeof useGame>['state']['characterSheet']
  onBuy: (itemId: string, qty: number) => void
}) {
  const [hovered, setHovered] = useState(false)
  const [tooltipPos, setTooltipPos] = useState({ x: 0, y: 0 })
  const tdRef = useRef<HTMLTableCellElement>(null)
  const id = item.itemId ?? item.item_id ?? ''
  const name = item.name ?? id
  const buy = item.buyPrice ?? item.buy_price ?? 0
  const sell = item.sellPrice ?? item.sell_price ?? 0
  const stock = item.stock ?? 0
  const equipped = equippedLabel(item, sheet)
  const hasStats = !!(item.kind && (item.kind === 'weapon' || item.kind === 'armor' || item.description))

  function handleMouseEnter() {
    if (tdRef.current) {
      const rect = tdRef.current.getBoundingClientRect()
      setTooltipPos({ x: rect.left, y: rect.bottom + 4 })
    }
    setHovered(true)
  }

  return (
    <tr
      style={{ ...styles.row, ...(hovered ? styles.rowHovered : {}) }}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={() => setHovered(false)}
    >
      <td ref={tdRef} style={styles.tdName}>
        <div style={styles.nameCell}>
          <span style={{ cursor: hasStats ? 'help' : 'default' }}>
            {name}
            {hasStats && <span style={styles.statsHint}>ⓘ</span>}
          </span>
          {equipped && <span style={styles.equippedBadge}>{equipped}</span>}
        </div>
        {hovered && hasStats && <ItemTooltip item={item} pos={tooltipPos} />}
      </td>
      <td style={styles.tdNum}>{buy} Crypto</td>
      <td style={styles.tdNum}>{sell} Crypto</td>
      <td style={{ ...styles.tdNum, color: stock === 0 ? '#666' : '#ccc' }}>
        {stock === 0 ? 'out' : stock}
      </td>
      <td style={styles.tdAction}>
        {stock > 0 && (
          <button
            style={styles.buyBtn}
            onClick={() => onBuy(id, 1)}
            type="button"
          >
            Buy
          </button>
        )}
      </td>
    </tr>
  )
}

// ---------- Sell row ----------

function SellRow({
  instanceId,
  name,
  quantity,
  sellPrice,
  onSell,
}: {
  instanceId: string
  name: string
  quantity: number
  sellPrice: number
  onSell: (instanceId: string, qty: number) => void
}) {
  return (
    <tr style={styles.row}>
      <td style={styles.tdName}>{name}{quantity > 1 ? ` (×${quantity})` : ''}</td>
      <td style={styles.tdNum}>{sellPrice} Crypto</td>
      <td style={styles.tdAction}>
        <button
          style={styles.sellBtn}
          onClick={() => onSell(instanceId, 1)}
          type="button"
        >
          Sell
        </button>
        {quantity > 1 && (
          <button
            style={{ ...styles.sellBtn, marginLeft: '0.25rem' }}
            onClick={() => onSell(instanceId, quantity)}
            type="button"
          >
            All
          </button>
        )}
      </td>
    </tr>
  )
}

// ---------- Modal ----------

interface ShopModalProps {
  shop: ShopView
  onClose: () => void
}

function ShopModal({ shop, onClose }: ShopModalProps) {
  const { state, sendMessage, sendCommand } = useGame()
  const [tab, setTab] = useState<'buy' | 'sell'>('buy')
  const npcName = shop.npcName ?? shop.npc_name ?? 'Merchant'

  // Ensure inventory is loaded for the sell tab
  useEffect(() => {
    if (!state.inventoryView) {
      sendMessage('InventoryRequest', {})
    }
  }, [state.inventoryView, sendMessage])
  const items = shop.items ?? []
  const currency = state.characterSheet?.currency ?? state.inventoryView?.currency ?? null
  const invItems = state.inventoryView?.items ?? []

  // Build a sell-price lookup from shop inventory (item def ID → sell price)
  const sellPriceById = new Map<string, number>()
  for (const si of items) {
    const id = si.itemId ?? si.item_id ?? ''
    const price = si.sellPrice ?? si.sell_price ?? 0
    if (id && price > 0) sellPriceById.set(id, price)
  }

  // Player inventory items that this merchant will buy (matching item def ID, sell price > 0)
  const sellableItems = invItems.filter((inv) => {
    const defId = inv.itemDefId ?? inv.item_def_id ?? ''
    return defId !== '' && sellPriceById.has(defId)
  })

  function handleBuy(itemId: string, quantity: number) {
    sendMessage('BuyRequest', { npc_name: npcName, item_id: itemId, quantity })
    // Force inventory refresh: the server pushes InventoryView after a buy, but
    // PushBlocking can fail under load. An explicit InventoryRequest guarantees
    // the client sees the updated backpack even if the push was dropped.
    sendMessage('InventoryRequest', {})
  }

  function handleSell(itemDefId: string, quantity: number) {
    sendMessage('SellRequest', { npc_name: npcName, item_id: itemDefId, quantity })
  }

  function handleNegotiate() {
    sendCommand(`negotiate ${npcName}`)
    onClose()
  }

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{npcName}'s Wares</h3>
            {currency && <span style={styles.currency}>{currency}</span>}
          </div>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.tabs}>
          <button
            style={{ ...styles.tab, ...(tab === 'buy' ? styles.tabActive : {}) }}
            onClick={() => setTab('buy')}
            type="button"
          >
            Buy
          </button>
          <button
            style={{ ...styles.tab, ...(tab === 'sell' ? styles.tabActive : {}) }}
            onClick={() => setTab('sell')}
            type="button"
          >
            Sell {sellableItems.length > 0 ? `(${sellableItems.length})` : ''}
          </button>
        </div>
        <div style={styles.body}>
          {tab === 'buy' ? (
            <>
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
                    {items.map((item, i) => (
                      <ShopRow
                        key={item.itemId ?? item.item_id ?? i}
                        item={item}
                        sheet={state.characterSheet}
                        onBuy={handleBuy}
                      />
                    ))}
                  </tbody>
                </table>
              )}
              <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem', flexWrap: 'wrap' }}>
                <button style={styles.negotiateBtn} onClick={handleNegotiate} type="button">
                  Negotiate
                </button>
              </div>
            </>
          ) : (
            <>
              {sellableItems.length === 0 ? (
                <p style={{ color: '#666' }}>
                  {invItems.length === 0
                    ? 'Your inventory is empty.'
                    : 'This merchant is not buying any items you carry.'}
                </p>
              ) : (
                <table style={styles.table}>
                  <thead>
                    <tr>
                      <th style={{ ...styles.th, textAlign: 'left' }}>Item</th>
                      <th style={styles.th}>Price</th>
                      <th style={styles.th}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {sellableItems.map((inv, i) => {
                      const defId = inv.itemDefId ?? inv.item_def_id ?? ''
                      const price = sellPriceById.get(defId) ?? 0
                      return (
                        <SellRow
                          key={inv.instanceId ?? i}
                          instanceId={defId}
                          name={inv.name ?? ''}
                          quantity={inv.quantity ?? 1}
                          sellPrice={price}
                          onSell={handleSell}
                        />
                      )
                    })}
                  </tbody>
                </table>
              )}
            </>
          )}
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
    width: 'min(640px, 95vw)',
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
  headerLeft: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.75rem',
  },
  title: {
    margin: 0,
    color: '#e0c060',
    fontSize: '1rem',
    fontFamily: 'monospace',
  },
  currency: {
    color: '#7bc',
    fontSize: '0.8rem',
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
  rowHovered: {
    background: '#1a1a1a',
  },
  nameCell: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.4rem',
    flexWrap: 'wrap' as const,
    position: 'relative' as const,
  },
  statsHint: {
    color: '#555',
    fontSize: '0.7rem',
    marginLeft: '3px',
    cursor: 'help',
  },
  equippedBadge: {
    fontSize: '0.62rem',
    padding: '0.08rem 0.35rem',
    borderRadius: '3px',
    background: '#1a2a3a',
    border: '1px solid #2a4a6a',
    color: '#7bc',
    whiteSpace: 'nowrap' as const,
  },
  tdName: {
    color: '#ddd',
    padding: '0.35rem 0.5rem',
    textAlign: 'left' as const,
    position: 'relative' as const,
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
  tabs: {
    display: 'flex',
    borderBottom: '1px solid #2a2a2a',
    flexShrink: 0,
  },
  tab: {
    padding: '0.4rem 1rem',
    background: 'none',
    border: 'none',
    borderBottom: '2px solid transparent',
    color: '#666',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.8rem',
  },
  tabActive: {
    color: '#e0c060',
    borderBottom: '2px solid #e0c060',
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
  sellBtn: {
    padding: '0.15rem 0.5rem',
    background: '#2a1a1a',
    border: '1px solid #6a3a2a',
    color: '#e84',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  negotiateBtn: {
    padding: '0.25rem 0.75rem',
    background: '#1a1a2a',
    border: '1px solid #3a3a8a',
    color: '#88c',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.8rem',
  },
  // Tooltip
  tooltip: {
    position: 'fixed' as const,
    zIndex: 1000,
    background: '#1a1a1a',
    border: '1px solid #444',
    borderRadius: '5px',
    padding: '0.5rem 0.7rem',
    minWidth: '200px',
    maxWidth: '280px',
    boxShadow: '0 4px 12px rgba(0,0,0,0.6)',
    pointerEvents: 'none' as const,
  },
  tooltipName: {
    color: '#e0c060',
    fontSize: '0.85rem',
    fontWeight: 'bold' as const,
    marginBottom: '0.1rem',
    fontFamily: 'monospace',
  },
  tooltipKind: {
    color: '#666',
    fontSize: '0.65rem',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.08em',
    marginBottom: '0.3rem',
  },
  tooltipDesc: {
    color: '#999',
    fontSize: '0.75rem',
    margin: '0 0 0.4rem',
    lineHeight: 1.4,
  },
  tooltipRow: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: '1rem',
    fontSize: '0.78rem',
    padding: '0.1rem 0',
    fontFamily: 'monospace',
  },
  tooltipLabel: {
    color: '#7af',
  },
  tooltipValue: {
    color: '#ccc',
    textAlign: 'right' as const,
  },
}
