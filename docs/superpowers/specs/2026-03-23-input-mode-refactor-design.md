# Input Mode Refactor Design

**Date:** 2026-03-23
**Status:** Approved
**Feature:** `input-mode-refactor`

---

## Problem

The current telnet frontend uses a `mapModeState.mapMode bool` flag to intercept input for map navigation. `forwardServerEvents` runs concurrently and writes the room prompt (`buildPrompt()`) on every server event without checking whether map mode is active. This causes the normal room prompt to overwrite the map prompt every few seconds, breaking map mode UX. Additionally, the `bool` flag does not scale to additional interactive modes (inventory, character sheet, editor, combat display).

---

## Solution

Replace the scattered `mapMode bool` flag with a `ModeHandler` interface and a `SessionInputState` controller. All prompt rendering and input routing are centralized through the active handler.

---

## Requirements

### InputMode Type

- REQ-IMR-1: The frontend MUST define `type InputMode int` with constants: `ModeRoom`, `ModeMap`, `ModeInventory`, `ModeCharSheet`, `ModeEditor`, `ModeCombat`.
- REQ-IMR-2: Each `InputMode` constant MUST have a human-readable `String()` method for logging.

### ModeHandler Interface

- REQ-IMR-3: The frontend MUST define a `ModeHandler` interface with methods: `Mode() InputMode`, `OnEnter(conn *telnet.Conn)`, `OnExit(conn *telnet.Conn)`, `HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int, session *SessionInputState)`, and `Prompt() string`.
- REQ-IMR-4: `OnEnter` MUST set the visual state for entering the mode (e.g., clear console, write mode-specific prompt).
- REQ-IMR-5: `OnExit` MUST restore any state changed by `OnEnter` (e.g., clear mode display, restore room prompt).
- REQ-IMR-6: `HandleInput` MUST process a single trimmed input line for the mode and MUST NOT block. The `session *SessionInputState` parameter MUST be used for all mode transitions (e.g., `q`/ESC returning to room mode).
- REQ-IMR-7: `Prompt() string` MUST return the prompt string appropriate for the mode.

### SessionInputState

- REQ-IMR-8: A `SessionInputState` struct MUST own the active `ModeHandler` and expose `SetMode(m ModeHandler)`, `CurrentPrompt() string`, `Mode() InputMode`, and `Room() *RoomModeHandler`.
- REQ-IMR-8A: `SessionInputState.Mode()` MUST return the active handler's `Mode()` value.
- REQ-IMR-9: `SetMode` MUST guard against a nil outgoing handler. If the outgoing handler is non-nil, `SetMode` MUST call `OnExit` on it before calling `OnEnter` on the incoming handler.
- REQ-IMR-9A: `SessionInputState` MUST be initialized with a `RoomModeHandler` as the active handler (so no `SetMode` call is ever made with a nil outgoing handler in production).
- REQ-IMR-10: `SessionInputState` MUST be safe for concurrent execution: `SetMode` (called from `commandLoop`) MUST be safe under concurrent reads of `CurrentPrompt()` and `Mode()` from `forwardServerEvents`. A `sync.RWMutex` or equivalent MUST protect the active handler field.
- REQ-IMR-11: `SessionInputState` MUST hold a reference to the `RoomModeHandler` accessible via `Room()` so that `forwardServerEvents` can call `session.SetMode(session.Room())` when a server event requires exiting the current mode (e.g., a `RoomView` event received while in map mode forces a return to room mode).
- REQ-IMR-11A: When `forwardServerEvents` receives a `RoomView` event and the current mode is not `ModeRoom`, it MUST call `session.SetMode(session.Room())` before rendering the room view.

### RoomModeHandler

- REQ-IMR-12: `RoomModeHandler` MUST implement `ModeHandler` and encapsulate all current room-command dispatch logic from `commandLoop`, including the command registry, travel resolver, and help function.
- REQ-IMR-12A: `RoomModeHandler` MUST be constructed with all live-state references it needs for `Prompt()` via constructor injection: character name (`string`), role (`string`), current HP (`*atomic.Int32`), max HP (`*atomic.Int32`), current room (`*atomic.Value`), current time (`*atomic.Value`), and active conditions (`map[string]string` with its mutex).
- REQ-IMR-13: `RoomModeHandler.Prompt()` MUST return the current room prompt string using the injected live-state references (character name, HP, conditions, etc.).
- REQ-IMR-14: `RoomModeHandler.HandleInput` MUST dispatch to the command registry and handle empty-line prompt redraw.

### MapModeHandler

- REQ-IMR-15: The existing `mapModeState` struct and `handleMapModeInput` function MUST be refactored into a `MapModeHandler` implementing `ModeHandler`.
- REQ-IMR-16: `MapModeHandler.OnEnter` MUST clear the console region and write the map prompt.
- REQ-IMR-17: `MapModeHandler.OnExit` MUST clear the map display and signal the room view to redraw.
- REQ-IMR-18: All map-mode-specific state (`mapView`, `mapSelectedZone`, `lastMapResponse`) MUST remain on `MapModeHandler`.

