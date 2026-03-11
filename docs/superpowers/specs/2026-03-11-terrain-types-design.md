# Terrain Types Design

**Date:** 2026-03-11
**Feature:** Climbable Surfaces (Climb command) + Water Terrain (Swim command)

---

## Requirements

- TERRAIN-1: A room with `properties.climbable == "true"` MUST accept the `climb` command.
- TERRAIN-2: A room with `properties.water_terrain == "true"` MUST accept the `swim` command.
- TERRAIN-3: The `climb_dc` and `water_dc` properties MUST default to `15` and `12` respectively when absent.
- TERRAIN-4: All skill checks for Climb and Swim MUST use `d20 + skillRankBonus(sess.Skills["athletics"])` vs the room DC.
- TERRAIN-5: Outcome thresholds MUST follow PF2E 4-tier rules as implemented in `combat.OutcomeFor`: CritSuccess when total >= dc+10; Success when dc <= total < dc+10; Failure when dc-10 <= total < dc; CritFailure when total < dc-10.
- TERRAIN-6: Climb MUST cost 2 AP in combat; the AP cost is irrelevant out of combat.
- TERRAIN-7: Swim MUST cost 2 AP in combat; the AP cost is irrelevant out of combat.
- TERRAIN-8: Climb CritFailure MUST deal 1d6 falling damage (minimum 1) to the player.
- TERRAIN-9: Climb CritFailure MUST apply the `prone` condition to the player via `sess.Conditions.Apply("prone")` only when a combat session is active; the prone condition MUST already exist in `content/conditions/prone.yaml`.
- TERRAIN-10: Swim CritFailure MUST deal 1d6 drowning damage to the player.
- TERRAIN-11: Swim CritFailure MUST apply the `submerged` condition to the player via `sess.Conditions.Apply("submerged")`.
- TERRAIN-12: While the player has the `submerged` condition, the commands `attack`, `burst`, `auto`, and `reload` MUST be blocked at the gRPC dispatch level with a descriptive error message; this enforcement MUST be added as guard logic inside each blocked command handler (not via `RestrictActions` YAML field, which has no runtime enforcement).
- TERRAIN-13: While the player has the `submerged` condition, each round at turn-start the player MUST receive 1d6 automatic drowning damage; this MUST be applied at the existing `onTurnStart` hook point in `CombatHandler` (a new hook if not present).
- TERRAIN-14: Out-of-combat, drowning damage (1d6) MUST be applied on every gRPC dispatch while the player has the `submerged` condition.
- TERRAIN-15: The `submerged` condition MUST be cleared by a successful Swim check OR a successful Escape check (Athletics vs `water_dc`); no other mechanism clears it.
- TERRAIN-16: The `swim` command MUST be accepted when the player has the `submerged` condition, regardless of whether the current room has `water_terrain`.
- TERRAIN-17: Climb MUST use the first `up` or `down` exit in `room.Exits` declaration order; if both exist, `up` is preferred; if neither exists the command MUST fail with a descriptive error.
- TERRAIN-18: Climb CritSuccess MUST move the player via the vertical exit at no additional cost; there is no mechanical bonus beyond movement (narrative flavor only).
- TERRAIN-19: All new commands MUST complete all CMD-1 through CMD-7 pipeline steps.
- TERRAIN-20: All tests MUST pass via `mise run go test ./...` before any step is marked done (SWENG-6, SWENG-6A).

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

A new condition file `content/conditions/submerged.yaml` is added. It MUST include all required fields for `ConditionDef`:

