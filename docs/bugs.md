# Bug Tracker

## UI

### BUG-24: StripANSI passes through incomplete ANSI escape sequences
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** `StripANSI` does not strip partial ANSI escape sequences (e.g. a trailing `\x1b[` with no terminator), causing `WriteConsole` on headless connections to emit raw escape bytes when the message contains an unterminated sequence.
**Steps:** `TestHeadlessConn_WriteConsole_NeverEmitsANSI` rapid property test reproduces: draw a string ending in `\x1b[`, call `WriteConsole`, observe `\x1b[` in output.
**Fix:** When the inner loop reaches end-of-string without finding `m`, set `i = j` (skip the entire partial sequence) instead of falling through and emitting the raw bytes.

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

## Combat

### BUG-32: Post-combat movement blocked; reconnect shows "already logged in"
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** After combat ends, the player cannot move between rooms, and logging out and back in produces "That character is already logged in" — the session is stuck in combat mode and the old session is not cleaned up on disconnect.
**Steps:** Enter combat with any NPC; let combat end (NPC dies or flees); attempt a movement command (e.g. `north`) — movement is blocked; disconnect and reconnect — observe "That character is already logged in."
**Fix:** Three fixes: (1) Added `h.stopTimerLocked(roomID)` in `resolveAndAdvanceLocked` combat-end block so `IsRoomInCombat` returns false after combat ends. (2) Changed `AddPlayer` in session manager to evict stale sessions instead of rejecting duplicate UIDs, fixing the "already logged in" error on reconnect. (3) The `onCombatEndFn` callback (wired in grpc_service.go) already resets `sess.Status` to idle — verified working via TDD. Five regression tests added in `combat_handler_end_test.go`.

### BUG-25: Allied faction NPC attacks Team Machete player on sight
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Marshal Ironsides (a Machete-faction NPC) initiates combat against a Team Machete player on room entry, indicating the NPC aggression check does not correctly exclude allied-team players.
**Steps:** Log in as a Team Machete character; enter a room containing Marshal Ironsides; observe "[Morning] Marshal Ironsides attacks you — attacked on sight."
**Root Cause:** Two compounding issues:
1. `content/npcs/marshal_ironsides.yaml` has no `disposition` field and no `faction_id` field. The NPC instance initializer in `internal/game/npc/instance.go:299-303` defaults any NPC with an empty `Disposition` to `"hostile"`.
2. The threat-assessment block in `internal/gameserver/grpc_service.go:4261` sets `isHostileToPlayers = true` directly from `inst.Disposition == "hostile"`. The faction-enemy check on lines 4262-4268 only runs when `!isHostileToPlayers`, so it is **never reached** for Marshal Ironsides. There is no allied-faction exclusion anywhere in the path: the code checks whether the player is an *enemy* of the NPC's faction but never checks whether the player is an *ally*, so even a correctly-configured faction NPC with a non-hostile disposition would still have no protection against attacking same-faction players.
**Fix:** Two-part fix. (1) Added `IsAllyOf(*session.PlayerSession, string) bool` to `faction.Service` — returns true iff both faction IDs are non-empty and equal. (2) In the threat-assessment block in `grpc_service.go`, added an allied-faction exclusion pass before the existing enemy-faction promotion pass: if any player in the room is an ally of the NPC, `isHostileToPlayers` is suppressed to false, preventing combat initiation regardless of disposition default. Also corrected `content/npcs/marshal_ironsides.yaml` to set `disposition: neutral` and `faction_id: machete`.

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
**Status:** fixed
**Category:** World
**Description:** Interacting with a zone map object (e.g., in NE Portland zone) shows the console confirmation but does not populate the player's map — unexplored rooms remain grey and POIs in unexplored rooms are not revealed.
**Steps:** Enter the NE Portland zone. Locate and interact with the zone map. Observe console confirmation message. Check map — all unexplored rooms remain grey, no POIs revealed.
**Fix (commit 8267ef39):** `handleActivate` in `grpc_service_activate.go` was calling `s.scriptMgr.CallHook(zoneID, result.Script)` without forwarding the player `uid` as a Lua argument. The zone map Lua hook `zone_map_use(uid)` received nil for `uid`, so `engine.map.reveal_zone(uid, zoneID)` silently failed to resolve the player session. Fixed by passing `lua.LString(uid)` to `CallHook`.

**Root cause (still broken after 8267ef39):** Zone-specific Lua scripts (e.g., `content/scripts/zones/downtown/zone_map.lua`) were never loaded into the scripting manager. `NewManagerFromDirs` in `internal/scripting/providers.go` only loaded global condition and AI scripts; it never called `mgr.LoadZone` for any zone subdirectory. Consequently, `CallHook("downtown", "zone_map_use", uid)` found no VM for the zone, fell back to `__global__`, found no `zone_map_use` there either, and returned `LNil` without calling `reveal_zone`. Fixed by scanning `<scriptRoot>/zones/*/` and calling `mgr.LoadZone(zoneID, dir)` for each subdirectory in `NewManagerFromDirs`.

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
**Status:** fixed
**Category:** UI
**Description:** The `loadout` command output displays the raw Technology item ID (e.g., `stim_pack`) instead of the human-readable item name (e.g., `Stim Pack`).
**Steps:** Create a character, equip or load out with any Technology item, run `loadout`; observe the ID is shown instead of the display name.
**Fix:** Updated `FormatPreparedTechs` to accept a `*technology.Registry` parameter and look up the display name via `reg.Get(slot.TechID)`; falls back to raw TechID if registry is nil or tech not found. Updated call site in `grpc_service.go` to pass `s.techRegistry`.

### BUG-18: use command displays technology IDs instead of display names
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The `use` command output displays raw technology IDs instead of human-readable display names.
**Steps:** Run `use` with no argument to list available active abilities; observe technology IDs shown instead of display names.
**Fix:** In `handleUse` (`internal/gameserver/grpc_service.go`), added `s.techRegistry.Get(techID)` display name lookups for prepared, spontaneous, and innate tech entries when building the no-arg ability list. The `Name` field and description strings now use the resolved display name when the registry is available, falling back to the raw ID otherwise.

