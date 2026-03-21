# Ready Action

Players spend 2 AP in combat to ready a (trigger, action) pair. When the trigger fires during round resolution, the readied action executes as a free Reaction and then expires. If the round ends without the trigger firing, the readied action also expires with no AP refund.

See `docs/superpowers/specs/2026-03-20-ready-action-design.md` for the full design spec.

## Requirements

- [x] REQ-RA-1: `ready <action> when <trigger>` command MUST cost 2 AP and be available only in combat.
- [x] REQ-RA-2: Valid actions MUST be: strike, step, raise_shield (plus aliases).
- [x] REQ-RA-3: Valid triggers MUST be: enemy_enters, enemy_attacks_me, ally_attacked (plus aliases).
- [x] REQ-RA-4: A player MUST NOT ready an action when they already have one pending.
- [x] REQ-RA-5: A readied action MUST fire exactly once when its trigger fires, then be cleared.
- [x] REQ-RA-6: A readied action MUST expire at end-of-round with no AP refund if the trigger never fired.
- [x] REQ-RA-7: The `enemy_enters` trigger MUST fire when any enemy combatant moves in combat.
- [x] REQ-RA-8: The `enemy_attacks_me` trigger MUST fire when the player takes damage.
- [x] REQ-RA-9: The `ally_attacked` trigger MUST fire when an ally takes damage.

## Implementation

Completed 2026-03-21. `HandlerReady = "ready"` command dispatches to `handleReady` in `grpc_service_ready.go`. Session fields `ReadiedTrigger`/`ReadiedAction` on `PlayerSession` store the pending action. `buildReactionCallback` in `reaction_handler.go` checks `matchesReadyTrigger` before existing tech reactions and fires via `executeReadiedAction`. `checkEnemyEntersReadyTrigger` in `grpc_service_trap.go` fires the `TriggerOnEnemyEntersRoom` fire point from the `onCombatantMoved` callback. End-of-round expiry in `combat_handler.go` clears stale readied actions with an expiry notification.
