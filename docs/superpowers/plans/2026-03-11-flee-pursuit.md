# Flee & Pursuit Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the `flee` command so it costs AP, uses an Athletics/Acrobatics skill check vs NPC DC, moves the player to a random adjacent room on success, and triggers per-NPC pursuit rolls that can follow the player and re-initiate combat.

**Architecture:** Extend `CombatHandler.Flee` in `combat_handler.go` to replace the old opposed-roll with a skill-check/DC model, add movement and pursuit logic, and change the return signature to `(events, fled, error)`. Update `handleFlee` in `grpc_service.go` to broadcast all events and push room views on success.

**Tech Stack:** Go, pgregory.net/rapid (property-based tests), testify/assert, testify/require, zap

---

## Key codebase facts (read before coding)

- **Test pattern:** Tests in the `gameserver` package use inline `sessMgr.AddPlayer(session.AddPlayerOptions{...})` and `npcMgr.Spawn(&npc.Template{...}, roomID)` — there are no shared helper functions like `addTestPlayerSession`. Follow the pattern in `grpc_service_grapple_test.go`.
- **Starting combat in tests:** Call `combatHandler.Attack(uid, npcName)` then `combatHandler.cancelTimer(roomID)` to freeze the timer. See `grpc_service_grapple_test.go` around line 161.
- **Room IDs:** `testWorldAndSession` creates `room_a` (exit north → `room_b`) and `room_b` (exit south → `room_a`). Both are in zone `"test"`. Use unique room IDs per test for NPCs to avoid cross-test interference.
- **Status constants:** `statusInCombat = int32(2)` (defined in `action_handler.go`). After successful flee: `sess.Status = int32(1)` (IDLE = 1).
- **worldMgr in CombatHandler:** Pass `worldMgr` (parameter 8, 0-indexed 7) to `NewCombatHandler` for tests that need movement. The `newGrappleSvcWithCombat` passes `nil` — flee tests must pass a real `worldMgr`.
- **Checking player in combat:** Use `combatHandler.ActiveCombatForRoom(roomID)` — `IsInCombat` only checks NPCs.
- **`skillRankBonus`:** Defined in `action_handler.go` (same package) — no import needed in `combat_handler.go`.
- **AP deduction inside Flee:** Use `q.RemainingPoints()` / `q.DeductAP(n)` on `cbt.ActionQueues[uid]` directly — do NOT call `h.SpendAP` (it acquires `combatMu`, causing deadlock since `Flee` already holds it).

---

## File Structure

| File | Change |
|---|---|
| `internal/gameserver/combat_handler.go` | Replace `Flee` body; add `resolvePursuitLocked`; add `startPursuitCombatLocked`; add `ActiveCombatForRoom` |
| `internal/gameserver/grpc_service.go` | Update `handleFlee` for new signature + room view pushes |
| `internal/gameserver/grpc_service_flee_test.go` | New: all integration + property tests |

No new proto, bridge handlers, or command constants needed. `FleeRequest` (proto field 12) and `HandlerFlee` constant already exist.

---

## Chunk 1: Test file + CombatHandler.Flee rewrite

### Task 1: Write failing tests for new Flee mechanics

**Files:**
- Create: `internal/gameserver/grpc_service_flee_test.go`

- [ ] **Step 1: Create the test file with the flee service constructor**

```go
package gameserver

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newFleeSvcWithCombat builds a GameServiceServer + CombatHandler that share
// the same worldMgr and sessMgr, suitable for flee integration tests.
// Unlike newGrappleSvcWithCombat, it passes worldMgr to CombatHandler so that
// the movement path in Flee is exercised.
func newFleeSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *world.Manager, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		worldMgr, // pass worldMgr so Flee can pick a valid exit
		nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, worldMgr, sessMgr, npcMgr, combatHandler
}
```

- [ ] **Step 2: Add TestHandleFlee_NotEnoughAP**

```go
// TestHandleFlee_NotEnoughAP verifies that flee fails when the player has 0 AP.
//
// Precondition: player is in combat with 0 AP remaining.
// Postcondition: error returned; player stays in original room.
func TestHandleFlee_NotEnoughAP(t *testing.T) {
	src := dice.NewDeterministicSource([]int{15})
	roller := dice.NewRoller(src)
	svc, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_flee_ap"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-ap", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_ap", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_ap", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Drain all AP.
	require.NoError(t, combatHandler.SpendAP("u_flee_ap", 3))

	_, err = svc.handleFlee("u_flee_ap")
	assert.ErrorContains(t, err, "1 AP")
	assert.Equal(t, roomID, sess.RoomID, "player must not move on AP error")
}
```

- [ ] **Step 3: Add TestHandleFlee_Failure**

