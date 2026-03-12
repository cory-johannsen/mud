# Forced Action Execution — Design Spec

**Date:** 2026-03-12

---

## Goal

When certain mental state conditions are active, the player loses control of their action each combat round. The condition forces a specific action regardless of player input.

---

## Affected Conditions

| Condition | Severity | Forced Behavior |
|---|---|---|
| `fear_panicked` | Fear 2 | Forced random attack — attacks a random alive combatant (any faction, including allies or self) |
| `fear_psychotic` | Fear 3 | Forced random attack — same as panicked, deeper loss of control |
| `rage_berserker` | Rage 3 | Forced lowest-HP attack — attacks the alive combatant with the lowest current HP (any faction) |

---

## Architecture

### ConditionDef Field (`internal/game/condition/definition.go`)

Add a new field to `ConditionDef`:

```go
ForcedAction string `yaml:"forced_action"`
```

Valid values:
- `"random_attack"` — attack a random alive combatant
- `"lowest_hp_attack"` — attack the alive combatant with the lowest current HP
- `""` (empty) — no forced action

### Condition Modifier Helper (`internal/game/condition/modifiers.go`)

```go
// ForcedActionType returns the forced_action string from the highest-priority
// active condition that has one, or empty string if none.
func ForcedActionType(s *ActiveSet) string
```

### YAML Updates

- `content/conditions/mental/fear_panicked.yaml` → add `forced_action: random_attack`
- `content/conditions/mental/fear_psychotic.yaml` → add `forced_action: random_attack`
- `content/conditions/mental/rage_berserker.yaml` → add `forced_action: lowest_hp_attack`

### ActionQueue Method (`internal/game/combat/action.go`)

```go
// ClearActions drains all queued actions and restores full AP for this round.
func (q *ActionQueue) ClearActions()
```

### Combat Handler Extension (`internal/gameserver/combat_handler.go`, `autoQueuePlayersLocked`)

Before the existing `SkipTurn` check, insert the following logic:

1. Call `condition.ForcedActionType(sess.Conditions)`.
2. If non-empty:
   - Call `q.ClearActions()` to override any player-submitted actions.
   - `"random_attack"`: pick a random alive combatant (any faction) using `rand.Intn`. Queue `ActionAttack` against it. Notify player: `"Panic grips you — you lash out wildly at [target]!"`
   - `"lowest_hp_attack"`: scan all alive combatants for minimum `CurrentHP`. Queue `ActionAttack` against that target. Notify player: `"Berserker rage drives you to destroy the weakest target — you attack [target]!"`
3. `continue` to skip normal action selection.

Target resolution uses the combatants list already available in `autoQueuePlayersLocked`. If no valid target exists (edge case: all others dead, only self remains), attack self.

---

## Testing

### `condition/modifiers_test.go`

- REQ-T1: `ForcedActionType` returns empty string when no forced condition is active.
- REQ-T2: `ForcedActionType` returns the correct type when a forced condition is active.
- REQ-T3: When multiple forced conditions are active, `ForcedActionType` returns the type from the highest-priority condition.

### `internal/game/combat/action_test.go`

- REQ-T4: `ClearActions` drains the queue and restores full AP for the round.

### `internal/gameserver/grpc_service_forced_action_test.go` (integration)

- REQ-T5: A player with `fear_panicked` active receives a forced random attack, not their submitted target.
- REQ-T6: A player with `rage_berserker` active attacks the lowest-HP alive combatant.
- REQ-T7: The forced action override applies even when the player pre-submitted an action.
- REQ-T8: A player with no forced condition proceeds through normal action selection unaffected.
- REQ-T9 (property): A forced action always targets an alive combatant.

All tests MUST use property-based testing per SWENG-5a.

---

## Out of Scope (Future Work)

- "Pass randomly" behavior for Panicked (currently always attacks).
- Forced flee for Panicked at maximum severity.
- NPC-side forced actions.
