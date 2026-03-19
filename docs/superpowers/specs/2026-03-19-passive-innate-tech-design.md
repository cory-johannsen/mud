# Passive Innate Tech Mechanics — Seismic Sense Design

**Date:** 2026-03-19
**Sub-project:** Passive Innate Tech Mechanics

---

## Goal

Make `seismic_sense` apply its mechanical effects passively — revealing all creatures in the room (including hidden) to the player who has the implant — without requiring the `use` command. The effect fires automatically whenever room state changes (player enters or leaves).

---

## Scope

In scope: `seismic_sense` tremorsense mechanical implementation. Out of scope: `moisture_reclaim` (separate sub-project — cantrip refactor), other passive techs, tick-based periodic passive evaluation, notifying other players that they have been detected, NPC hidden-state detection beyond what is already tracked.

---

## Context

`seismic_sense.yaml` already has `passive: true` and `action_cost: 0`. `triggerPassiveTechsForRoom(roomID)` already exists in `grpc_service.go` and fires on `handleMove` for both source and destination rooms. Currently the passive trigger emits only a human-readable `utility` description with no mechanical effect. `GameServiceServer` has access to `s.sessions.PlayersInRoomDetails(roomID)` for player enumeration and `s.npcH.InstancesInRoom(roomID)` (returns `[]*npc.Instance`) for NPC enumeration. Both `RoomQuerier` and `CreatureInfo` live in `package gameserver`. `applyEffect` already receives `sess *session.PlayerSession` as its first argument; `sess.UID` identifies the sensing player. `activateTechWithEffects` currently has the signature `(sess, uid, abilityID, targetID, fallbackMsg string)` returning `(*gamev1.ServerEvent, error)` — `querier RoomQuerier` will be appended as a sixth parameter, leaving all existing parameters unchanged. The existing `ResolveTechEffects` godoc postcondition ("Returns at least one message.") and its `"No effect."` fallback guard MUST both be updated. `grpc_service_passive_test.go` already exists and will be extended. Property-based tests MUST use `pgregory.net/rapid` (already a project dependency).

---

## Requirements

### REQ-PSV1
`tremorsense` MUST be added as a named constant to the `EffectType` type in `internal/game/technology/model.go`.

### REQ-PSV2
`tremorsense` MUST be added to the `validEffectTypes` map in `internal/game/technology/model.go` so that YAML files using `type: tremorsense` pass validation.

### REQ-PSV3
`seismic_sense.yaml` MUST be updated so that the `on_apply` effect block uses `type: tremorsense` in place of `type: utility`.

### REQ-PSV4
`RoomQuerier` MUST be defined as an interface in `internal/gameserver/tech_effect_resolver.go` with a single method `CreaturesInRoom(roomID, sensingUID string) []CreatureInfo`. `CreatureInfo` MUST be a struct with two exported fields: `Name string` and `Hidden bool`.

### REQ-PSV5
`FormatTremorsenseOutput(creatures []CreatureInfo) string` MUST be defined as an exported function in `internal/gameserver/tech_effect_resolver.go`. When `creatures` is empty, it MUST return `"[Seismic Sense] No creatures detected."`. When `creatures` is non-empty, it MUST return `"[Seismic Sense] Creatures detected in this room: "` followed by a comma-separated list of entries in slice order. Hidden entries (`Hidden == true`) MUST appear with their name suffixed by ` (concealed)`. Visible entries (`Hidden == false`) MUST appear with their name only.

### REQ-PSV6
The `"No effect."` fallback guard in `ResolveTechEffects` MUST be removed. `ResolveTechEffects` MUST filter empty strings from its result slice before returning. The `ResolveTechEffects` godoc postcondition MUST be updated to read "Returns zero or more non-empty messages."

### REQ-PSV7
`ResolveTechEffects` MUST accept a `querier RoomQuerier` parameter as its final argument and pass it to `applyEffect`. A `nil` querier MUST be treated as valid.

### REQ-PSV8
`activateTechWithEffects` MUST accept a `querier RoomQuerier` parameter appended after the existing `fallbackMsg string` parameter and forward it to `ResolveTechEffects`. All existing call sites MUST pass `nil` for `querier`, except `triggerPassiveTechsForRoom` which MUST pass `s`.

### REQ-PSV9
`applyEffect` MUST accept a `querier RoomQuerier` parameter threaded from `ResolveTechEffects`. When handling `type: tremorsense` with a `nil` querier, `applyEffect` MUST return an empty string.

### REQ-PSV10
When handling `type: tremorsense` with a non-nil querier, `applyEffect` MUST call `querier.CreaturesInRoom(sess.RoomID, sess.UID)`, pass the result to `FormatTremorsenseOutput`, and return the result. It MUST NOT decrement any innate tech uses.

### REQ-PSV11
`GameServiceServer` MUST implement `RoomQuerier` via `CreaturesInRoom(roomID, sensingUID string) []CreatureInfo`. For each `*npc.Instance` from `s.npcH.InstancesInRoom(roomID)` it MUST append `CreatureInfo{Name: inst.Name, Hidden: false}`. For each player session from `s.sessions.PlayersInRoomDetails(roomID)` whose `UID` equals `sensingUID` it MUST append `CreatureInfo{Name: "you", Hidden: false}`; all other players MUST be appended as `CreatureInfo{Name: sess.CharName, Hidden: false}`.

