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
**Status:** fixed
**Category:** Combat
**Description:** Using the `brutal_surge` technology via the `use brutal_surge` command produces no visible effect on the player or combat state.
**Steps:** Enter combat; execute `use brutal_surge`; observe that no effect is applied and no feedback is given.
**Fix:** Class feature condition application in handleUse now uses cbt.Conditions[uid] (combat-level set) when the player is in combat, instead of sess.Conditions (session-level set). Combat modifiers (AC penalty, damage bonus) only read from the combat set.

### BUG-73: Web UI Inventory Equip button does not prompt for loadout and hand selection
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** The Equip button in the web UI Inventory tab equips items without prompting the player to select a loadout and hand slot; the item is not removed from inventory and the Loadout tab does not reflect the change.
**Steps:** Log in to the web UI; open the Inventory tab; click the Equip button on a weapon; observe that no modal appears to select a loadout or hand, inventory is not updated, and the Loadout tab does not reflect any change.
**Fix:** Added 2-step preset+hand picker to WeaponRow; added EquipRequest to websocket dispatch; added preset field to EquipRequest proto; updated HandleEquip to accept presetIndex.

### BUG-78: Technology "Force Field Emitter" description references Dexterity
**Severity:** low
**Status:** fixed
**Category:** Content
**Description:** The Force Field Emitter technology description incorrectly references Dexterity and mentions "mystic armor", both of which are PF2E/fantasy terms not used in this system.
**Steps:** Log in; open the Technologies tab or view the Force Field Emitter technology description; observe the references to Dexterity and mystic armor.
**Fix:** Replaced "Dexterity" with "Quickness" (the game equivalent) in the Force Field Emitter utility effect description; also replaced "mystic armor" and "magical energy" with force-field-appropriate language.

### BUG-77: Web UI has a wide border surrounding the screen that should be removed
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** A wide border surrounds the entire web UI screen, consuming unnecessary screen space; it should be removed.
**Steps:** Log in to the web UI; observe the wide border surrounding the full screen layout.
**Fix:** Added CSS reset (margin:0, padding:0, overflow:hidden) to html/body/#root to eliminate the default browser 8px body margin.

### BUG-76: Web UI hotbar assignment popup anchors left and extends off-screen
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** When assigning a Feat or Technology to the hotbar, the hotbar selection popup is anchored to the left edge of the tab and expands rightward, causing it to extend off-screen and become partially invisible.
**Steps:** Log in to the web UI; open the Feats or Technologies tab; click the hotbar assignment control on any item; observe the hotbar popup anchors left and extends beyond the right edge of the viewport.
**Fix:** Removed position:absolute from SlotPicker in FeatsDrawer and TechnologyDrawer; picker now renders inline within the drawer flow, eliminating off-screen overflow.

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

### BUG-118: Boot job displays team designator in name instead of plain "Boot"
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The starter jobs "Boot (Machete)" and "Boot (Gun)" expose the internal team designator in their display name. Players should see the name "Boot" only — the team affinity is internal information and must not appear in any player-facing display (job selection, character sheet, trainer UI, web UI Job tab).
**Steps:** Create a character or view the job trainer; observe the job name is shown as "Boot (Machete)" or "Boot (Gun)" instead of "Boot".
**Fix:** Set the display name of both Boot job definitions to "Boot". The team designator should remain in the internal ID or a non-displayed field only.

### BUG-117: Web UI map POI legend does not use multiple columns when width is available
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The Points of Interest legend in the web UI map panel renders as a single column regardless of available width. When the panel is wide enough, the POI legend entries should flow into multiple columns to make better use of space and reduce vertical scrolling.
**Steps:** Open the map panel in the web UI at a wide viewport; observe the POI legend renders as a single column.
**Fix:** Apply a multi-column layout to the POI legend (e.g. CSS `column-count` or a responsive grid) so POI entries flow into additional columns when the available width allows.

### BUG-116: Player attack narrative never includes weapon name; unarmed damage type is empty
**Severity:** medium
**Status:** fixed
**Category:** Combat
**Description:** Two related combat narrative gaps: (1) `buildPlayerCombatant()` in `combat_handler.go:2574` only sets `WeaponDamageType` from the equipped weapon but never sets `WeaponName`, so attack narratives for all players — armed or unarmed — omit the weapon name (e.g. "Player strikes Target" instead of "Player strikes Target with a Riot Shotgun"); (2) unarmed attacks have an empty `DamageType`, which produces incomplete damage log entries.
**Steps:** Equip any weapon; enter combat; attack — observe the narrative contains no weapon name. Remove all weapons; attack — observe the narrative is identical and damage type is absent.
**Fix:** In `buildPlayerCombatant()` (`combat_handler.go`), set `WeaponName` from `Loadout.MainHand.Def.Name` when a weapon is equipped. For unarmed attacks, set `WeaponName` to "fists" and `WeaponDamageType` to "bludgeoning".

### BUG-115: Armor Training AC bonus not applied; untrained armor penalty not applied
**Severity:** critical
**Status:** fixed
**Category:** Combat
**Description:** Two related Armor Training mechanics are not functioning: (1) the AC bonus granted by Armor Training for the trained armor category is not applied to the player's effective AC; (2) the check penalty and speed penalty for equipping armor in a category the player is not trained in are not applied. Players receive neither the benefit of training nor the penalty for lacking it.
**Steps:** Train Armor Training for a category (e.g. Medium); equip armor of that category; observe AC is unchanged. Equip armor of an untrained category; observe no check or speed penalty is applied.
**Fix:** In the AC and penalty calculation paths, check the player's Armor Training feat and trained category against the equipped armor category. Apply the trained AC bonus when the categories match; apply the untrained check and speed penalties when they do not.

