# Use Command Expansion

Extends `use` to support room equipment and adds server-side tab completion. See `docs/superpowers/specs/2026-03-20-use-command-design.md` for full design spec.

## Requirements

### `use` Command Room Equipment Support

- [x] REQ-USE-1: `use <target>` attempts feat/ability lookup first; room equipment lookup only on no-match
- [x] REQ-USE-2: `interact <target>` routes directly to room equipment lookup, bypassing feat lookup
- [x] REQ-USE-3: Room equipment lookup uses `RoomEquipmentManager.GetInstance(roomID, target)` with case-insensitive description matching
- [x] REQ-USE-4: `use` resolving to room equipment follows same activation logic as `handleUseEquipment` (script → skill checks → fallback message)
- [x] `handleUse` in `grpc_service.go` extended to call `handleUseEquipment` flow on feat lookup miss

### Tab Completion

- [x] REQ-USE-5: Telnet frontend sends `TabCompleteRequest { prefix string }` to gameserver on tab key (`0x09`)
- [x] REQ-USE-6: Tab key does NOT modify current input buffer
- [x] REQ-USE-7: `handleTabComplete` completes command names for single-word prefixes
- [x] REQ-USE-8: `handleTabComplete` completes feat names + room equipment descriptions for `use <partial>` / `interact <partial>`
- [x] REQ-USE-9: Completions sorted alphabetically and deduplicated
- [x] REQ-USE-10: Completions displayed as console message; input buffer not auto-filled
- [x] REQ-USE-11: More than 10 completions shows first 10 + remaining count

### Proto

- [x] `TabCompleteRequest { prefix string }` added to `ClientMessage` oneof
- [x] `TabCompleteResponse { completions []string }` added to `ServerMessage` oneof

### Handler Pattern

- [x] `HandlerTabComplete` constant in command registry
- [x] `BuiltinCommands()` entry (hidden from help listing)
- [x] Bridge handler in frontend `BridgeHandlers`
- [x] `handleTabComplete` case in `grpc_service.go`
- [x] Frontend `conn.go`: tab character (`0x09`) triggers `TabCompleteRequest` instead of being filtered