### BUG-20: Players can use the map command while in combat
**Severity:** medium
**Status:** fixed
**Category:** Combat
**Description:** Players are able to use the `map` command while engaged in combat, which should not be permitted.
**Steps:** Initiate combat with any NPC; while in combat, issue the `map` command; observe that the map is displayed.
**Fix:** In `handleMap` (`internal/gameserver/grpc_service.go`), added a `statusInCombat` guard immediately after the player session lookup. When `sess.Status == statusInCombat`, the function returns `"You cannot use the map while in combat."` before any map logic runs. A dedicated test `TestHandleMap_BlockedInCombat` in `grpc_service_map_test.go` verifies the guard.

### BUG-19: Players can move rooms while in combat
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Players are able to use movement commands to change rooms while engaged in combat, which should not be permitted.
**Steps:** Initiate combat with any NPC; while in combat, issue a movement command (e.g., `north`); observe that the player is moved out of the room.
**Fix:** Added an in-combat guard at the top of `handleMove` in `internal/gameserver/grpc_service.go`. When `sess.Status == statusInCombat`, the handler returns a message event "You cannot move while in combat." and aborts before any room transition logic. Tests added in `grpc_service_move_test.go`.

### BUG-22: Automatic health recharge should be disabled in combat
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Health regeneration continues to tick during combat, which should be suspended while the player is engaged in an encounter.
**Steps:** Initiate combat with any NPC; observe that health recharges automatically during the fight.
**Fix:** Added an in-combat guard in `regenPlayers` in `internal/gameserver/regen.go`. When `sess.Status == inCombatStatus` (value 2), the player is skipped for that tick. Test `TestRegenManager_SkipsPlayerInCombat` in `regen_test.go` verifies the behaviour.

### BUG-21: Welcome screen AK-47 grip/clip not visible; machete guard/handle too small
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** On the welcome/splash screen, the AK-47 ASCII art is truncated — the grip and clip at the bottom are not rendered; the machete blade looks correct but the guard and handle are disproportionately small.
**Steps:** Connect to the MUD and observe the splash screen banner; note the AK-47 appears too short (missing lower body) and the machete guard/handle area is tiny relative to the blade.
**Fix:** Added two rows to the AK-47 art showing the pistol grip and magazine/clip below the receiver. Widened the machete guard/handle from 3 chars to 10 chars for proportional visibility.

## Character

### BUG-23: Character selection list does not show character team
**Severity:** medium
**Status:** fixed
**Category:** Character
**Description:** The character selection screen omits the team field, so players cannot see which team a character belongs to before selecting it.
**Steps:** Connect to the MUD and reach the character selection prompt; observe that listed characters show no team information.
**Fix:** Added `[team]` to `FormatCharacterSummary` in `internal/frontend/handlers/character_flow.go`. Character list now shows e.g. `Zara — Lvl 1 ganger from the Northeast [gun]`. Also fixed `handleChar`: `CharacterSheetView.Team` was populated from `sess.Team` but then unconditionally overwritten by `s.jobRegistry.TeamFor(sess.Class)`, which returns `""` for jobs without an explicit team affiliation. Fixed to only overwrite when the registry returns a non-empty team.

### BUG-27: Zone map exposes danger level and POIs for unexplored rooms
**Severity:** medium
**Status:** fixed
**Category:** World
**Description:** Using the Zone Map reveals danger level and points of interest for all rooms in the zone, including rooms the player has never visited; only room existence should be revealed — danger level and POIs must require the player to actually explore each room.
**Root Cause:** `AutomapCache` conflates map-revealed rooms (via `wireRevealZone`) with physically-visited rooms (via travel). `handleMap()` iterates `AutomapCache` and unconditionally populates `DangerLevel` and `Pois` for every room regardless of how it was discovered (`internal/gameserver/grpc_service.go:5599-5621`). Fix: add an `explored` column to `character_map_rooms`, introduce `ExploredCache` on `PlayerSession`, and gate danger level / POI output on `ExploredCache` membership.
**Plan:** `docs/superpowers/plans/2026-03-26-bug27-zone-map-exploration-gating.md`
**Steps:** Obtain a zone_map item, use it, and observe the map output; rooms the character has never entered show danger level and POI details that should be hidden until explored.
**Fix:** Added `explored` boolean column to `character_map_rooms` (migration 050). Updated `AutomapRepository.Insert/BulkInsert` to accept `explored bool`; `LoadAll` now returns `AutomapResult` with `AllKnown` and `ExploredOnly` maps. Added `ExploredCache map[string]map[string]bool` to `PlayerSession`. Login populates both caches from DB. Travel and room entry set `ExploredCache` and pass `explored=true` to Insert. `wireRevealZone` passes `explored=false`. `handleMap` gates `DangerLevel` and `Pois` on `ExploredCache` membership.

### BUG-31: AP not displayed to player during combat
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Action Points are never shown to the player — neither at the start of each round nor after spending AP on an action — leaving the player unable to make informed decisions about what actions they can take.
**Steps:** Enter combat with any NPC; observe the round-start message and any action confirmation messages — no AP total or remaining AP is displayed at any point.
**Fix:** Added per-player AP notification at round start via `pushMessageToUID` ("You have N AP this round.") before auto-queue runs. Added AP remaining to confirm event narratives for Attack, Strike, and Aid actions. Added AP remaining push messages for Reload, FireBurst, FireAutomatic, and Throw actions. Two regression tests in `combat_handler_ap_display_test.go`.

### BUG-30: NE Portland zone map has multiple disconnected sections
**Severity:** medium
**Status:** fixed
**Category:** World
**Description:** The NE Portland zone map renders as several isolated clusters of rooms with no connecting paths between them, indicating missing or broken exit links in `content/zones/ne_portland.yaml`.
**Steps:** Obtain a zone_map item in NE Portland and use it; observe that the map shows multiple disconnected room groups rather than a single connected graph.
**Fix:** Root cause was incorrect MapX/MapY coordinates and wrong exit directions in `ne_portland.yaml`. Two rooms had odd-numbered X coordinates (killingsworth_road X=1, alberta_ruins X=3) which inserted phantom grid columns breaking adjacent-room detection for all other rooms. Several rooms had exit directions that didn't match their actual coordinate relationship (e.g., brewery NE→williams should be SE, williams SW→brewery should be NW, rose_city_market S→killingsworth should be E since they share the same Y). Fixed 7 room coordinates, 7 exit directions, and added 1 reciprocal exit (killingsworth W→rose_city_market). Added regression tests `TestNEPortlandZone_MapVisuallyConnected` and `TestNEPortlandZone_NoCoordinateOverlap`.