### BUG-114: Hovering consumable items in Inventory tab shows no details tooltip
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** Hovering over a consumable item in the web UI Inventory tab does not display a tooltip with the item's description and effects. Other item types show hover details; consumables do not.
**Steps:** Open the Inventory tab; hover over a consumable item — no tooltip appears.
**Fix:** Add a hover tooltip to consumable inventory rows that renders the item's description and effects, consistent with hover behaviour on other item types.

### BUG-113: Mobile web UI region selection during character creation not scrollable
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** On mobile browsers the region selection step during character creation cannot be scrolled, preventing the user from reaching and selecting a region. This is a second instance of the mobile scrolling issue first reported in BUG-108 (which covered character creation generally); this specifically identifies the region selection screen as a non-scrollable blocker.
**Steps:** Open the web UI on a mobile browser; begin character creation; reach the region selection step; attempt to scroll to view all regions and make a selection — scrolling does not work.
**Fix:** Apply scrollable container styling (`overflow-y: auto` with constrained height) to the region selection screen, and audit all remaining character creation steps for the same issue.

### BUG-112: Web UI room exits not displayed as 8-point compass navigation control
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Room exit controls in the web UI are not arranged as a compass. They should be displayed in a fixed 8-point compass layout (N, NE, E, SE, S, SW, W, NW) with North at the top center. If no exit exists for a compass point the position should be left empty, preserving the layout so the player can immediately understand available directions spatially.
**Steps:** Enter any room with exits; observe the exit controls — exits are listed without compass orientation rather than in a fixed 8-point grid.
**Fix:** Replace the exit list with a 3x3 compass grid component (center cell unused). Each cell corresponds to a compass direction; populate it with a clickable exit button if that exit exists, or leave it empty if not.

### BUG-111: Job levelling does not grant new Feats and Technologies
**Severity:** critical
**Status:** fixed
**Category:** Character
**Description:** Advancing in job level does not grant the Feats and Technologies that should be awarded at each level. A player at job level 6 only has the Feats and Technologies granted at character creation, with none of the intervening level-up grants applied.
**Steps:** Advance a character to job level 2 or higher via training; open the Feats and Technologies tabs — only creation-time grants are present; no level-up grants have been applied.
**Fix:** `handleGrant()` was iterating `job.LevelUpGrants[lvl]` — but no job YAML defines level_up_grants; all level-up tech grants live on archetypes. Fixed by looking up the job's archetype via `s.archetypes[job.Archetype]` and merging its `LevelUpGrants` with the job's via `ruleset.MergeLevelUpGrants()`. The merged map is now iterated for each gained level so archetype technology grants are applied on level-up.

### BUG-110: Technology panel "Add to hotbar" inline slot selector overflows screen; should use modal
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** In the Technology panel, clicking "Add to hotbar" renders an inline hotbar slot selector that can be wider than the panel, causing it to extend off the side of the screen. The selector should instead open a modal displaying the full hotbar so the user can select a slot without layout overflow.
**Steps:** Open the Technology tab; click "Add to hotbar" on any technology; observe the inline slot selector extends beyond the panel edge and off-screen.
**Fix:** Converted SlotPicker from an inline div to a fixed-position overlay modal (position:fixed, z-index:300, centered flex). Modal uses width:max-content with maxWidth:95vw so it auto-expands to fit slot command text without clipping. Clicking the overlay dismisses the modal.

### BUG-109: Armor Training feat in Feats tab does not display the selected armor category
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** In the Feats tab, the Armor Training passive feat is displayed without indicating which armor category the player selected during training (e.g. "Light", "Medium", "Heavy"). The player has no way to see their trained armor type from the UI.
**Steps:** Have a character with Armor Training trained to a category; open the Feats tab; observe the Armor Training entry — no armor category is shown.
**Fix:** Include the trained armor category in the Armor Training feat display, e.g. "Armor Training (Medium)".

### BUG-108: Web UI character creation not scrollable on mobile browsers
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** On mobile browsers the character creation flow cannot be scrolled, making it impossible to reach controls below the visible viewport and complete character creation.
**Steps:** Open the web UI in a mobile browser; begin character creation; attempt to scroll down to reach lower sections or buttons — scrolling does not work and controls are unreachable.
**Fix:** Ensure the character creation screens use scrollable containers (e.g. `overflow-y: auto` with appropriate height constraints) so all content and action buttons are reachable on small-screen mobile viewports.

### BUG-107: Player respawn does not restore full HP, clear conditions, or recharge Feats and Technologies
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** When a player respawns after death they are not fully restored. On respawn the player should receive the equivalent of a full rest: HP restored to maximum, all conditions removed, and all Feat and Technology uses recharged. This ensures the player can continue playing immediately without being stuck in a degraded state.
**Steps:** Die in combat or from any damage source; observe the respawn state — HP is not full, conditions may persist, and Feat/Technology uses are not recharged.
**Fix:** respawnPlayer now sets CurrentHP to MaxHP (was hardcoded to 1), clears all conditions via ClearAll() (was only clearing encounter-duration conditions via ClearEncounter()), restores spontaneous use pools and innate tech slots via DB repos, un-expends all prepared tech slots, restores active feat uses to their PreparedUses maximum, and restores focus points to MaxFocusPoints. Added ClearAll() method to condition.ActiveSet.

