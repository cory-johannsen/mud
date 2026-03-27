# Exploration Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the Exploration Actions feature by implementing the `refocus` command — the only remaining unimplemented item. All seven exploration modes (Lay Low, Hold Ground, Active Sensors, Case It, Run Point, Shadow, Poke Around) and the Shadow skill-rank borrow are already fully implemented.

**Architecture:** `refocus` is a new out-of-combat command that starts a 1-minute real-time timer (same pattern as camping). `RefocusingActive bool` and `RefocusingStartTime time.Time` are added to `PlayerSession`. The game clock tick calls `checkRefocusStatus` to detect completion and restore 1 Focus Point. Cancels on combat start or room movement. On completion, `SaveFocusPoints` persists the change.

**Tech Stack:** Go, testify, rapid (property tests), `pgregory.net/rapid`

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/game/session/manager.go` | Add `RefocusingActive bool`, `RefocusingStartTime time.Time` |
| Modify | `internal/gameserver/grpc_service.go` | Add `handleRefocus`, `checkRefocusStatus`; wire into command dispatch and clock tick; cancel on combat start and move |
| Modify | `internal/gameserver/grpc_service_test.go` or new `grpc_service_refocus_test.go` | Tests for all refocus paths |
| Modify | `docs/features/exploration-actions.md` | Mark Refocus checkbox done |
| Modify | `docs/features/index.yaml` | Set status: done |

---

### Task 1: Add RefocusingActive/RefocusingStartTime to PlayerSession

**Files:**
- Modify: `internal/game/session/manager.go`
- Test: `internal/game/session/manager_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/game/session/manager_test.go`, add:

```go
func TestPlayerSession_RefocusingFields_DefaultZero(t *testing.T) {
    sess := &PlayerSession{}
    assert.False(t, sess.RefocusingActive)
    assert.True(t, sess.RefocusingStartTime.IsZero())
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
mise exec -- go test ./internal/game/session/... -run TestPlayerSession_RefocusingFields -v
```

Expected: FAIL — fields not yet defined.

- [ ] **Step 3: Add fields to PlayerSession**

In `internal/game/session/manager.go`, alongside the camping fields (search for `CampingActive`), add:

```go
// Refocus state — transient; not persisted.
RefocusingActive    bool      // true while a refocus rest is in progress (REQ-EXP-REFOCUS-1)
RefocusingStartTime time.Time // when refocus started
```

- [ ] **Step 4: Run test to verify it passes**

```bash
mise exec -- go test ./internal/game/session/... -run TestPlayerSession_RefocusingFields -v
```

Expected: PASS.

- [ ] **Step 5: Build check**

```bash
mise exec -- go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/game/session/manager.go internal/game/session/manager_test.go
git commit -m "feat(exploration-actions): add RefocusingActive/RefocusingStartTime to PlayerSession"
```

---

### Task 2: Implement `handleRefocus` and `checkRefocusStatus`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_refocus_test.go`

The `refocus` command is dispatched from the same place as `handleRest`. Search `grpc_service.go` for `"rest"` in the command dispatch switch to find the right location. Add a `"refocus"` case.

`handleRefocus` starts a 1-minute real-time timer. `checkRefocusStatus` is called on each clock tick (alongside `checkCampingStatus`). On completion, 1 Focus Point is restored and `SaveFocusPoints` is called.

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_refocus_test.go`:

```go
package gameserver

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/session"
)

// newRefocusTestSvc creates a minimal GameServiceServer for refocus tests.
// Follow the pattern from grpc_service_rest_test.go (newRestTestSvc or similar).
func newRefocusTestSvc(t *testing.T) (*GameServiceServer, string) {
    t.Helper()
    // Build a safe-room world, session manager, mock char saver.
    // Return server + uid.
    // Look at how grpc_service_rest_test.go builds its test server.
    panic("implement me — replace with actual test server setup following grpc_service_rest_test.go pattern")
}

func TestHandleRefocus_BlockedInCombat(t *testing.T) {
    svc, uid := newRefocusTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.Status = statusInCombat

    err := svc.handleRefocus(uid)
    require.NoError(t, err)
    assert.False(t, sess.RefocusingActive, "should not start refocus in combat")
}

func TestHandleRefocus_BlockedAtMaxFocusPoints(t *testing.T) {
    svc, uid := newRefocusTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.FocusPoints = 3
    sess.MaxFocusPoints = 3

    err := svc.handleRefocus(uid)
    require.NoError(t, err)
    assert.False(t, sess.RefocusingActive, "should not start refocus when already at max")
}

func TestHandleRefocus_StartsTimer(t *testing.T) {
    svc, uid := newRefocusTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.FocusPoints = 0
    sess.MaxFocusPoints = 2

    err := svc.handleRefocus(uid)
    require.NoError(t, err)
    assert.True(t, sess.RefocusingActive)
    assert.False(t, sess.RefocusingStartTime.IsZero())
}