### REQ-PSV12
The tremorsense message MUST NOT be pushed to any player's stream other than the stream of the player who has `seismic_sense`.

### REQ-PSV13
A property-based test using `pgregory.net/rapid` in `internal/gameserver/tech_effect_resolver_test.go` MUST assert the following invariant on `FormatTremorsenseOutput`: for any non-empty slice generated by `rapid.SliceOf(rapid.Custom(genCreatureInfo), rapid.MinLen(1))`, every element with `Hidden == true` appears in the output suffixed with ` (concealed)`, and every element with `Hidden == false` does not. `genCreatureInfo` MUST be:
```go
func genCreatureInfo(t *rapid.T) CreatureInfo {
    return CreatureInfo{
        Name:   rapid.StringN(1, 20, -1).Draw(t, "name"),
        Hidden: rapid.Bool().Draw(t, "hidden"),
    }
}
```

### REQ-PSV14
Table-driven unit tests in `internal/gameserver/tech_effect_resolver_test.go` MUST cover the following cases for `FormatTremorsenseOutput`: empty slice returns the no-creatures message; single visible creature; single hidden creature; mixed visible and hidden creatures.

### REQ-PSV15
Integration tests added to `internal/gameserver/grpc_service_passive_test.go` MUST use the following test double and verify that a player with `seismic_sense` receives the formatted creature list when `triggerPassiveTechsForRoom` fires, and that other players in the room do not:
```go
type mockRoomQuerier struct{ creatures []CreatureInfo }
func (m *mockRoomQuerier) CreaturesInRoom(_, _ string) []CreatureInfo { return m.creatures }
```

### REQ-PSV16
`go test ./internal/game/technology/... ./internal/gameserver/...` MUST pass after all changes are applied.

---

## Architecture

### Data Flow

```
handleMove
  └─ triggerPassiveTechsForRoom(roomID)
       └─ for each player in room with passive innate tech:
            └─ activateTechWithEffects(sess, uid, techID, "", "", s /*RoomQuerier*/)
                 └─ ResolveTechEffects(sess, tech, nil, nil, condRegistry, src, querier)
                      └─ applyEffect(sess, effect, ..., querier) → tremorsense handler
                           └─ FormatTremorsenseOutput(querier.CreaturesInRoom(sess.RoomID, sess.UID))
                                └─ "[Seismic Sense] Creatures detected in this room: ..."
                      └─ empty strings filtered; zero or more non-empty messages returned
                 └─ non-empty messages pushed to sess.Entity (player stream only)
```

### Modified Signatures

```go
// tech_effect_resolver.go — new types
type RoomQuerier interface {
    CreaturesInRoom(roomID, sensingUID string) []CreatureInfo
}
type CreatureInfo struct {
    Name   string
    Hidden bool
}

// New exported helper
func FormatTremorsenseOutput(creatures []CreatureInfo) string

// ResolveTechEffects — querier is the final new parameter; nil is safe
// Postcondition: returns zero or more non-empty messages.
func ResolveTechEffects(
    sess *session.PlayerSession,
    tech *technology.TechnologyDef,
    targets []*combat.Combatant,
    cbt *combat.Combat,
    condRegistry *condition.Registry,
    src combat.Source,
    querier RoomQuerier,
) []string

// activateTechWithEffects — querier appended after existing fallbackMsg param; nil is safe
func (s *GameServiceServer) activateTechWithEffects(
    sess *session.PlayerSession,
    uid, abilityID, targetID, fallbackMsg string,
    querier RoomQuerier,
) (*gamev1.ServerEvent, error)
```

---

## File Map

| File | Change |
|------|--------|
| `internal/game/technology/model.go` | Add `tremorsense` to `EffectType` constants and to `validEffectTypes` map |
| `content/technologies/innate/seismic_sense.yaml` | Change `type: utility` → `type: tremorsense` in `on_apply` |
| `internal/gameserver/tech_effect_resolver.go` | Define `RoomQuerier`, `CreatureInfo`, `FormatTremorsenseOutput`; add `querier` param to `ResolveTechEffects` and `applyEffect`; add tremorsense handler; remove `"No effect."` guard; filter empty strings before return; update `ResolveTechEffects` godoc postcondition |
| `internal/gameserver/grpc_service.go` | Add `querier` param to `activateTechWithEffects`; implement `CreaturesInRoom` on `GameServiceServer`; pass `s` in `triggerPassiveTechsForRoom`; pass `nil` at all other call sites |
| `internal/gameserver/tech_effect_resolver_test.go` | Add PBT invariant test (REQ-PSV13) and table-driven tests (REQ-PSV14) for `FormatTremorsenseOutput` |
| `internal/gameserver/grpc_service_passive_test.go` | Extend with integration tests using `mockRoomQuerier` (REQ-PSV15) |
