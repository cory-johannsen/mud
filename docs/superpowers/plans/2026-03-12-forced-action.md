# Forced Action Execution Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When certain mental state conditions are active (Panicked, Psychotic, Berserker), the player's combat action is forced each round regardless of player input.

**Architecture:** New `ForcedAction` field on ConditionDef drives data-driven forced behavior. `ForcedActionType` helper reads it. `ClearActions()` on ActionQueue enables override of pre-submitted actions. `autoQueuePlayersLocked` restructured to check forced action before the existing early-exit guard.

**Tech Stack:** Go, pgregory.net/rapid (property tests), protobuf

---

## Task 1: ConditionDef field, ForcedActionType helper, YAML updates

**Files:**
- Modify: `internal/game/condition/definition.go`
- Modify: `internal/game/condition/modifiers.go`
- Modify: `internal/game/condition/modifiers_test.go`
- Modify: `content/conditions/mental/fear_panicked.yaml`
- Modify: `content/conditions/mental/fear_psychotic.yaml`
- Modify: `content/conditions/mental/rage_berserker.yaml`

### Steps:

- [ ] 1. Write failing tests in `modifiers_test.go` (append these tests):

```go
func TestForcedActionType_NoConditions(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, "", condition.ForcedActionType(s))
}

func TestForcedActionType_WithForcedCondition(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{
		ID:           "fear_panicked",
		Name:         "Panicked",
		ForcedAction: "random_attack",
		DurationType: "rounds",
	}
	require.NoError(t, s.Apply("uid", def, 1, 3))
	assert.Equal(t, "random_attack", condition.ForcedActionType(s))
}

func TestForcedActionType_NilSet(t *testing.T) {
	assert.Equal(t, "", condition.ForcedActionType(nil))
}

func TestForcedActionType_MultipleConditions_ReturnsNonEmpty(t *testing.T) {
	s := condition.NewActiveSet()
	def1 := &condition.ConditionDef{ID: "c1", ForcedAction: "random_attack", DurationType: "rounds"}
	def2 := &condition.ConditionDef{ID: "c2", ForcedAction: "lowest_hp_attack", DurationType: "rounds"}
	require.NoError(t, s.Apply("uid", def1, 1, 3))
	require.NoError(t, s.Apply("uid", def2, 1, 3))
	got := condition.ForcedActionType(s)
	assert.True(t, got == "random_attack" || got == "lowest_hp_attack", "expected a non-empty forced action type, got %q", got)
}

func TestProperty_ForcedActionType_AlwaysValidOrEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		forcedAction := rapid.SampledFrom([]string{"", "random_attack", "lowest_hp_attack"}).Draw(rt, "forced_action")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test_forced", ForcedAction: forcedAction, DurationType: "rounds"}
		require.NoError(t, s.Apply("uid", def, 1, 3))
		got := condition.ForcedActionType(s)
		valid := got == "" || got == "random_attack" || got == "lowest_hp_attack"
		assert.True(t, valid, "ForcedActionType returned unexpected value %q", got)
	})
}
```

- [ ] 2. Run tests to verify they fail:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run "TestForcedActionType|TestProperty_ForcedActionType" -v 2>&1 | tail -5
```
Expected: compile error — `ForcedActionType` not defined.

- [ ] 3. Add `ForcedAction` field to `ConditionDef` in `internal/game/condition/definition.go`, after `SkillPenalty`:
```go
// ForcedAction, if non-empty, forces a specific action type each combat round.
// Valid values: "random_attack" (attack random alive combatant), "lowest_hp_attack" (attack lowest-HP alive combatant).
ForcedAction string `yaml:"forced_action"`
```

- [ ] 4. Add `ForcedActionType` helper to `internal/game/condition/modifiers.go`:
```go
// ForcedActionType returns the forced_action value from the first active condition
// that has one, or empty string if none. Map iteration order is non-deterministic;
// simultaneous forced conditions from different tracks are not expected in practice.
//
// Precondition: s may be nil.
// Postcondition: Returns "" or one of "random_attack", "lowest_hp_attack".
func ForcedActionType(s *ActiveSet) string {
	if s == nil {
		return ""
	}
	for _, ac := range s.conditions {
		if ac.Def.ForcedAction != "" {
			return ac.Def.ForcedAction
		}
	}
	return ""
}
```

- [ ] 5. Run tests to verify they pass:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run "TestForcedActionType|TestProperty_ForcedActionType" -v 2>&1
```
Expected: all PASS.