func TestHandleRefocus_AlreadyRefocusing(t *testing.T) {
    svc, uid := newRefocusTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.RefocusingActive = true
    sess.RefocusingStartTime = time.Now().Add(-30 * time.Second)

    err := svc.handleRefocus(uid)
    require.NoError(t, err)
    // Still active, not reset
    assert.True(t, sess.RefocusingActive)
}

func TestCheckRefocusStatus_CompletesAfterOneMinute(t *testing.T) {
    svc, uid := newRefocusTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.FocusPoints = 1
    sess.MaxFocusPoints = 3
    sess.RefocusingActive = true
    sess.RefocusingStartTime = time.Now().Add(-90 * time.Second) // past 1-minute threshold

    svc.checkRefocusStatus(uid)

    assert.False(t, sess.RefocusingActive, "refocus should complete")
    assert.Equal(t, 2, sess.FocusPoints, "should restore 1 focus point")
}

func TestCheckRefocusStatus_NotYetComplete(t *testing.T) {
    svc, uid := newRefocusTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.FocusPoints = 0
    sess.MaxFocusPoints = 2
    sess.RefocusingActive = true
    sess.RefocusingStartTime = time.Now().Add(-20 * time.Second) // only 20s elapsed

    svc.checkRefocusStatus(uid)

    assert.True(t, sess.RefocusingActive, "should still be refocusing")
    assert.Equal(t, 0, sess.FocusPoints, "no FP restored yet")
}

func TestCheckRefocusStatus_CapsAtMaxFocusPoints(t *testing.T) {
    svc, uid := newRefocusTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.FocusPoints = 2
    sess.MaxFocusPoints = 2 // already at max (shouldn't happen but guard it)
    sess.RefocusingActive = true
    sess.RefocusingStartTime = time.Now().Add(-90 * time.Second)

    svc.checkRefocusStatus(uid)

    assert.False(t, sess.RefocusingActive)
    assert.Equal(t, 2, sess.FocusPoints, "should not exceed MaxFocusPoints")
}
```

NOTE: Replace `panic("implement me...")` in `newRefocusTestSvc` with the actual setup — read `grpc_service_rest_test.go` to find `newRestTestSvc` (or equivalent) and follow the same pattern.

- [ ] **Step 2: Run tests to verify they fail (compile error expected)**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleRefocus|TestCheckRefocus -v 2>&1 | head -20
```

Expected: compile error — `handleRefocus` and `checkRefocusStatus` undefined.

- [ ] **Step 3: Implement `handleRefocus`**

In `internal/gameserver/grpc_service.go`, add alongside `handleRest` (search for `func (s *GameServiceServer) handleRest`):

```go
// RefocusDuration is the real-time duration of a Refocus action.
// REQ-EXP-REFOCUS-1: 1 minute real time (proxy for 10 in-game minutes).
const RefocusDuration = 1 * time.Minute

// handleRefocus starts a Refocus action — a 1-minute out-of-combat rest that restores 1 Focus Point.
// Precondition: uid refers to a connected player session.
// Postcondition: sess.RefocusingActive = true and sess.RefocusingStartTime set, or error message sent.
func (s *GameServiceServer) handleRefocus(uid string) error {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return fmt.Errorf("session not found")
    }

    // Block in combat.
    if sess.Status == statusInCombat {
        s.pushMessageToUID(uid, "You cannot refocus during combat.")
        return nil
    }

    // Already refocusing — show time remaining.
    if sess.RefocusingActive {
        elapsed := time.Since(sess.RefocusingStartTime)
        remaining := RefocusDuration - elapsed
        if remaining < 0 {
            remaining = 0
        }
        s.pushMessageToUID(uid, fmt.Sprintf("You are already refocusing. Time remaining: %s.", remaining.Round(time.Second)))
        return nil
    }

    // Already at max focus points.
    if sess.MaxFocusPoints > 0 && sess.FocusPoints >= sess.MaxFocusPoints {
        s.pushMessageToUID(uid, "Your focus is already at its maximum.")
        return nil
    }

    // No focus points to restore (character has no focus pool).
    if sess.MaxFocusPoints == 0 {
        s.pushMessageToUID(uid, "You have no focus pool to refill.")
        return nil
    }

    sess.RefocusingActive = true
    sess.RefocusingStartTime = time.Now()
    s.pushMessageToUID(uid, fmt.Sprintf("You take a moment to refocus. You will restore 1 Focus Point in %s.", RefocusDuration.Round(time.Second)))
    return nil
}
```

- [ ] **Step 4: Implement `checkRefocusStatus`**

In `internal/gameserver/grpc_service.go`, add alongside `checkCampingStatus`:

