# Exploration Mode — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `exploration` (priority 235)
**Dependencies:** `actions`, `persistent-calendar`, `npc-awareness`

---

## Overview

Exploration Mode gives players a persistent stance that modifies how they interact with the world between combat encounters. A player has at most one active mode at a time. The mode persists across room transitions until the player explicitly changes or clears it. Seven modes are defined; each fires effects on room entry or combat start via hooks on `PlayerSession`.

---

## 1. Data Model

`PlayerSession` gains two fields:

```go
// ExploreMode is the player's current exploration stance.
// Empty string means no active mode.
// Valid values: "lay_low", "hold_ground", "active_sensors", "case_it",
//               "run_point", "shadow", "poke_around"
ExploreMode string

// ExploreShadowTarget is the character ID of the ally being shadowed.
// Only meaningful when ExploreMode == "shadow".
ExploreShadowTarget int64
```

`ExploreMode` is session-only state — it is NOT persisted to the database. On reconnect the player starts with no active mode. This matches the PF2E pattern where exploration mode is re-declared at the start of each exploration sequence.

---

## 2. The `explore` Command

**Syntax:**

```
explore                        — display current mode and shadow target (if any), or "No active exploration mode."
explore <mode>                 — set mode; fires immediate hook for active_sensors and case_it
explore shadow <ally-name>     — set Shadow mode targeting a named player in the same room
explore off                    — clear active mode
```

When a mode IS active, `explore` with no argument displays:

```
Exploration mode: <Display Name> [shadowing <ally-name>]
```

The `[shadowing <ally-name>]` suffix is included only when `ExploreMode == "shadow"` and the target is currently in the same room. If the shadow target has left or disconnected, the suffix reads `[shadowing <ally-name> — not present]`.

**Valid mode names** (case-insensitive):

| Command argument | Mode ID | Display Name |
|---|---|---|
| `lay_low` | `lay_low` | Lay Low |
| `hold_ground` | `hold_ground` | Hold Ground |
| `active_sensors` | `active_sensors` | Active Sensors |
| `case_it` | `case_it` | Case It |
| `run_point` | `run_point` | Run Point |
| `shadow` | `shadow` | Shadow |
| `poke_around` | `poke_around` | Poke Around |

**Immediate-fire modes:** `active_sensors` and `case_it` fire their room-entry hook immediately on mode set, in addition to firing on subsequent room entries. This allows players to scan their current room without moving.

**Blocked in combat:** `explore` MUST be rejected with an error message if the player is currently in combat. Exploration modes are non-combat stances.

- REQ-EXP-1: `explore <mode>` MUST set `PlayerSession.ExploreMode` to the given mode ID and confirm to the player.
- REQ-EXP-2: `explore off` MUST clear `PlayerSession.ExploreMode` to empty string and confirm to the player.
- REQ-EXP-3: Setting a new mode while one is already active MUST replace the old mode without requiring `explore off` first.
- REQ-EXP-4: `explore` MUST be rejected with an error message when the player is in active combat.
- REQ-EXP-5: `active_sensors` and `case_it` MUST fire their room-entry hooks immediately upon being set.
- REQ-EXP-6: `explore shadow` without a valid player name in the same room MUST fail with an error message.

---

## 3. Mode Behaviors

### 3.1 Lay Low

**Trigger:** Room entry.

The player makes a secret Ghosting check against the highest Awareness DC among all NPCs currently in the room. "Awareness DC" is `10 + npc.Awareness` (the `awareness` field on `npc.Instance`, per the `npc-awareness` feature).

| Outcome | Effect |
|---|---|
| Critical Success | Player gains `hidden` and `undetected` conditions against all NPCs in the room |
| Success | Player gains `hidden` condition against all NPCs in the room |
| Failure | No effect; player enters normally |
| Critical Failure | NPCs are alerted; player cannot gain `hidden` or `undetected` in this room for the current visit |

If no NPCs are present in the room, no check is made and no conditions are applied.

Combat start automatically clears `lay_low` mode — the player cannot maintain a hiding stance once initiative begins.

- REQ-EXP-7: Lay Low MUST make a secret Ghosting check vs. the highest NPC Awareness DC (`10 + instance.Awareness`) in the room on room entry.
- REQ-EXP-8: Critical success MUST apply both `hidden` and `undetected` conditions; success MUST apply `hidden` only.
- REQ-EXP-8a: Failure MUST have no effect. Critical failure MUST prevent the player from gaining `hidden` or `undetected` in the current room for the duration of the visit (cleared on room exit).
- REQ-EXP-9: Lay Low MUST be cleared automatically when combat starts.
- REQ-EXP-10: If no NPCs are present, Lay Low MUST make no check and apply no conditions.

### 3.2 Hold Ground

