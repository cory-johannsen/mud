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
**Status:** open
**Category:** World
**Description:** The `buy` command fails to match merchant items by display name, partial name, or slug — none of "Sawn-Off", "sawn-off", "sawn-off-shotgun", or "Sawn-Off Shotgun" resolve to the Sawn-Off Shotgun in Sergeant Mack's inventory.
**Steps:** Browse Sergeant Mack's wares (`browse mack`); attempt `buy mack sawn-off`, `buy mack sawn-off-shotgun`, `buy mack Sawn-Off`, `buy mack Sawn-Off Shotgun`; all return "Sergeant Mack doesn't sell <term>".
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
**Status:** open
**Category:** UI
**Description:** In the web client, hotbar slot data appears as a system message in the FeedPanel after every server event, instead of only updating the fixed HotbarPanel row.
**Steps:** Log in with a character; observe the FeedPanel; after any server event (room entry, combat message, status update) a hotbar line appears in the feed output.
**Fix:**
