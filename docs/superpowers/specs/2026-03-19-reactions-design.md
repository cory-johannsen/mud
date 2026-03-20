# Reactions System Design

**Date:** 2026-03-19
**Feature:** Reactions — general-purpose player reaction system (Sub-project 1 of 3)

---

## Goal

Implement a general-purpose reaction system for players, modeled on PF2E reactions. Players get one reaction per round, declared via feat YAML with a trigger field. When a trigger fires during round resolution, the player is prompted interactively via their stream. If they accept, the reaction effect executes inline before the triggering action continues.

Sub-project 2 (Initial Reactions: Reactive Strike, chrome_reflex) and Sub-project 3 (PF2E reaction import) depend on this infrastructure.

---

## Scope

In scope: `ReactionDef` on `FeatDef` and `TechnologyDef`, `ReactionTriggerType` enum, `ReactionRegistry` on session, `ReactionsRemaining` on session, `ReactionCallback` parameter on `ResolveRound`, prompt-and-execute flow, reaction effect types (`reroll_save`, `strike`, `reduce_damage`), reset at round start, all callers of `ResolveRound` updated to pass `nil`.

Out of scope: NPC reactions (Advanced Enemies), sub-project 2 content (Reactive Strike, chrome_reflex), sub-project 3 content (Shield Block, Grab an Edge, etc.), persistence of reactions, multiple reactions per round.

---

## Data Model

### REQ-RXN1
`ReactionTriggerType` MUST be defined as a string enum in `internal/game/reaction/trigger.go` with the following values: `TriggerOnSaveFail`, `TriggerOnSaveCritFail`, `TriggerOnDamageTaken`, `TriggerOnEnemyMoveAdjacent`, `TriggerOnConditionApplied`, `TriggerOnAllyDamaged`, `TriggerOnFall`.

### REQ-RXN2
`ReactionEffectType` MUST be defined as a string enum in `internal/game/reaction/trigger.go` with values: `ReactionEffectRerollSave`, `ReactionEffectStrike`, `ReactionEffectReduceDamage`.

### REQ-RXN3
`ReactionEffect` MUST be defined as a struct in `internal/game/reaction/trigger.go`:
```go
type ReactionEffect struct {
    Type   ReactionEffectType `yaml:"type"`
    Target string             `yaml:"target,omitempty"` // "trigger_source" etc.
    Keep   string             `yaml:"keep,omitempty"`   // "better" for reroll_save
}
```

### REQ-RXN4
`ReactionDef` MUST be defined as a struct in `internal/game/reaction/trigger.go`:
```go
type ReactionDef struct {
    Trigger     ReactionTriggerType `yaml:"trigger"`
    Requirement string              `yaml:"requirement,omitempty"`
    Effect      ReactionEffect      `yaml:"effect"`
}
```

### REQ-RXN5
`FeatDef` in `internal/game/feat/model.go` MUST gain an optional `Reaction *ReactionDef` field tagged `yaml:"reaction,omitempty"`.

### REQ-RXN6
`TechnologyDef` in `internal/game/technology/model.go` MUST gain an optional `Reaction *ReactionDef` field tagged `yaml:"reaction,omitempty"`. This enables innate techs like `chrome_reflex` to declare reactions.

### REQ-RXN7
YAML feat files may declare a `reaction` block. Example:
```yaml
id: reactive_strike
name: Reactive Strike
reaction:
  trigger: on_enemy_move_adjacent
  requirement: wielding_melee_weapon
  effect:
    type: strike
    target: trigger_source
```

---

## Reaction Economy

### REQ-RXN8
`PlayerSession` in `internal/game/session/session.go` MUST gain an `int` field `ReactionsRemaining`.

### REQ-RXN9
At the start of each round, before any combatant actions are resolved, `ReactionsRemaining` MUST be set to 1 for every player in the combat. This reset MUST occur in the same location as AP reset.

### REQ-RXN10
When a reaction is spent, `ReactionsRemaining` MUST be decremented by 1. `ReactionsRemaining` MUST NOT go below 0.

### REQ-RXN11
`ReactionsRemaining` is in-session only and MUST NOT be persisted to the database.

---

## Reaction Registry