### BUG-29: Range to target not displayed during combat
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** The player's current range to their combat target is never shown, leaving them unable to determine whether their equipped weapon can reach the NPC or whether they need to close/increase distance.
**Steps:** Initiate combat with any NPC; observe the combat output — no range information is displayed at any point during the encounter.
**Fix:** Range events were already generated at round 2+ start (in `resolveAndAdvanceLocked`) but missing from round 1 combat initiation. Added range events to `startCombatLocked` using the same `bestNPCCombatant`/`PosDist` pattern. Regression test in `combat_handler_range_init_test.go`.

### BUG-28: Grinder's Row zone exit (west to Cully Road) missing from map
**Severity:** low
**Status:** fixed
**Category:** World
**Description:** The west exit from Grinder's Row in Ruskbucket Ridge leads to Cully Road but does not appear on the zone map.
**Steps:** Navigate to Grinder's Row in Ruskbucket Ridge; use the zone map or examine exits; observe the west exit to Cully Road is absent from the map display despite being traversable.
**Fix:** `RenderMap` in `internal/frontend/handlers/text_renderer.go` only rendered east stubs (">") for the last column and south stubs (".") for the last row, but had no equivalent handling for west exits at the leftmost column. Grinder's Row sits at `map_x: 0` (the minimum x) and its west exit leads to `ne_cully_road` in a different zone, so no in-zone tile exists to the left. Added west-stub rendering: when any tile at the minimum x has a west exit with no in-zone western neighbor, a "<" stub is prepended to that tile's row (and a " " spacer is prepended to rows without a west exit), keeping all rows aligned. The same stub is propagated to POI suffix rows and south connector rows for visual consistency. Added `TestRenderMap_WestStub_CrossZoneExit` to enforce the invariant.

### BUG-26: Zone map rooms are not marked safe in most zones
**Severity:** medium
**Status:** fixed
**Category:** World
**Description:** The room containing the `zone_map` equipment item should have `danger_level: safe` in every zone, but 14 of 16 zones are missing this designation — players can be attacked or have traps triggered while using the zone map.
**Steps:** Inspect any zone YAML under `content/zones/`; find the room with `item_id: zone_map`; observe it either inherits a non-safe zone danger level or has a non-safe level set explicitly. Affected zones: vantucky, the_couve, lake_oswego, aloha, se_industrial, sauvie_island, ross_island, pdx_international, downtown, felony_flats, hillsboro, troutdale, ne_portland, battleground.
**Fix:** Added `danger_level: safe` to the zone_map room in all 14 affected zone YAML files. Added `TestZoneMapRoomIsSafe` in `internal/game/world/noncombat_coverage_test.go` to enforce this invariant going forward.

## Vendor

### BUG-33: Vendor wares display item ID instead of display name
**Severity:** low
**Status:** fixed
**Category:** Vendor
**Description:** Vendor shop listings show the raw item definition ID (e.g., `stim_pack`) instead of the human-readable display name (e.g., `Stim Pack`).
**Steps:** Visit any vendor NPC and browse their wares; observe that items are listed by internal ID rather than display name.
**Fix:** Resolved `row.ItemID` to display name via `s.invRegistry.Item()` in `grpc_service_merchant.go` `handleBrowse`. Falls back to raw ID when registry is nil or item not found.

## UI

### BUG-34: Help command output is not alphabetized within categories
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The `help` command lists commands in an arbitrary order within each category; commands should be sorted alphabetically for easier scanning.
**Steps:** Run `help`; observe that commands within each category are not in alphabetical order.
**Fix:** Added `sort.Slice` by `cmd.Name` in `Registry.CommandsByCategory()` so each category's commands are returned alphabetically.

### BUG-35: Trainer NPC requires player to know job name instead of offering a menu
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Trainer NPC interactions require the player to already know the job they want to train; the trainer should present a menu of available jobs to choose from, or inform the player if none are available.
**Steps:** Interact with a Trainer NPC; observe that the player must type the exact job name with no prompt or list of options offered.
**Fix:** `train <npc>` with no job now lists the trainer's offered jobs with cost and eligibility status. Bridge handler updated to allow omitting the job argument.

### BUG-36: look &lt;direction&gt; produces no console output
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The `look <direction>` command is accepted without error but produces no console output, giving the player no information about what lies in the specified direction.
**Steps:** Enter any room with exits; type `look north` (or any valid direction); observe that no output is displayed in the console.
**Fix:** Root cause was `bridgeLook` ignoring parsed args entirely — it always sent a bare `LookRequest` to the server. Added directional look handling in `bridgeLook`: when args contain a direction (including aliases like "n"), the handler resolves it against the cached `lastRoomView` exits and returns a local description ("Looking north: Town Square.") with locked status if applicable. Added `roomViewFn` to `bridgeContext` and `consoleMsg` to `bridgeResult` to support local output. Five regression tests in `bridge_handlers_test.go`.

### BUG-40: Merchant items display as raw IDs instead of display names
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Some merchant items show raw item IDs (e.g. `sawed_off_shotgun`, `pipe_pistol`) instead of display names because the item IDs referenced in NPC YAML files don't match any registered item definitions.
**Steps:** Browse Sergeant Mack's wares; observe `sawed_off_shotgun` and `pipe_pistol` listed instead of proper names.
**Fix:** Two root causes: (1) `sawed_off_shotgun` referenced in `sergeant_mack.yaml`, `gang_enforcer.yaml`, and `vantucky_scavenger.yaml` but the canonical item/weapon ID is `sawn_off` — updated all three files. (2) `pipe_pistol` referenced but no item/weapon definition existed — created `content/weapons/pipe_pistol.yaml` and `content/items/pipe_pistol.yaml`.

### BUG-39: Battle map shows absolute positions instead of distance between combatants
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The battle map renders each combatant's absolute position on the battlefield axis (e.g. `[*Jorkin:25ft]───[Ganger:25ft]`), not the distance between them. When both combatants are at position 25ft they are at 0ft distance (melee range), but the display looks like they are 25ft apart, causing player confusion.
**Steps:** Start combat (player at 0ft, NPC at 25ft); stride toward NPC; observe map shows `[*Jorkin:25ft]───[Ganger:25ft]` which appears to still show 25ft separation but they are actually at melee range.
**Fix:** Changed `RenderBattlefield` to show distance between adjacent combatants on the separator instead of absolute positions in each token. New format: `[*Jorkin]──0ft──[Ganger]` for melee (0ft apart) and `[*Jorkin]──25ft──[Ganger]` for one stride away. Regression tests: `TestRenderBattlefield_ShowsDistanceOnSeparator` and `TestRenderBattlefield_ShowsMeleeDistance`.

