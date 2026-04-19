---
issue: 192
title: Multi-row hotbar with hotkey switching
slug: multi-hotbar
date: 2026-04-19
---

## Summary

Allow players to create and switch between up to 4 independent hotbars (each
with 10 slots). The default remains a single hotbar. New hotbars are created on
demand. Switching is done via UI controls and Ctrl+Up / Ctrl+Down hotkeys.
Maximum hotbar count is server-side configurable.

---

## Current Architecture

| Layer | Current state |
|---|---|
| Domain | `PlayerSession.Hotbar [10]HotbarSlot` (single bar) |
| DB | `characters.hotbar TEXT` (JSON array of 10 `{kind,ref}` objects) |
| Proto | `HotbarUpdateEvent { repeated HotbarSlot slots }` (10 entries) |
| Frontend | `state.hotbarSlots: HotbarSlot[]` (10 elements); `HotbarPanel.tsx` renders one row |

---

## Requirements

### REQ-MHB-1: Server Configuration
A `HotbarConfig` struct MUST be added to `internal/config/config.go`:

```go
type HotbarConfig struct {
    MaxHotbars int `mapstructure:"max_hotbars"`
}
```

- REQ-MHB-1a: Default value for `max_hotbars` MUST be `4` via `setDefaults()`.
- REQ-MHB-1b: `Config.Hotbar HotbarConfig` MUST be added to the root `Config` struct.
- REQ-MHB-1c: `GameServiceServer` MUST receive and store `maxHotbars int` at
  construction time, sourced from `Config.Hotbar.MaxHotbars`.

### REQ-MHB-2: Domain Model
`PlayerSession` MUST be updated to support multiple hotbars:

- REQ-MHB-2a: Replace `Hotbar [10]HotbarSlot` with `Hotbars [][10]HotbarSlot`.
- REQ-MHB-2b: Add `ActiveHotbarIndex int` to `PlayerSession`.
- REQ-MHB-2c: `Hotbars` MUST always have at least 1 element (the initial empty
  bar). It MUST NOT exceed `maxHotbars` elements.
- REQ-MHB-2d: All existing code that referenced `sess.Hotbar` MUST be updated
  to `sess.Hotbars[sess.ActiveHotbarIndex]`.

### REQ-MHB-3: Database Schema
A new migration MUST add two columns to the `characters` table:

```sql
ALTER TABLE characters
  ADD COLUMN IF NOT EXISTS hotbars          TEXT,
  ADD COLUMN IF NOT EXISTS active_hotbar_idx INTEGER NOT NULL DEFAULT 0;
```

- REQ-MHB-3a: The existing `hotbar TEXT` column MUST be left untouched (backward
  compat; read-only migration source).
- REQ-MHB-3b: `hotbars` stores a JSON array of bar arrays:
  `[[{kind,ref},...], [{kind,ref},...]]` — outer array is bars, inner is 10 slots.
- REQ-MHB-3c: All-empty bars MUST be stored as NULL (optimised, matching current
  single-bar behaviour).

### REQ-MHB-4: Persistence — Load
`LoadHotbar` in `internal/storage/postgres/character_hotbar.go` MUST be
replaced by `LoadHotbars`:

```go
func (r *CharacterRepository) LoadHotbars(ctx context.Context, characterID int64) ([][10]HotbarSlot, int, error)
```

- REQ-MHB-4a: If `hotbars` column is non-null, unmarshal and return it along
  with `active_hotbar_idx`.
- REQ-MHB-4b: If `hotbars` is null but legacy `hotbar` is non-null, migrate:
  unmarshal `hotbar` as bar 0, return `[][10]HotbarSlot{bar0}` with index `0`.
- REQ-MHB-4c: If both are null, return one empty bar with index `0`.
- REQ-MHB-4d: `MarshalHotbars([][10]HotbarSlot) ([]byte, error)` and
  `UnmarshalHotbars([]byte) ([][10]HotbarSlot, error)` helper functions MUST be
  added.

### REQ-MHB-5: Persistence — Save
`SaveHotbar` MUST be replaced by `SaveHotbars`:

