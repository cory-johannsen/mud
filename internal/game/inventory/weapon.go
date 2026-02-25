// Package inventory provides definitions and loaders for weapons and explosives
// used in the MUD game engine.
package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FiringMode represents the firing mode of a ranged weapon.
type FiringMode string

const (
	// FiringModeSingle fires one round per action.
	FiringModeSingle FiringMode = "single"
	// FiringModeBurst fires a short burst per action.
	FiringModeBurst FiringMode = "burst"
	// FiringModeAutomatic fires continuously while the trigger is held.
	FiringModeAutomatic FiringMode = "automatic"
)

// WeaponDef defines the static properties of a weapon loaded from YAML.
type WeaponDef struct {
	ID               string       `yaml:"id"`
	Name             string       `yaml:"name"`
	DamageDice       string       `yaml:"damage_dice"`
	DamageType       string       `yaml:"damage_type"`
	RangeIncrement   int          `yaml:"range_increment"`   // 0 = melee
	ReloadActions    int          `yaml:"reload_actions"`    // 0 = not a firearm
	MagazineCapacity int          `yaml:"magazine_capacity"` // 0 = not a firearm
	FiringModes      []FiringMode `yaml:"firing_modes"`
	Traits           []string     `yaml:"traits"`
}

// IsMelee reports whether the weapon is a melee weapon (RangeIncrement == 0).
func (w *WeaponDef) IsMelee() bool {
	return w.RangeIncrement == 0
}

// IsFirearm reports whether the weapon is a firearm (has at least one firing mode).
func (w *WeaponDef) IsFirearm() bool {
	return len(w.FiringModes) > 0
}

// SupportsBurst reports whether the weapon supports burst fire.
func (w *WeaponDef) SupportsBurst() bool {
	for _, m := range w.FiringModes {
		if m == FiringModeBurst {
			return true
		}
	}
	return false
}

// SupportsAutomatic reports whether the weapon supports automatic fire.
func (w *WeaponDef) SupportsAutomatic() bool {
	for _, m := range w.FiringModes {
		if m == FiringModeAutomatic {
			return true
		}
	}
	return false
}

// Validate checks that the WeaponDef satisfies its invariants.
// Precondition: w is non-nil.
// Postcondition: returns nil iff all fields are valid.
func (w *WeaponDef) Validate() error {
	var errs []error
	if w.ID == "" {
		errs = append(errs, errors.New("ID must not be empty"))
	}
	if w.Name == "" {
		errs = append(errs, errors.New("Name must not be empty"))
	}
	if w.DamageDice == "" {
		errs = append(errs, errors.New("DamageDice must not be empty"))
	}
	if w.DamageType == "" {
		errs = append(errs, errors.New("DamageType must not be empty"))
	}
	if w.IsFirearm() && w.MagazineCapacity <= 0 {
		errs = append(errs, errors.New("firearm MagazineCapacity must be > 0"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("weapon validation failed: %v", errs)
	}
	return nil
}

// LoadWeapons reads all *.yaml files from dir, parses each as a WeaponDef,
// validates it, and returns the collected slice.
// Precondition: dir is a readable directory path.
// Postcondition: returns all valid WeaponDefs or the first encountered error.
func LoadWeapons(dir string) ([]*WeaponDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("LoadWeapons: cannot read directory %q: %w", dir, err)
	}

	var weapons []*WeaponDef
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadWeapons: cannot read file %q: %w", path, err)
		}
		var w WeaponDef
		if err := yaml.Unmarshal(data, &w); err != nil {
			return nil, fmt.Errorf("LoadWeapons: cannot parse file %q: %w", path, err)
		}
		if err := w.Validate(); err != nil {
			return nil, fmt.Errorf("LoadWeapons: invalid weapon in %q: %w", path, err)
		}
		weapons = append(weapons, &w)
	}
	return weapons, nil
}