```yaml
id: submerged
name: Submerged
description: "You are pulled under the water. You cannot attack or reload. Swim or Escape to surface."
duration_type: permanent
max_stacks: 1
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
damage_bonus: 0
reflex_bonus: 0
stealth_bonus: 0
restrict_actions: []    # restriction enforced at dispatch, not via this field
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

> Note: `restrict_actions` in the YAML has no runtime enforcement. Blocking MUST be done as explicit guard logic in each affected command handler (TERRAIN-12).

---

## 2. Climb Command (CMD-1 through CMD-7)

- **CMD-1:** `HandlerClimb` constant in `internal/game/command/commands.go`.
- **CMD-2:** `Command{Handler: HandlerClimb, ...}` entry in `BuiltinCommands()`.
- **CMD-3:** `HandleClimb` function in `internal/game/command/climb.go` with full property-based TDD coverage.
- **CMD-4:** `ClimbRequest` proto message added to `api/proto/game/v1/game.proto`; added to `ClientMessage` oneof; `make proto` run.
- **CMD-5:** `bridgeClimb` added to `internal/frontend/handlers/bridge_handlers.go` and registered in `bridgeHandlerMap`; `TestAllCommandHandlersAreWired` MUST pass.
- **CMD-6:** `handleClimb` implemented in `internal/gameserver/grpc_service.go` and wired into the `dispatch` type switch.
- **CMD-7:** All steps complete; all tests pass.

**Skill check:** `d20 + skillRankBonus(sess.Skills["athletics"])` vs `climb_dc` (default 15; TERRAIN-5 thresholds).

**4-tier outcomes:**

| Outcome | Effect |
|---|---|
| Critical Success | Player moves via vertical exit (TERRAIN-17); no additional cost |
| Success | Player moves via vertical exit |
| Failure | No movement; informational message |
| Critical Failure | 1d6 falling damage (TERRAIN-8) + apply `prone` if in combat (TERRAIN-9) |

---

## 3. Swim Command (CMD-1 through CMD-7)

- **CMD-1:** `HandlerSwim` constant in `internal/game/command/commands.go`.
- **CMD-2:** `Command{Handler: HandlerSwim, ...}` entry in `BuiltinCommands()`.
- **CMD-3:** `HandleSwim` function in `internal/game/command/swim.go` with full property-based TDD coverage.
- **CMD-4:** `SwimRequest` proto message added to `api/proto/game/v1/game.proto`; added to `ClientMessage` oneof; `make proto` run.
- **CMD-5:** `bridgeSwim` added to `internal/frontend/handlers/bridge_handlers.go` and registered in `bridgeHandlerMap`; `TestAllCommandHandlersAreWired` MUST pass.
- **CMD-6:** `handleSwim` implemented in `internal/gameserver/grpc_service.go` and wired into the `dispatch` type switch.
- **CMD-7:** All steps complete; all tests pass.

**Skill check:** `d20 + skillRankBonus(sess.Skills["athletics"])` vs `water_dc` (default 12; TERRAIN-5 thresholds).

**4-tier outcomes:**

| Outcome | Effect |
|---|---|
| Critical Success | Player moves to water exit or surfaces (clears `submerged`) |
| Success | Player moves / surfaces (clears `submerged`) |
| Failure | No movement; informational message |
| Critical Failure | 1d6 drowning damage + apply `submerged` (TERRAIN-10, TERRAIN-11) |

### Submerged Condition Enforcement

- **In combat (TERRAIN-13):** At turn-start (in `CombatHandler`), if player has `submerged`, apply 1d6 drowning damage before the player's action.
- **Out of combat (TERRAIN-14):** On every gRPC dispatch, if player has `submerged`, apply 1d6 drowning damage.
- **Blocked commands (TERRAIN-12):** `attack`, `burst`, `auto`, `reload` handlers MUST check `sess.Conditions.Has("submerged")` and return an error if true.
- **Clear condition (TERRAIN-15):** On Swim Success or CritSuccess, call `sess.Conditions.Remove("submerged")`. Escape command likewise clears it on success.

---

## 4. Testing

All tests MUST be in the correct packages. All property-based tests MUST use `pgregory.net/rapid`.

### `internal/game/command/` package

- `TestClimbOutcomes` (property-based) — random DC and roll values; assert correct 4-tier outcome per TERRAIN-5; assert falling damage >= 1 on CritFail; assert prone applied iff `inCombat` flag is set.
- `TestSwimOutcomes` (property-based) — random DC and rolls; assert correct outcome; assert `submerged` applied on CritFail; assert 1d6 damage applied.
- `TestSubmergedBlocksAttack` — assert that each of `attack`, `burst`, `auto`, `reload` returns error when player has `submerged`.

### `internal/gameserver/` package

- `TestHandleClimb_RoomNotClimbable` — error when `climbable` absent from properties.
- `TestHandleClimb_NoVerticalExit` — error when room is climbable but has no `up`/`down` exit.
- `TestHandleClimb_Success` / `TestHandleClimb_Failure` / `TestHandleClimb_CritFailure`
- `TestHandleSwim_RoomNotWater_NotSubmerged` — error when neither condition is met.
- `TestHandleSwim_Success` / `TestHandleSwim_Failure` / `TestHandleSwim_CritFailure`
- `TestHandleSwim_SubmergedSurface` — player with `submerged` successfully swims to surface; condition cleared.
- All gRPC tests MUST include `submerged` and `prone` in the test condition registry (extend `makeTestConditionRegistry` or equivalent).

### `internal/game/condition/` package

- `TestSubmergedConditionLoads` — assert `submerged.yaml` parses without error and `id == "submerged"`, `MaxStacks == 1`, `DurationType == "permanent"`.

**Completion criterion (TERRAIN-20):** `mise run go test ./...` MUST pass with 0 failures before any step is considered done.

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
