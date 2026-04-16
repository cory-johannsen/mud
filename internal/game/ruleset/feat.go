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
	ConditionID     string `yaml:"condition_id"`     // optional; non-empty means Use applies this condition
	ConditionTarget string `yaml:"condition_target"` // "foe" = apply to combat target; "" or "self" = apply to self
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
	// AoeRadius is the radius in feet of an area-of-effect burst centered on the target grid square.
	// 0 means single-target (default). When > 0 and UseRequest.target_x/target_y are non-zero,
	// the feat's condition effect is applied to every combatant within Chebyshev distance AoeRadius.
	AoeRadius int `yaml:"aoe_radius,omitempty"`
	// RequiresCombat, when true, means this feat may only be activated while the player is in
	// an active combat encounter. Attempting to use it outside combat returns an error message.
	RequiresCombat bool `yaml:"requires_combat,omitempty"`
	// RechargeCondition is a human-readable string describing when limited uses restore.
	// Empty for unlimited-use feats. Examples: "Recharges on rest", "1 per combat".
	RechargeCondition string `yaml:"recharge_condition,omitempty"`
	// ActionCost is the number of action points this active feat costs to use.
	// 0 means the engine defaults to 1 AP. Only meaningful for Active == true feats.
	ActionCost int `yaml:"action_cost,omitempty"`
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
	byPF2E      map[string]*Feat
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
		byPF2E:      make(map[string]*Feat, len(feats)),
		byCategory:  make(map[string][]*Feat),
		bySkill:     make(map[string][]*Feat),
		byArchetype: make(map[string][]*Feat),
	}
	for _, f := range feats {
		r.byID[f.ID] = f
		if f.PF2E != "" {
			r.byPF2E[f.PF2E] = f
		}
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

// NewFeatRegistryFromSlice builds a FeatRegistry from the given feat slice.
// It is an alias for NewFeatRegistry provided for call-site clarity.
//
// Precondition: feats must not be nil.
// Postcondition: Returns a fully indexed registry.
func NewFeatRegistryFromSlice(feats []*Feat) *FeatRegistry {
	return NewFeatRegistry(feats)
}

// Feat returns the feat with the given ID and true, or nil and false if not found.
// Falls back to matching against the pf2e field to resolve legacy IDs.
//
// Precondition: id must be non-empty.
// Postcondition: Returns the feat and true, or nil and false.
func (r *FeatRegistry) Feat(id string) (*Feat, bool) {
	if f, ok := r.byID[id]; ok {
		return f, true
	}
	// Fallback: resolve legacy PF2E IDs (e.g. "rage" → wrath feat).
	if f, ok := r.byPF2E[id]; ok {
		return f, true
	}
	return nil, false
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

// MergeFeatGrants merges two FeatGrants into a single combined grant.
// Fixed lists are unioned; Choices pools are unioned with summed counts.
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned grant contains all fixed IDs and all pool entries from both inputs.
func MergeFeatGrants(a, b *FeatGrants) *FeatGrants {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := &FeatGrants{
		GeneralCount: a.GeneralCount + b.GeneralCount,
		Fixed:        append(append([]string(nil), a.Fixed...), b.Fixed...),
	}
	if a.Choices != nil || b.Choices != nil {
		merged.Choices = &FeatChoices{}
		if a.Choices != nil {
			merged.Choices.Count += a.Choices.Count
			merged.Choices.Pool = append(merged.Choices.Pool, a.Choices.Pool...)
		}
		if b.Choices != nil {
			merged.Choices.Count += b.Choices.Count
			merged.Choices.Pool = append(merged.Choices.Pool, b.Choices.Pool...)
		}
	}
	return merged
}

// MergeFeatLevelUpGrants merges two level-keyed feat grant maps.
// Keys present in both are merged via MergeFeatGrants.
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned map contains all keys from both inputs.
func MergeFeatLevelUpGrants(archetype, job map[int]*FeatGrants) map[int]*FeatGrants {
	if len(archetype) == 0 && len(job) == 0 {
		return nil
	}
	out := make(map[int]*FeatGrants)
	for lvl, g := range archetype {
		out[lvl] = g
	}
	for lvl, g := range job {
		if existing, ok := out[lvl]; ok {
			out[lvl] = MergeFeatGrants(existing, g)
		} else {
			out[lvl] = g
		}
	}
	return out
}
