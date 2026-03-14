# Combat Commands Design Spec
**Date:** 2026-03-14
**System:** Gunchete (Post-Apocalyptic PF2e)
**Scope:** Climb/Swim rework, Sense Motive redesign, Delay command, Rename cleanup

---

## 1. Overview

This spec defines five enhancements to Gunchete combat and exploration mechanics:

1. **Climb Rework** — Exit-based DC with terrain defaults and height-based fall damage
2. **Swim Rework** — Exit-based DC with terrain defaults and drowning mechanics
3. **Sense Motive Redesign** — Skill check revealing NPC state; NPC field rename (`Deception` → `Hustle`)
4. **Delay Command** — New AP banking mechanic with AC penalty
5. **Rename Cleanup** — Update stale skill names and remove deprecated room properties

All features use the Gunchete skill renames:
- Athletics → **Muscle** (Brutality-based, `sess.Skills["muscle"]`)
- Perception → **Awareness** (Savvy-based passive bonus, `sess.Skills["awareness"]`)
- Deception → **Hustle** (Flair-based, `inst.Hustle`)

Critical success/failure uses `combat.OutcomeFor(total, dc)` returning `CritSuccess`, `Success`, `Failure`, `CritFailure`.

Skill rank bonuses use the existing `skillRankBonus(rank string) int` function in `internal/gameserver/grpc_service.go`.

---

## 2. Feature 1: Climb Rework

### 2.1 Problem Statement

Current `handleClimb` implementation (in `internal/gameserver/grpc_service.go`):
- Reads `room.Properties["climbable"]` and `room.Properties["climb_dc"]`
- Uses deprecated skill name `sess.Skills["athletics"]`
- Fall damage is flat 1d6 with no height dependency
- No integration with exit data model

### 2.2 Design

#### 2.2.1 Exit Data Model

Add two optional fields to `world.Exit` struct in `internal/game/world/model.go` (YAML tags: `climb_dc`, `height`):

```go
type Exit struct {
    // ... existing fields ...
    ClimbDC int // If 0, exit is not climbable (unless terrain default applies)
    Height  int // Height in feet; used for fall damage calculation. 0 = ground level
}
```

#### 2.2.2 Room Terrain Field

Add a `Terrain` field to `world.Room` struct in `internal/game/world/model.go` (YAML tag: `terrain`):

```go
type Room struct {
    // ... existing fields ...
    Terrain string // Optional terrain type; used for climb/swim DC defaults
}
```

#### 2.2.3 Terrain Default Table

When an exit `ClimbDC` is 0 and the origin room has a `Terrain` field, apply defaults:

| Terrain | Default Climb DC |
|---------|-----------------|
| `rubble` | 12 |
| `cliff` | 20 |
| `wall` | 15 |
| `sewer` | 10 |

**REQ-CLIMB-1:** MUST apply terrain default DC when exit `ClimbDC` is 0 and `room.Terrain` matches a table entry.

**REQ-CLIMB-1a:** If exit `ClimbDC` is 0 and `room.Terrain` has no matching entry, the exit is not climbable; MUST respond with "`There is nothing to climb here.`"

#### 2.2.4 Command Behavior

**Syntax:** `climb <direction>`

**Zero-argument case:**
- **REQ-CLIMB-1b:** If no direction is provided, MUST respond with "`Climb which direction?`" and take no action.

**Action Economy:**
- **REQ-CLIMB-2:** In combat, MUST cost 2 AP.
- **REQ-CLIMB-3:** Outside combat, MUST be free.

**Skill Check:**
- **REQ-CLIMB-4:** MUST roll 1d20 + `skillRankBonus(sess.Skills["muscle"])` vs effective `ClimbDC`.
- **REQ-CLIMB-5:** MUST use `combat.OutcomeFor(total, dc)` to determine result.

**Outcomes:**

| Result | Effect |
|--------|--------|
| Critical Success | Move to destination room |
| Success | Move to destination room |
| Failure | Remain in current room; message: "`You fail to gain purchase on the climb.`" |
| Critical Failure | Fall damage: `max(1, floor(height/10))` d6; apply `prone` condition if in combat |

**REQ-CLIMB-6:** MUST calculate fall damage as `max(1, floor(height/10))` d6 for any height (height=0 → 1d6 minimum).

**REQ-CLIMB-6a:** Fall damage MUST be dealt as untyped damage via the existing damage application path.

#### 2.2.5 Migration

**REQ-CLIMB-7:** MUST remove `room.Properties["climbable"]` and `room.Properties["climb_dc"]` fallback reads from handler code.

