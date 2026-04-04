# Hotbar — Context-Aware Slots

## Overview

Upgrades hotbar slots from plain command strings to typed references. Each slot carries a `kind` (command/feat/technology/throwable/consumable) and a `ref` (item def ID, feat ID, or raw command text). The server resolves `display_name` and `description` from the appropriate registry and sends them in `HotbarUpdateEvent`. Feats, Technology, and Inventory drawers expose "+ Hotbar" buttons for typed slot assignment. The telnet and web frontends display the resolved name instead of the raw ref, and the web client auto-fills the nearest enemy as a combat target for bare attack commands.

---

## Requirements

- REQ-HCA-1: The domain MUST define `HotbarSlot{Kind string; Ref string}` replacing the existing `[10]string` hotbar field on `PlayerSession`. Five kind constants MUST be defined: `HotbarSlotKindCommand`, `HotbarSlotKindFeat`, `HotbarSlotKindTechnology`, `HotbarSlotKindThrowable`, `HotbarSlotKindConsumable`. `ActivationCommand()` MUST return `"use <ref>"` for feat/technology/consumable, `"throw <ref>"` for throwable, and `ref` directly for command/empty kind.
- REQ-HCA-2: Hotbar persistence MUST store slots as a JSON array of `{kind, ref}` objects. `LoadHotbar` MUST auto-migrate the legacy plain-string format (JSON array of strings) by treating each non-empty string as a command slot.
- REQ-HCA-3: The `HotbarSlot` protobuf message MUST carry `kind`, `ref`, `display_name`, and `description` fields. `HotbarUpdateEvent.slots` MUST be `repeated HotbarSlot` (not `repeated string`). `HotbarRequest` MUST gain `kind` and `ref` fields for typed slot assignment.
- REQ-HCA-4: `handleHotbar` MUST prefer `kind`+`ref` from the request to construct a typed slot; it MUST fall back to a `CommandSlot(text)` when `kind` is empty. Set and clear MUST return `HotbarUpdateEvent` (not `MessageEvent`).
- REQ-HCA-5: The server MUST resolve `display_name` and `description` from the feat, technology, or inventory registry (whichever matches the slot kind) and populate those fields on every `HotbarSlot` in the emitted `HotbarUpdateEvent`. Command slots MUST have empty `display_name` and `description`.
- REQ-HCA-6: The telnet bridge MUST use `display_name` (falling back to `ref`) from the received `HotbarSlot` proto message when rendering the hotbar row label.
- REQ-HCA-7: The web client hotbar panel MUST display `displayName ?? display_name ?? ref` as the slot label (not the raw ref).
- REQ-HCA-8: The web client hover tooltip MUST display `"{ref} (right-click to edit)"` for command/empty-kind slots, and `"{name}\n{description}\n(right-click to edit)"` (with description line omitted if absent) for typed slots.
- REQ-HCA-9: The FeatsDrawer MUST add a `+ Hotbar` button to each feat entry that sends `HotbarRequest{action:"set", slot:N, kind:"feat", ref:<featId>}`.
- REQ-HCA-10: The TechnologyDrawer MUST add a `+ Hotbar` button to each technology entry that sends `HotbarRequest{action:"set", slot:N, kind:"technology", ref:<techId>}`.
- REQ-HCA-11: The InventoryDrawer MUST add a `+ Hotbar` button to consumable items sending `kind:"consumable"` and to throwable items sending `kind:"throwable"`. The `SlotPicker` component MUST show current slot occupants from `hotbarSlots`.
- REQ-HCA-12: `handleInventory` MUST populate `InventoryItem.throwable = true` for any item whose `ItemDef` carries the `"throwable"` tag.

---

## Architecture

### Domain Model

`internal/game/session/hotbar_slot.go` defines:

```go
type HotbarSlot struct { Kind string; Ref string }

const (
    HotbarSlotKindCommand     = "command"
    HotbarSlotKindFeat        = "feat"
    HotbarSlotKindTechnology  = "technology"
    HotbarSlotKindThrowable   = "throwable"
    HotbarSlotKindConsumable  = "consumable"
)

func (s HotbarSlot) ActivationCommand() string
func (s HotbarSlot) IsEmpty() bool
func CommandSlot(text string) HotbarSlot
```

