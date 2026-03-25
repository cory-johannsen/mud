# Gunchete Splash Screen Redesign

**Date:** 2026-03-25
**Status:** Approved

---

## Overview

Replace the existing block-letter GUNCHETE ASCII title on the connection splash screen with a new three-column layout: an AK-47 ASCII weapon on the left, the word "GUNCHETE" rendered in a medium ASCII font in the center, and a machete ASCII weapon on the right. All elements are colored with ANSI escape codes and fit within an 80-column terminal.

---

## Requirements

- SPLASH-1: The splash screen MUST display ASCII art of an AK-47 to the left of the title.
- SPLASH-2: The splash screen MUST display ASCII art of a machete to the right of the title.
- SPLASH-3: The title "GUNCHETE" MUST be rendered in a medium ASCII font that is exactly 4–6 lines tall and fits within the 52-character center column budget defined in SPLASH-7.
- SPLASH-4: The AK-47 art MUST be rendered in bright green using `telnet.BrightGreen`.
- SPLASH-5: The GUNCHETE title MUST be rendered in bright cyan using `telnet.BrightCyan`.
- SPLASH-6: The machete art MUST be rendered in bright yellow using `telnet.BrightYellow`, consistent with the bright-tier colors used for the other elements.
- SPLASH-7: The banner MUST use exactly three fixed-width columns: AK-47 padded to 14 chars, title padded to 52 chars, machete padded to 14 chars (14 + 52 + 14 = 80). Each row MUST be exactly 80 visible characters (excluding ANSI escape sequences).
- SPLASH-8: All three columns MUST be rendered to the same number of rows. Shorter columns MUST be padded with blank lines at the bottom to match the tallest column.
- SPLASH-9: All existing content below the title art (subtitle, version, login instructions) MUST remain unchanged.
- SPLASH-10: The change MUST be confined to `buildWelcomeBanner()` in `internal/frontend/handlers/auth.go`.
- SPLASH-11: Every ANSI color sequence MUST be immediately followed by `telnet.Reset` before the next color sequence or end of line, preventing color bleed.

---

## Layout

```
[AK-47, 14 chars] [GUNCHETE medium ASCII font, 52 chars] [machete, 14 chars]
```

Total width: exactly 80 visible characters per row.

### AK-47 (left column, bright green, 14 chars wide)

Recognizable features: banana-curve magazine, long barrel pointing right, stock on left. Padded with trailing spaces to exactly 14 visible characters per row.

### GUNCHETE title (center column, bright cyan, 52 chars wide)

4–6 line tall medium ASCII font. The following is illustrative only (not the required output):

```
  ___  _ _ _  ___  _  _ ___  ___ ___
 / __|| | | ||  _ |  |  |  _||_  |_  |
| (__||  V  || (__|  '  | __|  / /  / /
 \___| \_/ \_|\___|\_ /|___|/_/  /_/
```

Each row MUST be padded with trailing spaces to exactly 52 visible characters.

### Machete (right column, bright yellow, 14 chars wide)

Recognizable features: long curved blade pointing left, short handle at bottom-right. Padded with trailing spaces to exactly 14 visible characters per row.

---

## Implementation

- IMPL-1: All art lines MUST be assembled as a Go string concatenation within `buildWelcomeBanner()`, building each row by concatenating the colored left column, colored center column, and colored right column with resets between colors.
- IMPL-2: Each row MUST be constructed as: `BrightGreen + leftCol[i] + Reset + BrightCyan + centerCol[i] + Reset + BrightYellow + rightCol[i] + Reset`.
- IMPL-3: ANSI color codes MUST use `telnet.BrightGreen`, `telnet.BrightCyan`, and `telnet.BrightYellow` constants from `internal/frontend/telnet`.
- IMPL-4: Each color segment MUST be terminated with `telnet.Reset` before the next color code or end of line.

---

## Testing

- TEST-1: A unit test MUST assert that `buildWelcomeBanner()` output contains `telnet.BrightCyan` followed within the same segment by at least 4 lines of multi-character ASCII art content (verifying the title art block is present, not just the word "GUNCHETE" in prose).
- TEST-2: A unit test MUST assert that `buildWelcomeBanner()` output contains `telnet.BrightGreen` (AK-47 color present).
- TEST-3: A unit test MUST assert that `buildWelcomeBanner()` output contains `telnet.BrightYellow` (machete color present).
- TEST-4: A unit test MUST assert that every line of `buildWelcomeBanner()` output contains at most 80 visible characters, where visible characters are counted after stripping all ANSI escape sequences (pattern `\x1b\[[0-9;]*m`).
- TEST-5: A unit test MUST assert that every occurrence of `telnet.BrightGreen`, `telnet.BrightCyan`, or `telnet.BrightYellow` in the output is followed by `telnet.Reset` before the next color code or end of the full banner string.

---

## Out of Scope

- Changes to the auth flow, telnet negotiation, or session handling.
- Responsive or dynamic resizing of the banner based on terminal width.
- Animation or color cycling effects.
- Dim-yellow (`telnet.Yellow`) — bright yellow (`telnet.BrightYellow`) is used throughout for consistency.
