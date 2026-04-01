# Web UI — Equipment Tab Controls

**Slug:** web-ui-equipment-controls
**Status:** spec
**Priority:** 482
**Category:** ui
**Effort:** S

## Overview

The web UI Equipment tab currently displays equipped items per slot in read-only mode. This feature adds an **Unequip** button to each occupied slot so players can remove equipped items without typing commands. Equipping items into empty slots is handled by the Inventory tab (see `web-ui-inventory-equip`); this feature covers only the Equipment tab's unequip controls.

## Dependencies

- `web-client` — Equipment tab in the web game client

## Spec

### REQ-WEC-1: Unequip Button on Occupied Slots

The `EquipmentDrawer` component MUST render an **Unequip** button beside each occupied equipment slot. A slot is occupied when its value in the `CharacterSheetView` is a non-empty string. Unoccupied (empty) slots MUST NOT display any button.

### REQ-WEC-2: Slot Coverage

Unequip buttons MUST be provided for all slot categories currently rendered by `EquipmentDrawer`:

- Weapon slots: `main` (main hand), `off` (off hand) — sourced from `CharacterSheetView.main_hand` and `CharacterSheetView.off_hand`
- Armor slots: `head`, `torso`, `left_arm`, `right_arm`, `hands`, `left_leg`, `right_leg`, `feet` — sourced from `CharacterSheetView.armor`
- Accessory slots: `neck`, `left_ring_1`–`left_ring_5`, `right_ring_1`–`right_ring_5` — sourced from `CharacterSheetView.accessories`

### REQ-WEC-3: Command Dispatch

When the player clicks Unequip on a slot, the client MUST send a `CommandText` WebSocket frame with the text `unequip <slot>`, where `<slot>` is the slot identifier string (e.g. `unequip main`, `unequip head`, `unequip neck`).

### REQ-WEC-4: Character Sheet Refresh

After sending the unequip command, the client MUST send a `CharacterSheetRequest` frame to refresh the Equipment tab display, reflecting the now-empty slot.

### REQ-WEC-5: Inventory Refresh on Unequip

After unequipping, the client MUST also send an `InventoryRequest` frame so that the Inventory tab reflects the returned item.

### REQ-WEC-6: Cursed Item Handling

The server already rejects unequip of cursed items with a console error message. The client MUST NOT implement any special cursed-item logic; server rejection is surfaced through the normal console output path.

### REQ-WEC-7: No New Proto or Backend Changes

All unequip dispatches MUST reuse the existing `CommandText` → `unequip` command pathway. No new proto messages, gRPC handlers, or WebSocket frame types are required for this feature.

## Implementation Notes

- **File to modify:** `cmd/webclient/ui/src/game/drawers/EquipmentDrawer.tsx`
- The `EquipSlot` component currently renders label + value; it MUST be extended (or a wrapper added) to optionally render an Unequip button when an `onUnequip` callback is provided
- `CharacterSheetView.main_hand` and `.off_hand` are top-level strings; armor and accessory slots are `map<string, string>` keyed by slot name
- The server-side `command.HandleUnequip()` in `internal/game/command/unequip.go` accepts slot name or item def ID; passing the slot name directly is sufficient
- `handleUnequip` in `internal/gameserver/grpc_service.go` (line 5412) does not currently persist the unequip state to DB or update the combat handler cache — this is a pre-existing gap (not in scope for this feature; log separately if it causes visible bugs)
