# Combat Commands Design Spec
**Date:** 2026-03-14
**System:** Gunchete (Post-Apocalyptic PF2e)
**Scope:** Climb/Swim rework, Sense Motive redesign, Delay command

---

## 1. Overview

This spec defines four enhancements to Gunchete combat and exploration mechanics:

1. **Climb Rework** — Exit-based DC with terrain defaults and height-based fall damage
2. **Swim Rework** — Exit-based DC with terrain defaults and drowning mechanics
3. **Sense Motive Redesign** — Skill check revealing NPC state; NPC field rename (`Deception` → `Hustle`)
4. **Delay Command** — New AP banking mechanic with AC penalty

All features use the Gunchete skill renames:
- Athletics → **Muscle** (Brutality-based, `sess.Skills["muscle"]`)
- Perception → **Awareness** (Savvy-based passive bonus)
- Deception → **Hustle** (Flair-based, `inst.Hustle`)

Critical success/failure uses `combat.OutcomeFor(total, dc)` returning `CritSuccess`, `Success`, `Failure`, `CritFailure`.

---

## 2. Feature 1: Climb Rework

### 2.1 Problem Statement

Current `handleClimb` implementation:
- Reads `room.Properties["climbable"]` and `room.Properties["climb_dc"]`
- Uses deprecated skill name `sess.Skills["athletics"]`
- Fall damage is flat 1d6 with no height dependency
- No integration with exit data model

### 2.2 Design

#### 2.2.1 Exit Data Model

Add two optional fields to the exit struct (YAML tags: `climb_dc`, `height`):

```go
type Exit struct {
    // ... existing fields ...
    ClimbDC int // If 0, exit is not climbable
    Height  int // Height in feet; used for fall damage calculation
}
```

#### 2.2.2 Terrain Default Table

When an exit omits `climb_dc` but the origin room has a `terrain` field, apply defaults:

| Terrain | Default Climb DC |
|---------|-----------------|
| `rubble` | 12 |
| `cliff` | 20 |
| `wall` | 15 |
| `sewer` | 10 |

**REQ-CLIMB-1:** MUST apply terrain default DC when exit `ClimbDC` is 0 and room `terrain` matches a table entry.

#### 2.2.3 Command Behavior

**Syntax:** `climb <direction>`

**Action Economy:**
- REQ-CLIMB-2: In combat, MUST cost 2 AP
- REQ-CLIMB-3: Outside combat, MUST be free

**Skill Check:**
- REQ-CLIMB-4: MUST roll 1d20 + Muscle rank bonus vs exit `ClimbDC`
- REQ-CLIMB-5: MUST use `combat.OutcomeFor(total, dc)` to determine result

**Outcomes:**

| Result | Effect |
|--------|--------|
| Critical Success | Move to destination room |
| Success | Move to destination room |
| Failure | Remain in current room; narrative message "`You fail to gain purchase on the climb.`" |
| Critical Failure | Fall damage: `max(1, floor(height/10))` d6; apply `prone` condition if in combat |

**REQ-CLIMB-6:** MUST calculate fall damage as `max(1, floor(height/10))` d6 for any height.

#### 2.2.4 Migration

**REQ-CLIMB-7:** MUST remove `room.Properties["climbable"]` and `room.Properties["climb_dc"]`.

**REQ-CLIMB-8:** MUST update all YAML room definitions that reference removed properties.

---

## 3. Feature 2: Swim Rework

### 3.1 Problem Statement

Current `handleSwim` implementation:
- Reads `room.Properties["water_terrain"]` and `room.Properties["water_dc"]`
- Uses deprecated skill name `sess.Skills["athletics"]`
- Critical failure drowning mechanics are correct but skill naming is stale
- No integration with exit data model

### 3.2 Design

#### 3.2.1 Exit Data Model

Add optional field to the exit struct (YAML tag: `swim_dc`):

```go
type Exit struct {
    // ... existing fields ...
    SwimDC int // If 0, exit is not swimmable
}
```

#### 3.2.2 Terrain Default Table

When an exit omits `swim_dc` but the origin room has a `terrain` field, apply defaults:

| Terrain | Default Swim DC |
|---------|----------------|
| `sewer` | 10 |
| `river` | 15 |
| `ocean` | 20 |
| `flooded` | 12 |

**REQ-SWIM-1:** MUST apply terrain default DC when exit `SwimDC` is 0 and room `terrain` matches a table entry.

