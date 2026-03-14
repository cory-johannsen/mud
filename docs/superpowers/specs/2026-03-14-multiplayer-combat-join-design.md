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
//   cbt.Conditions[c.ID] initialized to a new condition.ActiveSet.
//   NOTE: ActionQueues is NOT populated here — StartRound rebuilds the entire
//   ActionQueues map each round via make(map[string]*ActionQueue), so the new
//   combatant's queue will appear naturally at the start of the next round.
//
// Locking: AddCombatant acquires e.mu (write lock) for the full duration of
//   the method (map lookup + Combatants slice modification + Participants append
//   + Conditions initialization), consistent with StartCombat. The caller must
//   NOT hold e.mu. The handler-level combatMu may be held by the caller.
//   e.mu is the guard for all Combat struct field mutations within Engine methods.
//
// Insertion algorithm: find the insertion index i such that all combatants at
//   positions 0..i-1 have initiative >= c.Initiative, then insert c at index i
//   (a single-position slice expansion, not a full re-sort). This preserves the
//   relative order of all existing combatants including dead ones.
//
// turnIndex: If insertion index i <= cbt.turnIndex, cbt.turnIndex is incremented
//   by 1 to preserve the identity of the current actor.
func (e *Engine) AddCombatant(roomID string, c *Combatant) error
```

Location: `internal/game/combat/engine.go`.

---

## Feature 2: Join Flow

### Trigger — room entry

**Location:** `internal/gameserver/combat_handler.go`, in the post-move hook (called after a player successfully moves rooms).

**Precondition:** Player has just entered `roomID`; player is not already a combatant in that room's combat; `sess.Status != statusInCombat` (player is not already in a different active combat).

**Synchronization:** The post-move hook runs while `combatMu` is held. All reads and writes to `sess.PendingCombatJoin` and `sess.Status` occur under `combatMu`. `handleJoin` and `handleDecline` must acquire `combatMu` before reading or writing `PendingCombatJoin`.

**Algorithm:**
```
if sess.Status != statusInCombat
   AND h.engine.GetCombat(newRoomID) exists
   AND player not already in cbt.Combatants:
    // Overwrite any previous pending join offer unconditionally;
    // no notification is sent for the overridden offer.
    sess.PendingCombatJoin = newRoomID
    send player: "Active combat in progress. Join the fight? (join / decline)"
```

### `join` command (CMD-1–7)

- **HandlerJoin** constant: `"join"`
- **BuiltinCommands entry:** `{Name: "join", Help: "Join active combat in the current room.", Category: CategoryCombat, Handler: HandlerJoin}`
- **Proto:** `message JoinRequest {}` added to `game.proto`; wired as `JoinRequest join = 73` in `ClientMessage` oneof.
- **bridgeJoin:** no args; emits `ClientMessage_Join{Join: &gamev1.JoinRequest{}}`.
- **handleJoin:**
  1. If `sess.PendingCombatJoin == ""`: return `"No combat to join."`.
  2. Look up combat for `sess.PendingCombatJoin`; if gone (ended), clear flag, return `"The combat has ended."`.
  3. Build `*combat.Combatant` from session using the same pattern as `startCombatLocked` in `internal/gameserver/combat_handler.go` (lines 1547–1592). Fields to populate:
     - `ID = sess.UID`, `Kind = KindPlayer`, `Name = sess.CharName`
     - `MaxHP = sess.CurrentHP` (note: existing code sets MaxHP from CurrentHP — preserve this pattern)
     - `CurrentHP = sess.CurrentHP`, `AC` computed from `sess.Equipment.ComputedDefenses`
     - `Level = 1`, `StrMod = 2`, `DexMod = 1` (same hardcoded placeholders as `startCombatLocked`)
     - `Loadout` from `h.loadouts[sess.UID]` (under `loadoutsMu`)
     - `WeaponProficiencyRank` from `sess.Proficiencies[weaponCategory]`
     - `WeaponDamageType` from `Loadout.MainHand.Def.DamageType`
     - `Resistances = sess.Resistances`, `Weaknesses = sess.Weaknesses`
     - `GritMod`, `QuicknessMod`, `SavvyMod` from `combat.AbilityMod(sess.Abilities.*)`
     - `ToughnessRank`, `HustleRank`, `CoolRank` from `combat.DefaultSaveRank(sess.Proficiencies[*])`
     - `ArmorProficiencyRank` — intentionally omitted (same gap as `startCombatLocked`; leave zero value `""`)
  4. Roll initiative: `combat.RollInitiative([]*Combatant{playerCbt}, h.dice.Src())`.
  5. Call `h.engine.AddCombatant(sess.PendingCombatJoin, playerCbt)`.
  6. Set `sess.Status = statusInCombat`; clear `sess.PendingCombatJoin`.
  7. Broadcast to room: `"<CharName> joins the fight!"`.
  8. Return narrative: `"You join the combat (initiative %d)."`.

### `decline` command (CMD-1–7)

- **HandlerDecline** constant: `"decline"`
- **BuiltinCommands entry:** `{Name: "decline", Help: "Decline to join active combat.", Category: CategoryCombat, Handler: HandlerDecline}`
- **Proto:** `message DeclineRequest {}` wired as `DeclineRequest decline = 74`. `make proto` is run once after both `JoinRequest` and `DeclineRequest` are added to the proto file.
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

**StartCombat pseudocode (insertion point):**
```
// After cbt.Combatants is populated from the initial combatant list:
// cbt.Participants is nil at this point (zero value of []string); append on nil is
// valid Go and produces a new slice. No explicit initialization is required.
for each c in combatants:
    if c.Kind == KindPlayer:
        cbt.Participants = append(cbt.Participants, c.ID)
