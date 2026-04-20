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
		Methods:   []*ai.Method{{TaskID: "behave", ID: "m1", Subtasks: []string{"op1"}}},
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
