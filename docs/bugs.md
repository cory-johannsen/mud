# Bug Tracker

## UI

### BUG-4: Room section does not display zone name
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The room display section does not show the name of the current zone, leaving players without zone context when navigating.
**Steps:** Enter any room and observe the room display; no zone name is shown.
**Fix:** Added `zone_name` (field 12) to `RoomView` proto. In `buildRoomView()`, populated `ZoneName` by calling `h.world.GetZone(room.ZoneID)`. In `RenderRoomView()`, prepended `[ZoneName] ` before the room title when `rv.ZoneName` is non-empty, yielding a header of the form `[Northeast Portland] Cully Road — date period hour`.

### BUG-1: Technology selection list — poor text alignment at login
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The technology selection list shown at login does not wrap and indent continuation lines correctly — the first line of each item overruns and subsequent lines are not indented to align with the start of the item text.
**Steps:** Log in and reach the technology selection prompt; observe the list of technologies with multi-line descriptions.
**Fix:** Added `wrapOption(prefix, text string, width int) string` helper to `grpc_service.go` that word-wraps option text at 78 characters, prefixing the first line with the numbered prefix (e.g. `"  1) "`) and indenting continuation lines with spaces equal to the prefix width. Updated `promptFeatureChoice` to use `wrapOption` for each option instead of a bare `fmt.Fprintf`.

### BUG-3: ne_portland zone — ne_prescott_street and ne_cully_road isolated from rest of map
**Severity:** high
**Status:** fixed
**Category:** World
**Description:** In the `ne_portland` zone, `ne_prescott_street` and `ne_cully_road` are connected to each other but have no exits linking them to the rest of the zone map, making them unreachable.
**Steps:** View the ne_portland zone map; observe the two isolated rooms.
**Fix:** Added west exit from `ne_prescott_street` to `ne_alberta_street` (reciprocal of existing east exit). Added west exit from `ne_cully_road` to `ne_bike_shop_ruins` and east exit from `ne_bike_shop_ruins` to `ne_cully_road`.

### BUG-5: Technology descriptions reference magic and PF2E saves
**Severity:** medium
**Status:** fixed
**Category:** Content
**Description:** Many technology item and ability descriptions use fantasy/magic flavor text and reference PF2E save types (Fortitude, Reflex, Will) instead of sci-fi/tech flavor appropriate to the game setting.
**Steps:** Browse technology items and ability descriptions; observe magic/PF2E save language.
**Fix:** Bulk-replaced PF2E save names and spell/magic terminology across all 900+ technology YAML files: Fortitude save → Toughness save, Will save → Willpower save, spell/Spell → tech, Cast the Spell → Use this tech, spell DC → tech DC, spell attack roll → tech attack roll, spell level → tech level.

### BUG-6: Technology selection displays technology ID instead of display name
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** During character creation, the technology selection prompt displays the raw technology ID (e.g. `bio_synthetic`) instead of the human-readable display name (e.g. `Bio-Synthetic`).
**Steps:** Create a new character and reach the technology selection step; observe the technology list shows internal IDs rather than display names.
**Fix:** Updated `buildOptions` in `internal/gameserver/technology_assignment.go` to format options as `[id] Name — description` when the registry has a matching entry, exposing the display name to the player. Updated `parseTechID` to extract the ID from the `[id]` bracket prefix when present, falling back to the legacy `id — description` split for backward compatibility.

### BUG-7: `switch` command does not clear console scrollback buffer
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The `switch` command does not clear the console scrollback buffer when switching characters, leaving previous session output visible.
**Steps:** Play as one character, run `switch`, observe that prior console output remains in the scrollback buffer.
**Fix:** Added `ClearConsoleBuf()` to `Conn` in `internal/frontend/telnet/conn.go`. The method acquires the mutex and sets `consoleBuf = nil`, `scrollOffset = 0`, and `pendingNew = 0`. Called it in `internal/frontend/handlers/game_bridge.go` after `conn.EnableSplitScreen()` so the in-memory scrollback is wiped each time a new game session is initialized (including after `switch`).

