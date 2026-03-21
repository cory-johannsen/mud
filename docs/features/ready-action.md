# Ready Action

Implement the PF2E Ready action: costs 2 AP, declares a trigger condition and a stored readied action (Strike, Step, Raise Shield) that fires automatically as a Reaction when the trigger occurs. Integrates with the existing `ReactionCallback` mechanism. See `docs/superpowers/specs/2026-03-20-ready-action-design.md` for full design spec.

## Requirements

### Data Model

- [ ] `PlayerSession.ReadiedTrigger string` — `"enemy_enters"` | `"enemy_attacks_me"` | `"ally_attacked"`; `""` = no readied action
- [ ] `PlayerSession.ReadiedAction string` — `"strike"` | `"step"` | `"raise_shield"`
- [ ] REQ-READY-1: Both fields MUST be cleared at end of every round

### Command

- [ ] `ready <action> when <trigger>` — `HandlerReady` constant, proto `ReadyRequest{action, trigger}`, bridge handler, `handleReady` in `grpc_service.go`
- [ ] Action aliases: `strike`, `step`, `shield`
- [ ] Trigger aliases: `enters` (enemy enters room), `attacks` (enemy attacks me), `ally` (ally is attacked)
- [ ] REQ-READY-2: Fail if not in combat
- [ ] REQ-READY-3: Fail if fewer than 2 AP remain
- [ ] REQ-READY-4: Fail if readied action already set
- [ ] REQ-READY-5: Fail if `ReactionUsed == true`
- [ ] REQ-READY-6: Fail if action or trigger alias unrecognized
- [ ] On success: deduct 2 AP, set `ReadiedTrigger`/`ReadiedAction`, notify player

### Trigger Evaluation

- [ ] REQ-READY-7: Trigger evaluation occurs after each qualifying combat event (not batch at end-of-round)
- [ ] REQ-READY-8: `enemy_enters` reuses room-entry event from consumable-traps floor-state system
- [ ] REQ-READY-9: `enemy_attacks_me` uses `TriggerOnDamageTaken` (fires after damage calculated, before applied); Raise Shield subtracts hardness from pending damage
- [ ] New `TriggerOnEnemyEntersRoom ReactionTriggerType` added to `internal/game/reaction/trigger.go`
- [ ] REQ-READY-14: Integrates with existing `ReactionCallback` mechanism; `reactionFn` closure in `grpc_service.go` extended to check `ReadiedTrigger`
- [ ] REQ-READY-15: `TriggerOnEnemyEntersRoom` fires after trap trigger evaluation when NPC enters room

| Trigger | ReactionTriggerType |
|---|---|
| `enemy_enters` | `TriggerOnEnemyEntersRoom` (new) |
| `enemy_attacks_me` | `TriggerOnDamageTaken` (existing) |
| `ally_attacked` | `TriggerOnAllyDamaged` (existing) |

### Readied Action Execution

- [ ] REQ-READY-10: Execution uses same resolution logic as non-readied Strike/Step/Raise Shield
- [ ] REQ-READY-11: Execution costs no AP
- [ ] REQ-READY-12: On execution: set `ReactionUsed = true`, clear `ReadiedTrigger`/`ReadiedAction`
- [ ] Notify player "Your readied <action> fires!" and room "<player> reacts with a <action>!"
- [ ] If `ReactionUsed == true` when trigger fires: skip silently, clear readied state

### Expiry

- [ ] REQ-READY-13: If trigger not met by end of round: notify "Your readied action expires. (No refund.)", clear state; no AP refund
