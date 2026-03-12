# Flee & Pursuit Design

**Date:** 2026-03-11
**Feature:** Flee command + NPC Pursuit

---

## Requirements

- FLEE-1: The `flee` command MUST require at least 1 AP; if `playerCbt.AP < 1` the command MUST return an error. This check MUST occur before the skill roll (FLEE-3).
- FLEE-2: After confirming `q.RemainingPoints() >= 1` (where `q = cbt.ActionQueues[uid]`), the command MUST spend all remaining AP by calling `q.DeductAP(q.RemainingPoints())`. Do NOT call `h.SpendAP` — it acquires `combatMu`, which `Flee` already holds, causing a guaranteed deadlock. This inline deduction is safe because `combatMu` is held throughout. This MUST occur before the skill roll (FLEE-3).
- FLEE-3: The flee skill check MUST use `d20 + max(skillRankBonus(sess.Skills["athletics"]), skillRankBonus(sess.Skills["acrobatics"]))`. `skillRankBonus` is defined in `action_handler.go` (same `gameserver` package) and requires no move or duplication.
- FLEE-4: The flee DC MUST be `10 + highest StrMod among living NPC combatants in the combat`. `bestNPCCombatant` can be reused.
- FLEE-5: On failure the player MUST remain in the room and combat MUST continue uninterrupted.
- FLEE-6: On success the player MUST be removed from the combat roster via `removeCombatant` and their status set to idle.
- FLEE-7: On success the player MUST be moved to a randomly selected exit where `exit.Hidden == false && exit.Locked == false`, by iterating `room.Exits` and filtering both conditions. Do NOT rely on `room.VisibleExits()` alone — it only filters hidden exits, not locked ones.
- FLEE-8: If no valid exit exists, the player MUST still escape combat (FLEE-6 applies) but remain in the current room; a descriptive message MUST be returned; pursuit MUST NOT occur (PURSUIT-4).
- FLEE-9: On success with movement, `handleFlee` MUST push a fresh `RoomView` to the fleeing player for the new room. This MUST be done by inline-building: `rv := s.worldH.buildRoomView(uid, newRoom); sess.Entity.PushBlocking(data, 2*time.Second)`, following the same pattern as `pushRoomViewToAllInRoom`.
- FLEE-10: On success, `handleFlee` MUST call `s.pushRoomViewToAllInRoom(origRoomID)` to refresh all players remaining in the original room.
- FLEE-11: If no living players remain in the original room after the flee, `h.stopTimerLocked(origRoomID)`, `h.engine.EndCombat(origRoomID)`, and `h.onCombatEndFn(origRoomID)` (if non-nil) MUST all be called, matching the three-call pattern used throughout `combat_handler.go`. These MUST use `origRoomID`, not `sess.RoomID`.
- FLEE-12: `CombatHandler.Flee` signature MUST change to `func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, bool, error)` where the `bool` is `true` on flee success. The caller `handleFlee` MUST capture `origRoomID := sess.RoomID` before calling `Flee` (since `Flee` mutates `sess.RoomID` on success). `Flee` does NOT return `origRoomID`.
- FLEE-13: The existing opposed-roll logic in `CombatHandler.Flee` (`playerTotal > npcTotal`) MUST be replaced with the skill-check/DC logic in FLEE-3 and FLEE-4; the success condition MUST be `playerTotal >= DC`.

