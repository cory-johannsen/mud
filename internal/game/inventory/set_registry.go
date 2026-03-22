package inventory

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SetThreshold represents the piece count threshold for a set bonus.
// It unmarshals from either an integer or the string "full".
type SetThreshold struct {
	// IsFull is true when the threshold was specified as "full" in YAML.
	IsFull bool
	// Count is the resolved piece count (set to len(pieces) when IsFull is true after load).
	Count int
}

// UnmarshalYAML implements yaml.Unmarshaler to parse "full" or an integer.
func (t *SetThreshold) UnmarshalYAML(value *yaml.Node) error {
	// Try integer first.
	var n int
	if err := value.Decode(&n); err == nil {
		t.Count = n
		t.IsFull = false
		return nil
	}
	// Try string "full".
	var s string
	if err := value.Decode(&s); err != nil {
		return fmt.Errorf("set threshold must be an integer or \"full\", got: %v", value.Value)
	}
	if s != "full" {
		return fmt.Errorf("set threshold string must be \"full\", got %q", s)
	}
	t.IsFull = true
	t.Count = 0
	return nil
}

// SetPiece is a single item in an equipment set definition.
type SetPiece struct {
	ItemDefID string `yaml:"item_def_id"`
}

// SetEffect describes the mechanical effect of a set bonus.
type SetEffect struct {
	Type        string `yaml:"type"`
	Skill       string `yaml:"skill,omitempty"`
	Stat        string `yaml:"stat,omitempty"`
	ConditionID string `yaml:"condition_id,omitempty"`
	Amount      int    `yaml:"amount,omitempty"`
}

// SetBonus defines a single bonus granted when a piece threshold is met.
type SetBonus struct {
	Threshold   SetThreshold `yaml:"threshold"`
	Description string       `yaml:"description"`
	Effect      SetEffect    `yaml:"effect"`
}

// SetDef is a complete equipment set definition loaded from YAML.
type SetDef struct {
	ID      string     `yaml:"id"`
	Name    string     `yaml:"name"`
	Pieces  []SetPiece `yaml:"pieces"`
	Bonuses []SetBonus `yaml:"bonuses"`
}

// validSetEffectTypes is the set of recognized set bonus effect type strings.
var validSetEffectTypes = map[string]bool{
	"skill_bonus":         true,
	"ac_bonus":            true,
	"speed_bonus":         true,
	"stat_bonus":          true,
	"condition_immunity":  true,
}

// SetRegistry holds all loaded equipment set definitions.
type SetRegistry struct {
	sets []*SetDef
}

// AllSets returns all loaded SetDefs.
//
// Postcondition: returned slice is a defensive copy; mutations do not affect the registry.
func (r *SetRegistry) AllSets() []*SetDef {
	out := make([]*SetDef, len(r.sets))
	copy(out, r.sets)
	return out
}

// ActiveBonuses returns all SetBonuses whose piece-count thresholds are met
// by the given slice of equipped item definition IDs.
//
// This is a pure function with no side effects (REQ-EM-34).
//
// Precondition: equippedItemIDs may be nil or empty.
// Postcondition: returned slice contains only bonuses whose Count <= number of
// set pieces present in equippedItemIDs. A piece is counted at most once per set
// (duplicate item def IDs are deduplicated per set).
func (r *SetRegistry) ActiveBonuses(equippedItemIDs []string) []SetBonus {
	// Build a set for fast lookup.
	equipped := make(map[string]bool, len(equippedItemIDs))
	for _, id := range equippedItemIDs {
		equipped[id] = true
	}

	var result []SetBonus
	for _, def := range r.sets {
		// Count how many pieces of this set are equipped (each piece counted once).
		count := 0
		for _, piece := range def.Pieces {
			if equipped[piece.ItemDefID] {
				count++
			}
		}
		// Collect bonuses whose threshold is met.
		for _, bonus := range def.Bonuses {
			if count >= bonus.Threshold.Count {
				result = append(result, bonus)
			}
		}
	}
	return result
}

