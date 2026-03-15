# NPC Ability Triggers Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `apply_mental_state` HTN operators so NPCs can inflict Rage, Despair, and Delirium conditions on players during combat, with per-NPC cooldowns, AP costs, and taunt messages.

**Architecture:** Extend `Operator` and `PlannedAction` with mental state metadata. Track damage dealt per player on the `Combat` struct for `highest_damage_enemy` targeting. Add `AbilityCooldowns` to `npc.Instance`. In `applyPlanLocked`, handle `apply_mental_state` by resolving target, calling `mentalStateMgr.ApplyTrigger`, pushing a taunt, and setting cooldown. Create per-NPC HTN domain YAML files and Lua scripts for all 18 NPCs.

**Tech Stack:** Go, YAML (HTN domains), Lua (HTN preconditions), `pgregory.net/rapid` (property-based tests), `go test ./... -race`

---

## Chunk 1: Data Model + Combat Execution

### Task 1: Data Model Extensions

**Files:**
- Modify: `internal/game/ai/domain.go`
- Modify: `internal/game/ai/planner.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/combat/engine.go`
- Test: `internal/game/ai/domain_test.go` (new or add to existing)
- Test: `internal/game/combat/engine_test.go` (add to existing)

**Background:** The current `Operator` struct has three fields (`ID`, `Action`, `Target`). `PlannedAction` has two (`Action`, `Target`). We need metadata to flow from operator definition through the planner into execution without reverse lookups. We also need `DamageDealt` on `Combat` to support the `highest_damage_enemy` target selector.

---

- [ ] **Step 1: Write failing test — Operator YAML unmarshaling with new fields**

In `internal/game/ai/domain_test.go`:

```go
package ai_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/ai"
    "gopkg.in/yaml.v3"
)

func TestOperator_UnmarshalNewFields(t *testing.T) {
    raw := `
domain:
  id: test_domain
  tasks:
    - id: behave
      description: root
    - id: fight
      description: fight
  methods:
    - task: behave
      id: go_fight
      precondition: ""
      subtasks: [fight]
    - task: fight
      id: do_taunt
      precondition: ""
      subtasks: [taunt_op]
  operators:
    - id: taunt_op
      action: apply_mental_state
      target: nearest_enemy
      track: rage
      severity: mild
      cooldown_rounds: 3
      ap_cost: 1
`
    var wrapper struct {
        Domain ai.Domain `yaml:"domain"`
    }
    if err := yaml.Unmarshal([]byte(raw), &wrapper); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    d := wrapper.Domain
    if len(d.Operators) != 1 {
        t.Fatalf("want 1 operator, got %d", len(d.Operators))
    }
    op := d.Operators[0]
    if op.Track != "rage" {
        t.Errorf("Track: want %q, got %q", "rage", op.Track)
    }
    if op.Severity != "mild" {
        t.Errorf("Severity: want %q, got %q", "mild", op.Severity)
    }
    if op.CooldownRounds != 3 {
        t.Errorf("CooldownRounds: want 3, got %d", op.CooldownRounds)
    }
    if op.APCost != 1 {
        t.Errorf("APCost: want 1, got %d", op.APCost)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestOperator_UnmarshalNewFields -v
```

Expected: FAIL (fields not defined on Operator struct).

- [ ] **Step 3: Add four fields to Operator in `internal/game/ai/domain.go`**

Find the `Operator` struct (currently: `ID`, `Action`, `Target`). Add after `Target`:

```go
// Track is the mental state track for apply_mental_state operators.
// One of "rage", "despair", "delirium". Empty string = not a mental state ability.
Track string `yaml:"track,omitempty"`

// Severity is the minimum severity to apply: "mild", "moderate", or "severe".
Severity string `yaml:"severity,omitempty"`

// CooldownRounds is the number of rounds before this operator can fire again.
CooldownRounds int `yaml:"cooldown_rounds,omitempty"`

// APCost is the AP consumed when this operator executes. Zero treated as 1.
APCost int `yaml:"ap_cost,omitempty"`
```

Also update the `Action` field doc comment and `Validate()` doc comment to include `"apply_mental_state"` in the list of valid action strings.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestOperator_UnmarshalNewFields -v
```

Expected: PASS.

- [ ] **Step 5: Write failing test — PlannedAction fields propagated from Operator**

Add to `internal/game/ai/domain_test.go`:

```go
func TestPlanner_PropagatesOperatorMetadata(t *testing.T) {
    // Build a domain with one apply_mental_state operator
    raw := `
domain:
  id: meta_test
  tasks:
    - id: behave
      description: root
    - id: fight
      description: fight
  methods:
    - task: behave
      id: start_fight
      precondition: ""
      subtasks: [fight]
    - task: fight
      id: do_taunt
      precondition: ""
      subtasks: [ability_op]
  operators:
    - id: ability_op
      action: apply_mental_state
      target: nearest_enemy
      track: despair
      severity: moderate
      cooldown_rounds: 4
      ap_cost: 2
`
    var wrapper struct {
        Domain ai.Domain `yaml:"domain"`
    }
    if err := yaml.Unmarshal([]byte(raw), &wrapper); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    // mockScriptCaller (already defined in combat_handler_htn_test.go) returns
    // lua.LTrue for every precondition, making all Lua checks pass.
    caller := &mockScriptCaller{}
    planner := ai.NewPlanner(&wrapper.Domain, caller, "")

    ws := &ai.WorldState{
        NPC: &ai.NPCState{UID: "npc1"},
        Combatants: []ai.CombatantState{
            {UID: "p1", Kind: "player", HP: 20, MaxHP: 40, Dead: false},
        },
    }
    actions, err := planner.Plan(ws)
    if err != nil {
        t.Fatalf("Plan: %v", err)
    }
    if len(actions) != 1 {
        t.Fatalf("want 1 action, got %d", len(actions))
    }
    a := actions[0]
    if a.OperatorID != "ability_op" {
        t.Errorf("OperatorID: want %q, got %q", "ability_op", a.OperatorID)
    }
    if a.Track != "despair" {
        t.Errorf("Track: want %q, got %q", "despair", a.Track)
    }
    if a.Severity != "moderate" {
        t.Errorf("Severity: want %q, got %q", "moderate", a.Severity)
    }
    if a.CooldownRounds != 4 {
        t.Errorf("CooldownRounds: want 4, got %d", 4)
    }
    if a.APCost != 2 {
        t.Errorf("APCost: want 2, got %d", a.APCost)
    }
}
```

Note: Both `TestPlanner_PropagatesOperatorMetadata` and `TestOperator_UnmarshalNewFields` live in `package ai_test`. The `mockScriptCaller` type is defined in `internal/gameserver/combat_handler_htn_test.go` (package `gameserver`), which is not visible here. Define a local `noopScriptCaller` at the top of `domain_test.go`:

```go
// noopScriptCaller satisfies ai.ScriptCaller; every precondition returns true.
type noopScriptCaller struct{}
func (n *noopScriptCaller) CallHook(_, _ string, _ ...lua.LValue) (lua.LValue, error) {
    return lua.LTrue, nil
}
```

Import `lua "github.com/yuin/gopher-lua"` at the top of `domain_test.go`. Then use `&noopScriptCaller{}` in the `NewPlanner` call above.

- [ ] **Step 6: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestPlanner_PropagatesOperatorMetadata -v
```

