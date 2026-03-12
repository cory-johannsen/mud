# Flee & Pursuit Design

**Date:** 2026-03-11
**Feature:** Flee command + NPC Pursuit

---

## Requirements

- FLEE-1: The `flee` command MUST require at least 1 AP; if the player has 0 AP the command MUST return an error.
- FLEE-2: The `flee` command MUST cost all remaining AP on use (minimum 1).
- FLEE-3: The flee skill check MUST use `d20 + max(skillRankBonus(sess.Skills["athletics"]), skillRankBonus(sess.Skills["acrobatics"]))`.
- FLEE-4: The flee DC MUST be `10 + highest StrMod among living NPC combatants in the room`.
- FLEE-5: On failure the player MUST remain in the room and combat MUST continue uninterrupted.
- FLEE-6: On success the player MUST be removed from the combat roster and their status set to idle.
- FLEE-7: On success the player MUST be moved to a random non-locked, non-hidden exit in the room.
- FLEE-8: If no valid exit exists, the player MUST still escape combat but remain in the current room; a descriptive message MUST be returned explaining there is nowhere to run.
- FLEE-9: On success, a fresh `RoomView` MUST be pushed to the player for the new room.
- FLEE-10: On success, a fresh `RoomView` MUST be pushed to all players remaining in the original room.
- FLEE-11: If no living players remain in the original room after the flee, `EndCombat` MUST be called on that room.

- PURSUIT-1: After a successful flee, each living NPC combatant MUST roll `d20 + StrMod` vs the player's flee total.
- PURSUIT-2: NPCs whose pursuit roll **meets or exceeds** the player's flee total MUST be moved to the destination room via `npcMgr` and MUST immediately initiate a new combat there.
- PURSUIT-3: NPCs whose pursuit roll is **less than** the player's flee total MUST remain in the original room.
- PURSUIT-4: If the player had no valid exit (FLEE-8), pursuit MUST NOT occur.
- PURSUIT-5: Pursuit combat MUST begin as a fresh combat (new initiative, new round) via the existing `StartCombat` path.
- PURSUIT-6: A single batch of narrative `CombatEvent` entries MUST be returned covering: flee outcome, each NPC's pursuit roll result, and which NPCs followed.

---

## 1. Skill Check Mechanics

### AP Cost

- FLEE-1 and FLEE-2 are enforced at the top of `CombatHandler.Flee`: call `SpendAllAP(uid)` only after confirming `playerCbt.AP >= 1`.

### Skill Roll

```
playerTotal = d20 + max(skillRankBonus(sess.Skills["athletics"]), skillRankBonus(sess.Skills["acrobatics"]))
```

`skillRankBonus` is the existing helper already used throughout `grpc_service.go`.

### DC

```
DC = 10 + max(NPC.StrMod for living NPC combatants in cbt)
```

NPCs have no discrete skill ranks; `StrMod` is their athletics-equivalent modifier. `bestNPCCombatant` already finds the highest-StrMod NPC and can be reused.

---

## 2. On Flee Success — Movement

1. Remove the player from the combat roster (`removeCombatant`).
2. Set `sess.Status = idle`.
3. Collect all non-locked, non-hidden exits from the current room (`room.VisibleExits()`).
4. If exits exist: pick one at random; update `sess.RoomID` to the destination; push a `RoomView` to the player for the new room.
5. If no exits exist: player stays in current room; narrative message explains there is nowhere to run; pursuit does not occur (FLEE-8, PURSUIT-4).
6. Push a fresh `RoomView` to all players remaining in the original room (FLEE-10).
7. If no living players remain in the original room, call `EndCombat` on it (FLEE-11).

---

## 3. NPC Pursuit

For each living NPC combatant (after player removal):

```
npcPursuitTotal = d20 + npc.StrMod
```

- If `npcPursuitTotal >= playerTotal`:
  - Move NPC instance to destination room via `npcMgr` (update `inst.RoomID`).
  - Initiate new combat in destination room via existing `StartCombat` path.
  - Append pursuit-success narrative event.
- Else:
  - NPC stays; append pursuit-fail narrative event.

All pursuit outcomes are bundled into the same `[]CombatEvent` slice returned by `Flee`.

---

## 4. Files Changed

| File | Change |
|---|---|
| `internal/gameserver/combat_handler.go` | Extend `Flee`: AP guard, skill-based roll, DC, movement, pursuit |
| `internal/gameserver/grpc_service.go` | Update `handleFlee` to push room views after successful flee |
| `internal/gameserver/grpc_service_flee_test.go` | New test file with all flee/pursuit integration tests |

No new proto messages, bridge handlers, or command constants are needed — the `FleeRequest` proto and `HandlerFlee` constant already exist.

---

## 5. Testing

All tests use `pgregory.net/rapid` for property-based coverage (SWENG-5a).

### `internal/gameserver/` package

- `TestHandleFlee_NotEnoughAP` — player has 0 AP; command returns error.
- `TestHandleFlee_Failure` — player roll < DC; player stays, combat continues.
- `TestHandleFlee_Success_NoValidExits` — no non-locked exits; player escapes combat but stays in room; pursuit does not occur.
- `TestHandleFlee_Success_NPCPursues` — NPC roll >= player total; NPC moves to destination room; new combat initiated.
- `TestHandleFlee_Success_NPCFails` — NPC roll < player total; NPC stays in original room.
- `TestHandleFlee_Success_OriginalCombatEnds` — no remaining players after flee; `EndCombat` called on original room.
- `TestProperty_Flee_SkillCheckBoundary` (property-based) — random roll and DC values; asserts correct success/fail outcome per FLEE-3/FLEE-4.

---

## 6. Completion Criterion

`mise run go test ./...` MUST pass with 0 failures before the feature is considered done.

**Reference patterns:** `grpc_service_grapple_test.go`, `grpc_service_climb_test.go`.