### REQ-RXN12
`ReactionRegistry` MUST be defined in `internal/game/reaction/registry.go`:
```go
type PlayerReaction struct {
    UID  string
    Feat string // feat or tech ID
    Def  ReactionDef
}

type ReactionRegistry struct {
    byTrigger map[ReactionTriggerType][]PlayerReaction
}

func NewReactionRegistry() *ReactionRegistry
func (r *ReactionRegistry) Register(uid string, featID string, def ReactionDef)
func (r *ReactionRegistry) Get(uid string, trigger ReactionTriggerType) *PlayerReaction
```

### REQ-RXN13
`ReactionRegistry.Get` MUST return the first registered `PlayerReaction` for the given `uid` and `trigger`, or `nil` if none exists.

### REQ-RXN14
`PlayerSession` MUST gain a `*ReactionRegistry` field `Reactions`. It MUST be initialised to `NewReactionRegistry()` at session creation.

### REQ-RXN15
At login, after loading feats, the session handler MUST iterate all loaded feats and innate techs for the player. For each with a non-nil `Reaction`, it MUST call `sess.Reactions.Register(uid, featID, *feat.Reaction)`.

---

## ResolveRound Callback

### REQ-RXN16
`ReactionContext` MUST be defined in `internal/game/reaction/trigger.go`:
```go
type ReactionContext struct {
    TriggerUID    string          // UID of the player whose reaction may fire
    SourceUID     string          // UID or NPC ID of the entity that caused the trigger
    DamagePending *int            // pointer to pending damage amount (for reduce_damage)
    SaveOutcome   *combat.Outcome // pointer to save outcome (for reroll_save)
    ConditionID   string          // condition being applied (for on_condition_applied)
}
```

### REQ-RXN17
`ReactionCallback` MUST be defined in `internal/game/reaction/trigger.go`:
```go
type ReactionCallback func(uid string, trigger ReactionTriggerType, ctx ReactionContext) (spent bool, err error)
```

### REQ-RXN18
`Combat.ResolveRound` in `internal/game/combat/round.go` MUST accept a new final parameter `reactionFn ReactionCallback`. A nil `reactionFn` MUST be treated as a no-op (no panic). All existing callers of `ResolveRound` MUST be updated to pass `nil`.

### REQ-RXN19
At each trigger fire point, `ResolveRound` (or its callees) MUST call `reactionFn(uid, trigger, ctx)` when `reactionFn != nil`. Fire points:

| Trigger | Location |
|---|---|
| `TriggerOnSaveFail` / `TriggerOnSaveCritFail` | After save roll outcome determined, before applying outcome effects |
| `TriggerOnDamageTaken` | After damage calculated, before `ApplyDamage` |
| `TriggerOnEnemyMoveAdjacent` | After NPC move action brings them into melee range of player |
| `TriggerOnConditionApplied` | Inside condition application, before condition takes effect |
| `TriggerOnAllyDamaged` | After an ally's `ApplyDamage` completes |
| `TriggerOnFall` | When fall damage would be applied |

---

## Reaction Prompt and Execution

### REQ-RXN20
The session handler MUST construct the `ReactionCallback` closure before calling `ResolveRound`, capturing `stream` and `sess`. The callback logic MUST:
1. Check `sess.ReactionsRemaining > 0` — if not, return `false, nil`
2. Call `sess.Reactions.Get(uid, trigger)` — if nil, return `false, nil`
3. Check requirement via `checkReactionRequirement(sess, reaction.Def.Requirement)` — if not met, return `false, nil`
4. Prompt the player: `"Use reaction: <feat.Name>? (yes/no)"` via `promptFeatureChoice`
5. If player responds `"no"`, return `false, nil`
6. Decrement `sess.ReactionsRemaining`
7. Call `applyReactionEffect(sess, reaction.Def.Effect, ctx)`
8. Return `true, nil`

### REQ-RXN21
`checkReactionRequirement(sess *session.PlayerSession, req string) bool` MUST be defined in `internal/gameserver/reaction_handler.go`. It MUST return `true` for `""` (no requirement). It MUST return `true` for `"wielding_melee_weapon"` if the player has a melee weapon equipped in their main hand.

### REQ-RXN22
`applyReactionEffect` MUST be defined in `internal/gameserver/reaction_handler.go` and handle each `ReactionEffectType`:

- **`ReactionEffectRerollSave`**: Reroll the save (same dice + modifiers). Replace `ctx.SaveOutcome` with the better of the two outcomes (higher save total wins). Return the new outcome.
- **`ReactionEffectStrike`**: Execute an immediate attack by the player against `ctx.SourceUID`. Append the resulting `RoundEvent` to the current round's event list.
- **`ReactionEffectReduceDamage`**: Subtract the player's equipped shield hardness from `*ctx.DamagePending`, clamping at 0.