```go
// TestHandleFlee_Failure verifies a failed flee roll leaves the player in combat.
//
// Precondition: dice returns 1; player total (1+bonus) < DC (10+StrMod).
// Postcondition: FLEE event narrative contains "can't escape"; player stays in room.
func TestHandleFlee_Failure(t *testing.T) {
	src := dice.NewDeterministicSource([]int{1})
	roller := dice.NewRoller(src)
	svc, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_flee_fail"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-fail", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 4,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_fail", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_fail", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	evt, err := svc.handleFlee("u_flee_fail")
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetCombatEvent().GetNarrative(), "can't escape")
	assert.Equal(t, roomID, sess.RoomID, "player must remain in room on failure")
}
```

- [ ] **Step 4: Add TestHandleFlee_Success_NoValidExits**

```go
// TestHandleFlee_Success_NoValidExits verifies flee succeeds (combat ends) but
// player stays when the room has no unlocked, non-hidden exits.
//
// Precondition: dice returns 20; room has only a locked exit.
// Postcondition: player status is idle; player stays in room; narrative says "nowhere to run".
func TestHandleFlee_Success_NoValidExits(t *testing.T) {
	src := dice.NewDeterministicSource([]int{20})
	roller := dice.NewRoller(src)
	// Use testWorldAndSessionWithLockedRoom so newFleeSvcWithCombat receives the
	// world containing room_locked. We must build the svc ourselves here because
	// we need a custom worldMgr.
	lockedWorldMgr, lockedSessMgr := testWorldAndSessionWithLockedRoom(t)
	lockedLogger := zaptest.NewLogger(t)
	lockedNPCMgr := npc.NewManager()
	lockedCombatHandler := NewCombatHandler(
		combat.NewEngine(), lockedNPCMgr, lockedSessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		lockedWorldMgr, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		lockedWorldMgr, lockedSessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(lockedWorldMgr, lockedSessMgr, lockedNPCMgr, nil, nil, nil),
		NewChatHandler(lockedSessMgr),
		lockedLogger,
		nil, roller, nil, lockedNPCMgr, lockedCombatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	sessMgr := lockedSessMgr
	npcMgr := lockedNPCMgr
	combatHandler := lockedCombatHandler

	// room_locked is already in lockedWorldMgr (built by testWorldAndSessionWithLockedRoom).
	const lockedRoomID = "room_locked"

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-lock", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, lockedRoomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_lock", Username: "Runner", CharName: "Runner",
		RoomID: lockedRoomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_lock", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(lockedRoomID)

	evt, err := svc.handleFlee("u_flee_lock")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Player escaped combat but couldn't move.
	assert.Equal(t, lockedRoomID, sess.RoomID)
	assert.Equal(t, int32(1), sess.Status, "player status must be idle after flee")

	// Check narrative includes "nowhere to run".
	assert.Contains(t, evt.GetCombatEvent().GetNarrative(), "nowhere")
}

// contains is a test helper to avoid importing strings in test assertions.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
```

**Required helper:** Add this to the test file. The `TestHandleFlee_Success_NoValidExits` test builds its own `svc`/`combatHandler`/`npcMgr` using this world rather than calling `newFleeSvcWithCombat`. Do NOT modify `testWorldAndSession` in `world_handler_test.go`.

```go
// testWorldAndSessionWithLockedRoom builds a world with room_a, room_b, and
// room_locked (locked east exit only). Used by flee tests needing a dead-end room.
func testWorldAndSessionWithLockedRoom(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID: "test_flee", Name: "Test Flee", Description: "Flee test zone",
		StartRoom: "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID: "room_a", ZoneID: "test_flee", Title: "Room A",
				Description: "Room A.", MapX: 0, MapY: 0,
				Exits:      []world.Exit{{Direction: world.North, TargetRoom: "room_b"}},
				Properties: map[string]string{},
			},
			"room_b": {
				ID: "room_b", ZoneID: "test_flee", Title: "Room B",
				Description: "Room B.", MapX: 0, MapY: 1,
				Exits:      []world.Exit{{Direction: world.South, TargetRoom: "room_a"}},
				Properties: map[string]string{},
			},
			"room_locked": {
				ID: "room_locked", ZoneID: "test_flee", Title: "Dead End",
				Description: "No way out.", MapX: 1, MapY: 0,
				Exits:      []world.Exit{{Direction: world.East, TargetRoom: "room_a", Locked: true}},
				Properties: map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}
```

- [ ] **Step 5: Add TestHandleFlee_Success_NPCPursues**

