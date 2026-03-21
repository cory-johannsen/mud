# Bug Tracker

## UI

### BUG-4: Room section does not display zone name
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The room display section does not show the name of the current zone, leaving players without zone context when navigating.
**Steps:** Enter any room and observe the room display; no zone name is shown.
**Fix:**

### BUG-1: Technology selection list — poor text alignment at login
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The technology selection list shown at login does not wrap and indent continuation lines correctly — the first line of each item overruns and subsequent lines are not indented to align with the start of the item text.
**Steps:** Log in and reach the technology selection prompt; observe the list of technologies with multi-line descriptions.
**Fix:**

### BUG-3: ne_portland zone — ne_prescott_street and ne_cully_road isolated from rest of map
**Severity:** high
**Status:** open
**Category:** World
**Description:** In the `ne_portland` zone, `ne_prescott_street` and `ne_cully_road` are connected to each other but have no exits linking them to the rest of the zone map, making them unreachable.
**Steps:** View the ne_portland zone map; observe the two isolated rooms.
**Fix:**

### BUG-5: Technology descriptions reference magic and PF2E saves
**Severity:** medium
**Status:** open
**Category:** Content
**Description:** Many technology item and ability descriptions use fantasy/magic flavor text and reference PF2E save types (Fortitude, Reflex, Will) instead of sci-fi/tech flavor appropriate to the game setting.
**Steps:** Browse technology items and ability descriptions; observe magic/PF2E save language.
**Fix:**

### BUG-6: Technology selection displays technology ID instead of display name
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** During character creation, the technology selection prompt displays the raw technology ID (e.g. `bio_synthetic`) instead of the human-readable display name (e.g. `Bio-Synthetic`).
**Steps:** Create a new character and reach the technology selection step; observe the technology list shows internal IDs rather than display names.
**Fix:**

### BUG-7: `switch` command does not clear console scrollback buffer
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The `switch` command does not clear the console scrollback buffer when switching characters, leaving previous session output visible.
**Steps:** Play as one character, run `switch`, observe that prior console output remains in the scrollback buffer.
**Fix:**

### BUG-8: `smooth_talk` XP reward message displays skill ID instead of display name
**Severity:** low
**Status:** open
**Category:** UI
**Description:** The XP reward message shown after a successful `smooth_talk` action displays the raw skill ID (e.g. `smooth_talk`) instead of the human-readable display name.
**Steps:** Use `smooth_talk` successfully; observe the XP reward message shows the skill ID.
**Fix:**

### BUG-2: eq command displays armor item IDs instead of names
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The `eq` command displays armor slots using the item definition ID (e.g. `tactical_boots`) instead of the human-readable item name (e.g. `Tactical Boots`).
**Steps:** Equip any armor item and run `eq`; observe armor slot values show raw IDs.
**Fix:**
