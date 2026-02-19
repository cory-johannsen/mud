package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ClassFeature describes a single class feature gained at a specific level.
type ClassFeature struct {
	Name        string `yaml:"name"`
	Level       int    `yaml:"level"`
	Description string `yaml:"description"`
}

// Class defines a playable character class for character creation.
//
// Precondition: ID, Name, and KeyAbility must be non-empty after loading.
type Class struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	Description       string            `yaml:"description"`
	KeyAbility        string            `yaml:"key_ability"`
	HitPointsPerLevel int               `yaml:"hit_points_per_level"`
	Proficiencies     map[string]string `yaml:"proficiencies"`
	Features          []ClassFeature    `yaml:"features"`
}

// LoadClasses reads all .yaml files in dir and parses each as a Class.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns all parsed classes (may be empty slice) or a non-nil error.
func LoadClasses(dir string) ([]*Class, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	classes := make([]*Class, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var c Class
		if err := yaml.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parsing class file %s: %w", path, err)
		}
		classes = append(classes, &c)
	}
	return classes, nil
}