Expected: FAIL (PlannedAction has no OperatorID/Track/Severity/etc. fields).

- [ ] **Step 7: Extend PlannedAction in `internal/game/ai/planner.go`**

Find the `PlannedAction` struct (currently has `Action string` and `Target string`). Add:

```go
// OperatorID is the ID of the operator that produced this action.
// Empty for legacy/fallback-generated actions.
OperatorID string

// Track is the mental state track for "apply_mental_state" actions.
Track string

// Severity is the severity level for "apply_mental_state" actions.
Severity string

// CooldownRounds is the cooldown to set after execution.
CooldownRounds int

// APCost is the AP consumed by this action. Zero means default (1).
APCost int
```

Find where `PlannedAction` is constructed from an `Operator` in `Plan()`. Update the construction to copy all fields:

```go
pa := PlannedAction{
    Action:         op.Action,
    Target:         op.Target,
    OperatorID:     op.ID,
    Track:          op.Track,
    Severity:       op.Severity,
    CooldownRounds: op.CooldownRounds,
    APCost:         op.APCost,
}
```

- [ ] **Step 8: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestPlanner_PropagatesOperatorMetadata -v
```

Expected: PASS.

- [ ] **Step 9: Write failing test — DamageDealt on new Combat, RecordDamage accumulation**

Add to or create `internal/game/combat/engine_ability_test.go`:

```go
package combat_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/combat"
)

func TestCombat_DamageDealt_InitializedNonNil(t *testing.T) {
    e := combat.NewEngine()
    c1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 10}
    c2 := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 20, CurrentHP: 20, Initiative: 5}
    cbt, err := e.StartCombat("room1", []*combat.Combatant{c1, c2}, nil, nil, "zone1")
    if err != nil {
        t.Fatalf("StartCombat: %v", err)
    }
    if cbt.DamageDealt == nil {
        t.Fatal("DamageDealt is nil; want initialized map")
    }
}

func TestCombat_RecordDamage_Accumulates(t *testing.T) {
    e := combat.NewEngine()
    c1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 10}
    c2 := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 100, CurrentHP: 100, Initiative: 5}
    cbt, err := e.StartCombat("room1", []*combat.Combatant{c1, c2}, nil, nil, "zone1")
    if err != nil {
        t.Fatalf("StartCombat: %v", err)
    }
    cbt.RecordDamage("p1", 10)
    cbt.RecordDamage("p1", 7)
    if got := cbt.DamageDealt["p1"]; got != 17 {
        t.Errorf("DamageDealt[p1]: want 17, got %d", got)
    }
}
```

- [ ] **Step 10: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestCombat_DamageDealt|TestCombat_RecordDamage" -v
```

Expected: FAIL (no DamageDealt field, no RecordDamage method).

- [ ] **Step 11: Add DamageDealt to Combat struct and RecordDamage method in `internal/game/combat/engine.go`**

In the `Combat` struct, add after `Conditions`:

```go
// DamageDealt maps combatant UID → total damage dealt this combat.
// Initialized in StartCombat. Used for highest_damage_enemy target selection.
DamageDealt map[string]int
```

Add method (new function at end of engine.go):

```go
// RecordDamage adds amount to the running total for attackerUID.
// Precondition: amount >= 0.
func (c *Combat) RecordDamage(attackerUID string, amount int) {
    c.DamageDealt[attackerUID] += amount
}
```

In `StartCombat`, add `DamageDealt: make(map[string]int)` to the `cbt := &Combat{...}` struct literal alongside `ActionQueues` and `Conditions`.

- [ ] **Step 12: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestCombat_DamageDealt|TestCombat_RecordDamage" -v
```

Expected: PASS.

- [ ] **Step 13: Write test — AbilityCooldowns lazy init (no panic on nil map write)**

Add to `internal/game/npc/instance_test.go` (or new file):

```go
func TestInstance_AbilityCooldowns_LazyInit(t *testing.T) {
    // Verify AbilityCooldowns starts nil and can be safely ranged over.
    inst := &npc.Instance{}
    if inst.AbilityCooldowns != nil {
        t.Error("AbilityCooldowns should be nil at zero value")
    }
    // Range over nil map is safe in Go — this must not panic.
    count := 0
    for range inst.AbilityCooldowns {
        count++
    }
    if count != 0 {
        t.Errorf("expected 0 iterations over nil map, got %d", count)
    }
}
```

- [ ] **Step 14: Run test to verify it passes (field already exists at zero value)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestInstance_AbilityCooldowns_LazyInit -v
```