- [ ] 6. Update YAML files — add `forced_action` field to each:

`content/conditions/mental/fear_panicked.yaml` — add at end before lua fields:
```yaml
forced_action: random_attack
```

`content/conditions/mental/fear_psychotic.yaml` — add at end before lua fields:
```yaml
forced_action: random_attack
```

`content/conditions/mental/rage_berserker.yaml` — add at end before lua fields:
```yaml
forced_action: lowest_hp_attack
```

- [ ] 7. Verify YAML loads without error:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v 2>&1 | tail -10
```
Expected: all PASS (KnownFields(true) will accept the new field now that the struct has it).

- [ ] 8. Commit:
```bash
cd /home/cjohannsen/src/mud
git add internal/game/condition/definition.go internal/game/condition/modifiers.go \
        internal/game/condition/modifiers_test.go \
        content/conditions/mental/fear_panicked.yaml \
        content/conditions/mental/fear_psychotic.yaml \
        content/conditions/mental/rage_berserker.yaml
git commit -m "feat(condition): add ForcedAction field and ForcedActionType helper"
```

---

## Task 2: ClearActions on ActionQueue

**Files:**
- Modify: `internal/game/combat/action.go`
- Modify: `internal/game/combat/action_test.go` (or create if not present — check first)

### Steps:

- [ ] 1. Find the action test file:
```bash
ls /home/cjohannsen/src/mud/internal/game/combat/*test* 2>&1
```

- [ ] 2. Write failing tests — append to the action test file (or create it as `internal/game/combat/action_test.go`):

```go
func TestClearActions_EmptyQueue(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	q.ClearActions()
	assert.Equal(t, 0, len(q.QueuedActions()))
	assert.Equal(t, q.MaxPoints, q.RemainingPoints())
	assert.False(t, q.IsSubmitted())
}

func TestClearActions_AfterEnqueue(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"})
	require.NoError(t, err)
	require.Greater(t, len(q.QueuedActions()), 0)

	q.ClearActions()
	assert.Equal(t, 0, len(q.QueuedActions()))
	assert.Equal(t, q.MaxPoints, q.RemainingPoints())
	assert.False(t, q.IsSubmitted())
}

func TestClearActions_AfterPass(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err)
	require.True(t, q.IsSubmitted())

	q.ClearActions()
	assert.Equal(t, 0, len(q.QueuedActions()))
	assert.Equal(t, q.MaxPoints, q.RemainingPoints())
	assert.False(t, q.IsSubmitted())
}

func TestProperty_ClearActions_AlwaysRestoresMaxPoints(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxAP := rapid.IntRange(1, 5).Draw(rt, "maxAP")
		q := combat.NewActionQueue("uid", maxAP)
		// Optionally enqueue a pass to mark submitted
		if rapid.Bool().Draw(rt, "enqueue_pass") {
			_ = q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
		}
		q.ClearActions()
		assert.Equal(t, 0, len(q.QueuedActions()), "queue must be empty after ClearActions")
		assert.Equal(t, maxAP, q.RemainingPoints(), "remaining must equal MaxPoints after ClearActions")
		assert.False(t, q.IsSubmitted(), "IsSubmitted must be false after ClearActions")
	})
}
```

Note: `combat.NewActionQueue` — check if this constructor exists. If not, look for how ActionQueue is created in tests (may be `&combat.ActionQueue{UID: "uid", MaxPoints: 3, ...}` or a constructor). Read the file first.

- [ ] 3. Run tests to verify they fail:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestClearActions|TestProperty_ClearActions" -v 2>&1 | tail -5
```
Expected: compile error or "method not defined".

- [ ] 4. Add `ClearActions` to `ActionQueue` in `internal/game/combat/action.go`:

```go
// ClearActions drains all queued actions, restores remaining AP to MaxPoints,
// and marks the queue as unsubmitted (IsSubmitted() returns false after this call).
//
// Postcondition: len(QueuedActions()) == 0; RemainingPoints() == MaxPoints; IsSubmitted() == false.
func (q *ActionQueue) ClearActions() {
	q.actions = q.actions[:0]
	q.remaining = q.MaxPoints
}
```

- [ ] 5. Run tests to verify they pass:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestClearActions|TestProperty_ClearActions" -v 2>&1
```
Expected: all PASS.

- [ ] 6. Run full combat package tests:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v 2>&1 | tail -10
```
Expected: all PASS.

- [ ] 7. Commit:
```bash
cd /home/cjohannsen/src/mud
git add internal/game/combat/action.go internal/game/combat/action_test.go
git commit -m "feat(combat): add ClearActions to ActionQueue — drains queue and restores MaxPoints"
```

---

## Task 3: Combat handler extension + integration tests

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/grpc_service_forced_action_test.go`

### Steps:

- [ ] 1. Write failing tests — create `internal/gameserver/grpc_service_forced_action_test.go`:

The correct scaffolding pattern is from `combat_handler_autoqueue_test.go`:
- Use `npcMgr.Spawn(&npc.Template{...}, roomID)` to create NPCs (NOT `npc.NPC` — that type does not exist)
- Use `h.Attack(playerUID, npcName)` to start combat, then `h.engine.GetCombat(roomID)` to retrieve the combat object (NOT `activeCombats` map or `StartCombat`)
- Use `h.cancelTimer(roomID)` right after `Attack` to prevent the round timer from firing during the test
- To set NPC HP lower after spawning: `inst.CurrentHP = 3` (inst is `*npc.Instance`)

```go
package gameserver

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// makeForcedActionHandler builds a CombatHandler with a MentalStateManager for forced-action tests.
//
// Postcondition: Returns non-nil handler, npcMgr, sessMgr.
func makeForcedActionHandler(t *testing.T, mentalMgr *mentalstate.Manager) (*CombatHandler, *npc.Manager, *session.Manager) {
	t.Helper()
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := NewCombatHandler(
		engine, npcMgr, sessMgr, (*dice.Roller)(nil), broadcastFn,
		10*time.Second, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, mentalMgr,
	)
	return h, npcMgr, sessMgr
}

// setupForcedActionCombat creates a combat scenario with one player and two NPCs.
// npc2 has lower HP (3) than npc1 (20) for lowest-HP tests.
// Returns handler, sessMgr, the active combat, npc1 name, npc2 name.
//
// Postcondition: combat is started (timer cancelled); player uid is "u_forced".
func setupForcedActionCombat(t *testing.T, mentalMgr *mentalstate.Manager) (*CombatHandler, *session.Manager, *combat.Combat, string, string) {
	t.Helper()
	h, npcMgr, sessMgr := makeForcedActionHandler(t, mentalMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_forced", Username: "T", CharName: "T", RoomID: "room_a",
		CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Spawn two NPCs in the same room.
	tmpl1 := &npc.Template{ID: "goblin", Name: "Goblin", Level: 1, MaxHP: 20, AC: 12}
	inst1, err := npcMgr.Spawn(tmpl1, "room_a")
	require.NoError(t, err)
	_ = inst1 // inst1.CurrentHP defaults to MaxHP=20

	tmpl2 := &npc.Template{ID: "orc", Name: "Orc", Level: 1, MaxHP: 20, AC: 12}
	inst2, err := npcMgr.Spawn(tmpl2, "room_a")
	require.NoError(t, err)
	inst2.CurrentHP = 3 // Set Orc to lower HP for lowest-HP tests

	// Start combat by having the player attack the first NPC.
	_, err = h.Attack("u_forced", "Goblin")
	require.NoError(t, err)
	h.cancelTimer("room_a")

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat("room_a")
	h.combatMu.Unlock()
	require.True(t, ok, "combat must exist after Attack")

	return h, sessMgr, cbt, "Goblin", "Orc"
}

// applyForcedCondition applies a condition with the given forced_action value to the session.
func applyForcedCondition(t *testing.T, sess *session.PlayerSession, uid, condID, forcedAction string) {
	t.Helper()
	def := &condition.ConditionDef{
		ID:           condID,
		Name:         condID,
		ForcedAction: forcedAction,
		DurationType: "rounds",
	}
	require.NoError(t, sess.Conditions.Apply(uid, def, 1, 5))
}

// TestForcedAction_RandomAttack_Panicked verifies a panicked player attacks a random combatant.
func TestForcedAction_RandomAttack_Panicked(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, cbt, npc1Name, npc2Name := setupForcedActionCombat(t, mentalMgr)

	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "fear_panicked", "random_attack")

	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	// Target must be one of the alive combatants (player self, Goblin, or Orc).
	assert.True(t, actions[0].Target == npc1Name || actions[0].Target == npc2Name || actions[0].Target == "T",
		"target must be an alive combatant, got %q", actions[0].Target)
}

// TestForcedAction_LowHPAttack_Berserker verifies a berserker player attacks the lowest-HP combatant.
func TestForcedAction_LowHPAttack_Berserker(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, cbt, _, npc2Name := setupForcedActionCombat(t, mentalMgr)

	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "rage_berserker", "lowest_hp_attack")

	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	// Orc has CurrentHP=3, Goblin has CurrentHP=20, player has CurrentHP=10 — Orc is lowest.
	assert.Equal(t, npc2Name, actions[0].Target)
}

// TestForcedAction_OverridesPreSubmitted verifies forced action overrides a pre-submitted player action.
func TestForcedAction_OverridesPreSubmitted(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, cbt, npc1Name, _ := setupForcedActionCombat(t, mentalMgr)

	// Player pre-submits an attack against Goblin.
	combatH.combatMu.Lock()
	err := cbt.QueueAction("u_forced", combat.QueuedAction{Type: combat.ActionAttack, Target: npc1Name})
	combatH.combatMu.Unlock()
	require.NoError(t, err)

	// Verify pre-submit landed.
	q := cbt.ActionQueues["u_forced"]
	require.Len(t, q.QueuedActions(), 1)

	// Apply forced condition AFTER pre-submit.
	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "fear_panicked", "random_attack")

	// autoQueuePlayersLocked must override the pre-submitted action.
	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	// The action was replaced — we can't assert the exact target (random), but it must be an attack.
}

// TestForcedAction_NoCondition_NormalBehavior verifies no forced action when condition is absent.
func TestForcedAction_NoCondition_NormalBehavior(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, _, cbt, npc1Name, _ := setupForcedActionCombat(t, mentalMgr)

	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	// Default action: attack the first NPC (Goblin).
	assert.Equal(t, npc1Name, actions[0].Target)
}

// TestProperty_ForcedAction_AlwaysTargetsAliveCombatant verifies forced actions always pick alive targets.
func TestProperty_ForcedAction_AlwaysTargetsAliveCombatant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		forcedType := rapid.SampledFrom([]string{"random_attack", "lowest_hp_attack"}).Draw(rt, "forced_type")

		mentalMgr := mentalstate.NewManager()
		combatH, sessMgr, cbt, npc1Name, npc2Name := setupForcedActionCombat(t, mentalMgr)

		sess, ok := sessMgr.GetPlayer("u_forced")
		require.True(t, ok)
		applyForcedCondition(t, sess, "u_forced", "test_forced", forcedType)

		combatH.combatMu.Lock()
		combatH.autoQueuePlayersLocked(cbt)
		combatH.combatMu.Unlock()

		q := cbt.ActionQueues["u_forced"]
		require.NotNil(t, q)
		actions := q.QueuedActions()
		require.Len(t, actions, 1, "forced action must produce exactly one action")
		assert.Equal(t, combat.ActionAttack, actions[0].Type, "forced action must be an attack")

		// Target must be one of the alive combatants.
		aliveCombatants := []string{npc1Name, npc2Name, "T"}
		found := false
		for _, name := range aliveCombatants {
			if actions[0].Target == name {
				found = true
				break
			}
		}
		assert.True(t, found, "target %q must be an alive combatant", actions[0].Target)
	})
}
```

// applyForcedCondition applies a condition with the given forced_action value to the session.
func applyForcedCondition(t *testing.T, sess *session.PlayerSession, uid, condID, forcedAction string) {
	t.Helper()
	def := &condition.ConditionDef{
		ID:           condID,
		Name:         condID,
		ForcedAction: forcedAction,
		DurationType: "rounds",
	}
	require.NoError(t, sess.Conditions.Apply(uid, def, 1, 5))
}

// TestForcedAction_RandomAttack_Panicked verifies a panicked player attacks a random combatant.
func TestForcedAction_RandomAttack_Panicked(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, combatID, npc1Name, npc2Name := setupForcedActionCombat(t, mentalMgr)

	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "fear_panicked", "random_attack")

	combatH.combatMu.Lock()
	cbt := combatH.activeCombats[combatID]
	require.NotNil(t, cbt)
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	assert.True(t, actions[0].Target == npc1Name || actions[0].Target == npc2Name || actions[0].Target == "T",
		"target must be an alive combatant, got %q", actions[0].Target)
}

// TestForcedAction_LowHPAttack_Berserker verifies a berserker player attacks the lowest-HP combatant.
func TestForcedAction_LowHPAttack_Berserker(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, combatID, _, npc2Name := setupForcedActionCombat(t, mentalMgr)

	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "rage_berserker", "lowest_hp_attack")

	combatH.combatMu.Lock()
	cbt := combatH.activeCombats[combatID]
	require.NotNil(t, cbt)
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	// Orc has CurrentHP=3, Goblin has CurrentHP=8 — Orc is lower
	assert.Equal(t, npc2Name, actions[0].Target)
}

// TestForcedAction_OverridesPreSubmitted verifies forced action overrides a pre-submitted player action.
func TestForcedAction_OverridesPreSubmitted(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, combatID, npc1Name, npc2Name := setupForcedActionCombat(t, mentalMgr)

	// Player pre-submits an attack against npc1.
	combatH.combatMu.Lock()
	cbt := combatH.activeCombats[combatID]
	require.NotNil(t, cbt)
	err := cbt.QueueAction("u_forced", combat.QueuedAction{Type: combat.ActionAttack, Target: npc1Name})
	combatH.combatMu.Unlock()
	require.NoError(t, err)

	// Apply forced condition.
	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "fear_panicked", "random_attack")

	// autoQueuePlayersLocked should override the pre-submitted action.
	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	// Target may be anything alive — the point is it was overridden and is an attack
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	_ = npc2Name // suppress unused warning
}

// TestForcedAction_NoCondition_NormalBehavior verifies no forced action when condition is absent.
func TestForcedAction_NoCondition_NormalBehavior(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, _, combatID, npc1Name, _ := setupForcedActionCombat(t, mentalMgr)

	combatH.combatMu.Lock()
	cbt := combatH.activeCombats[combatID]
	require.NotNil(t, cbt)
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	// Default action should target npc1 (first NPC)
	assert.Equal(t, npc1Name, actions[0].Target)
}

// TestProperty_ForcedAction_AlwaysTargetsAliveCombatant verifies forced actions always pick alive targets.
func TestProperty_ForcedAction_AlwaysTargetsAliveCombatant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		forcedType := rapid.SampledFrom([]string{"random_attack", "lowest_hp_attack"}).Draw(rt, "forced_type")

		mentalMgr := mentalstate.NewManager()
		combatH, sessMgr, combatID, npc1Name, npc2Name := setupForcedActionCombat(t, mentalMgr)

		sess, ok := sessMgr.GetPlayer("u_forced")
		require.True(t, ok)
		applyForcedCondition(t, sess, "u_forced", "test_forced", forcedType)

		combatH.combatMu.Lock()
		cbt := combatH.activeCombats[combatID]
		require.NotNil(t, cbt)
		combatH.autoQueuePlayersLocked(cbt)
		combatH.combatMu.Unlock()

		q := cbt.ActionQueues["u_forced"]
		require.NotNil(t, q)
		actions := q.QueuedActions()
		require.Len(t, actions, 1, "forced action must produce exactly one action")
		assert.Equal(t, combat.ActionAttack, actions[0].Type, "forced action must be an attack")

		aliveCombatants := []string{npc1Name, npc2Name, "T"} // player self is also a valid target
		found := false
		for _, name := range aliveCombatants {
			if actions[0].Target == name {
				found = true
				break
			}
		}
		assert.True(t, found, "target %q must be an alive combatant", actions[0].Target)
	})
}
```

Note: `autoQueuePlayersLocked` is package-internal — fine since the test is also `package gameserver`.

- [ ] 2. Run tests to verify they fail:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestForcedAction|TestProperty_ForcedAction" -v 2>&1 | tail -10
```
Expected: compile error or test failures.

- [ ] 3. Modify `autoQueuePlayersLocked` in `internal/gameserver/combat_handler.go`:

Replace the existing early-exit guard and add forced-action logic. The current guard is:
```go
q, ok := cbt.ActionQueues[c.ID]
if !ok || len(q.QueuedActions()) > 0 {
    continue
}
```

Replace with:
```go
q, ok := cbt.ActionQueues[c.ID]
if !ok {
    continue
}

forcedAction := condition.ForcedActionType(sess.Conditions)
if forcedAction == "" && len(q.QueuedActions()) > 0 {
    continue // player submitted and no forced override
}

if forcedAction != "" {
    q.ClearActions()
    var forcedTarget string
    switch forcedAction {
    case "random_attack":
        // Collect all alive combatants (any faction).
        var targets []string
        for _, combatant := range cbt.Combatants {
            if !combatant.IsDead() {
                targets = append(targets, combatant.Name)
            }
        }
        if len(targets) > 0 {
            forcedTarget = targets[rand.Intn(len(targets))]
        }
    case "lowest_hp_attack":
        // Find alive combatant with minimum CurrentHP.
        lowestHP := int(^uint(0) >> 1) // MaxInt
        for _, combatant := range cbt.Combatants {
            if !combatant.IsDead() && combatant.CurrentHP < lowestHP {
                lowestHP = combatant.CurrentHP
                forcedTarget = combatant.Name
            }
        }
    }
    if forcedTarget == "" {
        // Edge case: no valid target — attack self.
        forcedTarget = c.Name
    }
    if err := cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionAttack, Target: forcedTarget}); err != nil {
        continue
    }
    var msg string
    switch forcedAction {
    case "random_attack":
        msg = fmt.Sprintf("Panic grips you — you lash out wildly at %s!", forcedTarget)
    case "lowest_hp_attack":
        msg = fmt.Sprintf("Berserker rage drives you to destroy the weakest target — you attack %s!", forcedTarget)
    }
    notifyEvt := &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_Message{
            Message: &gamev1.MessageEvent{
                Content: msg,
                Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
            },
        },
    }
    if data, marshalErr := proto.Marshal(notifyEvt); marshalErr == nil {
        _ = sess.Entity.Push(data)
    }
    continue
}
```

**Important notes for the implementer:**
- `sess` is resolved before the guard check in the current code — move `sess` resolution to before the forced-action block. Read the existing function to understand the order.
- `rand` import: check if `math/rand` is already imported; if not add it.
- `combatant.CurrentHP` — check the field name on the `combat.Combatant` struct; it may be `HP` or `CurrentHP`. Read the struct definition.
- `combatant.Name` — check the field name on `combat.Combatant`. It may be `Name` or `ID`.

- [ ] 4. Run tests:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestForcedAction|TestProperty_ForcedAction" -v 2>&1
```
Expected: all PASS.

- [ ] 5. Run full test suite:
```bash
cd /home/cjohannsen/src/mud && go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -10
```
Expected: all PASS.

- [ ] 6. Update `docs/requirements/FEATURES.md` — find the "Forced action execution" line and mark it `[x]`:

Find:
```
      - [ ] Forced action execution — Panicked (random action), Psychotic (attack nearest), Berserker (attack nearest) — requires combat auto-execution mechanism
```
Replace with:
```
      - [x] Forced action execution — Panicked/Psychotic (random attack any combatant), Berserker (attack lowest-HP combatant); overrides player pre-submitted actions each round
```

- [ ] 7. Commit:
```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go \
        internal/gameserver/grpc_service_forced_action_test.go \
        docs/requirements/FEATURES.md
git commit -m "feat(combat): forced action execution — Panicked random attack, Berserker lowest-HP attack"
```
