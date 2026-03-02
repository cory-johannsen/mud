package ruleset

import (
	"fmt"
	"os"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"gopkg.in/yaml.v3"
)

// JobFeature describes a single feature gained at a specific level.
type JobFeature struct {
	Name        string `yaml:"name"`
	Level       int    `yaml:"level"`
	Description string `yaml:"description"`
}

// JobDrawback describes a mandatory flaw or restriction tied to the job.
type JobDrawback struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Job defines a concrete playable job (replaces PF2E class for Gunchete).
// Team is empty for jobs available to all players; non-empty for team-exclusive jobs.
//
// Precondition: ID, Name, Archetype, KeyAbility, and HitPointsPerLevel must be non-zero after loading.
type Job struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	Archetype         string            `yaml:"archetype"`
	Team              string            `yaml:"team"` // empty = all teams; "gun" or "machete" = exclusive
	Description       string            `yaml:"description"`
	KeyAbility        string            `yaml:"key_ability"`
	HitPointsPerLevel int               `yaml:"hit_points_per_level"`
	Proficiencies     map[string]string                    `yaml:"proficiencies"`
	Features          []JobFeature                         `yaml:"features"`
	Drawbacks         []JobDrawback                        `yaml:"drawbacks"`
	StartingInventory *inventory.StartingLoadoutOverride   `yaml:"starting_inventory"`
}

// LoadJobs reads all .yaml files in dir and parses each as a Job.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns all parsed jobs (may be empty slice) or a non-nil error.
func LoadJobs(dir string) ([]*Job, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	jobs := make([]*Job, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var j Job
		if err := yaml.Unmarshal(data, &j); err != nil {
			return nil, fmt.Errorf("parsing job file %s: %w", path, err)
		}
		jobs = append(jobs, &j)
	}
	return jobs, nil
}