Expected: FAIL because `AbilityCooldowns` field doesn't exist yet.

- [ ] **Step 15: Add AbilityCooldowns to Instance in `internal/game/npc/instance.go`**

In the `Instance` struct, add after the last field:

```go
// AbilityCooldowns maps operator ID → rounds remaining until usable again.
// Nil at spawn; initialized lazily on first write in applyPlanLocked.
AbilityCooldowns map[string]int
```

Do NOT initialize this in `NewInstanceWithResolver` — it is nil by design (lazy init).

- [ ] **Step 16: Run all tests to verify no regressions**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/... -v -count=1 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 17: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/ai/domain.go internal/game/ai/planner.go internal/game/npc/instance.go internal/game/combat/engine.go internal/game/ai/domain_test.go internal/game/combat/engine_ability_test.go internal/game/npc/instance_test.go
git commit -m "feat(ai): extend Operator/PlannedAction with mental state fields; add DamageDealt to Combat; add AbilityCooldowns to Instance"
```

---

### Task 2: Combat Execution

**Files:**
- Modify: `internal/game/combat/round.go` — call `cbt.RecordDamage` at all player-hits-NPC damage sites
- Modify: `internal/gameserver/combat_handler.go` — cooldown decrement in `autoQueueNPCsLocked`; `apply_mental_state` case in `applyPlanLocked`
- Test: `internal/gameserver/grpc_service_ability_test.go` (new, `package gameserver`)

**Background:**

`round.go` calls `target.ApplyDamage(dmg)` at six sites. We need to call `cbt.RecordDamage(actor.ID, dmg)` **only when `actor.Kind == KindPlayer && target.Kind == KindNPC && dmg > 0`**. The six sites are approximately at lines 569 (ActionAttack), 663 (ActionStrike first hit), 733 (ActionStrike second hit), 831 (ActionFireBurst), 909 (ActionFireAutomatic), 950 (ActionThrow). Search `round.go` for `ApplyDamage` to locate all of them exactly.

`autoQueueNPCsLocked` (starts ~line 1910 of `combat_handler.go`) iterates over `cbt.Combatants`. We add cooldown decrement at the START of the function, before any planning loop.

`applyPlanLocked` (~line 1971) has a switch on `a.Action`. We add a `"apply_mental_state"` case alongside `"attack"`, `"strike"`, `"pass"`.

**Package convention:** All tests in this task use `package gameserver` (white-box). The existing `combat_handler_htn_test.go` already defines `mockScriptCaller` and `makeHTNCombatHandler` — reuse them. `makeTestConditionRegistry()` is also already defined in the test suite; use it to get a condition registry that has all mental-state condition definitions loaded.

**`cbt.Combatants` is a `[]*Combatant` slice** (not a map), so iteration order is deterministic (initiative order). The `highest_damage_enemy` tie-breaking rule (first in `Combatants`) is therefore guaranteed by slice iteration order.

**AP cost:** Each `apply_mental_state` action deducts `APCost` AP units by calling `cbt.QueueAction` with `ActionPass` exactly `APCost` times (each pass consumes 1 AP slot without executing a game action). If any QueueAction call returns an error (AP budget exhausted), execution stops immediately.

---

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_ability_test.go` (package `gameserver`, white-box):

