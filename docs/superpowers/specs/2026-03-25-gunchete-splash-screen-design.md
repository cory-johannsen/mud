# Gunchete Splash Screen Redesign (v2 — Stacked Layout)

**Date:** 2026-03-25
**Status:** Approved

---

## Overview

Replace the current three-column splash screen layout (weapons flanking title) with a stacked vertical layout: a horizontal AK-47 ASCII art block above the title, the original GUNCHETE Unicode block-letter title in the middle, and a horizontal machete ASCII art block below the title. Weapons are left-aligned to match the title's left margin, not centered in the terminal.

---

## Requirements

- SPLASH-1: The splash screen MUST display a horizontal AK-47 ASCII art block above the GUNCHETE title.
- SPLASH-2: The splash screen MUST display a horizontal machete ASCII art block below the GUNCHETE title.
- SPLASH-3: The GUNCHETE title MUST be the original Unicode block-letter art (using `█`, `╗`, `╔`, `═`, `╝`, `╚` box-drawing characters) exactly as it existed before the three-column redesign, rendered in BrightCyan + Bold.
- SPLASH-4: The AK-47 art MUST be rendered in BrightGreen using `telnet.BrightGreen`.
- SPLASH-5: The machete art MUST be rendered in BrightYellow using `telnet.BrightYellow`.
- SPLASH-6: Both weapon art blocks MUST be custom ASCII art drawn as horizontal silhouettes (weapon lying on its side), not vertical portraits.
- SPLASH-7: Both weapon art blocks MUST be approximately 60 visible characters wide, left-aligned at the same left margin as the title text (column 0 — no indent beyond what is embedded in the art itself).
- SPLASH-8: All weapon art rows MUST be ≤ 80 visible characters wide (excluding ANSI escape sequences).
- SPLASH-9: The AK-47 art MUST be 4–6 rows tall and recognizable as a rifle: long barrel, magazine curve, stock.
- SPLASH-10: The machete art MUST be 3–5 rows tall and recognizable as a blade: long straight/curved blade, short handle.
- SPLASH-11: A blank line MUST appear between the AK-47 art and the title, and between the title and the machete art.
- SPLASH-12: All existing content below the machete art (subtitle, version, login/register/quit instructions) MUST remain unchanged.
- SPLASH-13: The change MUST be confined to `buildWelcomeBanner()` in `internal/frontend/handlers/auth.go`.
- SPLASH-14: Every ANSI color sequence MUST be immediately followed by `telnet.Reset` before the next color sequence or end of line, preventing color bleed.

---

## Layout

```
[blank line]
[AK-47 horizontal art, BrightGreen, ~60 cols, left-aligned, 4–6 rows]
[blank line]
  ██████╗ ██╗   ██╗███╗   ██╗ ██████╗██╗  ██╗███████╗████████╗███████╗
 ██╔════╝ ██║   ██║████╗  ██║██╔════╝██║  ██║██╔════╝╚══██╔══╝██╔════╝
 ██║  ███╗██║   ██║██╔██╗ ██║██║     ███████║█████╗     ██║   █████╗
 ██║   ██║██║   ██║██║╚██╗██║██║     ██╔══██║██╔══╝     ██║   ██╔══╝
 ╚██████╔╝╚██████╔╝██║ ╚████║╚██████╗██║  ██║███████╗   ██║   ███████╗
  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚══════╝
[blank line]
[machete horizontal art, BrightYellow, ~60 cols, left-aligned, 3–5 rows]
[blank line]
  Post-Collapse Portland, OR — A Dystopian Sci-Fi MUD
  v0.x.x

  Type login to connect.
  Type register to create an account.
  Type quit to disconnect.
```

---

## Art Descriptions

### AK-47 (horizontal, BrightGreen, ~60 cols, 4–6 rows)

Drawn as a side-profile silhouette of a Kalashnikov-pattern rifle lying horizontally. Key recognizable features:
- Long horizontal barrel pointing right
- Curved/banana magazine below the receiver
- Rectangular receiver body
- Stock on the left end

Illustrative sketch (exact characters to be finalized in implementation plan):
```
  ,----.__________________________________________
 ( (_)  )______________________________________ [|==>
  \    /|  [_________________________]  |      |
   `--' |__________________________|____|______|
              |__|
```

### Machete (horizontal, BrightYellow, ~60 cols, 3–5 rows)

Drawn as a side-profile silhouette of a machete lying horizontally. Key recognizable features:
- Long straight-to-slightly-curved blade
- Narrow tip at one end
- Short stubby handle at the other end with a guard

Illustrative sketch (exact characters to be finalized in implementation plan):
```
   ____________________________________________/
  /____________________________________________\
 |                                              |===|
  \____________________________________________/|___|
```

---

## Implementation

- IMPL-1: All art is assembled as Go string literals within `buildWelcomeBanner()`. No external files or templates.
- IMPL-2: The banner is built in order: blank, AK-47 block (each row wrapped in BrightGreen+Reset), blank, title block (wrapped in Bold+BrightCyan+Reset — the single Reset clears both Bold and BrightCyan together), blank, machete block (each row wrapped in BrightYellow+Reset), blank, subtitle/version/commands.
- IMPL-3: ANSI constants `telnet.BrightGreen`, `telnet.BrightCyan`, `telnet.BrightYellow`, `telnet.Bold`, `telnet.Reset` MUST be used — no raw escape literals.
- IMPL-4: Each weapon art row is a separate string in a slice. The slice is iterated and each row is written as `telnet.BrightGreen + row + telnet.Reset + "\n"`.

---

## Testing

- TEST-1: A unit test MUST assert that `buildWelcomeBanner()` contains `telnet.BrightCyan` with at least 4 contiguous lines of multi-character content (the title block is present).
- TEST-2: A unit test MUST assert that `buildWelcomeBanner()` contains `telnet.BrightGreen` (AK-47 art present).
- TEST-3: A unit test MUST assert that `buildWelcomeBanner()` contains `telnet.BrightYellow` (machete art present).
- TEST-4: A unit test MUST assert that every line of `buildWelcomeBanner()` output is ≤ 80 visible characters after stripping ANSI sequences.
- TEST-5: A unit test MUST assert that every `BrightGreen`, `BrightCyan`, `BrightYellow`, or `Bold` occurrence is followed by `telnet.Reset` before the next color/attribute code or end of banner. Note: when `Bold` and `BrightCyan` are adjacent on the same title row, a single `telnet.Reset` after `BrightCyan` satisfies this requirement for both.
- TEST-6: A unit test MUST assert that the BrightGreen block appears before (earlier line index than) the first BrightCyan line — gun is above the title.
- TEST-7: A unit test MUST assert that the BrightYellow block appears after (later line index than) the last BrightCyan line — machete is below the title.

---

## Out of Scope

- Changes to auth flow, telnet negotiation, or session handling.
- Responsive or dynamic resizing based on terminal width.
- Animation or color cycling.
- Centered alignment (weapons are left-aligned, not centered in the 80-col terminal).
