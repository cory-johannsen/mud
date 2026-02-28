package ruleset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Region defines a home region (PF2E ancestry replacement) for character creation.
//
// Precondition: ID and Name must be non-empty after loading.
type Region struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Article     string         `yaml:"article"`
	Description string         `yaml:"description"`
	Modifiers   map[string]int `yaml:"modifiers"`
	Traits      []string       `yaml:"traits"`
}

// DisplayName returns the human-readable region name with its grammatical article.
// If Article is empty, returns Name alone.
//
// Precondition: Name must be non-empty.
// Postcondition: Returns a non-empty string.
func (r *Region) DisplayName() string {
	if r.Article == "" {
		return r.Name
	}
	return r.Article + " " + r.Name
}

// LoadRegions reads all .yaml files in dir and parses each as a Region.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns all parsed regions (may be empty slice) or a non-nil error.
func LoadRegions(dir string) ([]*Region, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	regions := make([]*Region, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var r Region
		if err := yaml.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("parsing region file %s: %w", path, err)
		}
		regions = append(regions, &r)
	}
	return regions, nil
}

func yamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	return paths, nil
}