### BUG-106: Player HP damage not immediately reflected in Character panel during combat
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** During combat, when the player takes damage their current HP in the Character panel updates with a noticeable delay rather than immediately when the damage event is received. The HP value lags behind the actual game state.
**Steps:** Enter combat; take damage from an NPC attack; observe the Character panel HP — it does not update immediately and reflects the correct value only after a delay.
**Fix:** Added UPDATE_PLAYER_HP dispatch to the CombatEvent handler in GameContext.tsx when the combat target matches the player's name, so player HP updates immediately without waiting for the next CharacterInfo event.

### BUG-105: Clicking Fixer NPC shows examine output instead of Fixer interaction modal
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** Clicking a Fixer NPC (e.g. Dex in Rotgut Alley) in the web UI displays the generic examine output instead of a Fixer interaction modal. The player should be shown a modal with Fixer-specific options (e.g. wanted level clearing, black market services).
**Steps:** Navigate to Rotgut Alley; click Dex; observe the generic examine description is shown instead of a Fixer modal.
**Fix:** Added FixerView proto message; added buildFixerView server handler in grpc_service_npc_examine.go; added case "fixer" to handleExamine switch in grpc_service.go; added FixerView TypeScript interface to proto/index.ts; added SET_FIXER_VIEW action and fixerView state to GameContext.tsx; added FixerModal component to NpcInteractModal.tsx.

### BUG-104: Character selection screen location shows room only, not zone and room
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The Location field on the character selection screen displays only the room name. It should display both the zone and the room (e.g. "Rustbucket Ridge — Grinder's Row").
**Steps:** Log in; observe the character selection screen — the Location field shows only the room name with no zone.
**Fix:** Updated `ListByAccount()` in `internal/storage/postgres/character.go` to JOIN rooms and zones tables, formatting Location as "Zone Name — Room Name". Updated `CharactersPage.tsx` to render the pre-formatted location string directly instead of applying raw-ID text transforms. Added rooms and zones tables to the postgres test setup so integration tests pass.

### BUG-103: Web UI has no Job tab
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** The web UI has no Job tab. Players have no way to view their current job, job progression, or available job advancement options from the web UI.
**Steps:** Log in; browse all tabs in the web UI — no Job tab exists.
**Fix:** Added a Job drawer to the web UI toolbar displaying the player's current job name, archetype, team badge, level, XP bar with current/max XP, and notices for pending ability boosts and skill increases.

### BUG-102: Merchant stock quantity does not update in web UI after purchase
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** After buying an item from a merchant in the web UI, the stock quantity displayed in the merchant screen does not decrease. The purchase is processed correctly server-side but the merchant view is not refreshed in the client.
**Steps:** Open a merchant shop; note the stock quantity of an item; purchase the item; observe the stock quantity is unchanged in the UI.
**Fix:** Extracted `buildShopView` helper from `handleBrowse` in `grpc_service_merchant.go`. After the InventoryView push in `handleBuy`, an updated ShopView is now pushed via `sess.Entity.PushBlocking` so the client immediately reflects the new stock quantities. Test `TestHandleBuy_PushesUpdatedShopViewAfterPurchase` added to verify the behaviour.

### BUG-101: Equipped armor item remains visible in Inventory tab after wearing
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** After clicking "Wear" on an armor item in the Inventory tab, the item correctly appears in the Equipment tab but is not removed from the Inventory tab. The item is double-displayed — once as equipped and once as still in the backpack.
**Steps:** Purchase an armor item; open the Inventory tab; click "Wear"; observe the item appears in the Equipment tab; return to the Inventory tab — the item is still listed there.
**Fix:** Added `pushInventory(sess)` call in `handleWear` after a successful "Wore ..." result so the web UI receives a fresh InventoryView immediately after equipping.

### BUG-100: Player death outside combat does not trigger respawn at zone spawn point
**Severity:** critical
**Status:** fixed
**Category:** Combat
**Description:** If a player dies outside of combat (e.g. from a trap, environmental effect, or other non-combat damage source) they do not respawn at the zone spawn point. Respawn must always occur at the zone spawn point regardless of how the player died.
**Steps:** Cause the player to die outside of combat; observe the player does not respawn and remains in a dead/stuck state.
**Fix:** Added `checkNonCombatDeath(uid, sess)` helper to `GameServiceServer` which sets `sess.Dead = true` and calls `go respawnPlayer(uid)` when `sess.CurrentHP <= 0`. Called from all 7 non-combat damage sites: trap damage (grpc_service_trap.go), continuous drown (dispatch loop), radiation tick, skill-check effect damage, reactive-strike tumble damage, fall damage (handleClimb), and swim drown damage (handleSwim).

### BUG-99: Web UI periodically sends map request during combat, producing spurious error messages
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** During combat the console periodically displays `You cannot use the map while in combat` even though the player is not interacting with the map. The web client is automatically issuing map requests in the background (likely a polling or auto-refresh mechanism) that are rejected by the server during combat.
**Steps:** Enter combat; do not interact with the map; observe the console — `You cannot use the map while in combat` appears periodically without player input.
**Fix:** Root cause: `MapPanel.tsx` useEffect fired on every `state.connected` change including when combat starts/ends. Added `state.combatRound === null` guard and added `state.combatRound` to the dependency array so the auto-request only fires when connected AND not in combat.

