# Passive Innate Tech Mechanics — Seismic Sense Design

**Date:** 2026-03-19
**Sub-project:** Passive Innate Tech Mechanics

---

## Goal

Make `seismic_sense` apply its mechanical effects passively — revealing all creatures in the room (including hidden) to the player who has the implant — without requiring the `use` command. The effect fires automatically whenever room state changes (player enters or leaves).

---

## Scope

- **In scope:** `seismic_sense` tremorsense mechanical implementation.
- **Out of scope:** `moisture_reclaim` (separate sub-project — cantrip refactor). Other passive techs. Tick-based periodic passive evaluation.

---

## Context

- `seismic_sense.yaml` already has `passive: true` and `action_cost: 0`.
- `triggerPassiveTechsForRoom(roomID)` already exists in `grpc_service.go` and fires on `handleMove` (both source and destination rooms).
- Currently the passive trigger emits only a human-readable `utility` description with no mechanical effect.
- The `GameServiceServer` has access to `s.sessions.PlayersInRoomDetails(roomID)` for player enumeration. NPC room presence is accessible via the combat/world system.

---

## Requirements

### REQ-PSV1
The `tremorsense` value MUST be added to the `EffectType` enum in `internal/game/technology/model.go`.

### REQ-PSV2
`seismic_sense.yaml` MUST be updated to replace `type: utility` with `type: tremorsense` in the `on_apply` effect block.

### REQ-PSV3
`ResolveTechEffects` MUST accept a `RoomQuerier` interface parameter. Existing callers that pass `nil` MUST continue to work without change.

### REQ-PSV4
The `RoomQuerier` interface MUST be defined as:
```go
type RoomQuerier interface {
    CreaturesInRoom(roomID string) []CreatureInfo
}

type CreatureInfo struct {
    Name   string
    Hidden bool
}
```

### REQ-PSV5
`applyEffect` MUST handle `type: tremorsense` by calling `querier.CreaturesInRoom(sess.RoomID)`, formatting the result as a creature list, and returning it as the effect message string.

### REQ-PSV6
Hidden creatures MUST appear in the tremorsense output tagged as `(concealed)`. They MUST NOT be omitted.

### REQ-PSV7
The effect message MUST be prefixed with `[Seismic Sense]` and list all creatures including the sensing player (shown as `you`). Example:
```
[Seismic Sense] Creatures detected in this room: Guard Captain, Street Rat (concealed), you
```

### REQ-PSV8
`GameServiceServer` MUST implement `RoomQuerier` via `CreaturesInRoom(roomID string) []CreatureInfo`, querying both `s.sessions.PlayersInRoomDetails` (players) and active NPCs in the room.

### REQ-PSV9
The tremorsense passive effect MUST NOT decrement innate tech uses (consistent with existing passive activation behavior).

### REQ-PSV10
The tremorsense message MUST be pushed only to the player who has `seismic_sense`. Other players in the room MUST NOT receive it.

### REQ-PSV11
`go test ./internal/game/technology/... ./internal/gameserver/... -run` MUST pass after all changes. Property-based tests MUST cover: tremorsense with no creatures, with visible creatures only, with hidden creatures only, with mixed visible and hidden creatures.

---

## Architecture

### Data Flow

```
handleMove
  └─ triggerPassiveTechsForRoom(roomID)
       └─ for each player in room with passive innate tech:
            └─ activateTechWithEffects(sess, uid, techID, "", "")
                 └─ ResolveTechEffects(sess, tech, nil, nil, condRegistry, nil, querier)
                      └─ applyEffect → tremorsense handler
                           └─ querier.CreaturesInRoom(sess.RoomID)
                                └─ []CreatureInfo → format → "[Seismic Sense] Creatures detected..."
                      └─ message pushed to sess.Entity (player stream only)
```

### RoomQuerier Interface

Defined in `internal/gameserver/tech_effect_resolver.go` alongside `ResolveTechEffects`. `GameServiceServer` implements it. Tests use a mock implementation.

### Modified Signatures

```go
// tech_effect_resolver.go
func ResolveTechEffects(
    sess *session.PlayerSession,
    tech *technology.TechnologyDef,
    targets []*combat.Combatant,
    cbt *combat.Combat,
    condRegistry *condition.Registry,
    src combat.Source,
    querier RoomQuerier,  // NEW — nil-safe
) []string
```

All existing call sites pass `nil` for `querier`. Only `triggerPassiveTechsForRoom` passes a non-nil querier (the `GameServiceServer` itself).

---

## File Map

| File | Change |
|------|--------|
| `internal/game/technology/model.go` | Add `tremorsense` to `EffectType` |
| `content/technologies/innate/seismic_sense.yaml` | Change `type: utility` → `type: tremorsense` |
| `internal/gameserver/tech_effect_resolver.go` | Add `RoomQuerier` interface, `CreatureInfo` type; add `querier` param to `ResolveTechEffects`; add tremorsense handler in `applyEffect` |
| `internal/gameserver/grpc_service.go` | Implement `CreaturesInRoom` on `GameServiceServer`; pass `s` as querier in `triggerPassiveTechsForRoom`; update all `ResolveTechEffects` call sites to pass `nil` |
| `internal/gameserver/tech_effect_resolver_test.go` | Add property-based tests for tremorsense effect |
| `internal/gameserver/grpc_service_passive_test.go` | Add/extend tests for seismic_sense passive activation with creature reveal |

---

## Out of Scope

- `moisture_reclaim` passive behavior (separate cantrip refactor sub-project)
- NPC hidden state detection beyond what is already tracked
- Tick-based periodic passive evaluation
- Notifying other players that they have been detected