**REQ-CLIMB-8:** MUST update all YAML room definitions that reference removed properties.

---

## 3. Feature 2: Swim Rework

### 3.1 Problem Statement

Current `handleSwim` implementation (in `internal/gameserver/grpc_service.go`):
- Reads `room.Properties["water_terrain"]` and `room.Properties["water_dc"]`
- Uses deprecated skill name `sess.Skills["athletics"]`
- Critical failure drowning mechanics are correct but skill naming is stale
- No integration with exit data model

### 3.2 Design

#### 3.2.1 Exit Data Model

Add optional field to `world.Exit` struct in `internal/game/world/model.go` (YAML tag: `swim_dc`):

```go
type Exit struct {
    // ... existing fields ...
    SwimDC int // If 0, exit is not swimmable (unless terrain default applies)
}
```

(Note: `world.Room.Terrain` is added in Feature 1 and shared by both swim and climb.)

#### 3.2.2 Terrain Default Table

When an exit `SwimDC` is 0 and the origin room has a `Terrain` field, apply defaults:

| Terrain | Default Swim DC |
|---------|----------------|
| `sewer` | 10 |
| `river` | 15 |
| `ocean` | 20 |
| `flooded` | 12 |

**REQ-SWIM-1:** MUST apply terrain default DC when exit `SwimDC` is 0 and `room.Terrain` matches a table entry.

**REQ-SWIM-1a:** If exit `SwimDC` is 0 and `room.Terrain` has no matching entry, the exit is not swimmable; MUST respond with "`There is no water here.`"

#### 3.2.3 Command Behavior

**Syntax:** `swim <direction>`

**Zero-argument case:**
- **REQ-SWIM-1b:** If no direction is provided, MUST respond with "`Swim which direction?`" and take no action.

**Action Economy:**
- **REQ-SWIM-2:** In combat, MUST cost 2 AP.
- **REQ-SWIM-3:** Outside combat, MUST be free.

**Skill Check:**
- **REQ-SWIM-4:** MUST roll 1d20 + `skillRankBonus(sess.Skills["muscle"])` vs effective `SwimDC`.
- **REQ-SWIM-5:** MUST use `combat.OutcomeFor(total, dc)` to determine result.

**Outcomes:**

| Result | Effect |
|--------|--------|
| Critical Success | Move to destination room; if `submerged` condition active, remove it |
| Success | Move to destination room; if `submerged` condition active, remove it |
| Failure | Remain in current room; message: "`You struggle against the current.`" |
| Critical Failure | Apply 1d6 drowning damage; apply `submerged` condition |

**REQ-SWIM-6:** MUST use existing drowning damage and `submerged` condition logic.

**REQ-SWIM-6a:** On Critical Success or Success when the player has the `submerged` condition, MUST remove `submerged` before moving the player.

**REQ-SWIM-6b:** "Surface if submerged" means: if the player is `submerged` and succeeds, they move to the destination exit room. There is no automatic return-to-origin; the exit direction determines the destination room.

#### 3.2.4 Migration

**REQ-SWIM-7:** MUST remove `room.Properties["water_terrain"]` and `room.Properties["water_dc"]` fallback reads from handler code.

**REQ-SWIM-8:** MUST update all YAML room definitions that reference removed properties.

---

## 4. Feature 3: Sense Motive Rework

### 4.1 Problem Statement

Current `handleMotive` implementation (in `internal/gameserver/grpc_service.go`):
- In combat: only reveals HP tier (already visible in room view)
- Out of combat: stubbed; no information revealed
- NPC field is called `Deception` (inconsistent with Gunchete skill rename to `Hustle`)
- No strategic information about NPC state

### 4.2 Design

#### 4.2.1 NPC Field Rename

**REQ-MOTIVE-1:** MUST rename `npc.Template.Deception` → `npc.Template.Hustle` (YAML tag: `hustle`).

**REQ-MOTIVE-2:** MUST rename `npc.Instance.Deception` → `npc.Instance.Hustle`.

**REQ-MOTIVE-3:** MUST update all references across handlers, tests, and message strings.

#### 4.2.2 New NPC Fields

Add the following fields to `npc.Template` in `internal/game/npc/template.go` and copy to `npc.Instance` at spawn time:

