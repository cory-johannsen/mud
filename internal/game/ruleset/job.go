package ruleset

import (
	"fmt"
	"os"
	"time"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"gopkg.in/yaml.v3"
)

// JobFeature describes a single feature gained at a specific level.
type JobFeature struct {
	Name        string `yaml:"name"`
	Level       int    `yaml:"level"`
	Description string `yaml:"description"`
}

// DrawbackDef defines a mandatory flaw tied to a job (replaces JobDrawback).
// Type is "passive" or "situational" (REQ-JD-8, REQ-JD-10).
type DrawbackDef struct {
	ID                string        `yaml:"id"`
	Type              string        `yaml:"type"` // "passive" | "situational"
	Description       string        `yaml:"description"`
	ConditionID       string        `yaml:"condition_id,omitempty"`
	StatModifier      *StatModifier `yaml:"stat_modifier,omitempty"`
	Trigger           string        `yaml:"trigger,omitempty"`
	EffectConditionID string        `yaml:"effect_condition_id,omitempty"`
	Duration          string        `yaml:"duration,omitempty"` // Go time.Duration string; default "1h"
}

// StatModifier is a persistent stat penalty applied while a job is held.
type StatModifier struct {
	Stat   string `yaml:"stat"`
	Amount int    `yaml:"amount"`
}

// AdvancementRequirements defines prerequisites for advancing to this job tier.
type AdvancementRequirements struct {
	MinLevel           int               `yaml:"min_level,omitempty"`
	RequiredFeats      []string          `yaml:"required_feats,omitempty"`
	RequiredSkillRanks map[string]string `yaml:"required_skill_ranks,omitempty"`
	PrerequisiteJobs   []string          `yaml:"prerequisite_jobs,omitempty"`
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
// Precondition: ID, Name, Archetype, KeyAbility, HitPointsPerLevel, and Tier must be non-zero after loading.
// REQ-DTQ-13: Tier must be present in the YAML; missing or zero value is a fatal load error.
type Job struct {
	ID                      string                             `yaml:"id"`
	Name                    string                             `yaml:"name"`
	Archetype               string                             `yaml:"archetype"`
	Tier                    int                                `yaml:"tier"` // REQ-DTQ-13: required; 1=entry, 2=skilled, 3=specialist
	Team                    string                             `yaml:"team"` // empty = all teams; "gun" or "machete" = exclusive
	Description             string                             `yaml:"description"`
	KeyAbility              string                             `yaml:"key_ability"`
	HitPointsPerLevel       int                                `yaml:"hit_points_per_level"`
	Proficiencies           map[string]string                  `yaml:"proficiencies"`
	SkillGrants             *SkillGrants                       `yaml:"skills"`
	FeatGrants              *FeatGrants                        `yaml:"feats"`
	ClassFeatureGrants      []string                           `yaml:"class_features"`
	Features                []JobFeature                       `yaml:"features"`
	AdvancementRequirements AdvancementRequirements            `yaml:"advancement_requirements,omitempty"`
	Drawbacks               []DrawbackDef                      `yaml:"drawbacks,omitempty"`
	StartingInventory       *inventory.StartingLoadoutOverride `yaml:"starting_inventory"`
	TechnologyGrants        *TechnologyGrants                  `yaml:"technology_grants,omitempty"`
	LevelUpGrants           map[int]*TechnologyGrants          `yaml:"level_up_grants,omitempty"`
	LevelUpFeatGrants       map[int]*FeatGrants                `yaml:"level_up_feat_grants,omitempty"`
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
		if j.Tier == 0 {
			return nil, fmt.Errorf("job %q: missing required field 'tier' (REQ-DTQ-13)", j.ID)
		}
		if j.Tier < 1 || j.Tier > 3 {
			return nil, fmt.Errorf("job %q has invalid tier %d: must be 1, 2, or 3 (REQ-DTQ-13)", j.ID, j.Tier)
		}
		// REQ-JD-2: default min_level for advancement_requirements if absent
		if j.Tier == 2 && j.AdvancementRequirements.MinLevel == 0 {
			j.AdvancementRequirements.MinLevel = 10
		}
		if j.Tier == 3 && j.AdvancementRequirements.MinLevel == 0 {
			j.AdvancementRequirements.MinLevel = 15
		}
		for _, db := range j.Drawbacks {
			if db.Duration != "" {
				if _, err := time.ParseDuration(db.Duration); err != nil {
					return nil, fmt.Errorf("job %q drawback %q: invalid duration %q: %w", j.ID, db.ID, db.Duration, err)
				}
			}
		}
		if j.TechnologyGrants != nil {
			if err := j.TechnologyGrants.Validate(); err != nil {
				return nil, fmt.Errorf("job %q technology_grants: %w", j.ID, err)
			}
		}
		for charLevel, grants := range j.LevelUpGrants {
			if charLevel < 1 {
				return nil, fmt.Errorf("job %q level_up_grants: key %d must be >= 1", j.ID, charLevel)
			}
			if err := grants.Validate(); err != nil {
				return nil, fmt.Errorf("job %q level_up_grants[%d]: %w", j.ID, charLevel, err)
			}
		}
		jobs = append(jobs, &j)
	}
	return jobs, nil
}
