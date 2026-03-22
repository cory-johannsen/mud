# Resting — Design Spec

## Overview

The `rest` command is extended from its current unrestricted behavior into a location-aware system with two modes: motel rest (instant, in safe rooms with a motel NPC, costs credits) and camping rest (timed, in non-safe zones during exploration mode, requires gear). Both modes deliver the same full long-rest restoration (HP, tech pools, prepared tech rearrangement). Camping can be interrupted by enemies or voluntary departure with partial restoration proportional to elapsed time.

## Requirements

### Command Routing

- REQ-REST-1: The `rest` command MUST be blocked when the player is in combat, with an error message stating that resting during combat is not permitted.
- REQ-REST-2: If the player is in a safe room containing a motel NPC, the `rest` command MUST execute a motel rest.
- REQ-REST-3: If REQ-REST-2 does not apply, and the player is in a non-safe zone and has an active exploration mode (as defined by the exploration feature), the `rest` command MUST execute a camping rest.
- REQ-REST-4: If neither REQ-REST-2 nor REQ-REST-3 applies, the `rest` command MUST be blocked. The error message MUST indicate the specific reason: no motel available if in a safe room without a motel NPC, or not in exploration mode if in a non-safe zone without an active exploration mode.

### Motel Rest

- REQ-REST-5: Motel rest MUST be instant (no timer).
- REQ-REST-6: Motel rest MUST deduct credits from the player equal to the cost defined on the motel NPC.
- REQ-REST-7: Motel rest MUST be blocked if the player has insufficient credits, with an error message stating the required cost.
- REQ-REST-8: Motel NPC cost MUST be defined as an integer credit value in the NPC YAML.
- REQ-REST-9: Motel rest MUST deliver full long-rest restoration: HP restored to maximum, tech use pools reset to maximum, prepared tech rearrangement applied.

### Camping Rest

- REQ-REST-10: Camping rest MUST require a `sleeping_bag` item (matched by exact item ID) in the player's inventory.
- REQ-REST-11: Camping rest MUST require at least one item tagged `fire_material` in the player's inventory.
- REQ-REST-12: If required gear is missing, the `rest` command MUST be blocked with an error message identifying each missing item or tag.
- REQ-REST-13: The base camping rest timer MUST be 5 minutes real-time.
- REQ-REST-14: Each item tagged `camping_gear` in the player's inventory MUST reduce the camping timer by 30 seconds, to a minimum of 2 minutes regardless of gear count.
- REQ-REST-15: If an enemy enters the room while camping is in progress, camping MUST be cancelled and partial restoration MUST be applied: `floor(elapsed / duration * maxHP)` HP restored, `floor(elapsed / duration * maxPool)` use slots restored per tech pool. Fractional values MUST be truncated (floor). Gear is NOT consumed.
- REQ-REST-16: If the player voluntarily leaves the room while camping is in progress, camping MUST be cancelled and the same partial restoration formula from REQ-REST-15 MUST be applied.
- REQ-REST-17: Gear items (sleeping_bag, fire_material, camping_gear) MUST NOT be consumed on camping rest completion or cancellation.
- REQ-REST-18: On successful completion, camping rest MUST deliver full long-rest restoration identical to REQ-REST-9.

## Design

### Command Routing

`handleRest()` in `grpc_service.go` is extended with a pre-check sequence before invoking the existing long-rest logic:

1. Combat check (existing) — return error if in combat
2. If safe room + motel NPC present → motel rest path (REQ-REST-2 takes precedence)
3. If non-safe zone + active exploration mode → camping rest path
4. Otherwise → blocked with reason-specific error

### Motel Rest

The motel NPC YAML carries a `rest_cost` field (integer credits). The handler deducts credits, emits a flavor message, then calls the existing long-rest restoration logic. No timer is needed.

### Camping Rest

A camping session tracks start time and computed duration (base 5 min minus 30 s per `camping_gear` item, minimum 2 min). When an enemy enters the room or the player moves, the session is interrupted: elapsed fraction is computed and applied to HP and tech pool restoration using floor division before cancelling.

On natural completion, the existing long-rest restoration logic is invoked in full.

### Item Tags

- `sleeping_bag` — exact item ID match required
- `fire_material` — item tag; at least one required to start camping
- `camping_gear` — item tag; each reduces timer by 30 seconds (minimum 2 minutes total)

### Dependencies

- `room-danger-levels` — required to distinguish safe vs non-safe rooms
- `exploration` — required to check whether player has an active exploration mode
- `non-combat-npcs` — required for motel NPC type; motel rest path is unavailable until this feature is implemented

## Out of Scope

- Camping in safe rooms is not permitted.
- Motel rest in non-safe zones is not permitted.
- Partial motel rest (leaving early) is not included.
- Multiplayer shared camping (party members sharing a campfire) is not included.
- Motel cost scaling by zone danger level is a content concern; the spec requires only that cost is defined in NPC YAML.
- Camping gear item definitions and motel NPC YAML content are content work belonging to equipment-mechanics and non-combat-npcs respectively.
