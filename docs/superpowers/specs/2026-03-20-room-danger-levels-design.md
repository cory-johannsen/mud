# Room Danger Levels — Design Spec

**Date:** 2026-03-20
**Goal:** Define the data model, package API, enforcement logic, trap probability integration, and map display for the Room Danger Levels feature.

---

## Requirements

- REQ-DL-1: Zones MUST declare a `danger_level` field; rooms MAY override it.
- REQ-DL-2: Safe room second violation MUST increment WantedLevel and trigger guard combat.
- REQ-DL-3: WantedLevel MUST decay by 1 level per in-game day when no new violations occur.
- REQ-DL-4: Active clearing is handled by the `wanted-clearing` feature and is out of scope here.
- REQ-DL-5: WantedLevel 1 MUST cause merchants to add a 10% surcharge (out of scope — no merchants yet).
- REQ-DL-6: WantedLevel 2+ MUST cause guards to initiate combat to detain (non-lethal intent; see Section 3).
- REQ-DL-7: WantedLevel 3-4 MUST cause guards to attack on sight (lethal intent; see Section 3).
- REQ-DL-8: Zone YAML MAY override default trap probabilities.
- REQ-DL-9: Map cells MUST be color-coded by danger level.
- REQ-DL-10: Unexplored rooms MUST display as light gray; explored state is tracked per player.

---

## 1. Data Model

### DangerLevel Type

A `DangerLevel` typed string is defined in `internal/game/danger/level.go` with four constants:

| Constant | Value |
|----------|-------|
| `DangerLevelSafe` | `"safe"` |
| `DangerLevelSketchy` | `"sketchy"` |
| `DangerLevelDangerous` | `"dangerous"` |
| `DangerLevelAllOutWar` | `"all_out_war"` |

`EffectiveDangerLevel(zoneDanger, roomDanger string) DangerLevel` MUST return the room override if set, otherwise MUST return the zone default.

### Zone and Room YAML Fields

Per REQ-DL-1, `Zone` gains a required `danger_level string` YAML field. `Room` gains an optional `danger_level string` override field (same field name). Rooms without the field MUST inherit the zone value via `EffectiveDangerLevel`.

Trap probability overrides (REQ-DL-8):

- `room_trap_chance *int` — optional pointer. A nil value MUST cause the handler to use the danger level default. A non-nil value (including a pointer to `0`) MUST be used as the explicit override, overriding the danger level default entirely.
- `cover_trap_chance *int` — optional pointer. Same nil/non-nil semantics as `room_trap_chance`. In YAML, omitting the field yields nil; setting `cover_trap_chance: 0` yields a pointer to `0`.

Default trap probabilities by danger level:

| Danger Level | Room Trap Chance | Cover Trap Chance |
|--------------|-----------------|------------------|
| Safe | 0% | 0% |
| Sketchy | 0% | 15% |
| Dangerous | 35% | 50% |
| All Out War | 60% | 75% |

### WantedLevel

Per-player per-zone notoriety, stored as 5 discrete levels (0 = None through 4 = D.O.S.):

| Level | Name |
|-------|------|
| 0 | None |
| 1 | Flagged |
| 2 | Burned |
| 3 | Hunted |
| 4 | D.O.S. |

WantedLevel is persisted in a new `character_wanted_levels(character_id, zone_id, wanted_level)` database table and cached in `PlayerSession.WantedLevel map[string]int` (key = zone_id).

### Safe Room Violations

`PlayerSession.SafeViolations map[string]int` (key = zone_id) tracks per-player per-zone unsafe combat attempts. This counter MUST reset when WantedLevel increments for that zone.

### WantedLevel Decay

Per REQ-DL-3, WantedLevel MUST decay by 1 level per in-game day when no new violations occurred. Decay is driven by a Persistent Calendar hook.

`PlayerSession` MUST carry a `LastViolationDay map[string]int` field (key = zone_id, value = in-game day number from the calendar). The decay hook MUST compare the current in-game day to `LastViolationDay[zoneID]`. If the current day is greater than `LastViolationDay[zoneID]`, WantedLevel for that zone MUST decrement by 1. `CheckSafeViolation` MUST set `LastViolationDay[zoneID] = currentDay` on each violation so that the decay hook can correctly detect days without violations.

### WantedRepository Contract

`WantedRepository` MUST implement the following upsert/load contract:

