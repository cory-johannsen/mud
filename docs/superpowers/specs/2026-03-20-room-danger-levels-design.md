# Room Danger Levels — Design Spec

**Date:** 2026-03-20
**Status:** Approved
**Feature:** `room-danger-levels` (priority 10)

---

## Overview

Rooms and zones are classified by danger level: Safe, Sketchy, Dangerous, or All Out War. Danger level governs combat initiation rules, NPC behavior, cover availability, and map/room display. It also drives the Wanted system, which tracks player notoriety and modifies NPC responses across all zone types.

---

## 1. Data Model

### DangerLevel Enum

Four values (stored as lowercase strings in YAML):

| Value | Display Name |
|-------|-------------|
| `safe` | Safe |
| `sketchy` | Sketchy |
| `dangerous` | Dangerous |
| `all_out_war` | All Out War |

### YAML Schema

`danger_level` is a **required** top-level field on zones. A zone YAML missing `danger_level` MUST be treated as a fatal load error. It is **optional** on rooms; rooms without the field inherit the zone value at load time.

```yaml
# Zone level — required
zone:
  id: rustbucket_ridge
  name: Rustbucket Ridge
  danger_level: sketchy      # default for all rooms in zone
  rooms:
    - id: last_stand_lodge
      title: Last Stand Lodge
      danger_level: safe     # overrides zone default for this room
    - id: the_heap
      title: The Heap
                             # inherits zone default (sketchy)
```

### Go Structs

- `Zone` gains field: `DangerLevel DangerLevel`
- `Room` gains field: `DangerLevel DangerLevel`
- Populated at zone load time: if a room's YAML `danger_level` field is empty, the room inherits the zone's value. A zone with an empty `danger_level` is a fatal load error.
- An unknown `DangerLevel` string value (not one of the four valid values) MUST be a fatal load error.
- `Zone.Validate()` MUST be updated to enforce this.

---

## 2. Combat Enforcement

Rules are enforced at **action resolution time** (not command parse time), so the attempt is logged before rejection.

### Per-Zone Rules

| Zone Type | Player initiates combat | NPC initiates combat |
|-----------|------------------------|---------------------|
| Safe | Blocked (see violation flow) | Never |
| Sketchy | Allowed | Never (unless WantedLevel overrides — see Section 3) |
| Dangerous | Allowed | Allowed |
| All Out War | Allowed | Allowed — attack on sight |

### Safe Room Violation Flow

A per-session, per-room violation counter is tracked on the player session (not persisted):

- REQ-DL-1: On the **first** attempt to initiate combat in a Safe room, the action MUST be rejected with a warning message. The violation counter for that room MUST be incremented.
- REQ-DL-2: On the **second** attempt in the same room within the same session, the action MUST be rejected, the player's `WantedLevel` MUST be incremented by 1 (capped at 4), and any guards currently present in the room MUST immediately enter combat with the player. If no guards are present at that moment, `WantedLevel` still increments. Guards that enter the room after the violation event do NOT retroactively trigger combat; guard engagement is evaluated only at violation time.
- REQ-DL-3: The violation counter MUST reset when the player exits the room.

### All Out War — Attack on Sight

- REQ-DL-4: When a player enters an All Out War room, all combat-capable NPCs in the room MUST immediately initiate combat. This check occurs on room entry, not on a periodic tick.

---

## 3. Wanted System

### WantedLevel Field

`WantedLevel` is an integer 0–4 stored on the player character and persisted to the database.

### Levels

| Level | Name | NPC Behavior |
|-------|------|-------------|
| 0 | None | Normal interactions |
| 1 | Flagged | Guards watch the player; merchants add a 10% surcharge to all transactions |
| 2 | Burned | Guards detain (initiate combat with) the player on sight in Safe rooms |
| 3 | Hunted | Guards attack on sight in Safe rooms; combat-capable NPCs (those that can participate in combat, as opposed to non-combat NPCs defined in the `non-combat-npcs` feature) engage on sight in Sketchy rooms, overriding the normal Sketchy no-NPC-initiation rule |
| 4 | D.O.S. | All combat-capable NPCs attack the player on sight, regardless of zone type |

*D.O.S. = Dead on Sight. Cyberpunk nod to denial-of-service — the player has been flagged for termination.*

**WantedLevel 3+ and Sketchy zones:** At WantedLevel 3 or higher, combat-capable NPCs in Sketchy rooms MUST initiate combat on sight. This explicitly overrides the standard Sketchy rule that NPCs never initiate combat.