### BUG-38: Hotbar rendered to console after every server event
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** The hotbar bar (`[1:stride] [2:---] ...`) is written to the console output stream after every server event message instead of appearing only in the fixed UI row at H-1.
**Steps:** Enter combat; observe that after every combat message the full hotbar line is echoed to the console, repeating on every round update, action message, and status line.
**Fix:** Added `\033[H-1;1H\033[2K` clear before the scroll loop in `WriteConsole`. Each `\r\n` at promptRow (row H) scrolls the entire screen up; without clearing the hotbar row first, its content scrolled into the console region. Now the hotbar row is blanked before any scroll so only empty rows scroll upward.

### BUG-42: buy command cannot match items by display name or partial name
**Severity:** high
**Status:** fixed
**Category:** World
**Description:** The `buy` command fails to match merchant items by display name, partial name, or slug — none of "Sawn-Off", "sawn-off", "sawn-off-shotgun", or "Sawn-Off Shotgun" resolve to the Sawn-Off Shotgun in Sergeant Mack's inventory.
**Steps:** Browse Sergeant Mack's wares (`browse mack`); attempt `buy mack sawn-off`, `buy mack sawn-off-shotgun`, `buy mack Sawn-Off`, `buy mack Sawn-Off Shotgun`; all return "Sergeant Mack doesn't sell <term>".
**Fix:** Added `normalizeMerchantQuery` helper and fuzzy matching in `handleBuy`: exact → case-insensitive → slug-normalized → display name from `invRegistry`.
**Fix:**

