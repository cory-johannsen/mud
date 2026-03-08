# Console Notifications Design

## Goal

Notify players in the console every time a skill is used automatically and every time XP is awarded.

## Architecture

Two targeted changes, no new files:

1. **Skill check notifications** — `applyRoomSkillChecks` and `applyNPCSkillChecks` in `internal/gameserver/grpc_service.go` prepend a mechanical detail line to the messages they return, before any YAML outcome message.

2. **XP award notifications** — `AwardRoomDiscovery` and `AwardSkillCheck` in `internal/game/xp/service.go` prepend an XP grant message to their returned `[]string` slice. `AwardSkillCheck` gains a `skillName string` parameter so it can name the skill in the message.

Kill XP is already notified (built at the call site in `combat_handler.go`) and requires no change.

## Tech Stack

Go; no new dependencies.

---

## Section 1: Automatic skill check notifications

**Where:** `applyRoomSkillChecks` and `applyNPCSkillChecks` in `internal/gameserver/grpc_service.go`.

**Format:**
```
{SkillName} check (DC {dc}): rolled {roll}+{bonus}={total} — {outcome}.
```

Example: `Parkour check (DC 12): rolled 14+2=16 — success.`

Outcome strings (matching existing action system): `critical success`, `success`, `failure`, `critical failure`.

**Implementation:** After `skillcheck.Resolve(...)` returns, build the detail line from `trigger.Skill`, `trigger.DC`, `roll`, `amod` (bonus), `result.Total`, and `result.Outcome.String()`. Prepend it to `msgs` before appending `outcome.Message`.

The existing YAML outcome message (if any) still appears on the next line. No YAML content changes required.

---

## Section 2: XP award notifications

**Where:** `internal/game/xp/service.go`.

**`AwardRoomDiscovery`:** Prepend `"You gain X XP for discovering a new room."` to the returned `[]string`.

**`AwardSkillCheck`:** Add `skillName string` parameter. Prepend `"You gain X XP for the {SkillName} check."` to the returned `[]string`.

**Call site update:** All callers of `AwardSkillCheck` in `grpc_service.go` pass `trigger.Skill` as the new first argument.

**No change to `AwardKill`** — already notified at the call site in `combat_handler.go`.

---

## Testing

- Unit tests for `AwardRoomDiscovery` and `AwardSkillCheck` verify the XP grant message is the first element of the returned slice (SWENG-5a property test: any valid XP amount produces a non-empty first message matching expected prefix).
- Unit tests for `applyRoomSkillChecks` / `applyNPCSkillChecks` (or a helper) verify the skill detail line format.
- Existing XP service tests updated to account for the new prepended message.
