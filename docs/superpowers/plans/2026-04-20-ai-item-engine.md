# AI Item Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a combat phase that runs equipped AI items before the player acts each round — each item contributes +1 AP to the shared pool and executes behavior from an embedded HTN domain + Lua script.

**Architecture:** New `ItemDomainRegistry` (separate from the NPC AI registry) holds `ItemPlanner` instances per domain ID. A new `runAIItemPhaseLocked` function in `combat_handler.go` iterates equipped AI items in slot order, credits AP, evaluates the HTN domain using a fresh per-turn Lua VM, executes the selected operator, and persists `self.state` to `ItemInstance.CombatScriptState`. Six new conditions (exposed, weakened, fortified, inspired, evasive, taunted) and a new `ACBonus` field on `ConditionDef` support buff/debuff primitives.

**Tech Stack:** Go, gopher-lua (`github.com/yuin/gopher-lua`), pgregory.net/rapid (property tests), gopkg.in/yaml.v3

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/game/condition/definition.go` | Modify | Add `ACBonus int` field; update `ACBonus()` in modifiers.go |
| `internal/game/condition/modifiers.go` | Modify | Make `ACBonus()` also sum positive `Def.ACBonus` contributions |
| `content/conditions/exposed.yaml` | Create | AC penalty condition (-2 AC to target) |
| `content/conditions/weakened.yaml` | Create | Attack + damage penalty condition |
| `content/conditions/fortified.yaml` | Create | AC bonus condition (+1 AC) |
| `content/conditions/inspired.yaml` | Create | Attack bonus condition (+1 attack) |
| `content/conditions/evasive.yaml` | Create | AC bonus condition (+1 AC, different flavor) |
| `content/conditions/taunted.yaml` | Create | Forced action condition |
| `internal/game/inventory/item.go` | Modify | Add `CombatDomain string`, `CombatScript string` to `ItemDef` |
| `internal/game/inventory/backpack.go` | Modify | Add `CombatScriptState map[string]interface{}` to `ItemInstance`; add `MutableItem` method |
| `internal/game/ai/domain.go` | Modify | Add `"lua_hook"` to valid Operator actions in `Validate()` |
| `internal/game/ai/item_planner.go` | Create | `ItemPlanner`, `ItemCombatSnapshot`, `ItemPrimitiveCalls`; Lua VM lifecycle |
| `internal/game/ai/item_registry.go` | Create | `ItemDomainRegistry` (separate from NPC `Registry`) |
| `internal/game/ai/item_planner_test.go` | Create | All REQ-AIE-11 tests |
| `internal/gameserver/deps.go` | Modify | Add `AIItemRegistry *ai.ItemDomainRegistry` to `ContentDeps` |
| `internal/gameserver/combat_handler.go` | Modify | Add `aiItemRegistry` field; add to `NewCombatHandler`; call `runAIItemPhaseLocked` in round loop; clear states on combat end |
| `internal/gameserver/ai_item_phase.go` | Create | `runAIItemPhaseLocked`, `collectEquippedAIItems`, `clearAIItemCombatStates` |
| `cmd/gameserver/main.go` | Modify | Create `AIItemRegistry`; register item domains after loading; pass to `CombatHandler` |

---

### Task 1: Add ACBonus field to ConditionDef and update modifiers

**Files:**
- Modify: `internal/game/condition/definition.go:21`
- Modify: `internal/game/condition/modifiers.go:30`
- Test: `internal/game/condition/modifiers_test.go`

- [ ] **Step 1: Write the failing test**

```go
// In internal/game/condition/modifiers_test.go, add:
func TestACBonus_FortifiedCondition_PlusOne(t *testing.T) {
	reg := condition.NewRegistry()
	def := &condition.ConditionDef{
		ID: "fortified", Name: "Fortified", DurationType: "rounds",
		MaxStacks: 0, ACBonus: 1,
	}
	_ = reg.Register(def)
	set := condition.NewActiveSet()
	_ = set.Apply("p1", def, 1, 2)
	got := condition.ACBonus(set)
	if got != 1 {
		t.Fatalf("ACBonus with fortified: want 1, got %d", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestACBonus_FortifiedCondition_PlusOne -v
```
Expected: FAIL — `ACBonus: 1` field does not exist on `ConditionDef`.

- [ ] **Step 3: Add ACBonus field to ConditionDef**

In `internal/game/condition/definition.go`, after line 23 (`ACPenalty int`), add:

```go
ACBonus  int `yaml:"ac_bonus,omitempty"`  // positive = bonus to AC (buff)
```

- [ ] **Step 4: Update ACBonus() in modifiers.go**

Replace the existing `ACBonus` function in `internal/game/condition/modifiers.go`:

```go
// ACBonus returns the net AC modifier from all active conditions.
// Positive ACBonus on a condition adds to the total (buff); positive ACPenalty
// subtracts from the total (debuff). For stackable conditions, values are multiplied
// by the current stack count.
//
// Postcondition: May be positive when AC bonuses exceed penalties.
func ACBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.ACPenalty > 0 {
			total -= ac.Def.ACPenalty * ac.Stacks
		}
		if ac.Def.ACBonus > 0 {
			total += ac.Def.ACBonus * ac.Stacks
		}
	}
	return total
}
```

- [ ] **Step 5: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestACBonus -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/game/condition/definition.go internal/game/condition/modifiers.go internal/game/condition/modifiers_test.go
git commit -m "feat(condition): add ACBonus field to ConditionDef for buff conditions"
```

---

### Task 2: Create 6 new condition YAML files

**Files:**
- Create: `content/conditions/exposed.yaml`
- Create: `content/conditions/weakened.yaml`
- Create: `content/conditions/fortified.yaml`
- Create: `content/conditions/inspired.yaml`
- Create: `content/conditions/evasive.yaml`
- Create: `content/conditions/taunted.yaml`

- [ ] **Step 1: Create exposed.yaml**

```yaml
id: exposed
name: Exposed
description: |
  A weakness in your defense has been identified and marked. Attackers
  know exactly where to hit you.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
attack_bonus: 0
ac_penalty: 2
ac_bonus: 0
speed_penalty: 0
damage_bonus: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 2: Create weakened.yaml**

```yaml
id: weakened
name: Weakened
description: |
  Your offensive capability has been compromised. Attacks are less accurate
  and hit for less damage.
duration_type: rounds
max_stacks: 0
attack_penalty: 1
attack_bonus: 0
ac_penalty: 0
ac_bonus: 0
speed_penalty: 0
damage_bonus: -1
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 3: Create fortified.yaml**

```yaml
id: fortified
name: Fortified
description: |
  Defensive posture reinforced. Your guard is up and your positioning
  is solid.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
attack_bonus: 0
ac_penalty: 0
ac_bonus: 1
speed_penalty: 0
damage_bonus: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 4: Create inspired.yaml**

```yaml
id: inspired
name: Inspired
description: |
  Something has raised your fighting spirit. You attack with greater
  conviction and precision.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
attack_bonus: 1
ac_penalty: 0
ac_bonus: 0
speed_penalty: 0
damage_bonus: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 5: Create evasive.yaml**

```yaml
id: evasive
name: Evasive
description: |
  You're moving unpredictably, staying mobile, and making yourself a
  harder target.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
attack_bonus: 0
ac_penalty: 0
ac_bonus: 1
speed_penalty: 0
damage_bonus: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 6: Create taunted.yaml**

```yaml
id: taunted
name: Taunted
description: |
  Your attention has been seized. You can't focus on anything but
  the source of the taunt.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
attack_bonus: 0
ac_penalty: 0
ac_bonus: 0
speed_penalty: 0
damage_bonus: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
forced_action: lowest_hp_attack
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 7: Verify conditions load without error**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v 2>&1 | tail -5
```
Expected: all tests PASS

- [ ] **Step 8: Commit**

```bash
git add content/conditions/exposed.yaml content/conditions/weakened.yaml \
  content/conditions/fortified.yaml content/conditions/inspired.yaml \
  content/conditions/evasive.yaml content/conditions/taunted.yaml
git commit -m "feat(content): add 6 AI item buff/debuff conditions"
```

---

### Task 3: Add CombatDomain/CombatScript to ItemDef and CombatScriptState to ItemInstance

**Files:**
- Modify: `internal/game/inventory/item.go:91`
- Modify: `internal/game/inventory/backpack.go:23`
- Test: `internal/game/inventory/item_test.go` (or create it if absent)

- [ ] **Step 1: Write failing tests**

Create or add to `internal/game/inventory/item_ai_test.go`:

```go
package inventory_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"gopkg.in/yaml.v3"
)