#### 3.2.3 Command Behavior

**Syntax:** `swim <direction>`

**Action Economy:**
- REQ-SWIM-2: In combat, MUST cost 2 AP
- REQ-SWIM-3: Outside combat, MUST be free

**Skill Check:**
- REQ-SWIM-4: MUST roll 1d20 + Muscle rank bonus vs exit `SwimDC`
- REQ-SWIM-5: MUST use `combat.OutcomeFor(total, dc)` to determine result

**Outcomes:**

| Result | Effect |
|--------|--------|
| Critical Success | Move to destination room (or surface if submerged) |
| Success | Move to destination room (or surface if submerged) |
| Failure | Remain in current room; narrative message "`You struggle against the current.`" |
| Critical Failure | Apply 1d6 drowning damage; apply `submerged` condition |

**REQ-SWIM-6:** MUST use existing drowning damage and submerged condition logic.

#### 3.2.4 Migration

**REQ-SWIM-7:** MUST remove `room.Properties["water_terrain"]` and `room.Properties["water_dc"]`.

**REQ-SWIM-8:** MUST update all YAML room definitions that reference removed properties.

---

## 4. Feature 3: Sense Motive Rework

### 4.1 Problem Statement

Current `handleMotive` implementation:
- In combat: only reveals HP tier (already visible in room view)
- Out of combat: stubbed; no information revealed
- NPC field is called `Deception` (inconsistent with Gunchete skill rename to `Hustle`)
- No strategic information about NPC state

### 4.2 Design

#### 4.2.1 NPC Field Rename

**REQ-MOTIVE-1:** MUST rename `npc.Template.Deception` → `npc.Template.Hustle` (YAML tag: `hustle`).

**REQ-MOTIVE-2:** MUST rename `npc.Instance.Deception` → `npc.Instance.Hustle`.

**REQ-MOTIVE-3:** MUST update all references across handlers, tests, and message strings.

#### 4.2.2 Command Behavior

**Syntax:** `motive <target>`

**Action Economy:**
- REQ-MOTIVE-4: In combat, MUST cost 1 AP
- REQ-MOTIVE-5: Outside combat, MUST be free

**Skill Check:**
- REQ-MOTIVE-6: MUST roll 1d20 + Awareness rank bonus vs `10 + inst.Hustle`
- REQ-MOTIVE-7: MUST use `combat.OutcomeFor(total, dc)` to determine result

#### 4.2.3 In-Combat Outcomes

**REQ-MOTIVE-8:** Critical Success MUST reveal:
- Next intended action (from heuristic, see 4.2.5)
- Unused special abilities list
- Resistances and weaknesses

**REQ-MOTIVE-9:** Success MUST reveal:
- Next intended action only

**REQ-MOTIVE-10:** Failure MUST reveal:
- No information

**REQ-MOTIVE-11:** Critical Failure MUST:
- Provide no information
- Apply +2 circumstance bonus to NPC's next attack against the player

#### 4.2.4 Out-of-Combat Outcomes

**REQ-MOTIVE-12:** Critical Success or Success MUST reveal:
- NPC disposition (hostile/wary/neutral/friendly)

**REQ-MOTIVE-13:** Failure or Critical Failure MUST reveal:
- No information

**REQ-MOTIVE-14:** Critical Failure MUST:
- Change NPC disposition to hostile if currently neutral or wary

#### 4.2.5 NPC "Next Intended Action" Heuristic

Apply the following heuristic in priority order:

```
if HP% < 25:
  return "looks ready to flee"
else if has unused special ability:
  return "seems to be holding something back"
else:
  return "looks focused on the fight"
```

**REQ-MOTIVE-15:** MUST check NPC `Abilities []string` field (existing or to be added).

**REQ-MOTIVE-16:** MUST check NPC `Resistances []string` and `Weaknesses []string` fields (existing or to be added).

#### 4.2.6 Required NPC Fields

Verify the following fields exist on `npc.Template` and `npc.Instance`; add if missing:

- `Abilities []string` — list of available special abilities
- `Resistances []string` — list of damage type resistances
- `Weaknesses []string` — list of damage type weaknesses

**REQ-MOTIVE-17:** MUST add missing fields to `npc.Template` with YAML tags: `abilities`, `resistances`, `weaknesses`.

---

## 5. Feature 4: Delay (New Command)

### 5.1 Problem Statement

