# Prepared Tech Rearrangement at Rest — Design Spec

**Date:** 2026-03-16

---

## Goal

Add a `rest` command that allows a player to re-select which technologies fill their non-fixed prepared slots. Fixed slots remain fixed. The player is prompted for each open slot from the aggregated pool of all applicable grants (creation + level-up). Slot counts are preserved. A combat guard prevents resting mid-fight.

---

## Context

The prepared technology system (Technology Grants sprint, 2026-03-15) assigns slots at character creation via `AssignTechnologies` and appends new slots at level-up via `LevelUpTechnologies`. The `PreparedTechRepo` (`character_prepared_technologies` table) stores `(character_id, slot_level, slot_index, tech_id)` tuples. `fillFromPreparedPool` in `technology_assignment.go` already handles fixed pre-fill + pool prompt. This spec adds `RearrangePreparedTechs` and the `rest` command on top of that infrastructure.

---

## Feature 1: `RearrangePreparedTechs` function

New function in `internal/gameserver/technology_assignment.go` alongside `LevelUpTechnologies`:

```go
// RearrangePreparedTechs deletes all existing prepared slots and re-fills them
// by aggregating grants from job.TechnologyGrants and all job.LevelUpGrants
// entries for levels 1..sess.Level.
//
// Precondition: sess, job, prepRepo are non-nil.
// Postcondition: sess.PreparedTechs and prepRepo reflect the re-selected slots.
// If the aggregated grants have no SlotsByLevel entries, returns nil (no-op).
func RearrangePreparedTechs(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    job *ruleset.Job,
    techReg *technology.Registry,
    promptFn TechPromptFn,
    prepRepo PreparedTechRepo,
) error
```

### Aggregation logic

Build a synthetic `PreparedGrants` by merging:
- `job.TechnologyGrants.Prepared` (if non-nil)
- `job.LevelUpGrants[lvl].Prepared` for all `lvl` where `1 ≤ lvl ≤ sess.Level` (if non-nil)

Merge rules:
- `SlotsByLevel`: use `len(sess.PreparedTechs[level])` as the authoritative slot count per level (session is source of truth, not the grants)
- `Fixed`: concatenate all Fixed entries across all applicable grants
- `Pool`: concatenate all Pool entries across all applicable grants (duplicates allowed — `fillFromPreparedPool` deduplicates by consuming chosen entries)

If the merged `SlotsByLevel` is empty, return nil immediately (no-op).

### Re-fill logic

1. Call `prepRepo.DeleteAll(ctx, characterID)` to clear all existing prepared slots.
2. Reset `sess.PreparedTechs` to an empty map.
3. For each level in merged `SlotsByLevel`, call `fillFromPreparedPool(ctx, lvl, slots, 0, mergedGrants, techReg, promptFn, characterID, prepRepo)`.
4. Store returned slots in `sess.PreparedTechs[lvl]`.

---

## Feature 2: `rest` command

Follows CMD-1 through CMD-7.

### CMD-1 / CMD-2: `commands.go`

```go
HandlerRest = "rest"
```

```go
Command{Handler: HandlerRest, Description: "Rest and rearrange your prepared technologies."}
```

### CMD-3: `internal/game/command/rest.go`

```go
// HandleRest handles the rest command. No arguments required.
func HandleRest(cmd Command, args []string, sess *session.PlayerSession) *CommandResult {
    return &CommandResult{Handler: HandlerRest}
}
```

### CMD-4: proto

Add to `api/proto/game/v1/game.proto`:

```protobuf
message RestRequest {}
```

Add to `ClientMessage` oneof:
```protobuf
RestRequest rest = <next_field_number>;
```

Run `make proto`.

### CMD-5: `bridge_handlers.go`

```go
func bridgeRest(cmd game.Command, args []string, sess *session.PlayerSession) (*gamev1.GameRequest, error) {
    return &gamev1.GameRequest{
        Message: &gamev1.GameRequest_Rest{
            Rest: &gamev1.RestRequest{},
        },
    }, nil
}
```

Register in `bridgeHandlerMap`: `game.HandlerRest: bridgeRest`.

### CMD-6: `handleRest` in `grpc_service.go`

```go
func (s *GameServiceServer) handleRest(
    ctx context.Context,
    sess *session.PlayerSession,
    stream gamev1.GameService_StreamServer,
) error
```

Behavior:
1. **Combat guard**: if `sess.CombatStatus != gamev1.CombatStatus_NONE`, send message `"You can't rest while in combat."` and return nil.
2. **Job lookup**: look up `sess.Class` in `s.jobRegistry`. If not found or `s.jobRegistry == nil`, send message `"You rest briefly but have no technologies to rearrange."` and return nil.
3. **promptFn**: construct from the player's own stream using `s.promptFeatureChoice`.
4. **Call `RearrangePreparedTechs`**: pass `sess`, `sess.CharacterID`, job, `s.techRegistry`, promptFn, `s.preparedTechRepo`.
5. On success: send message `"You finish your rest and your technologies are prepared."`.
6. On error: log at Warn, send message `"Something went wrong preparing your technologies."`, return nil (non-fatal).

Wire into the `dispatch` type switch on `*gamev1.GameRequest_Rest`.

---

## Testing

All tests use TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-RAR1** (property): For any valid combination of creation grants and level-up grants, total slot count per level after `RearrangePreparedTechs` equals `len(sess.PreparedTechs[level])` before the call.
- **REQ-RAR2**: Fixed entries always occupy indices 0..n-1 at each level after rearrangement; pool selections follow at n..m-1.
- **REQ-RAR3**: `LevelUpGrants` entries for levels above `sess.Level` are excluded from the pool; entries at or below `sess.Level` are included.
- **REQ-RAR4**: Job with no prepared grants (nil `Prepared` in all applicable grants) → `RearrangePreparedTechs` is a no-op; `DeleteAll` is never called.
- **REQ-RAR5**: Auto-assign fires when `len(pool at level) == open slots`; prompt is never invoked.
- **REQ-RAR6**: `handleRest` with player in combat sends the "can't rest" message; `RearrangePreparedTechs` is not called.
- **REQ-RAR7**: `handleRest` with player not in combat calls `RearrangePreparedTechs` and sends confirmation message.
- **TestAllCommandHandlersAreWired**: passes (CMD-5 compliance).

---

## Constraints

- No new DB tables or repo interfaces — `PreparedTechRepo.DeleteAll` + `Set` are sufficient.
- No new proto messages beyond `RestRequest`.
- Spontaneous tech reset and HP restoration are out of scope (covered by the separate Long Rest feature).
- `UsesByLevel` tracking is out of scope.
