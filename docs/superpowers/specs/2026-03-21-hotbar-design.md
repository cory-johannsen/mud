# Hotbar Design

## Overview

A persistent 10-slot hotbar that gives players quick access to frequently-used commands. Each slot stores arbitrary command text (e.g., `use fireball`, `smooth_talk`, `move north`). Slots are activated by typing their number (`1`–`9`, `0` for slot 10) at the prompt. The hotbar renders as a fixed row just above the input prompt, always visible.

---

## Requirements

- REQ-HB-1: Players MUST have exactly 10 hotbar slots, numbered 1–10. Slot 10 is activated by typing `0` at the prompt.
- REQ-HB-2: Each slot MUST store an arbitrary command string (empty string = unassigned). No type validation is performed on the stored text.
- REQ-HB-3: `hotbar <slot> <text>` MUST assign `<text>` to `<slot>` and persist immediately. `<slot>` is 1–10 (decimal); if out of range, a `messageEvent("Slot out of range (1-10).")` MUST be returned with no other side effect.
- REQ-HB-4: `hotbar clear <slot>` MUST clear the slot (set to empty string) and persist immediately. `<slot>` is 1–10 (decimal); if out of range, a `messageEvent("Slot out of range (1-10).")` MUST be returned.
- REQ-HB-5: `hotbar` with no arguments MUST display all 10 slots, one per line, in the format `[N] <text>` (or `[N] ---` if empty), where N is the slot number 1–10.
- REQ-HB-6: Typing `1`–`9` or `0` at the prompt MUST re-send the stored command text as if the player typed it, if the slot is non-empty. If the slot is empty, a message MUST inform the player the slot is unassigned.
- REQ-HB-7: The hotbar row MUST be rendered as a fixed pinned row at terminal row H-1 (one row above the input prompt), visible at all times in the split-screen UI. The console scrollable area MUST shrink by 1 row (rows 10 to H-2); the prompt remains at row H.
- REQ-HB-8: The hotbar row format MUST be: `[1:<label>] [2:<label>] ... [0:<label>]` where each `<label>` is the stored command truncated to `max(3, (width/10)-4)` characters (integer division; no ellipsis appended) or `---` if empty. On very narrow terminals where even 3-char labels with brackets would overflow, the hotbar row is rendered as a single line of as many complete slots as fit, left to right.
- REQ-HB-9: Hotbar assignments MUST be persisted per character in the database and restored on login.
- REQ-HB-10: The hotbar row MUST be redrawn on terminal resize.
- REQ-HB-11: `hotbar` assignments MUST survive server restart (DB-backed, not in-memory only).

---

## Architecture

### Data Model

`PlayerSession` gains a `Hotbar [10]string` field. Index 0 = slot 1, index 9 = slot 10. The DB stores hotbar data as a `TEXT` column `hotbar` on the `characters` table (10 slots JSON-encoded as a string array; NULL = all empty). Loaded into session at login via the existing `CharacterRepository`; saved synchronously on any `hotbar` write command.

### Commands

The `hotbar` command is parsed by the frontend's command parser in `bridge_handlers.go` before dispatch to the gameserver:

- `hotbar` (no args) — sends `HotbarRequest{action:"show"}` to the gameserver. The gameserver responds with a console message listing all slots (REQ-HB-5 format), not a `HotbarUpdateEvent`.
- `hotbar <slot> <text>` — sends `HotbarRequest{action:"set", slot:<slot>, text:<text>}`. `<slot>` is parsed as an integer 1–10.
- `hotbar clear <slot>` — sends `HotbarRequest{action:"clear", slot:<slot>}`. `<slot>` is parsed as an integer 1–10.

New handler `handleHotbar` in `internal/gameserver/grpc_service_hotbar.go` processes the three sub-actions.

### Slot Activation

In `internal/frontend/handlers/bridge_handlers.go`, command input is intercepted before dispatch to the gameserver: if the raw input is exactly one character `1`–`9` or `0`, it is mapped to slot index 0–9 (digit `0` → slot 10, index 9), looked up in `sess.Hotbar`, and the stored text is injected as the actual command (re-entering the dispatch pipeline). If the slot is empty, a `"Slot N is unassigned."` message is written locally without contacting the gameserver. All other input is dispatched normally.

### UI Rendering

`internal/frontend/telnet/screen.go` gains a `WriteHotbar(slots [10]string, width int)` method that renders the hotbar row at row H-1 using ANSI cursor positioning. The console scrollable area shrinks by 1 row (rows 10 to H-2); the prompt remains at row H. `WriteHotbar` is called:
- During initial screen setup (`InitScreen`)
- On receipt of a `HotbarUpdateEvent` from the gameserver (handled in `game_bridge.go`, which calls `screen.WriteHotbar`)
- On terminal resize (the existing resize handler in `bridge_handlers.go` calls `screen.WriteHotbar` with the current hotbar state)

