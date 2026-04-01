# Web UI — Inventory Consume Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Consume button to consumable items in the web UI Inventory tab that sends `use <item_def_id>` and refreshes the inventory, with no backend changes required.

**Architecture:** A new `ConsumableRow` component (mirroring the existing `WeaponRow`/`ArmorRow` pattern) is added to `InventoryDrawer.tsx`. The render map gains a `kind === 'consumable'` branch. On click, `CommandText` is sent (reusing the server's existing `use` pathway), followed by `InventoryRequest` to refresh.

**Tech Stack:** React 18, TypeScript, Vitest, @testing-library/react

---

## File Map

| Action | File |
|--------|------|
| Modify | `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx` |
| Create | `cmd/webclient/ui/src/game/drawers/InventoryDrawer.test.tsx` |

---

### Task 1: Write failing tests for ConsumableRow

**Files:**
- Create: `cmd/webclient/ui/src/game/drawers/InventoryDrawer.test.tsx`

- [ ] **Step 1: Create the test file**

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ConsumableRow } from './InventoryDrawer'
import type { InventoryItem } from '../../proto'

function makeItem(overrides: Partial<InventoryItem> = {}): InventoryItem {
  return {
    name: 'Stim Pack',
    kind: 'consumable',
    quantity: 3,
    weight: 0.2,
    item_def_id: 'stim_pack',
    itemDefId: 'stim_pack',
    instance_id: 'inst-1',
    armor_slot: '',
    armorSlot: '',
    ...overrides,
  } as InventoryItem
}