// SetBonusSummary aggregates all active set bonus effects into a single struct
// for efficient consultation at skill check, speed, stat, and condition resolution.
type SetBonusSummary struct {
	// ACBonus is the total flat AC bonus from all active set bonuses.
	ACBonus int
	// SkillBonuses maps skill ID → total skill bonus from all active set bonuses.
	SkillBonuses map[string]int
	// SpeedBonus is the total flat speed bonus from all active set bonuses.
	SpeedBonus int
	// StatBonuses maps stat ID → total stat bonus from all active set bonuses.
	StatBonuses map[string]int
	// ConditionImmunities is the list of condition IDs the character is immune to.
	ConditionImmunities []string
}

// ComputeSetBonusSummary aggregates a slice of active SetBonuses into a SetBonusSummary.
//
// Precondition: bonuses may be nil.
// Postcondition: all maps in the result are non-nil.
func ComputeSetBonusSummary(bonuses []SetBonus) SetBonusSummary {
	s := SetBonusSummary{
		SkillBonuses: make(map[string]int),
		StatBonuses:  make(map[string]int),
	}
	for _, b := range bonuses {
		switch b.Effect.Type {
		case "ac_bonus":
			s.ACBonus += b.Effect.Amount
		case "speed_bonus":
			s.SpeedBonus += b.Effect.Amount
		case "skill_bonus":
			s.SkillBonuses[b.Effect.Skill] += b.Effect.Amount
		case "stat_bonus":
			s.StatBonuses[b.Effect.Stat] += b.Effect.Amount
		case "condition_immunity":
			s.ConditionImmunities = append(s.ConditionImmunities, b.Effect.ConditionID)
		}
	}
	return s
}

// LoadSetRegistry reads all *.yaml files from dir, parses each as a SetDef, validates it,
// resolves "full" thresholds to piece counts, and returns a SetRegistry.
//
// Preconditions:
//   - dir must be a readable directory path.
//   - knownConditions is the set of valid condition IDs (may be nil, in which case
//     condition_immunity effects are accepted without validation).
//
// Postconditions:
//   - Returns a non-nil *SetRegistry and nil error on success.
//   - Returns a non-nil error if any set file is invalid, contains an unrecognized effect type,
//     or references an unknown condition ID.
func LoadSetRegistry(dir string, knownConditions map[string]bool) (*SetRegistry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &SetRegistry{}, nil
		}
		return nil, fmt.Errorf("LoadSetRegistry: cannot read directory %q: %w", dir, err)
	}

	var sets []*SetDef
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadSetRegistry: cannot read %q: %w", path, err)
		}
		var def SetDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("LoadSetRegistry: cannot parse %q: %w", path, err)
		}
		// Validate and resolve thresholds.
		for i := range def.Bonuses {
			bonus := &def.Bonuses[i]
			if !validSetEffectTypes[bonus.Effect.Type] {
				return nil, fmt.Errorf("LoadSetRegistry: set %q bonus %d has unrecognized effect type %q",
					def.ID, i, bonus.Effect.Type)
			}
			if bonus.Effect.Type == "condition_immunity" && knownConditions != nil {
				if !knownConditions[bonus.Effect.ConditionID] {
					return nil, fmt.Errorf("LoadSetRegistry: set %q bonus %d references unknown condition_id %q",
						def.ID, i, bonus.Effect.ConditionID)
				}
			}
			if bonus.Threshold.IsFull {
				bonus.Threshold.Count = len(def.Pieces)
			}
		}
		sets = append(sets, &def)
	}
	return &SetRegistry{sets: sets}, nil
}

// ParseYAML is a helper that parses a YAML file at path into v.
// This is exposed for testing purposes.
//
// Precondition: path is a readable YAML file.
// Postcondition: v is populated on success.
func ParseYAML(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}
