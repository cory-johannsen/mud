package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

// Feat defines one Gunchete feat and its P2FE equivalent.
//
// Category is one of: "general", "skill", "job".
// Skill is non-empty only for category="skill"; Archetype is non-empty only for category="job".
// Active feats require player action to use; passive feats are always-on.
type Feat struct {
	ID           string          `yaml:"id"`
	Name         string          `yaml:"name"`
	Category     string          `yaml:"category"`
	Skill        string          `yaml:"skill"`
	Archetype    string          `yaml:"archetype"`
	PF2E         string          `yaml:"pf2e"`
	Active       bool            `yaml:"active"`
	ActivateText string          `yaml:"activate_text"`
	ConditionID  string          `yaml:"condition_id"` // optional; non-empty means Use applies this condition
	Description  string          `yaml:"description"`
	Choices      *FeatureChoices `yaml:"choices"`
	// PreparedUses is the number of times per long rest this active feat may be activated.
	// 0 means unlimited (no use count is tracked or enforced).
	PreparedUses int `yaml:"prepared_uses"`
	// Reaction declares this feat as a player reaction with the given trigger and effect.
	// Nil means this feat is not a reaction.
	Reaction *reaction.ReactionDef `yaml:"reaction,omitempty"`
	// AllowNPC when true allows this feat to be assigned to NPC templates.
	// Default false. Only feats with AllowNPC == true may appear in Template.Feats.
	AllowNPC bool `yaml:"allow_npc"`
	// TargetTags is an optional list of NPC tags; when non-empty, the feat bonus
	// applies only when the combat target has at least one matching tag.
	TargetTags []string `yaml:"target_tags"`
	// GrantsFocusPoint when true means this feat grants one Focus Point slot to the character's pool.
	GrantsFocusPoint bool `yaml:"grants_focus_point,omitempty"`
}

// featsFile is the top-level YAML structure for content/feats.yaml.
type featsFile struct {
	Feats []*Feat `yaml:"feats"`
}

// LoadFeatsFromBytes parses feats from raw YAML bytes.
//
// Precondition: data must be valid YAML matching the featsFile schema.
// Postcondition: Returns all feats or a non-nil error.
func LoadFeatsFromBytes(data []byte) ([]*Feat, error) {
	var f featsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing feats: %w", err)
	}
	return f.Feats, nil
}

// LoadFeats reads the feats master YAML file and returns all feat definitions.
//
// Precondition: path must be a readable file containing valid YAML.
// Postcondition: Returns all feats or a non-nil error.
func LoadFeats(path string) ([]*Feat, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading feats file %s: %w", path, err)
	}
	return LoadFeatsFromBytes(data)
}

// FeatRegistry provides fast lookup of feats by ID, category, skill, and archetype.
type FeatRegistry struct {
	byID        map[string]*Feat
	byCategory  map[string][]*Feat
	bySkill     map[string][]*Feat
	byArchetype map[string][]*Feat
}

// NewFeatRegistry builds a FeatRegistry from the given feat slice.
//
// Precondition: feats must not be nil.
// Postcondition: Returns a fully indexed registry.
func NewFeatRegistry(feats []*Feat) *FeatRegistry {
	r := &FeatRegistry{
		byID:        make(map[string]*Feat, len(feats)),
		byCategory:  make(map[string][]*Feat),
		bySkill:     make(map[string][]*Feat),
		byArchetype: make(map[string][]*Feat),
	}
	for _, f := range feats {
		r.byID[f.ID] = f
		r.byCategory[f.Category] = append(r.byCategory[f.Category], f)
		if f.Skill != "" {
			r.bySkill[f.Skill] = append(r.bySkill[f.Skill], f)
		}
		if f.Archetype != "" {
			r.byArchetype[f.Archetype] = append(r.byArchetype[f.Archetype], f)
		}
	}
	return r
}

// Feat returns the feat with the given ID and true, or nil and false if not found.
//
// Precondition: id must be non-empty.
// Postcondition: Returns the feat and true, or nil and false.
func (r *FeatRegistry) Feat(id string) (*Feat, bool) {
	f, ok := r.byID[id]
	return f, ok
}

// ByCategory returns all feats in the given category.
//
// Precondition: category must be non-empty.
// Postcondition: Returns a slice (may be empty).
func (r *FeatRegistry) ByCategory(category string) []*Feat {
	return r.byCategory[category]
}

// BySkill returns all skill feats unlocked by the given skill ID.
//
// Precondition: skillID must be non-empty.
// Postcondition: Returns a slice (may be empty).
func (r *FeatRegistry) BySkill(skillID string) []*Feat {
	return r.bySkill[skillID]
}

// ByArchetype returns all job feats for the given archetype.
//
// Precondition: archetype must be non-empty.
// Postcondition: Returns a slice (may be empty).
func (r *FeatRegistry) ByArchetype(archetype string) []*Feat {
	return r.byArchetype[archetype]
}

// SkillFeatsForTrainedSkills returns the union of skill feat pools for all trained skills.
// trained is a map of skill_id → proficiency (only "trained" or better are included).
func (r *FeatRegistry) SkillFeatsForTrainedSkills(trained map[string]string) []*Feat {
	seen := make(map[string]bool)
	var out []*Feat
	for skillID, prof := range trained {
		if prof == "untrained" {
			continue
		}
		for _, f := range r.bySkill[skillID] {
			if !seen[f.ID] {
				seen[f.ID] = true
				out = append(out, f)
			}
		}
	}
	return out
}
