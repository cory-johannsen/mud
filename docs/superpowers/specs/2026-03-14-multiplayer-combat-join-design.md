# Multiplayer Combat Join — Design Spec

**Date:** 2026-03-14

---

## Goal

When a player enters a room with active combat, offer them a yes/no prompt to join. On accept, insert them as a full combatant with a fresh initiative roll. XP, currency, and item loot are split equally (or round-robin) among all participants.

---

## Feature 1: Data Model

### PlayerSession — new field

```go
// PendingCombatJoin holds the RoomID of a combat the player has been invited to join.
// Empty string means no pending join offer.
PendingCombatJoin string
```

Location: `internal/game/session/manager.go`, `PlayerSession` struct.

### Combat — new field

```go
// Participants is the ordered list of player UIDs who were ever active combatants
// in this encounter. Used for XP and loot distribution. Never shrunk after join.
Participants []string
```

Location: `internal/game/combat/engine.go`, `Combat` struct.

Populated when a player combatant is added — both at `StartCombat` time and via `AddCombatant`. A player who joins mid-fight and then dies still counts for the split.

### Engine — new method

```go
// AddCombatant inserts c into the combat for roomID in initiative order.
//
// Precondition: a Combat for roomID exists; c.Initiative has already been rolled.
// Postcondition: c appears in cbt.Combatants sorted by initiative descending;
//   c.ID appended to cbt.Participants if c.Kind == KindPlayer;
//   c.ID added to cbt.ActionQueues and cbt.Conditions.
func (e *Engine) AddCombatant(roomID string, c *Combatant) error
```

Location: `internal/game/combat/engine.go`.

---

## Feature 2: Join Flow

### Trigger — room entry

**Location:** `internal/gameserver/combat_handler.go`, in the post-move hook (called after a player successfully moves rooms).

**Precondition:** Player has just entered `roomID`; player is not already a combatant in that room's combat.

**Algorithm:**
```
if h.engine.GetCombat(newRoomID) exists AND player not already in cbt.Combatants:
    sess.PendingCombatJoin = newRoomID
    send player: "Active combat in progress. Join the fight? (join / decline)"
```

### `join` command (CMD-1–7)

- **HandlerJoin** constant: `"join"`
- **BuiltinCommands entry:** `{Name: "join", Help: "Join active combat in the current room.", Category: CategoryCombat, Handler: HandlerJoin}`
- **Proto:** `message JoinRequest {}` added to `game.proto`; wired as `JoinRequest join = 73` in `ClientMessage` oneof; `make proto` run.
- **bridgeJoin:** no args; emits `ClientMessage_Join{Join: &gamev1.JoinRequest{}}`.
- **handleJoin:**
  1. If `sess.PendingCombatJoin == ""`: return `"No combat to join."`.
  2. Look up combat for `sess.PendingCombatJoin`; if gone (ended), clear flag, return `"The combat has ended."`.
  3. Build `*combat.Combatant` from session (same pattern as `startCombatLocked`).
  4. Roll initiative: `combat.RollInitiative([]*Combatant{playerCbt}, h.dice.Src())`.
  5. Call `h.engine.AddCombatant(sess.PendingCombatJoin, playerCbt)`.
  6. Set `sess.Status = statusInCombat`; clear `sess.PendingCombatJoin`.
  7. Broadcast to room: `"<CharName> joins the fight!"`.
  8. Return narrative: `"You join the combat (initiative %d)."`.

### `decline` command (CMD-1–7)

- **HandlerDecline** constant: `"decline"`
- **BuiltinCommands entry:** `{Name: "decline", Help: "Decline to join active combat.", Category: CategoryCombat, Handler: HandlerDecline}`
- **Proto:** `message DeclineRequest {}` wired as `DeclineRequest decline = 74`.
- **bridgeDecline:** emits `ClientMessage_Decline`.
- **handleDecline:**
  1. If `sess.PendingCombatJoin == ""`: return `"Nothing to decline."`.
  2. Clear `sess.PendingCombatJoin`.
  3. Return `"You stay back and watch."`.

### Combat ends while player is pending