### BUG-98: Clicking quest giver NPC shows examine modal instead of quest selection
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** Clicking a quest giver NPC in the web UI opens the generic examine modal instead of a quest interaction modal. The player should be shown a list of available quests to accept, or a message indicating no quests are currently available.
**Steps:** Navigate to a room with a quest giver NPC; click the NPC; observe the generic examine description is shown instead of a quest modal.
**Fix:** Root cause: RoomPanel.tsx only had special-case routing for merchant NPCs; all others fell through to ExamineRequest. Added `quest_giver` case that sends `TalkRequest` with the NPC name. The server's existing handleTalk returns quest list and accept instructions as a console message instead of opening an examine modal.

### BUG-97: Rest technology preparation prompts appear as console text with no interactive modal
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** When resting, technology slot preparation prompts (e.g. "Level 1, slot 1: choose from pool") are printed as plain console messages with no way for the player to act on them. The player must be presented with a modal allowing them to select technologies for each slot before the rest completes.
**Steps:** Visit a motel keeper; pay to rest; observe the console — preparation slot prompts appear as text only (e.g. `Level 1, slot 1: choose from pool`, `Level 1, slot 2: choose from pool`) with no modal or interactive control to make selections.
**Fix:** Root cause: `handleMotelRest` used an auto-select `promptFn` (picking `options[0]`) with no stream access, so the sentinel-encoded `FeatureChoicePrompt` was never sent. Updated `handleMotelRest` to accept `requestID` and `stream`, and replaced the auto-select `promptFn` with `s.promptFeatureChoice(stream, "tech_choice", ...)`, mirroring `applyFullLongRest`. The stream context is now used for the rest operation. Added test `TestHandleRest_MotelRest_SendsInteractiveChoicePrompt` to verify the sentinel is sent and the prompt resolves via user input.

### BUG-96: Post-combat item loot shown in console but not added to inventory
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** At the end of combat the console correctly reports looted items (e.g. `You looted: Scrap Metal (x4)`) but the items are not present in the player's inventory. The loot message is generated but the item grant is either not persisted or not reflected in the inventory view.
**Steps:** Enter combat; defeat all NPCs; observe the console shows a loot message (e.g. `You looted: Scrap Metal (x4)`); open the Inventory tab — the looted items are absent.
**Fix:** Root cause: items were dropped to the room floor via `FloorManager` instead of being granted directly to each player's backpack. Replaced floor-drop with `distributeItemsLocked` in `CombatHandler`, which adds items to each participant's `Backpack`, persists via `saveInventoryFn` callback (wired to `charSaver.SaveInventory` in `grpc_service.go`), and calls `pushInventoryFn` to refresh the web UI. Materials still drop to the floor.

### BUG-95: Clicking motel keeper (Scrap Inn Clerk) shows examine result instead of rest modal
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** Clicking the motel keeper NPC in the web UI opens a panel showing the examine/description output instead of a rest interaction modal. The player should be presented with a modal to purchase a rest (showing cost and available options) when clicking a motel keeper.
**Steps:** Navigate to a room with a motel keeper; click the NPC; observe the examine description is displayed instead of a rest modal.
**Fix:** Root cause: RoomPanel.tsx only had special-case routing for merchant NPCs; all others fell through to ExamineRequest. Added `motel_keeper` case that sends `RestRequest`. The server's existing handleRest processes the rest flow and returns console messages, preventing the examine modal from opening.

### BUG-94: Consumable merchant inventory shows item IDs instead of display names; no hover description
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The consumable merchant shop displays raw item IDs instead of human-readable display names. Additionally, hovering over an item shows no tooltip with the item's description and effects.
**Steps:** Visit a consumable merchant; open the shop; observe item IDs listed instead of names; hover over an item and observe no description or effects tooltip.
**Fix:** Added `effects_summary` field to `ShopItem` proto (field 17). In `handleBrowse`, when `def.Kind == consumable`, populate `EffectsSummary` via the new `buildConsumableEffectsSummary` helper. Added `else if (kind === 'consumable')` block to `ItemTooltip` in `NpcModal.tsx` that renders the effects summary. The display name was already being populated correctly from `def.Name`; the investigation confirmed the root cause was the tooltip lacking a consumable branch.

### BUG-93: Hovering Consume button on consumable item shows no effect description tooltip
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** In the Inventory panel, hovering over the Consume button on a consumable item shows no tooltip. A popup describing the item's effects should appear on hover so the player knows what they are about to consume before clicking.
**Steps:** Open the Inventory tab; hover over the Consume button on a consumable item; observe no tooltip or popup appears.
**Fix:** Added `effects_summary` field to `InventoryItem` proto (field 9). In `handleInventory`, populate `EffectsSummary` for consumable items via `buildConsumableEffectsSummary`. Added `title` attribute to the Consume button in `InventoryDrawer.tsx` using the effects summary, falling back to "Consume {name}".

### BUG-92: Post-combat loot grant not notified and inventory not refreshed in web UI
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** After defeating NPCs the player receives no loot notification in the console and the inventory panel does not update to reflect the Crypto gain. The Crypto was in fact awarded — logging out and back in shows the correct balance. The server-side loot grant is correct; the client is not being told about it.
**Steps:** Enter combat; defeat all NPCs; observe the end-of-combat console — no loot message appears and the inventory Crypto total is unchanged; log out and log back in; observe the inventory now shows the correct Crypto balance.
**Fix:** Added `pushCurrencyMessages` to `CombatHandler` which sends a console message and calls the new `pushInventoryFn` callback for each surviving participant after `distributeCurrencyLocked`. Added `pushInventory` to `GameServiceServer` (parallel to `pushCharacterSheet`) and wired it via `SetPushInventoryFn` during combat handler initialization.

