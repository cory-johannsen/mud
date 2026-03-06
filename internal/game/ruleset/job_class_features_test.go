package ruleset_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestJob_ClassFeatureGrants_Parses(t *testing.T) {
	content := `
id: test_job
name: Test Job
archetype: aggressor
class_features:
  - resilience
  - working_stiff
`
	var j ruleset.Job
	if err := yaml.Unmarshal([]byte(content), &j); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(j.ClassFeatureGrants) != 2 {
		t.Errorf("expected 2 ClassFeatureGrants, got %d: %v", len(j.ClassFeatureGrants), j.ClassFeatureGrants)
	}
	if j.ClassFeatureGrants[0] != "resilience" {
		t.Errorf("expected resilience, got %q", j.ClassFeatureGrants[0])
	}
	if j.ClassFeatureGrants[1] != "working_stiff" {
		t.Errorf("expected working_stiff, got %q", j.ClassFeatureGrants[1])
	}
}