```go
package gameserver

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/ai"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/dice"
    "github.com/cory-johannsen/mud/internal/game/mentalstate"
    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    "go.uber.org/zap"
)

// makeAbilityCombatHandler constructs a CombatHandler suitable for ability tests.
// Uses makeHTNCombatHandler (defined in combat_handler_htn_test.go) with a nil aiRegistry
// and injects a mentalStateMgr and condRegistry.
func makeAbilityCombatHandler(t *testing.T, msMgr *mentalstate.Manager) *CombatHandler {
    t.Helper()
    src := dice.NewCryptoSource()
    roller := dice.NewLoggedRoller(src, zap.NewNop())
    engine := combat.NewEngine()
    npcMgr := npc.NewManager()
    sessMgr := session.NewManager()
    condReg := makeTestConditionRegistry()
    h := NewCombatHandler(
        engine,
        npcMgr,
        sessMgr,
        roller,
        func(string, []*gamev1.CombatEvent) {}, // noop broadcast
        testRoundDuration,
        condReg,
        nil, nil, nil, nil, nil, nil,
        msMgr,
    )
    return h
}

// TestApplyPlanLocked_ApplyMentalState_OnCooldown verifies that when
// AbilityCooldowns[operatorID] > 0, the apply_mental_state action is skipped.
func TestApplyPlanLocked_ApplyMentalState_OnCooldown(t *testing.T) {
    msMgr := mentalstate.NewManager()
    h := makeAbilityCombatHandler(t, msMgr)

    // Register NPC instance with active cooldown.
    npcInst := &npc.Instance{ID: "npc1", AbilityCooldowns: map[string]int{"ganger_taunt": 2}}
    h.npcMgr.Add(npcInst)

    // Register player session.
    sess := &session.PlayerSession{UID: "player1", CharName: "Alice", RoomID: "room1"}
    sess.Conditions = condition.NewActiveSet()
    h.sessions.AddPlayer(sess)

    playerCbt := &combat.Combatant{ID: "player1", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 10}
    npcCbt := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 20, CurrentHP: 20, Initiative: 5}
    cbt, _ := h.engine.StartCombat("room1", []*combat.Combatant{playerCbt, npcCbt}, h.condRegistry, nil, "zone1")
    _ = cbt.StartRound(3)

    actions := []ai.PlannedAction{{
        Action: "apply_mental_state", Target: "nearest_enemy",
        OperatorID: "ganger_taunt", Track: "rage", Severity: "mild",
        CooldownRounds: 3, APCost: 1,
    }}

    h.applyPlanLocked(cbt, npcCbt, actions)

    // Ability should NOT have fired — cooldown active.
    if got := msMgr.CurrentSeverity("player1", mentalstate.TrackRage); got != mentalstate.SeverityNone {
        t.Errorf("expected SeverityNone (cooldown active), got %v", got)
    }
    if npcInst.AbilityCooldowns["ganger_taunt"] != 2 {
        t.Errorf("cooldown should remain 2, got %d", npcInst.AbilityCooldowns["ganger_taunt"])
    }
}

// TestApplyPlanLocked_ApplyMentalState_Execute verifies that when cooldown == 0,
// the ability fires, mental state is applied, and cooldown is set.
func TestApplyPlanLocked_ApplyMentalState_Execute(t *testing.T) {
    msMgr := mentalstate.NewManager()
    h := makeAbilityCombatHandler(t, msMgr)

    npcInst := &npc.Instance{ID: "npc1", Taunts: []string{"You call that fighting?"}}
    h.npcMgr.Add(npcInst)

    sess := &session.PlayerSession{UID: "player1", CharName: "Alice", RoomID: "room1"}
    sess.Conditions = condition.NewActiveSet()
    h.sessions.AddPlayer(sess)

    playerCbt := &combat.Combatant{ID: "player1", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 10}
    npcCbt := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 20, CurrentHP: 20, Initiative: 5}
    cbt, _ := h.engine.StartCombat("room1", []*combat.Combatant{playerCbt, npcCbt}, h.condRegistry, nil, "zone1")
    _ = cbt.StartRound(3)

    actions := []ai.PlannedAction{{
        Action: "apply_mental_state", Target: "nearest_enemy",
        OperatorID: "ganger_taunt", Track: "rage", Severity: "mild",
        CooldownRounds: 3, APCost: 1,
    }}

    h.applyPlanLocked(cbt, npcCbt, actions)

    if got := msMgr.CurrentSeverity("player1", mentalstate.TrackRage); got != mentalstate.SeverityMild {
        t.Errorf("expected SeverityMild, got %v", got)
    }
    if npcInst.AbilityCooldowns["ganger_taunt"] != 3 {
        t.Errorf("cooldown: want 3, got %d", npcInst.AbilityCooldowns["ganger_taunt"])
    }
}

// TestAutoQueueNPCsLocked_CooldownDecrement verifies that AbilityCooldowns
// are decremented (floored at 0) at the start of each call to autoQueueNPCsLocked.
func TestAutoQueueNPCsLocked_CooldownDecrement(t *testing.T) {
    msMgr := mentalstate.NewManager()
    h := makeAbilityCombatHandler(t, msMgr)

    npcInst := &npc.Instance{
        ID:               "npc1",
        AbilityCooldowns: map[string]int{"ganger_taunt": 2, "other_op": 0},
    }
    h.npcMgr.Add(npcInst)

    npcCbt := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 20, CurrentHP: 20, Initiative: 5}
    cbt, _ := h.engine.StartCombat("room1", []*combat.Combatant{npcCbt}, h.condRegistry, nil, "zone1")
    _ = cbt.StartRound(3)

    h.autoQueueNPCsLocked(cbt)

    if npcInst.AbilityCooldowns["ganger_taunt"] != 1 {
        t.Errorf("ganger_taunt cooldown: want 1, got %d", npcInst.AbilityCooldowns["ganger_taunt"])
    }
    if npcInst.AbilityCooldowns["other_op"] != 0 {
        t.Errorf("other_op cooldown: want 0 (floored), got %d", npcInst.AbilityCooldowns["other_op"])
    }
}

// TestApplyPlanLocked_TargetSelector_HighestDamage verifies highest_damage_enemy
// picks the player with highest DamageDealt entry; on tie, first in Combatants slice.
func TestApplyPlanLocked_TargetSelector_HighestDamage(t *testing.T) {
    msMgr := mentalstate.NewManager()
    h := makeAbilityCombatHandler(t, msMgr)

    npcInst := &npc.Instance{ID: "npc1"}
    h.npcMgr.Add(npcInst)

    sess1 := &session.PlayerSession{UID: "p1", CharName: "Alice", RoomID: "room1"}
    sess2 := &session.PlayerSession{UID: "p2", CharName: "Bob", RoomID: "room1"}
    sess1.Conditions = condition.NewActiveSet()
    sess2.Conditions = condition.NewActiveSet()
    h.sessions.AddPlayer(sess1)
    h.sessions.AddPlayer(sess2)

    // p1 has higher initiative (first in Combatants after sort).
    p1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 10}
    p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 9}
    npcCbt := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 20, CurrentHP: 20, Initiative: 5}
    cbt, _ := h.engine.StartCombat("room1", []*combat.Combatant{p1, p2, npcCbt}, h.condRegistry, nil, "zone1")
    _ = cbt.StartRound(3)

    cbt.RecordDamage("p1", 5)
    cbt.RecordDamage("p2", 15) // p2 dealt more damage → should be targeted

    actions := []ai.PlannedAction{{
        Action: "apply_mental_state", Target: "highest_damage_enemy",
        OperatorID: "op1", Track: "despair", Severity: "mild", CooldownRounds: 3, APCost: 1,
    }}
    h.applyPlanLocked(cbt, npcCbt, actions)

    if got := msMgr.CurrentSeverity("p2", mentalstate.TrackDespair); got != mentalstate.SeverityMild {
        t.Errorf("p2 (highest damage) should have despair mild, got %v", got)
    }
    if got := msMgr.CurrentSeverity("p1", mentalstate.TrackDespair); got != mentalstate.SeverityNone {
        t.Errorf("p1 should NOT have despair, got %v", got)
    }
}
```