**Trigger:** Combat start — specifically, when AP is granted to the player for their first initiative slot in a new combat.

When the player's first initiative slot fires and AP is granted, `shield_raised` is automatically applied at no AP cost. If the player has no shield equipped, the mode fires silently with no effect (no error, no message).

`shield_raised` applied by Hold Ground follows the normal `shield_raised` rules: it expires at the start of the player's next initiative slot (i.e., when AP is next granted to them).

- REQ-EXP-11: Hold Ground MUST apply `shield_raised` when AP is granted to the player for their first initiative slot in a new combat, at no AP cost.
- REQ-EXP-12: If the player has no shield equipped, Hold Ground MUST have no effect and MUST NOT produce an error message.
- REQ-EXP-13: The applied `shield_raised` MUST follow normal shield rules and expire when AP is next granted to the player (their next initiative slot).

### 3.3 Active Sensors

**Trigger:** Room entry (and immediately on mode set).

The player makes a secret Tech Lore check (DC set by room danger level — see table below). On success, a separate console message is sent to the scanning player listing all active or activatable Technology present in the room. On critical success, concealed technology is also revealed (hidden tech items, trap tech triggers). On failure, no message is sent.

This message is delivered after the room description, as a console suffix — it does NOT modify the room view struct.

| Danger Level | DC |
|---|---|
| Safe | 12 |
| Sketchy | 16 |
| Dangerous | 20 |
| All Out War | 24 |
| Unset (fallback) | 16 (treat as Sketchy) |

"Active or activatable Technology" means: any Technology item on the room floor, in a room equipment slot, or carried by an NPC in the room that has at least one charge or use remaining.

- REQ-EXP-14: Active Sensors MUST make a secret Tech Lore check on room entry and immediately on mode set.
- REQ-EXP-15: Success MUST send a console message to the scanning player listing detected technology. The room view struct MUST NOT be modified.
- REQ-EXP-16: Critical success MUST additionally reveal concealed technology (hidden items, trap tech triggers).
- REQ-EXP-17: Failure MUST send no message and reveal nothing.
- REQ-EXP-18: The DC MUST be determined by the room's current danger level per the table above. If no danger level is set, DC MUST default to 16 (Sketchy).

### 3.4 Case It

**Trigger:** Room entry (and immediately on mode set).

**Note:** `case_it` is the implementation of the "Search mode" referenced in REQ-TR-3 and REQ-TR-4 of the `traps` feature spec. When the traps system checks `sess.ExploreMode`, it checks for the string `"case_it"`.

The player makes a secret Awareness check (DC set by room danger level — same table as Active Sensors). On success, hidden exits, concealed items, and trap triggers are revealed to the player via a console message delivered after the room description. On critical success, trap details are also revealed (trap type and rough DC range). On failure, nothing is revealed.

| Danger Level | DC |
|---|---|
| Safe | 12 |
| Sketchy | 16 |
| Dangerous | 20 |
| All Out War | 24 |
| Unset (fallback) | 16 (treat as Sketchy) |

- REQ-EXP-19: Case It MUST make a secret Awareness check on room entry and immediately on mode set.
- REQ-EXP-20: Success MUST reveal hidden exits, concealed items, and trap triggers via a console message. The room view struct MUST NOT be modified.
- REQ-EXP-21: Critical success MUST additionally reveal trap type and rough DC range.
- REQ-EXP-22: Failure MUST reveal nothing.
- REQ-EXP-23: The DC MUST be determined by the room's current danger level per the table above. If no danger level is set, DC MUST default to 16 (Sketchy).
- REQ-EXP-24: Case It MUST satisfy REQ-TR-3 and REQ-TR-4 from the `traps` feature spec. The mode ID checked by the traps system MUST be `"case_it"`.

### 3.5 Run Point

**Trigger:** Initiative roll (combat start).

While Run Point is active, all other players **in the same room** at the moment Initiative is rolled gain a +1 circumstance bonus to their Initiative rolls. "Other players in the same room" means any other connected player whose session room ID matches the Run Point player's room ID at the time Initiative is rolled — no formal party system is required. The Run Point player themselves does NOT receive this bonus.

Room co-location is evaluated at the time Initiative is rolled, not at mode-set time.

- REQ-EXP-25: Run Point MUST grant +1 circumstance bonus to Initiative for all other players in the same room at combat start.
- REQ-EXP-26: The Run Point player MUST NOT receive the bonus themselves.
- REQ-EXP-27: Room co-location MUST be evaluated at Initiative roll time, not at mode-set time.

### 3.6 Shadow

**Trigger:** Room entry (on the first skill check made by room-entry hooks).