### BUG-91: Same character can be selected by multiple concurrent sessions; no active-session indicators or force-logout control
**Severity:** critical
**Status:** fixed
**Category:** Meta
**Description:** A user can open two browsers, log in to the same account, and select the same character in both — resulting in duplicate concurrent sessions for a single character. Three related defects must be fixed together: (1) the character selection screen does not indicate which characters are currently logged in; (2) a character that is actively logged in can still be selected, which must be prevented; (3) there is no way for the user to force-logout a stuck or duplicate session from the character selection screen.
**Steps:** Open two browser windows; log in to the same account in both; select the same character in both; observe that both sessions enter the game simultaneously.
**Fix:** Added `ActiveCharacterRegistry` in `cmd/webclient/handlers/active_characters.go` — a thread-safe in-process map tracking active character IDs. `WSHandler` registers on WS connect, deregisters on disconnect. `HandlePlay` returns HTTP 409 if the character is already active (unless `?force=true`). `ListCharacters` now includes `is_online` per character. TypeScript `Character` interface updated; `CharactersPage` shows an Online badge, replaces Play with Force Logout for online characters.

### BUG-90: Character selection screen displays player location in lowercase instead of room display name
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** On the character selection screen, the player's current location is shown in all lowercase (e.g. `grinders_row`) instead of the room's display name (e.g. "Grinder's Row").
**Steps:** Log in; observe the character selection screen — the location field shows the raw room ID in lowercase.
**Fix:** Applied title-case formatting in `CharactersPage.tsx` — `location.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())` converts e.g. `grinders_row` to `Grinders Row`. Full room title resolution requires injecting a world registry into the webclient handler (future work).

### BUG-89: Enfeeble technology description saving throw results presented as paragraph instead of formatted list
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The Enfeeble technology description contains a saving throw result table (critical success / success / failure / critical failure outcomes) that is rendered as a single paragraph instead of a formatted list. The YAML scalar was also truncated mid-sentence.
**Steps:** Open the Technology tab or hover over Enfeeble; read the description — saving throw outcomes run together as prose.
**Fix:** Rewrote `enfeeble_technical.yaml` `on_apply.description` as a block literal scalar with one outcome per line; added missing Failure and Critical Failure entries.

### BUG-88: Heal technology description has formatting issues and Foundry template code
**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** The Heal technology description contains three issues: (1) the action-point-to-effect mapping is written as three run-on sentences instead of a formatted list; (2) the third entry contains a raw Foundry VTT template macro `@Template[emanation|distance:30]` that must be replaced with plain-text wording; (3) the Heightened entry runs on inline instead of appearing on its own line.
**Steps:** Open the Technology tab or hover over Heal; read the description.
**Fix:** Rewrote description in `heal_bio_synthetic.yaml` and `heal_fanatic_doctrine.yaml` — removed action-variant run-on sentences (action cost is already in `action_cost` field), removed `@Template[emanation|distance:30]` Foundry markup, kept Heightened note as a clean paragraph.

### BUG-87: Rustbucket Ridge motel keeper not clickable and shows wrong descriptor
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The motel keeper NPC in Rustbucket Ridge is not clickable in the web UI room panel, and displays the descriptor `Scrap Inn Clerk (unharmed)` instead of their name. Non-combat NPCs should be clickable to interact, and should display their defined name, not a generic role label with a health suffix.
**Steps:** Navigate to Grinder's Row in Rustbucket Ridge; observe the motel keeper in the NPC list — displayed as `Scrap Inn Clerk (unharmed)` and not clickable.
**Fix:** Added `case 'motel_keeper': return '[motel]'` to `npcTypeTag()` in `RoomPanel.tsx`. The motel_keeper type was absent from the switch, causing it to fall through to the combat NPC path which adds the health suffix and omits the click handler.

### BUG-86: Player AC not displayed on character sheet
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The character sheet does not display the player's Armor Class. AC should be shown beneath the hit points field.
**Steps:** Open the character sheet; observe that AC is absent.
**Fix:** Added `AC {characterSheet.totalAc}` display to `CharacterPanel.tsx` immediately after the HP text, using the `totalAc` field already present in `CharacterSheetView`.

### BUG-85: Technology "Chrome Reflex" is a Reaction but has an "active" tag
**Severity:** medium
**Status:** fixed
**Category:** Combat
**Description:** The Chrome Reflex technology is a Reaction but is incorrectly tagged as `active` in its definition, causing it to appear in the Active category instead of the Reactions category.
**Steps:** Equip Chrome Reflex; open the Feats/Technologies tab in the web UI; observe Chrome Reflex is listed as Active instead of Reaction.
**Fix:** Updated `InnateItem` in `TechnologyDrawer.tsx` to render a "reaction" badge (blue) when `slot.isReaction` is true, instead of always rendering the "active" badge (green). Added `badgeReaction` style to the styles map.

### BUG-84: Web UI character sheet displays job ID instead of job display name
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The character sheet in the web UI shows the raw job ID (e.g. `street_fighter`) instead of the human-readable display name (e.g. "Street Fighter").
**Steps:** Log in; open the character sheet; observe the job field.
**Fix:** Reordered the `className` fallback chain in `CharacterPanel.tsx` to prefer `characterSheet?.job` (the server-resolved display name) over `characterInfo?.class` (the raw ID). New order: `characterSheet?.job ?? characterInfo?.className ?? characterInfo?.class_name ?? characterInfo?.class ?? ''`.