**Note:** `h.npcMgr.Add(inst)`, `h.sessions.AddPlayer(sess)`, `h.engine` etc. are package-level fields accessible from `package gameserver`. Check the exact method names by looking at existing white-box tests like `combat_handler_htn_test.go`. If the manager uses a different method name (e.g., `AddInstance`, `Register`), use that. Also check whether `npc.Instance` needs `Name` set for the fallback taunt — add `TemplateID: "npc1"` or a `Template` ref if `inst.Name()` panics on a nil template.

- [ ] **Step 2: Run to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestApplyPlanLocked|TestAutoQueueNPCsLocked" -v 2>&1 | head -30
```

Expected: FAIL (undefined fields on PlannedAction, no apply_mental_state case).

- [ ] **Step 3: Add cooldown decrement to `autoQueueNPCsLocked` in `combat_handler.go`**

At the very start of `autoQueueNPCsLocked`, before the room lookup, add:

```go
// Decrement ability cooldowns for all living NPCs before planning.
for _, c := range cbt.Combatants {
    if c.Kind != combat.KindNPC || c.IsDead() {
        continue
    }
    if inst, ok := h.npcMgr.Get(c.ID); ok {
        for k := range inst.AbilityCooldowns {
            if inst.AbilityCooldowns[k] > 0 {
                inst.AbilityCooldowns[k]--
            }
        }
    }
}
```

- [ ] **Step 4: Add `"apply_mental_state"` case to `applyPlanLocked` in `combat_handler.go`**

In `applyPlanLocked`, add a new case to the switch before `default`. The case resolves target, checks cooldown, applies the mental state, then deducts AP by queueing `APCost` pass actions. If the NPC instance is not found, log a warning and skip (no AP deducted). If any AP queue call fails, return immediately.

```go
case "apply_mental_state":
    // Resolve target selector.
    targetUID := h.resolveAbilityTarget(cbt, a.Target)
    if targetUID == "" {
        continue // no valid target; no AP deducted, no cooldown set
    }

    // Parse track and severity.
    track, ok := abilityTrack(a.Track)
    if !ok {
        h.logger.Warn("apply_mental_state: unknown track", zap.String("track", a.Track))
        continue
    }
    sev, ok := abilitySeverity(a.Severity)
    if !ok {
        h.logger.Warn("apply_mental_state: unknown severity", zap.String("severity", a.Severity))
        continue
    }

    // Look up NPC instance; skip if not found (should not happen in practice).
    inst, ok := h.npcMgr.Get(actor.ID)
    if !ok {
        h.logger.Warn("apply_mental_state: NPC instance not found", zap.String("id", actor.ID))
        continue // no AP deducted
    }

    // Cooldown gate — reading a nil map returns zero value (safe).
    if inst.AbilityCooldowns[a.OperatorID] > 0 {
        continue // still on cooldown; no AP deducted
    }

    // Apply mental state trigger.
    if h.mentalStateMgr != nil {
        changes := h.mentalStateMgr.ApplyTrigger(targetUID, track, sev)
        msgs := h.applyMentalStateChanges(targetUID, changes)
        if targSess, ok := h.sessions.GetPlayer(targetUID); ok && targSess.Entity != nil {
            for _, msg := range msgs {
                _ = targSess.Entity.Push([]byte(msg + "\n"))
            }
        }
    }

    // Push taunt message to target.
    taunt := h.pickTaunt(inst)
    if targSess, ok := h.sessions.GetPlayer(targetUID); ok && targSess.Entity != nil {
        _ = targSess.Entity.Push([]byte(taunt + "\n"))
    }

    // Set cooldown (lazy-initialize map on first write).
    if inst.AbilityCooldowns == nil {
        inst.AbilityCooldowns = make(map[string]int)
    }
    inst.AbilityCooldowns[a.OperatorID] = a.CooldownRounds

    // Deduct AP: queue APCost pass actions (each costs 1 AP slot).
    apCost := a.APCost
    if apCost == 0 {
        apCost = 1
    }
    for i := 0; i < apCost; i++ {
        if err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionPass}); err != nil {
            return // AP budget exhausted
        }
    }
    continue
```

Add the helper functions (unexported, at the bottom of `combat_handler.go`):

```go
// resolveAbilityTarget resolves a target selector to a living player UID.
// cbt.Combatants is a slice in initiative-descending order; iteration is deterministic.
// Returns "" if no valid target exists.
func (h *CombatHandler) resolveAbilityTarget(cbt *combat.Combat, selector string) string {
    switch selector {
    case "nearest_enemy":
        for _, c := range cbt.Combatants {
            if c.Kind == combat.KindPlayer && !c.IsDead() {
                return c.ID
            }
        }
    case "lowest_hp_enemy":
        var best *combat.Combatant
        for _, c := range cbt.Combatants {
            if c.Kind != combat.KindPlayer || c.IsDead() {
                continue
            }
            if best == nil || c.CurrentHP < best.CurrentHP {
                best = c
            }
        }
        if best != nil {
            return best.ID
        }
    case "highest_damage_enemy":
        // Iterate in Combatants slice order (initiative-descending).
        // On tie, first encountered player wins (deterministic tie-break).
        var bestUID string
        bestDmg := -1
        for _, c := range cbt.Combatants {
            if c.Kind != combat.KindPlayer || c.IsDead() {
                continue
            }
            dmg := cbt.DamageDealt[c.ID]
            if dmg > bestDmg {
                bestUID = c.ID
                bestDmg = dmg
            }
        }
        return bestUID
    }
    return ""
}

// abilityTrack converts a string track name to a mentalstate.Track.
func abilityTrack(s string) (mentalstate.Track, bool) {
    switch s {
    case "rage":
        return mentalstate.TrackRage, true
    case "despair":
        return mentalstate.TrackDespair, true
    case "delirium":
        return mentalstate.TrackDelirium, true
    }
    return 0, false
}

