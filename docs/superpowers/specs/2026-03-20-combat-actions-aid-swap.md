# Combat Actions: Aid and Swap — Design Spec

**Date:** 2026-03-20

---

## Goal

Add **Aid** (boost an ally's attack roll via a DC 20 skill check) and enforce an **AP cost on Swap** (weapon preset swap costs 1 AP when in combat).

Note: Delay is already implemented (`handleDelay`, `BankedAP` injection, `SpendAllAP`, `-2 AC` penalty).

Note: Aid in PF2E is implemented here as a simplified single-roll model (2 AP, DC 20 check, immediate outcome, condition applied to ally for their next attack) rather than the two-phase Prepare+Reaction model described in the feature doc. This design is intentional: the Reaction system is not yet built.

---

## Scope

**In scope:**
- REQ-ACT1: `aid <ally>` command: `ActionAid` type, `CombatHandler.Aid`, `handleAid`, `HandlerAid`, proto `AidRequest`, three condition YAML files, `ResolveRound` roll resolution, `attack_bonus` field on `ConditionDef`.
- REQ-ACT2: Swap AP enforcement: `handleLoadout` MUST deduct 1 AP when player is in combat.

**Out of scope:**
- REQ-ACT3: NPC use of Aid is out of scope.
- REQ-ACT4: Aiding outside combat is out of scope — Aid is a combat-only action.
- REQ-ACT5: General armor/accessory slot swapping is out of scope — Swap means weapon preset swap only.

---

## Condition Engine Extension

### REQ-ACT0a

`ConditionDef` in `internal/game/condition/definition.go` MUST gain a new field:
```go
AttackBonus int `yaml:"attack_bonus"` // positive = bonus to attack rolls
```

A unit test in `internal/game/condition/definition_test.go` MUST verify that `attack_bonus: 3` round-trips through YAML decode into `ConditionDef.AttackBonus == 3`.

### REQ-ACT0b

`AttackBonus(s *ActiveSet) int` in `internal/game/condition/modifiers.go` MUST:
- Guard `if s == nil { return 0 }` (matching the nil-guard pattern in `APReduction`, `SkipTurn`, etc.).
- Also add `ac.Def.AttackBonus * ac.Stacks` to the total for each active condition (in addition to the existing subtraction of `AttackPenalty`).
- Update its postcondition comment from `Returns <= 0` to `Returns the net modifier; may be positive when attack bonuses are active`.
- Update its precondition comment to `Precondition: s may be nil` (was implicitly non-nil).

### REQ-ACT0c

Convention: positive `attack_penalty` = penalty (decreases attack roll). Positive `attack_bonus` = bonus (increases attack roll). Negative values of either field MUST NOT be used in condition YAML files.

---

## Aid

### REQ-ACT6

`ActionAid` MUST be added to the `ActionType` iota in `internal/game/combat/action.go`. Its `Cost()` MUST return `2`. Its `String()` MUST return `"aid"`, and the `String()` postcondition comment MUST be updated to enumerate `"aid"` among the valid return values.

### REQ-ACT7

`QueuedAction.Target string` MUST carry the ally name for `ActionAid`. No new struct fields are required.

### REQ-ACT8

`CombatHandler.Aid(uid, allyName string) ([]*gamev1.CombatEvent, error)` MUST validate:
- REQ-ACT8a: The actor is in active combat (player session found and combat exists in their room).
- REQ-ACT8b: `allyName` is non-empty; return a descriptive error if empty.
- REQ-ACT8c: `allyName` matches (case-insensitive) the `CharName` of a living (HP > 0) player combatant registered in the same combat. Ally lookup iterates the combat's player sessions and matches on `CharName`. The case where the named player is in the room but not registered as a combatant is treated identically to "ally not found in combat" and MUST return the same error.
- REQ-ACT8d: `allyName` does not match the actor's own `CharName` (case-insensitive) and does not equal the actor's `uid`; return a descriptive error if it does.

`CombatHandler.Aid` MUST return a descriptive error if any precondition fails. The 2 AP cost is enforced by `cbt.QueueAction` (REQ-ACT9).

### REQ-ACT9

`CombatHandler.Aid` MUST enqueue the action via `cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAid, Target: allyName})`, following the same pattern as `CombatHandler.Strike`. `QueueAction` atomically deducts 2 AP (via `ActionAid.Cost()`) and appends the action; it returns an error if AP is insufficient. `CombatHandler.Aid` MUST propagate that error to its caller.

On success, `CombatHandler.Aid` MUST return a confirmation `CombatEvent` of type `COMBAT_EVENT_TYPE_CONDITION` with `Narrative` containing the actor name and ally name (e.g., `"Alice prepares to aid Bob."`). It MUST also check `cbt.AllActionsSubmitted()` and invoke `h.resolveAndAdvanceLocked` if all actions are in, following the `Strike` pattern.

### REQ-ACT10

`ResolveRound` in `internal/game/combat/round.go` MUST resolve `ActionAid` by rolling `d20 + max(actor.GritMod, actor.QuicknessMod, actor.SavvyMod)` vs DC 20. The fields `GritMod`, `QuicknessMod`, and `SavvyMod` exist on `combat.Combatant` (verified in `internal/game/combat/combat.go:87-93`). The roll MUST use the existing dice roller already available in `ResolveRound`. The condition applied to the target combatant depends on the total:

- REQ-ACT10a: Total ≥ 30 (Critical Success): apply condition `aided_strong` (duration 1 round) to target.
- REQ-ACT10b: Total 20–29 (Success): apply condition `aided` (duration 1 round) to target.
- REQ-ACT10c: Total 10–19 (Failure): no condition applied; emit narrative only.
- REQ-ACT10d: Total ≤ 9 (Critical Failure): apply condition `aided_penalty` (duration 1 round) to target.

The DC 20 banding logic MUST be extracted into an unexported helper function `aidOutcome(total int) string` (returns `"critical_success"`, `"success"`, `"failure"`, `"critical_failure"`) within `round.go` to allow the property-based test (REQ-ACT23) to exercise it directly.

If the ally named in `QueuedAction.Target` is not found or is dead at resolution time (e.g., killed earlier in the same round), `ResolveRound` MUST emit a narrative event such as `"<actor> attempts to aid <target>, but <target> is already down."` and skip condition application.

### REQ-ACT11

`ResolveRound` MUST emit a `RoundEvent` for each Aid outcome. Narratives MUST include the actor name and target name, and:

- REQ-ACT11a: Critical Success: MUST contain `"critical aid"`.
- REQ-ACT11b: Success: MUST contain `"aids"`.
- REQ-ACT11c: Failure: MUST contain `"fails to aid"`.
- REQ-ACT11d: Critical Failure: MUST contain `"fumbles the aid"`.

### REQ-ACT12

`handleAid` in `internal/gameserver/grpc_service.go` MUST:
- REQ-ACT12a: Return an informational `messageEvent` (not an error) when the player is not in combat; the message MUST contain the substring `"only valid in combat"`.
- REQ-ACT12b: Return an informational `messageEvent` (not an error) when `req.GetAid().GetTarget()` is empty; the message MUST contain `"specify an ally name"`.

### REQ-ACT13

`HandlerAid` constant in `internal/game/command/commands.go` MUST equal `"aid"`. The `BuiltinCommands()` entry MUST use `Category: CategoryCombat` and `Help: "Aid an ally (DC 20 check; crit +3, success +2, fail 0, crit fail -1 to ally attack). Costs 2 AP."`.

### REQ-ACT14

`bridge_handlers.go` in `internal/frontend/handlers` MUST map `command.HandlerAid` to a handler that parses the first whitespace-delimited token as the ally name (empty string if no token present) and emits `ClientMessage_Aid{Aid: &gamev1.AidRequest{Target: allyName}}`.

### REQ-ACT15

`api/proto/game/v1/game.proto` MUST define:

```proto
message AidRequest {
  string target = 1;
}
```

`ClientMessage.payload` MUST include `AidRequest aid = <next_field_number>;`. The generated `game.pb.go` MUST be regenerated via `make proto` and committed.

---

## Conditions

`max_stacks: 0` means non-stacking (maximum 1 stack) per the existing condition engine convention (consistent with `prone`, `frightened` at `max_stacks: 0`).

### REQ-ACT16

`content/conditions/aided_strong.yaml` MUST define a condition with:
- `id: aided_strong`
- `name: Aided (Strong)`
- `description`: states +3 to attack rolls for 1 round
- `duration_type: rounds`
- `max_stacks: 0` (non-stacking)
- `attack_bonus: 3` (positive = bonus per REQ-ACT0c)
- `attack_penalty: 0`, `ac_penalty: 0`, `damage_bonus: 0`, `speed_penalty: 0`
- All Lua hook fields empty strings

### REQ-ACT17

`content/conditions/aided.yaml` MUST define a condition with:
- `id: aided`
- `name: Aided`
- `description`: states +2 to attack rolls for 1 round
- `duration_type: rounds`
- `max_stacks: 0`
- `attack_bonus: 2`
- `attack_penalty: 0`, `ac_penalty: 0`, `damage_bonus: 0`, `speed_penalty: 0`
- All Lua hook fields empty strings

### REQ-ACT18

`content/conditions/aided_penalty.yaml` MUST define a condition with:
- `id: aided_penalty`
- `name: Aided (Fumble)`
- `description`: states -1 to attack rolls for 1 round (misguided assistance)
- `duration_type: rounds`
- `max_stacks: 0`
- `attack_penalty: 1` (positive = penalty per REQ-ACT0c)
- `attack_bonus: 0`, `ac_penalty: 0`, `damage_bonus: 0`, `speed_penalty: 0`
- All Lua hook fields empty strings

---

## Swap (AP Cost Enforcement)

### REQ-ACT19

`handleLoadout` in `internal/gameserver/grpc_service.go` MUST, when `sess.Status == statusInCombat` and `req.GetArg()` is non-empty, call `s.combatH.SpendAP(uid, 1)` before invoking `command.HandleLoadout`.

### REQ-ACT20

If `SpendAP` returns an error in the in-combat swap path, `handleLoadout` MUST return `messageEvent("Not enough AP to swap loadouts.")` without executing the swap.

### REQ-ACT21

Out-of-combat loadout swaps MUST remain free — `handleLoadout` MUST NOT check or deduct AP when `sess.Status != statusInCombat`.

---

## Testing

### REQ-ACT22

Unit tests in `internal/gameserver/combat_handler_aid_test.go` MUST verify `CombatHandler.Aid` rejects: empty ally name, self-targeting (by CharName), dead ally (HP == 0), ally not registered in the same combat (covers both "different room" and "in room but not a combatant"), and insufficient AP (< 2).

### REQ-ACT23

A property-based test using `pgregory.net/rapid` MUST call `aidOutcome(total int)` directly (the unexported helper from REQ-ACT10) and assert the correct string is returned for all roll totals sampled across the four outcome bands: ≤9, 10–19, 20–29, ≥30.

### REQ-ACT24

A unit test MUST verify `handleAid` returns a message containing `"only valid in combat"` when the player is not in combat, and returns no error.

### REQ-ACT25

A unit test MUST verify `handleLoadout` in combat with 0 remaining AP returns `"Not enough AP to swap loadouts."` and does not alter the active preset.

### REQ-ACT26

A unit test MUST verify `handleLoadout` in combat with ≥ 1 AP succeeds, deducts exactly 1 AP, and updates the active preset.

### REQ-ACT27

A unit test MUST verify `handleLoadout` out of combat succeeds without any AP check.

### REQ-ACT28

The dispatch coverage test (bridge handler dispatch test) MUST pass after `HandlerAid` is added — `bridgeAid` MUST be registered in `BridgeHandlers()`.

### REQ-ACT29

`go test ./internal/gameserver/... ./internal/game/combat/... ./internal/game/condition/... ./internal/frontend/handlers/... ./internal/game/command/...` MUST pass at 100% after all changes.

---

## File Map

| File | Change |
|------|--------|
| `internal/game/condition/definition.go` | Add `AttackBonus int` field |
| `internal/game/condition/definition_test.go` | Add YAML round-trip test for `attack_bonus` |
| `internal/game/condition/modifiers.go` | Update `AttackBonus()`: nil guard, add `attack_bonus` accumulation, update postcondition comment |
| `internal/game/combat/action.go` | Add `ActionAid`; update `Cost()` and `String()` |
| `internal/game/combat/round.go` | Add `aidOutcome` helper and `ActionAid` case in `ResolveRound` |
| `internal/gameserver/combat_handler.go` | Add `CombatHandler.Aid` method |
| `internal/gameserver/grpc_service.go` | Add `handleAid`; patch `handleLoadout` with in-combat AP gate; add `Aid` to dispatch switch |
| `internal/game/command/commands.go` | Add `HandlerAid` constant and `aid` entry in `BuiltinCommands()` |
| `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeAid`; map `HandlerAid` |
| `api/proto/game/v1/game.proto` | Add `AidRequest` message and `ClientMessage.aid` oneof field |
| `internal/gameserver/gamev1/game.pb.go` | Regenerated via `make proto` |
| `content/conditions/aided_strong.yaml` | New file |
| `content/conditions/aided.yaml` | New file |
| `content/conditions/aided_penalty.yaml` | New file |
| `internal/gameserver/combat_handler_aid_test.go` | New — `CombatHandler.Aid` unit tests (REQ-ACT22) |
| `internal/gameserver/grpc_service_aid_test.go` | New — `handleAid` handler test (REQ-ACT24) + PBT for `aidOutcome` (REQ-ACT23) |
| `internal/gameserver/grpc_service_loadout_test.go` | New or updated — Swap AP enforcement tests |
