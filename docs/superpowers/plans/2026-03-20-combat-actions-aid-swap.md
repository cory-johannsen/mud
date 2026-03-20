# Combat Actions: Aid and Swap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Aid combat action (DC 20 roll, four outcomes, applies attack buff/penalty to ally) and enforce a 1 AP cost when swapping weapon presets in combat.

**Architecture:** Eight layers touched in dependency order: condition engine extension → action type → round resolver → condition YAML → combat handler → proto + generated code → command registry + bridge → gRPC handlers. The Swap change is a targeted patch to one existing handler. All new code is test-driven; property-based tests cover Aid roll banding and PBT for condition engine.

**Tech Stack:** Go 1.22, `pgregory.net/rapid` (PBT), `github.com/stretchr/testify`, protobuf/grpc (`make proto`), YAML condition files.

---

## Codebase Context

**Key files:**
- `internal/game/condition/definition.go` — `ConditionDef` struct (YAML tags)
- `internal/game/condition/modifiers.go` — `AttackBonus(s *ActiveSet) int` (currently ignores negative values; guards on `> 0`)
- `internal/game/combat/action.go` — `ActionType` iota, `Cost()`, `String()`
- `internal/game/combat/round.go` — `ResolveRound(...)`, `findCombatantByNameOrID` (case-insensitive, line 1141)
- `internal/gameserver/combat_handler.go` — `CombatHandler` methods; use `cbt.QueueAction(uid, QueuedAction{...})` pattern (see Strike at line 289)
- `internal/gameserver/grpc_service.go` — `handleLoadout` (line 3511), dispatch switch (line 1387)
- `internal/game/command/commands.go` — `HandlerXxx` constants and `BuiltinCommands()`
- `internal/frontend/handlers/bridge_handlers.go` — `bridgeHandlerMap` and bridge funcs
- `api/proto/game/v1/game.proto` — `ClientMessage.payload` oneof; next field number = **83**

**Test helpers (package `gameserver`, internal tests):**
- `makeCombatHandler(t, broadcastFn)` — creates a `CombatHandler` with crypto dice
- `makeCombatHandlerWithDice(t, src, broadcastFn)` — deterministic dice
- `addTestPlayer(t, sessMgr, uid, roomID)` — registers a player session (CharName="Hero")
- `spawnTestNPC(t, npcMgr, roomID)` — spawns "Goblin" NPC
- `makeTestConditionRegistry()` — returns standard test registry
- `testWorldAndSession(t)` — returns `(*world.Manager, *session.Manager)`
- `newHideSvc(t, roller, npcMgr, ch)` — builds minimal `GameServiceServer` (see `grpc_service_hide_test.go:19`)

**Condition durations:** Use `DurationType: "rounds"` (plural) and `ApplyCondition(uid, condID, stacks=1, duration=1)` for 1-round conditions. The spec (REQ-ACT16/17/18) erroneously says `duration_type: round` (singular) — the codebase's existing YAML files (e.g., `frightened`, `flat_footed`) and the `Tick()` implementation all use `"rounds"` (plural). The plan's `"rounds"` is authoritative; the spec has a typo.

**Proto event type:** Use `COMBAT_EVENT_TYPE_CONDITION` (value 6) for the Aid confirmation event.

**Status constant:** In-combat status check uses `sess.Status == statusInCombat` (unexported constant in `grpc_service.go`).

---

## Task 1: Condition Engine Extension — `attack_bonus` field

**Spec refs:** REQ-ACT0a, REQ-ACT0b, REQ-ACT0c

**Files:**
- Modify: `internal/game/condition/definition.go`
- Modify: `internal/game/condition/modifiers.go`
- Modify: `internal/game/condition/definition_test.go`

- [ ] **Step 1: Write failing test for `attack_bonus` YAML round-trip**

In `internal/game/condition/definition_test.go`, add to the existing test file (it's in `package condition_test`):

```go
func TestConditionDef_AttackBonus_YAMLRoundTrip(t *testing.T) {
	yaml := `
id: test_bonus
name: Test Bonus
description: grants attack bonus
duration_type: rounds
max_stacks: 0
attack_bonus: 3
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
`
	var def condition.ConditionDef
	dec := yaml3.NewDecoder(strings.NewReader(yaml))
	dec.KnownFields(true)
	if err := dec.Decode(&def); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if def.AttackBonus != 3 {
		t.Errorf("expected AttackBonus=3, got %d", def.AttackBonus)
	}
}
```

Check existing imports in `definition_test.go` — add `"strings"` and `yaml3 "gopkg.in/yaml.v3"` if not present.

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestConditionDef_AttackBonus_YAMLRoundTrip -v
```

Expected: FAIL — `unknown field "attack_bonus"` or similar.

- [ ] **Step 3: Add `AttackBonus` field to `ConditionDef`**

In `internal/game/condition/definition.go`, after the `AttackPenalty` field:

```go
AttackPenalty   int      `yaml:"attack_penalty"`
AttackBonus     int      `yaml:"attack_bonus"`   // positive = bonus to attack rolls
```

- [ ] **Step 4: Write failing test for `AttackBonus` modifier with nil guard**

In `internal/game/condition/modifiers_test.go` (check existing file for package name — it's `package condition_test`), add:

```go
func TestAttackBonus_NilActiveSet_ReturnsZero(t *testing.T) {
	result := condition.AttackBonus(nil)
	if result != 0 {
		t.Errorf("expected 0 for nil ActiveSet, got %d", result)
	}
}

func TestAttackBonus_WithBonus_ReturnsPositive(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID: "aided", Name: "Aided", DurationType: "rounds", MaxStacks: 0,
		AttackBonus: 2,
	})
	s := condition.NewActiveSet(reg)
	_ = s.Apply("p1", reg.Must("aided"), 1, 1)
	result := condition.AttackBonus(s)
	if result != 2 {
		t.Errorf("expected AttackBonus=2, got %d", result)
	}
}

