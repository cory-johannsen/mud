# Immobilized (Grabbed blocks room movement) Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent a player with the `grabbed` condition from leaving the current room via `move` or `flee`.

**Architecture:** Add `"move"` to `grabbed.yaml`'s `restrict_actions` list; check `condition.IsActionRestricted(sess.Conditions, "move")` at the top of `handleMove` and at the top of `CombatHandler.Flee` before any other logic; return a blocked message event in both cases.

**Tech Stack:** Go, existing `condition.IsActionRestricted` helper, `content/conditions/grabbed.yaml`

---

## Chunk 1: All tasks

### Task 1: Block move and flee when grabbed

**Files:**
- Modify: `content/conditions/grabbed.yaml`
- Modify: `internal/gameserver/grpc_service.go` (handleMove, ~line 1243)
- Modify: `internal/gameserver/combat_handler.go` (Flee, ~line 646)
- Create: `internal/gameserver/grpc_service_immobilized_test.go`

**Background — key types and helpers:**

`condition.IsActionRestricted(s *condition.ActiveSet, actionType string) bool` — iterates all active conditions and returns true if any has `actionType` in its `RestrictActions` slice. Located at `internal/game/condition/modifiers.go:34`.

`makeTestConditionRegistry()` — builds a `*condition.Registry` with pre-registered conditions including `grabbed` (without `RestrictActions`). Located at `internal/gameserver/combat_handler_test.go:69`. **After this task, `grabbed` in that registry must include `RestrictActions: []string{"move"}`** — or the tests must build their own registry that includes it. Use the latter approach (build registry inline in test helpers) to avoid cross-file coupling.

`newMoveTestService` (grpc_service_move_test.go:95) — constructs a `GameServiceServer` without a condition registry. Move-blocked tests need a condReg, so they use a different helper.

`newEscapeSvcWithCombat` (grpc_service_escape_test.go:43) — constructs a full service with condReg and combatHandler. **Use this same pattern** for the flee-while-grabbed test.

`testWorldAndSession(t)` — returns `(*world.Manager, *session.Manager)` with room_a→north→room_b world. Defined in `combat_handler_test.go`.

`session.AddPlayerOptions` — struct with fields: `UID`, `Username`, `CharName`, `CharacterID`, `RoomID`, `CurrentHP`, `MaxHP`, `Abilities`, `Role`.

`messageEvent(text string) *gamev1.ServerEvent` — builds a `ServerEvent` wrapping a `MessageEvent`. Defined in `grpc_service.go`.

`errorEvent(text string) *gamev1.ServerEvent` — builds a `ServerEvent` wrapping an `ErrorEvent`. Defined in `grpc_service.go`.

**The `handleMove` signature:**
```go
func (s *GameServiceServer) handleMove(uid string, req *gamev1.MoveRequest) (*gamev1.ServerEvent, error)
```
It currently calls `s.worldH.MoveWithContext(uid, dir)` immediately. The guard goes before that call.

**The `Flee` signature:**
```go
func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, bool, error)
```
It currently checks AP immediately after acquiring `combatMu`. The grabbed guard goes after `combatMu.Lock()` and after the `cbt`/`playerCbt` nil checks (so we know the player is in combat), but before the AP deduction.

**Grabbed event message for move:** `"You are grabbed and cannot move!"`

**Grabbed event message for flee:** Return a `[]*gamev1.CombatEvent` with a single narrative event whose detail is `"You are grabbed and cannot flee!"`, with `fled = false`.

The combat event for flee blocked should follow the same pattern as the AP-insufficient case — wrap a `*gamev1.CombatEvent` with `ActionNarrative`. Look at how flee failure events are built in `combat_handler.go` for the exact proto structure.

---

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_immobilized_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// makeImmobilizedConditionRegistry returns a registry with grabbed having restrict_actions: ["move"].
func makeImmobilizedConditionRegistry() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4, RestrictActions: []string{"attack", "strike", "pass"}})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1, ACPenalty: 1})
	reg.Register(&condition.ConditionDef{ID: "grabbed", Name: "Grabbed", DurationType: "permanent", MaxStacks: 0, ACPenalty: 2, RestrictActions: []string{"move"}})
	reg.Register(&condition.ConditionDef{ID: "hidden", Name: "Hidden", DurationType: "permanent", MaxStacks: 0})
	return reg
}

// newImmobilizedMoveSvc builds a GameServiceServer with a condReg that has grabbed restricting move.
func newImmobilizedMoveSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeImmobilizedConditionRegistry()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr
}