// abilitySeverity converts a string severity name to a mentalstate.Severity.
func abilitySeverity(s string) (mentalstate.Severity, bool) {
    switch s {
    case "mild":
        return mentalstate.SeverityMild, true
    case "moderate":
        return mentalstate.SeverityMod, true
    case "severe":
        return mentalstate.SeveritySevere, true
    }
    return 0, false
}

// pickTaunt returns a random taunt from inst.Taunts, or a generic fallback.
func (h *CombatHandler) pickTaunt(inst *npc.Instance) string {
    if len(inst.Taunts) == 0 {
        return fmt.Sprintf("The %s unsettles you.", inst.Name())
    }
    idx := h.dice.Src().Int63() % int64(len(inst.Taunts))
    return inst.Taunts[idx]
}
```

- [ ] **Step 5: Call `cbt.RecordDamage` in `round.go` at all player-→-NPC damage sites**

Open `internal/game/combat/round.go`. Search for all `ApplyDamage` call sites. At each site where the attacker is a player and the target is an NPC and `dmg > 0`, add **immediately after** `target.ApplyDamage(dmg)`:

```go
if actor.Kind == KindPlayer && target.Kind == KindNPC {
    cbt.RecordDamage(actor.ID, dmg)
}
```

Do this for all 6 ApplyDamage call sites (ActionAttack ~line 569, ActionStrike first ~line 663, ActionStrike second ~line 733, ActionFireBurst ~line 831, ActionFireAutomatic ~line 909, ActionThrow ~line 950). The local variable names for attacker/actor and target may differ per action type — inspect the surrounding code to find the right names. In every case `cbt *Combat` is available as the ResolveRound parameter.

- [ ] **Step 6: Run ability tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestApplyPlanLocked|TestAutoQueueNPCsLocked" -v
```

Expected: PASS.

- [ ] **Step 7: Run full test suite with race detector**

```bash
cd /home/cjohannsen/src/mud && go test ./... -race -count=1 2>&1 | tail -30
```

Expected: all PASS, no race conditions.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go internal/game/combat/round.go internal/gameserver/grpc_service_ability_test.go
git commit -m "feat(combat): add apply_mental_state action; cooldown decrement; RecordDamage in round resolution"
```

---

## Chunk 2: NPC Domain Files

### Task 3: Per-NPC HTN Domains for All 18 NPCs

**Files (per NPC — repeat for all 18):**
- Create: `content/ai/<npc_id>_combat.yaml`
- Create: `content/scripts/ai/<npc_id>.lua`
- Modify: `content/npcs/<npc_id>.yaml` — set `ai_domain: <npc_id>_combat`

**Background:**

Currently, most of the 18 NPCs share either `ganger_combat` or `scavenger_patrol` domains. Since each NPC needs its own unique ability operator, each must have an individual domain file. The new per-NPC domain is a standalone full domain (not inheriting from another) that includes the base combat behavior AND the ability operator.

**Base domain templates:**

**Ganger-style template** (for: ganger, mill_plain_thug, motel_raider, tarmac_raider, cargo_cultist, brew_warlord, gravel_pit_boss, commissar, bridge_troll):

```yaml
domain:
  id: <npc_id>_combat
  description: Combat behavior for <npc_name>. Attacks and can trigger <track> conditions.

  tasks:
    - id: behave
      description: Root task — fight or idle
    - id: fight
      description: Engage enemy, potentially using ability

  methods:
    - task: behave
      id: combat_mode
      precondition: <npc_id>_has_enemy
      subtasks: [fight]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]

    - task: fight
      id: strike_and_ability
      precondition: <npc_id>_enemy_below_half
      subtasks: [strike_enemy, <operator_id>]

    - task: fight
      id: attack_and_ability
      precondition: <npc_id>_has_enemy
      subtasks: [attack_enemy, <operator_id>]

    - task: fight
      id: attack_only
      precondition: ""
      subtasks: [attack_enemy]

  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy

    - id: strike_enemy
      action: strike
      target: weakest_enemy

    - id: do_pass
      action: pass
      target: ""

    - id: <operator_id>
      action: apply_mental_state
      track: <track>
      severity: <severity>
      target: <target>
      cooldown_rounds: <cooldown>
      ap_cost: <ap_cost>
```

**Ganger-style Lua template** (save as `content/scripts/ai/<npc_id>.lua`):

```lua
-- <npc_id>.lua: HTN preconditions for <npc_id>_combat domain.

function <npc_id>_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function <npc_id>_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
```

**Scavenger-style template** (for: strip_mall_scav, industrial_scav, outlet_scavenger, scavenger, terminal_squatter, highway_bandit, river_pirate, alberta_drifter):

```yaml
domain:
  id: <npc_id>_combat
  description: Combat behavior for <npc_name>. Cautious; uses ability when not outnumbered.

  tasks:
    - id: behave
      description: Root task — fight cautiously or pass
    - id: cautious_fight
      description: Fight only when not outnumbered

  methods:
    - task: behave
      id: cautious_combat
      precondition: <npc_id>_has_enemy
      subtasks: [cautious_fight]

    - task: behave
      id: idle
      precondition: ""
      subtasks: [do_pass]

    - task: cautious_fight
      id: fight_with_ability
      precondition: <npc_id>_not_outnumbered
      subtasks: [attack_enemy, <operator_id>]

    - task: cautious_fight
      id: flee_if_outnumbered
      precondition: ""
      subtasks: [do_pass]

  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy

    - id: do_pass
      action: pass
      target: ""

    - id: <operator_id>
      action: apply_mental_state
      track: <track>
      severity: <severity>
      target: <target>
      cooldown_rounds: <cooldown>
      ap_cost: <ap_cost>
```

**Scavenger-style Lua template** (`content/scripts/ai/<npc_id>.lua`):

```lua
-- <npc_id>.lua: HTN preconditions for <npc_id>_combat domain.

