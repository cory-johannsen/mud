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

// WeaponKind categorises a weapon for equip-slot constraint enforcement.
type WeaponKind string

const (
	// WeaponKindOneHanded fits in main hand or off-hand; enables dual wield.
	WeaponKindOneHanded WeaponKind = "one_handed"
	// WeaponKindTwoHanded occupies the main hand and locks off-hand empty.
	WeaponKindTwoHanded WeaponKind = "two_handed"
	// WeaponKindShield goes in the off-hand only; main hand must be one-handed or empty.
	WeaponKindShield WeaponKind = "shield"
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
	Kind                 WeaponKind   `yaml:"kind"`
	Group                string           `yaml:"group"`                // e.g. "blade", "club", "firearm", "brawling", "energy"
	ProficiencyCategory  string           `yaml:"proficiency_category"` // e.g. "simple_weapons", "martial_ranged", "specialized"
	TeamAffinity     string           `yaml:"team_affinity"`     // "gun", "machete", or ""
	CrossTeamEffect  *CrossTeamEffect `yaml:"cross_team_effect"` // nil = no side effect
	// Rarity is required (REQ-EM-1). One of: salvage, street, mil_spec, black_market, ghost.
	Rarity string `yaml:"rarity"`
	// RarityStatMultiplier is set at load time from the rarity tier constants (REQ-EM-2).
	// It is NOT loaded from YAML — it is derived from Rarity after parsing.
	RarityStatMultiplier float64 `yaml:"-"`
	// UpgradeSlots is the number of material upgrade slots available on this weapon.
	// Derived from RarityDef.FeatureSlots at load time. NOT loaded from YAML.
	UpgradeSlots int `yaml:"-"`
	Hardness     int `yaml:"hardness"`
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

// IsOneHanded reports whether the weapon is one-handed.
func (w *WeaponDef) IsOneHanded() bool { return w.Kind == WeaponKindOneHanded }

// IsTwoHanded reports whether the weapon is two-handed.
func (w *WeaponDef) IsTwoHanded() bool { return w.Kind == WeaponKindTwoHanded }

// IsShield reports whether the weapon is a shield.
func (w *WeaponDef) IsShield() bool { return w.Kind == WeaponKindShield }

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
	validWeaponProfCategories := map[string]bool{
		"simple_weapons": true, "simple_ranged": true, "martial_weapons": true,
		"martial_ranged": true, "martial_melee": true, "unarmed": true, "specialized": true,
	}
	if !validWeaponProfCategories[w.ProficiencyCategory] {
		errs = append(errs, fmt.Errorf("proficiency_category %q is not valid", w.ProficiencyCategory))
	}
	// REQ-EM-1: rarity is required.
	if _, ok := LookupRarity(w.Rarity); !ok {
		errs = append(errs, fmt.Errorf("rarity %q is not valid; must be one of salvage, street, mil_spec, black_market, ghost", w.Rarity))
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
		// REQ-EM-2: set RarityStatMultiplier and UpgradeSlots from the rarity constants at load time.
		if def, ok := LookupRarity(w.Rarity); ok {
			w.RarityStatMultiplier = def.StatMultiplier
			w.UpgradeSlots = def.FeatureSlots
		}
		weapons = append(weapons, &w)
	}
	return weapons, nil
}
