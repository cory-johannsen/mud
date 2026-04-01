# Web UI — Equipment Tab Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Unequip buttons to occupied slots in the web UI Equipment tab so players can remove equipped items without typing commands.

**Architecture:** `EquipSlot` gains an optional `onUnequip` callback prop; when the slot is occupied and a callback is provided the button renders. `EquipmentDrawer` defines a single `handleUnequip(slot)` helper that sends `CommandText`→`unequip <slot>`, then `CharacterSheetRequest` and `InventoryRequest` to refresh both tabs. No backend changes are needed.

**Tech Stack:** React 18, TypeScript, Vitest, @testing-library/react

---

## File Map

| Action | File |
|--------|------|
| Modify | `cmd/webclient/ui/src/game/drawers/EquipmentDrawer.tsx` |
| Create | `cmd/webclient/ui/src/game/drawers/EquipmentDrawer.test.tsx` |

---

### Task 1: Write failing tests

**Files:**
- Create: `cmd/webclient/ui/src/game/drawers/EquipmentDrawer.test.tsx`

- [ ] **Step 1: Create the test file**

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { EquipSlot, EquipmentDrawer } from './EquipmentDrawer'
import type { CharacterSheetView } from '../../proto'

// ── EquipSlot unit tests (no context needed) ────────────────────────────────

describe('EquipSlot', () => {
  it('renders the label', () => {
    render(<EquipSlot label="Main Hand" value={null} />)
    expect(screen.getByText('Main Hand')).toBeDefined()
  })

  it('renders em dash when value is null', () => {
    render(<EquipSlot label="Main Hand" value={null} />)
    expect(screen.getByText('—')).toBeDefined()
  })

  it('renders the item name when value is set', () => {
    render(<EquipSlot label="Main Hand" value="Iron Pipe" />)
    expect(screen.getByText(/Iron Pipe/)).toBeDefined()
  })

  it('does not render Unequip button when slot is empty', () => {
    const onUnequip = vi.fn()
    render(<EquipSlot label="Main Hand" value={null} onUnequip={onUnequip} />)
    expect(screen.queryByRole('button', { name: 'Unequip' })).toBeNull()
  })

  it('does not render Unequip button when onUnequip is not provided', () => {
    render(<EquipSlot label="Main Hand" value="Iron Pipe" />)
    expect(screen.queryByRole('button', { name: 'Unequip' })).toBeNull()
  })

  it('renders Unequip button when slot is occupied and onUnequip is provided', () => {
    const onUnequip = vi.fn()
    render(<EquipSlot label="Main Hand" value="Iron Pipe" onUnequip={onUnequip} />)
    expect(screen.getByRole('button', { name: 'Unequip' })).toBeDefined()
  })

  it('calls onUnequip when Unequip is clicked', () => {
    const onUnequip = vi.fn()
    render(<EquipSlot label="Main Hand" value="Iron Pipe" onUnequip={onUnequip} />)
    fireEvent.click(screen.getByRole('button', { name: 'Unequip' }))
    expect(onUnequip).toHaveBeenCalledOnce()
  })

  it('renders bonus and damage alongside item name', () => {
    render(<EquipSlot label="Main Hand" value="Iron Pipe" bonus="+3" dmg="1d6+2" />)
    expect(screen.getByText(/Iron Pipe.*\+3.*1d6\+2/)).toBeDefined()
  })
})

// ── EquipmentDrawer integration tests ───────────────────────────────────────

const mockSend = vi.fn()

function makeSheet(overrides: Partial<CharacterSheetView> = {}): CharacterSheetView {
  return {
    main_hand: '',
    mainHand: '',
    off_hand: '',
    offHand: '',
    main_hand_attack_bonus: '',
    mainHandAttackBonus: '',
    main_hand_damage: '',
    mainHandDamage: '',
    off_hand_attack_bonus: '',
    offHandAttackBonus: '',
    off_hand_damage: '',
    offHandDamage: '',
    armor: {},
    accessories: {},
    ...overrides,
  } as CharacterSheetView
}

vi.mock('../GameContext', () => ({
  useGame: () => ({
    state: { characterSheet: makeSheet() },
    sendMessage: mockSend,
  }),
}))

describe('EquipmentDrawer — unequip dispatch', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sends unequip main, CharacterSheetRequest, InventoryRequest on main-hand unequip', () => {
    vi.mocked(vi.importMock('../GameContext')).useGame = () => ({
      state: { characterSheet: makeSheet({ main_hand: 'Iron Pipe', mainHand: 'Iron Pipe' }) },
      sendMessage: mockSend,
    })
    render(<EquipmentDrawer onClose={vi.fn()} />)
    const btns = screen.getAllByRole('button', { name: 'Unequip' })
    fireEvent.click(btns[0])
    expect(mockSend).toHaveBeenCalledWith('CommandText', { text: 'unequip main' })
    expect(mockSend).toHaveBeenCalledWith('CharacterSheetRequest', {})
    expect(mockSend).toHaveBeenCalledWith('InventoryRequest', {})
  })
})
```

> **Note:** The `EquipmentDrawer` mock approach above uses `vi.mock` at the module level. The drawer integration test block is intentionally minimal — the full unequip dispatch behaviour for all slot types is verified through the `EquipSlot` unit tests plus one representative drawer integration test (main hand). The slot wiring for armor and accessory slots follows identical code paths and does not require exhaustive slot-by-slot tests.

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
cd cmd/webclient/ui && npx vitest run src/game/drawers/EquipmentDrawer.test.tsx 2>&1 | tail -20
```

