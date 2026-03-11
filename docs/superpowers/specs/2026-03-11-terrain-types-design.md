# Terrain Types Design

**Date:** 2026-03-11
**Feature:** Climbable Surfaces (Climb command) + Water Terrain (Swim command)

---

## 1. Data Model

Terrain is declared in the existing `Room.Properties map[string]string`. No struct changes are required.

**Climbable rooms:**
```yaml
properties:
  climbable: "true"
  climb_dc: "15"   # optional; default 15
```

**Water rooms:**
```yaml
properties:
  water_terrain: "true"
  water_dc: "12"   # optional; default 12
```

A new condition file `content/conditions/submerged.yaml` is added with:
- `restrict_actions: [attack, burst, auto, reload]`
- No AC or attack-roll penalties
- Cleared only by a successful Swim or Escape action

---

## 2. Climb Command

**CMD-1–7 pipeline:** `HandlerClimb` constant → `BuiltinCommands()` entry → `HandleClimb` in `internal/game/command/climb.go` → `ClimbRequest` proto → `bridgeClimb` in bridge_handlers → `handleClimb` in grpc_service.

**Precondition:** The current room must have `properties.climbable == "true"`. If not, respond with an error message.

**Skill check:** `d20 + athleticsBonus(sess)` vs `climb_dc` (parsed from Properties; default 15).

**4-tier outcomes:**

| Outcome | Effect |
|---|---|
| Critical Success | Player moves via the `up`/`down` exit; +5 ft bonus movement narrative |
| Success | Player moves via the `up`/`down` exit |
| Failure | No movement; informational message |
| Critical Failure | Falling damage (`1d6`, minimum 1) + apply `prone` condition if in combat |

**AP cost (combat):** 2 AP.

**Climb direction:** The first exit with direction `up` or `down` in the room. If `climbable: "true"` but no vertical exit exists, the command fails with a descriptive error.

**Out-of-combat:** Prone condition is not applied outside combat. Falling damage still applies.

---

## 3. Swim Command

**CMD-1–7 pipeline:** `HandlerSwim` constant → `BuiltinCommands()` entry → `HandleSwim` in `internal/game/command/swim.go` → `SwimRequest` proto → `bridgeSwim` in bridge_handlers → `handleSwim` in grpc_service.

**Precondition:** The current room must have `properties.water_terrain == "true"`, OR the player must have the `submerged` condition. If neither, respond with an error message.

**Skill check:** `d20 + athleticsBonus(sess)` vs `water_dc` (parsed from Properties; default 12).

**4-tier outcomes:**

| Outcome | Effect |
|---|---|
| Critical Success | Player moves to water exit (or surfaces if submerged); no penalty |
| Success | Player moves / surfaces |
| Failure | No movement; informational message |
| Critical Failure | `1d6` drowning damage + apply `submerged` condition |

**AP cost (combat):** 2 AP.

### Submerged Condition

- All attack actions restricted: `attack`, `burst`, `auto`, `reload`.
- Each round the player starts their turn submerged: automatic `1d6` drowning damage (applied before the player's action).
- **Exit:** successful Swim check OR successful Escape check (Athletics vs `water_dc`).
- Cleared only by Swim/Escape success; no other mechanism removes it.
- Out-of-combat: drowning damage applied each time the player takes any action while submerged.

---

## 4. Testing

### Property-Based Tests (pgregory.net/rapid)

- `TestClimbOutcomes` — random DC and roll values → assert correct 4-tier outcome; assert falling damage ≥ 1 on CritFail; assert prone applied iff in-combat flag is set.
- `TestSwimOutcomes` — random DC and rolls → assert correct outcome; assert submerged applied on CritFail; assert drowning damage applied each round while submerged.
- `TestSubmergedCondition` — assert all restricted actions are blocked; assert Swim/Escape clears it; assert other actions do not clear it.

### gRPC Service Tests

- `TestHandleClimb_RoomNotClimbable` — error response when properties lack `climbable`.
- `TestHandleClimb_Success` / `TestHandleClimb_Failure` / `TestHandleClimb_CritFailure`
- `TestHandleSwim_RoomNotWater` — error response when properties lack `water_terrain` and player is not submerged.
- `TestHandleSwim_Success` / `TestHandleSwim_Failure` / `TestHandleSwim_CritFailure`
- `TestHandleSwim_SubmergedSurface` — player with `submerged` condition successfully swims to surface.

### Condition YAML Test

- `TestSubmergedConditionLoads` — assert the YAML parses correctly and `restrict_actions` contains the expected values.

**Reference patterns:** `grpc_service_grapple_test.go`, `grapple_test.go`, `grpc_service_trip_test.go`.

---

## 5. Zone YAML Examples

```yaml
# Climbable wall room
- id: room_cliff_base
  title: "Base of the Cliff"
  description: "A sheer rock face rises before you."
  properties:
    climbable: "true"
    climb_dc: "18"
  exits:
    - direction: up
      target_room: room_cliff_top

# Water room
- id: room_flooded_corridor
  title: "Flooded Corridor"
  description: "Murky water fills the passage to your knees—and deeper ahead."
  properties:
    water_terrain: "true"
    water_dc: "12"
  exits:
    - direction: east
      target_room: room_dry_chamber
```
