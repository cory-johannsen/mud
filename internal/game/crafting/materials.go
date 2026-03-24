package crafting

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Material represents a raw crafting ingredient with an ID, human-readable name,
// category grouping, and base trade value.
type Material struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	Value    int    `yaml:"value"`
}

type materialFile struct {
	Materials []Material `yaml:"materials"`
}

// MaterialRegistry provides O(1) lookup of materials by ID.
type MaterialRegistry struct {
	materials map[string]*Material
}

// LoadMaterialRegistry reads the YAML file at path and returns a populated registry.
// Precondition: path must point to a readable YAML file with a top-level "materials" list.
// Postcondition: every entry in the file is accessible via Material(id).
func LoadMaterialRegistry(path string) (*MaterialRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read materials: %w", err)
	}
	var f materialFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse materials: %w", err)
	}
	reg := &MaterialRegistry{materials: make(map[string]*Material, len(f.Materials))}
	for i := range f.Materials {
		m := &f.Materials[i]
		reg.materials[m.ID] = m
	}
	return reg, nil
}

// Material returns the Material with the given id and true, or nil and false if not found.
func (r *MaterialRegistry) Material(id string) (*Material, bool) {
	m, ok := r.materials[id]
	return m, ok
}

// All returns all materials in the registry in unspecified order.
func (r *MaterialRegistry) All() []*Material {
	out := make([]*Material, 0, len(r.materials))
	for _, m := range r.materials {
		out = append(out, m)
	}
	return out
}

// HasID reports whether a material with the given id exists in the registry.
func (r *MaterialRegistry) HasID(id string) bool {
	_, ok := r.materials[id]
	return ok
}
