---
name: mud-ui
description: Telnet split-screen UI — screen layout, rendering, resize handling
type: reference
---

## Trigger

Invoke this skill when working on any of the following:
- Split-screen telnet layout, row positioning, or scroll behaviour
- Terminal resize / NAWS negotiation handling
- `InitScreen`, `WriteRoom`, `WriteConsole`, `WritePromptSplit`
- Room or character-sheet rendering (`RenderRoomView`, `RenderCharacterSheet`)
- ANSI color, width calculation, or text wrapping utilities

## Responsibility Boundary

This skill covers the **presentation layer** only:

- `internal/frontend/telnet/` — low-level telnet protocol, ANSI helpers, screen writes
- `internal/frontend/handlers/text_renderer.go` — high-level proto-to-string rendering
- `internal/frontend/handlers/game_bridge.go` — resize loop, `lastRoomView` store

Out of scope: game logic (`internal/gameserver/`), gRPC transport, command dispatch.

## Key Files

| File | Purpose |
|------|---------|
| `internal/frontend/telnet/screen.go` | `InitScreen`, `WriteRoom`, `WriteConsole`, `WritePromptSplit`, `appendRoomRedraw`, `wrapText` (internal) / `WrapText` (exported), scroll state |
| `internal/frontend/telnet/conn.go` | `Conn` struct, NAWS parsing (`handleIAC`), `AwaitNAWS`, `ReadLineSplit`, `ResizeCh` |
| `internal/frontend/telnet/ansi.go` | ANSI color constants, `Colorize`, `Colorf`, `StripANSI` |
| `internal/frontend/handlers/text_renderer.go` | `RenderRoomView(rv, width, maxLines)`, `RenderCharacterSheet(csv, width)` |
| `internal/frontend/handlers/game_bridge.go` | Session loop, resize goroutine, `lastRoomView atomic.Value` |

## Core Data Structures

```go
// telnet.Conn — per-connection state (all fields guarded by mu)
type Conn struct {
    width, height int          // from NAWS negotiation
    splitScreen   bool
    inputBuf      string       // partial user input for redraw
    roomBuf       string       // last rendered room string for redraw after scroll
    consoleBuf    []string     // ring buffer, max 1000 lines
    scrollOffset  int          // 0 = live view
    pendingNew    int          // lines received while scrolled back
    resizeCh      chan struct{} // capacity-1; signals NAWS update
    cmdHistory    []string     // max 100 entries
}

// roomLayout — derived from terminal height h
type roomLayout struct {
    firstRow   int // 1
    lastRow    int // RoomRegionRows (10)
    dividerRow int // RoomRegionRows + 1 (11)
    consoleTop int // RoomRegionRows + 2 (12)
    promptRow  int // h (terminal height)
}
```

`RoomRegionRows = 10` is an exported constant so `text_renderer.go` can enforce the same limit.

## Primary Data Flow

### Connection setup and NAWS negotiation

1. Client connects via TCP; `NewConn` wraps the socket.
2. `Negotiate()` sends `IAC WILL SuppressGoAhead`, `IAC DO NAWS`, `IAC DONT Linemode`.
3. `AwaitNAWS(timeout)` reads bytes until an `IAC SB NAWS` subnegotiation arrives, parsing 4-byte `(W-hi, W-lo, H-hi, H-lo)` into `conn.width` / `conn.height`.
4. Subsequent NAWS updates (terminal resize) are parsed in `handleIAC` called from `ReadLine` / `ReadLineSplit`; a signal is sent to `conn.resizeCh`.

### Screen initialisation

5. `game_bridge.go` calls `conn.InitScreen()` after NAWS is resolved.
6. `InitScreen` sends `\033[?25l` (hide cursor), `\033[2J\033[H` (clear + home), clears rows 1–`RoomRegionRows` with `\033[2K\r\n`, clears the divider row, positions to `promptRow`, then `\033[?25h` (show cursor). **No DECSTBM is ever sent.**

### Room entry / render

