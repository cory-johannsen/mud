# Hotbar Design

## Overview

A persistent 10-slot hotbar that gives players quick access to frequently-used commands. Each slot stores arbitrary command text (e.g., `use fireball`, `smooth_talk`, `move north`). Slots are activated by typing their number (`1`–`9`, `0` for slot 10) at the prompt. The hotbar renders as a fixed row just above the input prompt, always visible.

---

## Requirements

- REQ-HB-1: Players MUST have exactly 10 hotbar slots, numbered 1–10. Slot 10 is activated by typing `0`.
- REQ-HB-2: Each slot MUST store an arbitrary command string (empty string = unassigned). No type validation is performed on the stored text.
- REQ-HB-3: `hotbar <slot> <text>` MUST assign `<text>` to `<slot>` and persist immediately. `<slot>` must be 0–9 (where 0 = slot 10); fatal error if out of range.
- REQ-HB-4: `hotbar clear <slot>` MUST clear the slot (set to empty string) and persist immediately.
- REQ-HB-5: `hotbar` with no arguments MUST display all 10 slots and their current assignments.
- REQ-HB-6: Typing `1`–`9` or `0` at the prompt MUST re-send the stored command text as if the player typed it, if the slot is non-empty. If the slot is empty, a message MUST inform the player the slot is unassigned.
- REQ-HB-7: The hotbar row MUST be rendered as a fixed pinned row at terminal row H-1 (one row above the input prompt), visible at all times in the split-screen UI.
- REQ-HB-8: The hotbar row format MUST be: `[1:<label>] [2:<label>] ... [0:<label>]` where `<label>` is the first N characters of the stored command (truncated to fit terminal width) or `---` if empty.
- REQ-HB-9: Hotbar assignments MUST be persisted per character in the database and restored on login.
- REQ-HB-10: The hotbar row MUST be redrawn on terminal resize.
- REQ-HB-11: `hotbar` assignments MUST survive server restart (DB-backed, not in-memory only).

---

## Architecture

### Data Model

`PlayerSession` gains a `Hotbar [10]string` field. Index 0 = slot 1, index 9 = slot 10. The DB stores hotbar data as a `VARCHAR(2000)` column `hotbar` on the `characters` table (10 slots JSON-encoded as a string array). Loaded into session at login via the existing `CharacterRepository`; saved synchronously on any `hotbar` write command.

### Commands

New handler `handleHotbar` in `internal/gameserver/grpc_service_hotbar.go` (mirrors the healer/job trainer pattern). Dispatched from the main switch on a new proto message `HotbarRequest { string action = 1; int32 slot = 2; string text = 3; }` at field 101 in the `ClientMessage` oneof.

Three sub-actions dispatched by the `action` field:
- `"set"` — assign text to slot
- `"clear"` — clear slot
- `"show"` — display all slots

### Slot Activation

In `internal/frontend/handlers/bridge_handlers.go`, command input is intercepted before dispatch: if the input is a single character `0`–`9`, it is looked up in `sess.Hotbar` and the stored text is injected as the actual command. If the slot is empty, a `"Slot N is unassigned."` message is sent instead.

### UI Rendering

`internal/frontend/telnet/screen.go` gains a `WriteHotbar(slots [10]string, width int)` method that renders the hotbar row at row H-1. The prompt moves to row H (no change). Console scrollable area shrinks by 1 row (rows 10 to H-2). `WriteHotbar` is called on:
- Initial screen setup (`InitScreen`)
- Any hotbar assignment change (server sends a `HotbarUpdateEvent` proto message)
- Terminal resize

### Proto

New messages at fields 101–102:
```protobuf
message HotbarRequest {
  string action = 1;  // "set", "clear", "show"
  int32 slot = 2;
  string text = 3;
}

message HotbarUpdateEvent {
  repeated string slots = 1;  // 10 entries
}
```

### Persistence

New DB migration adds `hotbar TEXT` column to `characters` table (nullable; NULL = all empty). `CharacterRepository` gains `SaveHotbar(charID int64, slots [10]string) error` and loads hotbar in `GetCharacter`. JSON-encoded as a simple string array.

---

## File Map

| File | Change |
|------|--------|
| `api/proto/game/v1/game.proto` | Add `HotbarRequest` (101) + `HotbarUpdateEvent` (102) |
| `internal/gameserver/grpc_service_hotbar.go` | New: handleHotbar, saveHotbar |
| `internal/gameserver/grpc_service_hotbar_test.go` | New: tests for set/clear/show/activation |
| `internal/gameserver/grpc_service.go` | Wire Hotbar dispatch case; send HotbarUpdateEvent |
| `internal/game/session/session.go` | Add `Hotbar [10]string` field |
| `internal/storage/postgres/character.go` | Load/save hotbar column |
| `migrations/032_character_hotbar.up.sql` | `ALTER TABLE characters ADD COLUMN hotbar TEXT` |
| `migrations/032_character_hotbar.down.sql` | `ALTER TABLE characters DROP COLUMN hotbar` |
| `internal/frontend/telnet/screen.go` | Add `WriteHotbar`; adjust console row range |
| `internal/frontend/telnet/screen_test.go` | Tests for WriteHotbar rendering |
| `internal/frontend/handlers/bridge_handlers.go` | Intercept `0`–`9` input for slot activation; handle HotbarUpdateEvent |
| `internal/frontend/handlers/game_bridge.go` | Pass HotbarUpdateEvent to WriteHotbar |

---

## Non-Goals

- No type validation on stored text — any string is valid.
- No per-slot icons, colors, or cooldown indicators.
- No hotbar profiles or multiple hotbar pages.
- No keybind customization beyond `0`–`9`.