### BUG-13: Up arrow history + Enter does not resubmit command
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Scrolling through command history with the up arrow key and pressing Enter does not resubmit the selected command.
**Steps:** Enter any command, press up arrow to recall it, press Enter; observe the command is not executed.
**Fix:** In `ReadLineSplit` (`internal/frontend/telnet/conn.go`), seed the `line` buffer from `c.inputBuf` at the start of each call (under the mutex). When history navigation sets `inputBuf` via `SetInputLine`, the next `ReadLineSplit` call (triggered by Enter) picks up the recalled command and returns it, resubmitting it as the player intended.

### BUG-8: `smooth_talk` XP reward message displays skill ID instead of display name
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The XP reward message shown after a successful `smooth_talk` action displays the raw skill ID (e.g. `smooth_talk`) instead of the human-readable display name.
**Steps:** Use `smooth_talk` successfully; observe the XP reward message shows the skill ID.
**Fix:** Added `skillDisplayName()` helper to `internal/game/xp/service.go` that converts snake_case skill IDs to Title Case. `AwardSkillCheck` now formats the grant message with the display name (e.g. "Smooth Talk" instead of "smooth_talk").

### BUG-2: eq command displays armor item IDs instead of names
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The `eq` command displays armor slots using the item definition ID (e.g. `tactical_boots`) instead of the human-readable item name (e.g. `Tactical Boots`).
**Steps:** Equip any armor item and run `eq`; observe armor slot values show raw IDs.
**Fix:** Added `hydrateEquipmentNames(eq *inventory.Equipment, reg *inventory.Registry)` in `internal/gameserver/grpc_service.go`. After `LoadEquipment` succeeds at login, this function iterates `eq.Armor` and `eq.Accessories`, looks up each `ItemDefID` via `reg.Item()`, and sets `item.Name` to `ItemDef.Name` when found. Items whose IDs are not registered remain unchanged. (Initial implementation incorrectly used `reg.Armor()` with an item ID — armor is registered under its `ArmorDef.ID`, not the item ID; the correct lookup is via `reg.Item()`.)

## World

### BUG-11: Technologies assigned at login (backfilled on existing characters) are not persisted
**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** Technologies backfilled onto existing characters at login are not saved to the database, so they are lost on the next session.
**Steps:** Log in with an existing character that lacks a technology assignment; observe the technology is backfilled; log out and back in; observe the technology is missing again.
**Fix:** The "already assigned" guard in `Session()` checked only the hardwired tech repo. Jobs with no hardwired grants (prepared/spontaneous/innate only) always appeared unassigned, so `AssignTechnologies` re-ran on every login, overwriting persisted assignments. Fixed by checking all four repos (hardwired, prepared, spontaneous, innate) before running backfill assignment. Also added a nil guard for `dbChar` on the region lookup passed to `AssignTechnologies`.

### BUG-12: Active feats do not track prepared uses and cannot be activated
**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** Active feats do not track prepared use counts and cannot be activated by the player.
**Steps:** Select an active feat during character creation; attempt to use the feat in play; observe it cannot be activated.
**Fix:** Added `PreparedUses int` field to `ruleset.Feat` (0 = unlimited). Added `ActiveFeatUses map[string]int` to `session.PlayerSession`, populated at login for all active feats with `PreparedUses > 0`. Updated `handleUse` to enforce the use count: 0 remaining returns a failure message; successful activation decrements the counter and appends the remaining count to the response. Updated `handleRest` to restore `ActiveFeatUses` to each feat's `PreparedUses` maximum. Unlimited feats (PreparedUses=0) are unaffected.

### BUG-10: rustbucket_ridge — blood_camp has an illegal placement; move east of blade_house
**Severity:** high
**Status:** fixed
**Category:** World
**Description:** In the `rustbucket_ridge` zone, `blood_camp` has an illegal map placement. It should be moved to east of `blade_house`.
**Steps:** View the rustbucket_ridge zone map; observe blood_camp placement is illegal.
**Fix:** Moved blood_camp to (5,6), one step east of blade_house (4,6). Replaced the_cutthroat→blood_camp exit with blade_house↔blood_camp exits.

