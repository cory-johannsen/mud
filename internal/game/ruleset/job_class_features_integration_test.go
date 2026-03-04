package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestAllJobsHaveClassFeatureGrants(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../../content/jobs")
	if err != nil {
		t.Fatalf("LoadJobs: %v", err)
	}
	if len(jobs) != 76 {
		t.Errorf("expected 76 jobs, got %d", len(jobs))
	}
	for _, j := range jobs {
		if len(j.ClassFeatureGrants) == 0 {
			t.Errorf("job %q has no ClassFeatureGrants", j.ID)
		}
		if len(j.ClassFeatureGrants) != 3 {
			t.Errorf("job %q has %d ClassFeatureGrants, expected 3: %v", j.ID, len(j.ClassFeatureGrants), j.ClassFeatureGrants)
		}
	}
}