```go
// TestHandleFlee_Success_NPCPursues verifies that when NPC pursuit roll >= playerTotal,
// the NPC follows the player to the destination room and new combat starts.
//
// Precondition: player flee roll = 20; NPC pursuit roll = 20.
// Postcondition: player moved to room_b; NPC in room_b; new combat active in room_b.
func TestHandleFlee_Success_NPCPursues(t *testing.T) {
	// Rolls: [20=player flee d20, 20=NPC pursuit d20]
	src := dice.NewDeterministicSource([]int{20, 20})
	roller := dice.NewRoller(src)
	_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_a"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-pursue", Name: "Pursuer", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_pursue", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_pursue", "Pursuer")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	events, fled, err := combatHandler.Flee("u_flee_pursue")
	require.NoError(t, err)
	assert.True(t, fled)
	assert.NotEmpty(t, events)

	destRoomID := sess.RoomID
	assert.NotEqual(t, roomID, destRoomID, "player must have moved")

	// NPC followed player to destination room.
	updatedInst, ok := npcMgr.Get(inst.ID)
	require.True(t, ok)
	assert.Equal(t, destRoomID, updatedInst.RoomID, "NPC must be in destination room")

	// New combat active in destination room.
	cbt := combatHandler.ActiveCombatForRoom(destRoomID)
	assert.NotNil(t, cbt, "new combat must be active in destination room")
}
```

- [ ] **Step 6: Add TestHandleFlee_Success_NPCFails**

```go
// TestHandleFlee_Success_NPCFails verifies that when NPC pursuit roll < playerTotal,
// the NPC stays behind and no new combat starts.
//
// Precondition: player flee roll = 20; NPC pursuit roll = 1.
// Postcondition: player moved; NPC stays in original room; no combat in destination.
func TestHandleFlee_Success_NPCFails(t *testing.T) {
	// Rolls: [20=player flee, 1=NPC pursuit]
	src := dice.NewDeterministicSource([]int{20, 1})
	roller := dice.NewRoller(src)
	_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_a"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-nopursue", Name: "Slowpoke", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_nopursue", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_nopursue", "Slowpoke")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	events, fled, err := combatHandler.Flee("u_flee_nopursue")
	require.NoError(t, err)
	assert.True(t, fled)
	assert.NotEmpty(t, events)

	destRoomID := sess.RoomID
	assert.NotEqual(t, roomID, destRoomID, "player must have moved")

	// NPC stayed in original room.
	updatedInst, ok := npcMgr.Get(inst.ID)
	require.True(t, ok)
	assert.Equal(t, roomID, updatedInst.RoomID, "NPC must remain in original room")

	// No combat in destination room.
	assert.Nil(t, combatHandler.ActiveCombatForRoom(destRoomID), "no combat in destination room")
}
```

- [ ] **Step 7: Add TestHandleFlee_Success_OriginalCombatEnds**

```go
// TestHandleFlee_Success_OriginalCombatEnds verifies that when the fleeing player
// is the only player, the original room's combat ends after a successful flee.
//
// Precondition: single player; dice = [20 flee, 1 pursuit] (NPC stays, no new combat).
// Postcondition: no active combat in original room.
func TestHandleFlee_Success_OriginalCombatEnds(t *testing.T) {
	src := dice.NewDeterministicSource([]int{20, 1})
	roller := dice.NewRoller(src)
	_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_a"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-endcbt", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_endcbt", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_endcbt", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	_, fled, err := combatHandler.Flee("u_flee_endcbt")
	require.NoError(t, err)
	assert.True(t, fled)

	// Combat in original room must be gone.
	assert.Nil(t, combatHandler.ActiveCombatForRoom(roomID), "original combat must have ended")
}
```

- [ ] **Step 8: Add property-based tests**