### BUG-83: No UI indication that player is in cover
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** When a player takes cover, there is no visible indicator in the web UI showing their current cover status or tier. Cover is valid both in and out of combat, so the indicator must be visible in both contexts.
**Steps:** Enter a room with cover equipment; take cover (in or out of combat); observe the web UI — no badge, condition icon, or status text reflects the active cover state.
**Fix:** Populated `RoomView.active_conditions` from `sess.Conditions.All()` in `world_handler.go:buildRoomView`. Cover (and all other conditions) are now sent to the client on every room view refresh. `CharacterPanel.tsx` already renders `activeConditions` as badges — conditions including cover tiers now appear.

### BUG-82: No command to exit cover
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Once a player takes cover there is no command or UI control to leave cover. The cover condition persists with no way to remove it voluntarily. Cover is valid both in and out of combat, so the exit mechanism must work in both contexts.
**Steps:** Take cover (in or out of combat); attempt to find a command or button to exit cover — none exists.
**Fix:** Added `uncover` command (alias `uc`). Added `UncoverRequest` proto message (field 135), `HandleUncover` in `command/take_cover.go`, `handleUncover` in `grpc_service.go` (calls existing `clearPlayerCover`), wired into `commands.go`, `bridge_handlers.go`, and `websocket_dispatch.go`. Returns "You are not taking cover." if no cover active, otherwise "You leave cover."

### BUG-81: Cover message references "Stealth" instead of "Ghosting"
**Severity:** low
**Status:** fixed
**Category:** Combat
**Description:** The console message shown when taking cover reads `You take standard cover. (+2 AC, +2 Stealth)` but the correct in-game skill name is Ghosting, not Stealth.
**Steps:** Enter a room with cover equipment; click the cover item or use the cover command; observe the console message.
**Fix:** Changed format string in `grpc_service.go:handleTakeCover` from `+%d Stealth` to `+%d Ghosting`.

### BUG-80: Web UI "Wear" button does nothing for armor items
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** Clicking the "Wear" button on an armor item in the Inventory tab sends a WebSocket message of type `'Wear'` that is never dispatched to the game server, so no equip action occurs.
**Steps:** Purchase an armor item from a merchant; open Inventory tab; click "Wear" on the armor item; observe no equip feedback and item remains unequipped.
**Fix:** Added `"WearRequest"` to `protoMessageByName` typeMap and `case *gamev1.WearRequest:` to `wrapProtoAsClientMessage` in `websocket_dispatch.go`. Updated `InventoryDrawer.tsx` to send `'WearRequest'` instead of `'Wear'`.

### BUG-79: Loadout Switch button fails on second use outside combat
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Outside of combat, clicking the Switch button on a loadout preset works the first time but returns "You have already swapped loadouts this round." on subsequent clicks.
**Steps:** Log in; open Equipment tab; click Switch on a loadout preset; click Switch again; observe the error.
**Fix:** In `handleLoadout`, reset `SwappedThisRound` before swapping when the player is not in combat. The once-per-round limit only applies during combat rounds; outside combat no round ever resets the flag.

### BUG-123: Weather effects not active — no weather observed across multiple seasons
**Severity:** high
**Status:** fixed
**Category:** World
**Description:** Seasonal weather effects are not being applied or displayed in-game; no weather events have been observed across multiple in-game seasons despite the WeatherManager feature being implemented.
**Steps:** Play through multiple in-game seasons; observe that no weather effects (console messages, UI indicators, room effects) appear at any point.
**Fix:** The WeatherManager was broadcasting a WeatherEvent on start/end but never the human-readable Announce/EndAnnounce text. Added MessageEvent broadcasts in OnTick (start) and endEvent (end) using the announce strings from weather.yaml. Also fixed endEvent to match the active weather by Name or ID since LoadState stores the ID while OnTick sets the Name. Added warn log in wire_gen.go when LoadWeatherTypes fails.

### BUG-122: Job tab does not show feat and technology grants with grant level
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The Job tab on the character sheet displays the job but omits the feat and technology selections granted by that job, along with the level at which each was granted.
**Steps:** Log in via web UI; open the Job tab; observe that feat and technology grants (and their grant levels) are not displayed.
**Fix:** Added JobGrantsRequest/JobGrantsResponse proto messages and a handleJobGrants server handler that reads fixed feat and tech grants (initial + level-up) from the Job registry. JobDrawer now sends JobGrantsRequest on mount and renders a "Job Grants" section grouped by level, showing feat/tech name with type badge (feat, hardwired, prepared, spontaneous) and grant level.

### BUG-121: Feat Snap Shot is not implemented as a passive feat
**Severity:** medium
**Status:** fixed
**Category:** Character
**Description:** The Snap Shot feat should be passive and apply its effect automatically; it currently requires manual activation or does not apply passively.
**Steps:** Grant a character the Snap Shot feat; observe that its benefit is not applied automatically without player action.
**Fix:** Changed snap_shot in feats.yaml from active:true to active:false. Added loop in grpc_service.go to populate sess.PassiveFeats from characterFeatsRepo (not just class features). Added MAP waiver in round.go ActionStrike: if first strike missed and actor has snap_shot passive feat, the -5 MAP on the second strike is restored.

### BUG-120: Web UI inventory consume routes item through prepared tech handler
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** Consuming an item via the web UI Inventory tab triggers the prepared technology use path instead of the item consume path, producing `No prepared uses of <item_id> remaining.`
**Steps:** Log in via web UI; open Inventory tab; click Consume on a consumable item (e.g. canadian_bacon); observe console message `No prepared uses of canadian_bacon remaining.`
**Fix:** Added plain-consumable path to handleUse in grpc_service.go: if abilityID matches a backpack item with Kind==consumable and no SubstanceID, apply its Effect (if any) via ApplyConsumable, remove one unit from backpack, and return a success message — before the prepared-tech fallback loop is reached.

