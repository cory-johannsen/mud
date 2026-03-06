# Console Scroll (PgUp/PgDn) Design

**Date:** 2026-03-06
**Status:** Approved

## Problem

Players cannot scroll back through the Console section to review recent messages. PgUp/PgDn key sequences are currently unrecognized and swallowed.

## Approach

In-memory ring buffer managed by `telnet.Conn`. The client owns all rendering — no VT100 scroll regions, consistent with the existing no-DECSTBM policy.

## Data Model

`telnet.Conn` gains three fields:

- `consoleBuf []string` — ring buffer of all console lines, max 1000 entries
- `scrollOffset int` — number of lines scrolled back from live; 0 = live view
- `pendingNew int` — count of new messages received while `scrollOffset > 0`

`WriteConsole` always appends to `consoleBuf`. When `scrollOffset == 0` it renders normally (unchanged behavior). When `scrollOffset > 0` it increments `pendingNew` and does not update the display.

## Escape Sequence Parsing

`tryReadEscapeSeq` is extended to recognize 3-byte CSI sequences:

| Sequence      | Sentinel    |
|---------------|-------------|
| ESC [ A       | `\x00UP`    |
| ESC [ B       | `\x00DOWN`  |
| ESC [ 5 ~     | `\x00PGUP`  |
| ESC [ 6 ~     | `\x00PGDN`  |

After reading `[`, if the next byte is a digit, one additional byte (`~`) is consumed to complete the sequence. Unrecognized digit sequences are swallowed.

## Console Redraw

A new `Conn` method `redrawConsole()`:

- Console height = terminal height − `RoomRegionRows` − 2 (divider row + prompt row)
- Slices `consoleBuf` to `consoleHeight` lines ending at `len(consoleBuf) - scrollOffset`
- Clears and rewrites the console region using absolute cursor positioning
- When `scrollOffset > 0`: writes a dim status line at the bottom of the console region:
  - `[scrolled back — N new messages]` when `pendingNew > 0`
  - `[scrolled back]` when `pendingNew == 0`
- When returning to live (`scrollOffset` drops to 0): clears `pendingNew`, redraws latest lines, removes status line

## Input Handling (`commandLoop`)

Sentinels handled before command parsing:

- `\x00PGUP` → `conn.ScrollUp()`: increments `scrollOffset` by `consoleHeight`, calls `redrawConsole()`
- `\x00PGDN` → `conn.ScrollDown()`: decrements `scrollOffset` by `consoleHeight` (min 0), calls `redrawConsole()`
- Any non-empty, non-sentinel command: if `scrollOffset > 0`, snap back to live first, then process

Scroll actions do **not** reset the idle timer.

## Testing

- **Unit — ring buffer**: `consoleBuf` truncates at 1000 lines (oldest dropped); `pendingNew` increments when scrolled back and clears on return to live; `ScrollUp`/`ScrollDown` clamp at buffer bounds / 0
- **Unit — escape parsing**: `ESC [ 5 ~` → `\x00PGUP`; `ESC [ 6 ~` → `\x00PGDN`; unrecognized digit sequences swallowed cleanly; existing UP/DOWN sentinels unchanged
- **Integration**: write 200 console lines, scroll up two pages, verify rendered slice is correct; write more lines while scrolled back, verify `[N new messages]` status; PgDn returns to live and clears status

All tests use TDD (SWENG-5) with property-based testing via `pgregory.net/rapid` where applicable.
