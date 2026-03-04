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
	if len(features) != 88 {
		t.Errorf("expected 88 features, got %d", len(features))
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
	if len(jobFeatures) != 1 {
		t.Errorf("expected 1 soldier job feature, got %d", len(jobFeatures))
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
