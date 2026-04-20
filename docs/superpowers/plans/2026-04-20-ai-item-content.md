# AI Item Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the AI Chainsaw and AI AK-47 item data files (YAML + HTN domains) and their behavioral tests.

**Architecture:** Pure data addition — no Go source changes. Each AI item is a YAML item definition with an embedded Lua `combat_script` and a matching HTN domain YAML in `content/ai/`. The engine (Plan 1) loads both files and wires them together at startup.

**Tech Stack:** YAML content files, Go test suite (`pgregory.net/rapid` for property tests), existing `ItemRegistry` and `ItemDomainRegistry` loaders.

**Dependency:** This plan MUST NOT be executed until `docs/superpowers/plans/2026-04-20-ai-item-engine.md` is fully implemented and merged.

---

## File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `content/items/ai_chainsaw.yaml` | AI Chainsaw item definition with embedded Lua script |
| Create | `content/ai/ai_chainsaw_combat.yaml` | HTN domain for AI Chainsaw |
| Create | `content/items/ai_ak47.yaml` | AI AK-47 item definition with embedded Lua script |
| Create | `content/ai/ai_ak47_combat.yaml` | HTN domain for AI AK-47 |
| Create | `internal/game/ai/item_content_test.go` | Behavioral tests for both items |

---

### Task 1: AI Chainsaw item YAML

**Files:**
- Create: `content/items/ai_chainsaw.yaml`

- [ ] **Step 1: Write the failing test for item load**

In `internal/game/ai/item_content_test.go`:

```go
package ai_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestAIChainsaw_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	if err != nil {
		t.Fatalf("read ai_chainsaw.yaml: %v", err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal ai_chainsaw.yaml: %v", err)
	}
	if def.ID != "ai_chainsaw" {
		t.Errorf("expected id ai_chainsaw, got %q", def.ID)
	}
	if def.CombatDomain != "ai_chainsaw_combat" {
		t.Errorf("expected combat_domain ai_chainsaw_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIChainsaw_LoadsWithoutError -v
```
Expected: FAIL — file not found.

- [ ] **Step 3: Create `content/items/ai_chainsaw.yaml`**

```yaml
id: ai_chainsaw
name: AI Chainsaw
description: >
  A salvaged power tool with a neural interface spliced into the grip. It has
  opinions. Loud ones. It really wants to be used.
kind: weapon
weapon_ref: chainsaw
weight: 3.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_chainsaw_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.frenzy_active = function(self)
    return (self.state.kills or 0) >= 2
  end

  operators.overkill_strike = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "2d6+4", 2)
      if target.hp <= 0 then
        self.state.kills = (self.state.kills or 0) + 1
      end
      self.engine.say({"I CAN'T STOP!", "RAAAH!", "GIVE ME EVERYTHING!", "DO IT AGAIN!"})
    end
  end

  operators.attack_weakest = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "1d6+2")
      if target.hp <= 0 then
        self.state.kills = (self.state.kills or 0) + 1
      end
      self.engine.say({"YES!", "MORE!", "BLOOD!", "KEEP GOING!"})
    end
  end

  operators.say_hungry = function(self)
    self.engine.say({"Feed me...", "Who's next?", "I can smell them.", "Let me at them."})
  end
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIChainsaw_LoadsWithoutError -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/items/ai_chainsaw.yaml internal/game/ai/item_content_test.go
git commit -m "feat(content): add AI Chainsaw item YAML and load test"
```

---

### Task 2: AI Chainsaw HTN domain YAML

**Files:**
- Create: `content/ai/ai_chainsaw_combat.yaml`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ai/item_content_test.go`:

```go
func TestAIChainsawDomain_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	if err != nil {
		t.Fatalf("read ai_chainsaw_combat.yaml: %v", err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		t.Fatalf("unmarshal ai_chainsaw_combat.yaml: %v", err)
	}
	if df.Domain.ID != "ai_chainsaw_combat" {
		t.Errorf("expected domain id ai_chainsaw_combat, got %q", df.Domain.ID)
	}
	if err := df.Domain.Validate(); err != nil {
		t.Errorf("domain Validate(): %v", err)
	}
}
```

Note: add `"github.com/cory-johannsen/mud/internal/game/ai"` to imports.

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIChainsawDomain_LoadsWithoutError -v
```
Expected: FAIL — file not found.

- [ ] **Step 3: Create `content/ai/ai_chainsaw_combat.yaml`**

```yaml
domain:
  id: ai_chainsaw_combat
  description: Bloodthirsty berserker. Targets weakest enemy, escalates on kills.

  tasks:
    - id: behave
      description: Root task — hunt or idle
    - id: hunt
      description: Engage and kill

  methods:
    - task: behave
      id: frenzy_mode
      precondition: frenzy_active
      subtasks: [overkill_strike]

    - task: behave
      id: hunt_mode
      precondition: has_enemy
      subtasks: [attack_weakest]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [say_hungry]

  operators:
    - id: overkill_strike
      action: lua_hook
      ap_cost: 2

    - id: attack_weakest
      action: lua_hook
      ap_cost: 1

    - id: say_hungry
      action: lua_hook
      ap_cost: 0
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIChainsawDomain_LoadsWithoutError -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/ai/ai_chainsaw_combat.yaml internal/game/ai/item_content_test.go
git commit -m "feat(content): add AI Chainsaw HTN domain YAML and domain load test"
```

