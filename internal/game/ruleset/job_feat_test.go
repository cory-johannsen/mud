package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestJob_FeatGrants_LoadsFromYAML(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../../content/jobs")
	if err != nil {
		t.Fatalf("LoadJobs: %v", err)
	}
	var found bool
	for _, j := range jobs {
		if j.FeatGrants != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one job with FeatGrants set")
	}
}