function <npc_id>_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function <npc_id>_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
```

**Lieutenant template** (currently has no `ai_domain`; use ganger-style):

Same as ganger-style template above. After creating the domain, add `ai_domain: lieutenant_combat` to `content/npcs/lieutenant.yaml`.

---

**Operator assignment table** (use these values when filling in `<operator_id>`, `<track>`, `<severity>`, `<target>`, `<cooldown>`, `<ap_cost>` in the templates above):

| NPC ID | Style | Operator ID | Track | Severity | Target | Cooldown | AP |
|--------|-------|-------------|-------|----------|--------|----------|----|
| ganger | ganger | ganger_taunt | rage | mild | highest_damage_enemy | 3 | 1 |
| highway_bandit | scavenger | bandit_intimidate | despair | mild | nearest_enemy | 3 | 1 |
| tarmac_raider | ganger | raider_unsettle | delirium | mild | nearest_enemy | 3 | 1 |
| mill_plain_thug | ganger | thug_taunt | rage | mild | highest_damage_enemy | 3 | 1 |
| motel_raider | ganger | motel_raider_intimidate | despair | mild | nearest_enemy | 3 | 1 |
| river_pirate | scavenger | pirate_unsettle | delirium | mild | nearest_enemy | 4 | 1 |
| strip_mall_scav | scavenger | scav_demoralize | despair | mild | lowest_hp_enemy | 3 | 1 |
| industrial_scav | scavenger | iscav_demoralize | despair | mild | lowest_hp_enemy | 3 | 1 |
| outlet_scavenger | scavenger | outlet_scav_confuse | delirium | mild | nearest_enemy | 4 | 1 |
| scavenger | scavenger | scavenger_confuse | delirium | mild | nearest_enemy | 4 | 1 |
| alberta_drifter | scavenger | drifter_taunt | rage | mild | nearest_enemy | 4 | 1 |
| terminal_squatter | scavenger | squatter_demoralize | despair | mild | lowest_hp_enemy | 4 | 1 |
| cargo_cultist | ganger | cultist_unsettle | delirium | moderate | nearest_enemy | 5 | 2 |
| lieutenant | ganger | lt_intimidate | despair | moderate | lowest_hp_enemy | 4 | 2 |
| brew_warlord | ganger | warlord_enrage | rage | moderate | highest_damage_enemy | 4 | 2 |
| gravel_pit_boss | ganger | pitboss_demoralize | despair | moderate | lowest_hp_enemy | 4 | 2 |
| commissar | ganger | commissar_taunt | rage | moderate | highest_damage_enemy | 4 | 2 |
| bridge_troll | ganger | troll_unsettle | delirium | moderate | lowest_hp_enemy | 5 | 2 |

---

- [ ] **Step 1: Write a failing domain-load test**

Add to `internal/game/ai/domain_test.go`:

```go
func TestLoadDomains_AllNPCDomainsValid(t *testing.T) {
    domains, err := ai.LoadDomains("/home/cjohannsen/src/mud/content/ai")
    if err != nil {
        t.Fatalf("LoadDomains: %v", err)
    }

    requiredDomains := []string{
        "ganger_npc_combat", "highway_bandit_combat", "tarmac_raider_combat",
        "mill_plain_thug_combat", "motel_raider_combat", "river_pirate_combat",
        "strip_mall_scav_combat", "industrial_scav_combat", "outlet_scavenger_combat",
        "scavenger_combat", "alberta_drifter_combat", "terminal_squatter_combat",
        "cargo_cultist_combat", "lieutenant_combat", "brew_warlord_combat",
        "gravel_pit_boss_combat", "commissar_combat", "bridge_troll_combat",
    }

    domainSet := make(map[string]bool, len(domains))
    for _, d := range domains {
        domainSet[d.ID] = true
    }

    for _, req := range requiredDomains {
        if !domainSet[req] {
            t.Errorf("missing domain: %q", req)
        }
    }
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestLoadDomains_AllNPCDomainsValid -v
```

Expected: FAIL (most domain files do not exist yet).

- [ ] **Step 3: Create all 18 per-NPC domain YAML files**

For each NPC in the operator assignment table, create `content/ai/<npc_id>_combat.yaml` using the appropriate template (ganger-style or scavenger-style) and fill in the table values. Below is the **complete ganger example** — follow the same pattern for all others:

`content/ai/ganger_combat.yaml` — **update the existing file** to add the ability (ganger already points to `ganger_combat`):

Wait — ganger currently uses `ganger_combat` which is shared. Instead, create `content/ai/ganger_npc_combat.yaml` for the ganger NPC specifically, and update `content/npcs/ganger.yaml` to use `ai_domain: ganger_npc_combat`. The existing `ganger_combat.yaml` is unchanged (still used by NPCs not in our list).

```yaml
# content/ai/ganger_npc_combat.yaml
domain:
  id: ganger_npc_combat
  description: Combat behavior for ganger. Attacks and can trigger Rage conditions.

  tasks:
    - id: behave
      description: Root task — fight or idle
    - id: fight
      description: Engage enemy, potentially using taunt

  methods:
    - task: behave
      id: combat_mode
      precondition: ganger_npc_has_enemy
      subtasks: [fight]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]

    - task: fight
      id: strike_and_taunt
      precondition: ganger_npc_enemy_below_half
      subtasks: [strike_enemy, ganger_taunt]

    - task: fight
      id: attack_and_taunt
      precondition: ganger_npc_has_enemy
      subtasks: [attack_enemy, ganger_taunt]

    - task: fight
      id: attack_only
      precondition: ""
      subtasks: [attack_enemy]

  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy

    - id: strike_enemy
      action: strike
      target: weakest_enemy

    - id: do_pass
      action: pass
      target: ""

    - id: ganger_taunt
      action: apply_mental_state
      track: rage
      severity: mild
      target: highest_damage_enemy
      cooldown_rounds: 3
      ap_cost: 1