func TestAttackBonus_PenaltyAndBonus_Net(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID: "aided_penalty", Name: "Aided (Fumble)", DurationType: "rounds", MaxStacks: 0,
		AttackPenalty: 1,
	})
	s := condition.NewActiveSet(reg)
	_ = s.Apply("p1", reg.Must("aided_penalty"), 1, 1)
	result := condition.AttackBonus(s)
	if result != -1 {
		t.Errorf("expected net -1, got %d", result)
	}
}
```

**Note:** Check whether `condition.NewActiveSet` and `reg.Must` exist; if not, use the pattern in existing modifiers tests to construct an `ActiveSet`.

- [ ] **Step 5: Run new modifier tests — expect FAIL**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run "TestAttackBonus_Nil|TestAttackBonus_With|TestAttackBonus_Penalty" -v
```

Expected: compile error or FAIL.

- [ ] **Step 6: Update `AttackBonus` in `modifiers.go`**

Replace the existing `AttackBonus` function entirely:

```go
// AttackBonus returns the net attack roll modifier from all active conditions.
// Positive AttackBonus on a condition adds to the total (buff); positive AttackPenalty
// subtracts from the total (debuff). For stackable conditions, values are multiplied
// by the current stack count.
//
// Precondition: s may be nil.
// Postcondition: Returns the net modifier; may be positive when attack bonuses are active.
func AttackBonus(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.AttackPenalty > 0 {
			total -= ac.Def.AttackPenalty * ac.Stacks
		}
		if ac.Def.AttackBonus > 0 {
			total += ac.Def.AttackBonus * ac.Stacks
		}
	}
	return total
}
```

- [ ] **Step 7: Run all condition tests — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v
```

Expected: all PASS.

**Note:** If `condition.NewActiveSet` or `reg.Must` don't exist, look at how existing `modifiers_test.go` constructs ActiveSets and mirror that pattern exactly.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/condition/definition.go internal/game/condition/modifiers.go internal/game/condition/definition_test.go internal/game/condition/modifiers_test.go && git commit -m "feat: add attack_bonus field to ConditionDef and update AttackBonus modifier"
```

---

## Task 2: `ActionAid` type

**Spec refs:** REQ-ACT6

**Files:**
- Modify: `internal/game/combat/action.go`
- Modify (existing tests may need condition registry update): `internal/game/combat/action_test.go`

- [ ] **Step 1: Write failing tests for ActionAid**

In `internal/game/combat/action_test.go` (package `combat_test`), add:

```go
func TestActionAid_Cost(t *testing.T) {
	if combat.ActionAid.Cost() != 2 {
		t.Errorf("expected ActionAid.Cost()=2, got %d", combat.ActionAid.Cost())
	}
}

func TestActionAid_String(t *testing.T) {
	if combat.ActionAid.String() != "aid" {
		t.Errorf("expected ActionAid.String()=%q, got %q", "aid", combat.ActionAid.String())
	}
}

func TestActionAid_Enqueue_DeductsAP(t *testing.T) {
	q := combat.NewActionQueue("p1", 3)
	if err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAid, Target: "Bob"}); err != nil {
		t.Fatalf("Enqueue ActionAid: %v", err)
	}
	if q.RemainingPoints() != 1 {
		t.Errorf("expected 1 AP remaining after ActionAid, got %d", q.RemainingPoints())
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestActionAid" -v
```

Expected: compile error — `ActionAid` undefined.

- [ ] **Step 3: Add `ActionAid` to `action.go`**

In `internal/game/combat/action.go`, add `ActionAid` to the iota after `ActionCoverDestroy`:

```go
ActionCoverDestroy                    // informational: cover object destroyed
ActionAid                             // costs 2 AP; assist ally's next attack
```

Update `Cost()` — add before the `default` case:

```go
case ActionAid:
    return 2
```

Update `String()` — add before the `default` case, and update the postcondition comment to include `"aid"`:

```go
case ActionAid:
    return "aid"
```

Update the `String()` postcondition comment to include `"aid"`:
```
// Postcondition: returns "attack", "strike", "pass", "reload", "burst",
// "automatic", "throw", "use_ability", "stride", "aid", or "unknown".
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestActionAid" -v
```

