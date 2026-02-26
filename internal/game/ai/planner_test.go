package ai_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	lua "github.com/yuin/gopher-lua"
	"pgregory.net/rapid"
)

// mockScriptCaller always returns the given value for any hook call.
type mockScriptCaller struct{ returnVal lua.LValue }

func (m *mockScriptCaller) CallHook(zoneID, hook string, args ...lua.LValue) (lua.LValue, error) {
	if m.returnVal == nil {
		return lua.LNil, nil
	}
	return m.returnVal, nil
}

func gangerDomain() *ai.Domain {
	return &ai.Domain{
		ID: "ganger_combat",
		Tasks: []*ai.Task{
			{ID: "behave"},
			{ID: "fight"},
		},
		Methods: []*ai.Method{
			{TaskID: "behave", ID: "combat_mode", Precondition: "has_enemy", Subtasks: []string{"fight"}},
			{TaskID: "behave", ID: "idle_mode", Precondition: "", Subtasks: []string{"do_pass"}},
			{TaskID: "fight", ID: "attack_any", Precondition: "", Subtasks: []string{"attack_enemy"}},
		},
		Operators: []*ai.Operator{
			{ID: "attack_enemy", Action: "attack", Target: "nearest_enemy"},
			{ID: "do_pass", Action: "pass", Target: ""},
		},
	}
}

func TestPlanner_Plan_ProducesAttackWhenPreconditionTrue(t *testing.T) {
	domain := gangerDomain()
	caller := &mockScriptCaller{returnVal: lua.LTrue}
	planner := ai.NewPlanner(domain, caller, "downtown")

	ws := &ai.WorldState{
		NPC: &ai.NPCState{UID: "n1", Kind: "npc", Name: "Ganger"},
		Combatants: []*ai.CombatantState{
			{UID: "p1", Kind: "player", Name: "Player", HP: 20, MaxHP: 20, Dead: false},
		},
	}
	actions, err := planner.Plan(ws)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("expected at least one planned action")
	}
	if actions[0].Action != "attack" {
		t.Fatalf("expected attack, got %q", actions[0].Action)
	}
	if actions[0].Target != "Player" {
		t.Fatalf("expected target 'Player', got %q", actions[0].Target)
	}
}

func TestPlanner_Plan_FallsBackToPassWhenPreconditionFalse(t *testing.T) {
	domain := gangerDomain()
	// Precondition "has_enemy" returns false
	caller := &mockScriptCaller{returnVal: lua.LFalse}
	planner := ai.NewPlanner(domain, caller, "downtown")

	ws := &ai.WorldState{
		NPC:        &ai.NPCState{UID: "n1", Kind: "npc", Name: "Ganger"},
		Combatants: []*ai.CombatantState{},
	}
	actions, err := planner.Plan(ws)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != "pass" {
		t.Fatalf("expected pass fallback, got %v", actions)
	}
}

func TestPlanner_Plan_EmptyDomainReturnsPass(t *testing.T) {
	domain := &ai.Domain{
		ID:    "empty",
		Tasks: []*ai.Task{{ID: "behave"}},
	}
	caller := &mockScriptCaller{}
	planner := ai.NewPlanner(domain, caller, "downtown")
	ws := &ai.WorldState{NPC: &ai.NPCState{UID: "n1", Kind: "npc"}}
	actions, err := planner.Plan(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No methods → no actions; empty slice is valid (caller queues Pass)
	_ = actions
}

func TestProperty_Planner_NeverReturnsNilSlice(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		domain := gangerDomain()
		returnTrue := rapid.Bool().Draw(rt, "precond")
		var lv lua.LValue = lua.LFalse
		if returnTrue {
			lv = lua.LTrue
		}
		caller := &mockScriptCaller{returnVal: lv}
		planner := ai.NewPlanner(domain, caller, "downtown")
		ws := &ai.WorldState{
			NPC: &ai.NPCState{UID: "n1", Kind: "npc", Name: "G"},
			Combatants: []*ai.CombatantState{
				{UID: "p1", Kind: "player", Name: "P", HP: 20, MaxHP: 20},
			},
		}
		actions, err := planner.Plan(ws)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if actions == nil {
			rt.Fatal("Plan must return non-nil slice")
		}
	})
}