### Stub Handlers (Inventory, CharSheet, Editor, Combat)

- REQ-IMR-19: `InventoryModeHandler`, `CharSheetModeHandler`, `EditorModeHandler`, and `CombatModeHandler` MUST be created as stub implementations. Each stub's `OnEnter` MUST write a placeholder message to the console. Each stub's `HandleInput` MUST call `session.SetMode(session.Room())` when the input is `"q"` or `"\x1b"` (ESC).
- REQ-IMR-20: Stubs MUST compile and be registered in `SessionInputState`; they are not required to have full UI logic in this feature.

### commandLoop Refactor

- REQ-IMR-21: `commandLoop` MUST replace the `if mapState.isActive()` interceptor with `session.HandleInput(line, conn, stream, &requestID, session)`.
- REQ-IMR-22: The `mapState *mapModeState` parameter MUST be removed from `commandLoop` and replaced with `session *SessionInputState`.
- REQ-IMR-23: Navigation sentinel handling (`\x00UP`, `\x00DOWN`, scroll keys) MUST remain in `commandLoop` before the `session.HandleInput` dispatch. All `buildPrompt()` calls within sentinel handling MUST be replaced with `session.CurrentPrompt()`.

### forwardServerEvents Refactor

- REQ-IMR-24: Every `WritePromptSplit`/`WritePrompt` call in `forwardServerEvents` MUST be replaced with `conn.WritePromptSplit(session.CurrentPrompt())` or equivalent. The `buildPrompt func()` parameter MUST be removed from `forwardServerEvents`.
- REQ-IMR-25: `forwardServerEvents` MUST NOT reference `mapModeState` or `mapState` directly; all mode awareness MUST go through `session`.

### Mode Transitions

- REQ-IMR-26: The `map` command handler MUST call `session.SetMode(mapModeHandler)` instead of `mapState.enter(view)`.
- REQ-IMR-27: Map-mode exit (`q`/ESC) MUST call `session.SetMode(session.Room())` via the `session` parameter in `HandleInput`.
- REQ-IMR-28: All future mode transitions MUST go through `session.SetMode`.

### Testing

- REQ-IMR-29: Unit tests MUST verify that `SetMode` calls `OnExit` on the old handler and `OnEnter` on the new handler in the correct order, and that `SetMode` with a nil outgoing handler does not panic.
- REQ-IMR-30: Unit tests MUST verify that `CurrentPrompt()` returns the active handler's `Prompt()` string after each `SetMode` call.
- REQ-IMR-31: Property-based tests using `pgregory.net/rapid` MUST verify that: (a) concurrent reads of `CurrentPrompt()` never race with `SetMode`; (b) repeated `SetMode` calls with arbitrary handler sequences leave `SessionInputState` in a consistent state (the active handler's `Mode()` always matches `session.Mode()`).
- REQ-IMR-32: A unit test using a mock `ModeHandler` MUST verify that after `session.SetMode(mapHandler)`, `session.CurrentPrompt()` returns the map handler's prompt string (not the room prompt), simulating what `forwardServerEvents` would write.

---

## Architecture

```
commandLoop
    │
    ▼
session.HandleInput(line, conn, stream, &requestID, session)
    │
    ├─ ModeRoom    → RoomModeHandler.HandleInput   → command registry
    ├─ ModeMap     → MapModeHandler.HandleInput    → map nav logic
    ├─ ModeInventory → InventoryModeHandler (stub) → q exits to room
    ├─ ModeCharSheet → CharSheetModeHandler (stub) → q exits to room
    ├─ ModeEditor  → EditorModeHandler (stub)      → q exits to room
    └─ ModeCombat  → CombatModeHandler (stub)      → q exits to room

forwardServerEvents
    │
    ├─ RoomView event + mode != ModeRoom → session.SetMode(session.Room())
    └─ conn.WritePromptSplit(session.CurrentPrompt())
         └─ delegates to active ModeHandler.Prompt()
```

---

## Files Affected

| File | Change |
|---|---|
| `internal/frontend/handlers/game_bridge.go` | Remove `mapState`/`buildPrompt`; add `SessionInputState`; refactor `commandLoop` and `forwardServerEvents` |
| `internal/frontend/handlers/input_mode.go` | New: `InputMode` type, `ModeHandler` interface, `SessionInputState` |
| `internal/frontend/handlers/mode_room.go` | New: `RoomModeHandler` |
| `internal/frontend/handlers/mode_map.go` | Refactor from `game_bridge_mapmode.go` → `MapModeHandler` |
| `internal/frontend/handlers/mode_inventory.go` | New: stub |
| `internal/frontend/handlers/mode_charsheet.go` | New: stub |
| `internal/frontend/handlers/mode_editor.go` | New: stub |
| `internal/frontend/handlers/mode_combat.go` | New: stub |
| `internal/frontend/handlers/game_bridge_mapmode.go` | Replaced by `mode_map.go`; deleted |

---

## Non-Goals

- Full UI implementation for inventory, character sheet, editor, or combat modes (stubs only)
- Changes to the gameserver gRPC protocol
- Changes to the split-screen layout or screen dimensions