```go
// checkRefocusStatus is called on each game clock tick.
// Completes refocus if 1 minute has elapsed, restoring 1 Focus Point.
// Precondition: uid refers to a connected player session.
// Postcondition: if elapsed, FocusPoints increased by 1 (capped at MaxFocusPoints) and persisted.
func (s *GameServiceServer) checkRefocusStatus(uid string) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok || !sess.RefocusingActive {
        return
    }

    if time.Since(sess.RefocusingStartTime) < RefocusDuration {
        return
    }

    sess.RefocusingActive = false
    sess.RefocusingStartTime = time.Time{}

    if sess.FocusPoints < sess.MaxFocusPoints {
        sess.FocusPoints++
        if s.charSaver != nil {
            if err := s.charSaver.SaveFocusPoints(context.Background(), sess.CharacterID, sess.FocusPoints); err != nil {
                if s.logger != nil {
                    s.logger.Warn("checkRefocusStatus: failed to save focus points", zap.Error(err))
                }
            }
        }
        s.pushMessageToUID(uid, fmt.Sprintf("You feel your focus restore. (%d/%d Focus Points)", sess.FocusPoints, sess.MaxFocusPoints))
    } else {
        s.pushMessageToUID(uid, "You feel centered but your focus was already full.")
    }
}
```

- [ ] **Step 5: Wire `checkRefocusStatus` into the clock tick**

Find the clock tick loop that calls `checkCampingStatus`. Add `checkRefocusStatus` alongside it:

```go
for _, uid := range s.sessions.AllUIDs() {
    s.checkDowntimeCompletion(uid)
    s.checkCampingStatus(uid)
    s.checkRefocusStatus(uid) // REQ-EXP-REFOCUS-1
}
```

- [ ] **Step 6: Wire `handleRefocus` into the command dispatch**

Find the command dispatch switch (search for `case "rest":` in `grpc_service.go`). Add:

```go
case "refocus":
    return s.handleRefocus(uid)
```

- [ ] **Step 7: Cancel refocus on combat start**

Find where camping is cancelled on combat start (search for `CampingActive` near combat initiation in `combat_handler.go` or `grpc_service.go`). Add analogous cancellation:

```go
// Cancel refocus if combat starts.
if sess.RefocusingActive {
    sess.RefocusingActive = false
    sess.RefocusingStartTime = time.Time{}
    s.pushMessageToUID(uid, "Your focus is broken — combat has started!")
}
```

- [ ] **Step 8: Cancel refocus on movement**

Find where camping is cancelled in `handleMove` (search for `cancelCamping` in `handleMove`). Add:

```go
// Cancel refocus if player moves.
if sess.RefocusingActive {
    sess.RefocusingActive = false
    sess.RefocusingStartTime = time.Time{}
    s.pushMessageToUID(uid, "You stop refocusing.")
}
```

- [ ] **Step 9: Fix test setup — implement `newRefocusTestSvc`**

Read `internal/gameserver/grpc_service_rest_test.go` to find how `newRestTestSvc` (or equivalent) builds the test server. Replicate the pattern in `newRefocusTestSvc`. The server needs:
- `sessions` (`*session.Manager`)
- `charSaver` (mock implementing `SaveFocusPoints` + `SaveState`)
- A player session with `UID` set

- [ ] **Step 10: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestHandleRefocus|TestCheckRefocus" -v
```

Expected: all PASS.

- [ ] **Step 11: Run full gameserver suite**

```bash
mise exec -- go test ./internal/gameserver/... -count=1 -timeout 120s
```

Expected: all PASS.

- [ ] **Step 12: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_refocus_test.go
git commit -m "feat(exploration-actions): implement refocus command (REQ-EXP-REFOCUS-1)"
```

---

### Task 3: Mark feature done

**Files:**
- Modify: `docs/features/exploration-actions.md`
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Update exploration-actions.md**

In `docs/features/exploration-actions.md`, mark the Refocus checkbox as complete:

```markdown
  - [x] Refocus command — implement `refocus` (unavailable in combat; costs in-game time; restores 1 Focus Point; requires Focus Point system and in-game time tracking)
```

Also mark all the exploration mode checkboxes (Avoid Notice, Defend, Detect Magic, Search, Scout, Follow the Expert, Investigate) as complete since those modes are fully implemented.

- [ ] **Step 2: Update index.yaml**

In `docs/features/index.yaml`, change:

```yaml
  - slug: exploration-actions
    name: Exploration Actions
    status: planned
```

to:

```yaml
  - slug: exploration-actions
    name: Exploration Actions
    status: done
```

- [ ] **Step 3: Commit**

```bash
git add docs/features/exploration-actions.md docs/features/index.yaml
git commit -m "docs: mark exploration-actions as done"
```

---

## Verification Checklist

- [ ] `refocus` blocked in combat with message
- [ ] `refocus` blocked when `MaxFocusPoints == 0` (no focus pool)
- [ ] `refocus` blocked when already at max focus points
- [ ] `refocus` while already refocusing shows time remaining
- [ ] `refocus` starts 1-minute timer; `RefocusingActive = true`
- [ ] Completing 1-minute timer restores 1 FP (capped at max); `SaveFocusPoints` called
- [ ] Combat start cancels refocus with message
- [ ] Movement cancels refocus with message
- [ ] Full test suite passes with zero failures
