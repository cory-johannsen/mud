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
**Status:** fixed
**Category:** World
**Description:** In the `ne_portland` zone, `ne_prescott_street` and `ne_cully_road` are connected to each other but have no exits linking them to the rest of the zone map, making them unreachable.
**Steps:** View the ne_portland zone map; observe the two isolated rooms.
**Fix:** Added west exit from `ne_prescott_street` to `ne_alberta_street` (reciprocal of existing east exit). Added west exit from `ne_cully_road` to `ne_bike_shop_ruins` and east exit from `ne_bike_shop_ruins` to `ne_cully_road`.

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

### BUG-13: Up arrow history + Enter does not resubmit command
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** Scrolling through command history with the up arrow key and pressing Enter does not resubmit the selected command.
**Steps:** Enter any command, press up arrow to recall it, press Enter; observe the command is not executed.
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

## World

### BUG-11: Technologies assigned at login (backfilled on existing characters) are not persisted
**Severity:** high
**Status:** open
**Category:** Character
**Description:** Technologies backfilled onto existing characters at login are not saved to the database, so they are lost on the next session.
**Steps:** Log in with an existing character that lacks a technology assignment; observe the technology is backfilled; log out and back in; observe the technology is missing again.
**Fix:**

### BUG-12: Active feats do not track prepared uses and cannot be activated
**Severity:** high
**Status:** open
**Category:** Character
**Description:** Active feats do not track prepared use counts and cannot be activated by the player.
**Steps:** Select an active feat during character creation; attempt to use the feat in play; observe it cannot be activated.
**Fix:**

### BUG-10: rustbucket_ridge — blood_camp has an illegal placement; move east of blade_house
**Severity:** high
**Status:** fixed
**Category:** World
**Description:** In the `rustbucket_ridge` zone, `blood_camp` has an illegal map placement. It should be moved to east of `blade_house`.
**Steps:** View the rustbucket_ridge zone map; observe blood_camp placement is illegal.
**Fix:** Moved blood_camp to (5,6), one step east of blade_house (4,6). Replaced the_cutthroat→blood_camp exit with blade_house↔blood_camp exits.

### BUG-9: rustbucket_ridge — scorchside_camp illegally overlaps the_embers_edge
**Severity:** high
**Status:** fixed
**Category:** World
**Description:** In the `rustbucket_ridge` zone, `scorchside_camp` has an illegal map placement that overlaps with `the_embers_edge`. `scorchside_camp` should be moved south to connect to `smokers_den` instead.
**Steps:** View the rustbucket_ridge zone map; observe scorchside_camp placement conflicts with the_embers_edge.
**Fix:** Moved scorchside_camp to (4,12), south of smokers_den (4,10). Updated smokers_den to exit south→scorchside_camp and scorchside_camp to exit north→smokers_den.