---

### Task 3: AI AK-47 item YAML

**Files:**
- Create: `content/items/ai_ak47.yaml`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ai/item_content_test.go`:

```go
func TestAIAK47_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/items/ai_ak47.yaml")
	if err != nil {
		t.Fatalf("read ai_ak47.yaml: %v", err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal ai_ak47.yaml: %v", err)
	}
	if def.ID != "ai_ak47" {
		t.Errorf("expected id ai_ak47, got %q", def.ID)
	}
	if def.CombatDomain != "ai_ak47_combat" {
		t.Errorf("expected combat_domain ai_ak47_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIAK47_LoadsWithoutError -v
```
Expected: FAIL — file not found.

- [ ] **Step 3: Create `content/items/ai_ak47.yaml`**

```yaml
id: ai_ak47
name: AI AK-47
description: >
  A battered Kalashnikov with a cracked polymer stock and a jury-rigged
  cognitive module zip-tied to the receiver. It's figured out the pattern.
  It's always figured out the pattern.
kind: weapon
weapon_ref: assault_rifle
weight: 3.5
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_ak47_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.enough_ap_for_full_sequence = function(self)
    return self.combat.player.ap >= 3
  end

  operators.burst_and_theorize = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "2d6", 2)
      self.engine.debuff(target.id, "fear", 2)
      self.engine.say({
        "They sent this one specifically for you.",
        "Classic distraction unit.",
        "I've seen this pattern before — it ends badly for them.",
        "Every faction has one of these. You know what that means.",
        "This connects to something much bigger. Trust me."
      })
    end
  end

  operators.quick_shot = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "1d6+1")
      self.engine.say({"Noted.", "As expected.", "They're all connected."})
    end
  end
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIAK47_LoadsWithoutError -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/items/ai_ak47.yaml internal/game/ai/item_content_test.go
git commit -m "feat(content): add AI AK-47 item YAML and load test"
```

---

### Task 4: AI AK-47 HTN domain YAML

**Files:**
- Create: `content/ai/ai_ak47_combat.yaml`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ai/item_content_test.go`:

```go
func TestAIAK47Domain_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	if err != nil {
		t.Fatalf("read ai_ak47_combat.yaml: %v", err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		t.Fatalf("unmarshal ai_ak47_combat.yaml: %v", err)
	}
	if df.Domain.ID != "ai_ak47_combat" {
		t.Errorf("expected domain id ai_ak47_combat, got %q", df.Domain.ID)
	}
	if err := df.Domain.Validate(); err != nil {
		t.Errorf("domain Validate(): %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIAK47Domain_LoadsWithoutError -v
```
Expected: FAIL — file not found.

- [ ] **Step 3: Create `content/ai/ai_ak47_combat.yaml`**

```yaml
domain:
  id: ai_ak47_combat
  description: Paranoid conspiracy theorist. Burst attack + fear debuff + monologue.

  tasks:
    - id: behave
      description: Root task — theorize or suppress

  methods:
    - task: behave
      id: full_sequence
      precondition: enough_ap_for_full_sequence
      subtasks: [burst_and_theorize]

    - task: behave
      id: quick_mode
      precondition: has_enemy
      subtasks: [quick_shot]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [quick_shot]

  operators:
    - id: burst_and_theorize
      action: lua_hook
      ap_cost: 3

    - id: quick_shot
      action: lua_hook
      ap_cost: 1
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run TestAIAK47Domain_LoadsWithoutError -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/ai/ai_ak47_combat.yaml internal/game/ai/item_content_test.go
git commit -m "feat(content): add AI AK-47 HTN domain YAML and domain load test"
```

---

### Task 5: AI Chainsaw behavioral tests

**Files:**
- Modify: `internal/game/ai/item_content_test.go`

These tests exercise `ItemPlanner.Execute` directly using stub `ItemPrimitiveCalls`. They validate the spec invariants (REQ-AIC-3a through REQ-AIC-3d).

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/ai/item_content_test.go`:

```go
// TestAIChainsaw_KillCountEscalates verifies that after 2 kills in an encounter,
// the frenzy method is selected over the standard hunt method.
func TestAIChainsaw_KillCountEscalates(t *testing.T) {
	// Load item def to get the combat script
	data, err := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	if err != nil {
		t.Fatalf("read ai_chainsaw.yaml: %v", err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Load domain
	domData, err := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	if err != nil {
		t.Fatalf("read domain: %v", err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(domData, &df); err != nil {
		t.Fatalf("unmarshal domain: %v", err)
	}

	planner := ai.NewItemPlanner(df.Domain)

	// state with 2 kills already recorded — frenzy should activate
	state := map[string]interface{}{"kills": float64(2)}

	var attackFormula string
	var attackCost int
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(targetID, formula string, cost int) bool {
			attackFormula = formula
			attackCost = cost
			return true
		},
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}

	snapshot := ai.ItemCombatSnapshot{
		Player: ai.ItemPlayerSnapshot{ID: "player1", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{
			{ID: "enemy1", HP: 20},
		},
	}

	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// frenzy_active is true (kills=2), so overkill_strike fires
	if attackFormula != "2d6+4" {
		t.Errorf("expected overkill formula 2d6+4, got %q", attackFormula)
	}
	if attackCost != 2 {
		t.Errorf("expected overkill AP cost 2, got %d", attackCost)
	}
}

// TestAIChainsaw_OverkillCosts2AP verifies frenzy operator decrements pool by 2.
func TestAIChainsaw_OverkillCosts2AP(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(df.Domain)

	state := map[string]interface{}{"kills": float64(2)}
	spent := 0
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { spent += cost; return true },
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 10}},
	}
	planner.Execute(def.CombatScript, state, snapshot, cbs)
	if spent != 2 {
		t.Errorf("expected overkill to spend 2 AP, got %d", spent)
	}
}

// TestAIChainsaw_IdleFallback verifies that with no enemies, say_hungry fires and costs 0 AP.
func TestAIChainsaw_IdleFallback(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(df.Domain)

	state := map[string]interface{}{}
	said := false
	spent := 0
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { return false },
		Say:     func(pool []string) { said = true },
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { spent += n; return true },
	}
	// No enemies
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{},
	}
	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !said {
		t.Error("expected say_hungry to fire with no enemies")
	}
	if spent != 0 {
		t.Errorf("expected idle to spend 0 AP, got %d", spent)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run "TestAIChainsaw_KillCountEscalates|TestAIChainsaw_OverkillCosts2AP|TestAIChainsaw_IdleFallback" -v
```
Expected: FAIL — `NewItemPlanner` not yet defined (engine not merged yet; these tests are written in advance).

> **Note:** If the engine plan has been merged, these tests should pass after Task 4. If the engine is not yet merged, commit the test file and it will be validated during engine integration.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/ai/item_content_test.go
git commit -m "test(content): add AI Chainsaw behavioral invariant tests"
```

---

### Task 6: AI AK-47 behavioral tests

**Files:**
- Modify: `internal/game/ai/item_content_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/ai/item_content_test.go`:

```go
// TestAIAK47_FullSequenceAt3AP verifies burst_and_theorize fires when AP >= 3.
func TestAIAK47_FullSequenceAt3AP(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_ak47.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(df.Domain)

	state := map[string]interface{}{}
	var attackFormula string
	var attackCost int
	debuffCalled := false
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(targetID, formula string, cost int) bool {
			attackFormula = formula
			attackCost = cost
			return true
		},
		Say:    func(pool []string) {},
		Buff:   func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff: func(targetID, effectID string, rounds, cost int) bool { debuffCalled = true; return true },
		GetAP:  func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 30}},
	}
	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if attackFormula != "2d6" {
		t.Errorf("expected burst formula 2d6, got %q", attackFormula)
	}
	if attackCost != 2 {
		t.Errorf("expected burst attack cost 2, got %d", attackCost)
	}
	if !debuffCalled {
		t.Error("expected fear debuff to be applied")
	}
}

// TestAIAK47_QuickShotBelow3AP verifies quick_shot fires when AP < 3.
func TestAIAK47_QuickShotBelow3AP(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_ak47.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(df.Domain)

	state := map[string]interface{}{}
	var attackFormula string
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { attackFormula = formula; return true },
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 2 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 2},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 30}},
	}
	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if attackFormula != "1d6+1" {
		t.Errorf("expected quick_shot formula 1d6+1, got %q", attackFormula)
	}
}

// TestAIAK47_FearDebuffDuration verifies fear condition lasts exactly 2 rounds.
func TestAIAK47_FearDebuffDuration(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_ak47.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(df.Domain)

	state := map[string]interface{}{}
	var debuffRounds int
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { return true },
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { debuffRounds = rounds; return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 30}},
	}
	planner.Execute(def.CombatScript, state, snapshot, cbs)
	if debuffRounds != 2 {
		t.Errorf("expected fear debuff to last 2 rounds, got %d", debuffRounds)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (or pass if engine is merged)**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/ai/... -run "TestAIAK47_FullSequenceAt3AP|TestAIAK47_QuickShotBelow3AP|TestAIAK47_FearDebuffDuration" -v
```

- [ ] **Step 3: Run full test suite**

```
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```
Expected: All pre-existing tests PASS. New tests pass if engine is merged; compile errors are acceptable if engine is not yet merged.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/ai/item_content_test.go
git commit -m "test(content): add AI AK-47 behavioral invariant tests"
```
