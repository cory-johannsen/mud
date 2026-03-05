# Telnet UI Upgrade Design

**Date:** 2026-03-04

## Goal

Upgrade the telnet frontend from a pure scrolling stream to a three-region split-screen layout: room display pinned at top, scrolling console in the middle, prompt+input pinned at bottom. Supports standard VT100 terminal emulators and TinTin++.

---

## Section 1: Terminal Negotiation & Screen Model

On connection the server sends `IAC DO NAWS` (Telnet option 31). The client responds with `IAC SB NAWS <W-hi> <W-lo> <H-hi> <H-lo> IAC SE`. Parsed in `Conn.Negotiate()`; `Width` and `Height` stored on `Conn`. Mid-session resize triggers re-parse and full redraw.

Screen layout (H = terminal height, W = terminal width):

```
Row 1..6     Room region       (fixed 6 rows, truncated/padded)
Row 7        Top divider       (═ × W)
Row 8..H-2   Console region    (VT100 scroll region \033[8;{H-2}r)
Row H-1      Bottom divider    (═ × W)
Row H        Prompt + input
```

VT100 scroll region `\033[8;{H-2}r` confines console output so it can never overwrite the room or input rows. Room region height of 6 rows is a constant — content is truncated/wrapped to fit.

**Graceful degradation:** If NAWS times out or reports `0×0`, fall back to existing scrolling-stream mode with no cursor positioning.

---

## Section 2: Room Display (pinned top)

Triggered by: `RoomViewEvent` (room entry, explicit `look`, combat state change).

Render sequence:
1. Save cursor: `\033[s`
2. Move to row 1: `\033[1;1H`
3. Clear 6 room rows: 6× (`\033[2K` + `\033[1B`)
4. Return to row 1, write room content (name + description + exits), wrapped to Width, truncated to 6 rows
5. When in combat, append `[IN COMBAT]` indicator to room name line
6. Draw top divider at row 7: `\033[7;1H` + `═` × Width
7. Restore cursor: `\033[u`

Room content comes from the existing `text_renderer.go` `RenderRoomView` output, reformatted to fit the fixed region.

---

## Section 3: Console Output

All non-room server events (combat, chat, skill checks, system messages) write into the VT100 scroll region. For each console message:

1. Move to `\033[{H-2};1H` (bottom of console region)
2. Print `\r\n` to scroll region up one line
3. Write message text, wrapped to Width
4. Redraw bottom divider: `\033[{H-1};1H` + `═` × Width
5. Redraw prompt: `\033[{H};1H` + prompt string
6. Restore cursor to end of current input: `\033[{H};{promptLen+inputLen+1}H`

Player's partially-typed input is preserved across all server events.

---

## Section 4: Input Region & Arrow Key Support

`WritePrompt` positions cursor at `\033[{H};1H`, writes prompt, leaves cursor for typing.

`Conn` gains an `inputBuf string` field tracking the player's current partially-typed line, redrawn after each server event.

`ReadLine` extended to detect VT100 escape sequences:
- `\033[A` (up arrow) → returns sentinel `"\x00UP"`
- `\033[B` (down arrow) → returns sentinel `"\x00DOWN"`
- Other escape sequences ignored/swallowed

Selection prompts in `character_flow.go` (feat/feature choices, etc.) check for sentinels and move a highlighted selection cursor through the option list, re-rendering in place using save/restore cursor.

---

## Section 5: Initialization, Resize & Graceful Degradation

**On connect (after NAWS):**
1. `\033[?25l` — hide cursor
2. `\033[2J\033[H` — clear screen, home
3. `\033[8;{H-2}r` — set scroll region
4. Draw top divider (row 7) and bottom divider (row H-1)
5. `\033[?25h` — show cursor
6. Render initial room view
7. Write initial prompt at row H

**On resize (new NAWS mid-session):**
- Store new Width/Height, repeat initialization sequence, re-render current room and prompt

**Graceful degradation:**
- If NAWS not received within 1 second of connect, or dimensions are `0×0`: skip all cursor/region setup, operate in legacy scrolling-stream mode

---

## Out of Scope

- Custom game client (Ebiten — separate feature)
- Terminal type negotiation (TTYPE)
- Mouse support
- Colour scheme configuration
- Scrollback buffer management