`PlayerSession.Hotbar` is `[10]HotbarSlot` (was `[10]string`).

### Display Resolution Pipeline

```
HotbarRequest{kind, ref}
    → buildHotbarSlot(req) → session.HotbarSlot
    → session.Hotbar[slot-1] = slot
    → hotbarUpdateEvent(slots) → resolveHotbarSlotDisplay(slot) per slot
        → featRegistry.Feat(ref) → (name, "")
        → techRegistry.Tech(ref) → (shortName ?? name, description)
        → invRegistry.Item(ref)  → (item.Name, "")
    → gamev1.HotbarUpdateEvent{Slots: [10]*gamev1.HotbarSlot{...}}
```

### Throwable Detection

```
handleInventory(uid)
    → sess.Backpack.Items()
    → invRegistry.Item(inst.ItemDefID) → def
    → def.HasTag("throwable") → InventoryItem.Throwable = true/false
```

### Web Client Activation

`slotActivationCommand(slot)` maps kind to command verb:

| Kind | Command |
|------|---------|
| `feat`, `technology`, `consumable` | `use <ref>` |
| `throwable` | `throw <ref>` |
| `command`, `` | `<ref>` (raw) |

Auto-combat-target: when in combat (`combatRound` set), bare attack verbs (`attack`, `att`, `kill`, `strike`, `st`, `burst`, `bf`) with no argument are auto-suffixed with the first non-player name from `combatRound.turnOrder`.

---

## File Map

| File | Change |
|------|--------|
| `internal/game/session/hotbar_slot.go` | New: domain type + kind constants + helpers |
| `internal/game/session/manager.go` | `Hotbar [10]string` → `Hotbar [10]HotbarSlot` |
| `api/proto/game/v1/game.proto` | New `HotbarSlot` message; `HotbarUpdateEvent.slots` typed; `HotbarRequest` gains `kind`, `ref` |
| `internal/gameserver/gamev1/game.pb.go` | Regenerated |
| `internal/storage/postgres/character_hotbar.go` | Typed JSON marshal/unmarshal + legacy migration |
| `internal/gameserver/grpc_service_hotbar.go` | `buildHotbarSlot`, `hotbarUpdateEvent`, `resolveHotbarSlotDisplay` |
| `internal/gameserver/grpc_service.go` | Throwable flag in `handleInventory` |
| `internal/gameserver/grpc_service_hotbar_test.go` | Updated + new tests incl. typed slots, throwable flag, nil registry guard |
| `internal/frontend/handlers/game_bridge.go` | `currentHotbar` stores `[10]*gamev1.HotbarSlot`; `hotbarSlotCommand`, `hotbarLabels` helpers |
| `internal/frontend/handlers/game_bridge_hotbar_test.go` | New: unit + property tests for hotbar helpers |
| `cmd/webclient/ui/src/proto/index.ts` | `HotbarSlot` interface; `InventoryItem.throwable` |
| `cmd/webclient/ui/src/game/GameContext.tsx` | `hotbarSlots: HotbarSlot[]` |
| `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx` | `slotActivationCommand`, `slotDisplayLabel`, `slotTooltip`; typed `EditPopup` |
| `cmd/webclient/ui/src/game/panels/HotbarPanel.test.ts` | 15 unit tests for all three exported helpers |
| `cmd/webclient/ui/src/game/drawers/FeatsDrawer.tsx` | `SlotPicker` + `+ Hotbar` button for feats |
| `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx` | `SlotPicker` + `+ Hotbar` button for all tech categories |
| `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx` | `SlotPicker`, `ConsumableRow`, `ThrowableRow` + `+ Hotbar` buttons |

---

## Non-Goals

- No per-slot icons, colors, or cooldown indicators.
- No hotbar profiles or multiple hotbar pages.
- No keybind customization.
- No client-side validation of ref values — the server ignores unknown refs at display resolution time (returns empty display name).