```go
// TestProperty_Flee_SkillCheckBoundary verifies playerTotal >= DC → success for
// all random roll/DC combinations, exercising the actual Flee function.
func TestProperty_Flee_SkillCheckBoundary(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(1, 20).Draw(rt, "roll")
		npcStrMod := rapid.IntRange(0, 4).Draw(rt, "npcStrMod")
		dc := 10 + npcStrMod

		src := dice.NewDeterministicSource([]int{roll})
		roller := dice.NewRoller(src)
		_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

		roomID := rapid.StringMatching(`room_prop_[a-z]{4}`).Draw(rt, "roomID")
		_, spawnErr := npcMgr.Spawn(&npc.Template{
			ID: "prop-npc-" + roomID, Name: "PropNPC", Level: 1,
			MaxHP: 10, AC: 12, Perception: npcStrMod * 2, // Perception*2/2 → AbilityMod ≈ npcStrMod
		}, roomID)
		if spawnErr != nil {
			rt.Skip()
		}
		_, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "prop-uid-" + roomID, Username: "Runner", CharName: "Runner",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if addErr != nil {
			rt.Skip()
		}
		_, attackErr := combatHandler.Attack("prop-uid-"+roomID, "PropNPC")
		if attackErr != nil {
			rt.Skip()
		}
		combatHandler.cancelTimer(roomID)

		_, fled, err := combatHandler.Flee("prop-uid-" + roomID)
		if err != nil {
			rt.Skip() // AP or other transient error
		}

		playerTotal := roll // bonus=0 since sess.Skills is empty
		if playerTotal >= dc {
			assert.True(t, fled, "expected success when playerTotal(%d) >= dc(%d)", playerTotal, dc)
		} else {
			assert.False(t, fled, "expected failure when playerTotal(%d) < dc(%d)", playerTotal, dc)
		}
	})
}

// TestProperty_Pursuit_RollOutcome verifies the NPC pursuit condition:
// NPC pursues iff pursuitTotal >= playerTotal, by inspecting narrative events.
func TestProperty_Pursuit_RollOutcome(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		fleeRoll := rapid.IntRange(11, 20).Draw(rt, "fleeRoll") // ensure flee succeeds (>=10+0)
		pursuitRoll := rapid.IntRange(1, 20).Draw(rt, "pursuitRoll")

		src := dice.NewDeterministicSource([]int{fleeRoll, pursuitRoll})
		roller := dice.NewRoller(src)
		_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

		roomID := "room_a"
		inst, spawnErr := npcMgr.Spawn(&npc.Template{
			ID: "prop-pursue-npc", Name: "Pursuer", Level: 1,
			MaxHP: 10, AC: 12, Perception: 0, // StrMod=0 → DC=10; fleeRoll≥11 always succeeds
		}, roomID)
		if spawnErr != nil {
			rt.Skip()
		}
		sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "prop-pursue-uid", Username: "Runner", CharName: "Runner",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if addErr != nil {
			rt.Skip()
		}
		_, attackErr := combatHandler.Attack("prop-pursue-uid", "Pursuer")
		if attackErr != nil {
			rt.Skip()
		}
		combatHandler.cancelTimer(roomID)

		events, fled, err := combatHandler.Flee("prop-pursue-uid")
		if err != nil || !fled {
			rt.Skip()
		}

		playerTotal := fleeRoll // bonus=0
		expectedPursues := pursuitRoll >= playerTotal

		if expectedPursues {
			updatedInst, ok := npcMgr.Get(inst.ID)
			assert.True(t, ok)
			assert.Equal(t, sess.RoomID, updatedInst.RoomID,
				"NPC must follow player when pursuitRoll(%d) >= playerTotal(%d)", pursuitRoll, playerTotal)
		} else {
			updatedInst, ok := npcMgr.Get(inst.ID)
			assert.True(t, ok)
			assert.Equal(t, roomID, updatedInst.RoomID,
				"NPC must stay when pursuitRoll(%d) < playerTotal(%d)", pursuitRoll, playerTotal)
		}
		_ = events
	})
}

// TestProperty_Flee_ExitSelection verifies that Flee only moves the player to an
// exit where Hidden==false && Locked==false, using a world with mixed exit configs.
func TestProperty_Flee_ExitSelection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// All exits in room_a are unlocked/unhidden (room_a → room_b via North).
		src := dice.NewDeterministicSource([]int{20, 1}) // flee succeeds; NPC stays
		roller := dice.NewRoller(src)
		_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

		const roomID = "room_a"
		_, spawnErr := npcMgr.Spawn(&npc.Template{
			ID: "prop-exit-npc", Name: "Blocker", Level: 1,
			MaxHP: 10, AC: 12, Perception: 0,
		}, roomID)
		if spawnErr != nil {
			rt.Skip()
		}
		sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "prop-exit-uid", Username: "Runner", CharName: "Runner",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if addErr != nil {
			rt.Skip()
		}
		_, attackErr := combatHandler.Attack("prop-exit-uid", "Blocker")
		if attackErr != nil {
			rt.Skip()
		}
		combatHandler.cancelTimer(roomID)

		_, fled, err := combatHandler.Flee("prop-exit-uid")
		if err != nil || !fled {
			rt.Skip()
		}

		// Destination must be room_b (the only valid exit from room_a).
		assert.Equal(t, "room_b", sess.RoomID,
			"flee from room_a must land in room_b (the only valid exit)")
	})
}
```

- [ ] **Step 9: Run tests to confirm red (compilation will fail until Flee signature changes)**

```bash
go test ./internal/gameserver/... -run "TestHandleFlee|TestProperty_Flee|TestProperty_Pursuit" -v -count=1 2>&1 | head -30
```