```go
func (r *CharacterRepository) SaveHotbars(ctx context.Context, characterID int64, bars [][10]HotbarSlot, activeIdx int) error
```

- REQ-MHB-5a: Updates both `hotbars` and `active_hotbar_idx` in one statement.
- REQ-MHB-5b: If all bars are empty, stores NULL for `hotbars`.

### REQ-MHB-6: Proto Changes
`api/proto/game/v1/game.proto` MUST be updated:

```protobuf
message HotbarRequest {
  // existing fields unchanged (1–8)
  int32  hotbar_index = 9;  // 1-based target bar; 0 = current active
}

message HotbarUpdateEvent {
  repeated HotbarSlot slots             = 1;  // slots for active bar (unchanged)
  int32               active_hotbar_index = 2; // 1-based
  int32               hotbar_count        = 3; // total bars created
  int32               max_hotbars         = 4; // server-configured max
}
```

- REQ-MHB-6a: `HotbarRequest.hotbar_index = 0` (default) MUST mean "current
  active hotbar" — no behaviour change for existing clients.
- REQ-MHB-6b: `HotbarUpdateEvent` MUST always include `active_hotbar_index`,
  `hotbar_count`, and `max_hotbars` so the UI can render controls correctly.

### REQ-MHB-7: Server — New Actions
`handleHotbar` in `internal/gameserver/grpc_service_hotbar.go` MUST handle two
new `action` values:

#### "create"
- REQ-MHB-7a: If `len(sess.Hotbars) >= maxHotbars`, return a `MessageEvent`:
  `"Hotbar limit reached (max N)."` — do not create.
- REQ-MHB-7b: Otherwise, append a new empty `[10]HotbarSlot` to `sess.Hotbars`,
  set `sess.ActiveHotbarIndex` to the new bar's index (0-based), persist via
  `SaveHotbars`, send `HotbarUpdateEvent`.

#### "switch"
- REQ-MHB-7c: `HotbarRequest.hotbar_index` is the 1-based target bar.
- REQ-MHB-7d: If the target index is out of range, return a `MessageEvent`:
  `"Invalid hotbar index."` — do not switch.
- REQ-MHB-7e: Set `sess.ActiveHotbarIndex = hotbar_index - 1`, persist, send
  `HotbarUpdateEvent`.

### REQ-MHB-8: Server — Existing Actions
Existing `set`, `clear`, and `show` actions MUST operate on
`sess.Hotbars[sess.ActiveHotbarIndex]` without requiring a `hotbar_index`
override. If `hotbar_index != 0` is provided in a `set`/`clear` request, the
server MAY ignore it (out of scope for this feature).

### REQ-MHB-9: Frontend — State
`GameContext.tsx` MUST be updated:

- REQ-MHB-9a: Add `activeHotbarIndex: number` (1-based), `hotbarCount: number`,
  `maxHotbars: number` to the game state interface.
- REQ-MHB-9b: `SET_HOTBAR` reducer MUST extract and store these three fields
  from `HotbarUpdateEvent` in addition to the existing `hotbarSlots`.
- REQ-MHB-9c: Default values: `activeHotbarIndex: 1`, `hotbarCount: 1`,
  `maxHotbars: 4`.

### REQ-MHB-10: Frontend — HotbarPanel Controls
`HotbarPanel.tsx` MUST be updated:

- REQ-MHB-10a: **Numeric indicator** — render `<activeHotbarIndex>/<hotbarCount>`
  as a non-interactive label at the far left of the hotbar row.
- REQ-MHB-10b: **▲ (up) control** — clicking sends
  `HotbarRequest{action: "switch", hotbar_index: activeHotbarIndex - 1}`.
  Wraps: if `activeHotbarIndex == 1`, switch to `hotbarCount`.
  Disabled (greyed) if `hotbarCount == 1`.
- REQ-MHB-10c: **▼ (down) control** — clicking sends
  `HotbarRequest{action: "switch", hotbar_index: activeHotbarIndex + 1}`.
  Wraps: if `activeHotbarIndex == hotbarCount`, switch to `1`.
  Disabled (greyed) if `hotbarCount == 1`.