// newImmobilizedFleeSvc builds a GameServiceServer with a condReg that has grabbed restricting move,
// a full combat handler, and an NPC in combat with the player.
func newImmobilizedFleeSvc(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeImmobilizedConditionRegistry()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, worldMgr, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleMove_GrabbedBlocked verifies that a player with the grabbed condition
// cannot move to another room.
//
// Precondition: Player is in room_a with grabbed condition active.
// Postcondition: handleMove returns an error event with "grabbed" in the message; player remains in room_a.
func TestHandleMove_GrabbedBlocked(t *testing.T) {
	svc, sessMgr := newImmobilizedMoveSvc(t)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_imm_move",
		Username:  "Tester",
		CharName:  "Tester",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Apply grabbed condition.
	sess.Conditions = condition.NewActiveSet()
	condReg := makeImmobilizedConditionRegistry()
	grabbedDef, ok := condReg.Get("grabbed")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

	evt, err := svc.handleMove("u_imm_move", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Must return an error event containing "grabbed".
	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected ErrorEvent, got: %T", evt.Payload)
	assert.Contains(t, errEvt.Message, "grabbed")

	// Player must still be in room_a.
	sess2, ok := sessMgr.GetPlayer("u_imm_move")
	require.True(t, ok)
	assert.Equal(t, "room_a", sess2.RoomID, "player must not have moved")
}

// TestHandleMove_NotGrabbed_MovesNormally verifies that a player without the grabbed condition
// can move normally.
//
// Precondition: Player is in room_a with no grabbed condition.
// Postcondition: handleMove returns a RoomView event; player is in room_b.
func TestHandleMove_NotGrabbed_MovesNormally(t *testing.T) {
	svc, sessMgr := newImmobilizedMoveSvc(t)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_imm_free",
		Username:  "Tester",
		CharName:  "Tester",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	evt, err := svc.handleMove("u_imm_free", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Must return a RoomView (successful move), not an error.
	assert.Nil(t, evt.GetError(), "expected no error event for un-grabbed player")

	sess2, ok := sessMgr.GetPlayer("u_imm_free")
	require.True(t, ok)
	assert.Equal(t, "room_b", sess2.RoomID, "player must have moved to room_b")
}

// TestHandleFlee_GrabbedBlocked verifies that a player with the grabbed condition
// cannot flee combat.
//
// Precondition: Player is in combat with an NPC in room_a, has grabbed condition active, has AP.
// Postcondition: Flee returns a narrative event with "grabbed" in the detail; fled==false; player remains in room_a.
func TestHandleFlee_GrabbedBlocked(t *testing.T) {
	src := dice.NewDeterministicSource([]int{5, 3}) // initiative rolls only
	roller := dice.NewRoller(src)
	svc, sessMgr, npcMgr, _ := newImmobilizedFleeSvc(t, roller)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_imm_flee",
		Username:  "Runner",
		CharName:  "Runner",
		RoomID:    "room_a",
		CurrentHP: 20,
		MaxHP:     20,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Spawn NPC and start combat.
	tmpl := &npc.Template{
		ID: "ganger-imm-room_a-1", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 10,
	}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	_, err = svc.combatH.Attack("u_imm_flee", inst.ID)
	require.NoError(t, err)

	// Apply grabbed condition to the player session.
	sess.Conditions = condition.NewActiveSet()
	condReg := makeImmobilizedConditionRegistry()
	grabbedDef, ok := condReg.Get("grabbed")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

	evt, err := svc.handleFlee("u_imm_flee")
	require.NoError(t, err)
	require.NotNil(t, evt)

	combatEvt := evt.GetCombatEvent()
	require.NotNil(t, combatEvt, "expected CombatEvent")
	narrative := combatEvt.GetActionNarrative()
	require.NotNil(t, narrative)
	assert.Contains(t, narrative.Detail, "grabbed")

	// Player must still be in room_a.
	sess2, ok := sessMgr.GetPlayer("u_imm_flee")
	require.True(t, ok)
	assert.Equal(t, "room_a", sess2.RoomID, "grabbed player must not have moved")
}
```

Also add this property-based test (SWENG-5a) to the same file:

```go
// TestProperty_GrabbedAlwaysBlocksMove verifies that whenever the grabbed condition
// (with restrict_actions: ["move"]) is active, handleMove always returns an error event
// and the player never changes rooms.
func TestProperty_GrabbedAlwaysBlocksMove(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newImmobilizedMoveSvc(t)
		uid := rapid.StringMatching(`u_prop_[a-z]{6}`).Draw(rt, "uid")

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:       uid,
			Username:  "Prop",
			CharName:  "Prop",
			RoomID:    "room_a",
			CurrentHP: 10,
			MaxHP:     10,
			Abilities: character.AbilityScores{},
			Role:      "player",
		})
		require.NoError(rt, err)

		sess.Conditions = condition.NewActiveSet()
		condReg := makeImmobilizedConditionRegistry()
		grabbedDef, ok := condReg.Get("grabbed")
		require.True(rt, ok)
		require.NoError(rt, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

		evt, err := svc.handleMove(uid, &gamev1.MoveRequest{Direction: "north"})
		require.NoError(rt, err)
		require.NotNil(rt, evt)

		errEvt := evt.GetError()
		assert.NotNil(rt, errEvt, "grabbed player must always receive an error event")

		sess2, ok := sessMgr.GetPlayer(uid)
		require.True(rt, ok)
		assert.Equal(rt, "room_a", sess2.RoomID, "grabbed player must never move rooms")
	})
}
```

Note: Add `"pgregory.net/rapid"` to the imports in the test file.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleMove_GrabbedBlocked|TestHandleMove_NotGrabbed_MovesNormally|TestHandleFlee_GrabbedBlocked" -v 2>&1 | tail -30
```

