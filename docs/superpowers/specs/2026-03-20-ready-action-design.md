# Ready Action — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `ready-action` (priority 242)
**Dependencies:** `actions`, `consumable-traps`

---

## Overview

The Ready action costs 2 AP and stores a (trigger, readied action) pair on the player's session. During round resolution, the combat engine evaluates trigger conditions after each combat event. When a trigger fires, the readied action executes automatically as a Reaction (outside normal AP), targeting the triggering entity. The player's Reaction is consumed. If the trigger is not met by end of round, the readied action expires with no AP refund.

---

## 1. Data Model

`PlayerSession` gains:

```go
ReadiedTrigger string  // "enemy_enters" | "enemy_attacks_me" | "ally_attacked"; "" = no readied action
ReadiedAction  string  // "strike" | "step" | "raise_shield"
```

`ReactionUsed bool` — already tracked on `PlayerSession` (from the actions feature). The readied action execution sets `ReactionUsed = true`.

- REQ-READY-1: `ReadiedTrigger` and `ReadiedAction` MUST be cleared at end of every round, whether or not the trigger fired.

---

## 2. Command

```
ready <action> when <trigger>
```

### 2.1 Action Aliases

| Alias | Readied Action |
|---|---|
| `strike` | Strike |
| `step` | Step |
| `shield` | Raise Shield |

### 2.2 Trigger Aliases

| Alias | Trigger ID | Description |
|---|---|---|
| `enters` | `enemy_enters` | An enemy moves into the current room |
| `attacks` | `enemy_attacks_me` | An enemy declares an attack against this player |
| `ally` | `ally_attacked` | Any attack is declared against any player in the same room |

### 2.3 Validation

- REQ-READY-2: `ready <action> when <trigger>` MUST fail if the player is not in combat.
- REQ-READY-3: `ready <action> when <trigger>` MUST fail if the player has fewer than 2 AP remaining this round.
- REQ-READY-4: `ready <action> when <trigger>` MUST fail if the player already has a readied action this round.
- REQ-READY-5: `ready <action> when <trigger>` MUST fail if `ReactionUsed == true` (Reaction already spent).
- REQ-READY-6: `ready <action> when <trigger>` MUST fail if the action or trigger alias is unrecognized.

On success: deduct 2 AP, set `ReadiedTrigger` and `ReadiedAction`, notify player "You ready a <action name>. Waiting for: <trigger description>."

---

## 3. Trigger Evaluation

Trigger evaluation runs inside the round resolution engine after each of these events:

| Event | Triggers evaluated |
|---|---|
| Enemy move resolves (room entry) | `enemy_enters` for all players in destination room |
| Attack declaration (before damage) | `enemy_attacks_me` for the attack target; `ally_attacked` for all other players in same room |

Evaluation order: triggers are evaluated in event order within the round. If multiple players have readied actions with the same trigger, all fire (each consuming their own Reaction).

- REQ-READY-7: Trigger evaluation MUST occur after each qualifying combat event within the round resolution engine, not at end-of-round batch.
- REQ-READY-8: `enemy_enters` MUST integrate with the room-entry event already tracked by the consumable-traps feature's floor-state system. The same room-entry event MUST fire both trap triggers and ready-action triggers.

### 3.1 Trigger: `enemy_enters`

Fires when an enemy's movement action results in entering the current room. "Enemy" = any non-player combatant in the encounter. The triggering entity is the entering enemy.

### 3.2 Trigger: `enemy_attacks_me`

Fires when an enemy declares an attack targeting this player, after the attack is committed but before damage resolves. The triggering entity is the attacking enemy. The readied action fires before damage from the triggering attack is applied.

- REQ-READY-9: `enemy_attacks_me` uses `TriggerOnDamageTaken`, which fires after damage is calculated but before it is applied. Readied Raise Shield subtracts shield hardness from pending damage. Readied Strike executes against the attacker at this point.

### 3.3 Trigger: `ally_attacked`

Fires when any attack is declared against any other player (not this player) in the same room. The triggering entity is the attacking enemy.

---

## 4. Readied Action Execution

When a trigger fires:

1. Check `ReactionUsed`. If true: skip (Reaction already spent), clear readied action silently.
2. Execute the readied action targeting the triggering entity:
   - **Strike**: resolve a Strike against the triggering entity using the player's equipped weapon (standard attack roll + damage). Does not cost AP.
   - **Step**: the player takes a Step action (standard Step resolution). No target required; triggering entity is ignored.
   - **Raise Shield**: the player raises their shield (standard Raise Shield resolution). No target required.
3. Set `ReactionUsed = true`.
4. Clear `ReadiedTrigger` and `ReadiedAction`.
5. Notify player: "Your readied <action name> fires!" and display the action result.
6. Notify room: "<player name> reacts with a <action name>!"

- REQ-READY-10: Readied action execution MUST use the same resolution logic as the corresponding non-readied action (Strike, Step, Raise Shield handlers).
- REQ-READY-11: Readied action execution MUST NOT cost AP.
- REQ-READY-12: Readied action execution MUST set `ReactionUsed = true` and clear `ReadiedTrigger`/`ReadiedAction`.

### 4.1 Expiry

At end of round (after all AP are resolved and no trigger fired):

1. If `ReadiedTrigger != ""`: notify player "Your readied action expires. (No refund.)"
2. Clear `ReadiedTrigger` and `ReadiedAction`.