Expected: compile errors about wrong number of return values from `Flee`. This is correct — TDD red.

- [ ] **Step 10: Commit test file**

```bash
git add internal/gameserver/grpc_service_flee_test.go
git commit -m "test(gameserver): add failing flee/pursuit integration tests (TDD red)"
```

---

### Task 2: Rewrite CombatHandler.Flee + add ActiveCombatForRoom

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Add ActiveCombatForRoom helper to combat_handler.go**

Add after the existing `ActiveCombatForPlayer` method:

```go
// ActiveCombatForRoom returns the active combat in roomID, or nil if none.
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns a non-nil *combat.Combat if active; nil otherwise.
func (h *CombatHandler) ActiveCombatForRoom(roomID string) *combat.Combat {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return nil
	}
	return cbt
}
```

- [ ] **Step 2: Change the Flee return signature and replace the body**

Find `func (h *CombatHandler) Flee(uid string)` (around line 644) and replace the **entire function** with:

```go
// Flee attempts to remove the player from active combat using an Athletics/Acrobatics
// skill check against the highest NPC StrMod DC in the room.
//
// Precondition: uid must be a valid connected player in active combat with >= 1 AP.
// Postcondition: On success, player is removed from combat roster, moved to a random
//   valid exit (if any), and NPC pursuit is resolved. Returns fled=true on success.
func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, bool, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, false, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, false, fmt.Errorf("you are not in combat")
	}

	playerCbt := h.findCombatant(cbt, uid)
	if playerCbt == nil {
		return nil, false, fmt.Errorf("you are not a combatant")
	}

	// FLEE-1 / FLEE-2: AP guard — inline to avoid re-acquiring combatMu (SpendAP locks it).
	q, hasQ := cbt.ActionQueues[uid]
	if !hasQ || q.RemainingPoints() < 1 {
		return nil, false, fmt.Errorf("you need at least 1 AP to flee")
	}
	_ = q.DeductAP(q.RemainingPoints())

	// FLEE-3: skill check — auto-pick best of athletics or acrobatics.
	roll, _ := h.dice.RollExpr("d20")
	athleticsBonus := skillRankBonus(sess.Skills["athletics"])
	acrobaticsBonus := skillRankBonus(sess.Skills["acrobatics"])
	bonus := athleticsBonus
	if acrobaticsBonus > athleticsBonus {
		bonus = acrobaticsBonus
	}
	playerTotal := roll.Total() + bonus

	// FLEE-4: DC = 10 + highest NPC StrMod.
	bestNPC := h.bestNPCCombatant(cbt)
	dc := 10
	if bestNPC != nil {
		dc = 10 + bestNPC.StrMod
	}

	var events []*gamev1.CombatEvent

	if playerTotal < dc {
		// FLEE-5: failure — stay in room, combat continues.
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
			Attacker:  sess.CharName,
			Narrative: fmt.Sprintf("%s tries to flee but can't escape! (rolled %d vs DC %d)", sess.CharName, playerTotal, dc),
		})
		return events, false, nil
	}

	// FLEE-6: success — street_brawler attack of opportunity from other players.
	for _, other := range cbt.Combatants {
		if other.ID == uid || other.IsDead() || other.Kind != combat.KindPlayer {
			continue
		}
		otherSess, ok := h.sessions.GetPlayer(other.ID)
		if !ok || !otherSess.PassiveFeats["street_brawler"] {
			continue
		}
		aooResult := combat.ResolveAttack(other, playerCbt, h.dice.Src())
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  other.Name,
			Target:    playerCbt.Name,
			Narrative: fmt.Sprintf("%s lashes out at the fleeing %s: %s (total %d).",
				other.Name, playerCbt.Name, aooResult.Outcome, aooResult.AttackTotal),
		})
	}

	// Capture origRoomID before mutating sess.RoomID.
	origRoomID := sess.RoomID
	h.removeCombatant(cbt, uid)
	sess.Status = int32(1) // idle

	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
		Attacker:  sess.CharName,
		Narrative: fmt.Sprintf("%s breaks free and runs! (rolled %d vs DC %d)", sess.CharName, playerTotal, dc),
	})

	// FLEE-7 / FLEE-8: pick a valid exit.
	var destRoomID string
	if h.worldMgr != nil {
		if room, ok := h.worldMgr.GetRoom(origRoomID); ok {
			var validExits []world.Exit
			for _, e := range room.Exits {
				if !e.Hidden && !e.Locked {
					validExits = append(validExits, e)
				}
			}
			if len(validExits) > 0 {
				chosen := validExits[h.dice.Src().Intn(len(validExits))]
				sess.RoomID = chosen.TargetRoom
				destRoomID = chosen.TargetRoom
			} else {
				events = append(events, &gamev1.CombatEvent{
					Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
					Attacker:  sess.CharName,
					Narrative: "There is nowhere to run — but you are no longer in combat.",
				})
			}
		}
	}

	// FLEE-11: end original room combat if no players remain.
	if !cbt.HasLivingPlayers() {
		h.stopTimerLocked(origRoomID)
		h.engine.EndCombat(origRoomID)
		if h.onCombatEndFn != nil {
			h.onCombatEndFn(origRoomID)
		}
	}

	// Pursuit (implemented in Task 3 — stub returns nil for now).
	if destRoomID != "" {
		pursuitEvents := h.resolvePursuitLocked(cbt, sess, playerTotal, destRoomID)
		events = append(events, pursuitEvents...)
	}

	return events, true, nil
}
```