### BUG-41: Non-combat NPCs do not appear as POIs on map after visiting room
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Merchant (Sergeant Mack), healer (Welder's Medic), and trainer (Shop Foreman) NPCs do not appear as POI symbols on the zone map even after the player has visited the room containing them.
**Steps:** Visit the room containing Sergeant Mack, Welder's Medic, Shop Foreman, Vera Coldcoin, or Marshal Ironsides; open the zone map; observe no POI symbol at those room locations.
**Fix:** Root cause: `handleMap` skipped any NPC with empty `npc_role`, but only 3 Dawg-family NPC YAMLs had `npc_role` explicitly set. Added `POIRoleFromNPCType(npcType)` in `maputil/poi.go` — returns `""` for `"combat"` and `""`, otherwise returns the npc_type (flows into existing `NpcRoleToPOIID`). Changed `handleMap` to use `npc_role` when set, else fall back to `POIRoleFromNPCType(npc_type)`. Regression tests: `TestPOIRoleFromNPCType_KnownTypes` and `TestHandleMap_POI_NPCTypeFallback`.

### BUG-37: Combat mode did not engage when entering combat
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** The terminal UI never switched to combat mode when the player entered combat; the combat screen was never displayed.
**Steps:** Attack an NPC; observe that the UI remained in room mode instead of switching to the combat display.
**Fix:** Root cause: the gameserver only broadcast `CombatEvent` messages for round starts, never a `RoundStartEvent` proto. Added `roundStartBroadcastFn` to `CombatHandler` and wired it in `grpc_service.go` to broadcast `ServerEvent_RoundStart`. Called from all three combat-start sites: `startCombatLocked`, `startPursuitCombatLocked`, and `resolveAndAdvanceLocked`.

### BUG-43: Hotbar rendered into feed/console after every server event (web client)
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** In the web client, hotbar slot data appears as a system message in the FeedPanel after every server event, instead of only updating the fixed HotbarPanel row.
**Steps:** Log in with a character; observe the FeedPanel; after any server event (room entry, combat message, status update) a hotbar line appears in the feed output.
**Fix:** Investigation confirmed the `case 'HotbarUpdate':` branch in `GameContext.tsx` correctly dispatches `SET_HOTBAR` without `APPEND_FEED`. The code was already correct in the deployed codebase; the bug was resolved by a prior deployment.

### BUG-44: New characters created with 0 HP
**Severity:** critical
**Status:** fixed
**Category:** Character
**Description:** Characters created via the web client wizard start with MaxHP and CurrentHP both at 0 because CreateCharacter builds the struct manually instead of calling BuildWithJob().
**Steps:** Create a new character; observe CurrentHP/MaxHP both 0 in character screen.
**Fix:** `CreateCharacter` handler now calls `character.BuildWithJob()` when options are available, which computes MaxHP = job.HitPointsPerLevel + GRT modifier and sets CurrentHP = MaxHP.

### BUG-45: Tech selection prompts appear on login for drifter-archetype characters
**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** Drifter archetype grants a prepared tech pool (19 archetype + job pool) with 1 slot at level 1 requiring player choice; the creation wizard does not handle archetype prepared tech grants, so the gameserver must prompt interactively on first login.
**Steps:** Create a Drifter-archetype character (Free Spirit, Pirate, Bagman, Tracker); log in; observe "Choose a technology: 1) ..." prompt in feed.
**Fix:** Exposed archetype TechGrants in ListOptions API; added prepared tech choice UI to CharacterWizard TechnologyStep (per-level dropdowns merging archetype+job pools); wired PreparedTechRepo through Server/CharacterHandler to persist choices on creation.

### BUG-47: Player combat input ignored in rounds 2+
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** From round 2 onward, any command the player typed during a combat round was rejected with "insufficient AP" because `autoQueuePlayersLocked` ran immediately at new-round setup (before the round-start broadcast and timer), spending all 3 of the player's AP on the default action before the player could see "Round X begins!" and type anything.
**Steps:** Start combat with `attack <npc>`; wait for round 2; type `attack <npc>` again; observe ⚠ insufficient AP error (or no apparent response).
**Fix:** Removed `autoQueuePlayersLocked` from the new-round setup block in `resolveAndAdvanceLocked`. Moved it to `resolveAndAdvance` (the timer-fired path) where it now runs as a last resort just before resolving, so players have the full round duration to submit their own action.

### BUG-46: Ranged weapon attacks non-functional after player login
**Severity:** critical
**Status:** fixed
**Category:** Combat
**Description:** Three root causes combined to make ranged weapons non-functional after player login:
1. `buildPlayerCombatant` read `h.loadouts[uid]` but that cache was only populated by `RegisterLoadout()` or `Equip()`. On login, `sess.LoadoutSet` was populated from DB but `h.loadouts` was never seeded, so `playerCbt.Loadout = nil` → `mainHandDef = nil` → `isMelee = true` → ranged weapon completely ignored.
2. `ResolveFirearmAttack` returned `AttackResult` without `DamageType` and `WeaponName` fields, causing resistance/weakness calculations and narrative to use blank weapon info.
3. `ActionAttack` ranged path had no ammo consumption, unlike `ActionFireBurst` and `ActionFireAutomatic`.
**Steps:** Log in with a character that has a ranged weapon in their active loadout preset; attempt to attack an NPC; observe melee attack used instead of ranged.
**Fix:**
- `buildPlayerCombatant` now falls back to `sess.LoadoutSet.ActivePreset()` when `h.loadouts[uid]` is missing.
- `ResolveFirearmAttack` now populates `DamageType` and `WeaponName` from `weapon.DamageType` and `weapon.Name`.
- `ActionAttack` ranged path now consumes one round of ammo via `eq.Magazine.Consume(1)` after attack resolves.


### BUG-47: Merchant Buy button does not give item to player and does not refresh UI
**Severity:** critical
**Status:** fixed
**Category:** Vendor
**Description:** Clicking Buy in the web merchant modal deducts stock from the NPC's inventory but does not add the item to the player's backpack, and neither the InventoryView nor CharacterSheetView is pushed after the transaction, so the player's currency and inventory appear unchanged.
**Steps:** Open Sergeant Mack's shop via the web UI; click Buy on any item; observe that item does not appear in inventory, carried currency does not visually decrease, and shop stock decrements.
**Fix:** `handleBuy` now calls `sess.Backpack.Add(itemID, qty, s.invRegistry)` after the successful transaction, saves inventory and currency to the DB, and pushes an `InventoryView` event via `sess.Entity.PushBlocking` so the frontend immediately reflects the purchase.

### BUG-49: Web UI map legend exceeds available screen width
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The map legend renders with 5 fixed columns regardless of panel width, causing it to overflow and be clipped on typical screen sizes.
**Steps:** Open the web client; enter a room and view the map panel; observe the legend overflows horizontally.
**Fix:** Reduced `LEGEND_COLS` in `mapRenderer.ts` from 5 to 3. Each column is 20 characters wide; 3 columns = 60 chars fits comfortably in typical panel widths (~300–400px at 0.75rem monospace).

### BUG-50: Web UI shop item hover tooltips clipped inside modal
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Hover detail tooltips for items in the merchant shop modal are clipped by the modal's overflow boundary instead of rendering on top of it.
**Steps:** Open the web client; interact with a merchant NPC; hover over an item in the shop listing; observe the tooltip is cut off by the modal edges rather than floating above all content.
**Fix:** Changed `ItemTooltip` in `NpcModal.tsx` to render via `ReactDOM.createPortal` into `document.body` with `position: fixed` and coordinates computed from `tdName.getBoundingClientRect()` on `mouseEnter`. This places the tooltip completely outside the modal's DOM subtree, bypassing all ancestor `overflow` clipping.

### BUG-51: Web UI level-up ability boost not reflected in Stats tab and levelup command non-functional
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** On level-up, the console correctly announces a pending ability boost but the Stats tab shows no pending boosts and the `levelup` command has no effect.
**Steps:** Level up a character in the web client; observe the console messages announcing the level-up and pending ability boost; open the Stats drawer — no pending boost is shown; type `levelup` in the input — nothing happens.
**Fix:** Root cause: neither `handleGrant` (on level-up) nor `handleLevelUp` pushed an updated `CharacterSheetView` after mutating `PendingBoosts`. Added `s.pushCharacterSheet(target)` in `handleGrant` after level-up messages are sent, and `s.pushCharacterSheet(sess)` in `handleLevelUp` after the boost is applied. The Stats tab now immediately reflects both the new pending boost count and the updated ability scores.

### BUG-52: Web UI character selection screen does not show zone and room
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The character selection screen in the web client lists characters but does not display their current zone or room location. Players cannot tell where a character is positioned before selecting them.
**Steps:** Open the web client; reach the character selection screen; observe that each character entry shows name/class/level but no zone or room.
**Fix:** Added `Location string \`json:"location,omitempty"\`` to `CharacterResponse` in `characters.go` and populated it from `c.Location` in `characterToResponse`. Added `location?: string` to the `Character` TypeScript interface in `client.ts`. Updated `CharactersPage.tsx` `CharacterCard` to render the room ID (underscores replaced with spaces) when present.

### BUG-53: Web UI character creation technology selection shows no descriptions
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** During character creation, the technology selection step displays technology names but no descriptions. Players cannot see damage values, healing amounts, range, AP cost, or other stat details needed to make an informed choice.
**Steps:** Create a new character in the web client; reach the technology selection step; observe that technologies are listed by name only with no stat details, damage, healing, or flavor text.
**Fix:** Added `description`, `action_cost`, `range`, `tradition`, `passive`, and `focus_cost` fields to `preparedEntryResponse`/`spontaneousEntryResponse` in `characters.go`. Added `preparedEntry`/`spontaneousEntry` helpers that populate these from `TechRegistry`. Updated TypeScript interfaces and `TechnologyStep` UI to display description and key stats (AP cost, range, tradition) beside each technology name.

### BUG-54: Web UI combat HUD and battlemap persist after successful flee
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** When a player successfully flees from combat, the server-side combat state is cleared but the web UI still shows the combat HUD and battlemap as active. The player is stuck seeing combat UI even though they are no longer in combat.
**Steps:** Enter combat in the web client; use the flee action; observe that the flee succeeds but the combat HUD remains visible and the battlemap is still active.
**Fix:** Root cause: `GameContext.tsx` only dispatched `SET_COMBAT_ROUND(null)` on `COMBAT_EVENT_TYPE_END`; a flee sends `COMBAT_EVENT_TYPE_FLEE` which was unhandled. Added `COMBAT_EVENT_TYPE_FLEE` to the clearing condition so the combat HUD and battlemap are dismissed immediately on a successful flee.

### BUG-55: New character created in web UI has no starting equipment, money, or inventory
**Severity:** high
**Status:** fixed
**Category:** Content
**Description:** A newly created character starts with an empty inventory, zero credits, and no equipped items. Players cannot engage with combat, shops, or any gear-dependent content immediately after creation.
**Steps:** Create a new character in the web client through the full creation flow; inspect the character's inventory and equipment; observe all slots are empty and credits are 0.
**Fix:** Root cause: web client's `JoinWorldRequest` sends `char.Team` ("gun"/"machete") as the `Archetype` field, but `grantStartingInventory` needs the job's archetype ID (e.g., "aggressor"). `LoadStartingLoadoutWithOverride` couldn't find the loadout file and silently skipped the grant. Fixed in `grpc_service.go`: resolved archetype from `jobRegistry.Job(sess.Class).Archetype` (authoritative), overriding the client-supplied value.

### BUG-56: Invalid wire-format proto data causes forwardEvents to log parse errors
**Severity:** high
**Status:** fixed
**Category:** Gameserver
**Description:** `grpc_service.go:3154` logs `proto: cannot parse invalid wire-format data` when unmarshaling an entity event inside `forwardEvents`, indicating a corrupted or misencoded protobuf message is being written to the entity event stream.
**Steps:** Observe server console — error appears at `gameserver/grpc_service.go:3154` in `forwardEvents` called from `Session.func3` at line 1654.
**Fix:** Root cause: two sites in `combat_handler.go` (mental-state messages ~line 3316, NPC taunt ~line 3324) pushed raw UTF-8 strings via `Entity.Push([]byte(msg))` instead of proto-marshaled `ServerEvent` data. `forwardEvents` always calls `proto.Unmarshal` on everything in the channel, so raw strings caused parse errors. Fixed both sites to use the existing `pushMessageToUID` helper which correctly wraps the message in a `MessageEvent` proto before pushing.

### BUG-57: Web UI hotbar layout in Feats/Technologies tab forces scrolling instead of overlaying
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** When using "Add to Hotbar" in the Feats or Technologies tab, the hotbar layout panel does not fit within the tab area and forces the user to scroll. The layout panel should overlay the tab content and expand as necessary to show all slots.
**Steps:** Open the web client; navigate to the Feats or Technologies tab; click "Add to Hotbar" on any feat or technology; observe that the hotbar layout panel is too large for the tab area and requires scrolling to interact with it.
**Fix:** Added `position: 'relative'` to `featItem`/`techItem` styles and changed `slotPicker` to `position: 'absolute', top: 0, left: 0, right: 0, zIndex: 10` in both `FeatsDrawer.tsx` and `TechnologyDrawer.tsx`. The slot picker now overlays the list item instead of expanding the document flow and forcing scroll.

### BUG-58: Stride command rejected in combat with "can only be used in combat" error
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Using the `stride` command during active combat fails with an error indicating stride can only be used in combat, despite the player already being in combat.
**Steps:** Enter combat in the web client; type `stride` in the input; observe the command is rejected with an error that stride can only be used in combat.
**Fix:** Root cause: `AddPlayer` always initialises `sess.Status = 1` (idle); on reconnect mid-combat the new session has status idle even though the combat engine still has the player as a combatant. Added a post-`AddPlayer` check in the Session handler: if the player's room has an active combat and the player is a combatant, `sess.Status` is restored to `statusInCombat` before the command loop starts.

### BUG-59: Character creation ability boost pools for region and job are not independent
**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** During character creation, the ability boost selections for region and job share a single pool of available boosts instead of being independent. This leaves the player unable to fill all required boost selections — for example, Archetype: Nerd + Job: Engineer produces fewer available boost choices than slots to fill.
**Steps:** Start character creation in the web client; select Archetype: Nerd and Job: Engineer; proceed to the ability boost selection screen; observe that region and job boost slots compete for the same pool of abilities, leaving insufficient options to complete all required selections.
**Fix:** Root cause: `takenAbilities` in `AbilityBoostsStep` aggregated both archetype and region choices into a single exclusion set, blocking the same ability from being chosen by two different sources. Fixed to maintain per-source exclusion: archetype dropdowns only exclude abilities already taken within the archetype source; region dropdowns only exclude within the region source. Cross-source double-boosting is now correctly permitted.

### BUG-60: Web UI character creation skill selection shows IDs instead of names; no hover descriptions
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** On the skill selection screen during web UI character creation, skill IDs are displayed instead of human-readable skill names, and no description is shown on hover.
**Steps:** Open the web client; begin character creation; proceed to the skill selection step; observe that each skill is identified by its raw ID string rather than its display name; hover over a skill entry and observe no tooltip or description appears.
**Fix:** Three-layer fix matching the feat selection pattern. (1) Server: added `skillResponse` struct to `characters.go` and populated a `skills` array in `ListOptions` with `id`, `name`, `description`, and `ability` fields, adding it to the JSON response alongside `feats`. (2) TypeScript: added `SkillOption` interface to `client.ts` and added `skills: SkillOption[]` to `CharacterOptions`. (3) Client: updated `SkillsStep` in `CharacterWizard.tsx` to accept `availableSkills: SkillOption[]` prop, build a `skillByID` lookup map with `useMemo`, render `skill?.name ?? id` instead of raw IDs, display `skill.description` below each skill name, and updated fixed-skill display to use `optionCard` style matching the feat pattern. Updated the call site to pass `availableSkills={options?.skills ?? []}`.

### BUG-69: Armor Training armor category selection uses console prompt instead of modal in web UI
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** In the web UI, the Armor Training feat category selection is delivered as a numbered console prompt which the player cannot respond to — input is interpreted as a movement command instead; the selection should be presented as a modal popup.
**Steps:** Create or level a character with the Armor Training feat in the web UI; observe the console shows "Choose an armor category to gain proficiency in: 1) light_armor 2) medium_armor 3) heavy_armor Enter 1-3:"; type "3"; observe the response is "Invalid selection. You will be prompted again on next login." and the input was routed to the movement handler as `no exit "3"`.
**Fix:** Sentinel-encoded the choice prompt as `"\x00choice\x00" + JSON` in `promptFeatureChoice`. The recv loop now skips non-Say/non-Move messages (StatusRequest, MapRequest) and non-numeric text so web client status polls no longer consume the choice slot. The websocket handler decodes the sentinel and sends a `FeatureChoicePrompt` frame; the web client renders it as a `FeatureChoiceModal` overlay with clickable option buttons. Telnet clients receive human-readable numbered text via `renderChoicePrompt` in game_bridge.go.

### BUG-68: Reaction feats and technologies displayed as Active instead of Reactions
**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** Feats and technologies with a reaction trigger are categorized and displayed as Active abilities rather than Reactions, and the player is never prompted to apply them when the trigger condition occurs.
**Steps:** Create a character with a reaction feat or technology (e.g. a counterattack reaction); observe the character sheet or abilities panel shows it listed under Active rather than a Reactions category; enter a situation that triggers the reaction and observe no prompt appears asking whether to use it.
**Fix:** Added `is_reaction` (bool) to `FeatEntry` and `InnateSlotView` proto messages. Server sets `IsReaction = feat.Reaction != nil` in `handleChar`, `handleFeats`, and `handleUse`; sets `IsReaction = def.Reaction != nil` on `InnateSlotView` in `handleChar`. `FeatsDrawer.tsx` now partitions feats into Reactions/Active/Passive and renders a Reactions section with amber badge styling above Active. `TechnologyDrawer.tsx` separates reaction innate techs into their own Reactions section. Removed obsolete hand-written `loadout_types.go` (now generated by proto). Four tests added including a rapid property test.

### BUG-67: Web UI does not display loot notification after killing an NPC
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** After killing an NPC the web UI console shows the XP earned message but displays no notification for loot obtained, leaving the player unaware of items added to their inventory.
**Steps:** Open the web client; engage and kill an NPC that drops loot; observe the console shows XP granted but no loot message appears.
**Fix:** Added `pushLootMessages` method to `CombatHandler` (`internal/gameserver/combat_handler.go`). Called after dropping items on the floor in `removeDeadNPCsLocked`. Sends a `MessageEvent` with content "You looted: Item Name (xN), ..." to each living combat participant. Item display names are resolved via `invRegistry.Item()` with fallback to `ItemDefID` when the registry is nil or the item definition is not found. No message is sent when loot produces no items (e.g. currency-only tables).

### BUG-66: Web UI console stops autoscrolling during combat
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** During combat the web UI console intermittently stops autoscrolling to new content, requiring the user to scroll manually to see the latest messages.
**Steps:** Open the web client; enter combat; observe the console as combat messages arrive; intermittently the console stops scrolling to the bottom and new messages appear above the visible area.
**Fix:** Two root causes addressed in `FeedPanel.tsx`. (1) Replaced `useEffect` with `useLayoutEffect` so scroll happens after DOM layout but before paint, eliminating the race where `scrollTop = scrollHeight` fired before the new message node was measured. (2) Added a `programmaticScrollRef` boolean flag that suppresses the `onScroll` handler while a programmatic scroll is in flight, preventing the `atBottom` check from reading a stale `scrollHeight` and falsely setting `userScrolledRef = true`. A bottom sentinel `<div ref={bottomRef} />` is used as the scroll target via `scrollIntoView({ behavior: 'instant' })`.

### BUG-65: Armor Training feat selection never prompted — feat has no effect
**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** The Armor Training feat grants proficiency in one additional armor category, but the player is never prompted to choose the category, so the feat applies no benefit.
**Steps:** Create or level a character; select the Armor Training feat; observe that no armor category selection prompt appears; observe that the character sheet shows no new armor proficiency from the feat.
**Fix:** Added `choices` block to the `armor_training` feat in `content/feats.yaml` (key: `armor_category`, options: `light_armor`, `medium_armor`, `heavy_armor`). Added proficiency application logic in `internal/gameserver/grpc_service.go` after the feat-choice resolution loop: when `sess.FeatureChoices["armor_training"]["armor_category"]` is non-empty, the chosen category is upserted via `characterProficienciesRepo` and loaded into `sess.Proficiencies`.

### BUG-64: Web UI does not display or manage loadouts
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** The web UI provides no way to view loadouts, switch between them, or edit the equipped items in each loadout.
**Steps:** Open the web client; navigate to the Equipment or Character tab; observe that no loadout selector, loadout list, or loadout editing controls are present.
**Fix:** Added a `LoadoutView` structured proto message (hand-written in `game.pb.go`) and `LoadoutWeaponPreset` sub-message. Modified `handleLoadout` in `grpc_service.go` to return a `LoadoutView` ServerEvent (field 33) when called with no arg, building preset data from the session's `LoadoutSet`. Added `serverEventLoadoutView` in `websocket.go` to marshal `LoadoutView` via `encoding/json` (bypassing protojson) and dispatch it to web clients as `"LoadoutView"` frames. Added `LoadoutRequest` to the websocket dispatch type map. Added a `LoadoutView` case to the telnet `forwardServerEvents` bridge that renders the structured data as plain text. Added TypeScript types `LoadoutView` and `LoadoutWeaponPreset` to `proto/index.ts`. Added `loadoutView` state and `SET_LOADOUT_VIEW` reducer to `GameContext.tsx`. Created `LoadoutDrawer.tsx` that sends a `LoadoutRequest` on open, displays all presets in cards (active preset highlighted), and provides a Switch button for each inactive preset. Wired `LoadoutDrawer` into `DrawerContainer.tsx` and added a "Loadout" toolbar button to `GamePage.tsx`.

### BUG-63: Motel keeper NPCs display with combat health status
**Severity:** medium
**Status:** fixed
**Category:** World
**Description:** Motel keeper NPCs render with a combat-style health label (e.g. "Scrap Inn Clerk (unharmed)") instead of the non-combat NPC format (e.g. "Scrap Inn Clerk [motel]").
**Steps:** Enter any zone hub safe room containing a motel keeper NPC; observe the NPC label in the room description includes a health status in parentheses rather than a bracketed role tag.
**Fix:** Added `"motel_keeper"` case to `npcTypeTag` in `internal/frontend/handlers/text_renderer.go` returning `"[motel]"`, so motel keeper NPCs are recognized as non-combat and display the role tag instead of health status.

### BUG-62: Vantucky map has disconnected rooms
**Severity:** medium
**Status:** fixed
**Category:** World
**Description:** One or more rooms in the Vantucky area are not connected to the rest of the map, making them unreachable via normal navigation.
**Steps:** Explore the Vantucky area and attempt to navigate to all rooms; observe that some rooms cannot be reached from any adjacent room.
**Fix:** Audited all exits in `content/zones/vantucky.yaml` via BFS reachability analysis. Found 6 rooms unreachable from the start room (`vantucky_abandoned_mall`, `vantucky_east_side`, `vantucky_overgrown_freeway`, `vantucky_rail_spur`, `vantucky_river_cliffs`, `vantucky_trailer_park`) due to 7 broken or missing bidirectional exits. Fixed: (1) added `east→abandoned_mall` to `shooting_range`; (2) changed `east_side.west` from `164th_ave` to `ammo_depot` (correct by map coordinates) and added `ammo_depot.east→east_side`; (3) added `ammo_depot.south→rail_spur`; (4) added `gun_market.west→gas_station_ruins`; (5) added `fishers_landing.east→river_cliffs`; (6) added `i84_onramp.south→trailer_park` and `i84_onramp.west→overgrown_freeway`, and corrected `trailer_park.north` from `gas_station_ruins` to `i84_onramp`, removing the stale `trailer_park.east→burnt_bridge_creek` orphaned exit. Added `TestLoadZone_Vantucky_AllRoomsReachable` property test to `internal/game/world/loader_test.go` verifying bidirectionality and full reachability.

### BUG-71: Exploring rooms does not update map with danger level
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** When a player explores a room, the map display does not update to reflect the room's danger level; danger level remains absent or stale on the map after visitation.
**Steps:** Open the map; explore one or more rooms with a known danger level; observe that the map does not show or update the danger level for the visited rooms.
**Fix:** ExploredCache update was gated inside `if !sess.AutomapCache[zID][newRoom.ID]` — rooms pre-loaded via zone reveal were in AutomapCache but never marked as explored. Moved ExploredCache update outside the AutomapCache gate so it always runs on room entry; added automapRepo persistence upsert.

### BUG-70: NPC list order is non-deterministic and changes on refresh
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** NPCs displayed in room descriptions are not sorted, causing their order to change between refreshes and making placement inconsistent.
**Steps:** Enter a room containing multiple NPCs; observe the NPC list order; refresh or re-enter the room and observe that the order may differ.
**Fix:** Added `sort.Slice` by instance ID in `InstancesInRoom` after collecting instances from the map iteration.

### BUG-74: `use brutal_surge` has no effect
**Severity:** high
**Status:** open
**Category:** Combat
**Description:** Using the `brutal_surge` technology via the `use brutal_surge` command produces no visible effect on the player or combat state.
**Steps:** Enter combat; execute `use brutal_surge`; observe that no effect is applied and no feedback is given.
**Fix:**

### BUG-73: Web UI Inventory Equip button does not prompt for loadout and hand selection
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** The Equip button in the web UI Inventory tab equips items without prompting the player to select a loadout and hand slot; the item is not removed from inventory and the Loadout tab does not reflect the change.
**Steps:** Log in to the web UI; open the Inventory tab; click the Equip button on a weapon; observe that no modal appears to select a loadout or hand, inventory is not updated, and the Loadout tab does not reflect any change.
**Fix:** Added 2-step preset+hand picker to WeaponRow; added EquipRequest to websocket dispatch; added preset field to EquipRequest proto; updated HandleEquip to accept presetIndex.

### BUG-78: Technology "Force Field Emitter" description references Dexterity
**Severity:** low
**Status:** open
**Category:** Content
**Description:** The Force Field Emitter technology description incorrectly references Dexterity and mentions "mystic armor", both of which are PF2E/fantasy terms not used in this system.
**Steps:** Log in; open the Technologies tab or view the Force Field Emitter technology description; observe the references to Dexterity and mystic armor.
**Fix:**

### BUG-77: Web UI has a wide border surrounding the screen that should be removed
**Severity:** low
**Status:** open
**Category:** UI
**Description:** A wide border surrounds the entire web UI screen, consuming unnecessary screen space; it should be removed.
**Steps:** Log in to the web UI; observe the wide border surrounding the full screen layout.
**Fix:**

### BUG-76: Web UI hotbar assignment popup anchors left and extends off-screen
**Severity:** medium
**Status:** open
**Category:** UI
**Description:** When assigning a Feat or Technology to the hotbar, the hotbar selection popup is anchored to the left edge of the tab and expands rightward, causing it to extend off-screen and become partially invisible.
**Steps:** Log in to the web UI; open the Feats or Technologies tab; click the hotbar assignment control on any item; observe the hotbar popup anchors left and extends beyond the right edge of the viewport.
**Fix:**

### BUG-75: Web UI Loadouts tab should be inside Equipment tab, not a separate tab
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The Loadouts tab exists as a standalone top-level tab; it should be moved inside the Equipment tab, replacing the Main Hand and Off Hand sections at the top, with the two loadouts displayed side by side horizontally.
**Steps:** Log in to the web UI; observe the Loadouts tab as a separate top-level tab; note that the Equipment tab separately displays Main Hand and Off Hand rather than integrating loadout selection inline.
**Fix:** Merged loadout preset cards into EquipmentDrawer at the top, displayed side by side. Removed standalone Loadout toolbar button and DrawerType entry.

### BUG-72: Web UI Technology tab items show no description or effects
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** In the web UI Technology tab, each listed technology displays no description or effect details, leaving players unable to understand what any technology does.
**Steps:** Log in to the web UI; open the Technology tab; observe that technology entries show no description or effect information.
**Fix:** Added `description` field to `PreparedSlotView` and `SpontaneousKnownEntry` proto messages; server now populates description from tech registry for all slot types.

### BUG-61: Web UI Stats tab does not update XP after combat
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** After combat ends, the console correctly displays the XP granted message, but the Stats tab continues to show the pre-combat XP value and does not reflect the updated total.
**Steps:** Open the web client; engage and complete combat; observe the XP granted message in the console; navigate to the Stats tab and observe that the XP value has not updated.
**Fix:** `CombatHandler.pushXPMessages` was sending XP grant text messages but never pushing a `CharacterSheetView` to the client. The `StatsDrawer` reads XP from `state.characterSheet`, which is only updated when a `CharacterSheetView` arrives. Added a `pushCharacterSheetFn func(*session.PlayerSession)` callback field to `CombatHandler` with `SetPushCharacterSheetFn`, called unconditionally at the end of `pushXPMessages`. Wired `s.pushCharacterSheet` as the callback in `grpc_service.go`.
