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
)

// validKinds is the set of valid ItemDef kinds.
var validKinds = map[string]bool{
	KindWeapon:     true,
	KindExplosive:  true,
	KindConsumable: true,
	KindJunk:       true,
}

// ItemDef defines the static properties of an inventory item loaded from YAML.
type ItemDef struct {
	ID           string  `yaml:"id"`
	Name         string  `yaml:"name"`
	Description  string  `yaml:"description"`
	Kind         string  `yaml:"kind"`
	Weight       float64 `yaml:"weight"`
	WeaponRef    string  `yaml:"weapon_ref"`
	ExplosiveRef string  `yaml:"explosive_ref"`
	Stackable    bool    `yaml:"stackable"`
	MaxStack     int     `yaml:"max_stack"`
	Value        int     `yaml:"value"`
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
		errs = append(errs, fmt.Errorf("Kind must be one of weapon, explosive, consumable, junk; got %q", d.Kind))
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
	if len(errs) > 0 {
		return fmt.Errorf("item validation failed: %v", errs)
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
