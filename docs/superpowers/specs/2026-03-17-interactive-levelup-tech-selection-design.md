# Interactive Level-Up Technology Selection — Design Spec

**Date:** 2026-03-17

---

## Goal

When a player levels up and their job's `LevelUpGrants` include technology pool choices, they are prompted interactively to select their technologies. Grants that require no player input (hardwired, fixed, auto-assign) are applied immediately with a console notification. Grants requiring player choice are deferred to the next login or resolved on demand via a `selecttech` command. The character sheet reflects pending selections.

---

## Context

`LevelUpTechnologies` in `internal/gameserver/technology_assignment.go` already supports interactive prompting via `TechPromptFn`. When `handleGrant` detects a level-up it calls `LevelUpTechnologies` with `nil` promptFn (auto-assign all) because the grant runs in the admin's stream context, not the target player's. This spec adds the deferred-selection path on top of the existing infrastructure.

**Prerequisites:** The Prepared Tech Rearrangement sprint (2026-03-16) must be merged first (`RearrangePreparedTechs`, `rest` command, and setter methods on `GameServiceServer` are all required).

---

## Feature 1: `partitionTechGrants`

New function in `internal/gameserver/technology_assignment.go`:

```go
// partitionTechGrants splits grants into two parts:
//   - immediate: hardwired, fixed prepared/spontaneous, and any level/type where
//     pool size <= open slots (auto-assign fires; no player choice needed)
//   - deferred: levels/types where pool size > open slots (player must choose)
//
// Precondition: grants is non-nil and valid.
// Postcondition: immediate + deferred together cover all grants in the input.
func partitionTechGrants(grants *ruleset.TechnologyGrants) (immediate, deferred *ruleset.TechnologyGrants)
```

### Partition rules

For **hardwired**: always immediate (no choice possible).

For **prepared** (per tech level in `SlotsByLevel`):
- Count fixed entries at this tech level → `nFixed`
- Count pool entries at this tech level → `nPool`
- Open slots = `slots - nFixed`
- If `nPool <= open` → immediate (auto-assign)
- If `nPool > open` → deferred

For **spontaneous** (per tech level in `KnownByLevel`):
- Same logic substituting `KnownByLevel` and spontaneous pool/fixed entries.

The returned `immediate` grant contains all hardwired entries, all fixed entries, and all prepared/spontaneous levels where auto-assign fires. The returned `deferred` grant contains only the prepared/spontaneous levels where the player must choose. Either may be nil if empty.

---

## Feature 2: `ResolvePendingTechGrants`

New function in `internal/gameserver/technology_assignment.go`:

```go
// ResolvePendingTechGrants interactively resolves all pending tech grants for a session.
// For each entry in sess.PendingTechGrants (ascending level order), calls LevelUpTechnologies
// with a live promptFn. Removes each entry after successful resolution.
//
// Precondition: sess, promptFn, and all repos are non-nil.
// Postcondition: sess.PendingTechGrants is empty on full success; partially cleared on error.
func ResolvePendingTechGrants(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    job *ruleset.Job,
    techReg *technology.Registry,
    promptFn TechPromptFn,
    hwRepo HardwiredTechRepo,
    prepRepo PreparedTechRepo,
    spontRepo SpontaneousTechRepo,
    innateRepo InnateTechRepo,
) error
```

Iterates `sess.PendingTechGrants` in ascending key (character level) order. For each level, calls `LevelUpTechnologies` with the stored grants and the provided `promptFn`. On success, deletes the entry from `sess.PendingTechGrants`. Returns the first error encountered (remaining levels stay pending).

---

## Feature 3: `PendingTechGrants` on `PlayerSession`

Add to `internal/game/session/manager.go` `PlayerSession` struct:

```go
// PendingTechGrants maps character level to the technology grants that require
// interactive player selection (pool > open slots). Populated at level-up;
// cleared by ResolvePendingTechGrants.
PendingTechGrants map[int]*ruleset.TechnologyGrants
```

**Import note:** `internal/game/session/manager.go` does not currently import `internal/game/ruleset`. Before adding this field, verify no import cycle exists (`ruleset` must not import `session`). A quick `grep -r "session" internal/game/ruleset/` confirms `ruleset` does not import `session`, so the import is safe.

### Persistence: `character_pending_tech_levels` table