```go
// SpecialAbilities lists named special abilities (e.g. "Rage", "Poison Spit").
// YAML tag: special_abilities. Used by Sense Motive to reveal hidden abilities.
SpecialAbilities []string `yaml:"special_abilities"`

// Disposition is the NPC's default stance toward players.
// Valid values: "hostile", "wary", "neutral", "friendly". Default: "hostile".
// YAML tag: disposition.
Disposition string `yaml:"disposition"`
```

(Note: `npc.Template` already has `Resistances map[string]int` and `Weaknesses map[string]int`; these are NOT replaced. The existing fields are used as-is for the motive critical success reveal.)

Add to `npc.Instance` in `internal/game/npc/instance.go`, initialized from template at spawn time:
- `SpecialAbilities []string` — copy from `tmpl.SpecialAbilities` at spawn.
- `Disposition string` — copy from `tmpl.Disposition` at spawn; default to `"hostile"` if empty.

The heuristic in §4.2.7 accesses `inst.SpecialAbilities` (the instance field, not the template field directly).

**REQ-MOTIVE-15:** MUST add `SpecialAbilities []string` (YAML: `special_abilities`) to `npc.Template`. This is a NEW field; it does NOT conflict with the existing `Abilities npc.Abilities` struct field.

**REQ-MOTIVE-16:** MUST use existing `npc.Template.Resistances map[string]int` and `npc.Template.Weaknesses map[string]int` for motive critical success reveal. Display format: `"Resistant to: fire (5), cold (3)"` and `"Weak to: electric (10)"` (iterate map in sorted key order for determinism).

**REQ-MOTIVE-17:** MUST add `Disposition string` (YAML: `disposition`) to `npc.Template` and `npc.Instance` as described above.

#### 4.2.3 Motive Bonus Field

Add to `npc.Instance`:

```go
// MotiveBonus is a one-time +2 circumstance bonus to NPC's next attack,
// applied when a player critically fails a Sense Motive check in combat.
// Consumed (zeroed) immediately after the first attack it applies to.
MotiveBonus int
```

**REQ-MOTIVE-11a:** The combat attack resolution MUST check `inst.MotiveBonus` and add it to the NPC's attack roll when nonzero, then set `inst.MotiveBonus = 0`.

#### 4.2.4 Command Behavior

**Syntax:** `motive <target>`

**Action Economy:**
- **REQ-MOTIVE-4:** In combat, MUST cost 1 AP.
- **REQ-MOTIVE-5:** Outside combat, MUST be free.

**Skill Check:**
- **REQ-MOTIVE-6:** MUST roll 1d20 + `skillRankBonus(sess.Skills["awareness"])` vs `10 + inst.Hustle`.
- **REQ-MOTIVE-7:** MUST use `combat.OutcomeFor(total, dc)` to determine result.

#### 4.2.5 In-Combat Outcomes

**REQ-MOTIVE-8:** Critical Success MUST reveal:
- Next intended action (from heuristic, see §4.2.7)
- `SpecialAbilities` list (if non-empty, display as "`Hidden abilities: Rage, Poison Spit`")
- Resistances and weaknesses (formatted as per REQ-MOTIVE-16)

**REQ-MOTIVE-9:** Success MUST reveal:
- Next intended action only

**REQ-MOTIVE-10:** Failure MUST reveal:
- No information; MUST send "`You cannot read their intentions.`"

**REQ-MOTIVE-11:** Critical Failure MUST:
- Send "`You misread them completely — they notice.`"
- Set `inst.MotiveBonus = 2`

#### 4.2.6 Out-of-Combat Outcomes

**REQ-MOTIVE-12:** Critical Success or Success MUST reveal:
- NPC disposition (e.g., `"The ganger seems hostile."` / `"The merchant seems friendly."`)

**REQ-MOTIVE-13:** Failure MUST reveal:
- No information; MUST send "`You cannot get a read on them.`"

**REQ-MOTIVE-14:** Critical Failure MUST:
- Send "`You misread them badly.`"
- If `inst.Disposition` is `"neutral"` or `"wary"`, set `inst.Disposition = "hostile"`

#### 4.2.7 NPC "Next Intended Action" Heuristic

Apply in priority order:

```
if HP% < 25:
  return "looks ready to flee"
else if len(inst.SpecialAbilities) > 0:
  return "seems to be holding something back"
else:
  return "looks focused on the fight"
```

---

## 5. Feature 4: Delay (New Command)

### 5.1 Problem Statement

No delay command exists in the current implementation. Players cannot bank Action Points (AP) for strategic advantage in future combat rounds.

### 5.2 Design

#### 5.2.1 Session State Extension

Add one new field to `PlayerSession` in `internal/game/session/manager.go`:

