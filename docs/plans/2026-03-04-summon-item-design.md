# summon_item Admin Command Design

**Date:** 2026-03-04

## Goal

Add a `summon_item` editor/admin command that spawns an item instance onto the floor of the caller's current room.

---

## Section 1: Command Interface

**Syntax:** `summon_item <item_id> [quantity]`

- Requires **editor** or **admin** role (checked server-side in gRPC handler; `sess.Role == "editor" || sess.Role == "admin"`)
- `item_id` — required; must match a known item in the item registry
- `quantity` — optional integer, defaults to 1, minimum 1
- Item is placed on the floor of the caller's current room as a single `ItemInstance` with `Quantity` set to the given value
- Success: `"Summoned 3x Assault Rifle to the room."`
- Errors: unknown item ID, invalid quantity (non-integer or < 1), permission denied

---

## Section 2: Implementation

Follows the full CMD-1 through CMD-7 pattern:

- **CMD-1**: `HandlerSummonItem = "summon_item"` constant in `internal/game/command/commands.go`
- **CMD-2**: `BuiltinCommands()` entry with `CategoryAdmin`
- **CMD-3**: `HandleSummonItem` function in `internal/game/command/summon_item.go` — parses and validates args
- **CMD-4**: `SummonItemRequest { item_id string = 1, quantity int32 = 2 }` proto message + `ClientMessage` oneof field; `make proto` run
- **CMD-5**: `bridgeSummonItem` in `internal/frontend/handlers/bridge_handlers.go` — parses quantity (default 1), builds proto request
- **CMD-6**: `handleSummonItem` in `internal/gameserver/grpc_service.go`:
  - Check `sess.Role == "editor" || sess.Role == "admin"`
  - Lookup item via `s.invRegistry.Item(itemID)`
  - Create `ItemInstance{InstanceID: uuid.New().String(), ItemDefID: itemID, Quantity: qty}`
  - Call `s.floorMgr.Drop(sess.RoomID, inst)`
  - Return success message
- **CMD-7**: TDD with property-based tests in `internal/gameserver/summon_item_handler_test.go`

---

## Out of Scope

- Spawning items into rooms other than the caller's current room
- Non-stackable multi-instance spawning (quantity always maps to `ItemInstance.Quantity`)
- Persistence of summoned items across server restart
