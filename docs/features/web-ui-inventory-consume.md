# Web UI — Inventory Consume Control

**Slug:** web-ui-inventory-consume
**Status:** spec
**Priority:** 481
**Category:** ui
**Effort:** S

## Overview

In the web UI Inventory tab, consumable items should display an inline **Consume** button so players can use them without typing a command. Clicking the button sends `use <item_def_id>` as a `CommandText` frame, reusing the existing server-side `use` command pathway.

## Dependencies

- `web-client` — Inventory tab in the web game client

## Spec

### REQ-WIC-1: Consumable Row Rendering

The `InventoryDrawer` component MUST render a `ConsumableRow` for every inventory item whose `kind` field equals `"consumable"`. All other item kinds MUST continue to render as they do today (`WeaponRow`, `ArmorRow`, `PlainRow`).

### REQ-WIC-2: Consume Button

Each `ConsumableRow` MUST display a **Consume** button in the Action column. The button MUST be enabled whenever the item quantity is greater than zero.

### REQ-WIC-3: Command Dispatch

When the player clicks Consume, the client MUST send a `CommandText` WebSocket frame with the text `use <item_def_id>`, where `item_def_id` is the `item_def_id` field from the `InventoryItem` proto message. No modal or confirmation dialog is required.

### REQ-WIC-4: Inventory Refresh

After sending the consume command, the client MUST send an `InventoryRequest` frame to refresh the inventory list, reflecting the updated quantity or item removal.

### REQ-WIC-5: Quantity Display

The `ConsumableRow` MUST display the item's current quantity in the Qty column, consistent with other row types.

### REQ-WIC-6: No New Proto or Backend Changes

The consume action MUST reuse the existing `CommandText` → `use` command pathway. No new proto messages, gRPC handlers, or WebSocket frame types are required for this feature.

## Implementation Notes

- **File to modify:** `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx`
- Consumables are identified by `item.kind === 'consumable'` in the `InventoryItem` proto
- The server-side `use` command already handles substance-based and effect-based consumables via `handleUse()` → `handleConsumeSubstanceItem()` in `internal/gameserver/grpc_service.go`
- The `item_def_id` field on `InventoryItem` is the correct identifier to pass to `use`