### BUG-119: `use tamper` has no effect
**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Activating the `tamper` technology via `use tamper` produces no output and applies no game effect.
**Steps:** Play a character with the tamper technology; in combat or outside, run `use tamper`; observe no console feedback and no effect applied.
**Fix:** Added ConditionTarget field to Feat struct ("foe" = apply to combat target). Created tamper_debuff condition (-2 attack penalty, encounter duration). Updated tamper feat with condition_id: tamper_debuff and condition_target: foe. Modified handleUse to route foe-targeted conditions to the enemy's combat condition set using LastCombatTarget.

### BUG-141: Battle map has no controls for Close Distance or Step

**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The battle map UI does not provide buttons for the Close Distance and Step combat actions, forcing players to type these commands manually.
**Steps:** Enter combat; observe the battle map; confirm no Close Distance or Step controls are present.
**Fix:** Added "Close" (stride toward, 1 AP, 25ft) and "Step" (step toward, 1 AP, no Reactive Strikes) buttons to the battle map header alongside the existing Flee! button.

### BUG-140: Web UI enters broken state on server redeploy — stream termination not handled with auto-reconnect

**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** When a new server version is deployed the game stream is terminated, leaving the web UI in a broken/stale state with no recovery. The web UI should detect stream termination and automatically reconnect (with exponential backoff), restoring the session or redirecting to the character selection screen if the session cannot be resumed.
**Steps:** Log in to the web UI; deploy a new server version; observe the UI becomes unresponsive or shows stale state with no reconnection attempt.
**Fix:** Removed code 1001 (CloseGoingAway) from the no-reconnect list in GameContext.tsx. Only code 1000 (explicit client-initiated close on unmount) now suppresses reconnect. On reconnect, stale combat state is cleared and a system feed entry is appended.

### BUG-139: Zone exits not visually distinct on map, missing from legend, and no hover tooltip

**Severity:** medium
**Status:** open
**Category:** UI
**Description:** Zone exit rooms are not visually distinguished from normal rooms on the map, are absent from the map legend, and hovering a room with a zone exit shows no tooltip indicating the exit or its destination. Explored zone exits should show the destination zone name in the tooltip.
**Steps:** Navigate to a room with a zone exit; open the map; observe the exit room looks identical to other rooms; check the legend for a zone exit entry (none present); hover the exit room and observe no zone exit information in the tooltip.
**Fix:**

### BUG-138: Web UI character selection cards do not display XP

**Severity:** low
**Status:** fixed
**Category:** UI
**Description:** Character selection screen cards do not show the character's current XP, leaving the player with no quick view of progression before logging in.
**Steps:** Open the web UI; navigate to the character selection screen; observe character cards show no XP value.
**Fix:** Added `experience` field to `CharacterResponse` struct and `characterToResponse()` in `characters.go`. Added `experience?` to TypeScript `Character` interface. Displayed XP below HP bar in `CharacterCard` component.

### BUG-136: Player earns no XP for exploring new rooms

**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** Moving into a previously unexplored room grants no exploration XP to the player.
**Steps:** Move into a room not yet visited; observe no XP gain message and no XP increase on the character sheet.
**Fix:** Moved XP award and questSvc.RecordExplore from the AutomapCache block into the ExploredCache block in handleMove. XP is now granted on first physical entry only, not on zone-reveal pre-loading.

### BUG-135: NPC modals have no Steal option

**Severity:** medium
**Status:** open
**Category:** UI
**Description:** NPC interaction modals do not include a Steal action, leaving players with no way to attempt theft through the web UI.
**Steps:** Click any non-combat NPC to open their modal; observe no Steal button or prompt is present.
**Fix:**

### BUG-134: Merchant modals have no Negotiate button for price negotiation

**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The merchant buy/sell modal does not include a Negotiate option, leaving players with no way to attempt to negotiate better prices through the web UI.
**Steps:** Approach a merchant NPC; open the merchant modal; observe no Negotiate button or prompt is present.
**Fix:**

### BUG-133: Hotbar does not support Actions; built-in actions assigned to hotbar display as `-`

**Severity:** medium
**Status:** open
**Category:** UI
**Description:** The hotbar has no first-class support for built-in Actions (`stride`, `close`, `attack`, etc.); slots assigned to these actions render as `-` instead of a label. Additionally, the Help screen should provide a way for the player to add any Action directly to a hotbar slot.
**Steps:** Assign `stride`, `close`, or `attack` to a hotbar slot; observe the slot displays `-` instead of the action name. Open the Help screen; observe no option to add actions to the hotbar.
**Fix:**

### BUG-132: Adrenaline Surge requires Enraged condition but no mechanism exists to trigger Enraged

**Severity:** high
**Status:** open
**Category:** Combat
**Description:** Adrenaline Surge has a prerequisite of the Enraged condition, but no technology, feat, or game event applies the Enraged condition, making Adrenaline Surge permanently unusable.
**Steps:** Attempt to use Adrenaline Surge in combat; observe it is blocked due to missing Enraged condition; confirm no other ability grants Enraged.
**Fix:**

### BUG-131: Brutal Charge has no mechanical effect beyond AP cost and console text