Expected: FAIL — `handleMove` and `Flee` do not yet check for grabbed.

- [ ] **Step 3: Update grabbed.yaml to add "move" to restrict_actions**

Edit `content/conditions/grabbed.yaml`:

```yaml
id: grabbed
name: Grabbed
description: |
  You are held in place. You are flat-footed (-2 AC) while grabbed.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 2
damage_bonus: 0
speed_penalty: 0
restrict_actions:
  - move
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 4: Add grabbed guard to handleMove**

In `internal/gameserver/grpc_service.go`, in `handleMove` (line ~1243), add the guard before `s.worldH.MoveWithContext`:

```go
func (s *GameServiceServer) handleMove(uid string, req *gamev1.MoveRequest) (*gamev1.ServerEvent, error) {
	dir := world.Direction(req.Direction)

	// IMMOBILIZED: grabbed condition prevents leaving the room.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil {
		if condition.IsActionRestricted(sess.Conditions, "move") {
			return errorEvent("You are grabbed and cannot move!"), nil
		}
	}

	result, err := s.worldH.MoveWithContext(uid, dir)
	// ... rest of function unchanged ...
```

- [ ] **Step 5: Add grabbed guard to CombatHandler.Flee**

In `internal/gameserver/combat_handler.go`, in `Flee` (line ~660), add the guard after the `playerCbt` nil check and before the AP deduction:

```go
	playerCbt := h.findCombatant(cbt, uid)
	if playerCbt == nil {
		h.combatMu.Unlock()
		return nil, false, fmt.Errorf("you are not a combatant")
	}

	// IMMOBILIZED: grabbed condition prevents fleeing.
	if sess.Conditions != nil && condition.IsActionRestricted(sess.Conditions, "move") {
		h.combatMu.Unlock()
		evt := &gamev1.CombatEvent{
			Action: &gamev1.CombatEvent_ActionNarrative{
				ActionNarrative: &gamev1.ActionNarrativeEvent{
					ActorId: uid,
					Detail:  "You are grabbed and cannot flee!",
				},
			},
		}
		return []*gamev1.CombatEvent{evt}, false, nil
	}

	// FLEE-1 / FLEE-2: AP guard — inline to avoid re-acquiring combatMu (SpendAP locks it).
	q, hasQ := cbt.ActionQueues[uid]
```

Note: `condition` package import is already present in `combat_handler.go` — verify with `grep "condition" internal/gameserver/combat_handler.go` before assuming. If not present, add `"github.com/cory-johannsen/mud/internal/game/condition"` to the import block.

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleMove_GrabbedBlocked|TestHandleMove_NotGrabbed_MovesNormally|TestHandleFlee_GrabbedBlocked" -v 2>&1
```

Expected: PASS for all three.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -25
```

Expected: all packages PASS, 0 failures.

- [ ] **Step 8: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, change:

```
  - [ ] Immobilized — prevent grabbed creatures from moving between rooms
```

to:

```
  - [x] Immobilized — prevent grabbed creatures from moving between rooms
```

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/conditions/grabbed.yaml \
        internal/gameserver/grpc_service.go \
        internal/gameserver/combat_handler.go \
        internal/gameserver/grpc_service_immobilized_test.go \
        docs/requirements/FEATURES.md
git commit -m "feat(combat): immobilized — grabbed condition blocks room movement and flee"
```