```go
type PlayerSession struct {
    // ... existing fields ...
    BankedAP int // AP reserved for next round (session-only, not persisted)
}
```

**REQ-DELAY-1:** `BankedAP` MUST be session-only (not persisted to database).

**Architecture note:** AP state lives in `combat.ActionQueue` (via `CombatHandler.RemainingAP`, `SpendAP`, `SpendAllAP` in `internal/gameserver/combat_handler.go`), not in `PlayerSession`. The AC penalty uses `Combatant.ACMod` (defined on `combat.Combatant` in `internal/game/combat/combat.go`), which is automatically zeroed by `cbt.StartRound(N)` at the beginning of each round.

#### 5.2.2 Command Behavior

**Syntax:** `delay`

**Availability:**
- **REQ-DELAY-2:** MUST be available only during active combat.
- **REQ-DELAY-3:** MUST fail with "`You cannot delay outside of combat.`" if used out of combat.

**Action Economy:**
- **REQ-DELAY-4:** MUST cost 1 AP.
- **REQ-DELAY-5:** MUST fail with "`Not enough AP to delay.`" if player has < 1 AP remaining.

**Effect (order of operations):**

1. Read current remaining AP: `remaining := h.combatH.RemainingAP(uid)` (pre-spend value)
2. Spend 1 AP: `h.combatH.SpendAP(uid, 1)` (fails if remaining < 1)
3. Compute banked AP: `sess.BankedAP = min(remaining-1, 2)` (remaining-1 = post-spend remaining)
4. Drain remaining AP: `h.combatH.SpendAllAP(uid)`
5. Apply AC penalty: find player's `*combat.Combatant` in the active combat by uid; set `combatant.ACMod -= 2`

- **REQ-DELAY-6:** MUST compute `BankedAP = min(remaining-1, 2)` where `remaining` is captured BEFORE `SpendAP` is called. This equals `min(postSpendRemaining, 2)`.
- **REQ-DELAY-7:** MUST call `h.combatH.SpendAllAP(uid)` to zero current round AP.
- **REQ-DELAY-8:** MUST set `combatant.ACMod -= 2` on the player's `*combat.Combatant`. This penalty is automatically cleared by `cbt.StartRound(N)` at the start of the next round — no `DelayedUntilRound` field is needed.
- **REQ-DELAY-9:** MUST display: `"You delay, banking {N} AP for next round. You are exposed (-2 AC)."`

#### 5.2.3 Round Start Logic

In `resolveAndAdvanceLocked` in `internal/gameserver/combat_handler.go`, after `cbt.StartRound(3)` at line ~1400 (which rebuilds all ActionQueues), add a loop over player combatants:

```go
// Inject banked AP from delayed players.
for _, c := range cbt.Combatants {
    if c.Kind != combat.KindPlayer { continue }
    sess, ok := h.sessions.GetPlayer(c.ID)
    if !ok || sess.BankedAP <= 0 { continue }
    q := cbt.ActionQueues[c.ID]
    if q != nil {
        q.AddAP(sess.BankedAP) // new method — see REQ-DELAY-10 note
    }
    sess.BankedAP = 0
}
```

- **REQ-DELAY-10:** MUST add a new method `func (q *ActionQueue) AddAP(n int)` in `internal/game/combat/action.go` that increments `q.remaining` by `n`. Precondition: `n >= 0`. Postcondition: `q.remaining` increases by `n`.
- **REQ-DELAY-11:** MUST clear `sess.BankedAP = 0` after calling `q.AddAP`.
- **REQ-DELAY-12:** The `-2 AC` penalty (`combatant.ACMod -= 2`) is automatically cleared because `cbt.StartRound(N)` zeroes `ACMod` for all combatants at the start of each new round. No additional cleanup is needed. The penalty does NOT carry into the next round.

---

## 6. Feature 5: Rename Cleanup

### 6.1 Skill Name Updates

**REQ-RENAME-1:** MUST replace all occurrences of `sess.Skills["athletics"]` with `sess.Skills["muscle"]` in:
- `handleClimb`
- `handleSwim`
- All handler test files
- Any other skill check in combat/exploration handlers

**REQ-RENAME-2:** MUST replace all message strings and `Help` fields in `BuiltinCommands()` that reference "athletics" with "muscle".

### 6.2 NPC Deception → Hustle

**REQ-RENAME-3:** MUST replace all occurrences of `inst.Deception` with `inst.Hustle`.

**REQ-RENAME-4:** MUST replace all occurrences of `template.Deception` with `template.Hustle`.