The `game_bridge.go` file receives `HotbarUpdateEvent` from the gRPC stream and calls `screen.WriteHotbar`. The `bridge_handlers.go` file handles slot activation intercept, resize-triggered redraw, and frontend-local "unassigned" messages.

### Proto

`HotbarRequest` (field 101) is added to the `ClientMessage` oneof. `HotbarUpdateEvent` (field 102) is added to the `ServerMessage` oneof.

```protobuf
message HotbarRequest {
  string action = 1;  // "set", "clear", "show"
  int32 slot = 2;     // 1–10 for set/clear; ignored for show
  string text = 3;    // non-empty for set; ignored for clear/show
}

message HotbarUpdateEvent {
  // Always exactly 10 entries; slots[0] = slot 1, slots[9] = slot 10.
  // Empty string = unassigned slot.
  repeated string slots = 1;
}
```

`HotbarUpdateEvent` is sent by the server: (a) after any successful `set` or `clear` hotbar command, and (b) once immediately after the character loads at login (so the frontend can render the initial hotbar row).

### Persistence

New DB migration adds `hotbar TEXT` column to `characters` table (nullable; NULL = all empty). `CharacterRepository` gains `SaveHotbar(charID int64, slots [10]string) error` and loads hotbar in `GetCharacter`. JSON-encoded as a simple string array.

---

## File Map

| File | Change |
|------|--------|
| `api/proto/game/v1/game.proto` | Add `HotbarRequest` (101) to `ClientMessage` oneof; add `HotbarUpdateEvent` (102) to `ServerMessage` oneof |
| `internal/gameserver/grpc_service_hotbar.go` | New: `handleHotbar` (set/clear/show sub-actions) |
| `internal/gameserver/grpc_service_hotbar_test.go` | New: tests for set/clear/show, out-of-range, persistence round-trip, HotbarUpdateEvent emission |
| `internal/gameserver/grpc_service.go` | Wire `HotbarRequest` dispatch case; send `HotbarUpdateEvent` at login |
| `internal/game/session/session.go` | Add `Hotbar [10]string` field |
| `internal/storage/postgres/character.go` | Load/save hotbar column |
| `migrations/032_character_hotbar.up.sql` | `ALTER TABLE characters ADD COLUMN hotbar TEXT` |
| `migrations/032_character_hotbar.down.sql` | `ALTER TABLE characters DROP COLUMN hotbar` |
| `internal/frontend/telnet/screen.go` | Add `WriteHotbar(slots [10]string, width int)`; adjust console row range to H-2 |
| `internal/frontend/telnet/screen_test.go` | Tests for `WriteHotbar` rendering |
| `internal/frontend/handlers/bridge_handlers.go` | Parse `hotbar` command variants; intercept `0`–`9` single-char input for slot activation; trigger `WriteHotbar` on resize |
| `internal/frontend/handlers/game_bridge.go` | Handle `HotbarUpdateEvent`: call `screen.WriteHotbar` with received slots |

---

## Test Strategy

- REQ-HB-TS-1: `grpc_service_hotbar_test.go` MUST cover: set valid slot (1, 10), set out-of-range slot (0, 11) returns error message, clear valid slot, clear out-of-range slot, show (returns formatted list of all 10 slots), persistence round-trip (save + reload via DB stub), `HotbarUpdateEvent` sent after set, `HotbarUpdateEvent` sent after clear.
- REQ-HB-TS-2: Property-based tests (`pgregory.net/rapid`) in `screen_test.go` MUST cover: for any 10-slot array of arbitrary strings and any terminal width ≥ 10, `WriteHotbar` MUST produce a single line that fits within `width` bytes; each label MUST be at most `max(3, (width/10)-4)` characters; the line MUST contain exactly 10 slot segments.
- REQ-HB-TS-3: `screen_test.go` MUST cover: `WriteHotbar` renders `---` for empty slots, renders truncated label for long commands, renders the correct activation key (`1`–`9`, `0`) for each slot position.
- REQ-HB-TS-4: `bridge_handlers_test.go` MUST cover: single-char input `1`–`9`/`0` with non-empty slot injects the stored command, single-char input with empty slot sends unassigned message, multi-char input (e.g. `10`) passes through unchanged, non-digit single-char input passes through unchanged.

---

## Non-Goals

- No type validation on stored text — any string is valid.
- No per-slot icons, colors, or cooldown indicators.
- No hotbar profiles or multiple hotbar pages.
- No keybind customization beyond `0`–`9`.