`PendingBoosts` is persisted in `character_pending_boosts` and `PendingSkillIncreases` in a similar table. For consistency, pending tech levels must also survive server restarts.

Persist only the **list of character levels** with pending grants (not the grants themselves — those are deterministically re-derived from `job.LevelUpGrants[lvl]` at login):

New repo method on `ProgressRepo` interface in `internal/gameserver/grpc_service.go`:

```go
GetPendingTechLevels(ctx context.Context, id int64) ([]int, error)
SetPendingTechLevels(ctx context.Context, id int64, levels []int) error
```

New DB table (migration in `internal/storage/postgres/testdata/`):

```sql
CREATE TABLE character_pending_tech_levels (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    level        INT    NOT NULL,
    PRIMARY KEY (character_id, level)
);
```

**At level-up** (in `handleGrant`): when deferred grants are stored in `sess.PendingTechGrants[lvl]`, also call `progressRepo.SetPendingTechLevels(ctx, characterID, pendingLevels)` to persist the updated list.

**At login** (in `Session`): load `GetPendingTechLevels` → reconstruct `sess.PendingTechGrants[lvl] = job.LevelUpGrants[lvl]` for each persisted level. If `job.LevelUpGrants[lvl]` is nil (job YAML changed), skip that level silently.

**After resolution** (in `ResolvePendingTechGrants`): after clearing each entry from `sess.PendingTechGrants`, call `SetPendingTechLevels` to update the persisted list.

---

## Feature 4: `handleGrant` changes

In `internal/gameserver/grpc_service.go`, replace the existing `LevelUpTechnologies` call block with:

1. Call `partitionTechGrants(techGrants)` → `immediate`, `deferred`
2. If `immediate != nil`: call `LevelUpTechnologies(ctx, target, ..., nil, ...)` (auto-assign)
   - Collect newly assigned tech names (compare `target.HardwiredTechs`, `target.PreparedTechs`, `target.SpontaneousTechs` before and after)
   - For each newly assigned tech, push notification to target entity: `"You gained [tech name] (auto-assigned)."`
3. If `deferred != nil`:
   - Store in `target.PendingTechGrants[lvl] = deferred`
4. After processing all levels: if `len(target.PendingTechGrants) > 0`, push notification: `"You have pending technology selections! Type 'selecttech' to choose your technologies."`

**Collecting auto-assigned tech names:** snapshot the relevant session fields before calling `LevelUpTechnologies`, diff against after. For hardwired: new entries in `HardwiredTechs` slice. For prepared: new entries in `PreparedTechs` map values. For spontaneous: new entries in `SpontaneousTechs` map values.

---

## Feature 5: Login-time resolution

In `internal/gameserver/grpc_service.go` `Session` function, after the existing ability-boost prompts and before `commandLoop`, add:

```go
if len(sess.PendingTechGrants) > 0 {
    if job, ok := s.jobRegistry.Job(sess.Class); ok {
        promptFn := func(options []string) (string, error) {
            choices := &ruleset.FeatureChoices{
                Prompt:  "Choose a technology:",
                Options: options,
                Key:     "tech_choice",
            }
            return s.promptFeatureChoice(stream, "tech_choice", choices)
        }
        if err := ResolvePendingTechGrants(ctx, sess, characterID,
            job, s.techRegistry, promptFn,
            s.hardwiredTechRepo, s.preparedTechRepo,
            s.spontaneousTechRepo, s.innateTechRepo,
        ); err != nil {
            s.logger.Warn("login: ResolvePendingTechGrants failed", zap.Error(err))
        }
    }
}
```

---

## Feature 6: `selecttech` command (CMD-1–7)

Follows the same CMD-1–7 pattern as `rest`.

### CMD-1 / CMD-2: `commands.go`

```go
HandlerSelectTech = "selecttech"
```

```go
Command{Handler: HandlerSelectTech, Description: "Select pending technology upgrades from levelling up."}
```

### CMD-3: `internal/game/command/selecttech.go`

```go
func HandleSelectTech(cmd Command, args []string, sess *session.PlayerSession) *CommandResult {
    return &CommandResult{Handler: HandlerSelectTech}
}
```

### CMD-4: proto

Add to `api/proto/game/v1/game.proto`:

```protobuf
message SelectTechRequest {}
```