- Rows MUST be absent from the database when `WantedLevel == 0`. When a decrement reduces `WantedLevel` to 0, the repository MUST delete the row rather than store a zero value.
- Primary key is `(character_id, zone_id)`.
- `Load(characterID string) map[string]int` MUST return all zones with non-zero WantedLevel for that character. Zones with no row MUST be treated as WantedLevel 0.
- `Upsert(characterID, zoneID string, level int)` MUST insert or update the row. If `level == 0`, `Upsert` MUST delete the row instead.

### Cover Tier

`CoverTier string` MUST be added to the equipment/item model in `internal/game/world/model.go`. Valid values are `""` (not usable as cover), `"light"`, `"medium"`, and `"heavy"`. `RenderRoomView` MUST append `[Cover: light]`, `[Cover: medium]`, or `[Cover: heavy]` to an item's display only when `CoverTier` is non-empty.

### Explored Room Persistence

Explored room state MUST be persisted in a `character_map_rooms(character_id, zone_id, room_id)` table. `handleMove` MUST call `AutomapRepository.Insert(characterID, zoneID, roomID)` after each successful move; this operation MUST be idempotent (INSERT OR IGNORE semantics). `AutomapCache` MUST be populated at session load from `AutomapRepository.LoadAll(characterID)`. `RenderMap()` MUST check `AutomapCache` to determine whether each cell is explored or unexplored, and MUST render unexplored cells in light gray regardless of danger level.

---

## 2. `danger` Package API

The `danger` package lives in `internal/game/danger/` and contains only pure functions with no state.

```go
func EffectiveDangerLevel(zoneDanger, roomDanger string) DangerLevel
func CanInitiateCombat(level DangerLevel, initiator string) bool
func RollRoomTrap(level DangerLevel, overridePct *int, rng Roller) bool
func RollCoverTrap(level DangerLevel, overridePct *int, rng Roller) bool
```

`RollRoomTrap` and `RollCoverTrap` MUST accept `overridePct *int`. When `overridePct` is nil, the function MUST use the danger level default probability. When `overridePct` is non-nil, the function MUST use `*overridePct` as the roll threshold, even if `*overridePct == 0`.

The `Roller` interface is defined as:

```go
type Roller interface {
    Roll(max int) int
}
```

Production code MUST use `math/rand`. Tests MUST inject a deterministic source.

`CanInitiateCombat` behavior by danger level:

| Danger Level | player | npc |
|--------------|--------|-----|
| safe | false | false |
| sketchy | true | false |
| dangerous | true | true |
| all_out_war | true | true (attack on sight) |

`CheckSafeViolation` is NOT in the `danger` package. It lives in the enforcement layer (see Section 3).

**Important:** Guard enforcement via `InitiateGuardCombat` MUST bypass `CanInitiateCombat`. The `CanInitiateCombat` function governs normal NPC combat initiation only. Guard enforcement is a separate code path called from `CheckSafeViolation` and `handleMove` (WantedLevel check) and MUST NOT consult `CanInitiateCombat`.

---

## 3. NPC Combat Enforcement

`CheckSafeViolation(sess, zoneID, combatH)` is defined in `internal/gameserver/enforcement.go`:

1. If the effective danger level is not `safe`, this function MUST be a no-op.
2. `sess.SafeViolations[zoneID]++`; `sess.LastViolationDay[zoneID] = currentDay`.
3. If violations == 1: emit a warning message and block the attack.
4. If violations >= 2 (REQ-DL-2): `sess.WantedLevel[zoneID]++`, reset `sess.SafeViolations[zoneID] = 0`, call `combatH.InitiateGuardCombat(uid, zoneID, sess.WantedLevel[zoneID])`.

WantedLevel NPC effects:

- WantedLevel 2 (REQ-DL-6): Guards MUST call `InitiateGuardCombat` with non-lethal intent. Guards MUST emit a narrative call for surrender (e.g., "Guards shout: Drop your weapon and surrender!"). This is a narrative difference only; mechanical surrender processing is deferred to the `wanted-clearing` feature.
- WantedLevel 3-4 (REQ-DL-7): Guards MUST attack to kill. Guards MUST NOT emit a surrender call. The `wantedLevel int` parameter passed to `InitiateGuardCombat` determines which behavior the guard NPC exhibits.

`InitiateGuardCombat(uid, zoneID string, wantedLevel int)` on `CombatHandler` MUST find guard NPCs in the player's current room and start combat with them. The `wantedLevel` parameter MUST be used to distinguish non-lethal (level 2) from lethal (level 3-4) guard behavior. This function MUST be a no-op if no guards are present.