```

### Living participant definition

**`livingParticipants`** in all split algorithms is a `[]*session.PlayerSession` built by the helper `livingParticipantSessions` (a new private method on `CombatHandler`):

```go
// livingParticipantSessions returns the player sessions for all participants
// in cbt whose Dead field is false, in initiative-descending order (the same
// order as cbt.Combatants).
//
// Dead player combatants are NOT removed from cbt.Combatants by removeDeadNPCsLocked
// (only NPCs are removed). They remain in cbt.Combatants with Dead==true, so the
// Dead filter correctly excludes them. The participantSet filter ensures only players
// who were ever in the combat (cbt.Participants) are included, not observers.
//
// A player whose CurrentHP == 0 but Dead == false (the dying state) IS included.
//
// Caller must hold combatMu.
func (h *CombatHandler) livingParticipantSessions(cbt *combat.Combat) []*session.PlayerSession {
    participantSet := map[string]bool{}
    for _, uid := range cbt.Participants {
        participantSet[uid] = true
    }
    var result []*session.PlayerSession
    for _, c := range cbt.Combatants {
        if !participantSet[c.ID] || c.Dead {
            continue
        }
        if sess, ok := h.sessions.GetPlayer(c.ID); ok {
            result = append(result, sess)
        }
    }
    return result
}
```

Location: `internal/gameserver/combat_handler.go`.

**Locking for `cbt.Participants` reads:** `removeDeadNPCsLocked` is called with `combatMu` held. `AddCombatant` holds `e.mu` when appending to `cbt.Participants`. Since `combatMu` is always acquired before any `Engine` method that acquires `e.mu` is called, reads of `cbt.Participants` under `combatMu` are safe from concurrent `AddCombatant` mutation.

### XP split

**New method required — `xp.Service`:**

```go
// AwardXPAmount awards a pre-computed XP amount to a player.
// It exists so that callers can split a kill reward before calling award.
//
// Precondition: sess non-nil; xpAmount >= 0.
// Postcondition: same as AwardKill (XP, level, HP updated; persisted).
func (s *Service) AwardXPAmount(ctx context.Context, sess *session.PlayerSession, characterID int64, xpAmount int) ([]string, error)
```

Add to `internal/game/xp/service.go`; implementation delegates to the existing `award` helper.

**Location:** `removeDeadNPCsLocked` in `combat_handler.go`, replacing the `firstLivingPlayer` XP award. Also remove the now-dead `xpAmount` local variable at line 2208 and the `AwardKill` call at line 2209.

**Algorithm:**
```go
if h.xpSvc != nil {
    cfg := h.xpSvc.Config()
    livingParticipants := h.livingParticipantSessions(cbt)
    if len(livingParticipants) > 0 {
        totalXP := inst.Level * cfg.Awards.KillXPPerNPCLevel
        share := totalXP / len(livingParticipants)  // integer division; remainder discarded
        if share == 0 && totalXP > 0 {
            // Award 1 XP to first participant only; no XP created from nothing.
            // AwardXPAmount is a new method on xp.Service that delegates to the internal
            // award helper. It is distinct from AwardKill (which multiplies by npcLevel
            // internally). Do NOT use AwardKill here (REQ-XP4).
            xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), livingParticipants[0], livingParticipants[0].CharacterID, 1)
            if xpErr != nil && h.logger != nil {
                h.logger.Warn("AwardXPAmount failed", zap.Error(xpErr))
            }
            if xpErr == nil {
                // announce xpMsgs to livingParticipants[0]
            }
        } else {
            for _, p := range livingParticipants {
                xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), p, p.CharacterID, share)
                if xpErr != nil && h.logger != nil {
                    h.logger.Warn("AwardXPAmount failed", zap.Error(xpErr))
                }
                if xpErr == nil {
                    // announce "You gain %d XP for killing %s." % (share, inst.Name()) to p
                }
            }
        }
    }
}
```

REQ-XP1: Integer division; remainder discarded. No XP is created from nothing.
REQ-XP2: If all participants are dead, no XP is awarded.
REQ-XP3: Single-participant case is identical to current behavior.
REQ-XP4: `AwardKill` MUST NOT be called in the multi-player XP path. The pre-computed `share` MUST be passed to `AwardXPAmount` directly to avoid re-multiplying by `KillXPPerNPCLevel`.

### Currency split

**Location:** `removeDeadNPCsLocked` in `combat_handler.go`. The existing code has two currency award paths:
1. NPC has a loot table: `result.Currency` from `GenerateLoot` + `inst.Currency` robbed wallet.
2. NPC has no loot table but has a non-zero `inst.Currency` wallet only.

Both paths MUST distribute currency among `livingParticipants` (obtained via `h.livingParticipantSessions(cbt)`). Extract the shared distribution logic into a method on `CombatHandler` and call it from both branches.

**Algorithm (shared method):**
```go
// distributeCurrencyLocked distributes totalCurrency among livingParticipants.
// All SaveCurrency errors are logged as warnings; no error is returned.
// Caller must hold combatMu.
func (h *CombatHandler) distributeCurrencyLocked(ctx context.Context, livingParticipants []*session.PlayerSession, totalCurrency int) {
    if totalCurrency == 0 || len(livingParticipants) == 0 {
        return
    }
    share := totalCurrency / len(livingParticipants)  // integer division; remainder discarded
    if share == 0 {
        // totalCurrency > 0 but fewer units than participants: give 1 to first player only
        livingParticipants[0].Currency++
        if h.currencySaver != nil {
            if err := h.currencySaver.SaveCurrency(ctx, livingParticipants[0].CharacterID, livingParticipants[0].Currency); err != nil && h.logger != nil {
                h.logger.Warn("SaveCurrency failed", zap.Error(err))
            }
        }
        return
    }
    for _, p := range livingParticipants {
        p.Currency += share
        if h.currencySaver != nil {
            if err := h.currencySaver.SaveCurrency(ctx, p.CharacterID, p.Currency); err != nil && h.logger != nil {
                h.logger.Warn("SaveCurrency failed", zap.Error(err))
            }
        }
    }
}
```

**Call sites in `removeDeadNPCsLocked`:**
```go
// Loot-table path:
livingParticipants := h.livingParticipantSessions(cbt)
totalCurrency := result.Currency + inst.Currency
inst.Currency = 0
h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)

