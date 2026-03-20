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
- REQ-DL-6: WantedLevel 2+ MUST cause guards to initiate combat to detain.
- REQ-DL-7: WantedLevel 3-4 MUST cause guards to attack on sight.
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

`EffectiveDangerLevel(zoneDanger, roomDanger string) DangerLevel` returns the room override if set, otherwise returns the zone default.

### Zone and Room YAML Fields

Per REQ-DL-1, `Zone` gains a required `danger_level string` YAML field. `Room` gains an optional `danger_level string` override field (same field name). Rooms without the field inherit the zone value via `EffectiveDangerLevel`.

Trap probability overrides (REQ-DL-8):

- `room_trap_chance int` — optional; 0 means use the danger level default.
- `cover_trap_chance int` — optional; 0 means use the danger level default.

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

`PlayerSession.SafeViolations map[string]int` (key = zone_id) tracks per-player per-zone unsafe combat attempts. This counter resets when WantedLevel increments for that zone.

### WantedLevel Decay

Per REQ-DL-3, WantedLevel decays by 1 level per in-game day when no new violations occurred. Decay is driven by a Persistent Calendar hook.

---

## 2. `danger` Package API

The `danger` package lives in `internal/game/danger/` and contains only pure functions with no state.

```go
func EffectiveDangerLevel(zoneDanger, roomDanger string) DangerLevel
func CanInitiateCombat(level DangerLevel, initiator string) bool
func RollRoomTrap(level DangerLevel, overridePct int, rng Roller) bool
func RollCoverTrap(level DangerLevel, overridePct int, rng Roller) bool
```

The `Roller` interface is defined as:

```go
type Roller interface {
    Roll(max int) int
}
```

Production code uses `math/rand`. Tests inject a deterministic source.

`CanInitiateCombat` behavior by danger level:

| Danger Level | player | npc |
|--------------|--------|-----|
| safe | false | false |
| sketchy | true | false |
| dangerous | true | true |
| all_out_war | true | true (attack on sight) |

`CheckSafeViolation` is NOT in the `danger` package. It lives in the enforcement layer (see Section 3).

---

## 3. NPC Combat Enforcement

`CheckSafeViolation(sess, zoneID, combatH)` is defined in `internal/gameserver/enforcement.go`:

1. If the effective danger level is not `safe`, this function is a no-op.
2. `sess.SafeViolations[zoneID]++`
3. If violations == 1: emit a warning message and block the attack.
4. If violations >= 2 (REQ-DL-2): `sess.WantedLevel[zoneID]++`, reset `sess.SafeViolations[zoneID] = 0`, call `combatH.InitiateGuardCombat(uid, zoneID)`.

WantedLevel NPC effects:

- WantedLevel 2+ (REQ-DL-6): Guards call `InitiateGuardCombat` when the player enters a room.
- WantedLevel 3-4 (REQ-DL-7): Same as WantedLevel 2+; guards attack on sight.

`InitiateGuardCombat(uid, zoneID string)` on `CombatHandler` finds guard NPCs in the player's current room and starts combat with them. This function is a no-op if no guards are present.

WantedLevel decay is hooked into the calendar tick handler. On each in-game day: decrement WantedLevel by 1 for each zone where `WantedLevel > 0` and no new violations occurred that day (REQ-DL-3).

---

## 4. Trap Probability Enforcement

Zone and Room YAML fields `room_trap_chance int` and `cover_trap_chance int` are optional overrides. A value of 0 means use the danger level defaults (REQ-DL-8).

Roll sites:

- **Room trap:** Called in `handleMove` after the player enters the room, before the room description is sent. If the roll returns true, emit `// TODO(traps): trigger trap`.
- **Cover trap:** Called in `handleUse` when the player uses or examines cover equipment. If the roll returns true, emit `// TODO(traps): trigger trap`.

Cover tier display: Room equipment descriptions append `[Cover: light/medium/heavy]` based on the item's cover tier field. This is a display-only change to `RenderRoomView`.

---

## 5. Map Display and Architecture Documentation

`MapTile` proto gains a `danger_level string` field. `handleMap` populates it from the room's effective danger level.

`RenderMap()` color-codes cells using ANSI escape codes:

| Danger Level | Color | ANSI Code |
|--------------|-------|-----------|
| safe | Green | `\033[32m` |
| sketchy | Yellow | `\033[33m` |
| dangerous | Orange | `\033[38;5;208m` |
| all_out_war | Red | `\033[31m` |
| unexplored | Light Gray | `\033[37m` |

Per REQ-DL-10, unexplored rooms MUST display as light gray regardless of their actual danger level.

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
| `internal/game/world/model.go` | Add DangerLevel, trap override fields to Zone + Room |
| `internal/game/session/manager.go` | Add WantedLevel, SafeViolations to PlayerSession |
| `internal/storage/postgres/wanted.go` | New — WantedRepository (upsert/load) |
| `internal/gameserver/enforcement.go` | New — CheckSafeViolation, calendar decay hook wiring |
| `internal/gameserver/combat_handler.go` | Add InitiateGuardCombat |
| `internal/gameserver/grpc_service.go` | Wire enforcement into handleMove, handleAttack |
| `api/proto/game/v1/game.proto` | Add danger_level to MapTile |
| `internal/frontend/handlers/text_renderer.go` | Color-code map cells by danger level |
| `content/zones/**/*.yaml` | Add danger_level to all zone YAML files |
| `docs/architecture/map-system.md` | New — map system architecture doc |
