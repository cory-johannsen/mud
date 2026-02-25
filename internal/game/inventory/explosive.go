package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AreaType represents the area of effect for an explosive.
type AreaType string

const (
	// AreaTypeRoom affects all targets in the same room.
	AreaTypeRoom AreaType = "room"
	// AreaTypeBurst affects targets within a burst radius.
	AreaTypeBurst AreaType = "burst"
)

// FuseType represents when an explosive detonates.
type FuseType string

const (
	// FuseImmediate detonates upon use.
	FuseImmediate FuseType = "immediate"
	// FuseDelayed detonates after a delay.
	FuseDelayed FuseType = "delayed"
)

// ExplosiveDef defines the static properties of an explosive loaded from YAML.
type ExplosiveDef struct {
	ID         string   `yaml:"id"`
	Name       string   `yaml:"name"`
	DamageDice string   `yaml:"damage_dice"`
	DamageType string   `yaml:"damage_type"`
	AreaType   AreaType `yaml:"area_type"`
	SaveType   string   `yaml:"save_type"` // e.g. "reflex"
	SaveDC     int      `yaml:"save_dc"`
	Fuse       FuseType `yaml:"fuse"`
	Traits     []string `yaml:"traits"`
}

// Validate checks that the ExplosiveDef satisfies its invariants.
// Precondition: e is non-nil.
// Postcondition: returns nil iff all fields are valid.
func (e *ExplosiveDef) Validate() error {
	var errs []error
	if e.ID == "" {
		errs = append(errs, errors.New("ID must not be empty"))
	}
	if e.Name == "" {
		errs = append(errs, errors.New("Name must not be empty"))
	}
	if e.DamageDice == "" {
		errs = append(errs, errors.New("DamageDice must not be empty"))
	}
	if e.DamageType == "" {
		errs = append(errs, errors.New("DamageType must not be empty"))
	}
	if e.SaveType == "" {
		errs = append(errs, errors.New("SaveType must not be empty"))
	}
	if e.SaveDC <= 0 {
		errs = append(errs, errors.New("SaveDC must be > 0"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("explosive validation failed: %v", errs)
	}
	return nil
}

// LoadExplosives reads all *.yaml files from dir, parses each as an ExplosiveDef,
// validates it, and returns the collected slice.
// Precondition: dir is a readable directory path.
// Postcondition: returns all valid ExplosiveDefs or the first encountered error.
func LoadExplosives(dir string) ([]*ExplosiveDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("LoadExplosives: cannot read directory %q: %w", dir, err)
	}

	var explosives []*ExplosiveDef
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadExplosives: cannot read file %q: %w", path, err)
		}
		var e ExplosiveDef
		if err := yaml.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("LoadExplosives: cannot parse file %q: %w", path, err)
		}
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("LoadExplosives: invalid explosive in %q: %w", path, err)
		}
		explosives = append(explosives, &e)
	}
	return explosives, nil
}
