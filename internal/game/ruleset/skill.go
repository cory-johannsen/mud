package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Skill defines one Gunchete skill and its P2FE equivalent.
type Skill struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Ability     string `yaml:"ability"`
	PF2E        string `yaml:"pf2e"`
	Description string `yaml:"description"`
}

// skillsFile is the top-level YAML structure for content/skills.yaml.
type skillsFile struct {
	Skills []*Skill `yaml:"skills"`
}

// LoadSkills reads the skills master YAML file and returns all skill definitions.
//
// Precondition: path must be a readable file containing valid YAML.
// Postcondition: Returns all skills or a non-nil error.
func LoadSkills(path string) ([]*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skills file %s: %w", path, err)
	}
	var f skillsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing skills file %s: %w", path, err)
	}
	return f.Skills, nil
}
