# Resting — Design Spec

## Overview

The `rest` command is extended from its current unrestricted behavior into a location-aware system with two modes: motel rest (instant, in safe rooms with a motel NPC, costs credits) and camping rest (timed, in non-safe zones during exploration mode, requires gear). Both modes deliver the same full long-rest restoration (HP, tech pools, prepared tech rearrangement). Camping can be interrupted by enemies with partial restoration proportional to elapsed time.

## Requirements

### Command Routing

- REQ-REST-1: The `rest` command MUST be blocked when the player is in combat.
- REQ-REST-2: If the player is in a safe room containing a motel NPC, the `rest` command MUST execute a motel rest.
- REQ-REST-3: If the player is in a non-safe zone and in an exploration mode, the `rest` command MUST execute a camping rest.
- REQ-REST-4: If neither REQ-REST-2 nor REQ-REST-3 applies, the `rest` command MUST be blocked with a descriptive error message indicating why rest is unavailable.

### Motel Rest

- REQ-REST-5: Motel rest MUST be instant (no timer).
- REQ-REST-6: Motel rest MUST deduct credits from the player equal to the cost defined on the motel NPC.
- REQ-REST-7: Motel rest MUST be blocked if the player has insufficient credits, with an error message stating the cost.
- REQ-REST-8: Motel NPC cost MUST be defined in the NPC YAML and SHOULD scale with zone danger level.
- REQ-REST-9: Motel rest MUST deliver full long-rest restoration: HP restored to maximum, tech use pools reset, prepared tech rearrangement applied.

### Camping Rest

- REQ-REST-10: Camping rest MUST require a `sleeping_bag` item in the player's inventory.
- REQ-REST-11: Camping rest MUST require at least one item tagged `fire_material` in the player's inventory.
- REQ-REST-12: If required gear is missing, the `rest` command MUST be blocked with an error message identifying the missing item(s).
- REQ-REST-13: The base camping rest timer MUST be 5 minutes real-time.
- REQ-REST-14: Each item tagged `camping_gear` in the player's inventory MUST reduce the camping timer, with a minimum of 2 minutes regardless of gear count.
- REQ-REST-15: If an enemy enters the room while camping is in progress, camping MUST be cancelled and partial restoration MUST be applied proportional to elapsed time (e.g. 60% of timer elapsed → 60% of max HP restored, 60% of tech use slots restored).
- REQ-REST-16: On successful completion, camping rest MUST deliver full long-rest restoration identical to REQ-REST-9.

## Design

### Command Routing

`handleRest()` in `grpc_service.go` is extended with a pre-check sequence before invoking the existing long-rest logic:

1. Combat check (existing) — return error if in combat
2. Room type check — query the current room's danger level and NPC list
3. If safe room + motel NPC present → motel rest path
4. If non-safe zone + player in exploration mode → camping rest path
5. Otherwise → blocked

### Motel Rest

The motel NPC YAML carries a `rest_cost` field (integer credits). The handler deducts credits, emits a flavor message, then calls the existing long-rest restoration logic. No timer or goroutine is needed.

### Camping Rest

A `CampingSession` is tracked on the session (or as a short-lived goroutine) with:
- `startTime time.Time`
- `duration time.Duration` (computed from base 5 min minus gear reductions)
- `playerID`

When an enemy enters the room, the `TriggerOnEnemyEntersRoom` fire point (already wired for ready-action) fires a camping interruption check. Elapsed fraction is computed and applied to HP and tech pool restoration before cancelling the session.

On natural completion, the camping goroutine calls the existing long-rest restoration logic.

### Item Tags

- `sleeping_bag` — exact item ID match required
- `fire_material` — item tag; at least one required
- `camping_gear` — item tag; each reduces timer by a fixed decrement (total capped at minimum 2 min)

### Dependencies

- `room-danger-levels` — required to distinguish safe vs non-safe rooms
- `exploration` — required to check whether player is in an exploration mode
- `non-combat-npcs` — required for motel NPC type; motel rest path is unavailable until this feature is implemented

## Out of Scope

- Camping in safe rooms is not permitted.
- Motel rest in non-safe zones is not permitted.
- Partial motel rest (leaving early) is not included.
- Multiplayer shared camping (party members sharing a campfire) is not included.
- Camping gear item definitions and motel NPC YAML content are not part of this spec; they are content work belonging to equipment-mechanics and non-combat-npcs respectively.
