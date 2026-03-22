package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Kind constants for ItemDef.Kind.
const (
	KindWeapon     = "weapon"
	KindExplosive  = "explosive"
	KindConsumable = "consumable"
	KindJunk       = "junk"
	KindArmor      = "armor"
	KindTrap       = "trap"
)

// validKinds is the set of valid ItemDef kinds.
var validKinds = map[string]bool{
	KindWeapon:     true,
	KindExplosive:  true,
	KindConsumable: true,
	KindJunk:       true,
	KindArmor:      true,
	KindTrap:       true,
}

// ItemDef defines the static properties of an inventory item loaded from YAML.
type ItemDef struct {
	ID           string  `yaml:"id"`
	Name         string  `yaml:"name"`
	Description  string  `yaml:"description"`
	Kind         string  `yaml:"kind"`
	Weight       float64 `yaml:"weight"`
	WeaponRef    string  `yaml:"weapon_ref"`
	ArmorRef     string  `yaml:"armor_ref"`    // references an ArmorDef ID; set when Kind == "armor"
	ExplosiveRef    string  `yaml:"explosive_ref"`
	TrapTemplateRef string  `yaml:"trap_template_ref"` // references a TrapTemplate ID; set when Kind == "trap"
	Stackable       bool    `yaml:"stackable"`
	MaxStack     int     `yaml:"max_stack"`
	Value        int     `yaml:"value"`
	// Team is optional; "gun" | "machete" | "". Applies a team effectiveness multiplier
	// when used as a consumable (REQ-EM-36 through REQ-EM-39).
	Team   string           `yaml:"team,omitempty"`
	// Effect holds consumable effect data; nil for non-consumable items.
	Effect *ConsumableEffect `yaml:"effect,omitempty"`
}

// Validate checks that the ItemDef satisfies its invariants.
//
// Precondition: d is non-nil.
// Postcondition: returns nil iff all fields are valid.
func (d *ItemDef) Validate() error {
	var errs []error
	if d.ID == "" {
		errs = append(errs, errors.New("ID must not be empty"))
	}
	if d.Name == "" {
		errs = append(errs, errors.New("Name must not be empty"))
	}
	if !validKinds[d.Kind] {
		errs = append(errs, fmt.Errorf("Kind must be one of weapon, explosive, consumable, junk, armor, trap; got %q", d.Kind))
	}
	if d.MaxStack < 1 {
		errs = append(errs, errors.New("MaxStack must be >= 1"))
	}
	if d.Weight < 0 {
		errs = append(errs, errors.New("Weight must be >= 0"))
	}
	if d.Kind == KindWeapon && d.WeaponRef == "" {
		errs = append(errs, errors.New("WeaponRef is required when Kind is weapon"))
	}
	if d.Kind == KindExplosive && d.ExplosiveRef == "" {
		errs = append(errs, errors.New("ExplosiveRef is required when Kind is explosive"))
	}
	if d.Kind == KindArmor && d.ArmorRef == "" {
		errs = append(errs, errors.New("ArmorRef is required when Kind is armor"))
	}
	if d.Kind == KindTrap && d.TrapTemplateRef == "" {
		errs = append(errs, fmt.Errorf("item %q: TrapTemplateRef is required when Kind is %q", d.ID, KindTrap))
	}
	// REQ-EM-36: Team must be "" | "gun" | "machete".
	if d.Team != "" && d.Team != "gun" && d.Team != "machete" {
		errs = append(errs, fmt.Errorf("Team %q is not valid; must be \"gun\", \"machete\", or \"\"", d.Team))
	}
	if len(errs) > 0 {
		return fmt.Errorf("item validation failed: %v", errs)
	}
	return nil
}

// requiredConsumableIDs lists the six consumable item IDs that MUST be present
// at startup (REQ-EM-40).
var requiredConsumableIDs = []string{
	"whores_pasta",
	"poontangesca",
	"four_loko",
	"old_english",
	"penjamin_franklin",
	"repair_kit",
}

// ValidateRequiredConsumables checks that all six required consumable item IDs
// are present in items (REQ-EM-40). Missing IDs produce a fatal error.
//
// Precondition: items may be nil (treated as empty).
// Postcondition: returns nil iff all required IDs are present.
func ValidateRequiredConsumables(items []*ItemDef) error {
	present := make(map[string]bool, len(items))
	for _, it := range items {
		present[it.ID] = true
	}
	var missing []string
	for _, id := range requiredConsumableIDs {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("LoadItems: required consumable item(s) missing: %v", missing)
	}
	return nil
}

// ValidateConsumableEffects checks that all disease_id and toxin_id values
// referenced in consumable effects appear in knownConditions (REQ-EM-42).
// An unresolvable ID is a fatal load error.
//
// Precondition: knownConditions may be nil (treated as empty — all IDs unknown).
// Postcondition: returns nil iff all referenced IDs are present in knownConditions.
func ValidateConsumableEffects(items []*ItemDef, knownConditions map[string]bool) error {
	for _, it := range items {
		if it.Effect == nil {
			continue
		}
		cc := it.Effect.ConsumeCheck
		if cc == nil || cc.OnCriticalFailure == nil {
			continue
		}
		cf := cc.OnCriticalFailure
		if cf.ApplyDisease != nil {
			id := cf.ApplyDisease.DiseaseID
			if !knownConditions[id] {
				return fmt.Errorf("item %q: disease_id %q is not a known condition ID (REQ-EM-42)", it.ID, id)
			}
		}
		if cf.ApplyToxin != nil {
			id := cf.ApplyToxin.ToxinID
			if !knownConditions[id] {
				return fmt.Errorf("item %q: toxin_id %q is not a known condition ID (REQ-EM-42)", it.ID, id)
			}
		}
	}
	return nil
}

// LoadItems reads all *.yaml and *.yml files from dir, parses each as an
// ItemDef, validates it, and returns the collected slice.
//
// Precondition: dir is a readable directory path.
// Postcondition: returns all valid ItemDefs or the first encountered error.
func LoadItems(dir string) ([]*ItemDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("LoadItems: cannot read directory %q: %w", dir, err)
	}

	var items []*ItemDef
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadItems: cannot read file %q: %w", path, err)
		}
		var d ItemDef
		if err := yaml.Unmarshal(data, &d); err != nil {
			return nil, fmt.Errorf("LoadItems: cannot parse file %q: %w", path, err)
		}
		if err := d.Validate(); err != nil {
			return nil, fmt.Errorf("LoadItems: invalid item in %q: %w", path, err)
		}
		items = append(items, &d)
	}
	return items, nil
}
