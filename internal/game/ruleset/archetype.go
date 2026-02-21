package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Archetype defines a broad character archetype (replaces PF2E core class grouping).
//
// Precondition: ID, Name, KeyAbility, and HitPointsPerLevel must be non-zero after loading.
type Archetype struct {
	ID                string `yaml:"id"`
	Name              string `yaml:"name"`
	Description       string `yaml:"description"`
	KeyAbility        string `yaml:"key_ability"`
	HitPointsPerLevel int    `yaml:"hit_points_per_level"`
}

// LoadArchetypes reads all .yaml files in dir and parses each as an Archetype.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns all parsed archetypes (may be empty slice) or a non-nil error.
func LoadArchetypes(dir string) ([]*Archetype, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	archetypes := make([]*Archetype, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var a Archetype
		if err := yaml.Unmarshal(data, &a); err != nil {
			return nil, fmt.Errorf("parsing archetype file %s: %w", path, err)
		}
		archetypes = append(archetypes, &a)
	}
	return archetypes, nil
}