7. Server emits a `RoomView` proto event.
8. `game_bridge.go` calls `RenderRoomView(rv, w, telnet.RoomRegionRows)` → ANSI string.
9. `conn.WriteRoom(rendered)` stores the string in `conn.roomBuf`, calls `appendRoomRedraw` (positions to `\033[1;1H`, writes rows 1–`RoomRegionRows`, writes divider), then moves cursor to `promptRow`.
10. `lastRoomView.Store(rv)` saves the raw proto for resize re-render.

### Console output

11. Server emits any non-RoomView event (combat, speech, system message).
12. `game_bridge.go` calls `conn.WriteConsole(text)`.
13. `WriteConsole` word-wraps the text with `wrapText(text, w)`, appends each line to `consoleBuf` via `appendConsoleLine`.
14. Positions cursor at `promptRow`, clears line, emits `\r\n` per wrapped line (each `\r\n` at `promptRow = h` scrolls the full-screen region up one row), emits one extra `\r\n` to push last line above prompt.
15. Calls `appendRoomRedraw` to restore room rows 1–11 that were shifted up by the scroll.
16. Moves cursor to `promptRow`, redraws `inputBuf`.

### Terminal resize

17. NAWS arrives → `handleIAC` updates `conn.width/height`, sends to `resizeCh`.
18. Resize goroutine in `game_bridge.go` receives from `resizeCh`, reads new `(rw, rh)`.
19. Calls `conn.InitScreen()` → full redraw.
20. Loads `lastRoomView` (raw proto), calls `RenderRoomView(rv, rw, telnet.RoomRegionRows)` → `conn.WriteRoom(...)` to re-render at the correct new width.

### Prompt

21. `conn.WritePromptSplit(prompt)` writes `\r\033[2K<prompt><inputBuf>` in-place at `promptRow`.

## Invariants & Contracts

- **NEVER use DECSTBM** (`\033[top;botR`) — TinTin++ applies DECOM-like coordinate offsets when a non-1-based scroll region is active, silently shifting all absolute-position writes `scrollTop` rows down into the scrolling area. The default full-screen scroll region (1..h) is kept at all times.
- `appendRoomRedraw` always positions via `\033[1;1H` (absolute row 1) — safe because no custom scroll region is ever set.
- `RenderRoomView(rv, width, maxLines)`: wraps description to `width` columns, renders exits 4 per row with `"Exits: "` label, locked exits shown as `direction*`, active feats shown as `[A]`. Hard-capped at `maxLines` rows (= `RoomRegionRows`).
- `RenderCharacterSheet(csv, width)`: two-column layout when `width >= 73`, single-column otherwise.
- `game_bridge.go` stores `*gamev1.RoomView` (not the rendered string) in `lastRoomView` so it can re-render at the correct terminal width after a resize.
- `consoleBuf` is a ring buffer capped at 1000 lines; oldest entries are dropped.
- `scrollOffset == 0` means live view; `scrollOffset > 0` means user has scrolled back and new lines increment `pendingNew` only (no screen update until scroll returns to live).
- `roomBuf` (the rendered string) is cached in `Conn` for `redrawConsole` to restore the room region during scroll operations without a gRPC round-trip.

## Extension Points

- **New room content sections**: add rendering logic inside `RenderRoomView` before the `maxLines` trim.
- **New console-area widgets** (e.g. minimap): add new layout rows by increasing `RoomRegionRows` and adjusting `roomLayout`.
- **Colour themes**: all colours are constants in `ansi.go`; swap them without touching rendering logic.
- **Scrollback depth**: change `consoleBufMax` in `conn.go`.

## Common Pitfalls

- Do not send `\033[?47h` (alternate screen) or any `\033[r` scroll-region sequence — this breaks TinTin++ clients as described above.
- Do not measure string length with `len(s)` for visual width — ANSI escapes inflate byte count. Use `visualWidth(s)` (unexported) or `telnet.StripANSI`.
- When adding a new write path that scrolls content, always follow with `appendRoomRedraw` to restore the room region.
- `WriteConsole` skips the screen write when `scrollOffset > 0` — only `appendConsoleLine` is called. Callers must not assume the write is visible when the user has scrolled back.
- `conn.roomBuf` stores the pre-rendered string; `lastRoomView` stores the raw proto. Both must be kept in sync: update `lastRoomView` whenever a new `RoomView` proto is received, and let `WriteRoom` update `roomBuf` automatically.
