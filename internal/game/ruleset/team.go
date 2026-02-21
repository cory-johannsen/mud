package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TeamTrait describes a persistent trait shared by all members of a team.
type TeamTrait struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Effect      string `yaml:"effect"`
}

// Team defines a player faction (Gun or Machete) in Gunchete.
// TeamJobs lists the job IDs exclusive to this team.
//
// Precondition: ID and Name must be non-empty after loading.
type Team struct {
	ID          string      `yaml:"id"`
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Traits      []TeamTrait `yaml:"traits"`
	TeamJobs    []string    `yaml:"team_jobs"`
}

// LoadTeams reads all .yaml files in dir and parses each as a Team.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns all parsed teams (may be empty slice) or a non-nil error.
func LoadTeams(dir string) ([]*Team, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	teams := make([]*Team, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var team Team
		if err := yaml.Unmarshal(data, &team); err != nil {
			return nil, fmt.Errorf("parsing team file %s: %w", path, err)
		}
		teams = append(teams, &team)
	}
	return teams, nil
}