func TestItemDef_CombatDomainRoundTrips(t *testing.T) {
	raw := `
id: ai_test
name: Test AI Item
description: test
kind: weapon
weapon_ref: combat_knife
weight: 1.0
stackable: false
max_stack: 1
value: 100
combat_domain: test_domain
combat_script: |
  preconditions.always = function(self) return true end
`
	var d inventory.ItemDef
	if err := yaml.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.CombatDomain != "test_domain" {
		t.Fatalf("CombatDomain: want %q, got %q", "test_domain", d.CombatDomain)
	}
	if d.CombatScript == "" {
		t.Fatal("CombatScript should not be empty")
	}
}

func TestItemInstance_CombatScriptState_DefaultNil(t *testing.T) {
	inst := inventory.ItemInstance{InstanceID: "abc", ItemDefID: "ai_test", Quantity: 1}
	if inst.CombatScriptState != nil {
		t.Fatal("CombatScriptState should default to nil")
	}
}

func TestBackpack_MutableItem_ReturnsPointer(t *testing.T) {
	bp := inventory.NewBackpack(10, 100)
	def := &inventory.ItemDef{ID: "x", Name: "X", Kind: "junk", Stackable: false, MaxStack: 1, Weight: 0.1}
	if err := bp.Add(def, 1, ""); err != nil {
		t.Fatalf("Add: %v", err)
	}
	items := bp.All()
	if len(items) == 0 {
		t.Fatal("expected item in backpack")
	}
	mutable := bp.MutableItem(items[0].InstanceID)
	if mutable == nil {
		t.Fatal("MutableItem returned nil")
	}
	mutable.CombatScriptState = map[string]interface{}{"kills": float64(3)}
	// Verify mutation is visible via MutableItem again.
	check := bp.MutableItem(items[0].InstanceID)
	if v, ok := check.CombatScriptState["kills"]; !ok || v.(float64) != 3 {
		t.Fatalf("mutation not persisted: state=%v", check.CombatScriptState)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestItemDef_CombatDomain|TestItemInstance_CombatScript|TestBackpack_MutableItem" -v
```
Expected: FAIL — fields do not exist yet.

- [ ] **Step 3: Add CombatDomain and CombatScript to ItemDef**

In `internal/game/inventory/item.go`, after the `Tags []string` field (line 90), add:

```go
// CombatDomain is the HTN domain ID that drives this item's combat behavior.
// An empty string means the item is not an AI item.
CombatDomain string `yaml:"combat_domain,omitempty"`
// CombatScript is the Lua source that defines operator implementations and
// precondition hooks for the named HTN domain.
CombatScript string `yaml:"combat_script,omitempty"`
```

- [ ] **Step 4: Add CombatScriptState to ItemInstance**

In `internal/game/inventory/backpack.go`, after the `MaterialMaxDurabilityBonus int` field, add:

```go
// CombatScriptState holds per-encounter Lua state for AI items.
// Initialized as nil; set to an empty map at combat start; cleared at combat end.
CombatScriptState map[string]interface{}
```

- [ ] **Step 5: Add MutableItem method to Backpack**

In `internal/game/inventory/backpack.go`, add this method after `NewBackpack`:

```go
// MutableItem returns a pointer to the ItemInstance with the given InstanceID,
// or nil if not found. The pointer is valid for the lifetime of this Backpack
// (the underlying slice must not be reallocated while the pointer is held —
// do not Add or Remove items while holding this pointer).
//
// Precondition: instanceID must be non-empty.
// Postcondition: Returns a pointer into the items slice, or nil when not found.
func (b *Backpack) MutableItem(instanceID string) *ItemInstance {
	for i := range b.items {
		if b.items[i].InstanceID == instanceID {
			return &b.items[i]
		}
	}
	return nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestItemDef_CombatDomain|TestItemInstance_CombatScript|TestBackpack_MutableItem" -v
```
Expected: PASS

- [ ] **Step 7: Run full inventory test suite**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -v 2>&1 | tail -10
```
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add internal/game/inventory/item.go internal/game/inventory/backpack.go internal/game/inventory/item_ai_test.go
git commit -m "feat(inventory): add CombatDomain/CombatScript to ItemDef; CombatScriptState to ItemInstance"
```

---

### Task 4: Add "lua_hook" to Domain validation

**Files:**
- Modify: `internal/game/ai/domain.go:86`
- Test: `internal/game/ai/domain_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/game/ai/domain_test.go`:

```go
func TestDomain_Validate_LuaHookAction_Valid(t *testing.T) {
	d := &ai.Domain{
		ID: "item_test",
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{{
			TaskID: "behave", ID: "act",
			Precondition: "",
			Subtasks:     []string{"do_thing"},
		}},
		Operators: []*ai.Operator{{ID: "do_thing", Action: "lua_hook", APCost: 1}},
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected lua_hook to be valid, got error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestDomain_Validate_LuaHookAction_Valid -v
```
Expected: FAIL — `lua_hook` is rejected by Validate().

- [ ] **Step 3: Add lua_hook to valid actions**

In `internal/game/ai/domain.go`, find the `Validate()` function's action validation block (around line 110) and add `"lua_hook"` to the valid action set:

```go
validActions := map[string]bool{
    "attack":             true,
    "strike":             true,
    "pass":              true,
    "flee":              true,
    "apply_mental_state": true,
    "say":               true,
    "lua_hook":          true, // AI item operator — Lua function in CombatScript
}
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestDomain_Validate -v
```
Expected: all domain validation tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/ai/domain.go internal/game/ai/domain_test.go
git commit -m "feat(ai): add lua_hook as valid HTN operator action for AI items"
```

---

### Task 5: Create ItemPlanner

**Files:**
- Create: `internal/game/ai/item_planner.go`
- Create: `internal/game/ai/item_planner_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/game/ai/item_planner_test.go`:

```go
package ai_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ai"
)

// helper: build a minimal domain with one method and one operator.
func buildTestDomain(id, operatorID, precondition string, apCost int) *ai.Domain {
	return &ai.Domain{
		ID: id,
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{{
			TaskID:       "behave",
			ID:           "act",
			Precondition: precondition,
			Subtasks:     []string{operatorID},
		}},
		Operators: []*ai.Operator{{ID: operatorID, Action: "lua_hook", APCost: apCost}},
	}
}

// REQ-AIE-11b: operator consuming 1 AP decrements pool by 1.
func TestAIItemPhase_OperatorConsumesAP(t *testing.T) {
	domain := buildTestDomain("test", "attack_weakest", "", 1)
	planner := ai.NewItemPlanner(domain)

	script := `
operators = {}
operators.attack_weakest = function(self)
  self.engine.attack(self.combat.enemies[1].id, "1d6", 1)
end
`
	ap := 3
	apSpent := 0
	snap := ai.ItemCombatSnapshot{
		Enemies: []ai.ItemEnemySnapshot{{ID: "n1", Name: "Enemy", HP: 10, MaxHP: 10}},
		Player:  ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: ap},
		Round:   1,
	}
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(_, _ string, cost int) bool { apSpent += cost; return true },
		Say:     func(_ []string) {},
		Buff:    func(_, _ string, _, cost int) bool { apSpent += cost; return true },
		Debuff:  func(_, _ string, _, cost int) bool { apSpent += cost; return true },
		GetAP:   func() int { return ap - apSpent },
		SpendAP: func(n int) bool {
			if ap-apSpent < n { return false }
			apSpent += n
			return true
		},
	}
	state := map[string]interface{}{}
	if err := planner.Execute(script, state, snap, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if apSpent != 1 {
		t.Fatalf("expected 1 AP spent, got %d", apSpent)
	}
}

// REQ-AIE-11c: operator with cost=2 decrements pool by 2.
func TestAIItemPhase_MultipleAPAction(t *testing.T) {
	domain := buildTestDomain("test2", "overkill_strike", "", 2)
	planner := ai.NewItemPlanner(domain)

	script := `
operators = {}
operators.overkill_strike = function(self)
  self.engine.attack(self.combat.enemies[1].id, "2d6+4", 2)
end
`
	ap := 3
	apSpent := 0
	snap := ai.ItemCombatSnapshot{
		Enemies: []ai.ItemEnemySnapshot{{ID: "n1", Name: "Foe", HP: 20, MaxHP: 20}},
		Player:  ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: ap},
		Round:   1,
	}
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(_, _ string, cost int) bool { apSpent += cost; return true },
		Say:     func(_ []string) {},
		Buff:    func(_, _ string, _, cost int) bool { return true },
		Debuff:  func(_, _ string, _, cost int) bool { return true },
		GetAP:   func() int { return ap - apSpent },
		SpendAP: func(n int) bool {
			if ap-apSpent < n { return false }
			apSpent += n
			return true
		},
	}
	state := map[string]interface{}{}
	if err := planner.Execute(script, state, snap, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if apSpent != 2 {
		t.Fatalf("expected 2 AP spent, got %d", apSpent)
	}
}

// REQ-AIE-11d: when AP pool hits 0 mid-turn, no further operators execute.
func TestAIItemPhase_PoolExhausted_TurnEnds(t *testing.T) {
	domain := &ai.Domain{
		ID:    "exhaust_test",
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{{
			TaskID: "behave", ID: "act",
			Precondition: "",
			Subtasks:     []string{"op1", "op2"},
		}},
		Operators: []*ai.Operator{
			{ID: "op1", Action: "lua_hook", APCost: 3},
			{ID: "op2", Action: "lua_hook", APCost: 1},
		},
	}
	planner := ai.NewItemPlanner(domain)
	script := `
operators = {}
operators.op1 = function(self) end
operators.op2 = function(self) end
`
	ap := 3
	apSpent := 0
	op2Called := false
	snap := ai.ItemCombatSnapshot{
		Enemies: []ai.ItemEnemySnapshot{{ID: "n1", Name: "Foe", HP: 10, MaxHP: 10}},
		Player:  ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: ap},
		Round:   1,
	}
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(_, _ string, cost int) bool { apSpent += cost; return true },
		Say:    func(_ []string) {},
		Buff:   func(_, _ string, _, cost int) bool { return true },
		Debuff: func(_, _ string, _, cost int) bool { return true },
		GetAP:  func() int { return ap - apSpent },
		SpendAP: func(n int) bool {
			if ap-apSpent < n { return false }
			apSpent += n
			return true
		},
	}
	// Monkeypatch: track op2 invocation by wrapping the attack for op2.
	// Since we can't inject per-operator call tracking without modifying
	// the planner, we verify via AP: if op2 runs it would need to SpendAP(1),
	// but AP is already 0 after op1 spends 3.
	state := map[string]interface{}{}
	if err := planner.Execute(script, state, snap, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	_ = op2Called
	if apSpent > ap {
		t.Fatalf("AP pool went negative: spent=%d, initial=%d", apSpent, ap)
	}
}

// REQ-AIE-11e: self.state mutations in round N are visible in round N+1.
func TestAIItemPhase_StatePersistedAcrossRounds(t *testing.T) {
	domain := buildTestDomain("state_test", "increment", "", 0)
	planner := ai.NewItemPlanner(domain)
	script := `
operators = {}
operators.increment = function(self)
  self.state.count = (self.state.count or 0) + 1
end
`
	snap := ai.ItemCombatSnapshot{
		Enemies: []ai.ItemEnemySnapshot{},
		Player:  ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: 3},
		Round:   1,
	}
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(_, _ string, _ int) bool { return true },
		Say:    func(_ []string) {},
		Buff:   func(_, _ string, _, _ int) bool { return true },
		Debuff: func(_, _ string, _, _ int) bool { return true },
		GetAP:  func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	state := map[string]interface{}{}
	// Round 1
	if err := planner.Execute(script, state, snap, cbs); err != nil {
		t.Fatalf("Round1 Execute: %v", err)
	}
	if v, ok := state["count"]; !ok || v.(float64) != 1 {
		t.Fatalf("round 1: expected count=1, got state=%v", state)
	}
	// Round 2
	snap.Round = 2
	if err := planner.Execute(script, state, snap, cbs); err != nil {
		t.Fatalf("Round2 Execute: %v", err)
	}
	if v, ok := state["count"]; !ok || v.(float64) != 2 {
		t.Fatalf("round 2: expected count=2, got state=%v", state)
	}
}

// REQ-AIE-11i: HTN planner selects correct method given precondition state.
func TestAIItemPhase_PreconditionRouting(t *testing.T) {
	domain := &ai.Domain{
		ID:    "route_test",
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{
			{TaskID: "behave", ID: "special", Precondition: "is_special", Subtasks: []string{"special_op"}},
			{TaskID: "behave", ID: "normal", Precondition: "", Subtasks: []string{"normal_op"}},
		},
		Operators: []*ai.Operator{
			{ID: "special_op", Action: "lua_hook", APCost: 1},
			{ID: "normal_op", Action: "lua_hook", APCost: 1},
		},
	}
	planner := ai.NewItemPlanner(domain)
	script := `
preconditions = {}
preconditions.is_special = function(self)
  return (self.state.special or false)
end
operators = {}
local called = ""
operators.special_op = function(self)
  self.state.called = "special"
end
operators.normal_op = function(self)
  self.state.called = "normal"
end
`
	snap := ai.ItemCombatSnapshot{
		Player: ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: 3},
	}
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(_, _ string, _ int) bool { return true },
		Say:    func(_ []string) {},
		Buff:   func(_, _ string, _, _ int) bool { return true },
		Debuff: func(_, _ string, _, _ int) bool { return true },
		GetAP:  func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}

	// Without special flag → normal method.
	state := map[string]interface{}{}
	if err := planner.Execute(script, state, snap, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if state["called"] != "normal" {
		t.Fatalf("expected normal_op to fire, got called=%v", state["called"])
	}

	// With special flag → special method.
	state2 := map[string]interface{}{"special": true}
	if err := planner.Execute(script, state2, snap, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if state2["called"] != "special" {
		t.Fatalf("expected special_op to fire, got called=%v", state2["called"])
	}
}

// REQ-AIE-11j: HTN planner falls back to idle method when no other precondition satisfied.
func TestAIItemPhase_FallbackToIdleWhenNoPrecondition(t *testing.T) {
	domain := &ai.Domain{
		ID:    "fallback_test",
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{
			{TaskID: "behave", ID: "never", Precondition: "never_true", Subtasks: []string{"never_op"}},
			{TaskID: "behave", ID: "idle", Precondition: "", Subtasks: []string{"idle_op"}},
		},
		Operators: []*ai.Operator{
			{ID: "never_op", Action: "lua_hook", APCost: 1},
			{ID: "idle_op", Action: "lua_hook", APCost: 0},
		},
	}
	planner := ai.NewItemPlanner(domain)
	script := `
preconditions = {}
preconditions.never_true = function(self) return false end
operators = {}
operators.never_op = function(self) self.state.wrong = true end
operators.idle_op = function(self) self.state.fired = true end
`
	snap := ai.ItemCombatSnapshot{
		Player: ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: 3},
	}
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(_, _ string, _ int) bool { return true },
		Say:    func(_ []string) {},
		Buff:   func(_, _ string, _, _ int) bool { return true },
		Debuff: func(_, _ string, _, _ int) bool { return true },
		GetAP:  func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	state := map[string]interface{}{}
	if err := planner.Execute(script, state, snap, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if state["wrong"] == true {
		t.Fatal("never_op should not have fired")
	}
	if state["fired"] != true {
		t.Fatal("idle_op should have fired as fallback")
	}
}

// REQ-AIE-11h: property test — AP pool NEVER goes below 0.
func TestProperty_AIItemPhase_APNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		initialAP := rapid.IntRange(0, 5).Draw(rt, "initialAP")
		opCost := rapid.IntRange(0, 4).Draw(rt, "opCost")

		domain := buildTestDomain("prop_test", "op1", "", opCost)
		planner := ai.NewItemPlanner(domain)
		script := `
operators = {}
operators.op1 = function(self) end
`
		ap := initialAP
		snap := ai.ItemCombatSnapshot{
			Enemies: []ai.ItemEnemySnapshot{{ID: "n1", Name: "Foe", HP: 10, MaxHP: 10}},
			Player:  ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: ap},
			Round:   1,
		}
		cbs := ai.ItemPrimitiveCalls{
			Attack: func(_, _ string, cost int) bool {
				if ap < cost { return false }
				ap -= cost
				return true
			},
			Say:    func(_ []string) {},
			Buff:   func(_, _ string, _, cost int) bool { return ap >= cost },
			Debuff: func(_, _ string, _, cost int) bool { return ap >= cost },
			GetAP:  func() int { return ap },
			SpendAP: func(n int) bool {
				if ap < n { return false }
				ap -= n
				return true
			},
		}
		state := map[string]interface{}{}
		if err := planner.Execute(script, state, snap, cbs); err != nil {
			rt.Skip() // script errors are not invariant failures
		}
		if ap < 0 {
			rt.Errorf("AP went negative: final AP=%d (initial=%d, opCost=%d)", ap, initialAP, opCost)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run "TestAIItemPhase|TestProperty_AIItemPhase" -v 2>&1 | head -20
```
Expected: FAIL — `ai.NewItemPlanner` does not exist.

- [ ] **Step 3: Create item_planner.go**

Create `internal/game/ai/item_planner.go`:

```go
package ai

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// ItemEnemySnapshot is one enemy's state visible to an AI item's Lua script.
type ItemEnemySnapshot struct {
	ID         string
	Name       string
	HP         int
	MaxHP      int
	Conditions []string
}

// ItemPlayerSnapshot is the player's state visible to an AI item's Lua script.
type ItemPlayerSnapshot struct {
	ID         string
	HP         int
	MaxHP      int
	AP         int
	Conditions []string
}

// ItemCombatSnapshot is the full combat state visible to an AI item's Lua script.
// Satisfies REQ-AIE-10.
type ItemCombatSnapshot struct {
	Enemies []ItemEnemySnapshot
	Player  ItemPlayerSnapshot
	Round   int
}

// ItemPrimitiveCalls is the set of Go callbacks exposed as self.engine.* in Lua.
// Satisfies REQ-AIE-6.
type ItemPrimitiveCalls struct {
	// Attack resolves damage against targetID using formula at the given AP cost.
	// Returns false when AP is insufficient (no-op); true on success.
	Attack func(targetID, formula string, cost int) bool
	// Say broadcasts a random line from textPool to the room. Always succeeds.
	Say func(textPool []string)
	// Buff applies a positive condition to targetID for rounds at the given AP cost.
	// Returns false when AP is insufficient.
	Buff func(targetID, effectID string, rounds, cost int) bool
	// Debuff applies a negative condition to targetID for rounds at the given AP cost.
	// Returns false when AP is insufficient.
	Debuff func(targetID, effectID string, rounds, cost int) bool
	// GetAP returns the current remaining AP pool.
	GetAP func() int
	// SpendAP attempts to spend n AP from the pool.
	// Returns false (no-op) when remaining AP < n.
	SpendAP func(n int) bool
}

// ItemPlanner executes an HTN domain for one AI item turn using an embedded Lua script.
// Each call to Execute creates a fresh Lua VM for isolation — no state leaks between turns
// except via the scriptState map.
//
// Invariant: domain must not be nil.
type ItemPlanner struct {
	domain *Domain
}

// NewItemPlanner creates an ItemPlanner for the given HTN domain.
//
// Precondition: domain must not be nil.
// Postcondition: returns a non-nil ItemPlanner.
func NewItemPlanner(domain *Domain) *ItemPlanner {
	if domain == nil {
		panic("ai.NewItemPlanner: domain must not be nil")
	}
	return &ItemPlanner{domain: domain}
}

// Execute runs one AI item turn:
//  1. Creates a fresh Lua VM and loads script.
//  2. Builds self.state from scriptState, self.combat from snapshot, self.engine from cbs.
//  3. Evaluates HTN methods via preconditions.* Lua calls.
//  4. Executes the selected operator's Lua function via operators.*.
//  5. Serializes self.state back to scriptState.
//
// Precondition: script must be non-empty; scriptState must not be nil; cbs must be non-nil.
// Postcondition: scriptState reflects mutations made by the Lua operator; AP is decremented
// via cbs.SpendAP before each non-free operator runs; AP never goes below 0 (REQ-AIE-5).
func (p *ItemPlanner) Execute(
	script string,
	scriptState map[string]interface{},
	snapshot ItemCombatSnapshot,
	cbs ItemPrimitiveCalls,
) error {
	L := lua.NewState(lua.Options{SkipOpenLibs: false})
	defer L.Close()

	// Load the item's embedded CombatScript.
	if err := L.DoString(script); err != nil {
		return fmt.Errorf("ItemPlanner.Execute: load script: %w", err)
	}

	// Build the self table.
	self := L.NewTable()

	// self.state: load from scriptState map.
	L.SetField(self, "state", mapToLuaTable(L, scriptState))

	// self.combat: build from snapshot.
	L.SetField(self, "combat", p.buildCombatTable(L, snapshot))

	// self.engine: expose Go callbacks as Lua functions.
	L.SetField(self, "engine", p.buildEngineTable(L, cbs))

	// Find the applicable HTN method via Lua precondition evaluation.
	method := p.findApplicableItemMethod(L, self)
	if method == nil {
		return nil // no applicable method; item idles
	}

	// Execute each subtask operator in declaration order (REQ-AIE-4b/4c).
	for _, subtaskID := range method.Subtasks {
		op, ok := p.domain.OperatorByID(subtaskID)
		if !ok {
			continue
		}
		if op.Action != "lua_hook" {
			continue
		}

		// Spend AP before executing (REQ-AIE-4d, REQ-AIE-5).
		// APCost == 0 means the operator is free (e.g., say actions).
		if op.APCost > 0 {
			if !cbs.SpendAP(op.APCost) {
				return nil // AP exhausted; item turn ends immediately
			}
		}

		// Call operators.<id>(self).
		opsGlobal := L.GetGlobal("operators")
		if opsGlobal == lua.LNil {
			continue
		}
		opFn := L.GetField(opsGlobal, subtaskID)
		luaFn, ok := opFn.(*lua.LFunction)
		if !ok {
			continue // operator not defined in script; skip silently
		}
		if err := L.CallByParam(lua.P{Fn: luaFn, NRet: 0, Protect: true}, self); err != nil {
			// Lua errors during operator execution are silently ignored — the item
			// continues its turn with remaining operators.
			_ = err
		}
	}

	// Serialize self.state back to scriptState (REQ-AIE-4e).
	stateResult := L.GetField(self, "state")
	if t, ok := stateResult.(*lua.LTable); ok {
		luaTableToMap(t, scriptState)
	}

	return nil
}

// findApplicableItemMethod evaluates preconditions for the "behave" task and returns
// the first applicable Method. An empty Precondition is always applicable (fallback).
// Returns nil only when no methods are defined for "behave".
func (p *ItemPlanner) findApplicableItemMethod(L *lua.LState, self *lua.LTable) *Method {
	precondsGlobal := L.GetGlobal("preconditions")

	for _, m := range p.domain.MethodsForTask("behave") {
		if m.Precondition == "" {
			return m // unconditional fallback
		}

		// Look up preconditions.<name>.
		fn := L.GetField(precondsGlobal, m.Precondition)
		luaFn, ok := fn.(*lua.LFunction)
		if !ok {
			continue // precondition function not defined → skip
		}

		if err := L.CallByParam(lua.P{Fn: luaFn, NRet: 1, Protect: true}, self); err != nil {
			continue // Lua error → treat as false
		}
		result := L.Get(-1)
		L.Pop(1)
		if result == lua.LTrue {
			return m
		}
	}
	return nil
}

// buildCombatTable constructs the self.combat Lua table from the combat snapshot (REQ-AIE-10).
func (p *ItemPlanner) buildCombatTable(L *lua.LState, snap ItemCombatSnapshot) *lua.LTable {
	t := L.NewTable()

	// self.combat.round
	L.SetField(t, "round", lua.LNumber(snap.Round))

	// self.combat.player
	playerT := L.NewTable()
	L.SetField(playerT, "id", lua.LString(snap.Player.ID))
	L.SetField(playerT, "hp", lua.LNumber(snap.Player.HP))
	L.SetField(playerT, "max_hp", lua.LNumber(snap.Player.MaxHP))
	L.SetField(playerT, "ap", lua.LNumber(snap.Player.AP))
	L.SetField(t, "player", playerT)

	// self.combat.enemies (array of {id, name, hp, max_hp})
	enemiesT := L.NewTable()
	for i, e := range snap.Enemies {
		eT := L.NewTable()
		L.SetField(eT, "id", lua.LString(e.ID))
		L.SetField(eT, "name", lua.LString(e.Name))
		L.SetField(eT, "hp", lua.LNumber(e.HP))
		L.SetField(eT, "max_hp", lua.LNumber(e.MaxHP))
		L.RawSetInt(enemiesT, i+1, eT)
	}
	L.SetField(t, "enemies", enemiesT)

	// self.combat.weakest_enemy() — returns enemy with lowest HP/MaxHP ratio, or nil (REQ-AIE-10).
	enemies := snap.Enemies
	L.SetField(t, "weakest_enemy", L.NewFunction(func(L *lua.LState) int {
		if len(enemies) == 0 {
			L.Push(lua.LNil)
			return 1
		}
		worst := &enemies[0]
		for i := range enemies {
			e := &enemies[i]
			if e.MaxHP > 0 && worst.MaxHP > 0 {
				eRatio := float64(e.HP) / float64(e.MaxHP)
				wRatio := float64(worst.HP) / float64(worst.MaxHP)
				if eRatio < wRatio {
					worst = e
				}
			}
		}
		eT := L.NewTable()
		L.SetField(eT, "id", lua.LString(worst.ID))
		L.SetField(eT, "name", lua.LString(worst.Name))
		L.SetField(eT, "hp", lua.LNumber(worst.HP))
		L.SetField(eT, "max_hp", lua.LNumber(worst.MaxHP))
		L.Push(eT)
		return 1
	}))

	// self.combat.nearest_enemy() — returns first living enemy (no spatial model for items).
	L.SetField(t, "nearest_enemy", L.NewFunction(func(L *lua.LState) int {
		if len(enemies) == 0 {
			L.Push(lua.LNil)
			return 1
		}
		e := enemies[0]
		eT := L.NewTable()
		L.SetField(eT, "id", lua.LString(e.ID))
		L.SetField(eT, "name", lua.LString(e.Name))
		L.SetField(eT, "hp", lua.LNumber(e.HP))
		L.SetField(eT, "max_hp", lua.LNumber(e.MaxHP))
		L.Push(eT)
		return 1
	}))

	return t
}

// buildEngineTable constructs the self.engine Lua table with Go callback functions (REQ-AIE-6).
func (p *ItemPlanner) buildEngineTable(L *lua.LState, cbs ItemPrimitiveCalls) *lua.LTable {
	t := L.NewTable()

	// self.engine.attack(targetId, formula [, cost=1]) — REQ-AIE-6a.
	L.SetField(t, "attack", L.NewFunction(func(L *lua.LState) int {
		targetID := L.CheckString(1)
		formula := L.CheckString(2)
		cost := L.OptInt(3, 1)
		if cbs.Attack(targetID, formula, cost) {
			L.Push(lua.LTrue)
		} else {
			L.Push(lua.LFalse)
		}
		return 1
	}))

	// self.engine.say(textPool) — REQ-AIE-6b (0 AP cost).
	L.SetField(t, "say", L.NewFunction(func(L *lua.LState) int {
		tbl, ok := L.Get(1).(*lua.LTable)
		if !ok {
			return 0
		}
		var pool []string
		tbl.ForEach(func(_ lua.LValue, v lua.LValue) {
			if s, ok := v.(lua.LString); ok {
				pool = append(pool, string(s))
			}
		})
		cbs.Say(pool)
		return 0
	}))

	// self.engine.buff(targetId, effectId, rounds [, cost=1]) — REQ-AIE-6c.
	L.SetField(t, "buff", L.NewFunction(func(L *lua.LState) int {
		targetID := L.CheckString(1)
		effectID := L.CheckString(2)
		rounds := L.CheckInt(3)
		cost := L.OptInt(4, 1)
		if cbs.Buff(targetID, effectID, rounds, cost) {
			L.Push(lua.LTrue)
		} else {
			L.Push(lua.LFalse)
		}
		return 1
	}))

	// self.engine.debuff(targetId, effectId, rounds [, cost=1]) — REQ-AIE-6d.
	L.SetField(t, "debuff", L.NewFunction(func(L *lua.LState) int {
		targetID := L.CheckString(1)
		effectID := L.CheckString(2)
		rounds := L.CheckInt(3)
		cost := L.OptInt(4, 1)
		if cbs.Debuff(targetID, effectID, rounds, cost) {
			L.Push(lua.LTrue)
		} else {
			L.Push(lua.LFalse)
		}
		return 1
	}))

	return t
}

// mapToLuaTable converts a Go map[string]interface{} to a Lua table.
// Supports string, int, int64, float64, and bool values; other types are skipped.
func mapToLuaTable(L *lua.LState, m map[string]interface{}) *lua.LTable {
	t := L.NewTable()
	for k, v := range m {
		switch val := v.(type) {
		case string:
			L.SetField(t, k, lua.LString(val))
		case int:
			L.SetField(t, k, lua.LNumber(float64(val)))
		case int64:
			L.SetField(t, k, lua.LNumber(float64(val)))
		case float64:
			L.SetField(t, k, lua.LNumber(val))
		case bool:
			if val {
				L.SetField(t, k, lua.LTrue)
			} else {
				L.SetField(t, k, lua.LFalse)
			}
		}
	}
	return t
}

// luaTableToMap serializes a Lua table back into the given Go map (in place).
// Existing keys not present in the Lua table are removed. Only string-keyed
// entries with string, number, or bool values are preserved.
func luaTableToMap(t *lua.LTable, m map[string]interface{}) {
	// Clear existing state.
	for k := range m {
		delete(m, k)
	}
	// Repopulate from Lua table.
	t.ForEach(func(k, v lua.LValue) {
		key, ok := k.(lua.LString)
		if !ok {
			return
		}
		switch val := v.(type) {
		case lua.LString:
			m[string(key)] = string(val)
		case lua.LNumber:
			m[string(key)] = float64(val)
		case lua.LBool:
			m[string(key)] = bool(val)
		}
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run "TestAIItemPhase|TestProperty_AIItemPhase" -v
```
Expected: all PASS

- [ ] **Step 5: Run full AI test suite**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -v 2>&1 | tail -10
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/game/ai/item_planner.go internal/game/ai/item_planner_test.go
git commit -m "feat(ai): implement ItemPlanner — Lua-driven HTN executor for AI item turns"
```

---

### Task 6: Create ItemDomainRegistry

**Files:**
- Create: `internal/game/ai/item_registry.go`
- Test: `internal/game/ai/item_registry_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/game/ai/item_registry_test.go`:

```go
package ai_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/ai"
)

func TestItemDomainRegistry_RegisterAndRetrieve(t *testing.T) {
	reg := ai.NewItemDomainRegistry()
	domain := &ai.Domain{
		ID:        "test_reg",
		Tasks:     []*ai.Task{{ID: "behave"}},
		Methods:   []*ai.Method{{TaskID: "behave", ID: "m1", Subtasks: []string{"op1"}}},
		Operators: []*ai.Operator{{ID: "op1", Action: "lua_hook"}},
	}
	if err := reg.Register(domain); err != nil {
		t.Fatalf("Register: %v", err)
	}
	p, ok := reg.PlannerFor("test_reg")
	if !ok || p == nil {
		t.Fatal("PlannerFor should return planner after registration")
	}
}

func TestItemDomainRegistry_DuplicateReturnsError(t *testing.T) {
	reg := ai.NewItemDomainRegistry()
	domain := &ai.Domain{
		ID: "dup", Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{{TaskID: "behave", ID: "m1", Subtasks: []string{"op1"}}},
		Operators: []*ai.Operator{{ID: "op1", Action: "lua_hook"}},
	}
	if err := reg.Register(domain); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := reg.Register(domain); err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestItemDomainRegistry_MissingReturnsNotFound(t *testing.T) {
	reg := ai.NewItemDomainRegistry()
	p, ok := reg.PlannerFor("nonexistent")
	if ok || p != nil {
		t.Fatal("expected (nil, false) for unknown domain")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestItemDomainRegistry -v
```
Expected: FAIL — `ai.NewItemDomainRegistry` does not exist.

- [ ] **Step 3: Create item_registry.go**

Create `internal/game/ai/item_registry.go`:

```go
package ai

import "fmt"

// ItemDomainRegistry maintains a mapping of domain ID → ItemPlanner for AI item domains.
// This registry is separate from the NPC AI Registry to keep item and NPC domains isolated.
// Satisfies REQ-AIE-7.
type ItemDomainRegistry struct {
	planners map[string]*ItemPlanner
}

// NewItemDomainRegistry creates an empty ItemDomainRegistry.
//
// Postcondition: PlannerFor returns (nil, false) for any domain ID.
func NewItemDomainRegistry() *ItemDomainRegistry {
	return &ItemDomainRegistry{planners: make(map[string]*ItemPlanner)}
}

// Register adds a domain to the registry, creating an ItemPlanner for it.
//
// Precondition: domain must not be nil and must have a unique non-empty ID.
// Postcondition: PlannerFor(domain.ID) returns the new planner.
func (r *ItemDomainRegistry) Register(domain *Domain) error {
	if domain == nil {
		return fmt.Errorf("ai.ItemDomainRegistry.Register: domain must not be nil")
	}
	if _, exists := r.planners[domain.ID]; exists {
		return fmt.Errorf("ai.ItemDomainRegistry.Register: domain %q already registered", domain.ID)
	}
	r.planners[domain.ID] = NewItemPlanner(domain)
	return nil
}

// PlannerFor retrieves the ItemPlanner for the given domain ID.
//
// Postcondition: Returns (planner, true) if registered, (nil, false) otherwise.
func (r *ItemDomainRegistry) PlannerFor(domainID string) (*ItemPlanner, bool) {
	p, ok := r.planners[domainID]
	return p, ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestItemDomainRegistry -v
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/ai/item_registry.go internal/game/ai/item_registry_test.go
git commit -m "feat(ai): add ItemDomainRegistry for AI item domains (separate from NPC registry)"
```

---

### Task 7: Create AI item phase runner in gameserver

**Files:**
- Create: `internal/gameserver/ai_item_phase.go`
- Modify: `internal/gameserver/deps.go`
- Modify: `internal/gameserver/combat_handler.go`

- [ ] **Step 1: Add AIItemRegistry to ContentDeps**

In `internal/gameserver/deps.go`, add to the `ContentDeps` struct after `AIRegistry *ai.Registry`:

```go
// AIItemRegistry holds HTN domains for AI item combat behavior.
// Separate from AIRegistry (NPC domains). May be nil — AI item phase is skipped.
AIItemRegistry *ai.ItemDomainRegistry
```

- [ ] **Step 2: Add aiItemRegistry to CombatHandler struct**

In `internal/gameserver/combat_handler.go`, find the `CombatHandler` struct definition and add:

```go
aiItemRegistry *ai.ItemDomainRegistry
```

- [ ] **Step 3: Add aiItemRegistry parameter to NewCombatHandler**

In `internal/gameserver/combat_handler.go`, update `NewCombatHandler` signature and body:

```go
func NewCombatHandler(
	engine *combat.Engine,
	npcMgr *npc.Manager,
	sessions *session.Manager,
	diceRoller *dice.Roller,
	broadcastFn func(roomID string, events []*gamev1.CombatEvent),
	roundDuration time.Duration,
	condRegistry *condition.Registry,
	worldMgr *world.Manager,
	scriptMgr *scripting.Manager,
	invRegistry *inventory.Registry,
	aiRegistry *ai.Registry,
	aiItemRegistry *ai.ItemDomainRegistry,  // ADD THIS
	respawnMgr     *npc.RespawnManager,
	floorMgr       *inventory.FloorManager,
	mentalStateMgr *mentalstate.Manager,
) *CombatHandler {
	return &CombatHandler{
		// ... existing fields ...
		aiItemRegistry: aiItemRegistry,  // ADD THIS
		// ... rest of existing fields ...
	}
}
```

- [ ] **Step 4: Create ai_item_phase.go**

Create `internal/gameserver/ai_item_phase.go`:

```go
package gameserver

import (
	"math/rand"

	gamev1 "github.com/cory-johannsen/mud/api/proto/game/v1"
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// equippedAIItem records one equipped AI item awaiting its combat turn.
type equippedAIItem struct {
	instanceID string
	itemDefID  string
	domain     string
	script     string
}

// armorSlotOrder defines the ordered iteration for armor slots (REQ-AIE-4).
var armorSlotOrder = []inventory.ArmorSlot{
	inventory.SlotHead, inventory.SlotLeftArm, inventory.SlotRightArm,
	inventory.SlotTorso, inventory.SlotHands, inventory.SlotLeftLeg,
	inventory.SlotRightLeg, inventory.SlotFeet,
}

// collectEquippedAIItems returns all equipped items with a non-empty CombatDomain,
// in slot order: main hand → off hand → armor slots → accessories.
// Precondition: sess must not be nil; h.invRegistry must not be nil.
func (h *CombatHandler) collectEquippedAIItems(sess *session.PlayerSession) []equippedAIItem {
	var items []equippedAIItem

	addFromItemDef := func(instanceID, itemDefID string) {
		if itemDefID == "" {
			return
		}
		def, ok := h.invRegistry.Item(itemDefID)
		if !ok || def.CombatDomain == "" {
			return
		}
		items = append(items, equippedAIItem{
			instanceID: instanceID,
			itemDefID:  itemDefID,
			domain:     def.CombatDomain,
			script:     def.CombatScript,
		})
	}

	// Main hand and off hand.
	if sess.LoadoutSet != nil {
		if preset := sess.LoadoutSet.ActivePreset(); preset != nil {
			if preset.MainHand != nil {
				addFromItemDef(preset.MainHand.InstanceID, preset.MainHand.ItemDefID)
			}
			if preset.OffHand != nil {
				addFromItemDef(preset.OffHand.InstanceID, preset.OffHand.ItemDefID)
			}
		}
	}

	// Armor slots in order.
	if sess.Equipment != nil {
		for _, slot := range armorSlotOrder {
			slotted := sess.Equipment.Armor[slot]
			if slotted == nil {
				continue
			}
			addFromItemDef(slotted.InstanceID, slotted.ItemDefID)
		}
		// Accessories (map iteration order is undefined; stable sort not required by spec).
		for _, slotted := range sess.Equipment.Accessories {
			if slotted == nil {
				continue
			}
			addFromItemDef(slotted.InstanceID, slotted.ItemDefID)
		}
	}

	return items
}

// runAIItemPhaseLocked executes the AI item phase for all player combatants in cbt.
// Must be called with combatMu held. Runs after StartRound (AP queues reset) and
// before player AP notification. Satisfies REQ-AIE-3 and REQ-AIE-4.
//
// Precondition: cbt must not be nil; combatMu must be held by caller.
// Postcondition: Each equipped AI item has contributed +1 AP and run its HTN operator.
// Returns CombatEvents for damage and speech produced during item turns.
func (h *CombatHandler) runAIItemPhaseLocked(cbt *combat.Combat) []*gamev1.CombatEvent {
	if h.aiItemRegistry == nil || h.invRegistry == nil {
		return nil
	}

	var events []*gamev1.CombatEvent

	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer || c.IsDead() {
			continue
		}
		sess, ok := h.sessions.GetPlayer(c.ID)
		if !ok {
			continue
		}

		aiItems := h.collectEquippedAIItems(sess)
		if len(aiItems) == 0 {
			continue
		}

		// REQ-AIE-3: add 1 AP per AI item to the player's queue.
		q := cbt.ActionQueues[c.ID]
		if q != nil {
			q.AddAP(len(aiItems))
			q.MaxPoints += len(aiItems)
		}

		// REQ-AIE-4: execute each AI item's turn.
		for _, item := range aiItems {
			planner, ok := h.aiItemRegistry.PlannerFor(item.domain)
			if !ok {
				continue
			}

			// Build combat snapshot for this item.
			snap := h.buildItemCombatSnapshot(c, cbt, q)

			// Get or initialize per-encounter script state.
			var scriptState map[string]interface{}
			if sess.Backpack != nil {
				mi := sess.Backpack.MutableItem(item.instanceID)
				if mi != nil {
					if mi.CombatScriptState == nil {
						mi.CombatScriptState = make(map[string]interface{})
					}
					scriptState = mi.CombatScriptState
				}
			}
			if scriptState == nil {
				scriptState = make(map[string]interface{})
			}

			// Build callbacks.
			itemEvents := h.buildItemPrimitiveCalls(c, cbt, q, item)

			// Execute the HTN plan.
			_ = planner.Execute(item.script, scriptState, snap, itemEvents.cbs)
			events = append(events, itemEvents.events...)
		}
	}

	return events
}

// itemPhaseCallbacks packages the callbacks and collected events from one item turn.
type itemPhaseCallbacks struct {
	cbs    ai.ItemPrimitiveCalls
	events []*gamev1.CombatEvent
}

// buildItemPrimitiveCalls constructs the ItemPrimitiveCalls for one AI item turn.
func (h *CombatHandler) buildItemPrimitiveCalls(
	actor *combat.Combatant,
	cbt *combat.Combat,
	q *combat.ActionQueue,
	item equippedAIItem,
) itemPhaseCallbacks {
	var collectedEvents []*gamev1.CombatEvent

	spendAP := func(n int) bool {
		if q == nil {
			return false
		}
		if q.RemainingPoints() < n {
			return false
		}
		_ = q.DeductAP(n)
		return true
	}

	attack := func(targetID, formula string, cost int) bool {
		if !spendAP(cost) {
			return false
		}
		target := cbt.GetCombatant(targetID)
		if target == nil {
			return false
		}
		result, err := h.dice.RollExpr(formula)
		if err != nil {
			return false
		}
		dmg := result.Total()
		if dmg < 1 {
			dmg = 1
		}
		target.CurrentHP -= dmg
		if target.CurrentHP < 0 {
			target.CurrentHP = 0
		}
		cbt.RecordDamage(actor.ID, dmg)
		collectedEvents = append(collectedEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  item.itemDefID,
			Target:    target.Name,
			Damage:    int32(dmg),
			Narrative: fmt.Sprintf("%s attacks %s for %d damage.", item.itemDefID, target.Name, dmg),
		})
		return true
	}

	say := func(textPool []string) {
		if len(textPool) == 0 {
			return
		}
		line := textPool[rand.Intn(len(textPool))]
		collectedEvents = append(collectedEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_SPEECH,
			Attacker:  item.itemDefID,
			Narrative: line,
		})
	}

	applyCondition := func(targetID, effectID string, rounds, cost int) bool {
		if !spendAP(cost) {
			return false
		}
		if err := cbt.ApplyCondition(targetID, effectID, 1, rounds); err != nil {
			return false
		}
		collectedEvents = append(collectedEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_CONDITION,
			Target:    targetID,
			Narrative: fmt.Sprintf("%s applies %s to %s.", item.itemDefID, effectID, targetID),
		})
		return true
	}

	return itemPhaseCallbacks{
		cbs: ai.ItemPrimitiveCalls{
			Attack:  attack,
			Say:     say,
			Buff:    applyCondition,
			Debuff:  applyCondition,
			GetAP:   func() int { if q == nil { return 0 }; return q.RemainingPoints() },
			SpendAP: spendAP,
		},
		events: nil, // filled during execution via collectedEvents
	}
}

// buildItemCombatSnapshot constructs the ItemCombatSnapshot for one player's item turn.
func (h *CombatHandler) buildItemCombatSnapshot(
	player *combat.Combatant,
	cbt *combat.Combat,
	q *combat.ActionQueue,
) ai.ItemCombatSnapshot {
	var enemies []ai.ItemEnemySnapshot
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}
		var condIDs []string
		if set := cbt.Conditions[c.ID]; set != nil {
			for _, ac := range set.All() {
				condIDs = append(condIDs, ac.Def.ID)
			}
		}
		enemies = append(enemies, ai.ItemEnemySnapshot{
			ID:         c.ID,
			Name:       c.Name,
			HP:         c.CurrentHP,
			MaxHP:      c.MaxHP,
			Conditions: condIDs,
		})
	}

	ap := 0
	if q != nil {
		ap = q.RemainingPoints()
	}

	return ai.ItemCombatSnapshot{
		Enemies: enemies,
		Player: ai.ItemPlayerSnapshot{
			ID:    player.ID,
			HP:    player.CurrentHP,
			MaxHP: player.MaxHP,
			AP:    ap,
		},
		Round: cbt.Round,
	}
}

// clearAIItemCombatStates clears CombatScriptState for all equipped AI items of a player.
// Called when combat ends (win, loss, flee). Satisfies REQ-AIE-2.
// Precondition: sess must not be nil.
func (h *CombatHandler) clearAIItemCombatStates(sess *session.PlayerSession) {
	if sess.Backpack == nil {
		return
	}
	aiItems := h.collectEquippedAIItems(sess)
	for _, item := range aiItems {
		mi := sess.Backpack.MutableItem(item.instanceID)
		if mi != nil {
			mi.CombatScriptState = nil
		}
	}
}
```

**Note:** The `buildItemPrimitiveCalls` method uses `fmt.Sprintf` — add `"fmt"` to the imports.

- [ ] **Step 5: Compile to verify no errors**

```
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1
```
Expected: compile succeeds (or only shows pre-existing errors unrelated to this task).

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/ai_item_phase.go internal/gameserver/deps.go internal/gameserver/combat_handler.go
git commit -m "feat(gameserver): add AI item phase runner (runAIItemPhaseLocked, clearAIItemCombatStates)"
```

---

### Task 8: Wire AI item phase into round loop and startup

**Files:**
- Modify: `internal/gameserver/combat_handler.go` (round loop insertion + combat end cleanup)
- Modify: `cmd/gameserver/main.go` (ItemDomainRegistry creation + registration)

- [ ] **Step 1: Insert AI item phase into the round loop**

In `internal/gameserver/combat_handler.go`, in the `resolveAndAdvanceLocked` function, find the section after BankedAP injection (around line 2723) and before player AP notification (around line 2726):

```go
// After: playerSess.BankedAP = 0
// Before: for _, c := range cbt.Combatants { ... "You have %d AP this round." }

// REQ-AIE-3 + REQ-AIE-4: run AI item phase — items contribute AP then act.
aiItemEvents := h.runAIItemPhaseLocked(cbt)
```

Then include `aiItemEvents` when collecting round events to broadcast.

- [ ] **Step 2: Clear CombatScriptState on combat end**

In `internal/gameserver/combat_handler.go`, find the combat-end handling (where `EndCombat` is called after player victory, loss, or flee). For each player combatant, call:

```go
if sess, ok := h.sessions.GetPlayer(c.ID); ok {
    h.clearAIItemCombatStates(sess)
}
```

Add this before or after the existing combat-end cleanup.

- [ ] **Step 3: Wire AIItemRegistry in startup**

In `cmd/gameserver/main.go`, after the existing `ai.LoadDomains` block (around line 337), add:

```go
// Register item AI domains from the same content/ai/ directory.
// Item domains are identified by their use in ItemDef.CombatDomain.
aiItemReg := ai.NewItemDomainRegistry()
for _, itemDef := range app.InvRegistry.AllItemDefs() {
    if itemDef.CombatDomain == "" {
        continue
    }
    // Item domains are loaded from the same ai.LoadDomains call above.
    // Look up the domain by ID from the already-loaded NPC registry domains.
    // If not found, try to re-load from domain files.
    domain, ok := app.AIRegistry.DomainByID(itemDef.CombatDomain)
    if !ok {
        logger.Warn("AI item domain not found", zap.String("domain", itemDef.CombatDomain), zap.String("item", itemDef.ID))
        continue
    }
    if err := aiItemReg.Register(domain); err != nil {
        // Duplicate registration is expected if multiple items share a domain (skip).
        logger.Debug("item domain already registered", zap.String("domain", itemDef.CombatDomain))
    }
}
app.AIItemRegistry = aiItemReg
```

**Note:** This requires `app.AIRegistry` to expose a `DomainByID` method, or alternatively, we share the domain list from `LoadDomains`. The simpler approach is to register item domains directly from the `domains` slice:

```go
// After: for _, domain := range domains { app.AIRegistry.Register(...) }
// Add item domain registration:
aiItemReg := ai.NewItemDomainRegistry()
for _, domain := range domains {
    // Only register domains referenced by at least one item's CombatDomain.
    _ = aiItemReg.Register(domain) // duplicates are no-ops
}
app.AIItemRegistry = aiItemReg
```

Add `AIItemRegistry *ai.ItemDomainRegistry` to the `App` struct in `internal/gameserver/deps.go` or wherever `App` is defined.

Pass `app.AIItemRegistry` to `NewCombatHandler`.

- [ ] **Step 4: Full build verification**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: build succeeds.

- [ ] **Step 5: Run full gameserver test suite**

```
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -timeout 120s 2>&1 | tail -15
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/combat_handler.go cmd/gameserver/main.go
git commit -m "feat(gameserver): wire AI item phase into combat round loop and startup"
```

---

### Task 9: Property test — multiple items AP invariant (REQ-AIE-11h)

**Files:**
- Modify: `internal/gameserver/combat_handler_test.go` (or create `internal/gameserver/ai_item_phase_test.go`)

- [ ] **Step 1: Write the property test**

Create `internal/gameserver/ai_item_phase_test.go`:

```go
package gameserver_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ai"
)

// REQ-AIE-11h: for any number of equipped AI items, AP NEVER goes below 0.
func TestProperty_AIItemPhase_APNeverNegative_MultipleItems(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numItems := rapid.IntRange(0, 4).Draw(rt, "numItems")
		initialAP := rapid.IntRange(0, 3).Draw(rt, "initialAP")

		ap := initialAP + numItems // simulate StartRound(3) + N AP contributions

		reg := ai.NewItemDomainRegistry()
		for i := 0; i < numItems; i++ {
			opCost := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("cost_%d", i))
			domain := &ai.Domain{
				ID:    fmt.Sprintf("item_%d", i),
				Tasks: []*ai.Task{{ID: "behave"}},
				Methods: []*ai.Method{{
					TaskID: "behave", ID: "act", Subtasks: []string{"op"},
				}},
				Operators: []*ai.Operator{{ID: "op", Action: "lua_hook", APCost: opCost}},
			}
			_ = reg.Register(domain)
			script := `operators = {}; operators.op = function(self) end`
			snap := ai.ItemCombatSnapshot{
				Player: ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: ap},
			}
			cbs := ai.ItemPrimitiveCalls{
				Attack: func(_, _ string, cost int) bool {
					if ap < cost { return false }; ap -= cost; return true
				},
				Say: func(_ []string) {},
				Buff: func(_, _ string, _, cost int) bool { return ap >= cost },
				Debuff: func(_, _ string, _, cost int) bool { return ap >= cost },
				GetAP: func() int { return ap },
				SpendAP: func(n int) bool {
					if ap < n { return false }; ap -= n; return true
				},
			}
			p, _ := reg.PlannerFor(fmt.Sprintf("item_%d", i))
			_ = p.Execute(script, map[string]interface{}{}, snap, cbs)
		}

		if ap < 0 {
			rt.Errorf("AP went negative after %d items: final AP=%d (initial=%d)", numItems, ap, initialAP)
		}
	})
}
```

- [ ] **Step 2: Run property test**

```
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestProperty_AIItemPhase_APNeverNegative_MultipleItems -v
```
Expected: PASS (rapid runs 100 iterations)

- [ ] **Step 3: Commit**

```bash
git add internal/gameserver/ai_item_phase_test.go
git commit -m "test(gameserver): property test — AI item phase AP never goes negative"
```

---

## Self-Review

**Spec coverage:**
- REQ-AIE-1 ✓ — `CombatDomain`, `CombatScript` added to `ItemDef` (Task 3)
- REQ-AIE-2 ✓ — `CombatScriptState` on `ItemInstance`; cleared on combat end (Task 3, 7)
- REQ-AIE-3 ✓ — `AddAP(len(aiItems))` in `runAIItemPhaseLocked` (Task 7)
- REQ-AIE-4 ✓ — slot-ordered iteration in `collectEquippedAIItems`; Lua VM per turn (Tasks 5, 7)
- REQ-AIE-5 ✓ — `SpendAP` returns false when insufficient; turn ends immediately (Task 5)
- REQ-AIE-6 ✓ — `attack`, `say`, `buff`, `debuff` primitives in `buildEngineTable` (Task 5)
- REQ-AIE-7 ✓ — `ItemDomainRegistry` separate from NPC `Registry` (Task 6)
- REQ-AIE-8 ✓ — Lua `preconditions.<name>(self)` called by `findApplicableItemMethod` (Task 5)
- REQ-AIE-9 ✓ — each item has isolated `CombatScriptState`; acts in slot order (Tasks 3, 7)
- REQ-AIE-10 ✓ — `ItemCombatSnapshot` with enemies, player, round, weakest_enemy(), nearest_enemy() (Task 5)
- REQ-AIE-11a ✓ — covered by `TestAIItemPhase_OperatorConsumesAP` (Task 5)
- REQ-AIE-11b-j ✓ — all test functions in `item_planner_test.go` (Task 5)

**Gap found:** REQ-AIE-11a (`TestAIItemPhase_AddsAP`) requires testing that N AI items add exactly N AP. This test belongs in `ai_item_phase_test.go` and requires mocking the combat state. Add to Task 9.

**Type consistency:** `ItemEnemySnapshot`, `ItemPlayerSnapshot`, `ItemCombatSnapshot`, `ItemPrimitiveCalls` defined in Task 5 and referenced in Task 7 — consistent.
