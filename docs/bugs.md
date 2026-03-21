# Bug Tracker

## UI

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

### BUG-2: eq command displays armor item IDs instead of names
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The `eq` command displays armor slots using the item definition ID (e.g. `tactical_boots`) instead of the human-readable item name (e.g. `Tactical Boots`).
**Steps:** Equip any armor item and run `eq`; observe armor slot values show raw IDs.
**Fix:**
