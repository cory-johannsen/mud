package ai_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// noopScriptCaller satisfies ai.ScriptCaller; every precondition returns true.
type noopScriptCaller struct{}

func (n *noopScriptCaller) CallHook(_, _ string, _ ...lua.LValue) (lua.LValue, error) {
	return lua.LTrue, nil
}

func TestDomain_Validate_RejectsEmpty(t *testing.T) {
	d := &ai.Domain{}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty Domain")
	}
}

func TestDomain_Validate_AcceptsMinimal(t *testing.T) {
	d := &ai.Domain{
		ID:    "test",
		Tasks: []*ai.Task{{ID: "root"}},
		Methods: []*ai.Method{{
			TaskID:   "root",
			ID:       "m1",
			Subtasks: []string{"op1"},
		}},
		Operators: []*ai.Operator{{ID: "op1", Action: "pass"}},
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDomain_OperatorByID_Found(t *testing.T) {
	d := &ai.Domain{
		Operators: []*ai.Operator{{ID: "attack", Action: "attack", Target: "nearest_enemy"}},
	}
	op, ok := d.OperatorByID("attack")
	if !ok || op.Action != "attack" {
		t.Fatal("expected to find operator")
	}
}

func TestDomain_OperatorByID_NotFound(t *testing.T) {
	d := &ai.Domain{}
	_, ok := d.OperatorByID("missing")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestDomain_MethodsForTask_ReturnsOrdered(t *testing.T) {
	d := &ai.Domain{
		Methods: []*ai.Method{
			{TaskID: "fight", ID: "m1", Subtasks: []string{"op1"}},
			{TaskID: "fight", ID: "m2", Subtasks: []string{"op2"}},
			{TaskID: "other", ID: "m3", Subtasks: []string{"op3"}},
		},
	}
	methods := d.MethodsForTask("fight")
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}
	if methods[0].ID != "m1" || methods[1].ID != "m2" {
		t.Fatalf("expected methods in declaration order [m1, m2], got [%s, %s]", methods[0].ID, methods[1].ID)
	}
}

func TestLoadDomains_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `
domain:
  id: test_domain
  description: Test
  tasks:
    - id: behave
      description: root
  methods:
    - task: behave
      id: default
      subtasks: [idle]
  operators:
    - id: idle
      action: pass
      target: ""
`
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	domains, err := ai.LoadDomains(dir)
	if err != nil {
		t.Fatalf("LoadDomains: %v", err)
	}
	if len(domains) != 1 || domains[0].ID != "test_domain" {
		t.Fatalf("unexpected domains: %v", domains)
	}
}

func TestProperty_Domain_OperatorByID_ConsistentLookup(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Build a domain with 1-5 operators with distinct IDs
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		ops := make([]*ai.Operator, n)
		ids := make([]string, n)
		for i := range ops {
			id := fmt.Sprintf("op%d", i)
			ids[i] = id
			ops[i] = &ai.Operator{ID: id, Action: "pass"}
		}
		d := &ai.Domain{Operators: ops}

		// Property: every ID in the list is found
		for _, id := range ids {
			op, ok := d.OperatorByID(id)
			if !ok {
				rt.Fatalf("OperatorByID(%q) returned not found, expected found", id)
			}
			if op.ID != id {
				rt.Fatalf("OperatorByID(%q) returned op with ID %q", id, op.ID)
			}
		}

		// Property: a random ID not in the list is not found
		unknown := rapid.StringMatching(`[a-z_]{1,10}`).Draw(rt, "unknown")
		inList := false
		for _, id := range ids {
			if id == unknown {
				inList = true
				break
			}
		}
		if !inList {
			_, ok := d.OperatorByID(unknown)
			if ok {
				rt.Fatalf("OperatorByID(%q) returned found, expected not found", unknown)
			}
		}
	})
}

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

func TestPlanner_PropagatesOperatorMetadata(t *testing.T) {
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
	caller := &noopScriptCaller{}
	planner := ai.NewPlanner(&wrapper.Domain, caller, "")

	ws := &ai.WorldState{
		NPC: &ai.NPCState{UID: "npc1"},
		Combatants: []*ai.CombatantState{
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
		t.Errorf("CooldownRounds: want 4, got %d", a.CooldownRounds)
	}
	if a.APCost != 2 {
		t.Errorf("APCost: want 2, got %d", a.APCost)
	}
}
