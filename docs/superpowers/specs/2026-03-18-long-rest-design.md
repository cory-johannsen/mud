# Long Rest Design

**Date:** 2026-03-18
**Sub-project:** Long Rest

---

## Goal

Extend the existing `rest` command to restore the player's HP to maximum, persisting the change to the database.

---

## Context

The `rest` command (`handleRest` in `internal/gameserver/grpc_service.go`) already:
- Blocks rest while in combat
- Restores all spontaneous use pools via `spontaneousUsePoolRepo.RestoreAll()`
- Restores all innate tech use slots via `innateTechRepo.RestoreAll()`
- Allows rearrangement of prepared tech slots via `RearrangePreparedTechs()`

The only gap is HP restoration. `sess.CurrentHP` and `sess.MaxHP` are available on `PlayerSession`, and `s.charSaver.SaveState(ctx context.Context, id int64, location string, currentHP int) error` is the established pattern for persisting HP.

---

## Design

### REQ-LR1
`handleRest` MUST set `sess.CurrentHP = sess.MaxHP` as the first action after the combat guard, before any tech restoration or early-return paths (including the `jobRegistry == nil` guard and job-not-found guard). HP restore fires unconditionally once the combat guard passes — even if the player has no technologies to rearrange.

### REQ-LR2
`handleRest` MUST call `s.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, sess.CurrentHP)` immediately after setting `sess.CurrentHP`, when `s.charSaver != nil`. If `SaveState` returns an error, `handleRest` MUST return that error to the caller (same pattern as other `SaveState` call sites).

### REQ-LR3
The final success message (returned at the end of the happy path, after `RearrangePreparedTechs`) MUST indicate both HP restoration and tech preparation: e.g. `"You finish your rest. HP restored to maximum and technologies prepared."` The early-return messages (no job/no techs) MUST also mention HP: e.g. `"You rest and recover to full HP."`.

### REQ-LR4
If `s.charSaver` is nil (test/no-persistence mode), HP is still updated in memory (`sess.CurrentHP = sess.MaxHP`) and no error is returned.

### REQ-LR5
The combat guard MUST remain: rest is rejected while `sess.Status == int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)`. HP MUST NOT be modified when the combat guard fires.

---

## Files Changed

| File | Change |
|------|--------|
| `internal/gameserver/grpc_service.go` | Add HP restore + `charSaver.SaveState` call in `handleRest`; update success/early-return messages |
| `internal/gameserver/grpc_service_rest_test.go` | Extend existing test file: add REQ-LR1–LR5 tests; update `TestHandleRest_NotInCombat_Rearranges` to assert the new message text |

No schema changes. No new commands. No new files beyond the test file.

---

## Testing

Property-based tests (SWENG-5a) using `pgregory.net/rapid`:

- **REQ-LR1 (property)**: For any `CurrentHP` in `[0, MaxHP]` (including `CurrentHP == MaxHP` — idempotent), after `rest`, `sess.CurrentHP == sess.MaxHP`.
- **REQ-LR2 (property)**: For any `CurrentHP` in `[0, MaxHP]`, `charSaver.SaveState` is called exactly once with `currentHP == MaxHP`.
- **REQ-LR3 (unit)**: Success message contains `"HP"` and `"prepared"`. Early-return message (no job) contains `"HP"`.
- **REQ-LR4 (unit)**: Rest with nil `charSaver` succeeds and `sess.CurrentHP == sess.MaxHP`.
- **REQ-LR5 (unit)**: Rest while in combat returns an error message; `sess.CurrentHP` is unchanged.
- **REQ-LR2 error path (unit)**: When `charSaver.SaveState` returns an error, `handleRest` returns that error.

---

## Non-Goals

- In-game time tracking (deferred)
- Safe-room enforcement (deferred)
- Condition removal on rest (deferred)
- A separate `longrest` command (not needed; `rest` is sufficient per current scope)
