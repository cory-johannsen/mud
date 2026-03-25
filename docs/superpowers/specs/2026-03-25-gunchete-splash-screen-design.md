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
- SPLASH-3: The title "GUNCHETE" MUST be rendered in a medium ASCII font (4–6 lines tall, ≤44 chars wide).
- SPLASH-4: The AK-47 art MUST be rendered in bright green ANSI color.
- SPLASH-5: The GUNCHETE title MUST be rendered in bright cyan ANSI color.
- SPLASH-6: The machete art MUST be rendered in yellow ANSI color.
- SPLASH-7: The full banner line width MUST NOT exceed 80 characters.
- SPLASH-8: The AK-47, title, and machete MUST be vertically aligned on the same rows.
- SPLASH-9: All existing content below the title (subtitle, version, login instructions) MUST remain unchanged.
- SPLASH-10: The change MUST be confined to `buildWelcomeBanner()` in `internal/frontend/handlers/auth.go`.

---

## Layout

```
[AK-47 art, ~13 chars] [GUNCHETE medium ASCII font, ~44 chars] [machete art, ~13 chars]
```

Total width target: ≤80 columns. Each weapon art block is 6 lines tall to match the title height.

### AK-47 (left, bright green, ~13 chars wide)

Recognizable features: banana-curve magazine, long barrel pointing right, stock on left.

### GUNCHETE title (center, bright cyan, ~44 chars wide)

4–6 line tall medium ASCII font. Example style:

```
  ___  _ _ _  ___  _  _ ___  ___ ___
 / __|| | | ||  _ |  |  |  _||_  |_  |
| (__||  V  || (__|  '  | __|  / /  / /
 \___| \_/ \_|\___|\_ /|___|/_/  /_/
```

### Machete (right, yellow, ~13 chars wide)

Recognizable features: long curved blade pointing left, short handle at bottom-right.

---

## Implementation

- IMPL-1: All art lines MUST be assembled as a multi-line raw string constant or string concatenation within `buildWelcomeBanner()`.
- IMPL-2: Each row of the banner MUST concatenate the AK-47 column, title column, and machete column with appropriate padding to maintain alignment.
- IMPL-3: ANSI color codes MUST use the existing `telnet.BrightGreen`, `telnet.BrightCyan`, and `telnet.Yellow` constants (or equivalents defined in `internal/frontend/telnet`).
- IMPL-4: Each color segment MUST be terminated with `telnet.Reset` to prevent color bleed.

---

## Testing

- TEST-1: A unit test MUST assert that `buildWelcomeBanner()` output contains the string "GUNCHETE".
- TEST-2: A unit test MUST assert that `buildWelcomeBanner()` output contains the bright green ANSI code.
- TEST-3: A unit test MUST assert that `buildWelcomeBanner()` output contains the yellow ANSI code.
- TEST-4: A unit test MUST assert that no line in the banner art section exceeds 80 visible characters (excluding ANSI escape sequences).

---

## Out of Scope

- Changes to the auth flow, telnet negotiation, or session handling.
- Responsive/dynamic resizing of the banner based on terminal width.
- Animation or color cycling effects.