WantedLevel decay MUST be hooked into the calendar tick handler. On each in-game day: decrement WantedLevel by 1 for each zone where `WantedLevel > 0` and `currentDay > LastViolationDay[zoneID]` (REQ-DL-3). When WantedLevel decrements to 0, the row MUST be deleted from `character_wanted_levels` (see WantedRepository Contract).

---

## 4. Trap Probability Enforcement

Zone and Room YAML fields `room_trap_chance *int` and `cover_trap_chance *int` are optional pointer overrides (REQ-DL-8). A nil pointer MUST cause the handler to use the danger level default. A non-nil pointer (including `*0`) MUST be used as the explicit override probability, bypassing the danger level default.

Roll sites:

- **Room trap:** Called in `handleMove` after the player enters the room, before the room description is sent. If `RollRoomTrap` returns true, the handler MUST emit a `COMBAT_EVENT_TYPE_MESSAGE` narrative event (e.g., "You trigger a trap!") and MUST log the event. No further trap effect MUST be applied at this layer; actual trap damage and effects are deferred to the Traps feature. This is fully specified behavior, not a placeholder.
- **Cover trap:** Called in `handleUse` when the player uses or examines cover equipment. If `RollCoverTrap` returns true, the handler MUST emit a `COMBAT_EVENT_TYPE_MESSAGE` narrative event (e.g., "You trigger a trap!") and MUST log the event. No further trap effect MUST be applied at this layer; actual trap damage and effects are deferred to the Traps feature. This is fully specified behavior, not a placeholder.

Cover tier display: Room equipment descriptions MUST append `[Cover: light]`, `[Cover: medium]`, or `[Cover: heavy]` based on the item's `CoverTier` field only when `CoverTier` is non-empty. This is a display-only change to `RenderRoomView`.

---

## 5. Map Display and Architecture Documentation

`MapTile` proto MUST gain a `danger_level string` field. `handleMap` MUST populate it from the room's effective danger level.

`RenderMap()` MUST color-code cells using ANSI escape codes:

| Danger Level | Color | ANSI Code |
|--------------|-------|-----------|
| safe | Green | `\033[32m` |
| sketchy | Yellow | `\033[33m` |
| dangerous | Orange | `\033[38;5;208m` |
| all_out_war | Red | `\033[31m` |
| unexplored | Light Gray | `\033[37m` |

Per REQ-DL-10, unexplored rooms MUST display as light gray regardless of their actual danger level.

**Addendum: Architecture Documentation** *(not required by any REQ-DL-* requirement; included per user request)*

A new architecture document MUST be created at `docs/architecture/map-system.md` documenting:

- `character_map_rooms` DB table schema
- `AutomapRepository` (Insert/LoadAll)
- `PlayerSession.AutomapCache` structure
- `MapTile` proto fields
- `handleMap`/`handleMove` discovery flow
- `RenderMap()` rendering pipeline
- Danger-level color coding

---

## File Map

| File | Change |
|------|--------|
| `internal/game/danger/level.go` | New — DangerLevel type, constants, EffectiveDangerLevel |
| `internal/game/danger/combat.go` | New — CanInitiateCombat |
| `internal/game/danger/trap.go` | New — RollRoomTrap, RollCoverTrap, Roller interface |
| `internal/game/world/model.go` | Add DangerLevel, trap override fields, CoverTier to Zone + Room + Item |
| `internal/game/session/manager.go` | Add WantedLevel, SafeViolations, LastViolationDay to PlayerSession |
| `internal/storage/postgres/wanted.go` | New — WantedRepository (upsert/load/delete) |
| `internal/gameserver/enforcement.go` | New — CheckSafeViolation, calendar decay hook wiring |
| `internal/gameserver/combat_handler.go` | Add InitiateGuardCombat(uid, zoneID string, wantedLevel int) |
| `internal/gameserver/grpc_service.go` | Wire enforcement into handleMove, handleAttack |
| `api/proto/game/v1/game.proto` | Add danger_level to MapTile |
| `internal/frontend/handlers/text_renderer.go` | Color-code map cells by danger level |
| `content/zones/**/*.yaml` | Add danger_level to all zone YAML files |
| `docs/architecture/map-system.md` | New — map system architecture doc (addendum) |
