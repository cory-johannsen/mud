package character_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestBuildClassFeaturesFromJob_ReturnsAllGrants(t *testing.T) {
	job := &ruleset.Job{
		ID:                 "soldier",
		ClassFeatureGrants: []string{"street_brawler", "brutal_surge", "guerilla_warfare"},
	}

	result := character.BuildClassFeaturesFromJob(job)

	if len(result) != 3 {
		t.Errorf("expected 3 class features, got %d", len(result))
	}

	featureSet := make(map[string]bool)
	for _, id := range result {
		featureSet[id] = true
	}
	for _, expected := range []string{"street_brawler", "brutal_surge", "guerilla_warfare"} {
		if !featureSet[expected] {
			t.Errorf("expected feature %q not found in result %v", expected, result)
		}
	}
}

func TestBuildClassFeaturesFromJob_NilGrants(t *testing.T) {
	job := &ruleset.Job{ID: "test", ClassFeatureGrants: nil}
	result := character.BuildClassFeaturesFromJob(job)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil grants, got %v", result)
	}
}

func TestBuildClassFeaturesFromJob_EmptyGrants(t *testing.T) {
	job := &ruleset.Job{ID: "test", ClassFeatureGrants: []string{}}
	result := character.BuildClassFeaturesFromJob(job)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty grants, got %v", result)
	}
}

func TestBuildClassFeaturesFromJob_NilJob(t *testing.T) {
	result := character.BuildClassFeaturesFromJob(nil)
	if result != nil {
		t.Errorf("expected nil for nil job, got %v", result)
	}
}

func TestCharacter_HasClassFeaturesField(t *testing.T) {
	// This test verifies ClassFeatures field exists on Character
	c := &character.Character{
		ClassFeatures: []string{"street_brawler", "brutal_surge"},
	}
	if len(c.ClassFeatures) != 2 {
		t.Errorf("ClassFeatures field not working, got %v", c.ClassFeatures)
	}
}
