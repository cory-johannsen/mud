# Default Combat Actions Design

## Goal

Allow players to set a persistent default combat action that auto-queues when a round resolves without the player having issued a command.

## Command

- Handler constant: `HandlerCombatDefault = "combat_default"`, alias `"cd"`
- `HandleCombatDefault(rawArgs string) string` — validates action name; returns normalized name or error/usage string
- Valid actions: `attack`, `strike`, `pass`, `flee`, `reload`, `fire_burst`, `fire_automatic`, `throw`
- Default default: `pass`

## Proto

```proto
message CombatDefaultRequest {
  string action = 1;
}
```

Added to `ClientMessage` oneof.

## Bridge

`bridgeCombatDefault` in `bridge_handlers.go` — delegates validation to `HandleCombatDefault`; sends proto on valid input, writes error locally on invalid.

## gRPC Handler

`handleCombatDefault(uid, action string)` in `grpc_service.go`:
- Updates `sess.DefaultCombatAction`
- Persists via `charRepo.SaveDefaultCombatAction(ctx, characterID, action)`
- Sends confirmation message to player

## Data Model

### DB migration

```sql
ALTER TABLE characters
  ADD COLUMN IF NOT EXISTS default_combat_action TEXT NOT NULL DEFAULT 'pass';
```

### Session fields (new)

- `DefaultCombatAction string` — loaded at login from `characters.default_combat_action`
- `LastCombatTarget string` — set whenever player explicitly queues `attack` or `strike`

### Repository method

`SaveDefaultCombatAction(ctx context.Context, characterID int64, action string) error`
on `CharacterRepository` — single `UPDATE characters SET default_combat_action = $1 WHERE id = $2`.

### Session init

`default_combat_action` read from `dbChar` during join and stored in `sess.DefaultCombatAction`.

## Auto-Queue Logic

In `combat_handler.go`, `autoQueuePlayersLocked(cbt *combat.Combat)` runs just before `ResolveRound` inside `resolveAndAdvanceLocked`:

For each living player combatant with no queued action:

1. Look up `sess.DefaultCombatAction`
2. Attempt to queue; fall back to `pass` per action:

| Default | Condition to queue | Fallback |
|---|---|---|
| `attack` / `strike` | Living target exists (last target or first living NPC) | `pass` |
| `flee` | Always | — |
| `reload` | Player has equipped weapon with empty ammo | `pass` |
| `fire_burst` / `fire_automatic` | Player has appropriate ranged weapon | `pass` |
| `throw` | Player has explosive in inventory | `pass` |
| `pass` | Always | — |

3. `LastCombatTarget` updated in `CombatHandler.Attack()` and `CombatHandler.Strike()` whenever a player explicitly queues an action.

## Notification

When a default action fires, a narrative is included in the round events (e.g. `"Kira auto-attacks Ganger (default action)."`) so players know the round advanced without their input.

## Testing

- Unit: `HandleCombatDefault` — valid/invalid inputs, all 8 action names, case normalization
- Property-based: arbitrary string input never panics; valid action names always return themselves
- Integration: `SaveDefaultCombatAction` round-trip via shared postgres container
- Combat: `autoQueuePlayersLocked` queues correct action; fallback to `pass` when conditions not met; `LastCombatTarget` tracking
- Wiring: `TestAllCommandHandlersAreWired` passes
