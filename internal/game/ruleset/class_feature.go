package ruleset

import (
	"fmt"
	"os"
	"sort"

	"github.com/cory-johannsen/mud/internal/game/effect"
	"gopkg.in/yaml.v3"
)

// FeatureChoices declares an interactive player choice attached to a feature or feat.
// Precondition: Options must be non-empty; Key and Prompt must be non-empty strings.
type FeatureChoices struct {
	Key     string   `yaml:"key"`
	Prompt  string   `yaml:"prompt"`
	Options []string `yaml:"options"`
}

// ActionEffect describes the mechanical outcome of activating an action.
//
// Precondition: Type must be one of "condition", "heal", "damage", "skill_check".
type ActionEffect struct {
	Type        string `yaml:"type"`         // condition | heal | damage | skill_check
	Target      string `yaml:"target"`       // self | target
	ConditionID string `yaml:"condition_id"` // for type=condition
	Amount      string `yaml:"amount"`       // for type=heal|damage (dice string or flat int)
	DamageType  string `yaml:"damage_type"`  // for type=damage
	Skill       string `yaml:"skill"`        // for type=skill_check
	DC          int    `yaml:"dc"`           // for type=skill_check
}

// ClassFeature defines one Gunchete class feature and its P2FE equivalent.
//
// Archetype is non-empty for archetype-shared features; Job is non-empty for job-specific features.
// Active features require player action to use; passive features are always-on.
type ClassFeature struct {
	ID           string          `yaml:"id"`
	Name         string          `yaml:"name"`
	Archetype    string          `yaml:"archetype"`
	Job          string          `yaml:"job"`
	PF2E         string          `yaml:"pf2e"`
	Active       bool            `yaml:"active"`
	ActivateText string          `yaml:"activate_text"`
	ConditionID  string          `yaml:"condition_id"` // optional; non-empty means Use applies this condition
	Description  string          `yaml:"description"`
	Choices      *FeatureChoices `yaml:"choices"`
	Shortcut     string          `yaml:"shortcut"`    // direct command alias; empty = no shortcut
	ActionCost   int             `yaml:"action_cost"` // AP cost in combat; 1, 2, or 3
	Contexts     []string        `yaml:"contexts"`    // valid contexts: combat, exploration, downtime
	Effect            *ActionEffect   `yaml:"effect"`                        // nil for passive features
	GrantsFocusPoint  bool            `yaml:"grants_focus_point,omitempty"`  // true if this feature grants a Focus Point slot
	// AoeRadius is the radius in feet for area-of-effect feats. 0 = single target.
	AoeRadius int `yaml:"aoe_radius,omitempty"`
	// PassiveBonuses are always-on typed bonuses granted while this feature is active.
	// Only meaningful when Active == false (passive features).
	PassiveBonuses []effect.Bonus `yaml:"passive_bonuses,omitempty"`
}

// classFeaturesFile is the top-level YAML structure for content/class_features.yaml.
type classFeaturesFile struct {
	ClassFeatures []*ClassFeature `yaml:"class_features"`
}

// LoadClassFeaturesFromBytes parses class features from raw YAML bytes.
//
// Precondition: data must be valid YAML matching the classFeaturesFile schema.
// Postcondition: Returns all class features or a non-nil error.
func LoadClassFeaturesFromBytes(data []byte) ([]*ClassFeature, error) {
	var f classFeaturesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing class features: %w", err)
	}
	return f.ClassFeatures, nil
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
	return LoadClassFeaturesFromBytes(data)
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

// ActiveFeatures returns all features that are active (player-activated).
//
// Postcondition: Returns a slice of all active features; may be empty.
func (r *ClassFeatureRegistry) ActiveFeatures() []*ClassFeature {
	var out []*ClassFeature
	for _, f := range r.byID {
		if f.Active {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
