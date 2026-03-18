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

The only gap is HP restoration. `sess.CurrentHP` and `sess.MaxHP` are available on `PlayerSession`, and `s.charSaver.SaveState(ctx, id, location, currentHP)` is the established pattern for persisting HP.

---

## Design

### REQ-LR1
`handleRest` MUST set `sess.CurrentHP = sess.MaxHP` before any tech restoration steps.

### REQ-LR2
`handleRest` MUST call `s.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, sess.CurrentHP)` immediately after updating HP, when `s.charSaver != nil`.

### REQ-LR3
The success message MUST indicate both HP restoration and tech preparation: e.g. `"You finish your rest. HP restored to maximum and technologies prepared."`.

### REQ-LR4
If `s.charSaver` is nil (test/no-persistence mode), HP is still updated in memory and no error is returned.

### REQ-LR5
The combat guard MUST remain: rest is rejected while `sess.Status == COMBAT_STATUS_IN_COMBAT`.

---

## Files Changed

| File | Change |
|------|--------|
| `internal/gameserver/grpc_service.go` | Add HP restore + `charSaver.SaveState` call in `handleRest` |
| `internal/gameserver/grpc_service_rest_test.go` | New or extended test file covering REQ-LR1–LR5 |

No schema changes. No new commands. No new files beyond the test file.

---

## Testing

Property-based tests (SWENG-5a) using `pgregory.net/rapid`:

- **REQ-LR1/LR2**: Property — for any `CurrentHP` in `[0, MaxHP)`, after `rest`, `sess.CurrentHP == sess.MaxHP` and `charSaver` received `SaveState` with `MaxHP`.
- **REQ-LR3**: Unit — success message contains "HP" and "prepared" (or equivalent).
- **REQ-LR4**: Unit — rest with nil `charSaver` succeeds and `sess.CurrentHP == sess.MaxHP`.
- **REQ-LR5**: Unit — rest while in combat returns error message, HP unchanged.

---

## Non-Goals

- In-game time tracking (deferred)
- Safe-room enforcement (deferred)
- Condition removal on rest (deferred)
- A separate `longrest` command (not needed; `rest` is sufficient per current scope)
