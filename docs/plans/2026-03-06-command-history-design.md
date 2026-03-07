# Command History & Shift+Arrow Console Scroll Design

## Goal

Repurpose plain Up/Down arrows for command history navigation; move console line-scroll to Shift+Up/Shift+Down.

## Architecture

### Command History

- `Conn` gains `cmdHistory []string` (max 100 entries) and `historyIdx int` cursor.
- Every submitted command is appended; `historyIdx` resets to `len(cmdHistory)` (past-end = live position).
- `↑`: decrement `historyIdx` (clamp at 0), write recalled command into prompt input area.
- `↓`: increment `historyIdx` (clamp at `len`), write recalled command or empty string at live position.
- Input redraw: `Conn.SetInputLine(s string)` positions cursor at the prompt input area, clears the line, writes `s`, and leaves the cursor there ready for further input.

### Shift+Arrow Console Scroll

- `tryReadEscapeSeq` extended to recognize `ESC [ 1 ; 2 A` → `\x00SHIFT_UP` and `ESC [ 1 ; 2 B` → `\x00SHIFT_DOWN`.
- `game_bridge.go`: `\x00SHIFT_UP` → `conn.ScrollUpLine()`, `\x00SHIFT_DOWN` → `conn.ScrollDownLine()`.
- Plain `↑` / `↓` reassigned to history navigation.

### Input Flow

`game_bridge.go` `commandLoop` currently dispatches on sentinel strings returned by `ReadCommand`. History navigation sentinels (`\x00UP`, `\x00DOWN`) are handled in the sentinel switch before input is forwarded to the server — same pattern as the existing scroll handlers.

## Components

| Component | Change |
|-----------|--------|
| `internal/frontend/telnet/conn.go` | Add `cmdHistory`, `historyIdx`; add `AppendHistory`, `HistoryUp`, `HistoryDown`, `SetInputLine`; extend `tryReadEscapeSeq` for Shift+arrow |
| `internal/frontend/handlers/game_bridge.go` | `\x00UP` → history up + redraw; `\x00DOWN` → history down + redraw; `\x00SHIFT_UP` → `ScrollUpLine`; `\x00SHIFT_DOWN` → `ScrollDownLine` |
| `internal/frontend/telnet/conn_test.go` | Unit tests for history ring buffer and `SetInputLine` |

## Testing

- Property-based: appending N commands then navigating up N times yields them in reverse order.
- Unit: `HistoryUp` at oldest entry is a no-op; `HistoryDown` at live position returns "".
- Integration: `tryReadEscapeSeq` correctly parses `ESC [ 1 ; 2 A` as `\x00SHIFT_UP`.
