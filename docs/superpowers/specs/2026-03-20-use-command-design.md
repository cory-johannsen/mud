# Use Command Expansion — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `use-command` (priority 240)
**Dependencies:** none

---

## Overview

Two extensions to the existing `use` command:

1. **Room equipment support** — `use <description>` now matches room equipment instances in addition to feats/abilities. `interact` remains as an alias for room equipment only.
2. **Tab completion** — pressing Tab while typing sends a `TabCompleteRequest` to the gameserver, which returns matching command names and contextual targets. Results are displayed as a console message list.

---

## 1. `use` Command Expansion

### 1.1 Current Behavior

`use <abilityID>` activates a player feat or ability. `interact <description>` activates room equipment. These are two separate commands handled by `handleUse` and `handleUseEquipment` in `grpc_service.go`.

### 1.2 New Behavior

`use <target>` resolves in priority order:

1. Match against player's active feats and abilities (existing behavior, unchanged).
2. If no feat/ability match: match against room equipment instances via `RoomEquipmentManager.GetInstance(roomID, target)` (case-insensitive description match).
3. If no match in either: return "You don't know how to use that." (unchanged error message).

`interact <description>` continues to work and routes directly to step 2 (room equipment only), skipping feat lookup.

- REQ-USE-1: `use <target>` MUST attempt feat/ability lookup first; room equipment lookup MUST only occur if no feat/ability matches.
- REQ-USE-2: `interact <target>` MUST route directly to room equipment lookup, bypassing feat lookup.
- REQ-USE-3: Room equipment lookup MUST use `RoomEquipmentManager.GetInstance(roomID, target)` with case-insensitive description matching (existing behavior of that method).

### 1.3 Room Equipment Activation

When a room equipment instance is found via `use`:

1. If the instance has a `Script` field: execute the Lua script (existing `handleUseEquipment` flow).
2. If the instance has `SkillChecks`: trigger the skill check (existing `handleUseEquipment` flow).
3. If neither: return "You use the <description>." (existing fallback).

No new activation logic is required — this spec only routes `use` to the existing `handleUseEquipment` handler when a room equipment match is found.

- REQ-USE-4: When `use` resolves to room equipment, the activation MUST follow the same logic as `handleUseEquipment` (script execution → skill checks → fallback message).

### 1.4 Environmental Features

Room equipment with `cover_tier` set (light switches, hidden panels, alarms, etc.) are also targetable via `use`. No special handling is required — these are `EquipmentInstance` objects like any other and are returned by `RoomEquipmentManager.EquipmentInRoom`.

---

## 2. Tab Completion

### 2.1 Frontend: Tab Key Handling

The telnet frontend's input handler already filters the tab character (`0x09`). This spec changes that behavior: when `0x09` is received, the frontend captures the current input buffer content (the partial command typed so far) and sends a `TabCompleteRequest` to the gameserver instead of discarding the character.

The current input buffer is NOT modified. The player's partial input remains on screen unchanged. Completions are displayed as a console message below the input line.

- REQ-USE-5: The telnet frontend MUST send a `TabCompleteRequest { prefix: string }` to the gameserver when the tab character (`0x09`) is received.
- REQ-USE-6: The tab key MUST NOT modify the current input buffer.

### 2.2 Proto Messages

New messages in the `gamev1` proto package:

```protobuf
message TabCompleteRequest {
  string prefix = 1;  // current input buffer content
}

message TabCompleteResponse {
  repeated string completions = 1;  // matching completions, sorted alphabetically
}
```

`TabCompleteRequest` is added to `ClientMessage` oneof. `TabCompleteResponse` is added to `ServerMessage` oneof.

### 2.3 Gameserver: Completion Logic

`handleTabComplete(uid, prefix string)` in `grpc_service.go`:

**Parse the prefix:**
- If prefix is empty or has no space: complete against all command names. Filter `BuiltinCommands()` by names/aliases that start with `prefix` (case-insensitive).
- If prefix starts with `use ` or `interact `: complete against feat names + room equipment descriptions in the player's current room. Filter by the portion after the command name.
- If prefix starts with any other command name + space: return no completions (contextual completion only implemented for `use`/`interact`).

**Return:** sorted list of completions (full command strings, e.g., `["use medkit", "use medical station"]`).

- REQ-USE-7: `handleTabComplete` MUST complete command names for single-word prefixes.
- REQ-USE-8: `handleTabComplete` MUST complete feat names and room equipment descriptions for `use <partial>` and `interact <partial>` prefixes.
- REQ-USE-9: Completions MUST be sorted alphabetically and deduplicated.

### 2.4 Frontend: Displaying Completions

The frontend renders the `TabCompleteResponse` as a console message:

- If 0 completions: display "No completions found."
- If 1 completion: display the single completion as a console message (player still must type it or press Tab again — no auto-fill).
- If 2–10 completions: display as a single-line space-separated list, e.g., `[use] medkit  medical station  stimpak`.
- If >10 completions: display first 10 followed by `... (N more)`.

- REQ-USE-10: Tab completions MUST be displayed as a console message. The input buffer MUST NOT be auto-filled.
- REQ-USE-11: If more than 10 completions match, only the first 10 MUST be shown with a count of remaining.

### 2.5 Handler Pattern

`TabCompleteRequest` follows CMD-1 through CMD-7 pattern:
- `HandlerTabComplete` constant in command registry
- Entry in `BuiltinCommands()` (hidden from help listing — internal only)
- `TabCompleteRequest { prefix string }` proto message in `ClientMessage` oneof
- Bridge handler in frontend `BridgeHandlers`
- `handleTabComplete` case in `grpc_service.go`

---

## 3. Requirements Summary

- REQ-USE-1: `use <target>` MUST attempt feat/ability lookup first; room equipment lookup occurs only on no-match.
- REQ-USE-2: `interact <target>` MUST route directly to room equipment lookup, bypassing feat lookup.
- REQ-USE-3: Room equipment lookup MUST use `RoomEquipmentManager.GetInstance` with case-insensitive description matching.
- REQ-USE-4: `use` resolving to room equipment MUST follow the same activation logic as `handleUseEquipment`.
- REQ-USE-5: Frontend MUST send `TabCompleteRequest { prefix }` to the gameserver on tab key (`0x09`).
- REQ-USE-6: Tab key MUST NOT modify the current input buffer.
- REQ-USE-7: `handleTabComplete` MUST complete command names for single-word prefixes.
- REQ-USE-8: `handleTabComplete` MUST complete feat names and room equipment descriptions for `use <partial>` / `interact <partial>`.
- REQ-USE-9: Completions MUST be sorted alphabetically and deduplicated.
- REQ-USE-10: Completions MUST be displayed as a console message; input buffer MUST NOT be auto-filled.
- REQ-USE-11: More than 10 completions MUST show only the first 10 with a remaining count.