**REQ-RENAME-5:** MUST update YAML tag from `deception` to `hustle` in NPC struct.

**REQ-RENAME-6:** MUST replace all message strings and `Help` fields that reference "deception" with "hustle".

### 6.3 Room Property Removal

**REQ-RENAME-7:** MUST remove all room property fallbacks for deprecated keys:
- `room.Properties["climbable"]`
- `room.Properties["climb_dc"]`
- `room.Properties["water_terrain"]`
- `room.Properties["water_dc"]`

**REQ-RENAME-8:** MUST update handler code to reference exit fields exclusively.

---

## 7. Testing Requirements

### 7.1 Climb Tests

- **REQ-T1:** Climb success MUST move player to destination room.
- **REQ-T2:** Climb critical failure MUST deal height-based fall damage and apply `prone` condition (in combat).
- **REQ-T3a (property):** For all entries in the climb terrain default table, `effectiveClimbDC(exit, room)` MUST return the correct default when exit `ClimbDC` is 0.
- **REQ-T6 (property):** For any exit height in [0, 100], fall damage MUST equal `max(1, floor(height/10))` d6.

### 7.2 Swim Tests

- **REQ-T4:** Swim success MUST move player through water exit; if `submerged`, condition MUST be removed.
- **REQ-T5:** Swim critical failure MUST deal 1d6 drowning damage and apply `submerged` condition.
- **REQ-T3b (property):** For all entries in the swim terrain default table, `effectiveSwimDC(exit, room)` MUST return the correct default when exit `SwimDC` is 0.

### 7.3 Sense Motive Tests

- **REQ-T7:** Motive success in combat MUST reveal next intended action message.
- **REQ-T8:** Motive critical failure in combat MUST set `inst.MotiveBonus = 2` and NPC MUST apply it on next attack.
- **REQ-T9:** Motive success out of combat MUST reveal NPC disposition.
- **REQ-T10:** Motive critical failure out of combat with neutral/wary NPC MUST change `inst.Disposition` to `"hostile"`.

### 7.4 Delay Tests

- **REQ-T11:** Delay in combat MUST: deduct 1 AP, bank remaining (capped at 2), zero current AP, set `DelayedUntilRound`.
- **REQ-T12:** Banked AP at round start MUST be added to new AP total and `BankedAP` MUST be cleared.
- **REQ-T13 (property):** For any remaining AP in [0, 10], after delay `BankedAP` MUST equal `min(remainingAP - 1, 2)` (pre-cost remaining), equivalently `min(postCostRemaining, 2)` where `postCostRemaining = remainingAP - 1`.

### 7.5 Test Strategy

**REQ-SWENG-5a:** All tests MUST use Property-Based Testing (pgregory.net/rapid) where applicable:
- Fall damage formula (REQ-T6)
- AP banking formula (REQ-T13)
- Terrain default table coverage (REQ-T3a, REQ-T3b)

**REQ-SWENG-6:** All tests MUST pass with 100% success before marking feature complete.

---

## 8. Integration Points

### 8.1 Commands Registry

**REQ-CMD-1:** Verify or add handler constants in `internal/game/command/commands.go`:
- `HandlerClimb`, `HandlerSwim`, `HandlerMotive` — already present; verify correct values.
- `HandlerDelay` — does NOT exist yet. MUST add as a new constant (e.g., `HandlerDelay = "delay"`).

**REQ-CMD-2:** Append or update `Command{...}` entries in `BuiltinCommands()` with correct `Skill` references (muscle/awareness):
- `delay` command entry — does NOT exist yet. MUST add.
- `climb`, `swim`, `motive` entries — update `Help` strings and skill references (REQ-RENAME-2, REQ-RENAME-6).

### 8.2 Proto Messages

**REQ-CMD-4a:** Update existing proto messages in `api/proto/game/v1/game.proto`:
- `ClimbRequest` (line 802) — currently empty `{}`. MUST add `string direction = 1;` field.
- `SwimRequest` (line 805) — currently empty `{}`. MUST add `string direction = 1;` field.
- `MotiveRequest` (line 816) — already has `string target = 1;`. No change needed.
- `DelayRequest` — does NOT exist yet. MUST add new message: `message DelayRequest {}` (no fields).

**REQ-CMD-4b:** Add `DelayRequest` to the `ClientMessage` oneof as field number 72:
```proto
DelayRequest delay = 72;
```
(Note: `ClimbRequest = 67`, `SwimRequest = 68`, `MotiveRequest = 69` are already in the oneof.)