### BUG-14: Postgres integration tests fail due to missing columns in test schema setup
**Severity:** high
**Status:** fixed
**Category:** Meta
**Description:** `internal/storage/postgres` integration tests fail because `applyAllMigrations` in `main_test.go` did not include `ALTER TABLE` statements added in migrations 034 and 035: `detained_until` on `characters`, durability/modifier/rarity columns on `character_equipment` and `character_weapon_presets`, `team` on `characters`, and the new `character_inventory_instances` table.
**Steps:** Run `go test ./internal/storage/postgres/...`; observe `ERROR: column "detained_until" does not exist`.
**Fix:** Added all missing `ALTER TABLE` and `CREATE TABLE` DDL statements to `applyAllMigrations` in `main_test.go`, matching migrations 034 (detained_until) and 035 (equipment mechanics schema). All postgres tests now pass.

### BUG-15: Zone map interaction populates map but unexplored rooms show grey with no POIs revealed
**Severity:** high
**Status:** open
**Category:** World
**Description:** Interacting with a zone map object (e.g., in NE Portland zone) shows the console confirmation but does not populate the player's map — unexplored rooms remain grey and POIs in unexplored rooms are not revealed.
**Steps:** Enter the NE Portland zone. Locate and interact with the zone map. Observe console confirmation message. Check map — all unexplored rooms remain grey, no POIs revealed.
**Fix:**

### BUG-16: Lua AI hooks fail with "context canceled" after instruction budget exhausted
**Severity:** critical
**Status:** fixed
**Category:** Scripting
**Description:** The `countingContext` instruction limit is set once at LState creation and never reset between hook calls; after the zone VM exhausts its total opcode budget (100,000 opcodes), every subsequent `*_has_enemy` and other AI hook invocation fails with "context canceled", effectively disabling all NPC combat AI.
**Steps:** Run the gameserver and allow combat in any zone with AI NPCs. After sufficient hook invocations the scripting manager begins emitting hundreds of "scripting: Lua runtime error … context canceled" warn-level log lines for every combat tick — observable in `kubectl logs gameserver-<pod> -n mud`.
**Root Cause:** `sandbox.go:NewSandboxedState` creates a single `countingContext` with a shared `atomic.Int64` counter and calls `L.SetContext(ctx)` once. Every opcode across every hook call over the LState's lifetime decrements the same counter. Once it reaches zero the context is permanently canceled and all future hook calls fail immediately. The fix is to reset `L.SetContext` with a fresh per-invocation `countingContext` before each hook dispatch in `manager.go:CallHook` and `CallHookWithContext`.
**Fix:** Exported NewCountingContext from sandbox.go and added a resetContext() helper on zoneState. Both CallHook and CallHookWithContext now call resetContext() (via dispatchHook) before each CallByParam, installing a fresh per-call instruction budget. The lifetime cancel is preserved on zoneState for Close/reload only.

### BUG-9: rustbucket_ridge — scorchside_camp illegally overlaps the_embers_edge
**Severity:** high
**Status:** fixed
**Category:** World
**Description:** In the `rustbucket_ridge` zone, `scorchside_camp` has an illegal map placement that overlaps with `the_embers_edge`. `scorchside_camp` should be moved south to connect to `smokers_den` instead.
**Steps:** View the rustbucket_ridge zone map; observe scorchside_camp placement conflicts with the_embers_edge.
**Fix:** Moved scorchside_camp to (4,12), south of smokers_den (4,10). Updated smokers_den to exit south→scorchside_camp and scorchside_camp to exit north→smokers_den.

### BUG-17: loadout command displays Technology item ID instead of name
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The `loadout` command output displays the raw Technology item ID (e.g., `stim_pack`) instead of the human-readable item name (e.g., `Stim Pack`).
**Steps:** Create a character, equip or load out with any Technology item, run `loadout`; observe the ID is shown instead of the display name.
**Fix:**