- [ ] **Step 5: Run full combat test suite — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v 2>&1 | tail -20
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/action.go internal/game/combat/action_test.go && git commit -m "feat: add ActionAid to combat action types (costs 2 AP)"
```

---

## Task 3: `aidOutcome` helper and `ResolveRound` Aid case

**Spec refs:** REQ-ACT10, REQ-ACT11, REQ-ACT23

**Files:**
- Modify: `internal/game/combat/round.go`
- Modify: `internal/game/combat/round_test.go`

- [ ] **Step 1: Write failing PBT test for `AidOutcome`**

In `internal/game/combat/round_test.go` (package `combat_test`), add:

```go
func TestProperty_AidOutcome_Bands(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		total := rapid.IntRange(-100, 100).Draw(rt, "total")
		outcome := combat.AidOutcome(total)
		switch {
		case total <= 9:
			if outcome != "critical_failure" {
				rt.Fatalf("total=%d: expected critical_failure, got %q", total, outcome)
			}
		case total <= 19:
			if outcome != "failure" {
				rt.Fatalf("total=%d: expected failure, got %q", total, outcome)
			}
		case total <= 29:
			if outcome != "success" {
				rt.Fatalf("total=%d: expected success, got %q", total, outcome)
			}
		default:
			if outcome != "critical_success" {
				rt.Fatalf("total=%d: expected critical_success, got %q", total, outcome)
			}
		}
	})
}
```

**Decision:** The spec says `aidOutcome` (unexported) but also places the PBT in `grpc_service_aid_test.go` (`package gameserver`), which cannot access unexported symbols from `package combat`. These two requirements are mutually contradictory. Resolution: name the function `AidOutcome` (exported) and place the PBT in `round_test.go` (`package combat_test`), which is the architecturally correct location. The spec file map entry for `grpc_service_aid_test.go` referencing the PBT is incorrect.

- [ ] **Step 2: Write integration test for ActionAid in ResolveRound AND dead-ally path**

In `internal/game/combat/round_test.go`, add:

```go
func TestResolveRound_ActionAid_CriticalSuccess(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "aided_strong", Name: "Aided (Strong)", DurationType: "rounds", MaxStacks: 0, AttackBonus: 3})
	reg.Register(&condition.ConditionDef{ID: "aided", Name: "Aided", DurationType: "rounds", MaxStacks: 0, AttackBonus: 2})
	reg.Register(&condition.ConditionDef{ID: "aided_penalty", Name: "Aided (Fumble)", DurationType: "rounds", MaxStacks: 0, AttackPenalty: 1})

	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, GritMod: 2, QuicknessMod: 1, SavvyMod: 0},
		{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	_ = cbt.StartRound(3)

	// d20 returns 19 (0-indexed from Intn(20)), so roll = 20; GritMod=2 → total=22+2=24... wait
	// Intn(20) returns value in [0,19]; roll = Intn(20)+1 in [1,20].
	// fixedSrc{val:19} → Intn(20) returns min(19, 19) = 19 → roll = 20; +GritMod(2) = 22 → but we want >=30
	// Use fixedSrc{val:19} and GritMod=11 for total=31 (crit success).
	// Set GritMod on p1 so max(2,1,0) gives us control:
	// Actually use a combatant with GritMod=10; fixed val=19 → roll=20+10=30 → critical success.
	combatants[0].GritMod = 10

	// Restart with updated combatants
	cbt2, err := eng.StartCombat("room2", []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, GritMod: 10, QuicknessMod: 1, SavvyMod: 0},
		{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1},
	}, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat2: %v", err)
	}
	_ = cbt2.StartRound(3)

	require.NoError(t, cbt2.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAid, Target: "Bob"}))
	require.NoError(t, cbt2.QueueAction("p2", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt2.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	// fixedSrc{19} → Intn(20) = 19 → d20=20; max(GritMod=10, Quick=1, Savvy=0)=10; total=30 → critical success
	src := fixedSrc{val: 19}
	events := combat.ResolveRound(cbt2, src, noopUpdater, nil)

	var aidEvent *combat.RoundEvent
	for i := range events {
		if events[i].ActionType == combat.ActionAid {
			aidEvent = &events[i]
			break
		}
	}
	require.NotNil(t, aidEvent, "expected ActionAid event")
	assert.Contains(t, aidEvent.Narrative, "critical aid")

	// Bob should have aided_strong condition
	condSet := cbt2.Conditions["p2"]
	require.NotNil(t, condSet)
	assert.True(t, condSet.Has("aided_strong"), "Bob should have aided_strong")
}
```

Also add the dead-ally-at-resolution-time test (REQ-ACT10):

```go
func TestResolveRound_ActionAid_TargetDeadAtResolution(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "aided_strong", Name: "Aided (Strong)", DurationType: "rounds", MaxStacks: 0, AttackBonus: 3})
	reg.Register(&condition.ConditionDef{ID: "aided", Name: "Aided", DurationType: "rounds", MaxStacks: 0, AttackBonus: 2})
	reg.Register(&condition.ConditionDef{ID: "aided_penalty", Name: "Aided (Fumble)", DurationType: "rounds", MaxStacks: 0, AttackPenalty: 1})

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_dead", []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, GritMod: 10},
		{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 20, CurrentHP: 0, AC: 14, Level: 1}, // already dead
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1},
	}, reg, nil, "")
	require.NoError(t, err)
	_ = cbt.StartRound(3)

	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAid, Target: "Bob"}))
	require.NoError(t, cbt.QueueAction("p2", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	src := fixedSrc{val: 19}
	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

	var aidEvent *combat.RoundEvent
	for i := range events {
		if events[i].ActionType == combat.ActionAid {
			aidEvent = &events[i]
			break
		}
	}
	require.NotNil(t, aidEvent, "expected ActionAid event even for dead target")
	assert.Contains(t, aidEvent.Narrative, "already down")
	// Bob must NOT have any aided condition applied.
	if condSet := cbt.Conditions["p2"]; condSet != nil {
		assert.False(t, condSet.Has("aided_strong"), "dead target must not receive aided_strong")
		assert.False(t, condSet.Has("aided"), "dead target must not receive aided")
	}
}
```

Add imports as needed (`"github.com/stretchr/testify/assert"`, `"github.com/stretchr/testify/require"`).

- [ ] **Step 3: Run tests — expect FAIL (compile error)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestProperty_AidOutcome|TestResolveRound_ActionAid" -v
```

Expected: compile errors — `AidOutcome` and `ActionAid` case missing.

- [ ] **Step 4: Add `AidOutcome` helper to `round.go`**

In `internal/game/combat/round.go`, add before `ResolveRound`:

```go
// AidOutcome classifies an Aid roll total into one of four DC 20 outcome bands.
//
// Precondition: total is the sum of d20 + best ability modifier.
// Postcondition: Returns "critical_success" (≥30), "success" (20–29),
// "failure" (10–19), or "critical_failure" (≤9).
func AidOutcome(total int) string {
	switch {
	case total >= 30:
		return "critical_success"
	case total >= 20:
		return "success"
	case total >= 10:
		return "failure"
	default:
		return "critical_failure"
	}
}
```

- [ ] **Step 5: Add `ActionAid` case to `ResolveRound`**

In `internal/game/combat/round.go`, inside `ResolveRound`'s `switch action.Type` block, add after the `ActionPass` case:

```go
case ActionAid:
    target := findCombatantByNameOrID(cbt, action.Target)
    if target == nil || target.IsDead() {
        events = append(events, RoundEvent{
            ActionType: ActionAid,
            ActorID:    actor.ID,
            ActorName:  actor.Name,
            TargetID:   action.Target,
            Narrative:  fmt.Sprintf("%s attempts to aid %s, but %s is already down.", actor.Name, action.Target, action.Target),
        })
        continue
    }
    // Roll d20 + max(GritMod, QuicknessMod, SavvyMod).
    d20 := src.Intn(20) + 1
    bestMod := actor.GritMod
    if actor.QuicknessMod > bestMod {
        bestMod = actor.QuicknessMod
    }
    if actor.SavvyMod > bestMod {
        bestMod = actor.SavvyMod
    }
    total := d20 + bestMod
    outcome := AidOutcome(total)
    var narrative string
    switch outcome {
    case "critical_success":
        _ = cbt.ApplyCondition(target.ID, "aided_strong", 1, 1)
        narrative = fmt.Sprintf("%s provides critical aid to %s! (rolled %d) +3 to next attack.", actor.Name, target.Name, total)
    case "success":
        _ = cbt.ApplyCondition(target.ID, "aided", 1, 1)
        narrative = fmt.Sprintf("%s aids %s (rolled %d). +2 to next attack.", actor.Name, target.Name, total)
    case "failure":
        narrative = fmt.Sprintf("%s fails to aid %s (rolled %d). No effect.", actor.Name, target.Name, total)
    default: // critical_failure
        _ = cbt.ApplyCondition(target.ID, "aided_penalty", 1, 1)
        narrative = fmt.Sprintf("%s fumbles the aid attempt on %s (rolled %d)! -1 to next attack.", actor.Name, target.Name, total)
    }
    events = append(events, RoundEvent{
        ActionType: ActionAid,
        ActorID:    actor.ID,
        ActorName:  actor.Name,
        TargetID:   target.ID,
        TargetName: target.Name,
        Narrative:  narrative,
    })
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestProperty_AidOutcome|TestResolveRound_ActionAid" -v
```

- [ ] **Step 7: Run full combat test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/round.go internal/game/combat/round_test.go && git commit -m "feat: add AidOutcome helper and ActionAid resolution in ResolveRound"
```

---

## Task 4: Condition YAML files

**Spec refs:** REQ-ACT16, REQ-ACT17, REQ-ACT18

**Files:**
- Create: `content/conditions/aided_strong.yaml`
- Create: `content/conditions/aided.yaml`
- Create: `content/conditions/aided_penalty.yaml`

- [ ] **Step 1: Write failing test that the three aided conditions load from disk**

In `internal/game/condition/registry_test.go` (or wherever `LoadDirectory` is tested — grep for `LoadDirectory`), add:

```go
func TestLoadDirectory_AidedConditionsPresent(t *testing.T) {
	reg := condition.NewRegistry()
	err := reg.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	for _, id := range []string{"aided_strong", "aided", "aided_penalty"} {
		def, ok := reg.Lookup(id)
		if !ok {
			t.Errorf("condition %q not found after LoadDirectory", id)
			continue
		}
		switch id {
		case "aided_strong":
			assert.Equal(t, 3, def.AttackBonus, "aided_strong AttackBonus")
		case "aided":
			assert.Equal(t, 2, def.AttackBonus, "aided AttackBonus")
		case "aided_penalty":
			assert.Equal(t, 1, def.AttackPenalty, "aided_penalty AttackPenalty")
		}
	}
}
```

Run:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestLoadDirectory_AidedConditionsPresent -v
```

Expected: FAIL — conditions not found.

- [ ] **Step 2: Create `content/conditions/aided_strong.yaml`**

```yaml
id: aided_strong
name: Aided (Strong)
description: You've received skilled assistance; gain +3 to your next attack roll this round.
duration_type: rounds
max_stacks: 0
attack_bonus: 3
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 3: Create `content/conditions/aided.yaml`**

```yaml
id: aided
name: Aided
description: You've received assistance; gain +2 to your next attack roll this round.
duration_type: rounds
max_stacks: 0
attack_bonus: 2
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 4: Create `content/conditions/aided_penalty.yaml`**

```yaml
id: aided_penalty
name: Aided (Fumble)
description: Misguided assistance throws off your next attack; -1 to your next attack roll this round.
duration_type: rounds
max_stacks: 0
attack_bonus: 0
attack_penalty: 1
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 5: Run failing test — expect PASS now**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestLoadDirectory_AidedConditionsPresent -v
```

Expected: PASS.

- [ ] **Step 6: Verify full condition suite still passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v 2>&1 | tail -5
```

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/conditions/aided_strong.yaml content/conditions/aided.yaml content/conditions/aided_penalty.yaml internal/game/condition/ && git commit -m "feat: add aided_strong, aided, aided_penalty condition YAML files"
```

---

## Task 5: `CombatHandler.Aid` method

**Spec refs:** REQ-ACT8, REQ-ACT9, REQ-ACT22

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_aid_test.go`

- [ ] **Step 1: Write failing unit tests**

Create `internal/gameserver/combat_handler_aid_test.go`:

```go
package gameserver

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// addTestPlayerNamed registers a player with a specific CharName.
func addTestPlayerNamed(t *testing.T, h *CombatHandler, uid, roomID, charName string) *session.PlayerSession {
	t.Helper()
	sess, err := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "user", CharName: charName,
		CharacterID: 1, RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		Abilities: character.AbilityScores{},
	})
	if err != nil {
		t.Fatalf("addTestPlayerNamed: %v", err)
	}
	return sess
}