describe('ConsumableRow', () => {
  it('renders name, kind, quantity, and weight', () => {
    const send = vi.fn()
    render(
      <table><tbody>
        <ConsumableRow item={makeItem()} sendMessage={send} />
      </tbody></table>
    )
    expect(screen.getByText('Stim Pack')).toBeDefined()
    expect(screen.getByText('consumable')).toBeDefined()
    expect(screen.getByText('3')).toBeDefined()
    expect(screen.getByText('0.2')).toBeDefined()
  })

  it('renders a Consume button', () => {
    const send = vi.fn()
    render(
      <table><tbody>
        <ConsumableRow item={makeItem()} sendMessage={send} />
      </tbody></table>
    )
    expect(screen.getByRole('button', { name: 'Consume' })).toBeDefined()
  })

  it('Consume button is enabled when quantity > 0', () => {
    const send = vi.fn()
    render(
      <table><tbody>
        <ConsumableRow item={makeItem({ quantity: 1 })} sendMessage={send} />
      </tbody></table>
    )
    const btn = screen.getByRole('button', { name: 'Consume' }) as HTMLButtonElement
    expect(btn.disabled).toBe(false)
  })

  it('Consume button is disabled when quantity is 0', () => {
    const send = vi.fn()
    render(
      <table><tbody>
        <ConsumableRow item={makeItem({ quantity: 0 })} sendMessage={send} />
      </tbody></table>
    )
    const btn = screen.getByRole('button', { name: 'Consume' }) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
  })

  it('clicking Consume sends CommandText with use <item_def_id>', () => {
    const send = vi.fn()
    render(
      <table><tbody>
        <ConsumableRow item={makeItem()} sendMessage={send} />
      </tbody></table>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Consume' }))
    expect(send).toHaveBeenCalledWith('CommandText', { text: 'use stim_pack' })
  })

  it('clicking Consume sends InventoryRequest to refresh', () => {
    const send = vi.fn()
    render(
      <table><tbody>
        <ConsumableRow item={makeItem()} sendMessage={send} />
      </tbody></table>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Consume' }))
    expect(send).toHaveBeenCalledWith('InventoryRequest', {})
  })

  it('clicking Consume sends CommandText before InventoryRequest', () => {
    const send = vi.fn()
    render(
      <table><tbody>
        <ConsumableRow item={makeItem()} sendMessage={send} />
      </tbody></table>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Consume' }))
    const calls = send.mock.calls.map((c) => c[0])
    expect(calls[0]).toBe('CommandText')
    expect(calls[1]).toBe('InventoryRequest')
  })

  it('uses itemDefId camelCase when item_def_id is absent', () => {
    const send = vi.fn()
    const item = makeItem({ item_def_id: undefined, itemDefId: 'nano_inject' })
    render(
      <table><tbody>
        <ConsumableRow item={item} sendMessage={send} />
      </tbody></table>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Consume' }))
    expect(send).toHaveBeenCalledWith('CommandText', { text: 'use nano_inject' })
  })
})
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
cd cmd/webclient/ui && npx vitest run src/game/drawers/InventoryDrawer.test.tsx 2>&1 | tail -20
```

Expected: Tests fail with `ConsumableRow` not exported / not found.

---

### Task 2: Implement ConsumableRow and wire it into the drawer

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx`

- [ ] **Step 3: Export ConsumableRow — add it after the `ArmorRow` function and before `PlainRow` (around line 103)**

Add this block between `ArmorRow` and `PlainRow`:

```tsx
export function ConsumableRow({
  item,
  sendMessage,
}: {
  item: InventoryItem
  sendMessage: (type: string, payload: object) => void
}) {
  const itemDefId = item.itemDefId ?? item.item_def_id ?? ''
  const qty = item.quantity ?? 1
  const disabled = qty <= 0

  return (
    <tr>
      <td>{item.name}</td>
      <td>{item.kind}</td>
      <td>{qty}</td>
      <td>{(item.weight ?? 0).toFixed(1)}</td>
      <td>
        <button
          style={{ ...styles.actionBtn, ...(disabled ? styles.actionBtnDisabled : {}) }}
          disabled={disabled}
          onClick={() => {
            sendMessage('CommandText', { text: `use ${itemDefId}` })
            sendMessage('InventoryRequest', {})
          }}
          type="button"
        >
          Consume
        </button>
      </td>
    </tr>
  )
}
```

- [ ] **Step 4: Add the consumable branch to the item render map in `InventoryDrawer`**

Inside `InventoryDrawer`, find this block (around line 149–161):

```tsx
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
```

Replace with:

```tsx
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
  if (item.kind === 'consumable') {
    return (
      <ConsumableRow key={i} item={item} sendMessage={sendMessage} />
    )
  }
  return <PlainRow key={i} item={item} />
})}
```

- [ ] **Step 5: Run the tests to confirm they all pass**

```bash
cd cmd/webclient/ui && npx vitest run src/game/drawers/InventoryDrawer.test.tsx 2>&1 | tail -20
```

Expected output: All 8 tests pass, 0 failures.

- [ ] **Step 6: Run the full frontend test suite to confirm no regressions**

```bash
cd cmd/webclient/ui && npm test 2>&1 | tail -20
```

Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx \
        cmd/webclient/ui/src/game/drawers/InventoryDrawer.test.tsx
git commit -m "feat(web-ui): add Consume button for consumable items in Inventory tab

Adds ConsumableRow component to InventoryDrawer. Clicking Consume sends
'use <item_def_id>' via CommandText (existing server pathway) then
refreshes inventory via InventoryRequest. Button is disabled at qty=0.

Closes web-ui-inventory-consume."
```

---

## Self-Review

**Spec coverage:**

| Requirement | Covered by |
|-------------|-----------|
| REQ-WIC-1: ConsumableRow for kind=consumable | Task 2, Step 4 |
| REQ-WIC-2: Consume button, enabled at qty>0, disabled at qty=0 | Task 2, Step 3; tests in Task 1 |
| REQ-WIC-3: CommandText with `use <item_def_id>` | Task 2, Step 3; tests in Task 1 |
| REQ-WIC-4: InventoryRequest refresh after consume | Task 2, Step 3; tests in Task 1 |
| REQ-WIC-5: Qty displayed in Qty column | Task 2, Step 3; test in Task 1 |
| REQ-WIC-6: No new proto or backend changes | Confirmed — only InventoryDrawer.tsx modified |

**Placeholder scan:** None found. All steps contain complete code.

**Type consistency:** `ConsumableRow` defined in Task 2 Step 3, used in Task 2 Step 4, imported in test Task 1 Step 1 — all consistent.