Add to `ClientMessage` oneof:
```protobuf
SelectTechRequest select_tech = 82;
```

(Current highest is `RestRequest rest = 81`. Verify before implementing.)

Run `make proto`.

### CMD-5: `bridge_handlers.go`

```go
func bridgeSelectTech(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_SelectTech{SelectTech: &gamev1.SelectTechRequest{}},
    }}, nil
}
```

Register: `command.HandlerSelectTech: bridgeSelectTech`

### CMD-6: `handleSelectTech` in `grpc_service.go`

Pre-dispatch stream handler (like `handleRest`):

```go
func (s *GameServiceServer) handleSelectTech(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return fmt.Errorf("handleSelectTech: player %q not found", uid)
    }

    sendMsg := func(text string) error {
        return stream.Send(&gamev1.ServerEvent{
            RequestId: requestID,
            Payload:   &gamev1.ServerEvent_Message{Message: &gamev1.MessageEvent{Content: text}},
        })
    }

    if len(sess.PendingTechGrants) == 0 {
        return sendMsg("You have no pending technology selections.")
    }

    if s.jobRegistry == nil {
        return sendMsg("You have no pending technology selections.")
    }
    job, ok := s.jobRegistry.Job(sess.Class)
    if !ok {
        return sendMsg("You have no pending technology selections.")
    }

    promptFn := func(options []string) (string, error) {
        choices := &ruleset.FeatureChoices{
            Prompt:  "Choose a technology:",
            Options: options,
            Key:     "tech_choice",
        }
        return s.promptFeatureChoice(stream, "tech_choice", choices)
    }

    if err := ResolvePendingTechGrants(stream.Context(), sess, sess.CharacterID,
        job, s.techRegistry, promptFn,
        s.hardwiredTechRepo, s.preparedTechRepo,
        s.spontaneousTechRepo, s.innateTechRepo,
    ); err != nil {
        s.logger.Warn("handleSelectTech failed", zap.String("uid", uid), zap.Error(err))
        return sendMsg("Something went wrong selecting your technologies.")
    }

    return sendMsg("Your technology selections are complete.")
}
```

Wire in `commandLoop` pre-dispatch:

```go
if _, ok := msg.Payload.(*gamev1.ClientMessage_SelectTech); ok {
    if err := s.handleSelectTech(uid, msg.RequestId, stream); err != nil {
        // send error event
    }
    continue
}
```

### CMD-7: all tests pass including `TestAllCommandHandlersAreWired`

---

## Feature 7: Character sheet

In the character sheet rendering, add display of pending tech selections. Find where `PendingBoosts` and `PendingSkillIncreases` are rendered in the character sheet output (in `internal/frontend/` or `internal/gameserver/`) and add:

```
Pending Tech Selections: N
```

when `len(sess.PendingTechGrants) > 0`.

---

## Testing

All tests use TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-ILT1**: When level-up grants have pool choices (pool > open slots), `PendingTechGrants` is populated for those levels; `LevelUpTechnologies` is not called for deferred levels at grant time.
- **REQ-ILT2**: When level-up grants are hardwired/fixed/auto-assign only (pool <= open slots), `PendingTechGrants` remains empty; tech is applied immediately.
- **REQ-ILT3**: Auto-assigned tech pushes a notification containing the tech name to the target's entity stream.
- **REQ-ILT4**: When deferred grants are stored, a "Type 'selecttech'" notification is pushed to the target's entity stream.
- **REQ-ILT5**: `ResolvePendingTechGrants` with a live stream prompts for each pending level, calls `LevelUpTechnologies`, and clears each entry after resolution.
- **REQ-ILT6**: `handleSelectTech` with no pending grants sends "no pending technology selections."
- **REQ-ILT7** (property): For any combination of grants with pool choices, after `ResolvePendingTechGrants` all chosen tech IDs are valid pool members.
- **REQ-ILT8**: `Session` login path with pending grants resolves them before the main command loop.
- **REQ-ILT9**: Character sheet includes "Pending Tech Selections: N" when N > 0.
- **TestAllCommandHandlersAreWired**: passes.

---

## Constraints

- One new DB table (`character_pending_tech_levels`) and two new repo methods — consistent with how `PendingBoosts` and `PendingSkillIncreases` are persisted.
- No new proto messages beyond `SelectTechRequest`.
- Use count decrement is out of scope.