- REQ-READY-13: If the readied trigger is not met by end of round, the readied action MUST expire with a notification. AP spent on Ready MUST NOT be refunded.

---

## 5. Architecture

### 5.1 Command Pattern

`ready` follows CMD-1 through CMD-7: `HandlerReady` constant, `BuiltinCommands()` entry, `ReadyRequest { action string, trigger string }` proto message in `ClientMessage` oneof, bridge handler, `handleReady` case in `grpc_service.go`.

### 5.2 Round Resolution Integration

Ready-action trigger evaluation integrates with the **existing** `ReactionCallback` mechanism in `ResolveRound` — it does NOT add separate hooks. The `reactionFn` parameter already fires at every qualifying combat event; the callback implementation in `grpc_service.go` is extended to also check `PlayerSession.ReadiedTrigger` and dispatch readied actions.

**New `ReactionTriggerType` constant** added to `internal/game/reaction/trigger.go`:

```go
// TriggerOnEnemyEntersRoom fires when an enemy moves into the player's room.
// Distinct from TriggerOnEnemyMoveAdjacent (which fires on position change within a room).
TriggerOnEnemyEntersRoom ReactionTriggerType = "on_enemy_enters_room"
```

`TriggerOnDamageTaken` (existing, fires after damage calculated but before applied) maps to the `enemy_attacks_me` trigger — the correct point for Raise Shield to reduce pending damage and for Strike to counter-attack. No new trigger type needed for this case.

`TriggerOnAllyDamaged` (existing, fires after damage is applied to an ally) maps to the `ally_attacked` trigger. Note: Raise Shield readied on this trigger protects the player themselves for subsequent attacks, not the ally retroactively.

**Trigger-to-ReactionTriggerType mapping:**

| Ready-action trigger | ReactionTriggerType |
|---|---|
| `enemy_enters` | `TriggerOnEnemyEntersRoom` (new) |
| `enemy_attacks_me` | `TriggerOnDamageTaken` (existing) |
| `ally_attacked` | `TriggerOnAllyDamaged` (existing) |

**Fire points in `ResolveRound`:**
- `TriggerOnEnemyEntersRoom`: fired immediately after an NPC's `ActionStride` resolves as a room-entry event. Fired after trap trigger evaluation (traps fire first, then ready-action).
- `TriggerOnDamageTaken`: already fires after damage calculation, before damage application — no new fire point needed.
- `TriggerOnAllyDamaged`: already fires after ally damage is applied — no new fire point needed.

**Callback extension** in `grpc_service.go`: the `reactionFn` closure checks:
1. Existing tech reaction logic (unchanged).
2. `sess.ReadiedTrigger` — if the trigger type matches the session's readied trigger, execute the readied action (Strike/Step/Raise Shield) targeting `ctx.SourceUID`. Clear readied state. Set `ReactionUsed = true`.

- REQ-READY-14: Ready-action trigger evaluation MUST use the existing `ReactionCallback` mechanism. Only `TriggerOnEnemyEntersRoom` requires a new fire point in `ResolveRound`; `enemy_attacks_me` reuses the existing `TriggerOnDamageTaken` fire point; `ally_attacked` reuses the existing `TriggerOnAllyDamaged` fire point.
- REQ-READY-15: `TriggerOnEnemyEntersRoom` MUST fire after trap trigger evaluation when an NPC enters the room.

### 5.3 `handleReady` in `grpc_service.go`

```go
func (s *GameServiceServer) handleReady(uid, action, trigger string) {
    sess := s.sessions.Get(uid)
    // validate: in combat, ≥2 AP, no current readied action, reaction available
    // deduct 2 AP
    // set sess.ReadiedTrigger, sess.ReadiedAction
    // notify player
}
```

---

## 6. Requirements Summary

- REQ-READY-1: `ReadiedTrigger` and `ReadiedAction` MUST be cleared at end of every round.
- REQ-READY-2: `ready` MUST fail if the player is not in combat.
- REQ-READY-3: `ready` MUST fail if fewer than 2 AP remain this round.
- REQ-READY-4: `ready` MUST fail if a readied action is already set.
- REQ-READY-5: `ready` MUST fail if `ReactionUsed == true`.
- REQ-READY-6: `ready` MUST fail if the action or trigger alias is unrecognized.
- REQ-READY-7: Trigger evaluation MUST occur after each qualifying combat event, not batch at end-of-round.
- REQ-READY-8: `enemy_enters` MUST reuse the room-entry event from the consumable-traps system.
- REQ-READY-9: `enemy_attacks_me` MUST fire before damage from the triggering attack resolves.
- REQ-READY-10: Readied action execution MUST use the same resolution logic as the non-readied equivalent.
- REQ-READY-11: Readied action execution MUST NOT cost AP.
- REQ-READY-12: Readied action execution MUST set `ReactionUsed = true` and clear readied state.
- REQ-READY-13: Expired readied actions MUST notify the player with no AP refund.
- REQ-READY-14: Ready-action trigger evaluation MUST use the existing `ReactionCallback` mechanism. Only `TriggerOnEnemyEntersRoom` requires a new fire point in `ResolveRound`; the other two triggers reuse existing fire points.
- REQ-READY-15: `TriggerOnEnemyEntersRoom` MUST fire after trap trigger evaluation when an NPC enters the room.
