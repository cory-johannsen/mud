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

// SkillChoices defines a pool of skills the player picks from at creation.
type SkillChoices struct {
	Pool  []string `yaml:"pool"`
	Count int      `yaml:"count"`
}

// SkillGrants defines the skill proficiencies a job grants at creation.
type SkillGrants struct {
	Fixed   []string      `yaml:"fixed"`
	Choices *SkillChoices `yaml:"choices"`
}

// FeatChoices defines a pool of feats the player picks from at creation.
type FeatChoices struct {
	Pool  []string `yaml:"pool"`
	Count int      `yaml:"count"`
}

// FeatGrants defines feat grants from a job at character creation.
// GeneralCount is how many general feats the player freely picks.
// Fixed is always granted. Choices is an optional selection pool.
type FeatGrants struct {
	GeneralCount int          `yaml:"general_count"`
	Fixed        []string     `yaml:"fixed"`
	Choices      *FeatChoices `yaml:"choices"`
}

// Job defines a concrete playable job (replaces PF2E class for Gunchete).
// Team is empty for jobs available to all players; non-empty for team-exclusive jobs.
//
// Precondition: ID, Name, Archetype, KeyAbility, and HitPointsPerLevel must be non-zero after loading.
type Job struct {
	ID                 string                             `yaml:"id"`
	Name               string                             `yaml:"name"`
	Archetype          string                             `yaml:"archetype"`
	Team               string                             `yaml:"team"` // empty = all teams; "gun" or "machete" = exclusive
	Description        string                             `yaml:"description"`
	KeyAbility         string                             `yaml:"key_ability"`
	HitPointsPerLevel  int                                `yaml:"hit_points_per_level"`
	Proficiencies      map[string]string                  `yaml:"proficiencies"`
	SkillGrants        *SkillGrants                       `yaml:"skills"`
	FeatGrants         *FeatGrants                        `yaml:"feats"`
	ClassFeatureGrants []string                           `yaml:"class_features"`
	Features           []JobFeature                       `yaml:"features"`
	Drawbacks          []JobDrawback                      `yaml:"drawbacks"`
	StartingInventory  *inventory.StartingLoadoutOverride `yaml:"starting_inventory"`
	TechnologyGrants   *TechnologyGrants                  `yaml:"technology_grants,omitempty"`
	// LevelUpGrants maps character level to the technology grants gained at that level.
	// Each entry is a delta — only new slots/techs added at that character level, not the
	// full cumulative table.
	LevelUpGrants map[int]*TechnologyGrants `yaml:"level_up_grants,omitempty"`
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
		if j.TechnologyGrants != nil {
			if err := j.TechnologyGrants.Validate(); err != nil {
				return nil, fmt.Errorf("job %q technology_grants: %w", j.ID, err)
			}
		}
		for charLevel, grants := range j.LevelUpGrants {
			if charLevel < 1 {
				return nil, fmt.Errorf("job %q level_up_grants: level key %d must be >= 1", j.ID, charLevel)
			}
			if err := grants.Validate(); err != nil {
				return nil, fmt.Errorf("job %q level_up_grants[%d]: %w", j.ID, charLevel, err)
			}
		}
		jobs = append(jobs, &j)
	}
	return jobs, nil
}