**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Using Brutal Charge deducts AP and prints a message but applies no damage, movement, or condition effect.
**Steps:** Enter combat; use Brutal Charge; observe AP is spent and text appears in console but no mechanical effect is applied to target or player.
**Fix:** Implemented handleBrutalCharge: spends 1 AP for movement discount (2 free strides toward nearest enemy) then attacks via combatH.Attack (1 AP) for 2 AP total. Reactive Strikes fire during the charge. Movement is broadcast to all players in the room.

### BUG-130: Overpower has no mechanical effect beyond console text

**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Using Overpower prints a console message but applies no damage bonus, condition, or other mechanical effect.
**Steps:** Enter combat; use Overpower; observe console text but no change in combat state, target conditions, or damage output.
**Fix:** The feat self-condition path in handleUse only applied conditions to sess.Conditions (session-level), not cbt.Conditions[uid] (combat-level), so the brutal_surge_active condition (+2 damage, -2 AC) had no effect in combat. Fixed to apply to the combat condition set when in combat, falling back to session conditions outside combat.

### BUG-129: Web UI character creation screens do not expand to fill available screen space

**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Character creation screens are not full-height; they should always expand to fill the available viewport.
**Steps:** Open the web UI; begin character creation; observe that creation screens do not fill the full screen height.
**Fix:** Changed CharacterWizard container from maxHeight:100vh to height:100vh with box-sizing:border-box so it fills the full viewport. Increased optionGrid maxHeight from 55vh to 65vh to show more cards.

### BUG-128: Web UI displays `9 / 0 HP` — max HP calculated as zero

**Severity:** high
**Status:** fixed
**Category:** Character
**Description:** Player HP is displayed as `<current> / 0 HP` in the web UI, indicating max HP is being reported as zero.
**Steps:** Log in with an existing character; observe the HP display in the character panel showing a non-zero current HP over a zero max HP.
**Fix:** `handleStatus` and `handleExamine` in `grpc_service.go` were building `CharacterInfo` responses without the `MaxHp` field, so the proto default of 0 was used. Added `MaxHp: int32(sess.MaxHP)` to both call sites. The `CharacterSheetView` and `pushHPUpdate` already set MaxHp correctly.

### BUG-127: Battle Fervor technology has no implemented effects

**Severity:** high
**Status:** fixed
**Category:** Combat
**Description:** Using Battle Fervor produces no mechanical effect — no buff, condition, or damage modifier is applied to the player.
**Steps:** Prepare Battle Fervor; enter combat; use Battle Fervor; observe no change in player stats, conditions, or combat output.
**Fix:** Created missing content/conditions/battle_fervor_active.yaml condition definition (+2 damage, duration: 3 rounds). The technology YAML already referenced this condition ID and the tech effect resolver framework already applied conditions correctly — the definition was the only missing piece.

### BUG-126: Web UI Technology/Feat tab shows duplicate entries instead of grouped slots with use counters

**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** Technologies and Feats with multiple prepared slots appear as duplicate entries instead of being grouped by level with a per-level slot count and remaining-use counter.
**Steps:** Log in as a level 2 Street Preacher (Zealot archetype); open the Technology tab; observe Battle Fervor listed twice with no level or use-count indicator instead of two grouped entries (e.g., "Battle Fervor — Level 1 ×2 uses", "Battle Fervor — Level 2 ×1 use").
**Fix:** Added grouping of prepared slots by techId in TechnologyDrawer. Multiple slots for the same tech now display as a single entry with a remaining/total pip counter (e.g., ●●○ 2/3) instead of duplicate rows.

### BUG-125: Combat NPC names not clickable in web UI — clicking should initiate combat

**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** Combat NPC names displayed in the room view are not clickable; clicking an NPC name should trigger combat against that NPC.
**Steps:** Enter a room containing a combat NPC; observe NPC name in room panel; click the NPC name; observe no combat is initiated.
**Fix:** Made combat NPC names clickable buttons in RoomPanel.tsx. Clicking sends `attack <name>` via sendCommand, which the server parses as an AttackRequest.

### BUG-124: Web UI feat/ability hotbar activation shows no feedback
**Severity:** high
**Status:** fixed
**Category:** UI
**Description:** Clicking a hotbar slot assigned to a feat or ability in the web client shows no visible feedback even though the feat is activated server-side.
**Steps:** Add an active feat to a hotbar slot via the Feats drawer; click the hotbar slot; observe no message or effect in the console feed.
**Fix:** `UseResponse` was missing from `serverEventInner` in `cmd/webclient/handlers/websocket.go`, causing the server's activation feedback to be silently dropped. Added `case *gamev1.ServerEvent_UseResponse:` returning `(p.UseResponse, "UseResponse")`. Added `case 'UseResponse':` handler in `GameContext.tsx` that appends the message to the feed (or lists available abilities when `choices` is populated).

### BUG-137: Stale closure in GameContext.tsx ws.onmessage prevents player HP updates from CombatEvents
**Severity:** medium
**Status:** fixed
**Category:** UI
**Description:** The `ws.onmessage` handler in `connect` (useCallback with deps `[navigate]`) captures `state` at creation time. When a `CombatEvent` arrives with `ce.target == player name`, the `state.characterInfo?.name` check always fails because `state` is the initial null value, so `UPDATE_PLAYER_HP` is never dispatched.
**Steps:** Enter combat; take damage from an NPC attack targeting your character; observe that HP in the UI does not update from combat events (only from explicit CharacterSheet refreshes).
**Fix:** Moved the player-name comparison into the UPDATE_COMBATANT_HP reducer case where state is always current. The reducer now also updates characterInfo HP when the combatant name matches the player's name, eliminating the stale closure entirely.