No delay command exists in the current implementation. Players cannot bank Action Points (AP) for strategic advantage in future combat rounds.

### 5.2 Design

#### 5.2.1 Session State Extension

Add new field to `PlayerSession`:

```go
type PlayerSession struct {
    // ... existing fields ...
    BankedAP int // AP reserved for next round (session-only, not persisted)
}
```

**REQ-DELAY-1:** Field MUST be session-only (not persisted to database).

#### 5.2.2 Command Behavior

**Syntax:** `delay`

**Availability:**
- REQ-DELAY-2: MUST be available only during active combat
- REQ-DELAY-3: MUST fail with message "`You cannot delay outside of combat.`" if used out of combat

**Action Economy:**
- REQ-DELAY-4: MUST cost 1 AP

**Effect:**
- REQ-DELAY-5: MUST spend 1 AP (fail if player has < 1 AP)
- REQ-DELAY-6: MUST bank all remaining AP, capped at 2: `sess.BankedAP = min(sess.RemainingAP - 1, 2)`
- REQ-DELAY-7: MUST zero current round AP: `sess.RemainingAP = 0`
- REQ-DELAY-8: MUST apply -2 AC penalty until start of player's next turn
- REQ-DELAY-9: MUST display message: `"You delay, banking {N} AP for next round. You are exposed (-2 AC)."`

#### 5.2.3 Round Start Logic

In the combat handler's round-start function, before awarding AP for the new round:

**REQ-DELAY-10:** MUST add banked AP to current round: `sess.RemainingAP += sess.BankedAP`.

**REQ-DELAY-11:** MUST clear banked AP: `sess.BankedAP = 0`.

**REQ-DELAY-12:** MUST remove any `-2 AC` penalty applied by delay (new `delayed` condition or inline penalty tracking).

#### 5.2.4 AC Penalty Implementation

Choose one approach and document:

**Option A (Condition-based):** Create a new `delayed` condition that grants -2 AC. Remove at round start.

**Option B (Inline penalty):** Add `DelayedUntilRound int` field to session; apply -2 AC in combat AC calculation when `current_round <= sess.DelayedUntilRound`.

**REQ-DELAY-13:** MUST document chosen AC penalty mechanism.

---

## 6. Feature 5: Rename Cleanup

### 6.1 Skill Name Updates

**REQ-RENAME-1:** MUST replace all occurrences of `sess.Skills["athletics"]` with `sess.Skills["muscle"]` in:
- `handleClimb`
- `handleSwim`
- All handler test files
- Any other skill check in combat/exploration

**REQ-RENAME-2:** MUST replace all message strings referencing "athletics" with "muscle".

### 6.2 NPC Deception → Hustle

**REQ-RENAME-3:** MUST replace all occurrences of `inst.Deception` with `inst.Hustle`.

**REQ-RENAME-4:** MUST replace all occurrences of `template.Deception` with `template.Hustle`.

**REQ-RENAME-5:** MUST update YAML tag from `deception` to `hustle` in NPC struct.

**REQ-RENAME-6:** MUST replace all message strings referencing "deception" with "hustle".

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

- **REQ-T1:** Climb success MUST move player to destination room
- **REQ-T2:** Climb critical failure MUST deal height-based fall damage and apply `prone` condition (in combat)
- **REQ-T3:** Terrain default table MUST provide correct DC when exit omits explicit DC
- **REQ-T6 (property):** For any exit height in [0, 100], fall damage MUST equal `max(1, floor(height/10))` d6

### 7.2 Swim Tests

- **REQ-T4:** Swim success MUST move player through water exit (or surface if submerged)
- **REQ-T5:** Swim critical failure MUST deal 1d6 drowning damage and apply `submerged` condition
- **REQ-T3:** Terrain default table MUST provide correct DC when exit omits explicit DC

### 7.3 Sense Motive Tests

- **REQ-T7:** Motive success in combat MUST reveal next intended action message
- **REQ-T8:** Motive critical failure in combat MUST apply +2 circumstance bonus to NPC's next attack
- **REQ-T9:** Motive success out of combat MUST reveal NPC disposition
- **REQ-T10:** Motive critical failure out of combat with neutral/wary NPC MUST change disposition to hostile

### 7.4 Delay Tests

