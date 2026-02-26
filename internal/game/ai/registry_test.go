package ai_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
)

func TestRegistry_Register_And_PlannerFor(t *testing.T) {
	reg := ai.NewRegistry()
	domain := gangerDomain()
	caller := &mockScriptCaller{returnVal: nil}
	if err := reg.Register(domain, caller, "downtown"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	planner, ok := reg.PlannerFor("ganger_combat")
	if !ok || planner == nil {
		t.Fatal("expected planner for ganger_combat")
	}
}

func TestRegistry_Register_CollisionError(t *testing.T) {
	reg := ai.NewRegistry()
	domain := gangerDomain()
	caller := &mockScriptCaller{}
	_ = reg.Register(domain, caller, "downtown")
	if err := reg.Register(domain, caller, "downtown"); err == nil {
		t.Fatal("expected collision error on second Register")
	}
}

func TestRegistry_PlannerFor_NotFound(t *testing.T) {
	reg := ai.NewRegistry()
	_, ok := reg.PlannerFor("missing")
	if ok {
		t.Fatal("expected not found")
	}
}