- PURSUIT-1: After a successful flee with movement, each living NPC combatant MUST roll `d20 + StrMod` vs the player's flee total (`playerTotal`). The loop variable iterating `cbt.Combatants` MUST be named `c` or similar — do NOT use `npc` as a loop variable, as it shadows the `npc` package import.
- PURSUIT-2: NPCs where `pursuitTotal >= playerTotal` MUST be moved via `h.npcMgr.Move(c.ID, destRoomID)`. Do NOT use `NPCHandler.MoveNPC` — `CombatHandler` holds no `NPCHandler` reference.
- PURSUIT-3: NPCs where `pursuitTotal < playerTotal` MUST remain in the original room.
- PURSUIT-4: If the player had no valid exit (FLEE-8), pursuit MUST NOT occur.
- PURSUIT-5: All pursuing NPC instances MUST be collected first; then a single new combat MUST be initiated in the destination room via a new helper `startPursuitCombatLocked(playerSess *session.PlayerSession, insts []*npc.Instance)`. This helper MUST:
  - (a) Resolve `destRoom` via `h.worldMgr.GetRoom(playerSess.RoomID)` (already updated to destination).
  - (b) Build `[]*combat.Combatant` for each pursuing NPC instance following the same pattern as `startCombatLocked` for a single NPC.
  - (c) Build the player `*combat.Combatant` following the same pattern as `startCombatLocked`.
  - (d) Call `h.engine.StartCombat(playerSess.RoomID, combatants, h.condRegistry, h.scriptMgr, zoneID)` exactly once, where `zoneID` is resolved from the destination room via `h.worldMgr.GetRoom(playerSess.RoomID)` then `room.ZoneID`. Handle the `(*Combat, error)` return — return the error if non-nil.
  - (e) Call `cbt.StartRound(3)` exactly once.
  - (f) Return `[]*gamev1.CombatEvent` init events for deferred broadcasting (do NOT call `broadcastFn` inside this helper while `combatMu` is held, to avoid deadlock). The caller (`Flee`) MUST append these to the returned events slice so they are broadcast by `handleFlee` after the lock is released.
  - (g) Call `h.startTimerLocked(playerSess.RoomID)`.
  - Calling `startCombatLocked` in a loop MUST NOT be used.
- PURSUIT-6: A single batch of narrative `CombatEvent` entries MUST be returned covering: flee outcome, each NPC's pursuit roll result, and which NPCs followed.

---

## 1. Skill Check Mechanics

### AP Cost

FLEE-1 and FLEE-2 at the top of `CombatHandler.Flee`, before any roll. Do NOT call `h.SpendAP` (it acquires `combatMu`, deadlocking). Use the action queue directly:

```go
q, ok := cbt.ActionQueues[uid]
if !ok || q.RemainingPoints() < 1 {
    return nil, false, fmt.Errorf("you need at least 1 AP to flee")
}
_ = q.DeductAP(q.RemainingPoints()) // spend all remaining AP
```

### Skill Roll

```go
roll, _ := h.dice.RollExpr("d20")
athleticsBonus := skillRankBonus(sess.Skills["athletics"])
acrobaticsBonus := skillRankBonus(sess.Skills["acrobatics"])
bonus := athleticsBonus
if acrobaticsBonus > athleticsBonus {
    bonus = acrobaticsBonus
}
playerTotal := roll.Total() + bonus
```

### DC

```go
bestNPC := h.bestNPCCombatant(cbt)
dc := 10
if bestNPC != nil {
    dc = 10 + bestNPC.StrMod
}
```

### Return Signature Change

```go
// Before:
func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, error)
// After:
func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, bool, error)
```

---

## 2. On Flee Success — Movement

1. `removeCombatant(cbt, uid)`; set `sess.Status = idle`.
2. Build valid exits:
   ```go
   var validExits []world.Exit
   for _, e := range room.Exits {
       if !e.Hidden && !e.Locked {
           validExits = append(validExits, e)
       }
   }
   ```
3. If `len(validExits) > 0`: pick one at random; `sess.RoomID = exit.TargetRoom`; record `destRoomID = sess.RoomID`.
4. If `len(validExits) == 0`: return flee-success events with "nowhere to run" narrative; `fled = true`; `destRoomID = ""`; skip pursuit.
5. If no living players remain in the original room, call the three-call end-combat sequence using `origRoomID` (FLEE-11).
6. Return `(events, true, nil)`. Room view pushes (FLEE-9, FLEE-10) are done in `handleFlee`.

---

## 3. NPC Pursuit

After player moves (only when `destRoomID != ""`), still holding `combatMu`:

