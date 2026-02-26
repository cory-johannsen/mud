package ai_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"pgregory.net/rapid"
)

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

func TestProperty_Domain_OperatorByID_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z_]{1,10}`).Draw(rt, "id")
		d := &ai.Domain{
			Operators: []*ai.Operator{{ID: "op1", Action: "pass"}},
		}
		_, _ = d.OperatorByID(id) // must not panic for any id
	})
}