func TestCombatHandler_Aid_EmptyAllyName(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	spawnTestNPC(t, h.npcMgr, "room_aid_empty")
	addTestPlayerNamed(t, h, "p_aid_empty", "room_aid_empty", "Alice")
	_, err := h.Attack("p_aid_empty", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer("room_aid_empty")

	_, err = h.Aid("p_aid_empty", "")
	if err == nil {
		t.Fatal("expected error for empty allyName; got nil")
	}
}

func TestCombatHandler_Aid_SelfTargeting(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	spawnTestNPC(t, h.npcMgr, "room_aid_self")
	addTestPlayerNamed(t, h, "p_aid_self", "room_aid_self", "Alice")
	_, err := h.Attack("p_aid_self", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer("room_aid_self")

	_, err = h.Aid("p_aid_self", "Alice")
	if err == nil {
		t.Fatal("expected error for self-targeting; got nil")
	}
}

func TestCombatHandler_Aid_DeadAlly(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	spawnTestNPC(t, h.npcMgr, "room_aid_dead")
	addTestPlayerNamed(t, h, "p1_dead", "room_aid_dead", "Alice")
	addTestPlayerNamed(t, h, "p2_dead", "room_aid_dead", "Bob")
	sess2, _ := h.sessions.GetPlayer("p2_dead")
	sess2.CurrentHP = 0

	_, err := h.Attack("p1_dead", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer("room_aid_dead")

	_, err = h.Aid("p1_dead", "Bob")
	if err == nil {
		t.Fatal("expected error for dead ally; got nil")
	}
}

func TestCombatHandler_Aid_AllyNotInCombat(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	spawnTestNPC(t, h.npcMgr, "room_aid_nc")
	addTestPlayerNamed(t, h, "p1_nc", "room_aid_nc", "Alice")
	// Bob is in a different room — not a combatant.
	addTestPlayerNamed(t, h, "p2_nc", "room_other_nc", "Bob")

	_, err := h.Attack("p1_nc", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer("room_aid_nc")

	_, err = h.Aid("p1_nc", "Bob")
	if err == nil {
		t.Fatal("expected error for ally not in same combat; got nil")
	}
}

func TestCombatHandler_Aid_InsufficientAP(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	spawnTestNPC(t, h.npcMgr, "room_aid_ap")
	addTestPlayerNamed(t, h, "p1_ap", "room_aid_ap", "Alice")
	addTestPlayerNamed(t, h, "p2_ap", "room_aid_ap", "Bob")

	_, err := h.Attack("p1_ap", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer("room_aid_ap")

	// Spend remaining AP (started with 3, used 1 for Attack → 2 left; Strike costs 2).
	_, err = h.Strike("p1_ap", "Goblin")
	if err != nil {
		t.Fatalf("Strike to exhaust AP: %v", err)
	}
	// Now 0 AP — Aid costs 2 AP, must fail.
	_, err = h.Aid("p1_ap", "Bob")
	if err == nil {
		t.Fatal("expected error for insufficient AP; got nil")
	}
	if !strings.Contains(err.Error(), "AP") && !strings.Contains(err.Error(), "insufficient") {
		t.Errorf("expected AP-related error, got: %v", err)
	}
}

func TestCombatHandler_Aid_Success(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	spawnTestNPC(t, h.npcMgr, "room_aid_ok")
	addTestPlayerNamed(t, h, "p1_ok", "room_aid_ok", "Alice")
	addTestPlayerNamed(t, h, "p2_ok", "room_aid_ok", "Bob")

	_, err := h.Attack("p1_ok", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer("room_aid_ok")

	// 2 AP remaining after Attack; Aid costs 2.
	events, err := h.Aid("p1_ok", "Bob")
	if err != nil {
		t.Fatalf("Aid: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one confirmation event from Aid")
	}
}
```

**Note:** The test for dead ally sets `sess2.CurrentHP = 0` but the combat may not know about it — check how death is handled in tests. If this pattern doesn't work, start a combat first then set HP. Look at `TestCombatHandler_SpendAP_InsufficientAP` for the exact pattern of starting combat and having AP exhausted.

Also: the tests for dead ally and ally-not-in-combat need the player to be in active combat. The `Attack` call starts combat. For dead-ally: Bob needs to be a combatant in the combat. Use two real players in the same room and start combat, then mark Bob as dead via `sess.CurrentHP = 0`. But the combatant's CurrentHP is separate from the session's. You may need to use `h.sessions.GetPlayer("p2_dead")` and then find the combat and set the combatant HP. Or: just try to aid Bob before starting combat (he won't be a combatant) for the "not-in-same-combat" test.

Look at the existing test patterns for how player vs player combat is handled. Adjust as needed to make the tests correct.

- [ ] **Step 2: Run tests — expect FAIL (compile error)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestCombatHandler_Aid" -v
```

- [ ] **Step 3: Implement `CombatHandler.Aid`**

In `internal/gameserver/combat_handler.go`, add after `CombatHandler.Strike`:

```go
// Aid queues an ActionAid for the combatant identified by uid targeting allyName.
//
// Precondition: uid must be a valid connected player in active combat.
// Precondition: allyName must be non-empty, must match a living player combatant in the same
// combat (case-insensitive, by Name), and must not match the actor's own CharName or UID.
// Postcondition: Returns a confirmation CombatEvent and nil error on success.
func (h *CombatHandler) Aid(uid, allyName string) ([]*gamev1.CombatEvent, error) {
	if allyName == "" {
		return nil, fmt.Errorf("ally name must not be empty")
	}

	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Self-targeting check.
	if strings.EqualFold(allyName, sess.CharName) || allyName == uid {
		return nil, fmt.Errorf("you cannot aid yourself")
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in active combat")
	}

	// Find the ally: must be a living player combatant in this combat.
	var ally *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.IsPlayer() && strings.EqualFold(c.Name, allyName) {
			ally = c
			break
		}
	}
	if ally == nil {
		return nil, fmt.Errorf("no living ally named %q found in this combat", allyName)
	}
	if ally.IsDead() {
		return nil, fmt.Errorf("ally %q is already down", allyName)
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAid, Target: ally.Name}); err != nil {
		return nil, fmt.Errorf("queuing aid: %w", err)
	}

	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_CONDITION,
		Attacker:  sess.CharName,
		Target:    ally.Name,
		Narrative: fmt.Sprintf("%s prepares to aid %s.", sess.CharName, ally.Name),
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}
```

Add `"strings"` to the import if not already present.

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestCombatHandler_Aid" -v
```

- [ ] **Step 5: Run full gameserver tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_aid_test.go && git commit -m "feat: add CombatHandler.Aid method with validation and queueing"
```

---

## Task 6: Proto — `AidRequest` message and `ClientMessage.aid` field

**Spec refs:** REQ-ACT15

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go` (via `make proto`)

- [ ] **Step 1: Add `AidRequest` message to `game.proto`**

In `api/proto/game/v1/game.proto`, add a new message near other combat-action request messages (e.g., after `DelayRequest`):

```proto
// AidRequest asks the server to queue an Aid action targeting the named ally.
message AidRequest {
  string target = 1;
}
```

- [ ] **Step 2: Add `AidRequest` to `ClientMessage.payload` oneof**

In the `ClientMessage` `oneof payload` block, add after `SelectTechRequest select_tech = 82;`:

```proto
AidRequest aid = 83;
```

- [ ] **Step 3: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: `internal/gameserver/gamev1/game.pb.go` regenerated with `AidRequest` and `ClientMessage_Aid` types.

- [ ] **Step 4: Verify generated code compiles**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | head -20
```

Expected: clean build (or only pre-existing errors if handleAid not yet wired).

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go && git commit -m "feat: add AidRequest proto message and ClientMessage.aid field (83)"
```

---

## Task 7: `HandlerAid` constant and `BuiltinCommands` entry

**Spec refs:** REQ-ACT13

**Files:**
- Modify: `internal/game/command/commands.go`

- [ ] **Step 1: Add `HandlerAid` constant**

In `internal/game/command/commands.go`, add to the `const` block of handler identifiers (after `HandlerSelectTech`):

```go
HandlerAid = "aid"
```

- [ ] **Step 2: Add `aid` entry to `BuiltinCommands()`**

In the `BuiltinCommands()` return slice, add with the other combat commands (e.g., after the `stride` entry):

```go
{Name: "aid", Aliases: nil, Help: "Aid an ally (DC 20 check; crit +3, success +2, fail 0, crit fail -1 to ally attack). Costs 2 AP.", Category: CategoryCombat, Handler: HandlerAid},
```

- [ ] **Step 3: Verify compile**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/game/command/... 2>&1
```

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/command/commands.go && git commit -m "feat: add HandlerAid constant and aid entry to BuiltinCommands"
```

---

## Task 8: `bridgeAid` frontend handler

**Spec refs:** REQ-ACT14, REQ-ACT28

**Files:**
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Add `bridgeAid` function**

In `internal/frontend/handlers/bridge_handlers.go`, add the new function near other combat bridge handlers (e.g., after `bridgeDelay`):

```go
// bridgeAid builds an AidRequest targeting the first whitespace-delimited token.
// If no token is present, target is empty string (server will reject with helpful message).
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an AidRequest; done is false.
func bridgeAid(bctx *bridgeContext) (bridgeResult, error) {
	allyName := strings.Fields(bctx.parsed.RawArgs)
	target := ""
	if len(allyName) > 0 {
		target = allyName[0]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Aid{Aid: &gamev1.AidRequest{Target: target}},
	}}, nil
}
```

**Note:** `strings` is already imported in `bridge_handlers.go` (check the imports at the top).

- [ ] **Step 2: Register `bridgeAid` in `bridgeHandlerMap`**

In `bridgeHandlerMap`, add:

```go
command.HandlerAid: bridgeAid,
```

- [ ] **Step 3: Run bridge dispatch coverage test**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestAll" -v
```

Expected: PASS (the dispatch coverage test checks that every `BuiltinCommands()` handler is wired in `BridgeHandlers()`).

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/bridge_handlers.go && git commit -m "feat: add bridgeAid frontend handler and register in BridgeHandlers"
```

---

## Task 9: `handleAid` gRPC service handler

**Spec refs:** REQ-ACT12, REQ-ACT24

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_aid_test.go`

- [ ] **Step 1: Write failing test for `handleAid`**

Create `internal/gameserver/grpc_service_aid_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newAidSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npc.NewManager(), nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
	)
	return svc, sessMgr
}

// TestHandleAid_NotInCombat verifies handleAid returns an informational message
// (not an error) containing "only valid in combat" when the player is not in combat.
func TestHandleAid_NotInCombat(t *testing.T) {
	svc, sessMgr := newAidSvc(t)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_aid_nc", Username: "Fighter", CharName: "Alice", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleAid("u_aid_nc", &gamev1.AidRequest{Target: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msg := event.GetMessage()
	require.NotNil(t, msg, "expected message event, got: %v", event)
	assert.Contains(t, msg.Text, "only valid in combat")
}

// TestHandleAid_EmptyTarget verifies handleAid returns informational message
// containing "specify an ally name" when target is empty.
func TestHandleAid_EmptyTarget(t *testing.T) {
	svc, sessMgr := newAidSvc(t)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_aid_empty", Username: "Fighter", CharName: "Alice", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, _ := sessMgr.GetPlayer("u_aid_empty")
	sess.Status = statusInCombat

	event, err := svc.handleAid("u_aid_empty", &gamev1.AidRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	msg := event.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Text, "specify an ally name")
}
```

**Note:** Adjust `NewGameServiceServer` argument count to match the current signature by looking at the existing `newHideSvc` in `grpc_service_hide_test.go` and copying the exact argument list. The argument count MUST match.

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleAid" -v
```

- [ ] **Step 3: Implement `handleAid`**

In `internal/gameserver/grpc_service.go`, add the handler near `handleDelay`:

```go
// handleAid queues an Aid action for the player, targeting the named ally.
//
// Precondition: uid must identify a valid player session; req.Target is the ally name.
// Postcondition: On success in combat, queues ActionAid and returns combat events.
// Out-of-combat: returns an informational message.
func (s *GameServiceServer) handleAid(uid string, req *gamev1.AidRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if sess.Status != statusInCombat {
		return messageEvent("Aid is only valid in combat."), nil
	}

	target := req.GetTarget()
	if target == "" {
		return messageEvent("Please specify an ally name: aid <ally>"), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}

	events, err := s.combatH.Aid(uid, target)
	if err != nil {
		return messageEvent(err.Error()), nil
	}
	return combatEventsEvent(events), nil
}
```

**Note:** Check how other handlers return combat events — look at `handleStrike` or `handleAttack` for the `combatEventsEvent` helper pattern. If that helper doesn't exist, use `messageEvent(events[0].Narrative)` as a fallback or find the correct helper.

- [ ] **Step 4: Add `Aid` case to the dispatch switch**

In `internal/gameserver/grpc_service.go`, in the `switch p := req.Payload.(type)` dispatch block (around line 1434), add after the `*gamev1.ClientMessage_Delay` case:

```go
case *gamev1.ClientMessage_Aid:
    return s.handleAid(uid, p.Aid)
```

- [ ] **Step 5: Run tests — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleAid" -v
```

- [ ] **Step 6: Run full gameserver suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | tail -10
```

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_aid_test.go && git commit -m "feat: add handleAid gRPC handler and dispatch case"
```

---

## Task 10: Swap AP enforcement in `handleLoadout`

**Spec refs:** REQ-ACT19, REQ-ACT20, REQ-ACT21, REQ-ACT25, REQ-ACT26, REQ-ACT27

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_loadout_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_loadout_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newLoadoutSvcWithCombat(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeTestConditionRegistry()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// setupLoadoutSession creates a player with two weapon presets for loadout tests.
func setupLoadoutSession(t *testing.T, sessMgr *session.Manager, uid, roomID string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Gunner", CharName: "Gunner", RoomID: roomID,
		CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	// Give the player two presets.
	sess.LoadoutSet = &inventory.LoadoutSet{
		Presets: []*inventory.WeaponPreset{
			{Slot: 1, Name: "Preset 1"},
			{Slot: 2, Name: "Preset 2"},
		},
		ActivePreset: 1,
	}
	return sess
}

// TestHandleLoadout_InCombat_NoAP verifies that swapping in combat with 0 AP
// returns "Not enough AP to swap loadouts." and does not change the active preset.
func TestHandleLoadout_InCombat_NoAP(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newLoadoutSvcWithCombat(t)
	const roomID = "room_loadout_ap"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "enemy-lo", Name: "Enemy", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)
	sess := setupLoadoutSession(t, sessMgr, "u_lo_ap", roomID)
	sess.Status = statusInCombat

	// Start combat and exhaust AP.
	_, err = combatHandler.Attack("u_lo_ap", "Enemy")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)
	_, err = combatHandler.Strike("u_lo_ap", "Enemy")
	require.NoError(t, err) // 0 AP remaining

	event, err := svc.handleLoadout("u_lo_ap", &gamev1.LoadoutRequest{Arg: "2"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msg := event.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Text, "Not enough AP to swap loadouts.")
	assert.Equal(t, 1, sess.LoadoutSet.ActivePreset, "active preset must not change")
}

// TestHandleLoadout_InCombat_WithAP verifies that swapping in combat with >= 1 AP
// succeeds, deducts exactly 1 AP, and updates the active preset.
func TestHandleLoadout_InCombat_WithAP(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newLoadoutSvcWithCombat(t)
	const roomID = "room_loadout_ok"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "enemy-lo2", Name: "Enemy2", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)
	sess := setupLoadoutSession(t, sessMgr, "u_lo_ok", roomID)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_lo_ok", "Enemy2")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_lo_ok") // should be 2
	require.GreaterOrEqual(t, apBefore, 1)

	event, err := svc.handleLoadout("u_lo_ok", &gamev1.LoadoutRequest{Arg: "2"})
	require.NoError(t, err)
	require.NotNil(t, event)

	apAfter := combatHandler.RemainingAP("u_lo_ok")
	assert.Equal(t, apBefore-1, apAfter, "exactly 1 AP should be deducted")
	assert.Equal(t, 2, sess.LoadoutSet.ActivePreset, "active preset should be updated to 2")
}

// TestHandleLoadout_OutOfCombat_NoAPCheck verifies that out-of-combat swaps
// succeed without any AP check.
func TestHandleLoadout_OutOfCombat_NoAPCheck(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npc.NewManager(), nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
	)
	sess := setupLoadoutSession(t, sessMgr, "u_lo_ooc", "room_a")
	// Status is not statusInCombat (default).

	event, err := svc.handleLoadout("u_lo_ooc", &gamev1.LoadoutRequest{Arg: "2"})
	require.NoError(t, err)
	require.NotNil(t, event)
	assert.Equal(t, 2, sess.LoadoutSet.ActivePreset)
}
```

**Note:** The `NewGameServiceServer` argument lists in test helpers MUST match the current signature exactly. Count the arguments in `newHideSvc` (in `grpc_service_hide_test.go`) and use the same count and nil pattern. If `combatHandler.cancelTimer` is unexported from CombatHandler (it is — `cancelTimer(roomID string)` exists per test file), it's accessible from `package gameserver` tests directly.

Also: `inventory.LoadoutSet` and `inventory.WeaponPreset` — check their exact field names by reading `internal/game/inventory/loadout.go`.

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleLoadout" -v
```

- [ ] **Step 3: Patch `handleLoadout` in `grpc_service.go`**

Replace the current `handleLoadout` implementation:

```go
// handleLoadout displays or swaps weapon presets for the player.
// When in combat and arg is non-empty, costs 1 AP.
//
// Precondition: uid must be a valid connected player with a non-nil LoadoutSet.
// Postcondition: Returns a ServerEvent with the loadout display or swap result.
func (s *GameServiceServer) handleLoadout(uid string, req *gamev1.LoadoutRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	arg := req.GetArg()
	if arg != "" && sess.Status == statusInCombat {
		if s.combatH != nil {
			if err := s.combatH.SpendAP(uid, 1); err != nil {
				return messageEvent("Not enough AP to swap loadouts."), nil
			}
		}
	}
	if arg != "" {
		return messageEvent(command.HandleLoadout(sess, arg)), nil
	}
	flavor := technology.FlavorFor(technology.DominantTradition(sess.Class))
	weaponSection := command.HandleLoadout(sess, "")
	prepSection := technology.FormatPreparedTechs(sess.PreparedTechs, flavor)
	return messageEvent(weaponSection + "\n\n" + prepSection), nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleLoadout" -v
```

- [ ] **Step 5: Run full gameserver suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | tail -10
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_loadout_test.go && git commit -m "feat: enforce 1 AP cost on weapon preset swap in combat (handleLoadout)"
```

---

## Task 11: Full test suite + update feature docs

**Spec refs:** REQ-ACT29

**Files:**
- Modify: `docs/features/actions.md` (mark Aid and Swap done)
- Modify: `docs/features/index.yaml` (mark `actions` as `done` if remaining items are all blocked)

- [ ] **Step 1: Run the full required test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... ./internal/game/combat/... ./internal/game/condition/... ./internal/frontend/handlers/... ./internal/game/command/... -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -30
```

Expected: all `ok`, no `FAIL`.

- [ ] **Step 2: Run all tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "^(ok|FAIL)" | head -30
```

Expected: all `ok`, no `FAIL`.

- [ ] **Step 3: Update `docs/features/actions.md`**

Mark the Aid and Swap items as done:

- `[x]` Aid action
- `[x]` Swap (loadout swap costs 1 AP in combat)

- [ ] **Step 4: Update `docs/features/index.yaml` (conditional)**

Read `docs/features/actions.md` and check whether all non-blocked items are now done. Items that depend on unbuilt features (Exploration modes, Downtime activities, Gear actions requiring item infrastructure) are considered blocked/out-of-scope.

If all in-scope Action items are complete after marking Aid and Swap, update `status` to `done` in `docs/features/index.yaml`:

```yaml
  - slug: actions
    name: Actions
    status: done
    ...
```

If there are remaining in-scope items not yet implemented, leave `status: in_progress` and update only the `effort` comment to reflect what's still pending.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add docs/features/actions.md docs/features/index.yaml && git commit -m "docs: mark Aid, Swap, and Actions feature as complete"
```