```

`content/scripts/ai/ganger_npc.lua`:

```lua
-- ganger_npc.lua: HTN preconditions for ganger_npc_combat domain.

function ganger_npc_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function ganger_npc_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
```

Update `content/npcs/ganger.yaml`: change `ai_domain: ganger_combat` → `ai_domain: ganger_npc_combat`.

**Follow the same pattern for all 18 NPCs.** Use the operator table above for values. The domain `id` field in the YAML must match the filename (e.g., `id: highway_bandit_combat` in `highway_bandit_combat.yaml`). For scavenger-style NPCs, use the scavenger-style template and `<npc_id>_not_outnumbered` Lua function. For NPCs that currently have `ai_domain` set (all except lieutenant), update the `ai_domain` field in their YAML to point to the new per-NPC domain. For `lieutenant.yaml`, add `ai_domain: lieutenant_combat` (new field).

**Complete NPC → file mapping:**

| NPC YAML | Old ai_domain | New domain file | New lua file | Update NPC yaml |
|----------|---------------|-----------------|--------------|-----------------|
| ganger.yaml | ganger_combat | ganger_npc_combat.yaml | ganger_npc.lua | ganger_npc_combat |
| highway_bandit.yaml | scavenger_patrol | highway_bandit_combat.yaml | highway_bandit.lua | highway_bandit_combat |
| tarmac_raider.yaml | ganger_combat | tarmac_raider_combat.yaml | tarmac_raider.lua | tarmac_raider_combat |
| mill_plain_thug.yaml | ganger_combat | mill_plain_thug_combat.yaml | mill_plain_thug.lua | mill_plain_thug_combat |
| motel_raider.yaml | ganger_combat | motel_raider_combat.yaml | motel_raider.lua | motel_raider_combat |
| river_pirate.yaml | scavenger_patrol | river_pirate_combat.yaml | river_pirate.lua | river_pirate_combat |
| strip_mall_scav.yaml | scavenger_patrol | strip_mall_scav_combat.yaml | strip_mall_scav.lua | strip_mall_scav_combat |
| industrial_scav.yaml | scavenger_patrol | industrial_scav_combat.yaml | industrial_scav.lua | industrial_scav_combat |
| outlet_scavenger.yaml | scavenger_patrol | outlet_scavenger_combat.yaml | outlet_scavenger.lua | outlet_scavenger_combat |
| scavenger.yaml | scavenger_patrol | scavenger_combat.yaml | scavenger_npc.lua | scavenger_combat |
| alberta_drifter.yaml | scavenger_patrol | alberta_drifter_combat.yaml | alberta_drifter.lua | alberta_drifter_combat |
| terminal_squatter.yaml | scavenger_patrol | terminal_squatter_combat.yaml | terminal_squatter.lua | terminal_squatter_combat |
| cargo_cultist.yaml | ganger_combat | cargo_cultist_combat.yaml | cargo_cultist.lua | cargo_cultist_combat |
| lieutenant.yaml | (none) | lieutenant_combat.yaml | lieutenant.lua | lieutenant_combat |
| brew_warlord.yaml | ganger_combat | brew_warlord_combat.yaml | brew_warlord.lua | brew_warlord_combat |
| gravel_pit_boss.yaml | ganger_combat | gravel_pit_boss_combat.yaml | gravel_pit_boss.lua | gravel_pit_boss_combat |
| commissar.yaml | ganger_combat | commissar_combat.yaml | commissar.lua | commissar_combat |
| bridge_troll.yaml | ganger_combat | bridge_troll_combat.yaml | bridge_troll.lua | bridge_troll_combat |

**Important:** The Lua precondition function names must match the method `precondition:` strings in the YAML. Use `<npc_id>_has_enemy`, `<npc_id>_enemy_below_half`, `<npc_id>_not_outnumbered` consistently. **Exception: `scavenger`** — because `scavenger.lua` already defines `scavenger_has_enemy` and `scavenger_not_outnumbered` for the old domain, the new scavenger NPC files must use the `scavenger_npc_` prefix throughout. Below is the complete filled-in example for `scavenger`:

`content/ai/scavenger_combat.yaml`:

```yaml
domain:
  id: scavenger_combat
  description: Combat behavior for scavenger. Cautious; uses ability when not outnumbered.

  tasks:
    - id: behave
      description: Root task — fight cautiously or pass
    - id: cautious_fight
      description: Fight only when not outnumbered

  methods:
    - task: behave
      id: cautious_combat
      precondition: scavenger_npc_has_enemy
      subtasks: [cautious_fight]

    - task: behave
      id: idle
      precondition: ""
      subtasks: [do_pass]

    - task: cautious_fight
      id: fight_with_ability
      precondition: scavenger_npc_not_outnumbered
      subtasks: [attack_enemy, scavenger_confuse]

    - task: cautious_fight
      id: flee_if_outnumbered
      precondition: ""
      subtasks: [do_pass]

  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy

    - id: do_pass
      action: pass
      target: ""

    - id: scavenger_confuse
      action: apply_mental_state
      track: delirium
      severity: mild
      target: nearest_enemy
      cooldown_rounds: 4
      ap_cost: 1
```

`content/scripts/ai/scavenger_npc.lua`:

```lua
-- scavenger_npc.lua: HTN preconditions for scavenger_combat domain.
-- Uses scavenger_npc_ prefix to avoid collision with scavenger.lua.

function scavenger_npc_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function scavenger_npc_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
```

Update `content/npcs/scavenger.yaml`: change `ai_domain: scavenger_patrol` → `ai_domain: scavenger_combat`.

- [ ] **Step 4: Run domain-load test to verify all domains found**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestLoadDomains_AllNPCDomainsValid -v
```

Expected: PASS.

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... -race -count=1 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/ai/ content/scripts/ai/ content/npcs/
git commit -m "feat(npcs): add per-NPC HTN domains with apply_mental_state operators for all 18 NPCs"
```