**WantedLevel 4 and further offenses:** When a player at WantedLevel 4 commits an additional offense in a Safe room, the action is still rejected, a message informs the player that their bounty cannot get any worse, and guards present enter combat as normal per REQ-DL-2. WantedLevel remains at 4.

### Decay

- REQ-DL-5: On each daily calendar tick, if `WantedLevel` was not incremented during the preceding in-game day, `WantedLevel` MUST be decremented by 1.
- REQ-DL-6: `WantedLevel` MUST NOT decay below 0.

### Active Clearing

Active clearing methods (bribe, surrender, quest) are out of scope for this feature. They are defined in the `wanted-clearing` feature (priority 25), which depends on `non-combat-npcs`.

### Offenses

- REQ-DL-7: A second combat attempt in a Safe room MUST increment `WantedLevel` by 1, capped at 4.
- Additional offense triggers are defined in downstream feature specs (e.g., stealing from a merchant).

---

## 4. Display

### Room View

A color-coded danger level badge is displayed in the room header, on the same line as the room title or immediately below it:

```
[ Pioneer Courthouse Square ]  ◈ DANGEROUS
```

| Level | Color |
|-------|-------|
| Safe | Bright green |
| Sketchy | Yellow |
| Dangerous | Orange |
| All Out War | Red |

The badge uses the existing ANSI color system.

### Zone Map — Explored Room Tracking

Room danger level is only revealed once a player has entered a room. A new `ExploredRooms` field (type `map[string]bool`, keyed by `zoneID/roomID`) is added to the player character and persisted to the database. On every room entry event, the room is added to `ExploredRooms`. Key construction MUST use a named helper function (e.g., `exploredRoomKey(zoneID, roomID string) string`) that asserts both components are non-empty, preventing silent map pollution from zero-value IDs.

- REQ-DL-8: Unexplored rooms (not present in `ExploredRooms`) MUST render in light gray on the zone map, regardless of actual danger level.
- REQ-DL-9: Explored rooms MUST render in their danger-level color on the zone map.

### Cover Items

Cover is implemented via `cover_tier` on `RoomEquipmentConfig` (values: `"lesser"`, `"standard"`, `"greater"`, or `""`).

- REQ-DL-10: At render time, the room equipment listing MUST display a `[cover]` tag for any equipment entry whose `cover_tier` is non-empty. This tag MUST be derived from `CoverTier != ""` at render time — it is not stored as literal text on the item definition. If this tag is already rendered, no change is required; the implementation MUST verify and add it where missing.

---

## 5. Cover Reference

Cover is an existing mechanic. Danger level governs cover behavior as follows (no new cover implementation required):

| Zone Type | Cover present? | Cover destructible? |
|-----------|---------------|---------------------|
| Safe | No | N/A |
| Sketchy | Possible | No |
| Dangerous | Possible | Yes |
| All Out War | Possible | Yes |

**Safe rooms and cover:** The `take_cover` command MUST be rejected in Safe rooms at runtime (cover items may technically be present in room equipment data, but the action is unavailable because Safe rooms disable combat). No load-time validation is required for this case.

Cover trap probability per zone type is defined in the `traps` feature spec (priority 15).

The cover mechanic documentation MUST be updated to describe per-zone destructibility behavior if not already documented.

---

## 6. Out of Scope

The following are explicitly deferred to other features:

- Active Wanted clearing (bribe, surrender, quest) → `wanted-clearing` feature
- Non-combat NPC flee/cower behavior → `non-combat-npcs` feature
- Guard NPC implementation → `non-combat-npcs` feature
- Trap probability per zone type → `traps` feature
- Cover mechanic implementation → already done

---

## Requirements Summary

- REQ-DL-1: First combat attempt in a Safe room MUST be rejected with a warning message; violation counter incremented.
- REQ-DL-2: Second combat attempt in a Safe room MUST be rejected; WantedLevel incremented (capped at 4); guards present at that moment enter combat. Guards arriving later do NOT retroactively engage.
- REQ-DL-3: Violation counter MUST reset on room exit.
- REQ-DL-4: Entering an All Out War room MUST trigger immediate NPC combat initiation on room entry.
- REQ-DL-5: On each daily tick, if WantedLevel was not incremented in the preceding day, it MUST be decremented by 1.
- REQ-DL-6: WantedLevel MUST NOT decay below 0.
- REQ-DL-7: Second Safe room combat attempt MUST increment WantedLevel by 1, capped at 4.
- REQ-DL-8: Unexplored map rooms MUST render in light gray.
- REQ-DL-9: Explored map rooms MUST render in danger-level color.
- REQ-DL-10: Room equipment with non-empty `cover_tier` MUST display a `[cover]` tag in the room equipment listing, derived at render time.