- REQ-MHB-10d: **"+ New Hotbar" button** — displayed at the far right of the
  hotbar row. Clicking sends `HotbarRequest{action: "create"}`. Hidden when
  `hotbarCount >= maxHotbars`.
- REQ-MHB-10e: Control layout order (left to right): `▲` `▼` `<N>/<total>` `[slot 1]` … `[slot 10]` `[+ New Hotbar]`.

### REQ-MHB-11: Frontend — Keyboard Shortcuts
- REQ-MHB-11a: **Ctrl+Up** MUST trigger the same action as clicking ▲.
- REQ-MHB-11b: **Ctrl+Down** MUST trigger the same action as clicking ▼.
- REQ-MHB-11c: These key events MUST be captured globally (not scoped to hotbar
  focus) and MUST NOT propagate to the prompt or console.
- REQ-MHB-11d: If `hotbarCount == 1`, Ctrl+Up and Ctrl+Down MUST be no-ops.

### REQ-MHB-12: Frontend — HotbarSlotPicker
`HotbarSlotPicker.tsx` requires no changes — it always assigns to the current
active hotbar, which is server-side state.

### REQ-MHB-13: Backward Compatibility
- REQ-MHB-13a: A character with no `hotbars` DB data MUST load with exactly one
  hotbar (migrated from `hotbar` if present, otherwise empty).
- REQ-MHB-13b: Existing `HotbarRequest` messages with no `hotbar_index` (default
  `0`) MUST continue to behave identically to the current single-hotbar behaviour.
- REQ-MHB-13c: Existing `HotbarUpdateEvent` consumers that ignore the new fields
  MUST continue to function (proto field additions are backward-compatible).

### REQ-MHB-14: Test Coverage
- REQ-MHB-14a: Unit test for `LoadHotbars` with null `hotbars` and non-null
  legacy `hotbar` — verifies migration to single-bar result.
- REQ-MHB-14b: Unit test for `SaveHotbars` round-trip — save multiple bars, load
  back, verify identity.
- REQ-MHB-14c: Unit test for `handleHotbar("create")` — verify bar is appended,
  active index updated, limit enforced.
- REQ-MHB-14d: Unit test for `handleHotbar("switch")` — verify active index
  updated, out-of-range rejected.
- REQ-MHB-14e: Property-based test: `activeHotbarIndex` MUST always be in
  `[0, len(Hotbars)-1]` after any create or switch operation.

---

## Data Flow — Create New Hotbar

```
User clicks "+ New Hotbar"
  → sendMessage('HotbarRequest', {action: 'create'})
  → server: len(Hotbars) < maxHotbars?
      YES → append empty bar, ActiveHotbarIndex = new bar index
            SaveHotbars(ctx, characterID, sess.Hotbars, sess.ActiveHotbarIndex)
            send HotbarUpdateEvent{slots: emptyBar, active_hotbar_index: N, hotbar_count: N, max_hotbars: M}
      NO  → send MessageEvent{"Hotbar limit reached (max M)."}
  → client: SET_HOTBAR → activeHotbarIndex=N, hotbarCount=N
  → HotbarPanel re-renders: indicator shows N/N, no "+ New Hotbar" if N==M
```

## Data Flow — Switch Hotbar

```
User clicks ▲ (or Ctrl+Up)
  → compute targetIndex = (activeHotbarIndex - 2 + hotbarCount) % hotbarCount + 1  [wrap]
  → sendMessage('HotbarRequest', {action: 'switch', hotbar_index: targetIndex})
  → server: validate range, set ActiveHotbarIndex = targetIndex - 1
            SaveHotbars(ctx, characterID, sess.Hotbars, sess.ActiveHotbarIndex)
            send HotbarUpdateEvent{slots: bar[targetIndex-1], active_hotbar_index: targetIndex, ...}
  → client: SET_HOTBAR → hotbarSlots = new bar slots, activeHotbarIndex = targetIndex
  → HotbarPanel re-renders with new slot contents
```
