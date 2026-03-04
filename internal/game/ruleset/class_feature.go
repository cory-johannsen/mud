package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ClassFeature defines one Gunchete class feature and its P2FE equivalent.
//
// Archetype is non-empty for archetype-shared features; Job is non-empty for job-specific features.
// Active features require player action to use; passive features are always-on.
type ClassFeature struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Archetype    string `yaml:"archetype"`
	Job          string `yaml:"job"`
	PF2E         string `yaml:"pf2e"`
	Active       bool   `yaml:"active"`
	ActivateText string `yaml:"activate_text"`
	Description  string `yaml:"description"`
}

// classFeaturesFile is the top-level YAML structure for content/class_features.yaml.
type classFeaturesFile struct {
	ClassFeatures []*ClassFeature `yaml:"class_features"`
}

// LoadClassFeatures reads the class features master YAML file and returns all feature definitions.
//
// Precondition: path must be a readable file containing valid YAML.
// Postcondition: Returns all class features or a non-nil error.
func LoadClassFeatures(path string) ([]*ClassFeature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading class features file %s: %w", path, err)
	}
	var f classFeaturesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing class features file %s: %w", path, err)
	}
	return f.ClassFeatures, nil
}

// ClassFeatureRegistry provides fast lookup of class features by ID, archetype, and job.
type ClassFeatureRegistry struct {
	byID        map[string]*ClassFeature
	byArchetype map[string][]*ClassFeature
	byJob       map[string][]*ClassFeature
}

// NewClassFeatureRegistry builds a ClassFeatureRegistry from the given feature slice.
//
// Precondition: features must not be nil.
// Postcondition: Returns a fully indexed registry.
func NewClassFeatureRegistry(features []*ClassFeature) *ClassFeatureRegistry {
	r := &ClassFeatureRegistry{
		byID:        make(map[string]*ClassFeature),
		byArchetype: make(map[string][]*ClassFeature),
		byJob:       make(map[string][]*ClassFeature),
	}
	for _, f := range features {
		r.byID[f.ID] = f
		if f.Archetype != "" {
			r.byArchetype[f.Archetype] = append(r.byArchetype[f.Archetype], f)
		}
		if f.Job != "" {
			r.byJob[f.Job] = append(r.byJob[f.Job], f)
		}
	}
	return r
}

// ClassFeature returns the feature with the given ID, or false if not found.
//
// Precondition: id must be non-empty.
func (r *ClassFeatureRegistry) ClassFeature(id string) (*ClassFeature, bool) {
	f, ok := r.byID[id]
	return f, ok
}

// ByArchetype returns all features for the given archetype (shared features).
//
// Precondition: archetype must be non-empty.
// Postcondition: Returns a slice (may be empty).
func (r *ClassFeatureRegistry) ByArchetype(archetype string) []*ClassFeature {
	return r.byArchetype[archetype]
}

// ByJob returns all features for the given job (job-specific features).
//
// Precondition: job must be non-empty.
// Postcondition: Returns a slice (may be empty).
func (r *ClassFeatureRegistry) ByJob(job string) []*ClassFeature {
	return r.byJob[job]
}