The player nominates one other player in the same room via `explore shadow <player-name>`. While Shadow is active, the player uses the ally's **skill rank** (not modifier) for the first room-entry skill check where the ally's rank exceeds the player's rank. If the ally's rank is equal to or lower than the player's rank, the player's own rank is used instead.

"Skill rank" maps to the PF2E proficiency scale: untrained (0) < trained (1) < expert (2) < master (3) < legendary (4). Only the rank is borrowed — the player's own ability modifier and level bonus still apply.

**"Ally" definition:** Any other connected player currently in the same room. No formal party system is required.

**Suspension:** Shadow suspends silently (no effect, no error) when the nominated player is not in the same room as the shadowing player — including when the target has disconnected. Shadow resumes automatically when the target reconnects and re-enters the player's room, or re-enters from another room.

- REQ-EXP-28: Shadow MUST require a valid player name in the same room at the time of `explore shadow <player-name>`. "Valid player" means any other connected player in the same room.
- REQ-EXP-29: Shadow MUST use the ally's skill rank when the ally's rank exceeds the player's rank for the relevant check.
- REQ-EXP-30: Shadow MUST use the player's own rank when the ally's rank is equal to or lower than the player's rank.
- REQ-EXP-31: Shadow MUST suspend silently (no effect, no error) when the target player is not in the same room, including when disconnected.
- REQ-EXP-32: Shadow MUST resume automatically when the target player re-enters the player's room.

### 3.7 Poke Around

**Trigger:** Room entry.

The player makes a secret Recall Knowledge check. The skill used and the DC are determined by the room's `"context"` property (`world.Room.Properties["context"]`):

| `"context"` value | Skill | DC |
|---|---|---|
| `"history"` | Intel | 15 |
| `"faction"` | Conspiracy or Factions (higher rank; ties use Conspiracy) | 17 |
| `"technology"` | Tech Lore | 16 |
| `"creature"` | Wasteland | 14 |
| `""` or unset | Intel | 15 |

On success, one contextual lore fact is revealed to the player via a console message. On critical success, two facts are revealed. On failure, nothing is revealed. False information is never generated on any outcome.

- REQ-EXP-33: Poke Around MUST make a secret Recall Knowledge check on room entry.
- REQ-EXP-34: The skill and DC MUST be selected based on `world.Room.Properties["context"]` per the table above.
- REQ-EXP-35: Success MUST reveal one lore fact; critical success MUST reveal two facts.
- REQ-EXP-36: Failure MUST reveal nothing; false information MUST NOT be generated on any outcome.
- REQ-EXP-37: When both Conspiracy and Factions are candidates (`"context": "faction"`), the skill with the higher player rank MUST be used; ties use Conspiracy.

---

## 4. Hook Integration

Exploration mode effects integrate via two hook points on the room-entry and combat-start code paths:

**Room-entry hook** (`onRoomEntry`): Called after the player's room ID is updated and after the room description (`RoomView`) is sent to the player. Checks `sess.ExploreMode` and dispatches to the appropriate mode handler. Results are sent as separate console messages — the room view struct is never modified by exploration hooks. Lay Low, Active Sensors, Case It, Shadow, and Poke Around all fire here.

**Combat-start hook** (`onCombatStart`): Called when Initiative is rolled for a new combat, before the Initiative order is finalized. Checks `sess.ExploreMode` for all participants and dispatches. Hold Ground fires here — applying `shield_raised` when AP is first granted to the player's initiative slot. Run Point fires here — scanning all other players in the room and applying the +1 circumstance bonus before their Initiative rolls are resolved.

**Lay Low combat-start clear:** When `onCombatStart` fires, if `sess.ExploreMode == "lay_low"`, clear it to empty string before processing any other hooks.

- REQ-EXP-38: The room-entry hook MUST fire after the room description is sent to the player. Exploration hook results MUST be delivered as separate console messages; the room view struct MUST NOT be modified.
- REQ-EXP-39: The combat-start hook MUST fire before Initiative order is finalized.
- REQ-EXP-40: Lay Low MUST be cleared from `ExploreMode` when `onCombatStart` fires for the player, before other hooks are processed.

---

## 5. Architecture Notes

- `ExploreMode` and `ExploreShadowTarget` are added to `internal/game/session.PlayerSession`. They are not persisted — session-only.
- The `explore` command follows the CMD-1 through CMD-7 pattern: `HandlerExplore` constant, `BuiltinCommands()` entry, proto message in `ClientMessage` oneof, bridge handler in `bridgeHandlerMap`, `handleExplore` case in `grpc_service.go`.
- All secret checks use `internal/game/skillcheck` helpers with the roller from `internal/game/dice`. The roll result is never shown to the player — only the narrative outcome.
- Room context tags for Poke Around are read from `world.Room.Properties["context"]`. Valid values: `"history"`, `"faction"`, `"technology"`, `"creature"`. Rooms without this key default to Intel/DC 15.
- `case_it` is the mode ID the `traps` feature checks against `sess.ExploreMode` to satisfy REQ-TR-3 and REQ-TR-4.
- Extension point: the `onRoomEntry` and `onCombatStart` hook dispatchers are the integration points for all future exploration mode additions.