### REQ-RXN23
The prompt message MUST follow the format: `"Reaction available: <FeatName> — <trigger description>. Use it? (yes / no)"` where trigger description is a human-readable string per trigger type (e.g., `"you failed a saving throw"`, `"an enemy moved adjacent to you"`).

---

## Requirement Checks

### REQ-RXN24
`checkReactionRequirement` MUST support the following requirement strings (case-insensitive):
- `""` — always true
- `"wielding_melee_weapon"` — player has a melee weapon in main hand slot

Additional requirements will be added in sub-projects 2 and 3 as needed.

---

## Testing

### REQ-RXN25
Unit tests in `internal/game/reaction/registry_test.go` MUST cover: register and retrieve by trigger type; `Get` returns nil when no reaction registered for trigger; `Get` returns nil for wrong UID.

### REQ-RXN26
Unit tests in `internal/game/reaction/trigger_test.go` MUST cover: all `ReactionTriggerType` values are valid; `ReactionDef` round-trips through YAML marshal/unmarshal correctly.

### REQ-RXN27
Unit tests in `internal/gameserver/reaction_handler_test.go` MUST cover: `checkReactionRequirement` returns true for empty string; returns true/false correctly for `"wielding_melee_weapon"`; `applyReactionEffect` reroll-save picks better outcome; `applyReactionEffect` reduce-damage clamps at 0.

### REQ-RXN28
Integration tests in `internal/gameserver/grpc_service_reaction_test.go` MUST cover:
- `ReactionsRemaining == 0`: trigger fires but prompt is skipped
- Player declines prompt: triggering action continues with original outcome
- Player accepts `reroll_save`: better outcome used in resolution
- Two triggers in one round: second skipped (no reactions remaining after first)
- `reactionFn == nil`: `ResolveRound` completes without panic

### REQ-RXN29
`go test ./internal/game/reaction/... ./internal/game/feat/... ./internal/game/technology/... ./internal/gameserver/... ./internal/game/combat/...` MUST pass after all changes.

---

## Architecture

### File Map

| File | Change |
|---|---|
| `internal/game/reaction/trigger.go` | New: `ReactionTriggerType`, `ReactionEffectType`, `ReactionEffect`, `ReactionDef`, `ReactionContext`, `ReactionCallback` |
| `internal/game/reaction/registry.go` | New: `ReactionRegistry`, `PlayerReaction`, `NewReactionRegistry`, `Register`, `Get` |
| `internal/game/reaction/registry_test.go` | New: unit tests |
| `internal/game/reaction/trigger_test.go` | New: unit tests |
| `internal/game/feat/model.go` | Add `Reaction *ReactionDef` field |
| `internal/game/technology/model.go` | Add `Reaction *ReactionDef` field |
| `internal/game/session/session.go` | Add `ReactionsRemaining int`, `Reactions *ReactionRegistry` |
| `internal/game/combat/round.go` | Add `reactionFn ReactionCallback` param to `ResolveRound`; add 6 trigger fire points |
| `internal/gameserver/reaction_handler.go` | New: `checkReactionRequirement`, `applyReactionEffect`, reaction callback constructor |
| `internal/gameserver/reaction_handler_test.go` | New: unit tests |
| `internal/gameserver/grpc_service.go` | Construct reaction callback at login (register feats/techs); pass callback to `ResolveRound`; update `ReactionsRemaining` reset at round start |
| `internal/gameserver/grpc_service_reaction_test.go` | New: integration tests |

### Data Flow

```
Login
  └─ load feats + innate techs
  └─ sess.Reactions.Register(...) for each with Reaction != nil
  └─ sess.ReactionsRemaining = 1

ResolveRound(reactionFn)
  └─ reset ReactionsRemaining = 1 for all players
  └─ for each combatant action:
       └─ at trigger fire points: reactionFn(uid, trigger, ctx)
            └─ check ReactionsRemaining > 0
            └─ registry.Get(uid, trigger)
            └─ checkReactionRequirement
            └─ promptFeatureChoice (stream blocks until response)
            └─ if yes: ReactionsRemaining--; applyReactionEffect
            └─ round resolution continues with (possibly modified) ctx
```