// No-loot-table path:
livingParticipants := h.livingParticipantSessions(cbt)
totalCurrency := inst.Currency
inst.Currency = 0
h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)
```

REQ-CURR1: Remainder discarded (not re-distributed).
REQ-CURR2: Single-participant: identical to current behavior.
REQ-CURR3: The `share=0` fallback awards at most 1 unit of currency total; no currency is created from nothing.

### Item split — round-robin by initiative

**Location:** same block, replacing the floor-drop loop.

**Algorithm:**
```go
livingParticipants := h.livingParticipantSessions(cbt)
// livingParticipants is ordered by initiative descending (from cbt.Combatants ordering)
if len(livingParticipants) == 0 {
    // drop all items to floor
} else {
    for i, lootItem := range result.Items {
        recipient := livingParticipants[i%len(livingParticipants)]
        if recipient has inventory capacity {
            add lootItem to recipient inventory
        } else if h.floorMgr != nil {
            h.floorMgr.Drop(roomID, lootItem)
        }
    }
}
```

REQ-ITEM1: If no living participants, all items drop to floor.
REQ-ITEM2: Single-participant: identical to current behavior (items go to that player or floor).

---

## Testing

- REQ-T1 (example): Player enters room with active combat → `sess.PendingCombatJoin == roomID`; player receives join prompt.
- REQ-T2 (example): Player sends `join` → added to `cbt.Combatants` at correct initiative position; `sess.Status == statusInCombat`; `PendingCombatJoin == ""`.
- REQ-T3 (example): Player sends `decline` → `PendingCombatJoin == ""`; player not in combat.
- REQ-T4 (example): Combat ends while player is pending → `PendingCombatJoin` cleared; player receives "combat has ended" message.
- REQ-T5 (property): For any initiative value and any existing sorted Combatants slice, `AddCombatant` results in a slice sorted by initiative descending.
- REQ-T13 (example): Player already in combat (sess.Status == statusInCombat) enters a room with active combat → no join prompt sent; `PendingCombatJoin` remains empty.
- REQ-T6 (example): `handleJoin` when `PendingCombatJoin == ""` returns "No combat to join."
- REQ-T7 (example): Two players in combat; NPC dies → each receives `floor(totalXP/2)` XP.
- REQ-T8 (example): Two players in combat; NPC drops 10 currency → each receives 5.
- REQ-T9 (example): Two players in combat; NPC drops 3 items → player A gets items 0 and 2, player B gets item 1 (round-robin by initiative).
- REQ-T10 (property): For any `n` living participants and `totalCurrency` in [0,10000], each share is `floor(totalCurrency/n)`; no participant receives more than their share.
- REQ-T11 (property): For any `n` living participants and `m` items, each item goes to exactly one recipient; total items distributed == m.
- REQ-T12 (example): Single participant — XP, currency, and items behave identically to pre-feature behavior.
- REQ-T14 (property): For any sequence of `AddCombatant` calls on a valid combat, `len(cbt.Participants)` is monotonically non-decreasing and equals the number of player combatants ever added; `cbt.Conditions` contains an entry for every combatant added.
- REQ-T15 (example): `AddCombatant` with a non-existent roomID returns a non-nil error.
- REQ-T16 (example): A player in the `dying` state (`Dead == false`, `CurrentHP == 0`) who is in `cbt.Participants` IS included in `livingParticipants` for the split; their XP share and currency share are awarded normally.
- REQ-T17 (property): For any `n` living participants and NPC of level `L` with `KillXPPerNPCLevel = K`, total XP distributed equals `floor(L * K / n) * n` (each participant receives exactly `floor(totalXP/n)`; no participant receives more than that share).
- REQ-T18 (example): 3 living participants, 2 total currency → first participant receives 1 currency; second and third participants receive 0; total distributed is 1 (no currency created from nothing).
- REQ-T19 (example): NPC of level 1 with `KillXPPerNPCLevel=1`, 2 living participants → `totalXP=1`, `share=0` → first participant receives 1 XP; second participant receives 0 XP; total awarded is 1 (no XP created from nothing).
- REQ-T20 (example): Player who has `PendingCombatJoin != ""` moves into a second room with active combat → the old pending join is overwritten with the new `roomID`; a fresh join prompt is sent.
- REQ-T21 (example): Player joins combat in round N → player has no `ActionQueue` entry during round N; player's queue appears at the start of round N+1 when `StartRound` rebuilds the map.
- REQ-T22 (example): Given player A (initiative 15) is the current actor (`turnIndex=1`) when a new combatant with initiative 20 joins → `turnIndex` becomes 2; the next `AdvanceTurn` still advances to the combatant after player A in the original order.