```go
var pursuers []*npc.Instance
for _, c := range cbt.Combatants {
    if c.Kind != combat.KindNPC || c.IsDead() {
        continue
    }
    inst, ok := h.npcMgr.Get(c.ID)
    if !ok {
        continue
    }
    pursuitRoll, _ := h.dice.RollExpr("d20")
    pursuitTotal := pursuitRoll.Total() + c.StrMod
    if pursuitTotal >= playerTotal {
        h.npcMgr.Move(c.ID, destRoomID)
        pursuers = append(pursuers, inst)
        events = append(events, narrativeEvent(c.Name + " gives chase!"))
    } else {
        events = append(events, narrativeEvent(c.Name + " can't keep up."))
    }
}
if len(pursuers) > 0 {
    initEvents := h.startPursuitCombatLocked(playerSess, pursuers)
    events = append(events, initEvents...)
}
```

`broadcastFn` is NOT called inside `startPursuitCombatLocked`; its init events are returned and appended so `handleFlee` broadcasts them all after the gRPC call returns.

---

## 4. Event Broadcasting in `handleFlee`

`handleFlee` captures `origRoomID := sess.RoomID` before calling `Flee`, then:

```go
events, fled, err := s.combatH.Flee(uid)
// broadcast all events to the original room
for _, evt := range events {
    s.broadcastCombatEvent(origRoomID, uid, evt)
}
if fled {
    // push room view to remaining players in original room
    s.pushRoomViewToAllInRoom(origRoomID)
    // push room view to fleeing player in new room
    if sess, ok := s.sessions.GetPlayer(uid); ok {
        if newRoom, ok := s.world.GetRoom(sess.RoomID); ok {
            rv := s.worldH.buildRoomView(uid, newRoom)
            evt := &gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoomView{RoomView: rv}}
            data, _ := proto.Marshal(evt)
            _ = sess.Entity.PushBlocking(data, 2*time.Second)
        }
    }
}
```

Return the first event to the calling player as the direct response, as the existing `handleFlee` does.

---

## 5. Files Changed

| File | Change |
|---|---|
| `internal/gameserver/combat_handler.go` | Replace old opposed-roll in `Flee`; add AP guard; skill roll; DC; movement; pursuit; change return to `(events, fled, error)`; add `startPursuitCombatLocked` helper |
| `internal/gameserver/grpc_service.go` | Update `handleFlee` to `(events, fled, err)`; broadcast all events; push room views per §4 |
| `internal/gameserver/grpc_service_flee_test.go` | New integration tests |

No new proto, bridge handlers, or command constants needed — `FleeRequest` and `HandlerFlee` already exist.

---

## 6. Testing

All tests use `pgregory.net/rapid` (SWENG-5a).

### `internal/gameserver/` package

- `TestHandleFlee_NotEnoughAP` — player has 0 AP; returns error.
- `TestHandleFlee_Failure` — player roll < DC; player stays, combat continues.
- `TestHandleFlee_Success_NoValidExits` — all exits locked/hidden; player escapes combat but stays in room; pursuit does not occur.
- `TestHandleFlee_Success_NPCPursues` — NPC pursuit roll >= playerTotal; NPC moves to destination room; new combat initiated.
- `TestHandleFlee_Success_NPCFails` — NPC pursuit roll < playerTotal; NPC stays in original room.
- `TestHandleFlee_Success_OriginalCombatEnds` — no remaining players after flee; end-combat sequence called on original room.
- `TestProperty_Flee_SkillCheckBoundary` (property-based) — random roll and DC; correct success/fail per FLEE-3/FLEE-4.
- `TestProperty_Pursuit_RollOutcome` (property-based) — random `pursuitTotal` and `playerTotal`; NPC pursues iff `pursuitTotal >= playerTotal`.
- `TestProperty_Flee_ExitSelection` (property-based) — random room exit configs; selected destination always satisfies `Hidden == false && Locked == false`, or no movement when no qualifying exit exists.

---

## 7. Completion Criterion

`mise run go test ./...` MUST pass with 0 failures before the feature is considered done.

**Reference patterns:** `grpc_service_grapple_test.go`, `grpc_service_trip_test.go`.
