package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadClassFeatures_Count(t *testing.T) {
	features, err := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
	if err != nil {
		t.Fatalf("LoadClassFeatures: %v", err)
	}
	if len(features) != 140 {
		t.Errorf("expected 140 features, got %d", len(features))
	}
}

func TestClassFeatureRegistry_ByArchetype(t *testing.T) {
	features, err := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
	if err != nil {
		t.Fatalf("LoadClassFeatures: %v", err)
	}
	reg := ruleset.NewClassFeatureRegistry(features)

	archetypeFeatures := reg.ByArchetype("aggressor")
	if len(archetypeFeatures) != 2 {
		t.Errorf("expected 2 aggressor archetype features, got %d", len(archetypeFeatures))
	}
}

func TestClassFeatureRegistry_ByJob(t *testing.T) {
	features, err := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
	if err != nil {
		t.Fatalf("LoadClassFeatures: %v", err)
	}
	reg := ruleset.NewClassFeatureRegistry(features)

	jobFeatures := reg.ByJob("soldier")
	if len(jobFeatures) != 2 {
		t.Errorf("expected 2 soldier job features, got %d", len(jobFeatures))
	}
}

func TestClassFeatureRegistry_ClassFeature(t *testing.T) {
	features, err := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
	if err != nil {
		t.Fatalf("LoadClassFeatures: %v", err)
	}
	reg := ruleset.NewClassFeatureRegistry(features)

	f, ok := reg.ClassFeature("brutal_surge")
	if !ok {
		t.Fatal("brutal_surge not found")
	}
	if !f.Active {
		t.Error("brutal_surge should be active")
	}
	if f.Archetype != "aggressor" {
		t.Errorf("expected archetype=aggressor, got %q", f.Archetype)
	}
}

func TestLoadClassFeaturesFromBytes_ActiveFields(t *testing.T) {
	yaml := []byte(`
class_features:
  - id: brutal_surge
    name: Brutal Surge
    archetype: aggressor
    job: ""
    pf2e: rage
    active: true
    shortcut: surge
    action_cost: 1
    contexts:
      - combat
    activate_text: "The red haze drops."
    condition_id: brutal_surge_active
    description: "Enter a frenzy."
    effect:
      type: condition
      target: self
      condition_id: brutal_surge_active
`)
	features, err := ruleset.LoadClassFeaturesFromBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
	f := features[0]
	if f.Shortcut != "surge" {
		t.Errorf("Shortcut: got %q, want %q", f.Shortcut, "surge")
	}
	if f.ActionCost != 1 {
		t.Errorf("ActionCost: got %d, want 1", f.ActionCost)
	}
	if len(f.Contexts) != 1 || f.Contexts[0] != "combat" {
		t.Errorf("Contexts: got %v, want [combat]", f.Contexts)
	}
	if f.Effect == nil {
		t.Fatal("Effect must not be nil for active feature")
	}
	if f.Effect.Type != "condition" {
		t.Errorf("Effect.Type: got %q, want %q", f.Effect.Type, "condition")
	}
	if f.Effect.Target != "self" {
		t.Errorf("Effect.Target: got %q, want %q", f.Effect.Target, "self")
	}
	if f.Effect.ConditionID != "brutal_surge_active" {
		t.Errorf("Effect.ConditionID: got %q, want %q", f.Effect.ConditionID, "brutal_surge_active")
	}
}

func TestActionEffect_AllTypes(t *testing.T) {
	yaml := []byte(`
class_features:
  - id: heal_action
    name: Patch Job
    archetype: ""
    job: medic
    pf2e: treat_wounds
    active: true
    shortcut: patch
    action_cost: 2
    contexts:
      - combat
      - exploration
    activate_text: "You patch yourself up."
    description: "Restore HP."
    effect:
      type: heal
      amount: "1d6+2"
  - id: skill_action
    name: Assess
    archetype: ""
    job: scout
    pf2e: recall_knowledge
    active: true
    shortcut: assess
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You assess the situation."
    description: "Skill check."
    effect:
      type: skill_check
      skill: awareness
      dc: 15
`)
	features, err := ruleset.LoadClassFeaturesFromBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(features) != 2 {
		t.Fatalf("expected 2, got %d", len(features))
	}
	heal := features[0]
	if heal.Effect == nil || heal.Effect.Type != "heal" {
		t.Errorf("heal: wrong effect type")
	}
	if heal.Effect.Amount != "1d6+2" {
		t.Errorf("heal.Amount: got %q", heal.Effect.Amount)
	}
	skill := features[1]
	if skill.Effect == nil || skill.Effect.Type != "skill_check" {
		t.Errorf("skill: wrong effect type")
	}
	if skill.Effect.DC != 15 {
		t.Errorf("skill.DC: got %d", skill.Effect.DC)
	}
}

func TestClassFeatureRegistry_ActiveOnly(t *testing.T) {
	features := []*ruleset.ClassFeature{
		{ID: "passive_feat", Active: false},
		{ID: "surge", Active: true, Shortcut: "surge", ActionCost: 1, Contexts: []string{"combat"}},
		{ID: "patch", Active: true, Shortcut: "patch", ActionCost: 2, Contexts: []string{"exploration"}},
	}
	reg := ruleset.NewClassFeatureRegistry(features)
	active := reg.ActiveFeatures()
	if len(active) != 2 {
		t.Errorf("expected 2 active features, got %d", len(active))
	}
	// ActiveFeatures must return features sorted by ID for deterministic order.
	if active[0].ID != "patch" {
		t.Errorf("first active feature: got %q, want %q", active[0].ID, "patch")
	}
	if active[1].ID != "surge" {
		t.Errorf("second active feature: got %q, want %q", active[1].ID, "surge")
	}
}