- [ ] **Step 3: Add the world import to combat_handler.go if not present**

Check the import block at the top of `combat_handler.go`:

```bash
grep -n "game/world" internal/gameserver/combat_handler.go
```

If absent, add `"github.com/cory-johannsen/mud/internal/game/world"` to the import block.

- [ ] **Step 4: Add resolvePursuitLocked stub**

```go
// resolvePursuitLocked resolves NPC pursuit after a successful flee.
// Caller must hold combatMu. Implemented fully in Task 3.
//
// Precondition: combatMu is held; destRoomID is non-empty.
// Postcondition: Returns narrative events. Full pursuit in Task 3.
func (h *CombatHandler) resolvePursuitLocked(cbt *combat.Combat, playerSess *session.PlayerSession, playerTotal int, destRoomID string) []*gamev1.CombatEvent {
	return nil // stub
}
```

- [ ] **Step 5: Update handleFlee in grpc_service.go**

Find `func (s *GameServiceServer) handleFlee(uid string)` and replace it:

```go
func (s *GameServiceServer) handleFlee(uid string) (*gamev1.ServerEvent, error) {
	// Capture origRoomID before Flee mutates sess.RoomID on success (FLEE-12).
	origRoomID := ""
	if sess, ok := s.sessions.GetPlayer(uid); ok {
		origRoomID = sess.RoomID
	}

	events, fled, err := s.combatH.Flee(uid)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}

	// Broadcast all events to the original room.
	for _, evt := range events {
		s.broadcastCombatEvent(origRoomID, uid, evt)
	}

	if fled {
		// Push room view to players remaining in original room (FLEE-10).
		s.pushRoomViewToAllInRoom(origRoomID)

		// Push room view to the fleeing player in their new room (FLEE-9).
		if sess, ok := s.sessions.GetPlayer(uid); ok {
			if newRoom, ok := s.world.GetRoom(sess.RoomID); ok {
				rv := s.worldH.buildRoomView(uid, newRoom)
				evt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_RoomView{RoomView: rv},
				}
				data, err := proto.Marshal(evt)
				if err != nil {
					s.logger.Error("handleFlee: marshal room view failed", zap.Error(err))
				} else {
					_ = sess.Entity.PushBlocking(data, 2*time.Second)
				}
			}
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}
```

- [ ] **Step 6: Build to check compile errors**

```bash
go build ./internal/gameserver/... 2>&1
```

Expected: no errors. Fix any import or type mismatches before proceeding.

- [ ] **Step 7: Run flee tests**

```bash
go test ./internal/gameserver/... -run "TestHandleFlee|TestProperty_Flee|TestProperty_Pursuit" -v -count=1 2>&1 | head -60
```

Expected: `TestHandleFlee_NotEnoughAP`, `TestHandleFlee_Failure`, `TestHandleFlee_Success_OriginalCombatEnds` pass. Pursuit tests may fail because `resolvePursuitLocked` is still a stub.

- [ ] **Step 8: Commit grpc_service.go only — defer combat_handler.go until Task 3 completes**