**REQ-CMD-4c:** Run `make proto` to regenerate Go bindings.

### 8.3 Frontend Bridge

**REQ-CMD-5a:** Add bridge functions to `internal/frontend/handlers/bridge_handlers.go`:
- `bridgeClimb(session, msg)`
- `bridgeSwim(session, msg)`
- `bridgeMotive(session, msg)`
- `bridgeDelay(session, msg)`

**REQ-CMD-5b:** Register all functions in `bridgeHandlerMap`.

**REQ-CMD-5c:** Ensure test `TestAllCommandHandlersAreWired` passes.

### 8.4 gRPC Service

**REQ-CMD-6a:** Implement handlers in `internal/gameserver/grpc_service.go`:
- `handleClimb(uid string, req *gamev1.ClimbRequest)` — existing function currently discards the request (`_ *gamev1.ClimbRequest`). MUST change to use `req` to read `req.Direction`.
- `handleSwim(uid string, req *gamev1.SwimRequest)` — existing function currently discards the request (`_ *gamev1.SwimRequest`). MUST change to use `req` to read `req.Direction`.
- `handleMotive(uid string, req *gamev1.MotiveRequest)` — existing function, MUST rework per §4.
- `handleDelay(uid string, req *gamev1.DelayRequest)` — does NOT exist yet. MUST implement from scratch.

**REQ-CMD-6b:** Wire `handleDelay` into the `dispatch` type switch. (The other three are already wired.)

### 8.5 Completion Criteria

**REQ-CMD-7:** All steps (CMD-1 through CMD-6b) MUST be completed and all tests MUST pass before any command is considered done. A command registered in `BuiltinCommands()` but not wired end-to-end is a defect.

---

## 9. Migration Checklist

- [ ] Add `Terrain string` to `world.Room` (YAML tag: `terrain`)
- [ ] Add `ClimbDC int` and `Height int` to `world.Exit` (YAML tags: `climb_dc`, `height`)
- [ ] Add `SwimDC int` to `world.Exit` (YAML tag: `swim_dc`)
- [ ] Add `SpecialAbilities []string` to `npc.Template` (YAML tag: `special_abilities`) and `npc.Instance` (copied at spawn)
- [ ] Add `Disposition string` to `npc.Template` (YAML tag: `disposition`) and `npc.Instance` (copied at spawn, default "hostile")
- [ ] Add `MotiveBonus int` to `npc.Instance`
- [ ] Rename `npc.Template.Deception` → `npc.Template.Hustle` (YAML tag: `hustle`)
- [ ] Rename `npc.Instance.Deception` → `npc.Instance.Hustle`
- [ ] Add `HandlerDelay` constant to `internal/game/command/commands.go`
- [ ] Add `delay` entry to `BuiltinCommands()` in `internal/game/command/commands.go`
- [ ] Add `BankedAP int` to `PlayerSession` (session-only)
- [ ] Add `AddAP(n int)` method to `ActionQueue` in `internal/game/combat/action.go`
- [ ] Remove room property fallbacks: `climbable`, `climb_dc`, `water_terrain`, `water_dc`
- [ ] Update all YAML room and NPC definitions
- [ ] Update all handler code (skill name, field name, message strings, Help fields)
- [ ] Update all test code
- [ ] Add proto messages and run `make proto`
- [ ] Implement/rework command handlers
- [ ] Add/update bridge functions
- [ ] Wire gRPC handlers
- [ ] Run full test suite (100% pass)
- [ ] Commit with message: `feat(combat): climb/swim rework, sense motive redesign, delay command`

---

## 10. References

- Gunchete System: Post-apocalyptic reskin of Pathfinder 2e
- Critical Success/Failure: `combat.OutcomeFor(total, dc)` returns `CritSuccess | Success | Failure | CritFailure`
- Skill rank bonus: `skillRankBonus(rank string) int` in `internal/gameserver/grpc_service.go`
- Command Registry: `internal/game/command/commands.go`
- Handler Pattern: `internal/game/command/*.go` with `Handle<Name>(uid, inst, session) error`
- Bridge Pattern: `internal/frontend/handlers/bridge_handlers.go` with `bridge<Name>(session, msg) error`
- gRPC Pattern: `internal/gameserver/grpc_service.go` with `handle<Name>(uid, msg) error` and type switch dispatch
- NPC Model: `internal/game/npc/template.go`, `internal/game/npc/instance.go`
- Session Model: `internal/game/session/manager.go`
- World Model: `internal/game/world/model.go`
