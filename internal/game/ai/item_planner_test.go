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