- **REQ-T11:** Delay in combat MUST bank remaining AP (capped at 2), zero current AP, and apply AC penalty
- **REQ-T12:** Banked AP at round start MUST be awarded and cleared
- **REQ-T13 (property):** For any remaining AP in [0, 10], banked AP after delay MUST equal `min(remaining - 1, 2)`

### 7.5 Test Strategy

**REQ-SWENG-5a:** All tests MUST use Property-Based Testing where applicable:
- Fall damage formula (REQ-T6)
- AP banking formula (REQ-T13)
- Terrain default table coverage (REQ-T3, REQ-T5)

**REQ-SWENG-6:** All tests MUST pass with 100% success before marking feature complete.

---

## 8. Integration Points

### 8.1 Commands Registry

**REQ-CMD-1:** Add `HandlerClimb`, `HandlerSwim`, `HandlerMotive`, `HandlerDelay` constants to `internal/game/command/commands.go`.

**REQ-CMD-2:** Append `Command{...}` entries to `BuiltinCommands()` in `internal/game/command/commands.go`.

### 8.2 Proto Messages

**REQ-CMD-4:** Add proto request messages to `api/proto/game/v1/game.proto`:
- `ClimbRequest` with `direction` field
- `SwimRequest` with `direction` field
- `MotiveRequest` with `target` field
- `DelayRequest` (no fields)

**REQ-CMD-4:** Add all messages to the `ClientMessage` oneof.

**REQ-CMD-4:** Run `make proto` to regenerate.

### 8.3 Frontend Bridge

**REQ-CMD-5:** Add bridge functions to `internal/frontend/handlers/bridge_handlers.go`:
- `bridgeClimb(session, msg)`
- `bridgeSwim(session, msg)`
- `bridgeMotive(session, msg)`
- `bridgeDelay(session, msg)`

**REQ-CMD-5:** Register all functions in `bridgeHandlerMap`.

**REQ-CMD-5:** Ensure test `TestAllCommandHandlersAreWired` passes.

### 8.4 gRPC Service

**REQ-CMD-6:** Implement handlers in `internal/gameserver/grpc_service.go`:
- `handleClimb(ctx, uid, msg)`
- `handleSwim(ctx, uid, msg)`
- `handleMotive(ctx, uid, msg)`
- `handleDelay(ctx, uid, msg)`

**REQ-CMD-6:** Wire all handlers into the `dispatch` type switch.

### 8.5 Completion Criteria

**REQ-CMD-7:** All steps (CMD-1 through CMD-6) MUST be completed and all tests MUST pass before any command is considered done. A command registered in `BuiltinCommands()` but not wired end-to-end is a defect.

---

## 9. Migration Checklist

- [ ] Add `ClimbDC` and `Height` to exit struct (YAML tags)
- [ ] Add `SwimDC` to exit struct (YAML tag)
- [ ] Add `Abilities`, `Resistances`, `Weaknesses` to `npc.Template` (YAML tags)
- [ ] Rename `npc.Template.Deception` → `npc.Template.Hustle` (YAML tag: `hustle`)
- [ ] Rename `npc.Instance.Deception` → `npc.Instance.Hustle`
- [ ] Add `BankedAP` field to `PlayerSession` (session-only)
- [ ] Remove room property fallbacks: `climbable`, `climb_dc`, `water_terrain`, `water_dc`
- [ ] Update all YAML room and NPC definitions
- [ ] Update all handler code
- [ ] Update all test code
- [ ] Update all message strings
- [ ] Add proto messages and run `make proto`
- [ ] Implement command handlers
- [ ] Add bridge functions
- [ ] Wire gRPC handlers
- [ ] Run full test suite (100% pass)
- [ ] Commit with message: `feat(combat): climb/swim rework, sense motive redesign, delay command`

---

## 10. References

- Gunchete System: Post-apocalyptic reskin of Pathfinder 2e
- Critical Success/Failure: `combat.OutcomeFor(total, dc)` returns `CritSuccess | Success | Failure | CritFailure`
- Command Registry: `internal/game/command/commands.go`
- Handler Pattern: `internal/game/command/*.go` with `Handle<Name>(uid, inst, session) error`
- Bridge Pattern: `internal/frontend/handlers/bridge_handlers.go` with `bridge<Name>(session, msg) error`
- gRPC Pattern: `internal/gameserver/grpc_service.go` with `handle<Name>(ctx, uid, msg) error` and type switch dispatch

---

**Approved for Implementation:** Ready for subagent-driven development.