**Location:** combat-end cleanup in `combat_handler.go` (where `cbt.Over` is set and players' statuses are reset).

**Algorithm:**
```
for each session in h.sessions.AllPlayers():
    if sess.PendingCombatJoin == endedRoomID:
        sess.PendingCombatJoin = ""
        send player: "The combat has ended."
```

---

## Feature 3: XP and Loot Split

### Participants tracking

At `StartCombat` time, populate `cbt.Participants` with the UIDs of all player combatants in the initial list. In `AddCombatant`, append the new player's UID if `c.Kind == KindPlayer`.

### XP split

**Location:** `removeDeadNPCsLocked` in `combat_handler.go`, replacing the `firstLivingPlayer` XP award.

**Algorithm:**
```
livingParticipants := living players whose UID is in cbt.Participants
if len(livingParticipants) == 0: skip
share := (inst.Level * cfg.Awards.KillXPPerNPCLevel) / len(livingParticipants)
if share == 0: share = 1
for each p in livingParticipants:
    AwardKill(ctx, p, inst.Level, p.CharacterID)
    announce XP to p
```

REQ-XP1: Integer division; remainder discarded.
REQ-XP2: If all participants are dead, no XP is awarded.
REQ-XP3: Single-participant case is identical to current behavior.

### Currency split

**Location:** same `removeDeadNPCsLocked` block, replacing the `firstLivingPlayer` currency award.

**Algorithm:**
```
totalCurrency := result.Currency + inst.Currency
inst.Currency = 0
if totalCurrency == 0: skip
livingParticipants := living players in cbt.Participants
if len(livingParticipants) == 0: skip
share := totalCurrency / len(livingParticipants)  // integer division
if share == 0 && totalCurrency > 0: share = 1     // guarantee at least 1 to first player
for each p in livingParticipants:
    p.Currency += share
    SaveCurrency(ctx, p.CharacterID, p.Currency)
```

REQ-CURR1: Remainder discarded (not re-distributed).
REQ-CURR2: Single-participant: identical to current behavior.

### Item split — round-robin by initiative

**Location:** same block, replacing the floor-drop loop.

**Algorithm:**
```
livingParticipants := living players in cbt.Participants, ordered by initiative descending
                      (same order as cbt.Combatants)
for i, lootItem in enumerate(result.Items):
    recipient := livingParticipants[i % len(livingParticipants)]
    if recipient has inventory capacity:
        add lootItem to recipient inventory
    else:
        h.floorMgr.Drop(roomID, lootItem)
```

REQ-ITEM1: If no living participants, all items drop to floor.
REQ-ITEM2: Single-participant: identical to current behavior (items go to that player or floor).

---

## Testing

- REQ-T1 (example): Player enters room with active combat → `sess.PendingCombatJoin == roomID`; player receives join prompt.
- REQ-T2 (example): Player sends `join` → added to `cbt.Combatants` at correct initiative position; `sess.Status == statusInCombat`; `PendingCombatJoin == ""`.
- REQ-T3 (example): Player sends `decline` → `PendingCombatJoin == ""`; player not in combat.
- REQ-T4 (example): Combat ends while player is pending → `PendingCombatJoin` cleared; player receives "combat has ended" message.
- REQ-T5 (example): `AddCombatant` inserts combatant in correct initiative-sorted position.
- REQ-T6 (example): `handleJoin` when `PendingCombatJoin == ""` returns "No combat to join."
- REQ-T7 (example): Two players in combat; NPC dies → each receives `floor(totalXP/2)` XP.
- REQ-T8 (example): Two players in combat; NPC drops 10 currency → each receives 5.
- REQ-T9 (example): Two players in combat; NPC drops 3 items → player A gets items 0 and 2, player B gets item 1 (round-robin by initiative).
- REQ-T10 (property): For any `n` living participants and `totalCurrency` in [0,10000], each share is `floor(totalCurrency/n)`; no participant receives more than their share.
- REQ-T11 (property): For any `n` living participants and `m` items, each item goes to exactly one recipient; total items distributed == m.
- REQ-T12 (example): Single participant — XP, currency, and items behave identically to pre-feature behavior.