Expected: Tests fail — `EquipSlot` is not exported and has no `onUnequip` prop.

---

### Task 2: Extend EquipSlot with onUnequip prop

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/EquipmentDrawer.tsx` — `EquipSlot` function and `styles` const

- [ ] **Step 3: Export EquipSlot and add the `onUnequip` prop**

Replace the existing `EquipSlot` function (lines 4–17) with:

```tsx
export function EquipSlot({
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
        <div className="equip-slot-value" style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <span>
            {value}
            {bonus ? ` (${bonus})` : ''}
            {dmg ? ` [${dmg}]` : ''}
          </span>
          {onUnequip && (
            <button style={styles.unequipBtn} onClick={onUnequip} type="button">
              Unequip
            </button>
          )}
        </div>
      ) : (
        <div className="equip-slot-value equip-empty">—</div>
      )}
    </div>
  )
}
```

- [ ] **Step 4: Add the `unequipBtn` style**

At the bottom of the file, add a `styles` constant (the file currently has none — add before the final export or at the end of the file):

```tsx
const styles: Record<string, React.CSSProperties> = {
  unequipBtn: {
    background: '#a44',
    color: '#fff',
    border: 'none',
    cursor: 'pointer',
    padding: '2px 6px',
    fontSize: '0.75em',
    flexShrink: 0,
  },
}
```

- [ ] **Step 5: Run EquipSlot unit tests only to confirm they pass**

```bash
cd cmd/webclient/ui && npx vitest run src/game/drawers/EquipmentDrawer.test.tsx -t "EquipSlot" 2>&1 | tail -20
```

Expected: All `EquipSlot` tests pass.

- [ ] **Step 6: Commit the EquipSlot changes**

```bash
git add cmd/webclient/ui/src/game/drawers/EquipmentDrawer.tsx \
        cmd/webclient/ui/src/game/drawers/EquipmentDrawer.test.tsx
git commit -m "feat(web-ui): export EquipSlot with optional onUnequip button prop"
```

---

### Task 3: Wire unequip callbacks into EquipmentDrawer

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/EquipmentDrawer.tsx` — `EquipmentDrawer` function body

- [ ] **Step 7: Add the `handleUnequip` helper and pass callbacks to all slots**

Replace the `EquipmentDrawer` function body (starting at line 44, the current export) with:

```tsx
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

  const handleUnequip = (slot: string) => {
    sendMessage('CommandText', { text: `unequip ${slot}` })
    sendMessage('CharacterSheetRequest', {})
    sendMessage('InventoryRequest', {})
  }

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
              onUnequip={(sheet.mainHand ?? sheet.main_hand) ? () => handleUnequip('main') : undefined}
            />
            <EquipSlot
              label="Off Hand"
              value={sheet.offHand ?? sheet.off_hand}
              onUnequip={(sheet.offHand ?? sheet.off_hand) ? () => handleUnequip('off') : undefined}
            />
            {ARMOR_SLOTS.map(({ key, label }) => (
              <EquipSlot
                key={key}
                label={label}
                value={armor[key] || null}
                onUnequip={armor[key] ? () => handleUnequip(key) : undefined}
              />
            ))}
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
```

- [ ] **Step 8: Run the full test suite**

```bash
cd cmd/webclient/ui && npm test 2>&1 | tail -20
```

Expected: All tests pass, 0 failures.

- [ ] **Step 9: Commit**

```bash
git add cmd/webclient/ui/src/game/drawers/EquipmentDrawer.tsx
git commit -m "feat(web-ui): add Unequip buttons to occupied Equipment tab slots

Wires onUnequip callbacks for all slot categories (weapon/armor/accessory).
Clicking Unequip sends CommandText 'unequip <slot>' then refreshes both
CharacterSheetView and InventoryView.

Closes web-ui-equipment-controls."
```

---

## Self-Review

**Spec coverage:**

| Requirement | Covered by |
|-------------|-----------|
| REQ-WEC-1: Unequip button on occupied slots only | Task 2 Step 3 (`onUnequip` only passed when value is truthy); EquipSlot tests cover both cases |
| REQ-WEC-2: All slot categories (main, off, 8 armor, 11 accessory) | Task 3 Step 7 — all three slot groups wired |
| REQ-WEC-3: `CommandText` with `unequip <slot>` | Task 3 Step 7 `handleUnequip`; drawer integration test |
| REQ-WEC-4: `CharacterSheetRequest` after unequip | Task 3 Step 7 `handleUnequip`; drawer integration test |
| REQ-WEC-5: `InventoryRequest` after unequip | Task 3 Step 7 `handleUnequip`; drawer integration test |
| REQ-WEC-6: Cursed item rejection handled server-side only | No client logic added — confirmed |
| REQ-WEC-7: No new proto or backend changes | Only `EquipmentDrawer.tsx` modified — confirmed |

**Placeholder scan:** No TBDs, no "add appropriate error handling", no "similar to Task N" — all steps contain complete code.

**Type consistency:** `EquipSlot` exported in Task 2 Step 3, imported in test Task 1 Step 1, used with `onUnequip` in Task 3 Step 7 — all consistent. `handleUnequip(slot: string)` defined and called with string literals throughout.