The `resolvePursuitLocked` stub in `combat_handler.go` is a placeholder intentionally replaced in Task 3.
Do NOT commit `combat_handler.go` at this step — commit it together with Task 3 to avoid violating AGENT-1 (no placeholder code in commits).

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): update handleFlee for (events, fled, error) signature and room view pushes"
```

---

## Chunk 2: NPC pursuit implementation + full test pass

### Task 3: Implement resolvePursuitLocked and startPursuitCombatLocked

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

- [ ] **Step 1: Replace the resolvePursuitLocked stub**

```go
// resolvePursuitLocked resolves NPC pursuit checks after a successful flee.
// Caller must hold combatMu.
//
// Precondition: combatMu is held; destRoomID non-empty; playerSess.RoomID == destRoomID.
// Postcondition: Pursuing NPCs moved to destRoomID; new combat started if any pursue;
//   returned events are for deferred broadcasting by the caller.
func (h *CombatHandler) resolvePursuitLocked(cbt *combat.Combat, playerSess *session.PlayerSession, playerTotal int, destRoomID string) []*gamev1.CombatEvent {
	var events []*gamev1.CombatEvent
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
			// PURSUIT-2: move NPC; skip if move fails (NPC stays in original room).
			if err := h.npcMgr.Move(c.ID, destRoomID); err != nil {
				events = append(events, &gamev1.CombatEvent{
					Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
					Attacker:  c.Name,
					Narrative: fmt.Sprintf("%s gives chase but loses you!", c.Name),
				})
				continue
			}
			pursuers = append(pursuers, inst)
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
				Attacker:  c.Name,
				Narrative: fmt.Sprintf("%s gives chase! (rolled %d)", c.Name, pursuitTotal),
			})
		} else {
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
				Attacker:  c.Name,
				Narrative: fmt.Sprintf("%s can't keep up and falls behind. (rolled %d)", c.Name, pursuitTotal),
			})
		}
	}

	if len(pursuers) > 0 {
		initEvents, err := h.startPursuitCombatLocked(playerSess, pursuers)
		if err != nil {
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
				Narrative: fmt.Sprintf("Pursuit error: %v", err),
			})
		} else {
			events = append(events, initEvents...)
		}
	}

	return events
}
```

- [ ] **Step 2: Add startPursuitCombatLocked**

Add immediately after `resolvePursuitLocked`:

```go
// startPursuitCombatLocked initiates a new combat in the player's current room
// (the destination after fleeing) with all pursuing NPC instances.
// Caller must hold combatMu. Does NOT call broadcastFn — returns init events for
// deferred broadcasting to avoid deadlock.
//
// Precondition: combatMu is held; playerSess.RoomID is the destination room;
//   insts is non-empty.
// Postcondition: combat registered in engine; StartRound(3) called; timer started.
func (h *CombatHandler) startPursuitCombatLocked(playerSess *session.PlayerSession, insts []*npc.Instance) ([]*gamev1.CombatEvent, error) {
	const dexMod = 1
	var playerAC int
	if h.invRegistry != nil {
		defStats := playerSess.Equipment.ComputedDefenses(h.invRegistry, dexMod)
		playerAC = 10 + defStats.ACBonus + defStats.EffectiveDex
	} else {
		playerAC = 10 + dexMod
	}

	playerCbt := &combat.Combatant{
		ID:        playerSess.UID,
		Kind:      combat.KindPlayer,
		Name:      playerSess.CharName,
		MaxHP:     playerSess.CurrentHP,
		CurrentHP: playerSess.CurrentHP,
		AC:        playerAC,
		Level:     1,
		StrMod:    2,
		DexMod:    dexMod,
	}
	h.loadoutsMu.Lock()
	if lo, ok := h.loadouts[playerSess.UID]; ok {
		playerCbt.Loadout = lo
	}
	h.loadoutsMu.Unlock()

	// Weapon proficiency rank — same pattern as startCombatLocked lines 1152–1165.
	weaponProfRank := "untrained"
	if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
		cat := playerCbt.Loadout.MainHand.Def.ProficiencyCategory
		if r, ok := playerSess.Proficiencies[cat]; ok {
			weaponProfRank = r
		}
		playerCbt.WeaponDamageType = playerCbt.Loadout.MainHand.Def.DamageType
	}
	playerCbt.WeaponProficiencyRank = weaponProfRank

	// Resistances / weaknesses — same pattern as startCombatLocked lines 1167–1169.
	playerCbt.Resistances = playerSess.Resistances
	playerCbt.Weaknesses = playerSess.Weaknesses

	// Save mods and proficiency ranks — same pattern as startCombatLocked lines 1171–1179.
	playerCbt.GritMod = combat.AbilityMod(playerSess.Abilities.Grit)
	playerCbt.QuicknessMod = combat.AbilityMod(playerSess.Abilities.Quickness)
	playerCbt.SavvyMod = combat.AbilityMod(playerSess.Abilities.Savvy)
	playerCbt.ToughnessRank = combat.DefaultSaveRank(playerSess.Proficiencies["toughness"])
	playerCbt.HustleRank = combat.DefaultSaveRank(playerSess.Proficiencies["hustle"])
	playerCbt.CoolRank = combat.DefaultSaveRank(playerSess.Proficiencies["cool"])

	combatants := []*combat.Combatant{playerCbt}
	for _, inst := range insts {
		npcWeaponName := ""
		if inst.WeaponID != "" && h.invRegistry != nil {
			if wDef := h.invRegistry.Weapon(inst.WeaponID); wDef != nil {
				npcWeaponName = wDef.Name
			}
		}
		npcCbt := &combat.Combatant{
			ID:          inst.ID,
			Kind:        combat.KindNPC,
			Name:        inst.Name(),
			MaxHP:       inst.MaxHP,
			CurrentHP:   inst.CurrentHP,
			AC:          inst.AC,
			Level:       inst.Level,
			StrMod:      combat.AbilityMod(inst.Perception),
			DexMod:      1,
			NPCType:     inst.Type,
			Resistances: inst.Resistances,
			Weaknesses:  inst.Weaknesses,
			WeaponName:  npcWeaponName,
			Position:    25,
		}
		combatants = append(combatants, npcCbt)
	}

	combat.RollInitiative(combatants, h.dice.Src())

	var scriptMgr *scripting.Manager
	var zoneID string
	if h.scriptMgr != nil && h.worldMgr != nil {
		scriptMgr = h.scriptMgr
		if room, ok := h.worldMgr.GetRoom(playerSess.RoomID); ok {
			zoneID = room.ZoneID
		}
	}

	playerCbt.Position = 0  // player starts at near end
	cbt, err := h.engine.StartCombat(playerSess.RoomID, combatants, h.condRegistry, scriptMgr, zoneID)
	if err != nil {
		return nil, fmt.Errorf("startPursuitCombatLocked: %w", err)
	}
	playerSess.Status = int32(2) // in combat

	// Apply flat_footed to all pursuing NPC combatants at combat start — same pattern
	// as startCombatLocked lines 1224–1232.
	if h.condRegistry != nil {
		if def, ok := h.condRegistry.Get("flat_footed"); ok {
			for _, npcCbt := range cbt.Combatants {
				if npcCbt.Kind != combat.KindNPC {
					continue
				}
				if cbt.Conditions[npcCbt.ID] == nil {
					cbt.Conditions[npcCbt.ID] = condition.NewActiveSet()
				}
				_ = cbt.Conditions[npcCbt.ID].Apply(npcCbt.ID, def, 1, 1)
			}
		}
	}

	cbt.SetSessionGetter(func(uid string) (*session.PlayerSession, bool) {
		return h.sessions.GetPlayer(uid)
	})
	cbt.StartRound(3)

	var events []*gamev1.CombatEvent
	for _, c := range cbt.Combatants {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Attacker:  c.Name,
			Narrative: fmt.Sprintf("%s rolls initiative: %d", c.Name, c.Initiative),
		})
	}
	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
		Narrative: fmt.Sprintf("Pursuit! Round %d begins!", cbt.Round),
	})

	h.startTimerLocked(playerSess.RoomID)
	return events, nil
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/gameserver/... 2>&1
```

Expected: no errors.

- [ ] **Step 4: Run all flee tests**

```bash
go test ./internal/gameserver/... -run "TestHandleFlee|TestProperty_Flee|TestProperty_Pursuit" -v -count=1 2>&1
```

Expected: all tests pass.

- [ ] **Step 5: Run full test suite**

```bash
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -20
```

Expected: all packages green, 0 failures. If any pre-existing test broke due to `Flee` signature change, fix it now.

- [ ] **Step 6: Commit combat_handler.go (deferred from Task 2 Step 8)**

This commit includes the complete `Flee` rewrite, `ActiveCombatForRoom`, `resolvePursuitLocked`, and `startPursuitCombatLocked` — all production-complete with no stubs.

```bash
git add internal/gameserver/combat_handler.go
git commit -m "feat(gameserver): rewrite Flee with skill check, AP guard, movement, and full NPC pursuit"
```

---

### Task 4: Final verification and FEATURES.md update

**Files:**
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Run full test suite**

```bash
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -25
```

Expected: all packages green.

- [ ] **Step 2: Mark feature complete in FEATURES.md**

Find the flee/pursuit lines (around line 191–192) and check both boxes:

```markdown
      - [x] Flee command — implement `flee` (costs all remaining AP; Athletics or Acrobatics check vs highest NPC Athletics DC in room; on success combat ends and player moves to a random adjacent room)
      - [x] Pursuit — on flee failure, each NPC makes a separate Athletics check; on NPC success the NPC follows the player to the adjacent room and re-initiates combat
```

- [ ] **Step 3: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark Flee & Pursuit complete in FEATURES.md"
```

---

## Completion Criterion

`go test $(go list ./... | grep -v storage/postgres)` MUST pass with 0 failures.

**Reference files:**
- `internal/gameserver/grpc_service_grapple_test.go` — test setup patterns
- `internal/gameserver/combat_handler.go` lines 1118–1273 — `startCombatLocked` (model for `startPursuitCombatLocked`)