---

## Requirements Summary

- REQ-EXP-1: `explore <mode>` MUST set ExploreMode and confirm to the player.
- REQ-EXP-2: `explore off` MUST clear ExploreMode and confirm to the player.
- REQ-EXP-3: Setting a new mode MUST replace the old mode without requiring `explore off` first.
- REQ-EXP-4: `explore` MUST be rejected with an error message when the player is in active combat.
- REQ-EXP-5: `active_sensors` and `case_it` MUST fire their room-entry hooks immediately upon being set.
- REQ-EXP-6: `explore shadow` without a valid player name in the same room MUST fail with an error message.
- REQ-EXP-7: Lay Low MUST make a secret Ghosting check vs. the highest NPC Awareness DC in the room on room entry.
- REQ-EXP-8: Critical success MUST apply `hidden` + `undetected`; success MUST apply `hidden` only.
- REQ-EXP-8a: Failure MUST have no effect. Critical failure MUST prevent the player from gaining `hidden` or `undetected` in the current room for the duration of the visit.
- REQ-EXP-9: Lay Low MUST be cleared automatically when combat starts.
- REQ-EXP-10: If no NPCs are present, Lay Low MUST make no check and apply no conditions.
- REQ-EXP-11: Hold Ground MUST apply `shield_raised` when AP is granted for the player's first initiative slot in a new combat, at no AP cost.
- REQ-EXP-12: If no shield is equipped, Hold Ground MUST have no effect and MUST NOT produce an error.
- REQ-EXP-13: The applied `shield_raised` MUST follow normal shield rules and expire when AP is next granted to the player.
- REQ-EXP-14: Active Sensors MUST make a secret Tech Lore check on room entry and immediately on mode set.
- REQ-EXP-15: Success MUST send a console message listing detected technology. The room view struct MUST NOT be modified.
- REQ-EXP-16: Critical success MUST additionally reveal concealed technology.
- REQ-EXP-17: Failure MUST send no message and reveal nothing.
- REQ-EXP-18: Active Sensors DC MUST be determined by the room's current danger level; unset defaults to 16 (Sketchy).
- REQ-EXP-19: Case It MUST make a secret Awareness check on room entry and immediately on mode set.
- REQ-EXP-20: Success MUST reveal hidden exits, concealed items, and trap triggers via a console message. The room view struct MUST NOT be modified.
- REQ-EXP-21: Critical success MUST additionally reveal trap type and rough DC range.
- REQ-EXP-22: Case It failure MUST reveal nothing.
- REQ-EXP-23: Case It DC MUST be determined by the room's current danger level; unset defaults to 16 (Sketchy).
- REQ-EXP-24: Case It MUST satisfy REQ-TR-3 and REQ-TR-4 from the `traps` feature spec. The traps system MUST check for mode ID `"case_it"`.
- REQ-EXP-25: Run Point MUST grant +1 circumstance bonus to Initiative for all other players in the same room at combat start.
- REQ-EXP-26: The Run Point player MUST NOT receive the bonus themselves.
- REQ-EXP-27: Room co-location MUST be evaluated at Initiative roll time.
- REQ-EXP-28: Shadow MUST require a valid player name in the same room at nomination time.
- REQ-EXP-29: Shadow MUST use the ally's rank when higher than the player's rank.
- REQ-EXP-30: Shadow MUST use the player's own rank when the ally's rank is equal or lower.
- REQ-EXP-31: Shadow MUST suspend silently when the target player is not in the same room, including when disconnected.
- REQ-EXP-32: Shadow MUST resume automatically when the target player re-enters the player's room.
- REQ-EXP-33: Poke Around MUST make a secret Recall Knowledge check on room entry.
- REQ-EXP-34: The skill and DC MUST be selected based on `world.Room.Properties["context"]` per the table in section 3.7.
- REQ-EXP-35: Success MUST reveal one lore fact; critical success MUST reveal two facts.
- REQ-EXP-36: Failure MUST reveal nothing; false information MUST NOT be generated.
- REQ-EXP-37: Conspiracy vs Factions tie MUST use Conspiracy.
- REQ-EXP-38: The room-entry hook MUST fire after the room description is sent. Results MUST be console messages; the room view struct MUST NOT be modified.
- REQ-EXP-39: The combat-start hook MUST fire before Initiative order is finalized.
- REQ-EXP-40: Lay Low MUST be cleared from ExploreMode when combat starts, before other hooks are processed.
