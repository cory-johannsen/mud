# Web UI Inventory Equip Control Implementation Plan

**Goal:** Add inline equip buttons to the Inventory drawer so players can equip weapons (with Main/Off slot picker) and armor (direct Wear) without typing commands.

**Architecture:** Two-part change: (1) extend the `InventoryItem` proto with `item_def_id` and `armor_slot` fields and populate them server-side in `handleInventory`; (2) rewrite `InventoryDrawer.tsx` to add per-row equip controls using the SlotPicker overlay pattern from `TechnologyDrawer.tsx`. The UI sends `sendMessage('Equip', ...)` and `sendMessage('Wear', ...)` frames that route via the existing proto dispatch path in `websocket_dispatch.go`.

**Tech Stack:** React 18, TypeScript, Vite (`npm run build` = `tsc && vite build`), protobuf (hand-written TS mirror in `proto/index.ts`), Go 1.22+

**Key findings from exploration:**
- `equip` command takes `<item_def_id> [main|off]`; `wear` takes `<item_def_id> <slot>` (e.g. `head`, `torso`)
- `InventoryItem` proto currently lacks `item_def_id` and `armor_slot` — both required
- `sendMessage('Equip', { weapon_id, slot })` and `sendMessage('Wear', { item_id, slot })` route via existing proto dispatch

---

## Task 1: Extend InventoryItem proto with item_def_id and armor_slot

**Files:**
- `api/proto/game/v1/game.proto`
- `internal/gameserver/gamev1/game.pb.go`
- `internal/gameserver/grpc_service.go`

- [ ] Add `item_def_id` (field 6) and `armor_slot` (field 7) to `InventoryItem` in the proto file
- [ ] Hand-edit `game.pb.go` to add `ItemDefId` and `ArmorSlot` fields and getters to the `InventoryItem` struct (the project does not use `buf`/`protoc` in CI — hand-edit is the established pattern)
- [ ] In `handleInventory` in `grpc_service.go`, populate `ItemDefId: inst.ItemDefID` unconditionally; populate `ArmorSlot` by looking up the armor definition when `kind == "armor"`
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] Commit

---

## Task 2: Update TypeScript proto mirror

**Files:**
- `cmd/webclient/ui/src/proto/index.ts`

- [ ] Add `itemDefId?: string`, `item_def_id?: string`, `armorSlot?: string`, `armor_slot?: string` to the `InventoryItem` interface (both camelCase and snake_case per project pattern)
- [ ] `npm run build` passes
- [ ] Commit

---

## Task 3: Add equip/wear controls to InventoryDrawer

**Files:**
- `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx`

**UI spec:**
- Weapons (`kind === "weapon"`): "Equip" button → inline slot picker overlay (Main Hand / Off Hand) → `sendMessage('Equip', { weapon_id: itemDefId, slot: 'main'|'off' })`
- Armor (`kind === "armor"`): "Wear" button → `sendMessage('Wear', { item_id: itemDefId, slot: armorSlot })`; button disabled when `armorSlot` is empty
- All other kinds: no action button; render plain row
- New "Action" column header in the table
- Slot picker is a `position: absolute` overlay on the row, `zIndex: 10`, matching TechnologyDrawer SlotPicker pattern
- Styles follow existing drawer conventions: `#8d4` green for action buttons, `#9ab` blue for slot buttons

- [ ] Implement `WeaponRow`, `ArmorRow`, `PlainRow` sub-components
- [ ] `InventoryDrawer` dispatches to correct row component based on `item.kind`
- [ ] `npm run build` passes with zero TypeScript errors
- [ ] Commit

---

## Task 4: Mark feature done

**Files:**
- `docs/features/web-ui-inventory-equip.md`

- [ ] Change status from `backlog` to `done`
- [ ] Final `go test ./...` and `npm run build` green
- [ ] Commit

---

## Verification Checklist

- [ ] `InventoryItem` proto has `item_def_id` (field 6) and `armor_slot` (field 7)
- [ ] `handleInventory` populates both new fields
- [ ] `go build ./...` clean
- [ ] `go test ./...` all pass
- [ ] TypeScript `InventoryItem` has `itemDefId?`/`item_def_id?` and `armorSlot?`/`armor_slot?`
- [ ] Equip button + slot picker visible for `kind === 'weapon'`
- [ ] Wear button visible for `kind === 'armor'` (disabled when no slot)
- [ ] No button for other kinds
- [ ] `npm run build` clean
- [ ] Feature status `done`
